// Package asset 资产页面模块
// 支持管理员/租户双视图、资产分类、多维筛选、下钻详情
package asset

import (
	"encoding/json"
	"fmt"
	"net/http"
	"sort"
	"strings"
	"sync"
	"time"
)

// ==================== 双视图数据模型 ====================

// ViewMode 视图模式
type ViewMode string

const (
	ViewAdmin ViewMode = "admin" // 管理员视图 - 全部资产
	ViewTenant ViewMode = "tenant" // 租户视图 - 仅本租户资产
)

// AssetViewConfig 资产视图配置
type AssetViewConfig struct {
	Mode        ViewMode `json:"mode"`
	TenantID    string   `json:"tenant_id,omitempty"`    // 租户ID
	TenantName  string   `json:"tenant_name,omitempty"`  // 租户名称
	Labels      []string `json:"labels,omitempty"`       // 标签过滤
	Statuses    []string `json:"statuses,omitempty"`      // 状态过滤
	Types       []string `json:"types,omitempty"`        // 类型过滤
	BusinessGroups []string `json:"business_groups,omitempty"` // 业务组过滤
	Page        int      `json:"page"`
	PageSize    int      `json:"page_size"`
	SortBy      string   `json:"sort_by"`
	SortOrder   string   `json:"sort_order"` // asc/desc
}

// AssetDashboard 资产仪表盘
type AssetDashboard struct {
	ViewMode       ViewMode         `json:"view_mode"`
	TenantID       string           `json:"tenant_id,omitempty"`
	TenantName     string           `json:"tenant_name,omitempty"`
	Summary        AssetSummary     `json:"summary"`
	AssetsByType   []*AssetTypeGroup `json:"assets_by_type"`
	RecentAlerts   []*AssetAlert    `json:"recent_alerts"`
	TopAlertAssets []*AssetAlertItem `json:"top_alert_assets"`
	Timestamp      time.Time        `json:"timestamp"`
}

// AssetSummary 资产汇总
type AssetSummary struct {
	TotalAssets    int            `json:"total_assets"`
	HealthyAssets  int            `json:"healthy_assets"`
	WarningAssets  int            `json:"warning_assets"`
	ErrorAssets    int            `json:"error_assets"`
	OfflineAssets  int            `json:"offline_assets"`
	TotalAlerts    int            `json:"total_alerts"`
	CriticalAlerts int            `json:"critical_alerts"`
	WarningAlerts  int            `json:"warning_alerts"`
	InfoAlerts     int            `json:"info_alerts"`
	AvgCPUUsage    float64        `json:"avg_cpu_usage"`
	AvgMemoryUsage float64        `json:"avg_memory_usage"`
	AvgNetworkLoad float64        `json:"avg_network_load"`
}

// AssetTypeGroup 按类型分组的资产
type AssetTypeGroup struct {
	Type       string      `json:"type"`
	TypeName   string      `json:"type_name"`
	Icon       string      `json:"icon"`
	Color      string      `json:"color"`
	Count      int         `json:"count"`
	Assets     []*AssetItem `json:"assets"`
}

// AssetItem 资产列表项
type AssetItem struct {
	ID            string            `json:"id"`
	Name          string            `json:"name"`
	Type          AssetType         `json:"type"`
	TypeName      string            `json:"type_name"`
	Status        string            `json:"status"`
	IPAddresses   []string          `json:"ip_addresses,omitempty"`
	BusinessGroup string            `json:"business_group,omitempty"`
	TenantID      string            `json:"tenant_id,omitempty"`
	TenantName    string            `json:"tenant_name,omitempty"`
	Labels        map[string]string `json:"labels,omitempty"`
	
	// 实时指标摘要
	CPUUsage      float64 `json:"cpu_usage"`
	MemoryUsage   float64 `json:"memory_usage"`
	NetworkIn     uint64  `json:"network_in"`
	NetworkOut    uint64  `json:"network_out"`
	ErrorRate     float64 `json:"error_rate"`
	ResponseTime  float64 `json:"response_time_ms"`
	
	// 告警统计
	AlertCount    int     `json:"alert_count"`
	CriticalCount int     `json:"critical_count"`
	
	// 时间
	LastSeen      time.Time `json:"last_seen"`
	CreatedAt     time.Time `json:"created_at"`
}

// AssetAlert 资产告警
type AssetAlert struct {
	ID          string    `json:"id"`
	AssetID     string    `json:"asset_id"`
	AssetName   string    `json:"asset_name"`
	Severity    string    `json:"severity"` // critical/warning/info
	Title       string    `json:"title"`
	Message     string    `json:"message"`
	Status      string    `json:"status"`  // firing/resolved
	FiredAt     time.Time `json:"fired_at"`
	ResolvedAt  time.Time `json:"resolved_at,omitempty"`
}

// AssetAlertItem 告警资产排名项
type AssetAlertItem struct {
	AssetID      string  `json:"asset_id"`
	AssetName    string  `json:"asset_name"`
	AssetType    string  `json:"asset_type"`
	CriticalCount int   `json:"critical_count"`
	WarningCount  int   `json:"warning_count"`
	TotalAlerts   int   `json:"total_alerts"`
}

