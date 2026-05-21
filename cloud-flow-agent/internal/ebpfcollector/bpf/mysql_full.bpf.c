// mysql_full.bpf.c - eBPF程序用于完整解析MySQL协议字段
// 包括: 命令/库名/SQL/错误等全字段解析

#include "vmlinux.h"
#include <bpf/bpf_helpers.h>
#include <bpf/bpf_tracing.h>
#include <bpf/bpf_core_read.h>

#define AF_INET 2
#define IPPROTO_TCP 6
#define MYSQL_PORT 3306

#define MYSQL_MAX_SQL_LEN 1024
#define MYSQL_MAX_DB_LEN 64
#define MYSQL_MAX_USER_LEN 32
#define MYSQL_MAX_ERRMSG_LEN 512
#define MYSQL_MAX_TABLE_LEN 64

// MySQL包类型
#define MYSQL_PACKET_OK 0x00
#define MYSQL_PACKET_ERR 0xFF
#define MYSQL_PACKET_EOF 0xFE
#define MYSQL_PACKET_AUTH 0x01

// MySQL命令类型
#define MYSQL_COM_SLEEP 0
#define MYSQL_COM_QUIT 1
#define MYSQL_COM_INIT_DB 2
#define MYSQL_COM_QUERY 3
#define MYSQL_COM_FIELD_LIST 4
#define MYSQL_COM_CREATE_DB 5
#define MYSQL_COM_DROP_DB 6
#define MYSQL_COM_REFRESH 7
#define MYSQL_COM_SHUTDOWN 8
#define MYSQL_COM_STATISTICS 9
#define MYSQL_COM_PROCESS_INFO 10
#define MYSQL_COM_CONNECT 11
#define MYSQL_COM_PROCESS_KILL 12
#define MYSQL_COM_DEBUG 13
#define MYSQL_COM_PING 14
#define MYSQL_COM_TIME 15
#define MYSQL_COM_DELAYED_INSERT 16
#define MYSQL_COM_CHANGE_USER 17
#define MYSQL_COM_BINLOG_DUMP 18
#define MYSQL_COM_TABLE_DUMP 19
#define MYSQL_COM_CONNECT_OUT 20
#define MYSQL_COM_REGISTER_SLAVE 21
#define MYSQL_COM_STMT_PREPARE 22
#define MYSQL_COM_STMT_EXECUTE 23
#define MYSQL_COM_STMT_SEND_LONG_DATA 24
#define MYSQL_COM_STMT_CLOSE 25
#define MYSQL_COM_STMT_RESET 26
#define MYSQL_COM_SET_OPTION 27
#define MYSQL_COM_STMT_FETCH 28

// MySQL错误码
#define MYSQL_ERR_ACCESS_DENIED 1045
#define MYSQL_ERR_BAD_DB 1049
#define MYSQL_ERR_TABLE_EXISTS 1050
#define MYSQL_ERR_BAD_TABLE 1051
#define MYSQL_ERR_NO_SUCH_TABLE 1146
#define MYSQL_ERR_PARSE_ERROR 1064
#define MYSQL_ERR_CONN_LOST 2013

// MySQL连接标识
struct mysql_conn_key {
    __u32 saddr;
    __u32 daddr;
    __u16 sport;
    __u16 dport;
    __u32 pid;
    __u32 netns;
};

// MySQL握手信息
struct mysql_handshake {
    __u8 protocol_version;
    char server_version[32];
    __u32 connection_id;
    __u8 auth_plugin_data[21];
    __u32 capability_flags;
    __u8 character_set;
    __u16 status_flags;
    __u16 auth_plugin_data_len;
};

// MySQL认证信息
struct mysql_auth {
    __u32 capability_flags;
    __u32 max_packet_size;
    __u8 character_set;
    char username[MYSQL_MAX_USER_LEN];
    __u8 auth_response[256];
    __u8 auth_response_len;
    char database[MYSQL_MAX_DB_LEN];
    char auth_plugin_name[32];
};

