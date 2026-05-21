// cpu_profiler.bpf.c - eBPF ON-CPU剖析程序
// 基于 perf_event 实现无侵入、无需重启的连续性能剖析
// 支持 C/C++/Golang/Java，动态采样率调整，输出火焰图+热点函数

#include "vmlinux.h"
#include <bpf/bpf_helpers.h>
#include <bpf/bpf_tracing.h>
#include <bpf/bpf_core_read.h>

// 栈深度配置
#define MAX_STACK_DEPTH 127
#define MIN_STACK_DEPTH 8

// 用户态栈深度
#define USER_STACK_DEPTH 64

// 事件类型
#define EVENT_TYPE_SAMPLE  1
#define EVENT_TYPE_LOST    2

// 剖析事件
struct prof_event {
    __u64 timestamp;        // 时间戳(纳秒)
    __u32 pid;              // 进程ID
    __u32 tid;              // 线程ID
    __u32 cpu;              // CPU核心
    __u8  comm[16];         // 进程名
    __u64 user_stack_id;    // 用户态栈ID
    __u64 kernel_stack_id;  // 内核态栈ID
    __u64 stack_trace[MAX_STACK_DEPTH]; // 合并后的栈帧
    __u32 stack_len;        // 栈帧数量
    __u8  event_type;       // 事件类型
    __u8  padding[3];
};

// 进程信息
struct proc_info {
    __u32 pid;
    __u8  comm[16];
    __u8  lang_type;        // 语言类型: 0=unknown, 1=c/c++, 2=golang, 3=java
    __u8  padding[3];
};

// 采样统计
struct sample_stats {
    __u64 total_samples;     // 总采样数
    __u64 lost_samples;      // 丢失采样数
    __u64 user_samples;      // 用户态采样数
    __u64 kernel_samples;    // 内核态采样数
    __u64 last_update;       // 最后更新时间
};

// 控制参数
struct prof_control {
    __u32 sample_freq;       // 采样频率(Hz), 默认99
    __u32 enabled;           // 是否启用
    __u32 stack_depth;       // 最大栈深度
    __u32 duration_ms;       // 剖析持续时间(毫秒), 0=连续
    __u64 filter_pid;        // 过滤进程ID, 0=全部
    __u64 start_time;        // 开始时间
};

// BPF Maps

// 栈跟踪事件队列 (供用户态读取)
struct {
    __uint(type, BPF_MAP_TYPE_PERF_EVENT_ARRAY);
    __uint(max_entries, 0);  // 由用户态设置
    __type(key, int);
    __type(value, struct prof_event);
} events SEC(".maps");

// 用户态栈表
struct {
    __uint(type, BPF_MAP_TYPE_STACK_TRACE);
    __uint(max_entries, 8192);
    __type(key, __u32);
    __type(value, __u64[USER_STACK_DEPTH]);
} user_stacks SEC(".maps");

// 内核态栈表
struct {
    __uint(type, BPF_MAP_TYPE_STACK_TRACE);
    __uint(max_entries, 8192);
    __type(key, __u32);
    __type(value, __u64[MAX_STACK_DEPTH]);
} kernel_stacks SEC(".maps");

// 统计计数器
struct {
    __uint(type, BPF_MAP_TYPE_ARRAY);
    __uint(max_entries, 1);
    __type(key, __u32);
    __type(value, struct sample_stats);
} stats SEC(".maps");

// 控制参数
struct {
    __uint(type, BPF_MAP_TYPE_ARRAY);
    __uint(max_entries, 1);
    __type(key, __u32);
    __type(value, struct prof_control);
} control SEC(".maps");

// 进程信息缓存
struct {
    __uint(type, BPF_MAP_TYPE_HASH);
    __uint(max_entries, 4096);
    __type(key, __u32);  // pid
    __type(value, struct proc_info);
} proc_cache SEC(".maps");

// 热点函数计数 (按栈帧地址)
struct {
    __uint(type, BPF_MAP_TYPE_HASH);
    __uint(max_entries, 65536);
    __type(key, __u64);  // 指令地址
    __type(value, __u64); // 采样计数
} hot_functions SEC(".maps");

