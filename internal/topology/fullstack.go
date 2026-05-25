// Package topology 全栈拓扑模块
// 支持 Pod → Node → VM → VNF → 物理设备 全链路拓扑展示
package topology

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"sync"
	"time"
)

// FullStackEntityType 全栈实体类型
type FullStackEntityType string

const (
	EntityTypePodFull       FullStackEntityType = "pod"        // Pod容器
	EntityTypeNodeFull      FullStackEntityType = "node"       // K8s节点
	EntityTypeVMFull        FullStackEntityType = "vm"         // 虚拟机
	EntityTypeVNFFull       FullStackEntityType = "vnf"        // 虚拟网元
	EntityTypePhysicalFull  FullStackEntityType = "physical"   // 物理设备
	EntityTypeSwitchFull    FullStackEntityType = "switch"     // 交换机
	EntityTypeRouterFull    FullStackEntityType = "router"     // 路由器
	EntityTypeFirewallFull  FullStackEntityType = "firewall"   // 防火墙
	EntityTypeLoadBalancerFull FullStackEntityType = "lb"      // 负载均衡
)

// FullStackEntity 全栈拓扑实体
type FullStackEntity struct {
	ID          string                 `json:"id"`
	Name        string                 `json:"name"`
	Type        FullStackEntityType    `json:"type"`
	SubType     string                 `json:"sub_type,omitempty"`     // 子类型
	Layer       int                    `json:"layer"`                  // 层级: 1=Pod, 2=Node, 3=VM, 4=VNF, 5=物理
	IPAddresses []string               `json:"ip_addresses"`
	MACAddress  string                 `json:"mac_address,omitempty"`
	Status      string                 `json:"status"`                 // running/ready/active/up/error
	Namespace   string                 `json:"namespace,omitempty"`    // K8s命名空间
	NodeName    string                 `json:"node_name,omitempty"`    // 所属节点
	ClusterID   string                 `json:"cluster_id,omitempty"`   // 集群ID
	Provider    string                 `json:"provider,omitempty"`     // 云厂商
	Region      string                 `json:"region,omitempty"`       // 地域
	Zone        string                 `json:"zone,omitempty"`         // 可用区
	Metadata    map[string]interface{} `json:"metadata,omitempty"`     // 扩展元数据
	Metrics     EntityMetrics          `json:"metrics"`                // 实体指标
	Labels      map[string]string      `json:"labels,omitempty"`       // 标签
	CreatedAt   time.Time              `json:"created_at"`
	UpdatedAt   time.Time              `json:"updated_at"`
}

// EntityMetrics 实体指标
type EntityMetrics struct {
	CPUUsage       float64 `json:"cpu_usage,omitempty"`        // CPU使用率
	MemoryUsage    float64 `json:"memory_usage,omitempty"`     // 内存使用率
	DiskUsage      float64 `json:"disk_usage,omitempty"`       // 磁盘使用率
	NetworkRx      uint64  `json:"network_rx,omitempty"`       // 网络接收字节
	NetworkTx      uint64  `json:"network_tx,omitempty"`       // 网络发送字节
	RequestCount   int64   `json:"request_count,omitempty"`    // 请求数
	ErrorCount     int64   `json:"error_count,omitempty"`      // 错误数
	Latency        float64 `json:"latency,omitempty"`          // 平均延迟(ms)
	Temperature    float64 `json:"temperature,omitempty"`      // 温度(物理设备)
	PowerUsage     float64 `json:"power_usage,omitempty"`      // 功耗
}

// FullStackLink 全栈拓扑链路
type FullStackLink struct {
	ID          string            `json:"id"`
	SourceID    string            `json:"source_id"`
	TargetID    string            `json:"target_id"`
	Source      *FullStackEntity  `json:"source,omitempty"`
	Target      *FullStackEntity  `json:"target,omitempty"`
	Type        string            `json:"type"`              // 链路类型
	Status      string            `json:"status"`            // normal/warning/error
	Metrics     LinkMetrics       `json:"metrics"`           // 链路指标
	Properties  map[string]string `json:"properties,omitempty"`
	CreatedAt   time.Time         `json:"created_at"`
	UpdatedAt   time.Time         `json:"updated_at"`
}

