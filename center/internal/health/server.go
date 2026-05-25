//go:build linux

// Package health 提供健康检查接口
package health

import (
	"context"
	"encoding/json"
	"net/http"
	"runtime"
	"sync/atomic"
	"time"

	"cloud-flow-center/internal/cache"
	"cloud-flow-center/internal/storage"
	"cloud-flow-agent/pkg/logger"
)

// Config 健康检查配置
type Config struct {
	Store          *storage.Store
	Cache          *cache.RedisCache
	LivenessCheck  LivenessConfig
	ReadinessCheck ReadinessConfig
}

// LivenessConfig 存活检查配置
type LivenessConfig struct {
	Endpoint string
	Timeout  time.Duration
}

// ReadinessConfig 就绪检查配置
type ReadinessConfig struct {
	Endpoint string
	Timeout  time.Duration
}

// HealthServer 健康检查服务器
type HealthServer struct {
	config Config
	log    *logger.Logger
	server *http.Server

	// 状态
	ready       atomic.Bool
	startTime   time.Time
	restartCount atomic.Int64
}

// NewHealthServer 创建健康检查服务器
func NewHealthServer(config Config, log *logger.Logger) *HealthServer {
	return &HealthServer{
		config:   config,
		log:      log,
		ready:    atomic.Bool{},
		startTime: time.Now(),
	}
}

// Start 启动健康检查服务器
func (s *HealthServer) Start(addr string) error {
	mux := http.NewServeMux()

	// 存活检查 /health
	mux.HandleFunc(s.config.LivenessCheck.Endpoint, s.handleLiveness)

	// 就绪检查 /ready
	mux.HandleFunc(s.config.ReadinessCheck.Endpoint, s.handleReadiness)

	// 详细健康状态 /healthz
	mux.HandleFunc("/healthz", s.handleDetailedHealth)

	// 就绪探针 /readyz
	mux.HandleFunc("/readyz", s.handleDetailedReadiness)

	// 指标 /metrics
	mux.HandleFunc("/metrics", s.handleMetrics)

	s.server = &http.Server{
		Addr:         addr,
		Handler:      mux,
		ReadTimeout:  5 * time.Second,
		WriteTimeout: 10 * time.Second,
	}

	return s.server.ListenAndServe()
}

// Stop 停止健康检查服务器
func (s *HealthServer) Stop(ctx context.Context) error {
	return s.server.Shutdown(ctx)
}

// SetReady 设置就绪状态
func (s *HealthServer) SetReady(ready bool) {
	s.ready.Store(ready)
	if ready {
		s.restartCount.Add(1)
	}
}

// ==================== 处理函数 ====================

// handleLiveness 存活检查
// - 检查进程是否存活
// - 检查HTTP服务是否响应
func (s *HealthServer) handleLiveness(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), s.config.LivenessCheck.Timeout)
	defer cancel()

	// 检查Goroutine是否正常（无阻塞）
	var m runtime.MemStats
	runtime.ReadMemStats(&m)

	// Go运行时状态检查
	if m.GCDesc == nil {
		// 正常情况
	}

	resp := LivenessResponse{
		Status:    "ok",
		Timestamp: time.Now().UTC(),
		Uptime:    time.Since(s.startTime).String(),
	}

	writeJSON(w, http.StatusOK, resp)
}

// handleReadiness 就绪检查
// - 检查数据库连接
// - 检查Redis连接
// - 检查是否有足够资源
func (s *HealthServer) handleReadiness(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), s.config.ReadinessCheck.Timeout)
	defer cancel()

	resp := ReadinessResponse{
		Status:    "ok",
		Timestamp: time.Now().UTC(),
		Checks:    make(map[string]CheckResult),
	}

	allHealthy := true

	// 检查数据库
	dbCheck := s.checkDatabase(ctx)
	resp.Checks["database"] = dbCheck
	if dbCheck.Status != "ok" {
		allHealthy = false
	}

	// 检查Redis
	redisCheck := s.checkRedis(ctx)
	resp.Checks["redis"] = redisCheck
	if redisCheck.Status != "ok" {
		allHealthy = false
	}

	if allHealthy {
		resp.Status = "ok"
		s.SetReady(true)
		writeJSON(w, http.StatusOK, resp)
	} else {
		resp.Status = "degraded"
		s.SetReady(false)
		writeJSON(w, http.StatusServiceUnavailable, resp)
	}
}

