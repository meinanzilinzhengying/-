//go:build linux

// Package adapter 提供多数据库类型适配
// 本文件定义数据库类型枚举和通用配置
package adapter

import (
	"fmt"
	"strings"
)

// DBType 数据库类型
type DBType int

const (
	DBTypeUnknown DBType = iota
	DBTypeTiDB          // TiDB (MySQL兼容)
	DBTypeMySQL         // MySQL
	DBTypePostgreSQL    // PostgreSQL
	DBTypeDM             // 达梦数据库 V8+
	DBTypeGaussDB        // GaussDB 200/300
	DBTypeKingBase       // KingWow (金仓) V3+
	DBTypeOceanBase      // OceanBase
	DBTypeHighGo         // HighGo (瀚高)
	DBTypeMaxScale       // MaxScale
)

// String 返回数据库类型字符串
func (t DBType) String() string {
	switch t {
	case DBTypeTiDB:
		return "tidb"
	case DBTypeMySQL:
		return "mysql"
	case DBTypePostgreSQL:
		return "postgresql"
	case DBTypeDM:
		return "dameng"
	case DBTypeGaussDB:
		return "gaussdb"
	case DBTypeKingBase:
		return "kingbase"
	case DBTypeOceanBase:
		return "oceanbase"
	case DBTypeHighGo:
		return "highgo"
	case DBTypeMaxScale:
		return "maxscale"
	default:
		return "unknown"
	}
}

// ParseDBType 解析数据库类型字符串
func ParseDBType(s string) DBType {
	s = strings.ToLower(strings.TrimSpace(s))

	switch s {
	case "tidb", "TiDB":
		return DBTypeTiDB
	case "mysql", "MySQL":
		return DBTypeMySQL
	case "postgresql", "postgres", "pg":
		return DBTypePostgreSQL
	case "dm", "dameng", "达梦":
		return DBTypeDM
	case "gaussdb", "gauss", "gaussdb200", "gaussdb300":
		return DBTypeGaussDB
	case "kingbase", "kingwow", "kingwowv3", "金仓":
		return DBTypeKingBase
	case "oceanbase", "ob":
		return DBTypeOceanBase
	case "highgo", "highgoV5", "瀚高":
		return DBTypeHighGo
	case "maxscale":
		return DBTypeMaxScale
	default:
		return DBTypeUnknown
	}
}

// IsCompatibleWithMySQL 检查是否兼容MySQL协议
func (t DBType) IsCompatibleWithMySQL() bool {
	switch t {
	case DBTypeTiDB, DBTypeMySQL, DBTypeOceanBase, DBTypeMaxScale:
		return true
	default:
		return false
	}
}

// IsCompatibleWithPostgreSQL 检查是否兼容PostgreSQL协议
func (t DBType) IsCompatibleWithPostgreSQL() bool {
	switch t {
	case DBTypePostgreSQL, DBTypeGaussDB, DBTypeKingBase, DBTypeHighGo:
		return true
	default:
		return false
	}
}

// IsDomestic 检查是否为国产数据库
func (t DBType) IsDomestic() bool {
	switch t {
	case DBTypeDM, DBTypeGaussDB, DBTypeKingBase, DBTypeOceanBase, DBTypeHighGo:
		return true
	default:
		return false
	}
}

// DBFeatures 数据库特性
type DBFeatures struct {
	// 协议特性
	SupportsSSL        bool // 支持SSL连接
	SupportsCompression bool // 支持压缩
	SupportsPrepared   bool // 支持预处理语句
	SupportsBatch     bool // 支持批量操作

	// SQL特性
	SupportsCTE        bool // 支持CTE (WITH子句)
	SupportsWindowFunc bool // 支持窗口函数
	SupportsJSON       bool // 支持JSON类型
	SupportsArray     bool // 支持数组类型
	SupportsHLL       bool // 支持HyperLogLog
	SupportsTiFlash   bool // 支持TiFlash

	// 监控特性
	SupportsSlowQuery bool // 支持慢查询日志
	SupportsExplain   bool // 支持EXPLAIN
	SupportsPlan      bool // 支持执行计划
}

