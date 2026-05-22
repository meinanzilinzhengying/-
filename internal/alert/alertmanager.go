/*
 * Copyright (c) 2025 Yunlong Liao. All rights reserved.
 */

package alert

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"sync"
	"time"

	"cloud-flow-agent/pkg/logger"
)

// AlertSeverity 告警级别
type AlertSeverity string

const (
	SeverityCritical AlertSeverity = "critical" // 严重
	SeverityWarning  AlertSeverity = "warning"  // 警告
	SeverityInfo     AlertSeverity = "info"     // 信息
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
	// 系统资源告警
	AlertTypeCPU     AlertType = "cpu"
	AlertTypeMemory  AlertType = "memory"
	AlertTypeDisk    AlertType = "disk"
	AlertTypeIO      AlertType = "io"

	// 网络质量告警
	AlertTypeLatency      AlertType = "latency"       // 时延告警
	AlertTypePacketLoss   AlertType = "packet_loss"   // 丢包告警
	AlertTypeRetransmit   AlertType = "retransmit"    // 重传告警
	AlertTypeJitter       AlertType = "jitter"        // 抖动告警
	AlertTypeBandwidth    AlertType = "bandwidth"     // 带宽告警
	AlertTypeConnection   AlertType = "connection"    // 连接数告警
	AlertTypeTCPState     AlertType = "tcp_state"     // TCP状态告警

	// 应用性能告警
	AlertTypeResponseTime AlertType = "response_time" // 响应时间
	AlertTypeErrorRate    AlertType = "error_rate"    // 错误率
	AlertTypeThroughput   AlertType = "throughput"    // 吞吐量
	AlertTypeSlowQuery    AlertType = "slow_query"    // 慢查询

	// 服务可用性告警
	AlertTypeServiceDown  AlertType = "service_down"  // 服务宕机
	AlertTypeHealthCheck  AlertType = "health_check"  // 健康检查失败
)

// AlertRule 告警规则
type AlertRule struct {
	ID          string        `yaml:"id" json:"id"`
	Name        string        `yaml:"name" json:"name"`
	Type        AlertType     `yaml:"type" json:"type"`
	Severity    AlertSeverity `yaml:"severity" json:"severity"`
	Enabled     bool          `yaml:"enabled" json:"enabled"`
	Description string        `yaml:"description" json:"description"`

	// 触发条件
	Conditions []AlertCondition `yaml:"conditions" json:"conditions"`

	// 评估配置
	EvalInterval   time.Duration `yaml:"eval_interval" json:"eval_interval"`     // 评估间隔
	EvalWindow     time.Duration `yaml:"eval_window" json:"eval_window"`         // 评估窗口
	MinDuration    time.Duration `yaml:"min_duration" json:"min_duration"`       // 最小持续时间
	ResolveAfter   time.Duration `yaml:"resolve_after" json:"resolve_after"`     // 恢复判定时间

	// 通知配置
	Notification AlertNotification `yaml:"notification" json:"notification"`

	// 标签
	Labels map[string]string `yaml:"labels" json:"labels"`

	// 注解
	Annotations map[string]string `yaml:"annotations" json:"annotations"`
}

// AlertCondition 告警条件
type AlertCondition struct {
	Metric    string  `yaml:"metric" json:"metric"`       // 指标名称
	Operator  string  `yaml:"operator" json:"operator"`   // 操作符: >, <, ==, >=, <=, !=
	Threshold float64 `yaml:"threshold" json:"threshold"` // 阈值
	Duration  time.Duration `yaml:"duration" json:"duration"` // 持续时间
}

// AlertNotification 告警通知配置
type AlertNotification struct {
	Channels []string `yaml:"channels" json:"channels"` // 通知渠道: webhook, email, sms, dingtalk
	Cooldown time.Duration `yaml:"cooldown" json:"cooldown"` // 冷却时间
	RepeatInterval time.Duration `yaml:"repeat_interval" json:"repeat_interval"` // 重复间隔
	MaxPerHour int `yaml:"max_per_hour" json:"max_per_hour"` // 每小时最大通知数
}