// MySQL命令信息
struct mysql_command {
    __u64 timestamp_ns;
    __u8 command;
    __u32 arg_len;
    char argument[MYSQL_MAX_SQL_LEN];
    __u16 argument_len;
    
    // 解析的SQL信息
    char tables[4][MYSQL_MAX_TABLE_LEN];
    __u8 table_count;
    __u8 is_select : 1;
    __u8 is_insert : 1;
    __u8 is_update : 1;
    __u8 is_delete : 1;
    __u8 is_ddl : 1;
    __u8 is_dcl : 1;
    __u8 is_transaction : 1;
    __u8 padding : 1;
};

// MySQL响应信息
struct mysql_response {
    __u64 timestamp_ns;
    __u64 latency_ns;
    __u8 packet_type;
    
    // OK包字段
    __u64 affected_rows;
    __u64 last_insert_id;
    __u16 status_flags;
    __u16 warnings;
    char info[256];
    
    // 错误包字段
    __u16 error_code;
    char sql_state[6];
    char error_message[MYSQL_MAX_ERRMSG_LEN];
    __u16 error_message_len;
    
    // 结果集信息
    __u32 field_count;
    __u32 row_count;
    
    // 执行状态
    __u8 is_error;
    __u8 is_ok;
    __u8 is_eof;
};

// MySQL事务
struct mysql_transaction {
    struct mysql_handshake handshake;
    struct mysql_auth auth;
    struct mysql_command command;
    struct mysql_response response;
    __u8 has_handshake;
    __u8 has_auth;
    __u8 complete;
    __u8 padding;
};

// BPF Maps

// 跟踪MySQL连接
struct {
    __uint(type, BPF_MAP_TYPE_HASH);
    __uint(max_entries, 65536);
    __type(key, struct mysql_conn_key);
    __type(value, struct mysql_transaction);
} mysql_connections SEC(".maps");

// MySQL事件队列
struct {
    __uint(type, BPF_MAP_TYPE_QUEUE);
    __uint(max_entries, 10000);
    __type(value, struct mysql_transaction);
} mysql_events SEC(".maps");

// 命令统计
struct {
    __uint(type, BPF_MAP_TYPE_PERCPU_ARRAY);
    __uint(max_entries, 32);
    __type(key, __u32);
    __type(value, __u64);
} mysql_cmd_stats SEC(".maps");

// 错误统计
struct {
    __uint(type, BPF_MAP_TYPE_HASH);
    __uint(max_entries, 1024);
    __type(key, __u16);
    __type(value, __u64);
} mysql_error_stats SEC(".maps");

// 辅助函数: 获取当前时间戳
static __always_inline __u64 get_timestamp_ns(void) {
    return bpf_ktime_get_ns();
}

// 辅助函数: 解码长度编码整数
static __always_inline __u64 decode_length_encoded_int(const char *data, int *pos) {
    __u8 first_byte;
    bpf_probe_read_kernel(&first_byte, 1, data);
    
    if (first_byte < 0xFB) {
        (*pos)++;
        return first_byte;
    } else if (first_byte == 0xFC) {
        __u16 val;
        bpf_probe_read_kernel(&val, 2, data + 1);
        *pos += 3;
        return val;
    } else if (first_byte == 0xFD) {
        __u32 val;
        bpf_probe_read_kernel(&val, 3, data + 1);
        *pos += 4;
        return val & 0xFFFFFF;
    } else if (first_byte == 0xFE) {
        __u64 val;
        bpf_probe_read_kernel(&val, 8, data + 1);
        *pos += 9;
        return val;
    }
    (*pos)++;
    return 0;
}