// AssetDetail 资产详情（下钻）
type AssetDetail struct {
	AssetItem
	// 网络指标详情
	Network *DrilldownNetworkResult `json:"network,omitempty"`
	// 应用指标详情
	Application *DrilldownAppResult `json:"application,omitempty"`
	// SQL指标详情
	SQL *DrilldownSQLResult `json:"sql,omitempty"`
	// 系统指标详情
	System *DrilldownSystemResult `json:"system,omitempty"`
	// 告警历史
	Alerts []*AssetAlert `json:"alerts,omitempty"`
}

// DrilldownAppResult 应用指标下钻结果
type DrilldownAppResult struct {
	AvgCPUUsage    float64                      `json:"avg_cpu_usage"`
	AvgMemoryUsage float64                      `json:"avg_memory_usage"`
	AvgResponseTime float64                     `json:"avg_response_time_ms"`
	AvgErrorRate   float64                      `json:"avg_error_rate"`
	AvgThroughput  float64                      `json:"avg_throughput_rps"`
	Processes      map[uint32]*ProcessDrilldown `json:"processes"`
	ProcessCount   int                          `json:"process_count"`
}

// DrilldownSQLResult SQL指标下钻结果
type DrilldownSQLResult struct {
	TotalQueries   int               `json:"total_queries"`
	SlowQueries    int               `json:"slow_queries"`
	ErrorQueries   int               `json:"error_queries"`
	AvgLatency     float64           `json:"avg_latency_ms"`
	P95Latency     float64           `json:"p95_latency_ms"`
	P99Latency     float64           `json:"p99_latency_ms"`
	TopSlowQueries []*SQLQueryItem   `json:"top_slow_queries"`
	TopErrorQueries []*SQLQueryItem  `json:"top_error_queries"`
}

// SQLQueryItem SQL查询项
type SQLQueryItem struct {
	Query     string  `json:"query"`
	Database  string  `json:"database"`
	Calls     int     `json:"calls"`
	AvgTime   float64 `json:"avg_time_ms"`
	MaxTime   float64 `json:"max_time_ms"`
	RowsExamined int  `json:"rows_examined"`
}

// DrilldownSystemResult 系统指标下钻结果
type DrilldownSystemResult struct {
	AvgCPUUsage    float64 `json:"avg_cpu_usage"`
	AvgMemoryUsage float64 `json:"avg_memory_usage"`
	MemoryTotal    uint64  `json:"memory_total"`
	MemoryUsed     uint64  `json:"memory_used"`
	DiskTotal      uint64  `json:"disk_total"`
	DiskUsed       uint64  `json:"disk_used"`
	AvgLoad1       float64 `json:"avg_load1"`
	AvgLoad5       float64 `json:"avg_load5"`
	AvgLoad15      float64 `json:"avg_load15"`
	Uptime         string  `json:"uptime"`
	OS             string  `json:"os"`
	KernelVersion  string  `json:"kernel_version"`
}

// ==================== 资产页面管理器 ====================

// AssetPageManager 资产页面管理器
type AssetPageManager struct {
	collector *MetricsCollector
	aggregator *MetricsAggregator
	storage   *AssetStorage
	mu        sync.RWMutex
	
	// 租户映射
	tenantMap map[string]string // tenantID -> tenantName
	
	// 告警数据（模拟）
	alerts []*AssetAlert
}

// NewAssetPageManager 创建资产页面管理器
func NewAssetPageManager(collector *MetricsCollector, aggregator *MetricsAggregator, storage *AssetStorage) *AssetPageManager {
	mgr := &AssetPageManager{
		collector:  collector,
		aggregator: aggregator,
		storage:    storage,
		tenantMap:  make(map[string]string),
		alerts:     make([]*AssetAlert, 0),
	}
	
	// 初始化租户映射
	mgr.initTenantMap()
	// 初始化模拟告警
	mgr.initMockAlerts()
	
	return mgr
}

func (m *AssetPageManager) initTenantMap() {
	m.tenantMap["tenant-001"] = "交易平台"
	m.tenantMap["tenant-002"] = "支付中心"
	m.tenantMap["tenant-003"] = "用户中心"
	m.tenantMap["tenant-004"] = "商品中心"
	m.tenantMap["tenant-005"] = "营销中心"
}

