// dns_full.bpf.c - eBPF程序用于完整解析DNS协议字段
// 包括: 事务ID/响应码/域名/TTL等全字段解析

#include "vmlinux.h"
#include <bpf/bpf_helpers.h>
#include <bpf/bpf_tracing.h>
#include <bpf/bpf_core_read.h>

#define AF_INET 2
#define IPPROTO_UDP 17
#define DNS_PORT 53

#define DNS_MAX_NAME_LEN 256
#define DNS_MAX_RECORDS 10
#define DNS_MAX_RDATA_LEN 256

// DNS报文头部标志位
#define DNS_FLAG_QR_RESPONSE 0x8000
#define DNS_FLAG_OPCODE_MASK 0x7800
#define DNS_FLAG_AA 0x0400
#define DNS_FLAG_TC 0x0200
#define DNS_FLAG_RD 0x0100
#define DNS_FLAG_RA 0x0080
#define DNS_FLAG_Z_MASK 0x0070
#define DNS_FLAG_RCODE_MASK 0x000F

// DNS响应码
#define DNS_RCODE_NOERROR 0
#define DNS_RCODE_FORMERR 1
#define DNS_RCODE_SERVFAIL 2
#define DNS_RCODE_NXDOMAIN 3
#define DNS_RCODE_NOTIMP 4
#define DNS_RCODE_REFUSED 5

// DNS记录类型
#define DNS_TYPE_A 1
#define DNS_TYPE_NS 2
#define DNS_TYPE_CNAME 5
#define DNS_TYPE_SOA 6
#define DNS_TYPE_PTR 12
#define DNS_TYPE_MX 15
#define DNS_TYPE_TXT 16
#define DNS_TYPE_AAAA 28
#define DNS_TYPE_SRV 33
#define DNS_TYPE_ANY 255

// DNS查询类
#define DNS_CLASS_IN 1
#define DNS_CLASS_CS 2
#define DNS_CLASS_CH 3
#define DNS_CLASS_HS 4

// DNS连接标识
struct dns_conn_key {
    __u32 saddr;
    __u32 daddr;
    __u16 sport;
    __u16 dport;
    __u32 pid;
    __u16 transaction_id;
};

// DNS问题(查询)
struct dns_question {
    char name[DNS_MAX_NAME_LEN];
    __u16 name_len;
    __u16 qtype;
    __u16 qclass;
};

// DNS资源记录
struct dns_record {
    char name[DNS_MAX_NAME_LEN];
    __u16 name_len;
    __u16 rtype;
    __u16 rclass;
    __u32 ttl;
    __u16 rdlength;
    char rdata[DNS_MAX_RDATA_LEN];
    __u16 rdata_len;
};

// DNS请求完整信息
struct dns_request_full {
    __u64 timestamp_ns;
    __u16 transaction_id;
    __u16 flags;
    __u16 opcode;
    __u8 recursion_desired;
    
    // 计数
    __u16 question_count;
    __u16 answer_count;
    __u16 authority_count;
    __u16 additional_count;
    
    // 查询
    struct dns_question questions[4];  // 最多4个问题
    __u16 question_count_actual;
};

// DNS响应完整信息
struct dns_response_full {
    __u64 timestamp_ns;
    __u64 latency_ns;
    __u16 transaction_id;
    __u16 flags;
    __u8 is_response;
    __u8 authoritative;
    __u8 truncated;
    __u8 recursion_available;
    __u8 rcode;
    
    // 响应码文本
    char rcode_text[16];
    
    // 计数
    __u16 question_count;
    __u16 answer_count;
    __u16 authority_count;
    __u16 additional_count;
    
    // 查询
    struct dns_question questions[4];
    
    // 回答
    struct dns_record answers[DNS_MAX_RECORDS];
    __u16 answer_count_actual;
    
    // 权威记录
    struct dns_record authorities[4];
    __u16 authority_count_actual;
    
    // 附加记录
    struct dns_record additionals[4];
    __u16 additional_count_actual;
};

// DNS事务
struct dns_transaction {
    struct dns_request_full request;
    struct dns_response_full response;
    __u8 complete;
    __u8 padding[7];
};

// BPF Maps

// 跟踪进行中的DNS查询
struct {
    __uint(type, BPF_MAP_TYPE_HASH);
    __uint(max_entries, 65536);
    __type(key, struct dns_conn_key);
    __type(value, struct dns_request_full);
} dns_queries SEC(".maps");

// DNS事件队列
struct {
    __uint(type, BPF_MAP_TYPE_QUEUE);
    __uint(max_entries, 10000);
    __type(value, struct dns_transaction);
} dns_events SEC(".maps");

