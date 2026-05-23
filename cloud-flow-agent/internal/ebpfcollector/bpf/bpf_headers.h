// SPDX-License-Identifier: GPL-2.0 OR BSD-3-Clause
/*
 * bpf_headers.h - eBPF 公共头文件
 *
 * 本文件定义了 eBPF 程序与用户态（Go/C）之间共享的数据结构、
 * 内核版本检查宏以及跨架构兼容宏。
 *
 * 所有 .bpf.c 文件和用户态加载器均应包含此头文件，
 * 以确保数据结构布局一致。
 *
 * Copyright (c) 2024 cloud-flow contributors
 */

#ifndef __BPF_HEADERS_H__
#define __BPF_HEADERS_H__

#include <linux/types.h>
#include <linux/stddef.h>

/* ========================================================================
 * 内核版本检查宏
 * ======================================================================== */

/*
 * MIN_KERNEL_VERSION(major, minor, patch)
 *
 * 在编译时检查当前内核版本是否满足最低要求。
 * 使用方式：
 *   #if MIN_KERNEL_VERSION(5, 10, 0)
 *       // 需要 5.10+ 内核的代码
 *   #endif
 *
 * 依赖 LINUX_VERSION_CODE 和 KERNEL_VERSION 宏，
 * 由 <linux/version.h> 提供。
 */
#ifndef MIN_KERNEL_VERSION
#define MIN_KERNEL_VERSION(major, minor, patch) \
    (LINUX_VERSION_CODE >= KERNEL_VERSION(major, minor, patch))
#endif

/*
 * MAX_KERNEL_VERSION(major, minor, patch)
 *
 * 在编译时检查当前内核版本是否低于指定版本上限。
 * 用于需要兼容性分支的场景。
 */
#ifndef MAX_KERNEL_VERSION
#define MAX_KERNEL_VERSION(major, minor, patch) \
    (LINUX_VERSION_CODE < KERNEL_VERSION(major, minor, patch))
#endif

/*
 * KERNEL_RANGE(low_major, low_minor, low_patch, high_major, high_minor, high_patch)
 *
 * 检查内核版本是否在 [low, high) 范围内。
 */
#ifndef KERNEL_RANGE
#define KERNEL_RANGE(lmaj, lmin, lpat, hmaj, hmin, hpat) \
    (MIN_KERNEL_VERSION(lmaj, lmin, lpat) && MAX_KERNEL_VERSION(hmaj, hmin, hpat))
#endif

/* ========================================================================
 * 跨架构兼容宏（ARM64 / x86_64）
 * ======================================================================== */

/*
 * BPF_HOST_ENDIAN - 主机字节序标识
 *
 * 在 BPF 程序中，网络数据始终是大端序（网络字节序），
 * 而主机数据取决于 CPU 架构：
 *   - x86_64: 小端序
 *   - ARM64:  小端序（默认），可配置为大端序（罕见）
 *
 * 使用 bpf/bpf_endian.h 中的 bpf_htons / bpf_ntohs 进行转换。
 */

/*
 * BPF_SWAP_IF_BE(val) / BPF_SWAP_IF_LE(val)
 *
 * 条件字节序转换宏。在已知字节序的架构上为空操作，
 * 在未知架构上使用编译器内置函数。
 */
#if defined(__x86_64__)
    /* x86_64 始终为小端序 */
    #define BPF_HOST_IS_LITTLE_ENDIAN  1
    #define BPF_HOST_IS_BIG_ENDIAN     0
    #define BPF_SWAP_IF_BE(val)  __builtin_bswap16(val)
    #define BPF_SWAP_IF_LE(val)  (val)
#elif defined(__aarch64__)
    /* ARM64 默认小端序 */
    #if __BYTE_ORDER__ == __ORDER_LITTLE_ENDIAN__
        #define BPF_HOST_IS_LITTLE_ENDIAN  1
        #define BPF_HOST_IS_BIG_ENDIAN     0
        #define BPF_SWAP_IF_BE(val)  __builtin_bswap16(val)
        #define BPF_SWAP_IF_LE(val)  (val)
    #else
        #define BPF_HOST_IS_LITTLE_ENDIAN  0
        #define BPF_HOST_IS_BIG_ENDIAN     1
        #define BPF_SWAP_IF_BE(val)  (val)
        #define BPF_SWAP_IF_LE(val)  __builtin_bswap16(val)
    #endif
