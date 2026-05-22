// Package rca 根因推导引擎
// Copyright (c) 2026 Cloud Flow Team
// Licensed under the MIT License.

package rca

import (
	"fmt"
	"sort"
	"strings"
	"sync"
	"time"
)

// ============================================================
// 根因分析结果
// ============================================================

// RootCauseType 根因类型
type RootCauseType int

const (
	RCTypeInfrastructure RootCauseType = iota
	RCTypeNetwork
	RCTypeResource
	RCTypeApplication
	RCTypeConfiguration
	RCTypeExternal
	RCTypeUnknown
)

func (r RootCauseType) String() string {
	names := map[RootCauseType]string{
		RCTypeInfrastructure: "基础设施",
		RCTypeNetwork:        "网络",
		RCTypeResource:       "资源",
		RCTypeApplication:    "应用",
		RCTypeConfiguration:  "配置",
		RCTypeExternal:       "外部依赖",
		RCTypeUnknown:        "未知",
	}
	if name, ok := names[r]; ok {
		return name
	}
	return "未知"
}

// RootCause 根因
type RootCause struct {
	ID              string          `json:"id"`
	Timestamp       time.Time       `json:"timestamp"`
	Type            RootCauseType   `json:"type"`
	Category        AlertCategory   `json:"category"`
	SourceNode      string          `json:"source_node"`
	SourceAlert     *NodeAlert      `json:"source_alert"`
	Confidence      float64         `json:"confidence"`       // 置信度 0-1
	Description     string          `json:"description"`      // 根因描述
	ImpactScope     []string        `json:"impact_scope"`     // 影响范围（节点列表）
	AffectedAlerts  []*NodeAlert    `json:"affected_alerts"`  // 受影响的告警
	CausalChain     []CausalLink    `json:"causal_chain"`     // 因果链
	Suggestions     []Suggestion    `json:"suggestions"`      // 修复建议
	Evidence        []string        `json:"evidence"`         // 证据
}

// CausalLink 因果链链接
type CausalLink struct {
	FromAlert   *NodeAlert `json:"from_alert"`
	ToAlert     *NodeAlert `json:"to_alert"`
	Relation    string     `json:"relation"`     // 关系描述
	Probability float64    `json:"probability"`  // 因果概率
	Delay       time.Duration `json:"delay"`     // 延迟
}

// Suggestion 修复建议
type Suggestion struct {
	ID          string  `json:"id"`
	Priority    int     `json:"priority"`     // 优先级 1-10
	Action      string  `json:"action"`       // 操作描述
	Target      string  `json:"target"`       // 操作目标
	AutoFixable bool    `json:"auto_fixable"` // 是否可自动修复
	Risk        string  `json:"risk"`         // 风险等级
	EstimateTime string `json:"estimate_time"` // 预估耗时
}

// ============================================================
// 根因推导引擎
// ============================================================

// RootCauseEngine 根因推导引擎
type RootCauseEngine struct {
	config    *RCAConfig
	topology  *TopologyGraph
	correlator *AlertCorrelator
	rules     []*CausalRule
}

// CausalRule 因果规则
type CausalRule struct {
	Name           string
	CauseCategory  AlertCategory
	EffectCategory AlertCategory
	CausePattern   string // 告警消息匹配模式
	EffectPattern  string
	MaxDelay       time.Duration
	Weight         float64
	Description    string
}

// NewRootCauseEngine 创建根因推导引擎
func NewRootCauseEngine(cfg *RCAConfig, topo *TopologyGraph) *RootCauseEngine {
	engine := &RootCauseEngine{
		config:    cfg,
		topology:  topo,
		correlator: NewAlertCorrelator(cfg, topo),
	}

	engine.initRules()
	return engine
}

