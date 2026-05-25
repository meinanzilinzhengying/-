//go:build linux

// Package adapter 提供多数据库类型适配
// 本文件实现数据库适配器工厂和运行时切换
package adapter

import (
	"fmt"
	"sync"

	"cloud-flow-agent/pkg/logger"
)

// DBAdapter 数据库适配器接口
type DBAdapter interface {
	// 类型信息
	GetType() DBType
	GetVersion() string
	SetVersion(version string)
	GetFeatures() DBFeatures

	// 系统表
	GetSystemTables() []string

	// 监控SQL
	GetSlowQuerySQL() string
	GetActiveSessionSQL() string
	GetTopSQLSQL() string
	GetTableStatsSQL() string
	GetIndexStatsSQL() string
	GetConnectionInfoSQL() string
	GetLockInfoSQL() string

	// 工具SQL
	GetVersionSQL() string
	GetCurrentSessionSQL() string
	GetKillSessionSQL(pid interface{}) string
	GetProcessListSQL() string

	// SQL处理
	FormatSQL(sql string) string
	GetExplainSQL(sql string) string
}

// Factory 数据库适配器工厂
type Factory struct {
	mu       sync.RWMutex
	configs  map[DBType]*DBConfig
	adapters map[DBType]DBAdapter
	log      *logger.Logger
}

// NewFactory 创建工厂
func NewFactory(log *logger.Logger) *Factory {
	return &Factory{
		configs:  make(map[DBType]*DBConfig),
		adapters: make(map[DBType]DBAdapter),
		log:      log,
	}
}

// Register 注册数据库配置
func (f *Factory) Register(dbType DBType, config *DBConfig) error {
	if err := config.Validate(); err != nil {
		return fmt.Errorf("配置验证失败 [%s]: %w", dbType, err)
	}

	f.mu.Lock()
	defer f.mu.Unlock()

	f.configs[dbType] = config

	// 创建适配器
	adapter := f.createAdapter(dbType, config)
	f.adapters[dbType] = adapter

	f.log.Infof("注册数据库适配器: %s", dbType)
	return nil
}

// Unregister 注销数据库配置
func (f *Factory) Unregister(dbType DBType) {
	f.mu.Lock()
	defer f.mu.Unlock()

	delete(f.configs, dbType)
	delete(f.adapters, dbType)

	f.log.Infof("注销数据库适配器: %s", dbType)
}

// GetAdapter 获取适配器
func (f *Factory) GetAdapter(dbType DBType) (DBAdapter, error) {
	f.mu.RLock()
	defer f.mu.RUnlock()

	adapter, exists := f.adapters[dbType]
	if !exists {
		return nil, fmt.Errorf("未注册的数据库类型: %s", dbType)
	}

	return adapter, nil
}

// GetConfig 获取配置
func (f *Factory) GetConfig(dbType DBType) (*DBConfig, bool) {
	f.mu.RLock()
	defer f.mu.RUnlock()

	config, exists := f.configs[dbType]
	return config, exists
}

// ListTypes 列出所有已注册的类型
func (f *Factory) ListTypes() []DBType {
	f.mu.RLock()
	defer f.mu.RUnlock()

	types := make([]DBType, 0, len(f.configs))
	for dbType := range f.configs {
		types = append(types, dbType)
	}
	return types
}

// SwitchTo 切换到指定数据库类型
func (f *Factory) SwitchTo(dbType DBType) (DBAdapter, error) {
	f.mu.RLock()
	adapter, exists := f.adapters[dbType]
	config, configExists := f.configs[dbType]
	f.mu.RUnlock()

	if !exists || !configExists {
		return nil, fmt.Errorf("未注册的数据库类型: %s", dbType)
	}

	// 验证连接
	// TODO: 实现连接测试

	f.log.Infof("切换到数据库: %s", dbType)
	return adapter, nil
}

