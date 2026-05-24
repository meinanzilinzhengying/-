// Package edge 包含所有手写 proto 结构体和 gRPC 服务定义
//
// 设计选择说明：
// 本项目当前使用手写 Go 结构体替代标准 protobuf 工具链（protoc + protoc-gen-go），
// 主要原因如下：
//   - 项目初期为了快速迭代，避免了 protobuf 编译工具链的额外复杂度
//   - 使用 JSON 编解码替代 protobuf 二进制格式，便于调试和日志查看
//   - 所有消息结构体实现了 github.com/golang/protobuf/proto.Message 接口，
//     以兼容项目使用的 legacy codec（见 cloud-flow-edge/proto/compat.go）
//
// 未来迁移计划：
//   - 随着服务间通信协议趋于稳定，计划迁移到标准 protobuf 定义（.proto 文件）
//   - 迁移后将使用 protoc 自动生成 Go 代码，获得更好的跨语言兼容性和性能
//   - 迁移路径：先定义 .proto 文件 -> 生成代码 -> 逐步替换手写结构体 -> 移除 legacy codec
package edge

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/golang/protobuf/proto"
	"google.golang.org/grpc"
)

// ============================================================================
// 消息类型定义
// ============================================================================

// RegisterProbeRequest 探针注册请求
type RegisterProbeRequest struct {
	ProbeId  string `json:"probe_id,omitempty"`
	HostIp   string `json:"host_ip,omitempty"`
	Hostname string `json:"hostname,omitempty"`
	Version  string `json:"version,omitempty"`
}

func (m *RegisterProbeRequest) Reset()         { *m = RegisterProbeRequest{} }
func (m *RegisterProbeRequest) String() string  { return fmt.Sprintf("%+v", *m) }
func (m *RegisterProbeRequest) ProtoMessage()   {}

func (m *RegisterProbeRequest) Marshal() ([]byte, error) {
	return json.Marshal(m)
}

func (m *RegisterProbeRequest) Unmarshal(data []byte) error {
	return json.Unmarshal(data, m)
}

func (m *RegisterProbeRequest) GetProbeId() string  { return m.ProbeId }
func (m *RegisterProbeRequest) GetHostIp() string   { return m.HostIp }
func (m *RegisterProbeRequest) GetHostname() string { return m.Hostname }
func (m *RegisterProbeRequest) GetVersion() string  { return m.Version }

// RegisterProbeResponse 探针注册响应
type RegisterProbeResponse struct {
	Success            bool   `json:"success,omitempty"`
	Message            string `json:"message,omitempty"`
	HeartbeatInterval  int32  `json:"heartbeat_interval,omitempty"`
}

func (m *RegisterProbeResponse) Reset()         { *m = RegisterProbeResponse{} }
func (m *RegisterProbeResponse) String() string  { return fmt.Sprintf("%+v", *m) }
func (m *RegisterProbeResponse) ProtoMessage()   {}

func (m *RegisterProbeResponse) Marshal() ([]byte, error) {
	return json.Marshal(m)
}

func (m *RegisterProbeResponse) Unmarshal(data []byte) error {
	return json.Unmarshal(data, m)
}

func (m *RegisterProbeResponse) GetSuccess() bool            { return m.Success }
func (m *RegisterProbeResponse) GetMessage() string          { return m.Message }
func (m *RegisterProbeResponse) GetHeartbeatInterval() int32 { return m.HeartbeatInterval }

// HeartbeatRequest 心跳请求
type HeartbeatRequest struct {
	ProbeId   string `json:"probe_id,omitempty"`
	Timestamp int64  `json:"timestamp,omitempty"`
}

func (m *HeartbeatRequest) Reset()         { *m = HeartbeatRequest{} }
func (m *HeartbeatRequest) String() string  { return fmt.Sprintf("%+v", *m) }
func (m *HeartbeatRequest) ProtoMessage()   {}

func (m *HeartbeatRequest) Marshal() ([]byte, error) {
	return json.Marshal(m)
}

func (m *HeartbeatRequest) Unmarshal(data []byte) error {
	return json.Unmarshal(data, m)
}

func (m *HeartbeatRequest) GetProbeId() string  { return m.ProbeId }
func (m *HeartbeatRequest) GetTimestamp() int64 { return m.Timestamp }