// 合并栈计数 (用于火焰图)
struct {
    __uint(type, BPF_MAP_TYPE_HASH);
    __uint(max_entries, 65536);
    __type(key, struct stack_key);
    __type(value, __u64);
} stack_counts SEC(".maps");

// 栈键 (用于合并相同栈)
struct stack_key {
    __u32 pid;
    __u32 stack_id;
    __u8  comm[16];
};

// 辅助函数: 获取控制参数
static __always_inline struct prof_control *get_control(void) {
    __u32 key = 0;
    return bpf_map_lookup_elem(&control, &key);
}

// 辅助函数: 更新统计
static __always_inline void update_stats(__u8 is_user, __u8 is_kernel) {
    __u32 key = 0;
    struct sample_stats *s = bpf_map_lookup_elem(&stats, &key);
    if (s) {
        s->total_samples++;
        if (is_user) s->user_samples++;
        if (is_kernel) s->kernel_samples++;
        s->last_update = bpf_ktime_get_ns();
    }
}

// 辅助函数: 检测进程语言类型
static __always_inline __u8 detect_language(struct task_struct *task) {
    __u32 pid = BPF_CORE_READ(task, tgid);
    
    // 查找缓存
    struct proc_info *cached = bpf_map_lookup_elem(&proc_cache, &pid);
    if (cached) {
        return cached->lang_type;
    }
    
    struct proc_info info = {};
    info.pid = pid;
    
    // 获取进程名
    bpf_probe_read_kernel_str(info.comm, sizeof(info.comm), task->comm);
    
    // 通过 /proc/pid/maps 特征检测语言
    // Golang: 通常有 runtime.main 或 runtime.gopark
    // Java: 通常有 libjvm.so
    // C/C++: 默认
    
    struct mm_struct *mm = BPF_CORE_READ(task, mm);
    if (!mm) {
        info.lang_type = 1; // 默认C/C++
        bpf_map_update_elem(&proc_cache, &pid, &info, BPF_ANY);
        return info.lang_type;
    }
    
    // 读取进程的可执行文件路径
    struct file *exe_file = BPF_CORE_READ(mm, exe_file);
    if (exe_file) {
        struct path exe_path = BPF_CORE_READ(exe_file, f_path);
        struct dentry *exe_dentry = exe_path.dentry;
        struct qstr d_name = BPF_CORE_READ(exe_dentry, d_name);
        
        char exe_name[32] = {};
        bpf_probe_read_kernel_str(exe_name, sizeof(exe_name), d_name.name);
        
        // 简化检测: 通过进程名判断
        if (exe_name[0] != '\0') {
            // Golang 二进制通常是静态链接的
            // Java 进程名通常是 "java"
            // 其他默认为 C/C++
        }
    }
    
    // 通过进程名判断
    if (info.comm[0] == 'j' && info.comm[1] == 'a' && info.comm[2] == 'v' && info.comm[3] == 'a') {
        info.lang_type = 3; // Java
    } else {
        // 进一步检测: 检查是否是 Golang
        // Golang 进程通常有较大的只读数据段
        unsigned long start_code = BPF_CORE_READ(mm, start_code);
        unsigned long end_code = BPF_CORE_READ(mm, end_code);
        unsigned long text_size = end_code - start_code;
        
        // Golang 二进制通常较大 (>1MB text segment)
        if (text_size > 1024 * 1024) {
            info.lang_type = 2; // 可能是 Golang
        } else {
            info.lang_type = 1; // C/C++
        }
    }
    
    bpf_map_update_elem(&proc_cache, &pid, &info, BPF_ANY);
    return info.lang_type;
}

// ==================== ON-CPU 采样 ====================

