/*
 * Cloud Flow Agent - eBPF Programs (libbpf-based)
 * 
 * 使用 libbpf 框架重构，提升内核兼容性
 * 支持 CO-RE (Compile Once, Run Everywhere)
 */

#include "vmlinux.h"
#include <bpf/bpf_helpers.h>
#include <bpf/bpf_tracing.h>
#include <bpf/bpf_core_read.h>

#define TASK_COMM_LEN 16
#define MAX_CONN_ENTRIES 100000
#define MAX_PID_ENTRIES 10000

/* 事件类型定义 */
enum event_type {
    EVENT_TCP_CONNECT = 1,
    EVENT_TCP_ACCEPT,
    EVENT_TCP_CLOSE,
    EVENT_TCP_SEND,
    EVENT_TCP_RECV,
    EVENT_PROCESS_EXEC,
    EVENT_PROCESS_EXIT,
    EVENT_FILE_OPEN,
    EVENT_FILE_CLOSE,
};

/* 通用事件头 */
struct event_header {
    __u64 timestamp_ns;
    __u32 pid;
    __u32 tid;
    __u32 event_type;
    __u32 cpu;
    char comm[TASK_COMM_LEN];
};

/* TCP 连接事件 */
struct tcp_conn_event {
    struct event_header header;
    __u32 saddr;
    __u32 daddr;
    __u16 sport;
    __u16 dport;
    __u32 netns;
    __u8  is_ipv6;
    __u8  direction;  /* 0=outbound, 1=inbound */
    __u16 pad;
};

/* TCP 数据传输事件 */
struct tcp_data_event {
    struct event_header header;
    __u32 saddr;
    __u32 daddr;
    __u16 sport;
    __u16 dport;
    __u64 bytes;
    __u32 seq;
    __u32 ack;
    __u8  flags;
    __u8  is_retransmit;
    __u16 pad;
};

/* 进程事件 */
struct process_event {
    struct event_header header;
    __u32 ppid;
    __u32 uid;
    __u32 gid;
    __u32 exit_code;
    char  exe[128];
};

/* 文件事件 */
struct file_event {
    struct event_header header;
    __u64 inode;
    __u32 dev;
    __u32 pad;
    char  filename[256];
};

/* 全局配置 map - 运行时动态更新 */
struct {
    __uint(type, BPF_MAP_TYPE_ARRAY);
    __uint(max_entries, 1);
    __type(key, __u32);
    __type(value, struct global_config);
} config_map SEC(".maps");

struct global_config {
    __u32 enabled;           /* 全局开关 */
    __u32 sample_rate;       /* 采样率 1-10000 */
    __u32 max_events_per_sec; /* 每秒最大事件数 */
    __u32 flags;             /* 功能开关位图 */
    __u64 filter_pid;        /* 过滤的 PID */
    __u64 filter_netns;      /* 过滤的网络命名空间 */
};

/* 事件 ring buffer - 向用户态传输 */
struct {
    __uint(type, BPF_MAP_TYPE_RINGBUF);
    __uint(max_entries, 256 * 1024); /* 256KB ring buffer */
} events_rb SEC(".maps");

/* TCP 连接跟踪 map */
struct {
    __uint(type, BPF_MAP_TYPE_HASH);
    __uint(max_entries, MAX_CONN_ENTRIES);
    __type(key, struct conn_key);
    __type(value, struct conn_info);
} conn_map SEC(".maps");

struct conn_key {
    __u32 saddr;
    __u32 daddr;
    __u16 sport;
    __u16 dport;
    __u32 netns;
};

struct conn_info {
    __u64 start_ns;
    __u64 bytes_sent;
    __u64 bytes_recv;
    __u32 pid;
    __u32 tid;
    char comm[TASK_COMM_LEN];
};

/* PID 过滤 map - 动态更新 */
struct {
    __uint(type, BPF_MAP_TYPE_HASH);
    __uint(max_entries, MAX_PID_ENTRIES);
    __type(key, __u32);
    __type(value, __u8);  /* 1=include, 2=exclude */
} pid_filter_map SEC(".maps");

