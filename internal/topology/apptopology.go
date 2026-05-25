// Package topology 应用拓扑模块
// 提供服务调用依赖图谱、性能指标关联、快照对比功能
package topology

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"sort"
	"sync"
	"time"
)

// AppEntityType 应用实体类型
type AppEntityType string

const (
	AppTypeService    AppEntityType = "service"     // 微服务
	AppTypeAPI        AppEntityType = "api"         // API接口
	AppTypeDatabase   AppEntityType = "database"    // 数据库
	AppTypeCache      AppEntityType = "cache"       // 缓存
	AppTypeMQ         AppEntityType = "mq"          // 消息队列
	AppTypeGateway    AppEntityType = "gateway"     // 网关
	AppTypeExternal   AppEntityType = "external"    // 外部服务
)

// ProtocolType 协议类型
type ProtocolType string

const (
	ProtocolHTTP    ProtocolType = "http"
	ProtocolHTTPS   ProtocolType = "https"
	ProtocolGRPC    ProtocolType = "grpc"
	ProtocolDubbo   ProtocolType = "dubbo"
	ProtocolMySQL   ProtocolType = "mysql"
	ProtocolRedis   ProtocolType = "redis"
	ProtocolKafka   ProtocolType = "kafka"
	ProtocolDNS     ProtocolType = "dns"
)

// AppEntity 应用拓扑实体
type AppEntity struct {
	ID           string                 `json:"id"`
	Name         string                 `json:"name"`
	Type         AppEntityType          `json:"type"`
	Namespace    string                 `json:"namespace"`
	Version      string                 `json:"version"`
	Language     string                 `json:"language"`      // Java/Go/Python/Node.js
	BusinessGroup string                `json:"business_group"` // 业务分组
	Team         string                 `json:"team"`          // 所属团队
	Labels       map[string]string      `json:"labels"`
	Annotations  map[string]string      `json:"annotations"`
	Status       string                 `json:"status"`        // healthy/warning/error
	
	// 性能指标
	AppMetrics   AppMetrics             `json:"app_metrics"`   // 应用性能指标
	NetMetrics   NetMetrics             `json:"net_metrics"`   // 网络性能指标
	
	// 位置信息（用于布局）
	X            float64                `json:"x"`
	Y            float64                `json:"y"`
	
	CreatedAt    time.Time              `json:"created_at"`
	UpdatedAt    time.Time              `json:"updated_at"`
}

// AppMetrics 应用性能指标
type AppMetrics struct {
	// 吞吐量
	QPS          float64 `json:"qps"`           // 每秒查询率
	TPS          float64 `json:"tps"`           // 每秒事务数
	Throughput   float64 `json:"throughput"`    // 吞吐量(req/s)
	
	// 响应时间
	Latency      float64 `json:"latency"`       // 平均响应时间(ms)
	LatencyP50   float64 `json:"latency_p50"`   // P50
	LatencyP95   float64 `json:"latency_p95"`   // P95
	LatencyP99   float64 `json:"latency_p99"`   // P99
	
	// 错误率
	ErrorRate    float64 `json:"error_rate"`    // 错误率
	ErrorCount   int64   `json:"error_count"`   // 错误数
	SuccessRate  float64 `json:"success_rate"`  // 成功率
	
	// 饱和度
	CPUUsage     float64 `json:"cpu_usage"`     // CPU使用率
	MemoryUsage  float64 `json:"memory_usage"`  // 内存使用率
	ThreadCount  int     `json:"thread_count"`  // 线程数
	GoroutineCount int   `json:"goroutine_count"` // Goroutine数
	
	// 调用统计
	CallCount    int64   `json:"call_count"`    // 调用次数
	CallerCount  int     `json:"caller_count"`  // 调用方数量
	CalleeCount  int     `json:"callee_count"`  // 被调用方数量
}

// NetMetrics 网络性能指标
type NetMetrics struct {
	// 连接指标
	ConnTotal    int64   `json:"conn_total"`    // 总连接数
	ConnActive   int64   `json:"conn_active"`   // 活跃连接
	ConnIdle     int64   `json:"conn_idle"`     // 空闲连接
	ConnFailed   int64   `json:"conn_failed"`   // 失败连接
	
	// 流量指标
	BytesSent    uint64  `json:"bytes_sent"`    // 发送字节
	BytesRecv    uint64  `json:"bytes_recv"`    // 接收字节
	PacketsSent  uint64  `json:"packets_sent"`  // 发送包数
	PacketsRecv  uint64  `json:"packets_recv"`  // 接收包数
	
	// 网络质量
	RetransRate  float64 `json:"retrans_rate"`  // 重传率
	PacketLoss   float64 `json:"packet_loss"`   // 丢包率
	RTT          float64 `json:"rtt"`           // 往返时延
	Jitter       float64 `json:"jitter"`        // 抖动
	
	// TCP指标
	TCPEstablished int64 `json:"tcp_established"` // ESTABLISHED连接
	TCPTimeWait    int64 `json:"tcp_timewait"`    // TIME_WAIT连接
	TCPCloseWait   int64 `json:"tcp_closewait"`   // CLOSE_WAIT连接
}

// AppLink 应用调用链路
type AppLink struct {
	ID           string            `json:"id"`
	SourceID     string            `json:"source_id"`
	TargetID     string            `json:"target_id"`
	Source       *AppEntity        `json:"source,omitempty"`
	Target       *AppEntity        `json:"target,omitempty"`
	
	// 调用信息
	Protocol     ProtocolType      `json:"protocol"`      // 协议类型
	Method       string            `json:"method"`        // HTTP方法/操作类型
	Path         string            `json:"path"`          // 调用路径
	
	// 性能指标
	Metrics      LinkMetrics       `json:"metrics"`
	
	// 状态
	Status       string            `json:"status"`        // normal/warning/error
	Direction    string            `json:"direction"`     // unidirectional/bidirectional
	
	// 元数据
	Labels       map[string]string `json:"labels"`
	CreatedAt    time.Time         `json:"created_at"`
	UpdatedAt    time.Time         `json:"updated_at"`
}