// initRules 初始化因果规则
func (e *RootCauseEngine) initRules() {
	e.rules = []*CausalRule{
		// 网络根因
		{Name: "网络丢包导致应用超时", CauseCategory: CategoryNetwork, EffectCategory: CategoryApplication,
			CausePattern: "packet_loss", EffectPattern: "timeout", MaxDelay: 30 * time.Second, Weight: 0.9,
			Description: "网络丢包率升高导致应用请求超时"},
		{Name: "DNS解析失败", CauseCategory: CategoryDNS, EffectCategory: CategoryApplication,
			CausePattern: "dns", EffectPattern: "resolve", MaxDelay: 10 * time.Second, Weight: 0.95,
			Description: "DNS解析失败导致服务不可用"},
		{Name: "负载均衡器故障", CauseCategory: CategoryLoadBalancer, EffectCategory: CategoryApplication,
			CausePattern: "lb", EffectPattern: "unhealthy", MaxDelay: 10 * time.Second, Weight: 0.9,
			Description: "负载均衡器故障导致后端服务不可用"},
		{Name: "证书过期", CauseCategory: CategoryCertificate, EffectCategory: CategoryNetwork,
			CausePattern: "certificate", EffectPattern: "tls", MaxDelay: 0, Weight: 0.95,
			Description: "TLS证书过期导致连接失败"},

		// 资源根因
		{Name: "CPU过载", CauseCategory: CategoryCPU, EffectCategory: CategoryProcess,
			CausePattern: "cpu", EffectPattern: "process", MaxDelay: 60 * time.Second, Weight: 0.7,
			Description: "CPU使用率过高导致进程异常"},
		{Name: "内存不足", CauseCategory: CategoryMemory, EffectCategory: CategoryProcess,
			CausePattern: "memory", EffectPattern: "oom", MaxDelay: 60 * time.Second, Weight: 0.8,
			Description: "内存不足导致进程OOM"},
		{Name: "磁盘满", CauseCategory: CategoryDisk, EffectCategory: CategoryProcess,
			CausePattern: "disk", EffectPattern: "write", MaxDelay: 60 * time.Second, Weight: 0.75,
			Description: "磁盘空间不足导致写入失败"},

		// 级联故障
		{Name: "数据库故障级联", CauseCategory: CategoryDatabase, EffectCategory: CategoryApplication,
			CausePattern: "database", EffectPattern: "error", MaxDelay: 30 * time.Second, Weight: 0.85,
			Description: "数据库故障导致应用层错误"},
		{Name: "中间件故障级联", CauseCategory: CategoryMiddleware, EffectCategory: CategoryApplication,
			CausePattern: "middleware", EffectPattern: "error", MaxDelay: 30 * time.Second, Weight: 0.85,
			Description: "中间件故障导致应用层错误"},
		{Name: "网络故障级联", CauseCategory: CategoryNetwork, EffectCategory: CategoryDatabase,
			CausePattern: "network", EffectPattern: "connect", MaxDelay: 20 * time.Second, Weight: 0.8,
			Description: "网络故障导致数据库连接失败"},
	}
}

// Analyze 分析告警，推导根因
func (e *RootCauseEngine) Analyze(alerts []*NodeAlert) []*RootCause {
	if len(alerts) == 0 {
		return nil
	}

	// 1. 关联告警
	correlations := e.correlator.Correlate(alerts)

	// 2. 对每个关联组进行根因分析
	var rootCauses []*RootCause
	for _, corr := range correlations {
		rcs := e.analyzeCorrelation(corr)
		rootCauses = append(rootCauses, rcs...)
	}

	// 3. 如果没有关联结果，对单个告警分析
	if len(rootCauses) == 0 {
		for _, alert := range alerts {
			rc := e.analyzeSingleAlert(alert)
			if rc != nil {
				rootCauses = append(rootCauses, rc)
			}
		}
	}

	// 4. 去重和排序
	rootCauses = e.deduplicateRootCauses(rootCauses)
	sort.Slice(rootCauses, func(i, j int) bool {
		return rootCauses[i].Confidence > rootCauses[j].Confidence
	})

	// 5. 限制数量
	if len(rootCauses) > e.config.MaxRootCauses {
		rootCauses = rootCauses[:e.config.MaxRootCauses]
	}

	// 6. 生成建议
	for _, rc := range rootCauses {
		rc.Suggestions = e.generateSuggestions(rc)
	}

	return rootCauses
}

