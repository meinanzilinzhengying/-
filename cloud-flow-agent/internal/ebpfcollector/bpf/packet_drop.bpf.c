// SPDX-License-Identifier: GPL-2.0 OR BSD-3-Clause
/*
 * packet_drop.bpf.c - eBPF 内核态丢包监控程序
 *
 * 功能：
 * 1. 监控内核网络栈丢包事件（kfree_skb）
 * 2. 监控 ring buffer 溢出（通过 tracepoint）
 * 3. 按丢包原因分类统计
 * 4. 采集丢包时的五元组信息
 *
 * Copyright (c) 2024 cloud-flow contributors
 */

#include "vmlinux.h"
#include <bpf/bpf_helpers.h>
#include <bpf/bpf_tracing.h>
#include <bpf/bpf_core_read.h>

/* =======================================================================
 * 常量定义
 * ======================================================================= */

#define MAX_DROP_REASONS 64     /* 最大丢包原因数 */
#define MAX_TRACKED_FLOWS 1024  /* 最大追踪流数 */

/* 丢包原因分类 */
enum drop_reason {
    DROP_REASON_UNKNOWN = 0,
    DROP_REASON_NO_SOCKET,      /* 无对应socket */
    DROP_REASON_SOCKET_FILTER,  /* socket filter丢弃 */
    DROP_REASON_TCP_CSUM,       /* TCP校验和错误 */
    DROP_REASON_UDP_CSUM,       /* UDP校验和错误 */
    DROP_REASON_IP_CSUM,        /* IP校验和错误 */
    DROP_REASON_IP_HEADER,      /* IP头错误 */
    DROP_REASON_TCP_HEADER,     /* TCP头错误 */
    DROP_REASON_UDP_HEADER,     /* UDP头错误 */
    DROP_REASON_NO_ROUTE,       /* 无路由 */
    DROP_REASON_CONGESTION,     /* 拥塞丢弃 */
    DROP_REASON_RATELIMIT,      /* 速率限制 */
    DROP_REASON_RPFILTER,       /* 反向路径过滤 */
    DROP_REASON_NETFILTER,      /* netfilter丢弃 */
    DROP_REASON_TC,             /* TC丢弃 */
    DROP_REASON_XDP,            /* XDP丢弃 */
    DROP_REASON_MAX
};

/* =======================================================================
 * 数据结构
 * ======================================================================= */

/* 丢包事件（输出到用户态） */
struct drop_event {
    __u64 timestamp_ns;         /* 时间戳（纳秒） */
    __u32 pid;                  /* 进程ID */
    __u32 reason;               /* 丢包原因 */
    __u32 saddr;                /* 源IP（IPv4） */
    __u32 daddr;                /* 目的IP（IPv4） */
    __u16 sport;                /* 源端口 */
    __u16 dport;                /* 目的端口 */
    __u8  protocol;             /* 协议号 */
    __u8  pad[3];
    __u32 skb_len;              /* 数据包长度 */
    __u32 drop_count;           /* 连续丢包计数 */
};

/* 内核态丢包统计（按原因聚合） */
struct kernel_drop_stat {
    __u64 count;                /* 丢包总数 */
    __u64 bytes;                /* 丢包总字节数 */
    __u64 last_drop_ns;         /* 最后一次丢包时间 */
};

/* 流级丢包统计 */
struct flow_drop_stat {
    __u64 total_drops;          /* 总丢包数 */
    __u64 last_drop_ns;         /* 最后一次丢包时间 */
    __u32 saddr;
    __u32 daddr;
    __u16 sport;
    __u16 dport;
    __u8  protocol;
    __u8  pad[3];
};

/* =======================================================================
 * eBPF Maps
 * ======================================================================= */

/* Ring Buffer：丢包事件输出到用户态 */
struct {
    __uint(type, BPF_MAP_TYPE_RINGBUF);
    __uint(max_entries, 256 * 1024);  /* 256KB ring buffer */
} drop_events SEC(".maps");

/* Hash Map：按原因统计内核丢包 */
struct {
    __uint(type, BPF_MAP_TYPE_HASH);
    __uint(max_entries, MAX_DROP_REASONS);
    __type(key, __u32);         /* drop_reason */
    __type(value, struct kernel_drop_stat);
} kernel_drop_stats SEC(".maps");