// AppTopology 应用拓扑
type AppTopology struct {
	Entities        []*AppEntity       `json:"entities"`
	Links           []*AppLink         `json:"links"`
	
	// 分组信息
	BusinessGroups  []string           `json:"business_groups"`  // 业务分组列表
	Teams           []string           `json:"teams"`            // 团队列表
	Protocols       []string           `json:"protocols"`        // 协议类型列表
	Namespaces      []string           `json:"namespaces"`       // 命名空间列表
	
	// 统计
	Stats           AppTopologyStats   `json:"stats"`
	
	// 元数据
	Timestamp       time.Time          `json:"timestamp"`
	TimeRange       string             `json:"time_range"`
	Layout          string             `json:"layout"`           // 当前布局类型
}

// AppTopologyStats 应用拓扑统计
type AppTopologyStats struct {
	TotalServices   int                `json:"total_services"`
	TotalLinks      int                `json:"total_links"`
	HealthyServices int                `json:"healthy_services"`
	WarningServices int                `json:"warning_services"`
	ErrorServices   int                `json:"error_services"`
	
	// 按类型统计
	ServiceByType   map[string]int     `json:"service_by_type"`
	LinkByProtocol  map[string]int     `json:"link_by_protocol"`
	
	// 性能统计
	AvgLatency      float64            `json:"avg_latency"`
	AvgErrorRate    float64            `json:"avg_error_rate"`
	TotalQPS        float64            `json:"total_qps"`
}

// LayoutType 布局类型
type LayoutType string

const (
	LayoutHorizontal LayoutType = "horizontal"  // 横向布局
	LayoutVertical   LayoutType = "vertical"    // 纵向布局
	LayoutForce      LayoutType = "force"       // 力导向布局
	LayoutCircular   LayoutType = "circular"    // 环形布局
	LayoutHierarchical LayoutType = "hierarchical" // 层次布局
)

// AppTopologyManager 应用拓扑管理器
type AppTopologyManager struct {
	entities        map[string]*AppEntity
	links           map[string]*AppLink
	entityByGroup   map[string][]*AppEntity
	entityByTeam    map[string][]*AppEntity
	mu              sync.RWMutex
	
	discovery       *DiscoveryEngine
	tracer          *PathTracer
	storage         *TopologyStorage
	
	// 快照管理
	snapshots       map[string]*TopologySnapshot
	snapshotMu      sync.RWMutex
}

// TopologySnapshot 拓扑快照
type TopologySnapshot struct {
	ID          string            `json:"id"`
	Name        string            `json:"name"`
	Description string            `json:"description"`
	Topology    *AppTopology      `json:"topology"`
	Tags        []string          `json:"tags"`
	CreatedAt   time.Time         `json:"created_at"`
	CreatedBy   string            `json:"created_by"`
}

// SnapshotDiff 快照对比结果
type SnapshotDiff struct {
	BaseSnapshot    string                 `json:"base_snapshot"`
	CompareSnapshot string                 `json:"compare_snapshot"`
	
	// 实体变化
	AddedEntities   []*AppEntity           `json:"added_entities"`
	RemovedEntities []*AppEntity           `json:"removed_entities"`
	ModifiedEntities []*EntityDiff         `json:"modified_entities"`
	
	// 链路变化
	AddedLinks      []*AppLink             `json:"added_links"`
	RemovedLinks    []*AppLink             `json:"removed_links"`
	ModifiedLinks   []*LinkDiff            `json:"modified_links"`
	
	// 性能变化
	PerformanceChanges map[string]*PerfChange `json:"performance_changes"`
	
	Summary         DiffSummary            `json:"summary"`
}

// EntityDiff 实体差异
type EntityDiff struct {
	EntityID    string                 `json:"entity_id"`
	EntityName  string                 `json:"entity_name"`
	FieldChanges map[string]FieldChange `json:"field_changes"`
}

// LinkDiff 链路差异
type LinkDiff struct {
	LinkID      string                 `json:"link_id"`
	SourceName  string                 `json:"source_name"`
	TargetName  string                 `json:"target_name"`
	FieldChanges map[string]FieldChange `json:"field_changes"`
}

// FieldChange 字段变化
type FieldChange struct {
	OldValue    interface{} `json:"old_value"`
	NewValue    interface{} `json:"new_value"`
	ChangeType  string      `json:"change_type"` // increased/decreased/changed
}

// PerfChange 性能变化
type PerfChange struct {
	Metric      string      `json:"metric"`
	OldValue    float64     `json:"old_value"`
	NewValue    float64     `json:"new_value"`
	ChangePct   float64     `json:"change_pct"`
	Trend       string      `json:"trend"`       // up/down/stable
}

// DiffSummary 差异摘要
type DiffSummary struct {
	TotalChanges        int     `json:"total_changes"`
	EntityChanges       int     `json:"entity_changes"`
	LinkChanges         int     `json:"link_changes"`
	PerformanceChanges  int     `json:"performance_changes"`
	CriticalChanges     int     `json:"critical_changes"`    // 关键变化数量
}

// NewAppTopologyManager 创建应用拓扑管理器
func NewAppTopologyManager(discovery *DiscoveryEngine, tracer *PathTracer, storage *TopologyStorage) *AppTopologyManager {
	return &AppTopologyManager{
		entities:      make(map[string]*AppEntity),
		links:         make(map[string]*AppLink),
		entityByGroup: make(map[string][]*AppEntity),
		entityByTeam:  make(map[string][]*AppEntity),
		discovery:     discovery,
		tracer:        tracer,
		storage:       storage,
		snapshots:     make(map[string]*TopologySnapshot),
	}
}

