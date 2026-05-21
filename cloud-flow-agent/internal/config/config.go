package config

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/spf13/viper"

	"cloud-flow/pkg/utils"
)

// loadAPIKey 从环境变量或配置文件加载 API Key
// 支持以下方式（按优先级排序）：
// 1. CLOUD_FLOW_API_KEY_FILE - 从文件读取（推荐用于 Docker Secrets / K8s Secrets）
// 2. CLOUD_FLOW_API_KEY - 直接环境变量
// 3. config.yaml - 配置文件（仅开发环境）
// 4. 空值（将在后续处理中提示错误）
func loadAPIKey() (string, error) {
	// 1. 从文件读取
	if keyFile := os.Getenv("CLOUD_FLOW_API_KEY_FILE"); keyFile != "" {
		data, err := os.ReadFile(keyFile)
		if err != nil {
			return "", fmt.Errorf("读取 API Key 文件失败: %w", err)
		}
		return strings.TrimSpace(string(data)), nil
	}

	// 2. 直接环境变量
	if apiKey := os.Getenv("CLOUD_FLOW_API_KEY"); apiKey != "" {
		return apiKey, nil
	}

	// 3. 从配置文件读取（仅开发环境）
	apiKey := viper.GetString("api_key")
	if apiKey != "" {
		// 生产环境警告
		if os.Getenv("CLOUD_FLOW_ENV") != "development" {
			fmt.Fprintln(os.Stderr, "⚠️  安全警告: API Key 从配置文件加载，仅建议用于开发环境")
		}
		return apiKey, nil
	}

	return "", nil
}

type CollectConfig struct {
	CPU     bool
	Memory  bool
	Network bool
	Disk    bool
}

type LogConfig struct {
	Level  string
	Format string
}

type TLSConfig struct {
	Enabled    bool
	ServerName string
	CACert     string
	ClientCert string
	ClientKey  string
}

// NetworkConfig 网络配置
type NetworkConfig struct {
	MgmtIface            string // 管理网卡接口名称
	LocalAddr            string // 本地绑定地址
	PreferredSourceIface string // 优先使用的源网卡
}

// TCPMetricsConfig TCP深度指标配置
type TCPMetricsConfig struct {
	Enabled        bool // 启用TCP深度指标
	ConnectLatency bool // TCP建连时延
	Retransmit     bool // TCP重传率
	ZeroWindow     bool // 零窗口事件
	QueueOverflow  bool // 队列溢出
	ConnectionFail bool // 连接失败
}

// HTTPMetricsConfig HTTP应用层指标配置
type HTTPMetricsConfig struct {
	Enabled         bool // 启用HTTP指标采集
	SuccessRate     bool // 请求成功率
	ResponseLatency bool // 响应时延
	ErrorRate       bool // 异常比例
	RequestCount    bool // 请求数
	ResponseCount   bool // 响应数
}

// BaseTrafficConfig 基础流量采集配置
type BaseTrafficConfig struct {
	Enabled        bool // 启用基础流量采集
	CollectBytes   bool // 采集字节数
	CollectPackets bool // 采集包数
}

// ProtocolParsingConfig 协议全字段解析配置
type ProtocolParsingConfig struct {
	Enabled   bool // 启用协议全字段解析
	HTTPFull  bool // 启用HTTP全字段解析
	DNSFull   bool // 启用DNS全字段解析
	MySQLFull bool // 启用MySQL全字段解析
}

// ResourceLimitConfig 资源限制配置
type ResourceLimitConfig struct {
	Enabled       bool    // 启用资源限制
	MaxCPUCore    float64 // 最大CPU核心数
	MaxMemoryMB   float64 // 最大内存使用(MB)
	MaxGoroutines int     // 最大协程数
}

// CircuitBreakerConfig 熔断配置
type CircuitBreakerConfig struct {
	Enabled        bool          // 启用熔断
	MaxFailures    int           // 最大连续失败次数
	ResetTimeout   time.Duration // 熔断恢复超时
	SilentDuration time.Duration // 静默持续时间
}

// PerfOptimizerConfig 性能优化配置
type PerfOptimizerConfig struct {
	Enabled         bool    // 启用性能优化
	SampleRate      float64 // 采样率(0.0-1.0)
	BatchSize       int     // 批处理大小
	MaxEventsPerSec int     // 每秒最大事件数
	HighLoadMode    bool    // 高负载模式(700Mbps, RPS1400)
	EnableAdaptive  bool    // 启用自适应采样
}

