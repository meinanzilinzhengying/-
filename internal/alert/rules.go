//go:build linux

// Package alert 提供告警管理功能
// 本文件实现告警规则引擎，支持自定义规则和模板
package alert

import (
	"context"
	"fmt"
	"regexp"
	"strconv"
	"sync"
	"time"

	"cloud-flow-agent/pkg/logger"
)

// ==================== 操作符定义 ====================

// Operator 比较操作符
type Operator string

const (
	OpEqual         Operator = "=="
	OpNotEqual      Operator = "!="
	OpGreaterThan   Operator = ">"
	OpGreaterEqual  Operator = ">="
	OpLessThan      Operator = "<"
	OpLessEqual     Operator = "<="
	OpContains      Operator = "contains"
	OpNotContains   Operator = "not_contains"
	OpRegex         Operator = "regex"
	OpInRange       Operator = "in_range"
	OpNotInRange    Operator = "not_in_range"
)

// LogicOperator 逻辑操作符
type LogicOperator string

const (
	LogicAnd LogicOperator = "AND"
	LogicOr  LogicOperator = "OR"
)

// ==================== 规则条件 ====================

// RuleCondition 规则条件
type RuleCondition struct {
	Metric     string            `json:"metric" yaml:"metric"`
	Operator   Operator          `json:"operator" yaml:"operator"`
	Threshold  float64           `json:"threshold" yaml:"threshold"`
	Threshold2 float64           `json:"threshold2,omitempty" yaml:"threshold2,omitempty"` // 用于范围比较
	For        string            `json:"for,omitempty" yaml:"for,omitempty"`               // 持续时间要求，如 "5m"
	Labels     map[string]string `json:"labels,omitempty" yaml:"labels,omitempty"`
	Weight     float64           `json:"weight,omitempty" yaml:"weight,omitempty"`         // 条件权重
	Negate     bool              `json:"negate,omitempty" yaml:"negate,omitempty"`         // 是否取反
}

// Duration 解析持续时间
func (c *RuleCondition) Duration() time.Duration {
	d, _ := time.ParseDuration(c.For)
	return d
}

// Evaluate 评估条件
func (c *RuleCondition) Evaluate(value float64) bool {
	result := false
	switch c.Operator {
	case OpEqual:
		result = value == c.Threshold
	case OpNotEqual:
		result = value != c.Threshold
	case OpGreaterThan:
		result = value > c.Threshold
	case OpGreaterEqual:
		result = value >= c.Threshold
	case OpLessThan:
		result = value < c.Threshold
	case OpLessEqual:
		result = value <= c.Threshold
	case OpInRange:
		result = value >= c.Threshold && value <= c.Threshold2
	case OpNotInRange:
		result = value < c.Threshold || value > c.Threshold2
	default:
		result = false
	}

	if c.Negate {
		return !result
	}
	return result
}

// ==================== 告警规则 ====================

// AlertRule 告警规则
type AlertRule struct {
	ID             string            `json:"id" yaml:"id"`
	Name           string            `json:"name" yaml:"name"`
	Level          AlertLevel        `json:"level" yaml:"level"`
	Description    string            `json:"description,omitempty" yaml:"description,omitempty"`
	Conditions     []RuleCondition   `json:"conditions" yaml:"conditions"`
	Logic          LogicOperator     `json:"logic,omitempty" yaml:"logic,omitempty"` // AND/OR，默认AND
	FireThreshold  int               `json:"fire_threshold" yaml:"fire_threshold"`   // 触发阈值（连续满足次数）
	ResolveAfter   string            `json:"resolve_after,omitempty" yaml:"resolve_after,omitempty"`
	NotifyChannels []string          `json:"notify_channels,omitempty" yaml:"notify_channels,omitempty"`
	Labels         map[string]string `json:"labels,omitempty" yaml:"labels,omitempty"`
	Annotations    map[string]string `json:"annotations,omitempty" yaml:"annotations,omitempty"`
	Enabled        bool              `json:"enabled" yaml:"enabled"`
	CreatedAt      time.Time         `json:"created_at" yaml:"created_at"`
	UpdatedAt      time.Time         `json:"updated_at" yaml:"updated_at"`

	// 运行时状态
	state       map[string]*ruleState // key: instance fingerprint
	mu          sync.RWMutex
	template    BuiltInTemplate
	isTemplate  bool
}