// 辅助函数: 获取命令类型文本
static __always_inline void get_command_name(__u8 cmd, char *buf, int buf_size) {
    switch (cmd) {
        case MYSQL_COM_SLEEP: bpf_probe_read_kernel_str(buf, buf_size, "SLEEP"); break;
        case MYSQL_COM_QUIT: bpf_probe_read_kernel_str(buf, buf_size, "QUIT"); break;
        case MYSQL_COM_INIT_DB: bpf_probe_read_kernel_str(buf, buf_size, "INIT_DB"); break;
        case MYSQL_COM_QUERY: bpf_probe_read_kernel_str(buf, buf_size, "QUERY"); break;
        case MYSQL_COM_FIELD_LIST: bpf_probe_read_kernel_str(buf, buf_size, "FIELD_LIST"); break;
        case MYSQL_COM_CREATE_DB: bpf_probe_read_kernel_str(buf, buf_size, "CREATE_DB"); break;
        case MYSQL_COM_DROP_DB: bpf_probe_read_kernel_str(buf, buf_size, "DROP_DB"); break;
        case MYSQL_COM_REFRESH: bpf_probe_read_kernel_str(buf, buf_size, "REFRESH"); break;
        case MYSQL_COM_SHUTDOWN: bpf_probe_read_kernel_str(buf, buf_size, "SHUTDOWN"); break;
        case MYSQL_COM_STATISTICS: bpf_probe_read_kernel_str(buf, buf_size, "STATISTICS"); break;
        case MYSQL_COM_PROCESS_INFO: bpf_probe_read_kernel_str(buf, buf_size, "PROCESS_INFO"); break;
        case MYSQL_COM_CONNECT: bpf_probe_read_kernel_str(buf, buf_size, "CONNECT"); break;
        case MYSQL_COM_PROCESS_KILL: bpf_probe_read_kernel_str(buf, buf_size, "PROCESS_KILL"); break;
        case MYSQL_COM_DEBUG: bpf_probe_read_kernel_str(buf, buf_size, "DEBUG"); break;
        case MYSQL_COM_PING: bpf_probe_read_kernel_str(buf, buf_size, "PING"); break;
        case MYSQL_COM_CHANGE_USER: bpf_probe_read_kernel_str(buf, buf_size, "CHANGE_USER"); break;
        case MYSQL_COM_STMT_PREPARE: bpf_probe_read_kernel_str(buf, buf_size, "STMT_PREPARE"); break;
        case MYSQL_COM_STMT_EXECUTE: bpf_probe_read_kernel_str(buf, buf_size, "STMT_EXECUTE"); break;
        case MYSQL_COM_STMT_CLOSE: bpf_probe_read_kernel_str(buf, buf_size, "STMT_CLOSE"); break;
        default: bpf_probe_read_kernel_str(buf, buf_size, "UNKNOWN"); break;
    }
}

// 辅助函数: 解析SQL类型
static __always_inline void parse_sql_type(const char *sql, int len, struct mysql_command *cmd) {
    if (len < 6) return;
    
    // 转换为小写进行比较
    char first_word[7] = {};
    #pragma unroll
    for (int i = 0; i < 6 && i < len; i++) {
        char c = sql[i];
        if (c >= 'A' && c <= 'Z') c += 32;
        first_word[i] = c;
    }
    
    if (first_word[0] == 's' && first_word[1] == 'e' && first_word[2] == 'l') {
        cmd->is_select = 1;
    } else if (first_word[0] == 'i' && first_word[1] == 'n' && first_word[2] == 's') {
        cmd->is_insert = 1;
    } else if (first_word[0] == 'u' && first_word[1] == 'p' && first_word[2] == 'd') {
        cmd->is_update = 1;
    } else if (first_word[0] == 'd' && first_word[1] == 'e' && first_word[2] == 'l') {
        cmd->is_delete = 1;
    } else if (first_word[0] == 'c' && first_word[1] == 'r' && first_word[2] == 'e') {
        cmd->is_ddl = 1;
    } else if (first_word[0] == 'd' && first_word[1] == 'r' && first_word[2] == 'o') {
        cmd->is_ddl = 1;
    } else if (first_word[0] == 'a' && first_word[1] == 'l' && first_word[2] == 't') {
        cmd->is_ddl = 1;
    } else if (first_word[0] == 'g' && first_word[1] == 'r' && first_word[2] == 'a') {
        cmd->is_dcl = 1;
    } else if (first_word[0] == 'r' && first_word[1] == 'e' && first_word[2] == 'v') {
        cmd->is_dcl = 1;
    } else if (first_word[0] == 'b' && first_word[1] == 'e' && first_word[2] == 'g') {
        cmd->is_transaction = 1;
    } else if (first_word[0] == 'c' && first_word[1] == 'o' && first_word[2] == 'm') {
        cmd->is_transaction = 1;
    } else if (first_word[0] == 'r' && first_word[1] == 'o' && first_word[2] == 'l') {
        cmd->is_transaction = 1;
    }
}

