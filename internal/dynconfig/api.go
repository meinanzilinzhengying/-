// Package dynconfig Web管理API
// RESTful接口，支持配置查询/修改/重载/快照
package dynconfig

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"sync"
	"time"
)

// ============================================================
// API Server
// ============================================================

// APIServer Web管理API服务器
type APIServer struct {
	engine    *Engine
	server    *http.Server
	mu        sync.RWMutex
	
	// 认证
	authToken string
	authEnabled bool
	
	// 限流
	rateLimit int
	requests  map[string]int
	lastReset time.Time
}

// APIServerConfig API服务器配置
type APIServerConfig struct {
	ListenAddr   string `yaml:"listen_addr" json:"listen_addr"`     // 监听地址
	AuthEnabled  bool   `yaml:"auth_enabled" json:"auth_enabled"`   // 启用认证
	AuthToken    string `yaml:"auth_token" json:"auth_token"`       // 认证Token
	RateLimit    int    `yaml:"rate_limit" json:"rate_limit"`       // 每分钟请求限制
	ReadTimeout  int    `yaml:"read_timeout" json:"read_timeout"`   // 读超时（秒）
	WriteTimeout int    `yaml:"write_timeout" json:"write_timeout"` // 写超时（秒）
}

// DefaultAPIServerConfig 默认配置
func DefaultAPIServerConfig() *APIServerConfig {
	return &APIServerConfig{
		ListenAddr:   ":9090",
		AuthEnabled:  true,
		AuthToken:    "",
		RateLimit:    60,
		ReadTimeout:  10,
		WriteTimeout: 10,
	}
}

// NewAPIServer 创建API服务器
func NewAPIServer(engine *Engine, config *APIServerConfig) *APIServer {
	if config == nil {
		config = DefaultAPIServerConfig()
	}
	
	s := &APIServer{
		engine:      engine,
		authToken:  config.AuthToken,
		authEnabled: config.AuthEnabled,
		rateLimit:  config.RateLimit,
		requests:   make(map[string]int),
		lastReset:  time.Now(),
	}
	
	mux := http.NewServeMux()
	
	// 配置管理路由
	mux.HandleFunc("/api/v1/config", s.handleConfig)
	mux.HandleFunc("/api/v1/config/", s.handleConfigPath)
	mux.HandleFunc("/api/v1/config/reload", s.handleReload)
	mux.HandleFunc("/api/v1/config/snapshot", s.handleSnapshot)
	mux.HandleFunc("/api/v1/config/rollback", s.handleRollback)
	mux.HandleFunc("/api/v1/config/export", s.handleExport)
	mux.HandleFunc("/api/v1/config/import", s.handleImport)
	mux.HandleFunc("/api/v1/config/history", s.handleHistory)
	
	// 采样率管理
	mux.HandleFunc("/api/v1/sample-rate/", s.handleSampleRate)
	
	// 指标管理
	mux.HandleFunc("/api/v1/metrics/", s.handleMetrics)
	
	// 健康检查
	mux.HandleFunc("/api/v1/health", s.handleHealth)
	
	s.server = &http.Server{
		Addr:         config.ListenAddr,
		Handler:      mux,
		ReadTimeout:  time.Duration(config.ReadTimeout) * time.Second,
		WriteTimeout: time.Duration(config.WriteTimeout) * time.Second,
	}
	
	return s
}

// Start 启动API服务器
func (s *APIServer) Start() error {
	go func() {
		if err := s.server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			// 记录错误
		}
	}()
	return nil
}

// Stop 停止API服务器
func (s *APIServer) Stop() error {
	return s.server.Close()
}

// ============================================================
// 中间件
// ============================================================

func (s *APIServer) middleware(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		// 认证检查
		if s.authEnabled {
			token := r.Header.Get("Authorization")
			if token == "" {
				token = r.URL.Query().Get("token")
			}
			
			if token != s.authToken {
				s.writeError(w, http.StatusUnauthorized, "未授权")
				return
			}
		}
		
		// 限流检查
		if !s.checkRateLimit(r.RemoteAddr) {
			s.writeError(w, http.StatusTooManyRequests, "请求过于频繁")
			return
		}
		
		next(w, r)
	}
}

func (s *APIServer) checkRateLimit(addr string) bool {
	now := time.Now()
	if now.Sub(s.lastReset) > time.Minute {
		s.requests = make(map[string]int)
		s.lastReset = now
	}
	
	s.requests[addr]++
	return s.requests[addr] <= s.rateLimit
}

// ============================================================
// 路由处理
// ============================================================