// 统计计数器
struct {
    __uint(type, BPF_MAP_TYPE_PERCPU_ARRAY);
    __uint(max_entries, 16);
    __type(key, __u32);
    __type(value, __u64);
} dns_stats SEC(".maps");

// 辅助函数: 获取当前时间戳
static __always_inline __u64 get_timestamp_ns(void) {
    return bpf_ktime_get_ns();
}

// 辅助函数: 解码DNS域名
static __always_inline int decode_dns_name(const char *src, char *dst, int max_len) {
    int pos = 0;
    int dst_pos = 0;
    int jumped = 0;
    int jump_count = 0;
    
    #pragma unroll
    for (int i = 0; i < 128 && pos < max_len && dst_pos < max_len - 1; i++) {
        __u8 len;
        bpf_probe_read_kernel(&len, 1, src + pos);
        
        if (len == 0) {
            break;
        }
        
        // 检查是否是压缩指针
        if ((len & 0xC0) == 0xC0) {
            if (!jumped) {
                jumped = pos + 2;
            }
            __u16 offset;
            bpf_probe_read_kernel(&offset, 2, src + pos);
            pos = bpf_ntohs(offset) & 0x3FFF;
            if (++jump_count > 5) {
                break;
            }
            continue;
        }
        
        pos++;
        
        if (dst_pos > 0 && dst_pos < max_len - 1) {
            dst[dst_pos++] = '.';
        }
        
        int label_len = len < (max_len - dst_pos - 1) ? len : (max_len - dst_pos - 1);
        bpf_probe_read_kernel(dst + dst_pos, label_len, src + pos);
        dst_pos += label_len;
        pos += len;
    }
    
    dst[dst_pos] = '\0';
    return dst_pos;
}

// 辅助函数: 获取记录类型文本
static __always_inline void get_record_type_text(__u16 rtype, char *buf, int buf_size) {
    switch (rtype) {
        case DNS_TYPE_A: bpf_probe_read_kernel_str(buf, buf_size, "A"); break;
        case DNS_TYPE_NS: bpf_probe_read_kernel_str(buf, buf_size, "NS"); break;
        case DNS_TYPE_CNAME: bpf_probe_read_kernel_str(buf, buf_size, "CNAME"); break;
        case DNS_TYPE_SOA: bpf_probe_read_kernel_str(buf, buf_size, "SOA"); break;
        case DNS_TYPE_PTR: bpf_probe_read_kernel_str(buf, buf_size, "PTR"); break;
        case DNS_TYPE_MX: bpf_probe_read_kernel_str(buf, buf_size, "MX"); break;
        case DNS_TYPE_TXT: bpf_probe_read_kernel_str(buf, buf_size, "TXT"); break;
        case DNS_TYPE_AAAA: bpf_probe_read_kernel_str(buf, buf_size, "AAAA"); break;
        case DNS_TYPE_SRV: bpf_probe_read_kernel_str(buf, buf_size, "SRV"); break;
        case DNS_TYPE_ANY: bpf_probe_read_kernel_str(buf, buf_size, "ANY"); break;
        default: bpf_probe_read_kernel_str(buf, buf_size, "UNKNOWN"); break;
    }
}

// 辅助函数: 获取响应码文本
static __always_inline void get_rcode_text(__u8 rcode, char *buf, int buf_size) {
    switch (rcode) {
        case DNS_RCODE_NOERROR: bpf_probe_read_kernel_str(buf, buf_size, "NOERROR"); break;
        case DNS_RCODE_FORMERR: bpf_probe_read_kernel_str(buf, buf_size, "FORMERR"); break;
        case DNS_RCODE_SERVFAIL: bpf_probe_read_kernel_str(buf, buf_size, "SERVFAIL"); break;
        case DNS_RCODE_NXDOMAIN: bpf_probe_read_kernel_str(buf, buf_size, "NXDOMAIN"); break;
        case DNS_RCODE_NOTIMP: bpf_probe_read_kernel_str(buf, buf_size, "NOTIMP"); break;
        case DNS_RCODE_REFUSED: bpf_probe_read_kernel_str(buf, buf_size, "REFUSED"); break;
        default: bpf_probe_read_kernel_str(buf, buf_size, "UNKNOWN"); break;
    }
}

// ==================== DNS查询跟踪 ====================

