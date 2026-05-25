package collector

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"sync"
	"time"

	"cloud-flow-agent/pkg/logger"
)

// CollectorStatus 采集器状态
type CollectorStatus string

const (
	StatusRunning   CollectorStatus = "running"
	StatusStopped   CollectorStatus = "stopped"
	StatusDeploying CollectorStatus = "deploying"
	StatusError     CollectorStatus = "error"
	StatusUpdating  CollectorStatus = "updating"
)

// CollectorType 采集器类型
type CollectorType string

const (
	TypeNetwork     CollectorType = "network"
	TypeApplication CollectorType = "application"
	TypeSystem      CollectorType = "system"
	TypeDatabase    CollectorType = "database"
	TypeCloud       CollectorType = "cloud"
	TypeCustom      CollectorType = "custom"
)

// ProbeConfig 探针配置
type ProbeConfig struct {
	ID            string                 `json:"id" yaml:"id"`
	Name          string                 `json:"name" yaml:"name"`
	Type          CollectorType          `json:"type" yaml:"type"`
	Version       string                 `json:"version" yaml:"version"`
	Host          string                 `json:"host" yaml:"host"`
	Port          int                    `json:"port" yaml:"port"`
	Status        CollectorStatus        `json:"status" yaml:"status"`
	Enabled       bool                   `json:"enabled" yaml:"enabled"`
	AutoStart     bool                   `json:"auto_start" yaml:"auto_start"`
	Interval      time.Duration          `json:"interval" yaml:"interval"`
	Timeout       time.Duration          `json:"timeout" yaml:"timeout"`
	Config        map[string]interface{} `json:"config" yaml:"config"`
	Tags          []string               `json:"tags" yaml:"tags"`
	TenantID      string                 `json:"tenant_id" yaml:"tenant_id"`
	CreatedAt     time.Time              `json:"created_at" yaml:"created_at"`
	UpdatedAt     time.Time              `json:"updated_at" yaml:"updated_at"`
	LastStartedAt *time.Time             `json:"last_started_at,omitempty" yaml:"last_started_at,omitempty"`
	LastError     string                 `json:"last_error,omitempty" yaml:"last_error,omitempty"`
	Metrics       *ProbeMetrics          `json:"metrics,omitempty" yaml:"metrics,omitempty"`
}

// ProbeMetrics 探针指标
type ProbeMetrics struct {
	Uptime        time.Duration `json:"uptime"`
	CollectCount  int64         `json:"collect_count"`
	ErrorCount    int64         `json:"error_count"`
	LastCollectAt time.Time     `json:"last_collect_at"`
	CPUUsage      float64       `json:"cpu_usage"`
	MemoryUsage   float64       `json:"memory_usage"`
	NetworkIn     int64         `json:"network_in"`
	NetworkOut    int64         `json:"network_out"`
}

// DeploymentConfig 部署配置
type DeploymentConfig struct {
	TargetHost   string            `json:"target_host"`
	TargetPort   int               `json:"target_port"`
	SSHUser      string            `json:"ssh_user,omitempty"`
	SSHKey       string            `json:"ssh_key,omitempty"`
	SSHPwd       string            `json:"ssh_pwd,omitempty"`
	InstallPath  string            `json:"install_path"`
	DownloadURL  string            `json:"download_url"`
	Version      string            `json:"version"`
	Environment  map[string]string `json:"environment,omitempty"`
	Systemd      bool              `json:"systemd"`
	Docker       bool              `json:"docker"`
	ContainerID  string            `json:"container_id,omitempty"`
}

// CollectorManager 采集器管理器
type CollectorManager struct {
	mu        sync.RWMutex
	probes    map[string]*ProbeConfig
	deployments map[string]*DeploymentTask
	webServer *http.Server
	config    *ManagerConfig
	handlers  map[string]http.HandlerFunc
}

