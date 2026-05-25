// Package rca 提供多节点告警根因分析（Root Cause Analysis）能力
// 支持跨节点告警关联、因果链推导、自动修复建议
// Copyright (c) 2026 Cloud Flow Team
// Licensed under the MIT License.

package rca

import (
	"context"
	"fmt"
	"math"
	"sort"
	"strings"
	"sync"
	"sync/atomic"
	"time"
)

// ============================================================
// 配置定义
// ============================================================

// RCAConfig 根因分析配置
type RCAConfig struct {
	Enabled              bool          `yaml:"enabled" json:"enabled"`
	CorrelationWindow    time.Duration `yaml:"correlation_window" json:"correlation_window"`   // 告警关联时间窗口
	MaxCorrelationDepth  int           `yaml:"max_correlation_depth" json:"max_correlation_depth"` // 最大关联深度
	MinCorrelationScore  float64       `yaml:"min_correlation_score" json:"min_correlation_score"` // 最小关联分数
	EnableTopologyAware  bool          `yaml:"enable_topology_aware" json:"enable_topology_aware"` // 启用拓扑感知
	EnableMetricCorrelate bool         `yaml:"enable_metric_correlate" json:"enable_metric_correlate"` // 启用指标关联
	MaxIncidentAge       time.Duration `yaml:"max_incident_age" json:"max_incident_age"`       // 最大事件保留时间
	AnalysisInterval     time.Duration `yaml:"analysis_interval" json:"analysis_interval"`     // 分析间隔
	MaxRootCauses        int           `yaml:"max_root_causes" json:"max_root_causes"`         // 最大根因数量
}

func DefaultRCAConfig() *RCAConfig {
	return &RCAConfig{
		Enabled:              true,
		CorrelationWindow:    5 * time.Minute,
		MaxCorrelationDepth:  5,
		MinCorrelationScore:  0.5,
		EnableTopologyAware:  true,
		EnableMetricCorrelate: true,
		MaxIncidentAge:       1 * time.Hour,
		AnalysisInterval:     30 * time.Second,
		MaxRootCauses:        3,
	}
}

// ============================================================
// 数据模型
// ============================================================

// AlertSeverity 告警严重程度
type AlertSeverity int

const (
	SeverityInfo     AlertSeverity = iota
	SeverityWarning
	SeverityCritical
	SeverityFatal
)

func (s AlertSeverity) String() string {
	names := []string{"info", "warning", "critical", "fatal"}
	if int(s) < len(names) {
		return names[s]
	}
	return "unknown"
}

func (s AlertSeverity) Score() float64 {
	return float64(s) / float64(SeverityFatal)
}

// AlertCategory 告警分类
type AlertCategory int

const (
	CategoryUnknown AlertCategory = iota
	CategoryNetwork
	CategoryCPU
	CategoryMemory
	CategoryDisk
	CategoryProcess
	CategoryApplication
	CategoryDatabase
	CategoryMiddleware
	CategoryLoadBalancer
	CategoryDNS
	CategoryCertificate
)

func (c AlertCategory) String() string {
	names := map[AlertCategory]string{
		CategoryNetwork:     "network",
		CategoryCPU:         "cpu",
		CategoryMemory:      "memory",
		CategoryDisk:        "disk",
		CategoryProcess:     "process",
		CategoryApplication: "application",
		CategoryDatabase:    "database",
		CategoryMiddleware:  "middleware",
		CategoryLoadBalancer:"load_balancer",
		CategoryDNS:         "dns",
		CategoryCertificate: "certificate",
	}
	if name, ok := names[c]; ok {
		return name
	}
	return "unknown"
}

// NodeAlert 节点告警
type NodeAlert struct {
	ID           string            `json:"id"`
	Timestamp    time.Time         `json:"timestamp"`
	NodeID       string            `json:"node_id"`
	NodeName     string            `json:"node_name"`
	NodeRole     string            `json:"node_role"`      // master, worker, gateway, db等
	MetricName   string            `json:"metric_name"`
	Category     AlertCategory     `json:"category"`
	Severity     AlertSeverity     `json:"severity"`
	Message      string            `json:"message"`
	Value        float64           `json:"value"`
	Threshold    float64           `json:"threshold"`
	Labels       map[string]string `json:"labels,omitempty"`
	Resolved     bool              `json:"resolved"`
	ResolveTime  time.Time         `json:"resolve_time,omitempty"`
}