// createAdapter 创建适配器
func (f *Factory) createAdapter(dbType DBType, config *DBConfig) DBAdapter {
	switch dbType {
	case DBTypeDM:
		return NewDMAdapter(config)
	case DBTypeGaussDB:
		return NewGaussDBAdapter(config)
	case DBTypeKingBase:
		return NewKingBaseAdapter(config)
	case DBTypeTiDB:
		return NewTiDBAdapter(config)
	case DBTypeMySQL:
		return NewMySQLAdapter(config)
	case DBTypePostgreSQL:
		return NewPostgreSQLAdapter(config)
	case DBTypeOceanBase:
		return NewOceanBaseAdapter(config)
	case DBTypeHighGo:
		return NewHighGoAdapter(config)
	default:
		// 返回TiDB作为默认
		return NewTiDBAdapter(config)
	}
}

// GetDefaultSQL 获取指定类型的默认SQL
func GetDefaultSQL(dbType DBType, sqlType string) string {
	switch sqlType {
	case "slow_query":
		return getDefaultSlowQuerySQL(dbType)
	case "active_session":
		return getDefaultActiveSessionSQL(dbType)
	case "top_sql":
		return getDefaultTopSQLSQL(dbType)
	case "table_stats":
		return getDefaultTableStatsSQL(dbType)
	case "index_stats":
		return getDefaultIndexStatsSQL(dbType)
	case "connection_info":
		return getDefaultConnectionInfoSQL(dbType)
	case "lock_info":
		return getDefaultLockInfoSQL(dbType)
	case "version":
		return getDefaultVersionSQL(dbType)
	default:
		return ""
	}
}

func getDefaultSlowQuerySQL(dbType DBType) string {
	switch dbType {
	case DBTypeDM:
		return "SELECT * FROM V$SQLTEXT WHERE EXECUTE_TIME > ?"
	case DBTypeGaussDB:
		return "SELECT * FROM PG_CATALOG.PG_SLOW_QUERY_INFO"
	case DBTypeKingBase:
		return "SELECT * FROM SYS_CATALOG.PG_STAT_ACTIVITY"
	case DBTypeTiDB:
		return "SELECT * FROM INFORMATION_SCHEMA.CLUSTER_SLOW_QUERY"
	case DBTypeMySQL:
		return "SELECT * FROM INFORMATION_SCHEMA.PROFILING"
	default:
		return "SELECT 1"
	}
}

func getDefaultActiveSessionSQL(dbType DBType) string {
	switch dbType {
	case DBTypeDM:
		return "SELECT * FROM V$SESSION"
	case DBTypeGaussDB:
		return "SELECT * FROM PG_CATALOG.PG_STAT_ACTIVITY"
	case DBTypeKingBase:
		return "SELECT * FROM SYS_CATALOG.PG_STAT_ACTIVITY"
	case DBTypeTiDB:
		return "SELECT * FROM INFORMATION_SCHEMA.CLUSTER_PROCESSLIST"
	case DBTypeMySQL:
		return "SHOW PROCESSLIST"
	default:
		return "SELECT 1"
	}
}

func getDefaultTopSQLSQL(dbType DBType) string {
	switch dbType {
	case DBTypeDM:
		return "SELECT * FROM V$SQLTEXT ORDER BY EXECUTE_TIME DESC"
	case DBTypeGaussDB:
		return "SELECT * FROM PG_CATALOG.PG_SLOW_QUERY_INFO ORDER BY TOTAL_TIME DESC"
	case DBTypeKingBase:
		return "SELECT * FROM SYS_CATALOG.PG_STAT_STATEMENTS ORDER BY TOTAL_EXEC_TIME DESC"
	case DBTypeTiDB:
		return "SELECT * FROM INFORMATION_SCHEMA.CLUSTER_SLOW_QUERY ORDER BY QUERY_TIME DESC"
	default:
		return "SELECT 1"
	}
}

func getDefaultTableStatsSQL(dbType DBType) string {
	switch dbType {
	case DBTypeDM:
		return "SELECT * FROM SYSDBA.SYSTABLES"
	case DBTypeGaussDB:
		return "SELECT * FROM PG_CATALOG.PG_STAT_USER_TABLES"
	case DBTypeKingBase:
		return "SELECT * FROM SYS_CATALOG.PG_STAT_USER_TABLES"
	case DBTypeTiDB:
		return "SELECT * FROM INFORMATION_SCHEMA.TIKV_REGION_STATUS"
	default:
		return "SELECT 1"
	}
}

