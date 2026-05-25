//go:build linux

// Package alert 提供告警管理功能
// 本文件实现告警根因分析(RCA)引擎
// 基于服务依赖拓扑，多节点告警时自动推导故障根因节点
// 给出故障影响范围和初步解决建议，生成根因分析报告
package alert

import (
	"fmt"
	"sort"
	"strings"
	"sync"
	"time"

	"cloud-flow-agent/pkg/logger"
)

// ==================== 服务拓扑模型 ====================

// ServiceNode 服务节点
type ServiceNode struct {
	ID          string            `json:"id"`           // 节点ID
	Name        string            `json:"name"`         // 服务名称
	Type        ServiceType       `json:"type"`         // 服务类型
	Labels      map[string]string `json:"labels"`       // 标签
	Metadata    map[string]string `json:"metadata"`     // 元数据
	DependsOn   []string          `json:"depends_on"`   // 依赖的服务ID列表
	Dependents  []string          `json:"dependents"`   // 依赖本服务的服务ID列表
	LastUpdated time.Time         `json:"last_updated"` // 最后更新时间
}

// ServiceType 服务类型
type ServiceType string

const (
	ServiceTypeDatabase   ServiceType = "database"   // 数据库
	ServiceTypeCache      ServiceType = "cache"      // 缓存
	ServiceTypeGateway    ServiceType = "gateway"    // 网关
	ServiceTypeAPI        ServiceType = "api"        // API服务
	ServiceTypeWorker     ServiceType = "worker"     // 工作节点
	ServiceTypeQueue      ServiceType = "queue"      // 消息队列
	ServiceTypeExternal   ServiceType = "external"   // 外部依赖
)

// String 返回服务类型字符串
func (t ServiceType) String() string {
	return string(t)
}

// ServiceTopology 服务拓扑
type ServiceTopology struct {
	mu     sync.RWMutex
	nodes  map[string]*ServiceNode  // 节点ID -> 节点
	alerts map[string][]*AlertEvent // 节点ID -> 告警列表
}

// NewServiceTopology 创建服务拓扑
func NewServiceTopology() *ServiceTopology {
	return &ServiceTopology{
		nodes:  make(map[string]*ServiceNode),
		alerts: make(map[string][]*AlertEvent),
	}
}

// AddNode 添加节点
func (t *ServiceTopology) AddNode(node *ServiceNode) {
	t.mu.Lock()
	defer t.mu.Unlock()

	node.LastUpdated = time.Now()
	t.nodes[node.ID] = node

	// 更新依赖关系
	for _, depID := range node.DependsOn {
		if dep, ok := t.nodes[depID]; ok {
			// 检查是否已存在
			exists := false
			for _, d := range dep.Dependents {
				if d == node.ID {
					exists = true
					break
				}
			}
			if !exists {
				dep.Dependents = append(dep.Dependents, node.ID)
			}
		}
	}
}

// RemoveNode 移除节点
func (t *ServiceTopology) RemoveNode(id string) {
	t.mu.Lock()
	defer t.mu.Unlock()

	delete(t.nodes, id)
	delete(t.alerts, id)

	// 清理其他节点的依赖关系
	for _, node := range t.nodes {
		node.DependsOn = removeString(node.DependsOn, id)
		node.Dependents = removeString(node.Dependents, id)
	}
}

// GetNode 获取节点
func (t *ServiceTopology) GetNode(id string) *ServiceNode {
	t.mu.RLock()
	defer t.mu.RUnlock()
	return t.nodes[id]
}

// GetAllNodes 获取所有节点
func (t *ServiceTopology) GetAllNodes() []*ServiceNode {
	t.mu.RLock()
	defer t.mu.RUnlock()

	nodes := make([]*ServiceNode, 0, len(t.nodes))
	for _, node := range t.nodes {
		nodes = append(nodes, node)
	}
	return nodes
}

// AddAlert 添加告警到节点
func (t *ServiceTopology) AddAlert(nodeID string, alert *AlertEvent) {
	t.mu.Lock()
	defer t.mu.Unlock()

	t.alerts[nodeID] = append(t.alerts[nodeID], alert)
}

