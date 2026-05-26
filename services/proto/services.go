// Package proto 微服务间 gRPC 通信定义
//
// 服务拆分后的内部通信协议:
//
//	┌──────────────┐     ┌──────────────┐     ┌──────────────┐
//	│ control-plane│────>│  data-plane  │────>│ query-service│
//	│  (管理面)     │     │  (数据面)     │     │  (查询面)     │
//	└──────┬───────┘     └──────┬───────┘     └──────┬───────┘
//	       │                    │                    │
//	       v                    v                    v
//	┌──────────────┐     ┌──────────────┐     ┌──────────────┐
//	│ topology-eng │     │ alert-engine │     │ auth-service │
//	│  (拓扑引擎)   │     │  (告警引擎)   │     │  (认证服务)   │
//	└──────────────┘     └──────────────┘     └──────┬───────┘
//	                                                │
//	                                        ┌───────v───────┐
//	                                        │ tenant-service│
//	                                        │  (租户服务)    │
//	                                        └──────────────┘
package proto

import (
	"context"

	"google.golang.org/grpc"
)

// ============================================================================
// 服务发现与健康检查
// ============================================================================

// HealthCheckRequest 健康检查请求
type HealthCheckRequest struct {
	Service string `json:"service"`
}

// HealthCheckResponse 健康检查响应
type HealthCheckResponse struct {
	Healthy bool   `json:"healthy"`
	Version string `json:"version"`
	Uptime  int64  `json:"uptime"`
}

// ServiceInfo 服务信息
type ServiceInfo struct {
	Name      string `json:"name"`
	Version   string `json:"version"`
	Addr      string `json:"addr"`
	GrpcPort  int    `json:"grpc_port"`
	HttpPort  int    `json:"http_port"`
	Status    string `json:"status"`
}

// ============================================================================
// control-plane → data-plane
// ============================================================================

// IngestConfig 数据面配置
type IngestConfig struct {
	TenantId      string `json:"tenant_id"`
	Enabled       bool   `json:"enabled"`
	BatchSize     int    `json:"batch_size"`
	FlushInterval int64  `json:"flush_interval_ms"`
	// 采集策略
	CollectCPU    bool `json:"collect_cpu"`
	CollectMemory bool `json:"collect_memory"`
	CollectNetwork bool `json:"collect_network"`
	CollectL7     bool `json:"collect_l7"`
}

// UpdateIngestConfigRequest 更新数据面配置
type UpdateIngestConfigRequest struct {
	AgentId string        `json:"agent_id"`
	Config  *IngestConfig `json:"config"`
}

// UpdateIngestConfigResponse 更新配置响应
type UpdateIngestConfigResponse struct {
	Success bool   `json:"success"`
	Message string `json:"message"`
}

// ============================================================================
// control-plane → 各服务 (Agent/Edge 管理)
// ============================================================================

// AgentInfo Agent 信息
type AgentInfo struct {
	AgentId     string `json:"agent_id"`
	Hostname    string `json:"hostname"`
	Ip          string `json:"ip"`
	Os          string `json:"os"`
	Arch        string `json:"arch"`
	Version     string `json:"version"`
	Status      string `json:"status"`
	EdgeId      string `json:"edge_id"`
	Region      string `json:"region"`
	TenantId    string `json:"tenant_id"`
}

// EdgeInfo Edge 信息
type EdgeInfo struct {
	EdgeId      string `json:"edge_id"`
	Address     string `json:"address"`
	Region      string `json:"region"`
	Version     string `json:"version"`
	Status      string `json:"status"`
	AgentCount  int    `json:"agent_count"`
	TenantId    string `json:"tenant_id"`
}

// ListAgentsRequest 列出 Agent
type ListAgentsRequest struct {
	TenantId string `json:"tenant_id"`
	Region   string `json:"region"`
	Status   string `json:"status"`
}

// ListAgentsResponse Agent 列表响应
type ListAgentsResponse struct {
	Agents []*AgentInfo `json:"agents"`
	Total  int          `json:"total"`
}

// ListEdgesRequest 列出 Edge
type ListEdgesRequest struct {
	TenantId string `json:"tenant_id"`
	Region   string `json:"region"`
	Status   string `json:"status"`
}

// ListEdgesResponse Edge 列表响应
type ListEdgesResponse struct {
	Edges []*EdgeInfo `json:"edges"`
	Total int         `json:"total"`
}

// ============================================================================
// data-plane → query-service / topology-engine
// ============================================================================

// FlowBatch 流量批量数据
type FlowBatch struct {
	TenantId string            `json:"tenant_id"`
	Flows    []map[string]interface{} `json:"flows"`
	Count    int               `json:"count"`
}

// QueryFlowRequest 流量查询请求
type QueryFlowRequest struct {
	TenantId   string `json:"tenant_id"`
	StartTime  int64  `json:"start_time"`
	EndTime    int64  `json:"end_time"`
	SrcIp      string `json:"src_ip"`
	DstIp      string `json:"dst_ip"`
	Namespace  string `json:"namespace"`
	Service    string `json:"service"`
	Limit      int    `json:"limit"`
}