// ManagerConfig 管理器配置
type ManagerConfig struct {
	WebPort         int           `json:"web_port" yaml:"web_port"`
	AuthEnabled     bool          `json:"auth_enabled" yaml:"auth_enabled"`
	JWTSecret       string        `json:"jwt_secret" yaml:"jwt_secret"`
	SessionTimeout  time.Duration `json:"session_timeout" yaml:"session_timeout"`
	MaxProbes       int           `json:"max_probes" yaml:"max_probes"`
	DeployTimeout   time.Duration `json:"deploy_timeout" yaml:"deploy_timeout"`
	HeartbeatInterval time.Duration `json:"heartbeat_interval" yaml:"heartbeat_interval"`
}

// DeploymentTask 部署任务
type DeploymentTask struct {
	ID          string                 `json:"id"`
	ProbeID     string                 `json:"probe_id"`
	Status      string                 `json:"status"`
	Progress    int                    `json:"progress"`
	TargetHost  string                 `json:"target_host"`
	StartedAt   time.Time              `json:"started_at"`
	CompletedAt *time.Time             `json:"completed_at,omitempty"`
	Error       string                 `json:"error,omitempty"`
	Logs        []string               `json:"logs"`
	Config      *DeploymentConfig      `json:"config"`
}

// DefaultManagerConfig 返回默认管理器配置
func DefaultManagerConfig() *ManagerConfig {
	return &ManagerConfig{
		WebPort:           8080,
		AuthEnabled:       true,
		SessionTimeout:    30 * time.Minute,
		MaxProbes:         1000,
		DeployTimeout:     10 * time.Minute,
		HeartbeatInterval: 30 * time.Second,
	}
}

// NewCollectorManager 创建新的采集器管理器
func NewCollectorManager(config *ManagerConfig) *CollectorManager {
	if config == nil {
		config = DefaultManagerConfig()
	}

	cm := &CollectorManager{
		probes:      make(map[string]*ProbeConfig),
		deployments: make(map[string]*DeploymentTask),
		config:      config,
		handlers:    make(map[string]http.HandlerFunc),
	}

	cm.setupRoutes()
	return cm
}

// setupRoutes 设置Web路由
func (cm *CollectorManager) setupRoutes() {
	mux := http.NewServeMux()

	// API路由
	mux.HandleFunc("/api/v1/collectors", cm.handleCollectors)
	mux.HandleFunc("/api/v1/collectors/", cm.handleCollectorDetail)
	mux.HandleFunc("/api/v1/collectors/deploy", cm.handleDeploy)
	mux.HandleFunc("/api/v1/collectors/batch", cm.handleBatchOperation)
	mux.HandleFunc("/api/v1/collectors/types", cm.handleCollectorTypes)
	mux.HandleFunc("/api/v1/collectors/stats", cm.handleCollectorStats)

	// WebSocket用于实时监控
	mux.HandleFunc("/ws/collectors", cm.handleWebSocket)

	// 静态文件服务
	mux.Handle("/", http.FileServer(http.Dir("./web")))

	cm.webServer = &http.Server{
		Addr:    fmt.Sprintf(":%d", cm.config.WebPort),
		Handler: cm.corsMiddleware(mux),
	}
}

// corsMiddleware CORS中间件
func (cm *CollectorManager) corsMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization")

		if r.Method == "OPTIONS" {
			w.WriteHeader(http.StatusOK)
			return
		}

		next.ServeHTTP(w, r)
	})
}

// Start 启动Web管理界面
func (cm *CollectorManager) Start() error {
	logger.Info("Starting collector management web server on port %d", cm.config.WebPort)
	go func() {
		if err := cm.webServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			logger.Error("Web server error: %v", err)
		}
	}()
	return nil
}

// Stop 停止Web管理界面
func (cm *CollectorManager) Stop(ctx context.Context) error {
	logger.Info("Stopping collector management web server")
	return cm.webServer.Shutdown(ctx)
}

// ==================== API Handlers ====================

// handleCollectors 处理采集器列表请求
func (cm *CollectorManager) handleCollectors(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		cm.listCollectors(w, r)
	case http.MethodPost:
		cm.createCollector(w, r)
	default:
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
	}
}