// HeartbeatResponse 心跳响应
type HeartbeatResponse struct {
	Success bool `json:"success,omitempty"`
}

func (m *HeartbeatResponse) Reset()         { *m = HeartbeatResponse{} }
func (m *HeartbeatResponse) String() string  { return fmt.Sprintf("%+v", *m) }
func (m *HeartbeatResponse) ProtoMessage()   {}

func (m *HeartbeatResponse) Marshal() ([]byte, error) {
	return json.Marshal(m)
}

func (m *HeartbeatResponse) Unmarshal(data []byte) error {
	return json.Unmarshal(data, m)
}

func (m *HeartbeatResponse) GetSuccess() bool { return m.Success }

// SendResponse 通用发送响应
type SendResponse struct {
	Success bool  `json:"success,omitempty"`
	Accepted int32 `json:"accepted,omitempty"`
}

func (m *SendResponse) Reset()         { *m = SendResponse{} }
func (m *SendResponse) String() string  { return fmt.Sprintf("%+v", *m) }
func (m *SendResponse) ProtoMessage()   {}

func (m *SendResponse) Marshal() ([]byte, error) {
	return json.Marshal(m)
}

func (m *SendResponse) Unmarshal(data []byte) error {
	return json.Unmarshal(data, m)
}

func (m *SendResponse) GetSuccess() bool  { return m.Success }
func (m *SendResponse) GetAccepted() int32 { return m.Accepted }

// MetricData 指标数据
type MetricData struct {
	ProbeId     string            `json:"probe_id,omitempty"`
	Timestamp   int64             `json:"timestamp,omitempty"`
	SrcIp       string            `json:"src_ip,omitempty"`
	DstIp       string            `json:"dst_ip,omitempty"`
	SrcPort     int32             `json:"src_port,omitempty"`
	DstPort     int32             `json:"dst_port,omitempty"`
	Protocol    string            `json:"protocol,omitempty"`
	Bytes       int64             `json:"bytes,omitempty"`
	Packets     int64             `json:"packets,omitempty"`
	Latency     int64             `json:"latency,omitempty"`
	CpuUsage    float64           `json:"cpu_usage,omitempty"`
	MemoryUsage float64           `json:"memory_usage,omitempty"`
	DiskUsage   float64           `json:"disk_usage,omitempty"`
	Tags        map[string]string `json:"tags,omitempty"`
}

func (m *MetricData) Reset()         { *m = MetricData{} }
func (m *MetricData) String() string  { return fmt.Sprintf("%+v", *m) }
func (m *MetricData) ProtoMessage()   {}

func (m *MetricData) Marshal() ([]byte, error) {
	return json.Marshal(m)
}

func (m *MetricData) Unmarshal(data []byte) error {
	return json.Unmarshal(data, m)
}

func (m *MetricData) GetProbeId() string            { return m.ProbeId }
func (m *MetricData) GetTimestamp() int64           { return m.Timestamp }
func (m *MetricData) GetSrcIp() string              { return m.SrcIp }
func (m *MetricData) GetDstIp() string              { return m.DstIp }
func (m *MetricData) GetSrcPort() int32             { return m.SrcPort }
func (m *MetricData) GetDstPort() int32             { return m.DstPort }
func (m *MetricData) GetProtocol() string           { return m.Protocol }
func (m *MetricData) GetBytes() int64               { return m.Bytes }
func (m *MetricData) GetPackets() int64             { return m.Packets }
func (m *MetricData) GetLatency() int64             { return m.Latency }
func (m *MetricData) GetCpuUsage() float64          { return m.CpuUsage }
func (m *MetricData) GetMemoryUsage() float64       { return m.MemoryUsage }
func (m *MetricData) GetDiskUsage() float64         { return m.DiskUsage }
func (m *MetricData) GetTags() map[string]string    { return m.Tags }

// MetricsBatch 指标批量数据
type MetricsBatch struct {
	ProbeId string        `json:"probe_id,omitempty"`
	Metrics []*MetricData `json:"metrics,omitempty"`
}

func (m *MetricsBatch) Reset()         { *m = MetricsBatch{} }
func (m *MetricsBatch) String() string  { return fmt.Sprintf("%+v", *m) }
func (m *MetricsBatch) ProtoMessage()   {}

func (m *MetricsBatch) Marshal() ([]byte, error) {
	return json.Marshal(m)
}

