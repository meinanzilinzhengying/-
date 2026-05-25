//go:build linux

// Package config 提供环境变量配置管理
package config

import (
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"
)

// Config 边缘服务配置
type Config struct {
	// 实例标识
	InstanceID   string // 实例ID，为空则自动生成
	InstanceName string // 实例名称
	Region       string // 区域
	Zone         string // 可用区
	
	// 服务配置
	Server ServerConfig
	
	// gRPC配置
	GRPC GRPCConfig
	
	// Redis配置
	Redis RedisConfig
	
	// 上游Center配置
	Center CenterConfig
	
	// Agent连接配置
	Agent AgentConfig
	
	// 负载均衡配置
	LoadBalancer LoadBalancerConfig
	
	// 健康检查配置
	Health HealthConfig
	
	// 日志配置
	Log LogConfig
}

// ServerConfig 服务配置
type ServerConfig struct {
	Host         string        // 监听地址
	Port         int           // 监听端口
	ReadTimeout  time.Duration // 读取超时
	WriteTimeout time.Duration // 写入超时
}

// GRPCConfig gRPC配置
type GRPCConfig struct {
	Host              string        // gRPC监听地址
	Port              int           // gRPC监听端口
	MaxRecvMsgSize    int           // 最大接收消息大小
	MaxSendMsgSize    int           // 最大发送消息大小
	KeepaliveTime     time.Duration // 保活时间
	KeepaliveTimeout  time.Duration // 保活超时
	EnableTLS         bool          // 启用TLS
	TLSCertFile       string        // TLS证书文件
	TLSKeyFile        string        // TLS密钥文件
	TLSCAFile         string        // TLS CA文件
}

// RedisConfig Redis配置
type RedisConfig struct {
	Enabled      bool          // 启用Redis
	Address      string        // Redis地址
	Password     string        // Redis密码
	DB           int           // 数据库编号
	PoolSize     int           // 连接池大小
	MinIdleConns int           // 最小空闲连接
	MaxRetries   int           // 最大重试次数
	DialTimeout  time.Duration // 连接超时
	ReadTimeout  time.Duration // 读取超时
	WriteTimeout time.Duration // 写入超时
	KeyPrefix    string        // 键前缀
}

// CenterConfig 中心服务配置
type CenterConfig struct {
	Address       string        // Center地址
	AuthToken     string        // 认证Token
	HeartbeatInterval time.Duration // 心跳间隔
	ReconnectInterval time.Duration // 重连间隔
}

// AgentConfig Agent连接配置
type AgentConfig struct {
	MaxConnections    int           // 最大Agent连接数
	SessionTimeout    time.Duration // 会话超时
	AuthEnabled       bool          // 启用认证
	AuthToken         string        // 认证Token
}

// LoadBalancerConfig 负载均衡配置
type LoadBalancerConfig struct {
	Strategy          string        // 负载均衡策略: round_robin, least_conn, consistent_hash
	HealthCheckInterval time.Duration // 健康检查间隔
	UnhealthyThreshold  int           // 不健康阈值
	HealthyThreshold    int           // 健康阈值
}

// HealthConfig 健康检查配置
type HealthConfig struct {
	Enabled  bool // 启用健康检查
	Port     int  // 健康检查端口
	GRPCPort int  // gRPC健康检查端口
}

// LogConfig 日志配置
type LogConfig struct {
	Level  string // 日志级别
	Format string // 日志格式: json, text
	Output string // 日志输出: stdout, file
}