// GetFeatures 获取数据库特性
func (t DBType) GetFeatures() DBFeatures {
	switch t {
	case DBTypeTiDB:
		return DBFeatures{
			SupportsSSL:        true,
			SupportsCompression: true,
			SupportsPrepared:   true,
			SupportsBatch:     true,
			SupportsCTE:        true,
			SupportsWindowFunc: true,
			SupportsJSON:       true,
			SupportsArray:      true,
			SupportsHLL:        true,
			SupportsTiFlash:    true,
			SupportsSlowQuery:  true,
			SupportsExplain:    true,
			SupportsPlan:       true,
		}
	case DBTypeMySQL:
		return DBFeatures{
			SupportsSSL:        true,
			SupportsCompression: true,
			SupportsPrepared:   true,
			SupportsBatch:     true,
			SupportsCTE:        false, // MySQL 8.0+ 支持
			SupportsWindowFunc: true,
			SupportsJSON:       true,
			SupportsArray:      false,
			SupportsHLL:        false,
			SupportsSlowQuery:  true,
			SupportsExplain:    true,
			SupportsPlan:       true,
		}
	case DBTypePostgreSQL:
		return DBFeatures{
			SupportsSSL:        true,
			SupportsCompression: false,
			SupportsPrepared:   true,
			SupportsBatch:     true,
			SupportsCTE:        true,
			SupportsWindowFunc: true,
			SupportsJSON:       true,
			SupportsArray:      true,
			SupportsHLL:        false,
			SupportsSlowQuery:  true,
			SupportsExplain:    true,
			SupportsPlan:       true,
		}
	case DBTypeDM: // 达梦数据库
		return DBFeatures{
			SupportsSSL:        true,
			SupportsCompression: false,
			SupportsPrepared:   true,
			SupportsBatch:     true,
			SupportsCTE:        true,
			SupportsWindowFunc: true,
			SupportsJSON:       true, // DM8+
			SupportsArray:      true,
			SupportsHLL:        false,
			SupportsSlowQuery:  true,
			SupportsExplain:    true,
			SupportsPlan:       true,
		}
	case DBTypeGaussDB: // GaussDB
		return DBFeatures{
			SupportsSSL:        true,
			SupportsCompression: true,
			SupportsPrepared:   true,
			SupportsBatch:     true,
			SupportsCTE:        true,
			SupportsWindowFunc: true,
			SupportsJSON:       true,
			SupportsArray:      true,
			SupportsHLL:        true, // GaussDB基于PostgreSQL
			SupportsSlowQuery:  true,
			SupportsExplain:    true,
			SupportsPlan:       true,
		}
	case DBTypeKingBase: // 金仓数据库
		return DBFeatures{
			SupportsSSL:        true,
			SupportsCompression: false,
			SupportsPrepared:   true,
			SupportsBatch:     true,
			SupportsCTE:        true,
			SupportsWindowFunc: true,
			SupportsJSON:       true, // V8+
			SupportsArray:      true,
			SupportsHLL:        false,
			SupportsSlowQuery:  true,
			SupportsExplain:    true,
			SupportsPlan:       true,
		}
	case DBTypeOceanBase:
		return DBFeatures{
			SupportsSSL:        true,
			SupportsCompression: true,
			SupportsPrepared:   true,
			SupportsBatch:     true,
			SupportsCTE:        true,
			SupportsWindowFunc: true,
			SupportsJSON:       true,
			SupportsArray:      true,
			SupportsHLL:        false,
			SupportsSlowQuery:  true,
			SupportsExplain:    true,
			SupportsPlan:       true,
		}
	case DBTypeHighGo: // 瀚高数据库
		return DBFeatures{
			SupportsSSL:        true,
			SupportsCompression: false,
			SupportsPrepared:   true,
			SupportsBatch:     true,
			SupportsCTE:        true,
			SupportsWindowFunc: true,
			SupportsJSON:       true,
			SupportsArray:      true,
			SupportsHLL:        false,
			SupportsSlowQuery:  true,
			SupportsExplain:    true,
			SupportsPlan:       true,
		}
	default:
		return DBFeatures{}
	}
}