func (m *MetricsBatch) Unmarshal(data []byte) error {
	return json.Unmarshal(data, m)
}

func (m *MetricsBatch) GetProbeId() string        { return m.ProbeId }
func (m *MetricsBatch) GetMetrics() []*MetricData { return m.Metrics }

// TraceSpanData 链路追踪 Span 数据
type TraceSpanData struct {
	ProbeId   string            `json:"probe_id,omitempty"`
	TraceId   string            `json:"trace_id,omitempty"`
	SpanId    string            `json:"span_id,omitempty"`
	ParentId  string            `json:"parent_id,omitempty"`
	Service   string            `json:"service,omitempty"`
	Operation string            `json:"operation,omitempty"`
	StartTime int64             `json:"start_time,omitempty"`
	EndTime   int64             `json:"end_time,omitempty"`
	Duration  int64             `json:"duration,omitempty"`
	Status    string            `json:"status,omitempty"`
	Tags      map[string]string `json:"tags,omitempty"`
}

func (m *TraceSpanData) Reset()         { *m = TraceSpanData{} }
func (m *TraceSpanData) String() string  { return fmt.Sprintf("%+v", *m) }
func (m *TraceSpanData) ProtoMessage()   {}

func (m *TraceSpanData) Marshal() ([]byte, error) {
	return json.Marshal(m)
}

func (m *TraceSpanData) Unmarshal(data []byte) error {
	return json.Unmarshal(data, m)
}

func (m *TraceSpanData) GetProbeId() string            { return m.ProbeId }
func (m *TraceSpanData) GetTraceId() string            { return m.TraceId }
func (m *TraceSpanData) GetSpanId() string             { return m.SpanId }
func (m *TraceSpanData) GetParentId() string           { return m.ParentId }
func (m *TraceSpanData) GetService() string            { return m.Service }
func (m *TraceSpanData) GetOperation() string          { return m.Operation }
func (m *TraceSpanData) GetStartTime() int64           { return m.StartTime }
func (m *TraceSpanData) GetEndTime() int64             { return m.EndTime }
func (m *TraceSpanData) GetDuration() int64            { return m.Duration }
func (m *TraceSpanData) GetStatus() string             { return m.Status }
func (m *TraceSpanData) GetTags() map[string]string    { return m.Tags }

// TraceBatch 链路追踪批量数据
type TraceBatch struct {
	ProbeId string           `json:"probe_id,omitempty"`
	Spans   []*TraceSpanData `json:"spans,omitempty"`
}

func (m *TraceBatch) Reset()         { *m = TraceBatch{} }
func (m *TraceBatch) String() string  { return fmt.Sprintf("%+v", *m) }
func (m *TraceBatch) ProtoMessage()   {}

func (m *TraceBatch) Marshal() ([]byte, error) {
	return json.Marshal(m)
}

func (m *TraceBatch) Unmarshal(data []byte) error {
	return json.Unmarshal(data, m)
}

func (m *TraceBatch) GetProbeId() string             { return m.ProbeId }
func (m *TraceBatch) GetSpans() []*TraceSpanData     { return m.Spans }

// ProfilingData 性能分析数据
type ProfilingData struct {
	ProbeId   string            `json:"probe_id,omitempty"`
	Type      string            `json:"type,omitempty"`
	Stack     string            `json:"stack,omitempty"`
	Count     int64             `json:"count,omitempty"`
	TotalTime int64             `json:"total_time,omitempty"`
	Labels    map[string]string `json:"labels,omitempty"`
}

func (m *ProfilingData) Reset()         { *m = ProfilingData{} }
func (m *ProfilingData) String() string  { return fmt.Sprintf("%+v", *m) }
func (m *ProfilingData) ProtoMessage()   {}

func (m *ProfilingData) Marshal() ([]byte, error) {
	return json.Marshal(m)
}

func (m *ProfilingData) Unmarshal(data []byte) error {
	return json.Unmarshal(data, m)
}

func (m *ProfilingData) GetProbeId() string            { return m.ProbeId }
func (m *ProfilingData) GetType() string               { return m.Type }
func (m *ProfilingData) GetStack() string              { return m.Stack }
func (m *ProfilingData) GetCount() int64               { return m.Count }
func (m *ProfilingData) GetTotalTime() int64           { return m.TotalTime }
func (m *ProfilingData) GetLabels() map[string]string  { return m.Labels }