// BuildAppTopology 构建应用拓扑
func (m *AppTopologyManager) BuildAppTopology(filters *TopologyFilters) *AppTopology {
	m.mu.RLock()
	defer m.mu.RUnlock()

	topology := &AppTopology{
		Entities:       make([]*AppEntity, 0),
		Links:          make([]*AppLink, 0),
		BusinessGroups: make([]string, 0),
		Teams:          make([]string, 0),
		Protocols:      make([]string, 0),
		Namespaces:     make([]string, 0),
		Timestamp:      time.Now(),
		TimeRange:      filters.TimeRange,
		Layout:         string(filters.Layout),
	}

	// 收集分组信息
	groupSet := make(map[string]bool)
	teamSet := make(map[string]bool)
	protocolSet := make(map[string]bool)
	nsSet := make(map[string]bool)

	// 过滤实体
	for _, entity := range m.entities {
		if m.matchFilters(entity, filters) {
			topology.Entities = append(topology.Entities, entity)
			
			groupSet[entity.BusinessGroup] = true
			teamSet[entity.Team] = true
			nsSet[entity.Namespace] = true
		}
	}

	// 过滤链路
	for _, link := range m.links {
		// 检查源和目标是否都在过滤后的实体中
		sourceExists := false
		targetExists := false
		
		for _, entity := range topology.Entities {
			if entity.ID == link.SourceID {
				sourceExists = true
				link.Source = entity
			}
			if entity.ID == link.TargetID {
				targetExists = true
				link.Target = entity
			}
		}
		
		if sourceExists && targetExists {
			// 协议过滤
			if len(filters.Protocols) == 0 || contains(filters.Protocols, string(link.Protocol)) {
				topology.Links = append(topology.Links, link)
				protocolSet[string(link.Protocol)] = true
			}
		}
	}

	// 转换为切片
	for g := range groupSet {
		topology.BusinessGroups = append(topology.BusinessGroups, g)
	}
	for t := range teamSet {
		topology.Teams = append(topology.Teams, t)
	}
	for p := range protocolSet {
		topology.Protocols = append(topology.Protocols, p)
	}
	for ns := range nsSet {
		topology.Namespaces = append(topology.Namespaces, ns)
	}

	// 排序
	sort.Strings(topology.BusinessGroups)
	sort.Strings(topology.Teams)
	sort.Strings(topology.Protocols)
	sort.Strings(topology.Namespaces)

	// 计算统计
	topology.Stats = m.calculateStats(topology)

	// 应用布局
	m.applyLayout(topology, filters.Layout)

	return topology
}

// TopologyFilters 拓扑过滤器
type TopologyFilters struct {
	BusinessGroups []string   `json:"business_groups"`  // 业务分组过滤
	Teams          []string   `json:"teams"`            // 团队过滤
	Protocols      []string   `json:"protocols"`        // 协议过滤
	Namespaces     []string   `json:"namespaces"`       // 命名空间过滤
	ServiceTypes   []string   `json:"service_types"`    // 服务类型过滤
	Status         []string   `json:"status"`           // 状态过滤
	TimeRange      string     `json:"time_range"`       // 时间范围
	Layout         LayoutType `json:"layout"`           // 布局类型
	SearchQuery    string     `json:"search_query"`     // 搜索关键词
}

// matchFilters 检查实体是否匹配过滤器
func (m *AppTopologyManager) matchFilters(entity *AppEntity, filters *TopologyFilters) bool {
	// 业务分组过滤
	if len(filters.BusinessGroups) > 0 && !contains(filters.BusinessGroups, entity.BusinessGroup) {
		return false
	}
	
	// 团队过滤
	if len(filters.Teams) > 0 && !contains(filters.Teams, entity.Team) {
		return false
	}
	
	// 命名空间过滤
	if len(filters.Namespaces) > 0 && !contains(filters.Namespaces, entity.Namespace) {
		return false
	}
	
	// 服务类型过滤
	if len(filters.ServiceTypes) > 0 && !contains(filters.ServiceTypes, string(entity.Type)) {
		return false
	}
	
	// 状态过滤
	if len(filters.Status) > 0 && !contains(filters.Status, entity.Status) {
		return false
	}
	
	// 搜索过滤
	if filters.SearchQuery != "" {
		query := filters.SearchQuery
		if !containsString(entity.Name, query) && 
		   !containsString(entity.ID, query) &&
		   !containsString(entity.BusinessGroup, query) {
			return false
		}
	}
	
	return true
}

// applyLayout 应用布局算法
func (m *AppTopologyManager) applyLayout(topology *AppTopology, layout LayoutType) {
	if len(topology.Entities) == 0 {
		return
	}

	switch layout {
	case LayoutHorizontal:
		m.applyHorizontalLayout(topology)
	case LayoutVertical:
		m.applyVerticalLayout(topology)
	case LayoutCircular:
		m.applyCircularLayout(topology)
	case LayoutHierarchical:
		m.applyHierarchicalLayout(topology)
	default:
		// 力导向布局由前端处理，这里设置初始位置
		m.applyForceLayout(topology)
	}
}

// applyHorizontalLayout 横向布局
func (m *AppTopologyManager) applyHorizontalLayout(topology *AppTopology) {
	// 按业务分组分组
	groups := make(map[string][]*AppEntity)
	for _, entity := range topology.Entities {
		groups[entity.BusinessGroup] = append(groups[entity.BusinessGroup], entity)
	}
	
	groupNames := make([]string, 0, len(groups))
	for name := range groups {
		groupNames = append(groupNames, name)
	}
	sort.Strings(groupNames)
	
	groupWidth := 300.0
	nodeHeight := 80.0
	
	for i, groupName := range groupNames {
		x := float64(i) * groupWidth + 100
		entities := groups[groupName]
		
		for j, entity := range entities {
			entity.X = x
			entity.Y = float64(j) * nodeHeight + 100
		}
	}
}