// QueryFlowResponse 流量查询响应
type QueryFlowResponse struct {
	Records []map[string]interface{} `json:"records"`
	Total   int64                    `json:"total"`
	TookMs  int64                    `json:"took_ms"`
}

// ============================================================================
// topology-engine
// ============================================================================

// TopologyQueryRequest 拓扑查询请求
type TopologyQueryRequest struct {
	TenantId  string            `json:"tenant_id"`
	StartTime int64             `json:"start_time"`
	EndTime   int64             `json:"end_time"`
	Type      string            `json:"type"` // service/pod/ip
	GroupBy   []string          `json:"group_by"`
	Filters   map[string]string `json:"filters"`
	MaxDepth  int               `json:"max_depth"`
}

// TopologyNode 拓扑节点
type TopologyNode struct {
	Id        string            `json:"id"`
	Name      string            `json:"name"`
	Type      string            `json:"type"`
	Namespace string            `json:"namespace"`
	Metadata  map[string]string `json:"metadata"`
	BytesIn   uint64            `json:"bytes_in"`
	BytesOut  uint64            `json:"bytes_out"`
	Errors    uint64            `json:"errors"`
}

// TopologyEdge 拓扑边
type TopologyEdge struct {
	Source   string `json:"source"`
	Target   string `json:"target"`
	Protocol string `json:"protocol"`
	Port     uint16 `json:"port"`
	Bytes    uint64 `json:"bytes"`
	Latency  uint64 `json:"latency"`
	Errors   uint64 `json:"errors"`
}

// TopologyQueryResponse 拓扑查询响应
type TopologyQueryResponse struct {
	Nodes []*TopologyNode `json:"nodes"`
	Edges []*TopologyEdge `json:"edges"`
}

// HeatmapPoint 热力图单元格
type HeatmapPoint struct {
	Source    string  `json:"source"`
	Target    string  `json:"target"`
	Timestamp int64   `json:"timestamp"`
	Value     float64 `json:"value"`
	Count     uint64  `json:"count"`
}

// HeatmapResponse 热力图查询响应
type HeatmapResponse struct {
	Points    []*HeatmapPoint `json:"points"`
	MinValue  float64         `json:"min_value"`
	MaxValue  float64         `json:"max_value"`
	AvgValue  float64         `json:"avg_value"`
	StartTime int64           `json:"start_time"`
	EndTime   int64           `json:"end_time"`
	Interval  int64           `json:"interval"` // seconds
}

// TopologyDiffRequest 拓扑差异请求
type TopologyDiffRequest struct {
	TenantId     string `json:"tenant_id"`
	BaseTime     int64  `json:"base_time"`
	CompareTime  int64  `json:"compare_time"`
	Type         string `json:"type"` // service/pod/namespace/process
}

// TopologyDiff 单个差异项
type TopologyDiff struct {
	DiffType string `json:"diff_type"` // added_node, removed_node, added_edge, removed_edge, weight_change
	NodeId   string `json:"node_id,omitempty"`
	Source   string `json:"source,omitempty"`
	Target   string `json:"target,omitempty"`
	Field    string `json:"field,omitempty"`     // for weight_change: "bytes"/"latency"/"errors"
	OldValue uint64 `json:"old_value,omitempty"`
	NewValue uint64 `json:"new_value,omitempty"`
}

// TopologyDiffResponse 拓扑差异响应
type TopologyDiffResponse struct {
	Diffs       []*TopologyDiff `json:"diffs"`
	BaseTime    int64           `json:"base_time"`
	CompareTime int64           `json:"compare_time"`
	Summary     *DiffSummary    `json:"summary"`
}

// DiffSummary 差异摘要
type DiffSummary struct {
	AddedNodes   int `json:"added_nodes"`
	RemovedNodes int `json:"removed_nodes"`
	AddedEdges   int `json:"added_edges"`
	RemovedEdges int `json:"removed_edges"`
	ChangedEdges int `json:"changed_edges"`
}

// TopologySnapshot 版本化拓扑快照（用于缓存）
type TopologySnapshot struct {
	Version   uint64             `json:"version"`
	Timestamp int64              `json:"timestamp"`
	TenantId  string             `json:"tenant_id"`
	Type      string             `json:"type"`
	Nodes     []*TopologyNode    `json:"nodes"`
	Edges     []*TopologyEdge    `json:"edges"`
}

// FlowIngestRequest 实时流量摄入请求（来自数据面）
type FlowIngestRequest struct {
	TenantId string `json:"tenant_id"`
	Flows    []byte `json:"flows"` // serialized UnifiedFlow batch
	Count    int    `json:"count"`
}

// FlowIngestResponse 流量摄入响应
type FlowIngestResponse struct {
	Accepted int     `json:"accepted"`
	Rejected int     `json:"rejected"`
	Version  uint64  `json:"version"` // new topology version after processing
}

// ============================================================================
// alert-engine
// ============================================================================

