//go:build linux

// Package adapter 提供多数据库类型适配
// 本文件实现GaussDB 200/300 适配
package adapter

import (
	"fmt"
)

// GaussDBAdapter GaussDB数据库适配器
type GaussDBAdapter struct {
	config    *DBConfig
	version   string
	clusterMode bool // 是否为集群模式
}

// NewGaussDBAdapter 创建GaussDB适配器
func NewGaussDBAdapter(config *DBConfig) *GaussDBAdapter {
	return &GaussDBAdapter{
		config:      config,
		clusterMode: config.Mode == "cluster" || config.Mode == "HA",
	}
}

// GetType 返回数据库类型
func (a *GaussDBAdapter) GetType() DBType {
	return DBTypeGaussDB
}

// GetVersion 返回数据库版本
func (a *GaussDBAdapter) GetVersion() string {
	return a.version
}

// SetVersion 设置版本
func (a *GaussDBAdapter) SetVersion(version string) {
	a.version = version
}

// GetFeatures 返回数据库特性
func (a *GaussDBAdapter) GetFeatures() DBFeatures {
	return DBTypeGaussDB.GetFeatures()
}

// GetSystemTables 获取系统表
func (a *GaussDBAdapter) GetSystemTables() []string {
	return []string{
		"PG_CATALOG.PG_CLASS",        // 类信息
		"PG_CATALOG.PG_ATTRIBUTE",    // 属性信息
		"PG_CATALOG.PG_PROC",        // 过程/函数信息
		"PG_CATALOG.PG_TYPE",         // 类型信息
		"PG_CATALOG.PG_NAMESPACE",    // 命名空间
		"PG_CATALOG.PG_INDEX",       // 索引信息
		"PG_CATALOG.PG_CONSTRAINT",  // 约束信息
		"PG_CATALOG.PG_STAT_ACTIVITY", // 会话/进程信息
		"PG_CATALOG.PG_STAT_USER_TABLES", // 用户表统计
		"PG_CATALOG.PG_STAT_USER_INDEXES", // 用户索引统计
		"PG_CATALOG.PG_LOCKS",       // 锁信息
		"PG_CATALOG.PG_THREAD_WAIT_STATUS", // 线程等待状态
		"GS_SESSION_MEMORY_STATISTICS", // 会话内存统计
		"GS_WAIT_EVENTS",           // 等待事件
		"GS_TOTAL_MEMORY_DETAIL",    // 内存详情
		"GS_FREE_CONNECTION",        // 空闲连接
		"GS_LONG_WAIT",              // 长时间等待
		"GS_SQL_COUNT",              // SQL统计
	}
}

// GetSlowQuerySQL 获取慢查询SQL
func (a *GaussDBAdapter) GetSlowQuerySQL() string {
	return `SELECT 
		UNIQUE SQL_ID,
		MAX(DEBUGINFO) AS SQL_TEXT,
		MAX(START_TIME) AS START_TIME,
		MAX(TOTAL_TIME) AS TOTAL_TIME,
		MAX(EXECUTE_TIME) AS EXECUTE_TIME,
		MAX(PARSING_TIME) AS PARSING_TIME,
		MAX(PLAN_TIME) AS PLAN_TIME,
		MAX(QUERY_REWRITE_TIME) AS QUERY_REWRITE_TIME,
		MAX(QUERY_SLOW_TIME) AS QUERY_SLOW_TIME,
		MAX(ROWS_PROCESSED) AS ROWS_PROCESSED,
		MAX(N_NODES) AS N_NODES,
		COUNT(*) AS EXECUTION_COUNT
	FROM PG_CATALOG.PG_SLOW_QUERY_INFO
	WHERE TRACK_STMT = 'on'
		AND UNIQUE SQL_ID != 'undefined'
		AND (TOTAL_TIME / EXECUTION_COUNT) > ?
	GROUP BY UNIQUE SQL_ID
	ORDER BY MAX(TOTAL_TIME) DESC`
}

// GetActiveSessionSQL 获取活跃会话SQL
func (a *GaussDBAdapter) GetActiveSessionSQL() string {
	return `SELECT 
		sa.SID,
		sa.SESSIONID,
		sa.THREADID,
		sa.STATE,
		sa.USENAME,
		sa.DATNAME,
		sa.CLIENT_ADDR,
		sa.CLIENT_PORT,
		sa.APPLICATION_NAME,
		sa.XACT_START,
		sa.QUERY_START,
		sa.WAIT_EVENT_TYPE,
		sa.WAIT_EVENT,
		sa.QUERY
	FROM PG_CATALOG.PG_STAT_ACTIVITY sa
	WHERE sa.STATE = 'active'
		AND sa.USENAME != 'Ruby'
	ORDER BY sa.QUERY_START`
}