func (m *AssetPageManager) initMockAlerts() {
	now := time.Now()
	m.alerts = []*AssetAlert{
		{ID: "alert-1", AssetID: "host-node-1", AssetName: "k8s-node-1", Severity: "critical", Title: "CPU使用率超过90%", Message: "CPU使用率持续5分钟超过90%", Status: "firing", FiredAt: now.Add(-5 * time.Minute)},
		{ID: "alert-2", AssetID: "host-node-2", AssetName: "k8s-node-2", Severity: "warning", Title: "内存使用率超过80%", Message: "内存使用率达到82%", Status: "firing", FiredAt: now.Add(-10 * time.Minute)},
		{ID: "alert-3", AssetID: "vm-worker-1", AssetName: "vm-worker-1", Severity: "warning", Title: "磁盘使用率超过85%", Message: "磁盘使用率达到87%", Status: "firing", FiredAt: now.Add(-30 * time.Minute)},
		{ID: "alert-4", AssetID: "pod-order-1", AssetName: "order-service-pod-1", Severity: "critical", Title: "服务响应超时", Message: "P99延迟超过2秒", Status: "firing", FiredAt: now.Add(-2 * time.Minute)},
		{ID: "alert-5", AssetID: "pod-payment-1", AssetName: "payment-service-pod-1", Severity: "info", Title: "重启次数增加", Message: "过去1小时重启3次", Status: "firing", FiredAt: now.Add(-1 * time.Hour)},
		{ID: "alert-6", AssetID: "host-node-3", AssetName: "k8s-node-3", Severity: "warning", Title: "网络丢包率升高", Message: "eth0丢包率达到0.5%", Status: "firing", FiredAt: now.Add(-15 * time.Minute)},
	}
}

// GetDashboard 获取仪表盘数据
func (m *AssetPageManager) GetDashboard(config *AssetViewConfig) *AssetDashboard {
	m.mu.RLock()
	defer m.mu.RUnlock()
	
	dashboard := &AssetDashboard{
		ViewMode:   config.Mode,
		TenantID:   config.TenantID,
		TenantName: config.TenantName,
		Timestamp:  time.Now(),
	}
	
	// 获取所有资产
	allAssets := m.collectAllAssets(config)
	
	// 计算汇总
	dashboard.Summary = m.calculateSummary(allAssets)
	
	// 按类型分组
	dashboard.AssetsByType = m.groupByType(allAssets)
	
	// 获取告警
	filteredAlerts := m.filterAlerts(config)
	dashboard.RecentAlerts = filteredAlerts
	dashboard.TopAlertAssets = m.getTopAlertAssets(allAssets, filteredAlerts)
	
	return dashboard
}

// GetAssetList 获取资产列表
func (m *AssetPageManager) GetAssetList(config *AssetViewConfig) *AssetListResult {
	m.mu.RLock()
	defer m.mu.RUnlock()
	
	allAssets := m.collectAllAssets(config)
	
	// 排序
	allAssets = m.sortAssets(allAssets, config.SortBy, config.SortOrder)
	
	// 分页
	total := len(allAssets)
	if config.Page <= 0 {
		config.Page = 1
	}
	if config.PageSize <= 0 {
		config.PageSize = 20
	}
	
	start := (config.Page - 1) * config.PageSize
	end := start + config.PageSize
	if start > total {
		start = total
	}
	if end > total {
		end = total
	}
	
	return &AssetListResult{
		Assets:    allAssets[start:end],
		Total:     total,
		Page:      config.Page,
		PageSize:  config.PageSize,
		TotalPages: (total + config.PageSize - 1) / config.PageSize,
	}
}

// GetAssetDetail 获取资产详情（下钻）
func (m *AssetPageManager) GetAssetDetail(assetID string, dimensions []string) *AssetDetail {
	m.mu.RLock()
	defer m.mu.RUnlock()
	
	// 查找资产
	asset := m.findAsset(assetID)
	if asset == nil {
		return nil
	}
	
	detail := &AssetDetail{
		AssetItem: *asset,
	}
	
	// 获取指标数据
	if m.collector != nil {
		metrics := m.collector.GetLatestMetrics(assetID)
		if metrics != nil {
			// 网络下钻
			if containsStr(dimensions, "network") || containsStr(dimensions, "all") {
				detail.Network = m.drilldownNetwork(metrics)
			}
			// 应用下钻
			if containsStr(dimensions, "application") || containsStr(dimensions, "all") {
				detail.Application = m.drilldownApplication(metrics)
			}
			// 系统下钻
			if containsStr(dimensions, "system") || containsStr(dimensions, "all") {
				detail.System = m.drilldownSystem(metrics)
			}
		}
	}
	
	// SQL下钻（模拟）
	if containsStr(dimensions, "sql") || containsStr(dimensions, "all") {
		detail.SQL = m.drilldownSQL(assetID)
	}
	
	// 告警历史
	detail.Alerts = m.getAssetAlerts(assetID)
	
	return detail
}

// GetFilterOptions 获取筛选选项
func (m *AssetPageManager) GetFilterOptions() *FilterOptions {
	m.mu.RLock()
	defer m.mu.RUnlock()
	
	options := &FilterOptions{
		Types:         []string{"host", "vm", "container", "pod", "service", "node"},
		Statuses:      []string{"running", "warning", "error", "offline"},
		BusinessGroups: []string{"订单中心", "支付中心", "用户中心", "商品中心", "营销中心", "基础设施"},
		Tenants:       m.getTenantList(),
		Labels:        []string{"env:production", "env:test", "env:dev", "tier:frontend", "tier:backend", "critical:true"},
	}
	
	return options
}

