// Package asset 资产下钻API
// 提供资产指标查询与下钻功能
package asset

import (
	"encoding/json"
	"net/http"
	"strconv"
	"strings"
	"time"
)

// AssetAPIConfig API配置
type AssetAPIConfig struct {
	Enabled      bool   `json:"enabled" yaml:"enabled"`
	ListenAddr   string `json:"listen_addr" yaml:"listen_addr"`
	AuthEnabled  bool   `json:"auth_enabled" yaml:"auth_enabled"`
	AuthToken    string `json:"auth_token" yaml:"auth_token"`
	RateLimit    int    `json:"rate_limit" yaml:"rate_limit"`
	ReadTimeout  int    `json:"read_timeout" yaml:"read_timeout"`
	WriteTimeout int    `json:"write_timeout" yaml:"write_timeout"`
}

// AssetAPI 资产API服务
type AssetAPI struct {
	config      *AssetAPIConfig
	collector   *MetricsCollector
	aggregator  *MetricsAggregator
	storage     *AssetStorage
	pageManager *AssetPageManager
	pageHandler *AssetPageHandler
	server      *http.Server
}

// NewAssetAPI 创建资产API服务
func NewAssetAPI(config *AssetAPIConfig, collector *MetricsCollector, aggregator *MetricsAggregator, storage *AssetStorage) *AssetAPI {
	// 创建资产页面管理器
	pageManager := NewAssetPageManager(collector, aggregator, storage)
	pageHandler := NewAssetPageHandler(pageManager)
	
	return &AssetAPI{
		config:      config,
		collector:   collector,
		aggregator:  aggregator,
		storage:     storage,
		pageManager: pageManager,
		pageHandler: pageHandler,
	}
}

// Start 启动API服务
func (a *AssetAPI) Start() error {
	if !a.config.Enabled {
		return nil
	}
	
	mux := http.NewServeMux()
	
	// 资产列表与详情
	mux.HandleFunc("/api/v1/assets", a.authMiddleware(a.handleAssets))
	mux.HandleFunc("/api/v1/assets/", a.authMiddleware(a.handleAsset))
	
	// 资产指标查询
	mux.HandleFunc("/api/v1/assets/metrics/", a.authMiddleware(a.handleAssetMetrics))
	mux.HandleFunc("/api/v1/assets/metrics/history/", a.authMiddleware(a.handleAssetMetricsHistory))
	mux.HandleFunc("/api/v1/assets/metrics/latest/", a.authMiddleware(a.handleAssetMetricsLatest))
	mux.HandleFunc("/api/v1/assets/metrics/aggregated/", a.authMiddleware(a.handleAssetMetricsAggregated))
	
	// 资产对比与排名
	mux.HandleFunc("/api/v1/assets/compare", a.authMiddleware(a.handleAssetCompare))
	mux.HandleFunc("/api/v1/assets/rank", a.authMiddleware(a.handleAssetRank))
	
	// 资产下钻
	mux.HandleFunc("/api/v1/assets/drilldown/", a.authMiddleware(a.handleAssetDrilldown))
	
	// 资产页面API（双视图、筛选、下钻）
	a.pageHandler.RegisterRoutes(mux)
	
	// 健康检查
	mux.HandleFunc("/api/v1/assets/health", a.handleHealth)
	
	a.server = &http.Server{
		Addr:         a.config.ListenAddr,
		Handler:      mux,
		ReadTimeout:  time.Duration(a.config.ReadTimeout) * time.Second,
		WriteTimeout: time.Duration(a.config.WriteTimeout) * time.Second,
	}
	
	go func() {
		if err := a.server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		}
	}()
	
	return nil
}

// Stop 停止API服务
func (a *AssetAPI) Stop() error {
	if a.server != nil {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		return a.server.Shutdown(ctx)
	}
	return nil
}