// ruleState 规则状态（每个实例独立跟踪）
type ruleState struct {
	ConsecutiveViolations int       // 连续违反次数
	ConsecutiveOK         int       // 连续正常次数
	FirstViolationAt      time.Time // 首次违反时间
	LastViolationAt       time.Time // 最后违反时间
	Fired                 bool      // 是否已触发告警
	FiredAt               time.Time // 触发时间
}

// ShouldFire 检查是否应该触发告警
func (r *AlertRule) ShouldFire(instanceKey string, values map[string]float64) (bool, map[string]float64) {
	r.mu.Lock()
	defer r.mu.Unlock()

	if r.state == nil {
		r.state = make(map[string]*ruleState)
	}

	state, exists := r.state[instanceKey]
	if !exists {
		state = &ruleState{}
		r.state[instanceKey] = state
	}

	// 评估所有条件
	violatingConditions := make(map[string]float64)
	violationCount := 0
	now := time.Now()

	for _, cond := range r.Conditions {
		value, exists := values[cond.Metric]
		if !exists {
			continue
		}

		if cond.Evaluate(value) {
			violationCount++
			violatingConditions[cond.Metric] = value
		}
	}

	// 根据逻辑操作符判断是否违反
	isViolating := false
	if len(r.Conditions) > 0 {
		switch r.Logic {
		case LogicOr:
			isViolating = violationCount > 0
		default: // LogicAnd
			isViolating = violationCount == len(r.Conditions)
		}
	}

	// 更新状态
	if isViolating {
		state.ConsecutiveOK = 0
		state.ConsecutiveViolations++
		state.LastViolationAt = now

		if state.ConsecutiveViolations == 1 {
			state.FirstViolationAt = now
		}

		// 检查是否满足触发条件
		if !state.Fired && state.ConsecutiveViolations >= r.FireThreshold {
			state.Fired = true
			state.FiredAt = now
			return true, violatingConditions
		}
	} else {
		state.ConsecutiveViolations = 0
		state.ConsecutiveOK++
	}

	return false, nil
}

// IsFiring 检查是否正在触发
func (r *AlertRule) IsFiring(instanceKey string) bool {
	r.mu.RLock()
	defer r.mu.RUnlock()

	if state, exists := r.state[instanceKey]; exists {
		return state.Fired
	}
	return false
}

// MarkResolved 标记为已恢复
func (r *AlertRule) MarkResolved(instanceKey string) {
	r.mu.Lock()
	defer r.mu.Unlock()

	if state, exists := r.state[instanceKey]; exists {
		state.Fired = false
		state.ConsecutiveViolations = 0
		state.ConsecutiveOK = 0
	}
}

// GetState 获取规则状态
func (r *AlertRule) GetState(instanceKey string) *ruleState {
	r.mu.RLock()
	defer r.mu.RUnlock()

	if state, exists := r.state[instanceKey]; exists {
		return state
	}
	return nil
}

// ==================== RuleBuilder 规则构建器 ====================

// RuleBuilder 规则构建器
type RuleBuilder struct {
	rule *AlertRule
}

// NewRuleBuilder 创建规则构建器
func NewRuleBuilder() *RuleBuilder {
	return &RuleBuilder{
		rule: &AlertRule{
			ID:            generateRuleID(),
			Enabled:       true,
			Logic:         LogicAnd,
			FireThreshold: 1,
			CreatedAt:     time.Now(),
			UpdatedAt:     time.Now(),
			Labels:        make(map[string]string),
			Annotations:   make(map[string]string),
		},
	}
}

// ID 设置规则ID
func (b *RuleBuilder) ID(id string) *RuleBuilder {
	b.rule.ID = id
	return b
}