// applyVerticalLayout 纵向布局
func (m *AppTopologyManager) applyVerticalLayout(topology *AppTopology) {
	// 按类型分层
	layers := make(map[AppEntityType][]*AppEntity)
	for _, entity := range topology.Entities {
		layers[entity.Type] = append(layers[entity.Type], entity)
	}
	
	layerOrder := []AppEntityType{AppTypeGateway, AppTypeService, AppTypeAPI, AppTypeMQ, AppTypeCache, AppTypeDatabase, AppTypeExternal}
	
	nodeWidth := 180.0
	layerHeight := 150.0
	
	for layerIdx, entityType := range layerOrder {
		entities := layers[entityType]
		y := float64(layerIdx) * layerHeight + 100
		
		for i, entity := range entities {
			entity.X = float64(i) * nodeWidth + 100
			entity.Y = y
		}
	}
}

// applyCircularLayout 环形布局
func (m *AppTopologyManager) applyCircularLayout(topology *AppTopology) {
	centerX, centerY := 400.0, 400.0
	radius := 300.0
	
	count := len(topology.Entities)
	for i, entity := range topology.Entities {
		angle := float64(i) * 2 * 3.14159 / float64(count)
		entity.X = centerX + radius * cos(angle)
		entity.Y = centerY + radius * sin(angle)
	}
}

// applyHierarchicalLayout 层次布局
func (m *AppTopologyManager) applyHierarchicalLayout(topology *AppTopology) {
	// 计算每个节点的层级（基于调用关系）
	levels := m.calculateHierarchyLevels(topology)
	
	levelHeight := 120.0
	nodeWidth := 160.0
	
	// 按层级分组
	levelGroups := make(map[int][]*AppEntity)
	for _, entity := range topology.Entities {
		level := levels[entity.ID]
		levelGroups[level] = append(levelGroups[level], entity)
	}
	
	// 布局
	for level, entities := range levelGroups {
		y := float64(level) * levelHeight + 100
		startX := 100.0
		
		for i, entity := range entities {
			entity.X = startX + float64(i) * nodeWidth
			entity.Y = y
		}
	}
}

// calculateHierarchyLevels 计算层次级别
func (m *AppTopologyManager) calculateHierarchyLevels(topology *AppTopology) map[string]int {
	levels := make(map[string]int)
	
	// 初始化
	for _, entity := range topology.Entities {
		levels[entity.ID] = 0
	}
	
	// 迭代计算（简化版拓扑排序）
	changed := true
	for changed {
		changed = false
		for _, link := range topology.Links {
			sourceLevel := levels[link.SourceID]
			targetLevel := levels[link.TargetID]
			
			if targetLevel <= sourceLevel {
				levels[link.TargetID] = sourceLevel + 1
				changed = true
			}
		}
	}
	
	return levels
}

// applyForceLayout 力导向布局初始位置
func (m *AppTopologyManager) applyForceLayout(topology *AppTopology) {
	// 随机初始位置，前端D3.js会处理实际布局
	for i, entity := range topology.Entities {
		entity.X = float64(i%10) * 150 + 100
		entity.Y = float64(i/10) * 100 + 100
	}
}

// calculateStats 计算拓扑统计
func (m *AppTopologyManager) calculateStats(topology *AppTopology) AppTopologyStats {
	stats := AppTopologyStats{
		TotalServices:  len(topology.Entities),
		TotalLinks:     len(topology.Links),
		ServiceByType:  make(map[string]int),
		LinkByProtocol: make(map[string]int),
	}
	
	var totalLatency float64
	var totalErrorRate float64
	var latencyCount int
	
	for _, entity := range topology.Entities {
		stats.ServiceByType[string(entity.Type)]++
		
		switch entity.Status {
		case "healthy":
			stats.HealthyServices++
		case "warning":
			stats.WarningServices++
		case "error":
			stats.ErrorServices++
		}
		
		if entity.AppMetrics.Latency > 0 {
			totalLatency += entity.AppMetrics.Latency
			latencyCount++
		}
		totalErrorRate += entity.AppMetrics.ErrorRate
		stats.TotalQPS += entity.AppMetrics.QPS
	}
	
	for _, link := range topology.Links {
		stats.LinkByProtocol[string(link.Protocol)]++
	}
	
	if latencyCount > 0 {
		stats.AvgLatency = totalLatency / float64(latencyCount)
	}
	if len(topology.Entities) > 0 {
		stats.AvgErrorRate = totalErrorRate / float64(len(topology.Entities))
	}
	
	return stats
}

// DiscoverAppTopology 发现应用拓扑
func (m *AppTopologyManager) DiscoverAppTopology() error {
	m.mu.Lock()
	defer m.mu.Unlock()

	// 清空现有数据
	m.entities = make(map[string]*AppEntity)
	m.links = make(map[string]*AppLink)
	m.entityByGroup = make(map[string][]*AppEntity)
	m.entityByTeam = make(map[string][]*AppEntity)

	// 从追踪数据发现服务和调用关系
	if m.tracer != nil {
		paths := m.tracer.GetCompletedPaths(1000)
		
		for _, path := range paths {
			m.discoverFromPath(path)
		}
	}

	// 生成模拟数据用于演示
	m.generateMockData()

	return nil
}

// discoverFromPath 从路径发现服务
func (m *AppTopologyManager) discoverFromPath(path *TracePath) {
	// 源服务
	if path.Source != nil && path.Source.EntityID != "" {
		entity := m.getOrCreateEntity(path.Source.EntityID, path.Source.EntityName)
		entity.Type = AppTypeService
		if path.Source.Namespace != "" {
			entity.Namespace = path.Source.Namespace
		}
	}
	
	// 目标服务
	if path.Destination != nil && path.Destination.EntityID != "" {
		entity := m.getOrCreateEntity(path.Destination.EntityID, path.Destination.EntityName)
		entity.Type = AppTypeService
		if path.Destination.Namespace != "" {
			entity.Namespace = path.Destination.Namespace
		}
	}
	
	// 创建调用链路
	if path.Source != nil && path.Destination != nil {
		link := m.getOrCreateLink(path.Source.EntityID, path.Destination.EntityID)
		link.Protocol = ProtocolType(path.Protocol)
		link.Method = path.Method
		link.Path = path.Path
		link.Metrics.RequestCount++
		link.Metrics.Latency = float64(path.Duration.Milliseconds())
		
		if path.Status == "failed" {
			link.Metrics.ErrorCount++
		}
		
		link.Status = m.determineLinkStatus(link.Metrics)
	}
}