// FilterOptions 筛选选项
type FilterOptions struct {
	Types         []string `json:"types"`
	Statuses      []string `json:"statuses"`
	BusinessGroups []string `json:"business_groups"`
	Tenants       []TenantOption `json:"tenants"`
	Labels        []string `json:"labels"`
}

// TenantOption 租户选项
type TenantOption struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

// AssetListResult 资产列表结果
type AssetListResult struct {
	Assets     []*AssetItem `json:"assets"`
	Total      int           `json:"total"`
	Page       int           `json:"page"`
	PageSize   int           `json:"page_size"`
	TotalPages int           `json:"total_pages"`
}

// ==================== 内部方法 ====================

// collectAllAssets 收集所有资产
func (m *AssetPageManager) collectAllAssets(config *AssetViewConfig) []*AssetItem {
	var assets []*AssetItem
	
	if m.collector != nil {
		metricsMap := m.collector.GetAllLatestMetrics()
		
		for assetID, metrics := range metricsMap {
			item := m.metricsToAssetItem(assetID, metrics)
			
			// 视图过滤
			if config.Mode == ViewTenant && config.TenantID != "" {
				if item.TenantID != config.TenantID {
					continue
				}
			}
			
			// 类型过滤
			if len(config.Types) > 0 && !containsStr(config.Types, string(item.Type)) {
				continue
			}
			
			// 状态过滤
			if len(config.Statuses) > 0 && !containsStr(config.Statuses, item.Status) {
				continue
			}
			
			// 业务组过滤
			if len(config.BusinessGroups) > 0 && !containsStr(config.BusinessGroups, item.BusinessGroup) {
				continue
			}
			
			// 标签过滤
			if len(config.Labels) > 0 {
				matched := false
				for _, label := range config.Labels {
					for k, v := range item.Labels {
						if k+":"+v == label {
							matched = true
							break
						}
					}
					if matched {
						break
					}
				}
				if !matched {
					continue
				}
			}
			
			assets = append(assets, item)
		}
	}
	
	// 如果collector没有数据，生成模拟数据
	if len(assets) == 0 {
		assets = m.generateMockAssets(config)
	}
	
	return assets
}

// metricsToAssetItem 将指标转换为资产项
func (m *AssetPageManager) metricsToAssetItem(assetID string, metrics *AssetMetrics) *AssetItem {
	item := &AssetItem{
		ID:         assetID,
		Name:       metrics.AssetName,
		Type:       metrics.AssetType,
		TypeName:   getAssetTypeName(metrics.AssetType),
		Status:     "running",
		Labels:     metrics.Labels,
		LastSeen:   metrics.Timestamp,
		CreatedAt:  metrics.Timestamp,
		AlertCount: len(m.getAssetAlerts(assetID)),
	}
	
	// 填充指标
	if metrics.Network != nil {
		item.NetworkIn = metrics.Network.BytesRecv
		item.NetworkOut = metrics.Network.BytesSent
	}
	if metrics.Application != nil {
		item.CPUUsage = metrics.Application.CPUUsage
		item.MemoryUsage = metrics.Application.MemoryUsage
		item.ResponseTime = metrics.Application.ResponseTime
		item.ErrorRate = metrics.Application.ErrorRate
	}
	if metrics.System != nil {
		item.CPUUsage = metrics.System.CPUUsage
		item.MemoryUsage = metrics.System.MemoryUsage
	}
	
	// 确定状态
	if item.CPUUsage > 90 || item.MemoryUsage > 90 || item.ErrorRate > 0.05 {
		item.Status = "error"
	} else if item.CPUUsage > 70 || item.MemoryUsage > 80 || item.ErrorRate > 0.01 {
		item.Status = "warning"
	}
	
	// 统计告警级别
	for _, alert := range m.getAssetAlerts(assetID) {
		if alert.Severity == "critical" {
			item.CriticalCount++
		}
	}
	
	return item
}