// Name 设置规则名称
func (b *RuleBuilder) Name(name string) *RuleBuilder {
	b.rule.Name = name
	return b
}

// Level 设置告警级别
func (b *RuleBuilder) Level(level AlertLevel) *RuleBuilder {
	b.rule.Level = level
	return b
}

// Description 设置描述
func (b *RuleBuilder) Description(desc string) *RuleBuilder {
	b.rule.Description = desc
	return b
}

// Condition 添加条件
func (b *RuleBuilder) Condition(metric string, op Operator, threshold float64) *RuleBuilder {
	b.rule.Conditions = append(b.rule.Conditions, RuleCondition{
		Metric:    metric,
		Operator:  op,
		Threshold: threshold,
	})
	return b
}

// ConditionWithDuration 添加带持续时间的条件
func (b *RuleBuilder) ConditionWithDuration(metric string, op Operator, threshold float64, duration string) *RuleBuilder {
	b.rule.Conditions = append(b.rule.Conditions, RuleCondition{
		Metric:    metric,
		Operator:  op,
		Threshold: threshold,
		For:       duration,
	})
	return b
}

// RangeCondition 添加范围条件
func (b *RuleBuilder) RangeCondition(metric string, min, max float64) *RuleBuilder {
	b.rule.Conditions = append(b.rule.Conditions, RuleCondition{
		Metric:     metric,
		Operator:   OpInRange,
		Threshold:  min,
		Threshold2: max,
	})
	return b
}

// Logic 设置逻辑操作符
func (b *RuleBuilder) Logic(logic LogicOperator) *RuleBuilder {
	b.rule.Logic = logic
	return b
}

// FireThreshold 设置触发阈值
func (b *RuleBuilder) FireThreshold(threshold int) *RuleBuilder {
	b.rule.FireThreshold = threshold
	return b
}

// ResolveAfter 设置恢复时间
func (b *RuleBuilder) ResolveAfter(after string) *RuleBuilder {
	b.rule.ResolveAfter = after
	return b
}

// Labels 设置标签
func (b *RuleBuilder) Labels(labels map[string]string) *RuleBuilder {
	b.rule.Labels = labels
	return b
}

// Annotations 设置注释
func (b *RuleBuilder) Annotations(annotations map[string]string) *RuleBuilder {
	b.rule.Annotations = annotations
	return b
}

// NotifyChannels 设置通知渠道
func (b *RuleBuilder) NotifyChannels(channels []string) *RuleBuilder {
	b.rule.NotifyChannels = channels
	return b
}

// Build 构建规则
func (b *RuleBuilder) Build() *AlertRule {
	b.rule.UpdatedAt = time.Now()
	return b.rule
}

// ==================== RuleEngine 规则引擎 ====================

// EvaluationCallback 评估回调函数
type EvaluationCallback func(ruleID string, metric string, labels map[string]string, isViolating bool, currentValue float64)

// RuleEngine 规则引擎
type RuleEngine struct {
	rules        map[string]*AlertRule
	templates    map[BuiltInTemplate]*AlertRule
	templatesLib *TemplateLibrary
	mu           sync.RWMutex
	log          *logger.Logger

	// 回调函数
	evalCallback EvaluationCallback
	alertHandler func(*AlertEvent)
}

// NewRuleEngine 创建规则引擎
func NewRuleEngine(log *logger.Logger) *RuleEngine {
	return &RuleEngine{
		rules:        make(map[string]*AlertRule),
		templates:    make(map[string]*AlertRule),
		templatesLib: NewTemplateLibrary(),
		log:          log,
	}
}

// SetEvaluationCallback 设置评估回调
func (e *RuleEngine) SetEvaluationCallback(callback EvaluationCallback) {
	e.evalCallback = callback
}

// SetAlertHandler 设置告警处理器
func (e *RuleEngine) SetAlertHandler(handler func(*AlertEvent)) {
	e.alertHandler = handler
}