// GetTopSQLSQL 获取TOP SQL SQL
func (a *GaussDBAdapter) GetTopSQLSQL() string {
	return `SELECT 
		UNIQUE SQL_ID,
		MAX(DEBUGINFO) AS SQL_TEXT,
		MAX(TOTAL_TIME) AS TOTAL_TIME,
		MAX(EXECUTE_TIME) AS EXECUTE_TIME,
		MAX(ROWS_PROCESSED) AS ROWS_PROCESSED,
		COUNT(*) AS EXECUTION_COUNT
	FROM PG_CATALOG.PG_SLOW_QUERY_INFO
	WHERE TRACK_STMT = 'on'
	GROUP BY UNIQUE SQL_ID
	ORDER BY MAX(TOTAL_TIME) DESC
	LIMIT ?`
}

// GetTableStatsSQL 获取表统计SQL
func (a *GaussDBAdapter) GetTableStatsSQL() string {
	return `SELECT 
		schemaname,
		relname,
		n_live_tup,
		n_dead_tup,
		n_mod_since_analyze,
		last_vacuum,
		last_autovacuum,
		last_analyze,
		last_autoanalyze,
		vacuum_count,
		autovacuum_count,
		analyze_count,
		autoanalyze_count
	FROM PG_CATALOG.PG_STAT_USER_TABLES
	WHERE schemaname = ?
		AND relname LIKE ?
	ORDER BY n_live_tup DESC`
}

// GetIndexStatsSQL 获取索引统计SQL
func (a *GaussDBAdapter) GetIndexStatsSQL() string {
	return `SELECT 
		schemaname,
		relname,
		indexrelname,
		idx_scan,
		idx_tup_read,
		idx_tup_fetch,
		idx_blks_read,
		idx_blks_hit
	FROM PG_CATALOG.PG_STAT_USER_INDEXES
	WHERE schemaname = ?
	ORDER BY idx_scan DESC`
}

// GetConnectionInfoSQL 获取连接信息SQL
func (a *GaussDBAdapter) GetConnectionInfoSQL() string {
	return `SELECT 
		sa.SID,
		sa.SESSIONID,
		sa.USENAME,
		sa.DATNAME,
		sa.CLIENT_ADDR,
		sa.CLIENT_PORT,
		sa.PROTOCOL_NAME,
		sa.CONNECT_SOURCE,
		sa.START_TIME,
		sa.REFERENCE_COUNT,
		sa.CLOSE_TIMESTAMP
	FROM GS_FREE_CONNECTION sa
	WHERE sa.FLAG = 'normal'
	ORDER BY sa.START_TIME`
}

// GetLockInfoSQL 获取锁信息SQL
func (a *GaussDBAdapter) GetLockInfoSQL() string {
	return `SELECT 
		l.locktype,
		l.relation,
		l.page,
		l.tuple,
		l.transactionid,
		l.classid,
		l.objid,
		l.objsubid,
		l.database,
		l.pid,
		l.mode,
		l.granted,
		l.fastpath,
		sa.USENAME,
		sa.STATE,
		sa.QUERY
	FROM PG_CATALOG.PG_LOCKS l
	LEFT JOIN PG_CATALOG.PG_STAT_ACTIVITY sa ON l.pid = sa.SID
	WHERE l.granted = false
	ORDER BY l.transactionid`
}

// GetMemorySQL 获取内存信息SQL
func (a *GaussDBAdapter) GetMemorySQL() string {
	return `SELECT 
		SCHNAME,
		INST_ID,
		SESSION_ID,
		SESSION_NODE,
		DNAME,
		WORK_MEM,
		TOTAL_MEMORY,
		USED_MEMORY,
		PEAK_MEMORY,
		FREE_MEMORY
	FROM GS_TOTAL_MEMORY_DETAIL
	WHERE SESSION_ID != 0`
}

// GetWaitEventSQL 获取等待事件SQL
func (a *GaussDBAdapter) GetWaitEventSQL() string {
	return `SELECT 
		sa.SID,
		sa.USENAME,
		sa.WAIT_EVENT_TYPE,
		sa.WAIT_EVENT,
		COUNT(*) AS EVENT_COUNT
	FROM PG_CATALOG.PG_STAT_ACTIVITY sa
	WHERE sa.STATE = 'active'
	GROUP BY sa.SID, sa.USENAME, sa.WAIT_EVENT_TYPE, sa.WAIT_EVENT
	ORDER BY EVENT_COUNT DESC`
}