/* 采样计数器 */
struct {
    __uint(type, BPF_MAP_TYPE_PERCPU_ARRAY);
    __uint(max_entries, 1);
    __type(key, __u32);
    __type(value, __u32);
} sample_counter SEC(".maps");

/* 事件计数器 - 用于限流 */
struct {
    __uint(type, BPF_MAP_TYPE_ARRAY);
    __uint(max_entries, 1);
    __type(key, __u32);
    __type(value, __u64);
} event_counter SEC(".maps");

/* 辅助函数声明 */
static __always_inline struct global_config *get_config(void);
static __always_inline bool should_sample(void);
static __always_inline bool pid_allowed(__u32 pid);
static __always_inline void fill_header(struct event_header *hdr, __u32 type);
static __always_inline __u64 get_timestamp_ns(void);

/* ==================== TCP 探针 ==================== */

/*
 * tcp_connect - 跟踪 TCP 连接建立
 * 使用 fentry/fexit (BPF_TRACING) 替代 kprobe/kretprobe，性能更好
 */
SEC("fentry/tcp_connect")
int BPF_PROG(trace_tcp_connect, struct sock *sk)
{
    struct global_config *cfg = get_config();
    if (!cfg || !cfg->enabled)
        return 0;

    if (!should_sample())
        return 0;

    __u32 pid = bpf_get_current_pid_tgid() >> 32;
    if (!pid_allowed(pid))
        return 0;

    struct tcp_conn_event *event = bpf_ringbuf_reserve(&events_rb, sizeof(*event), 0);
    if (!event)
        return 0;

    fill_header(&event->header, EVENT_TCP_CONNECT);

    /* CO-RE 读取 sock 字段，兼容不同内核版本 */
    struct sock_common *skc = &sk->__sk_common;
    BPF_CORE_READ_INTO(&event->saddr, skc, skc_rcv_saddr);
    BPF_CORE_READ_INTO(&event->daddr, skc, skc_daddr);
    BPF_CORE_READ_INTO(&event->sport, skc, skc_num);
    BPF_CORE_READ_INTO(&event->dport, skc, skc_dport);
    
    /* 网络命名空间 */
    struct net *net = BPF_CORE_READ(sk, __sk_common.skc_net.net);
    if (net) {
        BPF_CORE_READ_INTO(&event->netns, net, ns.inum);
    }

    event->is_ipv6 = 0;
    event->direction = 0;

    /* 更新连接跟踪表 */
    struct conn_key key = {};
    key.saddr = event->saddr;
    key.daddr = event->daddr;
    key.sport = event->sport;
    key.dport = bpf_ntohs(event->dport);
    key.netns = event->netns;

    struct conn_info info = {};
    info.start_ns = get_timestamp_ns();
    info.pid = pid;
    info.tid = bpf_get_current_pid_tgid();
    bpf_get_current_comm(&info.comm, sizeof(info.comm));

    bpf_map_update_elem(&conn_map, &key, &info, BPF_ANY);

    bpf_ringbuf_submit(event, 0);
    return 0;
}

/*
 * tcp_close - 跟踪 TCP 连接关闭
 */
SEC("fentry/tcp_close")
int BPF_PROG(trace_tcp_close, struct sock *sk, long timeout)
{
    struct global_config *cfg = get_config();
    if (!cfg || !cfg->enabled)
        return 0;

    __u32 pid = bpf_get_current_pid_tgid() >> 32;
    if (!pid_allowed(pid))
        return 0;

    struct tcp_conn_event *event = bpf_ringbuf_reserve(&events_rb, sizeof(*event), 0);
    if (!event)
        return 0;

    fill_header(&event->header, EVENT_TCP_CLOSE);

    struct sock_common *skc = &sk->__sk_common;
    BPF_CORE_READ_INTO(&event->saddr, skc, skc_rcv_saddr);
    BPF_CORE_READ_INTO(&event->daddr, skc, skc_daddr);
    BPF_CORE_READ_INTO(&event->sport, skc, skc_num);
    BPF_CORE_READ_INTO(&event->dport, skc, skc_dport);

    /* 查找并清理连接跟踪 */
    struct conn_key key = {};
    key.saddr = event->saddr;
    key.daddr = event->daddr;
    key.sport = event->sport;
    key.dport = bpf_ntohs(event->dport);
    BPF_CORE_READ_INTO(&key.netns, sk, __sk_common.skc_net.net, ns.inum);

    struct conn_info *info = bpf_map_lookup_elem(&conn_map, &key);
    if (info) {
        /* 可以在这里计算连接持续时间等指标 */
        bpf_map_delete_elem(&conn_map, &key);
    }

    bpf_ringbuf_submit(event, 0);
    return 0;
}

