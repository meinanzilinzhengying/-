// Package edge 包含所有gRPC接口定义和消息结构体
// 统一Agent-Edge-Center通信数据结构
//
// 版本: v2.0
// 更新日期: 2026-05-24
// 变更说明:
//   - 统一所有请求/响应字段命名规范
//   - 补齐缺失的认证、资源、网络信息字段
//   - 新增配置管理、服务发现、日志采集消息类型
//   - 完善链路追踪Span结构
//   - 增加性能分析详细字段
//
// 设计原则：
// 1. 所有消息类型实现 proto.Message 接口
// 2. 使用JSON编解码便于调试
// 3. 字段命名统一使用驼峰式
// 4. 时间戳统一使用Unix时间戳（秒或毫秒）
// 5. 枚举类型使用string便于可读性
//
// 服务划分：
// - ProbeService: Agent -> Edge (探针注册、心跳、数据上报)
// - CenterService: Edge -> Center (边缘节点注册、数据转发)
// - ConfigService: Agent <-> Edge/Center (配置管理)
// - DiscoveryService: Agent -> Edge/Center (服务发现)
package edge

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/golang/protobuf/proto"
	"google.golang.org/grpc"
)

// ============================================================================
// 基础类型定义
// ============================================================================

// AssetType 资产类型
type AssetType string

const (
	AssetTypeHost       AssetType = "host"       // 主机
	AssetTypeVM         AssetType = "vm"         // 虚拟机
	AssetTypeContainer  AssetType = "container"  // 容器
	AssetTypeK8sPod     AssetType = "k8s_pod"    // K8s Pod
	AssetTypeNetwork    AssetType = "network"    // 网络设备
	AssetTypeDatabase   AssetType = "database"   // 数据库
	AssetTypeMiddleware AssetType = "middleware" // 中间件
)

// ProtocolType 协议类型
type ProtocolType string

const (
	ProtocolTCP   ProtocolType = "tcp"   // TCP
	ProtocolUDP   ProtocolType = "udp"   // UDP
	ProtocolHTTP  ProtocolType = "http"  // HTTP/1.x
	ProtocolHTTP2 ProtocolType = "http2" // HTTP/2
	ProtocolHTTPS ProtocolType = "https" // HTTPS
	ProtocolGRPC  ProtocolType = "grpc"  // gRPC
	ProtocolDNS   ProtocolType = "dns"   // DNS
	ProtocolMySQL ProtocolType = "mysql" // MySQL
	ProtocolRedis ProtocolType = "redis" // Redis
	ProtocolKafka ProtocolType = "kafka" // Kafka
)

// ProbeStatus 探针状态
type ProbeStatus string

const (
	ProbeStatusOnline      ProbeStatus = "online"      // 在线
	ProbeStatusOffline     ProbeStatus = "offline"     // 离线
	ProbeStatusRegistering ProbeStatus = "registering" // 注册中
	ProbeStatusError       ProbeStatus = "error"       // 错误
	ProbeStatusUpgrading   ProbeStatus = "upgrading"   // 升级中
)

// MetricType 指标类型
type MetricType string

const (
	MetricTypeCounter   MetricType = "counter"   // 计数器
	MetricTypeGauge     MetricType = "gauge"     // 仪表盘
	MetricTypeHistogram MetricType = "histogram" // 直方图
	MetricTypeSummary   MetricType = "summary"   // 摘要
)

// SpanStatus Span状态
type SpanStatus string

const (
	SpanStatusOk    SpanStatus = "ok"    // 成功
	SpanStatusError SpanStatus = "error" // 错误
	SpanStatusUnset SpanStatus = "unset" // 未设置
)

// ProfilingType 性能分析类型
type ProfilingType string

const (
	ProfilingTypeCPU       ProfilingType = "cpu"       // CPU分析
	ProfilingTypeMemory    ProfilingType = "memory"    // 内存分析
	ProfilingTypeGoroutine ProfilingType = "goroutine" // 协程分析
	ProfilingTypeBlock     ProfilingType = "block"     // 阻塞分析
	ProfilingTypeMutex     ProfilingType = "mutex"     // 锁分析
	ProfilingTypeTrace     ProfilingType = "trace"     // 追踪分析
)

// LogLevel 日志级别
type LogLevel string

const (
	LogLevelDebug LogLevel = "debug"
	LogLevelInfo  LogLevel = "info"
	LogLevelWarn  LogLevel = "warn"
	LogLevelError LogLevel = "error"
	LogLevelFatal LogLevel = "fatal"
)

// ============================================================================
// 通用消息类型
// ============================================================================

// CommonResponse 通用响应
// 用于简单的成功/失败响应场景
type CommonResponse struct {
	Success   bool   `json:"success,omitempty"`    // 是否成功
	Code      string `json:"code,omitempty"`       // 错误码
	Message   string `json:"message,omitempty"`    // 消息
	RequestId string `json:"request_id,omitempty"` // 请求ID（用于追踪）
}

func (m *CommonResponse) Reset()         { *m = CommonResponse{} }
func (m *CommonResponse) String() string { return fmt.Sprintf("%+v", *m) }
func (m *CommonResponse) ProtoMessage()  {}

func (m *CommonResponse) Marshal() ([]byte, error)         { return json.Marshal(m) }
func (m *CommonResponse) Unmarshal(data []byte) error      { return json.Unmarshal(data, m) }
func (m *CommonResponse) GetSuccess() bool                 { return m.Success }
func (m *CommonResponse) GetCode() string                  { return m.Code }
func (m *CommonResponse) GetMessage() string               { return m.Message }
func (m *CommonResponse) GetRequestId() string             { return m.RequestId }

// Pagination 分页参数
// 用于列表查询请求
type Pagination struct {
	Page     int32 `json:"page,omitempty"`      // 页码（从1开始）
	PageSize int32 `json:"page_size,omitempty"` // 每页大小
}

func (m *Pagination) Reset()         { *m = Pagination{} }
func (m *Pagination) String() string { return fmt.Sprintf("%+v", *m) }
func (m *Pagination) ProtoMessage()  {}

func (m *Pagination) Marshal() ([]byte, error)         { return json.Marshal(m) }
func (m *Pagination) Unmarshal(data []byte) error      { return json.Unmarshal(data, m) }
func (m *Pagination) GetPage() int32                   { return m.Page }
func (m *Pagination) GetPageSize() int32               { return m.PageSize }

// PageInfo 分页信息
// 用于列表查询响应
type PageInfo struct {
	Total       int64 `json:"total,omitempty"`        // 总记录数
	Page        int32 `json:"page,omitempty"`         // 当前页码
	PageSize    int32 `json:"page_size,omitempty"`    // 每页大小
	TotalPages  int32 `json:"total_pages,omitempty"`  // 总页数
	HasNext     bool  `json:"has_next,omitempty"`     // 是否有下一页
	HasPrevious bool  `json:"has_previous,omitempty"` // 是否有上一页
}

func (m *PageInfo) Reset()         { *m = PageInfo{} }
func (m *PageInfo) String() string { return fmt.Sprintf("%+v", *m) }
func (m *PageInfo) ProtoMessage()  {}

func (m *PageInfo) Marshal() ([]byte, error)         { return json.Marshal(m) }
func (m *PageInfo) Unmarshal(data []byte) error      { return json.Unmarshal(data, m) }
func (m *PageInfo) GetTotal() int64                  { return m.Total }
func (m *PageInfo) GetPage() int32                   { return m.Page }
func (m *PageInfo) GetPageSize() int32               { return m.PageSize }
func (m *PageInfo) GetTotalPages() int32             { return m.TotalPages }
func (m *PageInfo) GetHasNext() bool                 { return m.HasNext }
func (m *PageInfo) GetHasPrevious() bool             { return m.HasPrevious }

// ResourceInfo 资源信息
// 描述主机/容器的资源使用情况
type ResourceInfo struct {
	CpuCores      float64 `json:"cpu_cores,omitempty"`       // CPU核心数
	CpuUsage      float64 `json:"cpu_usage,omitempty"`       // CPU使用率(0-100)
	MemoryTotal   int64   `json:"memory_total,omitempty"`    // 内存总量(字节)
	MemoryUsed    int64   `json:"memory_used,omitempty"`     // 内存使用(字节)
	MemoryUsage   float64 `json:"memory_usage,omitempty"`    // 内存使用率(0-100)
	DiskTotal     int64   `json:"disk_total,omitempty"`      // 磁盘总量(字节)
	DiskUsed      int64   `json:"disk_used,omitempty"`       // 磁盘使用(字节)
	DiskUsage     float64 `json:"disk_usage,omitempty"`      // 磁盘使用率(0-100)
	NetworkRxRate int64   `json:"network_rx_rate,omitempty"` // 网络接收速率(B/s)
	NetworkTxRate int64   `json:"network_tx_rate,omitempty"` // 网络发送速率(B/s)
	LoadAvg1      float64 `json:"load_avg_1,omitempty"`      // 1分钟负载
	LoadAvg5      float64 `json:"load_avg_5,omitempty"`      // 5分钟负载
	LoadAvg15     float64 `json:"load_avg_15,omitempty"`     // 15分钟负载
}

func (m *ResourceInfo) Reset()         { *m = ResourceInfo{} }
func (m *ResourceInfo) String() string { return fmt.Sprintf("%+v", *m) }
func (m *ResourceInfo) ProtoMessage()  {}

func (m *ResourceInfo) Marshal() ([]byte, error)         { return json.Marshal(m) }
func (m *ResourceInfo) Unmarshal(data []byte) error      { return json.Unmarshal(data, m) }
func (m *ResourceInfo) GetCpuCores() float64             { return m.CpuCores }
func (m *ResourceInfo) GetCpuUsage() float64             { return m.CpuUsage }
func (m *ResourceInfo) GetMemoryTotal() int64            { return m.MemoryTotal }
func (m *ResourceInfo) GetMemoryUsed() int64             { return m.MemoryUsed }
func (m *ResourceInfo) GetMemoryUsage() float64          { return m.MemoryUsage }
func (m *ResourceInfo) GetDiskTotal() int64              { return m.DiskTotal }
func (m *ResourceInfo) GetDiskUsed() int64               { return m.DiskUsed }
func (m *ResourceInfo) GetDiskUsage() float64            { return m.DiskUsage }
func (m *ResourceInfo) GetNetworkRxRate() int64          { return m.NetworkRxRate }
func (m *ResourceInfo) GetNetworkTxRate() int64          { return m.NetworkTxRate }
func (m *ResourceInfo) GetLoadAvg1() float64             { return m.LoadAvg1 }
func (m *ResourceInfo) GetLoadAvg5() float64             { return m.LoadAvg5 }
func (m *ResourceInfo) GetLoadAvg15() float64            { return m.LoadAvg15 }

// NetworkInfo 网络信息
type NetworkInfo struct {
	Interfaces []InterfaceInfo `json:"interfaces,omitempty"`  // 网卡列表
	Routes     []RouteInfo     `json:"routes,omitempty"`      // 路由列表
	DnsServers []string        `json:"dns_servers,omitempty"` // DNS服务器
}

func (m *NetworkInfo) Reset()         { *m = NetworkInfo{} }
func (m *NetworkInfo) String() string { return fmt.Sprintf("%+v", *m) }
func (m *NetworkInfo) ProtoMessage()  {}

func (m *NetworkInfo) Marshal() ([]byte, error)         { return json.Marshal(m) }
func (m *NetworkInfo) Unmarshal(data []byte) error      { return json.Unmarshal(data, m) }
func (m *NetworkInfo) GetInterfaces() []InterfaceInfo   { return m.Interfaces }
func (m *NetworkInfo) GetRoutes() []RouteInfo           { return m.Routes }
func (m *NetworkInfo) GetDnsServers() []string          { return m.DnsServers }

// InterfaceInfo 网卡信息
type InterfaceInfo struct {
	Name       string   `json:"name,omitempty"`        // 网卡名称
	Mac        string   `json:"mac,omitempty"`         // MAC地址
	Ips        []string `json:"ips,omitempty"`         // IP地址列表
	IsLoopback bool     `json:"is_loopback,omitempty"` // 是否回环
	IsUp       bool     `json:"is_up,omitempty"`       // 是否启用
	Speed      int64    `json:"speed,omitempty"`       // 速率(bps)
}

func (m *InterfaceInfo) Reset()         { *m = InterfaceInfo{} }
func (m *InterfaceInfo) String() string { return fmt.Sprintf("%+v", *m) }
func (m *InterfaceInfo) ProtoMessage()  {}

func (m *InterfaceInfo) Marshal() ([]byte, error)         { return json.Marshal(m) }
func (m *InterfaceInfo) Unmarshal(data []byte) error      { return json.Unmarshal(data, m) }
func (m *InterfaceInfo) GetName() string                  { return m.Name }
func (m *InterfaceInfo) GetMac() string                   { return m.Mac }
func (m *InterfaceInfo) GetIps() []string                 { return m.Ips }
func (m *InterfaceInfo) GetIsLoopback() bool              { return m.IsLoopback }
func (m *InterfaceInfo) GetIsUp() bool                    { return m.IsUp }
func (m *InterfaceInfo) GetSpeed() int64                  { return m.Speed }

// RouteInfo 路由信息
type RouteInfo struct {
	Destination string `json:"destination,omitempty"` // 目标网络
	Gateway     string `json:"gateway,omitempty"`     // 网关
	Interface   string `json:"interface,omitempty"`   // 网卡
	Metric      int32  `json:"metric,omitempty"`      // 度量
}

func (m *RouteInfo) Reset()         { *m = RouteInfo{} }
func (m *RouteInfo) String() string { return fmt.Sprintf("%+v", *m) }
func (m *RouteInfo) ProtoMessage()  {}