// 辅助函数: 提取表名
static __always_inline void extract_tables(const char *sql, int len, struct mysql_command *cmd) {
    // 简化实现：查找FROM/JOIN/INTO/UPDATE后面的表名
    cmd->table_count = 0;
    
    // 这里可以实现更复杂的SQL解析
    // 由于eBPF限制，这里只做简单处理
}

// ==================== MySQL握手解析 ====================

// 跟踪tcp_recvmsg - 接收MySQL握手包
SEC("kprobe/tcp_recvmsg")
int BPF_KPROBE(trace_mysql_recvmsg, struct sock *sk, struct msghdr *msg, size_t len, int nonblock, int flags, int *addr_len) {
    struct mysql_conn_key key = {};
    
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
    
    // 获取消息数据
    struct iov_iter *iter = &msg->msg_iter;
    const struct iovec *iov = iter->iov;
    
    if (!iov || len < 4) {
        return 0;
    }
    
    // 读取MySQL数据
    char buf[1024] = {};
    int buf_len = len < sizeof(buf) ? len : sizeof(buf);
    bpf_probe_read_user(buf, buf_len, iov->iov_base);
    
    // 检查是否是握手包 (第一个字节是协议版本)
    __u8 protocol_version = buf[4];  // 跳过3字节长度和1字节序号
    
    if (protocol_version == 0x0A) {  // MySQL握手协议版本10
        struct mysql_transaction *txn = bpf_map_lookup_elem(&mysql_connections, &key);
        if (!txn) {
            struct mysql_transaction new_txn = {};
            bpf_map_update_elem(&mysql_connections, &key, &new_txn, BPF_ANY);
            txn = bpf_map_lookup_elem(&mysql_connections, &key);
            if (!txn) return 0;
        }
        
        // 解析握手包
        txn->handshake.protocol_version = protocol_version;
        
        // 服务器版本 (以\0结尾的字符串)
        int sv_len = 0;
        #pragma unroll
        for (int i = 5; i < 37 && i < buf_len; i++) {
            if (buf[i] == '\0') {
                sv_len = i - 5;
                break;
            }
        }
        bpf_probe_read_kernel(txn->handshake.server_version, sv_len > 31 ? 31 : sv_len, buf + 5);
        
        // 连接ID (4字节)
        txn->handshake.connection_id = *(__u32 *)(buf + 5 + sv_len + 1);
        
        // 能力标志 (在包的后半部分)
        if (buf_len > 13) {
            txn->handshake.capability_flags = *(__u16 *)(buf + buf_len - 4);
        }
        
        txn->has_handshake = 1;
        
        bpf_map_update_elem(&mysql_connections, &key, txn, BPF_ANY);
    }
    
    return 0;
}

// ==================== MySQL命令解析 ====================