#elif defined(__riscv)
    /* RISC-V 默认小端序 */
    #if __BYTE_ORDER__ == __ORDER_LITTLE_ENDIAN__
        #define BPF_HOST_IS_LITTLE_ENDIAN  1
        #define BPF_HOST_IS_BIG_ENDIAN     0
        #define BPF_SWAP_IF_BE(val)  __builtin_bswap16(val)
        #define BPF_SWAP_IF_LE(val)  (val)
    #else
        #define BPF_HOST_IS_LITTLE_ENDIAN  0
        #define BPF_HOST_IS_BIG_ENDIAN     1
        #define BPF_SWAP_IF_BE(val)  (val)
        #define BPF_SWAP_IF_LE(val)  __builtin_bswap16(val)
    #endif
#else
    /* 未知架构 - 运行时检测 */
    #define BPF_HOST_IS_LITTLE_ENDIAN  -1
    #define BPF_HOST_IS_BIG_ENDIAN     -1
    #define BPF_SWAP_IF_BE(val)  __builtin_bswap16(val)
    #define BPF_SWAP_IF_LE(val)  (val)
#endif

/*
 * 32 位字节序转换宏
 */
#if defined(__x86_64__) || (defined(__aarch64__) && __BYTE_ORDER__ == __ORDER_LITTLE_ENDIAN__) \
    || (defined(__riscv) && __BYTE_ORDER__ == __ORDER_LITTLE_ENDIAN__)
    #define BPF_SWAP32_IF_BE(val)  __builtin_bswap32(val)
    #define BPF_SWAP32_IF_LE(val)  (val)
#else
    #define BPF_SWAP32_IF_BE(val)  (val)
    #define BPF_SWAP32_IF_LE(val)  __builtin_bswap32(val)
#endif

/*
 * BPF_READ_NET16(ptr)
 *
 * 从网络数据包中安全读取 16 位值（大端序 -> 主机序）。
 * 用于 BPF 程序中解析网络头部字段。
 */
#define BPF_READ_NET16(ptr)  __builtin_bswap16(*(const __u16 *)(ptr))

/*
 * BPF_READ_NET32(ptr)
 *
 * 从网络数据包中安全读取 32 位值（大端序 -> 主机序）。
 */
#define BPF_READ_NET32(ptr)  __builtin_bswap32(*(const __u32 *)(ptr))

/*
 * BPF_WRITE_NET16(ptr, val)
 *
 * 将 16 位主机序值写入网络数据包（主机序 -> 大端序）。
 */
#define BPF_WRITE_NET16(ptr, val)  (*(ptr) = __builtin_bswap16(val))

/*
 * BPF_WRITE_NET32(ptr, val)
 *
 * 将 32 位主机序值写入网络数据包（主机序 -> 大端序）。
 */
#define BPF_WRITE_NET32(ptr, val)  (*(ptr) = __builtin_bswap32(val))

/* ========================================================================
 * 通用常量定义
 * ======================================================================== */

/* Map 容量限制 */
#define BPF_NETWORK_MAP_MAX_ENTRIES     10240
#define BPF_TCP_MAP_MAX_ENTRIES         65536
#define BPF_HTTP_MAP_MAX_ENTRIES        65536
#define BPF_STACK_MAP_MAX_ENTRIES       8192
#define BPF_HOT_FUNC_MAP_MAX_ENTRIES    65536
#define BPF_COUNTER_MAP_MAX_ENTRIES     1024
#define BPF_PROC_CACHE_MAX_ENTRIES      4096

/* 栈深度限制 */
#define BPF_MAX_STACK_DEPTH             127
#define BPF_MIN_STACK_DEPTH             8
#define BPF_USER_STACK_DEPTH            64

/* TCP 状态码 */
#define TCP_ESTABLISHED  1
#define TCP_SYN_SENT     2
#define TCP_SYN_RECV     3
#define TCP_FIN_WAIT1    4
#define TCP_FIN_WAIT2    5
#define TCP_TIME_WAIT    6
#define TCP_CLOSE        7
#define TCP_CLOSE_WAIT   8
#define TCP_LAST_ACK     9
#define TCP_LISTEN       10
#define TCP_CLOSING      11

/* 地址族 */
#define AF_INET          2
#define AF_INET6         10

/* 协议号 */
#define IPPROTO_ICMP     1
#define IPPROTO_TCP      6
#define IPPROTO_UDP      17

/* 事件类型 */
#define EVENT_TYPE_SAMPLE    1
#define EVENT_TYPE_LOST      2

