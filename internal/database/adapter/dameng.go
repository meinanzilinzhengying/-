//go:build linux

// Package adapter 提供多数据库类型适配
// 本文件实现达梦数据库 V8+ 适配
package adapter

import (
	"encoding/binary"
	"fmt"
)

// DMConstant 达梦数据库常量
const (
	DMPacketTypeHandshake    = 0x01 // 握手包
	DMPacketTypeLogin        = 0x02 // 登录包
	DMPacketTypeExecute      = 0x08 // 执行包
	DMPacketTypePrepare      = 0x09 // 预处理包
	DMPacketTypeBind         = 0x0A // 绑定变量包
	DMPacketTypeFetch        = 0x0B // 取数据包
	DMPacketTypeResponse     = 0x0C // 响应包
	DMPacketTypeError        = 0x0D // 错误包
	DMPacketTypeDisconnect   = 0x0E // 断开连接包
	DMPacketTypeHeartbeat    = 0x0F // 心跳包
	DMPacketTypeBatchExecute = 0x10 // 批量执行包

	// 达梦数据类型映射
	DMTypeTinyInt   = 1
	DMTypeSmallInt  = 2
	DMTypeInt       = 3
	DMTypeBigInt    = 4
	DMTypeFloat     = 5
	DMTypeDouble    = 6
	DMTypeChar      = 7
	DMTypeVarChar   = 8
	DMTypeVarChar2  = 9
	DMTypeDate      = 10
	DMTypeDateTime  = 11
	DMTypeTime      = 12
	DMTypeTimeStamp = 13
	DMTypeBinary    = 14
	DMTypeVarBinary = 15
	DMTypeBit       = 16
	DMTypeText      = 20
	DMTypeBlob      = 21
	DMTypeClob      = 22
	DMTypeBFile     = 23
	DMTypeCursor    = 24
	DMTypeRowID     = 25
	DMTypeLong      = 26
	DMTypeLongVar   = 27
)

// DMAdapter 达梦数据库适配器
type DMAdapter struct {
	config    *DBConfig
	version   string
	schema    string // 模式名
}

// NewDMAdapter 创建达梦适配器
func NewDMAdapter(config *DBConfig) *DMAdapter {
	return &DMAdapter{
		config: config,
		schema: config.DMSchema,
	}
}

// GetType 返回数据库类型
func (a *DMAdapter) GetType() DBType {
	return DBTypeDM
}

// GetVersion 返回数据库版本
func (a *DMAdapter) GetVersion() string {
	return a.version
}

// SetVersion 设置版本
func (a *DMAdapter) SetVersion(version string) {
	a.version = version
}

// GetFeatures 返回数据库特性
func (a *DMAdapter) GetFeatures() DBFeatures {
	return DBTypeDM.GetFeatures()
}

// GetSystemTables 获取系统表
func (a *DMAdapter) GetSystemTables() []string {
	return []string{
		"SYSDBA.SYSCOLUMNS",     // 系统列
		"SYSDBA.SYSTABLES",      // 系统表
		"SYSDBA.SYSINDEXES",     // 系统索引
		"SYSDBA.SYSVIEWS",       // 系统视图
		"SYSDBA.SYSPRIVILES",    // 系统权限
		"SYSDBA.SYSOBJECTS",     // 系统对象
		"SYSDBA.SYSTRIGGERS",    // 系统触发器
		"SYSDBA.SYSSEQUENCES",   // 系统序列
		"SYSDBA.SYSCONSTRAINTS", // 系统约束
		"SYSDBA.SYSCOLSTATS",    // 列统计
		"SYSDBA.SYSJAVACLASS",   // Java类
		"SYSDBA.V$INSTANCE",      // 实例视图
		"SYSDBA.V$SESSION",       // 会话视图
		"SYSDBA.V$SESSTAT",       // 会话统计
		"SYSDBA.V$SQL",           // SQL视图
		"SYSDBA.V$SQLTEXT",       // SQL文本
		"SYSDBA.V$PLAN",          // 执行计划
		"SYSDBA.V$LOCK",          // 锁视图
		"SYSDBA.V$TEMP tablespace", // 临时表空间
	}
}

// GetSlowQuerySQL 获取慢查询SQL
func (a *DMAdapter) GetSlowQuerySQL() string {
	return `SELECT 
		SESS_ID,
		SQL_TEXT,
		START_TIME,
		EXECUTE_TIME,
		READ_ROW,
		WRITE_ROW,
		TRX_REDO_LOG_SIZE,
		TRX_UNDO_LOG_SIZE
	FROM V$SQLTEXT 
	WHERE EXECUTE_TIME > ? 
	ORDER BY START_TIME DESC`
}

// GetActiveSessionSQL 获取活跃会话SQL
func (a *DMAdapter) GetActiveSessionSQL() string {
	return `SELECT 
		SESS_ID,
		USER_NAME,
		TRX_ID,
		SQL_TEXT,
		STATE,
		START_TIME,
		CLNT_IP,
		CLNT_HOST
	FROM V$SESSION 
	WHERE STATE = 'ACTIVE' 
		AND SESS_ID != (
			SELECT SESS_ID FROM V$SESSION WHERE USER_NAME = 'SYSDBA'
		)`
}

// GetTopSQLSQL 获取TOP SQL SQL
func (a *DMAdapter) GetTopSQLSQL() string {
	return `SELECT 
		SQL_TEXT,
		EXECUTE_TIME,
		EXECUTE_COUNT,
		AVG_TIME,
		DISK_READ_CNT,
		DISK_WRITE_CNT,
		HASH_VALUE
	FROM V$SQLTEXT 
	ORDER BY EXECUTE_TIME DESC 
	LIMIT ?`
}