// handleCollectorDetail 处理单个采集器详情
func (cm *CollectorManager) handleCollectorDetail(w http.ResponseWriter, r *http.Request) {
	probeID := r.URL.Path[len("/api/v1/collectors/"):]
	if probeID == "" {
		http.Error(w, "Probe ID required", http.StatusBadRequest)
		return
	}

	switch r.Method {
	case http.MethodGet:
		cm.getCollector(w, r, probeID)
	case http.MethodPut:
		cm.updateCollector(w, r, probeID)
	case http.MethodDelete:
		cm.deleteCollector(w, r, probeID)
	default:
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
	}
}

// listCollectors 获取采集器列表
func (cm *CollectorManager) listCollectors(w http.ResponseWriter, r *http.Request) {
	cm.mu.RLock()
	defer cm.mu.RUnlock()

	// 支持过滤参数
	probeType := r.URL.Query().Get("type")
	status := r.URL.Query().Get("status")
	tenantID := r.URL.Query().Get("tenant_id")

	var result []*ProbeConfig
	for _, probe := range cm.probes {
		if probeType != "" && string(probe.Type) != probeType {
			continue
		}
		if status != "" && string(probe.Status) != status {
			continue
		}
		if tenantID != "" && probe.TenantID != tenantID {
			continue
		}
		result = append(result, probe)
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"code":    0,
		"message": "success",
		"data":    result,
		"total":   len(result),
	})
}

// createCollector 创建新采集器
func (cm *CollectorManager) createCollector(w http.ResponseWriter, r *http.Request) {
	var probe ProbeConfig
	if err := json.NewDecoder(r.Body).Decode(&probe); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	// 验证必填字段
	if probe.Name == "" || probe.Host == "" {
		http.Error(w, "Name and Host are required", http.StatusBadRequest)
		return
	}

	cm.mu.Lock()
	defer cm.mu.Unlock()

	// 检查是否超过最大数量限制
	if len(cm.probes) >= cm.config.MaxProbes {
		http.Error(w, "Maximum number of probes reached", http.StatusForbidden)
		return
	}

	// 生成ID
	if probe.ID == "" {
		probe.ID = fmt.Sprintf("probe_%d", time.Now().UnixNano())
	}

	probe.CreatedAt = time.Now()
	probe.UpdatedAt = time.Now()
	probe.Status = StatusStopped

	if probe.AutoStart {
		probe.Status = StatusRunning
		now := time.Now()
		probe.LastStartedAt = &now
	}

	cm.probes[probe.ID] = &probe

	logger.Info("Created collector: %s (%s)", probe.Name, probe.ID)

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(map[string]interface{}{
		"code":    0,
		"message": "Collector created successfully",
		"data":    probe,
	})
}

// getCollector 获取采集器详情
func (cm *CollectorManager) getCollector(w http.ResponseWriter, r *http.Request, probeID string) {
	cm.mu.RLock()
	defer cm.mu.RUnlock()

	probe, exists := cm.probes[probeID]
	if !exists {
		http.Error(w, "Collector not found", http.StatusNotFound)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"code":    0,
		"message": "success",
		"data":    probe,
	})
}

