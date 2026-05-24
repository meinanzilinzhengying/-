// http_full.bpf.c - eBPF程序用于完整解析HTTP协议字段
// 包括: 方法/路径/IP/Cookie/状态码等全字段解析

#include "vmlinux.h"
#include <bpf/bpf_helpers.h>
#include <bpf/bpf_tracing.h>
#include <bpf/bpf_core_read.h>

#define AF_INET 2
#define ETH_P_IP 0x0800
#define IPPROTO_TCP 6

#define HTTP_METHOD_MAX 8
#define HTTP_PATH_MAX 256
#define HTTP_COOKIE_MAX 512
#define HTTP_HOST_MAX 128
#define HTTP_UA_MAX 256
#define HTTP_REFERER_MAX 256
#define HTTP_CONTENT_TYPE_MAX 64

// HTTP请求方法枚举
typedef enum {
    HTTP_GET = 1,
    HTTP_POST,
    HTTP_PUT,
    HTTP_DELETE,
    HTTP_HEAD,
    HTTP_OPTIONS,
    HTTP_PATCH,
    HTTP_CONNECT,
    HTTP_TRACE,
    HTTP_UNKNOWN
} http_method_t;

// HTTP连接标识
struct http_conn_key {
    __u32 saddr;
    __u32 daddr;
    __u16 sport;
    __u16 dport;
    __u32 pid;
    __u32 netns;
};

// HTTP请求完整信息
struct http_request_full {
    // 基本信息
    __u64 timestamp_ns;
    __u64 request_id;
    
    // 方法
    http_method_t method;
    
    // 路径
    char path[HTTP_PATH_MAX];
    __u16 path_len;
    
    // Host头
    char host[HTTP_HOST_MAX];
    __u16 host_len;
    
    // Cookie
    char cookie[HTTP_COOKIE_MAX];
    __u16 cookie_len;
    
    // User-Agent
    char user_agent[HTTP_UA_MAX];
    __u16 ua_len;
    
    // Referer
    char referer[HTTP_REFERER_MAX];
    __u16 referer_len;
    
    // Content-Type
    char content_type[HTTP_CONTENT_TYPE_MAX];
    __u16 content_type_len;
    
    // 请求体大小
    __u32 content_length;
    
    // 真实客户端IP（从代理头部提取）
    __u8 x_forwarded_for[64];
    __u8 xff_len;
    __u8 x_real_ip[32];
    __u8 xri_len;
    
    // 连接信息
    __u8 is_https;
    __u8 http_version; // 0=1.0, 1=1.1, 2=2.0
    __u8 padding[2];
};

// HTTP响应完整信息
struct http_response_full {
    // 基本信息
    __u64 timestamp_ns;
    __u64 request_id;
    __u64 latency_ns;
    
    // 状态码
    __u16 status_code;
    
    // 状态描述
    char status_text[32];
    __u8 status_text_len;
    
    // Content-Type
    char content_type[HTTP_CONTENT_TYPE_MAX];
    __u16 content_type_len;
    
    // Content-Length
    __u32 content_length;
    
    // Server
    char server[64];
    __u16 server_len;
    
    // Set-Cookie
    char set_cookie[HTTP_COOKIE_MAX];
    __u16 set_cookie_len;
    
    // 响应特征
    __u8 is_chunked;
    __u8 is_gzipped;
    __u8 is_cached;
    __u8 padding;
};

// HTTP事务(请求+响应)
struct http_transaction {
    struct http_request_full request;
    struct http_response_full response;
    __u8 complete;  // 是否完整
    __u8 padding[7];
};

// BPF Maps

// 跟踪进行中的HTTP请求
struct {
    __uint(type, BPF_MAP_TYPE_HASH);
    __uint(max_entries, 65536);
    __type(key, struct http_conn_key);
    __type(value, struct http_request_full);
} http_requests SEC(".maps");

// 完整的HTTP事务
struct {
    __uint(type, BPF_MAP_TYPE_HASH);
    __uint(max_entries, 65536);
    __type(key, struct http_conn_key);
    __type(value, struct http_transaction);
} http_transactions SEC(".maps");