// AlertRule 告警规则
type AlertRule struct {
	Id          string `json:"id"`
	Name        string `json:"name"`
	TenantId    string `json:"tenant_id"`
	Metric      string `json:"metric"`
	Operator    string `json:"operator"`
	Threshold   float64 `json:"threshold"`
	Duration    int64  `json:"duration_s"`
	Severity    string `json:"severity"`
	Enabled     bool   `json:"enabled"`
	Labels      map[string]string `json:"labels"`
}

// Alert 告警
type Alert struct {
	Id          string            `json:"id"`
	RuleId      string            `json:"rule_id"`
	Name        string            `json:"name"`
	TenantId    string            `json:"tenant_id"`
	Severity    string            `json:"severity"`
	Message     string            `json:"message"`
	Labels      map[string]string `json:"labels"`
	StartedAt   int64             `json:"started_at"`
	FiredAt     int64             `json:"fired_at"`
	ResolvedAt  int64             `json:"resolved_at"`
	Status      string            `json:"status"` // firing/resolved
}

// EvaluateAlertsRequest 评估告警请求
type EvaluateAlertsRequest struct {
	TenantId string            `json:"tenant_id"`
	Metrics  map[string]float64 `json:"metrics"`
}

// EvaluateAlertsResponse 评估告警响应
type EvaluateAlertsResponse struct {
	Alerts []*Alert `json:"alerts"`
}

// ============================================================================
// auth-service
// ============================================================================

// AuthenticateRequest 认证请求
type AuthenticateRequest struct {
	Username string `json:"username"`
	Password string `json:"password"`
}

// AuthenticateResponse 认证响应
type AuthenticateResponse struct {
	Token     string `json:"token"`
	ExpiresAt int64  `json:"expires_at"`
	UserId    string `json:"user_id"`
	Role      string `json:"role"`
}

// ValidateTokenRequest Token 验证请求
type ValidateTokenRequest struct {
	Token string `json:"token"`
}

// ValidateTokenResponse Token 验证响应
type ValidateTokenResponse struct {
	Valid    bool   `json:"valid"`
	UserId   string `json:"user_id"`
	Username string `json:"username"`
	Role     string `json:"role"`
	TenantId string `json:"tenant_id"`
}

// AuthorizeRequest 鉴权请求
type AuthorizeRequest struct {
	UserId string `json:"user_id"`
	Role   string `json:"role"`
	Action string `json:"action"`
	Resource string `json:"resource"`
}

// AuthorizeResponse 鉴权响应
type AuthorizeResponse struct {
	Allowed bool   `json:"allowed"`
	Reason  string `json:"reason"`
}

// ============================================================================
// tenant-service
// ============================================================================

// Tenant 租户
type Tenant struct {
	Id          string `json:"id"`
	Name        string `json:"name"`
	DisplayName string `json:"display_name"`
	Status      string `json:"status"`
	Plan        string `json:"plan"` // free/pro/enterprise
	Quota       *TenantQuota `json:"quota"`
	CreatedAt   int64  `json:"created_at"`
	UpdatedAt   int64  `json:"updated_at"`
}

// TenantQuota 租户配额
type TenantQuota struct {
	MaxAgents       int   `json:"max_agents"`
	MaxEdges        int   `json:"max_edges"`
	MaxFlowsPerDay  int64 `json:"max_flows_per_day"`
	MaxStorageGB    int   `json:"max_storage_gb"`
	MaxAlertRules   int   `json:"max_alert_rules"`
	RetentionDays   int   `json:"retention_days"`
}

// CreateTenantRequest 创建租户请求
type CreateTenantRequest struct {
	Name        string       `json:"name"`
	DisplayName string       `json:"display_name"`
	Plan        string       `json:"plan"`
	Quota       *TenantQuota `json:"quota"`
}

// CreateTenantResponse 创建租户响应
type CreateTenantResponse struct {
	Tenant *Tenant `json:"tenant"`
}

// GetTenantRequest 获取租户请求
type GetTenantRequest struct {
	TenantId string `json:"tenant_id"`
}

// GetTenantResponse 获取租户响应
type GetTenantResponse struct {
	Tenant *Tenant `json:"tenant"`
}

// ListTenantsRequest 列出租户请求
type ListTenantsRequest struct {
	Status string `json:"status"`
	Plan   string `json:"plan"`
}

// ListTenantsResponse 列出租户响应
type ListTenantsResponse struct {
	Tenants []*Tenant `json:"tenants"`
	Total   int       `json:"total"`
}

// UpdateTenantQuotaRequest 更新租户配额
type UpdateTenantQuotaRequest struct {
	TenantId string       `json:"tenant_id"`
	Quota    *TenantQuota `json:"quota"`
}

// UpdateTenantQuotaResponse 更新配额响应
type UpdateTenantQuotaResponse struct {
	Success bool   `json:"success"`
	Message string `json:"message"`
}

// ============================================================================
// RBAC / Project / Policy / OIDC 类型定义
// ============================================================================

