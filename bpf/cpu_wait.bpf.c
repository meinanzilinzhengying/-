/*
 * CPU Wait Profiler - eBPF Program
 * 支持C/C++/Golang/Java程序的CPU等待瓶颈分析
 * 
 * 特性:
 * - 无侵入式监控
 * - 动态采样率控制
 * - 多语言运行时支持
 */

#include "vmlinux.h"
#include <bpf/bpf_helpers.h>
#include <bpf/bpf_tracing.h>
#include <bpf/bpf_core_read.h>

#define TASK_COMM_LEN 16
#define MAX_STACK_DEPTH 128
#define MAX_PIDS 1024

/* 语言类型 */
enum language_type {
    LANG_UNKNOWN = 0,
    LANG_C,
    LANG_CPP,
    LANG_GO,
    LANG_JAVA,
};

/* 等待原因 */
enum wait_reason {
    WAIT_UNKNOWN = 0,
    WAIT_FUTEX,
    WAIT_IO,
    WAIT_NETWORK,
    WAIT_LOCK,
    WAIT_CONDVAR,
    WAIT_SLEEP,
    WAIT_PARK,
    WAIT_MONITOR,
    WAIT_PARK_NANOS,
};

/* CPU等待事件 */
struct cpu_wait_event {
    u64 timestamp;
    u32 pid;
    u32 tid;
    u32 cpu;
    u32 language;
    u32 wait_reason;
    u64 wait_time_ns;
    s64 stack_id;
    char comm[TASK_COMM_LEN];
};

/* 任务状态跟踪 */
struct task_state {
    u64 start_time;
    u32 wait_reason;
    u32 language;
};

/* 采样配置 */
struct sample_config {
    u32 sample_rate;        /* 采样率 (Hz) */
    u32 min_block_time_ns;  /* 最小阻塞时间 (ns) */
    u32 enabled;            /* 是否启用 */
};

/* 目标PID过滤器 */
struct pid_filter {
    u32 pid;
    u32 enabled;
};

/* BPF Maps */

/* 事件输出map */
struct {
    __uint(type, BPF_MAP_TYPE_PERF_EVENT_ARRAY);
    __uint(key_size, sizeof(u32));
    __uint(value_size, sizeof(u32));
} cpu_wait_events SEC(".maps");

/* 采样配置map */
struct {
    __uint(type, BPF_MAP_TYPE_ARRAY);
    __uint(max_entries, 1);
    __type(key, u32);
    __type(value, struct sample_config);
} sample_config_map SEC(".maps");

/* 采样率计数器 */
struct {
    __uint(type, BPF_MAP_TYPE_ARRAY);
    __uint(max_entries, 1);
    __type(key, u32);
    __type(value, u64);
} sample_counter SEC(".maps");

/* 任务状态map */
struct {
    __uint(type, BPF_MAP_TYPE_HASH);
    __uint(max_entries, 10000);
    __type(key, u32);  /* tid */
    __type(value, struct task_state);
} task_states SEC(".maps");

/* 栈跟踪map */
struct {
    __uint(type, BPF_MAP_TYPE_STACK_TRACE);
    __uint(max_entries, 10000);
    __uint(key_size, sizeof(u32));
    __uint(value_size, MAX_STACK_DEPTH * sizeof(u64));
} stack_traces SEC(".maps");

/* 目标PID过滤器map */
struct {
    __uint(type, BPF_MAP_TYPE_HASH);
    __uint(max_entries, MAX_PIDS);
    __type(key, u32);
    __type(value, u8);
} target_pids SEC(".maps");

/* 语言检测缓存 */
struct {
    __uint(type, BPF_MAP_TYPE_LRU_HASH);
    __uint(max_entries, 10000);
    __type(key, u32);  /* pid */
    __type(value, u32); /* language */
} pid_language_cache SEC(".maps");

/* 辅助函数 */

static __always_inline u64 get_timestamp(void)
{
    return bpf_ktime_get_ns();
}

static __always_inline int should_sample(void)
{
    u32 key = 0;
    struct sample_config *config;
    
    config = bpf_map_lookup_elem(&sample_config_map, &key);
    if (!config || !config->enabled)
        return 0;
    
    /* 简单的采样控制：基于计数器 */
    u64 *counter = bpf_map_lookup_elem(&sample_counter, &key);
    if (!counter)
        return 0;
    
    u64 new_val = __sync_fetch_and_add(counter, 1);
    
    /* 100Hz = 每10ms采样一次 */
    u64 sample_interval = 1000000000ULL / config->sample_rate;
    return (new_val % sample_interval) == 0;
}

