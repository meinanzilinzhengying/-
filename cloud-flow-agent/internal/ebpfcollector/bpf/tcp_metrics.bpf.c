// tcp_metrics.bpf.c - eBPF程序用于采集TCP深度指标
// 包括: TCP建连时延、重传率、零窗口事件、队列溢出、连接失败次数

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
};

// TCP连接时延数据
struct tcp_latency {
    __u64 syn_sent_ns;      // SYN发送时间
    __u64 synack_recv_ns;   // SYN-ACK接收时间
    __u64 established_ns;   // 连接建立时间
    __u64 latency_ns;       // 建连时延(纳秒)
    __u8 complete;          // 是否完成测量
    __u8 padding[7];
};

// TCP统计指标
struct tcp_stats {
    __u64 retrans_count;        // 重传次数
    __u64 zero_window_count;    // 零窗口事件次数
    __u64 queue_overflow_count; // 队列溢出次数
    __u64 conn_fail_count;      // 连接失败次数
    __u64 bytes_sent;           // 发送字节数
    __u64 bytes_recv;           // 接收字节数
    __u64 packets_sent;         // 发送包数
    __u64 packets_recv;         // 接收包数
    __u64 last_update;          // 最后更新时间
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

// 跟踪进行中的TCP连接(用于计算建连时延)
struct {
    __uint(type, BPF_MAP_TYPE_HASH);
    __uint(max_entries, 65536);
    __type(key, struct tcp_conn_key);
    __type(value, struct tcp_latency);
} tcp_latency_map SEC(".maps");

// TCP连接统计(按连接维度)
struct {
    __uint(type, BPF_MAP_TYPE_HASH);
    __uint(max_entries, 65536);
    __type(key, struct tcp_conn_key);
    __type(value, struct tcp_stats);
} tcp_stats_map SEC(".maps");

// 全局TCP指标
struct {
    __uint(type, BPF_MAP_TYPE_ARRAY);
    __uint(max_entries, 1);
    __type(key, __u32);
    __type(value, struct global_tcp_metrics);
} global_tcp_metrics_map SEC(".maps");

// 零窗口事件计数(按目标IP:端口)
struct {
    __uint(type, BPF_MAP_TYPE_HASH);
    __uint(max_entries, 1024);
    __type(key, struct tcp_conn_key);
    __type(value, __u64);
} zero_window_map SEC(".maps");

// 队列溢出计数(按监听端口)
struct {
    __uint(type, BPF_MAP_TYPE_HASH);
    __uint(max_entries, 1024);
    __type(key, __u16);  // 监听端口
    __type(value, __u64);
} queue_overflow_map SEC(".maps");

// 连接失败计数(按目标)
struct {
    __uint(type, BPF_MAP_TYPE_HASH);
    __uint(max_entries, 1024);
    __type(key, struct tcp_conn_key);
    __type(value, __u64);
} conn_fail_map SEC(".maps");

// 辅助函数: 获取当前时间戳(纳秒)
static __always_inline __u64 get_timestamp_ns(void) {
    return bpf_ktime_get_ns();
}

// 辅助函数: 更新全局指标
static __always_inline void update_global_metrics(void *map, void (*update_fn)(struct global_tcp_metrics *)) {
    __u32 key = 0;
    struct global_tcp_metrics *metrics = bpf_map_lookup_elem(map, &key);
    if (metrics) {
        update_fn(metrics);
    }
}

// ==================== TCP建连时延跟踪 ====================

// 跟踪tcp_v4_connect入口 - SYN发送
SEC("kprobe/tcp_v4_connect")
int BPF_KPROBE(trace_tcp_v4_connect_entry, struct sock *sk) {
    struct tcp_conn_key key = {};
    struct tcp_latency latency = {};
    
    // 读取连接信息
    BPF_CORE_READ_INTO(&key.saddr, sk, __sk_common.skc_rcv_saddr);
    BPF_CORE_READ_INTO(&key.daddr, sk, __sk_common.skc_daddr);
    BPF_CORE_READ_INTO(&key.sport, sk, __sk_common.skc_num);
    BPF_CORE_READ_INTO(&key.dport, sk, __sk_common.skc_dport);
    key.pid = bpf_get_current_pid_tgid() >> 32;
    
    // 记录SYN发送时间
    latency.syn_sent_ns = get_timestamp_ns();
    latency.complete = 0;
    
    bpf_map_update_elem(&tcp_latency_map, &key, &latency, BPF_ANY);
    
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
    struct tcp_latency *latency;
    __u8 old_state, new_state;
    
    // 读取连接信息
    BPF_CORE_READ_INTO(&key.saddr, sk, __sk_common.skc_rcv_saddr);
    BPF_CORE_READ_INTO(&key.daddr, sk, __sk_common.skc_daddr);
    BPF_CORE_READ_INTO(&key.sport, sk, __sk_common.skc_num);
    BPF_CORE_READ_INTO(&key.dport, sk, __sk_common.skc_dport);
    key.pid = bpf_get_current_pid_tgid() >> 32;
    
    latency = bpf_map_lookup_elem(&tcp_latency_map, &key);
    if (!latency)
        return 0;
    
    BPF_CORE_READ_INTO(&old_state, sk, __sk_common.skc_state);
    
    // 检测SYN-ACK接收 (SYN_SENT -> ESTABLISHED)
    if (old_state == TCP_SYN_SENT && latency->syn_sent_ns > 0 && latency->synack_recv_ns == 0) {
        latency->synack_recv_ns = get_timestamp_ns();
    }
    
    // 检测连接完全建立
    if (old_state == TCP_SYN_RECV) {
        latency->established_ns = get_timestamp_ns();
        if (latency->syn_sent_ns > 0 && !latency->complete) {
            latency->latency_ns = latency->established_ns - latency->syn_sent_ns;
            latency->complete = 1;
            
            // 更新全局时延统计
            __u32 gkey = 0;
            struct global_tcp_metrics *gmetrics = bpf_map_lookup_elem(&global_tcp_metrics_map, &gkey);
            if (gmetrics) {
                gmetrics->total_connections++;
                gmetrics->latency_samples++;
                // 使用近似平均计算
                gmetrics->avg_latency_ns = 
                    (gmetrics->avg_latency_ns * (gmetrics->latency_samples - 1) + latency->latency_ns) 
                    / gmetrics->latency_samples;
                if (latency->latency_ns > gmetrics->max_latency_ns)
                    gmetrics->max_latency_ns = latency->latency_ns;
                if (gmetrics->min_latency_ns == 0 || latency->latency_ns < gmetrics->min_latency_ns)
                    gmetrics->min_latency_ns = latency->latency_ns;
            }
        }
    }
    
    return 0;
}

// ==================== TCP重传跟踪 ====================

// 跟踪tcp_retransmit_skb - TCP重传
SEC("kprobe/tcp_retransmit_skb")
int BPF_KPROBE(trace_tcp_retransmit_skb, struct sock *sk, struct sk_buff *skb) {
    struct tcp_conn_key key = {};
    struct tcp_stats *stats;
    
    BPF_CORE_READ_INTO(&key.saddr, sk, __sk_common.skc_rcv_saddr);
    BPF_CORE_READ_INTO(&key.daddr, sk, __sk_common.skc_daddr);
    BPF_CORE_READ_INTO(&key.sport, sk, __sk_common.skc_num);
    BPF_CORE_READ_INTO(&key.dport, sk, __sk_common.skc_dport);
    key.pid = bpf_get_current_pid_tgid() >> 32;
    
    stats = bpf_map_lookup_elem(&tcp_stats_map, &key);
    if (!stats) {
        struct tcp_stats new_stats = {};
        new_stats.retrans_count = 1;
        new_stats.last_update = get_timestamp_ns();
        bpf_map_update_elem(&tcp_stats_map, &key, &new_stats, BPF_ANY);
    } else {
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
    struct tcp_conn_key key = {};
    
    // nwin为0表示零窗口
    if (nwin != 0)
        return 0;
    
    BPF_CORE_READ_INTO(&key.saddr, sk, __sk_common.skc_rcv_saddr);
    BPF_CORE_READ_INTO(&key.daddr, sk, __sk_common.skc_daddr);
    BPF_CORE_READ_INTO(&key.sport, sk, __sk_common.skc_num);
    BPF_CORE_READ_INTO(&key.dport, sk, __sk_common.skc_dport);
    key.pid = bpf_get_current_pid_tgid() >> 32;
    
    __u64 *count = bpf_map_lookup_elem(&zero_window_map, &key);
    if (!count) {
        __u64 initial = 1;
        bpf_map_update_elem(&zero_window_map, &key, &initial, BPF_ANY);
    } else {
        (*count)++;
    }
    
    // 更新连接统计
    struct tcp_stats *stats = bpf_map_lookup_elem(&tcp_stats_map, &key);
    if (stats) {
        stats->zero_window_count++;
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
    
    __u16 lport;
    BPF_CORE_READ_INTO(&lport, sk, __sk_common.skc_num);
    
    __u64 *count = bpf_map_lookup_elem(&queue_overflow_map, &lport);
    if (!count) {
        __u64 initial = 1;
        bpf_map_update_elem(&queue_overflow_map, &lport, &initial, BPF_ANY);
    } else {
        (*count)++;
    }
    
    // 更新全局队列溢出计数
    __u32 gkey = 0;
    struct global_tcp_metrics *gmetrics = bpf_map_lookup_elem(&global_tcp_metrics_map, &gkey);
    if (gmetrics) {
        gmetrics->queue_overflow_events++;
    }
    
    return 0;
}

// 跟踪tcp_drop - 连接被丢弃
SEC("kprobe/tcp_drop")
int BPF_KPROBE(trace_tcp_drop, struct sock *sk, struct sk_buff *skb) {
    struct tcp_conn_key key = {};
    
    BPF_CORE_READ_INTO(&key.saddr, sk, __sk_common.skc_rcv_saddr);
    BPF_CORE_READ_INTO(&key.daddr, sk, __sk_common.skc_daddr);
    BPF_CORE_READ_INTO(&key.sport, sk, __sk_common.skc_num);
    BPF_CORE_READ_INTO(&key.dport, sk, __sk_common.skc_dport);
    key.pid = bpf_get_current_pid_tgid() >> 32;
    
    __u64 *count = bpf_map_lookup_elem(&conn_fail_map, &key);
    if (!count) {
        __u64 initial = 1;
        bpf_map_update_elem(&conn_fail_map, &key, &initial, BPF_ANY);
    } else {
        (*count)++;
    }
    
    // 更新连接统计
    struct tcp_stats *stats = bpf_map_lookup_elem(&tcp_stats_map, &key);
    if (stats) {
        stats->conn_fail_count++;
    }
    
    // 更新全局失败计数
    __u32 gkey = 0;
    struct global_tcp_metrics *gmetrics = bpf_map_lookup_elem(&global_tcp_metrics_map, &gkey);
    if (gmetrics) {
        gmetrics->failed_connections++;
    }
    
    // 清理时延跟踪
    bpf_map_delete_elem(&tcp_latency_map, &key);
    
    return 0;
}

// ==================== TCP连接关闭清理 ====================

// 跟踪tcp_close - 连接关闭时清理
SEC("kprobe/tcp_close")
int BPF_KPROBE(trace_tcp_close, struct sock *sk, long timeout) {
    struct tcp_conn_key key = {};
    
    BPF_CORE_READ_INTO(&key.saddr, sk, __sk_common.skc_rcv_saddr);
    BPF_CORE_READ_INTO(&key.daddr, sk, __sk_common.skc_daddr);
    BPF_CORE_READ_INTO(&key.sport, sk, __sk_common.skc_num);
    BPF_CORE_READ_INTO(&key.dport, sk, __sk_common.skc_dport);
    key.pid = bpf_get_current_pid_tgid() >> 32;
    
    // 清理时延跟踪映射
    bpf_map_delete_elem(&tcp_latency_map, &key);
    
    return 0;
}

// 许可证声明
char LICENSE[] SEC("license") = "GPL";