// authMiddleware 认证中间件
func (a *AssetAPI) authMiddleware(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if a.config.AuthEnabled {
			token := r.Header.Get("Authorization")
			if token == "" {
				token = r.URL.Query().Get("token")
			}
			
			if token == "" || token != "Bearer "+a.config.AuthToken {
				a.respondError(w, http.StatusUnauthorized, "unauthorized")
				return
			}
		}
		
		next(w, r)
	}
}

// handleAssets 处理资产列表请求
func (a *AssetAPI) handleAssets(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		a.listAssets(w, r)
	default:
		a.respondError(w, http.StatusMethodNotAllowed, "method not allowed")
	}
}

// listAssets 列出资产
func (a *AssetAPI) listAssets(w http.ResponseWriter, r *http.Request) {
	assetType := r.URL.Query().Get("type")
	limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
	if limit <= 0 {
		limit = 100
	}
	
	var assets []string
	var err error
	
	if a.storage != nil && a.storage.config.Enabled {
		assets, err = a.storage.GetAssetList(AssetType(assetType), limit)
	} else if a.collector != nil {
		// 从采集器获取
		metrics := a.collector.GetAllMetrics()
		for id, m := range metrics {
			if assetType == "" || string(m.AssetType) == assetType {
				assets = append(assets, id)
			}
		}
	}
	
	if err != nil {
		a.respondError(w, http.StatusInternalServerError, err.Error())
		return
	}
	
	a.respondJSON(w, http.StatusOK, map[string]interface{}{
		"assets": assets,
		"count":  len(assets),
	})
}

// handleAsset 处理单个资产请求
func (a *AssetAPI) handleAsset(w http.ResponseWriter, r *http.Request) {
	assetID := strings.TrimPrefix(r.URL.Path, "/api/v1/assets/")
	
	switch r.Method {
	case http.MethodGet:
		a.getAsset(w, r, assetID)
	default:
		a.respondError(w, http.StatusMethodNotAllowed, "method not allowed")
	}
}

// getAsset 获取资产详情
func (a *AssetAPI) getAsset(w http.ResponseWriter, r *http.Request, assetID string) {
	var metrics *AssetMetrics
	var err error
	
	if a.storage != nil && a.storage.config.Enabled {
		metrics, err = a.storage.GetLatestMetrics(assetID)
	} else if a.collector != nil {
		metrics = a.collector.GetMetrics(assetID)
	}
	
	if err != nil || metrics == nil {
		a.respondError(w, http.StatusNotFound, "asset not found")
		return
	}
	
	a.respondJSON(w, http.StatusOK, metrics)
}

// handleAssetMetrics 处理资产指标请求
func (a *AssetAPI) handleAssetMetrics(w http.ResponseWriter, r *http.Request) {
	assetID := strings.TrimPrefix(r.URL.Path, "/api/v1/assets/metrics/")
	
	switch r.Method {
	case http.MethodGet:
		a.getAssetMetrics(w, r, assetID)
	default:
		a.respondError(w, http.StatusMethodNotAllowed, "method not allowed")
	}
}

// getAssetMetrics 获取资产指标
func (a *AssetAPI) getAssetMetrics(w http.ResponseWriter, r *http.Request, assetID string) {
	// 解析时间范围
	start, end := a.parseTimeRange(r)
	limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
	if limit <= 0 {
		limit = 100
	}
	
	var metrics []*AssetMetrics
	var err error
	
	if a.storage != nil && a.storage.config.Enabled {
		metrics, err = a.storage.GetMetrics(assetID, start, end, limit)
	} else if a.collector != nil {
		metrics = a.collector.GetMetricsHistory(assetID, limit)
	}
	
	if err != nil {
		a.respondError(w, http.StatusInternalServerError, err.Error())
		return
	}
	
	a.respondJSON(w, http.StatusOK, map[string]interface{}{
		"asset_id": assetID,
		"start":    start,
		"end":      end,
		"metrics":  metrics,
		"count":    len(metrics),
	})
}