// AlertInstance 告警实例
type AlertInstance struct {
	ID        string            `json:"id"`
	RuleID    string            `json:"rule_id"`
	RuleName  string            `json:"rule_name"`
	Type      AlertType         `json:"type"`
	Severity  AlertSeverity     `json:"severity"`
	Status    AlertStatus       `json:"status"`
	Labels    map[string]string `json:"labels"`
	Value     float64           `json:"value"`
	Threshold float64           `json:"threshold"`
	Message   string            `json:"message"`
	StartsAt  time.Time         `json:"starts_at"`
	EndsAt    *time.Time        `json:"ends_at,omitempty"`
	UpdatedAt time.Time         `json:"updated_at"`
}

// AlertManager 告警管理器
type AlertManager struct {
	rules      map[string]*AlertRule
	instances  map[string]*AlertInstance
	history    []AlertInstance
	metrics    *AlertMetrics
	notifier   *AlertNotifier
	stopCh     chan struct{}
	wg         sync.WaitGroup
	mu         sync.RWMutex
	config     *AlertManagerConfig
}

// AlertManagerConfig 告警管理器配置
type AlertManagerConfig struct {
	Enabled           bool          `yaml:"enabled" json:"enabled"`
	EvalInterval      time.Duration `yaml:"eval_interval" json:"eval_interval"`
	MaxInstances      int           `yaml:"max_instances" json:"max_instances"`
	HistoryRetention  time.Duration `yaml:"history_retention" json:"history_retention"`
	AutoResolve       bool          `yaml:"auto_resolve" json:"auto_resolve"`
}

// DefaultAlertManagerConfig 默认配置
func DefaultAlertManagerConfig() *AlertManagerConfig {
	return &AlertManagerConfig{
		Enabled:          true,
		EvalInterval:     30 * time.Second,
		MaxInstances:     1000,
		HistoryRetention: 7 * 24 * time.Hour,
		AutoResolve:      true,
	}
}

// AlertMetrics 告警指标
type AlertMetrics struct {
	TotalFired      uint64 `json:"total_fired"`
	TotalResolved   uint64 `json:"total_resolved"`
	ActiveCritical  uint64 `json:"active_critical"`
	ActiveWarning   uint64 `json:"active_warning"`
	NotificationsSent uint64 `json:"notifications_sent"`
}

// NewAlertManager 创建告警管理器
func NewAlertManager(config *AlertManagerConfig) *AlertManager {
	if config == nil {
		config = DefaultAlertManagerConfig()
	}

	return &AlertManager{
		rules:     make(map[string]*AlertRule),
		instances: make(map[string]*AlertInstance),
		history:   make([]AlertInstance, 0),
		metrics:   &AlertMetrics{},
		notifier:  NewAlertNotifier(),
		stopCh:    make(chan struct{}),
		config:    config,
	}
}

// Start 启动告警管理器
func (am *AlertManager) Start(ctx context.Context) error {
	logger.Info("Starting alert manager")

	// 注册默认告警规则
	am.registerDefaultRules()

	// 启动评估循环
	am.wg.Add(1)
	go am.evalLoop(ctx)

	// 启动自动恢复检测
	if am.config.AutoResolve {
		am.wg.Add(1)
		go am.autoResolveLoop(ctx)
	}

	logger.Info("Alert manager started")
	return nil
}

// Stop 停止告警管理器
func (am *AlertManager) Stop() error {
	logger.Info("Stopping alert manager")
	close(am.stopCh)
	am.wg.Wait()
	logger.Info("Alert manager stopped")
	return nil
}

// RegisterRule 注册告警规则
func (am *AlertManager) RegisterRule(rule *AlertRule) error {
	am.mu.Lock()
	defer am.mu.Unlock()

	if rule.ID == "" {
		rule.ID = fmt.Sprintf("rule_%s_%d", rule.Type, time.Now().UnixNano())
	}

	// 设置默认值
	if rule.EvalInterval == 0 {
		rule.EvalInterval = 30 * time.Second
	}
	if rule.EvalWindow == 0 {
		rule.EvalWindow = 5 * time.Minute
	}
	if rule.MinDuration == 0 {
		rule.MinDuration = 1 * time.Minute
	}
	if rule.ResolveAfter == 0 {
		rule.ResolveAfter = 5 * time.Minute
	}

	am.rules[rule.ID] = rule
	logger.Infof("Registered alert rule: %s (%s)", rule.Name, rule.ID)
	return nil
}

