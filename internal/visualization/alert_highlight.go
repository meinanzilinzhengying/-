// Package visualization 提供告警高亮与可视化功能
// 支持拓扑异常标识和故障快速定位
package visualization

import (
	"context"
	"encoding/json"
	"fmt"
	"math"
	"sort"
	"sync"
	"time"

	"github.com/cloud-flow-agent/internal/topology"
)

// AlertSeverity 告警级别
type AlertSeverity string

const (
	SeverityCritical AlertSeverity = "critical" // 严重
	SeverityWarning  AlertSeverity = "warning"  // 警告
	SeverityInfo     AlertSeverity = "info"     // 信息
	SeverityNormal   AlertSeverity = "normal"   // 正常
)

// AlertStatus 告警状态
type AlertStatus string

const (
	StatusFiring   AlertStatus = "firing"   // 触发中
	StatusResolved AlertStatus = "resolved" // 已恢复
	StatusPending  AlertStatus = "pending"  // 待定
)

// AlertType 告警类型
type AlertType string

const (
	AlertTypeNetwork    AlertType = "network"    // 网络告警
	AlertTypeApplication AlertType = "application" // 应用告警
	AlertTypeSystem     AlertType = "system"     // 系统告警
	AlertTypeTopology   AlertType = "topology"   // 拓扑告警
	AlertTypePerformance AlertType = "performance" // 性能告警
	AlertTypeSecurity   AlertType = "security"   // 安全告警
)

// Alert 告警
type Alert struct {
	ID          string            `json:"id"`
	Name        string            `json:"name"`
	Type        AlertType         `json:"type"`
	Severity    AlertSeverity     `json:"severity"`
	Status      AlertStatus       `json:"status"`
	Message     string            `json:"message"`
	Description string            `json:"description"`
	EntityID    string            `json:"entity_id"`    // 关联实体ID
	EntityType  string            `json:"entity_type"`  // 关联实体类型
	EntityName  string            `json:"entity_name"`  // 关联实体名称
	MetricName  string            `json:"metric_name"`  // 指标名
	MetricValue float64           `json:"metric_value"` // 指标值
	Threshold   float64           `json:"threshold"`    // 阈值
	Labels      map[string]string `json:"labels"`
	Annotations map[string]string `json:"annotations"`
	StartsAt    time.Time         `json:"starts_at"`
	EndsAt      *time.Time        `json:"ends_at,omitempty"`
	UpdatedAt   time.Time         `json:"updated_at"`
}

// HighlightStyle 高亮样式
type HighlightStyle struct {
	Color       string  `json:"color"`        // 颜色
	BorderColor string  `json:"border_color"` // 边框颜色
	BorderWidth int     `json:"border_width"` // 边框宽度
	Opacity     float64 `json:"opacity"`      // 透明度
	Size        float64 `json:"size"`         // 大小
	Pulse       bool    `json:"pulse"`        // 脉冲动画
	Blink       bool    `json:"blink"`        // 闪烁动画
	Icon        string  `json:"icon"`         // 图标
	Badge       string  `json:"badge"`        // 徽章
}

// HighlightConfig 高亮配置
type HighlightConfig struct {
	Enabled           bool            `json:"enabled" yaml:"enabled"`
	AutoHighlight     bool            `json:"auto_highlight" yaml:"auto_highlight"`     // 自动高亮
	HighlightDuration time.Duration   `json:"highlight_duration" yaml:"highlight_duration"` // 高亮持续时间
	FadeOutDuration   time.Duration   `json:"fade_out_duration" yaml:"fade_out_duration"`   // 淡出时间
	MaxHighlights     int             `json:"max_highlights" yaml:"max_highlights"`       // 最大高亮数
	SeverityStyles    map[AlertSeverity]*HighlightStyle `json:"severity_styles" yaml:"severity_styles"`
	TypeStyles        map[AlertType]*HighlightStyle     `json:"type_styles" yaml:"type_styles"`
}

