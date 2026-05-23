// tcp_metrics.bpf.c - eBPF程序用于采集TCP深度指标
// 包括: TCP建连时延、重传率、零窗口事件、队列溢出、连接失败次数、吞吐量

#include "vmlinux.h"
#include <bpf/bpf_helpers.h>
#include <bpf/bpf_tracing.h>
#include <bpf/bpf_core_read.h>

#define AF_INET 2
#define AF_INET6 10
#define TCP_ESTABLISHED 1
#define TCP_SYN_SENT 2
#define TCP_SYN_RECV 3
#define TCP_FIN_WAIT1 4
#define TCP_FIN_WAIT2 5
#define TCP_TIME_WAIT 6
#define TCP_CLOSE 7
#define TCP_CLOSE_WAIT 8
#define TCP_LAST_ACK 9
#define TCP_LISTEN 10
#define TCP_CLOSING 11

// TCP连接标识
struct tcp_conn_key {
    __u32 saddr;
    __u32 daddr;
    __u16 sport;
    __u16 dport;
    __u32 pid;
    __u8  comm[16];  // 进程名
};

// 统一的TCP流统计结构体 - 聚合所有指标
struct tcp_flow_stats {
    // 建连时延（端到端）
    __u64 connect_latency_ns;     // 建连时延(纳秒)
    __u64 syn_sent_ns;            // SYN 发送时间戳
    __u8  connect_complete;       // 连接是否已完成
    __u8  padding1[7];

    // 重传统计
    __u64 retrans_count;          // 重传次数

    // 零窗口事件
    __u64 zero_window_count;      // 零窗口事件次数

    // TCP 队列溢出
    __u64 queue_overflow_count;   // TCP 队列溢出次数

    // 连接失败
    __u64 conn_fail_count;        // 连接失败次数

    // 吞吐量
    __u64 bytes_sent;             // 发送字节数
    __u64 bytes_recv;             // 接收字节数
    __u64 packets_sent;           // 发送包数
    __u64 packets_recv;           // 接收包数

    // 元数据
    __u64 last_update;            // 最后更新时间
};

// 全局TCP指标汇总
struct global_tcp_metrics {
    __u64 total_connections;        // 总连接数
    __u64 failed_connections;       // 失败连接数
    __u64 total_retrans;            // 总重传数
    __u64 zero_window_events;       // 零窗口事件数
    __u64 queue_overflow_events;    // 队列溢出事件数
    __u64 avg_latency_ns;           // 平均建连时延
    __u64 max_latency_ns;           // 最大建连时延
    __u64 min_latency_ns;           // 最小建连时延
    __u64 latency_samples;          // 时延样本数
};

// BPF Maps定义

// 统一的TCP流统计Map（按连接维度聚合所有指标）
struct {
    __uint(type, BPF_MAP_TYPE_HASH);
    __uint(max_entries, 65536);
    __type(key, struct tcp_conn_key);
    __type(value, struct tcp_flow_stats);
} tcp_flow_stats_map SEC(".maps");

// 全局TCP指标汇总Map
struct {
    __uint(type, BPF_MAP_TYPE_ARRAY);
    __uint(max_entries, 1);
    __type(key, __u32);
    __type(value, struct global_tcp_metrics);
} global_tcp_metrics_map SEC(".maps");

// 辅助函数: 获取当前时间戳(纳秒)
static __always_inline __u64 get_timestamp_ns(void) {
    return bpf_ktime_get_ns();
}

// 辅助函数: 从 sock 结构体中提取连接 key
static __always_inline void fill_conn_key(struct sock *sk, struct tcp_conn_key *key) {
    key->saddr = BPF_CORE_READ(sk, __sk_common.skc_rcv_saddr);
    key->daddr = BPF_CORE_READ(sk, __sk_common.skc_daddr);
    key->sport = BPF_CORE_READ(sk, __sk_common.skc_num);
    key->dport = bpf_ntohs(BPF_CORE_READ(sk, __sk_common.skc_dport));
    key->pid = bpf_get_current_pid_tgid() >> 32;
    bpf_get_current_comm(&key->comm, sizeof(key->comm));
}