// RegisterRule 注册自定义规则
func (e *RuleEngine) RegisterRule(rule *AlertRule) error {
	e.mu.Lock()
	defer e.mu.Unlock()

	if rule.ID == "" {
		return fmt.Errorf("规则ID不能为空")
	}

	if err := e.validateRule(rule); err != nil {
		return fmt.Errorf("规则验证失败: %w", err)
	}

	rule.UpdatedAt = time.Now()
	e.rules[rule.ID] = rule
	e.log.Infof("注册告警规则: %s (%s)", rule.Name, rule.ID)
	return nil
}

// UnregisterRule 注销规则
func (e *RuleEngine) UnregisterRule(ruleID string) {
	e.mu.Lock()
	defer e.mu.Unlock()

	delete(e.rules, ruleID)
	e.log.Infof("注销告警规则: %s", ruleID)
}

// GetRule 获取规则
func (e *RuleEngine) GetRule(ruleID string) (*AlertRule, bool) {
	e.mu.RLock()
	defer e.mu.RUnlock()

	rule, ok := e.rules[ruleID]
	return rule, ok
}

// GetAllRules 获取所有规则
func (e *RuleEngine) GetAllRules() []*AlertRule {
	e.mu.RLock()
	defer e.mu.RUnlock()

	rules := make([]*AlertRule, 0, len(e.rules))
	for _, rule := range e.rules {
		rules = append(rules, rule)
	}
	return rules
}

// CreateRuleFromTemplate 从模板创建规则
func (e *RuleEngine) CreateRuleFromTemplate(template BuiltInTemplate, overrides map[string]interface{}) (*AlertRule, error) {
	tmplDef, exists := e.templatesLib.Get(template)
	if !exists {
		return nil, fmt.Errorf("模板不存在: %s", template)
	}

	builder := NewRuleBuilder().
		ID(generateRuleID()).
		Name(tmplDef.Name).
		Level(tmplDef.Level).
		Description(tmplDef.Description).
		Condition(tmplDef.Metric, tmplDef.Operator, tmplDef.Threshold).
		FireThreshold(tmplDef.FireThreshold).
		Labels(tmplDef.Labels).
		Annotations(tmplDef.Annotations)

	if tmplDef.ResolveAfter != "" {
		builder.ResolveAfter(tmplDef.ResolveAfter)
	}

	// 应用覆盖
	if name, ok := overrides["name"].(string); ok {
		builder.Name(name)
	}
	if threshold, ok := overrides["threshold"].(float64); ok {
		// 重新创建条件
		builder = NewRuleBuilder().
			ID(generateRuleID()).
			Name(tmplDef.Name).
			Level(tmplDef.Level).
			Description(tmplDef.Description).
			Condition(tmplDef.Metric, tmplDef.Operator, threshold).
			FireThreshold(tmplDef.FireThreshold).
			Labels(tmplDef.Labels).
			Annotations(tmplDef.Annotations)
	}

	return builder.Build(), nil
}