// TopologyHighlight 拓扑高亮
type TopologyHighlight struct {
	EntityID    string            `json:"entity_id"`
	EntityType  string            `json:"entity_type"`
	EntityName  string            `json:"entity_name"`
	Alerts      []*Alert          `json:"alerts"`
	Style       *HighlightStyle   `json:"style"`
	Severity    AlertSeverity     `json:"severity"`
	Status      AlertStatus       `json:"status"`
	StartTime   time.Time         `json:"start_time"`
	EndTime     *time.Time        `json:"end_time,omitempty"`
	ImpactScore float64           `json:"impact_score"` // 影响分数
	RelatedIDs  []string          `json:"related_ids"`  // 相关实体ID
}

// ImpactPath 影响路径
type ImpactPath struct {
	PathID      string    `json:"path_id"`
	SourceID    string    `json:"source_id"`
	TargetID    string    `json:"target_id"`
	Hops        []string  `json:"hops"`         // 路径上的实体ID
	ImpactScore float64   `json:"impact_score"` // 影响分数
	Latency     float64   `json:"latency_ms"`
}

// AlertHighlightEngine 告警高亮引擎
type AlertHighlightEngine struct {
	config      *HighlightConfig
	topology    *topology.DiscoveryEngine
	alerts      map[string]*Alert
	highlights  map[string]*TopologyHighlight
	impactPaths map[string]*ImpactPath
	mu          sync.RWMutex
	ctx         context.Context
	cancel      context.CancelFunc
	wg          sync.WaitGroup
	handlers    []HighlightHandler
}

// HighlightHandler 高亮事件处理器
type HighlightHandler interface {
	OnHighlightAdded(highlight *TopologyHighlight)
	OnHighlightUpdated(highlight *TopologyHighlight)
	OnHighlightRemoved(entityID string)
	OnImpactPathDiscovered(path *ImpactPath)
}

// NewAlertHighlightEngine 创建告警高亮引擎
func NewAlertHighlightEngine(config *HighlightConfig, topologyEngine *topology.DiscoveryEngine) *AlertHighlightEngine {
	ctx, cancel := context.WithCancel(context.Background())

	// 初始化默认样式
	if config.SeverityStyles == nil {
		config.SeverityStyles = defaultSeverityStyles()
	}
	if config.TypeStyles == nil {
		config.TypeStyles = defaultTypeStyles()
	}

	return &AlertHighlightEngine{
		config:      config,
		topology:    topologyEngine,
		alerts:      make(map[string]*Alert),
		highlights:  make(map[string]*TopologyHighlight),
		impactPaths: make(map[string]*ImpactPath),
		ctx:         ctx,
		cancel:      cancel,
	}
}

// defaultSeverityStyles 默认严重级别样式
func defaultSeverityStyles() map[AlertSeverity]*HighlightStyle {
	return map[AlertSeverity]*HighlightStyle{
		SeverityCritical: {
			Color:       "#FF0000",
			BorderColor: "#8B0000",
			BorderWidth: 3,
			Opacity:     1.0,
			Size:        1.5,
			Pulse:       true,
			Blink:       true,
			Icon:        "🔴",
			Badge:       "CRITICAL",
		},
		SeverityWarning: {
			Color:       "#FFA500",
			BorderColor: "#FF8C00",
			BorderWidth: 2,
			Opacity:     0.9,
			Size:        1.3,
			Pulse:       true,
			Blink:       false,
			Icon:        "🟡",
			Badge:       "WARNING",
		},
		SeverityInfo: {
			Color:       "#3498DB",
			BorderColor: "#2980B9",
			BorderWidth: 1,
			Opacity:     0.8,
			Size:        1.1,
			Pulse:       false,
			Blink:       false,
			Icon:        "🔵",
			Badge:       "INFO",
		},
		SeverityNormal: {
			Color:       "#2ECC71",
			BorderColor: "#27AE60",
			BorderWidth: 1,
			Opacity:     0.7,
			Size:        1.0,
			Pulse:       false,
			Blink:       false,
			Icon:        "🟢",
			Badge:       "",
		},
	}
}

