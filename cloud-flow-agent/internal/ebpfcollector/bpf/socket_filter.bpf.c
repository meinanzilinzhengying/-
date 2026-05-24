/*
 * Socket Filter eBPF Program - 非侵入式网络数据采集
 * 
 * 特点：
 * - 使用 BPF_PROG_TYPE_SOCKET_FILTER 类型
 * - 挂载到 raw socket，不修改内核函数
 * - 零拷贝读取网络包
 * - 部署/卸载不影响任何业务进程
 */

#include "vmlinux.h"
#include <bpf/bpf_helpers.h>
#include <bpf/bpf_endian.h>

#define ETH_P_IP    0x0800
#define ETH_P_IPV6  0x86DD

#define IPPROTO_TCP 6
#define IPPROTO_UDP 17

#define BPF_OK      0
#define BPF_DROP    2

/* 五元组信息 */
struct flow_key {
    __u32 src_ip;
    __u32 dst_ip;
    __u16 src_port;
    __u16 dst_port;
    __u8  protocol;
    __u8  pad[3];
};

/* 流量统计 */
struct flow_stats {
    __u64 bytes;
    __u64 packets;
    __u64 ts_start;
    __u64 ts_last;
};

/* BPF Maps */
struct {
    __uint(type, BPF_MAP_TYPE_HASH);
    __uint(max_entries, 100000);
    __type(key, struct flow_key);
    __type(value, struct flow_stats);
} flow_stats_map SEC(".maps");

/* 全局统计 */
struct {
    __uint(type, BPF_MAP_TYPE_ARRAY);
    __uint(max_entries, 1);
    __type(key, __u32);
    __type(value, __u64);
} global_stats_map SEC(".maps");

/* Ethernet Header */
struct ethhdr {
    __u8  h_dest[6];
    __u8  h_source[6];
    __u16 h_proto;
};

/* IP Header */
struct iphdr {
    __u8  ihl:4, version:4;
    __u8  tos;
    __u16 tot_len;
    __u16 id;
    __u16 frag_off;
    __u8  ttl;
    __u8  protocol;
    __u16 check;
    __u32 saddr;
    __u32 daddr;
};

/* TCP Header */
struct tcphdr {
    __u16 source;
    __u16 dest;
    __u32 seq;
    __u32 ack_seq;
    __u16 res1:4, doff:4, fin:1, syn:1, rst:1, psh:1, ack:1, urg:1, ece:1, cwr:1;
    __u16 window;
    __u16 check;
    __u16 urg_ptr;
};

/* UDP Header */
struct udphdr {
    __u16 source;
    __u16 dest;
    __u16 len;
    __u16 check;
};

/*
 * Socket Filter 主程序
 * 
 * 该程序挂载到 raw socket，接收所有网络包
 * 仅读取包头信息，不修改任何数据
 */
SEC("socket")
int socket_filter_prog(struct __sk_buff *skb)
{
    void *data = (void *)(long)skb->data;
    void *data_end = (void *)(long)skb->data_end;
    
    /* 包长度检查 */
    if (data + sizeof(struct ethhdr) > data_end)
        return BPF_OK;
    
    struct ethhdr *eth = data;
    __u16 eth_proto = bpf_ntohs(eth->h_proto);
    
    /* 只处理 IP 包 */
    if (eth_proto != ETH_P_IP && eth_proto != ETH_P_IPV6)
        return BPF_OK;
    
    /* 移动到 IP 头 */
    struct iphdr *ip = (struct iphdr *)(data + sizeof(struct ethhdr));
    if ((void *)(ip + 1) > data_end)
        return BPF_OK;
    
    /* 只处理 TCP/UDP */
    if (ip->protocol != IPPROTO_TCP && ip->protocol != IPPROTO_UDP)
        return BPF_OK;
    
    /* 构建 flow key */
    struct flow_key key = {};
    key.src_ip = ip->saddr;
    key.dst_ip = ip->daddr;
    key.protocol = ip->protocol;
    
    /* 提取端口 */
    if (ip->protocol == IPPROTO_TCP) {
        struct tcphdr *tcp = (struct tcphdr *)((void *)ip + (ip->ihl * 4));
        if ((void *)(tcp + 1) > data_end)
            return BPF_OK;
        key.src_port = bpf_ntohs(tcp->source);
        key.dst_port = bpf_ntohs(tcp->dest);
    } else {
        struct udphdr *udp = (struct udphdr *)((void *)ip + (ip->ihl * 4));
        if ((void *)(udp + 1) > data_end)
            return BPF_OK;
        key.src_port = bpf_ntohs(udp->source);
        key.dst_port = bpf_ntohs(udp->dest);
    }
    
    /* 更新流量统计 */
    struct flow_stats *stats = bpf_map_lookup_elem(&flow_stats_map, &key);
    if (stats) {
        /* 已存在的流，更新统计 */
        stats->bytes += skb->len;
        stats->packets += 1;
        stats->ts_last = bpf_ktime_get_ns();
    } else {
        /* 新流，创建统计 */
        struct flow_stats new_stats = {};
        new_stats.bytes = skb->len;
        new_stats.packets = 1;
        new_stats.ts_start = bpf_ktime_get_ns();
        new_stats.ts_last = new_stats.ts_start;
        bpf_map_update_elem(&flow_stats_map, &key, &new_stats, BPF_ANY);
    }
    
    /* 更新全局统计 */
    __u32 idx = 0;
    __u64 *total_packets = bpf_map_lookup_elem(&global_stats_map, &idx);
    if (total_packets) {
        __sync_fetch_and_add(total_packets, 1);
    }
    
    /* 返回 OK 表示允许包继续传输（不拦截） */
    return BPF_OK;
}

/*
 * 轻量级 Socket Filter
 * 
 * 仅做包计数，不做五元组解析
 * 性能更高，适合高吞吐场景
 */
SEC("socket")
int socket_filter_light(struct __sk_buff *skb)
{
    /* 简单计数 */
    __u32 idx = 0;
    __u64 *total_packets = bpf_map_lookup_elem(&global_stats_map, &idx);
    if (total_packets) {
        __sync_fetch_and_add(total_packets, 1);
    }
    
    return BPF_OK;
}

char _license[] SEC("license") = "GPL";
