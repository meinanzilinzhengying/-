//go:build linux

// Package adapter 提供多数据库类型适配
// 本文件实现KingBase (金仓) V3+ 适配
package adapter

import (
	"fmt"
)

// KingBaseAdapter KingBase数据库适配器
type KingBaseAdapter struct {
	config  *DBConfig
	version string
	schema  string // 模式名
}

// NewKingBaseAdapter 创建KingBase适配器
func NewKingBaseAdapter(config *DBConfig) *KingBaseAdapter {
	return &KingBaseAdapter{
		config: config,
		schema: "PUBLIC", // 默认PUBLIC模式
	}
}

// GetType 返回数据库类型
func (a *KingBaseAdapter) GetType() DBType {
	return DBTypeKingBase
}

// GetVersion 返回数据库版本
func (a *KingBaseAdapter) GetVersion() string {
	return a.version
}

// SetVersion 设置版本
func (a *KingBaseAdapter) SetVersion(version string) {
	a.version = version
}

// GetFeatures 返回数据库特性
func (a *KingBaseAdapter) GetFeatures() DBFeatures {
	return DBTypeKingBase.GetFeatures()
}

// GetSystemTables 获取系统表
func (a *KingBaseAdapter) GetSystemTables() []string {
	return []string{
		"SYS_CATALOG.PG_CLASS",         // 类信息
		"SYS_CATALOG.PG_ATTRIBUTE",     // 属性信息
		"SYS_CATALOG.PG_PROC",         // 过程/函数信息
		"SYS_CATALOG.PG_TYPE",          // 类型信息
		"SYS_CATALOG.PG_NAMESPACE",     // 命名空间
		"SYS_CATALOG.PG_INDEX",        // 索引信息
		"SYS_CATALOG.PG_CONSTRAINT",   // 约束信息
		"SYS_CATALOG.PG_STAT_ACTIVITY", // 会话/进程信息
		"SYS_CATALOG.PG_STAT_USER_TABLES", // 用户表统计
		"SYS_CATALOG.PG_LOCKS",        // 锁信息
		"SYS_CATALOG.PG_TABLES",       // 表信息
		"SYS_CATALOG.PG_VIEWS",        // 视图信息
		"SYS_CATALOG.PG_INDEXES",      // 索引信息
		"SYS_CATALOG.PG_SEQUENCES",    // 序列信息
		"SYS_CATALOG.PG_TRIGGERS",     // 触发器信息
		"SYS_CATALOG.PG_CONSTRAINT_TABLE", // 约束表信息
		"INFORMATION_SCHEMA.TABLES",    // 信息schema表
		"INFORMATION_SCHEMA.COLUMNS",   // 信息schema列
		"INFORMATION_SCHEMA.VIEWS",     // 信息schema视图
		"INFORMATION_SCHEMA.ROUTINES",   // 信息schema例程
	}
}

// GetSlowQuerySQL 获取慢查询SQL
func (a *KingBaseAdapter) GetSlowQuerySQL() string {
	return `SELECT 
		application_name,
		state,
		query,
		usesysid,
		usename,
		backend_xid,
		backend_xmin,
		query_start,
		backend_start,
		ssl,
		sslcompression,
		client_addr,
		client_hostname,
		client_port,
		backend_id,
		backend_type
	FROM SYS_CATALOG.PG_STAT_ACTIVITY
	WHERE state != 'idle'
		AND state != 'idle in transaction'
		AND query_start < NOW() - INTERVAL '? seconds'
	ORDER BY query_start`
}

// GetActiveSessionSQL 获取活跃会话SQL
func (a *KingBaseAdapter) GetActiveSessionSQL() string {
	return `SELECT 
		sa.pid,
		sa.usesysid,
		sa.usename,
		sa.application_name,
		sa.client_addr,
		sa.client_hostname,
		sa.client_port,
		sa.backend_start,
		sa.backend_xid,
		sa.backend_xmin,
		sa.state,
		sa.query,
		sa.query_start,
		sa.state_change,
		sa.wait_event_type,
		sa.wait_event,
		sa.backend_type
	FROM SYS_CATALOG.PG_STAT_ACTIVITY sa
	WHERE sa.state = 'active'
		AND sa.usename NOT IN ('sys_monitor', 'sys_backup')
	ORDER BY sa.query_start`
}