// generateMockAssets 生成模拟资产数据
func (m *AssetPageManager) generateMockAssets(config *AssetViewConfig) []*AssetItem {
	mockAssets := []struct {
		id           string
		name         string
		typ          AssetType
		group        string
		tenantID     string
		status       string
		cpu          float64
		mem          float64
		netIn        uint64
		netOut       uint64
		errRate      float64
		respTime     float64
		alerts       int
	}{
		{"host-node-1", "k8s-node-1", AssetTypeNode, "基础设施", "tenant-001", "error", 92.5, 85.3, 50000000, 30000000, 0.08, 250, 2},
		{"host-node-2", "k8s-node-2", AssetTypeNode, "基础设施", "tenant-001", "warning", 78.2, 82.1, 40000000, 25000000, 0.02, 120, 1},
		{"host-node-3", "k8s-node-3", AssetTypeNode, "基础设施", "tenant-002", "running", 45.6, 62.4, 35000000, 20000000, 0.005, 80, 0},
		{"vm-worker-1", "vm-worker-1", AssetTypeVM, "基础设施", "tenant-001", "warning", 68.3, 87.2, 60000000, 45000000, 0.01, 150, 1},
		{"vm-worker-2", "vm-worker-2", AssetTypeVM, "基础设施", "tenant-002", "running", 35.1, 55.8, 30000000, 18000000, 0.003, 60, 0},
		{"vm-worker-3", "vm-worker-3", AssetTypeVM, "基础设施", "tenant-003", "running", 42.7, 48.3, 25000000, 15000000, 0.002, 45, 0},
		{"pod-order-1", "order-service-pod-1", AssetTypePod, "订单中心", "tenant-001", "error", 88.0, 75.0, 10000000, 8000000, 0.12, 500, 1},
		{"pod-order-2", "order-service-pod-2", AssetTypePod, "订单中心", "tenant-001", "running", 52.3, 60.5, 8000000, 6000000, 0.008, 95, 0},
		{"pod-payment-1", "payment-service-pod-1", AssetTypePod, "支付中心", "tenant-002", "warning", 71.5, 68.2, 12000000, 9000000, 0.03, 200, 1},
		{"pod-payment-2", "payment-service-pod-2", AssetTypePod, "支付中心", "tenant-002", "running", 38.9, 45.6, 9000000, 7000000, 0.005, 85, 0},
		{"pod-user-1", "user-service-pod-1", AssetTypePod, "用户中心", "tenant-003", "running", 55.2, 58.3, 15000000, 10000000, 0.006, 70, 0},
		{"pod-user-2", "user-service-pod-2", AssetTypePod, "用户中心", "tenant-003", "running", 41.8, 50.1, 12000000, 8000000, 0.004, 65, 0},
		{"pod-product-1", "product-service-pod-1", AssetTypePod, "商品中心", "tenant-004", "running", 48.5, 55.7, 20000000, 15000000, 0.007, 110, 0},
		{"pod-product-2", "product-service-pod-2", AssetTypePod, "商品中心", "tenant-004", "running", 36.2, 42.3, 18000000, 13000000, 0.003, 75, 0},
		{"pod-promo-1", "promo-service-pod-1", AssetTypePod, "营销中心", "tenant-005", "running", 29.8, 38.5, 5000000, 3000000, 0.002, 50, 0},
		{"container-redis-1", "redis-master", AssetTypeContainer, "基础设施", "tenant-001", "running", 25.3, 72.1, 8000000, 6000000, 0.001, 5, 0},
		{"container-redis-2", "redis-slave-1", AssetTypeContainer, "基础设施", "tenant-002", "running", 18.7, 65.4, 6000000, 4500000, 0.001, 4, 0},
		{"container-mysql-1", "mysql-master", AssetTypeContainer, "基础设施", "tenant-001", "warning", 60.2, 78.5, 15000000, 12000000, 0.015, 35, 0},
		{"container-mysql-2", "mysql-slave-1", AssetTypeContainer, "基础设施", "tenant-002", "running", 42.1, 70.3, 10000000, 8000000, 0.008, 28, 0},
		{"container-kafka-1", "kafka-broker-1", AssetTypeContainer, "基础设施", "tenant-001", "running", 35.6, 55.2, 25000000, 20000000, 0.002, 15, 0},
	}
	
	var items []*AssetItem
	for _, a := range mockAssets {
		item := &AssetItem{
			ID:            a.id,
			Name:          a.name,
			Type:          a.typ,
			TypeName:      getAssetTypeName(a.typ),
			Status:        a.status,
			BusinessGroup: a.group,
			TenantID:      a.tenantID,
			TenantName:    m.tenantMap[a.tenantID],
			CPUUsage:      a.cpu,
			MemoryUsage:   a.mem,
			NetworkIn:     a.netIn,
			NetworkOut:    a.netOut,
			ErrorRate:     a.errRate,
			ResponseTime:  a.respTime,
			AlertCount:    a.alerts,
			Labels: map[string]string{
				"env": "production",
				"team": a.group,
			},
			LastSeen:  time.Now(),
			CreatedAt: time.Now().Add(-time.Duration(hashStr(a.id)%30) * 24 * time.Hour),
		}
		if a.alerts > 0 {
			item.CriticalCount = a.alerts
		}
		items = append(items, item)
	}
	
	return items
}

// calculateSummary 计算汇总
func (m *AssetPageManager) calculateSummary(assets []*AssetItem) AssetSummary {
	summary := AssetSummary{TotalAssets: len(assets)}
	
	var totalCPU, totalMem float64
	var totalAlerts int
	
	for _, a := range assets {
		switch a.Status {
		case "running":
			summary.HealthyAssets++
		case "warning":
			summary.WarningAssets++
		case "error":
			summary.ErrorAssets++
		case "offline":
			summary.OfflineAssets++
		}
		
		totalCPU += a.CPUUsage
		totalMem += a.MemoryUsage
		totalAlerts += a.AlertCount
	}
	
	if len(assets) > 0 {
		summary.AvgCPUUsage = totalCPU / float64(len(assets))
		summary.AvgMemoryUsage = totalMem / float64(len(assets))
	}
	
	// 告警统计
	for _, alert := range m.alerts {
		summary.TotalAlerts++
		switch alert.Severity {
		case "critical":
			summary.CriticalAlerts++
		case "warning":
			summary.WarningAlerts++
		default:
			summary.InfoAlerts++
		}
	}
	
	return summary
}

