// mysql_sql_agg.bpf.c - MySQL SQL聚合eBPF程序
// 按SQL语句聚合: 请求数、成功率、平均时延、异常统计
// 关联数据库进程性能

#include "vmlinux.h"
#include <bpf/bpf_helpers.h>
#include <bpf/bpf_tracing.h>
#include <bpf/bpf_core_read.h"

#define AF_INET 2
#define IPPROTO_TCP 6
#define MYSQL_PORT 3306

#define MAX_SQL_HASH_LEN 64  // SQL哈希键最大长度
#define MAX_SQL_BUCKETS 16384 // SQL聚合桶数量

// SQL聚合键 (基于SQL语句的哈希)
struct sql_agg_key {
    __u32 pid;               // 进程ID
    __u32 netns;           // 网络命名空间
    __u8 sql_type;          // SQL类型: 1=SELECT, 2=INSERT, 3=UPDATE, 4=DELETE, 5=DDL, 6=DCL, 0=OTHER
    __u8 database[32];     // 数据库名(哈希)
    __u8 table[32];        // 表名(哈希)
    __u8 cmd_type;         // MySQL命令类型
};

// SQL聚合统计值
struct sql_agg_value {
    __u64 request_count;     // 请求数
    __u64 success_count;     // 成功数
    __u64 error_count;      // 异常数
    __u64 total_latency_ns; // 总时延(纳秒)
    __u64 avg_latency_ns;    // 平均时延(纳秒)
    __u64 max_latency_ns;    // 最大时延
    __u64 min_latency_ns;    // 最小时延
    __u64 last_timestamp;    // 最后请求时间
    __u64 timeout_count;    // 超时次数
    __u64 retry_count;      // 重试次数
};

// 数据库进程性能统计
struct db_process_stats {
    __u32 pid;               // 进程ID
    __u64 cpu_time_ns;       // CPU时间(纳秒)
    __u64 memory_rss;       // 内存RSS(字节)
    __u64 io_read_bytes;     // I/O读字节
    __u64 io_write_bytes;    // I/O写字节
    __u64 connections;        // 当前连接数
    __u64 queries_per_sec;   // 每秒查询数
    __u64 transactions;       // 事务数
    __u64 lock_waits;       // 锁等待次数
    __u64 slow_queries;      // 慢查询数
    __u64 last_update;      // 最后更新时间
};

// 全局SQL聚合统计
struct global_sql_stats {
    __u64 total_requests;    // 总请求数
    __u64 total_success;    // 总成功数
    __u64 total_errors;     // 总错误数
    __u64 avg_latency_ns;   // 全局平均时延
    __u64 slow_queries;     // 慢查询总数
    __u64 queries_1s;      // 1秒内查询数
    __u64 queries_10s;      // 10秒内查询数
    __u64 queries_60s;     // 60秒内查询数
    __u64 last_flush;      // 最后刷新时间
};

// BPF Maps

// SQL聚合统计 (按SQL类型/库/表聚合)
struct {
    __uint(type, BPF_MAP_TYPE_HASH);
    __uint(max_entries, MAX_SQL_BUCKETS);
    __type(key, struct sql_agg_key);
    __type(value, struct sql_agg_value);
    __uint(map_flags, BPF_F_NO_PREALLOC);
} sql_aggregation SEC(".maps");

// SQL原始事件队列 (供用户态详细分析)
struct {
    __uint(type, BPF_MAP_TYPE_QUEUE);
    __uint(max_entries, 10000);
    __type(value, struct sql_event);
} sql_events SEC(".maps");

// SQL事件
struct sql_event {
    __u64 timestamp;
    __u32 pid;
    __u32 tid;
    __u32 netns;
    __u8 sql_type;
    __u8 status;       // 0=执行中, 1=成功, 2=错误, 3=超时
    __u64 latency_ns;
    __u16 error_code;
    __u8 database[32];
    __u8 table[32];
    __u8 cmd_type;
    char sql_fingerprint[64]; // SQL指纹(简化)
};