static __always_inline int is_target_pid(u32 pid)
{
    /* 空过滤器表示监控所有进程 */
    u32 count = bpf_map_get_max_entries(&target_pids);
    if (count == 0)
        return 1;
    
    u8 *enabled = bpf_map_lookup_elem(&target_pids, &pid);
    return enabled && *enabled;
}

static __always_inline u32 detect_language(u32 pid)
{
    /* 检查缓存 */
    u32 *cached = bpf_map_lookup_elem(&pid_language_cache, &pid);
    if (cached)
        return *cached;
    
    /* 默认C/C++ */
    u32 lang = LANG_C;
    
    /* 通过comm检测 */
    char comm[TASK_COMM_LEN];
    bpf_get_current_comm(&comm, sizeof(comm));
    
    /* 检测Java */
    #pragma unroll
    for (int i = 0; i < TASK_COMM_LEN - 4; i++) {
        if (comm[i] == 'j' && comm[i+1] == 'a' && 
            comm[i+2] == 'v' && comm[i+3] == 'a') {
            lang = LANG_JAVA;
            goto cache_result;
        }
    }
    
    /* 检测Go - 通过特定函数存在性判断 */
    /* 实际检测在Go探针中完成 */
    
cache_result:
    bpf_map_update_elem(&pid_language_cache, &pid, &lang, BPF_ANY);
    return lang;
}

static __always_inline void record_wait_event(
    struct pt_regs *ctx,
    u32 wait_reason,
    u64 wait_time_ns)
{
    u32 pid = bpf_get_current_pid_tgid() >> 32;
    u32 tid = bpf_get_current_pid_tgid();
    
    if (!is_target_pid(pid))
        return;
    
    struct sample_config *config;
    u32 key = 0;
    config = bpf_map_lookup_elem(&sample_config_map, &key);
    if (!config)
        return;
    
    /* 检查最小阻塞时间 */
    if (wait_time_ns < config->min_block_time_ns)
        return;
    
    struct cpu_wait_event event = {};
    event.timestamp = get_timestamp();
    event.pid = pid;
    event.tid = tid;
    event.cpu = bpf_get_smp_processor_id();
    event.language = detect_language(pid);
    event.wait_reason = wait_reason;
    event.wait_time_ns = wait_time_ns;
    
    /* 获取栈跟踪 */
    event.stack_id = bpf_get_stackid(ctx, &stack_traces, 
        BPF_F_FAST_STACK_CMP | BPF_F_USER_STACK);
    
    bpf_get_current_comm(&event.comm, sizeof(event.comm));
    
    bpf_perf_event_output(ctx, &cpu_wait_events, BPF_F_CURRENT_CPU,
        &event, sizeof(event));
}

/* ==================== Tracepoints ==================== */

/* sched_switch - 检测上下文切换 */
SEC("tp/sched/sched_switch")
int trace_sched_switch(struct trace_event_raw_sched_switch *ctx)
{
    u32 prev_pid = ctx->prev_pid;
    u32 next_pid = ctx->next_pid;
    
    /* 记录前一个任务的状态 */
    struct task_state *state = bpf_map_lookup_elem(&task_states, &prev_pid);
    if (state) {
        u64 now = get_timestamp();
        u64 wait_time = now - state->start_time;
        
        /* 发送等待事件 */
        struct cpu_wait_event event = {};
        event.timestamp = now;
        event.pid = prev_pid;
        event.tid = prev_pid;
        event.cpu = bpf_get_smp_processor_id();
        event.language = state->language;
        event.wait_reason = state->wait_reason;
        event.wait_time_ns = wait_time;
        
        bpf_get_current_comm(&event.comm, sizeof(event.comm));
        bpf_perf_event_output(ctx, &cpu_wait_events, BPF_F_CURRENT_CPU,
            &event, sizeof(event));
        
        bpf_map_delete_elem(&task_states, &prev_pid);
    }
    
    /* 记录新任务的开始 */
    struct task_state new_state = {};
    new_state.start_time = get_timestamp();
    new_state.language = detect_language(next_pid);
    
    /* 根据prev_state推断等待原因 */
    u8 prev_state = ctx->prev_state;
    if (prev_state & TASK_INTERRUPTIBLE)
        new_state.wait_reason = WAIT_SLEEP;
    else if (prev_state & TASK_UNINTERRUPTIBLE)
        new_state.wait_reason = WAIT_IO;
    else
        new_state.wait_reason = WAIT_UNKNOWN;
    
    bpf_map_update_elem(&task_states, &next_pid, &new_state, BPF_ANY);
    
    return 0;
}