// LoadFromEnv 从环境变量加载配置
func LoadFromEnv() (*Config, error) {
	cfg := &Config{
		// 实例标识
		InstanceID:   getEnv("EDGE_INSTANCE_ID", ""),
		InstanceName: getEnv("EDGE_INSTANCE_NAME", "edge-"+generateInstanceID()),
		Region:       getEnv("EDGE_REGION", "default"),
		Zone:         getEnv("EDGE_ZONE", "default"),
		
		// 服务配置
		Server: ServerConfig{
			Host:         getEnv("EDGE_SERVER_HOST", "0.0.0.0"),
			Port:         getEnvInt("EDGE_SERVER_PORT", 8080),
			ReadTimeout:  getEnvDuration("EDGE_SERVER_READ_TIMEOUT", 30*time.Second),
			WriteTimeout: getEnvDuration("EDGE_SERVER_WRITE_TIMEOUT", 30*time.Second),
		},
		
		// gRPC配置
		GRPC: GRPCConfig{
			Host:             getEnv("EDGE_GRPC_HOST", "0.0.0.0"),
			Port:             getEnvInt("EDGE_GRPC_PORT", 50051),
			MaxRecvMsgSize:   getEnvInt("EDGE_GRPC_MAX_RECV_MSG_SIZE", 64*1024*1024),
			MaxSendMsgSize:   getEnvInt("EDGE_GRPC_MAX_SEND_MSG_SIZE", 64*1024*1024),
			KeepaliveTime:    getEnvDuration("EDGE_GRPC_KEEPALIVE_TIME", 30*time.Second),
			KeepaliveTimeout: getEnvDuration("EDGE_GRPC_KEEPALIVE_TIMEOUT", 10*time.Second),
			EnableTLS:        getEnvBool("EDGE_GRPC_ENABLE_TLS", false),
			TLSCertFile:      getEnv("EDGE_GRPC_TLS_CERT_FILE", ""),
			TLSKeyFile:       getEnv("EDGE_GRPC_TLS_KEY_FILE", ""),
			TLSCAFile:        getEnv("EDGE_GRPC_TLS_CA_FILE", ""),
		},
		
		// Redis配置
		Redis: RedisConfig{
			Enabled:      getEnvBool("EDGE_REDIS_ENABLED", true),
			Address:      getEnv("EDGE_REDIS_ADDRESS", "localhost:6379"),
			Password:     getEnv("EDGE_REDIS_PASSWORD", ""),
			DB:           getEnvInt("EDGE_REDIS_DB", 0),
			PoolSize:     getEnvInt("EDGE_REDIS_POOL_SIZE", 10),
			MinIdleConns: getEnvInt("EDGE_REDIS_MIN_IDLE_CONNS", 5),
			MaxRetries:   getEnvInt("EDGE_REDIS_MAX_RETRIES", 3),
			DialTimeout:  getEnvDuration("EDGE_REDIS_DIAL_TIMEOUT", 5*time.Second),
			ReadTimeout:  getEnvDuration("EDGE_REDIS_READ_TIMEOUT", 3*time.Second),
			WriteTimeout: getEnvDuration("EDGE_REDIS_WRITE_TIMEOUT", 3*time.Second),
			KeyPrefix:    getEnv("EDGE_REDIS_KEY_PREFIX", "edge:"),
		},
		
		// Center配置
		Center: CenterConfig{
			Address:           getEnv("EDGE_CENTER_ADDRESS", "localhost:8080"),
			AuthToken:         getEnv("EDGE_CENTER_AUTH_TOKEN", ""),
			HeartbeatInterval: getEnvDuration("EDGE_CENTER_HEARTBEAT_INTERVAL", 30*time.Second),
			ReconnectInterval: getEnvDuration("EDGE_CENTER_RECONNECT_INTERVAL", 5*time.Second),
		},
		
		// Agent配置
		Agent: AgentConfig{
			MaxConnections: getEnvInt("EDGE_AGENT_MAX_CONNECTIONS", 10000),
			SessionTimeout: getEnvDuration("EDGE_AGENT_SESSION_TIMEOUT", 5*time.Minute),
			AuthEnabled:    getEnvBool("EDGE_AGENT_AUTH_ENABLED", true),
			AuthToken:      getEnv("EDGE_AGENT_AUTH_TOKEN", ""),
		},
		
		// 负载均衡配置
		LoadBalancer: LoadBalancerConfig{
			Strategy:            getEnv("EDGE_LB_STRATEGY", "least_conn"),
			HealthCheckInterval: getEnvDuration("EDGE_LB_HEALTH_CHECK_INTERVAL", 10*time.Second),
			UnhealthyThreshold:  getEnvInt("EDGE_LB_UNHEALTHY_THRESHOLD", 3),
			HealthyThreshold:    getEnvInt("EDGE_LB_HEALTHY_THRESHOLD", 2),
		},
		
		// 健康检查配置
		Health: HealthConfig{
			Enabled:  getEnvBool("EDGE_HEALTH_ENABLED", true),
			Port:     getEnvInt("EDGE_HEALTH_PORT", 8081),
			GRPCPort: getEnvInt("EDGE_HEALTH_GRPC_PORT", 50052),
		},
		
		// 日志配置
		Log: LogConfig{
			Level:  getEnv("EDGE_LOG_LEVEL", "info"),
			Format: getEnv("EDGE_LOG_FORMAT", "json"),
			Output: getEnv("EDGE_LOG_OUTPUT", "stdout"),
		},
	}
	
	// 验证配置
	if err := cfg.Validate(); err != nil {
		return nil, fmt.Errorf("config validation failed: %w", err)
	}
	
	return cfg, nil
}

