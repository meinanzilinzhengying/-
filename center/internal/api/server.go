//go:build linux

// Package api 提供无状态API服务
package api

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"cloud-flow-center/internal/storage"
	"cloud-flow-agent/pkg/logger"
)

// Config API配置
type Config struct {
	Addr           string
	ReadTimeout    time.Duration
	WriteTimeout   time.Duration
	IdleTimeout    time.Duration
	MaxHeaderBytes int
}

// Server API服务器
type Server struct {
	config Config
	store  *storage.StatelessManager
	cache  interface{} // *cache.RedisCache
	log    *logger.Logger
	server *http.Server
}

// NewServer 创建API服务器
func NewServer(config Config, store *storage.StatelessManager, cache interface{}, log *logger.Logger) *Server {
	return &Server{
		config: config,
		store:  store,
		cache:  cache,
		log:    log,
	}
}

// Start 启动服务器
func (s *Server) Start() error {
	mux := http.NewServeMux()

	// 健康检查路由
	mux.HandleFunc("/health", s.handleHealth)
	mux.HandleFunc("/ready", s.handleReady)

	// API路由
	s.registerRoutes(mux)

	s.server = &http.Server{
		Addr:           s.config.Addr,
		Handler:        mux,
		ReadTimeout:    s.config.ReadTimeout,
		WriteTimeout:   s.config.WriteTimeout,
		IdleTimeout:    s.config.IdleTimeout,
		MaxHeaderBytes: s.config.MaxHeaderBytes,
	}

	return s.server.ListenAndServe()
}

// Shutdown 关闭服务器
func (s *Server) Shutdown(ctx context.Context) error {
	return s.server.Shutdown(ctx)
}

// registerRoutes 注册路由
func (s *Server) registerRoutes(mux *http.ServeMux) {
	// 告警API
	mux.HandleFunc("/api/v1/alerts", s.handleAlerts)
	mux.HandleFunc("/api/v1/alerts/", s.handleAlertByID)

	// 租户API
	mux.HandleFunc("/api/v1/tenants", s.handleTenants)
	mux.HandleFunc("/api/v1/tenants/", s.handleTenantByID)

	// 配置API
	mux.HandleFunc("/api/v1/config", s.handleConfig)
	mux.HandleFunc("/api/v1/config/", s.handleConfigByKey)

	// 实例API（多实例协调）
	mux.HandleFunc("/api/v1/instances", s.handleInstances)

	// 指标API
	mux.HandleFunc("/api/v1/metrics", s.handleMetrics)
}

// ==================== 健康检查 ====================

func (s *Server) handleHealth(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]interface{}{
		"status": "ok",
		"service": "cloud-flow-center",
		"timestamp": time.Now().UTC(),
	})
}

func (s *Server) handleReady(w http.ResponseWriter, r *http.Request) {
	// 检查数据库连接
	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()

	if err := s.store.GetStore().Ping(ctx); err != nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]interface{}{
			"status": "not_ready",
			"reason": "database connection failed",
		})
		return
	}

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"status": "ready",
	})
}

// ==================== 告警API ====================

func (s *Server) handleAlerts(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	switch r.Method {
	case http.MethodGet:
		// 获取活跃告警
		tenantID := r.URL.Query().Get("tenant_id")
		alerts, err := s.store.GetActiveAlerts(ctx, tenantID)
		if err != nil {
			writeError(w, http.StatusInternalServerError, err.Error())
			return
		}
		writeJSON(w, http.StatusOK, map[string]interface{}{
			"alerts": alerts,
			"total":  len(alerts),
		})

	case http.MethodPost:
		// 创建告警
		var alert struct {
			ID       string `json:"id"`
			TenantID string `json:"tenant_id"`
			RuleID   string `json:"rule_id"`
			RuleName string `json:"rule_name"`
			Level    string `json:"level"`
			Fingerprint string `json:"fingerprint"`
			Metric   string `json:"metric"`
			Value    float64 `json:"value"`
			Threshold float64 `json:"threshold"`
			FiredAt  time.Time `json:"fired_at"`
		}

		if err := json.NewDecoder(r.Body).Decode(&alert); err != nil {
			writeError(w, http.StatusBadRequest, err.Error())
			return
		}

		a := &storage.Alert{
			ID:          alert.ID,
			TenantID:    alert.TenantID,
			RuleID:      alert.RuleID,
			RuleName:    alert.RuleName,
			Level:       alert.Level,
			Status:      "firing",
			Fingerprint: alert.Fingerprint,
			Metric:      alert.Metric,
			Value:       alert.Value,
			Threshold:   alert.Threshold,
			FiredAt:     alert.FiredAt,
		}

		if err := s.store.CreateAlert(ctx, a); err != nil {
			writeError(w, http.StatusInternalServerError, err.Error())
			return
		}

		writeJSON(w, http.StatusCreated, map[string]interface{}{
			"id": a.ID,
			"status": "created",
		})
	}
}

