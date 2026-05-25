//go:build linux

// Package config 提供基于 Nacos 的配置管理
// 支持配置热更新，无需重启服务
package config

import (
	"encoding/json"
	"fmt"
	"log"
	"os"
	"time"

	"cloud-flow-agent/pkg/config/nacos"
)

// NacosEnabled 是否启用 Nacos
var NacosEnabled = os.Getenv("NACOS_ENABLED") == "true"

// NacosManager Nacos 配置管理器
var NacosManager *nacos.ConfigManager

// NacosClient Nacos 客户端
var NacosClient *nacos.Client

// InitNacos 初始化 Nacos 配置
func InitNacos() error {
	if !NacosEnabled {
		log.Println("[Config] Nacos disabled, using environment variables")
		return nil
	}

	// 创建 Nacos 客户端
	client, err := nacos.NewClient(nil)
	if err != nil {
		return fmt.Errorf("failed to create nacos client: %w", err)
	}

	NacosClient = client

	// 创建配置管理器
	manager, err := nacos.NewConfigManager(client)
	if err != nil {
		return fmt.Errorf("failed to create config manager: %w", err)
	}

	NacosManager = manager

	log.Printf("[Config] Nacos initialized, env=%s", manager.GetEnv())
	return nil
}

// LoadConfigFromNacos 从 Nacos 加载配置
func LoadConfigFromNacos() (*Config, error) {
	if !NacosEnabled || NacosManager == nil {
		return nil, fmt.Errorf("nacos not enabled")
	}

	cfg := &Config{}

	// 加载中心服务配置
	centerConfig := &CenterServiceConfig{}
	if err := NacosManager.LoadConfig("center-service", centerConfig); err != nil {
		log.Printf("[Config] Failed to load center-service config from Nacos: %v, using defaults", err)
		// 使用默认配置
		centerConfig = defaultCenterServiceConfig()
	}

	// 应用配置
	applyCenterConfig(cfg, centerConfig)

	// 加载数据库配置
	dbConfig := &DatabaseConfigNacos{}
	if err := NacosManager.LoadConfig("database", dbConfig); err != nil {
		log.Printf("[Config] Failed to load database config from Nacos: %v, using defaults", err)
		dbConfig = defaultDatabaseConfigNacos()
	}
	applyDatabaseConfig(cfg, dbConfig)

	// 加载 Redis 配置
	redisConfig := &RedisConfigNacos{}
	if err := NacosManager.LoadConfig("redis", redisConfig); err != nil {
		log.Printf("[Config] Failed to load redis config from Nacos: %v, using defaults", err)
		redisConfig = defaultRedisConfigNacos()
	}
	applyRedisConfig(cfg, redisConfig)

	// 加载日志配置
	logConfig := &LogConfigNacos{}
	if err := NacosManager.LoadConfig("logging", logConfig); err != nil {
		log.Printf("[Config] Failed to load logging config from Nacos: %v, using defaults", err)
		logConfig = defaultLogConfigNacos()
	}
	applyLogConfig(cfg, logConfig)

	// 注册热更新监听
	if err := watchConfigChanges(cfg); err != nil {
		log.Printf("[Config] Failed to watch config changes: %v", err)
	}

	return cfg, nil
}

// watchConfigChanges 监听配置变更
func watchConfigChanges(cfg *Config) error {
	// 监听中心服务配置变更
	if err := NacosManager.WatchConfig("center-service", &CenterServiceConfig{}, func(oldVal, newVal interface{}) {
		log.Println("[Config] Center service config updated")
		if newConfig, ok := newVal.(*CenterServiceConfig); ok {
			applyCenterConfig(cfg, newConfig)
		}
	}); err != nil {
		return err
	}

	// 监听数据库配置变更
	if err := NacosManager.WatchConfig("database", &DatabaseConfigNacos{}, func(oldVal, newVal interface{}) {
		log.Println("[Config] Database config updated, please restart to apply changes")
		// 数据库配置通常需要重启才能生效
	}); err != nil {
		return err
	}

	// 监听日志配置变更（热生效）
	if err := NacosManager.WatchConfig("logging", &LogConfigNacos{}, func(oldVal, newVal interface{}) {
		log.Println("[Config] Logging config updated")
		if newConfig, ok := newVal.(*LogConfigNacos); ok {
			applyLogConfig(cfg, newConfig)
		}
	}); err != nil {
		return err
	}

	return nil
}