// UnregisterRule 注销告警规则
func (am *AlertManager) UnregisterRule(ruleID string) {
	am.mu.Lock()
	defer am.mu.Unlock()
	delete(am.rules, ruleID)
}

// GetRules 获取所有规则
func (am *AlertManager) GetRules() []*AlertRule {
	am.mu.RLock()
	defer am.mu.RUnlock()

	rules := make([]*AlertRule, 0, len(am.rules))
	for _, rule := range am.rules {
		rules = append(rules, rule)
	}
	return rules
}

// GetActiveAlerts 获取活跃告警
func (am *AlertManager) GetActiveAlerts() []*AlertInstance {
	am.mu.RLock()
	defer am.mu.RUnlock()

	alerts := make([]*AlertInstance, 0, len(am.instances))
	for _, instance := range am.instances {
		if instance.Status == StatusFiring || instance.Status == StatusPending {
			alerts = append(alerts, instance)
		}
	}
	return alerts
}

// GetAlertHistory 获取告警历史
func (am *AlertManager) GetAlertHistory(limit int) []AlertInstance {
	am.mu.RLock()
	defer am.mu.RUnlock()

	if limit <= 0 || limit > len(am.history) {
		limit = len(am.history)
	}

	// 返回最新的
	start := len(am.history) - limit
	if start < 0 {
		start = 0
	}

	result := make([]AlertInstance, limit)
	copy(result, am.history[start:])
	return result
}

// evalLoop 评估循环
func (am *AlertManager) evalLoop(ctx context.Context) {
	defer am.wg.Done()

	ticker := time.NewTicker(am.config.EvalInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-am.stopCh:
			return
		case <-ticker.C:
			am.evaluateRules(ctx)
		}
	}
}

// evaluateRules 评估所有规则
func (am *AlertManager) evaluateRules(ctx context.Context) {
	am.mu.RLock()
	rules := make([]*AlertRule, 0, len(am.rules))
	for _, rule := range am.rules {
		if rule.Enabled {
			rules = append(rules, rule)
		}
	}
	am.mu.RUnlock()

	for _, rule := range rules {
		select {
		case <-ctx.Done():
			return
		default:
		}

		if err := am.evaluateRule(ctx, rule); err != nil {
			logger.Errorf("Failed to evaluate rule %s: %v", rule.ID, err)
		}
	}
}

// evaluateRule 评估单个规则
func (am *AlertManager) evaluateRule(ctx context.Context, rule *AlertRule) error {
	// 获取指标值
	value, err := am.getMetricValue(rule.Type, rule.Conditions)
	if err != nil {
		return err
	}

	// 检查条件
	triggered := am.checkConditions(value, rule.Conditions)

	// 生成告警ID
	alertID := am.generateAlertID(rule)

	am.mu.Lock()
	defer am.mu.Unlock()

	instance, exists := am.instances[alertID]

	if triggered {
		if !exists {
			// 创建新告警
			instance = &AlertInstance{
				ID:        alertID,
				RuleID:    rule.ID,
				RuleName:  rule.Name,
				Type:      rule.Type,
				Severity:  rule.Severity,
				Status:    StatusPending,
				Labels:    rule.Labels,
				Value:     value,
				Threshold: rule.Conditions[0].Threshold,
				Message:   am.generateAlertMessage(rule, value),
				StartsAt:  time.Now(),
				UpdatedAt: time.Now(),
			}
			am.instances[alertID] = instance
			logger.Infof("Alert pending: %s", alertID)
		} else {
			// 更新现有告警
			instance.Value = value
			instance.UpdatedAt = time.Now()

			// 检查是否满足最小持续时间
			if instance.Status == StatusPending &&
				time.Since(instance.StartsAt) >= rule.MinDuration {
				instance.Status = StatusFiring
				am.metrics.TotalFired++
				if instance.Severity == SeverityCritical {
					am.metrics.ActiveCritical++
				} else {
					am.metrics.ActiveWarning++
				}
				logger.Warnf("Alert firing: %s (value: %.2f, threshold: %.2f)",
					alertID, value, instance.Threshold)

				// 发送通知
				go am.notifier.Notify(instance, &rule.Notification)
			}
		}
	} else {
		if exists && instance.Status != StatusResolved {
			// 标记为待恢复
			if instance.Status == StatusFiring {
				instance.Status = StatusPending
				now := time.Now()
				instance.EndsAt = &now
				logger.Infof("Alert pending resolve: %s", alertID)
			}
		}
	}

	return nil
}