/* ==================== Kprobes - Futex ==================== */

/* futex_wait - 用户态锁等待 */
SEC("kprobe/do_futex")
int BPF_KPROBE(kprobe_futex_wait, u32 __user *uaddr, int op, u32 val)
{
    u32 pid = bpf_get_current_pid_tgid() >> 32;
    if (!is_target_pid(pid))
        return 0;
    
    struct task_state state = {};
    state.start_time = get_timestamp();
    state.wait_reason = WAIT_FUTEX;
    state.language = detect_language(pid);
    
    u32 tid = bpf_get_current_pid_tgid();
    bpf_map_update_elem(&task_states, &tid, &state, BPF_ANY);
    
    return 0;
}

/* futex_wait结束 */
SEC("kretprobe/do_futex")
int BPF_KRETPROBE(kretprobe_futex_wait)
{
    u32 tid = bpf_get_current_pid_tgid();
    struct task_state *state = bpf_map_lookup_elem(&task_states, &tid);
    
    if (state && state->wait_reason == WAIT_FUTEX) {
        u64 now = get_timestamp();
        u64 wait_time = now - state->start_time;
        
        /* 通过perf event发送 */
        struct cpu_wait_event event = {};
        event.timestamp = now;
        event.pid = bpf_get_current_pid_tgid() >> 32;
        event.tid = tid;
        event.cpu = bpf_get_smp_processor_id();
        event.language = state->language;
        event.wait_reason = WAIT_FUTEX;
        event.wait_time_ns = wait_time;
        event.stack_id = bpf_get_stackid(ctx, &stack_traces, 
            BPF_F_FAST_STACK_CMP | BPF_F_USER_STACK);
        
        bpf_get_current_comm(&event.comm, sizeof(event.comm));
        bpf_perf_event_output(ctx, &cpu_wait_events, BPF_F_CURRENT_CPU,
            &event, sizeof(event));
        
        bpf_map_delete_elem(&task_states, &tid);
    }
    
    return 0;
}

/* ==================== Kprobes - IO ==================== */

/* io_schedule - IO调度等待 */
SEC("kprobe/io_schedule")
int BPF_KPROBE(kprobe_io_schedule)
{
    u32 pid = bpf_get_current_pid_tgid() >> 32;
    if (!is_target_pid(pid))
        return 0;
    
    struct task_state state = {};
    state.start_time = get_timestamp();
    state.wait_reason = WAIT_IO;
    state.language = detect_language(pid);
    
    u32 tid = bpf_get_current_pid_tgid();
    bpf_map_update_elem(&task_states, &tid, &state, BPF_ANY);
    
    return 0;
}

SEC("kretprobe/io_schedule")
int BPF_KRETPROBE(kretprobe_io_schedule)
{
    u32 tid = bpf_get_current_pid_tgid();
    struct task_state *state = bpf_map_lookup_elem(&task_states, &tid);
    
    if (state && state->wait_reason == WAIT_IO) {
        u64 now = get_timestamp();
        u64 wait_time = now - state->start_time;
        
        struct cpu_wait_event event = {};
        event.timestamp = now;
        event.pid = bpf_get_current_pid_tgid() >> 32;
        event.tid = tid;
        event.cpu = bpf_get_smp_processor_id();
        event.language = state->language;
        event.wait_reason = WAIT_IO;
        event.wait_time_ns = wait_time;
        event.stack_id = bpf_get_stackid(ctx, &stack_traces, 
            BPF_F_FAST_STACK_CMP | BPF_F_USER_STACK);
        
        bpf_get_current_comm(&event.comm, sizeof(event.comm));
        bpf_perf_event_output(ctx, &cpu_wait_events, BPF_F_CURRENT_CPU,
            &event, sizeof(event));
        
        bpf_map_delete_elem(&task_states, &tid);
    }
    
    return 0;
}