func (m *RouteInfo) Marshal() ([]byte, error)         { return json.Marshal(m) }
func (m *RouteInfo) Unmarshal(data []byte) error      { return json.Unmarshal(data, m) }
func (m *RouteInfo) GetDestination() string           { return m.Destination }
func (m *RouteInfo) GetGateway() string               { return m.Gateway }
func (m *RouteInfo) GetInterface() string             { return m.Interface }
func (m *RouteInfo) GetMetric() int32                 { return m.Metric }

// ============================================================================
// ProbeService 消息类型 (Agent -> Edge)
// ============================================================================

// RegisterProbeRequest 探针注册请求
type RegisterProbeRequest struct {
	ProbeId         string            `json:"probe_id,omitempty"`         // 探针唯一ID
	HostIp          string            `json:"host_ip,omitempty"`          // 主机IP
	Hostname        string            `json:"hostname,omitempty"`         // 主机名
	Version         string            `json:"version,omitempty"`          // 探针版本
	OsType          string            `json:"os_type,omitempty"`          // 操作系统类型
	OsVersion       string            `json:"os_version,omitempty"`       // 操作系统版本
	Arch            string            `json:"arch,omitempty"`             // 系统架构
	KernelVersion   string            `json:"kernel_version,omitempty"`   // 内核版本
	AssetId         string            `json:"asset_id,omitempty"`         // 资产ID（关联CMDB）
	AssetType       AssetType         `json:"asset_type,omitempty"`       // 资产类型
	CloudPlatform   string            `json:"cloud_platform,omitempty"`   // 云平台
	Region          string            `json:"region,omitempty"`           // 区域
	Zone            string            `json:"zone,omitempty"`             // 可用区
	VpcId           string            `json:"vpc_id,omitempty"`           // VPC ID
	Labels          map[string]string `json:"labels,omitempty"`           // 标签
	Token           string            `json:"token,omitempty"`            // 认证Token
	ClientCert      string            `json:"client_cert,omitempty"`      // 客户端证书（PEM格式）
	Resources       *ResourceInfo     `json:"resources,omitempty"`        // 资源信息
	Network         *NetworkInfo      `json:"network,omitempty"`          // 网络信息
	Capabilities    []string          `json:"capabilities,omitempty"`     // 支持的能力列表
}

func (m *RegisterProbeRequest) Reset()         { *m = RegisterProbeRequest{} }
func (m *RegisterProbeRequest) String() string { return fmt.Sprintf("%+v", *m) }
func (m *RegisterProbeRequest) ProtoMessage()  {}

func (m *RegisterProbeRequest) Marshal() ([]byte, error)         { return json.Marshal(m) }
func (m *RegisterProbeRequest) Unmarshal(data []byte) error      { return json.Unmarshal(data, m) }
func (m *RegisterProbeRequest) GetProbeId() string               { return m.ProbeId }
func (m *RegisterProbeRequest) GetHostIp() string                { return m.HostIp }
func (m *RegisterProbeRequest) GetHostname() string              { return m.Hostname }
func (m *RegisterProbeRequest) GetVersion() string               { return m.Version }
func (m *RegisterProbeRequest) GetOsType() string                { return m.OsType }
func (m *RegisterProbeRequest) GetOsVersion() string             { return m.OsVersion }
func (m *RegisterProbeRequest) GetArch() string                  { return m.Arch }
func (m *RegisterProbeRequest) GetKernelVersion() string         { return m.KernelVersion }
func (m *RegisterProbeRequest) GetAssetId() string               { return m.AssetId }
func (m *RegisterProbeRequest) GetAssetType() AssetType          { return m.AssetType }
func (m *RegisterProbeRequest) GetCloudPlatform() string         { return m.CloudPlatform }
func (m *RegisterProbeRequest) GetRegion() string                { return m.Region }
func (m *RegisterProbeRequest) GetZone() string                  { return m.Zone }
func (m *RegisterProbeRequest) GetVpcId() string                 { return m.VpcId }
func (m *RegisterProbeRequest) GetLabels() map[string]string     { return m.Labels }
func (m *RegisterProbeRequest) GetToken() string                 { return m.Token }
func (m *RegisterProbeRequest) GetClientCert() string            { return m.ClientCert }
func (m *RegisterProbeRequest) GetResources() *ResourceInfo      { return m.Resources }
func (m *RegisterProbeRequest) GetNetwork() *NetworkInfo         { return m.Network }
func (m *RegisterProbeRequest) GetCapabilities() []string        { return m.Capabilities }

// RegisterProbeResponse 探针注册响应
type RegisterProbeResponse struct {
	Success           bool            `json:"success,omitempty"`            // 是否成功
	Code              string          `json:"code,omitempty"`               // 错误码
	Message           string          `json:"message,omitempty"`            // 消息
	HeartbeatInterval int32           `json:"heartbeat_interval,omitempty"` // 心跳间隔(秒)
	ConfigVersion     int64           `json:"config_version,omitempty"`     // 配置版本号
	AssignedEdgeId    string          `json:"assigned_edge_id,omitempty"`   // 分配的Edge节点ID
	ServerTime        int64           `json:"server_time,omitempty"`        // 服务器时间戳
	SessionId         string          `json:"session_id,omitempty"`         // 会话ID
	Features          map[string]bool `json:"features,omitempty"`           // 启用的功能开关
}

func (m *RegisterProbeResponse) Reset()         { *m = RegisterProbeResponse{} }
func (m *RegisterProbeResponse) String() string { return fmt.Sprintf("%+v", *m) }
func (m *RegisterProbeResponse) ProtoMessage()  {}

func (m *RegisterProbeResponse) Marshal() ([]byte, error)         { return json.Marshal(m) }
func (m *RegisterProbeResponse) Unmarshal(data []byte) error      { return json.Unmarshal(data, m) }
func (m *RegisterProbeResponse) GetSuccess() bool                 { return m.Success }
func (m *RegisterProbeResponse) GetCode() string                  { return m.Code }
func (m *RegisterProbeResponse) GetMessage() string               { return m.Message }
func (m *RegisterProbeResponse) GetHeartbeatInterval() int32      { return m.HeartbeatInterval }
func (m *RegisterProbeResponse) GetConfigVersion() int64          { return m.ConfigVersion }
func (m *RegisterProbeResponse) GetAssignedEdgeId() string        { return m.AssignedEdgeId }
func (m *RegisterProbeResponse) GetServerTime() int64             { return m.ServerTime }
func (m *RegisterProbeResponse) GetSessionId() string             { return m.SessionId }
func (m *RegisterProbeResponse) GetFeatures() map[string]bool     { return m.Features }

// HeartbeatRequest 心跳请求
type HeartbeatRequest struct {
	ProbeId          string            `json:"probe_id,omitempty"`          // 探针ID
	Timestamp        int64             `json:"timestamp,omitempty"`         // 时间戳
	SessionId        string            `json:"session_id,omitempty"`        // 会话ID
	Status           ProbeStatus       `json:"status,omitempty"`            // 探针状态
	Uptime           int64             `json:"uptime,omitempty"`            // 运行时长(秒)
	Resources        *ResourceInfo     `json:"resources,omitempty"`         // 资源使用情况
	ActiveTasks      int32             `json:"active_tasks,omitempty"`      // 活跃任务数
	DroppedPackets   int64             `json:"dropped_packets,omitempty"`   // 丢包数
	CollectedMetrics int64             `json:"collected_metrics,omitempty"` // 已采集指标数
	ConfigVersion    int64             `json:"config_version,omitempty"`    // 当前配置版本
	Capabilities     []string          `json:"capabilities,omitempty"`      // 当前启用的能力
	Extensions       map[string]string `json:"extensions,omitempty"`        // 扩展字段
}

func (m *HeartbeatRequest) Reset()         { *m = HeartbeatRequest{} }
func (m *HeartbeatRequest) String() string { return fmt.Sprintf("%+v", *m) }
func (m *HeartbeatRequest) ProtoMessage()  {}

func (m *HeartbeatRequest) Marshal() ([]byte, error)         { return json.Marshal(m) }
func (m *HeartbeatRequest) Unmarshal(data []byte) error      { return json.Unmarshal(data, m) }
func (m *HeartbeatRequest) GetProbeId() string               { return m.ProbeId }
func (m *HeartbeatRequest) GetTimestamp() int64              { return m.Timestamp }
func (m *HeartbeatRequest) GetSessionId() string             { return m.SessionId }
func (m *HeartbeatRequest) GetStatus() ProbeStatus           { return m.Status }
func (m *HeartbeatRequest) GetUptime() int64                 { return m.Uptime }
func (m *HeartbeatRequest) GetResources() *ResourceInfo      { return m.Resources }
func (m *HeartbeatRequest) GetActiveTasks() int32            { return m.ActiveTasks }
func (m *HeartbeatRequest) GetDroppedPackets() int64         { return m.DroppedPackets }
func (m *HeartbeatRequest) GetCollectedMetrics() int64       { return m.CollectedMetrics }
func (m *HeartbeatRequest) GetConfigVersion() int64          { return m.ConfigVersion }
func (m *HeartbeatRequest) GetCapabilities() []string        { return m.Capabilities }
func (m *HeartbeatRequest) GetExtensions() map[string]string { return m.Extensions }

// HeartbeatResponse 心跳响应
type HeartbeatResponse struct {
	Success       bool             `json:"success,omitempty"`        // 是否成功
	Code          string           `json:"code,omitempty"`           // 错误码
	Message       string           `json:"message,omitempty"`        // 消息
	ServerTime    int64            `json:"server_time,omitempty"`    // 服务器时间戳
	ConfigVersion int64            `json:"config_version,omitempty"` // 最新配置版本
	ConfigUpdated bool             `json:"config_updated,omitempty"` // 配置是否有更新
	Commands      []*RemoteCommand `json:"commands,omitempty"`       // 待执行命令
	Actions       []string         `json:"actions,omitempty"`        // 建议动作
}

func (m *HeartbeatResponse) Reset()         { *m = HeartbeatResponse{} }
func (m *HeartbeatResponse) String() string { return fmt.Sprintf("%+v", *m) }
func (m *HeartbeatResponse) ProtoMessage()  {}

func (m *HeartbeatResponse) Marshal() ([]byte, error)         { return json.Marshal(m) }
func (m *HeartbeatResponse) Unmarshal(data []byte) error      { return json.Unmarshal(data, m) }
func (m *HeartbeatResponse) GetSuccess() bool                 { return m.Success }
func (m *HeartbeatResponse) GetCode() string                  { return m.Code }
func (m *HeartbeatResponse) GetMessage() string               { return m.Message }
func (m *HeartbeatResponse) GetServerTime() int64             { return m.ServerTime }
func (m *HeartbeatResponse) GetConfigVersion() int64          { return m.ConfigVersion }
func (m *HeartbeatResponse) GetConfigUpdated() bool           { return m.ConfigUpdated }
func (m *HeartbeatResponse) GetCommands() []*RemoteCommand    { return m.Commands }
func (m *HeartbeatResponse) GetActions() []string             { return m.Actions }

// RemoteCommand 远程命令
type RemoteCommand struct {
	CommandId string            `json:"command_id,omitempty"` // 命令ID
	Type      string            `json:"type,omitempty"`       // 命令类型
	Params    map[string]string `json:"params,omitempty"`     // 参数
	Timeout   int32             `json:"timeout,omitempty"`    // 超时(秒)
	ExecuteAt int64             `json:"execute_at,omitempty"` // 执行时间(0=立即)
	Priority  int32             `json:"priority,omitempty"`   // 优先级
}

func (m *RemoteCommand) Reset()         { *m = RemoteCommand{} }
func (m *RemoteCommand) String() string { return fmt.Sprintf("%+v", *m) }
func (m *RemoteCommand) ProtoMessage()  {}

func (m *RemoteCommand) Marshal() ([]byte, error)         { return json.Marshal(m) }
func (m *RemoteCommand) Unmarshal(data []byte) error      { return json.Unmarshal(data, m) }
func (m *RemoteCommand) GetCommandId() string             { return m.CommandId }
func (m *RemoteCommand) GetType() string                  { return m.Type }
func (m *RemoteCommand) GetParams() map[string]string     { return m.Params }
func (m *RemoteCommand) GetTimeout() int32                { return m.Timeout }
func (m *RemoteCommand) GetExecuteAt() int64              { return m.ExecuteAt }
func (m *RemoteCommand) GetPriority() int32               { return m.Priority }

// SendResponse 通用发送响应
type SendResponse struct {
	Success   bool   `json:"success,omitempty"`     // 是否成功
	Code      string `json:"code,omitempty"`        // 错误码
	Message   string `json:"message,omitempty"`     // 消息
	Accepted  int32  `json:"accepted,omitempty"`    // 接受的数据条数
	Rejected  int32  `json:"rejected,omitempty"`    // 拒绝的数据条数
	SeqId     int64  `json:"seq_id,omitempty"`      // 确认的序列号
	NextSeqId int64  `json:"next_seq_id,omitempty"` // 下一个期望的序列号
}

func (m *SendResponse) Reset()         { *m = SendResponse{} }
func (m *SendResponse) String() string { return fmt.Sprintf("%+v", *m) }
func (m *SendResponse) ProtoMessage()  {}

func (m *SendResponse) Marshal() ([]byte, error)         { return json.Marshal(m) }
func (m *SendResponse) Unmarshal(data []byte) error      { return json.Unmarshal(data, m) }
func (m *SendResponse) GetSuccess() bool                 { return m.Success }
func (m *SendResponse) GetCode() string                  { return m.Code }
func (m *SendResponse) GetMessage() string               { return m.Message }
func (m *SendResponse) GetAccepted() int32               { return m.Accepted }
func (m *SendResponse) GetRejected() int32               { return m.Rejected }
func (m *SendResponse) GetSeqId() int64                  { return m.SeqId }
func (m *SendResponse) GetNextSeqId() int64              { return m.NextSeqId }