// handleAssetMetricsHistory 处理资产历史指标请求
func (a *AssetAPI) handleAssetMetricsHistory(w http.ResponseWriter, r *http.Request) {
	assetID := strings.TrimPrefix(r.URL.Path, "/api/v1/assets/metrics/history/")
	
	switch r.Method {
	case http.MethodGet:
		a.getAssetMetricsHistory(w, r, assetID)
	default:
		a.respondError(w, http.StatusMethodNotAllowed, "method not allowed")
	}
}

// getAssetMetricsHistory 获取资产历史指标
func (a *AssetAPI) getAssetMetricsHistory(w http.ResponseWriter, r *http.Request, assetID string) {
	// 解析时间范围
	start, end := a.parseTimeRange(r)
	granularity := r.URL.Query().Get("granularity")
	if granularity == "" {
		granularity = "1m"
	}
	
	points, _ := strconv.Atoi(r.URL.Query().Get("points"))
	if points <= 0 {
		points = 100
	}
	
	var metrics []*AssetMetrics
	var err error
	
	// 使用降采样查询
	if a.storage != nil && a.storage.config.Enabled {
		metrics, err = a.storage.QueryMetricsWithDownsampling(assetID, start, end, points)
	} else if a.collector != nil {
		history := a.collector.GetMetricsHistory(assetID, 0)
		// 过滤时间范围
		for _, m := range history {
			if m.Timestamp.After(start) && m.Timestamp.Before(end) {
				metrics = append(metrics, m)
			}
		}
	}
	
	if err != nil {
		a.respondError(w, http.StatusInternalServerError, err.Error())
		return
	}
	
	a.respondJSON(w, http.StatusOK, map[string]interface{}{
		"asset_id":    assetID,
		"start":       start,
		"end":         end,
		"granularity": granularity,
		"points":      points,
		"metrics":     metrics,
		"count":       len(metrics),
	})
}

// handleAssetMetricsLatest 处理资产最新指标请求
func (a *AssetAPI) handleAssetMetricsLatest(w http.ResponseWriter, r *http.Request) {
	assetID := strings.TrimPrefix(r.URL.Path, "/api/v1/assets/metrics/latest/")
	
	switch r.Method {
	case http.MethodGet:
		a.getAssetMetricsLatest(w, r, assetID)
	default:
		a.respondError(w, http.StatusMethodNotAllowed, "method not allowed")
	}
}

// getAssetMetricsLatest 获取资产最新指标
func (a *AssetAPI) getAssetMetricsLatest(w http.ResponseWriter, r *http.Request, assetID string) {
	var metrics *AssetMetrics
	var err error
	
	if a.storage != nil && a.storage.config.Enabled {
		metrics, err = a.storage.GetLatestMetrics(assetID)
	} else if a.collector != nil {
		metrics = a.collector.GetMetrics(assetID)
	}
	
	if err != nil || metrics == nil {
		a.respondError(w, http.StatusNotFound, "asset not found")
		return
	}
	
	a.respondJSON(w, http.StatusOK, metrics)
}

// handleAssetMetricsAggregated 处理资产聚合指标请求
func (a *AssetAPI) handleAssetMetricsAggregated(w http.ResponseWriter, r *http.Request) {
	assetID := strings.TrimPrefix(r.URL.Path, "/api/v1/assets/metrics/aggregated/")
	
	switch r.Method {
	case http.MethodGet:
		a.getAssetMetricsAggregated(w, r, assetID)
	default:
		a.respondError(w, http.StatusMethodNotAllowed, "method not allowed")
	}
}

