// Package models 定义 Agent 核心数据模型
package models

import (
	"time"
)

// ArchType 系统架构类型
type ArchType string

const (
	ArchX86_64 ArchType = "x86_64"  // x86 64位 (海光)
	ArchARM64  ArchType = "aarch64" // ARM 64位 (鲲鹏)
	ArchUnknown ArchType = "unknown"
)

// KernelCapability 内核能力
type KernelCapability struct {
	Version         string    // 内核版本字符串
	Major           int       // 主版本号
	Minor           int       // 次版本号
	Patch           int       // 补丁版本号
	Arch            ArchType  // 系统架构
	SupportsEBPF    bool      // 是否支持 eBPF
	SupportsBTF     bool      // 是否支持 BTF (BPF Type Format)
	SupportsRingBuf bool      // 是否支持 Ring Buffer
	MinRequired     bool      // 是否满足最低要求 (>=3.10)
	DetectedAt      time.Time // 检测时间
}

// CollectorType 采集器类型
type CollectorType string

const (
	CollectorEBPF      CollectorType = "ebpf"      // eBPF 采集器
	CollectorTraditional CollectorType = "traditional" // 传统采集器
	CollectorMetrics   CollectorType = "metrics"   // 系统指标采集器
	CollectorProcess   CollectorType = "process"   // 进程事件采集器
)

// MetricType 指标类型
type MetricType string

const (
	MetricGauge   MetricType = "gauge"
	MetricCounter MetricType = "counter"
	MetricHistogram MetricType = "histogram"
)

// NetworkFlow 网络流量数据
type NetworkFlow struct {
	Timestamp     time.Time `json:"timestamp"`
	Protocol      string    `json:"protocol"`       // TCP/UDP/ICMP
	SourceIP      string    `json:"source_ip"`
	SourcePort    uint16    `json:"source_port"`
	DestIP        string    `json:"dest_ip"`
	DestPort      uint16    `json:"dest_port"`
	ProcessName   string    `json:"process_name"`
	ProcessPID    uint32    `json:"process_pid"`
	BytesSent     uint64    `json:"bytes_sent"`
	BytesRecv     uint64    `json:"bytes_recv"`
	PacketsSent   uint64    `json:"packets_sent"`
	PacketsRecv   uint64    `json:"packets_recv"`
	Duration      uint64    `json:"duration_ns"`    // 连接持续时间 (纳秒)
	TCPState      string    `json:"tcp_state"`      // TCP 连接状态
	CollectorType CollectorType `json:"collector_type"`
}

// SystemMetric 系统指标
type SystemMetric struct {
	Timestamp time.Time `json:"timestamp"`
	HostIP    string    `json:"host_ip"`
	Hostname  string    `json:"hostname"`

	// CPU 指标
	CPUUsage     float64 `json:"cpu_usage"`      // 总 CPU 使用率 (%)
	CPUUser      float64 `json:"cpu_user"`       // 用户态 CPU (%)
	CPUSystem    float64 `json:"cpu_system"`     // 内核态 CPU (%)
	CPUIdle      float64 `json:"cpu_idle"`       // 空闲 CPU (%)
	CPUSteal     float64 `json:"cpu_steal"`      // 窃取 CPU (%)
	Load1        float64 `json:"load_1"`         // 1分钟负载
	Load5        float64 `json:"load_5"`         // 5分钟负载
	Load15       float64 `json:"load_15"`        // 15分钟负载

	// 内存指标
	MemTotal     uint64  `json:"mem_total"`      // 总内存 (bytes)
	MemUsed      uint64  `json:"mem_used"`       // 已用内存 (bytes)
	MemFree      uint64  `json:"mem_free"`       // 空闲内存 (bytes)
	MemBuffers   uint64  `json:"mem_buffers"`    // 缓冲区 (bytes)
	MemCached    uint64  `json:"mem_cached"`     // 缓存 (bytes)
	MemUsage     float64 `json:"mem_usage"`      // 内存使用率 (%)
	SwapTotal    uint64  `json:"swap_total"`     // 总交换分区 (bytes)
	SwapUsed     uint64  `json:"swap_used"`      // 已用交换分区 (bytes)
	SwapUsage    float64 `json:"swap_usage"`     // 交换分区使用率 (%)

	// 磁盘指标
	DiskTotal    uint64  `json:"disk_total"`     // 总磁盘空间 (bytes)
	DiskUsed     uint64  `json:"disk_used"`      // 已用磁盘空间 (bytes)
	DiskFree     uint64  `json:"disk_free"`      // 空闲磁盘空间 (bytes)
	DiskUsage    float64 `json:"disk_usage"`     // 磁盘使用率 (%)
	DiskReadBytes  uint64 `json:"disk_read_bytes"`  // 磁盘读取字节数
	DiskWriteBytes uint64 `json:"disk_write_bytes"` // 磁盘写入字节数
	DiskReadOps    uint64 `json:"disk_read_ops"`    // 磁盘读取 IOPS
	DiskWriteOps   uint64 `json:"disk_write_ops"`   // 磁盘写入 IOPS

	// 网络指标
	NetBytesSent   uint64 `json:"net_bytes_sent"`   // 网络发送字节数
	NetBytesRecv   uint64 `json:"net_bytes_recv"`   // 网络接收字节数
	NetPacketsSent uint64 `json:"net_packets_sent"` // 网络发送包数
	NetPacketsRecv uint64 `json:"net_packets_recv"` // 网络接收包数
	NetTCPConns    uint64 `json:"net_tcp_conns"`    // TCP 连接数
	NetUDPConns    uint64 `json:"net_udp_conns"`    // UDP 连接数
}

// ProcessEvent 进程事件
type ProcessEvent struct {
	Timestamp   time.Time `json:"timestamp"`
	EventType   string    `json:"event_type"`   // exec, fork, exit, clone
	PID         uint32    `json:"pid"`
	PPID        uint32    `json:"ppid"`         // 父进程 ID
	TID         uint32    `json:"tid"`          // 线程 ID
	Comm        string    `json:"comm"`         // 进程名
	Exe         string    `json:"exe"`          // 可执行文件路径
	Cmdline     string    `json:"cmdline"`      // 命令行参数
	CWD         string    `json:"cwd"`          // 工作目录
	UID         uint32    `json:"uid"`          // 用户 ID
	GID         uint32    `json:"gid"`          // 组 ID
	ExitCode    int32     `json:"exit_code"`    // 退出码 (仅 exit 事件)
	Signal      uint32    `json:"signal"`       // 信号 (仅 exit 事件)
}

// SyscallEvent 系统调用事件
type SyscallEvent struct {
	Timestamp  time.Time `json:"timestamp"`
	PID        uint32    `json:"pid"`
	TID        uint32    `json:"tid"`
	Comm       string    `json:"comm"`
	SyscallID  int32     `json:"syscall_id"`   // 系统调用号
	SyscallName string   `json:"syscall_name"` // 系统调用名称
	Args       []uint64  `json:"args"`         // 系统调用参数
	Retval     int64     `json:"retval"`       // 返回值
	Duration   uint64    `json:"duration_ns"`  // 执行时间 (纳秒)
	Success    bool      `json:"success"`      // 是否成功
}

// CollectorStatus 采集器状态
type CollectorStatus struct {
	Name         string        `json:"name"`
	Type         CollectorType `json:"type"`
	Enabled      bool          `json:"enabled"`
	Running      bool          `json:"running"`
	StartTime    time.Time     `json:"start_time"`
	LastError    string        `json:"last_error"`
	EventsCount  uint64        `json:"events_count"`
	DropCount    uint64        `json:"drop_count"`
}

// AgentStatus Agent 状态
type AgentStatus struct {
	Hostname      string            `json:"hostname"`
	HostIP        string            `json:"host_ip"`
	Arch          ArchType          `json:"arch"`
	Kernel        KernelCapability  `json:"kernel"`
	Version       string            `json:"version"`
	StartTime     time.Time         `json:"start_time"`
	Uptime        time.Duration     `json:"uptime"`
	ConfigHash    string            `json:"config_hash"`
	Collectors    []CollectorStatus `json:"collectors"`
	EdgeConnected bool              `json:"edge_connected"`
	EdgeAddress   string            `json:"edge_address"`
}

// ============================================================
// 双中心配置模型
// ============================================================

// DualCenterPeerConfig 对端中心配置
type DualCenterPeerConfig struct {
	ID         string `yaml:"id" json:"id"`
	Name       string `yaml:"name" json:"name"`
	Role       string `yaml:"role" json:"role"`           // primary/secondary/active/standby
	Address    string `yaml:"address" json:"address"`
	Port       int    `yaml:"port" json:"port"`
	Region     string `yaml:"region" json:"region"`
	TLSEnabled bool   `yaml:"tls_enabled" json:"tls_enabled"`
}

// DualCenterConfig 双中心同步配置
type DualCenterConfig struct {
	Enabled         bool                   `yaml:"enabled" json:"enabled"`
	LocalCenterID   string                 `yaml:"local_center_id" json:"local_center_id"`
	LocalCenterName string                 `yaml:"local_center_name" json:"local_center_name"`
	LocalRole       string                 `yaml:"local_role" json:"local_role"`             // primary/secondary
	LocalAddress    string                 `yaml:"local_address" json:"local_address"`
	LocalPort       int                    `yaml:"local_port" json:"local_port"`
	Region          string                 `yaml:"region" json:"region"`
	PeerCenters     []DualCenterPeerConfig `yaml:"peer_centers" json:"peer_centers"`
	SyncMode        string                 `yaml:"sync_mode" json:"sync_mode"`             // sync/async/semi_sync
	BatchSize       int                    `yaml:"batch_size" json:"batch_size"`
	FlushInterval   int                    `yaml:"flush_interval" json:"flush_interval"`   // 秒
	CompressEnabled bool                   `yaml:"compress_enabled" json:"compress_enabled"`
	MaxRetries      int                    `yaml:"max_retries" json:"max_retries"`
	RetryDelay      int                    `yaml:"retry_delay" json:"retry_delay"`         // 秒
	QueueSize       int                    `yaml:"queue_size" json:"queue_size"`
	QueueOverflow   string                 `yaml:"queue_overflow" json:"queue_overflow"`   // drop_latest/drop_oldest/block
}