// GetTopSQLSQL 获取TOP SQL SQL
func (a *KingBaseAdapter) GetTopSQLSQL() string {
	return `SELECT 
		queryid,
		query,
		calls,
		total_exec_time,
		min_exec_time,
		max_exec_time,
		mean_exec_time,
		stddev_exec_time,
		rows
	FROM SYS_CATALOG.PG_STAT_STATEMENTS
	WHERE query LIKE '%%'
	ORDER BY total_exec_time DESC
	LIMIT ?`
}

// GetTableStatsSQL 获取表统计SQL
func (a *KingBaseAdapter) GetTableStatsSQL() string {
	return `SELECT 
		schemaname,
		relname,
		n_live_tup,
		n_dead_tup,
		n_mod_since_analyze,
		n_ins_since_vacuum,
		last_vacuum,
		last_autovacuum,
		last_analyze,
		last_autoanalyze,
		vacuum_count,
		autovacuum_count,
		analyze_count,
		autoanalyze_count,
		heap_blks_read,
		heap_blks_hit,
		idx_blks_read,
		idx_blks_hit,
		toast_blks_read,
		toast_blks_hit,
		tidx_blks_read,
		tidx_blks_hit
	FROM SYS_CATALOG.PG_STAT_USER_TABLES
	WHERE schemaname = ?
		AND relname LIKE ?
	ORDER BY n_live_tup DESC`
}

// GetIndexStatsSQL 获取索引统计SQL
func (a *KingBaseAdapter) GetIndexStatsSQL() string {
	return `SELECT 
		schemaname,
		relname,
		indexrelname,
		idx_scan,
		idx_tup_read,
		idx_tup_fetch,
		idx_blks_read,
		idx_blks_hit
	FROM SYS_CATALOG.PG_STAT_USER_INDEXES
	WHERE schemaname = ?
	ORDER BY idx_scan DESC`
}

// GetConnectionInfoSQL 获取连接信息SQL
func (a *KingBaseAdapter) GetConnectionInfoSQL() string {
	return `SELECT 
		sa.pid,
		sa.usename,
		sa.application_name,
		sa.client_addr,
		sa.client_hostname,
		sa.client_port,
		sa.backend_start,
		sa.state,
		sa.query
	FROM SYS_CATALOG.PG_STAT_ACTIVITY sa
	WHERE sa.state != 'idle'
	ORDER BY sa.backend_start`
}

// GetLockInfoSQL 获取锁信息SQL
func (a *KingBaseAdapter) GetLockInfoSQL() string {
	return `SELECT 
		l.locktype,
		l.relation,
		l.page,
		l.tuple,
		l.virtualxid,
		l.transactionid,
		l.classid,
		l.objid,
		l.objsubid,
		l.database,
		l.pid,
		l.mode,
		l.granted,
		l.fastpath,
		l.virtualtransaction,
		l.virtualxid
	FROM SYS_CATALOG.PG_LOCKS l
	WHERE l.granted = false
	ORDER BY l.transactionid`
}

// FormatSQL 格式化SQL语句
func (a *KingBaseAdapter) FormatSQL(sql string) string {
	// KingBase特定的SQL格式化
	return sql
}

// GetExplainSQL 获取执行计划SQL
func (a *KingBaseAdapter) GetExplainSQL(sql string) string {
	return fmt.Sprintf("EXPLAIN %s", sql)
}

// GetAnalyzeSQL 获取分析SQL
func (a *KingBaseAdapter) GetAnalyzeSQL(tableName string) string {
	return fmt.Sprintf("ANALYZE %s", tableName)
}

// GetVacuumSQL 获取清理SQL
func (a *KingBaseAdapter) GetVacuumSQL(tableName string) string {
	return fmt.Sprintf("VACUUM %s", tableName)
}

// GetKillSessionSQL 生成终止会话SQL
func (a *KingBaseAdapter) GetKillSessionSQL(pid int) string {
	return fmt.Sprintf("SELECT SYS_CATALOG.PG_CANCEL_BACKEND(%d)", pid)
}

// GetTerminateSessionSQL 生成终止会话SQL
func (a *KingBaseAdapter) GetTerminateSessionSQL(pid int) string {
	return fmt.Sprintf("SELECT SYS_CATALOG.PG_TERMINATE_BACKEND(%d)", pid)
}