/* Hash Map：按流统计丢包 */
struct {
    __uint(type, BPF_MAP_TYPE_LRU_HASH);
    __uint(max_entries, MAX_TRACKED_FLOWS);
    __type(key, __u64);         /* flow hash */
    __type(value, struct flow_drop_stat);
} flow_drop_stats SEC(".maps");

/* Array Map：全局丢包计数器 */
struct {
    __uint(type, BPF_MAP_TYPE_ARRAY);
    __uint(max_entries, 4);
    __type(key, __u32);
    __type(value, __u64);
} global_drop_counter SEC(".maps");

/* 全局计数器索引 */
enum counter_idx {
    COUNTER_TOTAL_DROPS = 0,    /* 总丢包数 */
    COUNTER_TOTAL_BYTES,        /* 总丢包字节数 */
    COUNTER_RINGBUF_FAILS,      /* ring buffer 写入失败次数 */
    COUNTER_LAST_TIMESTAMP,     /* 最后丢包时间戳 */
};

/* =======================================================================
 * 辅助函数
 * ======================================================================= */

/* 计算流哈希（五元组） */
static __always_inline __u64 calc_flow_hash(__u32 saddr, __u32 daddr, 
                                            __u16 sport, __u16 dport, 
                                            __u8 protocol)
{
    __u64 hash = saddr;
    hash = (hash << 5) + hash + daddr;
    hash = (hash << 5) + hash + sport;
    hash = (hash << 5) + hash + dport;
    hash = (hash << 5) + hash + protocol;
    return hash;
}

/* 从 skb 提取五元组信息 */
static __always_inline int extract_tuple(struct sk_buff *skb,
                                         __u32 *saddr, __u32 *daddr,
                                         __u16 *sport, __u16 *dport,
                                         __u8 *protocol)
{
    /* 获取网络头指针 */
    unsigned char *head = BPF_CORE_READ(skb, head);
    __u16 network_header = BPF_CORE_READ(skb, network_header);
    struct iphdr *ip = (struct iphdr *)(head + network_header);
    
    /* 读取IP头 */
    __u8 ip_protocol = BPF_CORE_READ(ip, protocol);
    __u32 src_ip = BPF_CORE_READ(ip, saddr);
    __u32 dst_ip = BPF_CORE_READ(ip, daddr);
    
    *saddr = src_ip;
    *daddr = dst_ip;
    *protocol = ip_protocol;
    
    /* 读取传输层端口 */
    __u16 transport_header = BPF_CORE_READ(skb, transport_header);
    if (ip_protocol == IPPROTO_TCP) {
        struct tcphdr *tcp = (struct tcphdr *)(head + transport_header);
        *sport = BPF_CORE_READ(tcp, source);
        *dport = BPF_CORE_READ(tcp, dest);
    } else if (ip_protocol == IPPROTO_UDP) {
        struct udphdr *udp = (struct udphdr *)(head + transport_header);
        *sport = BPF_CORE_READ(udp, source);
        *dport = BPF_CORE_READ(udp, dest);
    } else {
        *sport = 0;
        *dport = 0;
    }
    
    return 0;
}

/* 推断丢包原因 */
static __always_inline __u32 infer_drop_reason(struct sk_buff *skb)
{
    /* 尝试从 skb->drop_reason 读取（内核5.15+） */
    __u32 reason = 0;
    
    /* 检查校验和错误标志 */
    __u8 ip_summed = BPF_CORE_READ(skb, ip_summed);
    if (ip_summed == CHECKSUM_NONE) {
        return DROP_REASON_TCP_CSUM;
    }
    
    /* 检查路由相关 */
    struct dst_entry *dst = BPF_CORE_READ(skb, dst);
    if (!dst) {
        return DROP_REASON_NO_ROUTE;
    }
    
    /* 默认未知原因 */
    return DROP_REASON_UNKNOWN;
}

/* 更新全局计数器 */
static __always_inline void update_global_counter(__u32 idx, __u64 delta)
{
    __u64 *counter = bpf_map_lookup_elem(&global_drop_counter, &idx);
    if (counter) {
        __sync_fetch_and_add(counter, delta);
    }
}

