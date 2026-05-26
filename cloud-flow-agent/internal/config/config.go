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
	MgmtIP               string // 管理网卡IP（新增：指定用于上报的IP）
	LocalAddr            string // 本地绑定地址（废弃，使用MgmtIP）
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
	UseCgroup     bool    // 使用cgroup v2进行系统级限制
}

// CircuitBreakerConfig 过载熔断配置
type CircuitBreakerConfig struct {
	Enabled        bool          // 启用过载熔断
	MaxFailures    int           // 最大连续失败次数（gRPC熔断用）
	ResetTimeout   time.Duration // 熔断恢复超时（gRPC熔断用）
	SilentDuration time.Duration // 静默持续时间（gRPC熔断用）

	// 过载熔断阈值
	CheckInterval             time.Duration // 资源检查间隔（默认3s）
	CPUDegradedThreshold      float64       // CPU降级阈值百分比（默认80）
	CPUSilentThreshold        float64       // CPU静默阈值百分比（默认95）
	MemDegradedThreshold      float64       // 内存降级阈值百分比（默认90）
	MemSilentThreshold        float64       // 内存静默阈值百分比（默认95）
	CPUDegradedDuration       time.Duration // CPU持续超限触发降级的持续时间（默认30s）
	CPURecoverThreshold       float64       // CPU恢复阈值百分比（默认80）
	MemRecoverThreshold       float64       // 内存恢复阈值百分比（默认85）
	SilentCPURecoverThreshold float64       // 静默恢复CPU阈值百分比（默认70）
	SilentMemRecoverThreshold float64       // 静默恢复内存阈值百分比（默认80）
}