// GetProcessListSQL 获取进程列表SQL
func (a *KingBaseAdapter) GetProcessListSQL() string {
	return `SELECT 
		sa.pid,
		sa.usename,
		sa.application_name,
		sa.client_addr,
		sa.state,
		sa.query
	FROM SYS_CATALOG.PG_STAT_ACTIVITY sa
	WHERE sa.state != 'idle'
	ORDER BY sa.query_start`
}

// GetVersionSQL 获取版本SQL
func (a *KingBaseAdapter) GetVersionSQL() string {
	return "SELECT SYS_VERSION()"
}

// GetCurrentSessionSQL 获取当前会话SQL
func (a *KingBaseAdapter) GetCurrentSessionSQL() string {
	return "SELECT PG_BACKEND_PID()"
}

// GetDatabaseSizeSQL 获取数据库大小SQL
func (a *KingBaseAdapter) GetDatabaseSizeSQL() string {
	return `SELECT 
		datname,
		SYS_CATALOG.PG_SIZE_PRETTY(SYS_CATALOG.PG_DATABASE_SIZE(datname)) AS size
	FROM SYS_CATALOG.PG_DATABASE
	WHERE datname = current_database()`
}

// GetTableSizeSQL 获取表大小SQL
func (a *KingBaseAdapter) GetTableSizeSQL() string {
	return `SELECT 
		SCHEMANAME,
		RELNAME,
		SYS_CATALOG.PG_SIZE_PRETTY(SYS_CATALOG.PG_RELATION_SIZE(SCHEMANAME||'.'||RELNAME)) AS size,
		SYS_CATALOG.PG_RELATION_SIZE(SCHEMANAME||'.'||RELNAME) AS size_bytes
	FROM SYS_CATALOG.PG_TABLES
	WHERE SCHEMANAME = ? AND TABLENAME = ?`
}

// GetIndexSizeSQL 获取索引大小SQL
func (a *KingBaseAdapter) GetIndexSizeSQL() string {
	return `SELECT 
		schemaname,
		indexrelname,
		SYS_CATALOG.PG_SIZE_PRETTY(SYS_CATALOG.PG_RELATION_SIZE(schemaname||'.'||indexrelname)) AS size
	FROM SYS_CATALOG.PG_STAT_USER_INDEXES
	WHERE schemaname = ?
	ORDER BY SYS_CATALOG.PG_RELATION_SIZE(schemaname||'.'||indexrelname) DESC`
}

// GetSequenceSQL 获取序列SQL
func (a *KingBaseAdapter) GetSequenceSQL() string {
	return `SELECT 
		sequence_schema,
		sequence_name,
		data_type
	FROM INFORMATION_SCHEMA.SEQUENCES
	WHERE SEQUENCE_SCHEMA = ?`
}

// GetTriggerSQL 获取触发器SQL
func (a *KingBaseAdapter) GetTriggerSQL() string {
	return `SELECT 
		t.trigger_name,
		t.event_manipulation,
		t.event_object_schema,
		t.event_object_name,
		t.action_order,
		t.action_condition,
		t.action_statement,
		t.action_orientation,
		t.action_timing,
		t.action_reference_old_table,
		t.action_reference_new_table,
		t.created
	FROM INFORMATION_SCHEMA.TRIGGERS t
	WHERE t.EVENT_OBJECT_SCHEMA = ?`
}

// GetViewSQL 获取视图SQL
func (a *KingBaseAdapter) GetViewSQL() string {
	return `SELECT 
		TABLE_SCHEMA,
		TABLE_NAME,
		VIEW_DEFINITION
	FROM INFORMATION_SCHEMA.VIEWS
	WHERE TABLE_SCHEMA = ?`
}

// GetRoutineSQL 获取存储过程/函数SQL
func (a *KingBaseAdapter) GetRoutineSQL() string {
	return `SELECT 
		ROUTINE_SCHEMA,
		ROUTINE_NAME,
		ROUTINE_TYPE,
		DATA_TYPE,
		ROUTINE_DEFINITION
	FROM INFORMATION_SCHEMA.ROUTINES
	WHERE ROUTINE_SCHEMA = ?`
}