// perf_event 采样处理
SEC("perf_event")
int on_cpu_sample(struct bpf_perf_event_data *ctx) {
    struct prof_control *ctrl = get_control();
    if (!ctrl || !ctrl->enabled) {
        return 0;
    }
    
    // 检查持续时间
    if (ctrl->duration_ms > 0 && ctrl->start_time > 0) {
        __u64 now = bpf_ktime_get_ns();
        __u64 duration_ns = (__u64)ctrl->duration_ms * 1000000ULL;
        if (now - ctrl->start_time > duration_ns) {
            ctrl->enabled = 0;
            return 0;
        }
    }
    
    // 获取当前任务
    struct task_struct *task = (struct task_struct *)bpf_get_current_task();
    __u32 pid = bpf_get_current_pid_tgid() >> 32;
    __u32 tid = bpf_get_current_pid_tgid() & 0xFFFFFFFF;
    
    // 进程过滤
    if (ctrl->filter_pid > 0 && pid != (__u32)ctrl->filter_pid) {
        return 0;
    }
    
    // 构建事件
    struct prof_event event = {};
    event.timestamp = bpf_ktime_get_ns();
    event.pid = pid;
    event.tid = tid;
    event.cpu = bpf_get_smp_processor_id();
    event.event_type = EVENT_TYPE_SAMPLE;
    
    // 获取进程名
    bpf_get_current_comm(event.comm, sizeof(event.comm));
    
    // 获取用户态栈
    event.user_stack_id = bpf_get_stackid(ctx, &user_stacks, BPF_F_USER_STACK);
    
    // 获取内核态栈
    event.kernel_stack_id = bpf_get_stackid(ctx, &kernel_stacks, 0);
    
    // 合并栈帧到事件中
    event.stack_len = 0;
    
    // 读取用户态栈
    if (event.user_stack_id != ~0ULL) {
        const __u64 *u_stacks = bpf_map_lookup_elem(&user_stacks, &event.user_stack_id);
        if (u_stacks) {
            #pragma unroll
            for (int i = 0; i < USER_STACK_DEPTH && event.stack_len < MAX_STACK_DEPTH; i++) {
                if (u_stacks[i] == 0) break;
                event.stack_trace[event.stack_len++] = u_stacks[i];
                
                // 更新热点函数计数
                __u64 addr = u_stacks[i];
                __u64 *count = bpf_map_lookup_elem(&hot_functions, &addr);
                if (count) {
                    (*count)++;
                } else {
                    __u64 initial = 1;
                    bpf_map_update_elem(&hot_functions, &addr, &initial, BPF_ANY);
                }
            }
        }
    }
    
    // 读取内核态栈
    if (event.kernel_stack_id != ~0ULL) {
        const __u64 *k_stacks = bpf_map_lookup_elem(&kernel_stacks, &event.kernel_stack_id);
        if (k_stacks) {
            #pragma unroll
            for (int i = 0; i < MAX_STACK_DEPTH && event.stack_len < MAX_STACK_DEPTH; i++) {
                if (k_stacks[i] == 0) break;
                event.stack_trace[event.stack_len++] = k_stacks[i];
            }
        }
    }
    
    // 更新合并栈计数
    if (event.stack_len >= MIN_STACK_DEPTH) {
        struct stack_key skey = {};
        skey.pid = pid;
        skey.stack_id = event.user_stack_id != ~0ULL ? event.user_stack_id : event.kernel_stack_id;
        __builtin_memcpy(skey.comm, event.comm, sizeof(skey.comm));
        
        __u64 *count = bpf_map_lookup_elem(&stack_counts, &skey);
        if (count) {
            (*count)++;
        } else {
            __u64 initial = 1;
            bpf_map_update_elem(&stack_counts, &skey, &initial, BPF_ANY);
        }
    }
    
    // 更新统计
    update_stats(event.user_stack_id != ~0ULL, event.kernel_stack_id != ~0ULL);
    
    // 发送事件到用户态
    long ret = bpf_perf_event_output(ctx, &events, BPF_F_CURRENT_CPU, &event, sizeof(event));
    if (ret != 0) {
        // 丢失计数
        __u32 key = 0;
        struct sample_stats *s = bpf_map_lookup_elem(&stats, &key);
        if (s) {
            s->lost_samples++;
        }
    }
    
    return 0;
}

// 丢失事件处理
SEC("perf_event")
int on_cpu_lost(struct bpf_perf_event_data *ctx) {
    __u32 key = 0;
    struct sample_stats *s = bpf_map_lookup_elem(&stats, &key);
    if (s) {
        s->lost_samples++;
    }
    return 0;
}

// 许可证声明
char LICENSE[] SEC("license") = "GPL";