// groupByType 按类型分组
func (m *AssetPageManager) groupByType(assets []*AssetItem) []*AssetTypeGroup {
	groups := make(map[string][]*AssetItem)
	
	for _, a := range assets {
		typ := string(a.Type)
		groups[typ] = append(groups[typ], a)
	}
	
	typeInfo := map[string]struct {
		Name string
		Icon string
		Color string
	}{
		"host":     {"主机", "🖥️", "#4299e1"},
		"vm":       {"虚拟机", "💻", "#ed8936"},
		"container": {"容器", "📦", "#48bb78"},
		"pod":      {"Pod", "🟢", "#9f7aea"},
		"service":  {"服务", "⚙️", "#63b3ed"},
		"node":     {"节点", "🖥️", "#4299e1"},
	}
	
	var result []*AssetTypeGroup
	for typ, items := range groups {
		info := typeInfo[typ]
		if info.Name == "" {
			info = struct {
				Name  string
				Icon  string
				Color string
			}{typ, "❓", "#718096"}
		}
		
		result = append(result, &AssetTypeGroup{
			Type:     typ,
			TypeName: info.Name,
			Icon:     info.Icon,
			Color:    info.Color,
			Count:    len(items),
			Assets:   items,
		})
	}
	
	// 按数量排序
	sort.Slice(result, func(i, j int) bool {
		return result[i].Count > result[j].Count
	})
	
	return result
}

// filterAlerts 过滤告警
func (m *AssetPageManager) filterAlerts(config *AssetViewConfig) []*AssetAlert {
	if config.Mode == ViewTenant && config.TenantID != "" {
		// 租户视图：只显示该租户的告警
		var filtered []*AssetAlert
		for _, alert := range m.alerts {
			asset := m.findAsset(alert.AssetID)
			if asset != nil && asset.TenantID == config.TenantID {
				filtered = append(filtered, alert)
			}
		}
		return filtered
	}
	return m.alerts
}

// getTopAlertAssets 获取告警最多的资产
func (m *AssetPageManager) getTopAlertAssets(assets []*AssetItem, alerts []*AssetAlert) []*AssetAlertItem {
	alertCountMap := make(map[string]*AssetAlertItem)
	
	for _, a := range assets {
		alertCountMap[a.ID] = &AssetAlertItem{
			AssetID:   a.ID,
			AssetName: a.Name,
			AssetType: string(a.Type),
		}
	}
	
	for _, alert := range alerts {
		if item, ok := alertCountMap[alert.AssetID]; ok {
			switch alert.Severity {
			case "critical":
				item.CriticalCount++
			case "warning":
				item.WarningCount++
			}
			item.TotalAlerts++
		}
	}
	
	var result []*AssetAlertItem
	for _, item := range alertCountMap {
		if item.TotalAlerts > 0 {
			result = append(result, item)
		}
	}
	
	sort.Slice(result, func(i, j int) bool {
		return result[i].TotalAlerts > result[j].TotalAlerts
	})
	
	if len(result) > 10 {
		result = result[:10]
	}
	
	return result
}

// sortAssets 排序资产
func (m *AssetPageManager) sortAssets(assets []*AssetItem, sortBy, sortOrder string) []*AssetItem {
	if sortBy == "" {
		sortBy = "cpu_usage"
	}
	if sortOrder == "" {
		sortOrder = "desc"
	}
	
	sort.SliceStable(assets, func(i, j int) bool {
		var less bool
		switch sortBy {
		case "cpu_usage":
			less = assets[i].CPUUsage < assets[j].CPUUsage
		case "memory_usage":
			less = assets[i].MemoryUsage < assets[j].MemoryUsage
		case "error_rate":
			less = assets[i].ErrorRate < assets[j].ErrorRate
		case "response_time":
			less = assets[i].ResponseTime < assets[j].ResponseTime
		case "alert_count":
			less = assets[i].AlertCount < assets[j].AlertCount
		case "name":
			less = assets[i].Name < assets[j].Name
		default:
			less = assets[i].CPUUsage < assets[j].CPUUsage
		}
		
		if sortOrder == "desc" {
			return !less
		}
		return less
	})
	
	return assets
}

// findAsset 查找资产
func (m *AssetPageManager) findAsset(assetID string) *AssetItem {
	if m.collector != nil {
		metrics := m.collector.GetLatestMetrics(assetID)
		if metrics != nil {
			return m.metricsToAssetItem(assetID, metrics)
		}
	}
	
	// 从模拟数据查找
	allAssets := m.generateMockAssets(&AssetViewConfig{Mode: ViewAdmin})
	for _, a := range allAssets {
		if a.ID == assetID {
			return a
		}
	}
	return nil
}