// EBPFConfig eBPF采集配置
type EBPFConfig struct {
	Enabled         bool                  // 启用eBPF采集
	TCPMetrics      TCPMetricsConfig      // TCP深度指标配置
	HTTPMetrics     HTTPMetricsConfig     // HTTP应用层指标配置
	BaseTraffic     BaseTrafficConfig     // 基础流量采集配置
	ProtocolParsing ProtocolParsingConfig // 协议全字段解析配置
	ResourceLimit   ResourceLimitConfig   // 资源限制配置
	CircuitBreaker  CircuitBreakerConfig  // 熔断配置
	PerfOptimizer   PerfOptimizerConfig   // 性能优化配置
}

type Config struct {
	ProbeID         string
	EdgeAddr        string
	MetricsPort     string
	HealthPort      string
	MaxRetries      int
	ConnectTimeout  int
	// TODO(AE-L01): ConnectTimeout 当前未被使用。grpcclient.NewClient 使用内部 rpcTimeout 常量。
	// 应将 cfg.ConnectTimeout 传递给 NewClient 以支持配置化超时。
	CollectInterval int
	BatchSize       int
	APIKey          string
	TLS             TLSConfig
	Collect         CollectConfig
	Log             LogConfig
	Network         NetworkConfig // 网络配置
	EBPF            EBPFConfig    // eBPF配置
}