// LinkMetrics 链路指标
type LinkMetrics struct {
	RequestCount   int64   `json:"request_count"`      // 请求量
	RequestRate    float64 `json:"request_rate"`       // 请求率(req/s)
	Latency        float64 `json:"latency"`            // 平均时延(ms)
	LatencyP50     float64 `json:"latency_p50"`        // P50时延
	LatencyP95     float64 `json:"latency_p95"`        // P95时延
	LatencyP99     float64 `json:"latency_p99"`        // P99时延
	ErrorCount     int64   `json:"error_count"`        // 错误数
	ErrorRate      float64 `json:"error_rate"`         // 错误率
	Bandwidth      float64 `json:"bandwidth"`          // 带宽(Mbps)
	BandwidthUsage float64 `json:"bandwidth_usage"`    // 带宽使用率
	PacketLoss     float64 `json:"packet_loss"`        // 丢包率
	Jitter         float64 `json:"jitter"`             // 抖动
}

// FullStackTopology 全栈拓扑
type FullStackTopology struct {
	Entities      []*FullStackEntity `json:"entities"`
	Links         []*FullStackLink   `json:"links"`
	Layers        map[int][]*FullStackEntity `json:"layers"`  // 按层级分组
	Stats         TopologyStats      `json:"stats"`
	Timestamp     time.Time          `json:"timestamp"`
	TimeRange     string             `json:"time_range"`
}

// TopologyStats 拓扑统计
type TopologyStats struct {
	TotalEntities   int            `json:"total_entities"`
	TotalLinks      int            `json:"total_links"`
	EntityByType    map[string]int `json:"entity_by_type"`
	LinkByStatus    map[string]int `json:"link_by_status"`
	ErrorLinks      int            `json:"error_links"`
	WarningLinks    int            `json:"warning_links"`
	NormalLinks     int            `json:"normal_links"`
	AvgLatency      float64        `json:"avg_latency"`
	TotalRequests   int64          `json:"total_requests"`
	TotalErrors     int64          `json:"total_errors"`
}

// FullStackManager 全栈拓扑管理器
type FullStackManager struct {
	entities      map[string]*FullStackEntity
	links         map[string]*FullStackLink
	entityByLayer map[int][]*FullStackEntity
	mu            sync.RWMutex
	discovery     *DiscoveryEngine
	tracer        *PathTracer
	storage       *TopologyStorage
	metrics       *MetricsCollector
}

// MetricsCollector 指标收集器
type MetricsCollector struct {
	linkMetrics   map[string]*LinkMetrics
	entityMetrics map[string]*EntityMetrics
	mu            sync.RWMutex
}

// NewFullStackManager 创建全栈拓扑管理器
func NewFullStackManager(discovery *DiscoveryEngine, tracer *PathTracer, storage *TopologyStorage) *FullStackManager {
	return &FullStackManager{
		entities:      make(map[string]*FullStackEntity),
		links:         make(map[string]*FullStackLink),
		entityByLayer: make(map[int][]*FullStackEntity),
		discovery:     discovery,
		tracer:        tracer,
		storage:       storage,
		metrics: &MetricsCollector{
			linkMetrics:   make(map[string]*LinkMetrics),
			entityMetrics: make(map[string]*EntityMetrics),
		},
	}
}

// BuildFullStackTopology 构建全栈拓扑
func (m *FullStackManager) BuildFullStackTopology(timeRange string) *FullStackTopology {
	m.mu.RLock()
	defer m.mu.RUnlock()

	topology := &FullStackTopology{
		Entities:  make([]*FullStackEntity, 0, len(m.entities)),
		Links:     make([]*FullStackLink, 0, len(m.links)),
		Layers:    make(map[int][]*FullStackEntity),
		Timestamp: time.Now(),
		TimeRange: timeRange,
		Stats:     TopologyStats{},
	}

	// 复制实体
	for _, entity := range m.entities {
		topology.Entities = append(topology.Entities, entity)
		
		// 按层级分组
		if topology.Layers[entity.Layer] == nil {
			topology.Layers[entity.Layer] = make([]*FullStackEntity, 0)
		}
		topology.Layers[entity.Layer] = append(topology.Layers[entity.Layer], entity)
	}

	// 复制链路
	var totalLatency float64
	var latencyCount int
	
	for _, link := range m.links {
		topology.Links = append(topology.Links, link)
		
		// 统计
		if link.Status == "error" {
			topology.Stats.ErrorLinks++
		} else if link.Status == "warning" {
			topology.Stats.WarningLinks++
		} else {
			topology.Stats.NormalLinks++
		}
		
		if link.Metrics.Latency > 0 {
			totalLatency += link.Metrics.Latency
			latencyCount++
		}
		
		topology.Stats.TotalRequests += link.Metrics.RequestCount
		topology.Stats.TotalErrors += link.Metrics.ErrorCount
	}

	// 计算平均延迟
	if latencyCount > 0 {
		topology.Stats.AvgLatency = totalLatency / float64(latencyCount)
	}

	// 统计
	topology.Stats.TotalEntities = len(topology.Entities)
	topology.Stats.TotalLinks = len(topology.Links)
	topology.Stats.EntityByType = m.countEntitiesByType()
	topology.Stats.LinkByStatus = m.countLinksByStatus()

	return topology
}

