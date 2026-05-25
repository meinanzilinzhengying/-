//go:build linux

// Package ebpf 提供 eBPF 网络流量采集功能
package ebpf

// #include <linux/types.h>
// #include <bpf/bpf_helpers.h>
// #include <bpf/bpf_tracing.h>
// #include <bpf/bpf_endian.h>
/*
// 定义事件类型
#define EVENT_TCP_CONNECT   1
#define EVENT_TCP_ACCEPT    2
#define EVENT_TCP_CLOSE     3
#define EVENT_UDP_SEND      4
#define EVENT_UDP_RECV      5

// 网络事件结构
struct net_event_t {
    __u64 timestamp;
    __u32 event_type;
    __u32 pid;
    __u32 tid;
    char comm[16];
    __u8 saddr[4];
    __u8 daddr[4];
    __u16 sport;
    __u16 dport;
    __u8 protocol;
    __u8 tcp_state;
    __u64 bytes;
    __u64 packets;
    __u64 duration_ns;
};

// 进程信息结构
struct process_info_t {
    __u32 pid;
    char comm[16];
};

// Perf Event 输出
struct {
    __uint(type, BPF_MAP_TYPE_PERF_EVENT_ARRAY);
    __uint(key_size, sizeof(__u32));
    __uint(value_size, sizeof(__u32));
} events SEC(".maps");

// 进程信息哈希表
struct {
    __uint(type, BPF_MAP_TYPE_HASH);
    __uint(max_entries, 10240);
    __type(key, __u32);
    __type(value, struct process_info_t);
} process_map SEC(".maps");

// TCP 连接跟踪
struct {
    __uint(type, BPF_MAP_TYPE_HASH);
    __uint(max_entries, 65536);
    __type(key, struct sock *);
    __type(value, __u64);
} tcp_connect_map SEC(".maps");

// 获取进程信息
static __always_inline struct process_info_t* get_process_info(__u32 pid) {
    struct process_info_t *info = bpf_map_lookup_elem(&process_map, &pid);
    return info;
}

// 更新进程信息
static __always_inline void update_process_info(__u32 pid, const char *comm) {
    struct process_info_t info = {};
    info.pid = pid;
    __builtin_memcpy(&info.comm, comm, sizeof(info.comm));
    bpf_map_update_elem(&process_map, &pid, &info, BPF_ANY);
}

// 发送事件
static __always_inline void send_event(struct net_event_t *event) {
    bpf_perf_event_output((void *)0, &events, BPF_F_CURRENT_CPU,
                          event, sizeof(*event));
}

// TCP connect 钩子
SEC("kprobe/tcp_connect")
int BPF_KPROBE(kprobe__tcp_connect, struct sock *sk) {
    __u64 pid_tgid = bpf_get_current_pid_tgid();
    __u32 pid = pid_tgid >> 32;
    __u32 tid = (__u32)pid_tgid;

    // 记录连接开始时间
    __u64 ts = bpf_ktime_get_ns();
    bpf_map_update_elem(&tcp_connect_map, &sk, &ts, BPF_ANY);

    // 获取进程信息
    struct process_info_t proc_info = {};
    bpf_get_current_comm(&proc_info.comm, sizeof(proc_info.comm));
    proc_info.pid = pid;
    bpf_map_update_elem(&process_map, &pid, &proc_info, BPF_ANY);

    return 0;
}

// TCP accept 钩子
SEC("kretprobe/inet_csk_accept")
int BPF_KRETPROBE(kretprobe__inet_csk_accept, struct sock *sk) {
    if (!sk)
        return 0;

    __u64 pid_tgid = bpf_get_current_pid_tgid();
    __u32 pid = pid_tgid >> 32;

    struct net_event_t event = {};
    event.timestamp = bpf_ktime_get_ns();
    event.event_type = EVENT_TCP_ACCEPT;
    event.pid = pid;
    event.tid = (__u32)pid_tgid;
    bpf_get_current_comm(&event.comm, sizeof(event.comm));

    // 提取 socket 信息
    event.protocol = 6; // TCP
    event.tcp_state = 1; // ESTABLISHED

    send_event(&event);
    return 0;
}

// TCP close 钩子
SEC("kprobe/tcp_close")
int BPF_KPROBE(kprobe__tcp_close, struct sock *sk) {
    __u64 pid_tgid = bpf_get_current_pid_tgid();
    __u32 pid = pid_tgid >> 32;

    // 获取连接开始时间
    __u64 *start_ts = bpf_map_lookup_elem(&tcp_connect_map, &sk);
    __u64 duration = 0;
    if (start_ts) {
        duration = bpf_ktime_get_ns() - *start_ts;
        bpf_map_delete_elem(&tcp_connect_map, &sk);
    }

    struct net_event_t event = {};
    event.timestamp = bpf_ktime_get_ns();
    event.event_type = EVENT_TCP_CLOSE;
    event.pid = pid;
    event.tid = (__u32)pid_tgid;
    event.duration_ns = duration;
    bpf_get_current_comm(&event.comm, sizeof(event.comm));

    send_event(&event);
    return 0;
}

// TCP sendmsg 钩子 (统计发送字节数)
SEC("kprobe/tcp_sendmsg")
int BPF_KPROBE(kprobe__tcp_sendmsg, struct sock *sk, struct msghdr *msg, size_t size) {
    __u64 pid_tgid = bpf_get_current_pid_tgid();
    __u32 pid = pid_tgid >> 32;

    struct net_event_t event = {};
    event.timestamp = bpf_ktime_get_ns();
    event.event_type = EVENT_TCP_CONNECT; // 使用 connect 类型表示发送
    event.pid = pid;
    event.tid = (__u32)pid_tgid;
    event.bytes = size;
    event.packets = 1;
    bpf_get_current_comm(&event.comm, sizeof(event.comm));

    send_event(&event);
    return 0;
}

// TCP recvmsg 钩子 (统计接收字节数)
SEC("kprobe/tcp_recvmsg")
int BPF_KPROBE(kprobe__tcp_recvmsg, struct sock *sk, struct msghdr *msg, size_t len, int flags) {
    __u64 pid_tgid = bpf_get_current_pid_tgid();
    __u32 pid = pid_tgid >> 32;

    struct net_event_t event = {};
    event.timestamp = bpf_ktime_get_ns();
    event.event_type = EVENT_TCP_ACCEPT; // 使用 accept 类型表示接收
    event.pid = pid;
    event.tid = (__u32)pid_tgid;
    event.bytes = len;
    event.packets = 1;
    bpf_get_current_comm(&event.comm, sizeof(event.comm));

    send_event(&event);
    return 0;
}

char LICENSE[] SEC("license") = "GPL";
*/
import "C"