// HTTP事件队列(供用户态读取)
struct {
    __uint(type, BPF_MAP_TYPE_QUEUE);
    __uint(max_entries, 10000);
    __type(value, struct http_transaction);
} http_events SEC(".maps");

// 请求ID生成器
struct {
    __uint(type, BPF_MAP_TYPE_ARRAY);
    __uint(max_entries, 1);
    __type(key, __u32);
    __type(value, __u64);
} request_id_gen SEC(".maps");

// 统计计数器
struct {
    __uint(type, BPF_MAP_TYPE_PERCPU_ARRAY);
    __uint(max_entries, 1);
    __type(key, __u32);
    __type(value, struct http_stats);
} http_stats_counter SEC(".maps");

// 辅助函数: 获取当前时间戳
static __always_inline __u64 get_timestamp_ns(void) {
    return bpf_ktime_get_ns();
}

// 辅助函数: 生成请求ID
static __always_inline __u64 generate_request_id(void) {
    __u32 key = 0;
    __u64 *id = bpf_map_lookup_elem(&request_id_gen, &key);
    if (id) {
        __u64 new_id = *id + 1;
        bpf_map_update_elem(&request_id_gen, &key, &new_id, BPF_ANY);
        return new_id;
    }
    return 0;
}

// 辅助函数: 解析HTTP方法
static __always_inline http_method_t parse_http_method(const char *data, __u32 len) {
    if (len < 4) return HTTP_UNKNOWN;
    
    if (data[0] == 'G' && data[1] == 'E' && data[2] == 'T') {
        return HTTP_GET;
    } else if (data[0] == 'P' && data[1] == 'O' && data[2] == 'S' && data[3] == 'T') {
        return HTTP_POST;
    } else if (data[0] == 'P' && data[1] == 'U' && data[2] == 'T') {
        return HTTP_PUT;
    } else if (data[0] == 'D' && data[1] == 'E' && data[2] == 'L') {
        return HTTP_DELETE;
    } else if (data[0] == 'H' && data[1] == 'E' && data[2] == 'A' && data[3] == 'D') {
        return HTTP_HEAD;
    } else if (data[0] == 'O' && data[1] == 'P' && data[2] == 'T') {
        return HTTP_OPTIONS;
    } else if (data[0] == 'P' && data[1] == 'A' && data[2] == 'T') {
        return HTTP_PATCH;
    } else if (data[0] == 'C' && data[1] == 'O' && data[2] == 'N') {
        return HTTP_CONNECT;
    } else if (data[0] == 'T' && data[1] == 'R' && data[2] == 'A') {
        return HTTP_TRACE;
    }
    return HTTP_UNKNOWN;
}

// 辅助函数: 安全复制字符串
static __always_inline int safe_strcpy(char *dst, const char *src, int max_len, int src_len) {
    int len = src_len < max_len ? src_len : max_len - 1;
    bpf_probe_read_kernel(dst, len, src);
    dst[len] = '\0';
    return len;
}

// 辅助函数: 查找字符串中的子串
static __always_inline const char *bpf_strstr(const char *str, int str_len, const char *substr, int substr_len) {
    #pragma unroll
    for (int i = 0; i < 1024 && i < str_len - substr_len; i++) {
        int match = 1;
        #pragma unroll
        for (int j = 0; j < 16 && j < substr_len; j++) {
            if (str[i + j] != substr[j]) {
                match = 0;
                break;
            }
        }
        if (match) {
            return str + i;
        }
    }
    return NULL;
}

// 辅助函数: 解析HTTP头部字段
static __always_inline int parse_http_header(const char *data, __u32 data_len, 
                                              const char *header_name, int header_name_len,
                                              char *value_buf, int value_buf_size) {
    const char *header_start = bpf_strstr(data, data_len, header_name, header_name_len);
    if (!header_start) {
        return 0;
    }
    
    // 跳过头部名称和冒号空格
    const char *value_start = header_start + header_name_len + 2; // ": "
    
    // 查找行尾
    const char *line_end = bpf_strstr(value_start, data_len - (value_start - data), "\r\n", 2);
    if (!line_end) {
        line_end = data + data_len;
    }
    
    int value_len = line_end - value_start;
    if (value_len > value_buf_size - 1) {
        value_len = value_buf_size - 1;
    }
    
    bpf_probe_read_kernel(value_buf, value_len, value_start);
    value_buf[value_len] = '\0';
    
    return value_len;
}