// DiscoverFullStack 执行全栈发现
func (m *FullStackManager) DiscoverFullStack() error {
	m.mu.Lock()
	defer m.mu.Unlock()

	// 清空现有数据
	m.entities = make(map[string]*FullStackEntity)
	m.links = make(map[string]*FullStackLink)
	m.entityByLayer = make(map[int][]*FullStackEntity)

	// 1. 发现Pod层 (Layer 1)
	if err := m.discoverPods(); err != nil {
		return fmt.Errorf("discover pods failed: %w", err)
	}

	// 2. 发现Node层 (Layer 2)
	if err := m.discoverNodes(); err != nil {
		return fmt.Errorf("discover nodes failed: %w", err)
	}

	// 3. 发现VM层 (Layer 3)
	if err := m.discoverVMs(); err != nil {
		return fmt.Errorf("discover vms failed: %w", err)
	}

	// 4. 发现VNF层 (Layer 4)
	if err := m.discoverVNFs(); err != nil {
		return fmt.Errorf("discover vnfs failed: %w", err)
	}

	// 5. 发现物理设备层 (Layer 5)
	if err := m.discoverPhysicalDevices(); err != nil {
		return fmt.Errorf("discover physical devices failed: %w", err)
	}

	// 6. 建立链路关系
	m.buildLinks()

	// 7. 收集指标
	m.collectMetrics()

	return nil
}

// discoverPods 发现Pod层
func (m *FullStackManager) discoverPods() error {
	// 从DiscoveryEngine获取Pod信息
	if m.discovery == nil {
		return nil
	}

	pods := m.discovery.GetEntitiesByType(EntityTypePod)
	for _, pod := range pods {
		entity := &FullStackEntity{
			ID:          pod.ID,
			Name:        pod.Name,
			Type:        EntityTypePodFull,
			Layer:       1,
			IPAddresses: pod.IPAddresses,
			Status:      m.normalizeStatus(pod.Status),
			Namespace:   pod.Namespace,
			NodeName:    pod.NodeName,
			ClusterID:   pod.ClusterID,
			Labels:      pod.Labels,
			CreatedAt:   pod.CreatedAt,
			UpdatedAt:   pod.UpdatedAt,
		}
		m.entities[entity.ID] = entity
		m.addEntityByLayer(entity)
	}

	return nil
}

// discoverNodes 发现Node层
func (m *FullStackManager) discoverNodes() error {
	if m.discovery == nil {
		return nil
	}

	nodes := m.discovery.GetEntitiesByType(EntityTypeNode)
	for _, node := range nodes {
		entity := &FullStackEntity{
			ID:          node.ID,
			Name:        node.Name,
			Type:        EntityTypeNodeFull,
			Layer:       2,
			IPAddresses: node.IPAddresses,
			MACAddress:  node.MACAddress,
			Status:      m.normalizeStatus(node.Status),
			ClusterID:   node.ClusterID,
			Labels:      node.Labels,
			CreatedAt:   node.CreatedAt,
			UpdatedAt:   node.UpdatedAt,
		}
		m.entities[entity.ID] = entity
		m.addEntityByLayer(entity)
	}

	return nil
}