// ==================== 配置结构体 ====================

// CenterServiceConfig 中心服务配置
type CenterServiceConfig struct {
	CenterID     string `json:"centerId" yaml:"centerId"`
	Mode         string `json:"mode" yaml:"mode"`
	Addr         string `json:"addr" yaml:"addr"`
	Port         int    `json:"port" yaml:"port"`
	HealthPort   int    `json:"healthPort" yaml:"healthPort"`
	ReadTimeout  string `json:"readTimeout" yaml:"readTimeout"`
	WriteTimeout string `json:"writeTimeout" yaml:"writeTimeout"`
	IdleTimeout  string `json:"idleTimeout" yaml:"idleTimeout"`
}

// DatabaseConfigNacos 数据库配置
type DatabaseConfigNacos struct {
	Addr         string `json:"addr" yaml:"addr"`
	Username     string `json:"username" yaml:"username"`
	Password     string `json:"password" yaml:"password"`
	Name         string `json:"name" yaml:"name"`
	MaxOpenConns int    `json:"maxOpenConns" yaml:"maxOpenConns"`
	MaxIdleConns int    `json:"maxIdleConns" yaml:"maxIdleConns"`
	ConnMaxLife  string `json:"connMaxLife" yaml:"connMaxLife"`
	SSLMode      string `json:"sslMode" yaml:"sslMode"`
	ConnTimeout  string `json:"connTimeout" yaml:"connTimeout"`
}

// RedisConfigNacos Redis配置
type RedisConfigNacos struct {
	Addr         string `json:"addr" yaml:"addr"`
	Password     string `json:"password" yaml:"password"`
	DB           int    `json:"db" yaml:"db"`
	PoolSize     int    `json:"poolSize" yaml:"poolSize"`
	MinIdleConns int    `json:"minIdleConns" yaml:"minIdleConns"`
	DialTimeout  string `json:"dialTimeout" yaml:"dialTimeout"`
	ReadTimeout  string `json:"readTimeout" yaml:"readTimeout"`
	WriteTimeout string `json:"writeTimeout" yaml:"writeTimeout"`
	Mode         string `json:"mode" yaml:"mode"`
	MasterName   string `json:"masterName" yaml:"masterName"`
	Username     string `json:"username" yaml:"username"`
}

// LogConfigNacos 日志配置
type LogConfigNacos struct {
	Level  string `json:"level" yaml:"level"`
	Format string `json:"format" yaml:"format"`
}

// ==================== 默认配置 ====================

func defaultCenterServiceConfig() *CenterServiceConfig {
	return &CenterServiceConfig{
		CenterID:     "center-1",
		Mode:         "standalone",
		Addr:         ":8080",
		Port:         8080,
		HealthPort:   8081,
		ReadTimeout:  "60s",
		WriteTimeout: "60s",
		IdleTimeout:  "120s",
	}
}

func defaultDatabaseConfigNacos() *DatabaseConfigNacos {
	return &DatabaseConfigNacos{
		Addr:         "localhost:5432",
		Username:     "cloudflow",
		Password:     "",
		Name:         "cloudflow",
		MaxOpenConns: 100,
		MaxIdleConns: 10,
		ConnMaxLife:  "30m",
		SSLMode:      "disable",
		ConnTimeout:  "10s",
	}
}

func defaultRedisConfigNacos() *RedisConfigNacos {
	return &RedisConfigNacos{
		Addr:         "localhost:6379",
		Password:     "",
		DB:           0,
		PoolSize:     100,
		MinIdleConns: 10,
		DialTimeout:  "5s",
		ReadTimeout:  "3s",
		WriteTimeout: "3s",
		Mode:         "single",
		MasterName:   "mymaster",
		Username:     "",
	}
}

func defaultLogConfigNacos() *LogConfigNacos {
	return &LogConfigNacos{
		Level:  "info",
		Format: "json",
	}
}

// ==================== 配置应用 ====================