// getOrCreateEntity 获取或创建实体
func (m *AppTopologyManager) getOrCreateEntity(id, name string) *AppEntity {
	if entity, ok := m.entities[id]; ok {
		return entity
	}
	
	entity := &AppEntity{
		ID:        id,
		Name:      name,
		Status:    "healthy",
		Labels:    make(map[string]string),
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}
	m.entities[id] = entity
	return entity
}

// getOrCreateLink 获取或创建链路
func (m *AppTopologyManager) getOrCreateLink(sourceID, targetID string) *AppLink {
	linkID := fmt.Sprintf("link-%s-%s", sourceID, targetID)
	
	if link, ok := m.links[linkID]; ok {
		return link
	}
	
	link := &AppLink{
		ID:        linkID,
		SourceID:  sourceID,
		TargetID:  targetID,
		Status:    "normal",
		Labels:    make(map[string]string),
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}
	m.links[linkID] = link
	return link
}

// determineLinkStatus 确定链路状态
func (m *AppTopologyManager) determineLinkStatus(metrics LinkMetrics) string {
	if metrics.ErrorRate > 0.05 || metrics.Latency > 500 {
		return "error"
	}
	if metrics.ErrorRate > 0.01 || metrics.Latency > 100 {
		return "warning"
	}
	return "normal"
}

// generateMockData 生成模拟数据
func (m *AppTopologyManager) generateMockData() {
	// 业务分组
	businessGroups := []string{"订单中心", "支付中心", "用户中心", "商品中心", "营销中心"}
	teams := []string{"交易组", "支付组", "用户组", "商品组", "营销组"}
	
	// 服务定义
	services := []struct {
		id      string
		name    string
		appType AppEntityType
		group   string
		team    string
		ns      string
		lang    string
	}{
		{"svc-gateway", "API网关", AppTypeGateway, "订单中心", "交易组", "production", "Go"},
		{"svc-order", "订单服务", AppTypeService, "订单中心", "交易组", "production", "Java"},
		{"svc-payment", "支付服务", AppTypeService, "支付中心", "支付组", "production", "Java"},
		{"svc-user", "用户服务", AppTypeService, "用户中心", "用户组", "production", "Go"},
		{"svc-product", "商品服务", AppTypeService, "商品中心", "商品组", "production", "Java"},
		{"svc-promo", "促销服务", AppTypeService, "营销中心", "营销组", "production", "Python"},
		{"svc-cart", "购物车服务", AppTypeService, "订单中心", "交易组", "production", "Go"},
		{"svc-inventory", "库存服务", AppTypeService, "商品中心", "商品组", "production", "Java"},
		{"svc-message", "消息服务", AppTypeMQ, "订单中心", "交易组", "production", "Go"},
		{"svc-redis", "缓存集群", AppTypeCache, "用户中心", "用户组", "production", "C"},
		{"svc-mysql-order", "订单库", AppTypeDatabase, "订单中心", "交易组", "production", "SQL"},
		{"svc-mysql-user", "用户库", AppTypeDatabase, "用户中心", "用户组", "production", "SQL"},
		{"svc-es", "搜索服务", AppTypeService, "商品中心", "商品组", "production", "Java"},
		{"svc-alipay", "支付宝", AppTypeExternal, "支付中心", "支付组", "external", "Java"},
		{"svc-wechat", "微信支付", AppTypeExternal, "支付中心", "支付组", "external", "Java"},
	}
	
	// 创建实体
	for _, svc := range services {
		entity := &AppEntity{
			ID:             svc.id,
			Name:           svc.name,
			Type:           svc.appType,
			Namespace:      svc.ns,
			Language:       svc.lang,
			BusinessGroup:  svc.group,
			Team:           svc.team,
			Version:        "v1.0.0",
			Status:         randomStatus(),
			Labels:         map[string]string{"version": "v1.0.0"},
			AppMetrics: AppMetrics{
				QPS:         randomFloat(100, 5000),
				Latency:     randomFloat(5, 200),
				LatencyP50:  randomFloat(5, 150),
				LatencyP95:  randomFloat(20, 300),
				LatencyP99:  randomFloat(50, 500),
				ErrorRate:   randomFloat(0, 0.1),
				CPUUsage:    randomFloat(10, 80),
				MemoryUsage: randomFloat(20, 90),
				CallCount:   int64(randomFloat(10000, 1000000)),
			},
			NetMetrics: NetMetrics{
				ConnActive:  int64(randomFloat(10, 500)),
				BytesSent:   uint64(randomFloat(1000000, 100000000)),
				BytesRecv:   uint64(randomFloat(1000000, 100000000)),
				RetransRate: randomFloat(0, 0.01),
				RTT:         randomFloat(0.1, 10),
			},
			CreatedAt: time.Now(),
			UpdatedAt: time.Now(),
		}
		m.entities[svc.id] = entity
		m.entityByGroup[svc.group] = append(m.entityByGroup[svc.group], entity)
		m.entityByTeam[svc.team] = append(m.entityByTeam[svc.team], entity)
	}
	
	// 创建调用链路
	links := []struct {
		source   string
		target   string
		protocol ProtocolType
		method   string
		path     string
	}{
		{"svc-gateway", "svc-order", ProtocolHTTP, "POST", "/api/orders"},
		{"svc-gateway", "svc-user", ProtocolHTTP, "GET", "/api/users"},
		{"svc-gateway", "svc-product", ProtocolHTTP, "GET", "/api/products"},
		{"svc-gateway", "svc-cart", ProtocolHTTP, "POST", "/api/cart"},
		{"svc-order", "svc-payment", ProtocolHTTP, "POST", "/api/pay"},
		{"svc-order", "svc-inventory", ProtocolHTTP, "POST", "/api/inventory/deduct"},
		{"svc-order", "svc-mysql-order", ProtocolMySQL, "INSERT", "orders"},
		{"svc-order", "svc-message", ProtocolKafka, "PUBLISH", "order-events"},
		{"svc-payment", "svc-alipay", ProtocolHTTPS, "POST", "/gateway/pay"},
		{"svc-payment", "svc-wechat", ProtocolHTTPS, "POST", "/v3/pay"},
		{"svc-user", "svc-redis", ProtocolRedis, "GET", "user:*"},
		{"svc-user", "svc-mysql-user", ProtocolMySQL, "SELECT", "users"},
		{"svc-product", "svc-es", ProtocolHTTP, "POST", "/_search"},
		{"svc-cart", "svc-redis", ProtocolRedis, "HGETALL", "cart:*"},
		{"svc-promo", "svc-redis", ProtocolRedis, "GET", "promo:*"},
		{"svc-inventory", "svc-redis", ProtocolRedis, "DECR", "stock:*"},
		{"svc-es", "svc-product", ProtocolGRPC, "Search", "SearchProducts"},
	}
	
	for _, l := range links {
		linkID := fmt.Sprintf("link-%s-%s", l.source, l.target)
		latency := randomFloat(5, 300)
		errorRate := randomFloat(0, 0.05)
		
		status := "normal"
		if errorRate > 0.03 || latency > 200 {
			status = "error"
		} else if errorRate > 0.01 || latency > 100 {
			status = "warning"
		}
		
		m.links[linkID] = &AppLink{
			ID:       linkID,
			SourceID: l.source,
			TargetID: l.target,
			Protocol: l.protocol,
			Method:   l.method,
			Path:     l.path,
			Status:   status,
			Metrics: LinkMetrics{
				RequestCount: int64(randomFloat(1000, 100000)),
				Latency:      latency,
				LatencyP50:   latency * 0.8,
				LatencyP95:   latency * 1.5,
				LatencyP99:   latency * 2,
				ErrorRate:    errorRate,
				Bandwidth:    randomFloat(10, 1000),
			},
			CreatedAt: time.Now(),
			UpdatedAt: time.Now(),
		}
	}
}