/* HTTP 状态码分类 */
#define HTTP_STATUS_2XX    200
#define HTTP_STATUS_3XX    300
#define HTTP_STATUS_4XX    400
#define HTTP_STATUS_5XX    500

/* BPF 程序类型标识（用户态加载器使用） */
#define BPF_SUBSYS_TC              (1 << 0)  /* TC 流量采集 */
#define BPF_SUBSYS_TCP_METRICS     (1 << 1)  /* TCP 深度指标 */
#define BPF_SUBSYS_HTTP_METRICS    (1 << 2)  /* HTTP 指标 */
#define BPF_SUBSYS_DNS_FULL        (1 << 3)  /* DNS 全量采集 */
#define BPF_SUBSYS_HTTP_FULL       (1 << 4)  /* HTTP 全量采集 */
#define BPF_SUBSYS_MYSQL_FULL      (1 << 5)  /* MySQL 全量采集 */
#define BPF_SUBSYS_CPU_PROFILER    (1 << 6)  /* CPU 剖析 */
#define BPF_SUBSYS_ALL             0xFF      /* 启用全部子系统 */

/* 错误码定义 */
#define BPF_LOADER_OK                  0    /* 成功 */
#define BPF_LOADER_ERR_INVALID_ARG    -1    /* 无效参数 */
#define BPF_LOADER_ERR_OPEN_FILE      -2    /* 打开 BPF 对象文件失败 */
#define BPF_LOADER_ERR_LOAD           -3    /* 加载 BPF 程序到内核失败 */
#define BPF_LOADER_ERR_ATTACH_TC      -4    /* TC 挂载失败 */
#define BPF_LOADER_ERR_ATTACH_KPROBE  -5    /* kprobe 挂载失败 */
#define BPF_LOADER_ERR_MAP_OP         -6    /* Map 操作失败 */
#define BPF_LOADER_ERR_NOT_INITIALIZED -7   /* 未初始化 */
#define BPF_LOADER_ERR_ALREADY_INIT   -8    /* 已初始化 */
#define BPF_LOADER_ERR_KERN_VERSION   -9    /* 内核版本不满足要求 */
#define BPF_LOADER_ERR_NETLINK       -10    /* Netlink 操作失败 */
#define BPF_LOADER_ERR_NOMEM         -11    /* 内存不足 */

/* ========================================================================
 * 网络流量数据结构（TC 子系统）
 * ======================================================================== */

/*
 * flow_key_t - 网络流五元组键
 *
 * 注意：使用 __attribute__((packed)) 确保与 Go 端 bpfKeySize=12 一致。
 * 字段使用网络字节序（大端序），与 tc.bpf.c 中的定义保持一致。
 */
typedef struct __attribute__((packed)) {
    __be32 src_ip;     /* 源 IP 地址（网络字节序） */
    __be32 dst_ip;     /* 目的 IP 地址（网络字节序） */
    __be16 src_port;   /* 源端口号（网络字节序） */
    __be16 dst_port;   /* 目的端口号（网络字节序） */
} flow_key_t;

/*
 * network_data_t - 网络流量聚合数据
 *
 * 每个 flow_key_t 对应的流量统计信息。
 */
typedef struct __attribute__((packed)) {
    __be32 dst_ip;     /* 目的 IP 地址（冗余存储，便于快速查询） */
    __be16 dst_port;   /* 目的端口号（冗余存储） */
    __u8  protocol;    /* 传输层协议：IPPROTO_TCP / IPPROTO_UDP / IPPROTO_ICMP */
    __u8  _pad;        /* 对齐填充 */
    __u64 bytes;       /* 累计字节数 */
    __u64 packets;     /* 累计包数 */
    __u64 timestamp;   /* 最后更新时间戳（毫秒，bpf_ktime_get_ns() / 1000000） */
} network_data_t;

/* ========================================================================
 * TCP 连接数据结构（TCP Metrics 子系统）
 * ======================================================================== */

/*
 * tcp_conn_key_t - TCP 连接标识键
 *
 * 用于唯一标识一个 TCP 连接。与 Go 端 TcpConnKey 结构体对应。
 */
typedef struct {
    __u32 saddr;       /* 源 IP 地址（主机字节序） */
    __u32 daddr;       /* 目的 IP 地址（主机字节序） */
    __u16 sport;       /* 源端口号（主机字节序） */
    __u16 dport;       /* 目的端口号（主机字节序） */
    __u32 pid;         /* 进程 ID */
} tcp_conn_key_t;

/*
 * tcp_latency_t - TCP 连接时延数据
 *
 * 跟踪 TCP 三次握手各阶段的时间戳，用于计算建连时延。
 */