// 辅助函数: 查找或创建 flow stats 条目
static __always_inline struct tcp_flow_stats *get_or_create_flow_stats(struct tcp_conn_key *key) {
    struct tcp_flow_stats *stats = bpf_map_lookup_elem(&tcp_flow_stats_map, key);
    if (!stats) {
        struct tcp_flow_stats init = {};
        bpf_map_update_elem(&tcp_flow_stats_map, key, &init, BPF_NOEXIST);
        stats = bpf_map_lookup_elem(&tcp_flow_stats_map, key);
    }
    return stats;
}

// ==================== TCP建连时延跟踪 ====================

// 跟踪tcp_v4_connect入口 - SYN发送
SEC("kprobe/tcp_v4_connect")
int BPF_KPROBE(trace_tcp_v4_connect_entry, struct sock *sk) {
    struct tcp_conn_key key = {};
    struct tcp_flow_stats init = {};

    // 读取连接信息并填充key
    fill_conn_key(sk, &key);

    // 初始化流统计条目，记录SYN发送时间
    init.syn_sent_ns = get_timestamp_ns();
    init.last_update = init.syn_sent_ns;

    bpf_map_update_elem(&tcp_flow_stats_map, &key, &init, BPF_ANY);

    return 0;
}

// 跟踪tcp_v6_connect入口
SEC("kprobe/tcp_v6_connect")
int BPF_KPROBE(trace_tcp_v6_connect_entry, struct sock *sk) {
    // IPv6逻辑类似,简化处理,只跟踪IPv4
    return 0;
}

// 跟踪tcp_rcv_state_process - 处理TCP状态变化
SEC("kprobe/tcp_rcv_state_process")
int BPF_KPROBE(trace_tcp_rcv_state_process, struct sock *sk) {
    struct tcp_conn_key key = {};
    struct tcp_flow_stats *stats;
    __u8 old_state;

    // 读取连接信息并填充key
    fill_conn_key(sk, &key);

    stats = bpf_map_lookup_elem(&tcp_flow_stats_map, &key);
    if (!stats)
        return 0;

    BPF_CORE_READ_INTO(&old_state, sk, __sk_common.skc_state);

    // 检测连接完全建立 (SYN_SENT -> ESTABLISHED 或 SYN_RECV -> ESTABLISHED)
    if ((old_state == TCP_SYN_SENT || old_state == TCP_SYN_RECV) &&
        stats->syn_sent_ns > 0 && !stats->connect_complete) {
        __u64 now = get_timestamp_ns();
        stats->connect_latency_ns = now - stats->syn_sent_ns;
        stats->connect_complete = 1;
        stats->last_update = now;

        // 更新全局时延统计
        __u32 gkey = 0;
        struct global_tcp_metrics *gmetrics = bpf_map_lookup_elem(&global_tcp_metrics_map, &gkey);
        if (gmetrics) {
            gmetrics->total_connections++;
            gmetrics->latency_samples++;
            // 使用近似平均计算
            gmetrics->avg_latency_ns =
                (gmetrics->avg_latency_ns * (gmetrics->latency_samples - 1) + stats->connect_latency_ns)
                / gmetrics->latency_samples;
            if (stats->connect_latency_ns > gmetrics->max_latency_ns)
                gmetrics->max_latency_ns = stats->connect_latency_ns;
            if (gmetrics->min_latency_ns == 0 || stats->connect_latency_ns < gmetrics->min_latency_ns)
                gmetrics->min_latency_ns = stats->connect_latency_ns;
        }
    }

    return 0;
}

// ==================== TCP重传跟踪 ====================

// 跟踪tcp_retransmit_skb - TCP重传
SEC("kprobe/tcp_retransmit_skb")
int BPF_KPROBE(trace_tcp_retransmit_skb, struct sock *sk, struct sk_buff *skb) {
    struct tcp_conn_key key = {};
    struct tcp_flow_stats *stats;

    fill_conn_key(sk, &key);

    stats = get_or_create_flow_stats(&key);
    if (stats) {
        stats->retrans_count++;
        stats->last_update = get_timestamp_ns();
    }

    // 更新全局重传计数
    __u32 gkey = 0;
    struct global_tcp_metrics *gmetrics = bpf_map_lookup_elem(&global_tcp_metrics_map, &gkey);
    if (gmetrics) {
        gmetrics->total_retrans++;
    }

    return 0;
}

// ==================== 零窗口事件跟踪 ====================