// discoverVMs 发现VM层
func (m *FullStackManager) discoverVMs() error {
	if m.discovery == nil {
		return nil
	}

	vms := m.discovery.GetEntitiesByType(EntityTypeVM)
	for _, vm := range vms {
		entity := &FullStackEntity{
			ID:          vm.ID,
			Name:        vm.Name,
			Type:        EntityTypeVMFull,
			Layer:       3,
			IPAddresses: vm.IPAddresses,
			MACAddress:  vm.MACAddress,
			Status:      m.normalizeStatus(vm.Status),
			ClusterID:   vm.ClusterID,
			Provider:    vm.Labels["cloud_provider"],
			Region:      vm.Labels["cloud_region"],
			Zone:        vm.Labels["cloud_zone"],
			Labels:      vm.Labels,
			CreatedAt:   vm.CreatedAt,
			UpdatedAt:   vm.UpdatedAt,
		}
		m.entities[entity.ID] = entity
		m.addEntityByLayer(entity)
	}

	return nil
}

// discoverVNFs 发现虚拟网元层
func (m *FullStackManager) discoverVNFs() error {
	// 从配置或CMDB获取虚拟网元信息
	// 这里模拟一些常见的虚拟网元
	vnfs := []*FullStackEntity{
		{
			ID:          "vnf-vrouter-1",
			Name:        "vRouter-Core",
			Type:        EntityTypeVNFFull,
			SubType:     "router",
			Layer:       4,
			IPAddresses: []string{"10.100.1.1"},
			Status:      "active",
			Labels: map[string]string{
				"vendor":    "huawei",
				"version":   "v8.0",
				"ha_mode":   "active_standby",
			},
			CreatedAt: time.Now(),
			UpdatedAt: time.Now(),
		},
		{
			ID:          "vnf-vswitch-1",
			Name:        "vSwitch-Edge",
			Type:        EntityTypeVNFFull,
			SubType:     "switch",
			Layer:       4,
			IPAddresses: []string{"10.100.1.2"},
			Status:      "active",
			Labels: map[string]string{
				"vendor":    "huawei",
				"version":   "v8.0",
			},
			CreatedAt: time.Now(),
			UpdatedAt: time.Now(),
		},
		{
			ID:          "vnf-firewall-1",
			Name:        "vFirewall-Perimeter",
			Type:        EntityTypeVNFFull,
			SubType:     "firewall",
			Layer:       4,
			IPAddresses: []string{"10.100.1.3"},
			Status:      "active",
			Labels: map[string]string{
				"vendor":    "huawei",
				"version":   "v8.0",
			},
			CreatedAt: time.Now(),
			UpdatedAt: time.Now(),
		},
	}

	for _, vnf := range vnfs {
		m.entities[vnf.ID] = vnf
		m.addEntityByLayer(vnf)
	}

	return nil
}

// discoverPhysicalDevices 发现物理设备层
func (m *FullStackManager) discoverPhysicalDevices() error {
	// 从CMDB或SNMP发现物理设备
	// 这里模拟一些常见的物理设备
	devices := []*FullStackEntity{
		{
			ID:          "phy-core-sw-1",
			Name:        "Core-Switch-01",
			Type:        EntityTypePhysicalFull,
			SubType:     "switch",
			Layer:       5,
			IPAddresses: []string{"10.200.1.1"},
			Status:      "up",
			Labels: map[string]string{
				"vendor":    "huawei",
				"model":     "CE12800",
				"location":  "数据中心A-机房1",
				"rack":      "R01-U10",
			},
			CreatedAt: time.Now(),
			UpdatedAt: time.Now(),
		},
		{
			ID:          "phy-edge-rtr-1",
			Name:        "Edge-Router-01",
			Type:        EntityTypePhysicalFull,
			SubType:     "router",
			Layer:       5,
			IPAddresses: []string{"10.200.1.2"},
			Status:      "up",
			Labels: map[string]string{
				"vendor":    "huawei",
				"model":     "NE40E",
				"location":  "数据中心A-机房1",
				"rack":      "R01-U15",
			},
			CreatedAt: time.Now(),
			UpdatedAt: time.Now(),
		},
		{
			ID:          "phy-firewall-1",
			Name:        "Firewall-Core-01",
			Type:        EntityTypePhysicalFull,
			SubType:     "firewall",
			Layer:       5,
			IPAddresses: []string{"10.200.1.3"},
			Status:      "up",
			Labels: map[string]string{
				"vendor":    "huawei",
				"model":     "USG9500",
				"location":  "数据中心A-机房1",
				"rack":      "R01-U20",
			},
			CreatedAt: time.Now(),
			UpdatedAt: time.Now(),
		},
	}

	for _, device := range devices {
		m.entities[device.ID] = device
		m.addEntityByLayer(device)
	}

	return nil
}