// FailoverConfig 故障切换配置
type FailoverConfig struct {
	Enabled             bool   `yaml:"enabled" json:"enabled"`
	Mode                string `yaml:"mode" json:"mode"`                               // auto/manual/semi_auto
	HeartbeatInterval   int    `yaml:"heartbeat_interval" json:"heartbeat_interval"`     // 秒
	HeartbeatTimeout    int    `yaml:"heartbeat_timeout" json:"heartbeat_timeout"`       // 秒
	FailureThreshold    int    `yaml:"failure_threshold" json:"failure_threshold"`
	SuccessThreshold    int    `yaml:"success_threshold" json:"success_threshold"`
	SwitchTimeout       int    `yaml:"switch_timeout" json:"switch_timeout"`             // 秒
	PreSwitchDelay      int    `yaml:"pre_switch_delay" json:"pre_switch_delay"`         // 秒
	GracefulShutdown    bool   `yaml:"graceful_shutdown" json:"graceful_shutdown"`
	DrainTimeout        int    `yaml:"drain_timeout" json:"drain_timeout"`               // 秒
	AutoRecover         bool   `yaml:"auto_recover" json:"auto_recover"`
	RecoverDelay        int    `yaml:"recover_delay" json:"recover_delay"`               // 秒
	DataRepairOnRecover bool   `yaml:"data_repair_on_recover" json:"data_repair_on_recover"`
	FenceEnabled        bool   `yaml:"fence_enabled" json:"fence_enabled"`
	FenceTimeout        int    `yaml:"fence_timeout" json:"fence_timeout"`               // 秒
	QuorumRequired      bool   `yaml:"quorum_required" json:"quorum_required"`
}

// VIPConfig 虚拟IP配置
type VIPConfig struct {
	Enabled   bool   `yaml:"enabled" json:"enabled"`
	VirtualIP string `yaml:"virtual_ip" json:"virtual_ip"`
	Interface string `yaml:"interface" json:"interface"`
	Netmask   string `yaml:"netmask" json:"netmask"`
}

// ============================================================
// 熔断与降级配置模型
// ============================================================

// CircuitBreakerConfig 熔断器配置
type CircuitBreakerConfig struct {
	Enabled          bool `yaml:"enabled" json:"enabled"`
	MaxFailures      int  `yaml:"max_failures" json:"max_failures"`           // 最大连续失败次数
	ResetTimeoutSec  int  `yaml:"reset_timeout_sec" json:"reset_timeout_sec"` // 恢复超时（秒）
	HalfOpenMax      int  `yaml:"half_open_max" json:"half_open_max"`         // 半开探测数
	WindowTimeSec    int  `yaml:"window_time_sec" json:"window_time_sec"`     // 滑动窗口时间（秒）
	WindowBuckets    int  `yaml:"window_buckets" json:"window_buckets"`       // 滑动窗口桶数
	AdaptiveEnabled  bool `yaml:"adaptive_enabled" json:"adaptive_enabled"`   // 自适应恢复超时
	TimeoutSec       int  `yaml:"timeout_sec" json:"timeout_sec"`             // 单次执行超时（秒）
}

// DegradationPolicyConfig 降级策略配置
type DegradationPolicyConfig struct {
	ComponentID      string `yaml:"component_id" json:"component_id"`
	MaxErrorRate     float64 `yaml:"max_error_rate" json:"max_error_rate"`
	MaxErrors        int    `yaml:"max_errors" json:"max_errors"`
	ErrorWindowSec   int    `yaml:"error_window_sec" json:"error_window_sec"`
	FallbackTo       string `yaml:"fallback_to" json:"fallback_to"`
	AutoRecover      bool   `yaml:"auto_recover" json:"auto_recover"`
	RecoverAfterSec  int    `yaml:"recover_after_sec" json:"recover_after_sec"`
	RecoverThreshold int    `yaml:"recover_threshold" json:"recover_threshold"`
}

// ResilienceConfig 弹性管理配置
type ResilienceConfig struct {
	Enabled             bool                      `yaml:"enabled" json:"enabled"`
	HealthCheckInterval int                       `yaml:"health_check_interval" json:"health_check_interval"` // 秒
	SwitchTimeoutSec    int                       `yaml:"switch_timeout_sec" json:"switch_timeout_sec"`
	GracefulDegradation bool                      `yaml:"graceful_degradation" json:"graceful_degradation"`
	CircuitBreaker      CircuitBreakerConfig      `yaml:"circuit_breaker" json:"circuit_breaker"`
	Policies            []DegradationPolicyConfig `yaml:"policies" json:"policies"`
}

// ============================================================
// VXLAN 隧道解封装配置模型
// ============================================================

// VXLANConfig VXLAN 解封装配置
type VXLANConfig struct {
	Enabled       bool     `yaml:"enabled" json:"enabled"`
	ListenPort    uint16   `yaml:"listen_port" json:"listen_port"`       // VXLAN 监听端口 (默认 4789)
	FilterVNI     []uint32 `yaml:"filter_vni" json:"filter_vni"`         // 过滤的 VNI 列表
	FilterSrcIP   []string `yaml:"filter_src_ip" json:"filter_src_ip"`   // 过滤的内层源 IP
	FilterDstIP   []string `yaml:"filter_dst_ip" json:"filter_dst_ip"`   // 过滤的内层目的 IP
	BufferSize    int      `yaml:"buffer_size" json:"buffer_size"`       // 缓冲区大小
	MaxPacketSize int      `yaml:"max_packet_size" json:"max_packet_size"` // 最大包大小
}

// MirrorTargetConfig 镜像目标配置
type MirrorTargetConfig struct {
	Name        string `yaml:"name" json:"name"`
	Mode        string `yaml:"mode" json:"mode"`             // raw/gre/vxlan/udp/erspan
	Address     string `yaml:"address" json:"address"`       // 目标地址
	Port        uint16 `yaml:"port" json:"port"`             // 目标端口
	VNI         uint32 `yaml:"vni" json:"vni"`               // VXLAN VNI
	GREKey      uint32 `yaml:"gre_key" json:"gre_key"`       // GRE Key
	ERSPANID    uint32 `yaml:"erspan_id" json:"erspan_id"`   // ERSPAN Session ID
	Enabled     bool   `yaml:"enabled" json:"enabled"`
	BufferSize  int    `yaml:"buffer_size" json:"buffer_size"`
}

// MirrorFilterConfig 镜像过滤配置
type MirrorFilterConfig struct {
	SrcIPs     []string `yaml:"src_ips" json:"src_ips"`
	DstIPs     []string `yaml:"dst_ips" json:"dst_ips"`
	SrcPorts   []uint16 `yaml:"src_ports" json:"src_ports"`
	DstPorts   []uint16 `yaml:"dst_ports" json:"dst_ports"`
	Protocols  []uint8  `yaml:"protocols" json:"protocols"`   // 6=TCP, 17=UDP
	VNIs       []uint32 `yaml:"vnis" json:"vnis"`
	SampleRate int      `yaml:"sample_rate" json:"sample_rate"` // 1-100
}

// MirrorConfig 流量镜像配置
type MirrorConfig struct {
	Enabled      bool                `yaml:"enabled" json:"enabled"`
	SourceIP     string              `yaml:"source_ip" json:"source_ip"`       // 源 IP（用于封装）
	SourcePort   uint16              `yaml:"source_port" json:"source_port"`   // 源端口
	QueueSize    int                 `yaml:"queue_size" json:"queue_size"`     // 队列大小
	BatchSize    int                 `yaml:"batch_size" json:"batch_size"`     // 批量发送大小
	BatchTimeout int                 `yaml:"batch_timeout" json:"batch_timeout"` // 批量发送超时（毫秒）
	Filter       MirrorFilterConfig  `yaml:"filter" json:"filter"`             // 流量过滤
	Targets      []MirrorTargetConfig `yaml:"targets" json:"targets"`          // 镜像目标列表
}

// Config 配置结构
type Config struct {
	Agent       AgentConfig       `yaml:"agent" json:"agent"`
	Edge        EdgeConfig        `yaml:"edge" json:"edge"`
	Collectors  CollectorsConfig  `yaml:"collectors" json:"collectors"`
	Resources   ResourceConfig    `yaml:"resources" json:"resources"`
	Logging     LoggingConfig     `yaml:"logging" json:"logging"`
	DualCenter  DualCenterConfig  `yaml:"dual_center" json:"dual_center"`
	Failover    FailoverConfig    `yaml:"failover" json:"failover"`
	VIP         VIPConfig         `yaml:"vip" json:"vip"`
	Resilience  ResilienceConfig  `yaml:"resilience" json:"resilience"`
	VXLAN       VXLANConfig       `yaml:"vxlan" json:"vxlan"`
	Mirror      MirrorConfig      `yaml:"mirror" json:"mirror"`
	Protocol    ProtocolConfig    `yaml:"protocol" json:"protocol"`
	PacketLoss  PacketLossConfig  `yaml:"packet_loss" json:"packet_loss"`
	TimeSync    TimeSyncConfig    `yaml:"time_sync" json:"time_sync"`
	Profiler    ProfilerConfig    `yaml:"profiler" json:"profiler"`
	JVMMem      JVMMemConfig      `yaml:"jvm_memory" json:"jvm_memory"`
	AIModel     AIModelConfig     `yaml:"ai_model" json:"ai_model"`
	Alert       AlertConfig       `yaml:"alert" json:"alert"`
	RCA         RCAConfig         `yaml:"rca" json:"rca"`
	Tracing     TracingConfig     `yaml:"tracing" json:"tracing"`
	HealthScore HealthScoreConfig `yaml:"health_score" json:"health_score"`
	PCAPStorage PCAPStorageConfig `yaml:"pcap_storage" json:"pcap_storage"`
	Kernel      KernelConfig      `yaml:"kernel" json:"kernel"`
	Isolation   IsolationConfig   `yaml:"isolation" json:"isolation"`
	EBPFResource EBPFResourceConfig `yaml:"ebpf_resource" json:"ebpf_resource"`
	TCPMetrics      TCPMetricsConfig      `yaml:"tcp_metrics" json:"tcp_metrics"`
	Transport        TransportConfig       `yaml:"transport" json:"transport"`
	CircuitBreaker   CircuitBreakerConfig  `yaml:"circuit_breaker" json:"circuit_breaker"`
	SelfMonitor      SelfMonitorConfig     `yaml:"self_monitor" json:"self_monitor"`
	ReliableTransport ReliableTransportConfig `yaml:"reliable_transport" json:"reliable_transport"`
	DynConfig        DynConfigConfig       `yaml:"dyn_config" json:"dyn_config"`
	WebAPI           WebAPIConfig          `yaml:"web_api" json:"web_api"`
	LogMgr           LogConfig             `yaml:"log_mgr" json:"log_mgr"`
	LibBPF           LibBPFConfig          `yaml:"libbpf" json:"libbpf"`
	HotReload        HotReloadConfig       `yaml:"hot_reload" json:"hot_reload"`
	ConnPool         ConnPoolConfig        `yaml:"conn_pool" json:"conn_pool"`
	Aggregation      AggregationConfig     `yaml:"aggregation" json:"aggregation"`
	OfflineCache     OfflineCacheConfig   `yaml:"offline_cache" json:"offline_cache"`
	Auth            AuthConfig           `yaml:"auth" json:"auth"`
	LoadBalancer    LoadBalancerConfig   `yaml:"load_balancer" json:"load_balancer"`
	GRPC            GRPCConfig           `yaml:"grpc" json:"grpc"`
	Metrics         MetricsConfig        `yaml:"metrics" json:"metrics"`
}