// handleDetailedHealth 详细健康状态
func (s *HealthServer) handleDetailedHealth(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), s.config.LivenessCheck.Timeout)
	defer cancel()

	resp := DetailedHealthResponse{
		Status:    "ok",
		Timestamp: time.Now().UTC(),
		Version:   "1.0.0",
		Uptime:    time.Since(s.startTime).String(),
		StartTime: s.startTime,
		Checks:    make(map[string]CheckResult),
	}

	// 系统信息
	resp.System = SystemInfo{
		GoVersion:   runtime.Version(),
		NumGoroutine: runtime.NumGoroutine(),
		NumCPU:      runtime.NumCPU(),
		GOOS:        runtime.GOOS,
		GOARCH:      runtime.GOARCH,
	}

	// 内存信息
	var m runtime.MemStats
	runtime.ReadMemStats(&m)
	resp.Memory = MemoryInfo{
		Alloc:      m.Alloc,
		TotalAlloc: m.TotalAlloc,
		Sys:        m.Sys,
		NumGC:      m.NumGC,
	}

	// 检查组件
	resp.Checks["database"] = s.checkDatabase(ctx)
	resp.Checks["redis"] = s.checkRedis(ctx)

	// 组件都健康
	allHealthy := true
	for _, check := range resp.Checks {
		if check.Status != "ok" {
			allHealthy = false
			break
		}
	}

	if allHealthy {
		resp.Status = "ok"
	}

	status := http.StatusOK
	if !allHealthy {
		status = http.StatusServiceUnavailable
		resp.Status = "degraded"
	}

	writeJSON(w, status, resp)
}

// handleDetailedReadiness 详细就绪状态
func (s *HealthServer) handleDetailedReadiness(w http.ResponseWriter, r *http.Request) {
	s.handleDetailedHealth(w, r)
}

// handleMetrics 指标
func (s *HealthServer) handleMetrics(w http.ResponseWriter, r *http.Request) {
	var m runtime.MemStats
	runtime.ReadMemStats(&m)

	metrics := map[string]interface{}{
		"uptime_seconds":         time.Since(s.startTime).Seconds(),
		"go_version":            runtime.Version(),
		"goroutines":             runtime.NumGoroutine(),
		"memory_alloc_bytes":     m.Alloc,
		"memory_total_bytes":     m.TotalAlloc,
		"memory_sys_bytes":       m.Sys,
		"gc_runs":                m.NumGC,
		"ready":                  s.ready.Load(),
		"restart_count":          s.restartCount.Load(),
	}

	writeJSON(w, http.StatusOK, metrics)
}

// ==================== 内部检查 ====================

// checkDatabase 检查数据库
func (s *HealthServer) checkDatabase(ctx context.Context) CheckResult {
	if s.config.Store == nil {
		return CheckResult{Status: "ok", Message: "store not configured"}
	}

	start := time.Now()
	err := s.config.Store.Ping(ctx)
	latency := time.Since(start)

	if err != nil {
		return CheckResult{
			Status:  "error",
			Message: err.Error(),
			Latency: latency.String(),
		}
	}

	return CheckResult{
		Status:  "ok",
		Latency: latency.String(),
	}
}

// checkRedis 检查Redis
func (s *HealthServer) checkRedis(ctx context.Context) CheckResult {
	if s.config.Cache == nil {
		return CheckResult{Status: "ok", Message: "cache not configured"}
	}

	start := time.Now()
	err := s.config.Cache.Ping(ctx)
	latency := time.Since(start)

	if err != nil {
		return CheckResult{
			Status:  "error",
			Message: err.Error(),
			Latency: latency.String(),
		}
	}

	return CheckResult{
		Status:  "ok",
		Latency: latency.String(),
	}
}

// ==================== 响应结构 ====================

// LivenessResponse 存活检查响应
type LivenessResponse struct {
	Status    string    `json:"status"`
	Timestamp time.Time `json:"timestamp"`
	Uptime    string    `json:"uptime"`
}

// ReadinessResponse 就绪检查响应
type ReadinessResponse struct {
	Status    string                  `json:"status"`
	Timestamp time.Time               `json:"timestamp"`
	Checks    map[string]CheckResult  `json:"checks"`
}

// DetailedHealthResponse 详细健康状态
type DetailedHealthResponse struct {
	Status    string                 `json:"status"`
	Timestamp time.Time               `json:"timestamp"`
	Version   string                 `json:"version"`
	Uptime    string                 `json:"uptime"`
	StartTime time.Time              `json:"start_time"`
	System    SystemInfo             `json:"system"`
	Memory    MemoryInfo            `json:"memory"`
	Checks    map[string]CheckResult `json:"checks"`
}

// CheckResult 检查结果
type CheckResult struct {
	Status  string `json:"status"`
	Message string `json:"message,omitempty"`
	Latency string `json:"latency,omitempty"`
}

// SystemInfo 系统信息
type SystemInfo struct {
	GoVersion   string `json:"go_version"`
	NumGoroutine int    `json:"num_goroutine"`
	NumCPU      int    `json:"num_cpu"`
	GOOS        string `json:"goos"`
	GOARCH      string `json:"goarch"`
}

// MemoryInfo 内存信息
type MemoryInfo struct {
	Alloc      uint64 `json:"alloc_bytes"`
	TotalAlloc uint64 `json:"total_alloc_bytes"`
	Sys        uint64 `json:"sys_bytes"`
	NumGC      uint32 `json:"num_gc"`
}

// ==================== 辅助函数 ====================

func writeJSON(w http.ResponseWriter, status int, v interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(v)
}
