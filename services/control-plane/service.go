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
	"strings"
	"sync"
	"time"

	clientv3 "go.etcd.io/etcd/client/v3"
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

	// etcd
	etcdClient *clientv3.Client
	etcdLease  clientv3.LeaseID
	etcdCtx    context.Context
	etcdCancel context.CancelFunc

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
	running   bool
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
	// 初始化 etcd
	if err := s.initEtcd(); err != nil {
		return fmt.Errorf("etcd init failed: %w", err)
	}

	// 建立下游服务连接
	if err := s.connectToDownstream(); err != nil {
		return fmt.Errorf("connect downstream failed: %w", err)
	}

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
	
	// P1-05 修复: 添加认证中间件保护管理 API
	protected := s.authMiddleware(mux)
	protected.HandleFunc("/api/agents", s.listAgentsHandler)
	protected.HandleFunc("/api/edges", s.listEdgesHandler)

	s.httpServer = &http.Server{
		Addr:    s.config.HttpAddr,
		Handler: protected,
	}

	go func() {
		if err := s.httpServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			fmt.Printf("HTTP server error: %v\n", err)
		}
	}()

	// 设置健康状态
	s.health.SetServingStatus(s.config.ServiceName, healthpb.HealthCheckResponse_SERVING)

	// 注册到 etcd
	if err := s.registerToEtcd(); err != nil {
		return fmt.Errorf("etcd register failed: %w", err)
	}

	s.running = true

	fmt.Printf("Control Plane started: gRPC=%s, HTTP=%s\n", s.config.GrpcAddr, s.config.HttpAddr)
	return nil
}

// Stop 停止服务
func (s *Service) Stop() {
	s.running = false
	s.health.SetServingStatus(s.config.ServiceName, healthpb.HealthCheckResponse_NOT_SERVING)

	// 取消 etcd 租约
	if s.etcdCancel != nil {
		s.etcdCancel()
	}

	// 优雅停止 gRPC（带超时）
	if s.grpcServer != nil {
		stopChan := make(chan struct{})
		go func() {
			s.grpcServer.GracefulStop()
			close(stopChan)
		}()

		select {
		case <-stopChan:
			// 正常停止
		case <-time.After(15 * time.Second):
			s.grpcServer.Stop() // 强制停止
		}
	}

	// 停止 HTTP（带超时）
	if s.httpServer != nil {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		s.httpServer.Shutdown(ctx)
	}

	// 关闭连接
	if s.dataPlaneConn != nil {
		s.dataPlaneConn.Close()
	}
	if s.authConn != nil {
		s.authConn.Close()
	}
	if s.tenantConn != nil {
		s.tenantConn.Close()
	}

	// 关闭 etcd
	if s.etcdClient != nil {
		s.etcdClient.Close()
	}
}

// initEtcd 初始化 etcd 客户端
func (s *Service) initEtcd() error {
	if len(s.config.EtcdEndpoints) == 0 {
		return nil // etcd 未配置
	}

	client, err := clientv3.New(clientv3.Config{
		Endpoints:   s.config.EtcdEndpoints,
		DialTimeout: 5 * time.Second,
	})
	if err != nil {
		return err
	}

	s.etcdClient = client
	s.etcdCtx, s.etcdCancel = context.WithCancel(context.Background())
	return nil
}

// registerToEtcd 注册服务到 etcd
func (s *Service) registerToEtcd() error {
	if s.etcdClient == nil {
		return nil
	}

	// 创建租约
	leaseResp, err := s.etcdClient.Grant(s.etcdCtx, int64(s.config.AgentTTL.Seconds()))
	if err != nil {
		return err
	}
	s.etcdLease = leaseResp.ID

	// 注册服务
	serviceKey := s.config.EtcdPrefix + "control-plane/" + s.config.ServiceName
	serviceValue := s.config.GrpcAddr

	_, err = s.etcdClient.Put(s.etcdCtx, serviceKey, serviceValue, clientv3.WithLease(s.etcdLease))
	if err != nil {
		return err
	}

	// 保持租约
	go func() {
		for s.running {
			time.Sleep(s.config.AgentTTL / 2)
			if s.etcdClient != nil {
				s.etcdClient.KeepAliveOnce(s.etcdCtx, s.etcdLease)
			}
		}
	}()

	return nil
}

