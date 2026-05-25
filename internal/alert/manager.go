//go:build linux

// Package alert 提供告警管理功能
package alert

import (
	"context"
	"fmt"
	"sync"
	"sync/atomic"
	"time"

	"cloud-flow-agent/pkg/logger"
)

// AlertManagerConfig 告警管理器配置
type AlertManagerConfig struct {
	Enabled            bool          `yaml:"enabled" json:"enabled"`
	EvaluationInterval time.Duration `yaml:"evaluation_interval" json:"evaluation_interval"`
	ResolveTimeout     time.Duration `yaml:"resolve_timeout" json:"resolve_timeout"`
	ResolveThreshold   int           `yaml:"resolve_threshold" json:"resolve_threshold"`
	MaxActiveAlerts    int           `yaml:"max_active_alerts" json:"max_active_alerts"`
	MaxHistoryAlerts   int           `yaml:"max_history_alerts" json:"max_history_alerts"`
	EnableAutoResolve  bool          `yaml:"enable_auto_resolve" json:"enable_auto_resolve"`
	MetricCacheTTL     time.Duration `yaml:"metric_cache_ttl" json:"metric_cache_ttl"`

	// 根因分析配置
	EnableRCA bool `yaml:"enable_rca" json:"enable_rca"`
}

// DefaultAlertManagerConfig 默认配置
func DefaultAlertManagerConfig() AlertManagerConfig {
	return AlertManagerConfig{
		Enabled:            true,
		EvaluationInterval: time.Minute,
		ResolveTimeout:     10 * time.Minute,
		ResolveThreshold:   3,
		MaxActiveAlerts:    1000,
		MaxHistoryAlerts:   10000,
		EnableAutoResolve:  true,
		MetricCacheTTL:     5 * time.Minute,
		EnableRCA:          true,
	}
}

// AlertManager 告警管理器
type AlertManager struct {
	mu       sync.RWMutex
	notifier Notifier
	log      *logger.Logger

	// 告警状态
	activeAlerts map[string]*AlertEvent
	alertHistory []*AlertEvent
	silences     map[string]*Silence

	// 指标缓存
	metricCache    map[string]*MetricCacheEntry
	metricCacheTTL time.Duration

	// 根因分析引擎
	rcaEngine *RCAEngine
	enableRCA bool

	// 配置
	config AlertManagerConfig

	// 统计
	stats struct {
		totalAlerts    atomic.Uint64
		activeAlerts   atomic.Uint64
		resolvedAlerts atomic.Uint64
		silencedAlerts atomic.Uint64
		notifySuccess  atomic.Uint64
		notifyFailed   atomic.Uint64
		autoResolved   atomic.Uint64
	}

	// 控制
	stopCh chan struct{}
	wg     sync.WaitGroup
}

// NewAlertManager 创建告警管理器
func NewAlertManager(config AlertManagerConfig, notifier Notifier, log *logger.Logger) *AlertManager {
	if !config.Enabled {
		return &AlertManager{config: config, log: log}
	}

	if config.MetricCacheTTL == 0 {
		config.MetricCacheTTL = 5 * time.Minute
	}

	m := &AlertManager{
		notifier:       notifier,
		log:            log,
		config:         config,
		activeAlerts:   make(map[string]*AlertEvent),
		alertHistory:   make([]*AlertEvent, 0),
		silences:       make(map[string]*Silence),
		metricCache:    make(map[string]*MetricCacheEntry),
		metricCacheTTL: config.MetricCacheTTL,
		stopCh:         make(chan struct{}),
	}

	// 初始化根因分析引擎
	if config.EnableRCA {
		m.enableRCA = true
		m.rcaEngine = NewRCAEngine(log)
	}

	return m
}

// Start 启动告警管理器
func (m *AlertManager) Start() error {
	if !m.config.Enabled {
		return nil
	}

	// 启动自动消警检查
	m.wg.Add(1)
	go m.autoResolveLoop()

	// 启动指标缓存清理
	m.wg.Add(1)
	go m.metricCacheCleanup()

	// 启动根因分析引擎
	if m.enableRCA && m.rcaEngine != nil {
		m.rcaEngine.Start()
	}

	m.log.Infof("告警管理器已启动: 评估间隔=%v, 自动消警=%v, 根因分析=%v",
		m.config.EvaluationInterval, m.config.EnableAutoResolve, m.enableRCA)

	return nil
}

// Stop 停止告警管理器
func (m *AlertManager) Stop() {
	if m.rcaEngine != nil {
		m.rcaEngine.Stop()
	}
	close(m.stopCh)
	m.wg.Wait()
	m.log.Info("告警管理器已停止")
}

// ProcessMetrics 处理指标数据
func (m *AlertManager) ProcessMetrics(metrics []MetricData) error {
	if !m.config.Enabled {
		return nil
	}

	for _, metric := range metrics {
		m.processMetric(metric)
	}

	return nil
}