// GetAlerts 获取节点的告警
func (t *ServiceTopology) GetAlerts(nodeID string) []*AlertEvent {
	t.mu.RLock()
	defer t.mu.RUnlock()
	return t.alerts[nodeID]
}

// GetAllAlerts 获取所有节点的告警
func (t *ServiceTopology) GetAllAlerts() map[string][]*AlertEvent {
	t.mu.RLock()
	defer t.mu.RUnlock()

	result := make(map[string][]*AlertEvent)
	for k, v := range t.alerts {
		result[k] = v
	}
	return result
}

// ClearAlerts 清空告警
func (t *ServiceTopology) ClearAlerts() {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.alerts = make(map[string][]*AlertEvent)
}

// ==================== 根因分析引擎 ====================

// RCAEngine 根因分析引擎
type RCAEngine struct {
	topology    *ServiceTopology
	log         *logger.Logger
	config      RCAConfig
	
	// 分析结果缓存
	resultsMu   sync.RWMutex
	results     map[string]*RCAResult // 分析ID -> 结果
	
	// 控制
	stopCh      chan struct{}
	wg          sync.WaitGroup
}

// RCAConfig 根因分析配置
type RCAConfig struct {
	AnalysisWindow      time.Duration `json:"analysis_window"`       // 分析时间窗口
	MinAlertCount       int           `json:"min_alert_count"`       // 触发分析的最小告警数
	MaxDepth            int           `json:"max_depth"`             // 拓扑遍历最大深度
	EnableAutoAnalysis  bool          `json:"enable_auto_analysis"`  // 启用自动分析
	AnalysisInterval    time.Duration `json:"analysis_interval"`     // 自动分析间隔
}

// DefaultRCAConfig 默认RCA配置
func DefaultRCAConfig() RCAConfig {
	return RCAConfig{
		AnalysisWindow:     5 * time.Minute,
		MinAlertCount:      2,
		MaxDepth:           5,
		EnableAutoAnalysis: true,
		AnalysisInterval:   30 * time.Second,
	}
}

// NewRCAEngine 创建根因分析引擎
func NewRCAEngine(log *logger.Logger) *RCAEngine {
	return &RCAEngine{
		topology: NewServiceTopology(),
		log:      log,
		config:   DefaultRCAConfig(),
		results:  make(map[string]*RCAResult),
		stopCh:   make(chan struct{}),
	}
}

// Start 启动RCA引擎
func (e *RCAEngine) Start() {
	if e.config.EnableAutoAnalysis {
		e.wg.Add(1)
		go e.autoAnalysisLoop()
	}
	e.log.Info("根因分析引擎已启动")
}

// Stop 停止RCA引擎
func (e *RCAEngine) Stop() {
	close(e.stopCh)
	e.wg.Wait()
	e.log.Info("根因分析引擎已停止")
}

// autoAnalysisLoop 自动分析循环
func (e *RCAEngine) autoAnalysisLoop() {
	defer e.wg.Done()

	ticker := time.NewTicker(e.config.AnalysisInterval)
	defer ticker.Stop()

	for {
		select {
		case <-e.stopCh:
			return
		case <-ticker.C:
			e.RunAnalysis()
		}
	}
}

// RecordMetric 记录指标数据（用于拓扑发现）
func (e *RCAEngine) RecordMetric(metric MetricData) {
	// 从指标标签中提取服务信息
	instance := metric.Labels["instance"]
	if instance == "" {
		instance = metric.Labels["host"]
	}
	if instance == "" {
		return
	}

	// 检查节点是否存在，不存在则创建
	nodeID := instance
	if node := e.topology.GetNode(nodeID); node == nil {
		// 创建新节点
		node := &ServiceNode{
			ID:         nodeID,
			Name:       metric.Labels["service"] + "-" + instance,
			Type:       inferServiceType(metric.Labels["service"]),
			Labels:     metric.Labels,
			Metadata:   make(map[string]string),
			DependsOn:  []string{},
			Dependents: []string{},
		}
		e.topology.AddNode(node)
	}
}

// AnalyzeAlert 分析单个告警
func (e *RCAEngine) AnalyzeAlert(alert *AlertEvent) {
	// 从告警中提取节点ID
	instance := alert.Labels["instance"]
	if instance == "" {
		instance = alert.Labels["host"]
	}
	if instance == "" {
		return
	}

	// 添加告警到拓扑
	e.topology.AddAlert(instance, alert)

	// 检查是否达到分析阈值
	alerts := e.topology.GetAlerts(instance)
	if len(alerts) >= e.config.MinAlertCount {
		e.RunAnalysis()
	}
}