// getAssetMetricsAggregated 获取资产聚合指标
func (a *AssetAPI) getAssetMetricsAggregated(w http.ResponseWriter, r *http.Request, assetID string) {
	// 解析时间范围
	start, end := a.parseTimeRange(r)
	granularity := r.URL.Query().Get("granularity")
	if granularity == "" {
		granularity = "1m"
	}
	
	var metrics []*AggregatedMetrics
	var err error
	
	if a.storage != nil && a.storage.config.Enabled {
		metrics, err = a.storage.GetAggregatedMetrics(assetID, TimeGranularity(granularity), start, end)
	} else if a.aggregator != nil {
		metrics = a.aggregator.QueryAggregated(assetID, TimeGranularity(granularity), start, end)
	}
	
	if err != nil {
		a.respondError(w, http.StatusInternalServerError, err.Error())
		return
	}
	
	a.respondJSON(w, http.StatusOK, map[string]interface{}{
		"asset_id":    assetID,
		"start":       start,
		"end":         end,
		"granularity": granularity,
		"metrics":     metrics,
		"count":       len(metrics),
	})
}

// handleAssetCompare 处理资产对比请求
func (a *AssetAPI) handleAssetCompare(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		a.compareAssets(w, r)
	default:
		a.respondError(w, http.StatusMethodNotAllowed, "method not allowed")
	}
}

// compareAssets 对比多个资产
func (a *AssetAPI) compareAssets(w http.ResponseWriter, r *http.Request) {
	// 解析资产ID列表
	assetIDs := r.URL.Query()["asset_id"]
	if len(assetIDs) == 0 {
		a.respondError(w, http.StatusBadRequest, "asset_id is required")
		return
	}
	
	metricType := r.URL.Query().Get("metric_type")
	
	comparison := make(map[string]interface{})
	
	for _, assetID := range assetIDs {
		var metrics *AssetMetrics
		
		if a.storage != nil && a.storage.config.Enabled {
			metrics, _ = a.storage.GetLatestMetrics(assetID)
		} else if a.collector != nil {
			metrics = a.collector.GetMetrics(assetID)
		}
		
		if metrics != nil {
			comparison[assetID] = a.extractMetricByType(metrics, metricType)
		}
	}
	
	a.respondJSON(w, http.StatusOK, map[string]interface{}{
		"metric_type": metricType,
		"comparison":  comparison,
	})
}

// handleAssetRank 处理资产排名请求
func (a *AssetAPI) handleAssetRank(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		rankAssets(w, r)
	default:
		a.respondError(w, http.StatusMethodNotAllowed, "method not allowed")
	}
}

// rankAssets 资产排名
func rankAssets(w http.ResponseWriter, r *http.Request) {
	// 解析参数
	metricType := r.URL.Query().Get("metric")
	if metricType == "" {
		a.respondError(w, http.StatusBadRequest, "metric is required")
		return
	}
	
	assetType := r.URL.Query().Get("type")
	order := r.URL.Query().Get("order")
	if order == "" {
		order = "desc"
	}
	limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
	if limit <= 0 {
		limit = 10
	}
	
	// 这里需要实现排名逻辑
	// 简化处理，返回空结果
	a.respondJSON(w, http.StatusOK, map[string]interface{}{
		"metric": metricType,
		"type":   assetType,
		"order":  order,
		"limit":  limit,
		"rank":   []interface{}{},
	})
}

// handleAssetDrilldown 处理资产下钻请求
func (a *AssetAPI) handleAssetDrilldown(w http.ResponseWriter, r *http.Request) {
	assetID := strings.TrimPrefix(r.URL.Path, "/api/v1/assets/drilldown/")
	
	switch r.Method {
	case http.MethodGet:
		a.drilldownAsset(w, r, assetID)
	default:
		a.respondError(w, http.StatusMethodNotAllowed, "method not allowed")
	}
}

