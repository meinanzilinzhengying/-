package config

import (
	"fmt"
	"log"
	"os"
	"regexp"
	"strings"

	"github.com/spf13/viper"
)

type LogConfig struct {
	Level      string
	Format     string
	Output     string
	LogDir     string
	MaxSize    int
	MaxBackups int
	MaxAge     int
}

type TLSConfig struct {
	Enabled    bool
	ServerCert string
	ServerKey  string
	CACert     string
}

type PortalConfig struct {
	Port      int
	AuthUser  string
	AuthPass  string
	RedisAddr string // Redis 地址，留空则使用内存存储
}

type MixServerConfig struct {
	Port     int
	AuthUser string
	AuthPass string
}

type JWTConfig struct {
	SecretKey     string
	TokenDuration int // 令牌有效期（小时）
}

type ClusterConfig struct {
	Enabled       bool
	EtcdEndpoints []string
	LeaseTTL      int64
	NodeID        string
	NodeAddr      string
}

type EmailConfig struct {
	Enabled      bool
	SMTPHost     string
	SMTPPort     int
	SMTPUsername string
	SMTPPassword string `json:"-"`
	From         string
	To           []string
}

type WebhookConfig struct {
	Enabled bool
	URL     string
	Headers map[string]string
}

type AlertingConfig struct {
	RulesPath     string
	CheckInterval int
	Email         EmailConfig
	Webhook       WebhookConfig
}

type StorageConfig struct {
	DSN     string
	RetDays int
}

type RateLimitConfig struct {
	Enabled    bool // 是否启用限流
	BucketSize int  // 令牌桶容量
	RefillRate int  // 每秒填充令牌数
}

type Config struct {
	GRPCListenPort int
	DataDir        string
	RetentionDays  int
	TLS            TLSConfig
	APIKey         string
	JWT            JWTConfig
	Cluster        ClusterConfig
	Portal         PortalConfig
	MixServer      MixServerConfig
	RateLimit      RateLimitConfig
	Log            LogConfig
	Alerting       AlertingConfig
	Storage        StorageConfig
}

// envVarRegex 匹配 ${VAR:-default} 格式的正则表达式（包级别编译，避免每次调用时重复编译）
var envVarRegex = regexp.MustCompile(`\$\{([A-Za-z_][A-Za-z0-9_]*)(:-([^}]*))?\}`)

// expandEnvVarsInConfig 展开配置中的环境变量，如 ${VAR:-default} 格式
func expandEnvVarsInConfig() {
	// 遍历所有配置键值
	for _, key := range viper.AllKeys() {
		value := viper.GetString(key)
		if value == "" {
			continue
		}

		// 替换环境变量
		newValue := envVarRegex.ReplaceAllStringFunc(value, func(match string) string {
			parts := envVarRegex.FindStringSubmatch(match)
			if len(parts) < 2 {
				return match
			}
			envVar := parts[1]
			defaultValue := ""
			if len(parts) >= 4 {
				defaultValue = parts[3]
			}

			if envValue := os.Getenv(envVar); envValue != "" {
				return envValue
			}
			return defaultValue
		})

		if newValue != value {
			viper.Set(key, newValue)
		}
	}
}