// buildLinks 建立链路关系
func (m *FullStackManager) buildLinks() {
	// 1. Pod -> Node 链路
	for _, pod := range m.entityByLayer[1] {
		if pod.NodeName != "" {
			// 查找对应的Node
			for _, node := range m.entityByLayer[2] {
				if node.Name == pod.NodeName {
					link := m.createLink(pod.ID, node.ID, "pod-node")
					m.links[link.ID] = link
					break
				}
			}
		}
	}

	// 2. Node -> VM 链路 (通过IP网段匹配)
	for _, node := range m.entityByLayer[2] {
		for _, vm := range m.entityByLayer[3] {
			if m.isSameSubnet(node.IPAddresses, vm.IPAddresses) {
				link := m.createLink(node.ID, vm.ID, "node-vm")
				m.links[link.ID] = link
			}
		}
	}

	// 3. VM -> VNF 链路
	for _, vm := range m.entityByLayer[3] {
		for _, vnf := range m.entityByLayer[4] {
			if m.isReachable(vm.IPAddresses, vnf.IPAddresses) {
				link := m.createLink(vm.ID, vnf.ID, "vm-vnf")
				m.links[link.ID] = link
			}
		}
	}

	// 4. VNF -> 物理设备 链路
	for _, vnf := range m.entityByLayer[4] {
		for _, phy := range m.entityByLayer[5] {
			link := m.createLink(vnf.ID, phy.ID, "vnf-physical")
			m.links[link.ID] = link
		}
	}

	// 5. VNF间互联
	vnfs := m.entityByLayer[4]
	for i := 0; i < len(vnfs); i++ {
		for j := i + 1; j < len(vnfs); j++ {
			link := m.createLink(vnfs[i].ID, vnfs[j].ID, "vnf-vnf")
			m.links[link.ID] = link
		}
	}
}

// createLink 创建链路
func (m *FullStackManager) createLink(sourceID, targetID, linkType string) *FullStackLink {
	link := &FullStackLink{
		ID:         fmt.Sprintf("link-%s-%s", sourceID, targetID),
		SourceID:   sourceID,
		TargetID:   targetID,
		Type:       linkType,
		Status:     "normal",
		Metrics:    LinkMetrics{},
		CreatedAt:  time.Now(),
		UpdatedAt:  time.Now(),
	}

	// 设置源和目标实体引用
	if source, ok := m.entities[sourceID]; ok {
		link.Source = source
	}
	if target, ok := m.entities[targetID]; ok {
		link.Target = target
	}

	return link
}

// collectMetrics 收集指标
func (m *FullStackManager) collectMetrics() {
	// 从PathTracer获取链路指标
	if m.tracer != nil {
		graph := m.tracer.BuildTopologyGraph()
		
		for edgeID, edge := range graph.Edges {
			if link, ok := m.links[edgeID]; ok {
				link.Metrics.RequestCount = edge.RequestCount
				link.Metrics.ErrorCount = edge.ErrorCount
				if edge.RequestCount > 0 {
					link.Metrics.ErrorRate = float64(edge.ErrorCount) / float64(edge.RequestCount)
				}
				link.Metrics.Latency = float64(edge.AvgLatency.Milliseconds())
				
				// 确定链路状态
				link.Status = m.determineLinkStatus(link.Metrics)
				link.UpdatedAt = time.Now()
			}
		}
	}

	// 模拟一些指标数据
	for _, link := range m.links {
		if link.Metrics.RequestCount == 0 {
			// 生成模拟数据
			link.Metrics.RequestCount = int64(1000 + len(link.ID)%9000)
			link.Metrics.Latency = float64(5 + len(link.ID)%100)
			link.Metrics.ErrorRate = float64(len(link.ID)%50) / 1000.0
			link.Metrics.Bandwidth = float64(100 + len(link.ID)%900)
			link.Status = m.determineLinkStatus(link.Metrics)
		}
	}
}

