// SPDX-License-Identifier: GPL-2.0 OR BSD-3-Clause
/*
 * libbpf_loader.c - eBPF libbpf 加载器实现
 *
 * 本文件实现了基于 libbpf 的 eBPF 程序加载器，提供以下功能：
 *   - BPF 对象文件加载与内核注入
 *   - TC 分类器挂载（通过 netlink rtnetlink API）
 *   - kprobe/kretprobe 探针挂载
 *   - BPF Map 数据读取与遍历
 *   - CPU 剖析 perf_event 管理
 *   - 资源生命周期管理
 *
 * 依赖：libbpf, libnl-3 (TC 挂载)
 *
 * Copyright (c) 2024 cloud-flow contributors
 */

#include <stdio.h>
#include <stdlib.h>
#include <string.h>
#include <errno.h>
#include <unistd.h>
#include <fcntl.h>
#include <sys/stat.h>
#include <sys/types.h>
#include <sys/socket.h>
#include <sys/syscall.h>
#include <linux/version.h>
#include <linux/netlink.h>
#include <linux/rtnetlink.h>
#include <linux/tc_act/tc_bpf.h>
#include <net/if.h>
#include <arpa/inet.h>
#include <pthread.h>

#include <bpf/libbpf.h>
#include <bpf/bpf.h>

#include "libbpf_loader.h"
#include "bpf_headers.h"

/* ========================================================================
 * 内部日志宏
 * ======================================================================== */

#define _LOG_LEVEL_NONE    0
#define _LOG_LEVEL_ERROR   1
#define _LOG_LEVEL_WARN    2
#define _LOG_LEVEL_INFO    3
#define _LOG_LEVEL_DEBUG   4