func getDefaultIndexStatsSQL(dbType DBType) string {
	switch dbType {
	case DBTypeDM:
		return "SELECT * FROM SYSDBA.SYSINDEXES"
	case DBTypeGaussDB:
		return "SELECT * FROM PG_CATALOG.PG_STAT_USER_INDEXES"
	case DBTypeKingBase:
		return "SELECT * FROM SYS_CATALOG.PG_STAT_USER_INDEXES"
	default:
		return "SELECT 1"
	}
}

func getDefaultConnectionInfoSQL(dbType DBType) string {
	switch dbType {
	case DBTypeDM:
		return "SELECT * FROM V$SESSION"
	case DBTypeGaussDB:
		return "SELECT * FROM GS_FREE_CONNECTION"
	case DBTypeKingBase:
		return "SELECT * FROM SYS_CATALOG.PG_STAT_ACTIVITY"
	default:
		return "SELECT 1"
	}
}

func getDefaultLockInfoSQL(dbType DBType) string {
	switch dbType {
	case DBTypeDM:
		return "SELECT * FROM V$LOCK"
	case DBTypeGaussDB:
		return "SELECT * FROM PG_CATALOG.PG_LOCKS"
	case DBTypeKingBase:
		return "SELECT * FROM SYS_CATALOG.PG_LOCKS"
	case DBTypeTiDB:
		return "SELECT * FROM INFORMATION_SCHEMA.CLUSTER_LOCK_WAIT"
	default:
		return "SELECT 1"
	}
}

func getDefaultVersionSQL(dbType DBType) string {
	switch dbType {
	case DBTypeDM:
		return "SELECT * FROM V$VERSION"
	case DBTypeGaussDB:
		return "SELECT GS_VERSION()"
	case DBTypeKingBase:
		return "SELECT SYS_VERSION()"
	case DBTypeTiDB:
		return "SELECT * FROM INFORMATION_SCHEMA.CLUSTER_INFO"
	default:
		return "SELECT VERSION()"
	}
}

// ==================== 简化适配器（用于未完整实现的类型）====================

// TiDBAdapter TiDB适配器
type TiDBAdapter struct {
	config *DBConfig
}

func NewTiDBAdapter(config *DBConfig) *TiDBAdapter {
	return &TiDBAdapter{config: config}
}

func (a *TiDBAdapter) GetType() DBType                        { return DBTypeTiDB }
func (a *TiDBAdapter) SetVersion(version string)              {}
func (a *TiDBAdapter) GetVersion() string                     { return "" }
func (a *TiDBAdapter) GetFeatures() DBFeatures                { return DBTypeTiDB.GetFeatures() }
func (a *TiDBAdapter) GetSystemTables() []string             { return nil }
func (a *TiDBAdapter) GetSlowQuerySQL() string               { return "" }
func (a *TiDBAdapter) GetActiveSessionSQL() string            { return "SHOW PROCESSLIST" }
func (a *TiDBAdapter) GetTopSQLSQL() string                 { return "" }
func (a *TiDBAdapter) GetTableStatsSQL() string              { return "" }
func (a *TiDBAdapter) GetIndexStatsSQL() string              { return "" }
func (a *TiDBAdapter) GetConnectionInfoSQL() string          { return "SHOW PROCESSLIST" }
func (a *TiDBAdapter) GetLockInfoSQL() string               { return "" }
func (a *TiDBAdapter) GetVersionSQL() string                 { return "SELECT VERSION()" }
func (a *TiDBAdapter) GetCurrentSessionSQL() string          { return "" }
func (a *TiDBAdapter) GetKillSessionSQL(pid interface{}) string { return "" }
func (a *TiDBAdapter) GetProcessListSQL() string             { return "SHOW PROCESSLIST" }
func (a *TiDBAdapter) FormatSQL(sql string) string            { return sql }
func (a *TiDBAdapter) GetExplainSQL(sql string) string        { return "EXPLAIN " + sql }