func Load() (*Config, error) {
	viper.SetDefault("center.grpc_listen_port", 9090)
	viper.SetDefault("center.data_dir", "./data")
	viper.SetDefault("center.retention_days", 7)
	viper.SetDefault("center.tls.enabled", true)
	viper.SetDefault("center.tls.server_cert", "")
	viper.SetDefault("center.tls.server_key", "")
	viper.SetDefault("center.tls.ca_cert", "")
	viper.SetDefault("center.api_key", "")
	viper.SetDefault("center.jwt.secret_key", "")
	viper.SetDefault("center.jwt.token_duration", 24)
	viper.SetDefault("center.cluster.enabled", false)
	viper.SetDefault("center.cluster.etcd_endpoints", []string{"localhost:2379"})
	viper.SetDefault("center.cluster.lease_ttl", 30)
	viper.SetDefault("center.cluster.node_id", "")
	viper.SetDefault("center.cluster.node_addr", "")
	viper.SetDefault("center.portal.port", 8080)
	viper.SetDefault("center.portal.auth_user", "")
	viper.SetDefault("center.portal.auth_pass", "")
	viper.SetDefault("center.portal.redis_addr", "")
	viper.SetDefault("center.mixserver.port", 8081)
	viper.SetDefault("center.mixserver.auth_user", "")
	viper.SetDefault("center.mixserver.auth_pass", "")
	viper.SetDefault("center.rate_limit.enabled", true)
	viper.SetDefault("center.rate_limit.bucket_size", 100)
	viper.SetDefault("center.rate_limit.refill_rate", 50)
	viper.SetDefault("center.log.level", "info")
	viper.SetDefault("center.log.format", "json")
	viper.SetDefault("center.log.output", "stdout")
	viper.SetDefault("center.log.log_dir", "./logs")
	viper.SetDefault("center.log.max_size", 100)
	viper.SetDefault("center.log.max_backups", 10)
	viper.SetDefault("center.log.max_age", 7)
	viper.SetDefault("center.alerting.rules_path", "./rules")
	viper.SetDefault("center.alerting.check_interval", 30)
	viper.SetDefault("center.alerting.email.enabled", false)
	viper.SetDefault("center.alerting.email.smtp_host", "smtp.example.com")
	viper.SetDefault("center.alerting.email.smtp_port", 587)
	viper.SetDefault("center.alerting.email.smtp_username", "")
	viper.SetDefault("center.alerting.email.smtp_password", "")
	viper.SetDefault("center.alerting.email.from", "")
	viper.SetDefault("center.alerting.email.to", []string{})
	viper.SetDefault("center.alerting.webhook.enabled", false)
	viper.SetDefault("center.alerting.webhook.url", "")
	viper.SetDefault("center.alerting.webhook.headers", map[string]string{})
	viper.SetDefault("center.storage.dsn", "root:@tcp(127.0.0.1:4000)/cloud_flow?parseTime=true&charset=utf8mb4")
	viper.SetDefault("center.storage.retention_days", 7)


	viper.SetConfigName("config")
	viper.SetConfigType("yaml")
	viper.AddConfigPath("./configs")
	viper.AddConfigPath(".")
	if err := viper.ReadInConfig(); err != nil {
		if _, ok := err.(viper.ConfigFileNotFoundError); !ok {
			return nil, fmt.Errorf("读取配置文件失败: %w", err)
		}
	}

	viper.AutomaticEnv()
	viper.SetEnvKeyReplacer(strings.NewReplacer(".", "_"))

	// 精确绑定关键配置项到环境变量，避免模糊匹配
	_ = viper.BindEnv("center.jwt.secret_key", "CLOUD_FLOW_JWT_SECRET")
	_ = viper.BindEnv("center.api_key", "CLOUD_FLOW_API_KEY")
	_ = viper.BindEnv("center.storage.dsn", "CLOUD_FLOW_STORAGE_DSN")
	_ = viper.BindEnv("center.tls.enabled", "CLOUD_FLOW_TLS_ENABLED")
	_ = viper.BindEnv("center.tls.server_cert", "CLOUD_FLOW_TLS_SERVER_CERT")
	_ = viper.BindEnv("center.tls.server_key", "CLOUD_FLOW_TLS_SERVER_KEY")
	_ = viper.BindEnv("center.tls.ca_cert", "CLOUD_FLOW_TLS_CA_CERT")
	_ = viper.BindEnv("center.portal.port", "CLOUD_FLOW_PORTAL_PORT")
	_ = viper.BindEnv("center.log.level", "CLOUD_FLOW_LOG_LEVEL")
	_ = viper.BindEnv("center.data_dir", "CLOUD_FLOW_DATA_DIR")

	// 展开配置文件中的环境变量（如 ${VAR:-default}）
	expandEnvVarsInConfig()

	// C5 修复: 强制要求通过环境变量注入密钥，不再自动生成或写入配置文件
	// 原因:
	// 1. 多实例竞态: 多个 Center 实例同时启动时，各自生成不同密钥写入同一配置文件
	// 2. 安全风险: 密钥明文写入 YAML 文件，可能被日志、备份、配置中心泄露
	// 3. JWT 重启失效: 每次重启都生成新密钥，所有已登录用户被踢出
	apiKey := viper.GetString("center.api_key")
	if apiKey == "" {
		return nil, fmt.Errorf("API Key 未配置。请设置 CLOUD_FLOW_API_KEY 环境变量或 center.api_key 配置项")
	}

	jwtSecretKey := viper.GetString("center.jwt.secret_key")
	if jwtSecretKey == "" {
		return nil, fmt.Errorf("JWT Secret 未配置。请设置 CLOUD_FLOW_JWT_SECRET 环境变量或 center.jwt.secret_key 配置项")
	}

	return &Config{
		GRPCListenPort: viper.GetInt("center.grpc_listen_port"),
		DataDir:        viper.GetString("center.data_dir"),
		RetentionDays:  viper.GetInt("center.retention_days"),
		TLS: TLSConfig{
			Enabled:    viper.GetBool("center.tls.enabled"),
			ServerCert: viper.GetString("center.tls.server_cert"),
			ServerKey:  viper.GetString("center.tls.server_key"),
			CACert:     viper.GetString("center.tls.ca_cert"),
		},
		APIKey: apiKey,
		JWT: JWTConfig{
			SecretKey:     jwtSecretKey,
			TokenDuration: viper.GetInt("center.jwt.token_duration"),
		},
		Cluster: ClusterConfig{
			Enabled:       viper.GetBool("center.cluster.enabled"),
			EtcdEndpoints: viper.GetStringSlice("center.cluster.etcd_endpoints"),
			LeaseTTL:      viper.GetInt64("center.cluster.lease_ttl"),
			NodeID:        viper.GetString("center.cluster.node_id"),
			NodeAddr:      viper.GetString("center.cluster.node_addr"),
		},
		Portal: PortalConfig{
			Port:      viper.GetInt("center.portal.port"),
			AuthUser:  viper.GetString("center.portal.auth_user"),
			AuthPass:  viper.GetString("center.portal.auth_pass"),
			RedisAddr: viper.GetString("center.portal.redis_addr"),
		},
		MixServer: MixServerConfig{
			Port:     viper.GetInt("center.mixserver.port"),
			AuthUser: viper.GetString("center.mixserver.auth_user"),
			AuthPass: viper.GetString("center.mixserver.auth_pass"),
		},
		RateLimit: RateLimitConfig{
			Enabled:    viper.GetBool("center.rate_limit.enabled"),
			BucketSize: viper.GetInt("center.rate_limit.bucket_size"),
			RefillRate: viper.GetInt("center.rate_limit.refill_rate"),
		},
		Log: LogConfig{
			Level:      viper.GetString("center.log.level"),
			Format:     viper.GetString("center.log.format"),
			Output:     viper.GetString("center.log.output"),
			LogDir:     viper.GetString("center.log.log_dir"),
			MaxSize:    viper.GetInt("center.log.max_size"),
			MaxBackups: viper.GetInt("center.log.max_backups"),
			MaxAge:     viper.GetInt("center.log.max_age"),
		},
		Alerting: AlertingConfig{
			RulesPath:     viper.GetString("center.alerting.rules_path"),
			CheckInterval: viper.GetInt("center.alerting.check_interval"),
			Email: EmailConfig{
				Enabled:      viper.GetBool("center.alerting.email.enabled"),
				SMTPHost:     viper.GetString("center.alerting.email.smtp_host"),
				SMTPPort:     viper.GetInt("center.alerting.email.smtp_port"),
				SMTPUsername: viper.GetString("center.alerting.email.smtp_username"),
				SMTPPassword: viper.GetString("center.alerting.email.smtp_password"),
				From:         viper.GetString("center.alerting.email.from"),
				To:           viper.GetStringSlice("center.alerting.email.to"),
			},
			Webhook: WebhookConfig{
				Enabled: viper.GetBool("center.alerting.webhook.enabled"),
				URL:     viper.GetString("center.alerting.webhook.url"),
				Headers: viper.GetStringMapString("center.alerting.webhook.headers"),
			},
		},
		Storage: StorageConfig{
			DSN:     viper.GetString("center.storage.dsn"),
			RetDays: viper.GetInt("center.storage.retention_days"),
		},
	}, nil
}

func (c *Config) Summary() string {
	apiKeyMasked := ""
	if len(c.APIKey) > 8 {
		apiKeyMasked = c.APIKey[:4] + "****" + c.APIKey[len(c.APIKey)-4:]
	} else if c.APIKey != "" {
		apiKeyMasked = "****"
	}
	return fmt.Sprintf("ListenPort=%d, DataDir=%s, RetentionDays=%d, TLS=%v, APIKey=%s, PortalAuth=%v, LogLevel=%s",
		c.GRPCListenPort, c.DataDir, c.RetentionDays, c.TLS.Enabled, apiKeyMasked,
		c.Portal.AuthUser != "", c.Log.Level)
}