// Project 项目
type Project struct {
	Id          string   `json:"id"`
	TenantId    string   `json:"tenant_id"`
	Name        string   `json:"name"`
	DisplayName string   `json:"display_name"`
	Description string   `json:"description"`
	Status      string   `json:"status"` // active/archived
	Namespaces  []string `json:"namespaces"` // K8s namespaces belonging to this project
	CreatedAt   int64    `json:"created_at"`
	UpdatedAt   int64    `json:"updated_at"`
}

// Role 角色
type Role struct {
	Id          string   `json:"id"`
	TenantId    string   `json:"tenant_id"`
	ProjectId   string   `json:"project_id"` // empty = tenant-level role
	Name        string   `json:"name"`       // admin/editor/viewer/custom
	DisplayName string   `json:"display_name"`
	Description string   `json:"description"`
	IsBuiltin   bool     `json:"is_builtin"`  // true for default roles
	Permissions []string `json:"permissions"` // list of permission strings
	CreatedAt   int64    `json:"created_at"`
}

// Policy 策略
type Policy struct {
	Id          string            `json:"id"`
	TenantId    string            `json:"tenant_id"`
	ProjectId   string            `json:"project_id"`
	Name        string            `json:"name"`
	Description string            `json:"description"`
	Effect      string            `json:"effect"`    // allow/deny
	Actions     []string          `json:"actions"`   // e.g. ["flow:read", "alert:write"]
	Resources   []string          `json:"resources"` // e.g. ["flow:*", "alert:rule:*"]
	Conditions  map[string]string `json:"conditions"` // e.g. {"namespace": "production"}
	Priority    int               `json:"priority"`   // higher = evaluated first
	CreatedAt   int64             `json:"created_at"`
	UpdatedAt   int64             `json:"updated_at"`
}

// UserBinding 用户-角色-项目绑定
type UserBinding struct {
	UserId    string `json:"user_id"`
	TenantId  string `json:"tenant_id"`
	ProjectId string `json:"project_id"` // empty = tenant-level
	RoleId    string `json:"role_id"`
	RoleName  string `json:"role_name"`
}

// OIDCConfig OIDC 配置
type OIDCConfig struct {
	Issuer       string `json:"issuer"`
	ClientId     string `json:"client_id"`
	ClientSecret string `json:"client_secret"`
	RedirectURL  string `json:"redirect_url"`
	Scopes       string `json:"scopes"` // "openid profile email"
}

// TenantContext 租户上下文（用于 gRPC metadata 传播）
type TenantContext struct {
	TenantId   string   `json:"tenant_id"`
	UserId     string   `json:"user_id"`
	Username   string   `json:"username"`
	Role       string   `json:"role"`
	ProjectId  string   `json:"project_id,omitempty"`
	Namespaces []string `json:"namespaces,omitempty"`
}

// OIDCCallbackRequest OIDC 回调请求
type OIDCCallbackRequest struct {
	Code  string `json:"code"`
	State string `json:"state"`
}

// ============================================================================
// Project CRUD 请求/响应类型
// ============================================================================

// CreateProjectRequest 创建项目请求
type CreateProjectRequest struct {
	TenantId    string   `json:"tenant_id"`
	Name        string   `json:"name"`
	DisplayName string   `json:"display_name"`
	Description string   `json:"description"`
	Namespaces  []string `json:"namespaces"`
}

// CreateProjectResponse 创建项目响应
type CreateProjectResponse struct {
	Project *Project `json:"project"`
}

// ListProjectsRequest 列出项目请求
type ListProjectsRequest struct {
	TenantId string `json:"tenant_id"`
	Status   string `json:"status"`
}

// ListProjectsResponse 列出项目响应
type ListProjectsResponse struct {
	Projects []*Project `json:"projects"`
	Total    int        `json:"total"`
}

// ============================================================================
// Role / Policy 管理请求/响应类型
// ============================================================================

// CreateRoleRequest 创建角色请求
type CreateRoleRequest struct {
	TenantId    string   `json:"tenant_id"`
	ProjectId   string   `json:"project_id"`
	Name        string   `json:"name"`
	DisplayName string   `json:"display_name"`
	Description string   `json:"description"`
	Permissions []string `json:"permissions"`
}

// CreateRoleResponse 创建角色响应
type CreateRoleResponse struct {
	Role *Role `json:"role"`
}

// BindUserRoleRequest 绑定用户角色请求
type BindUserRoleRequest struct {
	TenantId  string `json:"tenant_id"`
	UserId    string `json:"user_id"`
	ProjectId string `json:"project_id"`
	RoleId    string `json:"role_id"`
}

// BindUserRoleResponse 绑定用户角色响应
type BindUserRoleResponse struct {
	Success bool   `json:"success"`
	Message string `json:"message"`
}

// CreatePolicyRequest 创建策略请求
type CreatePolicyRequest struct {
	TenantId    string            `json:"tenant_id"`
	ProjectId   string            `json:"project_id"`
	Name        string            `json:"name"`
	Description string            `json:"description"`
	Effect      string            `json:"effect"`
	Actions     []string          `json:"actions"`
	Resources   []string          `json:"resources"`
	Conditions  map[string]string `json:"conditions"`
	Priority    int               `json:"priority"`
}