// TopologyLink 拓扑链接
type TopologyLink struct {
	Source      string `json:"source"`
	Target      string `json:"target"`
	LinkType    string `json:"link_type"`    // depends_on, connects_to, serves, monitors
	Weight      float64 `json:"weight"`      // 关联权重
	Description string `json:"description"`
}

// TopologyGraph 拓扑图
type TopologyGraph struct {
	Nodes    []TopologyNode    `json:"nodes"`
	Links    []TopologyLink    `json:"links"`
	nodeMap  map[string]*TopologyNode
	linkMap  map[string][]*TopologyLink
	mu       sync.RWMutex
}

// TopologyNode 拓扑节点
type TopologyNode struct {
	ID       string   `json:"id"`
	Name     string   `json:"name"`
	Role     string   `json:"role"`
	IP       string   `json:"ip"`
	Tags     []string `json:"tags"`
}

// NewTopologyGraph 创建拓扑图
func NewTopologyGraph() *TopologyGraph {
	return &TopologyGraph{
		Nodes:   make([]TopologyNode, 0),
		Links:   make([]TopologyLink, 0),
		nodeMap: make(map[string]*TopologyNode),
		linkMap: make(map[string][]*TopologyLink),
	}
}

// AddNode 添加节点
func (g *TopologyGraph) AddNode(node TopologyNode) {
	g.mu.Lock()
	defer g.mu.Unlock()

	g.Nodes = append(g.Nodes, node)
	n := &g.Nodes[len(g.Nodes)-1]
	g.nodeMap[node.ID] = n
}

// AddLink 添加链接
func (g *TopologyGraph) AddLink(link TopologyLink) {
	g.mu.Lock()
	defer g.mu.Unlock()

	g.Links = append(g.Links, link)
	l := &g.Links[len(g.Links)-1]
	g.linkMap[link.Source] = append(g.linkMap[link.Source], l)
}

// GetNeighbors 获取邻居节点
func (g *TopologyGraph) GetNeighbors(nodeID string) []string {
	g.mu.RLock()
	defer g.mu.RUnlock()

	var neighbors []string
	if links, ok := g.linkMap[nodeID]; ok {
		for _, link := range links {
			neighbors = append(neighbors, link.Target)
		}
	}
	return neighbors
}

// GetUpstream 获取上游节点
func (g *TopologyGraph) GetUpstream(nodeID string) []string {
	g.mu.RLock()
	defer g.mu.RUnlock()

	var upstream []string
	for _, link := range g.Links {
		if link.Target == nodeID {
			upstream = append(upstream, link.Source)
		}
	}
	return upstream
}

// GetDownstream 获取下游节点
func (g *TopologyGraph) GetDownstream(nodeID string) []string {
	return g.GetNeighbors(nodeID)
}

// GetLinkWeight 获取链接权重
func (g *TopologyGraph) GetLinkWeight(source, target string) float64 {
	g.mu.RLock()
	defer g.mu.RUnlock()

	if links, ok := g.linkMap[source]; ok {
		for _, link := range links {
			if link.Target == target {
				return link.Weight
			}
		}
	}
	return 0.5 // 默认权重
}

// ============================================================
// 告警关联
// ============================================================

// CorrelationResult 关联结果
type CorrelationResult struct {
	AlertGroup    []*NodeAlert  `json:"alert_group"`
	CorrelationID string        `json:"correlation_id"`
	Score         float64       `json:"score"`          // 关联分数 0-1
	Reason        string        `json:"reason"`         // 关联原因
	Category      AlertCategory `json:"category"`       // 主要分类
	StartTime     time.Time     `json:"start_time"`
	EndTime       time.Time     `json:"end_time"`
	NodeCount     int           `json:"node_count"`
}

// AlertCorrelator 告警关联器
type AlertCorrelator struct {
	config  *RCAConfig
	topology *TopologyGraph
}

// NewAlertCorrelator 创建告警关联器
func NewAlertCorrelator(cfg *RCAConfig, topo *TopologyGraph) *AlertCorrelator {
	return &AlertCorrelator{
		config:   cfg,
		topology: topo,
	}
}