// defaultTypeStyles 默认告警类型样式
func defaultTypeStyles() map[AlertType]*HighlightStyle {
	return map[AlertType]*HighlightStyle{
		AlertTypeNetwork: {
			Color: "#9B59B6",
			Icon:  "🌐",
		},
		AlertTypeApplication: {
			Color: "#E74C3C",
			Icon:  "⚙️",
		},
		AlertTypeSystem: {
			Color: "#34495E",
			Icon:  "🖥️",
		},
		AlertTypeTopology: {
			Color: "#16A085",
			Icon:  "🔗",
		},
		AlertTypePerformance: {
			Color: "#F39C12",
			Icon:  "📊",
		},
		AlertTypeSecurity: {
			Color: "#C0392B",
			Icon:  "🔒",
		},
	}
}

// Start 启动高亮引擎
func (e *AlertHighlightEngine) Start() error {
	// 启动清理协程
	e.wg.Add(1)
	go e.cleanupLoop()

	// 启动影响分析协程
	e.wg.Add(1)
	go e.impactAnalysisLoop()

	return nil
}

// Stop 停止高亮引擎
func (e *AlertHighlightEngine) Stop() {
	e.cancel()
	e.wg.Wait()
}

// RegisterHandler 注册高亮事件处理器
func (e *AlertHighlightEngine) RegisterHandler(handler HighlightHandler) {
	e.handlers = append(e.handlers, handler)
}

// AddAlert 添加告警
func (e *AlertHighlightEngine) AddAlert(alert *Alert) error {
	e.mu.Lock()
	defer e.mu.Unlock()

	// 存储告警
	e.alerts[alert.ID] = alert

	// 更新或创建高亮
	e.updateHighlightForAlert(alert)

	// 分析影响范围
	e.analyzeImpact(alert)

	return nil
}

// UpdateAlert 更新告警
func (e *AlertHighlightEngine) UpdateAlert(alert *Alert) error {
	return e.AddAlert(alert)
}

// ResolveAlert 解决告警
func (e *AlertHighlightEngine) ResolveAlert(alertID string) error {
	e.mu.Lock()
	defer e.mu.Unlock()

	alert, exists := e.alerts[alertID]
	if !exists {
		return fmt.Errorf("alert not found: %s", alertID)
	}

	now := time.Now()
	alert.Status = StatusResolved
	alert.EndsAt = &now
	alert.UpdatedAt = now

	// 更新高亮
	e.updateHighlightForAlert(alert)

	return nil
}

// RemoveAlert 移除告警
func (e *AlertHighlightEngine) RemoveAlert(alertID string) error {
	e.mu.Lock()
	defer e.mu.Unlock()

	alert, exists := e.alerts[alertID]
	if !exists {
		return nil
	}

	delete(e.alerts, alertID)

	// 重新计算该实体的高亮
	e.recalculateHighlight(alert.EntityID)

	return nil
}

// updateHighlightForAlert 根据告警更新高亮
func (e *AlertHighlightEngine) updateHighlightForAlert(alert *Alert) {
	entityID := alert.EntityID

	highlight, exists := e.highlights[entityID]
	if !exists {
		highlight = &TopologyHighlight{
			EntityID:   entityID,
			EntityType: alert.EntityType,
			EntityName: alert.EntityName,
			Alerts:     make([]*Alert, 0),
			StartTime:  time.Now(),
		}
		e.highlights[entityID] = highlight
	}

	// 添加或更新告警
	found := false
	for i, a := range highlight.Alerts {
		if a.ID == alert.ID {
			highlight.Alerts[i] = alert
			found = true
			break
		}
	}
	if !found {
		highlight.Alerts = append(highlight.Alerts, alert)
	}

	// 计算最高严重级别
	highlight.Severity = e.calculateMaxSeverity(highlight.Alerts)
	highlight.Status = e.calculateOverallStatus(highlight.Alerts)

	// 应用样式
	highlight.Style = e.applyStyle(highlight.Severity, alert.Type)

	// 计算影响分数
	highlight.ImpactScore = e.calculateImpactScore(highlight)

	// 通知处理器
	if exists {
		for _, handler := range e.handlers {
			handler.OnHighlightUpdated(highlight)
		}
	} else {
		for _, handler := range e.handlers {
			handler.OnHighlightAdded(highlight)
		}
	}
}