// CreatePolicyResponse 创建策略响应
type CreatePolicyResponse struct {
	Policy *Policy `json:"policy"`
}

// CheckPermissionRequest 检查权限请求
type CheckPermissionRequest struct {
	TenantId string `json:"tenant_id"`
	UserId   string `json:"user_id"`
	Action   string `json:"action"`
	Resource string `json:"resource"`
}

// CheckPermissionResponse 检查权限响应
type CheckPermissionResponse struct {
	Allowed bool   `json:"allowed"`
	Reason  string `json:"reason"`
}

// ============================================================================
// gRPC 服务接口定义
// ============================================================================

// ControlPlaneService control-plane gRPC 服务
type ControlPlaneServiceServer interface {
	// 健康检查
	HealthCheck(ctx context.Context, req *HealthCheckRequest) (*HealthCheckResponse, error)
	// Agent 管理
	ListAgents(ctx context.Context, req *ListAgentsRequest) (*ListAgentsResponse, error)
	GetAgent(ctx context.Context, req *AgentInfo) (*AgentInfo, error)
	// Edge 管理
	ListEdges(ctx context.Context, req *ListEdgesRequest) (*ListEdgesResponse, error)
	GetEdge(ctx context.Context, req *EdgeInfo) (*EdgeInfo, error)
	// 配置下发
	UpdateIngestConfig(ctx context.Context, req *UpdateIngestConfigRequest) (*UpdateIngestConfigResponse, error)
}

// DataPlaneService data-plane gRPC 服务
type DataPlaneServiceServer interface {
	HealthCheck(ctx context.Context, req *HealthCheckRequest) (*HealthCheckResponse, error)
	// 数据接收
	IngestFlows(ctx context.Context, req *FlowBatch) (*IngestResponse, error)
	IngestMetrics(ctx context.Context, req *FlowBatch) (*IngestResponse, error)
	// 配置更新
	ApplyConfig(ctx context.Context, req *UpdateIngestConfigRequest) (*UpdateIngestConfigResponse, error)
}

// IngestResponse 数据接收响应
type IngestResponse struct {
	Accepted int  `json:"accepted"`
	Rejected int  `json:"rejected"`
	Success  bool `json:"success"`
}

// QueryServiceServer query-service gRPC 服务
type QueryServiceServer interface {
	HealthCheck(ctx context.Context, req *HealthCheckRequest) (*HealthCheckResponse, error)
	// 流量查询
	QueryFlows(ctx context.Context, req *QueryFlowRequest) (*QueryFlowResponse, error)
	QueryMetrics(ctx context.Context, req *QueryFlowRequest) (*QueryFlowResponse, error)
	QueryTraces(ctx context.Context, req *QueryFlowRequest) (*QueryFlowResponse, error)
	// Dashboard 聚合
	QueryDashboard(ctx context.Context, req *QueryFlowRequest) (*QueryFlowResponse, error)
}

// TopologyServiceServer topology-engine gRPC 服务
type TopologyServiceServer interface {
	HealthCheck(ctx context.Context, req *HealthCheckRequest) (*HealthCheckResponse, error)
	QueryTopology(ctx context.Context, req *TopologyQueryRequest) (*TopologyQueryResponse, error)
	GetServiceGraph(ctx context.Context, req *TopologyQueryRequest) (*TopologyQueryResponse, error)
	GetDependencyGraph(ctx context.Context, req *TopologyQueryRequest) (*TopologyQueryResponse, error)
	// New methods:
	GetLatencyHeatmap(ctx context.Context, req *TopologyQueryRequest) (*HeatmapResponse, error)
	GetErrorHeatmap(ctx context.Context, req *TopologyQueryRequest) (*HeatmapResponse, error)
	GetTopologyDiff(ctx context.Context, req *TopologyDiffRequest) (*TopologyDiffResponse, error)
	IngestFlows(ctx context.Context, req *FlowIngestRequest) (*FlowIngestResponse, error)
	GetSnapshot(ctx context.Context, req *TopologyQueryRequest) (*TopologySnapshot, error)
}

// UnimplementedTopologyServiceServer 可嵌入的默认实现，使现有代码无需实现新方法即可编译
type UnimplementedTopologyServiceServer struct{}

func (UnimplementedTopologyServiceServer) HealthCheck(ctx context.Context, req *HealthCheckRequest) (*HealthCheckResponse, error) {
	return nil, nil
}

func (UnimplementedTopologyServiceServer) QueryTopology(ctx context.Context, req *TopologyQueryRequest) (*TopologyQueryResponse, error) {
	return nil, nil
}

func (UnimplementedTopologyServiceServer) GetServiceGraph(ctx context.Context, req *TopologyQueryRequest) (*TopologyQueryResponse, error) {
	return nil, nil
}

func (UnimplementedTopologyServiceServer) GetDependencyGraph(ctx context.Context, req *TopologyQueryRequest) (*TopologyQueryResponse, error) {
	return nil, nil
}