// ==================== 快照管理 ====================

// SaveSnapshot 保存拓扑快照
func (m *AppTopologyManager) SaveSnapshot(name, description, createdBy string, tags []string, topology *AppTopology) (*TopologySnapshot, error) {
	m.snapshotMu.Lock()
	defer m.snapshotMu.Unlock()
	
	snapshot := &TopologySnapshot{
		ID:          fmt.Sprintf("snap-%d", time.Now().UnixNano()),
		Name:        name,
		Description: description,
		Topology:    topology,
		Tags:        tags,
		CreatedAt:   time.Now(),
		CreatedBy:   createdBy,
	}
	
	m.snapshots[snapshot.ID] = snapshot
	return snapshot, nil
}

// GetSnapshot 获取快照
func (m *AppTopologyManager) GetSnapshot(id string) (*TopologySnapshot, error) {
	m.snapshotMu.RLock()
	defer m.snapshotMu.RUnlock()
	
	snapshot, ok := m.snapshots[id]
	if !ok {
		return nil, fmt.Errorf("snapshot not found: %s", id)
	}
	
	return snapshot, nil
}

// ListSnapshots 列出所有快照
func (m *AppTopologyManager) ListSnapshots() []*TopologySnapshot {
	m.snapshotMu.RLock()
	defer m.snapshotMu.RUnlock()
	
	snapshots := make([]*TopologySnapshot, 0, len(m.snapshots))
	for _, snap := range m.snapshots {
		snapshots = append(snapshots, snap)
	}
	
	// 按时间倒序
	sort.Slice(snapshots, func(i, j int) bool {
		return snapshots[i].CreatedAt.After(snapshots[j].CreatedAt)
	})
	
	return snapshots
}

// DeleteSnapshot 删除快照
func (m *AppTopologyManager) DeleteSnapshot(id string) error {
	m.snapshotMu.Lock()
	defer m.snapshotMu.Unlock()
	
	if _, ok := m.snapshots[id]; !ok {
		return fmt.Errorf("snapshot not found: %s", id)
	}
	
	delete(m.snapshots, id)
	return nil
}