// connectToDownstream 建立下游服务连接
func (s *Service) connectToDownstream() error {
	var errs []error

	// 连接 Data Plane
	if s.config.DataPlaneAddr != "" {
		conn, err := grpc.Dial(
			s.config.DataPlaneAddr,
			grpc.WithInsecure(),
			grpc.WithTimeout(5*time.Second),
		)
		if err != nil {
			errs = append(errs, fmt.Errorf("connect data-plane failed: %w", err))
		} else {
			s.dataPlaneConn = conn
		}
	}

	// 连接 Auth Service
	if s.config.AuthAddr != "" {
		conn, err := grpc.Dial(
			s.config.AuthAddr,
			grpc.WithInsecure(),
			grpc.WithTimeout(5*time.Second),
		)
		if err != nil {
			errs = append(errs, fmt.Errorf("connect auth-service failed: %w", err))
		} else {
			s.authConn = conn
		}
	}

	// 连接 Tenant Service
	if s.config.TenantAddr != "" {
		conn, err := grpc.Dial(
			s.config.TenantAddr,
			grpc.WithInsecure(),
			grpc.WithTimeout(5*time.Second),
		)
		if err != nil {
			errs = append(errs, fmt.Errorf("connect tenant-service failed: %w", err))
		} else {
			s.tenantConn = conn
		}
	}

	if len(errs) > 0 {
		return fmt.Errorf("downstream connect errors: %v", errs)
	}
	return nil
}

// authMiddleware 认证中间件
func (s *Service) authMiddleware(next http.Handler) *http.ServeMux {
	protectedMux := http.NewServeMux()

	protectedMux.HandleFunc("/healthz", s.healthzHandler)

	protectedMux.HandleFunc("/api/agents", func(w http.ResponseWriter, r *http.Request) {
		if !s.authenticateRequest(r) {
			http.Error(w, "Unauthorized", http.StatusUnauthorized)
			return
		}
		s.listAgentsHandler(w, r)
	})

	protectedMux.HandleFunc("/api/edges", func(w http.ResponseWriter, r *http.Request) {
		if !s.authenticateRequest(r) {
			http.Error(w, "Unauthorized", http.StatusUnauthorized)
			return
		}
		s.listEdgesHandler(w, r)
	})

	return protectedMux
}

// authenticateRequest 验证请求
func (s *Service) authenticateRequest(r *http.Request) bool {
	// 检查 API Key
	apiKey := r.Header.Get("X-API-Key")
	if apiKey != "" {
		return s.validateAPIKey(r.Context(), apiKey)
	}

	// 检查 Bearer Token
	authHeader := r.Header.Get("Authorization")
	if len(authHeader) > 7 && strings.ToLower(authHeader[:7]) == "bearer " {
		token := authHeader[7:]
		return s.validateToken(r.Context(), token)
	}

	return false
}

// validateAPIKey 验证 API Key
func (s *Service) validateAPIKey(ctx context.Context, apiKey string) bool {
	if s.authConn == nil {
		return false
	}

	client := svcproto.NewAuthServiceClient(s.authConn)
	resp, err := client.ValidateToken(ctx, &svcproto.ValidateTokenRequest{
		Token: apiKey,
	})
	if err != nil {
		return false
	}

	return resp.Valid
}

// validateToken 验证 JWT Token
func (s *Service) validateToken(ctx context.Context, token string) bool {
	if s.authConn == nil {
		return false
	}

	client := svcproto.NewAuthServiceClient(s.authConn)
	resp, err := client.ValidateToken(ctx, &svcproto.ValidateTokenRequest{
		Token: token,
	})
	if err != nil {
		return false
	}

	return resp.Valid
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