/*
 * tcp_sendmsg - 跟踪 TCP 发送数据
 */
SEC("fentry/tcp_sendmsg")
int BPF_PROG(trace_tcp_sendmsg, struct sock *sk, struct msghdr *msg, size_t size)
{
    struct global_config *cfg = get_config();
    if (!cfg || !cfg->enabled)
        return 0;

    if (!should_sample())
        return 0;

    __u32 pid = bpf_get_current_pid_tgid() >> 32;
    if (!pid_allowed(pid))
        return 0;

    struct tcp_data_event *event = bpf_ringbuf_reserve(&events_rb, sizeof(*event), 0);
    if (!event)
        return 0;

    fill_header(&event->header, EVENT_TCP_SEND);

    struct sock_common *skc = &sk->__sk_common;
    BPF_CORE_READ_INTO(&event->saddr, skc, skc_rcv_saddr);
    BPF_CORE_READ_INTO(&event->daddr, skc, skc_daddr);
    BPF_CORE_READ_INTO(&event->sport, skc, skc_num);
    BPF_CORE_READ_INTO(&event->dport, skc, skc_dport);

    event->bytes = size;
    BPF_CORE_READ_INTO(&event->seq, sk, tcp_sequenced.packets_out);
    event->is_retransmit = 0;

    bpf_ringbuf_submit(event, 0);
    return 0;
}

/*
 * tcp_recvmsg - 跟踪 TCP 接收数据
 */
SEC("fentry/tcp_recvmsg")
int BPF_PROG(trace_tcp_recvmsg, struct sock *sk, struct msghdr *msg, 
              size_t len, int nonblock, int flags, int *addr_len)
{
    struct global_config *cfg = get_config();
    if (!cfg || !cfg->enabled)
        return 0;

    if (!should_sample())
        return 0;

    __u32 pid = bpf_get_current_pid_tgid() >> 32;
    if (!pid_allowed(pid))
        return 0;

    /* 在 kretprobe 中获取实际接收字节数，这里只记录请求 */
    return 0;
}

/* ==================== 进程探针 ==================== */

/*
 * sched_process_exec - 跟踪进程执行
 * 使用 tracepoint 替代 kprobe，更稳定
 */
SEC("tp/sched/sched_process_exec")
int trace_sched_process_exec(struct trace_event_raw_sched_process_exec *ctx)
{
    struct global_config *cfg = get_config();
    if (!cfg || !cfg->enabled)
        return 0;

    if (!(cfg->flags & 0x01))  /* 检查进程事件开关 */
        return 0;

    __u32 pid = bpf_get_current_pid_tgid() >> 32;
    if (!pid_allowed(pid))
        return 0;

    struct process_event *event = bpf_ringbuf_reserve(&events_rb, sizeof(*event), 0);
    if (!event)
        return 0;

    fill_header(&event->header, EVENT_PROCESS_EXEC);

    event->ppid = ctx->old_pid;
    event->uid = bpf_get_current_uid_gid() & 0xFFFFFFFF;
    event->gid = bpf_get_current_uid_gid() >> 32;
    event->exit_code = 0;

    /* 读取可执行文件路径 */
    bpf_probe_read_kernel_str(&event->exe, sizeof(event->exe), 
                              ctx->filename);

    bpf_ringbuf_submit(event, 0);
    return 0;
}

/*
 * sched_process_exit - 跟踪进程退出
 */