// ============================================================================
// 数据上报消息类型
// ============================================================================

// MetricData 指标数据点
type MetricData struct {
	Name            string            `json:"name,omitempty"`             // 指标名称
	Type            MetricType        `json:"type,omitempty"`             // 指标类型
	Value           float64           `json:"value,omitempty"`            // 数值
	Count           int64             `json:"count,omitempty"`            // 计数
	Sum             float64           `json:"sum,omitempty"`              // 总和
	Min             float64           `json:"min,omitempty"`              // 最小值
	Max             float64           `json:"max,omitempty"`              // 最大值
	Avg             float64           `json:"avg,omitempty"`              // 平均值
	Timestamp       int64             `json:"timestamp,omitempty"`        // 时间戳(毫秒)
	ProbeId         string            `json:"probe_id,omitempty"`         // 探针ID
	AssetId         string            `json:"asset_id,omitempty"`         // 资产ID
	SrcIp           string            `json:"src_ip,omitempty"`           // 源IP
	DstIp           string            `json:"dst_ip,omitempty"`           // 目的IP
	SrcPort         int32             `json:"src_port,omitempty"`         // 源端口
	DstPort         int32             `json:"dst_port,omitempty"`         // 目的端口
	Protocol        ProtocolType      `json:"protocol,omitempty"`         // 协议类型
	Service         string            `json:"service,omitempty"`          // 服务名称
	Endpoint        string            `json:"endpoint,omitempty"`         // 端点
	Bytes           int64             `json:"bytes,omitempty"`            // 字节数
	Packets         int64             `json:"packets,omitempty"`          // 包数
	Latency         int64             `json:"latency,omitempty"`          // 延迟(微秒)
	LatencyP50      int64             `json:"latency_p50,omitempty"`      // P50延迟
	LatencyP90      int64             `json:"latency_p90,omitempty"`      // P90延迟
	LatencyP99      int64             `json:"latency_p99,omitempty"`      // P99延迟
	ErrorCount      int64             `json:"error_count,omitempty"`      // 错误数
	ErrorRate       float64           `json:"error_rate,omitempty"`       // 错误率
	CpuUsage        float64           `json:"cpu_usage,omitempty"`        // CPU使用率
	MemoryUsage     float64           `json:"memory_usage,omitempty"`     // 内存使用率
	DiskUsage       float64           `json:"disk_usage,omitempty"`       // 磁盘使用率
	Tags            map[string]string `json:"tags,omitempty"`             // 标签
}

func (m *MetricData) Reset()         { *m = MetricData{} }
func (m *MetricData) String() string { return fmt.Sprintf("%+v", *m) }
func (m *MetricData) ProtoMessage()  {}

func (m *MetricData) Marshal() ([]byte, error)         { return json.Marshal(m) }
func (m *MetricData) Unmarshal(data []byte) error      { return json.Unmarshal(data, m) }
func (m *MetricData) GetName() string                  { return m.Name }
func (m *MetricData) GetType() MetricType              { return m.Type }
func (m *MetricData) GetValue() float64                { return m.Value }
func (m *MetricData) GetCount() int64                  { return m.Count }
func (m *MetricData) GetSum() float64                  { return m.Sum }
func (m *MetricData) GetMin() float64                  { return m.Min }
func (m *MetricData) GetMax() float64                  { return m.Max }
func (m *MetricData) GetAvg() float64                  { return m.Avg }
func (m *MetricData) GetTimestamp() int64              { return m.Timestamp }
func (m *MetricData) GetProbeId() string               { return m.ProbeId }
func (m *MetricData) GetAssetId() string               { return m.AssetId }
func (m *MetricData) GetSrcIp() string                 { return m.SrcIp }
func (m *MetricData) GetDstIp() string                 { return m.DstIp }
func (m *MetricData) GetSrcPort() int32                { return m.SrcPort }
func (m *MetricData) GetDstPort() int32                { return m.DstPort }
func (m *MetricData) GetProtocol() ProtocolType        { return m.Protocol }
func (m *MetricData) GetService() string               { return m.Service }
func (m *MetricData) GetEndpoint() string              { return m.Endpoint }
func (m *MetricData) GetBytes() int64                  { return m.Bytes }
func (m *MetricData) GetPackets() int64                { return m.Packets }
func (m *MetricData) GetLatency() int64                { return m.Latency }
func (m *MetricData) GetLatencyP50() int64             { return m.LatencyP50 }
func (m *MetricData) GetLatencyP90() int64             { return m.LatencyP90 }
func (m *MetricData) GetLatencyP99() int64             { return m.LatencyP99 }
func (m *MetricData) GetErrorCount() int64             { return m.ErrorCount }
func (m *MetricData) GetErrorRate() float64            { return m.ErrorRate }
func (m *MetricData) GetCpuUsage() float64             { return m.CpuUsage }
func (m *MetricData) GetMemoryUsage() float64          { return m.MemoryUsage }
func (m *MetricData) GetDiskUsage() float64            { return m.DiskUsage }
func (m *MetricData) GetTags() map[string]string       { return m.Tags }

// MetricsBatch 指标批量数据
type MetricsBatch struct {
	ProbeId     string        `json:"probe_id,omitempty"`    // 探针ID
	AssetId     string        `json:"asset_id,omitempty"`    // 资产ID
	Metrics     []*MetricData `json:"metrics,omitempty"`     // 指标列表
	Count       int32         `json:"count,omitempty"`       // 数量
	Checksum    string        `json:"checksum,omitempty"`    // 校验和(SHA256)
	SeqId       int64         `json:"seq_id,omitempty"`      // 序列号
	Timestamp   int64         `json:"timestamp,omitempty"`   // 批次时间戳
	StartTime   int64         `json:"start_time,omitempty"`  // 数据起始时间
	EndTime     int64         `json:"end_time,omitempty"`    // 数据结束时间
	Aggregation string        `json:"aggregation,omitempty"` // 聚合方式
	Interval    int32         `json:"interval,omitempty"`    // 聚合间隔(秒)
}

func (m *MetricsBatch) Reset()         { *m = MetricsBatch{} }
func (m *MetricsBatch) String() string { return fmt.Sprintf("%+v", *m) }
func (m *MetricsBatch) ProtoMessage()  {}

func (m *MetricsBatch) Marshal() ([]byte, error)         { return json.Marshal(m) }
func (m *MetricsBatch) Unmarshal(data []byte) error      { return json.Unmarshal(data, m) }
func (m *MetricsBatch) GetProbeId() string               { return m.ProbeId }
func (m *MetricsBatch) GetAssetId() string               { return m.AssetId }
func (m *MetricsBatch) GetMetrics() []*MetricData        { return m.Metrics }
func (m *MetricsBatch) GetCount() int32                  { return m.Count }
func (m *MetricsBatch) GetChecksum() string              { return m.Checksum }
func (m *MetricsBatch) GetSeqId() int64                  { return m.SeqId }
func (m *MetricsBatch) GetTimestamp() int64              { return m.Timestamp }
func (m *MetricsBatch) GetStartTime() int64              { return m.StartTime }
func (m *MetricsBatch) GetEndTime() int64                { return m.EndTime }
func (m *MetricsBatch) GetAggregation() string           { return m.Aggregation }
func (m *MetricsBatch) GetInterval() int32               { return m.Interval }

// TraceSpanData 链路追踪Span数据
type TraceSpanData struct {
	TraceId         string            `json:"trace_id,omitempty"`           // Trace ID
	SpanId          string            `json:"span_id,omitempty"`            // Span ID
	ParentId        string            `json:"parent_id,omitempty"`          // 父Span ID
	ProbeId         string            `json:"probe_id,omitempty"`           // 探针ID
	Service         string            `json:"service,omitempty"`            // 服务名
	Operation       string            `json:"operation,omitempty"`          // 操作名
	StartTime       int64             `json:"start_time,omitempty"`         // 开始时间(微秒)
	EndTime         int64             `json:"end_time,omitempty"`           // 结束时间(微秒)
	Duration        int64             `json:"duration,omitempty"`           // 持续时间(微秒)
	Status          SpanStatus        `json:"status,omitempty"`             // 状态
	ErrorCode       string            `json:"error_code,omitempty"`         // 错误码
	ErrorMessage    string            `json:"error_message,omitempty"`      // 错误消息
	Protocol        ProtocolType      `json:"protocol,omitempty"`           // 协议
	SrcIp           string            `json:"src_ip,omitempty"`             // 源IP
	DstIp           string            `json:"dst_ip,omitempty"`             // 目的IP
	SrcPort         int32             `json:"src_port,omitempty"`           // 源端口
	DstPort         int32             `json:"dst_port,omitempty"`           // 目的端口
	RequestSize     int64             `json:"request_size,omitempty"`       // 请求大小
	ResponseSize    int64             `json:"response_size,omitempty"`      // 响应大小
	RequestHeaders  map[string]string `json:"request_headers,omitempty"`    // 请求头
	ResponseHeaders map[string]string `json:"response_headers,omitempty"`   // 响应头
	Tags            map[string]string `json:"tags,omitempty"`               // 标签
	Events          []*SpanEvent      `json:"events,omitempty"`             // 事件
	Links           []*SpanLink       `json:"links,omitempty"`              // 链接
}

func (m *TraceSpanData) Reset()         { *m = TraceSpanData{} }
func (m *TraceSpanData) String() string { return fmt.Sprintf("%+v", *m) }
func (m *TraceSpanData) ProtoMessage()  {}

func (m *TraceSpanData) Marshal() ([]byte, error)         { return json.Marshal(m) }
func (m *TraceSpanData) Unmarshal(data []byte) error      { return json.Unmarshal(data, m) }
func (m *TraceSpanData) GetTraceId() string               { return m.TraceId }
func (m *TraceSpanData) GetSpanId() string                { return m.SpanId }
func (m *TraceSpanData) GetParentId() string              { return m.ParentId }
func (m *TraceSpanData) GetProbeId() string               { return m.ProbeId }
func (m *TraceSpanData) GetService() string               { return m.Service }
func (m *TraceSpanData) GetOperation() string             { return m.Operation }
func (m *TraceSpanData) GetStartTime() int64              { return m.StartTime }
func (m *TraceSpanData) GetEndTime() int64                { return m.EndTime }
func (m *TraceSpanData) GetDuration() int64               { return m.Duration }
func (m *TraceSpanData) GetStatus() SpanStatus            { return m.Status }
func (m *TraceSpanData) GetErrorCode() string             { return m.ErrorCode }
func (m *TraceSpanData) GetErrorMessage() string          { return m.ErrorMessage }
func (m *TraceSpanData) GetProtocol() ProtocolType        { return m.Protocol }
func (m *TraceSpanData) GetSrcIp() string                 { return m.SrcIp }
func (m *TraceSpanData) GetDstIp() string                 { return m.DstIp }
func (m *TraceSpanData) GetSrcPort() int32                { return m.SrcPort }
func (m *TraceSpanData) GetDstPort() int32                { return m.DstPort }
func (m *TraceSpanData) GetRequestSize() int64            { return m.RequestSize }
func (m *TraceSpanData) GetResponseSize() int64           { return m.ResponseSize }
func (m *TraceSpanData) GetRequestHeaders() map[string]string  { return m.RequestHeaders }
func (m *TraceSpanData) GetResponseHeaders() map[string]string { return m.ResponseHeaders }
func (m *TraceSpanData) GetTags() map[string]string       { return m.Tags }
func (m *TraceSpanData) GetEvents() []*SpanEvent          { return m.Events }
func (m *TraceSpanData) GetLinks() []*SpanLink            { return m.Links }

// SpanEvent Span事件
type SpanEvent struct {
	Timestamp  int64             `json:"timestamp,omitempty"`  // 时间戳
	Name       string            `json:"name,omitempty"`       // 事件名
	Attributes map[string]string `json:"attributes,omitempty"` // 属性
}

func (m *SpanEvent) Reset()         { *m = SpanEvent{} }
func (m *SpanEvent) String() string { return fmt.Sprintf("%+v", *m) }
func (m *SpanEvent) ProtoMessage()  {}

func (m *SpanEvent) Marshal() ([]byte, error)         { return json.Marshal(m) }
func (m *SpanEvent) Unmarshal(data []byte) error      { return json.Unmarshal(data, m) }
func (m *SpanEvent) GetTimestamp() int64              { return m.Timestamp }
func (m *SpanEvent) GetName() string                  { return m.Name }
func (m *SpanEvent) GetAttributes() map[string]string { return m.Attributes }

// SpanLink Span链接
type SpanLink struct {
	TraceId    string            `json:"trace_id,omitempty"`   // Trace ID
	SpanId     string            `json:"span_id,omitempty"`    // Span ID
	Attributes map[string]string `json:"attributes,omitempty"` // 属性
}

func (m *SpanLink) Reset()         { *m = SpanLink{} }
func (m *SpanLink) String() string { return fmt.Sprintf("%+v", *m) }
func (m *SpanLink) ProtoMessage()  {}

func (m *SpanLink) Marshal() ([]byte, error)         { return json.Marshal(m) }
func (m *SpanLink) Unmarshal(data []byte) error      { return json.Unmarshal(data, m) }
func (m *SpanLink) GetTraceId() string               { return m.TraceId }
func (m *SpanLink) GetSpanId() string                { return m.SpanId }
func (m *SpanLink) GetAttributes() map[string]string { return m.Attributes }

// TraceBatch 链路追踪批量数据
type TraceBatch struct {
	ProbeId   string           `json:"probe_id,omitempty"`  // 探针ID
	AssetId   string           `json:"asset_id,omitempty"`  // 资产ID
	Spans     []*TraceSpanData `json:"spans,omitempty"`     // Span列表
	Count     int32            `json:"count,omitempty"`     // 数量
	Checksum  string           `json:"checksum,omitempty"`  // 校验和
	SeqId     int64            `json:"seq_id,omitempty"`    // 序列号
	Timestamp int64            `json:"timestamp,omitempty"` // 时间戳
}