// 数据库进程性能统计
struct {
    __uint(type, BPF_MAP_TYPE_HASH);
    __uint(max_entries, 1024);
    __type(key, __u32); // pid
    __type(value, struct db_process_stats);
} db_process_stats SEC(".maps");

// 全局SQL统计
struct {
    __uint(type, BPF_MAP_TYPE_ARRAY);
    __uint(max_entries, 1);
    __type(key, __u32);
    __type(value, struct global_sql_stats);
} global_sql_stats SEC(".maps");

// 慢SQL阈值配置 (由用户态设置)
struct {
    __uint(type, BPF_MAP_TYPE_ARRAY);
    __uint(max_entries, 1);
    __type(key, __u32);
    __type(value, __u64); // 慢SQL阈值(纳秒)
} slow_query_threshold SEC(".maps");

// 辅助函数: 获取当前时间戳
static __always_inline __u64 get_timestamp_ns(void) {
    return bpf_ktime_get_ns();
}

// 辅助函数: 简单哈希函数
static __always_inline __u32 simple_hash(const char *data, __u32 len) {
    __u32 hash = 5381;
    #pragma unroll
    for (__u32 i = 0; i < 64 && i < len; i++) {
        hash = ((hash << 5) + hash) + data[i];
    }
    return hash;
}

// 辅助函数: 确定SQL类型
static __always_inline __u8 get_sql_type(const char *sql, __u32 len) {
    if (len < 6) return 0;
    
    // 转换为小写比较
    #pragma unroll
    for (__u32 i = 0; i < 6; i++) {
        if (sql[i] >= 'A' && sql[i] <= 'Z') {
            char tmp = sql[i] + 32;
            if (i == 0) {
                if (tmp == 's') return 1; // SELECT
                if (tmp == 'i') return 2; // INSERT
                if (tmp == 'u') return 3; // UPDATE
                if (tmp == 'd') return 4; // DELETE
                if (tmp == 'c') return 5; // CREATE/ALTER
                if (tmp == 'd') return 5; // DROP
                if (tmp == 'g') return 5; // GRANT
                if (tmp == 'r') return 5; // REVOKE
            }
        }
    }
    
    // 进一步检查
    if (sql[0] == 's' || sql[0] == 'S') {
        if (sql[1] == 'e' || sql[1] == 'E') return 1; // SELECT
    } else if (sql[0] == 'i' || sql[0] == 'I') {
        if (sql[1] == 'n' || sql[1] == 'N') return 2; // INSERT
    } else if (sql[0] == 'u' || sql[0] == 'U') {
        if (sql[1] == 'p' || sql[1] == 'P') return 3; // UPDATE
    } else if (sql[0] == 'd' || sql[0] == 'D') {
        if (sql[1] == 'e' || sql[1] == 'E') return 4; // DELETE
        if (sql[1] == 'r' || sql[1] == 'R') return 5; // DROP
    }
    
    return 0; // OTHER
}

// 辅助函数: 提取表名
static __always_inline void extract_table(const char *sql, __u32 len, char *table_out, __u32 *table_len) {
    // 简化实现: 查找 FROM/JOIN/INTO/UPDATE 后面的表名
    *table_len = 0;
    
    const char *keywords[] = {" from ", " join ", " into ", " update ", " table "};
    
    #pragma unroll
    for (int k = 0; k < 5; k++) {
        const char *kw = keywords[k];
        int kw_len = 6;
        
        #pragma unroll
        for (__u32 i = 0; i < 128 && i < len - kw_len; i++) {
            int match = 1;
            #pragma unroll
            for (int j = 0; j < kw_len; j++) {
                char sc = sql[i + j];
                char kc = kw[j];
                if (sc >= 'A' && sc <= 'Z') sc += 32;
                if (kc >= 'A' && kc <= 'Z') kc += 32;
                if (sc != kc) {
                    match = 0;
                    break;
                }
            }
            
            if (match) {
                // 提取表名
                __u32 start = i + kw_len;
                __u32 end = start;
                #pragma unroll
                for (; end < start + 64 && end < len; end++) {
                    if (sql[end] == ' ' || sql[end] == ',' || sql[end] == ';' || 
                        sql[end] == '(' || sql[end] == '\n' || sql[end] == '\r' || sql[end] == '\t') {
                        break;
                    }
                }
                
                *table_len = end - start;
                if (*table_len > 31) *table_len = 31;
                
                #pragma unroll
                for (__u32 j = 0; j < *table_len; j++) {
                    table_out[j] = sql[start + j];
                }
                table_out[*table_len] = '\0';
                return;
            }
        }
    }
}