// analyzeCorrelation 分析关联组
func (e *RootCauseEngine) analyzeCorrelation(corr *CorrelationResult) []*RootCause {
	alerts := corr.AlertGroup

	// 按时间排序
	sorted := make([]*NodeAlert, len(alerts))
	copy(sorted, alerts)
	sort.Slice(sorted, func(i, j int) bool {
		return sorted[i].Timestamp.Before(sorted[j].Timestamp)
	})

	// 查找因果链
	causalChains := e.findCausalChains(sorted)

	// 为每条因果链生成根因
	var rootCauses []*RootCause
	for _, chain := range causalChains {
		if len(chain) > 0 {
			rc := e.buildRootCause(chain, corr)
			rootCauses = append(rootCauses, rc)
		}
	}

	return rootCauses
}

// findCausalChains 查找因果链
func (e *RootCauseEngine) findCausalChains(alerts []*NodeAlert) [][]CausalLink {
	var chains [][]CausalLink

	for i, cause := range alerts {
		var chain []CausalLink

		for j := i + 1; j < len(alerts); j++ {
			effect := alerts[j]

			// 检查因果规则
			link := e.evaluateCausalRule(cause, effect)
			if link != nil {
				chain = append(chain, *link)
			}
		}

		if len(chain) > 0 {
			chains = append(chains, chain)
		}
	}

	// 如果没有找到规则匹配的因果链，使用启发式方法
	if len(chains) == 0 {
		chain := e.heuristicCausalChain(alerts)
		if len(chain) > 0 {
			chains = append(chains, chain)
		}
	}

	return chains
}

// evaluateCausalRule 评估因果规则
func (e *RootCauseEngine) evaluateCausalRule(cause, effect *NodeAlert) *CausalLink {
	for _, rule := range e.rules {
		if cause.Category != rule.CauseCategory {
			continue
		}
		if effect.Category != rule.EffectCategory {
			continue
		}

		delay := effect.Timestamp.Sub(cause.Timestamp)
		if delay < 0 || delay > rule.MaxDelay {
			continue
		}

		// 检查模式匹配
		causeMatch := rule.CausePattern == "" ||
			strings.Contains(strings.ToLower(cause.Message), strings.ToLower(rule.CausePattern)) ||
			strings.Contains(strings.ToLower(cause.MetricName), strings.ToLower(rule.CausePattern))

		effectMatch := rule.EffectPattern == "" ||
			strings.Contains(strings.ToLower(effect.Message), strings.ToLower(rule.EffectPattern)) ||
			strings.Contains(strings.ToLower(effect.MetricName), strings.ToLower(rule.EffectPattern))

		if causeMatch && effectMatch {
			return &CausalLink{
				FromAlert:   cause,
				ToAlert:     effect,
				Relation:    rule.Description,
				Probability: rule.Weight,
				Delay:       delay,
			}
		}
	}

	return nil
}