func (UnimplementedTopologyServiceServer) GetLatencyHeatmap(ctx context.Context, req *TopologyQueryRequest) (*HeatmapResponse, error) {
	return nil, nil
}

func (UnimplementedTopologyServiceServer) GetErrorHeatmap(ctx context.Context, req *TopologyQueryRequest) (*HeatmapResponse, error) {
	return nil, nil
}

func (UnimplementedTopologyServiceServer) GetTopologyDiff(ctx context.Context, req *TopologyDiffRequest) (*TopologyDiffResponse, error) {
	return nil, nil
}

func (UnimplementedTopologyServiceServer) IngestFlows(ctx context.Context, req *FlowIngestRequest) (*FlowIngestResponse, error) {
	return nil, nil
}

func (UnimplementedTopologyServiceServer) GetSnapshot(ctx context.Context, req *TopologyQueryRequest) (*TopologySnapshot, error) {
	return nil, nil
}

// RegisterTopologyServiceServer 注册 TopologyServiceServer 到 gRPC server
func RegisterTopologyServiceServer(s *grpc.Server, srv TopologyServiceServer) {
	s.RegisterService(&_TopologyService_serviceDesc, srv)
}

var _TopologyService_serviceDesc = grpc.ServiceDesc{
	ServiceName: "proto.TopologyService",
	HandlerType: (*TopologyServiceServer)(nil),
	Methods: []grpc.MethodDesc{
		{
			MethodName: "HealthCheck",
			Handler:     topologyServiceHealthCheckHandler,
		},
		{
			MethodName: "QueryTopology",
			Handler:     topologyServiceQueryTopologyHandler,
		},
		{
			MethodName: "GetServiceGraph",
			Handler:     topologyServiceGetServiceGraphHandler,
		},
		{
			MethodName: "GetDependencyGraph",
			Handler:     topologyServiceGetDependencyGraphHandler,
		},
		{
			MethodName: "GetLatencyHeatmap",
			Handler:     topologyServiceGetLatencyHeatmapHandler,
		},
		{
			MethodName: "GetErrorHeatmap",
			Handler:     topologyServiceGetErrorHeatmapHandler,
		},
		{
			MethodName: "GetTopologyDiff",
			Handler:     topologyServiceGetTopologyDiffHandler,
		},
		{
			MethodName: "IngestFlows",
			Handler:     topologyServiceIngestFlowsHandler,
		},
		{
			MethodName: "GetSnapshot",
			Handler:     topologyServiceGetSnapshotHandler,
		},
	},
	Streams:  []grpc.StreamDesc{},
	Metadata: "proto/services.proto",
}

func topologyServiceHealthCheckHandler(srv interface{}, ctx context.Context, dec func(interface{}) error, interceptor grpc.UnaryServerInterceptor) (interface{}, error) {
	in := new(HealthCheckRequest)
	if err := dec(in); err != nil {
		return nil, err
	}
	if interceptor == nil {
		return srv.(TopologyServiceServer).HealthCheck(ctx, in)
	}
	info := &grpc.UnaryServerInfo{Server: srv, FullMethod: "/proto.TopologyService/HealthCheck"}
	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		return srv.(TopologyServiceServer).HealthCheck(ctx, req.(*HealthCheckRequest))
	}
	return interceptor(ctx, in, info, handler)
}

func topologyServiceQueryTopologyHandler(srv interface{}, ctx context.Context, dec func(interface{}) error, interceptor grpc.UnaryServerInterceptor) (interface{}, error) {
	in := new(TopologyQueryRequest)
	if err := dec(in); err != nil {
		return nil, err
	}
	if interceptor == nil {
		return srv.(TopologyServiceServer).QueryTopology(ctx, in)
	}
	info := &grpc.UnaryServerInfo{Server: srv, FullMethod: "/proto.TopologyService/QueryTopology"}
	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		return srv.(TopologyServiceServer).QueryTopology(ctx, req.(*TopologyQueryRequest))
	}
	return interceptor(ctx, in, info, handler)
}

func topologyServiceGetServiceGraphHandler(srv interface{}, ctx context.Context, dec func(interface{}) error, interceptor grpc.UnaryServerInterceptor) (interface{}, error) {
	in := new(TopologyQueryRequest)
	if err := dec(in); err != nil {
		return nil, err
	}
	if interceptor == nil {
		return srv.(TopologyServiceServer).GetServiceGraph(ctx, in)
	}
	info := &grpc.UnaryServerInfo{Server: srv, FullMethod: "/proto.TopologyService/GetServiceGraph"}
	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		return srv.(TopologyServiceServer).GetServiceGraph(ctx, req.(*TopologyQueryRequest))
	}
	return interceptor(ctx, in, info, handler)
}

