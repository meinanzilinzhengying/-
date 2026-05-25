/*
 * JVM Memory Profiler - eBPF Program
 * 支持ByteBuffer和JNI内存统计、泄漏检测
 * 
 * 特性:
 * - ByteBuffer.allocateDirect/free 追踪
 * - JNI内存分配追踪
 * - DirectMemory/Unsafe.allocateMemory 追踪
 * - 调用栈采集
 */

#include "vmlinux.h"
#include <bpf/bpf_helpers.h>
#include <bpf/bpf_tracing.h>
#include <bpf/bpf_core_read.h>

#define TASK_COMM_LEN 16
#define MAX_STACK_DEPTH 64
#define MAX_PIDS 1024
#define MAX_ALLOCATIONS 100000

/* 内存类型 */
enum memory_type {
    MEM_TYPE_UNKNOWN = 0,
    MEM_TYPE_BYTEBUFFER_ALLOC,
    MEM_TYPE_BYTEBUFFER_FREE,
    MEM_TYPE_JNI_MALLOC,
    MEM_TYPE_JNI_FREE,
    MEM_TYPE_DIRECT_ALLOC,
    MEM_TYPE_DIRECT_FREE,
    MEM_TYPE_UNSAFE_ALLOC,
    MEM_TYPE_UNSAFE_FREE,
};

/* JVM内存事件 */
struct jvm_mem_event {
    u64 timestamp;
    u32 pid;
    u32 tid;
    u32 memory_type;
    s64 size;           /* 正数分配，负数释放 */
    u64 address;        /* 内存地址 */
    s64 stack_id;
    u32 class_id;       /* Java类ID */
    u32 method_id;      /* Java方法ID */
    u32 line_num;       /* 行号 */
    char comm[TASK_COMM_LEN];
};

/* 分配记录 */
struct allocation_info {
    u64 timestamp;
    u32 pid;
    u32 tid;
    u32 memory_type;
    u64 size;
    s64 stack_id;
    u32 class_id;
    u32 method_id;
    u32 line_num;
    u8 active;
};

/* 采样配置 */
struct sample_config {
    u32 sample_rate;        /* 采样率 (0-100) */
    u32 min_size;           /* 最小追踪大小 */
    u32 enabled;            /* 是否启用 */
};

/* BPF Maps */

/* 事件输出map */
struct {
    __uint(type, BPF_MAP_TYPE_PERF_EVENT_ARRAY);
    __uint(key_size, sizeof(u32));
    __uint(value_size, sizeof(u32));
} jvm_mem_events SEC(".maps");

/* 采样配置map */
struct {
    __uint(type, BPF_MAP_TYPE_ARRAY);
    __uint(max_entries, 1);
    __type(key, u32);
    __type(value, struct sample_config);
} sample_config_map SEC(".maps");

/* 分配记录map */
struct {
    __uint(type, BPF_MAP_TYPE_HASH);
    __uint(max_entries, MAX_ALLOCATIONS);
    __type(key, u64);           /* address */
    __type(value, struct allocation_info);
} allocations SEC(".maps");

/* 栈跟踪map */
struct {
    __uint(type, BPF_MAP_TYPE_STACK_TRACE);
    __uint(max_entries, 10000);
    __uint(key_size, sizeof(u32));
    __uint(value_size, MAX_STACK_DEPTH * sizeof(u64));
} stack_traces SEC(".maps");

/* 目标PID过滤器 */
struct {
    __uint(type, BPF_MAP_TYPE_HASH);
    __uint(max_entries, MAX_PIDS);
    __type(key, u32);
    __type(value, u8);
} target_pids SEC(".maps");

/* 统计计数器 */
struct {
    __uint(type, BPF_MAP_TYPE_PERCPU_ARRAY);
    __uint(max_entries, 8);
    __type(key, u32);
    __type(value, u64);
} stats_counter SEC(".maps");

/* 辅助函数 */

static __always_inline u64 get_timestamp(void)
{
    return bpf_ktime_get_ns();
}

static __always_inline int should_sample(u32 size)
{
    u32 key = 0;
    struct sample_config *config;
    
    config = bpf_map_lookup_elem(&sample_config_map, &key);
    if (!config || !config->enabled)
        return 0;
    
    /* 检查最小大小 */
    if (size < config->min_size)
        return 0;
    
    /* 采样率检查 */
    if (config->sample_rate >= 100)
        return 1;
    
    /* 简单随机采样 */
    u32 rand = bpf_get_prandom_u32() % 100;
    return rand < config->sample_rate;
}