// ==================== MySQL SQL事件跟踪 ====================

// 跟踪MySQL查询命令
SEC("kprobe/tcp_sendmsg")
int BPF_KPROBE(trace_mysql_sql_sendmsg, struct sock *sk, struct msghdr *msg, size_t size) {
    struct mysql_conn_key key = {};
    struct mysql_transaction *txn;
    
    // 读取连接信息
    BPF_CORE_READ_INTO(&key.saddr, sk, __sk_common.skc_rcv_saddr);
    BPF_CORE_READ_INTO(&key.daddr, sk, __sk_common.skc_daddr);
    BPF_CORE_READ_INTO(&key.sport, sk, __sk_common.skc_num);
    BPF_CORE_READ_INTO(&key.dport, sk, __sk_common.skc_dport);
    key.pid = bpf_get_current_pid_tgid() >> 32;
    key.netns = BPF_CORE_READ(sk, __sk_common.skc_net.net, ns.inum);
    
    // 检查是否是MySQL端口
    if (key.dport != bpf_htons(MYSQL_PORT) && key.sport != bpf_htons(MYSQL_PORT)) {
        return 0;
    }
    
    // 获取或创建事务
    txn = bpf_map_lookup_elem(&mysql_connections, &key);
    if (!txn) {
        struct mysql_transaction new_txn = {};
        bpf_map_update_elem(&mysql_connections, &key, &new_txn, BPF_ANY);
        txn = bpf_map_lookup_elem(&mysql_connections, &key);
        if (!txn) return 0;
    }
    
    // 解析MySQL包
    struct iov_iter *iter = &msg->msg_iter;
    const struct iovec *iov = iter->iov;
    
    if (!iov || size < 5) return 0;
    
    char buf[2048] = {};
    int buf_len = size < sizeof(buf) ? size : sizeof(buf);
    bpf_probe_read_user(buf, buf_len, iov->iov_base);
    
    __u32 packet_len = buf[0] | (buf[1] << 8) | (buf[2] << 16);
    __u8 sequence_id = buf[3];
    __u8 command = buf[4];
    
    // 只处理QUERY命令
    if (command != MYSQL_COM_QUERY) return 0;
    
    // 记录SQL开始时间
    txn->command.timestamp_ns = get_timestamp_ns();
    txn->command.command = command;
    txn->command.arg_len = packet_len - 1;
    
    // 复制SQL语句
    int sql_len = txn->command.arg_len;
    if (sql_len > MYSQL_MAX_SQL_LEN - 1) {
        sql_len = MYSQL_MAX_SQL_LEN - 1;
    }
    bpf_probe_read_kernel(txn->command.argument, sql_len, buf + 5);
    txn->command.argument_len = sql_len;
    txn->command.argument[sql_len] = '\0';
    
    // 解析SQL类型
    txn->command.is_select = (get_sql_type(txn->command.argument, sql_len) == 1) ? 1 : 0;
    
    bpf_map_update_elem(&mysql_connections, &key, txn, BPF_ANY);
    
    return 0;
}

