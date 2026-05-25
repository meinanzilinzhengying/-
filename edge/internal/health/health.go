//go:build linux

// Package health 提供gRPC健康检查实现
package health

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"sync"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/health"
	"google.golang.org/grpc/health/grpc_health_v1"
	"google.golang.org/grpc/status"

	"cloud-flow-edge/edge/internal/cache"
	"cloud-flow-edge/edge/internal/config"
)

// HealthServer 健康检查服务器
type HealthServer struct {
	config *config.HealthConfig
	cache  *cache.RedisCache
	
	// gRPC健康检查
	grpcServer   *grpc.Server
	healthServer *health.Server
	
	// HTTP健康检查
	httpServer *http.Server
	
	// 状态
	mu          sync.RWMutex
	ready       bool
	startTime   time.Time
	checkers    map[string]HealthChecker
}

// HealthChecker 健康检查器接口
type HealthChecker interface {
	Check(ctx context.Context) error
	Name() string
}

// NewHealthServer 创建健康检查服务器
func NewHealthServer(cfg *config.HealthConfig, redisCache *cache.RedisCache) *HealthServer {
	s := &HealthServer{
		config:    cfg,
		cache:     redisCache,
		checkers:  make(map[string]HealthChecker),
		startTime: time.Now(),
	}
	
	return s
}

// RegisterChecker 注册健康检查器
func (s *HealthServer) RegisterChecker(checker HealthChecker) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.checkers[checker.Name()] = checker
}

// StartGRPC 启动gRPC健康检查服务
func (s *HealthServer) StartGRPC(addr string) error {
	if !s.config.Enabled {
		return nil
	}

	// 创建gRPC服务器
	s.grpcServer = grpc.NewServer()
	
	// 注册健康检查服务
	s.healthServer = health.NewServer()
	grpc_health_v1.RegisterHealthServer(s.grpcServer, s.healthServer)
	
	// 设置初始状态
	s.healthServer.SetServingStatus("", grpc_health_v1.HealthCheckResponse_NOT_SERVING)
	
	// 监听
	lis, err := net.Listen("tcp", addr)
	if err != nil {
		return fmt.Errorf("failed to listen on %s: %w", addr, err)
	}
	
	go func() {
		if err := s.grpcServer.Serve(lis); err != nil {
			// 记录错误但不返回，因为这是后台goroutine
		}
	}()
	
	return nil
}

// StartHTTP 启动HTTP健康检查服务
func (s *HealthServer) StartHTTP(addr string) error {
	if !s.config.Enabled {
		return nil
	}

	mux := http.NewServeMux()
	
	// Kubernetes风格的探针
	mux.HandleFunc("/healthz", s.handleLiveness)      // 存活检查
	mux.HandleFunc("/readyz", s.handleReadiness)      // 就绪检查
	mux.HandleFunc("/healthz/ready", s.handleReadiness)
	mux.HandleFunc("/healthz/live", s.handleLiveness)
	
	// gRPC健康检查HTTP桥接
	mux.HandleFunc("/grpc-health", s.handleGRPCHealth)
	
	// 详细健康状态
	mux.HandleFunc("/health", s.handleDetailedHealth)
	
	// Prometheus指标
	mux.HandleFunc("/metrics", s.handleMetrics)
	
	s.httpServer = &http.Server{
		Addr:         addr,
		Handler:      mux,
		ReadTimeout:  5 * time.Second,
		WriteTimeout: 10 * time.Second,
	}
	
	go func() {
		if err := s.httpServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			// 记录错误
		}
	}()
	
	return nil
}

// SetReady 设置就绪状态
func (s *HealthServer) SetReady(ready bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.ready = ready
	
	if s.healthServer != nil {
		if ready {
			s.healthServer.SetServingStatus("", grpc_health_v1.HealthCheckResponse_SERVING)
		} else {
			s.healthServer.SetServingStatus("", grpc_health_v1.HealthCheckResponse_NOT_SERVING)
		}
	}
}

// IsReady 检查是否就绪
func (s *HealthServer) IsReady() bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.ready
}

// Stop 停止健康检查服务
func (s *HealthServer) Stop(ctx context.Context) error {
	if s.grpcServer != nil {
		s.grpcServer.GracefulStop()
	}
	
	if s.httpServer != nil {
		return s.httpServer.Shutdown(ctx)
	}
	
	return nil
}