// determineLinkStatus 确定链路状态
func (m *FullStackManager) determineLinkStatus(metrics LinkMetrics) string {
	// 错误率 > 5% 或 延迟 > 500ms = 异常
	if metrics.ErrorRate > 0.05 || metrics.Latency > 500 {
		return "error"
	}
	// 错误率 > 1% 或 延迟 > 100ms = 警告
	if metrics.ErrorRate > 0.01 || metrics.Latency > 100 {
		return "warning"
	}
	return "normal"
}

// normalizeStatus 标准化状态
func (m *FullStackManager) normalizeStatus(status string) string {
	switch status {
	case "running", "ready", "active", "up", "Running", "Ready", "Active", "Up":
		return "running"
	case "pending", "Pending":
		return "pending"
	case "failed", "error", "down", "Failed", "Error", "Down":
		return "error"
	default:
		return status
	}
}

// addEntityByLayer 按层级添加实体
func (m *FullStackManager) addEntityByLayer(entity *FullStackEntity) {
	if m.entityByLayer[entity.Layer] == nil {
		m.entityByLayer[entity.Layer] = make([]*FullStackEntity, 0)
	}
	m.entityByLayer[entity.Layer] = append(m.entityByLayer[entity.Layer], entity)
}

// isSameSubnet 检查是否同一子网
func (m *FullStackManager) isSameSubnet(ips1, ips2 []string) bool {
	// 简化实现：检查IP前三个段是否相同
	for _, ip1 := range ips1 {
		for _, ip2 := range ips2 {
			if m.getSubnet(ip1) == m.getSubnet(ip2) {
				return true
			}
		}
	}
	return false
}

// getSubnet 获取子网
func (m *FullStackManager) getSubnet(ip string) string {
	// 简化实现：返回前三个段
	parts := []rune(ip)
	dotCount := 0
	for i, c := range parts {
		if c == '.' {
			dotCount++
			if dotCount == 3 {
				return string(parts[:i])
			}
		}
	}
	return ip
}

// isReachable 检查是否可达
func (m *FullStackManager) isReachable(ips1, ips2 []string) bool {
	// 简化实现：假设同一/16网段可达
	for _, ip1 := range ips1 {
		for _, ip2 := range ips2 {
			if m.getNetwork(ip1) == m.getNetwork(ip2) {
				return true
			}
		}
	}
	return false
}

// getNetwork 获取网络地址
func (m *FullStackManager) getNetwork(ip string) string {
	// 简化实现：返回前两个段
	parts := []rune(ip)
	dotCount := 0
	for i, c := range parts {
		if c == '.' {
			dotCount++
			if dotCount == 2 {
				return string(parts[:i])
			}
		}
	}
	return ip
}

// countEntitiesByType 按类型统计实体
func (m *FullStackManager) countEntitiesByType() map[string]int {
	counts := make(map[string]int)
	for _, entity := range m.entities {
		counts[string(entity.Type)]++
	}
	return counts
}

// countLinksByStatus 按状态统计链路
func (m *FullStackManager) countLinksByStatus() map[string]int {
	counts := make(map[string]int)
	for _, link := range m.links {
		counts[link.Status]++
	}
	return counts
}

// GetEntityDetail 获取实体详情
func (m *FullStackManager) GetEntityDetail(entityID string) (*FullStackEntity, []*FullStackLink, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	entity, ok := m.entities[entityID]
	if !ok {
		return nil, nil, fmt.Errorf("entity not found: %s", entityID)
	}

	// 查找相关链路
	var relatedLinks []*FullStackLink
	for _, link := range m.links {
		if link.SourceID == entityID || link.TargetID == entityID {
			relatedLinks = append(relatedLinks, link)
		}
	}

	return entity, relatedLinks, nil
}

// GetLinkDetail 获取链路详情
func (m *FullStackManager) GetLinkDetail(linkID string) (*FullStackLink, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	link, ok := m.links[linkID]
	if !ok {
		return nil, fmt.Errorf("link not found: %s", linkID)
	}

	return link, nil
}

