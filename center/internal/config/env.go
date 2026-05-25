//go:build linux

// Package config 提供配置管理，所有配置从环境变量读取
package config

import (
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"
)

// Config 服务配置（完全从环境变量读取）
type Config struct {
	// 服务标识
	CenterID string // CENTER_ID
	Mode    string // CENTER_MODE (standalone/cluster)

	// HTTP服务
	Addr           string        // HTTP_ADDR
	Port           int           // HTTP_PORT (默认8080)
	HealthPort     int           // HEALTH_PORT (默认8081)
	ReadTimeout    time.Duration // READ_TIMEOUT
	WriteTimeout   time.Duration // WRITE_TIMEOUT
	IdleTimeout    time.Duration // IDLE_TIMEOUT
	MaxHeaderBytes int           // MAX_HEADER_BYTES

	// 日志
	LogLevel  string // LOG_LEVEL (debug/info/warn/error)
	LogFormat string // LOG_FORMAT (json/text)

	// 数据库配置
	Database DatabaseConfig

	// Redis配置
	Redis RedisConfig
}

// DatabaseConfig 数据库配置
type DatabaseConfig struct {
	// 连接地址（支持多地址，逗号分隔）
	Addr        string // DATABASE_ADDR
	Username    string // DATABASE_USERNAME
	Password    string // DATABASE_PASSWORD
	Name        string // DATABASE_NAME
	
	// 连接池
	MaxOpenConns int // DATABASE_MAX_OPEN_CONNS
	MaxIdleConns int // DATABASE_MAX_IDLE_CONNS
	ConnMaxLife  time.Duration // DATABASE_CONN_MAX_LIFE
	
	// SSL
	SSLMode     string // DATABASE_SSL_MODE (disable/require/verify-ca/verify-full)
	SSLCA       string // DATABASE_SSL_CA
	SSLCert     string // DATABASE_SSL_CERT
	SSLKey      string // DATABASE_SSL_KEY
	
	// 连接超时
	ConnTimeout time.Duration // DATABASE_CONN_TIMEOUT
}

// RedisConfig Redis配置
type RedisConfig struct {
	// 连接地址（支持多地址，逗号分隔）
	Addr     string // REDIS_ADDR
	Password string // REDIS_PASSWORD
	DB       int    // REDIS_DB
	
	// 连接池
	PoolSize     int // REDIS_POOL_SIZE
	MinIdleConns int // REDIS_MIN_IDLE_CONNS
	
	// 超时
	DialTimeout  time.Duration // REDIS_DIAL_TIMEOUT
	ReadTimeout  time.Duration // REDIS_READ_TIMEOUT
	WriteTimeout time.Duration // REDIS_WRITE_TIMEOUT
	
	// 集群模式
	Mode     string // REDIS_MODE (single/cluster/sentinel)
	MasterName string // REDIS_MASTER_NAME (sentinel模式)
	
	// 密码认证
	Username string // REDIS_USERNAME
}