// processMetric 处理单个指标
func (m *AlertManager) processMetric(metric MetricData) {
	// 更新指标缓存
	m.updateMetricCache(metric)

	// 将指标数据传递给RCA引擎用于拓扑分析
	if m.enableRCA && m.rcaEngine != nil {
		m.rcaEngine.RecordMetric(metric)
	}
}

// ProcessAlert 处理告警事件（由外部规则引擎调用）
func (m *AlertManager) ProcessAlert(event *AlertEvent) {
	if !m.config.Enabled {
		return
	}

	fingerprint := event.GenerateFingerprint()

	m.mu.Lock()
	defer m.mu.Unlock()

	// 检查是否已有活跃告警
	if existing, ok := m.activeAlerts[fingerprint]; ok {
		// 更新现有告警
		existing.Value = event.Value
		existing.LastValueAt = time.Now()
		existing.Duration = time.Since(existing.FiredAt)
		existing.FireCount++
		return
	}

	// 新告警
	event.State = AlertStateFiring
	event.FiredAt = time.Now()
	event.FireCount = 1

	// 检查静默
	if m.isSilencedLocked(event) {
		m.stats.silencedAlerts.Add(1)
		return
	}

	// 发送通知
	if err := m.sendNotificationLocked(event); err != nil {
		m.log.Warnf("发送告警通知失败: %v", err)
	}

	// 记录告警
	m.activeAlerts[fingerprint] = event
	m.stats.totalAlerts.Add(1)
	m.stats.activeAlerts.Add(1)

	// 触发根因分析（多节点告警时）
	if m.enableRCA && m.rcaEngine != nil {
		m.rcaEngine.AnalyzeAlert(event)
	}

	m.log.Infof("告警触发: [%s] %s - %s=%s",
		event.Level.String(), event.RuleName, event.Metric,
		FormatValue(event.Value))
}

// updateMetricCache 更新指标缓存
func (m *AlertManager) updateMetricCache(metric MetricData) {
	m.mu.Lock()
	defer m.mu.Unlock()

	key := fmt.Sprintf("%s:%s", metric.Name, metric.Labels["instance"])
	m.metricCache[key] = &MetricCacheEntry{
		Value:     metric.Value,
		Timestamp: time.Now(),
		Labels:    metric.Labels,
	}
}

// metricCacheCleanup 指标缓存清理循环
func (m *AlertManager) metricCacheCleanup() {
	defer m.wg.Done()

	ticker := time.NewTicker(time.Minute)
	defer ticker.Stop()

	for {
		select {
		case <-m.stopCh:
			return
		case <-ticker.C:
			m.cleanupMetricCache()
		}
	}
}

// cleanupMetricCache 清理过期缓存
func (m *AlertManager) cleanupMetricCache() {
	m.mu.Lock()
	defer m.mu.Unlock()

	now := time.Now()
	for key, entry := range m.metricCache {
		if now.Sub(entry.Timestamp) > m.metricCacheTTL {
			delete(m.metricCache, key)
		}
	}
}

// autoResolveLoop 自动消警循环
func (m *AlertManager) autoResolveLoop() {
	defer m.wg.Done()

	ticker := time.NewTicker(m.config.EvaluationInterval)
	defer ticker.Stop()

	for {
		select {
		case <-m.stopCh:
			return
		case <-ticker.C:
			m.checkAutoResolve()
		}
	}
}