// FindPath 查找路径
func (m *FullStackManager) FindPath(sourceID, targetID string) ([]*FullStackLink, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	// 使用BFS查找最短路径
	visited := make(map[string]bool)
	queue := [][]string{{sourceID}}

	for len(queue) > 0 {
		path := queue[0]
		queue = queue[1:]

		current := path[len(path)-1]
		if current == targetID {
			// 构建链路列表
			var links []*FullStackLink
			for i := 0; i < len(path)-1; i++ {
				linkID := fmt.Sprintf("link-%s-%s", path[i], path[i+1])
				if link, ok := m.links[linkID]; ok {
					links = append(links, link)
				}
			}
			return links, nil
		}

		if visited[current] {
			continue
		}
		visited[current] = true

		// 查找相邻节点
		for _, link := range m.links {
			if link.SourceID == current && !visited[link.TargetID] {
				newPath := append([]string{}, path...)
				newPath = append(newPath, link.TargetID)
				queue = append(queue, newPath)
			}
		}
	}

	return nil, fmt.Errorf("path not found from %s to %s", sourceID, targetID)
}

// ==================== HTTP API Handler ====================

// FullStackHandler 全栈拓扑HTTP处理器
type FullStackHandler struct {
	manager *FullStackManager
}

// NewFullStackHandler 创建处理器
func NewFullStackHandler(manager *FullStackManager) *FullStackHandler {
	return &FullStackHandler{manager: manager}
}

// RegisterRoutes 注册路由
func (h *FullStackHandler) RegisterRoutes(mux *http.ServeMux) {
	mux.HandleFunc("/api/v1/topology/fullstack", h.handleFullStack)
	mux.HandleFunc("/api/v1/topology/fullstack/entity/", h.handleEntityDetail)
	mux.HandleFunc("/api/v1/topology/fullstack/link/", h.handleLinkDetail)
	mux.HandleFunc("/api/v1/topology/fullstack/path", h.handleFindPath)
	mux.HandleFunc("/api/v1/topology/fullstack/refresh", h.handleRefresh)
}

// handleFullStack 处理全栈拓扑请求
func (h *FullStackHandler) handleFullStack(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	timeRange := r.URL.Query().Get("timeRange")
	if timeRange == "" {
		timeRange = "15m"
	}

	topology := h.manager.BuildFullStackTopology(timeRange)

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(topology)
}

// handleEntityDetail 处理实体详情请求
func (h *FullStackHandler) handleEntityDetail(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	entityID := r.URL.Path[len("/api/v1/topology/fullstack/entity/"):]

	entity, links, err := h.manager.GetEntityDetail(entityID)
	if err != nil {
		http.Error(w, err.Error(), http.StatusNotFound)
		return
	}

	response := map[string]interface{}{
		"entity": entity,
		"links":  links,
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

// handleLinkDetail 处理链路详情请求
func (h *FullStackHandler) handleLinkDetail(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	linkID := r.URL.Path[len("/api/v1/topology/fullstack/link/"):]

	link, err := h.manager.GetLinkDetail(linkID)
	if err != nil {
		http.Error(w, err.Error(), http.StatusNotFound)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(link)
}

// handleFindPath 处理路径查找请求
func (h *FullStackHandler) handleFindPath(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	sourceID := r.URL.Query().Get("source")
	targetID := r.URL.Query().Get("target")

	if sourceID == "" || targetID == "" {
		http.Error(w, "source and target required", http.StatusBadRequest)
		return
	}

	links, err := h.manager.FindPath(sourceID, targetID)
	if err != nil {
		http.Error(w, err.Error(), http.StatusNotFound)
		return
	}

	response := map[string]interface{}{
		"source": sourceID,
		"target": targetID,
		"path":   links,
		"hops":   len(links),
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

// handleRefresh 处理刷新请求
func (h *FullStackHandler) handleRefresh(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	go h.manager.DiscoverFullStack()

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{
		"status":  "refreshing",
		"message": "Topology discovery started",
	})
}

// ==================== 启动全栈拓扑发现 ====================

// StartFullStackDiscovery 启动全栈拓扑发现服务
func StartFullStackDiscovery(ctx context.Context, manager *FullStackManager, interval time.Duration) {
	// 首次发现
	if err := manager.DiscoverFullStack(); err != nil {
		fmt.Printf("Initial fullstack discovery failed: %v\n", err)
	}

	// 定期发现
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			if err := manager.DiscoverFullStack(); err != nil {
				fmt.Printf("Fullstack discovery failed: %v\n", err)
			}
		}
	}
}