// checkConditions 检查条件
func (am *AlertManager) checkConditions(value float64, conditions []AlertCondition) bool {
	if len(conditions) == 0 {
		return false
	}

	// 目前只支持单个条件，AND/OR 逻辑可扩展
	cond := conditions[0]

	switch cond.Operator {
	case ">":
		return value > cond.Threshold
	case ">=":
		return value >= cond.Threshold
	case "<":
		return value < cond.Threshold
	case "<=":
		return value <= cond.Threshold
	case "==":
		return value == cond.Threshold
	case "!=":
		return value != cond.Threshold
	default:
		return false
	}
}

// generateAlertID 生成告警ID
func (am *AlertManager) generateAlertID(rule *AlertRule) string {
	// 基于规则ID和标签生成唯一ID
	labels := make([]string, 0, len(rule.Labels))
	for k, v := range rule.Labels {
		labels = append(labels, fmt.Sprintf("%s=%s", k, v))
	}
	return fmt.Sprintf("%s:%s", rule.ID, strings.Join(labels, ","))
}

// generateAlertMessage 生成告警消息
func (am *AlertManager) generateAlertMessage(rule *AlertRule, value float64) string {
	if msg, ok := rule.Annotations["summary"]; ok {
		return msg
	}

	return fmt.Sprintf("%s: current value %.2f exceeds threshold %.2f",
		rule.Name, value, rule.Conditions[0].Threshold)
}

// getMetricValue 获取指标值
func (am *AlertManager) getMetricValue(alertType AlertType, conditions []AlertCondition) (float64, error) {
	// 这里应该连接到实际的指标采集系统
	// 简化实现：返回模拟值
	return 0, nil
}

// autoResolveLoop 自动恢复检测循环
func (am *AlertManager) autoResolveLoop(ctx context.Context) {
	defer am.wg.Done()

	ticker := time.NewTicker(1 * time.Minute)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-am.stopCh:
			return
		case <-ticker.C:
			am.checkAutoResolve()
		}
	}
}

// checkAutoResolve 检查自动恢复
func (am *AlertManager) checkAutoResolve() {
	am.mu.Lock()
	defer am.mu.Unlock()

	now := time.Now()
	for id, instance := range am.instances {
		if instance.Status == StatusPending && instance.EndsAt != nil {
			// 获取规则
			rule, exists := am.rules[instance.RuleID]
			if !exists {
				continue
			}

			// 检查是否满足恢复条件
			if now.Sub(*instance.EndsAt) >= rule.ResolveAfter {
				instance.Status = StatusResolved
				am.metrics.TotalResolved++
				if instance.Severity == SeverityCritical {
					am.metrics.ActiveCritical--
				} else {
					am.metrics.ActiveWarning--
				}

				// 添加到历史
				am.history = append(am.history, *instance)
				if len(am.history) > 10000 {
					am.history = am.history[1000:]
				}

				// 删除活跃实例
				delete(am.instances, id)

				logger.Infof("Alert resolved: %s", id)

				// 发送恢复通知
				go am.notifier.NotifyResolved(instance)
			}
		}
	}
}

// GetMetrics 获取告警指标
func (am *AlertManager) GetMetrics() AlertMetrics {
	am.mu.RLock()
	defer am.mu.RUnlock()
	return *am.metrics
}

