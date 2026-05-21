// http_metrics.bpf.c - eBPF程序用于采集应用层HTTP/TCP请求响应指标
// 包括: 请求成功率、平均响应时延、最大响应时延、异常数量/比例；保留请求数、响应数

#include "vmlinux.h"
#include <bpf/bpf_helpers.h>
#include <bpf/bpf_tracing.h>
#include <bpf/bpf_core_read.h>

#define AF_INET 2
#define AF_INET6 10

// HTTP状态码分类
#define HTTP_STATUS_1XX 100
#define HTTP_STATUS_2XX 200
#define HTTP_STATUS_3XX 300
#define HTTP_STATUS_4XX 400
#define HTTP_STATUS_5XX 500

// 请求/响应跟踪键
struct http_flow_key {
    __u32 saddr;
    __u32 daddr;
    __u16 sport;
    __u16 dport;
    __u32 pid;
};

// 请求跟踪值
struct http_request {
    __u64 request_ns;       // 请求发送时间
    __u64 response_ns;      // 响应接收时间
    __u64 latency_ns;       // 响应时延
    __u16 status_code;      // HTTP状态码
    __u8  has_response;     // 是否有响应
    __u8  is_error;         // 是否异常(4xx/5xx)
    __u64 request_bytes;    // 请求字节数
    __u64 response_bytes;   // 响应字节数
};

// 聚合统计值
struct http_stats {
    __u64 request_count;        // 请求数
    __u64 response_count;       // 响应数
    __u64 success_count;        // 成功响应数(2xx)
    __u64 error_count;          // 异常数(4xx/5xx)
    __u64 total_latency_ns;     // 总时延
    __u64 avg_latency_ns;       // 平均时延
    __u64 max_latency_ns;       // 最大时延
    __u64 min_latency_ns;       // 最小时延
    __u64 total_request_bytes;  // 总请求字节数
    __u64 total_response_bytes; // 总响应字节数
    __u64 last_update;          // 最后更新时间
};

// 全局HTTP指标
struct global_http_metrics {
    __u64 total_requests;       // 总请求数
    __u64 total_responses;      // 总响应数
    __u64 success_responses;    // 成功响应数
    __u64 error_responses;      // 异常响应数
    __u64 avg_latency_ns;       // 全局平均时延
    __u64 max_latency_ns;       // 全局最大时延
    __u64 min_latency_ns;       // 全局最小时延
    __u64 latency_samples;      // 时延样本数
};

// BPF Maps定义

// 跟踪进行中的HTTP请求
struct {
    __uint(type, BPF_MAP_TYPE_HASH);
    __uint(max_entries, 65536);
    __type(key, struct http_flow_key);
    __type(value, struct http_request);
} http_request_map SEC(".maps");

// HTTP统计(按流维度)
struct {
    __uint(type, BPF_MAP_TYPE_HASH);
    __uint(max_entries, 65536);
    __type(key, struct http_flow_key);
    __type(value, struct http_stats);
} http_stats_map SEC(".maps");

// 全局HTTP指标
struct {
    __uint(type, BPF_MAP_TYPE_ARRAY);
    __uint(max_entries, 1);
    __type(key, __u32);
    __type(value, struct global_http_metrics);
} global_http_metrics_map SEC(".maps");

// 异常统计(按状态码)
struct {
    __uint(type, BPF_MAP_TYPE_HASH);
    __uint(max_entries, 1024);
    __type(key, __u16);  // 状态码
    __type(value, __u64);
} error_stats_map SEC(".maps");

// 辅助函数: 获取当前时间戳(纳秒)
static __always_inline __u64 get_timestamp_ns(void) {
    return bpf_ktime_get_ns();
}

// 辅助函数: 检查是否为HTTP数据包(简单启发式)
static __always_inline int is_http_request(const char *data, __u32 data_len) {
    if (data_len < 4)
        return 0;
    
    // 检查常见的HTTP方法
    if ((data[0] == 'G' && data[1] == 'E' && data[2] == 'T') ||
        (data[0] == 'P' && data[1] == 'O' && data[2] == 'S' && data[3] == 'T') ||
        (data[0] == 'P' && data[1] == 'U' && data[2] == 'T') ||
        (data[0] == 'D' && data[1] == 'E' && data[2] == 'L') ||
        (data[0] == 'H' && data[1] == 'E' && data[2] == 'A' && data[3] == 'D') ||
        (data[0] == 'O' && data[1] == 'P' && data[2] == 'T') ||
        (data[0] == 'P' && data[1] == 'A' && data[2] == 'T') ||
        (data[0] == 'C' && data[1] == 'O' && data[2] == 'N')) {
        return 1;
    }
    return 0;
}