// drilldownAsset 资产下钻
func (a *AssetAPI) drilldownAsset(w http.ResponseWriter, r *http.Request, assetID string) {
	// 解析维度
	dimension := r.URL.Query().Get("dimension")
	if dimension == "" {
		dimension = "all"
	}
	
	// 解析时间范围
	start, end := a.parseTimeRange(r)
	
	var result interface{}
	
	switch dimension {
	case "network":
		result = a.drilldownNetwork(assetID, start, end)
	case "application":
		result = a.drilldownApplication(assetID, start, end)
	case "system":
		result = a.drilldownSystem(assetID, start, end)
	case "process":
		result = a.drilldownProcess(assetID, start, end)
	case "interface":
		result = a.drilldownInterface(assetID, start, end)
	default:
		result = a.drilldownAll(assetID, start, end)
	}
	
	a.respondJSON(w, http.StatusOK, map[string]interface{}{
		"asset_id":  assetID,
		"dimension": dimension,
		"start":     start,
		"end":       end,
		"data":      result,
	})
}

// drilldownNetwork 网络维度下钻
func (a *AssetAPI) drilldownNetwork(assetID string, start, end time.Time) interface{} {
	var metrics []*AssetMetrics
	
	if a.storage != nil && a.storage.config.Enabled {
		metrics, _ = a.storage.GetMetrics(assetID, start, end, 0)
	} else if a.collector != nil {
		metrics = a.collector.GetMetricsHistory(assetID, 0)
	}
	
	// 聚合网络指标
	result := &DrilldownNetworkResult{
		Interfaces: make(map[string]*InterfaceDrilldown),
		TCPStates:  make(map[string]int),
	}
	
	for _, m := range metrics {
		if m.Network == nil {
			continue
		}
		
		// 聚合接口数据
		for ifaceName, iface := range m.Network.Interfaces {
			if result.Interfaces[ifaceName] == nil {
				result.Interfaces[ifaceName] = &InterfaceDrilldown{
					Name: ifaceName,
				}
			}
			
			dd := result.Interfaces[ifaceName]
			dd.BytesSent += iface.BytesSent
			dd.BytesRecv += iface.BytesRecv
			dd.PacketsSent += iface.PacketsSent
			dd.PacketsRecv += iface.PacketsRecv
			dd.Errors += iface.Errors
			dd.Drops += iface.Drops
		}
		
		// 聚合TCP状态
		for state, count := range m.Network.TCPStates {
			result.TCPStates[state] += count
		}
		
		result.TotalConnections += m.Network.Connections
		result.TotalRetransmits += m.Network.Retransmits
	}
	
	return result
}

// drilldownApplication 应用维度下钻
func (a *AssetAPI) drilldownApplication(assetID string, start, end time.Time) interface{} {
	var metrics []*AssetMetrics
	
	if a.storage != nil && a.storage.config.Enabled {
		metrics, _ = a.storage.GetMetrics(assetID, start, end, 0)
	} else if a.collector != nil {
		metrics = a.collector.GetMetricsHistory(assetID, 0)
	}
	
	result := &DrilldownApplicationResult{
		Processes: make(map[uint32]*ProcessDrilldown),
	}
	
	var totalCPU, totalMemory float64
	var count int
	
	for _, m := range metrics {
		if m.Application == nil {
			continue
		}
		
		totalCPU += m.Application.CPUUsage
		totalMemory += m.Application.MemoryUsage
		count++
		
		// 聚合进程数据
		for _, proc := range m.Application.Processes {
			if result.Processes[proc.PID] == nil {
				result.Processes[proc.PID] = &ProcessDrilldown{
					PID:  proc.PID,
					Name: proc.Name,
				}
			}
			
			pd := result.Processes[proc.PID]
			pd.CPUUsage += proc.CPUUsage
			pd.MemoryRSS += proc.MemoryRSS
			pd.Threads += proc.Threads
			pd.OpenFiles += proc.OpenFiles
			pd.SampleCount++
		}
	}
	
	if count > 0 {
		result.AvgCPUUsage = totalCPU / float64(count)
		result.AvgMemoryUsage = totalMemory / float64(count)
	}
	
	return result
}