/* 更新按原因统计 */
static __always_inline void update_kernel_drop_stat(__u32 reason, __u32 skb_len)
{
    struct kernel_drop_stat *stat = bpf_map_lookup_elem(&kernel_drop_stats, &reason);
    struct kernel_drop_stat new_stat = {0};
    
    if (!stat) {
        new_stat.count = 1;
        new_stat.bytes = skb_len;
        new_stat.last_drop_ns = bpf_ktime_get_ns();
        bpf_map_update_elem(&kernel_drop_stats, &reason, &new_stat, BPF_ANY);
    } else {
        __sync_fetch_and_add(&stat->count, 1);
        __sync_fetch_and_add(&stat->bytes, skb_len);
        stat->last_drop_ns = bpf_ktime_get_ns();
    }
}

/* 更新流级统计 */
static __always_inline void update_flow_drop_stat(__u32 saddr, __u32 daddr,
                                                  __u16 sport, __u16 dport,
                                                  __u8 protocol)
{
    __u64 flow_hash = calc_flow_hash(saddr, daddr, sport, dport, protocol);
    struct flow_drop_stat *stat = bpf_map_lookup_elem(&flow_drop_stats, &flow_hash);
    struct flow_drop_stat new_stat = {0};
    
    if (!stat) {
        new_stat.total_drops = 1;
        new_stat.last_drop_ns = bpf_ktime_get_ns();
        new_stat.saddr = saddr;
        new_stat.daddr = daddr;
        new_stat.sport = sport;
        new_stat.dport = dport;
        new_stat.protocol = protocol;
        bpf_map_update_elem(&flow_drop_stats, &flow_hash, &new_stat, BPF_ANY);
    } else {
        __sync_fetch_and_add(&stat->total_drops, 1);
        stat->last_drop_ns = bpf_ktime_get_ns();
    }
}

/* =======================================================================
 * eBPF 程序入口
 * ======================================================================= */

/* kprobe: kfree_skb - 内核丢包主入口 */
SEC("kprobe/kfree_skb")
int BPF_KPROBE(trace_kfree_skb, struct sk_buff *skb, void *location)
{
    if (!skb)
        return 0;
    
    /* 提取五元组 */
    __u32 saddr = 0, daddr = 0;
    __u16 sport = 0, dport = 0;
    __u8 protocol = 0;
    
    if (extract_tuple(skb, &saddr, &daddr, &sport, &dport, &protocol) < 0) {
        return 0;
    }
    
    /* 获取数据包长度 */
    __u32 skb_len = BPF_CORE_READ(skb, len);
    
    /* 推断丢包原因 */
    __u32 reason = infer_drop_reason(skb);
    
    /* 更新全局计数器 */
    __u64 now_ns = bpf_ktime_get_ns();
    update_global_counter(COUNTER_TOTAL_DROPS, 1);
    update_global_counter(COUNTER_TOTAL_BYTES, skb_len);
    update_global_counter(COUNTER_LAST_TIMESTAMP, now_ns);
    
    /* 更新按原因统计 */
    update_kernel_drop_stat(reason, skb_len);
    
    /* 更新流级统计 */
    update_flow_drop_stat(saddr, daddr, sport, dport, protocol);
    
    /* 构造丢包事件并发送到用户态 */
    struct drop_event *event = bpf_ringbuf_reserve(&drop_events, sizeof(*event), 0);
    if (!event) {
        /* ring buffer 满，记录失败 */
        update_global_counter(COUNTER_RINGBUF_FAILS, 1);
        return 0;
    }
    
    /* 填充事件数据 */
    event->timestamp_ns = now_ns;
    event->pid = bpf_get_current_pid_tgid() >> 32;
    event->reason = reason;
    event->saddr = saddr;
    event->daddr = daddr;
    event->sport = sport;
    event->dport = dport;
    event->protocol = protocol;
    event->skb_len = skb_len;
    event->drop_count = 1;
    
    bpf_ringbuf_submit(event, 0);
    
    return 0;
}