// ==================== HTTP处理函数 ====================

// LivenessResponse 存活检查响应
type LivenessResponse struct {
	Status    string    `json:"status"`
	Timestamp time.Time `json:"timestamp"`
	Uptime    string    `json:"uptime"`
}

// ReadinessResponse 就绪检查响应
type ReadinessResponse struct {
	Status    string                 `json:"status"`
	Timestamp time.Time              `json:"timestamp"`
	Checks    map[string]CheckResult `json:"checks"`
}

// CheckResult 检查结果
type CheckResult struct {
	Status  string `json:"status"`
	Message string `json:"message,omitempty"`
	Latency string `json:"latency,omitempty"`
}

// handleLiveness 存活检查
// Kubernetes livenessProbe 使用此端点
func (s *HealthServer) handleLiveness(w http.ResponseWriter, r *http.Request) {
	resp := LivenessResponse{
		Status:    "ok",
		Timestamp: time.Now().UTC(),
		Uptime:    time.Since(s.startTime).String(),
	}
	
	writeJSON(w, http.StatusOK, resp)
}

// handleReadiness 就绪检查
// Kubernetes readinessProbe 使用此端点
func (s *HealthServer) handleReadiness(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()
	
	resp := ReadinessResponse{
		Status:    "ok",
		Timestamp: time.Now().UTC(),
		Checks:    make(map[string]CheckResult),
	}
	
	allHealthy := true
	
	// 检查Redis连接
	if s.cache != nil {
		start := time.Now()
		err := s.cache.Ping(ctx)
		latency := time.Since(start)
		
		if err != nil {
			resp.Checks["redis"] = CheckResult{
				Status:  "error",
				Message: err.Error(),
				Latency: latency.String(),
			}
			allHealthy = false
		} else {
			resp.Checks["redis"] = CheckResult{
				Status:  "ok",
				Latency: latency.String(),
			}
		}
	}
	
	// 执行自定义检查器
	s.mu.RLock()
	checkers := make(map[string]HealthChecker)
	for name, checker := range s.checkers {
		checkers[name] = checker
	}
	s.mu.RUnlock()
	
	for name, checker := range checkers {
		start := time.Now()
		err := checker.Check(ctx)
		latency := time.Since(start)
		
		if err != nil {
			resp.Checks[name] = CheckResult{
				Status:  "error",
				Message: err.Error(),
				Latency: latency.String(),
			}
			allHealthy = false
		} else {
			resp.Checks[name] = CheckResult{
				Status:  "ok",
				Latency: latency.String(),
			}
		}
	}
	
	// 检查自身就绪状态
	if !s.IsReady() {
		resp.Checks["self"] = CheckResult{
			Status:  "error",
			Message: "service not ready",
		}
		allHealthy = false
	} else {
		resp.Checks["self"] = CheckResult{
			Status: "ok",
		}
	}
	
	if allHealthy {
		resp.Status = "ok"
		writeJSON(w, http.StatusOK, resp)
	} else {
		resp.Status = "not_ready"
		writeJSON(w, http.StatusServiceUnavailable, resp)
	}
}

// handleDetailedHealth 详细健康状态
func (s *HealthServer) handleDetailedHealth(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()
	
	type DetailedResponse struct {
		Status    string                 `json:"status"`
		Timestamp time.Time              `json:"timestamp"`
		Version   string                 `json:"version"`
		Uptime    string                 `json:"uptime"`
		Checks    map[string]CheckResult `json:"checks"`
		Services  map[string]interface{} `json:"services"`
	}
	
	resp := DetailedResponse{
		Status:    "ok",
		Timestamp: time.Now().UTC(),
		Version:   "1.0.0",
		Uptime:    time.Since(s.startTime).String(),
		Checks:    make(map[string]CheckResult),
		Services:  make(map[string]interface{}),
	}
	
	// 检查Redis
	if s.cache != nil {
		start := time.Now()
		err := s.cache.Ping(ctx)
		latency := time.Since(start)
		
		if err != nil {
			resp.Checks["redis"] = CheckResult{Status: "error", Message: err.Error(), Latency: latency.String()}
			resp.Status = "degraded"
		} else {
			resp.Checks["redis"] = CheckResult{Status: "ok", Latency: latency.String()}
		}
	}
	
	writeJSON(w, http.StatusOK, resp)
}

