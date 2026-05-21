// Package http 提供 HTTP 服务功能
package http

import (
	"context"
	"encoding/json"
	"net/http"
	"time"

	"google.golang.org/grpc/connectivity"

	"cloud-flow-agent/internal/collector"
	"cloud-flow-agent/internal/ebpfcollector"
	"cloud-flow-agent/internal/grpcclient"
	"cloud-flow-agent/pkg/logger"
)

// Version 版本号，由 main 包在启动时设置
var Version = "dev"

// ClientGetter 客户端获取接口
type ClientGetter interface {
	Get() *grpcclient.Client
}

// HealthHandler 健康检查处理器
type HealthHandler struct {
	clientGetter  ClientGetter
	collector     *collector.Collector
	ebpfCollector *ebpfcollector.Collector
	logger        *logger.Logger
	startTime     time.Time
}

// NewHealthHandler 创建健康检查处理器
func NewHealthHandler(clientGetter ClientGetter, collector *collector.Collector, ebpfCollector *ebpfcollector.Collector, log *logger.Logger) *HealthHandler {
	return &HealthHandler{
		clientGetter:  clientGetter,
		collector:     collector,
		ebpfCollector: ebpfCollector,
		logger:        log,
		startTime:     time.Now(),
	}
}

// HealthResponse 健康检查响应
type HealthResponse struct {
	Status             string            `json:"status"`
	Timestamp          time.Time         `json:"timestamp"`
	Uptime             string            `json:"uptime"`
	EdgeConnected      bool              `json:"edge_connected"`
	EBPFAvailable      bool              `json:"ebpf_available"`
	TCPMetricsEnabled  bool              `json:"tcp_metrics_enabled"`
	HTTPMetricsEnabled bool              `json:"http_metrics_enabled"`
	HTTPFullEnabled    bool              `json:"http_full_enabled"`
	DNSFullEnabled     bool              `json:"dns_full_enabled"`
	MySQLFullEnabled   bool              `json:"mysql_full_enabled"`
	Version            string            `json:"version"`
}

// HandleHealth 处理健康检查请求
func (h *HealthHandler) HandleHealth(w http.ResponseWriter, r *http.Request) {
	// 检查与Edge节点的连接
	edgeConnected := false
	if h.clientGetter != nil {
		client := h.clientGetter.Get()
		if client != nil {
			state := client.GetState()
			edgeConnected = state == connectivity.Ready
		}
	}

	// 检查EBPF采集器状态
	ebpfAvailable := h.ebpfCollector != nil

	// 检查TCP指标采集状态
	tcpMetricsEnabled := false
	if h.ebpfCollector != nil {
		tcpMetricsEnabled = h.ebpfCollector.IsTCPMetricsAvailable()
	}

	// 检查HTTP指标采集状态
	httpMetricsEnabled := false
	if h.ebpfCollector != nil {
		httpMetricsEnabled = h.ebpfCollector.IsHTTPMetricsAvailable()
	}

	// 检查协议全字段解析状态
	httpFullEnabled := false
	dnsFullEnabled := false
	mysqlFullEnabled := false
	if h.ebpfCollector != nil {
		httpFullEnabled = h.ebpfCollector.IsHTTPFullAvailable()
		dnsFullEnabled = h.ebpfCollector.IsDNSFullAvailable()
		mysqlFullEnabled = h.ebpfCollector.IsMySQLFullAvailable()
	}

	// 构建健康检查响应
	response := HealthResponse{
		Status:             "healthy",
		Timestamp:          time.Now(),
		Uptime:             time.Since(h.startTime).String(),
		EdgeConnected:      edgeConnected,
		EBPFAvailable:      ebpfAvailable,
		TCPMetricsEnabled:  tcpMetricsEnabled,
		HTTPMetricsEnabled: httpMetricsEnabled,
		HTTPFullEnabled:    httpFullEnabled,
		DNSFullEnabled:     dnsFullEnabled,
		MySQLFullEnabled:   mysqlFullEnabled,
		Version:            Version,
	}

	// 设置响应头
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)

	// 发送响应
	if err := json.NewEncoder(w).Encode(response); err != nil {
		h.logger.Warnf("发送健康检查响应失败: %v", err)
	}
}

// Server HTTP 服务器实例
type Server struct {
	server *http.Server
}

// StartHealthServer 启动健康检查 HTTP 服务器
func StartHealthServer(addr string, handler *HealthHandler) *Server {
	mux := http.NewServeMux()
	mux.HandleFunc("/health", handler.HandleHealth)

	// 也可以添加 /ready 端点
	mux.HandleFunc("/ready", handler.HandleHealth)

	// 也可以添加 /live 端点
	mux.HandleFunc("/live", handler.HandleHealth)

	server := &http.Server{
		Addr:    addr,
		Handler: mux, // 使用独立的 ServeMux，避免注册到全局 DefaultServeMux
	}

	s := &Server{server: server}

	// 启动服务器
	go func() {
		if err := s.server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			handler.logger.Warnf("健康检查 HTTP 服务器错误: %v", err)
		}
	}()

	return s
}

// Shutdown 优雅关闭 HTTP 服务器
func (s *Server) Shutdown(ctx context.Context) error {
	return s.server.Shutdown(ctx)
}