// 跟踪MySQL响应
SEC("kprobe/tcp_recvmsg")
int BPF_KPROBE(trace_mysql_sql_recvmsg, struct sock *sk, struct msghdr *msg, size_t len, int nonblock, int flags, int *addr_len) {
    struct mysql_conn_key key = {};
    
    // 读取连接信息
    BPF_CORE_READ_INTO(&key.saddr, sk, __sk_common.skc_rcv_saddr);
    BPF_CORE_READ_INTO(&key.daddr, sk, __sk_common.skc_daddr);
    BPF_CORE_READ_INTO(&key.sport, sk, __sk_common.skc_num);
    BPF_CORE_READ_INTO(&key.dport, sk, __sk_common.skc_dport);
    key.pid = bpf_get_current_pid_tgid() >> 32;
    key.netns = BPF_CORE_READ(sk, __sk_common.skc_net.net, ns.inum);
    
    // 查找事务
    struct mysql_transaction *txn = bpf_map_lookup_elem(&mysql_connections, &key);
    if (!txn || txn->command.timestamp_ns == 0) return 0;
    
    // 获取响应数据
    struct iov_iter *iter = &msg->msg_iter;
    const struct iovec *iov = iter->iov;
    
    if (!iov || len < 5) return 0;
    
    char buf[1024] = {};
    int buf_len = len < sizeof(buf) ? len : sizeof(buf);
    bpf_probe_read_user(buf, buf_len, iov->iov_base);
    
    // 解析响应
    __u8 packet_type = buf[4];
    
    // 计算延迟
    __u64 latency_ns = get_timestamp_ns() - txn->command.timestamp_ns;
    
    // 获取慢SQL阈值
    __u32 thresh_key = 0;
    __u64 *slow_threshold = bpf_map_lookup_elem(&slow_query_threshold, &thresh_key);
    __u64 threshold = slow_threshold ? *slow_threshold : 1000000000ULL; // 默认1秒
    
    // 构建SQL聚合键
    struct sql_agg_key agg_key = {};
    agg_key.pid = key.pid;
    agg_key.netns = key.netns;
    agg_key.sql_type = get_sql_type(txn->command.argument, txn->command.argument_len);
    agg_key.cmd_type = txn->command.command;
    
    // 提取表名
    char table_name[32] = {};
    __u32 table_len = 0;
    extract_table(txn->command.argument, txn->command.argument_len, table_name, &table_len);
    
    // 填充键
    if (table_len > 0) {
        __builtin_memcpy(agg_key.table, table_name, table_len);
    }
    
    // 获取或创建聚合值
    struct sql_agg_value *agg_value = bpf_map_lookup_elem(&sql_aggregation, &agg_key);
    if (!agg_value) {
        struct sql_agg_value new_value = {};
        new_value.request_count = 1;
        new_value.last_timestamp = get_timestamp_ns();
        new_value.min_latency_ns = latency_ns;
        new_value.max_latency_ns = latency_ns;
        
        if (packet_type == MYSQL_PACKET_OK) {
            new_value.success_count = 1;
        } else if (packet_type == MYSQL_PACKET_ERR) {
            new_value.error_count = 1;
        }
        new_value.total_latency_ns = latency_ns;
        new_value.avg_latency_ns = latency_ns;
        
        bpf_map_update_elem(&sql_aggregation, &agg_key, &new_value, BPF_ANY);
    } else {
        agg_value->request_count++;
        agg_value->total_latency_ns += latency_ns;
        agg_value->avg_latency_ns = agg_value->total_latency_ns / agg_value->request_count;
        agg_value->last_timestamp = get_timestamp_ns();
        
        if (latency_ns > agg_value->max_latency_ns) {
            agg_value->max_latency_ns = latency_ns;
        }
        if (latency_ns < agg_value->min_latency_ns) {
            agg_value->min_latency_ns = latency_ns;
        }
        
        if (packet_type == MYSQL_PACKET_OK) {
            agg_value->success_count++;
        } else if (packet_type == MYSQL_PACKET_ERR) {
            agg_value->error_count++;
        }
        
        if (latency_ns > threshold) {
            agg_value->timeout_count++;
        }
    }
    
    // 更新全局统计
    __u32 gkey = 0;
    struct global_sql_stats *gstats = bpf_map_lookup_elem(&global_sql_stats, &gkey);
    if (gstats) {
        gstats->total_requests++;
        
        if (packet_type == MYSQL_PACKET_OK) {
            gstats->total_success++;
        } else if (packet_type == MYSQL_PACKET_ERR) {
            gstats->total_errors++;
        }
        
        gstats->avg_latency_ns = (gstats->avg_latency_ns * (gstats->total_requests - 1) + latency_ns) / gstats->total_requests;
        
        if (latency_ns > threshold) {
            gstats->slow_queries++;
        }
        
        // 时间窗口统计
        gstats->queries_1s++;
        gstats->queries_10s++;
        gstats->queries_60s++;
    }
    
    // 更新数据库进程性能统计
    struct db_process_stats *proc_stats = bpf_map_lookup_elem(&db_process_stats, &key.pid);
    if (!proc_stats) {
        struct db_process_stats new_stats = {};
        new_stats.pid = key.pid;
        new_stats.last_update = get_timestamp_ns();
        bpf_map_update_elem(&db_process_stats, &key.pid, &new_stats, BPF_ANY);
        proc_stats = bpf_map_lookup_elem(&db_process_stats, &key.pid);
    }
    
    if (proc_stats) {
        proc_stats->queries_per_sec++;
        proc_stats->last_update = get_timestamp_ns();
        
        if (latency_ns > threshold) {
            proc_stats->slow_queries++;
        }
    }
    
    return 0;
}