// CompareSnapshots 对比两个快照
func (m *AppTopologyManager) CompareSnapshots(baseID, compareID string) (*SnapshotDiff, error) {
	base, err := m.GetSnapshot(baseID)
	if err != nil {
		return nil, fmt.Errorf("base snapshot not found: %w", err)
	}
	
	compare, err := m.GetSnapshot(compareID)
	if err != nil {
		return nil, fmt.Errorf("compare snapshot not found: %w", err)
	}
	
	diff := &SnapshotDiff{
		BaseSnapshot:       baseID,
		CompareSnapshot:    compareID,
		AddedEntities:      make([]*AppEntity, 0),
		RemovedEntities:    make([]*AppEntity, 0),
		ModifiedEntities:   make([]*EntityDiff, 0),
		AddedLinks:         make([]*AppLink, 0),
		RemovedLinks:       make([]*AppLink, 0),
		ModifiedLinks:      make([]*LinkDiff, 0),
		PerformanceChanges: make(map[string]*PerfChange),
	}
	
	// 对比实体
	baseEntities := make(map[string]*AppEntity)
	for _, e := range base.Topology.Entities {
		baseEntities[e.ID] = e
	}
	
	compareEntities := make(map[string]*AppEntity)
	for _, e := range compare.Topology.Entities {
		compareEntities[e.ID] = e
	}
	
	// 新增实体
	for id, entity := range compareEntities {
		if _, ok := baseEntities[id]; !ok {
			diff.AddedEntities = append(diff.AddedEntities, entity)
		}
	}
	
	// 删除实体
	for id, entity := range baseEntities {
		if _, ok := compareEntities[id]; !ok {
			diff.RemovedEntities = append(diff.RemovedEntities, entity)
		}
	}
	
	// 修改的实体
	for id, baseEntity := range baseEntities {
		if compareEntity, ok := compareEntities[id]; ok {
			entityDiff := m.compareEntity(baseEntity, compareEntity)
			if entityDiff != nil {
				diff.ModifiedEntities = append(diff.ModifiedEntities, entityDiff)
			}
		}
	}
	
	// 对比链路
	baseLinks := make(map[string]*AppLink)
	for _, l := range base.Topology.Links {
		baseLinks[l.ID] = l
	}
	
	compareLinks := make(map[string]*AppLink)
	for _, l := range compare.Topology.Links {
		compareLinks[l.ID] = l
	}
	
	// 新增链路
	for id, link := range compareLinks {
		if _, ok := baseLinks[id]; !ok {
			diff.AddedLinks = append(diff.AddedLinks, link)
		}
	}
	
	// 删除链路
	for id, link := range baseLinks {
		if _, ok := compareLinks[id]; !ok {
			diff.RemovedLinks = append(diff.RemovedLinks, link)
		}
	}
	
	// 修改的链路
	for id, baseLink := range baseLinks {
		if compareLink, ok := compareLinks[id]; ok {
			linkDiff := m.compareLink(baseLink, compareLink)
			if linkDiff != nil {
				diff.ModifiedLinks = append(diff.ModifiedLinks, linkDiff)
			}
		}
	}
	
	// 计算摘要
	diff.Summary = DiffSummary{
		TotalChanges:       len(diff.AddedEntities) + len(diff.RemovedEntities) + len(diff.ModifiedEntities) +
		                    len(diff.AddedLinks) + len(diff.RemovedLinks) + len(diff.ModifiedLinks),
		EntityChanges:      len(diff.AddedEntities) + len(diff.RemovedEntities) + len(diff.ModifiedEntities),
		LinkChanges:        len(diff.AddedLinks) + len(diff.RemovedLinks) + len(diff.ModifiedLinks),
		PerformanceChanges: len(diff.PerformanceChanges),
	}
	
	return diff, nil
}

// compareEntity 对比实体
func (m *AppTopologyManager) compareEntity(base, compare *AppEntity) *EntityDiff {
	diff := &EntityDiff{
		EntityID:     base.ID,
		EntityName:   base.Name,
		FieldChanges: make(map[string]FieldChange),
	}
	
	// 对比应用指标
	if base.AppMetrics.Latency != compare.AppMetrics.Latency {
		diff.FieldChanges["latency"] = FieldChange{
			OldValue:   base.AppMetrics.Latency,
			NewValue:   compare.AppMetrics.Latency,
			ChangeType: getChangeType(base.AppMetrics.Latency, compare.AppMetrics.Latency),
		}
	}
	
	if base.AppMetrics.ErrorRate != compare.AppMetrics.ErrorRate {
		diff.FieldChanges["error_rate"] = FieldChange{
			OldValue:   base.AppMetrics.ErrorRate,
			NewValue:   compare.AppMetrics.ErrorRate,
			ChangeType: getChangeType(base.AppMetrics.ErrorRate, compare.AppMetrics.ErrorRate),
		}
	}
	
	if base.AppMetrics.QPS != compare.AppMetrics.QPS {
		diff.FieldChanges["qps"] = FieldChange{
			OldValue:   base.AppMetrics.QPS,
			NewValue:   compare.AppMetrics.QPS,
			ChangeType: getChangeType(base.AppMetrics.QPS, compare.AppMetrics.QPS),
		}
	}
	
	if base.Status != compare.Status {
		diff.FieldChanges["status"] = FieldChange{
			OldValue:   base.Status,
			NewValue:   compare.Status,
			ChangeType: "changed",
		}
	}
	
	if len(diff.FieldChanges) == 0 {
		return nil
	}
	
	return diff
}

// compareLink 对比链路
func (m *AppTopologyManager) compareLink(base, compare *AppLink) *LinkDiff {
	diff := &LinkDiff{
		LinkID:       base.ID,
		SourceName:   base.Source.Name,
		TargetName:   base.Target.Name,
		FieldChanges: make(map[string]FieldChange),
	}
	
	if base.Metrics.Latency != compare.Metrics.Latency {
		diff.FieldChanges["latency"] = FieldChange{
			OldValue:   base.Metrics.Latency,
			NewValue:   compare.Metrics.Latency,
			ChangeType: getChangeType(base.Metrics.Latency, compare.Metrics.Latency),
		}
	}
	
	if base.Metrics.ErrorRate != compare.Metrics.ErrorRate {
		diff.FieldChanges["error_rate"] = FieldChange{
			OldValue:   base.Metrics.ErrorRate,
			NewValue:   compare.Metrics.ErrorRate,
			ChangeType: getChangeType(base.Metrics.ErrorRate, compare.Metrics.ErrorRate),
		}
	}
	
	if base.Status != compare.Status {
		diff.FieldChanges["status"] = FieldChange{
			OldValue:   base.Status,
			NewValue:   compare.Status,
			ChangeType: "changed",
		}
	}
	
	if len(diff.FieldChanges) == 0 {
		return nil
	}
	
	return diff
}

// ==================== 工具函数 ====================

func contains(slice []string, item string) bool {
	for _, s := range slice {
		if s == item {
			return true
		}
	}
	return false
}

func containsString(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(substr) == 0 || 
		(s[:len(substr)] == substr) || (s[len(s)-len(substr):] == substr) ||
		containsSubstring(s, substr))
}

func containsSubstring(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}

func randomFloat(min, max float64) float64 {
	return min + (max-min)*float64(time.Now().UnixNano()%1000)/1000.0
}

func randomStatus() string {
	r := time.Now().UnixNano() % 100
	if r < 70 {
		return "healthy"
	} else if r < 90 {
		return "warning"
	}
	return "error"
}

