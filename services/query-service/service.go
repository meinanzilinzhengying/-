// Package queryservice Query Service 服务
//
// 职责:
//   - Dashboard 查询聚合
//   - API Gateway (统一入口)
//   - 查询路由 (Flow→ClickHouse, Metrics→VM, Logs→Loki)
//   - 跨服务查询编排
//   - OTLP 数据接收与查询
//   - Trace-Flow 关联分析
//
// 端口:
//   - gRPC: 9003
//   - HTTP: 8003 (对外 API)
//   - OTLP gRPC: 4317
//   - OTLP HTTP: 4318
//   - Metrics: 9103
package queryservice

import (
	"context"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"sync"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/health"
	healthpb "google.golang.org/grpc/health/grpc_health_v1"

	svcproto "cloud-flow/services/proto"
	"cloud-flow/services/query-service/correlation"
	"cloud-flow/services/query-service/otel"
)

// Config 服务配置
type Config struct {
	ServiceName string
	Version     string
	GrpcAddr    string // :9003
	HttpAddr    string // :8003

	// 后端连接
	DataPlaneAddr      string
	TopologyAddr       string
	AlertAddr          string
	ClickHouseAddr     string
	VictoriaMetricsAddr string
	LokiAddr           string

	// 查询配置
	QueryTimeout        time.Duration
	MaxConcurrentQueries int
}