// handleGRPCHealth gRPC健康检查HTTP桥接
func (s *HealthServer) handleGRPCHealth(w http.ResponseWriter, r *http.Request) {
	if s.healthServer == nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]string{
			"status": "NOT_SERVING",
			"error":  "gRPC health server not started",
		})
		return
	}
	
	// 检查gRPC服务状态
	status := s.healthServer.GetServingStatus("")
	
	switch status {
	case grpc_health_v1.HealthCheckResponse_SERVING:
		writeJSON(w, http.StatusOK, map[string]string{"status": "SERVING"})
	case grpc_health_v1.HealthCheckResponse_NOT_SERVING:
		writeJSON(w, http.StatusServiceUnavailable, map[string]string{"status": "NOT_SERVING"})
	default:
		writeJSON(w, http.StatusServiceUnavailable, map[string]string{"status": "UNKNOWN"})
	}
}

// handleMetrics 指标端点
func (s *HealthServer) handleMetrics(w http.ResponseWriter, r *http.Request) {
	metrics := map[string]interface{}{
		"uptime_seconds": time.Since(s.startTime).Seconds(),
		"ready":          s.IsReady(),
		"timestamp":      time.Now().UTC(),
	}
	
	writeJSON(w, http.StatusOK, metrics)
}

// ==================== gRPC健康检查实现 ====================

// Check 实现grpc_health_v1.HealthServer接口
func (s *HealthServer) Check(ctx context.Context, req *grpc_health_v1.HealthCheckRequest) (*grpc_health_v1.HealthCheckResponse, error) {
	// 执行就绪检查
	if !s.IsReady() {
		return &grpc_health_v1.HealthCheckResponse{
			Status: grpc_health_v1.HealthCheckResponse_NOT_SERVING,
		}, nil
	}
	
	// 检查Redis
	if s.cache != nil {
		if err := s.cache.Ping(ctx); err != nil {
			return &grpc_health_v1.HealthCheckResponse{
				Status: grpc_health_v1.HealthCheckResponse_NOT_SERVING,
			}, nil
		}
	}
	
	return &grpc_health_v1.HealthCheckResponse{
		Status: grpc_health_v1.HealthCheckResponse_SERVING,
	}, nil
}

// Watch 实现grpc_health_v1.HealthServer接口（流式健康检查）
func (s *HealthServer) Watch(req *grpc_health_v1.HealthCheckRequest, stream grpc_health_v1.Health_WatchServer) error {
	// 简化实现：只发送当前状态
	status := grpc_health_v1.HealthCheckResponse_NOT_SERVING
	if s.IsReady() {
		status = grpc_health_v1.HealthCheckResponse_SERVING
	}
	
	return stream.Send(&grpc_health_v1.HealthCheckResponse{
		Status: status,
	})
}

// ==================== 辅助函数 ====================

func writeJSON(w http.ResponseWriter, status int, v interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	
	// 忽略编码错误
	_ = json.NewEncoder(w).Encode(v)
}

// ==================== 内置检查器 ====================

// RedisHealthChecker Redis健康检查器
type RedisHealthChecker struct {
	cache *cache.RedisCache
}

// NewRedisHealthChecker 创建Redis健康检查器
func NewRedisHealthChecker(cache *cache.RedisCache) *RedisHealthChecker {
	return &RedisHealthChecker{cache: cache}
}

// Name 返回检查器名称
func (c *RedisHealthChecker) Name() string {
	return "redis"
}

// Check 执行检查
func (c *RedisHealthChecker) Check(ctx context.Context) error {
	if c.cache == nil {
		return fmt.Errorf("redis cache not initialized")
	}
	return c.cache.Ping(ctx)
}

// CenterHealthChecker Center连接检查器
type CenterHealthChecker struct {
	centerAddr string
}

// NewCenterHealthChecker 创建Center健康检查器
func NewCenterHealthChecker(centerAddr string) *CenterHealthChecker {
	return &CenterHealthChecker{centerAddr: centerAddr}
}

// Name 返回检查器名称
func (c *CenterHealthChecker) Name() string {
	return "center"
}

// Check 执行检查
func (c *CenterHealthChecker) Check(ctx context.Context) error {
	// 简化实现：检查地址是否配置
	if c.centerAddr == "" {
		return fmt.Errorf("center address not configured")
	}
	return nil
}