// ==================== HTTP请求解析 ====================

// 跟踪tcp_sendmsg - 发送HTTP请求
SEC("kprobe/tcp_sendmsg")
int BPF_KPROBE(trace_http_sendmsg, struct sock *sk, struct msghdr *msg, size_t size) {
    struct http_conn_key key = {};
    struct http_request_full req = {};
    
    // 读取连接信息
    BPF_CORE_READ_INTO(&key.saddr, sk, __sk_common.skc_rcv_saddr);
    BPF_CORE_READ_INTO(&key.daddr, sk, __sk_common.skc_daddr);
    BPF_CORE_READ_INTO(&key.sport, sk, __sk_common.skc_num);
    BPF_CORE_READ_INTO(&key.dport, sk, __sk_common.skc_dport);
    key.pid = bpf_get_current_pid_tgid() >> 32;
    key.netns = BPF_CORE_READ(sk, __sk_common.skc_net.net, ns.inum);
    
    // 检查是否已有请求
    struct http_request_full *existing = bpf_map_lookup_elem(&http_requests, &key);
    if (existing) {
        // 更新请求体大小
        existing->content_length += size;
        return 0;
    }
    
    // 获取消息数据
    struct iov_iter *iter = &msg->msg_iter;
    const struct iovec *iov = iter->iov;
    
    if (!iov) {
        return 0;
    }
    
    // 读取HTTP请求数据
    char buf[1024] = {};
    int buf_len = size < sizeof(buf) ? size : sizeof(buf);
    bpf_probe_read_user(buf, buf_len, iov->iov_base);
    
    // 检查是否为HTTP请求
    http_method_t method = parse_http_method(buf, buf_len);
    if (method == HTTP_UNKNOWN) {
        return 0;
    }
    
    // 填充请求信息
    req.timestamp_ns = get_timestamp_ns();
    req.request_id = generate_request_id();
    req.method = method;
    
    // 解析路径 (在方法后面)
    char *path_start = buf;
    int path_offset = 0;
    if (method == HTTP_POST || method == HTTP_PUT || method == HTTP_HEAD) {
        path_offset = 5;
    } else if (method == HTTP_OPTIONS || method == HTTP_PATCH || method == HTTP_DELETE) {
        path_offset = 7;
    } else if (method == HTTP_CONNECT || method == HTTP_TRACE) {
        path_offset = 8;
    } else {
        path_offset = 4; // GET
    }
    
    if (path_offset < buf_len) {
        path_start = buf + path_offset;
        char *path_end = bpf_strstr(path_start, buf_len - path_offset, " HTTP", 5);
        if (path_end) {
            req.path_len = safe_strcpy(req.path, path_start, HTTP_PATH_MAX, path_end - path_start);
        }
    }
    
    // 解析Host头
    req.host_len = parse_http_header(buf, buf_len, "Host:", 5, req.host, HTTP_HOST_MAX);
    
    // 解析Cookie
    req.cookie_len = parse_http_header(buf, buf_len, "Cookie:", 7, req.cookie, HTTP_COOKIE_MAX);
    
    // 解析User-Agent
    req.ua_len = parse_http_header(buf, buf_len, "User-Agent:", 11, req.user_agent, HTTP_UA_MAX);
    
    // 解析Referer
    req.referer_len = parse_http_header(buf, buf_len, "Referer:", 8, req.referer, HTTP_REFERER_MAX);
    
    // 解析Content-Type
    req.content_type_len = parse_http_header(buf, buf_len, "Content-Type:", 13, req.content_type, HTTP_CONTENT_TYPE_MAX);
    
    // 解析Content-Length
    char content_length_str[16] = {};
    int cl_len = parse_http_header(buf, buf_len, "Content-Length:", 15, content_length_str, sizeof(content_length_str));
    if (cl_len > 0) {
        #pragma unroll
        for (int i = 0; i < 16 && i < cl_len; i++) {
            if (content_length_str[i] >= '0' && content_length_str[i] <= '9') {
                req.content_length = req.content_length * 10 + (content_length_str[i] - '0');
            }
        }
    }
    
    // 解析 X-Forwarded-For
    const char *xff_header = bpf_strstr(buf, buf_len, "X-Forwarded-For:", 16);
    if (xff_header) {
        const char *value_start = xff_header + 16;
        // 跳过冒号后的空格
        while (value_start < buf + buf_len && (*value_start == ' ' || *value_start == '\t')) {
            value_start++;
        }
        // 查找行尾
        const char *line_end = bpf_strstr(value_start, buf_len - (value_start - buf), "\r\n", 2);
        if (!line_end) {
            line_end = buf + buf_len;
        }
        int value_len = line_end - value_start;
        if (value_len > 0) {
            req.xff_len = value_len < 64 ? value_len : 63;
            bpf_probe_read_user(req.x_forwarded_for, req.xff_len, value_start);
            req.x_forwarded_for[req.xff_len] = '\0';
        }
    }
    
    // 解析 X-Real-IP
    const char *xri_header = bpf_strstr(buf, buf_len, "X-Real-IP:", 10);
    if (xri_header) {
        const char *value_start = xri_header + 10;
        // 跳过冒号后的空格
        while (value_start < buf + buf_len && (*value_start == ' ' || *value_start == '\t')) {
            value_start++;
        }
        // 查找行尾
        const char *line_end = bpf_strstr(value_start, buf_len - (value_start - buf), "\r\n", 2);
        if (!line_end) {
            line_end = buf + buf_len;
        }
        int value_len = line_end - value_start;
        if (value_len > 0) {
            req.xri_len = value_len < 32 ? value_len : 31;
            bpf_probe_read_user(req.x_real_ip, req.xri_len, value_start);
            req.x_real_ip[req.xri_len] = '\0';
        }
    }
    
    // 检测HTTP版本
    if (bpf_strstr(buf, buf_len, "HTTP/2", 6)) {
        req.http_version = 2;
    } else if (bpf_strstr(buf, buf_len, "HTTP/1.1", 8)) {
        req.http_version = 1;
    } else {
        req.http_version = 0;
    }
    
    // 检测HTTPS
    if (key.dport == 443 || key.sport == 443) {
        req.is_https = 1;
    }
    
    // 存储请求
    bpf_map_update_elem(&http_requests, &key, &req, BPF_ANY);
    
    return 0;
}