// ProfilingBatch 性能分析批量数据
type ProfilingBatch struct {
	ProbeId  string           `json:"probe_id,omitempty"`
	Profiles []*ProfilingData `json:"profiles,omitempty"`
}

func (m *ProfilingBatch) Reset()         { *m = ProfilingBatch{} }
func (m *ProfilingBatch) String() string  { return fmt.Sprintf("%+v", *m) }
func (m *ProfilingBatch) ProtoMessage()   {}

func (m *ProfilingBatch) Marshal() ([]byte, error) {
	return json.Marshal(m)
}

func (m *ProfilingBatch) Unmarshal(data []byte) error {
	return json.Unmarshal(data, m)
}

func (m *ProfilingBatch) GetProbeId() string              { return m.ProbeId }
func (m *ProfilingBatch) GetProfiles() []*ProfilingData   { return m.Profiles }

// ProbeInfo 探针信息
type ProbeInfo struct {
	ProbeId       string `json:"probe_id,omitempty"`
	HostIp        string `json:"host_ip,omitempty"`
	Hostname      string `json:"hostname,omitempty"`
	Status        string `json:"status,omitempty"`
	Version       string `json:"version,omitempty"`
	LastHeartbeat int64  `json:"last_heartbeat,omitempty"`
}

func (m *ProbeInfo) Reset()         { *m = ProbeInfo{} }
func (m *ProbeInfo) String() string  { return fmt.Sprintf("%+v", *m) }
func (m *ProbeInfo) ProtoMessage()   {}

func (m *ProbeInfo) Marshal() ([]byte, error) {
	return json.Marshal(m)
}

func (m *ProbeInfo) Unmarshal(data []byte) error {
	return json.Unmarshal(data, m)
}

func (m *ProbeInfo) GetProbeId() string              { return m.ProbeId }
func (m *ProbeInfo) GetHostIp() string               { return m.HostIp }
func (m *ProbeInfo) GetHostname() string             { return m.Hostname }
func (m *ProbeInfo) GetStatus() string               { return m.Status }
func (m *ProbeInfo) GetVersion() string              { return m.Version }
func (m *ProbeInfo) GetLastHeartbeat() int64         { return m.LastHeartbeat }

// ReportProbesRequest 上报探针列表请求
type ReportProbesRequest struct {
	EdgeNodeId    string      `json:"edge_node_id,omitempty"`
	CloudPlatform string      `json:"cloud_platform,omitempty"`
	Region        string      `json:"region,omitempty"`
	Probes        []*ProbeInfo `json:"probes,omitempty"`
}

func (m *ReportProbesRequest) Reset()         { *m = ReportProbesRequest{} }
func (m *ReportProbesRequest) String() string  { return fmt.Sprintf("%+v", *m) }
func (m *ReportProbesRequest) ProtoMessage()   {}

func (m *ReportProbesRequest) Marshal() ([]byte, error) {
	return json.Marshal(m)
}

func (m *ReportProbesRequest) Unmarshal(data []byte) error {
	return json.Unmarshal(data, m)
}

func (m *ReportProbesRequest) GetEdgeNodeId() string       { return m.EdgeNodeId }
func (m *ReportProbesRequest) GetCloudPlatform() string    { return m.CloudPlatform }
func (m *ReportProbesRequest) GetRegion() string           { return m.Region }
func (m *ReportProbesRequest) GetProbes() []*ProbeInfo     { return m.Probes }

// ReportProbesResponse 上报探针列表响应
type ReportProbesResponse struct {
	Success bool `json:"success,omitempty"`
}

func (m *ReportProbesResponse) Reset()         { *m = ReportProbesResponse{} }
func (m *ReportProbesResponse) String() string  { return fmt.Sprintf("%+v", *m) }
func (m *ReportProbesResponse) ProtoMessage()   {}

func (m *ReportProbesResponse) Marshal() ([]byte, error) {
	return json.Marshal(m)
}

func (m *ReportProbesResponse) Unmarshal(data []byte) error {
	return json.Unmarshal(data, m)
}

func (m *ReportProbesResponse) GetSuccess() bool { return m.Success }

// ForwardResponse 转发响应
type ForwardResponse struct {
	Success bool `json:"success,omitempty"`
}