// ==================== 数据库进程性能跟踪 ====================

// 跟踪进程创建/销毁以更新连接数
SEC("kprobe/tcp_connect")
int BPF_KPROBE(trace_db_connect, struct sock *sk) {
    __u32 dport = BPF_CORE_READ(sk, __sk_common.skc_dport);
    
    // 检查是否是MySQL连接
    if (dport != bpf_htons(MYSQL_PORT)) return 0;
    
    __u32 pid = bpf_get_current_pid_tgid() >> 32;
    
    struct db_process_stats *stats = bpf_map_lookup_elem(&db_process_stats, &pid);
    if (!stats) {
        struct db_process_stats new_stats = {};
        new_stats.pid = pid;
        new_stats.connections = 1;
        new_stats.last_update = get_timestamp_ns();
        bpf_map_update_elem(&db_process_stats, &pid, &new_stats, BPF_ANY);
    } else {
        stats->connections++;
        stats->last_update = get_timestamp_ns();
    }
    
    return 0;
}

// 跟踪事务开始
SEC("kprobe/tcp_close")
int BPF_KPROBE(trace_mysql_tcp_close, struct sock *sk, long timeout) {
    __u32 dport = BPF_CORE_READ(sk, __sk_common.skc_dport);
    
    if (dport != bpf_htons(MYSQL_PORT)) return 0;
    
    __u32 pid = bpf_get_current_pid_tgid() >> 32;
    
    // 清理连接跟踪
    struct mysql_conn_key key = {};
    BPF_CORE_READ_INTO(&key.saddr, sk, __sk_common.skc_rcv_saddr);
    BPF_CORE_READ_INTO(&key.daddr, sk, __sk_common.skc_daddr);
    BPF_CORE_READ_INTO(&key.sport, sk, __sk_common.skc_num);
    BPF_CORE_READ_INTO(&key.dport, sk, __sk_common.skc_dport);
    key.pid = pid;
    key.netns = BPF_CORE_READ(sk, __sk_common.skc_net.net, ns.inum);
    
    bpf_map_delete_elem(&mysql_connections, &key);
    
    return 0;
}

// 许可证声明
char LICENSE[] SEC("license") = "GPL";