static __always_inline int is_target_pid(u32 pid)
{
    /* 空过滤器表示监控所有进程 */
    u8 *enabled = bpf_map_lookup_elem(&target_pids, &pid);
    if (enabled)
        return *enabled;
    
    /* 如果没有特定过滤器，允许所有 */
    u32 count = 0;
    u32 k = 0;
    u8 *v = bpf_map_lookup_elem(&target_pids, &k);
    if (!v)
        return 1;  /* 空map表示全部允许 */
    
    return 0;
}

static __always_inline void record_event(
    struct pt_regs *ctx,
    u32 mem_type,
    s64 size,
    u64 address,
    u32 class_id,
    u32 method_id,
    u32 line_num)
{
    u32 pid = bpf_get_current_pid_tgid() >> 32;
    
    if (!is_target_pid(pid))
        return;
    
    struct jvm_mem_event event = {};
    event.timestamp = get_timestamp();
    event.pid = pid;
    event.tid = bpf_get_current_pid_tgid();
    event.memory_type = mem_type;
    event.size = size;
    event.address = address;
    event.class_id = class_id;
    event.method_id = method_id;
    event.line_num = line_num;
    
    /* 获取栈跟踪 */
    event.stack_id = bpf_get_stackid(ctx, &stack_traces, 
        BPF_F_FAST_STACK_CMP | BPF_F_USER_STACK);
    
    bpf_get_current_comm(&event.comm, sizeof(event.comm));
    
    bpf_perf_event_output(ctx, &jvm_mem_events, BPF_F_CURRENT_CPU,
        &event, sizeof(event));
}

static __always_inline void track_allocation(
    u32 pid,
    u32 mem_type,
    u64 size,
    u64 address,
    s64 stack_id,
    u32 class_id,
    u32 method_id,
    u32 line_num)
{
    struct allocation_info info = {};
    info.timestamp = get_timestamp();
    info.pid = pid;
    info.tid = bpf_get_current_pid_tgid();
    info.memory_type = mem_type;
    info.size = size;
    info.stack_id = stack_id;
    info.class_id = class_id;
    info.method_id = method_id;
    info.line_num = line_num;
    info.active = 1;
    
    bpf_map_update_elem(&allocations, &address, &info, BPF_ANY);
}

static __always_inline void track_free(u64 address)
{
    struct allocation_info *info = bpf_map_lookup_elem(&allocations, &address);
    if (info) {
        info->active = 0;
        bpf_map_delete_elem(&allocations, &address);
    }
}

/* ==================== ByteBuffer Probes ==================== */

/* ByteBuffer.allocateDirect entry */
SEC("usdt")
int usdt_bb_alloc_enter(struct pt_regs *ctx)
{
    /* 参数: capacity (int) */
    int capacity = (int)PT_REGS_PARM1(ctx);
    
    if (!should_sample((u32)capacity))
        return 0;
    
    /* 保存分配大小到per-cpu map供return使用 */
    u32 pid = bpf_get_current_pid_tgid() >> 32;
    u64 size = (u64)capacity;
    
    /* 使用临时存储 */
    u32 key = bpf_get_smp_processor_id();
    bpf_map_update_elem(&stats_counter, &key, &size, BPF_ANY);
    
    return 0;
}

/* ByteBuffer.allocateDirect return */
SEC("usdt")
int usdt_bb_alloc_return(struct pt_regs *ctx)
{
    /* 返回值: ByteBuffer对象地址 */
    u64 buffer_addr = (u64)PT_REGS_RC(ctx);
    
    if (buffer_addr == 0)
        return 0;
    
    u32 pid = bpf_get_current_pid_tgid() >> 32;
    
    /* 获取之前保存的大小 */
    u32 key = bpf_get_smp_processor_id();
    u64 *size_ptr = bpf_map_lookup_elem(&stats_counter, &key);
    if (!size_ptr)
        return 0;
    
    u64 size = *size_ptr;
    
    /* 记录事件 */
    record_event(ctx, MEM_TYPE_BYTEBUFFER_ALLOC, (s64)size, buffer_addr, 0, 0, 0);
    
    /* 追踪分配 */
    s64 stack_id = bpf_get_stackid(ctx, &stack_traces, 
        BPF_F_FAST_STACK_CMP | BPF_F_USER_STACK);
    track_allocation(pid, MEM_TYPE_BYTEBUFFER_ALLOC, size, buffer_addr, 
        stack_id, 0, 0, 0);
    
    return 0;
}

/* DirectByteBuffer.clean entry */
SEC("usdt")
int usdt_bb_free_enter(struct pt_regs *ctx)
{
    /* this指针即buffer地址 */
    u64 buffer_addr = (u64)PT_REGS_PARM1(ctx);
    
    u32 pid = bpf_get_current_pid_tgid() >> 32;
    
    /* 查找原始分配大小 */
    struct allocation_info *info = bpf_map_lookup_elem(&allocations, &buffer_addr);
    s64 size = 0;
    if (info) {
        size = -(s64)info->size;  /* 负数表示释放 */
    }
    
    record_event(ctx, MEM_TYPE_BYTEBUFFER_FREE, size, buffer_addr, 0, 0, 0);
    track_free(buffer_addr);
    
    return 0;
}