func DefaultConfig() *Config {
	return &Config{
		ServiceName:          "query-service",
		Version:              "1.0.0",
		GrpcAddr:             ":9003",
		HttpAddr:             ":8003",
		QueryTimeout:         30 * time.Second,
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
	dataPlaneConn *grpc.ClientConn
	topologyConn  *grpc.ClientConn
	alertConn     *grpc.ClientConn

	// gRPC
	grpcServer *grpc.Server
	health     *health.Server

	// HTTP
	httpServer *http.Server

	// OTLP
	otlpReceiver      *otel.OTLPReceiver
	correlationEngine *correlation.CorrelationEngine

	// 统计
	stats   Stats
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

	// OTLP Receiver
	s.otlpReceiver = otel.NewOTLPReceiver(otel.OTLPReceiverConfig{
		GRPCAddr: ":4317",
		HTTPAddr: ":4318",
	})

	// Correlation Engine
	s.correlationEngine = correlation.NewCorrelationEngine(1000000, 30*time.Minute)

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
	mux.HandleFunc("/api/otel/traces", s.otelTracesHandler)
	mux.HandleFunc("/api/otel/metrics", s.otelMetricsHandler)
	mux.HandleFunc("/api/otel/logs", s.otelLogsHandler)
	mux.HandleFunc("/api/otel/stats", s.otelStatsHandler)
	mux.HandleFunc("/api/rca", s.rcaHandler)
	mux.HandleFunc("/api/correlation", s.correlationHandler)

	s.httpServer = &http.Server{Addr: s.config.HttpAddr, Handler: mux}
	go func() {
		s.httpServer.ListenAndServe()
	}()

	// Start OTLP Receiver
	ctx := context.Background()
	s.otlpReceiver.Start(ctx)

	// Start Correlation Engine
	s.correlationEngine.Start(ctx)

	s.health.SetServingStatus(s.config.ServiceName, healthpb.HealthCheckResponse_SERVING)
	fmt.Printf("Query Service started: gRPC=%s, HTTP=%s\n", s.config.GrpcAddr, s.config.HttpAddr)
	return nil
}

func (s *Service) Stop() {
	s.health.SetServingStatus(s.config.ServiceName, healthpb.HealthCheckResponse_NOT_SERVING)
	s.otlpReceiver.Stop()
	s.correlationEngine.Stop()
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

// QueryOTLPTraces 查询 OTEL Traces
func (s *Service) QueryOTLPTraces(ctx context.Context, req *svcproto.TraceQueryRequest) (*svcproto.TraceQueryResponse, error) {
	// Convert to internal query and search
	query := otel.SpanQuery{
		ServiceName: req.ServiceName,
		MinDuration: time.Duration(req.MinDuration),
		HasError:    req.HasError,
		StartTime:   req.StartTime,
		EndTime:     req.EndTime,
		Limit:       req.Limit,
	}
	if req.Limit == 0 {
		query.Limit = 100
	}
	spans := s.otlpReceiver.TraceStore().SearchSpans(query)
	// Group spans by trace ID
	traceMap := make(map[string][]*otel.Span)
	for _, span := range spans {
		traceMap[span.TraceID] = append(traceMap[span.TraceID], span)
	}
	var traces []*svcproto.TraceInfo
	for traceID, spans := range traceMap {
		var minStart, maxEnd int64
		var errCount int
		var spanInfos []*svcproto.SpanInfo
		for _, sp := range spans {
			if minStart == 0 || sp.StartTime < minStart {
				minStart = sp.StartTime
			}
			if sp.EndTime > maxEnd {
				maxEnd = sp.EndTime
			}
			if sp.Status == "STATUS_CODE_ERROR" {
				errCount++
			}
			spanInfos = append(spanInfos, convertSpanToProto(sp))
		}
		traces = append(traces, &svcproto.TraceInfo{
			TraceId:     traceID,
			ServiceName: spans[0].ServiceName,
			SpanCount:   len(spans),
			ErrorCount:  errCount,
			DurationNs:  maxEnd - minStart,
			StartTime:   minStart,
			Spans:       spanInfos,
		})
	}
	return &svcproto.TraceQueryResponse{Traces: traces, Total: len(traces)}, nil
}

// GetRootCauseAnalysis 根因分析
func (s *Service) GetRootCauseAnalysis(ctx context.Context, req *svcproto.RootCauseRequest) (*svcproto.RootCauseResponse, error) {
	report := s.correlationEngine.GetRootCauseAnalysis(req.TraceId)
	if report == nil {
		return &svcproto.RootCauseResponse{}, nil
	}
	resp := &svcproto.RootCauseResponse{
		TraceId:          report.TraceID,
		AffectedServices: report.AffectedServices,
		SuggestedCauses:  report.SuggestedCauses,
	}
	for _, es := range report.ErrorSpans {
		resp.ErrorSpans = append(resp.ErrorSpans, &svcproto.SpanInfo{
			TraceId: report.TraceID, SpanId: es.SpanID,
			ServiceName: es.ServiceName, Name: es.OperationName,
			DurationNs: es.DurationNs,
		})
	}
	for _, ss := range report.SlowSpans {
		resp.SlowSpans = append(resp.SlowSpans, &svcproto.SpanInfo{
			TraceId: report.TraceID, SpanId: ss.SpanID,
			ServiceName: ss.ServiceName, Name: ss.OperationName,
			DurationNs: ss.DurationNs,
		})
	}
	for _, fl := range report.RelatedFlows {
		resp.RelatedFlows = append(resp.RelatedFlows, &svcproto.FlowTraceLink{
			TraceId: fl.TraceID, SpanId: fl.SpanID,
			ServiceName: fl.ServiceName,
			SrcIp: fl.FlowSrcIP, DstIp: fl.FlowDstIP,
			SrcPort: fl.FlowSrcPort, DstPort: fl.FlowDstPort,
			Protocol: fl.FlowProtocol, LatencyNs: fl.FlowLatencyNs,
			Bytes: fl.FlowBytes, Timestamp: fl.FlowTimestamp,
		})
	}
	return resp, nil
}

// QueryCorrelation 关联查询
func (s *Service) QueryCorrelation(ctx context.Context, req *svcproto.CorrelationQueryRequest) (*svcproto.CorrelationQueryResponse, error) {
	var traceIDs []string
	switch req.QueryType {
	case "trace_to_flow":
		links := s.correlationEngine.GetFlowsByTraceID(req.TraceId)
		var flows []*svcproto.FlowTraceLink
		for _, fl := range links {
			flows = append(flows, &svcproto.FlowTraceLink{
				TraceId: fl.TraceID, SpanId: fl.SpanID,
				ServiceName: fl.ServiceName,
				SrcIp: fl.FlowSrcIP, DstIp: fl.FlowDstIP,
				SrcPort: fl.FlowSrcPort, DstPort: fl.FlowDstPort,
				Protocol: fl.FlowProtocol, LatencyNs: fl.FlowLatencyNs,
				Bytes: fl.FlowBytes, Timestamp: fl.FlowTimestamp,
			})
		}
		return &svcproto.CorrelationQueryResponse{Flows: flows, Total: len(flows)}, nil
	case "service_to_trace":
		traceIDs = s.correlationEngine.GetTracesByService(req.ServiceName)
	case "process_to_trace":
		traceIDs = s.correlationEngine.GetTracesByProcess(req.ProcessName, req.Pid)
	}
	// Convert trace IDs to TraceInfo
	var traces []*svcproto.TraceInfo
	for _, tid := range traceIDs {
		trace, ok := s.otlpReceiver.TraceStore().GetTrace(tid)
		if !ok {
			continue
		}
		traces = append(traces, convertTraceToProto(trace))
	}
	return &svcproto.CorrelationQueryResponse{Traces: traces, Total: len(traces)}, nil
}

// GetOTLPStats 获取 OTLP 接收统计
func (s *Service) GetOTLPStats(ctx context.Context, req *svcproto.HealthCheckRequest) (*svcproto.OTLPIngestStats, error) {
	stats := s.otlpReceiver.Stats().Snapshot()
	return &svcproto.OTLPIngestStats{
		TracesReceived:     stats.TracesReceived,
		SpansReceived:      stats.SpansReceived,
		MetricsReceived:    stats.MetricsReceived,
		DataPointsReceived: stats.DataPointsReceived,
		LogsReceived:       stats.LogsReceived,
		LogRecordsReceived: stats.LogRecordsReceived,
		Errors:             stats.Errors,
	}, nil
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

func (s *Service) otelTracesHandler(w http.ResponseWriter, r *http.Request) {
	// Proxy to trace store search
	writeJSON(w, s.otlpReceiver.TraceStore().SearchSpans(otel.SpanQuery{Limit: 100}))
}
func (s *Service) otelMetricsHandler(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, s.otlpReceiver.MetricsStore().GetAllSeriesNames())
}
func (s *Service) otelLogsHandler(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, s.otlpReceiver.LogStore().Query(otel.LogQuery{Limit: 100}))
}
func (s *Service) otelStatsHandler(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, s.otlpReceiver.Stats().Snapshot())
}
func (s *Service) rcaHandler(w http.ResponseWriter, r *http.Request) {
	traceID := r.URL.Query().Get("trace_id")
	resp, _ := s.GetRootCauseAnalysis(r.Context(), &svcproto.RootCauseRequest{TraceId: traceID})
	writeJSON(w, resp)
}
func (s *Service) correlationHandler(w http.ResponseWriter, r *http.Request) {
	queryType := r.URL.Query().Get("type")
	traceID := r.URL.Query().Get("trace_id")
	serviceName := r.URL.Query().Get("service")
	resp, _ := s.QueryCorrelation(r.Context(), &svcproto.CorrelationQueryRequest{
		QueryType: queryType, TraceId: traceID, ServiceName: serviceName,
	})
	writeJSON(w, resp)
}

// Helper functions
func convertSpanToProto(sp *otel.Span) *svcproto.SpanInfo {
	return &svcproto.SpanInfo{
		TraceId: sp.TraceID, SpanId: sp.SpanID, ParentSpanId: sp.ParentSpanID,
		Name: sp.Name, ServiceName: sp.ServiceName, Kind: sp.Kind,
		StartTime: sp.StartTime, EndTime: sp.EndTime, DurationNs: sp.DurationNs,
		Status: sp.Status, StatusCode: sp.StatusCode, Attributes: sp.Attributes,
	}
}

func convertTraceToProto(t *otel.Trace) *svcproto.TraceInfo {
	var spans []*svcproto.SpanInfo
	for _, sp := range t.Spans {
		spans = append(spans, convertSpanToProto(sp))
	}
	return &svcproto.TraceInfo{
		TraceId:     t.TraceID,
		ServiceName: t.ServiceName,
		SpanCount:   len(t.Spans),
		ErrorCount:  t.ErrorCount,
		DurationNs:  t.DurationNs,
		StartTime:   t.StartTime,
		Spans:       spans,
	}
}

func writeJSON(w http.ResponseWriter, v interface{}) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(v)
}