// recalculateHighlight 重新计算高亮
func (e *AlertHighlightEngine) recalculateHighlight(entityID string) {
	highlight, exists := e.highlights[entityID]
	if !exists {
		return
	}

	// 过滤出未解决的告警
	var activeAlerts []*Alert
	for _, alert := range highlight.Alerts {
		if alert.Status != StatusResolved {
			activeAlerts = append(activeAlerts, alert)
		}
	}

	if len(activeAlerts) == 0 {
		// 没有活跃告警，移除高亮
		delete(e.highlights, entityID)
		for _, handler := range e.handlers {
			handler.OnHighlightRemoved(entityID)
		}
		return
	}

	// 更新告警列表
	highlight.Alerts = activeAlerts
	highlight.Severity = e.calculateMaxSeverity(activeAlerts)
	highlight.Status = e.calculateOverallStatus(activeAlerts)
	highlight.Style = e.applyStyle(highlight.Severity, activeAlerts[0].Type)
	highlight.ImpactScore = e.calculateImpactScore(highlight)

	for _, handler := range e.handlers {
		handler.OnHighlightUpdated(highlight)
	}
}

// calculateMaxSeverity 计算最高严重级别
func (e *AlertHighlightEngine) calculateMaxSeverity(alerts []*Alert) AlertSeverity {
	severityOrder := map[AlertSeverity]int{
		SeverityNormal:   0,
		SeverityInfo:     1,
		SeverityWarning:  2,
		SeverityCritical: 3,
	}

	maxSeverity := SeverityNormal
	maxOrder := 0

	for _, alert := range alerts {
		if alert.Status == StatusResolved {
			continue
		}
		order := severityOrder[alert.Severity]
		if order > maxOrder {
			maxOrder = order
			maxSeverity = alert.Severity
		}
	}

	return maxSeverity
}

// calculateOverallStatus 计算整体状态
func (e *AlertHighlightEngine) calculateOverallStatus(alerts []*Alert) AlertStatus {
	hasFiring := false
	hasPending := false

	for _, alert := range alerts {
		switch alert.Status {
		case StatusFiring:
			hasFiring = true
		case StatusPending:
			hasPending = true
		}
	}

	if hasFiring {
		return StatusFiring
	}
	if hasPending {
		return StatusPending
	}
	return StatusResolved
}

// applyStyle 应用样式
func (e *AlertHighlightEngine) applyStyle(severity AlertSeverity, alertType AlertType) *HighlightStyle {
	// 获取严重级别样式
	severityStyle := e.config.SeverityStyles[severity]
	if severityStyle == nil {
		severityStyle = e.config.SeverityStyles[SeverityNormal]
	}

	// 获取类型样式
	typeStyle := e.config.TypeStyles[alertType]
	if typeStyle == nil {
		return severityStyle
	}

	// 合并样式
	merged := &HighlightStyle{
		Color:       severityStyle.Color,
		BorderColor: severityStyle.BorderColor,
		BorderWidth: severityStyle.BorderWidth,
		Opacity:     severityStyle.Opacity,
		Size:        severityStyle.Size,
		Pulse:       severityStyle.Pulse,
		Blink:       severityStyle.Blink,
		Icon:        typeStyle.Icon,
		Badge:       severityStyle.Badge,
	}

	return merged
}

// calculateImpactScore 计算影响分数
func (e *AlertHighlightEngine) calculateImpactScore(highlight *TopologyHighlight) float64 {
	score := 0.0

	// 基于严重级别
	severityWeights := map[AlertSeverity]float64{
		SeverityCritical: 100,
		SeverityWarning:  50,
		SeverityInfo:     10,
		SeverityNormal:   0,
	}

	for _, alert := range highlight.Alerts {
		if alert.Status != StatusResolved {
			score += severityWeights[alert.Severity]
		}
	}

	// 基于告警数量
	score += float64(len(highlight.Alerts)) * 5

	// 基于持续时间
	if !highlight.StartTime.IsZero() {
		duration := time.Since(highlight.StartTime).Minutes()
		score += math.Min(duration, 100) // 最多加100分
	}

	return math.Min(score, 1000) // 最高1000分
}