// Evaluate 评估指标数据
func (e *RuleEngine) Evaluate(metrics []MetricData) []*AlertEvent {
	e.mu.RLock()
	rules := make([]*AlertRule, 0, len(e.rules))
	for _, rule := range e.rules {
		rules = append(rules, rule)
	}
	e.mu.RUnlock()

	var events []*AlertEvent
	now := time.Now()

	for _, rule := range rules {
		if !rule.Enabled {
			continue
		}

		// 按实例分组评估
		instanceMetrics := e.groupByInstance(metrics)

		for instanceKey, instanceData := range instanceMetrics {
			// 构建指标值映射
			values := make(map[string]float64)
			for _, m := range instanceData {
				values[m.Name] = m.Value
			}

			// 评估规则
			shouldFire, violatingMetrics := rule.ShouldFire(instanceKey, values)

			// 调用评估回调
			if e.evalCallback != nil {
				for metric, value := range values {
					_, isViolating := violatingMetrics[metric]
					e.evalCallback(rule.ID, metric, instanceData[0].Labels, isViolating, value)
				}
			}

			if shouldFire {
				// 获取第一个违反的指标值
				var value, threshold float64
				var metric string
				for m, v := range violatingMetrics {
					metric = m
					value = v
					break
				}

				if len(rule.Conditions) > 0 {
					threshold = rule.Conditions[0].Threshold
				}

				event := &AlertEvent{
					ID:          generateEventID(),
					RuleID:      rule.ID,
					RuleName:    rule.Name,
					Level:       rule.Level,
					State:       AlertStateFiring,
					Metric:      metric,
					Value:       value,
					Threshold:   threshold,
					Labels:      instanceData[0].Labels,
					Annotations: rule.Annotations,
					FiredAt:     now,
					LastValueAt: now,
				}

				if event.Annotations == nil {
					event.Annotations = make(map[string]string)
				}
				event.Annotations["description"] = rule.Description

				events = append(events, event)

				// 调用告警处理器
				if e.alertHandler != nil {
					e.alertHandler(event)
				}
			}
		}
	}

	return events
}

// groupByInstance 按实例分组指标
func (e *RuleEngine) groupByInstance(metrics []MetricData) map[string][]MetricData {
	groups := make(map[string][]MetricData)

	for _, m := range metrics {
		instance := m.Labels["instance"]
		if instance == "" {
			instance = m.Labels["host"]
		}
		if instance == "" {
			instance = "default"
		}

		key := instance
		groups[key] = append(groups[key], m)
	}

	return groups
}

// validateRule 验证规则
func (e *RuleEngine) validateRule(rule *AlertRule) error {
	if rule.Name == "" {
		return fmt.Errorf("规则名称不能为空")
	}

	if len(rule.Conditions) == 0 {
		return fmt.Errorf("规则必须至少有一个条件")
	}

	for i, cond := range rule.Conditions {
		if cond.Metric == "" {
			return fmt.Errorf("条件%d: 指标不能为空", i)
		}

		switch cond.Operator {
		case OpEqual, OpNotEqual, OpGreaterThan, OpGreaterEqual, OpLessThan, OpLessEqual, OpContains, OpNotContains, OpRegex, OpInRange, OpNotInRange:
			// 有效操作符
		default:
			return fmt.Errorf("条件%d: 无效的操作符: %s", i, cond.Operator)
		}

		if cond.Operator == OpInRange || cond.Operator == OpNotInRange {
			if cond.Threshold2 <= cond.Threshold {
				return fmt.Errorf("条件%d: 范围上限必须大于下限", i)
			}
		}
	}

	if rule.Logic != LogicAnd && rule.Logic != LogicOr {
		return fmt.Errorf("无效的逻辑操作符: %s", rule.Logic)
	}

	if rule.FireThreshold < 1 {
		return fmt.Errorf("触发阈值必须大于等于1")
	}

	return nil
}

// LoadBuiltinTemplates 加载内置模板为规则
func (e *RuleEngine) LoadBuiltinTemplates() {
	for tmpl, def := range e.templatesLib.GetAll() {
		rule := &AlertRule{
			ID:            string(tmpl),
			Name:          def.Name,
			Level:         def.Level,
			Description:   def.Description,
			Conditions:    []RuleCondition{{Metric: def.Metric, Operator: def.Operator, Threshold: def.Threshold}},
			FireThreshold: def.FireThreshold,
			ResolveAfter:  def.ResolveAfter,
			Labels:        def.Labels,
			Annotations:   def.Annotations,
			Enabled:       true,
			CreatedAt:     time.Now(),
			UpdatedAt:     time.Now(),
			template:      tmpl,
			isTemplate:    true,
		}

		e.templates[tmpl] = rule
		e.log.Debugf("加载告警模板: %s", tmpl)
	}
}