// RunAnalysis 执行根因分析
func (e *RCAEngine) RunAnalysis() *RCAResult {
	allAlerts := e.topology.GetAllAlerts()
	
	// 检查是否有足够告警
	totalAlerts := 0
	for _, alerts := range allAlerts {
		totalAlerts += len(alerts)
	}
	
	if totalAlerts < e.config.MinAlertCount {
		return nil
	}

	e.log.Infof("开始根因分析: %d个节点, %d个告警", len(allAlerts), totalAlerts)

	// 构建候选根因列表
	candidates := e.identifyRootCauseCandidates(allAlerts)
	
	// 排序并选择最可能的根因
	rootCause := e.selectRootCause(candidates)
	
	// 分析影响范围
	impact := e.analyzeImpact(rootCause)
	
	// 生成解决建议
	suggestions := e.generateSuggestions(rootCause, impact)
	
	// 生成报告
	result := &RCAResult{
		ID:            fmt.Sprintf("rca-%d", time.Now().UnixNano()),
		Timestamp:     time.Now(),
		RootCause:     rootCause,
		Impact:        impact,
		Suggestions:   suggestions,
		AllAlerts:     allAlerts,
		Confidence:    e.calculateConfidence(rootCause, candidates),
	}

	// 缓存结果
	e.resultsMu.Lock()
	e.results[result.ID] = result
	e.resultsMu.Unlock()

	e.log.Infof("根因分析完成: 根因=%s, 置信度=%.2f, 影响节点=%d",
		rootCause.NodeID, result.Confidence, len(impact.AffectedNodes))

	return result
}

// identifyRootCauseCandidates 识别根因候选
func (e *RCAEngine) identifyRootCauseCandidates(alerts map[string][]*AlertEvent) []*RootCauseCandidate {
	candidates := make([]*RootCauseCandidate, 0)

	for nodeID, nodeAlerts := range alerts {
		node := e.topology.GetNode(nodeID)
		if node == nil {
			continue
		}

		// 计算候选得分
		score := e.calculateCandidateScore(node, nodeAlerts)
		
		candidate := &RootCauseCandidate{
			NodeID:      nodeID,
			Node:        node,
			Alerts:      nodeAlerts,
			Score:       score,
			AlertCount:  len(nodeAlerts),
			CriticalCount: countCriticalAlerts(nodeAlerts),
		}
		
		candidates = append(candidates, candidate)
	}

	// 按得分排序
	sort.Slice(candidates, func(i, j int) bool {
		return candidates[i].Score > candidates[j].Score
	})

	return candidates
}

// calculateCandidateScore 计算候选得分
func (e *RCAEngine) calculateCandidateScore(node *ServiceNode, alerts []*AlertEvent) float64 {
	score := 0.0

	// 1. 基础分：告警数量
	score += float64(len(alerts)) * 10.0

	// 2. 严重级别加权
	for _, alert := range alerts {
		switch alert.Level {
		case AlertLevelCritical:
			score += 50.0
		case AlertLevelWarning:
			score += 20.0
		case AlertLevelInfo:
			score += 5.0
		}
	}

	// 3. 拓扑位置加权（依赖越多越可能是根因）
	score += float64(len(node.DependsOn)) * 5.0
	score += float64(len(node.Dependents)) * 3.0

	// 4. 服务类型加权
	switch node.Type {
	case ServiceTypeDatabase, ServiceTypeCache:
		score *= 1.5 // 基础设施权重更高
	case ServiceTypeGateway:
		score *= 1.3
	case ServiceTypeQueue:
		score *= 1.2
	}

	// 5. 时间集中度（告警越集中越可能是根因）
	if len(alerts) > 1 {
		timeRange := alerts[len(alerts)-1].FiredAt.Sub(alerts[0].FiredAt)
		if timeRange < time.Minute {
			score *= 1.2
		}
	}

	return score
}