// heuristicCausalChain 启发式因果链推导
func (e *RootCauseEngine) heuristicCausalChain(alerts []*NodeAlert) []CausalLink {
	if len(alerts) < 2 {
		return nil
	}

	var chain []CausalLink

	// 按优先级排序告警类别（基础设施 > 网络 > 资源 > 应用）
	categoryPriority := map[AlertCategory]int{
		CategoryNetwork:     1,
		CategoryDNS:         2,
		CategoryCertificate: 3,
		CategoryLoadBalancer:4,
		CategoryDisk:        5,
		CategoryCPU:         6,
		CategoryMemory:      7,
		CategoryDatabase:    8,
		CategoryMiddleware:  9,
		CategoryProcess:     10,
		CategoryApplication: 11,
	}

	sorted := make([]*NodeAlert, len(alerts))
	copy(sorted, alerts)
	sort.Slice(sorted, func(i, j int) bool {
		pi := categoryPriority[sorted[i].Category]
		pj := categoryPriority[sorted[j].Category]
		if pi != pj {
			return pi < pj
		}
		// 同类别按时间排序
		return sorted[i].Timestamp.Before(sorted[j].Timestamp)
	})

	// 构建因果链
	for i := 0; i < len(sorted)-1; i++ {
		cause := sorted[i]
		effect := sorted[i+1]

		delay := effect.Timestamp.Sub(cause.Timestamp)
		if delay < 0 {
			delay = 0
		}

		// 计算因果概率
		prob := e.heuristicCausalProbability(cause, effect)

		chain = append(chain, CausalLink{
			FromAlert:   cause,
			ToAlert:     effect,
			Relation:    fmt.Sprintf("%s 可能导致 %s", cause.Category, effect.Category),
			Probability: prob,
			Delay:       delay,
		})
	}

	return chain
}

// heuristicCausalProbability 启发式因果概率
func (e *RootCauseEngine) heuristicCausalProbability(cause, effect *NodeAlert) float64 {
	prob := 0.3 // 基础概率

	// 时间接近度加分
	delay := effect.Timestamp.Sub(cause.Timestamp)
	if delay <= 10*time.Second {
		prob += 0.3
	} else if delay <= 60*time.Second {
		prob += 0.2
	}

	// 拓扑邻近度加分
	if e.config.EnableTopologyAware && e.topology != nil {
		dist := e.correlator.topologyDistance(cause.NodeID, effect.NodeID)
		if dist <= 1 {
			prob += 0.2
		} else if dist <= 2 {
			prob += 0.1
		}
	}

	// 严重程度加分
	if cause.Severity > effect.Severity {
		prob += 0.1
	}

	// 多节点传播加分
	if cause.NodeID != effect.NodeID {
		prob += 0.1
	}

	if prob > 1.0 {
		prob = 1.0
	}

	return prob
}

// buildRootCause 构建根因
func (e *RootCauseEngine) buildRootCause(chain []CausalLink, corr *CorrelationResult) *RootCause {
	if len(chain) == 0 {
		return nil
	}

	// 根因是因果链的起点
	sourceAlert := chain[0].FromAlert

	// 计算置信度
	confidence := e.calculateConfidence(chain)

	// 收集影响范围
	impactScope := make(map[string]bool)
	impactScope[sourceAlert.NodeID] = true
	var affectedAlerts []*NodeAlert
	affectedAlerts = append(affectedAlerts, sourceAlert)

	for _, link := range chain {
		impactScope[link.ToAlert.NodeID] = true
		affectedAlerts = append(affectedAlerts, link.ToAlert)
	}

	nodes := make([]string, 0, len(impactScope))
	for node := range impactScope {
		nodes = append(nodes, node)
	}

	// 确定根因类型
	rcType := e.determineRootCauseType(sourceAlert)

	// 收集证据
	evidence := e.collectEvidence(chain, sourceAlert)

	// 生成描述
	description := e.generateDescription(sourceAlert, chain, nodes)

	return &RootCause{
		ID:             fmt.Sprintf("rc-%d", time.Now().UnixNano()),
		Timestamp:      time.Now(),
		Type:           rcType,
		Category:       sourceAlert.Category,
		SourceNode:     sourceAlert.NodeID,
		SourceAlert:    sourceAlert,
		Confidence:     confidence,
		Description:    description,
		ImpactScope:    nodes,
		AffectedAlerts: affectedAlerts,
		CausalChain:    chain,
		Evidence:       evidence,
	}
}