// LoadFromEnv 从环境变量加载配置
func LoadFromEnv() (*Config, error) {
	cfg := &Config{
		// 服务标识
		CenterID: getEnv("CENTER_ID", "center-1"),
		Mode:     getEnv("CENTER_MODE", "standalone"),
		
		// HTTP服务
		Addr:       getEnv("HTTP_ADDR", ":8080"),
		Port:       getEnvInt("HTTP_PORT", 8080),
		HealthPort: getEnvInt("HEALTH_PORT", 8081),
		ReadTimeout: getEnvDuration("READ_TIMEOUT", 60*time.Second),
		WriteTimeout: getEnvDuration("WRITE_TIMEOUT", 60*time.Second),
		IdleTimeout: getEnvDuration("IDLE_TIMEOUT", 120*time.Second),
		MaxHeaderBytes: getEnvInt("MAX_HEADER_BYTES", 1<<20),
		
		// 日志
		LogLevel:  getEnv("LOG_LEVEL", "info"),
		LogFormat: getEnv("LOG_FORMAT", "json"),
		
		// 数据库
		Database: DatabaseConfig{
			Addr:        getEnv("DATABASE_ADDR", "localhost:5432"),
			Username:    getEnv("DATABASE_USERNAME", "cloudflow"),
			Password:    getEnv("DATABASE_PASSWORD", ""),
			Name:        getEnv("DATABASE_NAME", "cloudflow"),
			MaxOpenConns: getEnvInt("DATABASE_MAX_OPEN_CONNS", 100),
			MaxIdleConns: getEnvInt("DATABASE_MAX_IDLE_CONNS", 10),
			ConnMaxLife:  getEnvDuration("DATABASE_CONN_MAX_LIFE", 30*time.Minute),
			SSLMode:     getEnv("DATABASE_SSL_MODE", "disable"),
			SSLCA:       getEnv("DATABASE_SSL_CA", ""),
			SSLCert:     getEnv("DATABASE_SSL_CERT", ""),
			SSLKey:      getEnv("DATABASE_SSL_KEY", ""),
			ConnTimeout: getEnvDuration("DATABASE_CONN_TIMEOUT", 10*time.Second),
		},
		
		// Redis
		Redis: RedisConfig{
			Addr:         getEnv("REDIS_ADDR", "localhost:6379"),
			Password:     getEnv("REDIS_PASSWORD", ""),
			DB:           getEnvInt("REDIS_DB", 0),
			PoolSize:     getEnvInt("REDIS_POOL_SIZE", 100),
			MinIdleConns: getEnvInt("REDIS_MIN_IDLE_CONNS", 10),
			DialTimeout:  getEnvDuration("REDIS_DIAL_TIMEOUT", 5*time.Second),
			ReadTimeout:  getEnvDuration("REDIS_READ_TIMEOUT", 3*time.Second),
			WriteTimeout: getEnvDuration("REDIS_WRITE_TIMEOUT", 3*time.Second),
			Mode:         getEnv("REDIS_MODE", "single"),
			MasterName:   getEnv("REDIS_MASTER_NAME", "mymaster"),
			Username:     getEnv("REDIS_USERNAME", ""),
		},
	}

	// 验证配置
	if err := cfg.Validate(); err != nil {
		return nil, err
	}

	return cfg, nil
}

// Validate 验证配置
func (c *Config) Validate() error {
	if c.CenterID == "" {
		return fmt.Errorf("CENTER_ID 不能为空")
	}

	if c.Database.Addr == "" {
		return fmt.Errorf("DATABASE_ADDR 不能为空")
	}

	if c.Redis.Addr == "" {
		return fmt.Errorf("REDIS_ADDR 不能为空")
	}

	return nil
}

// GetDSN 获取数据库连接字符串
func (c *DatabaseConfig) GetDSN() string {
	dsn := fmt.Sprintf("host=%s user=%s password=%s dbname=%s sslmode=%s",
		c.Addr, c.Username, c.Password, c.Name, c.SSLMode)

	if c.SSLCA != "" {
		dsn += fmt.Sprintf(" sslrootcert=%s", c.SSLCA)
	}
	if c.SSLCert != "" {
		dsn += fmt.Sprintf(" sslcert=%s", c.SSLCert)
	}
	if c.SSLKey != "" {
		dsn += fmt.Sprintf(" sslkey=%s", c.SSLKey)
	}

	return dsn
}

// GetRedisAddr 获取Redis地址列表
func (c *RedisConfig) GetAddrs() []string {
	return strings.Split(c.Addr, ",")
}

// ==================== 辅助函数 ====================

func getEnv(key, defaultVal string) string {
	if val := os.Getenv(key); val != "" {
		return val
	}
	return defaultVal
}

func getEnvInt(key string, defaultVal int) int {
	if val := os.Getenv(key); val != "" {
		if intVal, err := strconv.Atoi(val); err == nil {
			return intVal
		}
	}
	return defaultVal
}

func getEnvDuration(key string, defaultVal time.Duration) time.Duration {
	if val := os.Getenv(key); val != "" {
		if duration, err := time.ParseDuration(val); err == nil {
			return duration
		}
	}
	return defaultVal
}