// MySQLAdapter MySQL适配器
type MySQLAdapter struct {
	config *DBConfig
}

func NewMySQLAdapter(config *DBConfig) *MySQLAdapter {
	return &MySQLAdapter{config: config}
}

func (a *MySQLAdapter) GetType() DBType                        { return DBTypeMySQL }
func (a *MySQLAdapter) SetVersion(version string)              {}
func (a *MySQLAdapter) GetVersion() string                     { return "" }
func (a *MySQLAdapter) GetFeatures() DBFeatures                { return DBTypeMySQL.GetFeatures() }
func (a *MySQLAdapter) GetSystemTables() []string             { return nil }
func (a *MySQLAdapter) GetSlowQuerySQL() string               { return "SHOW FULL PROCESSLIST" }
func (a *MySQLAdapter) GetActiveSessionSQL() string            { return "SHOW PROCESSLIST" }
func (a *MySQLAdapter) GetTopSQLSQL() string                 { return "" }
func (a *MySQLAdapter) GetTableStatsSQL() string              { return "" }
func (a *MySQLAdapter) GetIndexStatsSQL() string              { return "" }
func (a *MySQLAdapter) GetConnectionInfoSQL() string          { return "SHOW PROCESSLIST" }
func (a *MySQLAdapter) GetLockInfoSQL() string               { return "" }
func (a *MySQLAdapter) GetVersionSQL() string                 { return "SELECT VERSION()" }
func (a *MySQLAdapter) GetCurrentSessionSQL() string          { return "" }
func (a *MySQLAdapter) GetKillSessionSQL(pid interface{}) string { return fmt.Sprintf("KILL %v", pid) }
func (a *MySQLAdapter) GetProcessListSQL() string             { return "SHOW PROCESSLIST" }
func (a *MySQLAdapter) FormatSQL(sql string) string            { return sql }
func (a *MySQLAdapter) GetExplainSQL(sql string) string        { return "EXPLAIN " + sql }

// PostgreSQLAdapter PostgreSQL适配器
type PostgreSQLAdapter struct {
	config *DBConfig
}

func NewPostgreSQLAdapter(config *DBConfig) *PostgreSQLAdapter {
	return &PostgreSQLAdapter{config: config}
}

func (a *PostgreSQLAdapter) GetType() DBType                        { return DBTypePostgreSQL }
func (a *PostgreSQLAdapter) SetVersion(version string)              {}
func (a *PostgreSQLAdapter) GetVersion() string                     { return "" }
func (a *PostgreSQLAdapter) GetFeatures() DBFeatures                { return DBTypePostgreSQL.GetFeatures() }
func (a *PostgreSQLAdapter) GetSystemTables() []string             { return nil }
func (a *PostgreSQLAdapter) GetSlowQuerySQL() string               { return "" }
func (a *PostgreSQLAdapter) GetActiveSessionSQL() string            { return "SELECT * FROM PG_STAT_ACTIVITY" }
func (a *PostgreSQLAdapter) GetTopSQLSQL() string                 { return "" }
func (a *PostgreSQLAdapter) GetTableStatsSQL() string              { return "" }
func (a *PostgreSQLAdapter) GetIndexStatsSQL() string              { return "" }
func (a *PostgreSQLAdapter) GetConnectionInfoSQL() string          { return "SELECT * FROM PG_STAT_ACTIVITY" }
func (a *PostgreSQLAdapter) GetLockInfoSQL() string               { return "SELECT * FROM PG_LOCKS" }
func (a *PostgreSQLAdapter) GetVersionSQL() string                 { return "SELECT VERSION()" }
func (a *PostgreSQLAdapter) GetCurrentSessionSQL() string          { return "SELECT PG_BACKEND_PID()" }
func (a *PostgreSQLAdapter) GetKillSessionSQL(pid interface{}) string { return fmt.Sprintf("SELECT PG_CANCEL_BACKEND(%v)", pid) }
func (a *PostgreSQLAdapter) GetProcessListSQL() string             { return "SELECT * FROM PG_STAT_ACTIVITY" }
func (a *PostgreSQLAdapter) FormatSQL(sql string) string            { return sql }
func (a *PostgreSQLAdapter) GetExplainSQL(sql string) string        { return "EXPLAIN " + sql }