// SelfMonitorConfig 自监控配置
type SelfMonitorConfig struct {
	Enabled          bool          // 启用自监控
	CollectInterval  time.Duration // 采集间隔（默认10秒）
	ReportInterval   time.Duration // 上报间隔（默认10秒）
	HeartbeatTimeout time.Duration // 心跳超时判定（默认5秒）

	// 告警阈值
	AlertHeartbeatFailCount   int     // 连续心跳失败次数触发告警
	AlertCPUPercent           float64 // CPU使用率告警阈值(%)
	AlertMemoryPercent        float64 // 内存使用率告警阈值(%)
	AlertPacketDropRate       float64 // 丢包率告警阈值(%)
	AlertReportSuccessRateMin float64 // 上报成功率最低阈值(%)
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

// CPUProfilerConfig ON-CPU剖析配置
type CPUProfilerConfig struct {
	Enabled      bool   // 启用ON-CPU剖析
	SampleFreq   int    // 采样频率(Hz), 默认99
	TargetPID    uint32 // 目标进程ID, 0=全部进程
	MaxStackDepth int   // 最大栈深度, 默认127
	DurationSec  int    // 单次剖析持续时间(秒), 0=连续模式
	OutputDir    string // 火焰图输出目录
	AutoDetect   bool   // 自动检测进程语言类型
}

// SQLAggregatorConfig SQL聚合分析配置
type SQLAggregatorConfig struct {
	Enabled              bool    // 启用SQL聚合分析
	SlowQueryThresholdMs uint64  // 慢SQL阈值(毫秒), 默认1000ms
	EnableCorrelation    bool    // 启用进程性能关联分析
	MaxSnapshots         int     // 最大性能快照数
	CPUMaxThreshold      float64 // CPU告警阈值(%)
	MemoryMaxMB          float64 // 内存告警阈值(MB)
	LatencyMaxMs         float64 // 延迟告警阈值(ms)
	SlowQueryMax         uint64  // 慢查询告警阈值
	ConnMax              uint64  // 连接数告警阈值
}

// StorageConfig 历史数据存储配置
type StorageConfig struct {
	Enabled             bool    // 启用历史数据存储
	BaseDir             string  // 存储目录
	RetentionDays       int     // 默认保留天数(默认60天)
	ChunkSize           int     // 数据块大小(条数)
	WriteBufferSize     int     // 写缓冲区大小
	CompressionType     string  // 压缩类型: zstd, lz4, delta
	EnableIndex         bool    // 启用索引
	RetentionIntervalMin int    // 保留期检查间隔(分钟)
	
	// 自定义类型保留天数
	MetricRetentionDays  int // 指标数据保留天数
	LogRetentionDays     int // 日志数据保留天数
	TraceRetentionDays   int // 追踪数据保留天数
	EventRetentionDays   int // 事件数据保留天数
	
	// 性能目标
	WriteRateMin         int // 写入速率目标(条/秒)
	QueryLatencyMaxMs    int // 查询延迟目标(毫秒)
}

// AlertConfig 告警配置
type AlertConfig struct {
	Enabled            bool          `yaml:"enabled" json:"enabled"`
	EvaluationInterval time.Duration `yaml:"evaluation_interval" json:"evaluation_interval"`
	ResolveTimeout     time.Duration `yaml:"resolve_timeout" json:"resolve_timeout"`
	EnableAutoResolve  bool          `yaml:"enable_auto_resolve" json:"enable_auto_resolve"`
	MaxActiveAlerts    int           `yaml:"max_active_alerts" json:"max_active_alerts"`
	
	// 内置模板开关
	EnableLatencyAlert    bool `yaml:"enable_latency_alert" json:"enable_latency_alert"`
	EnablePacketLossAlert bool `yaml:"enable_packet_loss_alert" json:"enable_packet_loss_alert"`
	EnableRetransmitAlert bool `yaml:"enable_retransmit_alert" json:"enable_retransmit_alert"`
	EnableCPUAlert        bool `yaml:"enable_cpu_alert" json:"enable_cpu_alert"`
	EnableMemoryAlert     bool `yaml:"enable_memory_alert" json:"enable_memory_alert"`
	EnableErrorRateAlert  bool `yaml:"enable_error_rate_alert" json:"enable_error_rate_alert"`
	
	// 通知配置
	NotifyKafka   KafkaNotifyConfig   `yaml:"notify_kafka" json:"notify_kafka"`
	NotifyAPI     APINotifyConfig     `yaml:"notify_api" json:"notify_api"`
	NotifyWebhook WebhookNotifyConfig `yaml:"notify_webhook" json:"notify_webhook"`
}

// KafkaNotifyConfig Kafka通知配置
type KafkaNotifyConfig struct {
	Enabled bool     `yaml:"enabled" json:"enabled"`
	Brokers []string `yaml:"brokers" json:"brokers"`
	Topic   string   `yaml:"topic" json:"topic"`
}

// APINotifyConfig API通知配置
type APINotifyConfig struct {
	Enabled  bool              `yaml:"enabled" json:"enabled"`
	URL      string            `yaml:"url" json:"url"`
	Headers  map[string]string `yaml:"headers" json:"headers"`
	AuthType string            `yaml:"auth_type" json:"auth_type"`
	AuthToken string           `yaml:"auth_token" json:"auth_token"`
}

// WebhookNotifyConfig Webhook通知配置
type WebhookNotifyConfig struct {
	Enabled bool              `yaml:"enabled" json:"enabled"`
	URL     string            `yaml:"url" json:"url"`
	Secret  string            `yaml:"secret" json:"secret"`
	Headers map[string]string `yaml:"headers" json:"headers"`
}

// TopologyConfig 拓扑配置
type TopologyConfig struct {
	Enabled         bool          `yaml:"enabled" json:"enabled"`
	RefreshInterval time.Duration `yaml:"refresh_interval" json:"refresh_interval"`
	AutoDiscovery   bool          `yaml:"auto_discovery" json:"auto_discovery"`
	DefaultLayout   string        `yaml:"default_layout" json:"default_layout"`
	MaxNodes        int           `yaml:"max_nodes" json:"max_nodes"`
	
	// 层级开关
	IncludePods     bool `yaml:"include_pods" json:"include_pods"`
	IncludeVMs      bool `yaml:"include_vms" json:"include_vms"`
	IncludePhysical bool `yaml:"include_physical" json:"include_physical"`
	
	// 告警集成
	EnableAlertIntegration bool `yaml:"enable_alert_integration" json:"enable_alert_integration"`
}

// TenantConfig 租户配置
type TenantConfig struct {
	Enabled       bool `yaml:"enabled" json:"enabled"`
	MultiTenant   bool `yaml:"multi_tenant" json:"multi_tenant"`
	MaxTenants    int  `yaml:"max_tenants" json:"max_tenants"`
	MaxUsersPerTenant int `yaml:"max_users_per_tenant" json:"max_users_per_tenant"`
}

// DashboardConfig 仪表盘配置
type DashboardConfig struct {
	Enabled           bool          `yaml:"enabled" json:"enabled"`
	RefreshInterval   time.Duration `yaml:"refresh_interval" json:"refresh_interval"`
	EnableDrillDown   bool          `yaml:"enable_drill_down" json:"enable_drill_down"`
	MaxAssetsPerPage  int           `yaml:"max_assets_per_page" json:"max_assets_per_page"`
}

// MetricsConfig 指标模块配置
type MetricsConfig struct {
	Enabled           bool          `yaml:"enabled" json:"enabled"`
	NetworkEnabled    bool          `yaml:"network_enabled" json:"network_enabled"`
	AppEnabled        bool          `yaml:"app_enabled" json:"app_enabled"`
	SQLEnabled        bool          `yaml:"sql_enabled" json:"sql_enabled"`
	MaxDataPoints     int           `yaml:"max_data_points" json:"max_data_points"`
	RetentionTime     time.Duration `yaml:"retention_time" json:"retention_time"`
}

// CMDBConfig CMDB对接配置
type CMDBConfig struct {
	Enabled       bool          `yaml:"enabled" json:"enabled"`
	Type          string        `yaml:"type" json:"type"`                   // 数据源类型: http/api
	Endpoint      string        `yaml:"endpoint" json:"endpoint"`           // CMDB服务地址
	AuthType      string        `yaml:"auth_type" json:"auth_type"`         // 认证方式: none/bearer/token/apikey/basic
	AuthToken     string        `yaml:"auth_token" json:"auth_token"`
	APIKey        string        `yaml:"api_key" json:"api_key"`
	Username      string        `yaml:"username" json:"username"`
	Password      string        `yaml:"password" json:"password"`
	SyncInterval  time.Duration `yaml:"sync_interval" json:"sync_interval"`
	FullSyncStart bool          `yaml:"full_sync_on_start" json:"full_sync_on_start"`
	Incremental   bool          `yaml:"incremental_sync" json:"incremental_sync"`
	LabelSync     bool          `yaml:"label_sync" json:"label_sync"`
	ConfigSync    bool          `yaml:"config_sync" json:"config_sync"`
	RelationSync  bool          `yaml:"relation_sync" json:"relation_sync"`
	ConflictPolicy string       `yaml:"conflict_policy" json:"conflict_policy"` // cmdb_wins/agent_wins/merge
	Timeout       time.Duration `yaml:"timeout" json:"timeout"`
	BatchSize     int           `yaml:"batch_size" json:"batch_size"`
}

// TraceConfig 追踪和火焰图配置
type TraceConfig struct {
	Enabled           bool          `yaml:"enabled" json:"enabled"`
	SampleRate        float64       `yaml:"sample_rate" json:"sample_rate"`
	MaxTraces         int           `yaml:"max_traces" json:"max_traces"`
	MaxSpansPerTrace  int           `yaml:"max_spans_per_trace" json:"max_spans_per_trace"`
	RetentionTime     time.Duration `yaml:"retention_time" json:"retention_time"`
	EnableCorrelation bool          `yaml:"enable_correlation" json:"enable_correlation"`
	FlameGraphEnabled bool          `yaml:"flamegraph_enabled" json:"flamegraph_enabled"`
	FlameSampleFreq   int           `yaml:"flame_sample_freq" json:"flame_sample_freq"`
	FlameMaxDepth     int           `yaml:"flame_max_depth" json:"flame_max_depth"`
	FlameDurationSec  int           `yaml:"flame_duration_sec" json:"flame_duration_sec"`
	FlameOutputDir    string        `yaml:"flame_output_dir" json:"flame_output_dir"`
	FlameMaxStored    int           `yaml:"flame_max_stored" json:"flame_max_stored"`
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
	SelfMonitor     SelfMonitorConfig     // 自监控配置
	VXLAN             VXLANConfig           // VXLAN解封装配置
	PluginFramework   PluginFrameworkConfig // 插件化协议解析框架配置
	DropMonitor       DropMonitorConfig     // 丢包监控配置
	NTP               NTPConfig             // NTP时钟校准配置
	PerfOptimizer     PerfOptimizerConfig   // 性能优化配置
	CPUProfiler     CPUProfilerConfig     // ON-CPU剖析配置
	SQLAggregator   SQLAggregatorConfig   // SQL聚合分析配置
}

// VXLANConfig VXLAN解封装配置
type VXLANConfig struct {
	Enabled           bool   // 启用VXLAN解封装
	EnableTapMirror   bool   // 启用TAP设备镜像
	TapDeviceName     string // TAP设备名称
	ParseInnerProtocol bool  // 解析内层协议
}

// PluginFrameworkConfig 插件化协议解析框架配置
type PluginFrameworkConfig struct {
	Enabled        bool          // 启用插件框架
	PluginDir      string        // 插件目录路径
	AutoDiscovery  bool          // 自动发现并加载插件
	CheckInterval  time.Duration // 健康检查间隔
	MaxMemoryMB    int           // 单插件内存限制(MB)
	GRPCTimeout    time.Duration // gRPC通信超时
	EnableBuiltin  bool          // 启用内置插件(Oracle/PG/Redis/Kafka/Dubbo)
}

// DropMonitorConfig 丢包监控配置
type DropMonitorConfig struct {
	Enabled          bool          // 启用丢包监控
	EnableKernelDrop bool          // 启用内核态丢包监控
	EnableUserDrop   bool          // 启用用户态丢包监控
	RingBufSize      int           // Ring Buffer大小
	SampleRate       float64       // 采样率(0-1)
	SnapshotInterval time.Duration // 快照间隔
	AlertThreshold   float64       // 丢包率告警阈值(%)
}

// NTPConfig NTP时钟校准配置
type NTPConfig struct {
	Enabled       bool          // 启用NTP校准
	Mode          string        // 同步模式: grpc/ntp/auto
	NTPServers    []string      // NTP服务器列表
	SyncInterval  time.Duration // 同步间隔
	MaxOffset     time.Duration // 最大允许偏差
	AdjustStep    bool          // 支持步进调整
	AdjustSlew    bool          // 支持渐进调整
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
	Storage         StorageConfig // 历史数据存储配置
	Alert           AlertConfig   // 告警配置
	Topology        TopologyConfig // 拓扑配置
	Tenant          TenantConfig   // 租户配置
	Dashboard       DashboardConfig // 仪表盘配置
	Metrics         MetricsConfig   // 指标模块配置
	CMDB            CMDBConfig      // CMDB对接配置
	Trace           TraceConfig     // 追踪和火焰图配置

	// L1 修复: 可配置的定时任务间隔
	FlushInterval         time.Duration // 日志刷新间隔（默认 30s）
	ReconnectBaseDelay    time.Duration // 重连基础延迟（默认 2s）
	ReconnectMaxDelay     time.Duration // 重连最大延迟（默认 30s）
	MaxReconnectAttempts  int           // 最大重连次数（默认 10）
	MaxBufferLimit        int           // 缓冲区上限（默认 1000）
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

	// L1 修复: 可配置的定时任务间隔默认值
	viper.SetDefault("flush_interval", "30s")
	viper.SetDefault("reconnect_base_delay", "2s")
	viper.SetDefault("reconnect_max_delay", "30s")
	viper.SetDefault("max_reconnect_attempts", 10)
	viper.SetDefault("max_buffer_limit", 1000)
	viper.SetDefault("collect.disk", false)
	viper.SetDefault("log.level", "info")
	viper.SetDefault("log.format", "json")

	// 网络配置默认值
	viper.SetDefault("network.mgmt_iface", "")
	viper.SetDefault("network.mgmt_ip", "")
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
	viper.SetDefault("ebpf.resource_limit.use_cgroup", true)

	// 熔断配置默认值
	viper.SetDefault("ebpf.circuit_breaker.enabled", true)
	viper.SetDefault("ebpf.circuit_breaker.max_failures", 3)
	viper.SetDefault("ebpf.circuit_breaker.reset_timeout", 30)
	viper.SetDefault("ebpf.circuit_breaker.silent_duration", 60)
	// 过载熔断阈值默认值
	viper.SetDefault("ebpf.circuit_breaker.check_interval", "3s")
	viper.SetDefault("ebpf.circuit_breaker.cpu_degraded_threshold", 80.0)
	viper.SetDefault("ebpf.circuit_breaker.cpu_silent_threshold", 95.0)
	viper.SetDefault("ebpf.circuit_breaker.mem_degraded_threshold", 90.0)
	viper.SetDefault("ebpf.circuit_breaker.mem_silent_threshold", 95.0)
	viper.SetDefault("ebpf.circuit_breaker.cpu_degraded_duration", "30s")
	viper.SetDefault("ebpf.circuit_breaker.cpu_recover_threshold", 80.0)
	viper.SetDefault("ebpf.circuit_breaker.mem_recover_threshold", 85.0)
	viper.SetDefault("ebpf.circuit_breaker.silent_cpu_recover_threshold", 70.0)
	viper.SetDefault("ebpf.circuit_breaker.silent_mem_recover_threshold", 80.0)

	// 自监控配置默认值
	viper.SetDefault("ebpf.self_monitor.enabled", true)
	viper.SetDefault("ebpf.self_monitor.collect_interval", "10s")
	viper.SetDefault("ebpf.self_monitor.report_interval", "10s")
	viper.SetDefault("ebpf.self_monitor.heartbeat_timeout", "5s")
	viper.SetDefault("ebpf.self_monitor.alert_heartbeat_fail_count", 3)
	viper.SetDefault("ebpf.self_monitor.alert_cpu_percent", 80.0)
	viper.SetDefault("ebpf.self_monitor.alert_memory_percent", 90.0)
	viper.SetDefault("ebpf.self_monitor.alert_packet_drop_rate", 5.0)
	viper.SetDefault("ebpf.self_monitor.alert_report_success_rate_min", 95.0)

	// VXLAN解封装配置默认值
	viper.SetDefault("ebpf.vxlan.enabled", false)
	viper.SetDefault("ebpf.vxlan.enable_tap_mirror", false)
	viper.SetDefault("ebpf.vxlan.tap_device_name", "vxlan-tap0")
	viper.SetDefault("ebpf.vxlan.parse_inner_protocol", true)

	// 插件化协议解析框架配置默认值
	viper.SetDefault("ebpf.plugin_framework.enabled", false)
	viper.SetDefault("ebpf.plugin_framework.plugin_dir", "/opt/cloud-flow-agent/plugins")
	viper.SetDefault("ebpf.plugin_framework.auto_discovery", true)
	viper.SetDefault("ebpf.plugin_framework.check_interval", "30s")
	viper.SetDefault("ebpf.plugin_framework.max_memory_mb", 256)
	viper.SetDefault("ebpf.plugin_framework.grpc_timeout", "5s")
	viper.SetDefault("ebpf.plugin_framework.enable_builtin", true)

	// 丢包监控配置默认值
	viper.SetDefault("ebpf.drop_monitor.enabled", false)
	viper.SetDefault("ebpf.drop_monitor.enable_kernel_drop", true)
	viper.SetDefault("ebpf.drop_monitor.enable_user_drop", true)
	viper.SetDefault("ebpf.drop_monitor.ringbuf_size", 262144) // 256KB
	viper.SetDefault("ebpf.drop_monitor.sample_rate", 1.0)
	viper.SetDefault("ebpf.drop_monitor.snapshot_interval", "10s")
	viper.SetDefault("ebpf.drop_monitor.alert_threshold", 1.0) // 1%

	// NTP时钟校准配置默认值
	viper.SetDefault("ebpf.ntp.enabled", false)
	viper.SetDefault("ebpf.ntp.mode", "auto") // auto/grpc/ntp
	viper.SetDefault("ebpf.ntp.ntp_servers", []string{"pool.ntp.org", "time.windows.com"})
	viper.SetDefault("ebpf.ntp.sync_interval", "5m")
	viper.SetDefault("ebpf.ntp.max_offset", "100ms")
	viper.SetDefault("ebpf.ntp.adjust_step", true)
	viper.SetDefault("ebpf.ntp.adjust_slew", true)

	// 性能优化配置默认值
	viper.SetDefault("ebpf.perf_optimizer.enabled", true)
	viper.SetDefault("ebpf.perf_optimizer.sample_rate", 1.0)
	viper.SetDefault("ebpf.perf_optimizer.batch_size", 100)
	viper.SetDefault("ebpf.perf_optimizer.max_events_per_sec", 10000)
	viper.SetDefault("ebpf.perf_optimizer.high_load_mode", false)
	viper.SetDefault("ebpf.perf_optimizer.enable_adaptive", true)

	// ON-CPU剖析配置默认值
	viper.SetDefault("ebpf.cpu_profiler.enabled", false)
	viper.SetDefault("ebpf.cpu_profiler.sample_freq", 99)
	viper.SetDefault("ebpf.cpu_profiler.target_pid", 0)
	viper.SetDefault("ebpf.cpu_profiler.max_stack_depth", 127)
	viper.SetDefault("ebpf.cpu_profiler.duration_sec", 0)
	viper.SetDefault("ebpf.cpu_profiler.output_dir", "/var/log/cloud-flow-agent/profiler")
	viper.SetDefault("ebpf.cpu_profiler.auto_detect", true)

	// SQL聚合分析配置默认值
	viper.SetDefault("ebpf.sql_aggregator.enabled", false)
	viper.SetDefault("ebpf.sql_aggregator.slow_query_threshold_ms", 1000)
	viper.SetDefault("ebpf.sql_aggregator.enable_correlation", true)
	viper.SetDefault("ebpf.sql_aggregator.max_snapshots", 60)
	viper.SetDefault("ebpf.sql_aggregator.cpu_max_threshold", 80.0)
	viper.SetDefault("ebpf.sql_aggregator.memory_max_mb", 1024.0)
	viper.SetDefault("ebpf.sql_aggregator.latency_max_ms", 1000.0)
	viper.SetDefault("ebpf.sql_aggregator.slow_query_max", 10)
	viper.SetDefault("ebpf.sql_aggregator.conn_max", 100)

	// 历史数据存储配置默认值
	viper.SetDefault("storage.enabled", false)
	viper.SetDefault("storage.base_dir", "/var/lib/cloud-flow-agent/storage")
	viper.SetDefault("storage.retention_days", 60)        // 默认60天
	viper.SetDefault("storage.chunk_size", 10000)
	viper.SetDefault("storage.write_buffer_size", 50000)
	viper.SetDefault("storage.compression_type", "zstd")   // 高压缩率
	viper.SetDefault("storage.enable_index", true)
	viper.SetDefault("storage.retention_interval_min", 60)
	viper.SetDefault("storage.metric_retention_days", 60)
	viper.SetDefault("storage.log_retention_days", 30)
	viper.SetDefault("storage.trace_retention_days", 7)
	viper.SetDefault("storage.event_retention_days", 90)
	viper.SetDefault("storage.write_rate_min", 50000)      // 写入≥5万条/秒
	viper.SetDefault("storage.query_latency_max_ms", 1000) // 1亿行查询≤1秒

	// 告警配置默认值
	viper.SetDefault("alert.enabled", false)
	viper.SetDefault("alert.evaluation_interval", "1m")
	viper.SetDefault("alert.resolve_timeout", "5m")
	viper.SetDefault("alert.enable_auto_resolve", true)
	viper.SetDefault("alert.max_active_alerts", 1000)
	
	// 内置模板开关
	viper.SetDefault("alert.enable_latency_alert", true)
	viper.SetDefault("alert.enable_packet_loss_alert", true)
	viper.SetDefault("alert.enable_retransmit_alert", true)
	viper.SetDefault("alert.enable_cpu_alert", true)
	viper.SetDefault("alert.enable_memory_alert", true)
	viper.SetDefault("alert.enable_error_rate_alert", true)
	
	// Kafka通知配置
	viper.SetDefault("alert.notify_kafka.enabled", false)
	viper.SetDefault("alert.notify_kafka.brokers", []string{"localhost:9092"})
	viper.SetDefault("alert.notify_kafka.topic", "alerts")
	
	// API通知配置
	viper.SetDefault("alert.notify_api.enabled", false)
	viper.SetDefault("alert.notify_api.url", "")
	viper.SetDefault("alert.notify_api.auth_type", "")
	
	// Webhook通知配置
	viper.SetDefault("alert.notify_webhook.enabled", false)
	viper.SetDefault("alert.notify_webhook.url", "")
	
	// 拓扑配置默认值
	viper.SetDefault("topology.enabled", false)
	viper.SetDefault("topology.refresh_interval", "5m")
	viper.SetDefault("topology.auto_discovery", true)
	viper.SetDefault("topology.default_layout", "vertical")
	viper.SetDefault("topology.max_nodes", 1000)
	viper.SetDefault("topology.include_pods", true)
	viper.SetDefault("topology.include_vms", true)
	viper.SetDefault("topology.include_physical", true)
	viper.SetDefault("topology.enable_alert_integration", true)

	// 租户配置默认值
	viper.SetDefault("tenant.enabled", false)
	viper.SetDefault("tenant.multi_tenant", false)
	viper.SetDefault("tenant.max_tenants", 100)
	viper.SetDefault("tenant.max_users_per_tenant", 50)

	// 仪表盘配置默认值
	viper.SetDefault("dashboard.enabled", true)
	viper.SetDefault("dashboard.refresh_interval", "30s")
	viper.SetDefault("dashboard.enable_drill_down", true)
	viper.SetDefault("dashboard.max_assets_per_page", 100)

	// 指标模块配置默认值
	viper.SetDefault("metrics.enabled", true)
	viper.SetDefault("metrics.network_enabled", true)
	viper.SetDefault("metrics.app_enabled", true)
	viper.SetDefault("metrics.sql_enabled", true)
	viper.SetDefault("metrics.max_data_points", 1440)
	viper.SetDefault("metrics.retention_time", "24h")

	// CMDB对接配置默认值
	viper.SetDefault("cmdb.enabled", false)
	viper.SetDefault("cmdb.type", "http")
	viper.SetDefault("cmdb.endpoint", "")
	viper.SetDefault("cmdb.auth_type", "none")
	viper.SetDefault("cmdb.sync_interval", "5m")
	viper.SetDefault("cmdb.full_sync_on_start", true)
	viper.SetDefault("cmdb.incremental_sync", true)
	viper.SetDefault("cmdb.label_sync", true)
	viper.SetDefault("cmdb.config_sync", true)
	viper.SetDefault("cmdb.relation_sync", true)
	viper.SetDefault("cmdb.conflict_policy", "cmdb_wins")
	viper.SetDefault("cmdb.timeout", "30s")
	viper.SetDefault("cmdb.batch_size", 100)

	// 追踪和火焰图配置默认值
	viper.SetDefault("trace.enabled", false)
	viper.SetDefault("trace.sample_rate", 1.0)
	viper.SetDefault("trace.max_traces", 10000)
	viper.SetDefault("trace.max_spans_per_trace", 1000)
	viper.SetDefault("trace.retention_time", "24h")
	viper.SetDefault("trace.enable_correlation", true)
	viper.SetDefault("trace.flamegraph_enabled", false)
	viper.SetDefault("trace.flame_sample_freq", 99)
	viper.SetDefault("trace.flame_max_depth", 127)
	viper.SetDefault("trace.flame_duration_sec", 30)
	viper.SetDefault("trace.flame_output_dir", "/var/log/cloud-flow-agent/flamegraph")
	viper.SetDefault("trace.flame_max_stored", 100)

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
			MgmtIP:               viper.GetString("network.mgmt_ip"),
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
			UseCgroup:     viper.GetBool("ebpf.resource_limit.use_cgroup"),
		},
			CircuitBreaker: CircuitBreakerConfig{
				Enabled:        viper.GetBool("ebpf.circuit_breaker.enabled"),
				MaxFailures:    viper.GetInt("ebpf.circuit_breaker.max_failures"),
				ResetTimeout:   viper.GetDuration("ebpf.circuit_breaker.reset_timeout") * time.Second,
				SilentDuration: viper.GetDuration("ebpf.circuit_breaker.silent_duration") * time.Second,
				// 过载熔断阈值
				CheckInterval:             viper.GetDuration("ebpf.circuit_breaker.check_interval"),
				CPUDegradedThreshold:      viper.GetFloat64("ebpf.circuit_breaker.cpu_degraded_threshold"),
				CPUSilentThreshold:        viper.GetFloat64("ebpf.circuit_breaker.cpu_silent_threshold"),
				MemDegradedThreshold:      viper.GetFloat64("ebpf.circuit_breaker.mem_degraded_threshold"),
				MemSilentThreshold:        viper.GetFloat64("ebpf.circuit_breaker.mem_silent_threshold"),
				CPUDegradedDuration:       viper.GetDuration("ebpf.circuit_breaker.cpu_degraded_duration"),
				CPURecoverThreshold:       viper.GetFloat64("ebpf.circuit_breaker.cpu_recover_threshold"),
				MemRecoverThreshold:       viper.GetFloat64("ebpf.circuit_breaker.mem_recover_threshold"),
				SilentCPURecoverThreshold: viper.GetFloat64("ebpf.circuit_breaker.silent_cpu_recover_threshold"),
				SilentMemRecoverThreshold: viper.GetFloat64("ebpf.circuit_breaker.silent_mem_recover_threshold"),
			},
			SelfMonitor: SelfMonitorConfig{
				Enabled:                   viper.GetBool("ebpf.self_monitor.enabled"),
				CollectInterval:           viper.GetDuration("ebpf.self_monitor.collect_interval"),
				ReportInterval:            viper.GetDuration("ebpf.self_monitor.report_interval"),
				HeartbeatTimeout:          viper.GetDuration("ebpf.self_monitor.heartbeat_timeout"),
				AlertHeartbeatFailCount:   viper.GetInt("ebpf.self_monitor.alert_heartbeat_fail_count"),
				AlertCPUPercent:           viper.GetFloat64("ebpf.self_monitor.alert_cpu_percent"),
				AlertMemoryPercent:        viper.GetFloat64("ebpf.self_monitor.alert_memory_percent"),
				AlertPacketDropRate:       viper.GetFloat64("ebpf.self_monitor.alert_packet_drop_rate"),
				AlertReportSuccessRateMin: viper.GetFloat64("ebpf.self_monitor.alert_report_success_rate_min"),
			},
			VXLAN: VXLANConfig{
				Enabled:            viper.GetBool("ebpf.vxlan.enabled"),
				EnableTapMirror:    viper.GetBool("ebpf.vxlan.enable_tap_mirror"),
				TapDeviceName:      viper.GetString("ebpf.vxlan.tap_device_name"),
				ParseInnerProtocol: viper.GetBool("ebpf.vxlan.parse_inner_protocol"),
			},
			PluginFramework: PluginFrameworkConfig{
			Enabled:       viper.GetBool("ebpf.plugin_framework.enabled"),
			PluginDir:     viper.GetString("ebpf.plugin_framework.plugin_dir"),
			AutoDiscovery: viper.GetBool("ebpf.plugin_framework.auto_discovery"),
			CheckInterval: viper.GetDuration("ebpf.plugin_framework.check_interval"),
			MaxMemoryMB:   viper.GetInt("ebpf.plugin_framework.max_memory_mb"),
			GRPCTimeout:   viper.GetDuration("ebpf.plugin_framework.grpc_timeout"),
			EnableBuiltin: viper.GetBool("ebpf.plugin_framework.enable_builtin"),
		},
		DropMonitor: DropMonitorConfig{
			Enabled:          viper.GetBool("ebpf.drop_monitor.enabled"),
			EnableKernelDrop: viper.GetBool("ebpf.drop_monitor.enable_kernel_drop"),
			EnableUserDrop:   viper.GetBool("ebpf.drop_monitor.enable_user_drop"),
			RingBufSize:      viper.GetInt("ebpf.drop_monitor.ringbuf_size"),
			SampleRate:       viper.GetFloat64("ebpf.drop_monitor.sample_rate"),
			SnapshotInterval: viper.GetDuration("ebpf.drop_monitor.snapshot_interval"),
			AlertThreshold:   viper.GetFloat64("ebpf.drop_monitor.alert_threshold"),
		},
		NTP: NTPConfig{
			Enabled:      viper.GetBool("ebpf.ntp.enabled"),
			Mode:         viper.GetString("ebpf.ntp.mode"),
			NTPServers:   viper.GetStringSlice("ebpf.ntp.ntp_servers"),
			SyncInterval: viper.GetDuration("ebpf.ntp.sync_interval"),
			MaxOffset:    viper.GetDuration("ebpf.ntp.max_offset"),
			AdjustStep:   viper.GetBool("ebpf.ntp.adjust_step"),
			AdjustSlew:   viper.GetBool("ebpf.ntp.adjust_slew"),
		},
		PerfOptimizer: PerfOptimizerConfig{
				Enabled:         viper.GetBool("ebpf.perf_optimizer.enabled"),
				SampleRate:      viper.GetFloat64("ebpf.perf_optimizer.sample_rate"),
				BatchSize:       viper.GetInt("ebpf.perf_optimizer.batch_size"),
				MaxEventsPerSec: viper.GetInt("ebpf.perf_optimizer.max_events_per_sec"),
				HighLoadMode:    viper.GetBool("ebpf.perf_optimizer.high_load_mode"),
				EnableAdaptive:  viper.GetBool("ebpf.perf_optimizer.enable_adaptive"),
			},
			CPUProfiler: CPUProfilerConfig{
				Enabled:       viper.GetBool("ebpf.cpu_profiler.enabled"),
				SampleFreq:    viper.GetInt("ebpf.cpu_profiler.sample_freq"),
				TargetPID:     uint32(viper.GetInt("ebpf.cpu_profiler.target_pid")),
				MaxStackDepth: viper.GetInt("ebpf.cpu_profiler.max_stack_depth"),
				DurationSec:   viper.GetInt("ebpf.cpu_profiler.duration_sec"),
				OutputDir:     viper.GetString("ebpf.cpu_profiler.output_dir"),
				AutoDetect:    viper.GetBool("ebpf.cpu_profiler.auto_detect"),
			},
			SQLAggregator: SQLAggregatorConfig{
				Enabled:              viper.GetBool("ebpf.sql_aggregator.enabled"),
				SlowQueryThresholdMs: viper.GetUint64("ebpf.sql_aggregator.slow_query_threshold_ms"),
				EnableCorrelation:    viper.GetBool("ebpf.sql_aggregator.enable_correlation"),
				MaxSnapshots:         viper.GetInt("ebpf.sql_aggregator.max_snapshots"),
				CPUMaxThreshold:      viper.GetFloat64("ebpf.sql_aggregator.cpu_max_threshold"),
				MemoryMaxMB:          viper.GetFloat64("ebpf.sql_aggregator.memory_max_mb"),
				LatencyMaxMs:         viper.GetFloat64("ebpf.sql_aggregator.latency_max_ms"),
				SlowQueryMax:         viper.GetUint64("ebpf.sql_aggregator.slow_query_max"),
				ConnMax:              viper.GetUint64("ebpf.sql_aggregator.conn_max"),
			},
		},
		Storage: StorageConfig{
			Enabled:              viper.GetBool("storage.enabled"),
			BaseDir:              viper.GetString("storage.base_dir"),
			RetentionDays:        viper.GetInt("storage.retention_days"),
			ChunkSize:            viper.GetInt("storage.chunk_size"),
			WriteBufferSize:      viper.GetInt("storage.write_buffer_size"),
			CompressionType:       viper.GetString("storage.compression_type"),
			EnableIndex:           viper.GetBool("storage.enable_index"),
			RetentionIntervalMin:  viper.GetInt("storage.retention_interval_min"),
			MetricRetentionDays:   viper.GetInt("storage.metric_retention_days"),
			LogRetentionDays:      viper.GetInt("storage.log_retention_days"),
			TraceRetentionDays:    viper.GetInt("storage.trace_retention_days"),
			EventRetentionDays:    viper.GetInt("storage.event_retention_days"),
			WriteRateMin:          viper.GetInt("storage.write_rate_min"),
			QueryLatencyMaxMs:     viper.GetInt("storage.query_latency_max_ms"),
		},
		Alert: AlertConfig{
			Enabled:              viper.GetBool("alert.enabled"),
			EvaluationInterval:   viper.GetDuration("alert.evaluation_interval"),
			ResolveTimeout:       viper.GetDuration("alert.resolve_timeout"),
			EnableAutoResolve:    viper.GetBool("alert.enable_auto_resolve"),
			MaxActiveAlerts:      viper.GetInt("alert.max_active_alerts"),
			EnableLatencyAlert:    viper.GetBool("alert.enable_latency_alert"),
			EnablePacketLossAlert: viper.GetBool("alert.enable_packet_loss_alert"),
			EnableRetransmitAlert: viper.GetBool("alert.enable_retransmit_alert"),
			EnableCPUAlert:        viper.GetBool("alert.enable_cpu_alert"),
			EnableMemoryAlert:     viper.GetBool("alert.enable_memory_alert"),
			EnableErrorRateAlert:  viper.GetBool("alert.enable_error_rate_alert"),
			NotifyKafka: KafkaNotifyConfig{
				Enabled: viper.GetBool("alert.notify_kafka.enabled"),
				Brokers: viper.GetStringSlice("alert.notify_kafka.brokers"),
				Topic:   viper.GetString("alert.notify_kafka.topic"),
			},
			NotifyAPI: APINotifyConfig{
				Enabled:   viper.GetBool("alert.notify_api.enabled"),
				URL:       viper.GetString("alert.notify_api.url"),
				Headers:   viper.GetStringMapString("alert.notify_api.headers"),
				AuthType:  viper.GetString("alert.notify_api.auth_type"),
				AuthToken: viper.GetString("alert.notify_api.auth_token"),
			},
			NotifyWebhook: WebhookNotifyConfig{
			Enabled: viper.GetBool("alert.notify_webhook.enabled"),
			URL:     viper.GetString("alert.notify_webhook.url"),
			Secret:  viper.GetString("alert.notify_webhook.secret"),
			Headers: viper.GetStringMapString("alert.notify_webhook.headers"),
		},
	},
	Topology: TopologyConfig{
		Enabled:                viper.GetBool("topology.enabled"),
		RefreshInterval:        viper.GetDuration("topology.refresh_interval"),
		AutoDiscovery:          viper.GetBool("topology.auto_discovery"),
		DefaultLayout:          viper.GetString("topology.default_layout"),
		MaxNodes:               viper.GetInt("topology.max_nodes"),
		IncludePods:            viper.GetBool("topology.include_pods"),
		IncludeVMs:             viper.GetBool("topology.include_vms"),
		IncludePhysical:        viper.GetBool("topology.include_physical"),
		EnableAlertIntegration: viper.GetBool("topology.enable_alert_integration"),
	},
	Tenant: TenantConfig{
		Enabled:           viper.GetBool("tenant.enabled"),
		MultiTenant:       viper.GetBool("tenant.multi_tenant"),
		MaxTenants:        viper.GetInt("tenant.max_tenants"),
		MaxUsersPerTenant: viper.GetInt("tenant.max_users_per_tenant"),
	},
	Dashboard: DashboardConfig{
		Enabled:          viper.GetBool("dashboard.enabled"),
		RefreshInterval:  viper.GetDuration("dashboard.refresh_interval"),
		EnableDrillDown:  viper.GetBool("dashboard.enable_drill_down"),
		MaxAssetsPerPage: viper.GetInt("dashboard.max_assets_per_page"),
	},
	Metrics: MetricsConfig{
		Enabled:       viper.GetBool("metrics.enabled"),
		NetworkEnabled: viper.GetBool("metrics.network_enabled"),
		AppEnabled:    viper.GetBool("metrics.app_enabled"),
		SQLEnabled:    viper.GetBool("metrics.sql_enabled"),
		MaxDataPoints: viper.GetInt("metrics.max_data_points"),
		RetentionTime: viper.GetDuration("metrics.retention_time"),
	},
	CMDB: CMDBConfig{
		Enabled:        viper.GetBool("cmdb.enabled"),
		Type:           viper.GetString("cmdb.type"),
		Endpoint:       viper.GetString("cmdb.endpoint"),
		AuthType:       viper.GetString("cmdb.auth_type"),
		AuthToken:      viper.GetString("cmdb.auth_token"),
		APIKey:         viper.GetString("cmdb.api_key"),
		Username:       viper.GetString("cmdb.username"),
		Password:       viper.GetString("cmdb.password"),
		SyncInterval:   viper.GetDuration("cmdb.sync_interval"),
		FullSyncStart:  viper.GetBool("cmdb.full_sync_on_start"),
		Incremental:    viper.GetBool("cmdb.incremental_sync"),
		LabelSync:      viper.GetBool("cmdb.label_sync"),
		ConfigSync:     viper.GetBool("cmdb.config_sync"),
		RelationSync:   viper.GetBool("cmdb.relation_sync"),
		ConflictPolicy: viper.GetString("cmdb.conflict_policy"),
		Timeout:        viper.GetDuration("cmdb.timeout"),
		BatchSize:      viper.GetInt("cmdb.batch_size"),
	},
	Trace: TraceConfig{
		Enabled:           viper.GetBool("trace.enabled"),
		SampleRate:        viper.GetFloat64("trace.sample_rate"),
		MaxTraces:         viper.GetInt("trace.max_traces"),
		MaxSpansPerTrace:  viper.GetInt("trace.max_spans_per_trace"),
		RetentionTime:     viper.GetDuration("trace.retention_time"),
		EnableCorrelation: viper.GetBool("trace.enable_correlation"),
		FlameGraphEnabled: viper.GetBool("trace.flamegraph_enabled"),
		FlameSampleFreq:   viper.GetInt("trace.flame_sample_freq"),
		FlameMaxDepth:     viper.GetInt("trace.flame_max_depth"),
		FlameDurationSec:  viper.GetInt("trace.flame_duration_sec"),
		FlameOutputDir:    viper.GetString("trace.flame_output_dir"),
		FlameMaxStored:    viper.GetInt("trace.flame_max_stored"),
	},
	// L1 修复: 可配置的定时任务间隔
	FlushInterval:        viper.GetDuration("flush_interval"),
	ReconnectBaseDelay:   viper.GetDuration("reconnect_base_delay"),
	ReconnectMaxDelay:    viper.GetDuration("reconnect_max_delay"),
	MaxReconnectAttempts: viper.GetInt("max_reconnect_attempts"),
	MaxBufferLimit:       viper.GetInt("max_buffer_limit"),
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