// GET /api/v1/config - 获取全部配置
// PUT /api/v1/config - 批量更新配置
func (s *APIServer) handleConfig(w http.ResponseWriter, r *http.Request) {
	s.middleware(func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodGet:
			s.handleGetConfig(w, r)
		case http.MethodPut:
			s.handlePutConfig(w, r)
		default:
			s.writeError(w, http.StatusMethodNotAllowed, "方法不允许")
		}
	})(w, r)
}

func (s *APIServer) handleGetConfig(w http.ResponseWriter, r *http.Request) {
	data, err := s.engine.Export()
	if err != nil {
		s.writeError(w, http.StatusInternalServerError, "导出配置失败")
		return
	}
	
	w.Header().Set("Content-Type", "application/json")
	w.Write(data)
}

func (s *APIServer) handlePutConfig(w http.ResponseWriter, r *http.Request) {
	var updates map[string]interface{}
	if err := json.NewDecoder(r.Body).Decode(&updates); err != nil {
		s.writeError(w, http.StatusBadRequest, "无效的JSON")
		return
	}
	
	for path, value := range updates {
		if err := s.engine.Set(path, value); err != nil {
			s.writeError(w, http.StatusBadRequest, fmt.Sprintf("设置 %s 失败: %s", path, err))
			return
		}
	}
	
	s.writeJSON(w, http.StatusOK, map[string]interface{}{
		"status":  "ok",
		"updated": len(updates),
	})
}

// GET/PUT /api/v1/config/{path} - 获取/设置单个配置
func (s *APIServer) handleConfigPath(w http.ResponseWriter, r *http.Request) {
	s.middleware(func(w http.ResponseWriter, r *http.Request) {
		path := strings.TrimPrefix(r.URL.Path, "/api/v1/config/")
		path = strings.TrimSuffix(path, "/")
		path = strings.ReplaceAll(path, "/", ".")
		
		if path == "" || path == "reload" || path == "snapshot" || path == "rollback" ||
			path == "export" || path == "import" || path == "history" {
			s.writeError(w, http.StatusBadRequest, "无效的配置路径")
			return
		}
		
		switch r.Method {
		case http.MethodGet:
			val, err := s.engine.Get(path)
			if err != nil {
				s.writeError(w, http.StatusNotFound, err.Error())
				return
			}
			s.writeJSON(w, http.StatusOK, map[string]interface{}{
				"path":  path,
				"value": val,
			})
			
		case http.MethodPut:
			var body struct {
				Value interface{} `json:"value"`
			}
			if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
				s.writeError(w, http.StatusBadRequest, "无效的JSON")
				return
			}
			
			if err := s.engine.Set(path, body.Value); err != nil {
				s.writeError(w, http.StatusBadRequest, err.Error())
				return
			}
			
			s.writeJSON(w, http.StatusOK, map[string]interface{}{
				"status": "ok",
				"path":   path,
				"value":  body.Value,
			})
			
		default:
			s.writeError(w, http.StatusMethodNotAllowed, "方法不允许")
		}
	})(w, r)
}

// POST /api/v1/config/reload - 重新加载配置文件
func (s *APIServer) handleReload(w http.ResponseWriter, r *http.Request) {
	s.middleware(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			s.writeError(w, http.StatusMethodNotAllowed, "方法不允许")
			return
		}
		
		if err := s.engine.Load(); err != nil {
			s.writeError(w, http.StatusInternalServerError, "重新加载失败: "+err.Error())
			return
		}
		
		s.writeJSON(w, http.StatusOK, map[string]string{
			"status": "ok",
			"message": "配置已重新加载",
		})
	})(w, r)
}

// GET /api/v1/config/snapshot - 创建配置快照
func (s *APIServer) handleSnapshot(w http.ResponseWriter, r *http.Request) {
	s.middleware(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			s.writeError(w, http.StatusMethodNotAllowed, "方法不允许")
			return
		}
		
		snapshot, err := s.engine.Snapshot()
		if err != nil {
			s.writeError(w, http.StatusInternalServerError, "创建快照失败")
			return
		}
		
		s.writeJSON(w, http.StatusOK, map[string]interface{}{
			"status":   "ok",
			"snapshot": snapshot,
		})
	})(w, r)
}

// POST /api/v1/config/rollback - 回滚到快照
func (s *APIServer) handleRollback(w http.ResponseWriter, r *http.Request) {
	s.middleware(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			s.writeError(w, http.StatusMethodNotAllowed, "方法不允许")
			return
		}
		
		var body map[string]interface{}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			s.writeError(w, http.StatusBadRequest, "无效的JSON")
			return
		}
		
		if err := s.engine.Rollback(body); err != nil {
			s.writeError(w, http.StatusInternalServerError, "回滚失败")
			return
		}
		
		s.writeJSON(w, http.StatusOK, map[string]string{
			"status":  "ok",
			"message": "配置已回滚",
		})
	})(w, r)
}