// getAssetAlerts 获取资产告警
func (m *AssetPageManager) getAssetAlerts(assetID string) []*AssetAlert {
	var alerts []*AssetAlert
	for _, alert := range m.alerts {
		if alert.AssetID == assetID {
			alerts = append(alerts, alert)
		}
	}
	return alerts
}

// getTenantList 获取租户列表
func (m *AssetPageManager) getTenantList() []TenantOption {
	var tenants []TenantOption
	for id, name := range m.tenantMap {
		tenants = append(tenants, TenantOption{ID: id, Name: name})
	}
	sort.Slice(tenants, func(i, j int) bool {
		return tenants[i].Name < tenants[j].Name
	})
	return tenants
}

// drilldownNetwork 网络指标下钻
func (m *AssetPageManager) drilldownNetwork(metrics *AssetMetrics) *DrilldownNetworkResult {
	result := &DrilldownNetworkResult{}
	
	if metrics.Network != nil {
		result.TCPStates = metrics.Network.TCPStates
		result.TotalConnections = metrics.Network.Connections
		result.TotalRetransmits = metrics.Network.Retransmits
		
		result.Interfaces = make(map[string]*InterfaceDrilldown)
		for name, iface := range metrics.Network.Interfaces {
			result.Interfaces[name] = &InterfaceDrilldown{
				Name:        iface.Name,
				BytesSent:   iface.BytesSent,
				BytesRecv:   iface.BytesRecv,
				PacketsSent: iface.PacketsSent,
				PacketsRecv: iface.PacketsRecv,
				Errors:      iface.Errors,
				Drops:       iface.Drops,
			}
		}
	}
	
	return result
}

// drilldownApplication 应用指标下钻
func (m *AssetPageManager) drilldownApplication(metrics *AssetMetrics) *DrilldownAppResult {
	result := &DrilldownAppResult{}
	
	if metrics.Application != nil {
		result.AvgCPUUsage = metrics.Application.CPUUsage
		result.AvgMemoryUsage = metrics.Application.MemoryUsage
		result.AvgResponseTime = metrics.Application.ResponseTime
		result.AvgErrorRate = metrics.Application.ErrorRate
		result.AvgThroughput = metrics.Application.Throughput
		result.ProcessCount = len(metrics.Application.Processes)
		
		result.Processes = make(map[uint32]*ProcessDrilldown)
		for _, proc := range metrics.Application.Processes {
			result.Processes[proc.PID] = &ProcessDrilldown{
				PID:         proc.PID,
				Name:        proc.Name,
				CPUUsage:    proc.CPUUsage,
				MemoryRSS:   proc.MemoryRSS,
				Threads:     proc.Threads,
				OpenFiles:   proc.OpenFiles,
				SampleCount: 1,
			}
		}
	}
	
	return result
}

// drilldownSystem 系统指标下钻
func (m *AssetPageManager) drilldownSystem(metrics *AssetMetrics) *DrilldownSystemResult {
	result := &DrilldownSystemResult{}
	
	if metrics.System != nil {
		result.AvgCPUUsage = metrics.System.CPUUsage
		result.AvgMemoryUsage = metrics.System.MemoryUsage
		result.MemoryTotal = metrics.System.MemoryTotal
		result.MemoryUsed = metrics.System.MemoryUsed
		result.DiskTotal = metrics.System.DiskTotal
		result.DiskUsed = metrics.System.DiskUsed
		result.AvgLoad1 = metrics.System.Load1
		result.AvgLoad5 = metrics.System.Load5
		result.AvgLoad15 = metrics.System.Load15
		result.OS = "Linux"
		result.KernelVersion = "5.10.0"
		result.Uptime = "30d 12h 45m"
	}
	
	return result
}

// drilldownSQL SQL指标下钻（模拟）
func (m *AssetPageManager) drilldownSQL(assetID string) *DrilldownSQLResult {
	result := &DrilldownSQLResult{
		TotalQueries:  15000,
		SlowQueries:   25,
		ErrorQueries:  3,
		AvgLatency:    12.5,
		P95Latency:    85.3,
		P99Latency:    250.8,
		TopSlowQueries: []*SQLQueryItem{
			{Query: "SELECT * FROM orders WHERE status = ? ORDER BY created_at DESC LIMIT 1000", Database: "order_db", Calls: 500, AvgTime: 320.5, MaxTime: 1200.3, RowsExamined: 500000},
			{Query: "SELECT o.*, u.name FROM orders o JOIN users u ON o.user_id = u.id WHERE o.amount > ?", Database: "order_db", Calls: 300, AvgTime: 180.2, MaxTime: 850.1, RowsExamined: 200000},
			{Query: "SELECT * FROM products WHERE category_id IN (?, ?, ?) AND status = 'active'", Database: "product_db", Calls: 200, AvgTime: 150.8, MaxTime: 600.5, RowsExamined: 150000},
		},
		TopErrorQueries: []*SQLQueryItem{
			{Query: "INSERT INTO payment_records (order_id, amount) VALUES (?, ?)", Database: "payment_db", Calls: 2, AvgTime: 50.0, MaxTime: 120.0, RowsExamined: 0},
			{Query: "UPDATE inventory SET stock = stock - ? WHERE product_id = ?", Database: "product_db", Calls: 1, AvgTime: 80.0, MaxTime: 80.0, RowsExamined: 1000},
		},
	}
	
	return result
}