// ==================== HTTP响应解析 ====================

// 跟踪tcp_recvmsg - 接收HTTP响应
SEC("kprobe/tcp_recvmsg")
int BPF_KPROBE(trace_http_recvmsg, struct sock *sk, struct msghdr *msg, size_t len, int nonblock, int flags, int *addr_len) {
    struct http_conn_key key = {};
    
    // 读取连接信息
    BPF_CORE_READ_INTO(&key.saddr, sk, __sk_common.skc_rcv_saddr);
    BPF_CORE_READ_INTO(&key.daddr, sk, __sk_common.skc_daddr);
    BPF_CORE_READ_INTO(&key.sport, sk, __sk_common.skc_num);
    BPF_CORE_READ_INTO(&key.dport, sk, __sk_common.skc_dport);
    key.pid = bpf_get_current_pid_tgid() >> 32;
    key.netns = BPF_CORE_READ(sk, __sk_common.skc_net.net, ns.inum);
    
    // 查找对应的请求
    struct http_request_full *req = bpf_map_lookup_elem(&http_requests, &key);
    if (!req) {
        return 0;
    }
    
    // 获取消息数据
    struct iov_iter *iter = &msg->msg_iter;
    const struct iovec *iov = iter->iov;
    
    if (!iov) {
        return 0;
    }
    
    // 读取HTTP响应数据
    char buf[1024] = {};
    int buf_len = len < sizeof(buf) ? len : sizeof(buf);
    bpf_probe_read_user(buf, buf_len, iov->iov_base);
    
    // 检查是否为HTTP响应
    if (buf[0] != 'H' || buf[1] != 'T' || buf[2] != 'T' || buf[3] != 'P') {
        return 0;
    }
    
    // 创建事务
    struct http_transaction txn = {};
    __builtin_memcpy(&txn.request, req, sizeof(struct http_request_full));
    
    // 解析状态码
    if (buf_len >= 12) {
        txn.response.status_code = (buf[9] - '0') * 100 + 
                                   (buf[10] - '0') * 10 + 
                                   (buf[11] - '0');
    }
    
    // 解析状态文本
    char *status_text_start = buf + 13;
    char *status_text_end = bpf_strstr(status_text_start, buf_len - 13, "\r\n", 2);
    if (status_text_end) {
        txn.response.status_text_len = safe_strcpy(txn.response.status_text, status_text_start, 
                                                    sizeof(txn.response.status_text), 
                                                    status_text_end - status_text_start);
    }
    
    // 解析Content-Type
    txn.response.content_type_len = parse_http_header(buf, buf_len, "Content-Type:", 13, 
                                                        txn.response.content_type, HTTP_CONTENT_TYPE_MAX);
    
    // 解析Content-Length
    char content_length_str[16] = {};
    int cl_len = parse_http_header(buf, buf_len, "Content-Length:", 15, content_length_str, sizeof(content_length_str));
    if (cl_len > 0) {
        #pragma unroll
        for (int i = 0; i < 16 && i < cl_len; i++) {
            if (content_length_str[i] >= '0' && content_length_str[i] <= '9') {
                txn.response.content_length = txn.response.content_length * 10 + (content_length_str[i] - '0');
            }
        }
    }
    
    // 解析Server
    txn.response.server_len = parse_http_header(buf, buf_len, "Server:", 7, 
                                                 txn.response.server, sizeof(txn.response.server));
    
    // 解析Set-Cookie
    txn.response.set_cookie_len = parse_http_header(buf, buf_len, "Set-Cookie:", 11, 
                                                     txn.response.set_cookie, HTTP_COOKIE_MAX);
    
    // 检测传输编码
    if (bpf_strstr(buf, buf_len, "chunked", 7)) {
        txn.response.is_chunked = 1;
    }
    
    // 检测压缩
    if (bpf_strstr(buf, buf_len, "gzip", 4)) {
        txn.response.is_gzipped = 1;
    }
    
    // 检测缓存
    if (bpf_strstr(buf, buf_len, "X-Cache: HIT", 12) || 
        bpf_strstr(buf, buf_len, "CF-Cache-Status: HIT", 20)) {
        txn.response.is_cached = 1;
    }
    
    // 计算时延
    txn.response.timestamp_ns = get_timestamp_ns();
    txn.response.latency_ns = txn.response.timestamp_ns - req->timestamp_ns;
    txn.response.request_id = req->request_id;
    txn.complete = 1;
    
    // 存储事务
    bpf_map_update_elem(&http_transactions, &key, &txn, BPF_ANY);
    
    // 推送到事件队列
    bpf_map_push_elem(&http_events, &txn, BPF_ANY);
    
    // 清理请求跟踪
    bpf_map_delete_elem(&http_requests, &key);
    
    return 0;
}

// ==================== 连接关闭清理 ====================

// 跟踪tcp_close - 连接关闭时清理
SEC("kprobe/tcp_close")
int BPF_KPROBE(trace_http_tcp_close, struct sock *sk, long timeout) {
    struct http_conn_key key = {};
    
    BPF_CORE_READ_INTO(&key.saddr, sk, __sk_common.skc_rcv_saddr);
    BPF_CORE_READ_INTO(&key.daddr, sk, __sk_common.skc_daddr);
    BPF_CORE_READ_INTO(&key.sport, sk, __sk_common.skc_num);
    BPF_CORE_READ_INTO(&key.dport, sk, __sk_common.skc_dport);
    key.pid = bpf_get_current_pid_tgid() >> 32;
    key.netns = BPF_CORE_READ(sk, __sk_common.skc_net.net, ns.inum);
    
    // 清理请求跟踪
    bpf_map_delete_elem(&http_requests, &key);
    bpf_map_delete_elem(&http_transactions, &key);
    
    return 0;
}

// 许可证声明
char LICENSE[] SEC("license") = "GPL";