// 跟踪tcp_ack_update_window - 检测零窗口通告
SEC("kprobe/tcp_ack_update_window")
int BPF_KPROBE(trace_tcp_ack_update_window, struct sock *sk, __u32 nwin) {
    // nwin为0表示零窗口
    if (nwin != 0)
        return 0;

    struct tcp_conn_key key = {};
    struct tcp_flow_stats *stats;

    fill_conn_key(sk, &key);

    stats = get_or_create_flow_stats(&key);
    if (stats) {
        stats->zero_window_count++;
        stats->last_update = get_timestamp_ns();
    }

    // 更新全局零窗口计数
    __u32 gkey = 0;
    struct global_tcp_metrics *gmetrics = bpf_map_lookup_elem(&global_tcp_metrics_map, &gkey);
    if (gmetrics) {
        gmetrics->zero_window_events++;
    }

    return 0;
}

// ==================== 队列溢出跟踪 ====================

// 跟踪tcp_v4_syn_recv_sock - SYN队列处理
SEC("kprobe/tcp_v4_syn_recv_sock")
int BPF_KPROBE(trace_tcp_v4_syn_recv_sock, struct sock *sk, struct sk_buff *skb,
               struct request_sock *req, struct dst_entry *dst,
               struct request_sock *req_unhash, bool *own_req) {
    // 如果req为NULL,表示队列已满
    if (!req)
        return 0;

    struct tcp_conn_key key = {};
    struct tcp_flow_stats *stats;

    fill_conn_key(sk, &key);

    stats = get_or_create_flow_stats(&key);
    if (stats) {
        stats->queue_overflow_count++;
        stats->last_update = get_timestamp_ns();
    }

    // 更新全局队列溢出计数
    __u32 gkey = 0;
    struct global_tcp_metrics *gmetrics = bpf_map_lookup_elem(&global_tcp_metrics_map, &gkey);
    if (gmetrics) {
        gmetrics->queue_overflow_events++;
    }

    return 0;
}

// ==================== 连接失败跟踪 ====================

// 跟踪tcp_drop - 连接被丢弃
SEC("kprobe/tcp_drop")
int BPF_KPROBE(trace_tcp_drop, struct sock *sk, struct sk_buff *skb) {
    struct tcp_conn_key key = {};
    struct tcp_flow_stats *stats;

    fill_conn_key(sk, &key);

    stats = get_or_create_flow_stats(&key);
    if (stats) {
        stats->conn_fail_count++;
        stats->last_update = get_timestamp_ns();
    }

    // 更新全局失败计数
    __u32 gkey = 0;
    struct global_tcp_metrics *gmetrics = bpf_map_lookup_elem(&global_tcp_metrics_map, &gkey);
    if (gmetrics) {
        gmetrics->failed_connections++;
    }

    return 0;
}

// ==================== TCP吞吐量跟踪 ====================

// 跟踪tcp_sendmsg - 发送数据
SEC("kprobe/tcp_sendmsg")
int BPF_PROG(trace_tcp_sendmsg, struct sock *sk, struct msghdr *msg, size_t size) {
    struct tcp_conn_key key = {};
    struct tcp_flow_stats *stats;

    fill_conn_key(sk, &key);

    stats = get_or_create_flow_stats(&key);
    if (stats && size > 0) {
        stats->bytes_sent += size;
        stats->packets_sent++;
        stats->last_update = get_timestamp_ns();
    }

    return 0;
}

// 跟踪tcp_recvmsg - 接收数据
SEC("kprobe/tcp_recvmsg")
int BPF_PROG(trace_tcp_recvmsg, struct sock *sk, struct msghdr *msg, size_t len, int flags) {
    struct tcp_conn_key key = {};
    struct tcp_flow_stats *stats;

    fill_conn_key(sk, &key);

    stats = get_or_create_flow_stats(&key);
    if (stats && len > 0) {
        // recvmsg入口时len为请求读取的大小，实际接收量在retprobe中获取
        // 此处记录请求大小的近似值用于流量估算
        stats->bytes_recv += len;
        stats->packets_recv++;
        stats->last_update = get_timestamp_ns();
    }

    return 0;
}

// ==================== TCP连接关闭清理 ====================

// 跟踪tcp_close - 连接关闭时清理
SEC("kprobe/tcp_close")
int BPF_KPROBE(trace_tcp_close, struct sock *sk, long timeout) {
    struct tcp_conn_key key = {};

    fill_conn_key(sk, &key);

    // 清理流统计映射
    bpf_map_delete_elem(&tcp_flow_stats_map, &key);

    return 0;
}

// 许可证声明
char LICENSE[] SEC("license") = "GPL";