SEC("tp/sched/sched_process_exit")
int trace_sched_process_exit(void *ctx)
{
    struct global_config *cfg = get_config();
    if (!cfg || !cfg->enabled)
        return 0;

    if (!(cfg->flags & 0x01))
        return 0;

    __u32 pid = bpf_get_current_pid_tgid() >> 32;
    if (!pid_allowed(pid))
        return 0;

    struct process_event *event = bpf_ringbuf_reserve(&events_rb, sizeof(*event), 0);
    if (!event)
        return 0;

    fill_header(&event->header, EVENT_PROCESS_EXIT);

    struct task_struct *task = (struct task_struct *)bpf_get_current_task();
    BPF_CORE_READ_INTO(&event->ppid, task, real_parent, tgid);
    event->uid = bpf_get_current_uid_gid() & 0xFFFFFFFF;
    event->gid = bpf_get_current_uid_gid() >> 32;
    BPF_CORE_READ_INTO(&event->exit_code, task, exit_code);

    bpf_get_current_comm(&event->exe, sizeof(event->exe));

    bpf_ringbuf_submit(event, 0);
    return 0;
}

/* ==================== 文件探针 ==================== */

/*
 * do_sys_open - 跟踪文件打开
 */
SEC("fentry/do_sys_openat2")
int BPF_PROG(trace_do_sys_open, int dfd, struct filename *filename, 
              struct open_how *how)
{
    struct global_config *cfg = get_config();
    if (!cfg || !cfg->enabled)
        return 0;

    if (!(cfg->flags & 0x02))  /* 检查文件事件开关 */
        return 0;

    __u32 pid = bpf_get_current_pid_tgid() >> 32;
    if (!pid_allowed(pid))
        return 0;

    struct file_event *event = bpf_ringbuf_reserve(&events_rb, sizeof(*event), 0);
    if (!event)
        return 0;

    fill_header(&event->header, EVENT_FILE_OPEN);

    /* 读取文件名 */
    bpf_probe_read_kernel_str(&event->filename, sizeof(event->filename),
                              filename->name);

    bpf_ringbuf_submit(event, 0);
    return 0;
}

/* ==================== 辅助函数实现 ==================== */

static __always_inline struct global_config *get_config(void)
{
    __u32 key = 0;
    return bpf_map_lookup_elem(&config_map, &key);
}

static __always_inline bool should_sample(void)
{
    struct global_config *cfg = get_config();
    if (!cfg)
        return false;

    __u32 rate = cfg->sample_rate;
    if (rate >= 10000)
        return true;
    if (rate == 0)
        return false;

    /* 简单采样：基于计数器 */
    __u32 key = 0;
    __u32 *counter = bpf_map_lookup_elem(&sample_counter, &key);
    if (!counter) {
        __u32 init = 1;
        bpf_map_update_elem(&sample_counter, &key, &init, BPF_ANY);
        return (1 * 10000 / rate) <= 1;
    }

    __u32 new_val = __sync_fetch_and_add(counter, 1);
    return (new_val * 10000 / rate) <= 1;
}

static __always_inline bool pid_allowed(__u32 pid)
{
    /* 检查 PID 过滤 map，如果没有配置则允许所有 */
    __u8 *val = bpf_map_lookup_elem(&pid_filter_map, &pid);
    if (val) {
        return *val == 1;  /* 1=include, 2=exclude */
    }
    /* 默认允许 */
    return true;
}

static __always_inline void fill_header(struct event_header *hdr, __u32 type)
{
    hdr->timestamp_ns = get_timestamp_ns();
    hdr->pid = bpf_get_current_pid_tgid() >> 32;
    hdr->tid = bpf_get_current_pid_tgid() & 0xFFFFFFFF;
    hdr->event_type = type;
    hdr->cpu = bpf_get_smp_processor_id();
    bpf_get_current_comm(&hdr->comm, sizeof(hdr->comm));
}

static __always_inline __u64 get_timestamp_ns(void)
{
    return bpf_ktime_get_ns();
}

/* 许可证声明 - GPL 允许使用更多内核功能 */
char LICENSE[] SEC("license") = "GPL";