typedef struct {
    __u64 syn_sent_ns;      /* SYN 发送时间（纳秒） */
    __u64 synack_recv_ns;   /* SYN-ACK 接收时间（纳秒） */
    __u64 established_ns;   /* 连接建立完成时间（纳秒） */
    __u64 latency_ns;       /* 建连时延 = established_ns - syn_sent_ns（纳秒） */
    __u8  complete;         /* 是否完成测量：0=进行中, 1=已完成 */
    __u8  padding[7];       /* 对齐填充，确保结构体大小为 40 字节 */
} tcp_latency_t;

/*
 * tcp_stats_t - TCP 连接级统计指标
 *
 * 按连接维度统计的 TCP 性能指标。
 */
typedef struct {
    __u64 retrans_count;        /* 重传次数 */
    __u64 zero_window_count;    /* 零窗口通告事件次数 */
    __u64 queue_overflow_count; /* 队列溢出次数 */
    __u64 conn_fail_count;      /* 连接失败次数 */
    __u64 bytes_sent;           /* 发送字节数 */
    __u64 bytes_recv;           /* 接收字节数 */
    __u64 packets_sent;         /* 发送包数 */
    __u64 packets_recv;         /* 接收包数 */
    __u64 last_update;          /* 最后更新时间戳（纳秒） */
} tcp_stats_t;

/*
 * global_tcp_metrics_t - 全局 TCP 指标汇总
 *
 * 存储在 BPF_MAP_TYPE_ARRAY 中（key=0），用于全局维度的 TCP 健康度统计。
 */
typedef struct {
    __u64 total_connections;        /* 总连接数 */
    __u64 failed_connections;       /* 失败连接数 */
    __u64 total_retrans;            /* 总重传次数 */
    __u64 zero_window_events;       /* 零窗口事件总数 */
    __u64 queue_overflow_events;    /* 队列溢出事件总数 */
    __u64 avg_latency_ns;           /* 平均建连时延（纳秒） */
    __u64 max_latency_ns;           /* 最大建连时延（纳秒） */
    __u64 min_latency_ns;           /* 最小建连时延（纳秒） */
    __u64 latency_samples;          /* 时延样本数 */
} global_tcp_metrics_t;

/* ========================================================================
 * HTTP 指标数据结构（HTTP Metrics 子系统）
 * ======================================================================== */

/*
 * http_flow_key_t - HTTP 请求流标识键
 *
 * 与 tcp_conn_key_t 结构相同，用于 HTTP 指标子系统。
 */
typedef struct {
    __u32 saddr;       /* 源 IP 地址 */
    __u32 daddr;       /* 目的 IP 地址 */
    __u16 sport;       /* 源端口号 */
    __u16 dport;       /* 目的端口号 */
    __u32 pid;         /* 进程 ID */
} http_flow_key_t;

/*
 * http_request_t - HTTP 请求跟踪数据
 *
 * 记录单个 HTTP 请求的完整生命周期信息。
 */
typedef struct {
    __u64 request_ns;       /* 请求发送时间（纳秒） */
    __u64 response_ns;      /* 响应接收时间（纳秒） */
    __u64 latency_ns;       /* 响应时延（纳秒） */
    __u16 status_code;      /* HTTP 状态码 */
    __u8  has_response;     /* 是否已收到响应 */
    __u8  is_error;         /* 是否为异常响应（4xx/5xx） */
    __u64 request_bytes;    /* 请求体字节数 */
    __u64 response_bytes;   /* 响应体字节数 */
} http_request_t;

/*
 * http_stats_t - HTTP 流级聚合统计
 *
 * 按流维度（saddr:daddr:sport:dport:pid）聚合的 HTTP 统计。
 */
typedef struct {
    __u64 request_count;        /* 请求数 */
    __u64 response_count;       /* 响应数 */
    __u64 success_count;        /* 成功响应数（2xx） */
    __u64 error_count;          /* 异常响应数（4xx/5xx） */
    __u64 total_latency_ns;     /* 总时延（纳秒） */
    __u64 avg_latency_ns;       /* 平均时延（纳秒） */
    __u64 max_latency_ns;       /* 最大时延（纳秒） */
    __u64 min_latency_ns;       /* 最小时延（纳秒） */
    __u64 total_request_bytes;  /* 总请求字节数 */
    __u64 total_response_bytes; /* 总响应字节数 */
    __u64 last_update;          /* 最后更新时间（纳秒） */
} http_stats_t;