func applyCenterConfig(cfg *Config, nc *CenterServiceConfig) {
	cfg.CenterID = nc.CenterID
	cfg.Mode = nc.Mode
	cfg.Addr = nc.Addr
	cfg.Port = nc.Port
	cfg.HealthPort = nc.HealthPort
	cfg.ReadTimeout = parseDuration(nc.ReadTimeout)
	cfg.WriteTimeout = parseDuration(nc.WriteTimeout)
	cfg.IdleTimeout = parseDuration(nc.IdleTimeout)
}

func applyDatabaseConfig(cfg *Config, dc *DatabaseConfigNacos) {
	cfg.Database.Addr = dc.Addr
	cfg.Database.Username = dc.Username
	cfg.Database.Password = dc.Password
	cfg.Database.Name = dc.Name
	cfg.Database.MaxOpenConns = dc.MaxOpenConns
	cfg.Database.MaxIdleConns = dc.MaxIdleConns
	cfg.Database.ConnMaxLife = parseDuration(dc.ConnMaxLife)
	cfg.Database.SSLMode = dc.SSLMode
	cfg.Database.ConnTimeout = parseDuration(dc.ConnTimeout)
}

func applyRedisConfig(cfg *Config, rc *RedisConfigNacos) {
	cfg.Redis.Addr = rc.Addr
	cfg.Redis.Password = rc.Password
	cfg.Redis.DB = rc.DB
	cfg.Redis.PoolSize = rc.PoolSize
	cfg.Redis.MinIdleConns = rc.MinIdleConns
	cfg.Redis.DialTimeout = parseDuration(rc.DialTimeout)
	cfg.Redis.ReadTimeout = parseDuration(rc.ReadTimeout)
	cfg.Redis.WriteTimeout = parseDuration(rc.WriteTimeout)
	cfg.Redis.Mode = rc.Mode
	cfg.Redis.MasterName = rc.MasterName
	cfg.Redis.Username = rc.Username
}

func applyLogConfig(cfg *Config, lc *LogConfigNacos) {
	cfg.LogLevel = lc.Level
	cfg.LogFormat = lc.Format
}

func parseDuration(s string) time.Duration {
	d, err := time.ParseDuration(s)
	if err != nil {
		return 0
	}
	return d
}

// ==================== 初始化配置 ====================

// InitConfig 初始化配置（优先从 Nacos 加载，失败则使用环境变量）
func InitConfig() (*Config, error) {
	// 尝试初始化 Nacos
	if err := InitNacos(); err != nil {
		log.Printf("[Config] Nacos init failed: %v, falling back to env", err)
	}

	// 如果 Nacos 启用，尝试从 Nacos 加载
	if NacosEnabled && NacosManager != nil {
		cfg, err := LoadConfigFromNacos()
		if err == nil {
			log.Println("[Config] Loaded config from Nacos")
			return cfg, nil
		}
		log.Printf("[Config] Failed to load from Nacos: %v, falling back to env", err)
	}

	// 从环境变量加载
	return LoadFromEnv()
}

// PublishDefaultConfigs 发布默认配置到 Nacos（用于初始化）
func PublishDefaultConfigs() error {
	if !NacosEnabled || NacosManager == nil {
		return fmt.Errorf("nacos not enabled")
	}

	// 发布中心服务配置
	if err := NacosManager.PublishEnvConfig("center-service", defaultCenterServiceConfig()); err != nil {
		return fmt.Errorf("publish center-service config failed: %w", err)
	}

	// 发布数据库配置
	if err := NacosManager.PublishEnvConfig("database", defaultDatabaseConfigNacos()); err != nil {
		return fmt.Errorf("publish database config failed: %w", err)
	}

	// 发布 Redis 配置
	if err := NacosManager.PublishEnvConfig("redis", defaultRedisConfigNacos()); err != nil {
		return fmt.Errorf("publish redis config failed: %w", err)
	}

	// 发布日志配置
	if err := NacosManager.PublishEnvConfig("logging", defaultLogConfigNacos()); err != nil {
		return fmt.Errorf("publish logging config failed: %w", err)
	}

	log.Println("[Config] Default configs published to Nacos")
	return nil
}

// GetConfigJSON 获取配置的 JSON 表示（用于调试）
func GetConfigJSON(cfg *Config) string {
	data, _ := json.MarshalIndent(cfg, "", "  ")
	return string(data)
}