// Validate 验证配置
func (c *Config) Validate() error {
	if c.Server.Port <= 0 || c.Server.Port > 65535 {
		return fmt.Errorf("invalid server port: %d", c.Server.Port)
	}
	
	if c.GRPC.Port <= 0 || c.GRPC.Port > 65535 {
		return fmt.Errorf("invalid gRPC port: %d", c.GRPC.Port)
	}
	
	if c.Redis.Enabled && c.Redis.Address == "" {
		return fmt.Errorf("redis address is required when redis is enabled")
	}
	
	if c.Center.Address == "" {
		return fmt.Errorf("center address is required")
	}
	
	validStrategies := map[string]bool{
		"round_robin":     true,
		"least_conn":      true,
		"consistent_hash": true,
	}
	if !validStrategies[c.LoadBalancer.Strategy] {
		return fmt.Errorf("invalid load balancer strategy: %s", c.LoadBalancer.Strategy)
	}
	
	return nil
}

// GetInstanceKey 获取实例唯一标识
func (c *Config) GetInstanceKey() string {
	if c.InstanceID != "" {
		return c.InstanceID
	}
	return fmt.Sprintf("%s-%s-%s", c.Region, c.Zone, c.InstanceName)
}

// GetGRPCAddress 获取gRPC监听地址
func (c *Config) GetGRPCAddress() string {
	return fmt.Sprintf("%s:%d", c.GRPC.Host, c.GRPC.Port)
}

// GetServerAddress 获取HTTP服务地址
func (c *Config) GetServerAddress() string {
	return fmt.Sprintf("%s:%d", c.Server.Host, c.Server.Port)
}

// GetHealthAddress 获取健康检查地址
func (c *Config) GetHealthAddress() string {
	return fmt.Sprintf("%s:%d", c.Server.Host, c.Health.Port)
}

// ==================== 辅助函数 ====================

func getEnv(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}

func getEnvInt(key string, defaultValue int) int {
	if value := os.Getenv(key); value != "" {
		if i, err := strconv.Atoi(value); err == nil {
			return i
		}
	}
	return defaultValue
}

func getEnvBool(key string, defaultValue bool) bool {
	if value := os.Getenv(key); value != "" {
		if b, err := strconv.ParseBool(strings.ToLower(value)); err == nil {
			return b
		}
	}
	return defaultValue
}

func getEnvDuration(key string, defaultValue time.Duration) time.Duration {
	if value := os.Getenv(key); value != "" {
		if d, err := time.ParseDuration(value); err == nil {
			return d
		}
	}
	return defaultValue
}

func generateInstanceID() string {
	hostname, _ := os.Hostname()
	if hostname == "" {
		hostname = "unknown"
	}
	return fmt.Sprintf("%s-%d", hostname, time.Now().Unix())
}