// updateCollector 更新采集器配置
func (cm *CollectorManager) updateCollector(w http.ResponseWriter, r *http.Request, probeID string) {
	cm.mu.Lock()
	defer cm.mu.Unlock()

	probe, exists := cm.probes[probeID]
	if !exists {
		http.Error(w, "Collector not found", http.StatusNotFound)
		return
	}

	var updates map[string]interface{}
	if err := json.NewDecoder(r.Body).Decode(&updates); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	// 应用更新
	if name, ok := updates["name"].(string); ok {
		probe.Name = name
	}
	if host, ok := updates["host"].(string); ok {
		probe.Host = host
	}
	if port, ok := updates["port"].(float64); ok {
		probe.Port = int(port)
	}
	if enabled, ok := updates["enabled"].(bool); ok {
		probe.Enabled = enabled
	}
	if interval, ok := updates["interval"].(float64); ok {
		probe.Interval = time.Duration(interval) * time.Second
	}
	if config, ok := updates["config"].(map[string]interface{}); ok {
		probe.Config = config
	}
	if tags, ok := updates["tags"].([]interface{}); ok {
		probe.Tags = make([]string, len(tags))
		for i, tag := range tags {
			probe.Tags[i] = tag.(string)
		}
	}

	probe.UpdatedAt = time.Now()

	logger.Info("Updated collector: %s (%s)", probe.Name, probe.ID)

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"code":    0,
		"message": "Collector updated successfully",
		"data":    probe,
	})
}

// deleteCollector 删除采集器
func (cm *CollectorManager) deleteCollector(w http.ResponseWriter, r *http.Request, probeID string) {
	cm.mu.Lock()
	defer cm.mu.Unlock()

	probe, exists := cm.probes[probeID]
	if !exists {
		http.Error(w, "Collector not found", http.StatusNotFound)
		return
	}

	// 如果正在运行，先停止
	if probe.Status == StatusRunning {
		probe.Status = StatusStopped
	}

	delete(cm.probes, probeID)

	logger.Info("Deleted collector: %s (%s)", probe.Name, probeID)

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"code":    0,
		"message": "Collector deleted successfully",
	})
}

// handleDeploy 处理部署请求
func (cm *CollectorManager) handleDeploy(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var deployConfig DeploymentConfig
	if err := json.NewDecoder(r.Body).Decode(&deployConfig); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	// 验证必填字段
	if deployConfig.TargetHost == "" || deployConfig.InstallPath == "" {
		http.Error(w, "TargetHost and InstallPath are required", http.StatusBadRequest)
		return
	}

	// 创建部署任务
	task := &DeploymentTask{
		ID:         fmt.Sprintf("deploy_%d", time.Now().UnixNano()),
		Status:     "pending",
		Progress:   0,
		TargetHost: deployConfig.TargetHost,
		StartedAt:  time.Now(),
		Logs:       []string{},
		Config:     &deployConfig,
	}

	cm.mu.Lock()
	cm.deployments[task.ID] = task
	cm.mu.Unlock()

	// 异步执行部署
	go cm.executeDeployment(task)

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusAccepted)
	json.NewEncoder(w).Encode(map[string]interface{}{
		"code":    0,
		"message": "Deployment task created",
		"data":    task,
	})
}

// executeDeployment 执行部署任务
func (cm *CollectorManager) executeDeployment(task *DeploymentTask) {
	cm.mu.Lock()
	task.Status = "running"
	task.Logs = append(task.Logs, fmt.Sprintf("[%s] Starting deployment to %s", time.Now().Format("2006-01-02 15:04:05"), task.Config.TargetHost))
	cm.mu.Unlock()

	// 模拟部署步骤
	steps := []string{
		"Connecting to target host...",
		"Checking system requirements...",
		"Downloading probe package...",
		"Installing dependencies...",
		"Configuring probe...",
		"Starting probe service...",
		"Verifying installation...",
	}

	for i, step := range steps {
		time.Sleep(2 * time.Second) // 模拟耗时操作

		cm.mu.Lock()
		task.Progress = (i + 1) * 100 / len(steps)
		task.Logs = append(task.Logs, fmt.Sprintf("[%s] %s", time.Now().Format("2006-01-02 15:04:05"), step))
		cm.mu.Unlock()
	}

	// 部署完成
	now := time.Now()
	cm.mu.Lock()
	task.Status = "completed"
	task.Progress = 100
	task.CompletedAt = &now
	task.Logs = append(task.Logs, fmt.Sprintf("[%s] Deployment completed successfully", time.Now().Format("2006-01-02 15:04:05")))

	// 创建对应的探针配置
	probe := &ProbeConfig{
		ID:        fmt.Sprintf("probe_%d", time.Now().UnixNano()),
		Name:      fmt.Sprintf("Probe-%s", task.Config.TargetHost),
		Type:      TypeNetwork,
		Host:      task.Config.TargetHost,
		Port:      task.Config.TargetPort,
		Status:    StatusRunning,
		Enabled:   true,
		Interval:  30 * time.Second,
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}
	probe.LastStartedAt = &now
	cm.probes[probe.ID] = probe
	task.ProbeID = probe.ID

	cm.mu.Unlock()

	logger.Info("Deployment completed: %s -> %s", task.ID, task.Config.TargetHost)
}