// calculateConfidence 计算根因置信度
func (e *RootCauseEngine) calculateConfidence(chain []CausalLink) float64 {
	if len(chain) == 0 {
		return 0
	}

	// 因果链概率的加权平均
	totalProb := 0.0
	totalWeight := 0.0

	for i, link := range chain {
		// 链条越靠前权重越高
		weight := float64(len(chain)-i) / float64(len(chain))
		totalProb += link.Probability * weight
		totalWeight += weight
	}

	if totalWeight > 0 {
		return totalProb / totalWeight
	}
	return 0
}

// determineRootCauseType 确定根因类型
func (e *RootCauseEngine) determineRootCauseType(alert *NodeAlert) RootCauseType {
	switch alert.Category {
	case CategoryNetwork, CategoryDNS, CategoryLoadBalancer, CategoryCertificate:
		return RCTypeNetwork
	case CategoryCPU, CategoryMemory, CategoryDisk:
		return RCTypeResource
	case CategoryApplication, CategoryProcess:
		return RCTypeApplication
	case CategoryDatabase, CategoryMiddleware:
		return RCTypeInfrastructure
	default:
		return RCTypeUnknown
	}
}

// collectEvidence 收集证据
func (e *RootCauseEngine) collectEvidence(chain []CausalLink, source *NodeAlert) []string {
	var evidence []string

	// 源告警信息
	evidence = append(evidence, fmt.Sprintf("根因告警: [%s] %s (节点: %s, 严重程度: %s)",
		source.Category, source.Message, source.NodeName, source.Severity))

	// 因果链证据
	for _, link := range chain {
		evidence = append(evidence, fmt.Sprintf("因果传播: %s -> %s (概率: %.0f%%, 延迟: %s)",
			link.FromAlert.NodeName, link.ToAlert.NodeName,
			link.Probability*100, link.Delay.Round(time.Second)))
	}

	// 拓扑证据
	if e.config.EnableTopologyAware && e.topology != nil {
		for _, link := range chain {
			dist := e.correlator.topologyDistance(link.FromAlert.NodeID, link.ToAlert.NodeID)
			if dist <= 2 {
				evidence = append(evidence, fmt.Sprintf("拓扑邻近: %s 与 %s 拓扑距离为 %.1f",
					link.FromAlert.NodeName, link.ToAlert.NodeName, dist))
			}
		}
	}

	return evidence
}

// generateDescription 生成根因描述
func (e *RootCauseEngine) generateDescription(source *NodeAlert, chain []CausalLink, affectedNodes []string) string {
	var sb strings.Builder

	sb.WriteString(fmt.Sprintf("根因: 节点 [%s] 发生 [%s] 类异常", source.NodeName, source.Category))
	sb.WriteString(fmt.Sprintf("（%s）", source.Message))
	sb.WriteString("\n")

	if len(chain) > 0 {
		sb.WriteString(fmt.Sprintf("该异常通过因果链传播，影响了 %d 个节点: %s",
			len(affectedNodes), strings.Join(affectedNodes, ", ")))
		sb.WriteString("\n")

		sb.WriteString("传播路径: ")
		for i, link := range chain {
			if i > 0 {
				sb.WriteString(" -> ")
			}
			sb.WriteString(fmt.Sprintf("%s(%s)", link.ToAlert.NodeName, link.ToAlert.Category))
		}
	}

	return sb.String()
}

// analyzeSingleAlert 分析单个告警
func (e *RootCauseEngine) analyzeSingleAlert(alert *NodeAlert) *RootCause {
	rcType := e.determineRootCauseType(alert)

	return &RootCause{
		ID:           fmt.Sprintf("rc-%d", time.Now().UnixNano()),
		Timestamp:    time.Now(),
		Type:         rcType,
		Category:     alert.Category,
		SourceNode:   alert.NodeID,
		SourceAlert:  alert,
		Confidence:   0.5,
		Description:  fmt.Sprintf("节点 [%s] 告警: %s", alert.NodeName, alert.Message),
		ImpactScope:  []string{alert.NodeID},
		AffectedAlerts: []*NodeAlert{alert},
		Evidence: []string{
			fmt.Sprintf("告警: [%s] %s (节点: %s)", alert.Category, alert.Message, alert.NodeName),
		},
	}
}