#define _log_error(ctx, fmt, ...) do { \
    if ((ctx) && (ctx)->config.log_level >= _LOG_LEVEL_ERROR) { \
        fprintf(stderr, "[BPF_LOADER ERROR] " fmt "\n", ##__VA_ARGS__); \
    } \
    _set_error(ctx, fmt, ##__VA_ARGS__); \
} while (0)

#define _log_warn(ctx, fmt, ...) do { \
    if ((ctx) && (ctx)->config.log_level >= _LOG_LEVEL_WARN) { \
        fprintf(stderr, "[BPF_LOADER WARN] " fmt "\n", ##__VA_ARGS__); \
    } \
} while (0)

#define _log_info(ctx, fmt, ...) do { \
    if ((ctx) && (ctx)->config.log_level >= _LOG_LEVEL_INFO) { \
        fprintf(stdout, "[BPF_LOADER INFO] " fmt "\n", ##__VA_ARGS__); \
    } \
} while (0)

#define _log_debug(ctx, fmt, ...) do { \
    if ((ctx) && (ctx)->config.log_level >= _LOG_LEVEL_DEBUG) { \
        fprintf(stdout, "[BPF_LOADER DEBUG] " fmt "\n", ##__VA_ARGS__); \
    } \
} while (0)

/* ========================================================================
 * 内部辅助函数
 * ======================================================================== */

/**
 * _set_error() - 设置上下文的最近错误信息
 */
static void _set_error(bpf_loader_ctx_t *ctx, const char *fmt, ...)
{
    if (!ctx)
        return;

    va_list args;
    va_start(args, fmt);
    vsnprintf(ctx->last_error, sizeof(ctx->last_error), fmt, args);
    va_end(args);
}

/**
 * _find_subsystem() - 根据 subsystem ID 查找子系统状态
 *
 * @return: 子系统状态指针，未找到返回 NULL
 */
static bpf_subsys_state_t *_find_subsystem(bpf_loader_ctx_t *ctx, __u32 subsys_id)
{
    if (!ctx)
        return NULL;

    for (int i = 0; i < ctx->subsystem_count; i++) {
        if (ctx->subsystems[i].subsys_id == subsys_id)
            return &ctx->subsystems[i];
    }
    return NULL;
}

/**
 * _get_obj_path() - 获取子系统对应的 BPF 对象文件路径
 *
 * 优先使用配置中自定义的路径，否则使用默认路径。
 */
static const char *_get_obj_path(const bpf_loader_config_t *config, __u32 subsys_id)
{
    switch (subsys_id) {
    case BPF_SUBSYS_TC:
        return (config->obj_path_tc[0] != '\0') ? config->obj_path_tc
                                                  : BPF_OBJ_PATH_TC;
    case BPF_SUBSYS_TCP_METRICS:
        return (config->obj_path_tcp_metrics[0] != '\0') ? config->obj_path_tcp_metrics
                                                          : BPF_OBJ_PATH_TCP_METRICS;
    case BPF_SUBSYS_HTTP_METRICS:
        return (config->obj_path_http_metrics[0] != '\0') ? config->obj_path_http_metrics
                                                           : BPF_OBJ_PATH_HTTP_METRICS;
    case BPF_SUBSYS_DNS_FULL:
        return (config->obj_path_dns_full[0] != '\0') ? config->obj_path_dns_full
                                                       : BPF_OBJ_PATH_DNS_FULL;
    case BPF_SUBSYS_HTTP_FULL:
        return (config->obj_path_http_full[0] != '\0') ? config->obj_path_http_full
                                                        : BPF_OBJ_PATH_HTTP_FULL;
    case BPF_SUBSYS_MYSQL_FULL:
        return (config->obj_path_mysql_full[0] != '\0') ? config->obj_path_mysql_full
                                                         : BPF_OBJ_PATH_MYSQL_FULL;
    case BPF_SUBSYS_CPU_PROFILER:
        return (config->obj_path_cpu_profiler[0] != '\0') ? config->obj_path_cpu_profiler
                                                           : BPF_OBJ_PATH_CPU_PROFILER;
    default:
        return NULL;
    }
}

/**
 * _get_subsys_name() - 获取子系统的可读名称（用于日志）
 */
static const char *_get_subsys_name(__u32 subsys_id)
{
    switch (subsys_id) {
    case BPF_SUBSYS_TC:           return "TC";
    case BPF_SUBSYS_TCP_METRICS:  return "TCP Metrics";
    case BPF_SUBSYS_HTTP_METRICS: return "HTTP Metrics";
    case BPF_SUBSYS_DNS_FULL:     return "DNS Full";
    case BPF_SUBSYS_HTTP_FULL:    return "HTTP Full";
    case BPF_SUBSYS_MYSQL_FULL:   return "MySQL Full";
    case BPF_SUBSYS_CPU_PROFILER: return "CPU Profiler";
    default:                      return "Unknown";
    }
}

/**
 * _load_single_subsystem() - 加载单个子系统的 BPF 对象
 *
 * 打开并加载指定子系统的 .o 文件到内核。
 */
static int _load_single_subsystem(bpf_loader_ctx_t *ctx, __u32 subsys_id)
{
    const char *obj_path;
    struct bpf_object *obj = NULL;
    struct bpf_program *prog = NULL;
    struct bpf_map *map = NULL;
    bpf_subsys_state_t *subsys;
    int prog_count = 0, map_count = 0;
    int ret;

    if (!ctx)
        return BPF_LOADER_ERR_INVALID_ARG;

    /* 检查是否已加载 */
    subsys = _find_subsystem(ctx, subsys_id);
    if (subsys && subsys->loaded) {
        _log_debug(ctx, "subsystem %s already loaded, skipping",
                   _get_subsys_name(subsys_id));
        return BPF_LOADER_OK;
    }

    /* 获取对象文件路径 */
    obj_path = _get_obj_path(&ctx->config, subsys_id);
    if (!obj_path) {
        _log_error(ctx, "unknown subsystem id %u", subsys_id);
        return BPF_LOADER_ERR_INVALID_ARG;
    }

    _log_info(ctx, "loading BPF object for subsystem %s from %s",
              _get_subsys_name(subsys_id), obj_path);

    /* 设置 libbpf 日志级别 */
    if (ctx->config.log_level >= _LOG_LEVEL_DEBUG) {
        libbpf_set_print(NULL);
    } else {
        libbpf_set_print(NULL);
    }

    /* 打开 BPF 对象文件 */
    obj = bpf_object__open_file(obj_path, NULL);
    if (libbpf_get_error(obj)) {
        _log_error(ctx, "failed to open BPF object %s: %s",
                   obj_path, strerror(errno));
        return BPF_LOADER_ERR_OPEN_FILE;
    }

    /* 加载 BPF 对象到内核 */
    ret = bpf_object__load(obj);
    if (ret) {
        _log_error(ctx, "failed to load BPF object %s: %s",
                   obj_path, strerror(-ret));
        bpf_object__close(obj);
        return BPF_LOADER_ERR_LOAD;
    }

    /* 分配子系统状态槽位 */
    if (ctx->subsystem_count >= MAX_BPF_OBJECTS) {
        _log_error(ctx, "too many subsystems (max %d)", MAX_BPF_OBJECTS);
        bpf_object__close(obj);
        return BPF_LOADER_ERR_INVALID_ARG;
    }

    subsys = &ctx->subsystems[ctx->subsystem_count];
    memset(subsys, 0, sizeof(*subsys));
    subsys->subsys_id = subsys_id;
    subsys->obj = obj;
    subsys->loaded = 1;
    subsys->attached = 0;
    subsys->tc_prog_fd = -1;
    subsys->tc_cls_fd = -1;
    subsys->tc_qdisc_fd = -1;

    /* 统计程序数量 */
    prog_count = 0;
    bpf_object__for_each_program(prog, obj) {
        prog_count++;
    }

    /* 统计 Map 数量 */
    map_count = 0;
    bpf_object__for_each_map(map, obj) {
        map_count++;
    }

    _log_info(ctx, "subsystem %s loaded: %d programs, %d maps",
              _get_subsys_name(subsys_id), prog_count, map_count);

    subsys->program_count = prog_count;
    subsys->map_count = map_count;

    /* 分配链接数组（预分配最大容量） */
    if (prog_count > 0) {
        subsys->links = calloc(prog_count, sizeof(struct bpf_link *));
        if (!subsys->links) {
            _log_error(ctx, "failed to allocate links array for %s",
                       _get_subsys_name(subsys_id));
            bpf_object__close(obj);
            memset(subsys, 0, sizeof(*subsys));
            return BPF_LOADER_ERR_NOMEM;
        }
    }

    ctx->subsystem_count++;

    return BPF_LOADER_OK;
}

/* ========================================================================
 * libbpf 日志回调
 * ======================================================================== */

/*
 * _libbpf_print_fn - libbpf 日志回调函数
 *
 * 将 libbpf 的内部日志转发到加载器的日志系统。
 */
static int _libbpf_print_fn(enum libbpf_print_level level,
                             const char *format, va_list args)
{
    if (level == LIBBPF_DEBUG)
        return 0;

    /*
     * 注意：此处无法访问 ctx，因此直接输出到 stderr。
     * 生产环境中可通过 thread-local 变量或全局回调实现更精细的控制。
     */
    return vfprintf(stderr, format, args);
}

/* ========================================================================
 * 初始化与配置实现
 * ======================================================================== */

bpf_loader_config_t bpf_loader_get_default_config(void)
{
    bpf_loader_config_t config;

    memset(&config, 0, sizeof(config));

    /* 默认启用 TC 和 TCP Metrics 子系统 */
    config.enabled_subsystems = BPF_SUBSYS_TC | BPF_SUBSYS_TCP_METRICS;

    /* TC 默认配置 */
    strncpy(config.tc_interface, "eth0", sizeof(config.tc_interface) - 1);
    config.tc_priority = TC_DEFAULT_PRIORITY;
    config.tc_handle = TC_DEFAULT_HANDLE;
    config.tc_direct_action = TC_DEFAULT_DIRECT_ACTION;

    /* CPU 剖析默认配置 */
    config.cpu_sample_freq = 99;
    config.cpu_duration_ms = 0;   /* 连续模式 */
    config.cpu_filter_pid = 0;    /* 全部进程 */

    /* 内核版本约束（默认不限制） */
    config.min_kernel_major = 0;
    config.min_kernel_minor = 0;
    config.min_kernel_patch = 0;

    /* 自动清理 */
    config.auto_cleanup = 1;

    /* 默认日志级别：警告 */
    config.log_level = _LOG_LEVEL_WARN;

    return config;
}

int bpf_loader_check_kernel_version(__u32 major, __u32 minor, __u32 patch)
{
    if (LINUX_VERSION_CODE < KERNEL_VERSION(major, minor, patch)) {
        return BPF_LOADER_ERR_KERN_VERSION;
    }
    return BPF_LOADER_OK;
}

const char *bpf_loader_get_last_error(const bpf_loader_ctx_t *ctx)
{
    if (!ctx)
        return "invalid context";
    return ctx->last_error;
}

int bpf_loader_init(bpf_loader_ctx_t *ctx, const bpf_loader_config_t *config)
{
    bpf_loader_config_t default_config;
    __u32 subsys_mask;
    int ret;

    if (!ctx) {
        return BPF_LOADER_ERR_INVALID_ARG;
    }

    /* 清零上下文 */
    memset(ctx, 0, sizeof(*ctx));

    /* 使用默认配置或用户提供的配置 */
    if (config) {
        memcpy(&ctx->config, config, sizeof(*config));
    } else {
        default_config = bpf_loader_get_default_config();
        memcpy(&ctx->config, &default_config, sizeof(default_config));
    }

    /* 设置 libbpf 日志回调 */
    libbpf_set_print(_libbpf_print_fn);

    _log_info(ctx, "initializing BPF loader (subsystems mask: 0x%x)",
              ctx->config.enabled_subsystems);

    /* 检查内核版本约束 */
    if (ctx->config.min_kernel_major > 0) {
        ret = bpf_loader_check_kernel_version(
            ctx->config.min_kernel_major,
            ctx->config.min_kernel_minor,
            ctx->config.min_kernel_patch);
        if (ret != BPF_LOADER_OK) {
            _log_error(ctx,
                "kernel version %u.%u.%u does not meet minimum requirement %u.%u.%u",
                (LINUX_VERSION_CODE >> 16) & 0xFF,
                (LINUX_VERSION_CODE >> 8) & 0xFF,
                LINUX_VERSION_CODE & 0xFF,
                ctx->config.min_kernel_major,
                ctx->config.min_kernel_minor,
                ctx->config.min_kernel_patch);
            return ret;
        }
    }

    /* 读取环境变量覆盖配置 */
    const char *env_iface = getenv("BPF_TC_INTERFACE");
    if (env_iface) {
        strncpy(ctx->config.tc_interface, env_iface,
                sizeof(ctx->config.tc_interface) - 1);
        _log_info(ctx, "TC interface overridden by env: %s",
                  ctx->config.tc_interface);
    }

    const char *env_log = getenv("BPF_LOG_LEVEL");
    if (env_log) {
        int level = atoi(env_log);
        if (level >= 0 && level <= 4) {
            ctx->config.log_level = level;
        }
    }

    const char *env_subsys = getenv("BPF_SUBSYSTEMS");
    if (env_subsys) {
        __u32 env_mask = (__u32)strtoul(env_subsys, NULL, 0);
        if (env_mask != 0) {
            ctx->config.enabled_subsystems = env_mask;
            _log_info(ctx, "subsystems overridden by env: 0x%x", env_mask);
        }
    }

    /* 逐个加载启用的子系统 */
    subsys_mask = ctx->config.enabled_subsystems;

    /*
     * 按优先级顺序加载子系统：
     * 先加载无依赖的子系统（TC），再加载有依赖的子系统（TCP/HTTP Metrics）。
     */
    __u32 load_order[] = {
        BPF_SUBSYS_TC,
        BPF_SUBSYS_TCP_METRICS,
        BPF_SUBSYS_HTTP_METRICS,
        BPF_SUBSYS_DNS_FULL,
        BPF_SUBSYS_HTTP_FULL,
        BPF_SUBSYS_MYSQL_FULL,
        BPF_SUBSYS_CPU_PROFILER,
    };
    int load_order_count = sizeof(load_order) / sizeof(load_order[0]);

    for (int i = 0; i < load_order_count; i++) {
        if (subsys_mask & load_order[i]) {
            ret = _load_single_subsystem(ctx, load_order[i]);
            if (ret != BPF_LOADER_OK) {
                _log_warn(ctx, "failed to load subsystem %s (error %d), continuing",
                          _get_subsys_name(load_order[i]), ret);
                /* 非致命错误：继续加载其他子系统 */
            }
        }
    }

    ctx->initialized = 1;

    _log_info(ctx, "BPF loader initialized: %d subsystem(s) loaded",
              ctx->subsystem_count);

    return BPF_LOADER_OK;
}

/* ========================================================================
 * TC 挂载实现
 * ======================================================================== */

/**
 * _tc_add_qdisc() - 通过 netlink 添加 clsact qdisc
 *
 * 使用 rtnetlink API 在指定网卡上创建 clsact qdisc。
 * clsact qdisc 是 TC BPF 程序的载体。
 */
static int _tc_add_qdisc(bpf_loader_ctx_t *ctx, int ifindex)
{
    struct {
        struct nlmsghdr  n;
        struct tcmsg     t;
        char             buf[256];
    } req;
    struct rtattr *rta;
    int sock_fd, ret;

    memset(&req, 0, sizeof(req));

    /* 创建 netlink 路由套接字 */
    sock_fd = socket(AF_NETLINK, SOCK_RAW, NETLINK_ROUTE);
    if (sock_fd < 0) {
        _log_error(ctx, "failed to create netlink socket: %s", strerror(errno));
        return BPF_LOADER_ERR_NETLINK;
    }

    /* 构造 RTM_NEWQDISC 消息 */
    req.n.nlmsg_len = NLMSG_LENGTH(sizeof(struct tcmsg));
    req.n.nlmsg_flags = NLM_F_REQUEST | NLM_F_EXCL | NLM_F_CREATE;
    req.n.nlmsg_type = RTM_NEWQDISC;

    req.t.tcm_family = AF_UNSPEC;
    req.t.tcm_ifindex = ifindex;
    req.t.tcm_handle = TC_H_MAKE(TC_H_CLSACT, 0);
    req.t.tcm_parent = TC_H_CLSACT;

    /* 添加 kind 属性: "clsact" */
    rta = (struct rtattr *)((char *)&req + NLMSG_ALIGN(req.n.nlmsg_len));
    rta->rta_type = TCA_KIND;
    rta->rta_len = RTA_LENGTH(sizeof("clsact"));
    strcpy(RTA_DATA(rta), "clsact");
    req.n.nlmsg_len += RTA_ALIGN(rta->rta_len);

    /* 发送 netlink 消息 */
    ret = send(sock_fd, &req, req.n.nlmsg_len, 0);
    if (ret < 0) {
        /*
         * EEXIST 表示 qdisc 已存在，这通常不是错误。
         * 其他错误则需要报告。
         */
        if (errno != EEXIST) {
            _log_error(ctx, "failed to add clsact qdisc: %s", strerror(errno));
            close(sock_fd);
            return BPF_LOADER_ERR_NETLINK;
        }
        _log_debug(ctx, "clsact qdisc already exists on interface");
    }

    close(sock_fd);
    return BPF_LOADER_OK;
}

/**
 * _tc_add_filter() - 通过 netlink 添加 TC filter
 *
 * 将 BPF 程序作为 TC filter 绑定到指定方向（ingress/egress）。
 */
static int _tc_add_filter(bpf_loader_ctx_t *ctx, int ifindex,
                           int prog_fd, int ingress,
                           __u32 priority, __u32 handle,
                           int direct_action)
{
    struct {
        struct nlmsghdr  n;
        struct tcmsg     t;
        char             buf[512];
    } req;
    struct rtattr *rta;
    struct nlattr *nested;
    int sock_fd, ret;
    __u32 parent;

    memset(&req, 0, sizeof(req));

    /* 创建 netlink 路由套接字 */
    sock_fd = socket(AF_NETLINK, SOCK_RAW, NETLINK_ROUTE);
    if (sock_fd < 0) {
        _log_error(ctx, "failed to create netlink socket: %s", strerror(errno));
        return BPF_LOADER_ERR_NETLINK;
    }

    /* 确定挂载方向 */
    parent = ingress ? TC_H_MAKE(TC_H_CLSACT, TC_H_MIN_INGRESS)
                     : TC_H_MAKE(TC_H_CLSACT, TC_H_MIN_EGRESS);

    /* 构造 RTM_NEWTFILTER 消息 */
    req.n.nlmsg_len = NLMSG_LENGTH(sizeof(struct tcmsg));
    req.n.nlmsg_flags = NLM_F_REQUEST | NLM_F_EXCL | NLM_F_CREATE;
    req.n.nlmsg_type = RTM_NEWTFILTER;

    req.t.tcm_family = AF_UNSPEC;
    req.t.tcm_ifindex = ifindex;
    req.t.tcm_handle = handle;
    req.t.tcm_parent = parent;
    req.t.tcm_info = TC_H_MAKE(priority << 16, htons(ETH_P_ALL));

    /* 添加 kind 属性: "bpf" */
    rta = (struct rtattr *)((char *)&req + NLMSG_ALIGN(req.n.nlmsg_len));
    rta->rta_type = TCA_KIND;
    rta->rta_len = RTA_LENGTH(sizeof("bpf"));
    strcpy(RTA_DATA(rta), "bpf");
    req.n.nlmsg_len += RTA_ALIGN(rta->rta_len);

    /* 添加 options 属性（嵌套） */
    rta = (struct rtattr *)((char *)&req + NLMSG_ALIGN(req.n.nlmsg_len));
    rta->rta_type = TCA_OPTIONS;
    rta->rta_len = RTA_LENGTH(0);

    /* 嵌套属性: fd */
    nested = (struct nlattr *)((char *)rta + RTA_ALIGN(rta->rta_len));
    nested->nla_type = TCA_BPF_FD;
    nested->nla_len = RTA_LENGTH(sizeof(int));
    memcpy(RTA_DATA(nested), &prog_fd, sizeof(prog_fd));
    rta->rta_len += RTA_ALIGN(nested->nla_len);

    /* 嵌套属性: name */
    nested = (struct nlattr *)((char *)rta + RTA_ALIGN(rta->rta_len));
    nested->nla_type = TCA_BPF_NAME;
    const char *bpf_name = "cloud-flow-tc";
    nested->nla_len = RTA_LENGTH(strlen(bpf_name) + 1);
    strcpy(RTA_DATA(nested), bpf_name);
    rta->rta_len += RTA_ALIGN(nested->nla_len);

    /* 嵌套属性: direct_action */
    if (direct_action) {
        nested = (struct nlattr *)((char *)rta + RTA_ALIGN(rta->rta_len));
        nested->nla_type = TCA_BPF_FLAGS;
        __u32 flags = TCA_BPF_FLAG_ACT_DIRECT;
        nested->nla_len = RTA_LENGTH(sizeof(flags));
        memcpy(RTA_DATA(nested), &flags, sizeof(flags));
        rta->rta_len += RTA_ALIGN(nested->nla_len);

        nested = (struct nlattr *)((char *)rta + RTA_ALIGN(rta->rta_len));
        nested->nla_type = TCA_BPF_FLAGS_GEN;
        __u32 gen_flags = 1;
        nested->nla_len = RTA_LENGTH(sizeof(gen_flags));
        memcpy(RTA_DATA(nested), &gen_flags, sizeof(gen_flags));
        rta->rta_len += RTA_ALIGN(nested->nla_len);
    }

    req.n.nlmsg_len += RTA_ALIGN(rta->rta_len);

    /* 发送 netlink 消息 */
    ret = send(sock_fd, &req, req.n.nlmsg_len, 0);
    if (ret < 0) {
        if (errno != EEXIST) {
            _log_error(ctx, "failed to add TC filter: %s", strerror(errno));
            close(sock_fd);
            return BPF_LOADER_ERR_NETLINK;
        }
        _log_debug(ctx, "TC filter already exists");
    }

    close(sock_fd);
    return BPF_LOADER_OK;
}

/**
 * _tc_del_filter() - 通过 netlink 删除 TC filter
 */
static int _tc_del_filter(bpf_loader_ctx_t *ctx, int ifindex,
                           int ingress, __u32 priority)
{
    struct {
        struct nlmsghdr  n;
        struct tcmsg     t;
        char             buf[256];
    } req;
    int sock_fd, ret;
    __u32 parent;

    memset(&req, 0, sizeof(req));

    sock_fd = socket(AF_NETLINK, SOCK_RAW, NETLINK_ROUTE);
    if (sock_fd < 0) {
        _log_error(ctx, "failed to create netlink socket: %s", strerror(errno));
        return BPF_LOADER_ERR_NETLINK;
    }

    parent = ingress ? TC_H_MAKE(TC_H_CLSACT, TC_H_MIN_INGRESS)
                     : TC_H_MAKE(TC_H_CLSACT, TC_H_MIN_EGRESS);

    req.n.nlmsg_len = NLMSG_LENGTH(sizeof(struct tcmsg));
    req.n.nlmsg_flags = NLM_F_REQUEST;
    req.n.nlmsg_type = RTM_DELTFILTER;

    req.t.tcm_family = AF_UNSPEC;
    req.t.tcm_ifindex = ifindex;
    req.t.tcm_parent = parent;
    req.t.tcm_info = TC_H_MAKE(priority << 16, htons(ETH_P_ALL));

    /* 添加 kind 属性 */
    struct rtattr *rta = (struct rtattr *)((char *)&req + NLMSG_ALIGN(req.n.nlmsg_len));
    rta->rta_type = TCA_KIND;
    rta->rta_len = RTA_LENGTH(sizeof("bpf"));
    strcpy(RTA_DATA(rta), "bpf");
    req.n.nlmsg_len += RTA_ALIGN(rta->rta_len);

    ret = send(sock_fd, &req, req.n.nlmsg_len, 0);
    if (ret < 0 && errno != ENOENT) {
        _log_warn(ctx, "failed to delete TC filter: %s", strerror(errno));
    }

    close(sock_fd);
    return BPF_LOADER_OK;
}

int bpf_loader_attach_tc(bpf_loader_ctx_t *ctx, const char *interface, int ingress)
{
    bpf_subsys_state_t *subsys;
    struct bpf_program *prog;
    struct bpf_map *map;
    const char *iface;
    int ifindex, prog_fd, ret;

    if (!ctx || !ctx->initialized) {
        return BPF_LOADER_ERR_NOT_INITIALIZED;
    }

    subsys = _find_subsystem(ctx, BPF_SUBSYS_TC);
    if (!subsys || !subsys->loaded) {
        _log_error(ctx, "TC subsystem not loaded");
        return BPF_LOADER_ERR_NOT_INITIALIZED;
    }

    /* 确定网卡名称 */
    iface = interface ? interface : ctx->config.tc_interface;
    if (!iface || iface[0] == '\0') {
        _log_error(ctx, "no interface specified for TC attach");
        return BPF_LOADER_ERR_INVALID_ARG;
    }

    /* 获取网卡索引 */
    ifindex = if_nametoindex(iface);
    if (ifindex == 0) {
        _log_error(ctx, "interface %s not found: %s", iface, strerror(errno));
        return BPF_LOADER_ERR_ATTACH_TC;
    }

    _log_info(ctx, "attaching TC BPF program to %s (ifindex=%d, %s)",
              iface, ifindex, ingress ? "ingress" : "egress");

    /* 查找 TC 程序 */
    prog = bpf_object__find_program_by_name(subsys->obj, "tc_prog");
    if (!prog) {
        /* 尝试通过 SEC 名称查找 */
        prog = bpf_object__find_program_by_title(subsys->obj, "tc");
    }
    if (!prog) {
        _log_error(ctx, "TC program not found in BPF object");
        return BPF_LOADER_ERR_ATTACH_TC;
    }

    /* 获取程序 FD */
    prog_fd = bpf_program__fd(prog);
    if (prog_fd < 0) {
        _log_error(ctx, "failed to get TC program FD: %s", strerror(errno));
        return BPF_LOADER_ERR_ATTACH_TC;
    }

    subsys->tc_prog_fd = prog_fd;

    /* 添加 clsact qdisc */
    ret = _tc_add_qdisc(ctx, ifindex);
    if (ret != BPF_LOADER_OK) {
        _log_error(ctx, "failed to add clsact qdisc on %s", iface);
        return ret;
    }

    /* 添加 TC filter */
    ret = _tc_add_filter(ctx, ifindex, prog_fd, ingress,
                          ctx->config.tc_priority,
                          ctx->config.tc_handle,
                          ctx->config.tc_direct_action);
    if (ret != BPF_LOADER_OK) {
        _log_error(ctx, "failed to add TC filter on %s", iface);
        return ret;
    }

    subsys->attached = 1;

    _log_info(ctx, "TC BPF program attached to %s (%s) successfully",
              iface, ingress ? "ingress" : "egress");

    return BPF_LOADER_OK;
}

int bpf_loader_detach_tc(bpf_loader_ctx_t *ctx, const char *interface)
{
    bpf_subsys_state_t *subsys;
    const char *iface;
    int ifindex, ret;

    if (!ctx || !ctx->initialized) {
        return BPF_LOADER_ERR_NOT_INITIALIZED;
    }

    subsys = _find_subsystem(ctx, BPF_SUBSYS_TC);
    if (!subsys) {
        return BPF_LOADER_OK; /* 未加载，无需清理 */
    }

    iface = interface ? interface : ctx->config.tc_interface;
    if (!iface || iface[0] == '\0') {
        return BPF_LOADER_OK;
    }

    ifindex = if_nametoindex(iface);
    if (ifindex == 0) {
        _log_warn(ctx, "interface %s not found during detach", iface);
        return BPF_LOADER_OK;
    }

    _log_info(ctx, "detaching TC BPF program from %s", iface);

    /* 删除 ingress filter */
    ret = _tc_del_filter(ctx, ifindex, 1, ctx->config.tc_priority);
    if (ret != BPF_LOADER_OK) {
        _log_warn(ctx, "failed to delete ingress TC filter");
    }

    /* 删除 egress filter */
    ret = _tc_del_filter(ctx, ifindex, 0, ctx->config.tc_priority);
    if (ret != BPF_LOADER_OK) {
        _log_warn(ctx, "failed to delete egress TC filter");
    }

    subsys->attached = 0;
    subsys->tc_prog_fd = -1;

    return BPF_LOADER_OK;
}

/* ========================================================================
 * kprobe 挂载实现
 * ======================================================================== */

int bpf_loader_attach_kprobes(bpf_loader_ctx_t *ctx)
{
    bpf_subsys_state_t *subsys;
    struct bpf_program *prog;
    struct bpf_link *link;
    const char *prog_name, *sec_name;
    int ret;

    if (!ctx || !ctx->initialized) {
        return BPF_LOADER_ERR_NOT_INITIALIZED;
    }

    _log_info(ctx, "attaching kprobes for all loaded subsystems");

    /* 遍历所有已加载的子系统 */
    for (int i = 0; i < ctx->subsystem_count; i++) {
        subsys = &ctx->subsystems[i];
        if (!subsys->loaded || !subsys->obj)
            continue;

        /* 遍历子系统中的所有 BPF 程序 */
        int link_idx = 0;
        bpf_object__for_each_program(prog, subsys->obj) {
            sec_name = bpf_program__section_name(prog);
            prog_name = bpf_program__name(prog);

            if (!sec_name)
                continue;

            /*
             * 识别 kprobe 类型的程序。
             * libbpf 的 SEC 名称格式：
             *   - "kprobe/<func_name>" -> kprobe
             *   - "kretprobe/<func_name>" -> kretprobe
             *   - "kprobe/+<func_name>" -> kprobe with offset
             *   - "tracepoint/<category>/<name>" -> tracepoint
             */
            int is_kretprobe = 0;
            const char *func_name = NULL;

            if (strncmp(sec_name, "kretprobe/", 10) == 0) {
                is_kretprobe = 1;
                func_name = sec_name + 10;
            } else if (strncmp(sec_name, "kprobe/", 7) == 0) {
                func_name = sec_name + 7;
            }

            if (!func_name)
                continue; /* 不是 kprobe 程序，跳过 */

            _log_debug(ctx, "attaching %s for function %s (program: %s)",
                       is_kretprobe ? "kretprobe" : "kprobe",
                       func_name, prog_name);

            /* 挂载 kprobe */
            if (is_kretprobe) {
                link = bpf_program__attach_kprobe(prog, true, func_name);
            } else {
                link = bpf_program__attach_kprobe(prog, false, func_name);
            }

            if (libbpf_get_error(link)) {
                _log_warn(ctx,
                    "failed to attach %s/%s: %s (continuing)",
                    is_kretprobe ? "kretprobe" : "kprobe",
                    func_name, strerror(errno));
                continue;
            }

            /* 存储链接句柄 */
            if (link_idx < subsys->program_count && subsys->links) {
                subsys->links[link_idx] = link;
                subsys->link_count++;
                link_idx++;
            }

            _log_info(ctx, "attached %s for %s",
                      is_kretprobe ? "kretprobe" : "kprobe", func_name);
        }

        subsys->attached = 1;
    }

    return BPF_LOADER_OK;
}

int bpf_loader_attach_kprobe(bpf_loader_ctx_t *ctx, __u32 subsys,
                              const char *prog_name, const char *func_name,
                              int retprobe)
{
    bpf_subsys_state_t *subsys_state;
    struct bpf_program *prog;
    struct bpf_link *link;

    if (!ctx || !ctx->initialized || !prog_name || !func_name) {
        return BPF_LOADER_ERR_INVALID_ARG;
    }

    subsys_state = _find_subsystem(ctx, subsys);
    if (!subsys_state || !subsys_state->loaded) {
        _log_error(ctx, "subsystem %s not loaded", _get_subsys_name(subsys));
        return BPF_LOADER_ERR_NOT_INITIALIZED;
    }

    /* 查找指定的 BPF 程序 */
    prog = bpf_object__find_program_by_name(subsys_state->obj, prog_name);
    if (!prog) {
        _log_error(ctx, "program %s not found in subsystem %s",
                   prog_name, _get_subsys_name(subsys));
        return BPF_LOADER_ERR_ATTACH_KPROBE;
    }

    /* 挂载 kprobe/kretprobe */
    link = bpf_program__attach_kprobe(prog, retprobe, func_name);
    if (libbpf_get_error(link)) {
        _log_error(ctx, "failed to attach %s/%s: %s",
                   retprobe ? "kretprobe" : "kprobe", func_name,
                   strerror(errno));
        return BPF_LOADER_ERR_ATTACH_KPROBE;
    }

    /* 存储链接句柄 */
    if (subsys_state->link_count < subsys_state->program_count &&
        subsys_state->links) {
        subsys_state->links[subsys_state->link_count] = link;
        subsys_state->link_count++;
    }

    _log_info(ctx, "attached %s for %s (program: %s)",
              retprobe ? "kretprobe" : "kprobe", func_name, prog_name);

    return BPF_LOADER_OK;
}

int bpf_loader_detach_kprobes(bpf_loader_ctx_t *ctx)
{
    bpf_subsys_state_t *subsys;

    if (!ctx || !ctx->initialized) {
        return BPF_LOADER_ERR_NOT_INITIALIZED;
    }

    _log_info(ctx, "detaching all kprobes");

    for (int i = 0; i < ctx->subsystem_count; i++) {
        subsys = &ctx->subsystems[i];

        /* 销毁所有链接 */
        for (int j = 0; j < subsys->link_count; j++) {
            if (subsys->links && subsys->links[j]) {
                bpf_link__destroy(subsys->links[j]);
                subsys->links[j] = NULL;
            }
        }
        subsys->link_count = 0;
        subsys->attached = 0;
    }

    return BPF_LOADER_OK;
}

/* ========================================================================
 * 数据采集实现
 * ======================================================================== */

int bpf_loader_collect_network(bpf_loader_ctx_t *ctx,
                                map_iterate_callback callback,
                                void *user_ctx)
{
    bpf_subsys_state_t *subsys;
    struct bpf_map *map;
    int map_fd, ret;
    flow_key_t key = {};
    network_data_t value = {};

    if (!ctx || !ctx->initialized || !callback) {
        return BPF_LOADER_ERR_INVALID_ARG;
    }

    subsys = _find_subsystem(ctx, BPF_SUBSYS_TC);
    if (!subsys || !subsys->loaded) {
        _log_error(ctx, "TC subsystem not loaded");
        return BPF_LOADER_ERR_NOT_INITIALIZED;
    }

    /* 查找 network_map */
    map = bpf_object__find_map_by_name(subsys->obj, "network_map");
    if (!map) {
        _log_error(ctx, "network_map not found");
        return BPF_LOADER_ERR_MAP_OP;
    }

    map_fd = bpf_map__fd(map);
    if (map_fd < 0) {
        _log_error(ctx, "failed to get network_map FD");
        return BPF_LOADER_ERR_MAP_OP;
    }

    /* 遍历 Map 中的所有条目 */
    ret = BPF_LOADER_OK;

    while (bpf_map_get_next_key(map_fd, &key, &key) == 0) {
        ret = bpf_map_lookup_elem(map_fd, &key, &value);
        if (ret != 0) {
            if (errno == ENOENT)
                continue; /* 条目在遍历过程中被删除 */
            _log_warn(ctx, "failed to lookup network_map entry: %s",
                      strerror(errno));
            continue;
        }

        /* 构造回调数据 */
        network_map_entry_t entry;
        memcpy(&entry.key, &key, sizeof(key));
        memcpy(&entry.value, &value, sizeof(value));

        /* 调用用户回调 */
        ret = callback(&entry, user_ctx);
        if (ret != 0) {
            _log_debug(ctx, "network map iteration stopped by callback (ret=%d)", ret);
            break;
        }
    }

    return BPF_LOADER_OK;
}

int bpf_loader_collect_tcp_metrics(bpf_loader_ctx_t *ctx,
                                    map_iterate_callback callback,
                                    void *user_ctx)
{
    bpf_subsys_state_t *subsys;
    struct bpf_map *map;
    int map_fd, ret;
    tcp_conn_key_t key = {};
    tcp_stats_t value = {};

    if (!ctx || !ctx->initialized || !callback) {
        return BPF_LOADER_ERR_INVALID_ARG;
    }

    subsys = _find_subsystem(ctx, BPF_SUBSYS_TCP_METRICS);
    if (!subsys || !subsys->loaded) {
        _log_error(ctx, "TCP Metrics subsystem not loaded");
        return BPF_LOADER_ERR_NOT_INITIALIZED;
    }

    /* 查找 tcp_flow_stats_map */
    map = bpf_object__find_map_by_name(subsys->obj, "tcp_flow_stats_map");
    if (!map) {
        _log_error(ctx, "tcp_flow_stats_map not found");
        return BPF_LOADER_ERR_MAP_OP;
    }

    map_fd = bpf_map__fd(map);
    if (map_fd < 0) {
        _log_error(ctx, "failed to get tcp_flow_stats_map FD");
        return BPF_LOADER_ERR_MAP_OP;
    }

    /* 遍历 Map */
    while (bpf_map_get_next_key(map_fd, &key, &key) == 0) {
        ret = bpf_map_lookup_elem(map_fd, &key, &value);
        if (ret != 0) {
            if (errno == ENOENT)
                continue;
            _log_warn(ctx, "failed to lookup tcp_flow_stats_map entry: %s",
                      strerror(errno));
            continue;
        }

        tcp_flow_stats_entry_t entry;
        memcpy(&entry.key, &key, sizeof(key));
        memcpy(&entry.value, &value, sizeof(value));

        ret = callback(&entry, user_ctx);
        if (ret != 0)
            break;
    }

    return BPF_LOADER_OK;
}

int bpf_loader_lookup_global_metrics(bpf_loader_ctx_t *ctx,
                                      global_tcp_metrics_t *metrics)
{
    bpf_subsys_state_t *subsys;
    struct bpf_map *map;
    int map_fd, ret;
    __u32 key = 0;

    if (!ctx || !ctx->initialized || !metrics) {
        return BPF_LOADER_ERR_INVALID_ARG;
    }

    subsys = _find_subsystem(ctx, BPF_SUBSYS_TCP_METRICS);
    if (!subsys || !subsys->loaded) {
        _log_error(ctx, "TCP Metrics subsystem not loaded");
        return BPF_LOADER_ERR_NOT_INITIALIZED;
    }

    map = bpf_object__find_map_by_name(subsys->obj, "global_tcp_metrics_map");
    if (!map) {
        _log_error(ctx, "global_tcp_metrics_map not found");
        return BPF_LOADER_ERR_MAP_OP;
    }

    map_fd = bpf_map__fd(map);
    if (map_fd < 0) {
        _log_error(ctx, "failed to get global_tcp_metrics_map FD");
        return BPF_LOADER_ERR_MAP_OP;
    }

    ret = bpf_map_lookup_elem(map_fd, &key, metrics);
    if (ret != 0) {
        _log_error(ctx, "failed to lookup global_tcp_metrics: %s",
                   strerror(errno));
        return BPF_LOADER_ERR_MAP_OP;
    }

    return BPF_LOADER_OK;
}

int bpf_loader_clear_global_metrics(bpf_loader_ctx_t *ctx)
{
    bpf_subsys_state_t *subsys;
    struct bpf_map *map;
    int map_fd, ret;
    __u32 key = 0;
    global_tcp_metrics_t empty_metrics;

    if (!ctx || !ctx->initialized) {
        return BPF_LOADER_ERR_INVALID_ARG;
    }

    subsys = _find_subsystem(ctx, BPF_SUBSYS_TCP_METRICS);
    if (!subsys || !subsys->loaded) {
        return BPF_LOADER_ERR_NOT_INITIALIZED;
    }

    map = bpf_object__find_map_by_name(subsys->obj, "global_tcp_metrics_map");
    if (!map) {
        return BPF_LOADER_ERR_MAP_OP;
    }

    map_fd = bpf_map__fd(map);
    if (map_fd < 0) {
        return BPF_LOADER_ERR_MAP_OP;
    }

    memset(&empty_metrics, 0, sizeof(empty_metrics));
    ret = bpf_map_update_elem(map_fd, &key, &empty_metrics, BPF_ANY);
    if (ret != 0) {
        _log_error(ctx, "failed to clear global_tcp_metrics: %s",
                   strerror(errno));
        return BPF_LOADER_ERR_MAP_OP;
    }

    return BPF_LOADER_OK;
}

int bpf_loader_iterate_map(bpf_loader_ctx_t *ctx, __u32 subsys,
                            const char *map_name,
                            map_iterate_callback callback,
                            void *user_ctx)
{
    bpf_subsys_state_t *subsys_state;
    struct bpf_map *map;
    int map_fd;
    __u8 key_buf[256] = {};
    __u8 value_buf[4096] = {};
    __u32 key_size, value_size;
    int ret;

    if (!ctx || !ctx->initialized || !map_name || !callback) {
        return BPF_LOADER_ERR_INVALID_ARG;
    }

    subsys_state = _find_subsystem(ctx, subsys);
    if (!subsys_state || !subsys_state->loaded) {
        _log_error(ctx, "subsystem %s not loaded", _get_subsys_name(subsys));
        return BPF_LOADER_ERR_NOT_INITIALIZED;
    }

    map = bpf_object__find_map_by_name(subsys_state->obj, map_name);
    if (!map) {
        _log_error(ctx, "map %s not found in subsystem %s",
                   map_name, _get_subsys_name(subsys));
        return BPF_LOADER_ERR_MAP_OP;
    }

    map_fd = bpf_map__fd(map);
    if (map_fd < 0) {
        _log_error(ctx, "failed to get map %s FD", map_name);
        return BPF_LOADER_ERR_MAP_OP;
    }

    key_size = bpf_map__key_size(map);
    value_size = bpf_map__value_size(map);

    /* 安全检查：确保缓冲区足够大 */
    if (key_size > sizeof(key_buf) || value_size > sizeof(value_buf)) {
        _log_error(ctx, "map %s key/value size too large (key=%u, value=%u)",
                   map_name, key_size, value_size);
        return BPF_LOADER_ERR_MAP_OP;
    }

    /* 遍历 Map */
    while (bpf_map_get_next_key(map_fd, key_buf, key_buf) == 0) {
        ret = bpf_map_lookup_elem(map_fd, key_buf, value_buf);
        if (ret != 0) {
            if (errno == ENOENT)
                continue;
            _log_warn(ctx, "failed to lookup map %s entry: %s",
                      map_name, strerror(errno));
            continue;
        }

        /*
         * 构造统一的遍历条目结构。
         * 注意：由于不同 Map 的 key/value 类型不同，
         * 回调函数需要根据 map_name 自行解析。
         * 这里将 key 和 value 连续存储在一个缓冲区中传递。
         */
        __u8 entry_buf[sizeof(key_buf) + sizeof(value_buf)];
        memcpy(entry_buf, key_buf, key_size);
        memcpy(entry_buf + key_size, value_buf, value_size);

        ret = callback(entry_buf, user_ctx);
        if (ret != 0)
            break;
    }

    return BPF_LOADER_OK;
}

/* ========================================================================
 * CPU 剖析实现
 * ======================================================================== */

int bpf_loader_start_cpu_profiler(bpf_loader_ctx_t *ctx)
{
    bpf_subsys_state_t *subsys;
    struct bpf_map *map;
    struct bpf_program *prog;
    int map_fd, prog_fd, cpu_count;
    int ret;

    if (!ctx || !ctx->initialized) {
        return BPF_LOADER_ERR_NOT_INITIALIZED;
    }

    subsys = _find_subsystem(ctx, BPF_SUBSYS_CPU_PROFILER);
    if (!subsys || !subsys->loaded) {
        _log_error(ctx, "CPU Profiler subsystem not loaded");
        return BPF_LOADER_ERR_NOT_INITIALIZED;
    }

    /* 获取 CPU 数量 */
    cpu_count = sysconf(_SC_NPROCESSORS_ONLN);
    if (cpu_count <= 0) {
        cpu_count = 1;
    }

    _log_info(ctx, "starting CPU profiler on %d CPUs (freq=%u Hz)",
              cpu_count, ctx->config.cpu_sample_freq);

    /* 查找 perf_event_array Map */
    map = bpf_object__find_map_by_name(subsys->obj, "events");
    if (!map) {
        _log_error(ctx, "perf events map not found");
        return BPF_LOADER_ERR_MAP_OP;
    }

    map_fd = bpf_map__fd(map);

    /* 查找 on_cpu_sample 程序 */
    prog = bpf_object__find_program_by_name(subsys->obj, "on_cpu_sample");
    if (!prog) {
        _log_error(ctx, "on_cpu_sample program not found");
        return BPF_LOADER_ERR_ATTACH_KPROBE;
    }

    prog_fd = bpf_program__fd(prog);

    /* 为每个 CPU 创建 perf event 并绑定 BPF 程序 */
    subsys->perf_event_fds = calloc(cpu_count, sizeof(int));
    if (!subsys->perf_event_fds) {
        _log_error(ctx, "failed to allocate perf event FDs");
        return BPF_LOADER_ERR_NOMEM;
    }

    subsys->perf_cpu_count = cpu_count;
    subsys->perf_event_count = 0;

    for (int cpu = 0; cpu < cpu_count; cpu++) {
        struct perf_event_attr attr;
        memset(&attr, 0, sizeof(attr));

        attr.type = PERF_TYPE_SOFTWARE;
        attr.size = sizeof(attr);
        attr.config = PERF_COUNT_SW_CPU_CLOCK;
        attr.sample_freq = ctx->config.cpu_sample_freq;
        attr.freq = 1; /* 使用频率模式而非周期模式 */
        attr.sample_type = PERF_SAMPLE_RAW | PERF_SAMPLE_CALLCHAIN |
                           PERF_SAMPLE_CPU | PERF_SAMPLE_TIME |
                           PERF_SAMPLE_PERIOD;
        attr.disabled = 1; /* 先禁用，统一启用 */

        /* 使用 syscall 直接创建 perf event */
        int pfd = syscall(__NR_perf_event_open, &attr, -1, cpu, -1, 0);
        if (pfd < 0) {
            _log_warn(ctx, "failed to create perf event on CPU %d: %s",
                      cpu, strerror(errno));
            continue;
        }

        /* 将 BPF 程序绑定到 perf event */
        ret = ioctl(pfd, PERF_EVENT_IOC_SET_BPF, prog_fd);
        if (ret < 0) {
            _log_warn(ctx, "failed to attach BPF prog to perf event on CPU %d: %s",
                      cpu, strerror(errno));
            close(pfd);
            continue;
        }

        /* 启用 perf event */
        ret = ioctl(pfd, PERF_EVENT_IOC_ENABLE, 0);
        if (ret < 0) {
            _log_warn(ctx, "failed to enable perf event on CPU %d: %s",
                      cpu, strerror(errno));
            close(pfd);
            continue;
        }

        subsys->perf_event_fds[cpu] = pfd;
        subsys->perf_event_count++;
    }

    _log_info(ctx, "CPU profiler started: %d/%d CPUs active",
              subsys->perf_event_count, cpu_count);

    return BPF_LOADER_OK;
}

int bpf_loader_stop_cpu_profiler(bpf_loader_ctx_t *ctx)
{
    bpf_subsys_state_t *subsys;

    if (!ctx || !ctx->initialized) {
        return BPF_LOADER_ERR_NOT_INITIALIZED;
    }

    subsys = _find_subsystem(ctx, BPF_SUBSYS_CPU_PROFILER);
    if (!subsys) {
        return BPF_LOADER_OK;
    }

    _log_info(ctx, "stopping CPU profiler");

    /* 禁用并关闭所有 perf event */
    for (int i = 0; i < subsys->perf_cpu_count; i++) {
        if (subsys->perf_event_fds && subsys->perf_event_fds[i] >= 0) {
            ioctl(subsys->perf_event_fds[i], PERF_EVENT_IOC_DISABLE, 0);
            close(subsys->perf_event_fds[i]);
            subsys->perf_event_fds[i] = -1;
        }
    }

    free(subsys->perf_event_fds);
    subsys->perf_event_fds = NULL;
    subsys->perf_event_count = 0;
    subsys->perf_cpu_count = 0;

    return BPF_LOADER_OK;
}

int bpf_loader_read_prof_events(bpf_loader_ctx_t *ctx,
                                 map_iterate_callback callback,
                                 void *user_ctx)
{
    bpf_subsys_state_t *subsys;
    struct bpf_map *map;
    int map_fd, cpu_count, ret;
    prof_event_t event;

    if (!ctx || !ctx->initialized || !callback) {
        return BPF_LOADER_ERR_INVALID_ARG;
    }

    subsys = _find_subsystem(ctx, BPF_SUBSYS_CPU_PROFILER);
    if (!subsys || !subsys->loaded) {
        return BPF_LOADER_ERR_NOT_INITIALIZED;
    }

    map = bpf_object__find_map_by_name(subsys->obj, "events");
    if (!map) {
        return BPF_LOADER_ERR_MAP_OP;
    }

    map_fd = bpf_map__fd(map);
    cpu_count = sysconf(_SC_NPROCESSORS_ONLN);
    if (cpu_count <= 0)
        cpu_count = 1;

    /*
     * 从 perf_event_array 中读取事件。
     * 注意：perf_event_array 的读取通常通过 perf_event_mmap 实现，
     * 这里提供基于 bpf_map_lookup_elem 的简化版本。
     * 生产环境中应使用 perf ring buffer 进行高效读取。
     */
    for (int cpu = 0; cpu < cpu_count; cpu++) {
        __u32 key = cpu;
        ret = bpf_map_lookup_elem(map_fd, &key, &event);
        if (ret != 0)
            continue;

        ret = callback(&event, user_ctx);
        if (ret != 0)
            break;
    }

    return BPF_LOADER_OK;
}

/* ========================================================================
 * 资源清理实现
 * ======================================================================== */

int bpf_loader_cleanup_subsystem(bpf_loader_ctx_t *ctx, __u32 subsys)
{
    bpf_subsys_state_t *subsys_state;

    if (!ctx)
        return BPF_LOADER_ERR_INVALID_ARG;

    subsys_state = _find_subsystem(ctx, subsys);
    if (!subsys_state)
        return BPF_LOADER_OK; /* 子系统未加载，无需清理 */

    _log_info(ctx, "cleaning up subsystem %s", _get_subsys_name(subsys));

    /* 销毁所有链接 */
    for (int j = 0; j < subsys_state->link_count; j++) {
        if (subsys_state->links && subsys_state->links[j]) {
            bpf_link__destroy(subsys_state->links[j]);
            subsys_state->links[j] = NULL;
        }
    }
    subsys_state->link_count = 0;

    /* 释放链接数组 */
    free(subsys_state->links);
    subsys_state->links = NULL;

    /* 关闭 perf event FDs */
    for (int j = 0; j < subsys_state->perf_cpu_count; j++) {
        if (subsys_state->perf_event_fds && subsys_state->perf_event_fds[j] >= 0) {
            ioctl(subsys_state->perf_event_fds[j], PERF_EVENT_IOC_DISABLE, 0);
            close(subsys_state->perf_event_fds[j]);
            subsys_state->perf_event_fds[j] = -1;
        }
    }
    free(subsys_state->perf_event_fds);
    subsys_state->perf_event_fds = NULL;
    subsys_state->perf_event_count = 0;
    subsys_state->perf_cpu_count = 0;

    /* 关闭 TC 相关 FD */
    if (subsys_state->tc_prog_fd >= 0) {
        close(subsys_state->tc_prog_fd);
        subsys_state->tc_prog_fd = -1;
    }
    if (subsys_state->tc_cls_fd >= 0) {
        close(subsys_state->tc_cls_fd);
        subsys_state->tc_cls_fd = -1;
    }
    if (subsys_state->tc_qdisc_fd >= 0) {
        close(subsys_state->tc_qdisc_fd);
        subsys_state->tc_qdisc_fd = -1;
    }

    /* 关闭 BPF 对象（会同时关闭所有 Map 和程序） */
    if (subsys_state->obj) {
        bpf_object__close(subsys_state->obj);
        subsys_state->obj = NULL;
    }

    subsys_state->loaded = 0;
    subsys_state->attached = 0;
    subsys_state->program_count = 0;
    subsys_state->map_count = 0;

    return BPF_LOADER_OK;
}

int bpf_loader_cleanup(bpf_loader_ctx_t *ctx)
{
    if (!ctx)
        return BPF_LOADER_ERR_INVALID_ARG;

    if (!ctx->initialized) {
        return BPF_LOADER_OK;
    }

    _log_info(ctx, "cleaning up BPF loader resources");

    /* 1. 停止 CPU 剖析 */
    bpf_loader_stop_cpu_profiler(ctx);

    /* 2. 卸载所有 kprobe 链接 */
    bpf_loader_detach_kprobes(ctx);

    /* 3. 卸载 TC 程序 */
    bpf_loader_detach_tc(ctx, NULL);

    /* 4. 逐个清理子系统 */
    for (int i = 0; i < ctx->subsystem_count; i++) {
        if (ctx->subsystems[i].loaded) {
            bpf_loader_cleanup_subsystem(ctx, ctx->subsystems[i].subsys_id);
        }
    }

    /* 5. 重置上下文 */
    ctx->subsystem_count = 0;
    ctx->initialized = 0;

    _log_info(ctx, "BPF loader cleanup completed");

    return BPF_LOADER_OK;
}

/* ========================================================================
 * 辅助查询实现
 * ======================================================================== */

int bpf_loader_is_subsys_loaded(const bpf_loader_ctx_t *ctx, __u32 subsys)
{
    bpf_subsys_state_t *subsys_state;

    if (!ctx)
        return 0;

    subsys_state = _find_subsystem((bpf_loader_ctx_t *)ctx, subsys);
    return (subsys_state && subsys_state->loaded) ? 1 : 0;
}

int bpf_loader_is_subsys_attached(const bpf_loader_ctx_t *ctx, __u32 subsys)
{
    bpf_subsys_state_t *subsys_state;

    if (!ctx)
        return 0;

    subsys_state = _find_subsystem((bpf_loader_ctx_t *)ctx, subsys);
    return (subsys_state && subsys_state->attached) ? 1 : 0;
}

int bpf_loader_get_map_fd(bpf_loader_ctx_t *ctx, __u32 subsys,
                           const char *map_name)
{
    bpf_subsys_state_t *subsys_state;
    struct bpf_map *map;

    if (!ctx || !map_name)
        return BPF_LOADER_ERR_INVALID_ARG;

    subsys_state = _find_subsystem(ctx, subsys);
    if (!subsys_state || !subsys_state->loaded) {
        _log_error(ctx, "subsystem %s not loaded", _get_subsys_name(subsys));
        return BPF_LOADER_ERR_NOT_INITIALIZED;
    }

    map = bpf_object__find_map_by_name(subsys_state->obj, map_name);
    if (!map) {
        _log_error(ctx, "map %s not found in subsystem %s",
                   map_name, _get_subsys_name(subsys));
        return BPF_LOADER_ERR_MAP_OP;
    }

    return bpf_map__fd(map);
}

int bpf_loader_get_prog_fd(bpf_loader_ctx_t *ctx, __u32 subsys,
                            const char *prog_name)
{
    bpf_subsys_state_t *subsys_state;
    struct bpf_program *prog;

    if (!ctx || !prog_name)
        return BPF_LOADER_ERR_INVALID_ARG;

    subsys_state = _find_subsystem(ctx, subsys);
    if (!subsys_state || !subsys_state->loaded) {
        _log_error(ctx, "subsystem %s not loaded", _get_subsys_name(subsys));
        return BPF_LOADER_ERR_NOT_INITIALIZED;
    }

    prog = bpf_object__find_program_by_name(subsys_state->obj, prog_name);
    if (!prog) {
        _log_error(ctx, "program %s not found in subsystem %s",
                   prog_name, _get_subsys_name(subsys));
        return BPF_LOADER_ERR_MAP_OP;
    }

    return bpf_program__fd(prog);
}

int bpf_loader_get_subsys_count(const bpf_loader_ctx_t *ctx)
{
    if (!ctx)
        return 0;
    return ctx->subsystem_count;
}
