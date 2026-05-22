// Package query 数据检索API
// 提供多条件筛选和历史趋势查询接口
package query

import (
	"context"
	"encoding/json"
	"net/http"
	"strconv"
	"strings"
	"time"
)

// QueryAPIConfig API配置
type QueryAPIConfig struct {
	Enabled      bool   `yaml:"enabled" json:"enabled"`
	ListenAddr   string `yaml:"listen_addr" json:"listen_addr"`
	AuthEnabled  bool   `yaml:"auth_enabled" json:"auth_enabled"`
	AuthToken    string `yaml:"auth_token" json:"auth_token"`
	RateLimit    int    `yaml:"rate_limit" json:"rate_limit"`
	ReadTimeout  int    `yaml:"read_timeout" json:"read_timeout"`
	WriteTimeout int    `yaml:"write_timeout" json:"write_timeout"`
}

// QueryAPI 数据检索API服务
type QueryAPI struct {
	config    *QueryAPIConfig
	filter    *FilterEngine
	trend     *TrendGenerator
	server    *http.Server
}

// NewQueryAPI 创建数据检索API服务
func NewQueryAPI(config *QueryAPIConfig, filter *FilterEngine, trend *TrendGenerator) *QueryAPI {
	return &QueryAPI{
		config: config,
		filter: filter,
		trend:  trend,
	}
}

// Start 启动API服务
func (a *QueryAPI) Start() error {
	if !a.config.Enabled {
		return nil
	}

	mux := http.NewServeMux()

	// 网络数据检索
	mux.HandleFunc("/api/v1/query/network", a.authMiddleware(a.handleNetworkQuery))
	mux.HandleFunc("/api/v1/query/network/filter", a.authMiddleware(a.handleNetworkFilter))
	mux.HandleFunc("/api/v1/query/network/trend", a.authMiddleware(a.handleNetworkTrend))

	// 应用数据检索
	mux.HandleFunc("/api/v1/query/application", a.authMiddleware(a.handleApplicationQuery))
	mux.HandleFunc("/api/v1/query/application/filter", a.authMiddleware(a.handleApplicationFilter))
	mux.HandleFunc("/api/v1/query/application/trend", a.authMiddleware(a.handleApplicationTrend))

	// SQL数据检索
	mux.HandleFunc("/api/v1/query/sql", a.authMiddleware(a.handleSQLQuery))
	mux.HandleFunc("/api/v1/query/sql/filter", a.authMiddleware(a.handleSQLFilter))
	mux.HandleFunc("/api/v1/query/sql/trend", a.authMiddleware(a.handleSQLTrend))

	// 通用查询
	mux.HandleFunc("/api/v1/query/execute", a.authMiddleware(a.handleExecuteQuery))
	mux.HandleFunc("/api/v1/query/multi", a.authMiddleware(a.handleMultiQuery))

	// 趋势图
	mux.HandleFunc("/api/v1/query/trend", a.authMiddleware(a.handleTrend))
	mux.HandleFunc("/api/v1/query/trend/heatmap", a.authMiddleware(a.handleHeatmap))
	mux.HandleFunc("/api/v1/query/trend/sparkline", a.authMiddleware(a.handleSparkline))
	mux.HandleFunc("/api/v1/query/trend/gauge", a.authMiddleware(a.handleGauge))

	// 保存的查询
	mux.HandleFunc("/api/v1/query/saved", a.authMiddleware(a.handleSavedQueries))
	mux.HandleFunc("/api/v1/query/saved/", a.authMiddleware(a.handleSavedQuery))

	// 健康检查
	mux.HandleFunc("/api/v1/query/health", a.handleHealth)

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
func (a *QueryAPI) Stop() error {
	if a.server != nil {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		return a.server.Shutdown(ctx)
	}
	return nil
}

// authMiddleware 认证中间件
func (a *QueryAPI) authMiddleware(next http.HandlerFunc) http.HandlerFunc {
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

// handleNetworkQuery 处理网络查询
func (a *QueryAPI) handleNetworkQuery(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		a.respondError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}

	// 解析参数
	filter := a.parseNetworkFilter(r)
	start, end := a.parseTimeRange(r)
	pagination := a.parsePagination(r)

	// 执行查询
	result, err := a.filter.QueryNetwork(r.Context(), filter, start, end, pagination)
	if err != nil {
		a.respondError(w, http.StatusInternalServerError, err.Error())
		return
	}

	a.respondJSON(w, http.StatusOK, result)
}

// handleNetworkFilter 处理网络筛选
func (a *QueryAPI) handleNetworkFilter(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		a.respondError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}

	var request struct {
		Filter     *NetworkFilter `json:"filter"`
		StartTime  time.Time      `json:"start_time"`
		EndTime    time.Time      `json:"end_time"`
		Pagination *Pagination    `json:"pagination"`
	}

	if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
		a.respondError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	result, err := a.filter.QueryNetwork(r.Context(), request.Filter, request.StartTime, request.EndTime, request.Pagination)
	if err != nil {
		a.respondError(w, http.StatusInternalServerError, err.Error())
		return
	}

	a.respondJSON(w, http.StatusOK, result)
}