// Correlate 关联告警
func (ac *AlertCorrelator) Correlate(alerts []*NodeAlert) []*CorrelationResult {
	if len(alerts) == 0 {
		return nil
	}

	// 按时间窗口分组
	timeGroups := ac.groupByTime(alerts)

	// 在每个时间组内进行关联
	var results []*CorrelationResult
	for _, group := range timeGroups {
		correlations := ac.correlateGroup(group)
		results = append(results, correlations...)
	}

	// 按关联分数排序
	sort.Slice(results, func(i, j int) bool {
		return results[i].Score > results[j].Score
	})

	return results
}

// groupByTime 按时间窗口分组
func (ac *AlertCorrelator) groupByTime(alerts []*NodeAlert) [][]*NodeAlert {
	if len(alerts) == 0 {
		return nil
	}

	// 按时间排序
	sorted := make([]*NodeAlert, len(alerts))
	copy(sorted, alerts)
	sort.Slice(sorted, func(i, j int) bool {
		return sorted[i].Timestamp.Before(sorted[j].Timestamp)
	})

	var groups [][]*NodeAlert
	currentGroup := []*NodeAlert{sorted[0]}
	windowStart := sorted[0].Timestamp

	for i := 1; i < len(sorted); i++ {
		if sorted[i].Timestamp.Sub(windowStart) <= ac.config.CorrelationWindow {
			currentGroup = append(currentGroup, sorted[i])
		} else {
			if len(currentGroup) >= 2 {
				groups = append(groups, currentGroup)
			}
			currentGroup = []*NodeAlert{sorted[i]}
			windowStart = sorted[i].Timestamp
		}
	}

	if len(currentGroup) >= 2 {
		groups = append(groups, currentGroup)
	}

	return groups
}

// correlateGroup 关联一组告警
func (ac *AlertCorrelator) correlateGroup(alerts []*NodeAlert) []*CorrelationResult {
	var results []*CorrelationResult

	// 1. 按分类分组
	categoryGroups := ac.groupByCategory(alerts)
	for _, group := range categoryGroups {
		if len(group) >= 2 {
			result := ac.buildCorrelation(group, "同类告警跨节点关联")
			if result.Score >= ac.config.MinCorrelationScore {
				results = append(results, result)
			}
		}
	}

	// 2. 按拓扑关联
	if ac.config.EnableTopologyAware && ac.topology != nil {
		topoGroups := ac.groupByTopology(alerts)
		for _, group := range topoGroups {
			if len(group) >= 2 {
				result := ac.buildCorrelation(group, "拓扑依赖关联")
				if result.Score >= ac.config.MinCorrelationScore {
					results = append(results, result)
				}
			}
		}
	}

	// 3. 跨分类因果关联
	causalGroups := ac.groupByCausality(alerts)
	for _, group := range causalGroups {
		if len(group) >= 2 {
			result := ac.buildCorrelation(group, "因果链关联")
			if result.Score >= ac.config.MinCorrelationScore {
				results = append(results, result)
			}
		}
	}

	return results
}

// groupByCategory 按分类分组
func (ac *AlertCorrelator) groupByCategory(alerts []*NodeAlert) [][]*NodeAlert {
	groups := make(map[AlertCategory][]*NodeAlert)
	for _, alert := range alerts {
		groups[alert.Category] = append(groups[alert.Category], alert)
	}

	var result [][]*NodeAlert
	for _, group := range groups {
		if len(group) >= 2 {
			result = append(result, group)
		}
	}
	return result
}

// groupByTopology 按拓扑分组
func (ac *AlertCorrelator) groupByTopology(alerts []*NodeAlert) [][]*NodeAlert {
	// 找出有告警的节点
	alertNodes := make(map[string][]*NodeAlert)
	for _, alert := range alerts {
		alertNodes[alert.NodeID] = append(alertNodes[alert.NodeID], alert)
	}

	// BFS遍历拓扑，找到连通的告警节点组
	visited := make(map[string]bool)
	var groups [][]*NodeAlert

	for nodeID := range alertNodes {
		if visited[nodeID] {
			continue
		}

		// BFS
		queue := []string{nodeID}
		visited[nodeID] = true
		var group []*NodeAlert

		for len(queue) > 0 {
			current := queue[0]
			queue = queue[1:]

			group = append(group, alertNodes[current]...)

			// 检查邻居
			neighbors := ac.topology.GetNeighbors(current)
			upstream := ac.topology.GetUpstream(current)
			allRelated := append(neighbors, upstream...)

			for _, neighbor := range allRelated {
				if !visited[neighbor] {
					if _, hasAlert := alertNodes[neighbor]; hasAlert {
						visited[neighbor] = true
						queue = append(queue, neighbor)
					}
				}
			}
		}

		if len(group) >= 2 {
			groups = append(groups, group)
		}
	}

	return groups
}