func (m *TraceBatch) Reset()         { *m = TraceBatch{} }
func (m *TraceBatch) String() string { return fmt.Sprintf("%+v", *m) }
func (m *TraceBatch) ProtoMessage()  {}

func (m *TraceBatch) Marshal() ([]byte, error)         { return json.Marshal(m) }
func (m *TraceBatch) Unmarshal(data []byte) error      { return json.Unmarshal(data, m) }
func (m *TraceBatch) GetProbeId() string               { return m.ProbeId }
func (m *TraceBatch) GetAssetId() string               { return m.AssetId }
func (m *TraceBatch) GetSpans() []*TraceSpanData       { return m.Spans }
func (m *TraceBatch) GetCount() int32                  { return m.Count }
func (m *TraceBatch) GetChecksum() string              { return m.Checksum }
func (m *TraceBatch) GetSeqId() int64                  { return m.SeqId }
func (m *TraceBatch) GetTimestamp() int64              { return m.Timestamp }

// ProfilingSample 性能分析样本
type ProfilingSample struct {
	LocationId uint64            `json:"location_id,omitempty"` // 位置ID
	Value      int64             `json:"value,omitempty"`       // 采样值
	Labels     map[string]string `json:"labels,omitempty"`      // 标签
}

func (m *ProfilingSample) Reset()         { *m = ProfilingSample{} }
func (m *ProfilingSample) String() string { return fmt.Sprintf("%+v", *m) }
func (m *ProfilingSample) ProtoMessage()  {}

func (m *ProfilingSample) Marshal() ([]byte, error)         { return json.Marshal(m) }
func (m *ProfilingSample) Unmarshal(data []byte) error      { return json.Unmarshal(data, m) }
func (m *ProfilingSample) GetLocationId() uint64            { return m.LocationId }
func (m *ProfilingSample) GetValue() int64                  { return m.Value }
func (m *ProfilingSample) GetLabels() map[string]string     { return m.Labels }

// ProfilingLocation 性能分析位置
type ProfilingLocation struct {
	Id       uint64 `json:"id,omitempty"`       // ID
	Function string `json:"function,omitempty"` // 函数名
	File     string `json:"file,omitempty"`     // 文件名
	Line     int32  `json:"line,omitempty"`     // 行号
}

func (m *ProfilingLocation) Reset()         { *m = ProfilingLocation{} }
func (m *ProfilingLocation) String() string { return fmt.Sprintf("%+v", *m) }
func (m *ProfilingLocation) ProtoMessage()  {}

func (m *ProfilingLocation) Marshal() ([]byte, error)         { return json.Marshal(m) }
func (m *ProfilingLocation) Unmarshal(data []byte) error      { return json.Unmarshal(data, m) }
func (m *ProfilingLocation) GetId() uint64                    { return m.Id }
func (m *ProfilingLocation) GetFunction() string              { return m.Function }
func (m *ProfilingLocation) GetFile() string                  { return m.File }
func (m *ProfilingLocation) GetLine() int32                   { return m.Line }

// ProfilingData 性能分析数据
type ProfilingData struct {
	ProbeId   string               `json:"probe_id,omitempty"`   // 探针ID
	AssetId   string               `json:"asset_id,omitempty"`   // 资产ID
	Type      ProfilingType        `json:"type,omitempty"`       // 分析类型
	StartTime int64                `json:"start_time,omitempty"` // 开始时间
	EndTime   int64                `json:"end_time,omitempty"`   // 结束时间
	Duration  int64                `json:"duration,omitempty"`   // 持续时间
	Samples   []*ProfilingSample   `json:"samples,omitempty"`    // 样本
	Locations []*ProfilingLocation `json:"locations,omitempty"`  // 位置
	Stack     string               `json:"stack,omitempty"`      // 调用栈（简化格式）
	Count     int64                `json:"count,omitempty"`      // 计数
	TotalTime int64                `json:"total_time,omitempty"` // 总时间
	Unit      string               `json:"unit,omitempty"`       // 单位
	Labels    map[string]string    `json:"labels,omitempty"`     // 标签
}

func (m *ProfilingData) Reset()         { *m = ProfilingData{} }
func (m *ProfilingData) String() string { return fmt.Sprintf("%+v", *m) }
func (m *ProfilingData) ProtoMessage()  {}

func (m *ProfilingData) Marshal() ([]byte, error)         { return json.Marshal(m) }
func (m *ProfilingData) Unmarshal(data []byte) error      { return json.Unmarshal(data, m) }
func (m *ProfilingData) GetProbeId() string               { return m.ProbeId }
func (m *ProfilingData) GetAssetId() string               { return m.AssetId }
func (m *ProfilingData) GetType() ProfilingType           { return m.Type }
func (m *ProfilingData) GetStartTime() int64              { return m.StartTime }
func (m *ProfilingData) GetEndTime() int64                { return m.EndTime }
func (m *ProfilingData) GetDuration() int64               { return m.Duration }
func (m *ProfilingData) GetSamples() []*ProfilingSample   { return m.Samples }
func (m *ProfilingData) GetLocations() []*ProfilingLocation { return m.Locations }
func (m *ProfilingData) GetStack() string                 { return m.Stack }
func (m *ProfilingData) GetCount() int64                  { return m.Count }
func (m *ProfilingData) GetTotalTime() int64              { return m.TotalTime }
func (m *ProfilingData) GetUnit() string                  { return m.Unit }
func (m *ProfilingData) GetLabels() map[string]string     { return m.Labels }

// ProfilingBatch 性能分析批量数据
type ProfilingBatch struct {
	ProbeId   string           `json:"probe_id,omitempty"`  // 探针ID
	AssetId   string           `json:"asset_id,omitempty"`  // 资产ID
	Profiles  []*ProfilingData `json:"profiles,omitempty"`  // 分析数据列表
	Count     int32            `json:"count,omitempty"`     // 数量
	Checksum  string           `json:"checksum,omitempty"`  // 校验和
	SeqId     int64            `json:"seq_id,omitempty"`    // 序列号
	Timestamp int64            `json:"timestamp,omitempty"` // 时间戳
}

func (m *ProfilingBatch) Reset()         { *m = ProfilingBatch{} }
func (m *ProfilingBatch) String() string { return fmt.Sprintf("%+v", *m) }
func (m *ProfilingBatch) ProtoMessage()  {}

func (m *ProfilingBatch) Marshal() ([]byte, error)         { return json.Marshal(m) }
func (m *ProfilingBatch) Unmarshal(data []byte) error      { return json.Unmarshal(data, m) }
func (m *ProfilingBatch) GetProbeId() string               { return m.ProbeId }
func (m *ProfilingBatch) GetAssetId() string               { return m.AssetId }
func (m *ProfilingBatch) GetProfiles() []*ProfilingData    { return m.Profiles }
func (m *ProfilingBatch) GetCount() int32                  { return m.Count }
func (m *ProfilingBatch) GetChecksum() string              { return m.Checksum }
func (m *ProfilingBatch) GetSeqId() int64                  { return m.SeqId }
func (m *ProfilingBatch) GetTimestamp() int64              { return m.Timestamp }

// LogData 日志数据
type LogData struct {
	Timestamp int64             `json:"timestamp,omitempty"` // 时间戳
	Level     LogLevel          `json:"level,omitempty"`     // 日志级别
	Message   string            `json:"message,omitempty"`   // 消息
	Source    string            `json:"source,omitempty"`    // 来源
	ProbeId   string            `json:"probe_id,omitempty"`  // 探针ID
	AssetId   string            `json:"asset_id,omitempty"`  // 资产ID
	Service   string            `json:"service,omitempty"`   // 服务名
	TraceId   string            `json:"trace_id,omitempty"`  // Trace ID
	SpanId    string            `json:"span_id,omitempty"`   // Span ID
	Fields    map[string]string `json:"fields,omitempty"`    // 附加字段
}

func (m *LogData) Reset()         { *m = LogData{} }
func (m *LogData) String() string { return fmt.Sprintf("%+v", *m) }
func (m *LogData) ProtoMessage()  {}

func (m *LogData) Marshal() ([]byte, error)         { return json.Marshal(m) }
func (m *LogData) Unmarshal(data []byte) error      { return json.Unmarshal(data, m) }
func (m *LogData) GetTimestamp() int64              { return m.Timestamp }
func (m *LogData) GetLevel() LogLevel               { return m.Level }
func (m *LogData) GetMessage() string               { return m.Message }
func (m *LogData) GetSource() string                { return m.Source }
func (m *LogData) GetProbeId() string               { return m.ProbeId }
func (m *LogData) GetAssetId() string               { return m.AssetId }
func (m *LogData) GetService() string               { return m.Service }
func (m *LogData) GetTraceId() string               { return m.TraceId }
func (m *LogData) GetSpanId() string                { return m.SpanId }
func (m *LogData) GetFields() map[string]string     { return m.Fields }

// LogBatch 日志批量数据
type LogBatch struct {
	ProbeId   string     `json:"probe_id,omitempty"`  // 探针ID
	AssetId   string     `json:"asset_id,omitempty"`  // 资产ID
	Logs      []*LogData `json:"logs,omitempty"`      // 日志列表
	Count     int32      `json:"count,omitempty"`     // 数量
	Checksum  string     `json:"checksum,omitempty"`  // 校验和
	SeqId     int64      `json:"seq_id,omitempty"`    // 序列号
	Timestamp int64      `json:"timestamp,omitempty"` // 时间戳
}

func (m *LogBatch) Reset()         { *m = LogBatch{} }
func (m *LogBatch) String() string { return fmt.Sprintf("%+v", *m) }
func (m *LogBatch) ProtoMessage()  {}

func (m *LogBatch) Marshal() ([]byte, error)         { return json.Marshal(m) }
func (m *LogBatch) Unmarshal(data []byte) error      { return json.Unmarshal(data, m) }
func (m *LogBatch) GetProbeId() string               { return m.ProbeId }
func (m *LogBatch) GetAssetId() string               { return m.AssetId }
func (m *LogBatch) GetLogs() []*LogData              { return m.Logs }
func (m *LogBatch) GetCount() int32                  { return m.Count }
func (m *LogBatch) GetChecksum() string              { return m.Checksum }
func (m *LogBatch) GetSeqId() int64                  { return m.SeqId }
func (m *LogBatch) GetTimestamp() int64              { return m.Timestamp }

// ============================================================================
// 探针信息类型
// ============================================================================

// ProbeInfo 探针信息
type ProbeInfo struct {
	ProbeId       string            `json:"probe_id,omitempty"`        // 探针ID
	HostIp        string            `json:"host_ip,omitempty"`         // 主机IP
	Hostname      string            `json:"hostname,omitempty"`        // 主机名
	Status        ProbeStatus       `json:"status,omitempty"`          // 状态
	Version       string            `json:"version,omitempty"`         // 版本
	OsType        string            `json:"os_type,omitempty"`         // 操作系统
	OsVersion     string            `json:"os_version,omitempty"`      // OS版本
	Arch          string            `json:"arch,omitempty"`            // 架构
	AssetId       string            `json:"asset_id,omitempty"`        // 资产ID
	AssetType     AssetType         `json:"asset_type,omitempty"`      // 资产类型
	CloudPlatform string            `json:"cloud_platform,omitempty"`  // 云平台
	Region        string            `json:"region,omitempty"`          // 区域
	Zone          string            `json:"zone,omitempty"`            // 可用区
	Labels        map[string]string `json:"labels,omitempty"`          // 标签
	LastHeartbeat int64             `json:"last_heartbeat,omitempty"`  // 最后心跳时间
	RegisteredAt  int64             `json:"registered_at,omitempty"`   // 注册时间
	Uptime        int64             `json:"uptime,omitempty"`          // 运行时长
	Resources     *ResourceInfo     `json:"resources,omitempty"`       // 资源信息
	Capabilities  []string          `json:"capabilities,omitempty"`    // 能力列表
}

func (m *ProbeInfo) Reset()         { *m = ProbeInfo{} }
func (m *ProbeInfo) String() string { return fmt.Sprintf("%+v", *m) }
func (m *ProbeInfo) ProtoMessage()  {}

func (m *ProbeInfo) Marshal() ([]byte, error)         { return json.Marshal(m) }
func (m *ProbeInfo) Unmarshal(data []byte) error      { return json.Unmarshal(data, m) }
func (m *ProbeInfo) GetProbeId() string               { return m.ProbeId }
func (m *ProbeInfo) GetHostIp() string                { return m.HostIp }
func (m *ProbeInfo) GetHostname() string              { return m.Hostname }
func (m *ProbeInfo) GetStatus() ProbeStatus           { return m.Status }
func (m *ProbeInfo) GetVersion() string               { return m.Version }
func (m *ProbeInfo) GetOsType() string                { return m.OsType }
func (m *ProbeInfo) GetOsVersion() string             { return m.OsVersion }
func (m *ProbeInfo) GetArch() string                  { return m.Arch }
func (m *ProbeInfo) GetAssetId() string               { return m.AssetId }
func (m *ProbeInfo) GetAssetType() AssetType          { return m.AssetType }
func (m *ProbeInfo) GetCloudPlatform() string         { return m.CloudPlatform }
func (m *ProbeInfo) GetRegion() string                { return m.Region }
func (m *ProbeInfo) GetZone() string                  { return m.Zone }
func (m *ProbeInfo) GetLabels() map[string]string     { return m.Labels }
func (m *ProbeInfo) GetLastHeartbeat() int64          { return m.LastHeartbeat }
func (m *ProbeInfo) GetRegisteredAt() int64           { return m.RegisteredAt }
func (m *ProbeInfo) GetUptime() int64                 { return m.Uptime }
func (m *ProbeInfo) GetResources() *ResourceInfo      { return m.Resources }
func (m *ProbeInfo) GetCapabilities() []string        { return m.Capabilities }