// handleNetworkTrend 处理网络趋势
func (a *QueryAPI) handleNetworkTrend(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		a.respondError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}

	var request TrendRequest
	if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
		a.respondError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	request.SourceType = "network"

	result, err := a.trend.Generate(r.Context(), &request)
	if err != nil {
		a.respondError(w, http.StatusInternalServerError, err.Error())
		return
	}

	a.respondJSON(w, http.StatusOK, result)
}

// handleApplicationQuery 处理应用查询
func (a *QueryAPI) handleApplicationQuery(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		a.respondError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}

	filter := a.parseApplicationFilter(r)
	start, end := a.parseTimeRange(r)
	pagination := a.parsePagination(r)

	result, err := a.filter.QueryApplication(r.Context(), filter, start, end, pagination)
	if err != nil {
		a.respondError(w, http.StatusInternalServerError, err.Error())
		return
	}

	a.respondJSON(w, http.StatusOK, result)
}

// handleApplicationFilter 处理应用筛选
func (a *QueryAPI) handleApplicationFilter(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		a.respondError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}

	var request struct {
		Filter     *ApplicationFilter `json:"filter"`
		StartTime  time.Time          `json:"start_time"`
		EndTime    time.Time          `json:"end_time"`
		Pagination *Pagination        `json:"pagination"`
	}

	if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
		a.respondError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	result, err := a.filter.QueryApplication(r.Context(), request.Filter, request.StartTime, request.EndTime, request.Pagination)
	if err != nil {
		a.respondError(w, http.StatusInternalServerError, err.Error())
		return
	}

	a.respondJSON(w, http.StatusOK, result)
}

// handleApplicationTrend 处理应用趋势
func (a *QueryAPI) handleApplicationTrend(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		a.respondError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}

	var request TrendRequest
	if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
		a.respondError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	request.SourceType = "application"

	result, err := a.trend.Generate(r.Context(), &request)
	if err != nil {
		a.respondError(w, http.StatusInternalServerError, err.Error())
		return
	}

	a.respondJSON(w, http.StatusOK, result)
}

// handleSQLQuery 处理SQL查询
func (a *QueryAPI) handleSQLQuery(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		a.respondError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}

	filter := a.parseSQLFilter(r)
	start, end := a.parseTimeRange(r)
	pagination := a.parsePagination(r)

	result, err := a.filter.QuerySQL(r.Context(), filter, start, end, pagination)
	if err != nil {
		a.respondError(w, http.StatusInternalServerError, err.Error())
		return
	}

	a.respondJSON(w, http.StatusOK, result)
}

// handleSQLFilter 处理SQL筛选
func (a *QueryAPI) handleSQLFilter(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		a.respondError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}

	var request struct {
		Filter     *SQLFilter  `json:"filter"`
		StartTime  time.Time   `json:"start_time"`
		EndTime    time.Time   `json:"end_time"`
		Pagination *Pagination `json:"pagination"`
	}

	if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
		a.respondError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	result, err := a.filter.QuerySQL(r.Context(), request.Filter, request.StartTime, request.EndTime, request.Pagination)
	if err != nil {
		a.respondError(w, http.StatusInternalServerError, err.Error())
		return
	}

	a.respondJSON(w, http.StatusOK, result)
}

// handleSQLTrend 处理SQL趋势
func (a *QueryAPI) handleSQLTrend(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		a.respondError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}

	var request TrendRequest
	if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
		a.respondError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	request.SourceType = "sql"

	result, err := a.trend.Generate(r.Context(), &request)
	if err != nil {
		a.respondError(w, http.StatusInternalServerError, err.Error())
		return
	}

	a.respondJSON(w, http.StatusOK, result)
}