// OceanBaseAdapter OceanBase适配器
type OceanBaseAdapter struct {
	config *DBConfig
}

func NewOceanBaseAdapter(config *DBConfig) *OceanBaseAdapter {
	return &OceanBaseAdapter{config: config}
}

func (a *OceanBaseAdapter) GetType() DBType                        { return DBTypeOceanBase }
func (a *OceanBaseAdapter) SetVersion(version string)              {}
func (a *OceanBaseAdapter) GetVersion() string                     { return "" }
func (a *OceanBaseAdapter) GetFeatures() DBFeatures                { return DBTypeOceanBase.GetFeatures() }
func (a *OceanBaseAdapter) GetSystemTables() []string             { return nil }
func (a *OceanBaseAdapter) GetSlowQuerySQL() string               { return "" }
func (a *OceanBaseAdapter) GetActiveSessionSQL() string            { return "" }
func (a *OceanBaseAdapter) GetTopSQLSQL() string                 { return "" }
func (a *OceanBaseAdapter) GetTableStatsSQL() string              { return "" }
func (a *OceanBaseAdapter) GetIndexStatsSQL() string              { return "" }
func (a *OceanBaseAdapter) GetConnectionInfoSQL() string          { return "" }
func (a *OceanBaseAdapter) GetLockInfoSQL() string               { return "" }
func (a *OceanBaseAdapter) GetVersionSQL() string                 { return "" }
func (a *OceanBaseAdapter) GetCurrentSessionSQL() string          { return "" }
func (a *OceanBaseAdapter) GetKillSessionSQL(pid interface{}) string { return "" }
func (a *OceanBaseAdapter) GetProcessListSQL() string             { return "" }
func (a *OceanBaseAdapter) FormatSQL(sql string) string            { return sql }
func (a *OceanBaseAdapter) GetExplainSQL(sql string) string        { return "EXPLAIN " + sql }

// HighGoAdapter HighGo适配器
type HighGoAdapter struct {
	config *DBConfig
}

func NewHighGoAdapter(config *DBConfig) *HighGoAdapter {
	return &HighGoAdapter{config: config}
}

func (a *HighGoAdapter) GetType() DBType                        { return DBTypeHighGo }
func (a *HighGoAdapter) SetVersion(version string)              {}
func (a *HighGoAdapter) GetVersion() string                     { return "" }
func (a *HighGoAdapter) GetFeatures() DBFeatures                { return DBTypeHighGo.GetFeatures() }
func (a *HighGoAdapter) GetSystemTables() []string             { return nil }
func (a *HighGoAdapter) GetSlowQuerySQL() string               { return "" }
func (a *HighGoAdapter) GetActiveSessionSQL() string            { return "" }
func (a *HighGoAdapter) GetTopSQLSQL() string                 { return "" }
func (a *HighGoAdapter) GetTableStatsSQL() string              { return "" }
func (a *HighGoAdapter) GetIndexStatsSQL() string              { return "" }
func (a *HighGoAdapter) GetConnectionInfoSQL() string          { return "" }
func (a *HighGoAdapter) GetLockInfoSQL() string               { return "" }
func (a *HighGoAdapter) GetVersionSQL() string                 { return "" }
func (a *HighGoAdapter) GetCurrentSessionSQL() string          { return "" }
func (a *HighGoAdapter) GetKillSessionSQL(pid interface{}) string { return "" }
func (a *HighGoAdapter) GetProcessListSQL() string             { return "" }
func (a *HighGoAdapter) FormatSQL(sql string) string            { return sql }
func (a *HighGoAdapter) GetExplainSQL(sql string) string        { return "EXPLAIN " + sql }