// GetThreadWaitSQL 获取线程等待SQL
func (a *GaussDBAdapter) GetThreadWaitSQL() string {
	return `SELECT 
		TID,
		NODE_NAME,
		TNAME,
		QUERY,
		TSTATUS,
		LWPCOUNT,
		TOTAL_TIME,
		BLK_CXT_SIZE,
		MAX_CXT_SIZE,
		STACK_DEPTH,
		STACK_SIZE,
		START_TIME,
		KILL_FLAG
	FROM GS_THREAD_WAIT_STATUS
	WHERE TSTATUS != 'pending'
		AND TSTATUS != 'ended'
	ORDER BY TOTAL_TIME DESC`
}

// FormatSQL 格式化SQL语句
func (a *GaussDBAdapter) FormatSQL(sql string) string {
	// GaussDB特定的SQL格式化
	return sql
}

// GetExplainSQL 获取执行计划SQL
func (a *GaussDBAdapter) GetExplainSQL(sql string) string {
	return fmt.Sprintf("EXPLAIN %s", sql)
}

// GetAnalyzeSQL 获取分析SQL
func (a *GaussDBAdapter) GetAnalyzeSQL(tableName string) string {
	return fmt.Sprintf("ANALYZE %s", tableName)
}

// GetVacuumSQL 获取清理SQL
func (a *GaussDBAdapter) GetVacuumSQL(tableName string) string {
	return fmt.Sprintf("VACUUM %s", tableName)
}

// GetKillSessionSQL 生成终止会话SQL
func (a *GaussDBAdapter) GetKillSessionSQL(pid int) string {
	return fmt.Sprintf("SELECT PG_TERMINATE_SESSION(%d, %d)", pid, pid)
}

// GetProcessListSQL 获取进程列表SQL
func (a *GaussDBAdapter) GetProcessListSQL() string {
	return `SELECT 
		sa.SID,
		sa.USENAME,
		sa.DATNAME,
		sa.CLIENT_ADDR,
		sa.PROTOCOL_NAME,
		sa.STATE,
		sa.QUERY
	FROM PG_CATALOG.PG_STAT_ACTIVITY sa
	WHERE sa.STATE != 'idle'
	ORDER BY sa.QUERY_START`
}

// GetVersionSQL 获取版本SQL
func (a *GaussDBAdapter) GetVersionSQL() string {
	return "SELECT GS_VERSION()"
}

// GetCurrentSessionSQL 获取当前会话SQL
func (a *GaussDBAdapter) GetCurrentSessionSQL() string {
	return "SELECT PG_BACKEND_PID()"
}

// GetReplicationInfoSQL 获取复制信息SQL
func (a *GaussDBAdapter) GetReplicationInfoSQL() string {
	return `SELECT 
		USENAME,
		APPLICATION_NAME,
		CLIENT_ADDR,
		CLIENT_HOSTNAME,
		BACKEND_START,
		STATE,
		SENT_LSN,
		WRITE_LSN,
		FLUSH_LSN,
		REPLAY_LSN,
		WRITE_LAG,
		FLUSH_LAG,
		REPLAY_LAG,
		SYNC_STATE
	FROM PG_CATALOG.PG_STAT_REPLICATION`
}

// GetDatabaseSizeSQL 获取数据库大小SQL
func (a *GaussDBAdapter) GetDatabaseSizeSQL() string {
	return `SELECT 
		datname,
		pg_size_pretty(pg_database_size(datname)) AS size,
		pg_database_size(datname) AS size_bytes
	FROM PG_CATALOG.PG_DATABASE
	WHERE datname = current_database()`
}

// GetTableSizeSQL 获取表大小SQL
func (a *GaussDBAdapter) GetTableSizeSQL() string {
	return `SELECT 
		schemaname,
		relname,
		pg_size_pretty(pg_total_relation_size(schemaname||'.'||relname)) AS total_size,
		pg_size_pretty(pg_relation_size(schemaname||'.'||relname)) AS table_size,
		pg_size_pretty(pg_indexes_size(schemaname||'.'||relname)) AS index_size
	FROM PG_CATALOG.PG_STAT_USER_TABLES
	WHERE schemaname = ? AND relname = ?`
}
