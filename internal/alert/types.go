//go:build linux

// Package alert 提供告警管理功能
// 本文件定义告警模块的基础类型
package alert

import (
	"context"
	"fmt"
	"sync"
	"sync/atomic"
	"time"

	"cloud-flow-agent/pkg/logger"
)

// ==================== 告警级别 ====================

// AlertLevel 告警级别
type AlertLevel int

const (
	AlertLevelInfo      AlertLevel = iota // 一般：信息性告警，无需立即处理
	AlertLevelWarning                     // 重要：需要关注，可能影响业务
	AlertLevelCritical                    // 紧急：严重问题，需要立即处理
)

// String 返回告警级别字符串
func (l AlertLevel) String() string {
	switch l {
	case AlertLevelInfo:
		return "info"
	case AlertLevelWarning:
		return "warning"
	case AlertLevelCritical:
		return "critical"
	default:
		return "unknown"
	}
}

// ==================== 告警状态 ====================

// AlertState 告警状态
type AlertState int

const (
	AlertStatePending  AlertState = iota // 待定（条件满足但未达到触发阈值）
	AlertStateFiring                     // 触发中
	AlertStateResolved                   // 已恢复
)

// String 返回状态字符串
func (s AlertState) String() string {
	switch s {
	case AlertStatePending:
		return "pending"
	case AlertStateFiring:
		return "firing"
	case AlertStateResolved:
		return "resolved"
	}
	return "unknown"
}

// ==================== 告警事件 ====================

// AlertEvent 告警事件
type AlertEvent struct {
	ID           string            `json:"id"`
	RuleID       string            `json:"rule_id"`
	RuleName     string            `json:"rule_name"`
	Level        AlertLevel        `json:"level"`
	State        AlertState        `json:"state"`
	Metric       string            `json:"metric"`
	Value        float64           `json:"value"`
	Threshold    float64           `json:"threshold"`
	Labels       map[string]string `json:"labels"`
	Annotations  map[string]string `json:"annotations"`
	FiredAt      time.Time         `json:"fired_at"`
	ResolvedAt   time.Time         `json:"resolved_at,omitempty"`
	Duration     time.Duration     `json:"duration"`
	Notified     bool              `json:"notified"`
	NotifyAt     time.Time         `json:"notify_at,omitempty"`
	NotifyError  string            `json:"notify_error,omitempty"`

	// 恢复检测相关
	LastValueAt    time.Time `json:"last_value_at"`     // 最后收到指标值的时间
	ConsecutiveOK  int       `json:"consecutive_ok"`    // 连续正常的次数
	FireCount      int       `json:"fire_count"`        // 触发次数累计
}

// GenerateFingerprint 生成告警指纹
func (e *AlertEvent) GenerateFingerprint() string {
	return fmt.Sprintf("%s:%s:%v", e.RuleID, e.Metric, e.Labels["instance"])
}

// ToMap 转换为map
func (e *AlertEvent) ToMap() map[string]interface{} {
	m := map[string]interface{}{
		"id":          e.ID,
		"rule_id":     e.RuleID,
		"rule_name":   e.RuleName,
		"level":       e.Level.String(),
		"state":       e.State.String(),
		"metric":      e.Metric,
		"value":       FormatValue(e.Value),
		"threshold":   FormatValue(e.Threshold),
		"labels":      e.Labels,
		"annotations": e.Annotations,
		"fired_at":    e.FiredAt.Format(time.RFC3339),
		"duration":    e.Duration.String(),
		"notified":    e.Notified,
	}

	if !e.ResolvedAt.IsZero() {
		m["resolved_at"] = e.ResolvedAt.Format(time.RFC3339)
	}

	if e.NotifyError != "" {
		m["notify_error"] = e.NotifyError
	}

	return m
}

// ==================== 指标数据 ====================

// MetricData 指标数据
type MetricData struct {
	Name      string            `json:"name"`
	Value     float64           `json:"value"`
	Labels    map[string]string `json:"labels"`
	Timestamp time.Time         `json:"timestamp"`
}

// ==================== 静默规则 ====================

// Silence 静默规则
type Silence struct {
	ID        string            `json:"id"`
	Matchers  map[string]string `json:"matchers"` // 匹配条件
	StartTime time.Time         `json:"start_time"`
	EndTime   time.Time         `json:"end_time"`
	Reason    string            `json:"reason"`
	CreatedBy string            `json:"created_by"`
}

// Matches 检查是否匹配静默规则
func (s *Silence) Matches(labels map[string]string) bool {
	for k, v := range s.Matchers {
		if lv, ok := labels[k]; !ok || lv != v {
			return false
		}
	}
	return true
}

// IsActive 检查静默规则是否活跃
func (s *Silence) IsActive() bool {
	now := time.Now()
	return now.After(s.StartTime) && now.Before(s.EndTime)
}

// ==================== 通知器接口 ====================

// Notifier 通知器接口
type Notifier interface {
	Notify(ctx context.Context, event *AlertEvent) error
}

// MultiNotifier 多通道通知器
type MultiNotifier struct {
	notifiers map[string]Notifier
	mu        sync.RWMutex
}

// NewMultiNotifier 创建多通道通知器
func NewMultiNotifier() *MultiNotifier {
	return &MultiNotifier{
		notifiers: make(map[string]Notifier),
	}
}

// AddNotifier 添加通知器
func (n *MultiNotifier) AddNotifier(name string, notifier Notifier) {
	n.mu.Lock()
	defer n.mu.Unlock()
	n.notifiers[name] = notifier
}

// Notify 发送通知
func (n *MultiNotifier) Notify(ctx context.Context, event *AlertEvent) error {
	n.mu.RLock()
	defer n.mu.RUnlock()

	var lastErr error
	for _, notifier := range n.notifiers {
		if err := notifier.Notify(ctx, event); err != nil {
			lastErr = err
		}
	}
	return lastErr
}

// ==================== 辅助函数 ====================

// FormatValue 格式化值
func FormatValue(v float64) string {
	if v >= 1000000 {
		return fmt.Sprintf("%.2fM", v/1000000)
	}
	if v >= 1000 {
		return fmt.Sprintf("%.2fK", v/1000)
	}
	if v >= 100 {
		return fmt.Sprintf("%.1f", v)
	}
	if v >= 1 {
		return fmt.Sprintf("%.2f", v)
	}
	if v >= 0.01 {
		return fmt.Sprintf("%.4f", v)
	}
	return fmt.Sprintf("%.6f", v)
}

func generateEventID() string {
	return fmt.Sprintf("alert-%d", time.Now().UnixNano())
}

func generateSilenceID() string {
	return fmt.Sprintf("silence-%d", time.Now().UnixNano())
}

func mergeLabels(base, extra map[string]string) map[string]string {
	result := make(map[string]string)
	for k, v := range base {
		result[k] = v
	}
	for k, v := range extra {
		result[k] = v
	}
	return result
}

// labelsMatch 检查标签是否匹配
func labelsMatch(eventLabels, metricLabels map[string]string) bool {
	keyLabels := []string{"instance", "host", "device", "service"}
	for _, key := range keyLabels {
		ev, eok := eventLabels[key]
		mv, mok := metricLabels[key]
		if eok && mok && ev != mv {
			return false
		}
	}
	return true
}

// ==================== 指标缓存 ====================

// MetricCacheEntry 指标缓存条目
type MetricCacheEntry struct {
	Value     float64
	Timestamp time.Time
	Labels    map[string]string
}