// 辅助函数: 检查是否为HTTP响应
static __always_inline int is_http_response(const char *data, __u32 data_len) {
    if (data_len < 8)
        return 0;
    
    // 检查HTTP/1.x或HTTP/2.0
    if (data[0] == 'H' && data[1] == 'T' && data[2] == 'T' && data[3] == 'P') {
        return 1;
    }
    return 0;
}

// 辅助函数: 解析HTTP状态码
static __always_inline __u16 parse_http_status(const char *data, __u32 data_len) {
    if (data_len < 12)
        return 0;
    
    // HTTP/1.1 XXX 格式，状态码在第9-11位
    __u16 status = 0;
    status += (data[9] - '0') * 100;
    status += (data[10] - '0') * 10;
    status += (data[11] - '0');
    
    return status;
}

// ==================== TCP发送跟踪(请求) ====================

// 跟踪tcp_sendmsg - 发送数据
SEC("kprobe/tcp_sendmsg")
int BPF_KPROBE(trace_tcp_sendmsg, struct sock *sk, struct msghdr *msg, size_t size) {
    struct http_flow_key key = {};
    struct http_request *req;
    
    // 读取连接信息
    BPF_CORE_READ_INTO(&key.saddr, sk, __sk_common.skc_rcv_saddr);
    BPF_CORE_READ_INTO(&key.daddr, sk, __sk_common.skc_daddr);
    BPF_CORE_READ_INTO(&key.sport, sk, __sk_common.skc_num);
    BPF_CORE_READ_INTO(&key.dport, sk, __sk_common.skc_dport);
    key.pid = bpf_get_current_pid_tgid() >> 32;
    
    // 检查是否已有跟踪
    req = bpf_map_lookup_elem(&http_request_map, &key);
    if (!req) {
        // 新请求
        struct http_request new_req = {};
        new_req.request_ns = get_timestamp_ns();
        new_req.request_bytes = size;
        new_req.has_response = 0;
        new_req.is_error = 0;
        bpf_map_update_elem(&http_request_map, &key, &new_req, BPF_ANY);
        
        // 更新全局请求计数
        __u32 gkey = 0;
        struct global_http_metrics *gmetrics = bpf_map_lookup_elem(&global_http_metrics_map, &gkey);
        if (gmetrics) {
            gmetrics->total_requests++;
        }
    } else {
        // 更新现有请求的字节数
        req->request_bytes += size;
    }
    
    return 0;
}

// ==================== TCP接收跟踪(响应) ====================

// 跟踪tcp_recvmsg - 接收数据
SEC("kprobe/tcp_recvmsg")
int BPF_KPROBE(trace_tcp_recvmsg, struct sock *sk, struct msghdr *msg, size_t len, int nonblock, int flags, int *addr_len) {
    struct http_flow_key key = {};
    struct http_request *req;
    
    // 读取连接信息
    BPF_CORE_READ_INTO(&key.saddr, sk, __sk_common.skc_rcv_saddr);
    BPF_CORE_READ_INTO(&key.daddr, sk, __sk_common.skc_daddr);
    BPF_CORE_READ_INTO(&key.sport, sk, __sk_common.skc_num);
    BPF_CORE_READ_INTO(&key.dport, sk, __sk_common.skc_dport);
    key.pid = bpf_get_current_pid_tgid() >> 32;
    
    req = bpf_map_lookup_elem(&http_request_map, &key);
    if (!req) {
        return 0;
    }
    
    // 记录响应时间和字节数
    if (!req->has_response) {
        req->response_ns = get_timestamp_ns();
        req->latency_ns = req->response_ns - req->request_ns;
        req->has_response = 1;
    }
    req->response_bytes += len;
    
    return 0;
}

// ==================== HTTP响应状态码跟踪 ====================