// handleExecuteQuery 执行通用查询
func (a *QueryAPI) handleExecuteQuery(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		a.respondError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}

	var request QueryRequest
	if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
		a.respondError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	result, err := a.filter.Execute(r.Context(), &request)
	if err != nil {
		a.respondError(w, http.StatusInternalServerError, err.Error())
		return
	}

	a.respondJSON(w, http.StatusOK, result)
}

// handleMultiQuery 处理多条件查询
func (a *QueryAPI) handleMultiQuery(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		a.respondError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}

	var request MultiFilterRequest
	if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
		a.respondError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	result, err := a.filter.MultiFilter(r.Context(), &request)
	if err != nil {
		a.respondError(w, http.StatusInternalServerError, err.Error())
		return
	}

	a.respondJSON(w, http.StatusOK, result)
}

// handleTrend 处理趋势请求
func (a *QueryAPI) handleTrend(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		a.respondError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}

	var request TrendRequest
	if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
		a.respondError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	result, err := a.trend.Generate(r.Context(), &request)
	if err != nil {
		a.respondError(w, http.StatusInternalServerError, err.Error())
		return
	}

	a.respondJSON(w, http.StatusOK, result)
}

// handleHeatmap 处理热力图请求
func (a *QueryAPI) handleHeatmap(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		a.respondError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}

	var request struct {
		TrendRequest
		XDimension string `json:"x_dimension"`
		YDimension string `json:"y_dimension"`
	}

	if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
		a.respondError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	result, err := a.trend.GenerateHeatmap(r.Context(), &request.TrendRequest, request.XDimension, request.YDimension)
	if err != nil {
		a.respondError(w, http.StatusInternalServerError, err.Error())
		return
	}

	a.respondJSON(w, http.StatusOK, result)
}

// handleSparkline 处理迷你图请求
func (a *QueryAPI) handleSparkline(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		a.respondError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}

	var request TrendRequest
	if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
		a.respondError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	result, err := a.trend.GenerateSparkline(r.Context(), &request)
	if err != nil {
		a.respondError(w, http.StatusInternalServerError, err.Error())
		return
	}

	a.respondJSON(w, http.StatusOK, result)
}

// handleGauge 处理仪表盘请求
func (a *QueryAPI) handleGauge(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		a.respondError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}

	var request TrendRequest
	if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
		a.respondError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	result, err := a.trend.GenerateGauge(r.Context(), &request)
	if err != nil {
		a.respondError(w, http.StatusInternalServerError, err.Error())
		return
	}

	a.respondJSON(w, http.StatusOK, result)
}

// handleSavedQueries 处理保存的查询列表
func (a *QueryAPI) handleSavedQueries(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		queries, err := a.filter.ListSavedQueries()
		if err != nil {
			a.respondError(w, http.StatusInternalServerError, err.Error())
			return
		}
		a.respondJSON(w, http.StatusOK, map[string]interface{}{
			"queries": queries,
		})

	case http.MethodPost:
		var request struct {
			Name  string       `json:"name"`
			Query *QueryRequest `json:"query"`
		}
		if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
			a.respondError(w, http.StatusBadRequest, "invalid request body")
			return
		}

		if err := a.filter.SaveQuery(request.Name, request.Query); err != nil {
			a.respondError(w, http.StatusInternalServerError, err.Error())
			return
		}

		a.respondJSON(w, http.StatusOK, map[string]string{"status": "saved"})

	default:
		a.respondError(w, http.StatusMethodNotAllowed, "method not allowed")
	}
}

// handleSavedQuery 处理单个保存的查询
func (a *QueryAPI) handleSavedQuery(w http.ResponseWriter, r *http.Request) {
	name := strings.TrimPrefix(r.URL.Path, "/api/v1/query/saved/")

	switch r.Method {
	case http.MethodGet:
		query, err := a.filter.LoadQuery(name)
		if err != nil {
			a.respondError(w, http.StatusNotFound, "query not found")
			return
		}
		a.respondJSON(w, http.StatusOK, query)

	case http.MethodDelete:
		if err := a.filter.DeleteQuery(name); err != nil {
			a.respondError(w, http.StatusInternalServerError, err.Error())
			return
		}
		a.respondJSON(w, http.StatusOK, map[string]string{"status": "deleted"})

	default:
		a.respondError(w, http.StatusMethodNotAllowed, "method not allowed")
	}
}