// analyzeImpact 分析影响范围
func (e *AlertHighlightEngine) analyzeImpact(alert *Alert) {
	if e.topology == nil {
		return
	}

	// 获取实体
	entity := e.topology.GetEntity(alert.EntityID)
	if entity == nil {
		return
	}

	// 查找相关实体
	relatedEntities := e.findRelatedEntities(entity)

	// 更新高亮的相关ID
	highlight := e.highlights[alert.EntityID]
	if highlight != nil {
		highlight.RelatedIDs = relatedEntities
	}

	// 发现影响路径
	e.discoverImpactPaths(entity, relatedEntities)
}

// findRelatedEntities 查找相关实体
func (e *AlertHighlightEngine) findRelatedEntities(entity *topology.Entity) []string {
	var related []string

	// 获取拓扑关系
	relations := e.topology.GetRelations()
	for _, rel := range relations {
		if rel.SourceID == entity.ID {
			related = append(related, rel.TargetID)
		} else if rel.TargetID == entity.ID {
			related = append(related, rel.SourceID)
		}
	}

	return related
}

// discoverImpactPaths 发现影响路径
func (e *AlertHighlightEngine) discoverImpactPaths(source *topology.Entity, targets []string) {
	for _, targetID := range targets {
		pathID := fmt.Sprintf("%s->%s", source.ID, targetID)

		path := &ImpactPath{
			PathID:   pathID,
			SourceID: source.ID,
			TargetID: targetID,
			Hops:     []string{source.ID, targetID},
		}

		// 计算影响分数
		path.ImpactScore = e.calculatePathImpactScore(path)

		e.impactPaths[pathID] = path

		for _, handler := range e.handlers {
			handler.OnImpactPathDiscovered(path)
		}
	}
}

// calculatePathImpactScore 计算路径影响分数
func (e *AlertHighlightEngine) calculatePathImpactScore(path *ImpactPath) float64 {
	// 基于源节点的高亮分数
	sourceHighlight := e.highlights[path.SourceID]
	if sourceHighlight == nil {
		return 0
	}

	return sourceHighlight.ImpactScore * 0.8 // 传播衰减
}

// GetHighlight 获取高亮
func (e *AlertHighlightEngine) GetHighlight(entityID string) *TopologyHighlight {
	e.mu.RLock()
	defer e.mu.RUnlock()

	return e.highlights[entityID]
}

// GetAllHighlights 获取所有高亮
func (e *AlertHighlightEngine) GetAllHighlights() []*TopologyHighlight {
	e.mu.RLock()
	defer e.mu.RUnlock()

	highlights := make([]*TopologyHighlight, 0, len(e.highlights))
	for _, h := range e.highlights {
		highlights = append(highlights, h)
	}

	return highlights
}

// GetHighlightsBySeverity 按严重级别获取高亮
func (e *AlertHighlightEngine) GetHighlightsBySeverity(severity AlertSeverity) []*TopologyHighlight {
	e.mu.RLock()
	defer e.mu.RUnlock()

	var highlights []*TopologyHighlight
	for _, h := range e.highlights {
		if h.Severity == severity {
			highlights = append(highlights, h)
		}
	}

	return highlights
}

// GetHighlightsByType 按类型获取高亮
func (e *AlertHighlightEngine) GetHighlightsByType(alertType AlertType) []*TopologyHighlight {
	e.mu.RLock()
	defer e.mu.RUnlock()

	var highlights []*TopologyHighlight
	for _, h := range e.highlights {
		for _, alert := range h.Alerts {
			if alert.Type == alertType {
				highlights = append(highlights, h)
				break
			}
		}
	}

	return highlights
}

// GetImpactPaths 获取影响路径
func (e *AlertHighlightEngine) GetImpactPaths(entityID string) []*ImpactPath {
	e.mu.RLock()
	defer e.mu.RUnlock()

	var paths []*ImpactPath
	for _, path := range e.impactPaths {
		if path.SourceID == entityID || path.TargetID == entityID {
			paths = append(paths, path)
		}
	}

	return paths
}

