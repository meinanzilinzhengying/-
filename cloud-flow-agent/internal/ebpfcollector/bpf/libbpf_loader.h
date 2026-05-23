// SPDX-License-Identifier: GPL-2.0 OR BSD-3-Clause
/*
 * libbpf_loader.h - eBPF libbpf 加载器接口定义
 *
 * 本文件定义了基于 libbpf 的 eBPF 程序加载器接口，
 * 提供 BPF 对象加载、TC 挂载、kprobe 挂载、数据采集和资源清理等功能。
 *
 * 设计要点：
 *   - 使用 libbpf 的 bpf_object API 加载编译好的 .o 文件
 *   - TC 挂载通过 netlink (libnl) 或 bpf_prog_attach 实现
 *   - 所有函数返回 int 错误码，0 表示成功
 *   - 支持通过配置选择加载哪些子系统
 *
 * Copyright (c) 2024 cloud-flow contributors
 */

#ifndef __LIBBPF_LOADER_H__
#define __LIBBPF_LOADER_H__

#include <linux/types.h>
#include <linux/if.h>
#include <bpf/libbpf.h>
#include <bpf/bpf.h>

#include "bpf_headers.h"

#ifdef __cplusplus
extern "C" {
#endif

/* ========================================================================
 * 常量定义
 * ======================================================================== */

/* BPF 对象文件路径（相对于工作目录或绝对路径） */
#define BPF_OBJ_PATH_TC              "tc.bpf.o"
#define BPF_OBJ_PATH_TCP_METRICS     "tcp_metrics.bpf.o"
#define BPF_OBJ_PATH_HTTP_METRICS    "http_metrics.bpf.o"
#define BPF_OBJ_PATH_DNS_FULL        "dns_full.bpf.o"
#define BPF_OBJ_PATH_HTTP_FULL       "http_full.bpf.o"
#define BPF_OBJ_PATH_MYSQL_FULL      "mysql_full.bpf.o"
#define BPF_OBJ_PATH_CPU_PROFILER    "cpu_profiler.bpf.o"

/* TC 挂载默认参数 */
#define TC_DEFAULT_PRIORITY          1       /* TC 分类器优先级 */
#define TC_DEFAULT_HANDLE            1       /* TC 分类器句柄 */
#define TC_DEFAULT_DIRECT_ACTION     1       /* TC 直接动作模式 */
#define TC_MAX_INTERFACE_NAME        IFNAMSIZ /* 网卡名称最大长度 */

/* kprobe 最大数量 */
#define MAX_KPROBE_LINKS             64

/* BPF 对象最大数量（每个子系统一个） */
#define MAX_BPF_OBJECTS              8

/* ========================================================================
 * 数据结构定义
 * ======================================================================== */

/*
 * bpf_loader_config - 加载器配置
 *
 * 控制加载器行为的运行时配置参数。
 */
typedef struct {
    /* 子系统启用掩码，按位 OR 组合 BPF_SUBSYS_* 常量 */
    __u32 enabled_subsystems;

    /* TC 子系统配置 */
    char tc_interface[TC_MAX_INTERFACE_NAME]; /* TC 挂载的网卡名称 */
    __u32 tc_priority;                        /* TC 分类器优先级 */
    __u32 tc_handle;                          /* TC 分类器句柄 */
    __u32 tc_direct_action;                   /* 是否使用 direct-action 模式 */

    /* CPU 剖析配置 */
    __u32 cpu_sample_freq;    /* 采样频率（Hz） */
    __u32 cpu_duration_ms;    /* 剖析持续时间（毫秒），0=连续 */
    __u64 cpu_filter_pid;     /* 过滤进程 ID，0=全部 */

    /* BPF 对象文件路径（可覆盖默认路径） */
    char obj_path_tc[PATH_MAX];
    char obj_path_tcp_metrics[PATH_MAX];
    char obj_path_http_metrics[PATH_MAX];
    char obj_path_dns_full[PATH_MAX];
    char obj_path_http_full[PATH_MAX];
    char obj_path_mysql_full[PATH_MAX];
    char obj_path_cpu_profiler[PATH_MAX];

    /* 内核版本约束（0 表示不限制） */
    __u32 min_kernel_major;
    __u32 min_kernel_minor;
    __u32 min_kernel_patch;

    /* 是否启用自动清理（进程退出时自动 detach） */
    int auto_cleanup;

    /* 日志级别：0=静默, 1=错误, 2=警告, 3=信息, 4=调试 */
    int log_level;
} bpf_loader_config_t;

/*
 * bpf_subsys_state - 单个子系统的运行时状态
 *
 * 记录每个子系统加载的 BPF 对象、链接和 Map 信息。
 */
typedef struct {
    /* 子系统标识（BPF_SUBSYS_* 常量） */
    __u32 subsys_id;

    /* 是否已加载和挂载 */
    int loaded;
    int attached;

    /* libbpf BPF 对象句柄 */
    struct bpf_object *obj;

    /* BPF 程序句柄（子系统内可能有多个程序） */
    struct bpf_program **programs;
    int program_count;

    /* BPF 链接句柄数组（kprobe/tracepoint 等链接） */
    struct bpf_link **links;
    int link_count;

    /* BPF Map 句柄数组 */
    struct bpf_map **maps;
    int map_count;

    /* TC 相关的文件描述符和配置 */
    int tc_prog_fd;           /* TC BPF 程序的文件描述符 */
    int tc_cls_fd;            /* TC 分类器文件描述符 */
    int tc_qdisc_fd;          /* TC qdisc 文件描述符 */
    __u32 tc_classid;         /* TC classid */

    /* perf event 文件描述符（CPU profiler 使用） */
    int *perf_event_fds;
    int perf_event_count;
    int perf_cpu_count;
} bpf_subsys_state_t;

/*
 * bpf_loader_ctx - BPF 加载器全局上下文
 *
 * 管理所有子系统的生命周期，是加载器的核心数据结构。
 * 使用前必须调用 bpf_loader_init() 进行初始化。
 */
typedef struct {
    /* 加载器是否已初始化 */
    int initialized;

    /* 加载器配置 */
    bpf_loader_config_t config;

    /* 子系统状态数组 */
    bpf_subsys_state_t subsystems[MAX_BPF_OBJECTS];
    int subsystem_count;

    /* 全局错误信息（最近一次操作的错误描述） */
    char last_error[256];
} bpf_loader_ctx_t;

/*
 * network_map_entry - 网络流量 Map 遍历回调数据
 *
 * 用于 iterate_network_map 回调函数，传递每条流记录。
 */
typedef struct {
    flow_key_t key;
    network_data_t value;
} network_map_entry_t;

/*
 * tcp_flow_stats_entry - TCP 流统计 Map 遍历回调数据（五元组+进程维度聚合）
 */
typedef struct {
    tcp_conn_key_t key;
    tcp_flow_stats_t value;
} tcp_flow_stats_entry_t;

/*
 * map_iterate_callback - Map 遍历回调函数类型
 *
 * @param entry: 当前遍历到的 Map 条目
 * @param ctx:   用户自定义上下文
 * @return: 0=继续遍历, 非0=停止遍历
 */
typedef int (*map_iterate_callback)(const void *entry, void *ctx);

/* ========================================================================
 * 初始化与配置函数
 * ======================================================================== */

/**
 * bpf_loader_get_default_config() - 获取默认加载器配置
 *
 * 返回一个预填充了默认值的配置结构体。
 * 调用者可修改此配置后传递给 bpf_loader_init()。
 *
 * @return: 默认配置结构体
 */
bpf_loader_config_t bpf_loader_get_default_config(void);

/**
 * bpf_loader_init() - 初始化 BPF 加载器
 *
 * 根据配置加载指定的 BPF 子系统。此函数会：
 *   1. 检查内核版本是否满足要求
 *   2. 读取环境变量覆盖配置（可选）
 *   3. 调用 bpf_object__open_file 加载 .o 文件
 *   4. 调用 bpf_object__load 将程序加载到内核
 *
 * @param ctx:     加载器上下文（调用者分配）
 * @param config:  加载器配置，NULL 使用默认配置
 * @return: 0=成功, 负数=错误码（BPF_LOADER_ERR_*）
 */
int bpf_loader_init(bpf_loader_ctx_t *ctx, const bpf_loader_config_t *config);

/**
 * bpf_loader_check_kernel_version() - 检查内核版本
 *
 * 验证当前内核版本是否满足最低要求。
 *
 * @param major: 最低主版本号
 * @param minor: 最低次版本号
 * @param patch: 最低补丁版本号
 * @return: 0=满足要求, BPF_LOADER_ERR_KERN_VERSION=不满足
 */
int bpf_loader_check_kernel_version(__u32 major, __u32 minor, __u32 patch);

/**
 * bpf_loader_get_last_error() - 获取最近一次错误描述
 *
 * @param ctx: 加载器上下文
 * @return: 错误描述字符串（只读，无需释放）
 */
const char *bpf_loader_get_last_error(const bpf_loader_ctx_t *ctx);

/* ========================================================================
 * TC 挂载函数
 * ======================================================================== */

/**
 * bpf_loader_attach_tc() - 挂载 TC BPF 程序
 *
 * 将 TC 子系统的 BPF 程序挂载到指定网卡。
 * 实现方式：
 *   - 使用 rtnetlink (libnl) 创建 clsact qdisc
 *   - 添加 tc filter 将 BPF 程序绑定到 ingress/egress 方向
 *   - 支持 direct-action 模式
 *
 * @param ctx:       加载器上下文
 * @param interface: 网卡名称（如 "eth0"），NULL 使用配置中的默认值
 * @param ingress:   挂载方向：1=ingress, 0=egress
 * @return: 0=成功, 负数=错误码
 */
int bpf_loader_attach_tc(bpf_loader_ctx_t *ctx, const char *interface, int ingress);

/**
 * bpf_loader_detach_tc() - 卸载 TC BPF 程序
 *
 * 从指定网卡卸载 TC BPF 程序并清理相关资源。
 *
 * @param ctx:       加载器上下文
 * @param interface: 网卡名称，NULL 使用配置中的默认值
 * @return: 0=成功, 负数=错误码
 */
int bpf_loader_detach_tc(bpf_loader_ctx_t *ctx, const char *interface);

/* ========================================================================
 * kprobe 挂载函数
 * ======================================================================== */

/**
 * bpf_loader_attach_kprobes() - 挂载所有 kprobe 探针
 *
 * 遍历所有已加载子系统的 BPF 程序，自动识别并挂载 kprobe 类型的程序。
 * 使用 bpf_program__attach_kprobe() API 进行挂载。
 *
 * @param ctx: 加载器上下文
 * @return: 0=成功, 负数=错误码
 */
int bpf_loader_attach_kprobes(bpf_loader_ctx_t *ctx);

/**
 * bpf_loader_attach_kprobe() - 挂载单个 kprobe 探针
 *
 * @param ctx:      加载器上下文
 * @param subsys:   目标子系统 ID（BPF_SUBSYS_*）
 * @param prog_name: BPF 程序名称（SEC 名称）
 * @param func_name: 内核函数名称
 * @param retprobe:  是否为返回探针（kretprobe）
 * @return: 0=成功, 负数=错误码
 */
int bpf_loader_attach_kprobe(bpf_loader_ctx_t *ctx, __u32 subsys,
                              const char *prog_name, const char *func_name,
                              int retprobe);

/**
 * bpf_loader_detach_kprobes() - 卸载所有 kprobe 探针
 *
 * @param ctx: 加载器上下文
 * @return: 0=成功, 负数=错误码
 */
int bpf_loader_detach_kprobes(bpf_loader_ctx_t *ctx);

/* ========================================================================
 * 数据采集函数
 * ======================================================================== */

/**
 * bpf_loader_collect_network() - 采集网络流量数据
 *
 * 遍历 TC 子系统的 network_map，收集所有流的聚合数据。
 *
 * @param ctx:      加载器上下文
 * @param callback: 遍历回调函数
 * @param user_ctx: 用户自定义上下文（传递给回调函数）
 * @return: 0=成功, 负数=错误码
 */
int bpf_loader_collect_network(bpf_loader_ctx_t *ctx,
                                map_iterate_callback callback,
                                void *user_ctx);

/**
 * bpf_loader_collect_tcp_metrics() - 采集 TCP 指标数据
 *
 * 遍历 TCP Metrics 子系统的 Map，收集 TCP 连接级和全局指标。
 *
 * @param ctx:      加载器上下文
 * @param callback: 遍历回调函数
 * @param user_ctx: 用户自定义上下文
 * @return: 0=成功, 负数=错误码
 */
int bpf_loader_collect_tcp_metrics(bpf_loader_ctx_t *ctx,
                                    map_iterate_callback callback,
                                    void *user_ctx);

/**
 * bpf_loader_lookup_global_metrics() - 查询全局 TCP 指标
 *
 * 从 global_tcp_metrics_map 中读取全局 TCP 汇总指标。
 *
 * @param ctx:     加载器上下文
 * @param metrics: 输出缓冲区，用于存储全局指标
 * @return: 0=成功, 负数=错误码
 */
int bpf_loader_lookup_global_metrics(bpf_loader_ctx_t *ctx,
                                      global_tcp_metrics_t *metrics);

/**
 * bpf_loader_clear_global_metrics() - 清零全局 TCP 指标
 *
 * 将 global_tcp_metrics_map 中的全局指标重置为零。
 *
 * @param ctx: 加载器上下文
 * @return: 0=成功, 负数=错误码
 */
int bpf_loader_clear_global_metrics(bpf_loader_ctx_t *ctx);

/**
 * bpf_loader_iterate_map() - 通用 Map 遍历函数
 *
 * 遍历指定子系统的指定 Map。
 *
 * @param ctx:       加载器上下文
 * @param subsys:    子系统 ID
 * @param map_name:  Map 名称
 * @param callback:  遍历回调函数
 * @param user_ctx:  用户自定义上下文
 * @return: 0=成功, 负数=错误码
 */
int bpf_loader_iterate_map(bpf_loader_ctx_t *ctx, __u32 subsys,
                            const char *map_name,
                            map_iterate_callback callback,
                            void *user_ctx);

/* ========================================================================
 * CPU 剖析函数
 * ======================================================================== */

/**
 * bpf_loader_start_cpu_profiler() - 启动 CPU 剖析
 *
 * 配置并启动 perf_event 采样。
 *
 * @param ctx: 加载器上下文
 * @return: 0=成功, 负数=错误码
 */
int bpf_loader_start_cpu_profiler(bpf_loader_ctx_t *ctx);

/**
 * bpf_loader_stop_cpu_profiler() - 停止 CPU 剖析
 *
 * @param ctx: 加载器上下文
 * @return: 0=成功, 负数=错误码
 */
int bpf_loader_stop_cpu_profiler(bpf_loader_ctx_t *ctx);

/**
 * bpf_loader_read_prof_events() - 读取剖析事件
 *
 * 从 perf_event_array 中读取采样事件。
 *
 * @param ctx:      加载器上下文
 * @param callback: 事件回调函数
 * @param user_ctx: 用户自定义上下文
 * @return: 0=成功, 负数=错误码
 */
int bpf_loader_read_prof_events(bpf_loader_ctx_t *ctx,
                                 map_iterate_callback callback,
                                 void *user_ctx);

/* ========================================================================
 * 资源清理函数
 * ======================================================================== */

/**
 * bpf_loader_cleanup() - 清理所有 BPF 资源
 *
 * 按顺序执行以下清理操作：
 *   1. 停止 CPU 剖析（如果正在运行）
 *   2. 卸载所有 kprobe 链接
 *   3. 卸载 TC 程序
 *   4. 关闭所有 BPF Map
 *   5. 释放所有 BPF 对象
 *   6. 重置上下文状态
 *
 * @param ctx: 加载器上下文
 * @return: 0=成功, 负数=错误码（会尝试继续清理其他资源）
 */
int bpf_loader_cleanup(bpf_loader_ctx_t *ctx);

/**
 * bpf_loader_cleanup_subsystem() - 清理指定子系统的资源
 *
 * @param ctx:    加载器上下文
 * @param subsys: 子系统 ID
 * @return: 0=成功, 负数=错误码
 */
int bpf_loader_cleanup_subsystem(bpf_loader_ctx_t *ctx, __u32 subsys);

/* ========================================================================
 * 辅助查询函数
 * ======================================================================== */

/**
 * bpf_loader_is_subsys_loaded() - 检查子系统是否已加载
 *
 * @param ctx:    加载器上下文
 * @param subsys: 子系统 ID
 * @return: 1=已加载, 0=未加载
 */
int bpf_loader_is_subsys_loaded(const bpf_loader_ctx_t *ctx, __u32 subsys);

/**
 * bpf_loader_is_subsys_attached() - 检查子系统是否已挂载
 *
 * @param ctx:    加载器上下文
 * @param subsys: 子系统 ID
 * @return: 1=已挂载, 0=未挂载
 */
int bpf_loader_is_subsys_attached(const bpf_loader_ctx_t *ctx, __u32 subsys);

/**
 * bpf_loader_get_map_fd() - 获取指定 Map 的文件描述符
 *
 * @param ctx:      加载器上下文
 * @param subsys:   子系统 ID
 * @param map_name: Map 名称
 * @return: Map 文件描述符（>=0），负数=错误
 */
int bpf_loader_get_map_fd(bpf_loader_ctx_t *ctx, __u32 subsys,
                           const char *map_name);

/**
 * bpf_loader_get_prog_fd() - 获取指定程序的文件描述符
 *
 * @param ctx:       加载器上下文
 * @param subsys:    子系统 ID
 * @param prog_name: 程序名称
 * @return: 程序文件描述符（>=0），负数=错误
 */
int bpf_loader_get_prog_fd(bpf_loader_ctx_t *ctx, __u32 subsys,
                            const char *prog_name);

/**
 * bpf_loader_get_subsys_count() - 获取已加载的子系统数量
 *
 * @param ctx: 加载器上下文
 * @return: 已加载的子系统数量
 */
int bpf_loader_get_subsys_count(const bpf_loader_ctx_t *ctx);

#ifdef __cplusplus
}
#endif

#endif /* __LIBBPF_LOADER_H__ */