// handleHealth 处理健康检查
func (a *QueryAPI) handleHealth(w http.ResponseWriter, r *http.Request) {
	status := map[string]interface{}{
		"status":    "healthy",
		"timestamp": time.Now(),
		"services": map[string]bool{
			"filter": a.filter != nil,
			"trend":  a.trend != nil,
		},
	}

	a.respondJSON(w, http.StatusOK, status)
}

// parseNetworkFilter 解析网络筛选参数
func (a *QueryAPI) parseNetworkFilter(r *http.Request) *NetworkFilter {
	filter := &NetworkFilter{}

	// IP筛选
	if sourceIP := r.URL.Query().Get("source_ip"); sourceIP != "" {
		filter.SourceIP = strings.Split(sourceIP, ",")
	}
	if destIP := r.URL.Query().Get("dest_ip"); destIP != "" {
		filter.DestIP = strings.Split(destIP, ",")
	}

	// 端口筛选
	if sourcePort := r.URL.Query().Get("source_port"); sourcePort != "" {
		filter.SourcePort = parseUint16Slice(sourcePort)
	}
	if destPort := r.URL.Query().Get("dest_port"); destPort != "" {
		filter.DestPort = parseUint16Slice(destPort)
	}

	// 协议筛选
	if protocol := r.URL.Query().Get("protocol"); protocol != "" {
		filter.Protocol = strings.Split(protocol, ",")
	}

	// 性能范围筛选
	if latencyMin := r.URL.Query().Get("latency_min"); latencyMin != "" {
		val, _ := strconv.ParseFloat(latencyMin, 64)
		filter.LatencyMin = &val
	}
	if latencyMax := r.URL.Query().Get("latency_max"); latencyMax != "" {
		val, _ := strconv.ParseFloat(latencyMax, 64)
		filter.LatencyMax = &val
	}

	// TCP状态筛选
	if tcpStates := r.URL.Query().Get("tcp_states"); tcpStates != "" {
		filter.TCPStates = strings.Split(tcpStates, ",")
	}

	return filter
}

// parseApplicationFilter 解析应用筛选参数
func (a *QueryAPI) parseApplicationFilter(r *http.Request) *ApplicationFilter {
	filter := &ApplicationFilter{}

	// 进程筛选
	if processName := r.URL.Query().Get("process_name"); processName != "" {
		filter.ProcessName = strings.Split(processName, ",")
	}
	if podName := r.URL.Query().Get("pod_name"); podName != "" {
		filter.PodName = strings.Split(podName, ",")
	}
	if namespace := r.URL.Query().Get("namespace"); namespace != "" {
		filter.Namespace = strings.Split(namespace, ",")
	}

	// 性能范围筛选
	if cpuMin := r.URL.Query().Get("cpu_min"); cpuMin != "" {
		val, _ := strconv.ParseFloat(cpuMin, 64)
		filter.CPUUsageMin = &val
	}
	if cpuMax := r.URL.Query().Get("cpu_max"); cpuMax != "" {
		val, _ := strconv.ParseFloat(cpuMax, 64)
		filter.CPUUsageMax = &val
	}
	if memMin := r.URL.Query().Get("memory_min"); memMin != "" {
		val, _ := strconv.ParseFloat(memMin, 64)
		filter.MemoryUsageMin = &val
	}
	if memMax := r.URL.Query().Get("memory_max"); memMax != "" {
		val, _ := strconv.ParseFloat(memMax, 64)
		filter.MemoryUsageMax = &val
	}

	return filter
}