/* ==================== JNI Memory Probes ==================== */

/* JNI NewByteArray entry */
SEC("usdt")
int usdt_jni_alloc_enter(struct pt_regs *ctx)
{
    /* 参数: JNIEnv*, jsize len */
    int len = (int)PT_REGS_PARM2(ctx);
    
    if (!should_sample((u32)len))
        return 0;
    
    u32 key = bpf_get_smp_processor_id();
    u64 size = (u64)len;
    bpf_map_update_elem(&stats_counter, &key, &size, BPF_ANY);
    
    return 0;
}

/* JNI NewByteArray return */
SEC("usdt")
int usdt_jni_alloc_return(struct pt_regs *ctx)
{
    u64 array_addr = (u64)PT_REGS_RC(ctx);
    
    if (array_addr == 0)
        return 0;
    
    u32 pid = bpf_get_current_pid_tgid() >> 32;
    
    u32 key = bpf_get_smp_processor_id();
    u64 *size_ptr = bpf_map_lookup_elem(&stats_counter, &key);
    if (!size_ptr)
        return 0;
    
    u64 size = *size_ptr;
    
    record_event(ctx, MEM_TYPE_JNI_MALLOC, (s64)size, array_addr, 0, 0, 0);
    
    s64 stack_id = bpf_get_stackid(ctx, &stack_traces, 
        BPF_F_FAST_STACK_CMP | BPF_F_USER_STACK);
    track_allocation(pid, MEM_TYPE_JNI_MALLOC, size, array_addr, 
        stack_id, 0, 0, 0);
    
    return 0;
}

/* JNI NewDirectByteBuffer entry */
SEC("usdt")
int usdt_jni_direct_alloc(struct pt_regs *ctx)
{
    /* 参数: JNIEnv*, void* address, jlong capacity */
    u64 address = (u64)PT_REGS_PARM2(ctx);
    long capacity = (long)PT_REGS_PARM3(ctx);
    
    if (!should_sample((u32)capacity))
        return 0;
    
    u32 pid = bpf_get_current_pid_tgid() >> 32;
    
    record_event(ctx, MEM_TYPE_DIRECT_ALLOC, (s64)capacity, address, 0, 0, 0);
    
    s64 stack_id = bpf_get_stackid(ctx, &stack_traces, 
        BPF_F_FAST_STACK_CMP | BPF_F_USER_STACK);
    track_allocation(pid, MEM_TYPE_DIRECT_ALLOC, (u64)capacity, address, 
        stack_id, 0, 0, 0);
    
    return 0;
}

/* JNI ReleaseByteArrayElements entry */
SEC("usdt")
int usdt_jni_release_elements(struct pt_regs *ctx)
{
    /* 参数: JNIEnv*, jbyteArray array, jbyte* elems, jint mode */
    u64 elems = (u64)PT_REGS_PARM3(ctx);
    
    u32 pid = bpf_get_current_pid_tgid() >> 32;
    
    struct allocation_info *info = bpf_map_lookup_elem(&allocations, &elems);
    s64 size = 0;
    if (info) {
        size = -(s64)info->size;
    }
    
    record_event(ctx, MEM_TYPE_JNI_FREE, size, elems, 0, 0, 0);
    track_free(elems);
    
    return 0;
}

/* ==================== Unsafe.allocateMemory Probes ==================== */

/* Unsafe.allocateMemory entry */
SEC("usdt")
int usdt_unsafe_alloc_enter(struct pt_regs *ctx)
{
    /* 参数: long bytes */
    long bytes = (long)PT_REGS_PARM1(ctx);
    
    if (!should_sample((u32)bytes))
        return 0;
    
    u32 key = bpf_get_smp_processor_id();
    u64 size = (u64)bytes;
    bpf_map_update_elem(&stats_counter, &key, &size, BPF_ANY);
    
    return 0;
}

/* Unsafe.allocateMemory return */
SEC("usdt")
int usdt_unsafe_alloc_return(struct pt_regs *ctx)
{
    /* 返回值: 内存地址 */
    u64 addr = (u64)PT_REGS_RC(ctx);
    
    if (addr == 0)
        return 0;
    
    u32 pid = bpf_get_current_pid_tgid() >> 32;
    
    u32 key = bpf_get_smp_processor_id();
    u64 *size_ptr = bpf_map_lookup_elem(&stats_counter, &key);
    if (!size_ptr)
        return 0;
    
    u64 size = *size_ptr;
    
    record_event(ctx, MEM_TYPE_UNSAFE_ALLOC, (s64)size, addr, 0, 0, 0);
    
    s64 stack_id = bpf_get_stackid(ctx, &stack_traces, 
        BPF_F_FAST_STACK_CMP | BPF_F_USER_STACK);
    track_allocation(pid, MEM_TYPE_UNSAFE_ALLOC, size, addr, 
        stack_id, 0, 0, 0);
    
    return 0;
}