// checkAutoResolve 检查自动消警
func (m *AlertManager) checkAutoResolve() {
	if !m.config.EnableAutoResolve {
		return
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	now := time.Now()

	for fingerprint, event := range m.activeAlerts {
		// 检查是否超过强制恢复超时
		if now.Sub(event.LastValueAt) > m.config.ResolveTimeout {
			m.resolveAlertInternal(event)
			m.stats.autoResolved.Add(1)
			m.log.Infof("告警超时恢复: [%s] %s - 超过%v无数据",
				event.Level.String(), event.RuleName, m.config.ResolveTimeout)
			continue
		}

		// 检查连续正常次数是否达到恢复阈值
		if event.ConsecutiveOK >= m.config.ResolveThreshold {
			m.resolveAlertInternal(event)
			m.stats.autoResolved.Add(1)
			m.log.Infof("告警自动恢复: [%s] %s - 连续%d次指标正常",
				event.Level.String(), event.RuleName, event.ConsecutiveOK)
			continue
		}
	}
}

// ProcessEvaluationResult 处理规则评估结果（由规则引擎调用）
// 用于更新告警的连续正常/异常计数，实现智能恢复检测
func (m *AlertManager) ProcessEvaluationResult(ruleID string, metric string, 
	labels map[string]string, isViolating bool, currentValue float64) {
	
	if !m.config.Enabled {
		return
	}

	fingerprint := generateFingerprint(ruleID, metric, labels)

	m.mu.Lock()
	defer m.mu.Unlock()

	event, exists := m.activeAlerts[fingerprint]
	if !exists {
		// 没有活跃告警，无需处理
		return
	}

	// 更新最后值时间
	event.LastValueAt = time.Now()
	event.Value = currentValue

	if isViolating {
		// 指标仍然异常，重置连续正常计数
		event.ConsecutiveOK = 0
	} else {
		// 指标恢复正常，增加连续正常计数
		event.ConsecutiveOK++
	}
}

// generateFingerprint 生成告警指纹
func generateFingerprint(ruleID, metric string, labels map[string]string) string {
	instance := labels["instance"]
	if instance == "" {
		instance = labels["host"]
	}
	return fmt.Sprintf("%s:%s:%s", ruleID, metric, instance)
}

// resolveAlertInternal 内部恢复告警
func (m *AlertManager) resolveAlertInternal(event *AlertEvent) {
	event.State = AlertStateResolved
	event.ResolvedAt = time.Now()
	event.Duration = event.ResolvedAt.Sub(event.FiredAt)

	// 发送恢复通知
	if err := m.sendNotificationLocked(event); err != nil {
		m.log.Warnf("发送恢复通知失败: %v", err)
	}

	// 移动到历史
	m.addToHistoryLocked(event)
	delete(m.activeAlerts, event.GenerateFingerprint())

	m.stats.activeAlerts.Add(^uint64(0))
	m.stats.resolvedAlerts.Add(1)

	m.log.Infof("告警恢复: [%s] %s - 持续时间=%v",
		event.Level.String(), event.RuleName, event.Duration)
}

// sendNotificationLocked 发送通知（已持有锁）
func (m *AlertManager) sendNotificationLocked(event *AlertEvent) error {
	if m.notifier == nil {
		return nil
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if err := m.notifier.Notify(ctx, event); err != nil {
		m.stats.notifyFailed.Add(1)
		event.NotifyError = err.Error()
		return err
	}

	m.stats.notifySuccess.Add(1)
	event.Notified = true
	event.NotifyAt = time.Now()

	return nil
}

// isSilencedLocked 检查是否被静默（已持有锁）
func (m *AlertManager) isSilencedLocked(event *AlertEvent) bool {
	for _, silence := range m.silences {
		if silence.IsActive() && silence.Matches(event.Labels) {
			return true
		}
	}
	return false
}

// addToHistoryLocked 添加到历史（已持有锁）
func (m *AlertManager) addToHistoryLocked(event *AlertEvent) {
	m.alertHistory = append(m.alertHistory, event)

	// 限制历史大小
	if len(m.alertHistory) > m.config.MaxHistoryAlerts {
		m.alertHistory = m.alertHistory[len(m.alertHistory)-m.config.MaxHistoryAlerts:]
	}
}

// GetActiveAlerts 获取活跃告警
func (m *AlertManager) GetActiveAlerts() []*AlertEvent {
	m.mu.RLock()
	defer m.mu.RUnlock()

	alerts := make([]*AlertEvent, 0, len(m.activeAlerts))
	for _, event := range m.activeAlerts {
		alerts = append(alerts, event)
	}
	return alerts
}

// GetAlertHistory 获取历史告警
func (m *AlertManager) GetAlertHistory(limit int) []*AlertEvent {
	m.mu.RLock()
	defer m.mu.RUnlock()

	if limit <= 0 || limit > len(m.alertHistory) {
		limit = len(m.alertHistory)
	}

	start := len(m.alertHistory) - limit
	if start < 0 {
		start = 0
	}

	result := make([]*AlertEvent, limit)
	copy(result, m.alertHistory[start:])
	return result
}

// GetStats 获取统计信息
func (m *AlertManager) GetStats() map[string]interface{} {
	m.mu.RLock()
	activeCount := len(m.activeAlerts)
	silenceCount := len(m.silences)
	m.mu.RUnlock()

	return map[string]interface{}{
		"enabled":          m.config.Enabled,
		"total_alerts":     m.stats.totalAlerts.Load(),
		"active_alerts":    activeCount,
		"resolved_alerts":  m.stats.resolvedAlerts.Load(),
		"auto_resolved":    m.stats.autoResolved.Load(),
		"silenced_alerts":  m.stats.silencedAlerts.Load(),
		"notify_success":   m.stats.notifySuccess.Load(),
		"notify_failed":    m.stats.notifyFailed.Load(),
		"silence_count":    silenceCount,
		"rca_enabled":      m.enableRCA,
	}
}

// GetRCAEngine 获取根因分析引擎
func (m *AlertManager) GetRCAEngine() *RCAEngine {
	return m.rcaEngine
}