// ============================================================================
// CenterService 消息类型 (Edge -> Center)
// ============================================================================

// ReportProbesRequest 上报探针列表请求
type ReportProbesRequest struct {
	EdgeNodeId    string       `json:"edge_node_id,omitempty"`    // Edge节点ID
	EdgeAddress   string       `json:"edge_address,omitempty"`    // Edge地址
	CloudPlatform string       `json:"cloud_platform,omitempty"`  // 云平台
	Region        string       `json:"region,omitempty"`          // 区域
	Zone          string       `json:"zone,omitempty"`            // 可用区
	Probes        []*ProbeInfo `json:"probes,omitempty"`          // 探针列表
	Timestamp     int64        `json:"timestamp,omitempty"`       // 时间戳
	TotalCount    int32        `json:"total_count,omitempty"`     // 总数量
	OnlineCount   int32        `json:"online_count,omitempty"`    // 在线数量
}

func (m *ReportProbesRequest) Reset()         { *m = ReportProbesRequest{} }
func (m *ReportProbesRequest) String() string { return fmt.Sprintf("%+v", *m) }
func (m *ReportProbesRequest) ProtoMessage()  {}

func (m *ReportProbesRequest) Marshal() ([]byte, error)         { return json.Marshal(m) }
func (m *ReportProbesRequest) Unmarshal(data []byte) error      { return json.Unmarshal(data, m) }
func (m *ReportProbesRequest) GetEdgeNodeId() string            { return m.EdgeNodeId }
func (m *ReportProbesRequest) GetEdgeAddress() string           { return m.EdgeAddress }
func (m *ReportProbesRequest) GetCloudPlatform() string         { return m.CloudPlatform }
func (m *ReportProbesRequest) GetRegion() string                { return m.Region }
func (m *ReportProbesRequest) GetZone() string                  { return m.Zone }
func (m *ReportProbesRequest) GetProbes() []*ProbeInfo          { return m.Probes }
func (m *ReportProbesRequest) GetTimestamp() int64              { return m.Timestamp }
func (m *ReportProbesRequest) GetTotalCount() int32             { return m.TotalCount }
func (m *ReportProbesRequest) GetOnlineCount() int32            { return m.OnlineCount }

// ReportProbesResponse 上报探针列表响应
type ReportProbesResponse struct {
	Success       bool             `json:"success,omitempty"`        // 是否成功
	Code          string           `json:"code,omitempty"`           // 错误码
	Message       string           `json:"message,omitempty"`        // 消息
	AcceptedCount int32            `json:"accepted_count,omitempty"` // 接受的探针数
	RejectedIds   []string         `json:"rejected_ids,omitempty"`   // 拒绝的探针ID
	ConfigUpdates map[string]int64 `json:"config_updates,omitempty"` // 各探针的配置版本
}

func (m *ReportProbesResponse) Reset()         { *m = ReportProbesResponse{} }
func (m *ReportProbesResponse) String() string { return fmt.Sprintf("%+v", *m) }
func (m *ReportProbesResponse) ProtoMessage()  {}

func (m *ReportProbesResponse) Marshal() ([]byte, error)         { return json.Marshal(m) }
func (m *ReportProbesResponse) Unmarshal(data []byte) error      { return json.Unmarshal(data, m) }
func (m *ReportProbesResponse) GetSuccess() bool                 { return m.Success }
func (m *ReportProbesResponse) GetCode() string                  { return m.Code }
func (m *ReportProbesResponse) GetMessage() string               { return m.Message }
func (m *ReportProbesResponse) GetAcceptedCount() int32          { return m.AcceptedCount }
func (m *ReportProbesResponse) GetRejectedIds() []string         { return m.RejectedIds }
func (m *ReportProbesResponse) GetConfigUpdates() map[string]int64 { return m.ConfigUpdates }

// ForwardResponse 转发响应
type ForwardResponse struct {
	Success       bool   `json:"success,omitempty"`        // 是否成功
	Code          string `json:"code,omitempty"`           // 错误码
	Message       string `json:"message,omitempty"`        // 消息
	AcceptedCount int32  `json:"accepted_count,omitempty"` // 接受数量
	RejectedCount int32  `json:"rejected_count,omitempty"` // 拒绝数量
	SeqId         int64  `json:"seq_id,omitempty"`         // 序列号
}

func (m *ForwardResponse) Reset()         { *m = ForwardResponse{} }
func (m *ForwardResponse) String() string { return fmt.Sprintf("%+v", *m) }
func (m *ForwardResponse) ProtoMessage()  {}

func (m *ForwardResponse) Marshal() ([]byte, error)         { return json.Marshal(m) }
func (m *ForwardResponse) Unmarshal(data []byte) error      { return json.Unmarshal(data, m) }
func (m *ForwardResponse) GetSuccess() bool                 { return m.Success }
func (m *ForwardResponse) GetCode() string                  { return m.Code }
func (m *ForwardResponse) GetMessage() string               { return m.Message }
func (m *ForwardResponse) GetAcceptedCount() int32          { return m.AcceptedCount }
func (m *ForwardResponse) GetRejectedCount() int32          { return m.RejectedCount }
func (m *ForwardResponse) GetSeqId() int64                  { return m.SeqId }

// EdgeHeartbeatRequest 边缘节点心跳请求
type EdgeHeartbeatRequest struct {
	EdgeNodeId        string        `json:"edge_node_id,omitempty"`        // Edge节点ID
	EdgeAddress       string        `json:"edge_address,omitempty"`        // Edge地址
	CloudPlatform     string        `json:"cloud_platform,omitempty"`      // 云平台
	Region            string        `json:"region,omitempty"`              // 区域
	Zone              string        `json:"zone,omitempty"`                // 可用区
	Timestamp         int64         `json:"timestamp,omitempty"`           // 时间戳
	ProbeCount        int32         `json:"probe_count,omitempty"`         // 探针数量
	OnlineCount       int32         `json:"online_count,omitempty"`        // 在线探针数
	MetricsCount      int64         `json:"metrics_count,omitempty"`       // 累计指标数
	TracesCount       int64         `json:"traces_count,omitempty"`        // 累计追踪数
	Resources         *ResourceInfo `json:"resources,omitempty"`           // Edge资源使用
	ActiveConnections int64         `json:"active_connections,omitempty"`  // 活跃连接数
	QueueSize         int64         `json:"queue_size,omitempty"`          // 队列大小
	Version           string        `json:"version,omitempty"`             // Edge版本
}

func (m *EdgeHeartbeatRequest) Reset()         { *m = EdgeHeartbeatRequest{} }
func (m *EdgeHeartbeatRequest) String() string { return fmt.Sprintf("%+v", *m) }
func (m *EdgeHeartbeatRequest) ProtoMessage()  {}

func (m *EdgeHeartbeatRequest) Marshal() ([]byte, error)         { return json.Marshal(m) }
func (m *EdgeHeartbeatRequest) Unmarshal(data []byte) error      { return json.Unmarshal(data, m) }
func (m *EdgeHeartbeatRequest) GetEdgeNodeId() string            { return m.EdgeNodeId }
func (m *EdgeHeartbeatRequest) GetEdgeAddress() string           { return m.EdgeAddress }
func (m *EdgeHeartbeatRequest) GetCloudPlatform() string         { return m.CloudPlatform }
func (m *EdgeHeartbeatRequest) GetRegion() string                { return m.Region }
func (m *EdgeHeartbeatRequest) GetZone() string                  { return m.Zone }
func (m *EdgeHeartbeatRequest) GetTimestamp() int64              { return m.Timestamp }
func (m *EdgeHeartbeatRequest) GetProbeCount() int32             { return m.ProbeCount }
func (m *EdgeHeartbeatRequest) GetOnlineCount() int32            { return m.OnlineCount }
func (m *EdgeHeartbeatRequest) GetMetricsCount() int64           { return m.MetricsCount }
func (m *EdgeHeartbeatRequest) GetTracesCount() int64            { return m.TracesCount }
func (m *EdgeHeartbeatRequest) GetResources() *ResourceInfo      { return m.Resources }
func (m *EdgeHeartbeatRequest) GetActiveConnections() int64      { return m.ActiveConnections }
func (m *EdgeHeartbeatRequest) GetQueueSize() int64              { return m.QueueSize }
func (m *EdgeHeartbeatRequest) GetVersion() string               { return m.Version }

// EdgeHeartbeatResponse 边缘节点心跳响应
type EdgeHeartbeatResponse struct {
	Success       bool     `json:"success,omitempty"`        // 是否成功
	Code          string   `json:"code,omitempty"`           // 错误码
	Message       string   `json:"message,omitempty"`        // 消息
	ServerTime    int64    `json:"server_time,omitempty"`    // 服务器时间
	ConfigUpdated bool     `json:"config_updated,omitempty"` // 配置是否有更新
	Actions       []string `json:"actions,omitempty"`        // 建议动作
}

func (m *EdgeHeartbeatResponse) Reset()         { *m = EdgeHeartbeatResponse{} }
func (m *EdgeHeartbeatResponse) String() string { return fmt.Sprintf("%+v", *m) }
func (m *EdgeHeartbeatResponse) ProtoMessage()  {}

func (m *EdgeHeartbeatResponse) Marshal() ([]byte, error)         { return json.Marshal(m) }
func (m *EdgeHeartbeatResponse) Unmarshal(data []byte) error      { return json.Unmarshal(data, m) }
func (m *EdgeHeartbeatResponse) GetSuccess() bool                 { return m.Success }
func (m *EdgeHeartbeatResponse) GetCode() string                  { return m.Code }
func (m *EdgeHeartbeatResponse) GetMessage() string               { return m.Message }
func (m *EdgeHeartbeatResponse) GetServerTime() int64             { return m.ServerTime }
func (m *EdgeHeartbeatResponse) GetConfigUpdated() bool           { return m.ConfigUpdated }
func (m *EdgeHeartbeatResponse) GetActions() []string             { return m.Actions }

// ============================================================================
// 配置管理消息类型
// ============================================================================

// CollectionConfig 采集策略配置
type CollectionConfig struct {
	Version     int64  `json:"version,omitempty"`      // 配置版本号
	GroupId     string `json:"group_id,omitempty"`     // 所属组别
	UpdatedAt   int64  `json:"updated_at,omitempty"`   // 更新时间戳
	UpdatedBy   string `json:"updated_by,omitempty"`   // 更新者

	// 采样率配置 (0.0-1.0)
	SampleRate      float64 `json:"sample_rate,omitempty"`       // 全局采样率
	TCPSampleRate   float64 `json:"tcp_sample_rate,omitempty"`   // TCP指标采样率
	HTTPSampleRate  float64 `json:"http_sample_rate,omitempty"`  // HTTP指标采样率
	TraceSampleRate float64 `json:"trace_sample_rate,omitempty"` // 链路追踪采样率

	// 采集项开关
	EnableTCPMetrics    bool `json:"enable_tcp_metrics,omitempty"`     // TCP深度指标
	EnableHTTPMetrics   bool `json:"enable_http_metrics,omitempty"`    // HTTP应用层指标
	EnableHTTPFull      bool `json:"enable_http_full,omitempty"`       // HTTP全字段解析
	EnableDNSFull       bool `json:"enable_dns_full,omitempty"`        // DNS全字段解析
	EnableMySQLFull     bool `json:"enable_mysql_full,omitempty"`      // MySQL全字段解析
	EnableRedisFull     bool `json:"enable_redis_full,omitempty"`      // Redis全字段解析
	EnableSQLAggregator bool `json:"enable_sql_aggregator,omitempty"`  // SQL聚合分析
	EnableCPUProfiler   bool `json:"enable_cpu_profiler,omitempty"`    // ON-CPU剖析
	EnableMemoryProfile bool `json:"enable_memory_profile,omitempty"`  // 内存剖析
	EnableTrace         bool `json:"enable_trace,omitempty"`           // 链路追踪
	EnableLogCollection bool `json:"enable_log_collection,omitempty"`  // 日志采集

	// 资源限额
	MaxCPUCore    float64 `json:"max_cpu_core,omitempty"`    // 最大CPU核心数
	MaxMemoryMB   float64 `json:"max_memory_mb,omitempty"`   // 最大内存(MB)
	MaxGoroutines int     `json:"max_goroutines,omitempty"`  // 最大协程数
	MaxStorageMB  int64   `json:"max_storage_mb,omitempty"`  // 最大存储(MB)

	// 采集间隔和批处理
	CollectInterval int `json:"collect_interval,omitempty"` // 采集间隔(秒)
	BatchSize       int `json:"batch_size,omitempty"`       // 批处理大小
	BatchTimeout    int `json:"batch_timeout,omitempty"`    // 批处理超时(秒)
	FlushInterval   int `json:"flush_interval,omitempty"`   // 刷新间隔(秒)

	// 熔断配置
	CircuitBreakerEnabled   bool    `json:"circuit_breaker_enabled,omitempty"`   // 启用熔断
	CircuitBreakerThreshold float64 `json:"circuit_breaker_threshold,omitempty"` // 熔断阈值

	// 聚合配置
	AggregationEnabled bool `json:"aggregation_enabled,omitempty"` // 启用聚合
	AggregationWindow  int  `json:"aggregation_window,omitempty"`  // 聚合窗口(秒)

	// 过滤器配置
	IncludePatterns []string `json:"include_patterns,omitempty"` // 包含模式
	ExcludePatterns []string `json:"exclude_patterns,omitempty"` // 排除模式
	IncludePorts    []int32  `json:"include_ports,omitempty"`    // 包含端口
	ExcludePorts    []int32  `json:"exclude_ports,omitempty"`    // 排除端口

	// 扩展配置
	Extensions map[string]string `json:"extensions,omitempty"` // 扩展字段
}