// handleBatchOperation 批量操作
func (cm *CollectorManager) handleBatchOperation(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req struct {
		Operation string   `json:"operation"`
		ProbeIDs  []string `json:"probe_ids"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	cm.mu.Lock()
	defer cm.mu.Unlock()

	var successCount, failCount int
	var errors []string

	for _, probeID := range req.ProbeIDs {
		probe, exists := cm.probes[probeID]
		if !exists {
			failCount++
			errors = append(errors, fmt.Sprintf("Probe %s not found", probeID))
			continue
		}

		switch req.Operation {
		case "start":
			if probe.Status == StatusStopped {
				probe.Status = StatusRunning
				now := time.Now()
				probe.LastStartedAt = &now
				successCount++
			}
		case "stop":
			if probe.Status == StatusRunning {
				probe.Status = StatusStopped
				successCount++
			}
		case "restart":
			probe.Status = StatusRunning
			now := time.Now()
			probe.LastStartedAt = &now
			successCount++
		case "enable":
			probe.Enabled = true
			successCount++
		case "disable":
			probe.Enabled = false
			if probe.Status == StatusRunning {
				probe.Status = StatusStopped
			}
			successCount++
		case "delete":
			delete(cm.probes, probeID)
			successCount++
		default:
			failCount++
			errors = append(errors, fmt.Sprintf("Unknown operation: %s", req.Operation))
		}

		probe.UpdatedAt = time.Now()
	}

	logger.Info("Batch operation %s: success=%d, fail=%d", req.Operation, successCount, failCount)

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"code":    0,
		"message": "Batch operation completed",
		"data": map[string]interface{}{
			"operation":     req.Operation,
			"success_count": successCount,
			"fail_count":    failCount,
			"errors":        errors,
		},
	})
}

// handleCollectorTypes 获取采集器类型列表
func (cm *CollectorManager) handleCollectorTypes(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	types := []map[string]interface{}{
		{
			"type":        TypeNetwork,
			"name":        "网络采集器",
			"description": "采集网络流量、延迟、丢包等指标",
			"icon":        "network",
		},
		{
			"type":        TypeApplication,
			"name":        "应用采集器",
			"description": "采集应用性能、请求量、错误率等指标",
			"icon":        "application",
		},
		{
			"type":        TypeSystem,
			"name":        "系统采集器",
			"description": "采集CPU、内存、磁盘、网络等系统指标",
			"icon":        "system",
		},
		{
			"type":        TypeDatabase,
			"name":        "数据库采集器",
			"description": "采集数据库连接数、查询性能、慢查询等指标",
			"icon":        "database",
		},
		{
			"type":        TypeCloud,
			"name":        "云资源采集器",
			"description": "采集云平台资源使用情况和费用指标",
			"icon":        "cloud",
		},
		{
			"type":        TypeCustom,
			"name":        "自定义采集器",
			"description": "支持自定义脚本和采集逻辑",
			"icon":        "custom",
		},
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"code":    0,
		"message": "success",
		"data":    types,
	})
}

// handleCollectorStats 获取采集器统计信息
func (cm *CollectorManager) handleCollectorStats(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	cm.mu.RLock()
	defer cm.mu.RUnlock()

	stats := map[string]interface{}{
		"total":       len(cm.probes),
		"running":     0,
		"stopped":     0,
		"error":       0,
		"deploying":   0,
		"by_type":     make(map[string]int),
		"by_tenant":   make(map[string]int),
	}

	byType := stats["by_type"].(map[string]int)
	byTenant := stats["by_tenant"].(map[string]int)

	for _, probe := range cm.probes {
		switch probe.Status {
		case StatusRunning:
			stats["running"] = stats["running"].(int) + 1
		case StatusStopped:
			stats["stopped"] = stats["stopped"].(int) + 1
		case StatusError:
			stats["error"] = stats["error"].(int) + 1
		case StatusDeploying:
			stats["deploying"] = stats["deploying"].(int) + 1
		}

		byType[string(probe.Type)]++
		if probe.TenantID != "" {
			byTenant[probe.TenantID]++
		}
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"code":    0,
		"message": "success",
		"data":    stats,
	})
}

// handleWebSocket WebSocket处理（用于实时监控）
func (cm *CollectorManager) handleWebSocket(w http.ResponseWriter, r *http.Request) {
	// 简化实现，实际应该使用gorilla/websocket
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"code":    0,
		"message": "WebSocket endpoint ready",
	})
}

// ==================== Management Methods ====================

// StartCollector 启动采集器
func (cm *CollectorManager) StartCollector(probeID string) error {
	cm.mu.Lock()
	defer cm.mu.Unlock()

	probe, exists := cm.probes[probeID]
	if !exists {
		return fmt.Errorf("collector not found: %s", probeID)
	}

	if probe.Status == StatusRunning {
		return fmt.Errorf("collector already running: %s", probeID)
	}

	probe.Status = StatusRunning
	now := time.Now()
	probe.LastStartedAt = &now
	probe.UpdatedAt = now

	logger.Info("Started collector: %s (%s)", probe.Name, probeID)
	return nil
}

// StopCollector 停止采集器
func (cm *CollectorManager) StopCollector(probeID string) error {
	cm.mu.Lock()
	defer cm.mu.Unlock()

	probe, exists := cm.probes[probeID]
	if !exists {
		return fmt.Errorf("collector not found: %s", probeID)
	}

	if probe.Status == StatusStopped {
		return fmt.Errorf("collector already stopped: %s", probeID)
	}

	probe.Status = StatusStopped
	probe.UpdatedAt = time.Now()

	logger.Info("Stopped collector: %s (%s)", probe.Name, probeID)
	return nil
}

// GetCollectorStatus 获取采集器状态
func (cm *CollectorManager) GetCollectorStatus(probeID string) (*ProbeConfig, error) {
	cm.mu.RLock()
	defer cm.mu.RUnlock()

	probe, exists := cm.probes[probeID]
	if !exists {
		return nil, fmt.Errorf("collector not found: %s", probeID)
	}

	return probe, nil
}

// ListCollectorsByTenant 按租户列出采集器
func (cm *CollectorManager) ListCollectorsByTenant(tenantID string) []*ProbeConfig {
	cm.mu.RLock()
	defer cm.mu.RUnlock()

	var result []*ProbeConfig
	for _, probe := range cm.probes {
		if probe.TenantID == tenantID {
			result = append(result, probe)
		}
	}
	return result
}

// UpdateProbeMetrics 更新探针指标
func (cm *CollectorManager) UpdateProbeMetrics(probeID string, metrics *ProbeMetrics) error {
	cm.mu.Lock()
	defer cm.mu.Unlock()

	probe, exists := cm.probes[probeID]
	if !exists {
		return fmt.Errorf("collector not found: %s", probeID)
	}

	probe.Metrics = metrics
	return nil
}

// GetDeploymentStatus 获取部署任务状态
func (cm *CollectorManager) GetDeploymentStatus(taskID string) (*DeploymentTask, error) {
	cm.mu.RLock()
	defer cm.mu.RUnlock()

	task, exists := cm.deployments[taskID]
	if !exists {
		return nil, fmt.Errorf("deployment task not found: %s", taskID)
	}

	return task, nil
}