// groupByCausality 按因果关系分组
func (ac *AlertCorrelator) groupByCausality(alerts []*NodeAlert) [][]*NodeAlert {
	// 预定义因果规则
	causalRules := []struct {
		cause      AlertCategory
		effect     AlertCategory
		maxDelay   time.Duration
		weight     float64
	}{
		{CategoryNetwork, CategoryApplication, 30 * time.Second, 0.9},
		{CategoryNetwork, CategoryDatabase, 30 * time.Second, 0.9},
		{CategoryNetwork, CategoryMiddleware, 30 * time.Second, 0.8},
		{CategoryNetwork, CategoryDNS, 10 * time.Second, 0.95},
		{CategoryDNS, CategoryApplication, 30 * time.Second, 0.85},
		{CategoryCPU, CategoryProcess, 60 * time.Second, 0.7},
		{CategoryMemory, CategoryProcess, 60 * time.Second, 0.7},
		{CategoryDisk, CategoryProcess, 60 * time.Second, 0.7},
		{CategoryCPU, CategoryApplication, 120 * time.Second, 0.6},
		{CategoryMemory, CategoryApplication, 120 * time.Second, 0.6},
		{CategoryLoadBalancer, CategoryApplication, 10 * time.Second, 0.9},
		{CategoryCertificate, CategoryNetwork, 0, 0.95},
		{CategoryNetwork, CategoryLoadBalancer, 10 * time.Second, 0.85},
	}

	// 按时间排序
	sorted := make([]*NodeAlert, len(alerts))
	copy(sorted, alerts)
	sort.Slice(sorted, func(i, j int) bool {
		return sorted[i].Timestamp.Before(sorted[j].Timestamp)
	})

	// 查找因果对
	pairs := make(map[string][]*NodeAlert)
	pairKeys := make(map[string]bool)

	for _, cause := range sorted {
		for _, rule := range causalRules {
			if cause.Category != rule.cause {
				continue
			}
			for _, effect := range sorted {
				if effect.Category != rule.effect {
					continue
				}
				if effect.Timestamp.Before(cause.Timestamp) {
					continue
				}
				delay := effect.Timestamp.Sub(cause.Timestamp)
				if delay <= rule.maxDelay {
					key := fmt.Sprintf("%s->%s:%s", cause.ID, effect.ID, rule.cause)
					if !pairKeys[key] {
						pairKeys[key] = true
						pairKey := fmt.Sprintf("%d_%d", rule.cause, rule.effect)
						pairs[pairKey] = append(pairs[pairKey], cause, effect)
					}
				}
			}
		}
	}

	var groups [][]*NodeAlert
	seen := make(map[string]bool)
	for _, group := range pairs {
		uniqueGroup := make([]*NodeAlert, 0)
		for _, alert := range group {
			if !seen[alert.ID] {
				seen[alert.ID] = true
				uniqueGroup = append(uniqueGroup, alert)
			}
		}
		if len(uniqueGroup) >= 2 {
			groups = append(groups, uniqueGroup)
		}
	}

	return groups
}

// buildCorrelation 构建关联结果
func (ac *AlertCorrelator) buildCorrelation(alerts []*NodeAlert, reason string) *CorrelationResult {
	if len(alerts) == 0 {
		return nil
	}

	// 计算关联分数
	score := ac.calculateCorrelationScore(alerts)

	// 确定主要分类
	categoryCount := make(map[AlertCategory]int)
	nodes := make(map[string]bool)
	for _, alert := range alerts {
		categoryCount[alert.Category]++
		nodes[alert.NodeID] = true
	}

	var mainCategory AlertCategory
	maxCount := 0
	for cat, count := range categoryCount {
		if count > maxCount {
			maxCount = count
			mainCategory = cat
		}
	}

	// 时间范围
	var startTime, endTime time.Time
	for _, alert := range alerts {
		if alert.Timestamp.Before(startTime) || startTime.IsZero() {
			startTime = alert.Timestamp
		}
		if alert.Timestamp.After(endTime) {
			endTime = alert.Timestamp
		}
	}

	return &CorrelationResult{
		AlertGroup:    alerts,
		CorrelationID: fmt.Sprintf("corr-%d", time.Now().UnixNano()),
		Score:         score,
		Reason:        reason,
		Category:      mainCategory,
		StartTime:     startTime,
		EndTime:       endTime,
		NodeCount:     len(nodes),
	}
}

