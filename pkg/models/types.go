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