// registerDefaultRules 注册默认告警规则
func (am *AlertManager) registerDefaultRules() {
	// 网络时延告警模板
	am.RegisterRule(&AlertRule{
		ID:          "network_latency_high",
		Name:        "网络时延过高",
		Type:        AlertTypeLatency,
		Severity:    SeverityWarning,
		Enabled:     true,
		Description: "网络往返时延超过阈值",
		Conditions: []AlertCondition{
			{Metric: "network_rtt_ms", Operator: ">", Threshold: 100, Duration: 2 * time.Minute},
		},
		EvalInterval: 30 * time.Second,
		EvalWindow:   5 * time.Minute,
		MinDuration:  2 * time.Minute,
		Labels: map[string]string{
			"category": "network",
			"service":  "connectivity",
		},
		Annotations: map[string]string{
			"summary":     "网络时延过高: {{ $value }}ms",
			"description": "网络往返时延持续超过100ms，可能影响业务响应",
			"runbook_url": "https://wiki/runbooks/network-latency",
		},
	})

	// 网络时延严重告警
	am.RegisterRule(&AlertRule{
		ID:          "network_latency_critical",
		Name:        "网络时延严重超标",
		Type:        AlertTypeLatency,
		Severity:    SeverityCritical,
		Enabled:     true,
		Description: "网络往返时延严重超标",
		Conditions: []AlertCondition{
			{Metric: "network_rtt_ms", Operator: ">", Threshold: 500, Duration: 1 * time.Minute},
		},
		EvalInterval: 30 * time.Second,
		EvalWindow:   5 * time.Minute,
		MinDuration:  1 * time.Minute,
		Labels: map[string]string{
			"category": "network",
			"service":  "connectivity",
		},
		Annotations: map[string]string{
			"summary":     "网络时延严重超标: {{ $value }}ms",
			"description": "网络往返时延超过500ms，严重影响业务",
			"runbook_url": "https://wiki/runbooks/network-latency-critical",
		},
	})

	// 丢包率告警
	am.RegisterRule(&AlertRule{
		ID:          "packet_loss_warning",
		Name:        "网络丢包率过高",
		Type:        AlertTypePacketLoss,
		Severity:    SeverityWarning,
		Enabled:     true,
		Description: "网络丢包率超过阈值",
		Conditions: []AlertCondition{
			{Metric: "packet_loss_percent", Operator: ">", Threshold: 1.0, Duration: 2 * time.Minute},
		},
		EvalInterval: 30 * time.Second,
		EvalWindow:   5 * time.Minute,
		MinDuration:  2 * time.Minute,
		Labels: map[string]string{
			"category": "network",
			"service":  "connectivity",
		},
		Annotations: map[string]string{
			"summary":     "网络丢包率: {{ $value }}%",
			"description": "网络丢包率超过1%，可能导致数据传输不完整",
			"runbook_url": "https://wiki/runbooks/packet-loss",
		},
	})

	// 丢包率严重告警
	am.RegisterRule(&AlertRule{
		ID:          "packet_loss_critical",
		Name:        "网络丢包率严重",
		Type:        AlertTypePacketLoss,
		Severity:    SeverityCritical,
		Enabled:     true,
		Description: "网络丢包率严重超标",
		Conditions: []AlertCondition{
			{Metric: "packet_loss_percent", Operator: ">", Threshold: 5.0, Duration: 1 * time.Minute},
		},
		EvalInterval: 30 * time.Second,
		EvalWindow:   5 * time.Minute,
		MinDuration:  1 * time.Minute,
		Labels: map[string]string{
			"category": "network",
			"service":  "connectivity",
		},
		Annotations: map[string]string{
			"summary":     "网络丢包率严重: {{ $value }}%",
			"description": "网络丢包率超过5%，严重影响数据传输",
			"runbook_url": "https://wiki/runbooks/packet-loss-critical",
		},
	})

	// TCP重传率告警
	am.RegisterRule(&AlertRule{
		ID:          "tcp_retransmit_warning",
		Name:        "TCP重传率过高",
		Type:        AlertTypeRetransmit,
		Severity:    SeverityWarning,
		Enabled:     true,
		Description: "TCP重传率超过阈值",
		Conditions: []AlertCondition{
			{Metric: "tcp_retransmit_percent", Operator: ">", Threshold: 2.0, Duration: 3 * time.Minute},
		},
		EvalInterval: 30 * time.Second,
		EvalWindow:   5 * time.Minute,
		MinDuration:  3 * time.Minute,
		Labels: map[string]string{
			"category": "network",
			"service":  "tcp",
		},
		Annotations: map[string]string{
			"summary":     "TCP重传率: {{ $value }}%",
			"description": "TCP重传率超过2%，可能存在网络拥塞或丢包",
			"runbook_url": "https://wiki/runbooks/tcp-retransmit",
		},
	})

	// TCP重传率严重告警
	am.RegisterRule(&AlertRule{
		ID:          "tcp_retransmit_critical",
		Name:        "TCP重传率严重",
		Type:        AlertTypeRetransmit,
		Severity:    SeverityCritical,
		Enabled:     true,
		Description: "TCP重传率严重超标",
		Conditions: []AlertCondition{
			{Metric: "tcp_retransmit_percent", Operator: ">", Threshold: 10.0, Duration: 1 * time.Minute},
		},
		EvalInterval: 30 * time.Second,
		EvalWindow:   5 * time.Minute,
		MinDuration:  1 * time.Minute,
		Labels: map[string]string{
			"category": "network",
			"service":  "tcp",
		},
		Annotations: map[string]string{
			"summary":     "TCP重传率严重: {{ $value }}%",
			"description": "TCP重传率超过10%，网络质量严重恶化",
			"runbook_url": "https://wiki/runbooks/tcp-retransmit-critical",
		},
	})

	// 网络抖动告警
	am.RegisterRule(&AlertRule{
		ID:          "network_jitter_high",
		Name:        "网络抖动过高",
		Type:        AlertTypeJitter,
		Severity:    SeverityWarning,
		Enabled:     true,
		Description: "网络时延抖动超过阈值",
		Conditions: []AlertCondition{
			{Metric: "network_jitter_ms", Operator: ">", Threshold: 20, Duration: 2 * time.Minute},
		},
		EvalInterval: 30 * time.Second,
		EvalWindow:   5 * time.Minute,
		MinDuration:  2 * time.Minute,
		Labels: map[string]string{
			"category": "network",
			"service":  "quality",
		},
		Annotations: map[string]string{
			"summary":     "网络抖动: {{ $value }}ms",
			"description": "网络时延抖动超过20ms，可能影响实时业务",
			"runbook_url": "https://wiki/runbooks/network-jitter",
		},
	})

	// 带宽利用率告警
	am.RegisterRule(&AlertRule{
		ID:          "bandwidth_high",
		Name:        "带宽利用率过高",
		Type:        AlertTypeBandwidth,
		Severity:    SeverityWarning,
		Enabled:     true,
		Description: "网络带宽利用率超过阈值",
		Conditions: []AlertCondition{
			{Metric: "bandwidth_usage_percent", Operator: ">", Threshold: 80, Duration: 5 * time.Minute},
		},
		EvalInterval: 1 * time.Minute,
		EvalWindow:   10 * time.Minute,
		MinDuration:  5 * time.Minute,
		Labels: map[string]string{
			"category": "network",
			"service":  "capacity",
		},
		Annotations: map[string]string{
			"summary":     "带宽利用率: {{ $value }}%",
			"description": "网络带宽利用率超过80%，接近容量上限",
			"runbook_url": "https://wiki/runbooks/bandwidth",
		},
	})

	// TCP连接数告警
	am.RegisterRule(&AlertRule{
		ID:          "tcp_connections_high",
		Name:        "TCP连接数过高",
		Type:        AlertTypeConnection,
		Severity:    SeverityWarning,
		Enabled:     true,
		Description: "TCP连接数超过阈值",
		Conditions: []AlertCondition{
			{Metric: "tcp_connections_count", Operator: ">", Threshold: 10000, Duration: 5 * time.Minute},
		},
		EvalInterval: 1 * time.Minute,
		EvalWindow:   10 * time.Minute,
		MinDuration:  5 * time.Minute,
		Labels: map[string]string{
			"category": "network",
			"service":  "connections",
		},
		Annotations: map[string]string{
			"summary":     "TCP连接数: {{ $value }}",
			"description": "TCP连接数超过10000，可能存在连接泄漏",
			"runbook_url": "https://wiki/runbooks/tcp-connections",
		},
	})

	// TCP零窗口告警
	am.RegisterRule(&AlertRule{
		ID:          "tcp_zero_window",
		Name:        "TCP零窗口事件",
		Type:        AlertTypeTCPState,
		Severity:    SeverityWarning,
		Enabled:     true,
		Description: "检测到TCP零窗口事件",
		Conditions: []AlertCondition{
			{Metric: "tcp_zero_window_count", Operator: ">", Threshold: 10, Duration: 1 * time.Minute},
		},
		EvalInterval: 30 * time.Second,
		EvalWindow:   5 * time.Minute,
		MinDuration:  1 * time.Minute,
		Labels: map[string]string{
			"category": "network",
			"service":  "tcp",
		},
		Annotations: map[string]string{
			"summary":     "TCP零窗口事件: {{ $value }}",
			"description": "检测到TCP零窗口事件，接收端缓冲区已满",
			"runbook_url": "https://wiki/runbooks/tcp-zero-window",
		},
	})
}