// ============================================================
// 协议解析插件配置模型
// ============================================================

// ProtocolPluginConfig 单个协议解析插件配置
type ProtocolPluginConfig struct {
	Name     string `yaml:"name" json:"name"`           // 协议名称: oracle/postgresql/redis/kafka/dubbo
	Enabled  bool   `yaml:"enabled" json:"enabled"`       // 是否启用
	Ports    []uint16 `yaml:"ports" json:"ports"`         // 自定义端口（覆盖默认）
}

// ProtocolConfig 协议解析配置
type ProtocolConfig struct {
	Enabled  bool                 `yaml:"enabled" json:"enabled"`   // 启用协议解析
	Plugins  []ProtocolPluginConfig `yaml:"plugins" json:"plugins"` // 协议插件列表
	BufferSize int                `yaml:"buffer_size" json:"buffer_size"` // 解析缓冲区大小
}

// AgentConfig Agent 配置
type AgentConfig struct {
	Hostname string `yaml:"hostname" json:"hostname"`
	HostIP   string `yaml:"host_ip" json:"host_ip"`
	Interval int    `yaml:"interval" json:"interval"` // 采集间隔 (秒)
}

// EdgeConfig Edge 连接配置
type EdgeConfig struct {
	Address    string `yaml:"address" json:"address"`       // Edge 地址
	Port       int    `yaml:"port" json:"port"`             // Edge 端口
	TLSEnabled bool   `yaml:"tls_enabled" json:"tls_enabled"`
	CAFile     string `yaml:"ca_file" json:"ca_file"`
	CertFile   string `yaml:"cert_file" json:"cert_file"`
	KeyFile    string `yaml:"key_file" json:"key_file"`
	Timeout    int    `yaml:"timeout" json:"timeout"`       // 连接超时 (秒)
	RetryMax   int    `yaml:"retry_max" json:"retry_max"`   // 最大重试次数
	RetryDelay int    `yaml:"retry_delay" json:"retry_delay"` // 重试间隔 (秒)
}

// CollectorsConfig 采集器配置
type CollectorsConfig struct {
	EBPF      EBPFCollectorConfig      `yaml:"ebpf" json:"ebpf"`
	Traditional TraditionalCollectorConfig `yaml:"traditional" json:"traditional"`
	Metrics    MetricsCollectorConfig  `yaml:"metrics" json:"metrics"`
	Process    ProcessCollectorConfig  `yaml:"process" json:"process"`
}

// EBPFCollectorConfig eBPF 采集器配置
type EBPFCollectorConfig struct {
	Enabled       bool     `yaml:"enabled" json:"enabled"`
	Events        []string `yaml:"events" json:"events"`         // tcp_connect, tcp_accept, tcp_close, etc.
	FilterPorts   []uint16 `yaml:"filter_ports" json:"filter_ports"`
	FilterIPs     []string `yaml:"filter_ips" json:"filter_ips"`
	SampleRate    int      `yaml:"sample_rate" json:"sample_rate"` // 采样率 (1-100)
	BufferSize    int      `yaml:"buffer_size" json:"buffer_size"` // Perf buffer 大小
}

// TraditionalCollectorConfig 传统采集器配置
type TraditionalCollectorConfig struct {
	Enabled       bool     `yaml:"enabled" json:"enabled"`
	ProcPath      string   `yaml:"proc_path" json:"proc_path"`
	NetlinkGroups []uint32 `yaml:"netlink_groups" json:"netlink_groups"`
}

// MetricsCollectorConfig 系统指标采集器配置
type MetricsCollectorConfig struct {
	Enabled       bool     `yaml:"enabled" json:"enabled"`
	Interval      int      `yaml:"interval" json:"interval"` // 采集间隔 (秒)
	CPU           bool     `yaml:"cpu" json:"cpu"`
	Memory        bool     `yaml:"memory" json:"memory"`
	Disk          bool     `yaml:"disk" json:"disk"`
	DiskPaths     []string `yaml:"disk_paths" json:"disk_paths"`
	Network       bool     `yaml:"network" json:"network"`
	NetworkIfaces []string `yaml:"network_ifaces" json:"network_ifaces"`
}

// ProcessCollectorConfig 进程事件采集器配置
type ProcessCollectorConfig struct {
	Enabled      bool     `yaml:"enabled" json:"enabled"`
	Events       []string `yaml:"events" json:"events"` // exec, fork, exit, clone
	FilterUsers  []uint32 `yaml:"filter_users" json:"filter_users"`
	FilterComms  []string `yaml:"filter_comms" json:"filter_comms"`
}

// ResourceConfig 资源限制配置
type ResourceConfig struct {
	CPUQuota      float64 `yaml:"cpu_quota" json:"cpu_quota"`       // CPU 配额 (核心数)
	MemoryLimit   uint64  `yaml:"memory_limit" json:"memory_limit"` // 内存限制 (MB)
	BufferMaxSize uint64  `yaml:"buffer_max_size" json:"buffer_max_size"` // 缓冲区最大大小 (MB)
	MaxGoroutines int     `yaml:"max_goroutines" json:"max_goroutines"`   // 最大协程数
}

// LoggingConfig 日志配置
type LoggingConfig struct {
	Level  string `yaml:"level" json:"level"`   // debug, info, warn, error
	Format string `yaml:"format" json:"format"` // json, text
	Output string `yaml:"output" json:"output"` // stdout, file
	Path   string `yaml:"path" json:"path"`     // 日志文件路径
	MaxSize int   `yaml:"max_size" json:"max_size"` // 单个日志文件最大大小 (MB)
	MaxBackups int `yaml:"max_backups" json:"max_backups"` // 最大备份文件数
	MaxAge    int `yaml:"max_age" json:"max_age"`       // 最大保留天数
	Compress  bool `yaml:"compress" json:"compress"`     // 是否压缩
}

// ============================================================
// 丢包监控配置模型
// ============================================================

// PacketLossConfig 丢包监控配置
type PacketLossConfig struct {
	Enabled          bool          `yaml:"enabled" json:"enabled"`
	CheckInterval    int           `yaml:"check_interval" json:"check_interval"`    // 检查间隔（秒）
	Interface        string        `yaml:"interface" json:"interface"`              // 监控的接口，空表示所有
	ThresholdPercent float64       `yaml:"threshold_percent" json:"threshold_percent"` // 丢包率阈值（%）
	ThresholdPackets uint64        `yaml:"threshold_packets" json:"threshold_packets"` // 丢包数阈值
	AlertCooldown    int           `yaml:"alert_cooldown" json:"alert_cooldown"`    // 告警冷却（秒）
	EnableTCPCheck   bool          `yaml:"enable_tcp_check" json:"enable_tcp_check"` // 启用TCP检测
	TCPCheckTarget   string        `yaml:"tcp_check_target" json:"tcp_check_target"` // TCP检测目标
	TCPCheckPort     uint16        `yaml:"tcp_check_port" json:"tcp_check_port"`    // TCP检测端口
}

// ============================================================
// 时钟同步配置模型
// ============================================================

// TimeSyncConfig 时钟同步配置
type TimeSyncConfig struct {
	Enabled        bool     `yaml:"enabled" json:"enabled"`
	SyncInterval   int      `yaml:"sync_interval" json:"sync_interval"`     // 同步间隔（秒）
	Servers        []string `yaml:"servers" json:"servers"`                 // NTP服务器列表
	Timeout        int      `yaml:"timeout" json:"timeout"`                 // 超时（秒）
	MaxDriftSec    int      `yaml:"max_drift_sec" json:"max_drift_sec"`     // 最大允许偏差（秒）
	AutoCorrect    bool     `yaml:"auto_correct" json:"auto_correct"`       // 自动校准
	DriftThreshold int      `yaml:"drift_threshold" json:"drift_threshold"` // 偏差告警阈值（秒）
	RetryAttempts  int      `yaml:"retry_attempts" json:"retry_attempts"`   // 重试次数
}

// ============================================================
// CPU等待分析器配置模型
// ============================================================

// ProfilerConfig CPU等待分析器配置
type ProfilerConfig struct {
	Enabled          bool                `yaml:"enabled" json:"enabled"`
	SampleRate       int                 `yaml:"sample_rate" json:"sample_rate"`           // 采样率 (Hz)
	MaxSamplesPerSec int                 `yaml:"max_samples_per_sec" json:"max_samples_per_sec"` // 每秒最大采样数
	TargetLanguages  []string            `yaml:"target_languages" json:"target_languages"`   // 目标语言: c,cpp,go,java
	TargetPIDs       []uint32            `yaml:"target_pids" json:"target_pids"`           // 目标进程ID，空表示所有
	MinBlockTimeMs   int                 `yaml:"min_block_time_ms" json:"min_block_time_ms"` // 最小阻塞时间（毫秒）
	MaxStackDepth    int                 `yaml:"max_stack_depth" json:"max_stack_depth"`   // 最大栈深度
	SymbolResolution bool                `yaml:"symbol_resolution" json:"symbol_resolution"` // 符号解析
	ReportInterval   int                 `yaml:"report_interval" json:"report_interval"`   // 报告间隔（秒）
	DynamicAdjust    bool                `yaml:"dynamic_adjust" json:"dynamic_adjust"`     // 动态调整采样率
	WaitTypes        ProfilerWaitTypes   `yaml:"wait_types" json:"wait_types"`             // 等待类型开关
}