// GetTableStatsSQL 获取表统计SQL
func (a *DMAdapter) GetTableStatsSQL() string {
	return `SELECT 
		TABLE_NAME,
		ROW_CNT,
		BLK_CNT,
		EMP_CNT,
		AVG_ROW_LEN,
		LAST_ANALYZED
	FROM SYSDBA.SYSTABLES 
	WHERE SCHEMA_NAME = ? AND TABLE_NAME LIKE ?`
}

// GetIndexStatsSQL 获取索引统计SQL
func (a *DMAdapter) GetIndexStatsSQL() string {
	return `SELECT 
		INDEX_NAME,
		TABLE_NAME,
		UNIQUIFIER,
		INDEX_TYPE,
		COL_COUNT,
		BLK_CNT,
		HEIGHT
	FROM SYSDBA.SYSINDEXES 
	WHERE SCHEMA_NAME = ?`
}

// GetConnectionInfoSQL 获取连接信息SQL
func (a *DMAdapter) GetConnectionInfoSQL() string {
	return `SELECT 
		SESS_ID,
		USER_NAME,
		CLNT_IP,
		CLNT_TYPE,
		TRX_ID,
		START_TIME,
		STATE
	FROM V$SESSION`
}

// GetLockInfoSQL 获取锁信息SQL
func (a *DMAdapter) GetLockInfoSQL() string {
	return `SELECT 
		L.TRX_ID,
		L.OBJ_ID,
		L.BLOCKED,
		L.BLOCKED_BY,
		T.SESS_ID,
		T.USER_NAME,
		T.SQL_TEXT
	FROM V$LOCK L
	LEFT JOIN V$TRX T ON L.TRX_ID = T.ID
	WHERE L.BLOCKED = 1`
}

// ParsePacket 解析达梦数据包
func (a *DMAdapter) ParsePacket(data []byte) (*DMPacket, error) {
	if len(data) < 9 {
		return nil, fmt.Errorf("数据包长度不足: %d", len(data))
	}

	packet := &DMPacket{
		Length:    binary.LittleEndian.Uint32(data[0:4]),
		Sequence: data[4],
		Type:     data[5],
	}

	// 解析数据
	packet.Data = data[6:]

	return packet, nil
}

// DMPacket 达梦数据包
type DMPacket struct {
	Length    uint32
	Sequence  byte
	Type      byte
	Data      []byte
}

// NewHandshakePacket 创建握手包
func (a *DMAdapter) NewHandshakePacket() []byte {
	// 达梦握手协议
	packet := make([]byte, 22)
	packet[0] = 0x00 // 包长度(占位)
	packet[1] = 0x00
	packet[2] = 0x00
	packet[3] = 0x00
	packet[4] = 0x01 // 序列号
	packet[5] = DMPacketTypeHandshake
	packet[6] = 0x00 // 协议版本
	packet[7] = 0x00
	packet[8] = 0x0D // 达梦版本号长度

	// 填充握手数据
	copy(packet[9:], []byte("DM 8"))

	return packet
}

// FormatSQL 格式化SQL语句
func (a *DMAdapter) FormatSQL(sql string) string {
	// 达梦特定的SQL格式化
	return sql
}

// ConvertType 转换数据类型
func (a *DMAdapter) ConvertType(dmType int) string {
	switch dmType {
	case DMTypeTinyInt:
		return "TINYINT"
	case DMTypeSmallInt:
		return "SMALLINT"
	case DMTypeInt:
		return "INT"
	case DMTypeBigInt:
		return "BIGINT"
	case DMTypeFloat:
		return "FLOAT"
	case DMTypeDouble:
		return "DOUBLE"
	case DMTypeChar:
		return "CHAR"
	case DMTypeVarChar:
		return "VARCHAR"
	case DMTypeVarChar2:
		return "VARCHAR2"
	case DMTypeDate:
		return "DATE"
	case DMTypeDateTime:
		return "DATETIME"
	case DMTypeTime:
		return "TIME"
	case DMTypeTimeStamp:
		return "TIMESTAMP"
	case DMTypeBinary:
		return "BINARY"
	case DMTypeVarBinary:
		return "VARBINARY"
	case DMTypeBit:
		return "BIT"
	case DMTypeText:
		return "TEXT"
	case DMTypeBlob:
		return "BLOB"
	case DMTypeClob:
		return "CLOB"
	default:
		return "UNKNOWN"
	}
}

// GetExplainSQL 获取执行计划SQL
func (a *DMAdapter) GetExplainSQL(sql string) string {
	return fmt.Sprintf("EXPLAIN %s", sql)
}

// GetKillSessionSQL 生成终止会话SQL
func (a *DMAdapter) GetKillSessionSQL(sessID string) string {
	return fmt.Sprintf("SP_CLOSE_SESSION('%s')", sessID)
}

// GetProcessListSQL 获取进程列表SQL
func (a *DMAdapter) GetProcessListSQL() string {
	return `SELECT 
		SESS_ID,
		USER_NAME,
		CLNT_IP,
		TRX_ID,
		SQL_TEXT
	FROM V$SESSION 
	WHERE STATE != 'IDLE'`
}

// GetVersionSQL 获取版本SQL
func (a *DMAdapter) GetVersionSQL() string {
	return "SELECT * FROM V$VERSION"
}

// GetCurrentSessionSQL 获取当前会话SQL
func (a *DMAdapter) GetCurrentSessionSQL() string {
	return "SELECT SESS_ID, USER_NAME FROM V$SESSION WHERE SESS_ID = SF_GET_SESSION_ID()"
}