// AlertNotifier 告警通知器
type AlertNotifier struct {
	channels map[string]NotificationChannel
	mu       sync.RWMutex
}

// NotificationChannel 通知渠道接口
type NotificationChannel interface {
	Send(alert *AlertInstance) error
	SendResolved(alert *AlertInstance) error
	Name() string
}

// NewAlertNotifier 创建告警通知器
func NewAlertNotifier() *AlertNotifier {
	return &AlertNotifier{
		channels: make(map[string]NotificationChannel),
	}
}

// RegisterChannel 注册通知渠道
func (n *AlertNotifier) RegisterChannel(name string, channel NotificationChannel) {
	n.mu.Lock()
	defer n.mu.Unlock()
	n.channels[name] = channel
}

// Notify 发送告警通知
func (n *AlertNotifier) Notify(alert *AlertInstance, config *AlertNotification) error {
	n.mu.RLock()
	defer n.mu.RUnlock()

	for _, channelName := range config.Channels {
		if channel, exists := n.channels[channelName]; exists {
			if err := channel.Send(alert); err != nil {
				logger.Errorf("Failed to send alert via %s: %v", channelName, err)
			}
		}
	}

	return nil
}

// NotifyResolved 发送恢复通知
func (n *AlertNotifier) NotifyResolved(alert *AlertInstance) error {
	n.mu.RLock()
	defer n.mu.RUnlock()

	// 获取规则通知配置
	// 简化实现：发送到所有渠道
	for _, channel := range n.channels {
		if err := channel.SendResolved(alert); err != nil {
			logger.Errorf("Failed to send resolved alert: %v", err)
		}
	}

	return nil
}