// ProfilerWaitTypes 等待类型配置
type ProfilerWaitTypes struct {
	Futex     bool `yaml:"futex" json:"futex"`       // futex等待
	IO        bool `yaml:"io" json:"io"`             // IO等待
	Network   bool `yaml:"network" json:"network"`   // 网络等待
	Lock      bool `yaml:"lock" json:"lock"`         // 锁等待
	Sleep     bool `yaml:"sleep" json:"sleep"`       // 睡眠等待
	Park      bool `yaml:"park" json:"park"`         // park等待
	Monitor   bool `yaml:"monitor" json:"monitor"`   // Java monitor
}

// ============================================================
// Java内存分析器配置模型
// ============================================================

// JVMMemConfig Java内存分析配置
type JVMMemConfig struct {
	Enabled           bool                `yaml:"enabled" json:"enabled"`
	SampleRate        int                 `yaml:"sample_rate" json:"sample_rate"`           // 采样率 (0-100%)
	TargetPIDs        []uint32            `yaml:"target_pids" json:"target_pids"`           // 目标JVM进程
	TrackByteBuffer   bool                `yaml:"track_bytebuffer" json:"track_bytebuffer"` // 追踪ByteBuffer
	TrackJNIMemory    bool                `yaml:"track_jni_memory" json:"track_jni_memory"` // 追踪JNI内存
	TrackDirectMemory bool                `yaml:"track_direct_memory" json:"track_direct_memory"` // 追踪堆外内存
	LeakCheckInterval int                 `yaml:"leak_check_interval" json:"leak_check_interval"` // 泄漏检查间隔（秒）
	LeakThreshold     float64             `yaml:"leak_threshold" json:"leak_threshold"`     // 泄漏阈值（增长率/小时）
	MinLeakSize       uint64              `yaml:"min_leak_size" json:"min_leak_size"`       // 最小泄漏大小（字节）
	MaxStackDepth     int                 `yaml:"max_stack_depth" json:"max_stack_depth"`   // 最大栈深度
	SymbolResolution  bool                `yaml:"symbol_resolution" json:"symbol_resolution"` // 符号解析
	ReportInterval    int                 `yaml:"report_interval" json:"report_interval"`   // 报告间隔（秒）
}

// ============================================================
// AI建模配置模型
// ============================================================

// AIModelConfig AI建模配置
type AIModelConfig struct {
	Enabled           bool     `yaml:"enabled" json:"enabled"`
	ModelType         string   `yaml:"model_type" json:"model_type"`             // 模型类型: statistical, ensemble, moving_average, exponential_smoothing
	TrainInterval     int      `yaml:"train_interval" json:"train_interval"`     // 训练间隔（秒）
	PredictionWindow  int      `yaml:"prediction_window" json:"prediction_window"` // 预测窗口大小
	HistoryWindow     int      `yaml:"history_window" json:"history_window"`     // 历史数据窗口大小
	ConfidenceLevel   float64  `yaml:"confidence_level" json:"confidence_level"` // 置信区间 (0-1)
	AnomalyThreshold  float64  `yaml:"anomaly_threshold" json:"anomaly_threshold"` // 异常阈值 (标准差倍数)
	MinDataPoints     int      `yaml:"min_data_points" json:"min_data_points"`   // 最小数据点数
	SeasonalityPeriod int      `yaml:"seasonality_period" json:"seasonality_period"` // 季节性周期
	EnableAutoTuning  bool     `yaml:"enable_auto_tuning" json:"enable_auto_tuning"` // 自动调参
	Metrics           []string `yaml:"metrics" json:"metrics"`                   // 监控的指标列表
}

// AlertConfig 告警配置
type AlertConfig struct {
	Enabled            bool     `yaml:"enabled" json:"enabled"`
	CooldownPeriod     int      `yaml:"cooldown_period" json:"cooldown_period"`         // 告警冷却期（秒）
	AggregationWindow  int      `yaml:"aggregation_window" json:"aggregation_window"`   // 聚合窗口（秒）
	MaxAlertsPerMin    int      `yaml:"max_alerts_per_min" json:"max_alerts_per_min"`   // 每分钟最大告警数
	SuppressDuplicates bool     `yaml:"suppress_duplicates" json:"suppress_duplicates"` // 抑制重复告警
	NotificationChannels []string `yaml:"notification_channels" json:"notification_channels"` // 通知渠道
}

// ============================================================
// 根因分析配置模型
// ============================================================

// RCAConfig 根因分析配置
type RCAConfig struct {
	Enabled              bool   `yaml:"enabled" json:"enabled"`
	CorrelationWindow    int    `yaml:"correlation_window" json:"correlation_window"`       // 告警关联时间窗口（秒）
	MaxCorrelationDepth  int    `yaml:"max_correlation_depth" json:"max_correlation_depth"` // 最大关联深度
	MinCorrelationScore  float64 `yaml:"min_correlation_score" json:"min_correlation_score"` // 最小关联分数
	EnableTopologyAware  bool   `yaml:"enable_topology_aware" json:"enable_topology_aware"` // 启用拓扑感知
	EnableMetricCorrelate bool  `yaml:"enable_metric_correlate" json:"enable_metric_correlate"` // 启用指标关联
	MaxIncidentAge       int    `yaml:"max_incident_age" json:"max_incident_age"`       // 最大事件保留时间（秒）
	AnalysisInterval     int    `yaml:"analysis_interval" json:"analysis_interval"`     // 分析间隔（秒）
	MaxRootCauses        int    `yaml:"max_root_causes" json:"max_root_causes"`         // 最大根因数量
}

// ============================================================
// 分布式追踪配置模型
// ============================================================

// TracingConfig 分布式追踪配置
type TracingConfig struct {
	Enabled              bool                 `yaml:"enabled" json:"enabled"`
	ServiceName          string               `yaml:"service_name" json:"service_name"`           // 服务名称
	SampleRate           float64              `yaml:"sample_rate" json:"sample_rate"`             // 采样率 (0-1)
	PropagationFormat    string               `yaml:"propagation_format" json:"propagation_format"` // 传播格式: w3c, b3, both
	MaxSpansPerTrace     int                  `yaml:"max_spans_per_trace" json:"max_spans_per_trace"` // 每个Trace最大Span数
	MaxTraceDuration     int                  `yaml:"max_trace_duration" json:"max_trace_duration"`   // Trace最大持续时间（秒）
	BufferSize           int                  `yaml:"buffer_size" json:"buffer_size"`               // 缓冲区大小
	ReportInterval       int                  `yaml:"report_interval" json:"report_interval"`       // 报告间隔（秒）
	EnableProcessProfile bool                 `yaml:"enable_process_profile" json:"enable_process_profile"` // 启用进程剖析
	EnableNetworkTrace   bool                 `yaml:"enable_network_trace" json:"enable_network_trace"`     // 启用网络追踪
	Tags                 map[string]string    `yaml:"tags" json:"tags"`                             // 全局标签
	HeaderPropagation    HeaderPropagationConfig `yaml:"header_propagation" json:"header_propagation"` // Header传播配置
}

// HeaderPropagationConfig Header传播配置
type HeaderPropagationConfig struct {
	InheritIncoming bool     `yaml:"inherit_incoming" json:"inherit_incoming"` // 继承传入的TraceID
	InjectOutgoing    bool     `yaml:"inject_outgoing" json:"inject_outgoing"`     // 向 outgoing 请求注入TraceID
	CustomHeaders     []string `yaml:"custom_headers" json:"custom_headers"`       // 自定义传播Header
}

// ============================================================
// 资源池/VPC健康评分配置模型
// ============================================================

// HealthScoreConfig 健康评分配置
type HealthScoreConfig struct {
	Enabled           bool              `yaml:"enabled" json:"enabled"`
	EvalInterval      int               `yaml:"eval_interval" json:"eval_interval"`           // 评估间隔（秒）
	HistoryRetention  int               `yaml:"history_retention" json:"history_retention"`   // 历史保留数量
	AlertThreshold    float64           `yaml:"alert_threshold" json:"alert_threshold"`       // 告警阈值（健康分）
	DegradeThreshold  float64           `yaml:"degrade_threshold" json:"degrade_threshold"`   // 降级阈值（健康分）
	EnableTrend       bool              `yaml:"enable_trend" json:"enable_trend"`             // 启用趋势分析
	TrendWindow       int               `yaml:"trend_window" json:"trend_window"`             // 趋势窗口大小
	Weights           HealthScoreWeights `yaml:"weights" json:"weights"`                       // 加权配置
	VPC               VPCHealthConfig   `yaml:"vpc" json:"vpc"`                               // VPC评估配置
	Pools             []PoolConfig      `yaml:"pools" json:"pools"`                           // 资源池列表
}

// HealthScoreWeights 健康评分权重
type HealthScoreWeights struct {
	Utilization float64 `yaml:"utilization" json:"utilization"` // 利用率权重
	Network     float64 `yaml:"network" json:"network"`         // 网络权重
	SLA         float64 `yaml:"sla" json:"sla"`                 // SLA权重
}