// 跟踪udp_sendmsg - 发送DNS查询
SEC("kprobe/udp_sendmsg")
int BPF_KPROBE(trace_dns_sendmsg, struct sock *sk, struct msghdr *msg, size_t len) {
    struct dns_conn_key key = {};
    
    // 读取连接信息
    BPF_CORE_READ_INTO(&key.saddr, sk, __sk_common.skc_rcv_saddr);
    BPF_CORE_READ_INTO(&key.daddr, sk, __sk_common.skc_daddr);
    BPF_CORE_READ_INTO(&key.sport, sk, __sk_common.skc_num);
    BPF_CORE_READ_INTO(&key.dport, sk, __sk_common.skc_dport);
    key.pid = bpf_get_current_pid_tgid() >> 32;
    
    // 检查是否是DNS端口
    if (key.dport != bpf_htons(DNS_PORT) && key.sport != bpf_htons(DNS_PORT)) {
        return 0;
    }
    
    // 获取消息数据
    struct iov_iter *iter = &msg->msg_iter;
    const struct iovec *iov = iter->iov;
    
    if (!iov || len < 12) {
        return 0;
    }
    
    // 读取DNS报文
    char buf[512] = {};
    int buf_len = len < sizeof(buf) ? len : sizeof(buf);
    bpf_probe_read_user(buf, buf_len, iov->iov_base);
    
    // 解析DNS头部
    struct dns_request_full req = {};
    req.timestamp_ns = get_timestamp_ns();
    
    // 事务ID
    key.transaction_id = req.transaction_id = bpf_ntohs(*(__u16 *)buf);
    
    // 标志
    req.flags = bpf_ntohs(*(__u16 *)(buf + 2));
    req.opcode = (req.flags & DNS_FLAG_OPCODE_MASK) >> 11;
    req.recursion_desired = (req.flags & DNS_FLAG_RD) ? 1 : 0;
    
    // 计数
    req.question_count = bpf_ntohs(*(__u16 *)(buf + 4));
    req.answer_count = bpf_ntohs(*(__u16 *)(buf + 6));
    req.authority_count = bpf_ntohs(*(__u16 *)(buf + 8));
    req.additional_count = bpf_ntohs(*(__u16 *)(buf + 10));
    
    // 解析问题部分
    int pos = 12;
    int max_questions = req.question_count < 4 ? req.question_count : 4;
    
    #pragma unroll
    for (int i = 0; i < 4 && i < max_questions; i++) {
        if (pos >= buf_len) break;
        
        // 解码域名
        req.questions[i].name_len = decode_dns_name(buf + pos, req.questions[i].name, DNS_MAX_NAME_LEN);
        
        // 跳过域名
        while (pos < buf_len) {
            __u8 label_len;
            bpf_probe_read_kernel(&label_len, 1, buf + pos);
            if (label_len == 0) {
                pos++;
                break;
            }
            if ((label_len & 0xC0) == 0xC0) {
                pos += 2;
                break;
            }
            pos += label_len + 1;
        }
        
        // 读取QTYPE和QCLASS
        if (pos + 4 <= buf_len) {
            req.questions[i].qtype = bpf_ntohs(*(__u16 *)(buf + pos));
            req.questions[i].qclass = bpf_ntohs(*(__u16 *)(buf + pos + 2));
            pos += 4;
        }
        
        req.question_count_actual++;
    }
    
    // 存储查询
    bpf_map_update_elem(&dns_queries, &key, &req, BPF_ANY);
    
    // 更新统计
    __u32 stat_key = 0;
    __u64 *count = bpf_map_lookup_elem(&dns_stats, &stat_key);
    if (count) {
        (*count)++;
    }
    
    return 0;
}

// ==================== DNS响应跟踪 ====================