/* ==================== Go Runtime Probes ==================== */

/* Go: runtime.futexsleep */
SEC("kprobe/runtime.futexsleep")
int BPF_KPROBE(kprobe_go_futexsleep)
{
    u32 pid = bpf_get_current_pid_tgid() >> 32;
    if (!is_target_pid(pid))
        return 0;
    
    struct task_state state = {};
    state.start_time = get_timestamp();
    state.wait_reason = WAIT_FUTEX;
    state.language = LANG_GO;
    
    /* 更新语言缓存 */
    u32 lang = LANG_GO;
    bpf_map_update_elem(&pid_language_cache, &pid, &lang, BPF_ANY);
    
    u32 tid = bpf_get_current_pid_tgid();
    bpf_map_update_elem(&task_states, &tid, &state, BPF_ANY);
    
    return 0;
}

/* Go: runtime.park_m - goroutine park */
SEC("kprobe/runtime.park_m")
int BPF_KPROBE(kprobe_go_park)
{
    u32 pid = bpf_get_current_pid_tgid() >> 32;
    if (!is_target_pid(pid))
        return 0;
    
    struct task_state state = {};
    state.start_time = get_timestamp();
    state.wait_reason = WAIT_PARK;
    state.language = LANG_GO;
    
    u32 lang = LANG_GO;
    bpf_map_update_elem(&pid_language_cache, &pid, &lang, BPF_ANY);
    
    u32 tid = bpf_get_current_pid_tgid();
    bpf_map_update_elem(&task_states, &tid, &state, BPF_ANY);
    
    return 0;
}

/* Go: runtime.lock - 互斥锁 */
SEC("kprobe/runtime.lock")
int BPF_KPROBE(kprobe_go_lock, void *l)
{
    u32 pid = bpf_get_current_pid_tgid() >> 32;
    if (!is_target_pid(pid))
        return 0;
    
    struct task_state state = {};
    state.start_time = get_timestamp();
    state.wait_reason = WAIT_LOCK;
    state.language = LANG_GO;
    
    u32 tid = bpf_get_current_pid_tgid();
    bpf_map_update_elem(&task_states, &tid, &state, BPF_ANY);
    
    return 0;
}

/* Go: runtime.semacquire - 信号量获取 */
SEC("kprobe/runtime.semacquire")
int BPF_KPROBE(kprobe_go_semacquire)
{
    u32 pid = bpf_get_current_pid_tgid() >> 32;
    if (!is_target_pid(pid))
        return 0;
    
    struct task_state state = {};
    state.start_time = get_timestamp();
    state.wait_reason = WAIT_CONDVAR;
    state.language = LANG_GO;
    
    u32 tid = bpf_get_current_pid_tgid();
    bpf_map_update_elem(&task_states, &tid, &state, BPF_ANY);
    
    return 0;
}

/* ==================== Java/USDT Probes ==================== */

/* Java: monitor enter */
SEC("usdt")
int usdt_monitor_enter(struct pt_regs *ctx)
{
    u32 pid = bpf_get_current_pid_tgid() >> 32;
    if (!is_target_pid(pid))
        return 0;
    
    struct task_state state = {};
    state.start_time = get_timestamp();
    state.wait_reason = WAIT_MONITOR;
    state.language = LANG_JAVA;
    
    u32 lang = LANG_JAVA;
    bpf_map_update_elem(&pid_language_cache, &pid, &lang, BPF_ANY);
    
    u32 tid = bpf_get_current_pid_tgid();
    bpf_map_update_elem(&task_states, &tid, &state, BPF_ANY);
    
    return 0;
}