// 跟踪tcp_data_queue - 处理接收到的数据
SEC("kprobe/tcp_data_queue")
int BPF_KPROBE(trace_tcp_data_queue, struct sock *sk, struct sk_buff *skb) {
    struct http_flow_key key = {};
    struct http_request *req;
    
    // 读取连接信息
    BPF_CORE_READ_INTO(&key.saddr, sk, __sk_common.skc_rcv_saddr);
    BPF_CORE_READ_INTO(&key.daddr, sk, __sk_common.skc_daddr);
    BPF_CORE_READ_INTO(&key.sport, sk, __sk_common.skc_num);
    BPF_CORE_READ_INTO(&key.dport, sk, __sk_common.skc_dport);
    key.pid = bpf_get_current_pid_tgid() >> 32;
    
    req = bpf_map_lookup_elem(&http_request_map, &key);
    if (!req || req->has_response) {
        return 0;
    }
    
    // 获取数据包内容
    void *data = (void *)(long)BPF_CORE_READ(skb, data);
    void *data_end = (void *)(long)BPF_CORE_READ(skb, data_end);
    
    if (data + 12 > data_end) {
        return 0;
    }
    
    // 检查是否为HTTP响应
    char *payload = data;
    if (is_http_response(payload, data_end - data)) {
        __u16 status = parse_http_status(payload, data_end - data);
        req->status_code = status;
        req->response_ns = get_timestamp_ns();
        req->latency_ns = req->response_ns - req->request_ns;
        req->has_response = 1;
        
        // 判断是否为异常(4xx/5xx)
        if (status >= 400) {
            req->is_error = 1;
        }
        
        // 更新统计
        struct http_stats *stats = bpf_map_lookup_elem(&http_stats_map, &key);
        if (!stats) {
            struct http_stats new_stats = {};
            new_stats.request_count = 1;
            new_stats.response_count = 1;
            if (status >= 200 && status < 300) {
                new_stats.success_count = 1;
            } else if (status >= 400) {
                new_stats.error_count = 1;
            }
            new_stats.total_latency_ns = req->latency_ns;
            new_stats.avg_latency_ns = req->latency_ns;
            new_stats.max_latency_ns = req->latency_ns;
            new_stats.min_latency_ns = req->latency_ns;
            new_stats.total_request_bytes = req->request_bytes;
            new_stats.total_response_bytes = req->response_bytes;
            new_stats.last_update = get_timestamp_ns();
            bpf_map_update_elem(&http_stats_map, &key, &new_stats, BPF_ANY);
        } else {
            stats->request_count++;
            stats->response_count++;
            if (status >= 200 && status < 300) {
                stats->success_count++;
            } else if (status >= 400) {
                stats->error_count++;
            }
            stats->total_latency_ns += req->latency_ns;
            stats->avg_latency_ns = stats->total_latency_ns / stats->response_count;
            if (req->latency_ns > stats->max_latency_ns) {
                stats->max_latency_ns = req->latency_ns;
            }
            if (stats->min_latency_ns == 0 || req->latency_ns < stats->min_latency_ns) {
                stats->min_latency_ns = req->latency_ns;
            }
            stats->total_request_bytes += req->request_bytes;
            stats->total_response_bytes += req->response_bytes;
            stats->last_update = get_timestamp_ns();
        }
        
        // 更新全局指标
        __u32 gkey = 0;
        struct global_http_metrics *gmetrics = bpf_map_lookup_elem(&global_http_metrics_map, &gkey);
        if (gmetrics) {
            gmetrics->total_responses++;
            if (status >= 200 && status < 300) {
                gmetrics->success_responses++;
            } else if (status >= 400) {
                gmetrics->error_responses++;
            }
            gmetrics->latency_samples++;
            gmetrics->avg_latency_ns = (gmetrics->avg_latency_ns * (gmetrics->latency_samples - 1) + req->latency_ns) / gmetrics->latency_samples;
            if (req->latency_ns > gmetrics->max_latency_ns) {
                gmetrics->max_latency_ns = req->latency_ns;
            }
            if (gmetrics->min_latency_ns == 0 || req->latency_ns < gmetrics->min_latency_ns) {
                gmetrics->min_latency_ns = req->latency_ns;
            }
        }
        
        // 更新异常统计
        if (status >= 400) {
            __u64 *err_count = bpf_map_lookup_elem(&error_stats_map, &status);
            if (!err_count) {
                __u64 initial = 1;
                bpf_map_update_elem(&error_stats_map, &status, &initial, BPF_ANY);
            } else {
                (*err_count)++;
            }
        }
        
        // 清理请求跟踪
        bpf_map_delete_elem(&http_request_map, &key);
    }
    
    return 0;
}

// ==================== 连接关闭清理 ====================

// 跟踪tcp_close - 连接关闭时清理未完成的请求
SEC("kprobe/tcp_close")
int BPF_KPROBE(trace_http_tcp_close, struct sock *sk, long timeout) {
    struct http_flow_key key = {};
    
    BPF_CORE_READ_INTO(&key.saddr, sk, __sk_common.skc_rcv_saddr);
    BPF_CORE_READ_INTO(&key.daddr, sk, __sk_common.skc_daddr);
    BPF_CORE_READ_INTO(&key.sport, sk, __sk_common.skc_num);
    BPF_CORE_READ_INTO(&key.dport, sk, __sk_common.skc_dport);
    key.pid = bpf_get_current_pid_tgid() >> 32;
    
    // 清理请求跟踪
    bpf_map_delete_elem(&http_request_map, &key);
    
    return 0;
}

// 许可证声明
char LICENSE[] SEC("license") = "GPL";