// selectRootCause 选择根因
func (e *RCAEngine) selectRootCause(candidates []*RootCauseCandidate) *RootCause {
	if len(candidates) == 0 {
		return nil
	}

	// 选择得分最高的候选
	best := candidates[0]
	
	return &RootCause{
		NodeID:       best.NodeID,
		NodeName:     best.Node.Name,
		NodeType:     best.Node.Type,
		Alerts:       best.Alerts,
		Score:        best.Score,
		Reason:       e.generateRootCauseReason(best),
	}
}

// generateRootCauseReason 生成根因原因描述
func (e *RCAEngine) generateRootCauseReason(candidate *RootCauseCandidate) string {
	reasons := make([]string, 0)

	// 根据告警类型生成原因
	alertTypes := make(map[string]int)
	for _, alert := range candidate.Alerts {
		alertTypes[alert.Metric]++
	}

	// 找出最多的告警类型
	var mainMetric string
	var maxCount int
	for metric, count := range alertTypes {
		if count > maxCount {
			maxCount = count
			mainMetric = metric
		}
	}

	reasons = append(reasons, fmt.Sprintf("节点 %s (%s) 出现 %d 个告警", 
		candidate.Node.Name, candidate.Node.Type, len(candidate.Alerts)))
	
	if mainMetric != "" {
		reasons = append(reasons, fmt.Sprintf("主要告警类型: %s (%d次)", mainMetric, maxCount))
	}

	if len(candidate.Node.DependsOn) > 0 {
		reasons = append(reasons, fmt.Sprintf("该节点依赖 %d 个上游服务", len(candidate.Node.DependsOn)))
	}

	if len(candidate.Node.Dependents) > 0 {
		reasons = append(reasons, fmt.Sprintf("有 %d 个下游服务依赖该节点", len(candidate.Node.Dependents)))
	}

	return strings.Join(reasons, "; ")
}

// analyzeImpact 分析影响范围
func (e *RCAEngine) analyzeImpact(rootCause *RootCause) *ImpactAnalysis {
	if rootCause == nil {
		return nil
	}

	impact := &ImpactAnalysis{
		RootCauseID:   rootCause.NodeID,
		AffectedNodes: []string{},
		AffectedPaths: []string{},
		Severity:      "medium",
	}

	// BFS遍历下游依赖
	visited := make(map[string]bool)
	queue := []string{rootCause.NodeID}
	depth := 0

	for len(queue) > 0 && depth < e.config.MaxDepth {
		size := len(queue)
		for i := 0; i < size; i++ {
			nodeID := queue[0]
			queue = queue[1:]

			if visited[nodeID] {
				continue
			}
			visited[nodeID] = true

			if nodeID != rootCause.NodeID {
				impact.AffectedNodes = append(impact.AffectedNodes, nodeID)
			}

			node := e.topology.GetNode(nodeID)
			if node != nil {
				for _, dep := range node.Dependents {
					if !visited[dep] {
						queue = append(queue, dep)
						path := fmt.Sprintf("%s -> %s", nodeID, dep)
						impact.AffectedPaths = append(impact.AffectedPaths, path)
					}
				}
			}
		}
		depth++
	}

	// 评估严重程度
	affectedCount := len(impact.AffectedNodes)
	if affectedCount > 10 {
		impact.Severity = "critical"
	} else if affectedCount > 5 {
		impact.Severity = "high"
	} else if affectedCount > 0 {
		impact.Severity = "medium"
	} else {
		impact.Severity = "low"
	}

	impact.AffectedCount = affectedCount

	return impact
}