// VPCHealthConfig VPC健康评估配置
type VPCHealthConfig struct {
	ProbeTimeout       int      `yaml:"probe_timeout" json:"probe_timeout"`               // 探测超时（毫秒）
	ProbeCount         int      `yaml:"probe_count" json:"probe_count"`                   // 每次探测次数
	ProbeIntervalMs    int      `yaml:"probe_interval_ms" json:"probe_interval_ms"`       // 探测间隔（毫秒）
	DefaultTargets     []string `yaml:"default_targets" json:"default_targets"`           // 默认探测目标
	CustomTargets      []string `yaml:"custom_targets" json:"custom_targets"`             // 自定义探测目标
	LatencyWarningMs   float64  `yaml:"latency_warning_ms" json:"latency_warning_ms"`     // 延迟告警阈值（ms）
	LatencyCriticalMs  float64  `yaml:"latency_critical_ms" json:"latency_critical_ms"`   // 延迟严重阈值（ms）
	LossWarningPct     float64  `yaml:"loss_warning_pct" json:"loss_warning_pct"`         // 丢包告警阈值（%）
	LossCriticalPct    float64  `yaml:"loss_critical_pct" json:"loss_critical_pct"`       // 丢包严重阈值（%）
	BandwidthMinMbps   float64  `yaml:"bandwidth_min_mbps" json:"bandwidth_min_mbps"`     // 最小带宽要求（Mbps）
	CheckInternalRouting bool   `yaml:"check_internal_routing" json:"check_internal_routing"` // 检查VPC内路由
}

// PoolConfig 资源池配置
type PoolConfig struct {
	ID          string            `yaml:"id" json:"id"`
	Name        string            `yaml:"name" json:"name"`
	Type        string            `yaml:"type" json:"type"`                 // pool / vpc
	VPCID       string            `yaml:"vpc_id" json:"vpc_id,omitempty"`
	SubnetCIDRs []string          `yaml:"subnet_cidrs" json:"subnet_cidrs,omitempty"`
	Hosts       []string          `yaml:"hosts" json:"hosts,omitempty"`
	Labels      map[string]string `yaml:"labels" json:"labels,omitempty"`
}

// ============================================================
// 全包存储与回放配置模型
// ============================================================

// PCAPStorageConfig 全包存储配置
type PCAPStorageConfig struct {
	Enabled         bool   `yaml:"enabled" json:"enabled"`
	BaseDir         string `yaml:"base_dir" json:"base_dir"`                           // 存储根目录
	BlockDuration   int    `yaml:"block_duration" json:"block_duration"`               // 每个块的时间跨度（分钟）
	MaxBlockSize    int    `yaml:"max_block_size" json:"max_block_size"`               // 单个块最大大小 (MB)
	CompressEnabled bool   `yaml:"compress_enabled" json:"compress_enabled"`           // 启用压缩
	CompressLevel   int    `yaml:"compress_level" json:"compress_level"`               // 压缩级别 1-9
	IndexEnabled    bool   `yaml:"index_enabled" json:"index_enabled"`                 // 启用索引
	MaxRetention    int    `yaml:"max_retention" json:"max_retention"`                 // 最大保留时间（小时）
	MaxTotalSize    int    `yaml:"max_total_size" json:"max_total_size"`               // 最大总容量 (GB)
	WriteBufferSize int    `yaml:"write_buffer_size" json:"write_buffer_size"`         // 写缓冲区大小
	FlushInterval   int    `yaml:"flush_interval" json:"flush_interval"`               // 刷新间隔（秒）
	Replay          PCAPReplayConfig `yaml:"replay" json:"replay"`                            // 回放配置
}

// PCAPReplayConfig 全包回放配置
type PCAPReplayConfig struct {
	Enabled         bool     `yaml:"enabled" json:"enabled"`
	DefaultSpeed    float64  `yaml:"default_speed" json:"default_speed"`               // 默认倍速
	MinSpeed        float64  `yaml:"min_speed" json:"min_speed"`                       // 最小倍速
	MaxSpeed        float64  `yaml:"max_speed" json:"max_speed"`                       // 最大倍速
	BufferSize      int      `yaml:"buffer_size" json:"buffer_size"`                   // 回放缓冲区大小
	LoopEnabled     bool     `yaml:"loop_enabled" json:"loop_enabled"`                 // 循环回放
	FilterRewrite   bool     `yaml:"filter_rewrite" json:"filter_rewrite"`             // 重写过滤
	TargetInterface string   `yaml:"target_interface" json:"target_interface"`         // 目标接口
	PauseOnError    bool     `yaml:"pause_on_error" json:"pause_on_error"`             // 错误时暂停
	MaxConcurrent   int      `yaml:"max_concurrent" json:"max_concurrent"`             // 最大并发回放数
}

// ============================================================
// 架构兼容性与安全隔离配置模型
// ============================================================

// KernelConfig 内核兼容性配置
type KernelConfig struct {
	Enabled           bool   `yaml:"enabled" json:"enabled"`
	MinKernelVersion  string `yaml:"min_kernel_version" json:"min_kernel_version"`     // 最低内核版本
	CheckOnStartup    bool   `yaml:"check_on_startup" json:"check_on_startup"`         // 启动时检查
	AutoDetectArch    bool   `yaml:"auto_detect_arch" json:"auto_detect_arch"`           // 自动检测架构
	FallbackToLegacy  bool   `yaml:"fallback_to_legacy" json:"fallback_to_legacy"`       // 降级到传统采集
	SupportedArchs    []string `yaml:"supported_archs" json:"supported_archs"`           // 支持的架构列表
}

// IsolationConfig 安全隔离配置
type IsolationConfig struct {
	Enabled           bool   `yaml:"enabled" json:"enabled"`
	CgroupPath        string `yaml:"cgroup_path" json:"cgroup_path"`                   // cgroup挂载路径
	CPUQuotaPercent   int    `yaml:"cpu_quota_percent" json:"cpu_quota_percent"`       // CPU限制百分比
	MemoryLimitMB     int    `yaml:"memory_limit_mb" json:"memory_limit_mb"`           // 内存限制MB
	IOWeight          int    `yaml:"io_weight" json:"io_weight"`                       // IO权重
	NiceLevel         int    `yaml:"nice_level" json:"nice_level"`                     // nice值
	OOMPriority       int    `yaml:"oom_priority" json:"oom_priority"`                 // OOM优先级
	NoNewPrivileges   bool   `yaml:"no_new_privileges" json:"no_new_privileges"`       // 禁止提权
	ZeroInterference  bool   `yaml:"zero_interference" json:"zero_interference"`       // 零干扰模式
}

// EBPFResourceConfig eBPF资源限制配置
type EBPFResourceConfig struct {
	Enabled          bool    `yaml:"enabled" json:"enabled"`
	CPUMaxPercent    float64 `yaml:"cpu_max_percent" json:"cpu_max_percent"`           // CPU最大使用率
	MemoryMaxMB      int     `yaml:"memory_max_mb" json:"memory_max_mb"`               // 内存最大使用
	SampleRateBase   int     `yaml:"sample_rate_base" json:"sample_rate_base"`         // 基础采样率
	SampleRateMin    int     `yaml:"sample_rate_min" json:"sample_rate_min"`           // 最小采样率
	SampleRateMax    int     `yaml:"sample_rate_max" json:"sample_rate_max"`           // 最大采样率
	AdaptiveEnabled  bool    `yaml:"adaptive_enabled" json:"adaptive_enabled"`         // 启用自适应采样
	TrafficThreshold int64   `yaml:"traffic_threshold" json:"traffic_threshold"`       // 流量阈值
	CheckIntervalSec int     `yaml:"check_interval_sec" json:"check_interval_sec"`     // 检查间隔
}

// TCPMetricsConfig TCP核心指标配置
type TCPMetricsConfig struct {
	Enabled            bool `yaml:"enabled" json:"enabled"`
	CollectIntervalSec int  `yaml:"collect_interval_sec" json:"collect_interval_sec"` // 采集间隔
	CollectLatency     bool `yaml:"collect_latency" json:"collect_latency"`           // 采集建连时延
	DetectZeroWindow   bool `yaml:"detect_zero_window" json:"detect_zero_window"`     // 检测零窗口
	DetectQueueOverflow bool `yaml:"detect_queue_overflow" json:"detect_queue_overflow"` // 检测队列溢出
	MaxHistorySize     int  `yaml:"max_history_size" json:"max_history_size"`         // 最大历史记录数
}

// ============================================================
// 协议解析配置模型
// ============================================================

// ProtocolConfig 协议解析配置
type ProtocolConfig struct {
	Enabled      bool                  `yaml:"enabled" json:"enabled"`
	HTTP        HTTPProtocolConfig    `yaml:"http" json:"http"`
	DNS         DNSProtocolConfig     `yaml:"dns" json:"dns"`
	MySQL       MySQLProtocolConfig   `yaml:"mysql" json:"mysql"`
}

// HTTPProtocolConfig HTTP协议解析配置
type HTTPProtocolConfig struct {
	Enabled       bool   `yaml:"enabled" json:"enabled"`
	ParseCookies  bool   `yaml:"parse_cookies" json:"parse_cookies"`
	ParseHeaders  bool   `yaml:"parse_headers" json:"parse_headers"`
	ParseBody     bool   `yaml:"parse_body" json:"parse_body"`
	MaxBodySize   int    `yaml:"max_body_size" json:"max_body_size"`
	ExtractPath   bool   `yaml:"extract_path" json:"extract_path"`
	ExtractQuery  bool   `yaml:"extract_query" json:"extract_query"`
	ExtractTrace  bool   `yaml:"extract_trace" json:"extract_trace"`
}

// DNSProtocolConfig DNS协议解析配置
type DNSProtocolConfig struct {
	Enabled      bool `yaml:"enabled" json:"enabled"`
	CollectTxID bool `yaml:"collect_txid" json:"collect_txid"` // 采集事务ID
	ParseAnswers bool `yaml:"parse_answers" json:"parse_answers"`
	ParseEDNS    bool `yaml:"parse_edns" json:"parse_edns"`
}

// MySQLProtocolConfig MySQL协议解析配置
type MySQLProtocolConfig struct {
	Enabled        bool   `yaml:"enabled" json:"enabled"`
	ParseSQL      bool   `yaml:"parse_sql" json:"parse_sql"`
	ParseErrors   bool   `yaml:"parse_errors" json:"parse_errors"`
	ParseResult   bool   `yaml:"parse_result" json:"parse_result"`
	MaxSQLLength  int    `yaml:"max_sql_length" json:"max_sql_length"`
}

// ============================================================
// 管理网传输配置模型
// ============================================================

