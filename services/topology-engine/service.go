// Package topologyengine Topology Engine 服务
//
// 职责:
//   - Service Graph (服务拓扑图)
//   - Dependency Graph (依赖关系图)
//   - 实时拓扑计算
//   - 拓扑变更事件
//
// 端口:
//   - gRPC: 9004
//   - HTTP: 8004
//   - Metrics: 9104
package topologyengine

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

// Config 服务配置
type Config struct {
	ServiceName string
	Version     string
	GrpcAddr    string // :9004
	HttpAddr    string // :8004

	ClickHouseAddr string
	DataPlaneAddr  string

	// 拓扑计算
	ComputeInterval time.Duration
	MaxNodes        int
	MaxEdges        int
}

func DefaultConfig() *Config {
	return &Config{
		ServiceName:     "topology-engine",
		Version:         "1.0.0",
		GrpcAddr:        ":9004",
		HttpAddr:        ":8004",
		ComputeInterval: 30 * time.Second,
		MaxNodes:        10000,
		MaxEdges:        100000,
	}
}

// Service Topology Engine
type Service struct {
	config *Config

	// 拓扑缓存
	topologyCache *TopologyCache

	// gRPC
	grpcServer *grpc.Server
	health     *health.Server

	// HTTP
	httpServer *http.Server

	startTime time.Time
}

// TopologyCache 拓扑缓存
type TopologyCache struct {
	mu          sync.RWMutex
	nodes       []*svcproto.TopologyNode
	edges       []*svcproto.TopologyEdge
	lastUpdate  time.Time
}

func New(config *Config) (*Service, error) {
	if config == nil {
		config = DefaultConfig()
	}

	s := &Service{
		config:         config,
		startTime:      time.Now(),
		health:         health.NewServer(),
		topologyCache:  &TopologyCache{},
	}

	s.grpcServer = grpc.NewServer()
	RegisterTopologyService(s.grpcServer, s)
	healthpb.RegisterHealthServer(s.grpcServer, s.health)

	return s, nil
}

func (s *Service) Start() error {
	lis, err := net.Listen("tcp", s.config.GrpcAddr)
	if err != nil {
		return err
	}
	go func() { s.grpcServer.Serve(lis) }()

	mux := http.NewServeMux()
	mux.HandleFunc("/healthz", s.healthzHandler)
	mux.HandleFunc("/api/topology", s.topologyHTTPHandler)
	s.httpServer = &http.Server{Addr: s.config.HttpAddr, Handler: mux}
	go func() { s.httpServer.ListenAndServe() }()

	s.health.SetServingStatus(s.config.ServiceName, healthpb.HealthCheckResponse_SERVING)
	fmt.Printf("Topology Engine started: gRPC=%s, HTTP=%s\n", s.config.GrpcAddr, s.config.HttpAddr)

	// 启动拓扑计算
	go s.computeLoop()

	return nil
}

func (s *Service) Stop() {
	s.health.SetServingStatus(s.config.ServiceName, healthpb.HealthCheckResponse_NOT_SERVING)
	if s.httpServer != nil { s.httpServer.Close() }
	if s.grpcServer != nil { s.grpcServer.GracefulStop() }
}

// QueryTopology 查询拓扑
func (s *Service) QueryTopology(ctx context.Context, req *svcproto.TopologyQueryRequest) (*svcproto.TopologyQueryResponse, error) {
	s.topologyCache.mu.RLock()
	defer s.topologyCache.mu.RUnlock()

	return &svcproto.TopologyQueryResponse{
		Nodes: s.topologyCache.nodes,
		Edges: s.topologyCache.edges,
	}, nil
}

// GetServiceGraph 获取服务拓扑图
func (s *Service) GetServiceGraph(ctx context.Context, req *svcproto.TopologyQueryRequest) (*svcproto.TopologyQueryResponse, error) {
	req.Type = "service"
	return s.QueryTopology(ctx, req)
}

// GetDependencyGraph 获取依赖关系图
func (s *Service) GetDependencyGraph(ctx context.Context, req *svcproto.TopologyQueryRequest) (*svcproto.TopologyQueryResponse, error) {
	// TODO: 计算依赖关系
	return &svcproto.TopologyQueryResponse{}, nil
}

// computeLoop 拓扑计算循环
func (s *Service) computeLoop() {
	ticker := time.NewTicker(s.config.ComputeInterval)
	defer ticker.Stop()

	for range ticker.C {
		s.computeTopology()
	}
}

// computeTopology 计算拓扑
func (s *Service) computeTopology() {
	// TODO: 从 ClickHouse topology 表查询
	s.topologyCache.mu.Lock()
	s.topologyCache.lastUpdate = time.Now()
	s.topologyCache.mu.Unlock()
}

func (s *Service) healthzHandler(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusOK)
	fmt.Fprintf(w, `{"status":"healthy","service":"%s"}`, s.config.ServiceName)
}

func (s *Service) topologyHTTPHandler(w http.ResponseWriter, r *http.Request) {
	resp, _ := s.QueryTopology(r.Context(), &svcproto.TopologyQueryRequest{})
	w.Header().Set("Content-Type", "application/json")
	fmt.Fprintf(w, `{"nodes":%d,"edges":%d}`, len(resp.Nodes), len(resp.Edges))
}
