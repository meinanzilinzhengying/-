// Package dataplane Data Plane 服务
//
// 职责:
//   - Flow/Metric/Trace/Log 数据接收 (Ingest)
//   - L7 协议解析
//   - 自适应采样 (Adaptive Sampling)
//   - 数据聚合 (实时/窗口)
//   - 写入存储后端 (ClickHouse/VictoriaMetrics/Loki)
//
// 端口:
//   - gRPC: 9002
//   - Metrics: 9102
package dataplane

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"sync"
	"sync/atomic"
	"time"

	_ "github.com/ClickHouse/clickhouse-go/v2"
	"google.golang.org/grpc"
	"google.golang.org/grpc/health"
	healthpb "google.golang.org/grpc/health/grpc_health_v1"

	"cloud-flow/pkg/flow"
	svcproto "cloud-flow/services/proto"
	"cloud-flow/services/data-plane/sampling"
)

// ============================================================================
// 配置
// ============================================================================

// Config 服务配置
type Config struct {
	ServiceName string
	Version     string

	GrpcAddr string // :9002
	MetricsAddr string // :9102

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

	// 采样配置
	Sampling *sampling.SamplingConfig
}

// DefaultConfig 默认配置
func DefaultConfig() *Config {
	return &Config{
		ServiceName:         "data-plane",
		Version:             "1.0.0",
		GrpcAddr:            ":9002",
		MetricsAddr:         ":9102",
		BatchSize:           10000,
		FlushInterval:       time.Second,
		QueueSize:           100000,
		WorkerCount:         4,
		ClickHouseAddr:      "clickhouse:9000",
		VictoriaMetricsAddr: "http://victoriametrics:8428",
		LokiAddr:            "http://loki:3100",
		Sampling:            sampling.NewSamplingConfig(),
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
	FlowsSampled    uint64 // 被采样保留的 flow
	FlowsWritten    uint64 // 成功写入存储的 flow
	WriteErrors     uint64
	AvgLatencyMs    uint64
}

// ============================================================================
// 服务
// ============================================================================

// Service Data Plane 服务
type Service struct {
	config *Config

	// 采样引擎
	samplingEngine *sampling.SamplingEngine

	// 队列
	flowQueue   chan *flow.UnifiedFlow
	metricQueue chan interface{}
	traceQueue  chan interface{}
	logQueue    chan interface{}

	// 存储
	clickHouseDB *sql.DB

	// 统计
	stats Stats
	statsMu sync.RWMutex

	// gRPC
	grpcServer *grpc.Server
	health     *health.Server

	// Metrics HTTP
	metricsServer *http.Server

	// 运行状态
	startTime time.Time
	running   atomic.Bool
}

// New 创建服务
func New(config *Config) (*Service, error) {
	if config == nil {
		config = DefaultConfig()
	}
	if config.Sampling == nil {
		config.Sampling = sampling.NewSamplingConfig()
	}

	// 初始化采样引擎
	samplingEngine := sampling.NewSamplingEngine(config.Sampling)

	s := &Service{
		config:         config,
		samplingEngine: samplingEngine,
		flowQueue:      make(chan *flow.UnifiedFlow, config.QueueSize),
		metricQueue:    make(chan interface{}, config.QueueSize),
		traceQueue:     make(chan interface{}, config.QueueSize),
		logQueue:       make(chan interface{}, config.QueueSize),
		startTime:      time.Now(),
		health:         health.NewServer(),
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
	// 启动采样引擎后台任务
	s.samplingEngine.Start(context.Background())

	// 初始化 ClickHouse 连接
	if err := s.initClickHouse(); err != nil {
		return fmt.Errorf("ClickHouse init failed: %w", err)
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

	// 启动 Metrics HTTP
	mux := http.NewServeMux()
	mux.HandleFunc("/health", s.healthzHandler)
	mux.HandleFunc("/metrics", s.statsHandler)
	mux.HandleFunc("/api/sampling/config", s.samplingConfigHandler)
	mux.HandleFunc("/api/sampling/stats", s.samplingStatsHandler)

	s.metricsServer = &http.Server{Addr: s.config.MetricsAddr, Handler: mux}
	go func() {
		if err := s.metricsServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			fmt.Printf("Metrics server error: %v\n", err)
		}
	}()

	// 启动 workers
	for i := 0; i < s.config.WorkerCount; i++ {
		go s.flowWorker()
	}

	s.running.Store(true)
	s.health.SetServingStatus(s.config.ServiceName, healthpb.HealthCheckResponse_SERVING)

	fmt.Printf("Data Plane started: gRPC=%s, Metrics=%s (sampling enabled, rate=1/%d)\n",
		s.config.GrpcAddr, s.config.MetricsAddr, s.config.Sampling.DefaultRate)
	return nil
}

// Stop 停止服务
func (s *Service) Stop() {
	s.running.Store(false)
	s.health.SetServingStatus(s.config.ServiceName, healthpb.HealthCheckResponse_NOT_SERVING)

	if s.metricsServer != nil {
		s.metricsServer.Close()
	}
	if s.grpcServer != nil {
		s.grpcServer.GracefulStop()
	}

	// 关闭 ClickHouse 连接
	if s.clickHouseDB != nil {
		s.clickHouseDB.Close()
	}
}

// initClickHouse 初始化 ClickHouse 连接
func (s *Service) initClickHouse() error {
	if s.config.ClickHouseAddr == "" {
		return nil // ClickHouse 未配置，跳过
	}

	dsn := fmt.Sprintf("clickhouse://%s/cloudflow", s.config.ClickHouseAddr)
	db, err := sql.Open("clickhouse", dsn)
	if err != nil {
		return err
	}

	db.SetMaxOpenConns(10)
	db.SetMaxIdleConns(5)

	// 测试连接
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := db.PingContext(ctx); err != nil {
		return err
	}

	s.clickHouseDB = db
	return nil
}

// writeToClickHouse 写入 Flow 到 ClickHouse
func (s *Service) writeToClickHouse(flows []*flow.UnifiedFlow) error {
	if s.clickHouseDB == nil {
		return fmt.Errorf("ClickHouse not configured")
	}

	tx, err := s.clickHouseDB.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	stmt, err := tx.Prepare(`
		INSERT INTO flows (
			timestamp, flow_id, schema_version,
			src_ip, dst_ip, src_port, dst_port, protocol, tcp_flags,
			l7_protocol, method, path, status_code, req_size, resp_size,
			grpc_service, grpc_method, grpc_status,
			pid, process_name, comm,
			container_id, container_name, image,
			pod, namespace, deployment, service, node,
			trace_id, span_id, parent_id,
			host_id, hostname, tenant_id,
			bytes, packets, latency_ns, direction, exception, tags
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`)
	if err != nil {
		return err
	}
	defer stmt.Close()

	for _, f := range flows {
		_, err := stmt.Exec(
			f.Timestamp, f.FlowID, f.SchemaVersion,
			f.SrcIP.String(), f.DstIP.String(), f.SrcPort, f.DstPort, f.Protocol, f.TCPFlags,
			f.L7Protocol, f.Method, f.Path.String(), f.StatusCode, f.ReqSize, f.RespSize,
			f.GetGRPCService(), f.GetGRPCMethod(), 0,
			f.PID, f.ProcessName.String(), f.Comm.String(),
			f.ContainerID.String(), f.ContainerName.String(), f.Image.String(),
			f.Pod.String(), f.Namespace.String(), f.Deployment.String(), f.Service.String(), f.Node.String(),
			f.TraceID.String(), f.SpanID.String(), f.ParentID.String(),
			f.HostID.String(), f.Hostname.String(), f.TenantID.String(),
			f.Bytes, f.Packets, f.LatencyNs, f.Direction, f.GetL7Exception(), "",
		)
		if err != nil {
			return err
		}
	}

	return tx.Commit()
}

// IngestFlow 接收 Flow（含采样决策）
func (s *Service) IngestFlow(ctx context.Context, batch *svcproto.FlowBatch) (*svcproto.IngestResponse, error) {
	accepted := 0
	sampled := 0

	for _, flowMap := range batch.Flows {
		// 反序列化 UnifiedFlow
		f := mapToUnifiedFlow(flowMap)
		if f == nil {
			continue
		}

		// 采样决策
		sCtx := sampling.FlowToContext(f)
		if !s.samplingEngine.ShouldKeep(&sCtx) {
			// 被采样丢弃
			s.statsMu.Lock()
			s.stats.FlowsSampled++
			s.statsMu.Unlock()
			continue
		}

		select {
		case s.flowQueue <- f:
			accepted++
		default:
			s.statsMu.Lock()
			s.stats.FlowsDropped++
			s.statsMu.Unlock()
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

// GetSamplingStats 获取采样统计
func (s *Service) GetSamplingStats() sampling.SamplingStats {
	return s.samplingEngine.GetStats()
}

// UpdateSamplingConfig 运行时更新采样配置
func (s *Service) UpdateSamplingConfig(cfg *sampling.SamplingConfig) {
	s.samplingEngine.UpdateConfig(cfg)
}

// flowWorker Flow 处理 worker
func (s *Service) flowWorker() {
	batch := make([]*flow.UnifiedFlow, 0, s.config.BatchSize)
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

	// 优雅退出：刷新剩余数据
	if len(batch) > 0 {
		s.flushFlows(batch)
	}
}

// flushFlows 刷新 Flow 到存储
func (s *Service) flushFlows(batch []*flow.UnifiedFlow) {
	if len(batch) == 0 {
		return
	}

	// 写入 ClickHouse
	if err := s.writeToClickHouse(batch); err != nil {
		s.statsMu.Lock()
		s.stats.WriteErrors += uint64(len(batch))
		s.statsMu.Unlock()
	} else {
		s.statsMu.Lock()
		s.stats.FlowsWritten += uint64(len(batch))
		s.statsMu.Unlock()
	}
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
	fmt.Fprintf(w, `{"flows_ingested":%d,"flows_dropped":%d,"flows_sampled":%d,"write_errors":%d}`,
		stats.FlowsIngested, stats.FlowsDropped, stats.FlowsSampled, stats.WriteErrors)
}

func (s *Service) samplingConfigHandler(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		writeJSON(w, s.config.Sampling)
	case http.MethodPut, http.MethodPost:
		var cfg sampling.SamplingConfig
		if err := json.NewDecoder(r.Body).Decode(&cfg); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		s.UpdateSamplingConfig(&cfg)
		writeJSON(w, map[string]string{"status": "updated"})
	default:
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
	}
}

func (s *Service) samplingStatsHandler(w http.ResponseWriter, r *http.Request) {
	stats := s.GetSamplingStats()
	writeJSON(w, stats)
}

// ============================================================================
// 辅助函数
// ============================================================================

// mapToUnifiedFlow 将 map[string]interface{} 转换为 UnifiedFlow
func mapToUnifiedFlow(m map[string]interface{}) *flow.UnifiedFlow {
	f := flow.New()

	if v, ok := m["src_ip"].(string); ok {
		f.SrcIP = flow.ParseIP(v)
	}
	if v, ok := m["dst_ip"].(string); ok {
		f.DstIP = flow.ParseIP(v)
	}
	if v, ok := m["src_port"].(float64); ok {
		f.SrcPort = uint16(v)
	}
	if v, ok := m["dst_port"].(float64); ok {
		f.DstPort = uint16(v)
	}
	if v, ok := m["protocol"].(string); ok {
		f.Protocol = flow.ParseProtocol(v)
	}
	if v, ok := m["bytes"].(float64); ok {
		f.Bytes = uint64(v)
	}
	if v, ok := m["packets"].(float64); ok {
		f.Packets = uint64(v)
	}
	if v, ok := m["latency_ns"].(float64); ok {
		f.LatencyNs = uint64(v)
	}
	if v, ok := m["status_code"].(float64); ok {
		f.StatusCode = uint16(v)
	}
	if v, ok := m["tcp_flags"].(float64); ok {
		f.TCPFlags = uint8(v)
	}
	if v, ok := m["l7_protocol"].(string); ok {
		f.L7Protocol = flow.ParseProtocol(v)
	}
	if v, ok := m["tenant_id"].(string); ok {
		f.TenantID.Set(v)
	}
	if v, ok := m["namespace"].(string); ok {
		f.Namespace.Set(v)
	}
	if v, ok := m["service"].(string); ok {
		f.Service.Set(v)
	}
	if v, ok := m["pod"].(string); ok {
		f.Pod.Set(v)
	}
	if v, ok := m["deployment"].(string); ok {
		f.Deployment.Set(v)
	}
	if v, ok := m["node"].(string); ok {
		f.Node.Set(v)
	}
	if v, ok := m["process_name"].(string); ok {
		f.ProcessName.Set(v)
	}
	if v, ok := m["comm"].(string); ok {
		f.Comm.Set(v)
	}
	if v, ok := m["container_id"].(string); ok {
		f.ContainerID.Set(v)
	}
	if v, ok := m["container_name"].(string); ok {
		f.ContainerName.Set(v)
	}
	if v, ok := m["hostname"].(string); ok {
		f.Hostname.Set(v)
	}
	if v, ok := m["timestamp"].(float64); ok {
		f.Timestamp = int64(v)
	}

	return f
}

func writeJSON(w http.ResponseWriter, v interface{}) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(v)
}