// drilldownSystem 系统维度下钻
func (a *AssetAPI) drilldownSystem(assetID string, start, end time.Time) interface{} {
	var metrics []*AssetMetrics
	
	if a.storage != nil && a.storage.config.Enabled {
		metrics, _ = a.storage.GetMetrics(assetID, start, end, 0)
	} else if a.collector != nil {
		metrics = a.collector.GetMetricsHistory(assetID, 0)
	}
	
	result := &DrilldownSystemResult{}
	
	var totalCPU, totalMemory, totalLoad1, totalLoad5, totalLoad15 float64
	var count int
	
	for _, m := range metrics {
		if m.System == nil {
			continue
		}
		
		totalCPU += m.System.CPUUsage
		totalMemory += m.System.MemoryUsage
		totalLoad1 += m.System.Load1
		totalLoad5 += m.System.Load5
		totalLoad15 += m.System.Load15
		count++
		
		if m.System.MemoryTotal > result.MemoryTotal {
			result.MemoryTotal = m.System.MemoryTotal
		}
	}
	
	if count > 0 {
		result.AvgCPUUsage = totalCPU / float64(count)
		result.AvgMemoryUsage = totalMemory / float64(count)
		result.AvgLoad1 = totalLoad1 / float64(count)
		result.AvgLoad5 = totalLoad5 / float64(count)
		result.AvgLoad15 = totalLoad15 / float64(count)
	}
	
	return result
}

// drilldownProcess 进程维度下钻
func (a *AssetAPI) drilldownProcess(assetID string, start, end time.Time) interface{} {
	// 复用应用下钻的进程数据
	return a.drilldownApplication(assetID, start, end)
}

// drilldownInterface 接口维度下钻
func (a *AssetAPI) drilldownInterface(assetID string, start, end time.Time) interface{} {
	// 复用网络下钻的接口数据
	return a.drilldownNetwork(assetID, start, end)
}

// drilldownAll 全维度下钻
func (a *AssetAPI) drilldownAll(assetID string, start, end time.Time) interface{} {
	return map[string]interface{}{
		"network":     a.drilldownNetwork(assetID, start, end),
		"application": a.drilldownApplication(assetID, start, end),
		"system":      a.drilldownSystem(assetID, start, end),
	}
}

// extractMetricByType 按类型提取指标
func (a *AssetAPI) extractMetricByType(metrics *AssetMetrics, metricType string) map[string]interface{} {
	result := make(map[string]interface{})
	
	switch metricType {
	case "network":
		if metrics.Network != nil {
			result["bytes_sent"] = metrics.Network.BytesSent
			result["bytes_recv"] = metrics.Network.BytesRecv
			result["connections"] = metrics.Network.Connections
			result["latency_ms"] = metrics.Network.LatencyMs
		}
	case "application":
		if metrics.Application != nil {
			result["cpu_usage"] = metrics.Application.CPUUsage
			result["memory_usage"] = metrics.Application.MemoryUsage
			result["memory_rss"] = metrics.Application.MemoryRSS
			result["threads"] = metrics.Application.Threads
		}
	case "system":
		if metrics.System != nil {
			result["cpu_usage"] = metrics.System.CPUUsage
			result["memory_usage"] = metrics.System.MemoryUsage
			result["load1"] = metrics.System.Load1
			result["load5"] = metrics.System.Load5
		}
	default:
		// 返回所有指标
		result["network"] = metrics.Network
		result["application"] = metrics.Application
		result["system"] = metrics.System
	}
	
	return result
}

// parseTimeRange 解析时间范围
func (a *AssetAPI) parseTimeRange(r *http.Request) (time.Time, time.Time) {
	now := time.Now()
	
	// 解析开始时间
	startStr := r.URL.Query().Get("start")
	var start time.Time
	if startStr != "" {
		start, _ = time.Parse(time.RFC3339, startStr)
	}
	if start.IsZero() {
		// 默认最近1小时
		start = now.Add(-time.Hour)
	}
	
	// 解析结束时间
	endStr := r.URL.Query().Get("end")
	var end time.Time
	if endStr != "" {
		end, _ = time.Parse(time.RFC3339, endStr)
	}
	if end.IsZero() {
		end = now
	}
	
	return start, end
}