// GET /api/v1/config/export - 导出配置
func (s *APIServer) handleExport(w http.ResponseWriter, r *http.Request) {
	s.middleware(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			s.writeError(w, http.StatusMethodNotAllowed, "方法不允许")
			return
		}
		
		data, err := s.engine.Export()
		if err != nil {
			s.writeError(w, http.StatusInternalServerError, "导出失败")
			return
		}
		
		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("Content-Disposition", "attachment; filename=config.json")
		w.Write(data)
	})(w, r)
}

// POST /api/v1/config/import - 导入配置
func (s *APIServer) handleImport(w http.ResponseWriter, r *http.Request) {
	s.middleware(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			s.writeError(w, http.StatusMethodNotAllowed, "方法不允许")
			return
		}
		
		data, err := io.ReadAll(r.Body)
		if err != nil {
			s.writeError(w, http.StatusBadRequest, "读取请求体失败")
			return
		}
		
		if err := s.engine.Import(data); err != nil {
			s.writeError(w, http.StatusBadRequest, "导入失败: "+err.Error())
			return
		}
		
		s.writeJSON(w, http.StatusOK, map[string]string{
			"status":  "ok",
			"message": "配置已导入",
		})
	})(w, r)
}

// GET /api/v1/config/history - 获取变更历史
func (s *APIServer) handleHistory(w http.ResponseWriter, r *http.Request) {
	s.middleware(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			s.writeError(w, http.StatusMethodNotAllowed, "方法不允许")
			return
		}
		
		history := s.engine.GetHistory()
		s.writeJSON(w, http.StatusOK, map[string]interface{}{
			"status":  "ok",
			"history": history,
		})
	})(w, r)
}

// GET/PUT /api/v1/sample-rate/{module} - 采样率管理
func (s *APIServer) handleSampleRate(w http.ResponseWriter, r *http.Request) {
	s.middleware(func(w http.ResponseWriter, r *http.Request) {
		module := strings.TrimPrefix(r.URL.Path, "/api/v1/sample-rate/")
		module = strings.TrimSuffix(module, "/")
		
		switch r.Method {
		case http.MethodGet:
			rate := s.engine.GetSampleRate(module)
			s.writeJSON(w, http.StatusOK, map[string]interface{}{
				"module": module,
				"rate":   rate,
			})
			
		case http.MethodPut:
			var body struct {
				Rate int `json:"rate"`
			}
			if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
				s.writeError(w, http.StatusBadRequest, "无效的JSON")
				return
			}
			
			if err := s.engine.AdjustSampleRate(module, body.Rate); err != nil {
				s.writeError(w, http.StatusBadRequest, err.Error())
				return
			}
			
			s.writeJSON(w, http.StatusOK, map[string]interface{}{
				"status": "ok",
				"module": module,
				"rate":   body.Rate,
			})
			
		default:
			s.writeError(w, http.StatusMethodNotAllowed, "方法不允许")
		}
	})(w, r)
}

// GET /api/v1/metrics/{module} - 指标配置
func (s *APIServer) handleMetrics(w http.ResponseWriter, r *http.Request) {
	s.middleware(func(w http.ResponseWriter, r *http.Request) {
		module := strings.TrimPrefix(r.URL.Path, "/api/v1/metrics/")
		module = strings.TrimSuffix(module, "/")
		
		if r.Method != http.MethodGet {
			s.writeError(w, http.StatusMethodNotAllowed, "方法不允许")
			return
		}
		
		configs := s.engine.GetMetricConfig(module)
		s.writeJSON(w, http.StatusOK, map[string]interface{}{
			"module": module,
			"metrics": configs,
		})
	})(w, r)
}

// GET /api/v1/health - 健康检查
func (s *APIServer) handleHealth(w http.ResponseWriter, r *http.Request) {
	s.writeJSON(w, http.StatusOK, map[string]interface{}{
		"status":    "healthy",
		"timestamp": time.Now().Unix(),
	})
}

// ============================================================
// 响应工具
// ============================================================

func (s *APIServer) writeJSON(w http.ResponseWriter, status int, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(data)
}

func (s *APIServer) writeError(w http.ResponseWriter, status int, message string) {
	s.writeJSON(w, status, map[string]interface{}{
		"error": message,
	})
}
