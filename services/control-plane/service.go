// Package controlplane Control Plane 服务
//
// 职责:
//   - Agent 管理 (注册/发现/状态)
//   - Edge 管理 (注册/心跳/状态)
//   - 配置下发 (采集策略/数据面配置)
//   - 集群管理 (etcd 选主/服务注册)
//
// 端口:
//   - gRPC: 9001
//   - HTTP: 8001 (管理 API)
//   - Metrics: 9101
package controlplane

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"sync"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/health"
	healthpb "google.golang.org/grpc/health/grpc_health_v1"

	svcproto "cloud-flow/services/proto"
)

// ============================================================================
// 配置
// ============================================================================

// Config 服务配置
type Config struct {
	// 服务标识
	ServiceName string
	Version     string

	// 监听地址
	GrpcAddr string // :9001
	HttpAddr string // :8001

	// etcd (服务发现 + 选主)
	EtcdEndpoints []string
	EtcdPrefix    string

	// 数据面连接
	DataPlaneAddr string

	// 其他服务连接
	AuthAddr     string
	TenantAddr   string

	// Agent 管理
	AgentTTL         time.Duration
	HeartbeatTimeout time.Duration
}

// DefaultConfig 默认配置
func DefaultConfig() *Config {
	return &Config{
		ServiceName:      "control-plane",
		Version:          "1.0.0",
		GrpcAddr:         ":9001",
		HttpAddr:         ":8001",
		EtcdEndpoints:    []string{"localhost:2379"},
		EtcdPrefix:       "cloudflow/services/",
		AgentTTL:         90 * time.Second,
		HeartbeatTimeout: 60 * time.Second,
	}
}

// ============================================================================
// 服务
// ============================================================================

// Service Control Plane 服务
type Service struct {
	config *Config

	// Agent 注册表
	agents sync.Map // agentId -> *svcproto.AgentInfo
	edges  sync.Map // edgeId -> *svcproto.EdgeInfo

	// gRPC
	grpcServer *grpc.Server
	health     *health.Server

	// HTTP
	httpServer *http.Server

	// 客户端连接
	dataPlaneConn *grpc.ClientConn
	authConn      *grpc.ClientConn
	tenantConn    *grpc.ClientConn

	// 运行状态
	startTime time.Time
}

// New 创建服务
func New(config *Config) (*Service, error) {
	if config == nil {
		config = DefaultConfig()
	}

	s := &Service{
		config:    config,
		startTime: time.Now(),
		health:    health.NewServer(),
	}

	// 初始化 gRPC
	s.grpcServer = grpc.NewServer(
		grpc.MaxRecvMsgSize(64*1024*1024),
		grpc.MaxSendMsgSize(64*1024*1024),
	)

	// 注册服务
	RegisterControlPlaneService(s.grpcServer, s)
	healthpb.RegisterHealthServer(s.grpcServer, s.health)

	return s, nil
}

// Start 启动服务
func (s *Service) Start() error {
	// 启动 gRPC
	lis, err := net.Listen("tcp", s.config.GrpcAddr)
	if err != nil {
		return fmt.Errorf("gRPC listen failed: %w", err)
	}

	go func() {
		if err := s.grpcServer.Serve(lis); err != nil {
			fmt.Printf("gRPC server error: %v\n", err)
		}
	}()

	// 启动 HTTP (管理 API)
	mux := http.NewServeMux()
	mux.HandleFunc("/healthz", s.healthzHandler)
	mux.HandleFunc("/api/agents", s.listAgentsHandler)
	mux.HandleFunc("/api/edges", s.listEdgesHandler)

	s.httpServer = &http.Server{
		Addr:    s.config.HttpAddr,
		Handler: mux,
	}

	go func() {
		if err := s.httpServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			fmt.Printf("HTTP server error: %v\n", err)
		}
	}()

	// 设置健康状态
	s.health.SetServingStatus(s.config.ServiceName, healthpb.HealthCheckResponse_SERVING)

	fmt.Printf("Control Plane started: gRPC=%s, HTTP=%s\n", s.config.GrpcAddr, s.config.HttpAddr)
	return nil
}