// handleHealth 处理健康检查
func (a *AssetAPI) handleHealth(w http.ResponseWriter, r *http.Request) {
	status := map[string]interface{}{
		"status":    "healthy",
		"timestamp": time.Now(),
		"services": map[string]bool{
			"collector":  a.collector != nil,
			"aggregator": a.aggregator != nil,
			"storage":    a.storage != nil,
		},
	}
	
	a.respondJSON(w, http.StatusOK, status)
}

// respondJSON 返回JSON响应
func (a *AssetAPI) respondJSON(w http.ResponseWriter, statusCode int, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusCode)
	json.NewEncoder(w).Encode(data)
}

// respondError 返回错误响应
func (a *AssetAPI) respondError(w http.ResponseWriter, statusCode int, message string) {
	a.respondJSON(w, statusCode, map[string]interface{}{
		"error": map[string]string{
			"code":    strconv.Itoa(statusCode),
			"message": message,
		},
	})
}

// DrilldownNetworkResult 网络下钻结果
type DrilldownNetworkResult struct {
	Interfaces       map[string]*InterfaceDrilldown `json:"interfaces"`
	TCPStates        map[string]int                 `json:"tcp_states"`
	TotalConnections int                            `json:"total_connections"`
	TotalRetransmits uint64                         `json:"total_retransmits"`
}

// InterfaceDrilldown 接口下钻数据
type InterfaceDrilldown struct {
	Name        string `json:"name"`
	BytesSent   uint64 `json:"bytes_sent"`
	BytesRecv   uint64 `json:"bytes_recv"`
	PacketsSent uint64 `json:"packets_sent"`
	PacketsRecv uint64 `json:"packets_recv"`
	Errors      uint64 `json:"errors"`
	Drops       uint64 `json:"drops"`
}

// DrilldownApplicationResult 应用下钻结果
type DrilldownApplicationResult struct {
	AvgCPUUsage     float64                       `json:"avg_cpu_usage"`
	AvgMemoryUsage  float64                       `json:"avg_memory_usage"`
	Processes       map[uint32]*ProcessDrilldown  `json:"processes"`
}

// ProcessDrilldown 进程下钻数据
type ProcessDrilldown struct {
	PID         uint32  `json:"pid"`
	Name        string  `json:"name"`
	CPUUsage    float64 `json:"cpu_usage"`
	MemoryRSS   uint64  `json:"memory_rss"`
	Threads     int     `json:"threads"`
	OpenFiles   int     `json:"open_files"`
	SampleCount int     `json:"sample_count"`
}

// DrilldownSystemResult 系统下钻结果
type DrilldownSystemResult struct {
	AvgCPUUsage    float64 `json:"avg_cpu_usage"`
	AvgMemoryUsage float64 `json:"avg_memory_usage"`
	AvgLoad1       float64 `json:"avg_load1"`
	AvgLoad5       float64 `json:"avg_load5"`
	AvgLoad15      float64 `json:"avg_load15"`
	MemoryTotal    uint64  `json:"memory_total"`
}

// AssetListResponse 资产列表响应
type AssetListResponse struct {
	Assets []string `json:"assets"`
	Count  int      `json:"count"`
}

// AssetMetricsResponse 资产指标响应
type AssetMetricsResponse struct {
	AssetID   string          `json:"asset_id"`
	Start     time.Time       `json:"start"`
	End       time.Time       `json:"end"`
	Metrics   []*AssetMetrics `json:"metrics"`
	Count     int             `json:"count"`
}

// AssetDrilldownResponse 资产下钻响应
type AssetDrilldownResponse struct {
	AssetID   string      `json:"asset_id"`
	Dimension string      `json:"dimension"`
	Start     time.Time   `json:"start"`
	End       time.Time   `json:"end"`
	Data      interface{} `json:"data"`
}