func (m *CollectionConfig) Reset()         { *m = CollectionConfig{} }
func (m *CollectionConfig) String() string { return fmt.Sprintf("%+v", *m) }
func (m *CollectionConfig) ProtoMessage()  {}

func (m *CollectionConfig) Marshal() ([]byte, error)         { return json.Marshal(m) }
func (m *CollectionConfig) Unmarshal(data []byte) error      { return json.Unmarshal(data, m) }
func (m *CollectionConfig) GetVersion() int64                { return m.Version }
func (m *CollectionConfig) GetGroupId() string               { return m.GroupId }
func (m *CollectionConfig) GetUpdatedAt() int64              { return m.UpdatedAt }
func (m *CollectionConfig) GetUpdatedBy() string             { return m.UpdatedBy }
func (m *CollectionConfig) GetSampleRate() float64           { return m.SampleRate }
func (m *CollectionConfig) GetTCPSampleRate() float64        { return m.TCPSampleRate }
func (m *CollectionConfig) GetHTTPSampleRate() float64       { return m.HTTPSampleRate }
func (m *CollectionConfig) GetTraceSampleRate() float64      { return m.TraceSampleRate }
func (m *CollectionConfig) GetEnableTCPMetrics() bool        { return m.EnableTCPMetrics }
func (m *CollectionConfig) GetEnableHTTPMetrics() bool       { return m.EnableHTTPMetrics }
func (m *CollectionConfig) GetEnableHTTPFull() bool          { return m.EnableHTTPFull }
func (m *CollectionConfig) GetEnableDNSFull() bool           { return m.EnableDNSFull }
func (m *CollectionConfig) GetEnableMySQLFull() bool         { return m.EnableMySQLFull }
func (m *CollectionConfig) GetEnableRedisFull() bool         { return m.EnableRedisFull }
func (m *CollectionConfig) GetEnableSQLAggregator() bool     { return m.EnableSQLAggregator }
func (m *CollectionConfig) GetEnableCPUProfiler() bool       { return m.EnableCPUProfiler }
func (m *CollectionConfig) GetEnableMemoryProfile() bool     { return m.EnableMemoryProfile }
func (m *CollectionConfig) GetEnableTrace() bool             { return m.EnableTrace }
func (m *CollectionConfig) GetEnableLogCollection() bool     { return m.EnableLogCollection }
func (m *CollectionConfig) GetMaxCPUCore() float64           { return m.MaxCPUCore }
func (m *CollectionConfig) GetMaxMemoryMB() float64          { return m.MaxMemoryMB }
func (m *CollectionConfig) GetMaxGoroutines() int            { return m.MaxGoroutines }
func (m *CollectionConfig) GetMaxStorageMB() int64           { return m.MaxStorageMB }
func (m *CollectionConfig) GetCollectInterval() int          { return m.CollectInterval }
func (m *CollectionConfig) GetBatchSize() int                { return m.BatchSize }
func (m *CollectionConfig) GetBatchTimeout() int             { return m.BatchTimeout }
func (m *CollectionConfig) GetFlushInterval() int            { return m.FlushInterval }
func (m *CollectionConfig) GetCircuitBreakerEnabled() bool   { return m.CircuitBreakerEnabled }
func (m *CollectionConfig) GetCircuitBreakerThreshold() float64 { return m.CircuitBreakerThreshold }
func (m *CollectionConfig) GetAggregationEnabled() bool      { return m.AggregationEnabled }
func (m *CollectionConfig) GetAggregationWindow() int        { return m.AggregationWindow }
func (m *CollectionConfig) GetIncludePatterns() []string     { return m.IncludePatterns }
func (m *CollectionConfig) GetExcludePatterns() []string     { return m.ExcludePatterns }
func (m *CollectionConfig) GetIncludePorts() []int32         { return m.IncludePorts }
func (m *CollectionConfig) GetExcludePorts() []int32         { return m.ExcludePorts }
func (m *CollectionConfig) GetExtensions() map[string]string { return m.Extensions }

// GetConfigRequest 获取配置请求
type GetConfigRequest struct {
	ProbeId   string `json:"probe_id,omitempty"`   // 探针ID
	GroupId   string `json:"group_id,omitempty"`   // 所属组别
	Version   int64  `json:"version,omitempty"`    // 当前配置版本
	AssetType string `json:"asset_type,omitempty"` // 资产类型
}

func (m *GetConfigRequest) Reset()         { *m = GetConfigRequest{} }
func (m *GetConfigRequest) String() string { return fmt.Sprintf("%+v", *m) }
func (m *GetConfigRequest) ProtoMessage()  {}

func (m *GetConfigRequest) Marshal() ([]byte, error)         { return json.Marshal(m) }
func (m *GetConfigRequest) Unmarshal(data []byte) error      { return json.Unmarshal(data, m) }
func (m *GetConfigRequest) GetProbeId() string               { return m.ProbeId }
func (m *GetConfigRequest) GetGroupId() string               { return m.GroupId }
func (m *GetConfigRequest) GetVersion() int64                { return m.Version }
func (m *GetConfigRequest) GetAssetType() string             { return m.AssetType }

// GetConfigResponse 获取配置响应
type GetConfigResponse struct {
	Success    bool              `json:"success,omitempty"`     // 是否成功
	Code       string            `json:"code,omitempty"`        // 错误码
	Message    string            `json:"message,omitempty"`     // 消息
	Config     *CollectionConfig `json:"config,omitempty"`      // 配置内容
	HasUpdate  bool              `json:"has_update,omitempty"`  // 是否有更新
	ServerTime int64             `json:"server_time,omitempty"` // 服务器时间
}

func (m *GetConfigResponse) Reset()         { *m = GetConfigResponse{} }
func (m *GetConfigResponse) String() string { return fmt.Sprintf("%+v", *m) }
func (m *GetConfigResponse) ProtoMessage()  {}

func (m *GetConfigResponse) Marshal() ([]byte, error)         { return json.Marshal(m) }
func (m *GetConfigResponse) Unmarshal(data []byte) error      { return json.Unmarshal(data, m) }
func (m *GetConfigResponse) GetSuccess() bool                 { return m.Success }
func (m *GetConfigResponse) GetCode() string                  { return m.Code }
func (m *GetConfigResponse) GetMessage() string               { return m.Message }
func (m *GetConfigResponse) GetConfig() *CollectionConfig     { return m.Config }
func (m *GetConfigResponse) GetHasUpdate() bool               { return m.HasUpdate }
func (m *GetConfigResponse) GetServerTime() int64             { return m.ServerTime }

// ConfigUpdate 配置更新推送
type ConfigUpdate struct {
	Config      *CollectionConfig `json:"config,omitempty"`       // 新配置
	ForceUpdate bool              `json:"force_update,omitempty"` // 强制更新
	Reason      string            `json:"reason,omitempty"`       // 更新原因
	UpdatedAt   int64             `json:"updated_at,omitempty"`   // 更新时间
	UpdatedBy   string            `json:"updated_by,omitempty"`   // 更新者
}

func (m *ConfigUpdate) Reset()         { *m = ConfigUpdate{} }
func (m *ConfigUpdate) String() string { return fmt.Sprintf("%+v", *m) }
func (m *ConfigUpdate) ProtoMessage()  {}

func (m *ConfigUpdate) Marshal() ([]byte, error)         { return json.Marshal(m) }
func (m *ConfigUpdate) Unmarshal(data []byte) error      { return json.Unmarshal(data, m) }
func (m *ConfigUpdate) GetConfig() *CollectionConfig     { return m.Config }
func (m *ConfigUpdate) GetForceUpdate() bool             { return m.ForceUpdate }
func (m *ConfigUpdate) GetReason() string                { return m.Reason }
func (m *ConfigUpdate) GetUpdatedAt() int64              { return m.UpdatedAt }
func (m *ConfigUpdate) GetUpdatedBy() string             { return m.UpdatedBy }

// ConfigUpdateAck 配置更新确认
type ConfigUpdateAck struct {
	ProbeId   string `json:"probe_id,omitempty"`   // 探针ID
	Version   int64  `json:"version,omitempty"`    // 确认的版本
	Success   bool   `json:"success,omitempty"`    // 是否应用成功
	Message   string `json:"message,omitempty"`    // 消息
	AppliedAt int64  `json:"applied_at,omitempty"` // 应用时间
}

func (m *ConfigUpdateAck) Reset()         { *m = ConfigUpdateAck{} }
func (m *ConfigUpdateAck) String() string { return fmt.Sprintf("%+v", *m) }
func (m *ConfigUpdateAck) ProtoMessage()  {}

func (m *ConfigUpdateAck) Marshal() ([]byte, error)         { return json.Marshal(m) }
func (m *ConfigUpdateAck) Unmarshal(data []byte) error      { return json.Unmarshal(data, m) }
func (m *ConfigUpdateAck) GetProbeId() string               { return m.ProbeId }
func (m *ConfigUpdateAck) GetVersion() int64                { return m.Version }
func (m *ConfigUpdateAck) GetSuccess() bool                 { return m.Success }
func (m *ConfigUpdateAck) GetMessage() string               { return m.Message }
func (m *ConfigUpdateAck) GetAppliedAt() int64              { return m.AppliedAt }

// ============================================================================
// 服务发现消息类型
// ============================================================================

// EdgeInstance Edge节点实例
type EdgeInstance struct {
	Id              string            `json:"id,omitempty"`               // 实例ID
	Address         string            `json:"address,omitempty"`          // 地址
	Port            int32             `json:"port,omitempty"`             // 端口
	Weight          int32             `json:"weight,omitempty"`           // 权重
	Healthy         bool              `json:"healthy,omitempty"`          // 是否健康
	LastHeartbeat   int64             `json:"last_heartbeat,omitempty"`   // 最后心跳
	Tags            map[string]string `json:"tags,omitempty"`             // 标签
	ConnectionCount int64             `json:"connection_count,omitempty"` // 连接数
	Region          string            `json:"region,omitempty"`           // 区域
	Zone            string            `json:"zone,omitempty"`             // 可用区
	Version         string            `json:"version,omitempty"`          // 版本
}

func (m *EdgeInstance) Reset()         { *m = EdgeInstance{} }
func (m *EdgeInstance) String() string { return fmt.Sprintf("%+v", *m) }
func (m *EdgeInstance) ProtoMessage()  {}

func (m *EdgeInstance) Marshal() ([]byte, error)         { return json.Marshal(m) }
func (m *EdgeInstance) Unmarshal(data []byte) error      { return json.Unmarshal(data, m) }
func (m *EdgeInstance) GetId() string                    { return m.Id }
func (m *EdgeInstance) GetAddress() string               { return m.Address }
func (m *EdgeInstance) GetPort() int32                   { return m.Port }
func (m *EdgeInstance) GetWeight() int32                 { return m.Weight }
func (m *EdgeInstance) GetHealthy() bool                 { return m.Healthy }
func (m *EdgeInstance) GetLastHeartbeat() int64          { return m.LastHeartbeat }
func (m *EdgeInstance) GetTags() map[string]string       { return m.Tags }
func (m *EdgeInstance) GetConnectionCount() int64        { return m.ConnectionCount }
func (m *EdgeInstance) GetRegion() string                { return m.Region }
func (m *EdgeInstance) GetZone() string                  { return m.Zone }
func (m *EdgeInstance) GetVersion() string               { return m.Version }

// DiscoverEdgesRequest 发现Edge节点请求
type DiscoverEdgesRequest struct {
	ProbeId       string `json:"probe_id,omitempty"`       // 探针ID
	Region        string `json:"region,omitempty"`         // 期望区域
	Zone          string `json:"zone,omitempty"`           // 期望可用区
	CloudPlatform string `json:"cloud_platform,omitempty"` // 云平台
	Version       string `json:"version,omitempty"`        // 期望版本
}

func (m *DiscoverEdgesRequest) Reset()         { *m = DiscoverEdgesRequest{} }
func (m *DiscoverEdgesRequest) String() string { return fmt.Sprintf("%+v", *m) }
func (m *DiscoverEdgesRequest) ProtoMessage()  {}

func (m *DiscoverEdgesRequest) Marshal() ([]byte, error)         { return json.Marshal(m) }
func (m *DiscoverEdgesRequest) Unmarshal(data []byte) error      { return json.Unmarshal(data, m) }
func (m *DiscoverEdgesRequest) GetProbeId() string               { return m.ProbeId }
func (m *DiscoverEdgesRequest) GetRegion() string                { return m.Region }
func (m *DiscoverEdgesRequest) GetZone() string                  { return m.Zone }
func (m *DiscoverEdgesRequest) GetCloudPlatform() string         { return m.CloudPlatform }
func (m *DiscoverEdgesRequest) GetVersion() string               { return m.Version }

// DiscoverEdgesResponse 发现Edge节点响应
type DiscoverEdgesResponse struct {
	Success   bool            `json:"success,omitempty"`   // 是否成功
	Code      string          `json:"code,omitempty"`      // 错误码
	Message   string          `json:"message,omitempty"`   // 消息
	Instances []*EdgeInstance `json:"instances,omitempty"` // Edge实例列表
	Strategy  string          `json:"strategy,omitempty"`  // 负载均衡策略
}

func (m *DiscoverEdgesResponse) Reset()         { *m = DiscoverEdgesResponse{} }
func (m *DiscoverEdgesResponse) String() string { return fmt.Sprintf("%+v", *m) }
func (m *DiscoverEdgesResponse) ProtoMessage()  {}

func (m *DiscoverEdgesResponse) Marshal() ([]byte, error)         { return json.Marshal(m) }
func (m *DiscoverEdgesResponse) Unmarshal(data []byte) error      { return json.Unmarshal(data, m) }
func (m *DiscoverEdgesResponse) GetSuccess() bool                 { return m.Success }
func (m *DiscoverEdgesResponse) GetCode() string                  { return m.Code }
func (m *DiscoverEdgesResponse) GetMessage() string               { return m.Message }
func (m *DiscoverEdgesResponse) GetInstances() []*EdgeInstance    { return m.Instances }
func (m *DiscoverEdgesResponse) GetStrategy() string              { return m.Strategy }