func topologyServiceGetDependencyGraphHandler(srv interface{}, ctx context.Context, dec func(interface{}) error, interceptor grpc.UnaryServerInterceptor) (interface{}, error) {
	in := new(TopologyQueryRequest)
	if err := dec(in); err != nil {
		return nil, err
	}
	if interceptor == nil {
		return srv.(TopologyServiceServer).GetDependencyGraph(ctx, in)
	}
	info := &grpc.UnaryServerInfo{Server: srv, FullMethod: "/proto.TopologyService/GetDependencyGraph"}
	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		return srv.(TopologyServiceServer).GetDependencyGraph(ctx, req.(*TopologyQueryRequest))
	}
	return interceptor(ctx, in, info, handler)
}

func topologyServiceGetLatencyHeatmapHandler(srv interface{}, ctx context.Context, dec func(interface{}) error, interceptor grpc.UnaryServerInterceptor) (interface{}, error) {
	in := new(TopologyQueryRequest)
	if err := dec(in); err != nil {
		return nil, err
	}
	if interceptor == nil {
		return srv.(TopologyServiceServer).GetLatencyHeatmap(ctx, in)
	}
	info := &grpc.UnaryServerInfo{Server: srv, FullMethod: "/proto.TopologyService/GetLatencyHeatmap"}
	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		return srv.(TopologyServiceServer).GetLatencyHeatmap(ctx, req.(*TopologyQueryRequest))
	}
	return interceptor(ctx, in, info, handler)
}

func topologyServiceGetErrorHeatmapHandler(srv interface{}, ctx context.Context, dec func(interface{}) error, interceptor grpc.UnaryServerInterceptor) (interface{}, error) {
	in := new(TopologyQueryRequest)
	if err := dec(in); err != nil {
		return nil, err
	}
	if interceptor == nil {
		return srv.(TopologyServiceServer).GetErrorHeatmap(ctx, in)
	}
	info := &grpc.UnaryServerInfo{Server: srv, FullMethod: "/proto.TopologyService/GetErrorHeatmap"}
	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		return srv.(TopologyServiceServer).GetErrorHeatmap(ctx, req.(*TopologyQueryRequest))
	}
	return interceptor(ctx, in, info, handler)
}

func topologyServiceGetTopologyDiffHandler(srv interface{}, ctx context.Context, dec func(interface{}) error, interceptor grpc.UnaryServerInterceptor) (interface{}, error) {
	in := new(TopologyDiffRequest)
	if err := dec(in); err != nil {
		return nil, err
	}
	if interceptor == nil {
		return srv.(TopologyServiceServer).GetTopologyDiff(ctx, in)
	}
	info := &grpc.UnaryServerInfo{Server: srv, FullMethod: "/proto.TopologyService/GetTopologyDiff"}
	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		return srv.(TopologyServiceServer).GetTopologyDiff(ctx, req.(*TopologyDiffRequest))
	}
	return interceptor(ctx, in, info, handler)
}

func topologyServiceIngestFlowsHandler(srv interface{}, ctx context.Context, dec func(interface{}) error, interceptor grpc.UnaryServerInterceptor) (interface{}, error) {
	in := new(FlowIngestRequest)
	if err := dec(in); err != nil {
		return nil, err
	}
	if interceptor == nil {
		return srv.(TopologyServiceServer).IngestFlows(ctx, in)
	}
	info := &grpc.UnaryServerInfo{Server: srv, FullMethod: "/proto.TopologyService/IngestFlows"}
	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		return srv.(TopologyServiceServer).IngestFlows(ctx, req.(*FlowIngestRequest))
	}
	return interceptor(ctx, in, info, handler)
}

func topologyServiceGetSnapshotHandler(srv interface{}, ctx context.Context, dec func(interface{}) error, interceptor grpc.UnaryServerInterceptor) (interface{}, error) {
	in := new(TopologyQueryRequest)
	if err := dec(in); err != nil {
		return nil, err
	}
	if interceptor == nil {
		return srv.(TopologyServiceServer).GetSnapshot(ctx, in)
	}
	info := &grpc.UnaryServerInfo{Server: srv, FullMethod: "/proto.TopologyService/GetSnapshot"}
	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		return srv.(TopologyServiceServer).GetSnapshot(ctx, req.(*TopologyQueryRequest))
	}
	return interceptor(ctx, in, info, handler)
}

// AlertServiceServer alert-engine gRPC 服务
type AlertServiceServer interface {
	HealthCheck(ctx context.Context, req *HealthCheckRequest) (*HealthCheckResponse, error)
	// 规则管理
	CreateRule(ctx context.Context, req *AlertRule) (*AlertRule, error)
	GetRule(ctx context.Context, req *AlertRule) (*AlertRule, error)
	ListRules(ctx context.Context, req *ListTenantsRequest) ([]*AlertRule, error)
	DeleteRule(ctx context.Context, req *AlertRule) (*AlertRule, error)
	// 告警查询
	ListAlerts(ctx context.Context, req *ListTenantsRequest) ([]*Alert, error)
	EvaluateAlerts(ctx context.Context, req *EvaluateAlertsRequest) (*EvaluateAlertsResponse, error)
}