// generateSuggestions 生成解决建议
func (e *RCAEngine) generateSuggestions(rootCause *RootCause, impact *ImpactAnalysis) []string {
	suggestions := make([]string, 0)

	if rootCause == nil {
		suggestions = append(suggestions, "未能确定根因，建议人工排查")
		return suggestions
	}

	// 根据服务类型生成建议
	switch rootCause.NodeType {
	case ServiceTypeDatabase:
		suggestions = append(suggestions, []string{
			"检查数据库连接池状态和连接数",
			"查看数据库慢查询日志",
			"检查数据库服务器资源使用情况（CPU/内存/磁盘IO）",
			"确认是否有锁等待或死锁",
		}...)

	case ServiceTypeCache:
		suggestions = append(suggestions, []string{
			"检查缓存命中率",
			"查看缓存内存使用情况",
			"检查缓存过期策略",
			"确认是否有缓存穿透或雪崩",
		}...)

	case ServiceTypeGateway:
		suggestions = append(suggestions, []string{
			"检查网关连接数和线程池状态",
			"查看网关路由配置",
			"检查后端服务健康状态",
			"确认是否有流量突增",
		}...)

	case ServiceTypeQueue:
		suggestions = append(suggestions, []string{
			"检查消息队列积压情况",
			"查看消费者状态和消费速率",
			"检查队列服务器资源使用",
			"确认是否有消息堆积",
		}...)

	default:
		suggestions = append(suggestions, []string{
			"检查服务进程状态和资源使用",
			"查看服务日志中的错误信息",
			"检查服务依赖的外部资源",
			"确认服务配置是否正确",
		}...)
	}

	// 根据影响范围添加建议
	if impact != nil && impact.AffectedCount > 0 {
		suggestions = append(suggestions, fmt.Sprintf(
			"该故障影响 %d 个下游服务，建议优先恢复根因节点", impact.AffectedCount))
	}

	return suggestions
}

// calculateConfidence 计算置信度
func (e *RCAEngine) calculateConfidence(rootCause *RootCause, candidates []*RootCauseCandidate) float64 {
	if rootCause == nil || len(candidates) == 0 {
		return 0.0
	}

	// 基于得分差距计算置信度
	if len(candidates) == 1 {
		return 0.9
	}

	// 计算与第二名的得分差距
	bestScore := candidates[0].Score
	secondScore := candidates[1].Score
	
	if secondScore == 0 {
		return 0.95
	}

	// 差距越大，置信度越高
	ratio := bestScore / secondScore
	confidence := 0.5 + (ratio-1)*0.2
	if confidence > 0.95 {
		confidence = 0.95
	}
	if confidence < 0.5 {
		confidence = 0.5
	}

	return confidence
}

// GetResult 获取分析结果
func (e *RCAEngine) GetResult(id string) *RCAResult {
	e.resultsMu.RLock()
	defer e.resultsMu.RUnlock()
	return e.results[id]
}

// GetAllResults 获取所有分析结果
func (e *RCAEngine) GetAllResults() []*RCAResult {
	e.resultsMu.RLock()
	defer e.resultsMu.RUnlock()

	results := make([]*RCAResult, 0, len(e.results))
	for _, r := range e.results {
		results = append(results, r)
	}
	
	// 按时间倒序
	sort.Slice(results, func(i, j int) bool {
		return results[i].Timestamp.After(results[j].Timestamp)
	})
	
	return results
}

// GetTopology 获取服务拓扑
func (e *RCAEngine) GetTopology() *ServiceTopology {
	return e.topology
}

// ==================== 分析结果模型 ====================

// RCAResult 根因分析结果
type RCAResult struct {
	ID          string                   `json:"id"`
	Timestamp   time.Time                `json:"timestamp"`
	RootCause   *RootCause               `json:"root_cause"`
	Impact      *ImpactAnalysis          `json:"impact"`
	Suggestions []string                 `json:"suggestions"`
	AllAlerts   map[string][]*AlertEvent `json:"all_alerts"`
	Confidence  float64                  `json:"confidence"`
}

// RootCause 根因
type RootCause struct {
	NodeID   string        `json:"node_id"`
	NodeName string        `json:"node_name"`
	NodeType ServiceType   `json:"node_type"`
	Alerts   []*AlertEvent `json:"alerts"`
	Score    float64       `json:"score"`
	Reason   string        `json:"reason"`
}

// ImpactAnalysis 影响分析
type ImpactAnalysis struct {
	RootCauseID   string   `json:"root_cause_id"`
	AffectedNodes []string `json:"affected_nodes"`
	AffectedPaths []string `json:"affected_paths"`
	AffectedCount int      `json:"affected_count"`
	Severity      string   `json:"severity"`
}

// RootCauseCandidate 根因候选
type RootCauseCandidate struct {
	NodeID        string        `json:"node_id"`
	Node          *ServiceNode  `json:"node"`
	Alerts        []*AlertEvent `json:"alerts"`
	Score         float64       `json:"score"`
	AlertCount    int           `json:"alert_count"`
	CriticalCount int           `json:"critical_count"`
}