// deduplicateRootCauses 去重根因
func (e *RootCauseEngine) deduplicateRootCauses(rcs []*RootCause) []*RootCause {
	seen := make(map[string]bool)
	var result []*RootCause

	for _, rc := range rcs {
		key := fmt.Sprintf("%s:%s:%s", rc.SourceNode, rc.Category, rc.Type)
		if !seen[key] {
			seen[key] = true
			result = append(result, rc)
		}
	}

	return result
}

// ============================================================
// 建议生成引擎
// ============================================================

// generateSuggestions 生成修复建议
func (e *RootCauseEngine) generateSuggestions(rc *RootCause) []Suggestion {
	var suggestions []Suggestion

	switch rc.Category {
	case CategoryNetwork:
		suggestions = append(suggestions, e.networkSuggestions(rc)...)
	case CategoryDNS:
		suggestions = append(suggestions, e.dnsSuggestions(rc)...)
	case CategoryCPU:
		suggestions = append(suggestions, e.cpuSuggestions(rc)...)
	case CategoryMemory:
		suggestions = append(suggestions, e.memorySuggestions(rc)...)
	case CategoryDisk:
		suggestions = append(suggestions, e.diskSuggestions(rc)...)
	case CategoryDatabase:
		suggestions = append(suggestions, e.databaseSuggestions(rc)...)
	case CategoryLoadBalancer:
		suggestions = append(suggestions, e.loadBalancerSuggestions(rc)...)
	case CategoryCertificate:
		suggestions = append(suggestions, e.certificateSuggestions(rc)...)
	case CategoryApplication:
		suggestions = append(suggestions, e.applicationSuggestions(rc)...)
	default:
		suggestions = append(suggestions, e.genericSuggestions(rc)...)
	}

	// 按优先级排序
	sort.Slice(suggestions, func(i, j int) bool {
		return suggestions[i].Priority > suggestions[j].Priority
	})

	return suggestions
}

func (e *RootCauseEngine) networkSuggestions(rc *RootCause) []Suggestion {
	return []Suggestion{
		{ID: "net-1", Priority: 9, Action: "检查网络链路状态，确认物理连接正常",
			Target: rc.SourceNode, AutoFixable: false, Risk: "低", EstimateTime: "5分钟"},
		{ID: "net-2", Priority: 8, Action: "检查交换机端口状态和VLAN配置",
			Target: rc.SourceNode, AutoFixable: false, Risk: "低", EstimateTime: "10分钟"},
		{ID: "net-3", Priority: 7, Action: "执行网络连通性测试（ping/traceroute）",
			Target: rc.SourceNode, AutoFixable: true, Risk: "低", EstimateTime: "2分钟"},
		{ID: "net-4", Priority: 6, Action: "检查防火墙规则和安全组配置",
			Target: rc.SourceNode, AutoFixable: false, Risk: "中", EstimateTime: "15分钟"},
		{ID: "net-5", Priority: 5, Action: "检查MTU设置和网络接口配置",
			Target: rc.SourceNode, AutoFixable: true, Risk: "低", EstimateTime: "5分钟"},
	}
}

func (e *RootCauseEngine) dnsSuggestions(rc *RootCause) []Suggestion {
	return []Suggestion{
		{ID: "dns-1", Priority: 9, Action: "检查DNS服务状态和配置",
			Target: rc.SourceNode, AutoFixable: false, Risk: "低", EstimateTime: "5分钟"},
		{ID: "dns-2", Priority: 8, Action: "验证DNS解析结果（dig/nslookup）",
			Target: rc.SourceNode, AutoFixable: true, Risk: "低", EstimateTime: "2分钟"},
		{ID: "dns-3", Priority: 7, Action: "检查/etc/resolv.conf配置",
			Target: rc.SourceNode, AutoFixable: true, Risk: "中", EstimateTime: "5分钟"},
		{ID: "dns-4", Priority: 6, Action: "检查DNS缓存和TTL设置",
			Target: rc.SourceNode, AutoFixable: true, Risk: "低", EstimateTime: "5分钟"},
	}
}