// calculateCorrelationScore 计算关联分数
func (ac *AlertCorrelator) calculateCorrelationScore(alerts []*NodeAlert) float64 {
	if len(alerts) < 2 {
		return 0
	}

	score := 0.0

	// 因子1: 时间接近度
	timeRange := alerts[len(alerts)-1].Timestamp.Sub(alerts[0].Timestamp)
	if timeRange <= ac.config.CorrelationWindow {
		timeScore := 1.0 - (timeRange.Seconds() / ac.config.CorrelationWindow.Seconds())
		score += timeScore * 0.3
	}

	// 因子2: 分类一致性
	categoryCount := make(map[AlertCategory]int)
	for _, alert := range alerts {
		categoryCount[alert.Category]++
	}
	maxCategoryRatio := 0.0
	for _, count := range categoryCount {
		ratio := float64(count) / float64(len(alerts))
		if ratio > maxCategoryRatio {
			maxCategoryRatio = ratio
		}
	}
	score += maxCategoryRatio * 0.2

	// 因子3: 跨节点分布
	nodes := make(map[string]bool)
	for _, alert := range alerts {
		nodes[alert.NodeID] = true
	}
	nodeRatio := float64(len(nodes)) / float64(len(alerts))
	if nodeRatio > 0.5 {
		score += 0.2 // 多节点告警更可能是关联的
	}

	// 因子4: 严重程度一致性
	severities := make(map[AlertSeverity]int)
	for _, alert := range alerts {
		severities[alert.Severity]++
	}
	maxSeverityRatio := 0.0
	for _, count := range severities {
		ratio := float64(count) / float64(len(alerts))
		if ratio > maxSeverityRatio {
			maxSeverityRatio = ratio
		}
	}
	score += maxSeverityRatio * 0.15

	// 因子5: 拓扑邻近度
	if ac.config.EnableTopologyAware && ac.topology != nil {
		topoScore := ac.calculateTopologyScore(alerts)
		score += topoScore * 0.15
	}

	return math.Min(1.0, score)
}

// calculateTopologyScore 计算拓扑分数
func (ac *AlertCorrelator) calculateTopologyScore(alerts []*NodeAlert) float64 {
	if len(alerts) < 2 {
		return 0
	}

	nodes := make(map[string]bool)
	for _, alert := range alerts {
		nodes[alert.NodeID] = true
	}

	nodeList := make([]string, 0, len(nodes))
	for node := range nodes {
		nodeList = append(nodeList, node)
	}

	// 计算节点间的拓扑距离
	totalDistance := 0.0
	pairs := 0

	for i := 0; i < len(nodeList); i++ {
		for j := i + 1; j < len(nodeList); j++ {
			dist := ac.topologyDistance(nodeList[i], nodeList[j])
			totalDistance += dist
			pairs++
		}
	}

	if pairs == 0 {
		return 0
	}

	avgDistance := totalDistance / float64(pairs)
	// 距离越近分数越高
	return 1.0 / (1.0 + avgDistance)
}

// topologyDistance 计算拓扑距离（BFS）
func (ac *AlertCorrelator) topologyDistance(from, to string) float64 {
	if from == to {
		return 0
	}

	// BFS
	visited := make(map[string]bool)
	queue := []struct {
		node string
		dist float64
	}{{from, 0}}
	visited[from] = true

	for len(queue) > 0 {
		current := queue[0]
		queue = queue[1:]

		if current.node == to {
			return current.dist
		}

		neighbors := ac.topology.GetNeighbors(current.node)
		upstream := ac.topology.GetUpstream(current.node)
		allRelated := append(neighbors, upstream...)

		for _, neighbor := range allRelated {
			if !visited[neighbor] {
				visited[neighbor] = true
				weight := ac.topology.GetLinkWeight(current.node, neighbor)
				queue = append(queue, struct {
					node string
					dist float64
				}{neighbor, current.dist + weight})
			}
		}
	}

	return 10.0 // 不可达
}