// ==================== 工具函数 ====================

func getAssetTypeName(typ AssetType) string {
	names := map[AssetType]string{
		AssetTypePod:       "Pod",
		AssetTypeVM:        "虚拟机",
		AssetTypePhysical:  "物理机",
		AssetTypeContainer: "容器",
		AssetTypeService:   "服务",
		AssetTypeNode:      "节点",
	}
	if name, ok := names[typ]; ok {
		return name
	}
	return string(typ)
}

func containsStr(slice []string, item string) bool {
	for _, s := range slice {
		if s == item {
			return true
		}
	}
	return false
}

func hashStr(s string) int {
	h := 0
	for _, c := range s {
		h = h*31 + int(c)
	}
	if h < 0 {
		h = -h
	}
	return h
}

// ==================== HTTP Handler ====================

// AssetPageHandler 资产页面HTTP处理器
type AssetPageHandler struct {
	manager *AssetPageManager
}

// NewAssetPageHandler 创建处理器
func NewAssetPageHandler(manager *AssetPageManager) *AssetPageHandler {
	return &AssetPageHandler{manager: manager}
}

// RegisterRoutes 注册路由
func (h *AssetPageHandler) RegisterRoutes(mux *http.ServeMux) {
	mux.HandleFunc("/api/v1/assets/page/dashboard", h.handleDashboard)
	mux.HandleFunc("/api/v1/assets/page/list", h.handleList)
	mux.HandleFunc("/api/v1/assets/page/detail/", h.handleDetail)
	mux.HandleFunc("/api/v1/assets/page/filters", h.handleFilters)
	mux.HandleFunc("/assets", h.handlePage)
}

// handleDashboard 处理仪表盘请求
func (h *AssetPageHandler) handleDashboard(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	
	config := &AssetViewConfig{
		Mode:        ViewMode(r.URL.Query().Get("mode")),
		TenantID:    r.URL.Query().Get("tenant_id"),
	}
	if config.Mode == "" {
		config.Mode = ViewAdmin
	}
	
	dashboard := h.manager.GetDashboard(config)
	
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(dashboard)
}

// handleList 处理列表请求
func (h *AssetPageHandler) handleList(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	
	config := &AssetViewConfig{
		Mode:          ViewMode(r.URL.Query().Get("mode")),
		TenantID:      r.URL.Query().Get("tenant_id"),
		Page:          1,
		PageSize:      20,
		SortBy:        r.URL.Query().Get("sort_by"),
		SortOrder:     r.URL.Query().Get("sort_order"),
	}
	
	if config.Mode == "" {
		config.Mode = ViewAdmin
	}
	
	// 解析数组参数
	config.Types = r.URL.Query()["type"]
	config.Statuses = r.URL.Query()["status"]
	config.BusinessGroups = r.URL.Query()["business_group"]
	config.Labels = r.URL.Query()["label"]
	
	result := h.manager.GetAssetList(config)
	
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(result)
}

// handleDetail 处理详情请求
func (h *AssetPageHandler) handleDetail(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	
	assetID := strings.TrimPrefix(r.URL.Path, "/api/v1/assets/page/detail/")
	dimensions := r.URL.Query()["dimension"]
	
	if len(dimensions) == 0 {
		dimensions = []string{"all"}
	}
	
	detail := h.manager.GetAssetDetail(assetID, dimensions)
	if detail == nil {
		http.Error(w, "Asset not found", http.StatusNotFound)
		return
	}
	
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(detail)
}

// handleFilters 处理筛选选项请求
func (h *AssetPageHandler) handleFilters(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	
	options := h.manager.GetFilterOptions()
	
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(options)
}

// handlePage 处理页面请求
func (h *AssetPageHandler) handlePage(w http.ResponseWriter, r *http.Request) {
	http.ServeFile(w, r, "web/assets.html")
}

// ==================== MetricsCollector 扩展方法 ====================

// GetLatestMetrics 获取所有资产的最新指标
func (c *MetricsCollector) GetLatestMetrics() map[string]*AssetMetrics {
	c.mu.RLock()
	defer c.mu.RUnlock()
	
	result := make(map[string]*AssetMetrics)
	for id, metrics := range c.metrics {
		result[id] = metrics
	}
	return result
}

// GetAllLatestMetrics 获取所有资产的最新指标（包含历史）
func (c *MetricsCollector) GetAllLatestMetrics() map[string]*AssetMetrics {
	c.mu.RLock()
	defer c.mu.RUnlock()
	
	result := make(map[string]*AssetMetrics)
	for id, metrics := range c.metrics {
		result[id] = metrics
	}
	return result
}