// TransportConfig 管理网传输配置
type TransportConfig struct {
	Enabled           bool                    `yaml:"enabled" json:"enabled"`
	Mode              string                  `yaml:"mode" json:"mode"`
	Protocol          string                  `yaml:"protocol" json:"protocol"`
	ManagementNetwork ManagementNetworkConfig `yaml:"management_network" json:"management_network"`
	BufferSize       int                     `yaml:"buffer_size" json:"buffer_size"`
	MaxConnections   int                     `yaml:"max_connections" json:"max_connections"`
	ConnTimeout      int                     `yaml:"conn_timeout" json:"conn_timeout"`
	WriteTimeout     int                     `yaml:"write_timeout" json:"write_timeout"`
	KeepAlive        int                     `yaml:"keepalive" json:"keepalive"`
	Reliability      ReliabilityConfig        `yaml:"reliability" json:"reliability"`
	LoadBalancing    LoadBalancingConfig      `yaml:"load_balancing" json:"load_balancing"`
}

// ManagementNetworkConfig 管理网配置
type ManagementNetworkConfig struct {
	Enabled       bool     `yaml:"enabled" json:"enabled"`
	InterfaceName string   `yaml:"interface_name" json:"interface_name"`
	BindAddress   string   `yaml:"bind_address" json:"bind_address"`
	SourceIP      string   `yaml:"source_ip" json:"source_ip"`
	AllowedIPs    []string `yaml:"allowed_ips" json:"allowed_ips"`
	BlockedIPs    []string `yaml:"blocked_ips" json:"blocked_ips"`
	AutoDetect    bool     `yaml:"auto_detect" json:"auto_detect"`
}

// ReliabilityConfig 可靠性配置
type ReliabilityConfig struct {
	Enabled       bool `yaml:"enabled" json:"enabled"`
	EnableACK     bool `yaml:"enable_ack" json:"enable_ack"`
	EnableRetry   bool `yaml:"enable_retry" json:"enable_retry"`
	EnableDedupe  bool `yaml:"enable_dedupe" json:"enable_dedupe"`
	RetryCount    int  `yaml:"retry_count" json:"retry_count"`
	RetryInterval int  `yaml:"retry_interval" json:"retry_interval"`
	AckTimeout    int  `yaml:"ack_timeout" json:"ack_timeout"`
	WindowSize    int  `yaml:"window_size" json:"window_size"`
}

// LoadBalancingConfig 负载均衡配置
type LoadBalancingConfig struct {
	Enabled            bool   `yaml:"enabled" json:"enabled"`
	Strategy           string `yaml:"strategy" json:"strategy"`
	HealthCheck        bool   `yaml:"health_check" json:"health_check"`
	HealthCheckInterval int    `yaml:"health_check_interval" json:"health_check_interval"`
}

// ============================================================
// 熔断保护配置模型
// ============================================================

// CircuitBreakerConfig 熔断保护配置
type CircuitBreakerConfig struct {
	Enabled              bool    `yaml:"enabled" json:"enabled"`
	CPUWarningThreshold float64 `yaml:"cpu_warning_threshold" json:"cpu_warning_threshold"`
	CPUCriticalThreshold float64 `yaml:"cpu_critical_threshold" json:"cpu_critical_threshold"`
	MemoryWarningThreshold float64 `yaml:"memory_warning_threshold" json:"memory_warning_threshold"`
	MemoryCriticalThreshold float64 `yaml:"memory_critical_threshold" json:"memory_critical_threshold"`
	FailureThreshold    int     `yaml:"failure_threshold" json:"failure_threshold"`
	SuccessThreshold    int     `yaml:"success_threshold" json:"success_threshold"`
	TimeoutThreshold    int     `yaml:"timeout_threshold" json:"timeout_threshold"`
	LatencyP99Threshold float64 `yaml:"latency_p99_threshold" json:"latency_p99_threshold"`
	WindowDuration      int     `yaml:"window_duration" json:"window_duration"`
	CircuitOpenDuration int     `yaml:"circuit_open_duration" json:"circuit_open_duration"`
	SilentMode          bool    `yaml:"silent_mode" json:"silent_mode"`
	DropPercentWhenOpen int     `yaml:"drop_percent_when_open" json:"drop_percent_when_open"`
}

// ============================================================
// 自监控配置模型
// ============================================================

// SelfMonitorConfig 自监控配置
type SelfMonitorConfig struct {
	Enabled           bool    `yaml:"enabled" json:"enabled"`
	IntervalSec       int     `yaml:"interval_sec" json:"interval_sec"`
	HeartbeatEnabled  bool    `yaml:"heartbeat_enabled" json:"heartbeat_enabled"`
	StatusEnabled     bool    `yaml:"status_enabled" json:"status_enabled"`
	ResourceEnabled   bool    `yaml:"resource_enabled" json:"resource_enabled"`
	HistoryEnabled    bool    `yaml:"history_enabled" json:"history_enabled"`
	ReportEndpoint    string  `yaml:"report_endpoint" json:"report_endpoint"`
	ReportTimeout     int     `yaml:"report_timeout" json:"report_timeout"`
	CPUWarningThreshold float64 `yaml:"cpu_warning_threshold" json:"cpu_warning_threshold"`
	MemoryWarningThreshold float64 `yaml:"memory_warning_threshold" json:"memory_warning_threshold"`
}

// ============================================================
// AgentConfig 主配置模型扩展
// ============================================================

// ReliableTransportConfig 可靠传输配置
type ReliableTransportConfig struct {
	Enabled      bool `yaml:"enabled" json:"enabled"`
	WindowSize   int  `yaml:"window_size" json:"window_size"`
	RetryCount   int  `yaml:"retry_count" json:"retry_count"`
	RetryInterval int  `yaml:"retry_interval" json:"retry_interval"`
	AckTimeout   int  `yaml:"ack_timeout" json:"ack_timeout"`
	DedupeEnabled bool `yaml:"dedupe_enabled" json:"dedupe_enabled"`
	DedupeSize   int  `yaml:"dedupe_size" json:"dedupe_size"`
	DedupeWindow int  `yaml:"dedupe_window" json:"dedupe_window"`
}

// ============================================================
// 动态配置与Web管理配置模型
// ============================================================

// DynConfigConfig 动态配置管理
type DynConfigConfig struct {
	Enabled      bool   `yaml:"enabled" json:"enabled"`
	WatchEnabled bool   `yaml:"watch_enabled" json:"watch_enabled"`     // 启用文件监听热加载
	WatchInterval int   `yaml:"watch_interval" json:"watch_interval"`   // 监听间隔（秒）
}

// WebAPIConfig Web管理API配置
type WebAPIConfig struct {
	Enabled      bool   `yaml:"enabled" json:"enabled"`
	ListenAddr   string `yaml:"listen_addr" json:"listen_addr"`       // 监听地址
	AuthEnabled  bool   `yaml:"auth_enabled" json:"auth_enabled"`     // 启用认证
	AuthToken    string `yaml:"auth_token" json:"auth_token"`         // 认证Token
	RateLimit    int    `yaml:"rate_limit" json:"rate_limit"`         // 每分钟请求限制
	ReadTimeout  int    `yaml:"read_timeout" json:"read_timeout"`     // 读超时（秒）
	WriteTimeout int    `yaml:"write_timeout" json:"write_timeout"`   // 写超时（秒）
}

// ============================================================
// 日志管理配置模型
// ============================================================

// LogConfig 日志管理配置
type LogConfig struct {
	Level             string `yaml:"level" json:"level"`                         // 最低日志级别
	OutputMode        string `yaml:"output_mode" json:"output_mode"`               // stdout/file/both
	LogDir            string `yaml:"log_dir" json:"log_dir"`                         // 日志目录
	LogFile           string `yaml:"log_file" json:"log_file"`                       // 日志文件名
	MaxFileSize       int64  `yaml:"max_file_size" json:"max_file_size"`             // 单文件最大大小 (MB)
	MaxTotalSize      int64  `yaml:"max_total_size" json:"max_total_size"`           // 总日志最大大小 (MB)
	MaxRetentionDays  int    `yaml:"max_retention_days" json:"max_retention_days"`     // 最大保留天数
	MaxFileCount      int    `yaml:"max_file_count" json:"max_file_count"`           // 最大文件数
	EnableColor       bool   `yaml:"enable_color" json:"enable_color"`               // 启用颜色
	EnableRotation    bool   `yaml:"enable_rotation" json:"enable_rotation"`         // 启用轮转
	RotationCheckSec  int    `yaml:"rotation_check_sec" json:"rotation_check_sec"`   // 轮转检查间隔（秒）
	EnableCompression bool   `yaml:"enable_compression" json:"enable_compression"`   // 启用压缩
	Format            string `yaml:"format" json:"format"`                         // 日志格式: text/json
}

// ============================================================
// libbpf 配置模型
// ============================================================

// LibBPFConfig libbpf 加载器配置
type LibBPFConfig struct {
	Enabled         bool              `yaml:"enabled" json:"enabled"`
	ObjectPath      string            `yaml:"object_path" json:"object_path"`           // BPF 对象文件路径
	BTFPath         string            `yaml:"btf_path" json:"btf_path"`                 // BTF 文件路径
	UseCORE         bool              `yaml:"use_core" json:"use_core"`                 // 启用 CO-RE
	MapSizes        map[string]int    `yaml:"map_sizes" json:"map_sizes"`               // Map 大小覆盖
	ProgramTypes    map[string]string `yaml:"program_types" json:"program_types"`       // 程序类型覆盖
	PinMaps         bool              `yaml:"pin_maps" json:"pin_maps"`                 // 固定 maps 到 BPF 文件系统
	PinPath         string            `yaml:"pin_path" json:"pin_path"`                 // 固定路径
}

// ============================================================
// 探针热更新配置模型
// ============================================================

// HotReloadConfig 探针热更新配置
type HotReloadConfig struct {
	Enabled             bool          `yaml:"enabled" json:"enabled"`
	AutoApply           bool          `yaml:"auto_apply" json:"auto_apply"`             // 自动应用变更
	CheckInterval       int           `yaml:"check_interval" json:"check_interval"`     // 变更检查间隔（秒）
	AttachTimeout       int           `yaml:"attach_timeout" json:"attach_timeout"`     // attach 超时（秒）
	DetachTimeout       int           `yaml:"detach_timeout" json:"detach_timeout"`     // detach 超时（秒）
	RetryCount          int           `yaml:"retry_count" json:"retry_count"`           // 失败重试次数
	RetryInterval       int           `yaml:"retry_interval" json:"retry_interval"`     // 重试间隔（秒）
	AllowPartialFailure bool          `yaml:"allow_partial_failure" json:"allow_partial_failure"`
	Probes              []ProbeConfig `yaml:"probes" json:"probes"`                     // 预定义探针列表
	Metrics             []MetricConfig `yaml:"metrics" json:"metrics"`                   // 预定义指标列表
}

