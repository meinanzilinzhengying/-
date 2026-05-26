// Package queryservice Query Service 服务
//
// 职责:
//   - Dashboard 查询聚合
//   - API Gateway (统一入口)
//   - 查询路由 (Flow→ClickHouse, Metrics→VM, Logs→Loki)
//   - 跨服务查询编排
//
// 端口:
//   - gRPC: 9003
//   - HTTP: 8003 (对外 API)
//   - Metrics: 9103
package queryservice

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
	GrpcAddr    string // :9003
	HttpAddr    string // :8003

	// 后端连接
	DataPlaneAddr    string
	TopologyAddr     string
	AlertAddr        string
	ClickHouseAddr   string
	VictoriaMetricsAddr string
	LokiAddr         string

	// 查询配置
	QueryTimeout     time.Duration
	MaxConcurrentQueries int
}

func DefaultConfig() *Config {
	return &Config{
		ServiceName:         "query-service",
		Version:             "1.0.0",
		GrpcAddr:            ":9003",
		HttpAddr:            ":8003",
		QueryTimeout:        30 * time.Second,
		MaxConcurrentQueries: 1000,
	}
}

// Stats 查询统计
type Stats struct {
	QueryCount     uint64
	QueryErrors    uint64
	QueryFromCache uint64
	AvgLatencyMs   uint64
}

// Service Query Service
type Service struct {
	config *Config

	// 客户端连接
	dataPlaneConn   *grpc.ClientConn
	topologyConn    *grpc.ClientConn
	alertConn       *grpc.ClientConn

	// gRPC
	grpcServer *grpc.Server
	health     *health.Server

	// HTTP
	httpServer *http.Server

	// 统计
	stats Stats
	statsMu sync.RWMutex

	startTime time.Time
}

func New(config *Config) (*Service, error) {
	if config == nil {
		config = DefaultConfig()
	}

	s := &Service{
		config:    config,
		startTime: time.Now(),
		health:    health.NewServer(),
	}

	s.grpcServer = grpc.NewServer(
		grpc.MaxRecvMsgSize(64*1024*1024),
		grpc.MaxSendMsgSize(64*1024*1024),
	)

	RegisterQueryService(s.grpcServer, s)
	healthpb.RegisterHealthServer(s.grpcServer, s.health)

	return s, nil
}

func (s *Service) Start() error {
	lis, err := net.Listen("tcp", s.config.GrpcAddr)
	if err != nil {
		return fmt.Errorf("gRPC listen failed: %w", err)
	}

	go func() {
		s.grpcServer.Serve(lis)
	}()

	// HTTP API Gateway
	mux := http.NewServeMux()
	mux.HandleFunc("/healthz", s.healthzHandler)
	mux.HandleFunc("/api/overview", s.overviewHandler)
	mux.HandleFunc("/api/metrics", s.metricsHandler)
	mux.HandleFunc("/api/flows", s.flowsHandler)
	mux.HandleFunc("/api/traces", s.tracesHandler)
	mux.HandleFunc("/api/topology", s.topologyHandler)
	mux.HandleFunc("/api/alerts", s.alertsHandler)

	s.httpServer = &http.Server{Addr: s.config.HttpAddr, Handler: mux}
	go func() {
		s.httpServer.ListenAndServe()
	}()

	s.health.SetServingStatus(s.config.ServiceName, healthpb.HealthCheckResponse_SERVING)
	fmt.Printf("Query Service started: gRPC=%s, HTTP=%s\n", s.config.GrpcAddr, s.config.HttpAddr)
	return nil
}

func (s *Service) Stop() {
	s.health.SetServingStatus(s.config.ServiceName, healthpb.HealthCheckResponse_NOT_SERVING)
	if s.httpServer != nil {
		s.httpServer.Close()
	}
	if s.grpcServer != nil {
		s.grpcServer.GracefulStop()
	}
}

// QueryFlows 查询 Flow
func (s *Service) QueryFlows(ctx context.Context, req *svcproto.QueryFlowRequest) (*svcproto.QueryFlowResponse, error) {
	// TODO: 查询 ClickHouse
	return &svcproto.QueryFlowResponse{Total: 0, TookMs: 0}, nil
}

// QueryMetrics 查询 Metrics
func (s *Service) QueryMetrics(ctx context.Context, req *svcproto.QueryFlowRequest) (*svcproto.QueryFlowResponse, error) {
	// TODO: 查询 VictoriaMetrics
	return &svcproto.QueryFlowResponse{Total: 0, TookMs: 0}, nil
}

// QueryTraces 查询 Traces
func (s *Service) QueryTraces(ctx context.Context, req *svcproto.QueryFlowRequest) (*svcproto.QueryFlowResponse, error) {
	// TODO: 查询 ClickHouse
	return &svcproto.QueryFlowResponse{Total: 0, TookMs: 0}, nil
}

// QueryDashboard Dashboard 聚合查询
func (s *Service) QueryDashboard(ctx context.Context, req *svcproto.QueryFlowRequest) (*svcproto.QueryFlowResponse, error) {
	// TODO: 跨服务聚合
	return &svcproto.QueryFlowResponse{Total: 0, TookMs: 0}, nil
}

// HTTP Handlers
func (s *Service) healthzHandler(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusOK)
	fmt.Fprintf(w, `{"status":"healthy","service":"%s"}`, s.config.ServiceName)
}
func (s *Service) overviewHandler(w http.ResponseWriter, r *http.Request)   { w.WriteHeader(http.StatusOK); fmt.Fprint(w, `{}`) }
func (s *Service) metricsHandler(w http.ResponseWriter, r *http.Request)    { w.WriteHeader(http.StatusOK); fmt.Fprint(w, `{}`) }
func (s *Service) flowsHandler(w http.ResponseWriter, r *http.Request)      { w.WriteHeader(http.StatusOK); fmt.Fprint(w, `{}`) }
func (s *Service) tracesHandler(w http.ResponseWriter, r *http.Request)     { w.WriteHeader(http.StatusOK); fmt.Fprint(w, `{}`) }
func (s *Service) topologyHandler(w http.ResponseWriter, r *http.Request)   { w.WriteHeader(http.StatusOK); fmt.Fprint(w, `{}`) }
func (s *Service) alertsHandler(w http.ResponseWriter, r *http.Request)     { w.WriteHeader(http.StatusOK); fmt.Fprint(w, `{}`) }