// DBConfig 数据库配置
type DBConfig struct {
	Type           DBType `yaml:"type" json:"type"`                     // 数据库类型
	Host           string `yaml:"host" json:"host"`                     // 主机地址
	Port           int    `yaml:"port" json:"port"`                     // 端口
	Database       string `yaml:"database" json:"database"`             // 数据库名
	Username       string `yaml:"username" json:"username"`             // 用户名
	Password       string `yaml:"password" json:"password"`             // 密码
	MaxOpenConns   int    `yaml:"max_open_conns" json:"max_open_conns"` // 最大连接数
	MaxIdleConns   int    `yaml:"max_idle_conns" json:"max_idle_conns"` // 最大空闲连接
	ConnMaxLifetime int    `yaml:"conn_max_lifetime" json:"conn_max_lifetime"` // 连接最大生命周期(秒)

	// SSL配置
	EnableSSL     bool   `yaml:"enable_ssl" json:"enable_ssl"`         // 启用SSL
	SSLCA         string `yaml:"ssl_ca" json:"ssl_ca"`                 // CA证书
	SSLCert       string `yaml:"ssl_cert" json:"ssl_cert"`             // 客户端证书
	SSLKey        string `yaml:"ssl_key" json:"ssl_key"`               // 客户端私钥
	SSLVerifyCert bool   `yaml:"ssl_verify_cert" json:"ssl_verify_cert"` // 验证证书

	// 国产数据库特有配置
	Mode            string `yaml:"mode" json:"mode"`                     // 连接模式 (普通/HA/安全)
	DMSchema        string `yaml:"dm_schema" json:"dm_schema"`         // 达梦模式名
	GaussDBInstance string `yaml:"gaussdb_instance" json:"gaussdb_instance"` // GaussDB实例名
}

// DefaultPort 获取数据库默认端口
func (t DBType) DefaultPort() int {
	switch t {
	case DBTypeTiDB, DBTypeMySQL, DBTypeOceanBase:
		return 4000 // TiDB/OceanBase通常使用4000
	case DBTypePostgreSQL:
		return 5432
	case DBTypeDM:
		return 5236 // 达梦默认端口
	case DBTypeGaussDB:
		return 8000 // GaussDB默认端口
	case DBTypeKingBase:
		return 54321 // 金仓默认端口
	case DBTypeHighGo:
		return 5866 // 瀚高默认端口
	default:
		return 0
	}
}

// Validate 验证配置
func (c *DBConfig) Validate() error {
	if c.Type == DBTypeUnknown {
		return fmt.Errorf("未知的数据库类型")
	}

	if c.Host == "" {
		return fmt.Errorf("数据库地址不能为空")
	}

	if c.Port <= 0 {
		c.Port = c.Type.DefaultPort()
		if c.Port == 0 {
			return fmt.Errorf("无法确定数据库默认端口，请手动指定")
		}
	}

	if c.Username == "" {
		return fmt.Errorf("用户名不能为空")
	}

	return nil
}

// Clone 克隆配置
func (c *DBConfig) Clone() *DBConfig {
	clone := *c
	return &clone
}

// DSN 获取数据源名称
func (c *DBConfig) DSN() string {
	switch c.Type {
	case DBTypeDM: // 达梦
		return fmt.Sprintf("dm://%s/%s?server=%s:%d",
			c.Username, c.Database, c.Host, c.Port)

	case DBTypeGaussDB: // GaussDB (兼容PostgreSQL)
		return fmt.Sprintf("host=%s port=%d user=%s password=%s dbname=%s sslmode=disable",
			c.Host, c.Port, c.Username, c.Password, c.Database)

	case DBTypeKingBase: // 金仓 (兼容PostgreSQL)
		return fmt.Sprintf("host=%s port=%d user=%s password=%s dbname=%s sslmode=disable",
			c.Host, c.Port, c.Username, c.Password, c.Database)

	case DBTypeHighGo: // 瀚高 (兼容PostgreSQL)
		return fmt.Sprintf("host=%s port=%d user=%s password=%s dbname=%s sslmode=disable",
			c.Host, c.Port, c.Username, c.Password, c.Database)

	default: // MySQL兼容
		return fmt.Sprintf("%s:%s@tcp(%s:%d)/%s?charset=utf8mb4&parseTime=True&loc=Local",
			c.Username, c.Password, c.Host, c.Port, c.Database)
	}
}