func Load() (*Config, error) {
	// 解析 -config 命令行参数（支持指定配置文件路径）
	configFile := flag.String("config", "", "配置文件路径（默认自动查找 ./configs/config.yaml 或 ./config.yaml）")
	flag.Parse()

	viper.SetDefault("probe_id", "")
	viper.SetDefault("edge_addr", "edge:50051")
	viper.SetDefault("metrics_port", "9090")
	viper.SetDefault("health_port", "8080")
	viper.SetDefault("max_retries", 0)
	viper.SetDefault("connect_timeout", 30)
	viper.SetDefault("collect_interval", 10)
	viper.SetDefault("batch_size", 10)
	viper.SetDefault("api_key", "")
	viper.SetDefault("tls.enabled", false)
	viper.SetDefault("tls.server_name", "")
	viper.SetDefault("tls.ca_cert", "")
	viper.SetDefault("tls.client_cert", "")
	viper.SetDefault("tls.client_key", "")
	viper.SetDefault("collect.cpu", true)
	viper.SetDefault("collect.memory", true)
	viper.SetDefault("collect.network", true)
	viper.SetDefault("collect.disk", false)
	viper.SetDefault("log.level", "info")
	viper.SetDefault("log.format", "json")

	// 网络配置默认值
	viper.SetDefault("network.mgmt_iface", "")
	viper.SetDefault("network.local_addr", "")
	viper.SetDefault("network.preferred_source_iface", "")

	// eBPF配置默认值
	viper.SetDefault("ebpf.enabled", true)
	viper.SetDefault("ebpf.tcp_metrics.enabled", true)
	viper.SetDefault("ebpf.tcp_metrics.connect_latency", true)
	viper.SetDefault("ebpf.tcp_metrics.retransmit", true)
	viper.SetDefault("ebpf.tcp_metrics.zero_window", true)
	viper.SetDefault("ebpf.tcp_metrics.queue_overflow", true)
	viper.SetDefault("ebpf.tcp_metrics.connection_fail", true)
	viper.SetDefault("ebpf.http_metrics.enabled", true)
	viper.SetDefault("ebpf.http_metrics.success_rate", true)
	viper.SetDefault("ebpf.http_metrics.response_latency", true)
	viper.SetDefault("ebpf.http_metrics.error_rate", true)
	viper.SetDefault("ebpf.http_metrics.request_count", true)
	viper.SetDefault("ebpf.http_metrics.response_count", true)
	viper.SetDefault("ebpf.base_traffic.enabled", true)
	viper.SetDefault("ebpf.base_traffic.collect_bytes", true)
	viper.SetDefault("ebpf.base_traffic.collect_packets", true)

	// 协议全字段解析配置默认值
	viper.SetDefault("ebpf.protocol_parsing.enabled", false)
	viper.SetDefault("ebpf.protocol_parsing.http_full", false)
	viper.SetDefault("ebpf.protocol_parsing.dns_full", false)
	viper.SetDefault("ebpf.protocol_parsing.mysql_full", false)

	// 资源限制配置默认值
	viper.SetDefault("ebpf.resource_limit.enabled", true)
	viper.SetDefault("ebpf.resource_limit.max_cpu_core", 1.0)
	viper.SetDefault("ebpf.resource_limit.max_memory_mb", 1024)
	viper.SetDefault("ebpf.resource_limit.max_goroutines", 10000)

	// 熔断配置默认值
	viper.SetDefault("ebpf.circuit_breaker.enabled", true)
	viper.SetDefault("ebpf.circuit_breaker.max_failures", 3)
	viper.SetDefault("ebpf.circuit_breaker.reset_timeout", 30)
	viper.SetDefault("ebpf.circuit_breaker.silent_duration", 60)

	// 性能优化配置默认值
	viper.SetDefault("ebpf.perf_optimizer.enabled", true)
	viper.SetDefault("ebpf.perf_optimizer.sample_rate", 1.0)
	viper.SetDefault("ebpf.perf_optimizer.batch_size", 100)
	viper.SetDefault("ebpf.perf_optimizer.max_events_per_sec", 10000)
	viper.SetDefault("ebpf.perf_optimizer.high_load_mode", false)
	viper.SetDefault("ebpf.perf_optimizer.enable_adaptive", true)

	if *configFile != "" {
		// 用户指定了配置文件路径
		abs, err := filepath.Abs(*configFile)
		if err != nil {
			return nil, fmt.Errorf("解析配置文件路径失败: %w", err)
		}
		viper.SetConfigFile(abs)
	} else {
		viper.SetConfigName("config")
		viper.SetConfigType("yaml")
		viper.AddConfigPath("./configs")
		viper.AddConfigPath(".")
	}
	if err := viper.ReadInConfig(); err != nil {
		if _, ok := err.(viper.ConfigFileNotFoundError); !ok {
			return nil, fmt.Errorf("读取配置文件失败: %w", err)
		}
	}

	// AutomaticEnv 启用自动读取环境变量功能。
	// viper 会自动将配置键映射到同名环境变量（大写），
	// 例如 "probe_id" 会映射到环境变量 PROBE_ID。
	// SetEnvKeyReplacer 将键中的 "." 替换为 "_"，
	// 例如 "tls.enabled" 会映射到环境变量 TLS_ENABLED。
	// 环境变量的优先级高于配置文件中的值。
	viper.AutomaticEnv()
	viper.SetEnvKeyReplacer(strings.NewReplacer(".", "_"))

	// 安全加载 API Key
	apiKey, err := loadAPIKey()
	if err != nil {
		return nil, fmt.Errorf("加载 API Key 失败: %w", err)
	}

	cfg := &Config{
		ProbeID:         viper.GetString("probe_id"),
		EdgeAddr:        viper.GetString("edge_addr"),
		MetricsPort:     viper.GetString("metrics_port"),
		HealthPort:      viper.GetString("health_port"),
		MaxRetries:      viper.GetInt("max_retries"),
		ConnectTimeout:  viper.GetInt("connect_timeout"),
		CollectInterval: viper.GetInt("collect_interval"),
		BatchSize:       viper.GetInt("batch_size"),
		APIKey:          apiKey,
		TLS: TLSConfig{
			Enabled:    viper.GetBool("tls.enabled"),
			ServerName: viper.GetString("tls.server_name"),
			CACert:     viper.GetString("tls.ca_cert"),
			ClientCert: viper.GetString("tls.client_cert"),
			ClientKey:  viper.GetString("tls.client_key"),
		},
		Collect: CollectConfig{
			CPU:     viper.GetBool("collect.cpu"),
			Memory:  viper.GetBool("collect.memory"),
			Network: viper.GetBool("collect.network"),
			Disk:    viper.GetBool("collect.disk"),
		},
		Log: LogConfig{
			Level:  viper.GetString("log.level"),
			Format: viper.GetString("log.format"),
		},
		Network: NetworkConfig{
			MgmtIface:            viper.GetString("network.mgmt_iface"),
			LocalAddr:            viper.GetString("network.local_addr"),
			PreferredSourceIface: viper.GetString("network.preferred_source_iface"),
		},
		EBPF: EBPFConfig{
			Enabled: viper.GetBool("ebpf.enabled"),
			TCPMetrics: TCPMetricsConfig{
				Enabled:        viper.GetBool("ebpf.tcp_metrics.enabled"),
				ConnectLatency: viper.GetBool("ebpf.tcp_metrics.connect_latency"),
				Retransmit:     viper.GetBool("ebpf.tcp_metrics.retransmit"),
				ZeroWindow:     viper.GetBool("ebpf.tcp_metrics.zero_window"),
				QueueOverflow:  viper.GetBool("ebpf.tcp_metrics.queue_overflow"),
				ConnectionFail: viper.GetBool("ebpf.tcp_metrics.connection_fail"),
			},
			HTTPMetrics: HTTPMetricsConfig{
				Enabled:         viper.GetBool("ebpf.http_metrics.enabled"),
				SuccessRate:     viper.GetBool("ebpf.http_metrics.success_rate"),
				ResponseLatency: viper.GetBool("ebpf.http_metrics.response_latency"),
				ErrorRate:       viper.GetBool("ebpf.http_metrics.error_rate"),
				RequestCount:    viper.GetBool("ebpf.http_metrics.request_count"),
				ResponseCount:   viper.GetBool("ebpf.http_metrics.response_count"),
			},
			BaseTraffic: BaseTrafficConfig{
				Enabled:        viper.GetBool("ebpf.base_traffic.enabled"),
				CollectBytes:   viper.GetBool("ebpf.base_traffic.collect_bytes"),
				CollectPackets: viper.GetBool("ebpf.base_traffic.collect_packets"),
			},
			ProtocolParsing: ProtocolParsingConfig{
				Enabled:   viper.GetBool("ebpf.protocol_parsing.enabled"),
				HTTPFull:  viper.GetBool("ebpf.protocol_parsing.http_full"),
				DNSFull:   viper.GetBool("ebpf.protocol_parsing.dns_full"),
				MySQLFull: viper.GetBool("ebpf.protocol_parsing.mysql_full"),
			},
			ResourceLimit: ResourceLimitConfig{
				Enabled:       viper.GetBool("ebpf.resource_limit.enabled"),
				MaxCPUCore:    viper.GetFloat64("ebpf.resource_limit.max_cpu_core"),
				MaxMemoryMB:   viper.GetFloat64("ebpf.resource_limit.max_memory_mb"),
				MaxGoroutines: viper.GetInt("ebpf.resource_limit.max_goroutines"),
			},
			CircuitBreaker: CircuitBreakerConfig{
				Enabled:        viper.GetBool("ebpf.circuit_breaker.enabled"),
				MaxFailures:    viper.GetInt("ebpf.circuit_breaker.max_failures"),
				ResetTimeout:   viper.GetDuration("ebpf.circuit_breaker.reset_timeout") * time.Second,
				SilentDuration: viper.GetDuration("ebpf.circuit_breaker.silent_duration") * time.Second,
			},
			PerfOptimizer: PerfOptimizerConfig{
				Enabled:         viper.GetBool("ebpf.perf_optimizer.enabled"),
				SampleRate:      viper.GetFloat64("ebpf.perf_optimizer.sample_rate"),
				BatchSize:       viper.GetInt("ebpf.perf_optimizer.batch_size"),
				MaxEventsPerSec: viper.GetInt("ebpf.perf_optimizer.max_events_per_sec"),
				HighLoadMode:    viper.GetBool("ebpf.perf_optimizer.high_load_mode"),
				EnableAdaptive:  viper.GetBool("ebpf.perf_optimizer.enable_adaptive"),
			},
		},
	}

	if cfg.ProbeID == "" {
		hostname, _ := os.Hostname()
		if hostname == "" {
			hostname = "probe-unknown"
		}
		cfg.ProbeID = fmt.Sprintf("probe-%s", hostname)
	}

	return cfg, nil
}

// Summary 返回配置摘要字符串，用于启动日志
// 注意：API Key 会被脱敏处理，不会明文打印
func (c *Config) Summary() string {
	// API Key 脱敏处理
	apiKeyMasked := maskSecret(c.APIKey)

	return fmt.Sprintf("ProbeID=%s, EdgeAddr=%s, Interval=%ds, BatchSize=%d, APIKey=%s, CPU=%v, Mem=%v, Net=%v, MgmtIface=%s, EBPF=%v, ResourceLimit=%v",
		c.ProbeID, c.EdgeAddr, c.CollectInterval, c.BatchSize, apiKeyMasked,
		c.Collect.CPU, c.Collect.Memory, c.Collect.Network, c.Network.MgmtIface, c.EBPF.Enabled, c.EBPF.ResourceLimit.Enabled)
}

// maskSecret 对敏感字符串进行脱敏处理，委托给 pkg/utils.MaskSecret 统一实现
func maskSecret(s string) string {
	return utils.MaskSecret(s)
}