/* Unsafe.freeMemory entry */
SEC("usdt")
int usdt_unsafe_free_enter(struct pt_regs *ctx)
{
    /* 参数: long address */
    u64 addr = (u64)PT_REGS_PARM1(ctx);
    
    u32 pid = bpf_get_current_pid_tgid() >> 32;
    
    struct allocation_info *info = bpf_map_lookup_elem(&allocations, &addr);
    s64 size = 0;
    if (info) {
        size = -(s64)info->size;
    }
    
    record_event(ctx, MEM_TYPE_UNSAFE_FREE, size, addr, 0, 0, 0);
    track_free(addr);
    
    return 0;
}

/* ==================== malloc/free Interception ==================== */

/* libc malloc */
SEC("uprobe/libc_malloc")
int BPF_KPROBE(libc_malloc, size_t size)
{
    u32 pid = bpf_get_current_pid_tgid() >> 32;
    
    if (!is_target_pid(pid))
        return 0;
    
    if (!should_sample((u32)size))
        return 0;
    
    /* 保存大小 */
    u32 key = bpf_get_smp_processor_id();
    u64 sz = (u64)size;
    bpf_map_update_elem(&stats_counter, &key, &sz, BPF_ANY);
    
    return 0;
}

/* libc malloc return */
SEC("uretprobe/libc_malloc")
int BPF_KRETPROBE(libc_malloc_return)
{
    u64 addr = (u64)PT_REGS_RC(ctx);
    
    if (addr == 0)
        return 0;
    
    u32 pid = bpf_get_current_pid_tgid() >> 32;
    
    u32 key = bpf_get_smp_processor_id();
    u64 *size_ptr = bpf_map_lookup_elem(&stats_counter, &key);
    if (!size_ptr)
        return 0;
    
    u64 size = *size_ptr;
    
    /* 检测是否是JNI调用 */
    s64 stack_id = bpf_get_stackid(ctx, &stack_traces, 
        BPF_F_FAST_STACK_CMP | BPF_F_USER_STACK);
    
    /* 记录为JNI分配 */
    record_event(ctx, MEM_TYPE_JNI_MALLOC, (s64)size, addr, 0, 0, 0);
    track_allocation(pid, MEM_TYPE_JNI_MALLOC, size, addr, 
        stack_id, 0, 0, 0);
    
    return 0;
}

/* libc free */
SEC("uprobe/libc_free")
int BPF_KPROBE(libc_free, void *ptr)
{
    u64 addr = (u64)ptr;
    
    if (addr == 0)
        return 0;
    
    u32 pid = bpf_get_current_pid_tgid() >> 32;
    
    if (!is_target_pid(pid))
        return 0;
    
    struct allocation_info *info = bpf_map_lookup_elem(&allocations, &addr);
    if (info) {
        s64 size = -(s64)info->size;
        record_event(ctx, MEM_TYPE_JNI_FREE, size, addr, 0, 0, 0);
        track_free(addr);
    }
    
    return 0;
}

/* ==================== JVM TI / JVMTI Hooks ==================== */

/* 对象分配事件 (需要JVMTI支持) */
SEC("usdt")
int usdt_jvmti_object_alloc(struct pt_regs *ctx)
{
    /* JVMTI ObjectAlloc回调 */
    u64 obj_addr = (u64)PT_REGS_PARM1(ctx);
    u64 size = (u64)PT_REGS_PARM2(ctx);
    u32 class_id = (u32)PT_REGS_PARM3(ctx);
    
    if (!should_sample((u32)size))
        return 0;
    
    u32 pid = bpf_get_current_pid_tgid() >> 32;
    
    record_event(ctx, MEM_TYPE_DIRECT_ALLOC, (s64)size, obj_addr, 
        class_id, 0, 0);
    
    return 0;
}

/* 类加载事件 */
SEC("usdt")
int usdt_class_load(struct pt_regs *ctx)
{
    u32 class_id = (u32)PT_REGS_PARM1(ctx);
    /* 类名在后续参数或需要通过其他方式获取 */
    
    return 0;
}

/* ==================== Periodic Stats ==================== */

/* 定期输出统计 */
SEC("tp/timer/timer_start")
int trace_timer(struct trace_event_raw_timer_start *ctx)
{
    /* 可以在这里定期输出统计信息 */
    return 0;
}

char LICENSE[] SEC("license") = "GPL";