// ============================================================================
// gRPC 服务接口定义
// ============================================================================

// ProbeServiceServer is the server API for ProbeService.
type ProbeServiceServer interface {
	RegisterProbe(context.Context, *RegisterProbeRequest) (*RegisterProbeResponse, error)
	Heartbeat(context.Context, *HeartbeatRequest) (*HeartbeatResponse, error)
	SendMetrics(context.Context, *MetricsBatch) (*SendResponse, error)
	SendTraces(context.Context, *TraceBatch) (*SendResponse, error)
	SendProfiling(context.Context, *ProfilingBatch) (*SendResponse, error)
	SendLogs(context.Context, *LogBatch) (*SendResponse, error)
	GetConfig(context.Context, *GetConfigRequest) (*GetConfigResponse, error)
	DiscoverEdges(context.Context, *DiscoverEdgesRequest) (*DiscoverEdgesResponse, error)
}

// UnimplementedProbeServiceServer can be embedded to have forward compatible implementations.
type UnimplementedProbeServiceServer struct{}

func (UnimplementedProbeServiceServer) RegisterProbe(context.Context, *RegisterProbeRequest) (*RegisterProbeResponse, error) {
	return nil, fmt.Errorf("not implemented")
}
func (UnimplementedProbeServiceServer) Heartbeat(context.Context, *HeartbeatRequest) (*HeartbeatResponse, error) {
	return nil, fmt.Errorf("not implemented")
}
func (UnimplementedProbeServiceServer) SendMetrics(context.Context, *MetricsBatch) (*SendResponse, error) {
	return nil, fmt.Errorf("not implemented")
}
func (UnimplementedProbeServiceServer) SendTraces(context.Context, *TraceBatch) (*SendResponse, error) {
	return nil, fmt.Errorf("not implemented")
}
func (UnimplementedProbeServiceServer) SendProfiling(context.Context, *ProfilingBatch) (*SendResponse, error) {
	return nil, fmt.Errorf("not implemented")
}
func (UnimplementedProbeServiceServer) SendLogs(context.Context, *LogBatch) (*SendResponse, error) {
	return nil, fmt.Errorf("not implemented")
}
func (UnimplementedProbeServiceServer) GetConfig(context.Context, *GetConfigRequest) (*GetConfigResponse, error) {
	return nil, fmt.Errorf("not implemented")
}
func (UnimplementedProbeServiceServer) DiscoverEdges(context.Context, *DiscoverEdgesRequest) (*DiscoverEdgesResponse, error) {
	return nil, fmt.Errorf("not implemented")
}

// UnsafeProbeServiceServer may be embedded to opt out of forward compatibility for this service.
type UnsafeProbeServiceServer interface {
	mustEmbedUnimplementedProbeServiceServer()
}

// ProbeServiceClient is the client API for ProbeService.
type ProbeServiceClient interface {
	RegisterProbe(ctx context.Context, in *RegisterProbeRequest, opts ...grpc.CallOption) (*RegisterProbeResponse, error)
	Heartbeat(ctx context.Context, in *HeartbeatRequest, opts ...grpc.CallOption) (*HeartbeatResponse, error)
	SendMetrics(ctx context.Context, in *MetricsBatch, opts ...grpc.CallOption) (*SendResponse, error)
	SendTraces(ctx context.Context, in *TraceBatch, opts ...grpc.CallOption) (*SendResponse, error)
	SendProfiling(ctx context.Context, in *ProfilingBatch, opts ...grpc.CallOption) (*SendResponse, error)
	SendLogs(ctx context.Context, in *LogBatch, opts ...grpc.CallOption) (*SendResponse, error)
	GetConfig(ctx context.Context, in *GetConfigRequest, opts ...grpc.CallOption) (*GetConfigResponse, error)
	DiscoverEdges(ctx context.Context, in *DiscoverEdgesRequest, opts ...grpc.CallOption) (*DiscoverEdgesResponse, error)
}

type probeServiceClient struct {
	cc *grpc.ClientConn
}

// NewProbeServiceClient creates a new ProbeServiceClient.
func NewProbeServiceClient(cc *grpc.ClientConn) ProbeServiceClient {
	return &probeServiceClient{cc}
}

func (c *probeServiceClient) RegisterProbe(ctx context.Context, in *RegisterProbeRequest, opts ...grpc.CallOption) (*RegisterProbeResponse, error) {
	out := new(RegisterProbeResponse)
	err := c.cc.Invoke(ctx, "/edge.ProbeService/RegisterProbe", in, out, opts...)
	if err != nil {
		return nil, err
	}
	return out, nil
}

func (c *probeServiceClient) Heartbeat(ctx context.Context, in *HeartbeatRequest, opts ...grpc.CallOption) (*HeartbeatResponse, error) {
	out := new(HeartbeatResponse)
	err := c.cc.Invoke(ctx, "/edge.ProbeService/Heartbeat", in, out, opts...)
	if err != nil {
		return nil, err
	}
	return out, nil
}

func (c *probeServiceClient) SendMetrics(ctx context.Context, in *MetricsBatch, opts ...grpc.CallOption) (*SendResponse, error) {
	out := new(SendResponse)
	err := c.cc.Invoke(ctx, "/edge.ProbeService/SendMetrics", in, out, opts...)
	if err != nil {
		return nil, err
	}
	return out, nil
}

func (c *probeServiceClient) SendTraces(ctx context.Context, in *TraceBatch, opts ...grpc.CallOption) (*SendResponse, error) {
	out := new(SendResponse)
	err := c.cc.Invoke(ctx, "/edge.ProbeService/SendTraces", in, out, opts...)
	if err != nil {
		return nil, err
	}
	return out, nil
}

func (c *probeServiceClient) SendProfiling(ctx context.Context, in *ProfilingBatch, opts ...grpc.CallOption) (*SendResponse, error) {
	out := new(SendResponse)
	err := c.cc.Invoke(ctx, "/edge.ProbeService/SendProfiling", in, out, opts...)
	if err != nil {
		return nil, err
	}
	return out, nil
}

func (c *probeServiceClient) SendLogs(ctx context.Context, in *LogBatch, opts ...grpc.CallOption) (*SendResponse, error) {
	out := new(SendResponse)
	err := c.cc.Invoke(ctx, "/edge.ProbeService/SendLogs", in, out, opts...)
	if err != nil {
		return nil, err
	}
	return out, nil
}

func (c *probeServiceClient) GetConfig(ctx context.Context, in *GetConfigRequest, opts ...grpc.CallOption) (*GetConfigResponse, error) {
	out := new(GetConfigResponse)
	err := c.cc.Invoke(ctx, "/edge.ProbeService/GetConfig", in, out, opts...)
	if err != nil {
		return nil, err
	}
	return out, nil
}

func (c *probeServiceClient) DiscoverEdges(ctx context.Context, in *DiscoverEdgesRequest, opts ...grpc.CallOption) (*DiscoverEdgesResponse, error) {
	out := new(DiscoverEdgesResponse)
	err := c.cc.Invoke(ctx, "/edge.ProbeService/DiscoverEdges", in, out, opts...)
	if err != nil {
		return nil, err
	}
	return out, nil
}

// RegisterProbeServiceServer registers the ProbeService server.
func RegisterProbeServiceServer(s *grpc.Server, srv ProbeServiceServer) {
	s.RegisterService(&_ProbeService_serviceDesc, srv)
}

var _ProbeService_serviceDesc = grpc.ServiceDesc{
	ServiceName: "edge.ProbeService",
	HandlerType: (*ProbeServiceServer)(nil),
	Methods: []grpc.MethodDesc{
		{MethodName: "RegisterProbe", Handler: _ProbeService_RegisterProbe_Handler},
		{MethodName: "Heartbeat", Handler: _ProbeService_Heartbeat_Handler},
		{MethodName: "SendMetrics", Handler: _ProbeService_SendMetrics_Handler},
		{MethodName: "SendTraces", Handler: _ProbeService_SendTraces_Handler},
		{MethodName: "SendProfiling", Handler: _ProbeService_SendProfiling_Handler},
		{MethodName: "SendLogs", Handler: _ProbeService_SendLogs_Handler},
		{MethodName: "GetConfig", Handler: _ProbeService_GetConfig_Handler},
		{MethodName: "DiscoverEdges", Handler: _ProbeService_DiscoverEdges_Handler},
	},
	Metadata: "edge.proto",
}

func _ProbeService_RegisterProbe_Handler(srv interface{}, ctx context.Context, dec func(interface{}) error, interceptor grpc.UnaryServerInterceptor) (interface{}, error) {
	in := new(RegisterProbeRequest)
	if err := dec(in); err != nil {
		return nil, err
	}
	if interceptor == nil {
		return srv.(ProbeServiceServer).RegisterProbe(ctx, in)
	}
	info := &grpc.UnaryServerInfo{
		Server:     srv,
		FullMethod: "/edge.ProbeService/RegisterProbe",
	}
	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		return srv.(ProbeServiceServer).RegisterProbe(ctx, req.(*RegisterProbeRequest))
	}
	return interceptor(ctx, in, info, handler)
}

func _ProbeService_Heartbeat_Handler(srv interface{}, ctx context.Context, dec func(interface{}) error, interceptor grpc.UnaryServerInterceptor) (interface{}, error) {
	in := new(HeartbeatRequest)
	if err := dec(in); err != nil {
		return nil, err
	}
	if interceptor == nil {
		return srv.(ProbeServiceServer).Heartbeat(ctx, in)
	}
	info := &grpc.UnaryServerInfo{
		Server:     srv,
		FullMethod: "/edge.ProbeService/Heartbeat",
	}
	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		return srv.(ProbeServiceServer).Heartbeat(ctx, req.(*HeartbeatRequest))
	}
	return interceptor(ctx, in, info, handler)
}

func _ProbeService_SendMetrics_Handler(srv interface{}, ctx context.Context, dec func(interface{}) error, interceptor grpc.UnaryServerInterceptor) (interface{}, error) {
	in := new(MetricsBatch)
	if err := dec(in); err != nil {
		return nil, err
	}
	if interceptor == nil {
		return srv.(ProbeServiceServer).SendMetrics(ctx, in)
	}
	info := &grpc.UnaryServerInfo{
		Server:     srv,
		FullMethod: "/edge.ProbeService/SendMetrics",
	}
	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		return srv.(ProbeServiceServer).SendMetrics(ctx, req.(*MetricsBatch))
	}
	return interceptor(ctx, in, info, handler)
}

func _ProbeService_SendTraces_Handler(srv interface{}, ctx context.Context, dec func(interface{}) error, interceptor grpc.UnaryServerInterceptor) (interface{}, error) {
	in := new(TraceBatch)
	if err := dec(in); err != nil {
		return nil, err
	}
	if interceptor == nil {
		return srv.(ProbeServiceServer).SendTraces(ctx, in)
	}
	info := &grpc.UnaryServerInfo{
		Server:     srv,
		FullMethod: "/edge.ProbeService/SendTraces",
	}
	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		return srv.(ProbeServiceServer).SendTraces(ctx, req.(*TraceBatch))
	}
	return interceptor(ctx, in, info, handler)
}

func _ProbeService_SendProfiling_Handler(srv interface{}, ctx context.Context, dec func(interface{}) error, interceptor grpc.UnaryServerInterceptor) (interface{}, error) {
	in := new(ProfilingBatch)
	if err := dec(in); err != nil {
		return nil, err
	}
	if interceptor == nil {
		return srv.(ProbeServiceServer).SendProfiling(ctx, in)
	}
	info := &grpc.UnaryServerInfo{
		Server:     srv,
		FullMethod: "/edge.ProbeService/SendProfiling",
	}
	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		return srv.(ProbeServiceServer).SendProfiling(ctx, req.(*ProfilingBatch))
	}
	return interceptor(ctx, in, info, handler)
}

func _ProbeService_SendLogs_Handler(srv interface{}, ctx context.Context, dec func(interface{}) error, interceptor grpc.UnaryServerInterceptor) (interface{}, error) {
	in := new(LogBatch)
	if err := dec(in); err != nil {
		return nil, err
	}
	if interceptor == nil {
		return srv.(ProbeServiceServer).SendLogs(ctx, in)
	}
	info := &grpc.UnaryServerInfo{
		Server:     srv,
		FullMethod: "/edge.ProbeService/SendLogs",
	}
	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		return srv.(ProbeServiceServer).SendLogs(ctx, req.(*LogBatch))
	}
	return interceptor(ctx, in, info, handler)
}

func _ProbeService_GetConfig_Handler(srv interface{}, ctx context.Context, dec func(interface{}) error, interceptor grpc.UnaryServerInterceptor) (interface{}, error) {
	in := new(GetConfigRequest)
	if err := dec(in); err != nil {
		return nil, err
	}
	if interceptor == nil {
		return srv.(ProbeServiceServer).GetConfig(ctx, in)
	}
	info := &grpc.UnaryServerInfo{
		Server:     srv,
		FullMethod: "/edge.ProbeService/GetConfig",
	}
	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		return srv.(ProbeServiceServer).GetConfig(ctx, req.(*GetConfigRequest))
	}
	return interceptor(ctx, in, info, handler)
}

func _ProbeService_DiscoverEdges_Handler(srv interface{}, ctx context.Context, dec func(interface{}) error, interceptor grpc.UnaryServerInterceptor) (interface{}, error) {
	in := new(DiscoverEdgesRequest)
	if err := dec(in); err != nil {
		return nil, err
	}
	if interceptor == nil {
		return srv.(ProbeServiceServer).DiscoverEdges(ctx, in)
	}
	info := &grpc.UnaryServerInfo{
		Server:     srv,
		FullMethod: "/edge.ProbeService/DiscoverEdges",
	}
	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		return srv.(ProbeServiceServer).DiscoverEdges(ctx, req.(*DiscoverEdgesRequest))
	}
	return interceptor(ctx, in, info, handler)
}