// WebhookChannel Webhook通知渠道
type WebhookChannel struct {
	URL     string
	Headers map[string]string
	client  *http.Client
}

// NewWebhookChannel 创建Webhook渠道
func NewWebhookChannel(url string) *WebhookChannel {
	return &WebhookChannel{
		URL:     url,
		Headers: make(map[string]string),
		client:  &http.Client{Timeout: 10 * time.Second},
	}
}

// Name 返回渠道名称
func (c *WebhookChannel) Name() string {
	return "webhook"
}

// Send 发送告警
func (c *WebhookChannel) Send(alert *AlertInstance) error {
	payload, err := json.Marshal(alert)
	if err != nil {
		return fmt.Errorf("marshal alert: %w", err)
	}

	req, err := http.NewRequest("POST", c.URL, strings.NewReader(string(payload)))
	if err != nil {
		return fmt.Errorf("create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	for k, v := range c.Headers {
		req.Header.Set(k, v)
	}

	resp, err := c.client.Do(req)
	if err != nil {
		return fmt.Errorf("send request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		return fmt.Errorf("webhook returned status %d", resp.StatusCode)
	}

	return nil
}

// SendResolved 发送恢复通知
func (c *WebhookChannel) SendResolved(alert *AlertInstance) error {
	// 添加恢复标记
	alert.Status = StatusResolved
	return c.Send(alert)
}