// 跟踪tcp_sendmsg - 发送MySQL命令
SEC("kprobe/tcp_sendmsg")
int BPF_KPROBE(trace_mysql_sendmsg, struct sock *sk, struct msghdr *msg, size_t size) {
    struct mysql_conn_key key = {};
    
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
    
    // 获取消息数据
    struct iov_iter *iter = &msg->msg_iter;
    const struct iovec *iov = iter->iov;
    
    if (!iov || size < 5) {
        return 0;
    }
    
    // 读取MySQL数据
    char buf[2048] = {};
    int buf_len = size < sizeof(buf) ? size : sizeof(buf);
    bpf_probe_read_user(buf, buf_len, iov->iov_base);
    
    // 获取或创建事务
    struct mysql_transaction *txn = bpf_map_lookup_elem(&mysql_connections, &key);
    if (!txn) {
        struct mysql_transaction new_txn = {};
        bpf_map_update_elem(&mysql_connections, &key, &new_txn, BPF_ANY);
        txn = bpf_map_lookup_elem(&mysql_connections, &key);
        if (!txn) return 0;
    }
    
    // 解析MySQL包头部
    __u32 packet_len = buf[0] | (buf[1] << 8) | (buf[2] << 16);
    __u8 sequence_id = buf[3];
    __u8 command = buf[4];
    
    // 记录命令
    txn->command.timestamp_ns = get_timestamp_ns();
    txn->command.command = command;
    txn->command.arg_len = packet_len - 1;  // 减去命令字节
    
    // 复制命令参数(SQL语句)
    int sql_len = txn->command.arg_len;
    if (sql_len > MYSQL_MAX_SQL_LEN - 1) {
        sql_len = MYSQL_MAX_SQL_LEN - 1;
    }
    bpf_probe_read_kernel(txn->command.argument, sql_len, buf + 5);
    txn->command.argument_len = sql_len;
    txn->command.argument[sql_len] = '\0';
    
    // 解析SQL类型
    if (command == MYSQL_COM_QUERY) {
        parse_sql_type(txn->command.argument, sql_len, &txn->command);
        extract_tables(txn->command.argument, sql_len, &txn->command);
    }
    
    // 处理特殊命令
    if (command == MYSQL_COM_INIT_DB) {
        bpf_probe_read_kernel(txn->auth.database, MYSQL_MAX_DB_LEN, buf + 5);
    }
    
    // 更新统计
    __u32 cmd_key = command < 32 ? command : 31;
    __u64 *count = bpf_map_lookup_elem(&mysql_cmd_stats, &cmd_key);
    if (count) {
        (*count)++;
    }
    
    bpf_map_update_elem(&mysql_connections, &key, txn, BPF_ANY);
    
    return 0;
}

// ==================== MySQL响应解析 ====================