func (e *RootCauseEngine) cpuSuggestions(rc *RootCause) []Suggestion {
	return []Suggestion{
		{ID: "cpu-1", Priority: 9, Action: "定位高CPU消耗进程（top/htop）",
			Target: rc.SourceNode, AutoFixable: true, Risk: "低", EstimateTime: "2分钟"},
		{ID: "cpu-2", Priority: 8, Action: "检查是否存在异常进程或死循环",
			Target: rc.SourceNode, AutoFixable: false, Risk: "低", EstimateTime: "10分钟"},
		{ID: "cpu-3", Priority: 7, Action: "分析应用线程栈（jstack/goroutine dump）",
			Target: rc.SourceNode, AutoFixable: true, Risk: "低", EstimateTime: "5分钟"},
		{ID: "cpu-4", Priority: 6, Action: "考虑水平扩容以分担负载",
			Target: rc.SourceNode, AutoFixable: false, Risk: "中", EstimateTime: "30分钟"},
		{ID: "cpu-5", Priority: 5, Action: "检查是否有定时任务或批处理在运行",
			Target: rc.SourceNode, AutoFixable: false, Risk: "低", EstimateTime: "10分钟"},
	}
}

func (e *RootCauseEngine) memorySuggestions(rc *RootCause) []Suggestion {
	return []Suggestion{
		{ID: "mem-1", Priority: 9, Action: "检查内存使用情况（free/top），定位高内存进程",
			Target: rc.SourceNode, AutoFixable: true, Risk: "低", EstimateTime: "2分钟"},
		{ID: "mem-2", Priority: 8, Action: "检查是否存在内存泄漏（对比历史趋势）",
			Target: rc.SourceNode, AutoFixable: false, Risk: "低", EstimateTime: "15分钟"},
		{ID: "mem-3", Priority: 7, Action: "检查JVM堆内存配置和GC日志",
			Target: rc.SourceNode, AutoFixable: false, Risk: "低", EstimateTime: "10分钟"},
		{ID: "mem-4", Priority: 6, Action: "考虑增加内存或优化应用内存使用",
			Target: rc.SourceNode, AutoFixable: false, Risk: "中", EstimateTime: "30分钟"},
	}
}

func (e *RootCauseEngine) diskSuggestions(rc *RootCause) []Suggestion {
	return []Suggestion{
		{ID: "disk-1", Priority: 9, Action: "检查磁盘使用率，清理不必要的文件",
			Target: rc.SourceNode, AutoFixable: true, Risk: "中", EstimateTime: "10分钟"},
		{ID: "disk-2", Priority: 8, Action: "检查日志文件大小，配置日志轮转",
			Target: rc.SourceNode, AutoFixable: true, Risk: "低", EstimateTime: "5分钟"},
		{ID: "disk-3", Priority: 7, Action: "清理临时文件和缓存",
			Target: rc.SourceNode, AutoFixable: true, Risk: "低", EstimateTime: "5分钟"},
		{ID: "disk-4", Priority: 6, Action: "考虑扩容磁盘或挂载额外存储",
			Target: rc.SourceNode, AutoFixable: false, Risk: "中", EstimateTime: "30分钟"},
	}
}