// CreateFromTemplate 从模板创建规则实例
func (e *RuleEngine) CreateFromTemplate(tmpl BuiltInTemplate, customID string) (*AlertRule, error) {
	templateRule, exists := e.templates[tmpl]
	if !exists {
		return nil, fmt.Errorf("模板不存在: %s", tmpl)
	}

	rule := &AlertRule{
		ID:             customID,
		Name:           templateRule.Name,
		Level:          templateRule.Level,
		Description:    templateRule.Description,
		Conditions:     templateRule.Conditions,
		Logic:          templateRule.Logic,
		FireThreshold:  templateRule.FireThreshold,
		ResolveAfter:   templateRule.ResolveAfter,
		NotifyChannels: templateRule.NotifyChannels,
		Labels:         templateRule.Labels,
		Annotations:    templateRule.Annotations,
		Enabled:        true,
		CreatedAt:      time.Now(),
		UpdatedAt:      time.Now(),
		template:       tmpl,
		isTemplate:     false,
	}

	return rule, nil
}

// ==================== 辅助函数 ====================

func generateRuleID() string {
	return fmt.Sprintf("rule-%d", time.Now().UnixNano())
}

// ParseLevel 解析告警级别字符串
func ParseLevel(s string) AlertLevel {
	switch s {
	case "info", "一般":
		return AlertLevelInfo
	case "warning", "warn", "重要":
		return AlertLevelWarning
	case "critical", "crit", "紧急":
		return AlertLevelCritical
	default:
		return AlertLevelInfo
	}
}

// ParseOperator 解析操作符字符串
func ParseOperator(s string) Operator {
	switch s {
	case "==", "eq", "equal":
		return OpEqual
	case "!=", "ne", "not_equal":
		return OpNotEqual
	case ">", "gt", "greater":
		return OpGreaterThan
	case ">=", "ge", "greater_equal":
		return OpGreaterEqual
	case "<", "lt", "less":
		return OpLessThan
	case "<=", "le", "less_equal":
		return OpLessEqual
	case "contains":
		return OpContains
	case "not_contains":
		return OpNotContains
	case "regex", "match":
		return OpRegex
	case "in_range", "range":
		return OpInRange
	case "not_in_range":
		return OpNotInRange
	default:
		return OpGreaterThan
	}
}

// MatchString 字符串匹配
func MatchString(value string, pattern string, op Operator) bool {
	switch op {
	case OpEqual:
		return value == pattern
	case OpNotEqual:
		return value != pattern
	case OpContains:
		return bytes.Contains([]byte(value), []byte(pattern))
	case OpNotContains:
		return !bytes.Contains([]byte(value), []byte(pattern))
	case OpRegex:
		matched, _ := regexp.MatchString(pattern, value)
		return matched
	default:
		return false
	}
}

// ParseThreshold 解析阈值（支持百分比）
func ParseThreshold(s string) (float64, error) {
	s = bytes.TrimSpace([]byte(s))

	// 处理百分比
	if len(s) > 0 && s[len(s)-1] == '%' {
		val, err := strconv.ParseFloat(string(s[:len(s)-1]), 64)
		if err != nil {
			return 0, err
		}
		return val / 100, nil
	}

	return strconv.ParseFloat(string(s), 64)
}

// bytes 包辅助函数（避免导入）
var bytes = struct {
	Contains  func([]byte, []byte) bool
	TrimSpace func([]byte) []byte
}{
	Contains: func(s, substr []byte) bool {
		return len(substr) == 0 || findSubstr(s, substr) >= 0
	},
	TrimSpace: func(s []byte) []byte {
		start := 0
		for start < len(s) && (s[start] == ' ' || s[start] == '\t' || s[start] == '\n' || s[start] == '\r') {
			start++
		}
		end := len(s)
		for end > start && (s[end-1] == ' ' || s[end-1] == '\t' || s[end-1] == '\n' || s[end-1] == '\r') {
			end--
		}
		return s[start:end]
	},
}

func findSubstr(s, substr []byte) int {
	for i := 0; i <= len(s)-len(substr); i++ {
		if string(s[i:i+len(substr)]) == string(substr) {
			return i
		}
	}
	return -1
}