// AuthServiceServer auth-service gRPC 服务
type AuthServiceServer interface {
	HealthCheck(ctx context.Context, req *HealthCheckRequest) (*HealthCheckResponse, error)
	Authenticate(ctx context.Context, req *AuthenticateRequest) (*AuthenticateResponse, error)
	ValidateToken(ctx context.Context, req *ValidateTokenRequest) (*ValidateTokenResponse, error)
	Authorize(ctx context.Context, req *AuthorizeRequest) (*AuthorizeResponse, error)
	// New RBAC methods:
	CreateRole(ctx context.Context, req *CreateRoleRequest) (*CreateRoleResponse, error)
	BindUserRole(ctx context.Context, req *BindUserRoleRequest) (*BindUserRoleResponse, error)
	CreatePolicy(ctx context.Context, req *CreatePolicyRequest) (*CreatePolicyResponse, error)
	CheckPermission(ctx context.Context, req *CheckPermissionRequest) (*CheckPermissionResponse, error)
	OIDCCallback(ctx context.Context, req *OIDCCallbackRequest) (*AuthenticateResponse, error)
}

// TenantServiceServer tenant-service gRPC 服务
type TenantServiceServer interface {
	HealthCheck(ctx context.Context, req *HealthCheckRequest) (*HealthCheckResponse, error)
	CreateTenant(ctx context.Context, req *CreateTenantRequest) (*CreateTenantResponse, error)
	GetTenant(ctx context.Context, req *GetTenantRequest) (*GetTenantResponse, error)
	ListTenants(ctx context.Context, req *ListTenantsRequest) (*ListTenantsResponse, error)
	UpdateQuota(ctx context.Context, req *UpdateTenantQuotaRequest) (*UpdateTenantQuotaResponse, error)
	// New project methods:
	CreateProject(ctx context.Context, req *CreateProjectRequest) (*CreateProjectResponse, error)
	ListProjects(ctx context.Context, req *ListProjectsRequest) (*ListProjectsResponse, error)
}

// UnimplementedAuthServiceServer 可嵌入的默认实现，使现有代码无需实现新方法即可编译
type UnimplementedAuthServiceServer struct{}

func (UnimplementedAuthServiceServer) HealthCheck(ctx context.Context, req *HealthCheckRequest) (*HealthCheckResponse, error) {
	return nil, nil
}

func (UnimplementedAuthServiceServer) Authenticate(ctx context.Context, req *AuthenticateRequest) (*AuthenticateResponse, error) {
	return nil, nil
}

func (UnimplementedAuthServiceServer) ValidateToken(ctx context.Context, req *ValidateTokenRequest) (*ValidateTokenResponse, error) {
	return nil, nil
}

func (UnimplementedAuthServiceServer) Authorize(ctx context.Context, req *AuthorizeRequest) (*AuthorizeResponse, error) {
	return nil, nil
}

func (UnimplementedAuthServiceServer) CreateRole(ctx context.Context, req *CreateRoleRequest) (*CreateRoleResponse, error) {
	return nil, nil
}

func (UnimplementedAuthServiceServer) BindUserRole(ctx context.Context, req *BindUserRoleRequest) (*BindUserRoleResponse, error) {
	return nil, nil
}

func (UnimplementedAuthServiceServer) CreatePolicy(ctx context.Context, req *CreatePolicyRequest) (*CreatePolicyResponse, error) {
	return nil, nil
}

func (UnimplementedAuthServiceServer) CheckPermission(ctx context.Context, req *CheckPermissionRequest) (*CheckPermissionResponse, error) {
	return nil, nil
}

func (UnimplementedAuthServiceServer) OIDCCallback(ctx context.Context, req *OIDCCallbackRequest) (*AuthenticateResponse, error) {
	return nil, nil
}

// UnimplementedTenantServiceServer 可嵌入的默认实现，使现有代码无需实现新方法即可编译
type UnimplementedTenantServiceServer struct{}

func (UnimplementedTenantServiceServer) HealthCheck(ctx context.Context, req *HealthCheckRequest) (*HealthCheckResponse, error) {
	return nil, nil
}

func (UnimplementedTenantServiceServer) CreateTenant(ctx context.Context, req *CreateTenantRequest) (*CreateTenantResponse, error) {
	return nil, nil
}

func (UnimplementedTenantServiceServer) GetTenant(ctx context.Context, req *GetTenantRequest) (*GetTenantResponse, error) {
	return nil, nil
}

func (UnimplementedTenantServiceServer) ListTenants(ctx context.Context, req *ListTenantsRequest) (*ListTenantsResponse, error) {
	return nil, nil
}

func (UnimplementedTenantServiceServer) UpdateQuota(ctx context.Context, req *UpdateTenantQuotaRequest) (*UpdateTenantQuotaResponse, error) {
	return nil, nil
}

func (UnimplementedTenantServiceServer) CreateProject(ctx context.Context, req *CreateProjectRequest) (*CreateProjectResponse, error) {
	return nil, nil
}

func (UnimplementedTenantServiceServer) ListProjects(ctx context.Context, req *ListProjectsRequest) (*ListProjectsResponse, error) {
	return nil, nil
}