// GetTopologyWithHighlights 获取带高亮的拓扑
func (e *AlertHighlightEngine) GetTopologyWithHighlights() *TopologyWithHighlights {
	e.mu.RLock()
	defer e.mu.RUnlock()

	result := &TopologyWithHighlights{
		Highlights:  make([]*TopologyHighlight, 0, len(e.highlights)),
		ImpactPaths: make([]*ImpactPath, 0, len(e.impactPaths)),
		Summary:     e.generateSummary(),
	}

	for _, h := range e.highlights {
		result.Highlights = append(result.Highlights, h)
	}

	for _, p := range e.impactPaths {
		result.ImpactPaths = append(result.ImpactPaths, p)
	}

	return result
}

// generateSummary 生成摘要
func (e *AlertHighlightEngine) generateSummary() *HighlightSummary {
	summary := &HighlightSummary{
		TotalAlerts:     len(e.alerts),
		ActiveAlerts:    0,
		CriticalCount:   0,
		WarningCount:    0,
		InfoCount:       0,
		AffectedEntities: len(e.highlights),
	}

	for _, alert := range e.alerts {
		if alert.Status != StatusResolved {
			summary.ActiveAlerts++
		}

		switch alert.Severity {
		case SeverityCritical:
			summary.CriticalCount++
		case SeverityWarning:
			summary.WarningCount++
		case SeverityInfo:
			summary.InfoCount++
		}
	}

	return summary
}

// cleanupLoop 清理循环
func (e *AlertHighlightEngine) cleanupLoop() {
	defer e.wg.Done()

	ticker := time.NewTicker(time.Minute)
	defer ticker.Stop()

	for {
		select {
		case <-e.ctx.Done():
			return
		case <-ticker.C:
			e.cleanup()
		}
	}
}

// cleanup 清理过期数据
func (e *AlertHighlightEngine) cleanup() {
	e.mu.Lock()
	defer e.mu.Unlock()

	cutoff := time.Now().Add(-e.config.HighlightDuration)

	// 清理已解决的高亮
	for entityID, highlight := range e.highlights {
		if highlight.Status == StatusResolved && highlight.EndTime != nil {
			if highlight.EndTime.Before(cutoff) {
				delete(e.highlights, entityID)
				for _, handler := range e.handlers {
					handler.OnHighlightRemoved(entityID)
				}
			}
		}
	}

	// 限制高亮数量
	if len(e.highlights) > e.config.MaxHighlights {
		// 按影响分数排序，保留高分
		type item struct {
			id    string
			score float64
		}

		var items []item
		for id, h := range e.highlights {
			items = append(items, item{id: id, score: h.ImpactScore})
		}

		sort.Slice(items, func(i, j int) bool {
			return items[i].score > items[j].score
		})

		// 删除低分高亮
		for i := e.config.MaxHighlights; i < len(items); i++ {
			delete(e.highlights, items[i].id)
		}
	}
}

// impactAnalysisLoop 影响分析循环
func (e *AlertHighlightEngine) impactAnalysisLoop() {
	defer e.wg.Done()

	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-e.ctx.Done():
			return
		case <-ticker.C:
			e.analyzeAllImpacts()
		}
	}
}

// analyzeAllImpacts 分析所有影响
func (e *AlertHighlightEngine) analyzeAllImpacts() {
	e.mu.Lock()
	defer e.mu.Unlock()

	for _, alert := range e.alerts {
		if alert.Status != StatusResolved {
			e.analyzeImpact(alert)
		}
	}
}

// TopologyWithHighlights 带高亮的拓扑
type TopologyWithHighlights struct {
	Highlights  []*TopologyHighlight `json:"highlights"`
	ImpactPaths []*ImpactPath        `json:"impact_paths"`
	Summary     *HighlightSummary    `json:"summary"`
}

// HighlightSummary 高亮摘要
type HighlightSummary struct {
	TotalAlerts      int `json:"total_alerts"`
	ActiveAlerts     int `json:"active_alerts"`
	CriticalCount    int `json:"critical_count"`
	WarningCount     int `json:"warning_count"`
	InfoCount        int `json:"info_count"`
	AffectedEntities int `json:"affected_entities"`
}

// ToJSON 转换为JSON
func (e *AlertHighlightEngine) ToJSON() ([]byte, error) {
	topology := e.GetTopologyWithHighlights()
	return json.Marshal(topology)
}