func (m *ForwardResponse) Reset()         { *m = ForwardResponse{} }
func (m *ForwardResponse) String() string  { return fmt.Sprintf("%+v", *m) }
func (m *ForwardResponse) ProtoMessage()   {}

func (m *ForwardResponse) Marshal() ([]byte, error) {
	return json.Marshal(m)
}

func (m *ForwardResponse) Unmarshal(data []byte) error {
	return json.Unmarshal(data, m)
}

func (m *ForwardResponse) GetSuccess() bool { return m.Success }

// EdgeHeartbeatRequest 边缘节点心跳请求
type EdgeHeartbeatRequest struct {
	EdgeNodeId    string `json:"edge_node_id,omitempty"`
	CloudPlatform string `json:"cloud_platform,omitempty"`
	Region        string `json:"region,omitempty"`
	Timestamp     int64  `json:"timestamp,omitempty"`
	ProbeCount    int32  `json:"probe_count,omitempty"`
}

func (m *EdgeHeartbeatRequest) Reset()         { *m = EdgeHeartbeatRequest{} }
func (m *EdgeHeartbeatRequest) String() string  { return fmt.Sprintf("%+v", *m) }
func (m *EdgeHeartbeatRequest) ProtoMessage()   {}

func (m *EdgeHeartbeatRequest) Marshal() ([]byte, error) {
	return json.Marshal(m)
}

func (m *EdgeHeartbeatRequest) Unmarshal(data []byte) error {
	return json.Unmarshal(data, m)
}

func (m *EdgeHeartbeatRequest) GetEdgeNodeId() string    { return m.EdgeNodeId }
func (m *EdgeHeartbeatRequest) GetCloudPlatform() string { return m.CloudPlatform }
func (m *EdgeHeartbeatRequest) GetRegion() string        { return m.Region }
func (m *EdgeHeartbeatRequest) GetTimestamp() int64      { return m.Timestamp }
func (m *EdgeHeartbeatRequest) GetProbeCount() int32     { return m.ProbeCount }

// EdgeHeartbeatResponse 边缘节点心跳响应
type EdgeHeartbeatResponse struct {
	Success bool `json:"success,omitempty"`
}

func (m *EdgeHeartbeatResponse) Reset()         { *m = EdgeHeartbeatResponse{} }
func (m *EdgeHeartbeatResponse) String() string  { return fmt.Sprintf("%+v", *m) }
func (m *EdgeHeartbeatResponse) ProtoMessage()   {}

func (m *EdgeHeartbeatResponse) Marshal() ([]byte, error) {
	return json.Marshal(m)
}

func (m *EdgeHeartbeatResponse) Unmarshal(data []byte) error {
	return json.Unmarshal(data, m)
}

func (m *EdgeHeartbeatResponse) GetSuccess() bool { return m.Success }

// ============================================================================
// 编译时检查：所有消息类型实现 proto.Message 接口
// ============================================================================

var (
	_ proto.Message = (*RegisterProbeRequest)(nil)
	_ proto.Message = (*RegisterProbeResponse)(nil)
	_ proto.Message = (*HeartbeatRequest)(nil)
	_ proto.Message = (*HeartbeatResponse)(nil)
	_ proto.Message = (*SendResponse)(nil)
	_ proto.Message = (*MetricData)(nil)
	_ proto.Message = (*MetricsBatch)(nil)
	_ proto.Message = (*TraceSpanData)(nil)
	_ proto.Message = (*TraceBatch)(nil)
	_ proto.Message = (*ProfilingData)(nil)
	_ proto.Message = (*ProfilingBatch)(nil)
	_ proto.Message = (*ProbeInfo)(nil)
	_ proto.Message = (*ReportProbesRequest)(nil)
	_ proto.Message = (*ReportProbesResponse)(nil)
	_ proto.Message = (*ForwardResponse)(nil)
	_ proto.Message = (*EdgeHeartbeatRequest)(nil)
	_ proto.Message = (*EdgeHeartbeatResponse)(nil)
)

// ============================================================================
// ProbeService — Agent -> Edge
// ============================================================================