// Stop 停止服务
func (s *Service) Stop() {
	s.health.SetServingStatus(s.config.ServiceName, healthpb.HealthCheckResponse_NOT_SERVING)

	if s.httpServer != nil {
		s.httpServer.Close()
	}
	if s.grpcServer != nil {
		s.grpcServer.GracefulStop()
	}
	if s.dataPlaneConn != nil {
		s.dataPlaneConn.Close()
	}
	if s.authConn != nil {
		s.authConn.Close()
	}
	if s.tenantConn != nil {
		s.tenantConn.Close()
	}
}

// ============================================================================
// Agent 管理
// ============================================================================

// RegisterAgent 注册 Agent
func (s *Service) RegisterAgent(agent *svcproto.AgentInfo) {
	agent.Status = "online"
	s.agents.Store(agent.AgentId, agent)
}

// DeregisterAgent 注销 Agent
func (s *Service) DeregisterAgent(agentId string) {
	s.agents.Delete(agentId)
}

// GetAgent 获取 Agent
func (s *Service) GetAgent(agentId string) (*svcproto.AgentInfo, bool) {
	v, ok := s.agents.Load(agentId)
	if !ok {
		return nil, false
	}
	return v.(*svcproto.AgentInfo), true
}

// ListAgents 列出 Agent
func (s *Service) ListAgents(tenantId, region, status string) []*svcproto.AgentInfo {
	var agents []*svcproto.AgentInfo
	s.agents.Range(func(_, v interface{}) bool {
		a := v.(*svcproto.AgentInfo)
		if tenantId != "" && a.TenantId != tenantId {
			return true
		}
		if region != "" && a.Region != region {
			return true
		}
		if status != "" && a.Status != status {
			return true
		}
		agents = append(agents, a)
		return true
	})
	return agents
}

// ============================================================================
// Edge 管理
// ============================================================================

// RegisterEdge 注册 Edge
func (s *Service) RegisterEdge(edge *svcproto.EdgeInfo) {
	edge.Status = "online"
	s.edges.Store(edge.EdgeId, edge)
}

// DeregisterEdge 注销 Edge
func (s *Service) DeregisterEdge(edgeId string) {
	s.edges.Delete(edgeId)
}

// GetEdge 获取 Edge
func (s *Service) GetEdge(edgeId string) (*svcproto.EdgeInfo, bool) {
	v, ok := s.edges.Load(edgeId)
	if !ok {
		return nil, false
	}
	return v.(*svcproto.EdgeInfo), true
}

// ListEdges 列出 Edge
func (s *Service) ListEdges(tenantId, region, status string) []*svcproto.EdgeInfo {
	var edges []*svcproto.EdgeInfo
	s.edges.Range(func(_, v interface{}) bool {
		e := v.(*svcproto.EdgeInfo)
		if tenantId != "" && e.TenantId != tenantId {
			return true
		}
		if region != "" && e.Region != region {
			return true
		}
		if status != "" && e.Status != status {
			return true
		}
		edges = append(edges, e)
		return true
	})
	return edges
}

// ============================================================================
// HTTP Handlers
// ============================================================================

func (s *Service) healthzHandler(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusOK)
	fmt.Fprintf(w, `{"status":"healthy","service":"%s","version":"%s","uptime":%d}`,
		s.config.ServiceName, s.config.Version, time.Since(s.startTime).Seconds())
}

func (s *Service) listAgentsHandler(w http.ResponseWriter, r *http.Request) {
	agents := s.ListAgents("", "", "")
	fmt.Fprintf(w, `{"agents":%d}`, len(agents))
}

func (s *Service) listEdgesHandler(w http.ResponseWriter, r *http.Request) {
	edges := s.ListEdges("", "", "")
	fmt.Fprintf(w, `{"edges":%d}`, len(edges))
}