func (s *Server) handleAlertByID(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	alertID := extractID(r.URL.Path)

	switch r.Method {
	case http.MethodGet:
		alert, err := s.store.GetAlert(ctx, alertID)
		if err != nil {
			writeError(w, http.StatusNotFound, err.Error())
			return
		}
		writeJSON(w, http.StatusOK, alert)

	case http.MethodPut, http.MethodPatch:
		// 更新告警（如解决告警）
		var update struct {
			Status string `json:"status"`
		}

		if err := json.NewDecoder(r.Body).Decode(&update); err != nil {
			writeError(w, http.StatusBadRequest, err.Error())
			return
		}

		if update.Status == "resolved" {
			if err := s.store.ResolveAlert(ctx, alertID); err != nil {
				writeError(w, http.StatusInternalServerError, err.Error())
				return
			}
			writeJSON(w, http.StatusOK, map[string]string{"status": "resolved"})
		} else {
			writeError(w, http.StatusBadRequest, "unknown status")
		}

	case http.MethodDelete:
		writeError(w, http.StatusMethodNotAllowed, "alerts cannot be deleted")
	}
}

// ==================== 租户API ====================

func (s *Server) handleTenants(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	switch r.Method {
	case http.MethodGet:
		tenants, err := s.store.ListTenants(ctx)
		if err != nil {
			writeError(w, http.StatusInternalServerError, err.Error())
			return
		}
		writeJSON(w, http.StatusOK, map[string]interface{}{
			"tenants": tenants,
			"total":   len(tenants),
		})

	case http.MethodPost:
		var req struct {
			Name        string `json:"name"`
			DisplayName string `json:"display_name"`
		}

		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			writeError(w, http.StatusBadRequest, err.Error())
			return
		}

		id, err := s.store.CreateTenant(ctx, req.Name, req.DisplayName)
		if err != nil {
			writeError(w, http.StatusInternalServerError, err.Error())
			return
		}

		writeJSON(w, http.StatusCreated, map[string]interface{}{
			"id": id,
		})
	}
}

func (s *Server) handleTenantByID(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	name := extractID(r.URL.Path)

	switch r.Method {
	case http.MethodGet:
		tenant, err := s.store.GetTenant(ctx, name)
		if err != nil {
			writeError(w, http.StatusNotFound, err.Error())
			return
		}
		writeJSON(w, http.StatusOK, tenant)
	}
}

// ==================== 配置API ====================

func (s *Server) handleConfig(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		// 获取所有配置（简化实现）
		writeJSON(w, http.StatusOK, map[string]string{
			"config": "use /api/v1/config/{key} to get specific config",
		})
	}
}

func (s *Server) handleConfigByKey(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	key := extractID(r.URL.Path)

	switch r.Method {
	case http.MethodGet:
		value, err := s.store.GetConfig(ctx, key)
		if err != nil {
			writeError(w, http.StatusNotFound, err.Error())
			return
		}
		writeJSON(w, http.StatusOK, map[string]string{key: value})

	case http.MethodPut, http.MethodPost:
		var req struct {
			Value string `json:"value"`
		}

		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			writeError(w, http.StatusBadRequest, err.Error())
			return
		}

		if err := s.store.SetConfig(ctx, key, req.Value); err != nil {
			writeError(w, http.StatusInternalServerError, err.Error())
			return
		}

		writeJSON(w, http.StatusOK, map[string]string{"status": "updated"})
	}
}

// ==================== 实例API ====================

func (s *Server) handleInstances(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	switch r.Method {
	case http.MethodGet:
		instances, err := s.store.GetActiveInstances(ctx, "center")
		if err != nil {
			writeError(w, http.StatusInternalServerError, err.Error())
			return
		}
		writeJSON(w, http.StatusOK, map[string]interface{}{
			"instances": instances,
			"total":     len(instances),
		})

	case http.MethodPost:
		// 注册心跳
		if err := s.store.Heartbeat(ctx); err != nil {
			writeError(w, http.StatusInternalServerError, err.Error())
			return
		}
		writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
	}
}

// ==================== 指标API ====================

func (s *Server) handleMetrics(w http.ResponseWriter, r *http.Request) {
	// Prometheus格式指标
	w.Header().Set("Content-Type", "text/plain; version=0.0.4")
	
	// 示例指标
	fmt.Fprintf(w, "# HELP cloudflow_center_up 服务是否运行\n")
	fmt.Fprintf(w, "# TYPE cloudflow_center_up gauge\n")
	fmt.Fprintf(w, "cloudflow_center_up 1\n")
	
	fmt.Fprintf(w, "# HELP cloudflow_center_alerts_active 活跃告警数\n")
	fmt.Fprintf(w, "# TYPE cloudflow_center_alerts_active gauge\n")
	fmt.Fprintf(w, "cloudflow_center_alerts_active 0\n")
}

// ==================== 辅助函数 ====================

func writeJSON(w http.ResponseWriter, status int, v interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(v)
}

func writeError(w http.ResponseWriter, status int, message string) {
	writeJSON(w, status, map[string]string{"error": message})
}

func extractID(path string) string {
	// 从路径中提取ID，如 /api/v1/alerts/123 -> 123
	for i := len(path) - 1; i >= 0; i-- {
		if path[i] == '/' {
			return path[i+1:]
		}
	}
	return path
}