// ==================== 辅助函数 ====================

// inferServiceType 推断服务类型
func inferServiceType(serviceName string) ServiceType {
	serviceName = strings.ToLower(serviceName)
	
	if strings.Contains(serviceName, "db") || strings.Contains(serviceName, "database") || 
	   strings.Contains(serviceName, "mysql") || strings.Contains(serviceName, "postgres") {
		return ServiceTypeDatabase
	}
	
	if strings.Contains(serviceName, "cache") || strings.Contains(serviceName, "redis") ||
	   strings.Contains(serviceName, "memcached") {
		return ServiceTypeCache
	}
	
	if strings.Contains(serviceName, "gateway") || strings.Contains(serviceName, "nginx") ||
	   strings.Contains(serviceName, "ingress") {
		return ServiceTypeGateway
	}
	
	if strings.Contains(serviceName, "queue") || strings.Contains(serviceName, "kafka") ||
	   strings.Contains(serviceName, "rabbitmq") {
		return ServiceTypeQueue
	}
	
	if strings.Contains(serviceName, "worker") || strings.Contains(serviceName, "job") {
		return ServiceTypeWorker
	}
	
	return ServiceTypeAPI
}

// countCriticalAlerts 统计严重告警数
func countCriticalAlerts(alerts []*AlertEvent) int {
	count := 0
	for _, alert := range alerts {
		if alert.Level == AlertLevelCritical {
			count++
		}
	}
	return count
}

// removeString 从切片中移除字符串
func removeString(slice []string, s string) []string {
	result := make([]string, 0, len(slice))
	for _, item := range slice {
		if item != s {
			result = append(result, item)
		}
	}
	return result
}

// GenerateReport 生成分析报告文本
func (r *RCAResult) GenerateReport() string {
	if r == nil {
		return "无分析结果"
	}

	var sb strings.Builder
	
	sb.WriteString("========================================\n")
	sb.WriteString("           根因分析报告\n")
	sb.WriteString("========================================\n\n")
	
	sb.WriteString(fmt.Sprintf("报告ID: %s\n", r.ID))
	sb.WriteString(fmt.Sprintf("生成时间: %s\n", r.Timestamp.Format("2006-01-02 15:04:05")))
	sb.WriteString(fmt.Sprintf("置信度: %.1f%%\n\n", r.Confidence*100))
	
	if r.RootCause != nil {
		sb.WriteString("【根因节点】\n")
		sb.WriteString(fmt.Sprintf("  节点ID: %s\n", r.RootCause.NodeID))
		sb.WriteString(fmt.Sprintf("  节点名称: %s\n", r.RootCause.NodeName))
		sb.WriteString(fmt.Sprintf("  节点类型: %s\n", r.RootCause.NodeType))
		sb.WriteString(fmt.Sprintf("  告警数量: %d\n", len(r.RootCause.Alerts)))
		sb.WriteString(fmt.Sprintf("  分析原因: %s\n\n", r.RootCause.Reason))
	}
	
	if r.Impact != nil {
		sb.WriteString("【影响范围】\n")
		sb.WriteString(fmt.Sprintf("  严重程度: %s\n", r.Impact.Severity))
		sb.WriteString(fmt.Sprintf("  影响节点数: %d\n", r.Impact.AffectedCount))
		if len(r.Impact.AffectedNodes) > 0 {
			sb.WriteString(fmt.Sprintf("  影响节点: %s\n", strings.Join(r.Impact.AffectedNodes, ", ")))
		}
		if len(r.Impact.AffectedPaths) > 0 {
			sb.WriteString("  影响路径:\n")
			for _, path := range r.Impact.AffectedPaths {
				sb.WriteString(fmt.Sprintf("    - %s\n", path))
			}
		}
		sb.WriteString("\n")
	}
	
	if len(r.Suggestions) > 0 {
		sb.WriteString("【解决建议】\n")
		for i, suggestion := range r.Suggestions {
			sb.WriteString(fmt.Sprintf("  %d. %s\n", i+1, suggestion))
		}
		sb.WriteString("\n")
	}
	
	sb.WriteString("========================================\n")
	
	return sb.String()
}