// 跟踪tcp_recvmsg - 接收MySQL响应
SEC("kprobe/tcp_recvmsg_response")
int BPF_KPROBE(trace_mysql_recvmsg_response, struct sock *sk, struct msghdr *msg, size_t len, int nonblock, int flags, int *addr_len) {
    struct mysql_conn_key key = {};
    
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
    
    // 查找对应的事务
    struct mysql_transaction *txn = bpf_map_lookup_elem(&mysql_connections, &key);
    if (!txn || txn->command.timestamp_ns == 0) {
        return 0;
    }
    
    // 获取消息数据
    struct iov_iter *iter = &msg->msg_iter;
    const struct iovec *iov = iter->iov;
    
    if (!iov || len < 5) {
        return 0;
    }
    
    // 读取MySQL数据
    char buf[1024] = {};
    int buf_len = len < sizeof(buf) ? len : sizeof(buf);
    bpf_probe_read_user(buf, buf_len, iov->iov_base);
    
    // 解析响应
    txn->response.timestamp_ns = get_timestamp_ns();
    txn->response.latency_ns = txn->response.timestamp_ns - txn->command.timestamp_ns;
    
    // 跳过包长度(3字节)和序号(1字节)
    __u8 packet_type = buf[4];
    txn->response.packet_type = packet_type;
    
    int pos = 5;
    
    if (packet_type == MYSQL_PACKET_OK) {
        // OK包
        txn->response.is_ok = 1;
        txn->response.affected_rows = decode_length_encoded_int(buf + pos, &pos);
        txn->response.last_insert_id = decode_length_encoded_int(buf + pos, &pos);
        
        if (pos + 2 <= buf_len) {
            txn->response.status_flags = *(__u16 *)(buf + pos);
            pos += 2;
        }
        if (pos + 2 <= buf_len) {
            txn->response.warnings = *(__u16 *)(buf + pos);
            pos += 2;
        }
        
        txn->complete = 1;
        
    } else if (packet_type == MYSQL_PACKET_ERR) {
        // 错误包
        txn->response.is_error = 1;
        
        if (pos + 2 <= buf_len) {
            txn->response.error_code = *(__u16 *)(buf + pos);
            pos += 2;
        }
        
        // SQL状态 (5字节，以#开头)
        if (pos + 6 <= buf_len && buf[pos] == '#') {
            bpf_probe_read_kernel(txn->response.sql_state, 5, buf + pos + 1);
            pos += 6;
        }
        
        // 错误消息
        int msg_len = buf_len - pos;
        if (msg_len > MYSQL_MAX_ERRMSG_LEN - 1) {
            msg_len = MYSQL_MAX_ERRMSG_LEN - 1;
        }
        bpf_probe_read_kernel(txn->response.error_message, msg_len, buf + pos);
        txn->response.error_message_len = msg_len;
        txn->response.error_message[msg_len] = '\0';
        
        txn->complete = 1;
        
        // 更新错误统计
        __u16 err_key = txn->response.error_code;
        __u64 *err_count = bpf_map_lookup_elem(&mysql_error_stats, &err_key);
        if (err_count) {
            (*err_count)++;
        } else {
            __u64 initial = 1;
            bpf_map_update_elem(&mysql_error_stats, &err_key, &initial, BPF_ANY);
        }
        
    } else if (packet_type == MYSQL_PACKET_EOF) {
        // EOF包
        txn->response.is_eof = 1;
        
        if (pos + 2 <= buf_len) {
            txn->response.warnings = *(__u16 *)(buf + pos);
            pos += 2;
        }
        if (pos + 2 <= buf_len) {
            txn->response.status_flags = *(__u16 *)(buf + pos);
        }
        
        // EOF不一定是事务结束，可能是结果集的一部分
        if (txn->command.command != MYSQL_COM_FIELD_LIST && 
            txn->command.command != MYSQL_COM_QUERY) {
            txn->complete = 1;
        }
    } else {
        // 可能是结果集的第一字节(字段数)
        txn->response.field_count = packet_type;
    }
    
    // 如果事务完成，推送到事件队列
    if (txn->complete) {
        bpf_map_push_elem(&mysql_events, txn, BPF_ANY);
        
        // 重置事务状态，保留连接信息
        struct mysql_transaction new_txn = {};
        __builtin_memcpy(&new_txn.handshake, &txn->handshake, sizeof(struct mysql_handshake));
        new_txn.has_handshake = txn->has_handshake;
        bpf_map_update_elem(&mysql_connections, &key, &new_txn, BPF_ANY);
    } else {
        bpf_map_update_elem(&mysql_connections, &key, txn, BPF_ANY);
    }
    
    return 0;
}

// ==================== 连接关闭清理 ====================

// 跟踪tcp_close - 连接关闭时清理
SEC("kprobe/tcp_close")
int BPF_KPROBE(trace_mysql_tcp_close, struct sock *sk, long timeout) {
    struct mysql_conn_key key = {};
    
    BPF_CORE_READ_INTO(&key.saddr, sk, __sk_common.skc_rcv_saddr);
    BPF_CORE_READ_INTO(&key.daddr, sk, __sk_common.skc_daddr);
    BPF_CORE_READ_INTO(&key.sport, sk, __sk_common.skc_num);
    BPF_CORE_READ_INTO(&key.dport, sk, __sk_common.skc_dport);
    key.pid = bpf_get_current_pid_tgid() >> 32;
    key.netns = BPF_CORE_READ(sk, __sk_common.skc_net.net, ns.inum);
    
    // 清理连接跟踪
    bpf_map_delete_elem(&mysql_connections, &key);
    
    return 0;
}

// 许可证声明
char LICENSE[] SEC("license") = "GPL";