/* Java: monitor exit */
SEC("usdt")
int usdt_monitor_exit(struct pt_regs *ctx)
{
    u32 tid = bpf_get_current_pid_tgid();
    struct task_state *state = bpf_map_lookup_elem(&task_states, &tid);
    
    if (state && state->wait_reason == WAIT_MONITOR) {
        u64 now = get_timestamp();
        u64 wait_time = now - state->start_time;
        
        struct cpu_wait_event event = {};
        event.timestamp = now;
        event.pid = bpf_get_current_pid_tgid() >> 32;
        event.tid = tid;
        event.cpu = bpf_get_smp_processor_id();
        event.language = LANG_JAVA;
        event.wait_reason = WAIT_MONITOR;
        event.wait_time_ns = wait_time;
        event.stack_id = bpf_get_stackid(ctx, &stack_traces, 
            BPF_F_FAST_STACK_CMP | BPF_F_USER_STACK);
        
        bpf_get_current_comm(&event.comm, sizeof(event.comm));
        bpf_perf_event_output(ctx, &cpu_wait_events, BPF_F_CURRENT_CPU,
            &event, sizeof(event));
        
        bpf_map_delete_elem(&task_states, &tid);
    }
    
    return 0;
}

/* Java: thread sleep begin */
SEC("usdt")
int usdt_thread_sleep_begin(struct pt_regs *ctx)
{
    u32 pid = bpf_get_current_pid_tgid() >> 32;
    if (!is_target_pid(pid))
        return 0;
    
    struct task_state state = {};
    state.start_time = get_timestamp();
    state.wait_reason = WAIT_SLEEP;
    state.language = LANG_JAVA;
    
    u32 lang = LANG_JAVA;
    bpf_map_update_elem(&pid_language_cache, &pid, &lang, BPF_ANY);
    
    u32 tid = bpf_get_current_pid_tgid();
    bpf_map_update_elem(&task_states, &tid, &state, BPF_ANY);
    
    return 0;
}

/* Java: thread park begin */
SEC("usdt")
int usdt_thread_park_begin(struct pt_regs *ctx)
{
    u32 pid = bpf_get_current_pid_tgid() >> 32;
    if (!is_target_pid(pid))
        return 0;
    
    struct task_state state = {};
    state.start_time = get_timestamp();
    state.wait_reason = WAIT_PARK_NANOS;
    state.language = LANG_JAVA;
    
    u32 lang = LANG_JAVA;
    bpf_map_update_elem(&pid_language_cache, &pid, &lang, BPF_ANY);
    
    u32 tid = bpf_get_current_pid_tgid();
    bpf_map_update_elem(&task_states, &tid, &state, BPF_ANY);
    
    return 0;
}

/* ==================== Network Wait Detection ==================== */

/* 网络接收等待 */
SEC("kprobe/tcp_recvmsg")
int BPF_KPROBE(kprobe_tcp_recvmsg)
{
    u32 pid = bpf_get_current_pid_tgid() >> 32;
    if (!is_target_pid(pid))
        return 0;
    
    struct task_state state = {};
    state.start_time = get_timestamp();
    state.wait_reason = WAIT_NETWORK;
    state.language = detect_language(pid);
    
    u32 tid = bpf_get_current_pid_tgid();
    bpf_map_update_elem(&task_states, &tid, &state, BPF_ANY);
    
    return 0;
}

SEC("kretprobe/tcp_recvmsg")
int BPF_KRETPROBE(kretprobe_tcp_recvmsg, int ret)
{
    u32 tid = bpf_get_current_pid_tgid();
    struct task_state *state = bpf_map_lookup_elem(&task_states, &tid);
    
    if (state && state->wait_reason == WAIT_NETWORK) {
        u64 now = get_timestamp();
        u64 wait_time = now - state->start_time;
        
        /* 只有阻塞接收才记录 */
        if (ret >= 0 || ret == -EAGAIN) {
            struct cpu_wait_event event = {};
            event.timestamp = now;
            event.pid = bpf_get_current_pid_tgid() >> 32;
            event.tid = tid;
            event.cpu = bpf_get_smp_processor_id();
            event.language = state->language;
            event.wait_reason = WAIT_NETWORK;
            event.wait_time_ns = wait_time;
            event.stack_id = bpf_get_stackid(ctx, &stack_traces, 
                BPF_F_FAST_STACK_CMP | BPF_F_USER_STACK);
            
            bpf_get_current_comm(&event.comm, sizeof(event.comm));
            bpf_perf_event_output(ctx, &cpu_wait_events, BPF_F_CURRENT_CPU,
                &event, sizeof(event));
        }
        
        bpf_map_delete_elem(&task_states, &tid);
    }
    
    return 0;
}

char LICENSE[] SEC("license") = "GPL";