func cos(angle float64) float64 {
	// 简化的余弦计算
	return 1 - angle*angle/2 + angle*angle*angle*angle/24
}

func sin(angle float64) float64 {
	// 简化的正弦计算
	return angle - angle*angle*angle/6 + angle*angle*angle*angle*angle/120
}

func getChangeType(oldVal, newVal float64) string {
	if newVal > oldVal {
		return "increased"
	} else if newVal < oldVal {
		return "decreased"
	}
	return "unchanged"
}

// ==================== HTTP Handler ====================

// AppTopologyHandler 应用拓扑HTTP处理器
type AppTopologyHandler struct {
	manager *AppTopologyManager
}

// NewAppTopologyHandler 创建处理器
func NewAppTopologyHandler(manager *AppTopologyManager) *AppTopologyHandler {
	return &AppTopologyHandler{manager: manager}
}

// RegisterRoutes 注册路由
func (h *AppTopologyHandler) RegisterRoutes(mux *http.ServeMux) {
	mux.HandleFunc("/api/v1/app-topology", h.handleGetTopology)
	mux.HandleFunc("/api/v1/app-topology/discover", h.handleDiscover)
	mux.HandleFunc("/api/v1/app-topology/entity/", h.handleGetEntity)
	mux.HandleFunc("/api/v1/app-topology/filters", h.handleGetFilters)
	
	// 快照管理
	mux.HandleFunc("/api/v1/app-topology/snapshots", h.handleSnapshots)
	mux.HandleFunc("/api/v1/app-topology/snapshots/", h.handleSnapshotDetail)
	mux.HandleFunc("/api/v1/app-topology/compare", h.handleCompare)
	
	// 页面
	mux.HandleFunc("/app-topology", h.handlePage)
}

// handleGetTopology 获取应用拓扑
func (h *AppTopologyHandler) handleGetTopology(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	
	// 解析过滤器
	filters := &TopologyFilters{
		TimeRange: r.URL.Query().Get("timeRange"),
		Layout:    LayoutType(r.URL.Query().Get("layout")),
	}
	
	if filters.TimeRange == "" {
		filters.TimeRange = "15m"
	}
	if filters.Layout == "" {
		filters.Layout = LayoutForce
	}
	
	// 解析数组参数
	filters.BusinessGroups = r.URL.Query()["businessGroup"]
	filters.Teams = r.URL.Query()["team"]
	filters.Protocols = r.URL.Query()["protocol"]
	filters.Namespaces = r.URL.Query()["namespace"]
	filters.Status = r.URL.Query()["status"]
	filters.SearchQuery = r.URL.Query().Get("q")
	
	topology := h.manager.BuildAppTopology(filters)
	
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(topology)
}

// handleDiscover 触发发现
func (h *AppTopologyHandler) handleDiscover(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	
	go h.manager.DiscoverAppTopology()
	
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{
		"status":  "discovering",
		"message": "Application topology discovery started",
	})
}

// handleGetEntity 获取实体详情
func (h *AppTopologyHandler) handleGetEntity(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	
	entityID := r.URL.Path[len("/api/v1/app-topology/entity/"):]
	
	// 获取实体
	h.manager.mu.RLock()
	entity, ok := h.manager.entities[entityID]
	h.manager.mu.RUnlock()
	
	if !ok {
		http.Error(w, "Entity not found", http.StatusNotFound)
		return
	}
	
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(entity)
}

// handleGetFilters 获取可用过滤器选项
func (h *AppTopologyHandler) handleGetFilters(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	
	filters := &TopologyFilters{}
	topology := h.manager.BuildAppTopology(filters)
	
	response := map[string]interface{}{
		"business_groups": topology.BusinessGroups,
		"teams":           topology.Teams,
		"protocols":       topology.Protocols,
		"namespaces":      topology.Namespaces,
		"layouts":         []string{"horizontal", "vertical", "force", "circular", "hierarchical"},
	}
	
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

// handleSnapshots 处理快照列表和创建
func (h *AppTopologyHandler) handleSnapshots(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		snapshots := h.manager.ListSnapshots()
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(snapshots)
		
	case http.MethodPost:
		var req struct {
			Name        string   `json:"name"`
			Description string   `json:"description"`
			Tags        []string `json:"tags"`
		}
		
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		
		// 获取当前拓扑
		filters := &TopologyFilters{TimeRange: "15m", Layout: LayoutForce}
		topology := h.manager.BuildAppTopology(filters)
		
		snapshot, err := h.manager.SaveSnapshot(req.Name, req.Description, "user", req.Tags, topology)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(snapshot)
		
	default:
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
	}
}

// handleSnapshotDetail 处理单个快照
func (h *AppTopologyHandler) handleSnapshotDetail(w http.ResponseWriter, r *http.Request) {
	id := r.URL.Path[len("/api/v1/app-topology/snapshots/"):]
	
	switch r.Method {
	case http.MethodGet:
		snapshot, err := h.manager.GetSnapshot(id)
		if err != nil {
			http.Error(w, err.Error(), http.StatusNotFound)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(snapshot)
		
	case http.MethodDelete:
		if err := h.manager.DeleteSnapshot(id); err != nil {
			http.Error(w, err.Error(), http.StatusNotFound)
			return
		}
		w.WriteHeader(http.StatusNoContent)
		
	default:
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
	}
}

// handleCompare 对比快照
func (h *AppTopologyHandler) handleCompare(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	
	baseID := r.URL.Query().Get("base")
	compareID := r.URL.Query().Get("compare")
	
	if baseID == "" || compareID == "" {
		http.Error(w, "base and compare required", http.StatusBadRequest)
		return
	}
	
	diff, err := h.manager.CompareSnapshots(baseID, compareID)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(diff)
}

// handlePage 处理页面请求
func (h *AppTopologyHandler) handlePage(w http.ResponseWriter, r *http.Request) {
	http.ServeFile(w, r, "web/app-topology.html")
}