// parseSQLFilter 解析SQL筛选参数
func (a *QueryAPI) parseSQLFilter(r *http.Request) *SQLFilter {
	filter := &SQLFilter{}

	// 数据库/表筛选
	if database := r.URL.Query().Get("database"); database != "" {
		filter.Database = strings.Split(database, ",")
	}
	if table := r.URL.Query().Get("table"); table != "" {
		filter.Table = strings.Split(table, ",")
	}

	// 操作类型筛选
	if operation := r.URL.Query().Get("operation"); operation != "" {
		filter.Operation = strings.Split(operation, ",")
	}

	// 执行时间筛选
	if execTimeMin := r.URL.Query().Get("exec_time_min"); execTimeMin != "" {
		val, _ := strconv.ParseFloat(execTimeMin, 64)
		filter.ExecTimeMin = &val
	}
	if execTimeMax := r.URL.Query().Get("exec_time_max"); execTimeMax != "" {
		val, _ := strconv.ParseFloat(execTimeMax, 64)
		filter.ExecTimeMax = &val
	}

	// 错误筛选
	if hasError := r.URL.Query().Get("has_error"); hasError != "" {
		val := hasError == "true"
		filter.HasError = &val
	}

	return filter
}

// parseTimeRange 解析时间范围
func (a *QueryAPI) parseTimeRange(r *http.Request) (time.Time, time.Time) {
	now := time.Now()

	startStr := r.URL.Query().Get("start")
	var start time.Time
	if startStr != "" {
		start, _ = time.Parse(time.RFC3339, startStr)
	}
	if start.IsZero() {
		start = now.Add(-time.Hour)
	}

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

// parsePagination 解析分页参数
func (a *QueryAPI) parsePagination(r *http.Request) *Pagination {
	page, _ := strconv.Atoi(r.URL.Query().Get("page"))
	if page <= 0 {
		page = 1
	}

	pageSize, _ := strconv.Atoi(r.URL.Query().Get("page_size"))
	if pageSize <= 0 {
		pageSize = 20
	}

	return &Pagination{
		Page:     page,
		PageSize: pageSize,
	}
}

// respondJSON 返回JSON响应
func (a *QueryAPI) respondJSON(w http.ResponseWriter, statusCode int, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusCode)
	json.NewEncoder(w).Encode(data)
}

// respondError 返回错误响应
func (a *QueryAPI) respondError(w http.ResponseWriter, statusCode int, message string) {
	a.respondJSON(w, statusCode, map[string]interface{}{
		"error": map[string]string{
			"code":    strconv.Itoa(statusCode),
			"message": message,
		},
	})
}

// parseUint16Slice 解析uint16切片
func parseUint16Slice(s string) []uint16 {
	parts := strings.Split(s, ",")
	var result []uint16
	for _, p := range parts {
		val, err := strconv.ParseUint(strings.TrimSpace(p), 10, 16)
		if err == nil {
			result = append(result, uint16(val))
		}
	}
	return result
}

// QueryAPIResponse API响应
type QueryAPIResponse struct {
	Success   bool        `json:"success"`
	Data      interface{} `json:"data,omitempty"`
	Error     *APIError   `json:"error,omitempty"`
	Timestamp time.Time   `json:"timestamp"`
}

// APIError API错误
type APIError struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

// NetworkQueryResponse 网络查询响应
type NetworkQueryResponse struct {
	Data       []map[string]interface{} `json:"data"`
	Total      int64                    `json:"total"`
	Page       int                      `json:"page"`
	PageSize   int                      `json:"page_size"`
	Filter     *NetworkFilter           `json:"filter"`
	StartTime  time.Time                `json:"start_time"`
	EndTime    time.Time                `json:"end_time"`
}

// ApplicationQueryResponse 应用查询响应
type ApplicationQueryResponse struct {
	Data       []map[string]interface{} `json:"data"`
	Total      int64                    `json:"total"`
	Page       int                      `json:"page"`
	PageSize   int                      `json:"page_size"`
	Filter     *ApplicationFilter       `json:"filter"`
	StartTime  time.Time                `json:"start_time"`
	EndTime    time.Time                `json:"end_time"`
}

// SQLQueryResponse SQL查询响应
type SQLQueryResponse struct {
	Data       []map[string]interface{} `json:"data"`
	Total      int64                    `json:"total"`
	Page       int                      `json:"page"`
	PageSize   int                      `json:"page_size"`
	Filter     *SQLFilter               `json:"filter"`
	StartTime  time.Time                `json:"start_time"`
	EndTime    time.Time                `json:"end_time"`
}

// TrendAPIResponse 趋势API响应
type TrendAPIResponse struct {
	TrendType   TrendType       `json:"trend_type"`
	Granularity string          `json:"granularity"`
	Series      []*TrendSeries  `json:"series"`
	Statistics  *TrendStatistics `json:"statistics"`
	Compare     *CompareResult  `json:"compare,omitempty"`
}