// ProbeConfig 探针配置
type ProbeConfig struct {
	ID          string                 `yaml:"id" json:"id"`
	Name        string                 `yaml:"name" json:"name"`
	Type        string                 `yaml:"type" json:"type"`                         // kprobe/kretprobe/tracepoint/fentry/fexit/raw_tracepoint
	Program     string                 `yaml:"program" json:"program"`                   // eBPF 程序名
	Target      string                 `yaml:"target" json:"target"`                     // 目标函数/tracepoint
	Category    string                 `yaml:"category,omitempty" json:"category,omitempty"` // tracepoint category
	Enabled     bool                   `yaml:"enabled" json:"enabled"`
	Priority    int                    `yaml:"priority" json:"priority"`
	Description string                 `yaml:"description" json:"description"`
	Labels      map[string]string      `yaml:"labels,omitempty" json:"labels,omitempty"`
	Params      map[string]interface{} `yaml:"params,omitempty" json:"params,omitempty"`
}

// MetricConfig 指标配置
type MetricConfig struct {
	ID          string            `yaml:"id" json:"id"`
	Name        string            `yaml:"name" json:"name"`
	Description string            `yaml:"description" json:"description"`
	Unit        string            `yaml:"unit" json:"unit"`
	Type        string            `yaml:"type" json:"type"`                         // counter/gauge/histogram
	Probes      []string          `yaml:"probes" json:"probes"`                     // 依赖的探针ID
	Enabled     bool              `yaml:"enabled" json:"enabled"`
	Labels      map[string]string `yaml:"labels,omitempty" json:"labels,omitempty"`
}

// ============================================================
// 连接池配置模型
// ============================================================

// ConnPoolConfig 连接池配置
type ConnPoolConfig struct {
	Enabled             bool              `yaml:"enabled" json:"enabled"`
	MaxConnections      int               `yaml:"max_connections" json:"max_connections"`
	InitialCap          int               `yaml:"initial_cap" json:"initial_cap"`
	MaxIdle             int               `yaml:"max_idle" json:"max_idle"`
	IdleTimeout         int               `yaml:"idle_timeout" json:"idle_timeout"`           // 秒
	MaxLifetime         int               `yaml:"max_lifetime" json:"max_lifetime"`           // 秒
	EnableRateLimit     bool              `yaml:"enable_rate_limit" json:"enable_rate_limit"`
	RateLimitPerConn    int               `yaml:"rate_limit_per_conn" json:"rate_limit_per_conn"`
	BurstSize           int               `yaml:"burst_size" json:"burst_size"`
	GlobalRateLimit     int               `yaml:"global_rate_limit" json:"global_rate_limit"`
	QueueSize           int               `yaml:"queue_size" json:"queue_size"`
	QueueTimeout        int               `yaml:"queue_timeout" json:"queue_timeout"`         // 秒
	MaxMemoryMB         int64             `yaml:"max_memory_mb" json:"max_memory_mb"`
	MemoryCheckInterval int               `yaml:"memory_check_interval" json:"memory_check_interval"` // 秒
	HealthCheckInterval int               `yaml:"health_check_interval" json:"health_check_interval"` // 秒
	HealthCheckTimeout  int               `yaml:"health_check_timeout" json:"health_check_timeout"`   // 秒
	EnableAdaptive      bool              `yaml:"enable_adaptive" json:"enable_adaptive"`
	ScaleUpThreshold    float64           `yaml:"scale_up_threshold" json:"scale_up_threshold"`
	ScaleDownThreshold  float64           `yaml:"scale_down_threshold" json:"scale_down_threshold"`
	ScaleUpFactor       int               `yaml:"scale_up_factor" json:"scale_up_factor"`
	ScaleDownFactor     int               `yaml:"scale_down_factor" json:"scale_down_factor"`
}

// ============================================================
// 数据聚合配置模型
// ============================================================

// AggregationConfig 数据聚合配置
type AggregationConfig struct {
	Enabled           bool                   `yaml:"enabled" json:"enabled"`
	DefaultLevel      string                 `yaml:"default_level" json:"default_level"`         // none/second/minute/hour/day
	WindowSize        int                    `yaml:"window_size" json:"window_size"`             // 秒
	BufferSize        int                    `yaml:"buffer_size" json:"buffer_size"`
	FlushInterval     int                    `yaml:"flush_interval" json:"flush_interval"`       // 秒
	MaxDimensions     int                    `yaml:"max_dimensions" json:"max_dimensions"`
	MaxCardinality    int                    `yaml:"max_cardinality" json:"max_cardinality"`
	EnableCompression bool                   `yaml:"enable_compression" json:"enable_compression"`
	CompressionLevel  int                    `yaml:"compression_level" json:"compression_level"`
	AggLevels         []AggLevelConfig       `yaml:"agg_levels" json:"agg_levels"`
	SamplingEnabled   bool                   `yaml:"sampling_enabled" json:"sampling_enabled"`
	SamplingRate      float64                `yaml:"sampling_rate" json:"sampling_rate"`
	AdaptiveSampling  bool                   `yaml:"adaptive_sampling" json:"adaptive_sampling"`
	MemoryThresholdMB int64                  `yaml:"memory_threshold_mb" json:"memory_threshold_mb"`
	DowngradeRatio    float64                `yaml:"downgrade_ratio" json:"downgrade_ratio"`
}

// AggLevelConfig 聚合级别配置
type AggLevelConfig struct {
	Level         string   `yaml:"level" json:"level"`                   // second/minute/hour/day
	WindowSize    int      `yaml:"window_size" json:"window_size"`       // 秒
	RetentionTime int      `yaml:"retention_time" json:"retention_time"` // 秒
	AggTypes      []string `yaml:"agg_types" json:"agg_types"`           // sum/avg/min/max/count/p99/p95/p90
	Dimensions    []string `yaml:"dimensions" json:"dimensions"`
}

// ============================================================
// 断网续传配置模型
// ============================================================

// OfflineCacheConfig 断网续传配置
type OfflineCacheConfig struct {
	Enabled          bool  `yaml:"enabled" json:"enabled"`
	CacheDir        string `yaml:"cache_dir" json:"cache_dir"`
	MaxSizeMB       int64  `yaml:"max_size_mb" json:"max_size_mb"`
	MaxItems        int    `yaml:"max_items" json:"max_items"`
	MaxAge          int    `yaml:"max_age" json:"max_age"`             // 天
	SegmentSize     int    `yaml:"segment_size" json:"segment_size"`
	Compression     bool   `yaml:"compression" json:"compression"`
	FlushInterval   int    `yaml:"flush_interval" json:"flush_interval"` // 秒
	MaxRetries      int    `yaml:"max_retries" json:"max_retries"`
	RetryInterval   int    `yaml:"retry_interval" json:"retry_interval"` // 秒
	BatchSize       int    `yaml:"batch_size" json:"batch_size"`
	Parallelism     int    `yaml:"parallelism" json:"parallelism"`
	AutoSync        bool   `yaml:"auto_sync" json:"auto_sync"`
}

// ============================================================
// 探针认证配置模型
// ============================================================

// AuthConfig 探针认证配置
type AuthConfig struct {
	Enabled           bool     `yaml:"enabled" json:"enabled"`
	TokenExpiry       int      `yaml:"token_expiry" json:"token_expiry"`           // 小时
	MaxTokenPerProbe  int      `yaml:"max_token_per_probe" json:"max_token_per_probe"`
	RateLimitPerMin   int      `yaml:"rate_limit_per_min" json:"rate_limit_per_min"`
	RequireTLS        bool     `yaml:"require_tls" json:"require_tls"`
	AllowedCipherSuites []string `yaml:"allowed_cipher_suites" json:"allowed_cipher_suites"`
	HMACKey           string   `yaml:"hmac_key" json:"hmac_key"`
	JWTSecret         string   `yaml:"jwt_secret" json:"jwt_secret"`
	AdminAPIKey       string   `yaml:"admin_api_key" json:"admin_api_key"`
}

// ============================================================
// 负载均衡配置模型
// ============================================================

// LoadBalancerConfig 负载均衡配置
type LoadBalancerConfig struct {
	Enabled         bool   `yaml:"enabled" json:"enabled"`
	Algorithm       string `yaml:"algorithm" json:"algorithm"`           // round_robin/weighted/least_conn/ip_hash/consistent_hash
	HealthCheck     bool   `yaml:"health_check" json:"health_check"`
	CheckInterval   int    `yaml:"check_interval" json:"check_interval"` // 秒
	CheckTimeout    int    `yaml:"check_timeout" json:"check_timeout"`  // 秒
	CheckPath       string `yaml:"check_path" json:"check_path"`
	CircuitBreaker  bool   `yaml:"circuit_breaker" json:"circuit_breaker"`
	CBThreshold     int    `yaml:"cb_threshold" json:"cb_threshold"`      // 熔断阈值
	CBTimeout       int    `yaml:"cb_timeout" json:"cb_timeout"`         // 秒
	RateLimitEnabled bool  `yaml:"rate_limit_enabled" json:"rate_limit_enabled"`
	RateLimitPerSec int    `yaml:"rate_limit_per_sec" json:"rate_limit_per_sec"`
	BurstSize       int    `yaml:"burst_size" json:"burst_size"`
	MaxConnPerBackend int   `yaml:"max_conn_per_backend" json:"max_conn_per_backend"`
	ConnTimeout     int    `yaml:"conn_timeout" json:"conn_timeout"`     // 秒
	EnableWeighted  bool   `yaml:"enable_weighted" json:"enable_weighted"`
}

// ============================================================
// gRPC 配置模型
// ============================================================