// 跟踪udp_recvmsg - 接收DNS响应
SEC("kprobe/udp_recvmsg")
int BPF_KPROBE(trace_dns_recvmsg, struct sock *sk, struct msghdr *msg, size_t len, int nonblock, int flags, int *addr_len) {
    struct dns_conn_key key = {};
    
    // 读取连接信息
    BPF_CORE_READ_INTO(&key.saddr, sk, __sk_common.skc_rcv_saddr);
    BPF_CORE_READ_INTO(&key.daddr, sk, __sk_common.skc_daddr);
    BPF_CORE_READ_INTO(&key.sport, sk, __sk_common.skc_num);
    BPF_CORE_READ_INTO(&key.dport, sk, __sk_common.skc_dport);
    key.pid = bpf_get_current_pid_tgid() >> 32;
    
    // 检查是否是DNS端口
    if (key.dport != bpf_htons(DNS_PORT) && key.sport != bpf_htons(DNS_PORT)) {
        return 0;
    }
    
    // 获取消息数据
    struct iov_iter *iter = &msg->msg_iter;
    const struct iovec *iov = iter->iov;
    
    if (!iov || len < 12) {
        return 0;
    }
    
    // 读取DNS报文
    char buf[1024] = {};
    int buf_len = len < sizeof(buf) ? len : sizeof(buf);
    bpf_probe_read_user(buf, buf_len, iov->iov_base);
    
    // 解析事务ID
    key.transaction_id = bpf_ntohs(*(__u16 *)buf);
    
    // 查找对应的查询
    struct dns_request_full *req = bpf_map_lookup_elem(&dns_queries, &key);
    if (!req) {
        return 0;
    }
    
    // 创建事务
    struct dns_transaction txn = {};
    __builtin_memcpy(&txn.request, req, sizeof(struct dns_request_full));
    
    // 解析DNS响应头部
    struct dns_response_full *resp = &txn.response;
    resp->timestamp_ns = get_timestamp_ns();
    resp->latency_ns = resp->timestamp_ns - req->timestamp_ns;
    resp->transaction_id = key.transaction_id;
    
    // 标志
    resp->flags = bpf_ntohs(*(__u16 *)(buf + 2));
    resp->is_response = (resp->flags & DNS_FLAG_QR_RESPONSE) ? 1 : 0;
    resp->authoritative = (resp->flags & DNS_FLAG_AA) ? 1 : 0;
    resp->truncated = (resp->flags & DNS_FLAG_TC) ? 1 : 0;
    resp->recursion_available = (resp->flags & DNS_FLAG_RA) ? 1 : 0;
    resp->rcode = resp->flags & DNS_FLAG_RCODE_MASK;
    
    // 响应码文本
    get_rcode_text(resp->rcode, resp->rcode_text, sizeof(resp->rcode_text));
    
    // 计数
    resp->question_count = bpf_ntohs(*(__u16 *)(buf + 4));
    resp->answer_count = bpf_ntohs(*(__u16 *)(buf + 6));
    resp->authority_count = bpf_ntohs(*(__u16 *)(buf + 8));
    resp->additional_count = bpf_ntohs(*(__u16 *)(buf + 10));
    
    // 复制问题
    __builtin_memcpy(resp->questions, req->questions, sizeof(resp->questions));
    
    // 解析回答记录
    int pos = 12;
    
    // 跳过问题部分
    #pragma unroll
    for (int i = 0; i < 4 && i < resp->question_count; i++) {
        while (pos < buf_len) {
            __u8 label_len;
            bpf_probe_read_kernel(&label_len, 1, buf + pos);
            if (label_len == 0) {
                pos++;
                break;
            }
            if ((label_len & 0xC0) == 0xC0) {
                pos += 2;
                break;
            }
            pos += label_len + 1;
        }
        pos += 4; // QTYPE + QCLASS
    }
    
    // 解析回答记录
    int max_answers = resp->answer_count < DNS_MAX_RECORDS ? resp->answer_count : DNS_MAX_RECORDS;
    
    #pragma unroll
    for (int i = 0; i < DNS_MAX_RECORDS && i < max_answers; i++) {
        if (pos + 12 > buf_len) break;
        
        // 解码域名
        resp->answers[i].name_len = decode_dns_name(buf + pos, resp->answers[i].name, DNS_MAX_NAME_LEN);
        
        // 跳过域名
        while (pos < buf_len) {
            __u8 label_len;
            bpf_probe_read_kernel(&label_len, 1, buf + pos);
            if (label_len == 0) {
                pos++;
                break;
            }
            if ((label_len & 0xC0) == 0xC0) {
                pos += 2;
                break;
            }
            pos += label_len + 1;
        }
        
        // 读取固定字段
        if (pos + 10 > buf_len) break;
        resp->answers[i].rtype = bpf_ntohs(*(__u16 *)(buf + pos));
        resp->answers[i].rclass = bpf_ntohs(*(__u16 *)(buf + pos + 2));
        resp->answers[i].ttl = bpf_ntohl(*(__u32 *)(buf + pos + 4));
        resp->answers[i].rdlength = bpf_ntohs(*(__u16 *)(buf + pos + 8));
        pos += 10;
        
        // 读取RDATA
        int rdata_len = resp->answers[i].rdlength < DNS_MAX_RDATA_LEN ? 
                        resp->answers[i].rdlength : DNS_MAX_RDATA_LEN;
        if (pos + rdata_len <= buf_len) {
            bpf_probe_read_kernel(resp->answers[i].rdata, rdata_len, buf + pos);
            resp->answers[i].rdata_len = rdata_len;
        }
        pos += resp->answers[i].rdlength;
        
        resp->answer_count_actual++;
    }
    
    txn.complete = 1;
    
    // 推送到事件队列
    bpf_map_push_elem(&dns_events, &txn, BPF_ANY);
    
    // 清理查询跟踪
    bpf_map_delete_elem(&dns_queries, &key);
    
    // 更新统计
    __u32 stat_key = resp->rcode + 1;
    if (stat_key < 16) {
        __u64 *count = bpf_map_lookup_elem(&dns_stats, &stat_key);
        if (count) {
            (*count)++;
        }
    }
    
    return 0;
}

// 许可证声明
char LICENSE[] SEC("license") = "GPL";
