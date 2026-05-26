// Package dataplane Data Plane 服务
//
// 职责:
//   - Flow/Metric/Trace/Log 数据接收 (Ingest)
//   - L7 协议解析
//   - 数据聚合 (实时/窗口)
//   - 写入存储后端 (ClickHouse/VictoriaMetrics/Loki)
//
// 端口:
//   - gRPC: 9002
//   - HTTP: 8002
//   - Metrics: 9102
package dataplane

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"sync"
	"sync/atomic"
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
	ServiceName string
	Version     string

	GrpcAddr string // :9002
	HttpAddr string // :8002

	// 批量写入
	BatchSize     int
	FlushInterval time.Duration
	QueueSize     int
	WorkerCount   int

	// 存储后端
	ClickHouseAddr      string
	VictoriaMetricsAddr string
	LokiAddr            string

	// 其他服务
	ControlPlaneAddr string
	TopologyAddr     string
}

// DefaultConfig 默认配置
func DefaultConfig() *Config {
	return &Config{
		ServiceName:         "data-plane",
		Version:             "1.0.0",
		GrpcAddr:            ":9002",
		HttpAddr:            ":8002",
		BatchSize:           10000,
		FlushInterval:       time.Second,
		QueueSize:           100000,
		WorkerCount:         4,
		ClickHouseAddr:      "clickhouse:9000",
		VictoriaMetricsAddr: "http://victoriametrics:8428",
		LokiAddr:            "http://loki:3100",
	}
}

// ============================================================================
// 统计
// ============================================================================

// Stats 数据面统计
type Stats struct {
	FlowsIngested   uint64
	MetricsIngested uint64
	TracesIngested  uint64
	LogsIngested    uint64
	FlowsDropped    uint64
	WriteErrors     uint64
	AvgLatencyMs    uint64
}

// ============================================================================
// 服务
// ============================================================================

// Service Data Plane 服务
type Service struct {
	config *Config

	// 队列
	flowQueue    chan interface{}
	metricQueue  chan interface{}
	traceQueue   chan interface{}
	logQueue     chan interface{}

	// 统计
	stats Stats
	statsMu sync.RWMutex

	// gRPC
	grpcServer *grpc.Server
	health     *health.Server

	// HTTP
	httpServer *http.Server

	// 运行状态
	startTime time.Time
	running   atomic.Bool
}

// New 创建服务
func New(config *Config) (*Service, error) {
	if config == nil {
		config = DefaultConfig()
	}

	s := &Service{
		config:     config,
		flowQueue:  make(chan interface{}, config.QueueSize),
		metricQueue: make(chan interface{}, config.QueueSize),
		traceQueue: make(chan interface{}, config.QueueSize),
		logQueue:   make(chan interface{}, config.QueueSize),
		startTime:  time.Now(),
		health:     health.NewServer(),
	}

	s.grpcServer = grpc.NewServer(
		grpc.MaxRecvMsgSize(64*1024*1024),
		grpc.MaxSendMsgSize(64*1024*1024),
	)

	RegisterDataPlaneService(s.grpcServer, s)
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

	// 启动 HTTP
	mux := http.NewServeMux()
	mux.HandleFunc("/healthz", s.healthzHandler)
	mux.HandleFunc("/api/stats", s.statsHandler)

	s.httpServer = &http.Server{Addr: s.config.HttpAddr, Handler: mux}
	go func() {
		if err := s.httpServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			fmt.Printf("HTTP server error: %v\n", err)
		}
	}()

	// 启动 workers
	for i := 0; i < s.config.WorkerCount; i++ {
		go s.flowWorker()
	}

	s.running.Store(true)
	s.health.SetServingStatus(s.config.ServiceName, healthpb.HealthCheckResponse_SERVING)

	fmt.Printf("Data Plane started: gRPC=%s, HTTP=%s\n", s.config.GrpcAddr, s.config.HttpAddr)
	return nil
}

// Stop 停止服务
func (s *Service) Stop() {
	s.running.Store(false)
	s.health.SetServingStatus(s.config.ServiceName, healthpb.HealthCheckResponse_NOT_SERVING)

	if s.httpServer != nil {
		s.httpServer.Close()
	}
	if s.grpcServer != nil {
		s.grpcServer.GracefulStop()
	}
}

// IngestFlow 接收 Flow
func (s *Service) IngestFlow(ctx context.Context, batch *svcproto.FlowBatch) (*svcproto.IngestResponse, error) {
	accepted := 0
	for _, flow := range batch.Flows {
		select {
		case s.flowQueue <- flow:
			accepted++
		default:
			s.stats.FlowsDropped++
		}
	}

	s.statsMu.Lock()
	s.stats.FlowsIngested += uint64(accepted)
	s.statsMu.Unlock()

	return &svcproto.IngestResponse{
		Accepted: accepted,
		Rejected: len(batch.Flows) - accepted,
		Success:  true,
	}, nil
}

// GetStats 获取统计
func (s *Service) GetStats() Stats {
	s.statsMu.RLock()
	defer s.statsMu.RUnlock()
	return s.stats
}

// flowWorker Flow 处理 worker
func (s *Service) flowWorker() {
	batch := make([]interface{}, 0, s.config.BatchSize)
	ticker := time.NewTicker(s.config.FlushInterval)
	defer ticker.Stop()

	for s.running.Load() {
		select {
		case <-ticker.C:
			if len(batch) > 0 {
				s.flushFlows(batch)
				batch = batch[:0]
			}
		case item := <-s.flowQueue:
			batch = append(batch, item)
			if len(batch) >= s.config.BatchSize {
				s.flushFlows(batch)
				batch = batch[:0]
			}
		}
	}
}

// flushFlows 刷新 Flow 到存储
func (s *Service) flushFlows(batch []interface{}) {
	// TODO: 写入 ClickHouse
	s.statsMu.Lock()
	s.stats.WriteErrors++
	s.statsMu.Unlock()
}

// ============================================================================
// HTTP Handlers
// ============================================================================

func (s *Service) healthzHandler(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusOK)
	fmt.Fprintf(w, `{"status":"healthy","service":"%s","version":"%s"}`,
		s.config.ServiceName, s.config.Version)
}

func (s *Service) statsHandler(w http.ResponseWriter, r *http.Request) {
	stats := s.GetStats()
	fmt.Fprintf(w, `{"flows_ingested":%d,"flows_dropped":%d,"write_errors":%d}`,
		stats.FlowsIngested, stats.FlowsDropped, stats.WriteErrors)
}