// ============================================================================
// CenterService — Edge -> Center
// ============================================================================

// CenterServiceServer is the server API for CenterService.
type CenterServiceServer interface {
	ReportProbes(context.Context, *ReportProbesRequest) (*ReportProbesResponse, error)
	ForwardMetrics(context.Context, *MetricsBatch) (*ForwardResponse, error)
	ForwardTraces(context.Context, *TraceBatch) (*ForwardResponse, error)
	ForwardProfiling(context.Context, *ProfilingBatch) (*ForwardResponse, error)
	ForwardLogs(context.Context, *LogBatch) (*ForwardResponse, error)
	Heartbeat(context.Context, *EdgeHeartbeatRequest) (*EdgeHeartbeatResponse, error)
}

// UnimplementedCenterServiceServer can be embedded to have forward compatible implementations.
type UnimplementedCenterServiceServer struct{}

func (UnimplementedCenterServiceServer) ReportProbes(context.Context, *ReportProbesRequest) (*ReportProbesResponse, error) {
	return nil, fmt.Errorf("not implemented")
}
func (UnimplementedCenterServiceServer) ForwardMetrics(context.Context, *MetricsBatch) (*ForwardResponse, error) {
	return nil, fmt.Errorf("not implemented")
}
func (UnimplementedCenterServiceServer) ForwardTraces(context.Context, *TraceBatch) (*ForwardResponse, error) {
	return nil, fmt.Errorf("not implemented")
}
func (UnimplementedCenterServiceServer) ForwardProfiling(context.Context, *ProfilingBatch) (*ForwardResponse, error) {
	return nil, fmt.Errorf("not implemented")
}
func (UnimplementedCenterServiceServer) ForwardLogs(context.Context, *LogBatch) (*ForwardResponse, error) {
	return nil, fmt.Errorf("not implemented")
}
func (UnimplementedCenterServiceServer) Heartbeat(context.Context, *EdgeHeartbeatRequest) (*EdgeHeartbeatResponse, error) {
	return nil, fmt.Errorf("not implemented")
}

// UnsafeCenterServiceServer may be embedded to opt out of forward compatibility for this service.
type UnsafeCenterServiceServer interface {
	mustEmbedUnimplementedCenterServiceServer()
}

// CenterServiceClient is the client API for CenterService.
type CenterServiceClient interface {
	ReportProbes(ctx context.Context, in *ReportProbesRequest, opts ...grpc.CallOption) (*ReportProbesResponse, error)
	ForwardMetrics(ctx context.Context, in *MetricsBatch, opts ...grpc.CallOption) (*ForwardResponse, error)
	ForwardTraces(ctx context.Context, in *TraceBatch, opts ...grpc.CallOption) (*ForwardResponse, error)
	ForwardProfiling(ctx context.Context, in *ProfilingBatch, opts ...grpc.CallOption) (*ForwardResponse, error)
	ForwardLogs(ctx context.Context, in *LogBatch, opts ...grpc.CallOption) (*ForwardResponse, error)
	Heartbeat(ctx context.Context, in *EdgeHeartbeatRequest, opts ...grpc.CallOption) (*EdgeHeartbeatResponse, error)
}

type centerServiceClient struct {
	cc *grpc.ClientConn
}

// NewCenterServiceClient creates a new CenterServiceClient.
func NewCenterServiceClient(cc *grpc.ClientConn) CenterServiceClient {
	return &centerServiceClient{cc}
}

func (c *centerServiceClient) ReportProbes(ctx context.Context, in *ReportProbesRequest, opts ...grpc.CallOption) (*ReportProbesResponse, error) {
	out := new(ReportProbesResponse)
	err := c.cc.Invoke(ctx, "/edge.CenterService/ReportProbes", in, out, opts...)
	if err != nil {
		return nil, err
	}
	return out, nil
}

func (c *centerServiceClient) ForwardMetrics(ctx context.Context, in *MetricsBatch, opts ...grpc.CallOption) (*ForwardResponse, error) {
	out := new(ForwardResponse)
	err := c.cc.Invoke(ctx, "/edge.CenterService/ForwardMetrics", in, out, opts...)
	if err != nil {
		return nil, err
	}
	return out, nil
}

func (c *centerServiceClient) ForwardTraces(ctx context.Context, in *TraceBatch, opts ...grpc.CallOption) (*ForwardResponse, error) {
	out := new(ForwardResponse)
	err := c.cc.Invoke(ctx, "/edge.CenterService/ForwardTraces", in, out, opts...)
	if err != nil {
		return nil, err
	}
	return out, nil
}

func (c *centerServiceClient) ForwardProfiling(ctx context.Context, in *ProfilingBatch, opts ...grpc.CallOption) (*ForwardResponse, error) {
	out := new(ForwardResponse)
	err := c.cc.Invoke(ctx, "/edge.CenterService/ForwardProfiling", in, out, opts...)
	if err != nil {
		return nil, err
	}
	return out, nil
}

func (c *centerServiceClient) ForwardLogs(ctx context.Context, in *LogBatch, opts ...grpc.CallOption) (*ForwardResponse, error) {
	out := new(ForwardResponse)
	err := c.cc.Invoke(ctx, "/edge.CenterService/ForwardLogs", in, out, opts...)
	if err != nil {
		return nil, err
	}
	return out, nil
}

func (c *centerServiceClient) Heartbeat(ctx context.Context, in *EdgeHeartbeatRequest, opts ...grpc.CallOption) (*EdgeHeartbeatResponse, error) {
	out := new(EdgeHeartbeatResponse)
	err := c.cc.Invoke(ctx, "/edge.CenterService/Heartbeat", in, out, opts...)
	if err != nil {
		return nil, err
	}
	return out, nil
}

// RegisterCenterServiceServer registers the CenterService server.
func RegisterCenterServiceServer(s *grpc.Server, srv CenterServiceServer) {
	s.RegisterService(&_CenterService_serviceDesc, srv)
}

var _CenterService_serviceDesc = grpc.ServiceDesc{
	ServiceName: "edge.CenterService",
	HandlerType: (*CenterServiceServer)(nil),
	Methods: []grpc.MethodDesc{
		{MethodName: "ReportProbes", Handler: _CenterService_ReportProbes_Handler},
		{MethodName: "ForwardMetrics", Handler: _CenterService_ForwardMetrics_Handler},
		{MethodName: "ForwardTraces", Handler: _CenterService_ForwardTraces_Handler},
		{MethodName: "ForwardProfiling", Handler: _CenterService_ForwardProfiling_Handler},
		{MethodName: "ForwardLogs", Handler: _CenterService_ForwardLogs_Handler},
		{MethodName: "Heartbeat", Handler: _CenterService_Heartbeat_Handler},
	},
	Metadata: "edge.proto",
}

func _CenterService_ReportProbes_Handler(srv interface{}, ctx context.Context, dec func(interface{}) error, interceptor grpc.UnaryServerInterceptor) (interface{}, error) {
	in := new(ReportProbesRequest)
	if err := dec(in); err != nil {
		return nil, err
	}
	if interceptor == nil {
		return srv.(CenterServiceServer).ReportProbes(ctx, in)
	}
	info := &grpc.UnaryServerInfo{
		Server:     srv,
		FullMethod: "/edge.CenterService/ReportProbes",
	}
	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		return srv.(CenterServiceServer).ReportProbes(ctx, req.(*ReportProbesRequest))
	}
	return interceptor(ctx, in, info, handler)
}

func _CenterService_ForwardMetrics_Handler(srv interface{}, ctx context.Context, dec func(interface{}) error, interceptor grpc.UnaryServerInterceptor) (interface{}, error) {
	in := new(MetricsBatch)
	if err := dec(in); err != nil {
		return nil, err
	}
	if interceptor == nil {
		return srv.(CenterServiceServer).ForwardMetrics(ctx, in)
	}
	info := &grpc.UnaryServerInfo{
		Server:     srv,
		FullMethod: "/edge.CenterService/ForwardMetrics",
	}
	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		return srv.(CenterServiceServer).ForwardMetrics(ctx, req.(*MetricsBatch))
	}
	return interceptor(ctx, in, info, handler)
}

func _CenterService_ForwardTraces_Handler(srv interface{}, ctx context.Context, dec func(interface{}) error, interceptor grpc.UnaryServerInterceptor) (interface{}, error) {
	in := new(TraceBatch)
	if err := dec(in); err != nil {
		return nil, err
	}
	if interceptor == nil {
		return srv.(CenterServiceServer).ForwardTraces(ctx, in)
	}
	info := &grpc.UnaryServerInfo{
		Server:     srv,
		FullMethod: "/edge.CenterService/ForwardTraces",
	}
	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		return srv.(CenterServiceServer).ForwardTraces(ctx, req.(*TraceBatch))
	}
	return interceptor(ctx, in, info, handler)
}

func _CenterService_ForwardProfiling_Handler(srv interface{}, ctx context.Context, dec func(interface{}) error, interceptor grpc.UnaryServerInterceptor) (interface{}, error) {
	in := new(ProfilingBatch)
	if err := dec(in); err != nil {
		return nil, err
	}
	if interceptor == nil {
		return srv.(CenterServiceServer).ForwardProfiling(ctx, in)
	}
	info := &grpc.UnaryServerInfo{
		Server:     srv,
		FullMethod: "/edge.CenterService/ForwardProfiling",
	}
	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		return srv.(CenterServiceServer).ForwardProfiling(ctx, req.(*ProfilingBatch))
	}
	return interceptor(ctx, in, info, handler)
}

func _CenterService_ForwardLogs_Handler(srv interface{}, ctx context.Context, dec func(interface{}) error, interceptor grpc.UnaryServerInterceptor) (interface{}, error) {
	in := new(LogBatch)
	if err := dec(in); err != nil {
		return nil, err
	}
	if interceptor == nil {
		return srv.(CenterServiceServer).ForwardLogs(ctx, in)
	}
	info := &grpc.UnaryServerInfo{
		Server:     srv,
		FullMethod: "/edge.CenterService/ForwardLogs",
	}
	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		return srv.(CenterServiceServer).ForwardLogs(ctx, req.(*LogBatch))
	}
	return interceptor(ctx, in, info, handler)
}

func _CenterService_Heartbeat_Handler(srv interface{}, ctx context.Context, dec func(interface{}) error, interceptor grpc.UnaryServerInterceptor) (interface{}, error) {
	in := new(EdgeHeartbeatRequest)
	if err := dec(in); err != nil {
		return nil, err
	}
	if interceptor == nil {
		return srv.(CenterServiceServer).Heartbeat(ctx, in)
	}
	info := &grpc.UnaryServerInfo{
		Server:     srv,
		FullMethod: "/edge.CenterService/Heartbeat",
	}
	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		return srv.(CenterServiceServer).Heartbeat(ctx, req.(*EdgeHeartbeatRequest))
	}
	return interceptor(ctx, in, info, handler)
}

// ============================================================================
// 编译时检查：所有消息类型实现 proto.Message 接口
// ============================================================================

var (
	// 通用类型
	_ proto.Message = (*CommonResponse)(nil)
	_ proto.Message = (*Pagination)(nil)
	_ proto.Message = (*PageInfo)(nil)
	_ proto.Message = (*ResourceInfo)(nil)
	_ proto.Message = (*NetworkInfo)(nil)
	_ proto.Message = (*InterfaceInfo)(nil)
	_ proto.Message = (*RouteInfo)(nil)

	// ProbeService 类型
	_ proto.Message = (*RegisterProbeRequest)(nil)
	_ proto.Message = (*RegisterProbeResponse)(nil)
	_ proto.Message = (*HeartbeatRequest)(nil)
	_ proto.Message = (*HeartbeatResponse)(nil)
	_ proto.Message = (*RemoteCommand)(nil)
	_ proto.Message = (*SendResponse)(nil)

	// 数据上报类型
	_ proto.Message = (*MetricData)(nil)
	_ proto.Message = (*MetricsBatch)(nil)
	_ proto.Message = (*TraceSpanData)(nil)
	_ proto.Message = (*SpanEvent)(nil)
	_ proto.Message = (*SpanLink)(nil)
	_ proto.Message = (*TraceBatch)(nil)
	_ proto.Message = (*ProfilingSample)(nil)
	_ proto.Message = (*ProfilingLocation)(nil)
	_ proto.Message = (*ProfilingData)(nil)
	_ proto.Message = (*ProfilingBatch)(nil)
	_ proto.Message = (*LogData)(nil)
	_ proto.Message = (*LogBatch)(nil)

	// 探针信息类型
	_ proto.Message = (*ProbeInfo)(nil)

	// CenterService 类型
	_ proto.Message = (*ReportProbesRequest)(nil)
	_ proto.Message = (*ReportProbesResponse)(nil)
	_ proto.Message = (*ForwardResponse)(nil)
	_ proto.Message = (*EdgeHeartbeatRequest)(nil)
	_ proto.Message = (*EdgeHeartbeatResponse)(nil)

	// 配置管理类型
	_ proto.Message = (*CollectionConfig)(nil)
	_ proto.Message = (*GetConfigRequest)(nil)
	_ proto.Message = (*GetConfigResponse)(nil)
	_ proto.Message = (*ConfigUpdate)(nil)
	_ proto.Message = (*ConfigUpdateAck)(nil)

	// 服务发现类型
	_ proto.Message = (*EdgeInstance)(nil)
	_ proto.Message = (*DiscoverEdgesRequest)(nil)
	_ proto.Message = (*DiscoverEdgesResponse)(nil)
)