// GRPCConfig gRPC 客户端配置
type GRPCConfig struct {
	Enabled            bool     `yaml:"enabled" json:"enabled"`
	ServerAddress      string   `yaml:"server_address" json:"server_address"`
	DialTimeout        int      `yaml:"dial_timeout" json:"dial_timeout"`           // 秒
	KeepaliveTime      int      `yaml:"keepalive_time" json:"keepalive_time"`       // 秒
	KeepaliveTimeout   int      `yaml:"keepalive_timeout" json:"keepalive_timeout"` // 秒
	MaxRecvMsgSize     int      `yaml:"max_recv_msg_size" json:"max_recv_msg_size"` // MB
	MaxSendMsgSize     int      `yaml:"max_send_msg_size" json:"max_send_msg_size"` // MB
	EnableTLS          bool     `yaml:"enable_tls" json:"enable_tls"`
	TLSCertFile        string   `yaml:"tls_cert_file" json:"tls_cert_file"`
	TLSKeyFile         string   `yaml:"tls_key_file" json:"tls_key_file"`
	TLSCAFile          string   `yaml:"tls_ca_file" json:"tls_ca_file"`
	InsecureSkipVerify bool     `yaml:"insecure_skip_verify" json:"insecure_skip_verify"`
	AuthToken          string   `yaml:"auth_token" json:"auth_token"`
	AuthInterceptor    bool     `yaml:"auth_interceptor" json:"auth_interceptor"`
	MaxRetries         int      `yaml:"max_retries" json:"max_retries"`
	RetryBackoff       int      `yaml:"retry_backoff" json:"retry_backoff"`         // 秒
}

// ============================================================
// 监控指标配置模型
// ============================================================

// MetricsConfig 监控指标配置
type MetricsConfig struct {
	Enabled            bool     `yaml:"enabled" json:"enabled"`
	CollectionInterval int      `yaml:"collection_interval" json:"collection_interval"` // 秒
	ReportInterval     int      `yaml:"report_interval" json:"report_interval"`         // 秒
	MaxMetrics         int      `yaml:"max_metrics" json:"max_metrics"`
	BufferSize         int      `yaml:"buffer_size" json:"buffer_size"`
	ReportEndpoint     string   `yaml:"report_endpoint" json:"report_endpoint"`
	EnableSystemMetrics bool    `yaml:"enable_system_metrics" json:"enable_system_metrics"`
	EnableRuntimeMetrics bool   `yaml:"enable_runtime_metrics" json:"enable_runtime_metrics"`
	EnableNetworkMetrics bool   `yaml:"enable_network_metrics" json:"enable_network_metrics"`
}

// ============================================================
// CPU 性能剖析配置模型
// ============================================================

// CPUProfilerConfig CPU性能剖析配置
type CPUProfilerConfig struct {
	Enabled          bool                `yaml:"enabled" json:"enabled"`
	OnCPUEnabled     bool                `yaml:"on_cpu_enabled" json:"on_cpu_enabled"`         // 启用ON-CPU剖析
	OffCPUEnabled    bool                `yaml:"off_cpu_enabled" json:"off_cpu_enabled"`       // 启用OFF-CPU剖析
	SampleRate       int                 `yaml:"sample_rate" json:"sample_rate"`               // 采样率 (Hz)
	Duration         int                 `yaml:"duration" json:"duration"`                     // 单次剖析时长（秒）
	Interval         int                 `yaml:"interval" json:"interval"`                     // 剖析间隔（秒）
	OutputDir        string              `yaml:"output_dir" json:"output_dir"`                 // 输出目录
	MaxProfiles      int                 `yaml:"max_profiles" json:"max_profiles"`             // 最大保留剖析文件数
	TargetPIDs       []uint32            `yaml:"target_pids" json:"target_pids"`               // 目标进程ID
	TargetProcesses  []string            `yaml:"target_processes" json:"target_processes"`     // 目标进程名
	MinBlockTimeMs   int                 `yaml:"min_block_time_ms" json:"min_block_time_ms"`   // 最小阻塞时间（毫秒）
	MaxStackDepth    int                 `yaml:"max_stack_depth" json:"max_stack_depth"`       // 最大栈深度
	SymbolResolution bool                `yaml:"symbol_resolution" json:"symbol_resolution"`   // 符号解析
	FlameGraphEnabled bool               `yaml:"flame_graph_enabled" json:"flame_graph_enabled"` // 生成火焰图
	ReportFormat     string              `yaml:"report_format" json:"report_format"`           // 报告格式: pprof/flame/svg
}

// ============================================================
// SQL性能剖析配置模型
// ============================================================

// SQLProfilerConfig SQL性能剖析配置
type SQLProfilerConfig struct {
	Enabled              bool     `yaml:"enabled" json:"enabled"`
	CaptureQueries       bool     `yaml:"capture_queries" json:"capture_queries"`           // 捕获SQL语句
	CaptureSlowQueries   bool     `yaml:"capture_slow_queries" json:"capture_slow_queries"` // 捕获慢查询
	SlowQueryThresholdMs int      `yaml:"slow_query_threshold_ms" json:"slow_query_threshold_ms"` // 慢查询阈值（毫秒）
	MaxQueryLength       int      `yaml:"max_query_length" json:"max_query_length"`         // 最大SQL长度
	AggregationEnabled   bool     `yaml:"aggregation_enabled" json:"aggregation_enabled"`   // 启用聚合
	AggregationWindow    int      `yaml:"aggregation_window" json:"aggregation_window"`     // 聚合窗口（秒）
	FingerprintEnabled   bool     `yaml:"fingerprint_enabled" json:"fingerprint_enabled"`   // 启用SQL指纹
	TopNQueries          int      `yaml:"top_n_queries" json:"top_n_queries"`               // TopN查询数
	OutputDir            string   `yaml:"output_dir" json:"output_dir"`                     // 输出目录
	TargetDatabases      []string `yaml:"target_databases" json:"target_databases"`         // 目标数据库类型
	TargetPorts          []uint16 `yaml:"target_ports" json:"target_ports"`                 // 目标端口
}

// ============================================================
// 高可用集群配置模型
// ============================================================

// HAClusterConfig 高可用集群配置
type HAClusterConfig struct {
	Enabled           bool           `yaml:"enabled" json:"enabled"`
	NodeID            string         `yaml:"node_id" json:"node_id"`                           // 节点ID
	NodeName          string         `yaml:"node_name" json:"node_name"`                       // 节点名称
	BindAddr          string         `yaml:"bind_addr" json:"bind_addr"`                       // 绑定地址
	BindPort          int            `yaml:"bind_port" json:"bind_port"`                       // 绑定端口
	Peers             []HAClusterPeer `yaml:"peers" json:"peers"`                              // 集群节点列表
	ElectionTimeout   int            `yaml:"election_timeout" json:"election_timeout"`         // 选举超时（毫秒）
	HeartbeatInterval int            `yaml:"heartbeat_interval" json:"heartbeat_interval"`     // 心跳间隔（毫秒）
	HeartbeatTimeout  int            `yaml:"heartbeat_timeout" json:"heartbeat_timeout"`       // 心跳超时（毫秒）
	DataDir           string         `yaml:"data_dir" json:"data_dir"`                         // 数据目录
	ReplicationFactor int            `yaml:"replication_factor" json:"replication_factor"`     // 复制因子
	ShardCount        int            `yaml:"shard_count" json:"shard_count"`                   // 分片数
	AutoFailover      bool           `yaml:"auto_failover" json:"auto_failover"`               // 自动故障转移
	AutoRecover       bool           `yaml:"auto_recover" json:"auto_recover"`                 // 自动恢复
	SplitBrainCheck   bool           `yaml:"split_brain_check" json:"split_brain_check"`       // 脑裂检测
	QuorumRequired    bool           `yaml:"quorum_required" json:"quorum_required"`           // 需要仲裁
}

// HAClusterPeer 集群节点配置
type HAClusterPeer struct {
	ID       string `yaml:"id" json:"id"`               // 节点ID
	Name     string `yaml:"name" json:"name"`           // 节点名称
	Address  string `yaml:"address" json:"address"`     // 节点地址
	Port     int    `yaml:"port" json:"port"`           // 节点端口
	Region   string `yaml:"region" json:"region"`       // 区域
	Priority int    `yaml:"priority" json:"priority"`   // 优先级（选举权重）
	Weight   int    `yaml:"weight" json:"weight"`       // 负载权重
}

// ============================================================
// 存储留存策略配置模型
// ============================================================

// RetentionConfig 数据留存策略配置
type RetentionConfig struct {
	Enabled           bool                    `yaml:"enabled" json:"enabled"`
	MaxAgeDays        int                     `yaml:"max_age_days" json:"max_age_days"`               // 最大保留天数（默认60天）
	MaxSizeGB         int64                   `yaml:"max_size_gb" json:"max_size_gb"`                 // 最大磁盘使用量 (GB)
	MinFreeSpaceGB    int64                   `yaml:"min_free_space_gb" json:"min_free_space_gb"`     // 最小剩余空间 (GB)
	CleanupInterval   int                     `yaml:"cleanup_interval" json:"cleanup_interval"`       // 清理检查间隔（分钟）
	ArchiveEnabled    bool                    `yaml:"archive_enabled" json:"archive_enabled"`         // 是否归档
	ArchivePath       string                  `yaml:"archive_path" json:"archive_path"`               // 归档路径
	ArchiveRetentionDays int                  `yaml:"archive_retention_days" json:"archive_retention_days"` // 归档保留天数
	EmergencyCleanup  bool                    `yaml:"emergency_cleanup" json:"emergency_cleanup"`     // 紧急清理
	Categories        []RetentionCategoryConfig `yaml:"categories" json:"categories"`                   // 分类配置
}

// RetentionCategoryConfig 留存分类配置
type RetentionCategoryConfig struct {
	Category       string `yaml:"category" json:"category"`                         // 类别名称
	MaxAgeDays     int    `yaml:"max_age_days" json:"max_age_days"`                 // 最大保留天数
	MaxSizeGB      int64  `yaml:"max_size_gb" json:"max_size_gb"`                   // 最大大小 (GB)
	ArchiveEnabled bool   `yaml:"archive_enabled" json:"archive_enabled"`           // 是否归档
	Compress       bool   `yaml:"compress" json:"compress"`                         // 是否压缩
}