/* tracepoint: skb/kfree_skb - 替代方案（更稳定） */
SEC("tracepoint/skb/kfree_skb")
int tracepoint_kfree_skb(struct trace_event_raw_kfree_skb *ctx)
{
    struct sk_buff *skb = (struct sk_buff *)ctx->skbaddr;
    
    if (!skb)
        return 0;
    
    /* 提取五元组 */
    __u32 saddr = 0, daddr = 0;
    __u16 sport = 0, dport = 0;
    __u8 protocol = 0;
    
    if (extract_tuple(skb, &saddr, &daddr, &sport, &dport, &protocol) < 0) {
        return 0;
    }
    
    /* 获取数据包长度 */
    __u32 skb_len = BPF_CORE_READ(skb, len);
    
    /* 从 tracepoint 获取丢包原因（内核5.15+） */
    __u32 reason = ctx->reason;
    if (reason >= DROP_REASON_MAX) {
        reason = DROP_REASON_UNKNOWN;
    }
    
    /* 更新全局计数器 */
    __u64 now_ns = bpf_ktime_get_ns();
    update_global_counter(COUNTER_TOTAL_DROPS, 1);
    update_global_counter(COUNTER_TOTAL_BYTES, skb_len);
    update_global_counter(COUNTER_LAST_TIMESTAMP, now_ns);
    
    /* 更新按原因统计 */
    update_kernel_drop_stat(reason, skb_len);
    
    /* 更新流级统计 */
    update_flow_drop_stat(saddr, daddr, sport, dport, protocol);
    
    /* 构造丢包事件 */
    struct drop_event *event = bpf_ringbuf_reserve(&drop_events, sizeof(*event), 0);
    if (!event) {
        update_global_counter(COUNTER_RINGBUF_FAILS, 1);
        return 0;
    }
    
    event->timestamp_ns = now_ns;
    event->pid = bpf_get_current_pid_tgid() >> 32;
    event->reason = reason;
    event->saddr = saddr;
    event->daddr = daddr;
    event->sport = sport;
    event->dport = dport;
    event->protocol = protocol;
    event->skb_len = skb_len;
    event->drop_count = 1;
    
    bpf_ringbuf_submit(event, 0);
    
    return 0;
}

/* kprobe: tcp_drop - TCP层丢包 */
SEC("kprobe/tcp_drop")
int BPF_KPROBE(trace_tcp_drop, struct sock *sk, struct sk_buff *skb)
{
    if (!skb)
        return 0;
    
    /* 提取五元组 */
    __u32 saddr = BPF_CORE_READ(sk, __sk_common.skc_rcv_saddr);
    __u32 daddr = BPF_CORE_READ(sk, __sk_common.skc_daddr);
    __u16 sport = BPF_CORE_READ(sk, __sk_common.skc_num);
    __u16 dport = BPF_CORE_READ(sk, __sk_common.skc_dport);
    __u8 protocol = IPPROTO_TCP;
    
    __u32 skb_len = BPF_CORE_READ(skb, len);
    __u64 now_ns = bpf_ktime_get_ns();
    
    /* 更新全局计数器 */
    update_global_counter(COUNTER_TOTAL_DROPS, 1);
    update_global_counter(COUNTER_TOTAL_BYTES, skb_len);
    update_global_counter(COUNTER_LAST_TIMESTAMP, now_ns);
    
    /* TCP层丢包原因 */
    update_kernel_drop_stat(DROP_REASON_TCP_HEADER, skb_len);
    update_flow_drop_stat(saddr, daddr, sport, dport, protocol);
    
    /* 发送事件 */
    struct drop_event *event = bpf_ringbuf_reserve(&drop_events, sizeof(*event), 0);
    if (!event) {
        update_global_counter(COUNTER_RINGBUF_FAILS, 1);
        return 0;
    }
    
    event->timestamp_ns = now_ns;
    event->pid = bpf_get_current_pid_tgid() >> 32;
    event->reason = DROP_REASON_TCP_HEADER;
    event->saddr = saddr;
    event->daddr = daddr;
    event->sport = sport;
    event->dport = bpf_ntohs(dport);
    event->protocol = protocol;
    event->skb_len = skb_len;
    event->drop_count = 1;
    
    bpf_ringbuf_submit(event, 0);
    
    return 0;
}

/* =======================================================================
 * License
 * ======================================================================= */

char LICENSE[] SEC("license") = "Dual BSD/GPL";