// ProbeServiceServer is the server API for ProbeService.
type ProbeServiceServer interface {
	RegisterProbe(context.Context, *RegisterProbeRequest) (*RegisterProbeResponse, error)
	Heartbeat(context.Context, *HeartbeatRequest) (*HeartbeatResponse, error)
	SendMetrics(context.Context, *MetricsBatch) (*SendResponse, error)
	SendTraces(context.Context, *TraceBatch) (*SendResponse, error)
	SendProfiling(context.Context, *ProfilingBatch) (*SendResponse, error)
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

// RegisterProbeServiceServer registers the ProbeService server.
func RegisterProbeServiceServer(s *grpc.Server, srv ProbeServiceServer) {
	s.RegisterService(&_ProbeService_serviceDesc, srv)
}

var _ProbeService_serviceDesc = grpc.ServiceDesc{
	ServiceName: "edge.ProbeService",
	HandlerType: (*ProbeServiceServer)(nil),
	Methods: []grpc.MethodDesc{
		{
			MethodName: "RegisterProbe",
			Handler:    _ProbeService_RegisterProbe_Handler,
		},
		{
			MethodName: "Heartbeat",
			Handler:    _ProbeService_Heartbeat_Handler,
		},
		{
			MethodName: "SendMetrics",
			Handler:    _ProbeService_SendMetrics_Handler,
		},
		{
			MethodName: "SendTraces",
			Handler:    _ProbeService_SendTraces_Handler,
		},
		{
			MethodName: "SendProfiling",
			Handler:    _ProbeService_SendProfiling_Handler,
		},
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

// ============================================================================
// CenterService — Edge -> Center
// ============================================================================

// CenterServiceServer is the server API for CenterService.
type CenterServiceServer interface {
	ReportProbes(context.Context, *ReportProbesRequest) (*ReportProbesResponse, error)
	ForwardMetrics(context.Context, *MetricsBatch) (*ForwardResponse, error)
	ForwardTraces(context.Context, *TraceBatch) (*ForwardResponse, error)
	ForwardProfiling(context.Context, *ProfilingBatch) (*ForwardResponse, error)
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
		{
			MethodName: "ReportProbes",
			Handler:    _CenterService_ReportProbes_Handler,
		},
		{
			MethodName: "ForwardMetrics",
			Handler:    _CenterService_ForwardMetrics_Handler,
		},
		{
			MethodName: "ForwardTraces",
			Handler:    _CenterService_ForwardTraces_Handler,
		},
		{
			MethodName: "ForwardProfiling",
			Handler:    _CenterService_ForwardProfiling_Handler,
		},
		{
			MethodName: "Heartbeat",
			Handler:    _CenterService_Heartbeat_Handler,
		},
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
// 远程配置管理 — Agent <-> Center/Edge
// ============================================================================

// CollectionConfig 采集策略配置
type CollectionConfig struct {
	Version     int64             `json:"version,omitempty"`      // 配置版本号
	GroupId     string            `json:"group_id,omitempty"`     // 所属组别
	UpdatedAt   int64             `json:"updated_at,omitempty"`   // 更新时间戳
	UpdatedBy   string            `json:"updated_by,omitempty"`   // 更新者

	// 采样率配置 (0.0-1.0)
	SampleRate      float64 `json:"sample_rate,omitempty"`       // 全局采样率
	TCPSampleRate   float64 `json:"tcp_sample_rate,omitempty"`   // TCP指标采样率
	HTTPSampleRate  float64 `json:"http_sample_rate,omitempty"`  // HTTP指标采样率

	// 采集项开关
	EnableTCPMetrics    bool `json:"enable_tcp_metrics,omitempty"`     // TCP深度指标
	EnableHTTPMetrics   bool `json:"enable_http_metrics,omitempty"`    // HTTP应用层指标
	EnableHTTPFull      bool `json:"enable_http_full,omitempty"`       // HTTP全字段解析
	EnableDNSFull       bool `json:"enable_dns_full,omitempty"`        // DNS全字段解析
	EnableMySQLFull     bool `json:"enable_mysql_full,omitempty"`      // MySQL全字段解析
	EnableSQLAggregator bool `json:"enable_sql_aggregator,omitempty"`  // SQL聚合分析
	EnableCPUProfiler   bool `json:"enable_cpu_profiler,omitempty"`    // ON-CPU剖析

	// 资源限额
	MaxCPUCore    float64 `json:"max_cpu_core,omitempty"`    // 最大CPU核心数
	MaxMemoryMB   float64 `json:"max_memory_mb,omitempty"`   // 最大内存(MB)
	MaxGoroutines int     `json:"max_goroutines,omitempty"`  // 最大协程数

	// 采集间隔和批处理
	CollectInterval int `json:"collect_interval,omitempty"` // 采集间隔(秒)
	BatchSize       int `json:"batch_size,omitempty"`       // 批处理大小

	// 熔断配置
	CircuitBreakerEnabled bool `json:"circuit_breaker_enabled,omitempty"` // 启用熔断
}

func (m *CollectionConfig) Reset()         { *m = CollectionConfig{} }
func (m *CollectionConfig) String() string  { return fmt.Sprintf("%+v", *m) }
func (m *CollectionConfig) ProtoMessage()   {}

func (m *CollectionConfig) Marshal() ([]byte, error) {
	return json.Marshal(m)
}

func (m *CollectionConfig) Unmarshal(data []byte) error {
	return json.Unmarshal(data, m)
}

func (m *CollectionConfig) GetVersion() int64     { return m.Version }
func (m *CollectionConfig) GetGroupId() string    { return m.GroupId }
func (m *CollectionConfig) GetSampleRate() float64 { return m.SampleRate }

// GetConfigRequest 获取配置请求
type GetConfigRequest struct {
	ProbeId   string `json:"probe_id,omitempty"`   // 探针ID
	GroupId   string `json:"group_id,omitempty"`   // 所属组别
	Version   int64  `json:"version,omitempty"`    // 当前配置版本
}

func (m *GetConfigRequest) Reset()         { *m = GetConfigRequest{} }
func (m *GetConfigRequest) String() string  { return fmt.Sprintf("%+v", *m) }
func (m *GetConfigRequest) ProtoMessage()   {}

func (m *GetConfigRequest) Marshal() ([]byte, error) {
	return json.Marshal(m)
}

func (m *GetConfigRequest) Unmarshal(data []byte) error {
	return json.Unmarshal(data, m)
}

// GetConfigResponse 获取配置响应
type GetConfigResponse struct {
	Success  bool              `json:"success,omitempty"`   // 是否成功
	Message  string            `json:"message,omitempty"`   // 消息
	Config   *CollectionConfig `json:"config,omitempty"`    // 配置内容
	HasUpdate bool             `json:"has_update,omitempty"` // 是否有更新
}

func (m *GetConfigResponse) Reset()         { *m = GetConfigResponse{} }
func (m *GetConfigResponse) String() string  { return fmt.Sprintf("%+v", *m) }
func (m *GetConfigResponse) ProtoMessage()   {}

func (m *GetConfigResponse) Marshal() ([]byte, error) {
	return json.Marshal(m)
}

func (m *GetConfigResponse) Unmarshal(data []byte) error {
	return json.Unmarshal(data, m)
}

// ConfigUpdate 配置更新推送
type ConfigUpdate struct {
	Config      *CollectionConfig `json:"config,omitempty"`       // 新配置
	ForceUpdate bool              `json:"force_update,omitempty"` // 强制更新
	Reason      string            `json:"reason,omitempty"`       // 更新原因
}

func (m *ConfigUpdate) Reset()         { *m = ConfigUpdate{} }
func (m *ConfigUpdate) String() string  { return fmt.Sprintf("%+v", *m) }
func (m *ConfigUpdate) ProtoMessage()   {}

func (m *ConfigUpdate) Marshal() ([]byte, error) {
	return json.Marshal(m)
}

func (m *ConfigUpdate) Unmarshal(data []byte) error {
	return json.Unmarshal(data, m)
}

// ConfigUpdateAck 配置更新确认
type ConfigUpdateAck struct {
	ProbeId   string `json:"probe_id,omitempty"`   // 探针ID
	Version   int64  `json:"version,omitempty"`    // 确认的版本
	Success   bool   `json:"success,omitempty"`    // 是否应用成功
	Message   string `json:"message,omitempty"`    // 消息
}

func (m *ConfigUpdateAck) Reset()         { *m = ConfigUpdateAck{} }
func (m *ConfigUpdateAck) String() string  { return fmt.Sprintf("%+v", *m) }
func (m *ConfigUpdateAck) ProtoMessage()   {}

func (m *ConfigUpdateAck) Marshal() ([]byte, error) {
	return json.Marshal(m)
}

func (m *ConfigUpdateAck) Unmarshal(data []byte) error {
	return json.Unmarshal(data, m)
}