func (e *RootCauseEngine) databaseSuggestions(rc *RootCause) []Suggestion {
	return []Suggestion{
		{ID: "db-1", Priority: 9, Action: "检查数据库连接状态和连接池配置",
			Target: rc.SourceNode, AutoFixable: false, Risk: "低", EstimateTime: "5分钟"},
		{ID: "db-2", Priority: 8, Action: "检查慢查询日志，优化SQL语句",
			Target: rc.SourceNode, AutoFixable: false, Risk: "低", EstimateTime: "30分钟"},
		{ID: "db-3", Priority: 7, Action: "检查数据库资源使用（CPU/内存/磁盘IO）",
			Target: rc.SourceNode, AutoFixable: true, Risk: "低", EstimateTime: "5分钟"},
		{ID: "db-4", Priority: 6, Action: "检查数据库主从同步状态",
			Target: rc.SourceNode, AutoFixable: false, Risk: "中", EstimateTime: "10分钟"},
		{ID: "db-5", Priority: 5, Action: "考虑增加连接池大小或读写分离",
			Target: rc.SourceNode, AutoFixable: false, Risk: "中", EstimateTime: "60分钟"},
	}
}

func (e *RootCauseEngine) loadBalancerSuggestions(rc *RootCause) []Suggestion {
	return []Suggestion{
		{ID: "lb-1", Priority: 9, Action: "检查负载均衡器健康检查配置",
			Target: rc.SourceNode, AutoFixable: false, Risk: "低", EstimateTime: "5分钟"},
		{ID: "lb-2", Priority: 8, Action: "检查后端服务器健康状态",
			Target: rc.SourceNode, AutoFixable: true, Risk: "低", EstimateTime: "5分钟"},
		{ID: "lb-3", Priority: 7, Action: "检查负载均衡算法和权重配置",
			Target: rc.SourceNode, AutoFixable: false, Risk: "中", EstimateTime: "15分钟"},
	}
}

func (e *RootCauseEngine) certificateSuggestions(rc *RootCause) []Suggestion {
	return []Suggestion{
		{ID: "cert-1", Priority: 10, Action: "立即续期TLS证书",
			Target: rc.SourceNode, AutoFixable: true, Risk: "低", EstimateTime: "10分钟"},
		{ID: "cert-2", Priority: 9, Action: "配置证书自动续期（如cert-manager）",
			Target: rc.SourceNode, AutoFixable: true, Risk: "低", EstimateTime: "30分钟"},
		{ID: "cert-3", Priority: 8, Action: "检查证书链是否完整",
			Target: rc.SourceNode, AutoFixable: false, Risk: "低", EstimateTime: "5分钟"},
	}
}

func (e *RootCauseEngine) applicationSuggestions(rc *RootCause) []Suggestion {
	return []Suggestion{
		{ID: "app-1", Priority: 9, Action: "检查应用日志，定位错误原因",
			Target: rc.SourceNode, AutoFixable: true, Risk: "低", EstimateTime: "10分钟"},
		{ID: "app-2", Priority: 8, Action: "检查应用配置和版本",
			Target: rc.SourceNode, AutoFixable: false, Risk: "低", EstimateTime: "10分钟"},
		{ID: "app-3", Priority: 7, Action: "检查依赖服务状态",
			Target: rc.SourceNode, AutoFixable: true, Risk: "低", EstimateTime: "5分钟"},
		{ID: "app-4", Priority: 6, Action: "考虑重启应用服务",
			Target: rc.SourceNode, AutoFixable: true, Risk: "中", EstimateTime: "5分钟"},
		{ID: "app-5", Priority: 5, Action: "检查是否为最近部署导致的回归",
			Target: rc.SourceNode, AutoFixable: false, Risk: "中", EstimateTime: "30分钟"},
	}
}

func (e *RootCauseEngine) genericSuggestions(rc *RootCause) []Suggestion {
	return []Suggestion{
		{ID: "gen-1", Priority: 8, Action: "收集系统日志和指标，进一步分析",
			Target: rc.SourceNode, AutoFixable: true, Risk: "低", EstimateTime: "10分钟"},
		{ID: "gen-2", Priority: 7, Action: "检查最近是否有配置变更",
			Target: rc.SourceNode, AutoFixable: false, Risk: "低", EstimateTime: "15分钟"},
		{ID: "gen-3", Priority: 6, Action: "通知相关团队进行排查",
			Target: rc.SourceNode, AutoFixable: false, Risk: "低", EstimateTime: "5分钟"},
	}
}