/*
 * global_http_metrics_t - 全局 HTTP 指标汇总
 */
typedef struct {
    __u64 total_requests;       /* 总请求数 */
    __u64 total_responses;      /* 总响应数 */
    __u64 success_responses;    /* 成功响应数 */
    __u64 error_responses;      /* 异常响应数 */
    __u64 avg_latency_ns;       /* 全局平均时延（纳秒） */
    __u64 max_latency_ns;       /* 全局最大时延（纳秒） */
    __u64 min_latency_ns;       /* 全局最小时延（纳秒） */
    __u64 latency_samples;      /* 时延样本数 */
} global_http_metrics_t;

/* ========================================================================
 * CPU 剖析数据结构（CPU Profiler 子系统）
 * ======================================================================== */

/*
 * prof_event_t - CPU 采样事件
 *
 * 通过 perf_event_array 发送到用户态的采样事件。
 */
typedef struct {
    __u64 timestamp;        /* 时间戳（纳秒） */
    __u32 pid;              /* 进程 ID */
    __u32 tid;              /* 线程 ID */
    __u32 cpu;              /* CPU 核心编号 */
    __u8  comm[16];         /* 进程名 */
    __u64 user_stack_id;    /* 用户态栈 ID */
    __u64 kernel_stack_id;  /* 内核态栈 ID */
    __u64 stack_trace[BPF_MAX_STACK_DEPTH]; /* 合并后的栈帧数组 */
    __u32 stack_len;        /* 有效栈帧数量 */
    __u8  event_type;       /* 事件类型：EVENT_TYPE_SAMPLE / EVENT_TYPE_LOST */
    __u8  padding[3];       /* 对齐填充 */
} prof_event_t;

/*
 * prof_control_t - CPU 剖析控制参数
 *
 * 用户态通过更新此结构体来动态控制剖析行为。
 */
typedef struct {
    __u32 sample_freq;       /* 采样频率（Hz），默认 99 */
    __u32 enabled;           /* 是否启用：0=禁用, 1=启用 */
    __u32 stack_depth;       /* 最大栈深度 */
    __u32 duration_ms;       /* 剖析持续时间（毫秒），0=连续 */
    __u64 filter_pid;        /* 过滤进程 ID，0=全部进程 */
    __u64 start_time;        /* 开始时间（纳秒） */
} prof_control_t;

/*
 * sample_stats_t - 采样统计
 */
typedef struct {
    __u64 total_samples;     /* 总采样数 */
    __u64 lost_samples;      /* 丢失采样数 */
    __u64 user_samples;      /* 用户态采样数 */
    __u64 kernel_samples;    /* 内核态采样数 */
    __u64 last_update;       /* 最后更新时间（纳秒） */
} sample_stats_t;

/*
 * proc_info_t - 进程信息缓存
 */
typedef struct {
    __u32 pid;               /* 进程 ID */
    __u8  comm[16];          /* 进程名 */
    __u8  lang_type;         /* 语言类型：0=未知, 1=C/C++, 2=Go, 3=Java */
    __u8  padding[3];        /* 对齐填充 */
} proc_info_t;

/* ========================================================================
 * 编译时断言 - 确保结构体大小与 Go 端一致
 * ======================================================================== */

/*
 * _STATIC_ASSERT - 编译时静态断言
 * 如果条件为 false，将产生编译错误。
 */
#define _STATIC_ASSERT(cond, msg) typedef char static_assertion_##msg[(cond) ? 1 : -1]

/* flow_key_t 必须为 12 字节（与 Go 端 bpfKeySize=12 一致） */
_STATIC_ASSERT(sizeof(flow_key_t) == 12, flow_key_must_be_12_bytes);

/* tcp_conn_key_t 必须为 16 字节 */
_STATIC_ASSERT(sizeof(tcp_conn_key_t) == 16, tcp_conn_key_must_be_16_bytes);

/* tcp_latency_t 必须为 40 字节 */
_STATIC_ASSERT(sizeof(tcp_latency_t) == 40, tcp_latency_must_be_40_bytes);

/* tcp_stats_t 必须为 80 字节 */
_STATIC_ASSERT(sizeof(tcp_stats_t) == 80, tcp_stats_must_be_80_bytes);

/* global_tcp_metrics_t 必须为 80 字节 */
_STATIC_ASSERT(sizeof(global_tcp_metrics_t) == 80, global_tcp_metrics_must_be_80_bytes);

#endif /* __BPF_HEADERS_H__ */
