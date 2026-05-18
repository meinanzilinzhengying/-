// Package alerting 提供告警规则引擎和通知功能
package alerting

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"

	"cloud-flow-center/pkg/logger"
)

// Duration 是对 time.Duration 的包装，支持 "5m"、"1h" 等字符串格式的 JSON 序列化/反序列化
type Duration struct {
	time.Duration
}

// UnmarshalJSON 实现自定义 JSON 反序列化，支持字符串格式（如 "5m"、"1h"、"30s"）和数字格式（纳秒）
func (d *Duration) UnmarshalJSON(data []byte) error {
	var v interface{}
	if err := json.Unmarshal(data, &v); err != nil {
		return err
	}
	switch value := v.(type) {
	case float64:
		d.Duration = time.Duration(value)
		return nil
	case string:
		parsed, err := time.ParseDuration(value)
		if err != nil {
			return fmt.Errorf("无法解析 duration 字符串 %q: %w", value, err)
		}
		d.Duration = parsed
		return nil
	default:
		return fmt.Errorf("duration 字段不支持类型 %T，请使用字符串（如 \"5m\"）或数字（纳秒）", v)
	}
}

// MarshalJSON 实现自定义 JSON 序列化，输出为字符串格式（如 "5m0s"）
func (d Duration) MarshalJSON() ([]byte, error) {
	return json.Marshal(d.Duration.String())
}

// RuleType 规则类型
type RuleType string

const (
	RuleTypeCPU      RuleType = "cpu"
	RuleTypeMemory   RuleType = "memory"
	RuleTypeNetwork  RuleType = "network"
	RuleTypeDisk     RuleType = "disk"
	RuleTypeTraffic  RuleType = "traffic"
)

// ConditionOperator 条件操作符
type ConditionOperator string

const (
	OperatorGreaterThan    ConditionOperator = ">"
	OperatorLessThan       ConditionOperator = "<"
	OperatorGreaterOrEqual ConditionOperator = ">="
	OperatorLessOrEqual    ConditionOperator = "<="
	OperatorEqual          ConditionOperator = "="
	OperatorNotEqual       ConditionOperator = "!="
)

// Rule 告警规则
type Rule struct {
	ID          string            `json:"id"`
	Name        string            `json:"name"`
	Description string            `json:"description"`
	Type        RuleType          `json:"type"`
	Enabled     bool              `json:"enabled"`
	Condition   Condition         `json:"condition"`
	Threshold   float64           `json:"threshold"`
	Duration    Duration          `json:"duration"`
	Severity    string            `json:"severity"`
	Labels      map[string]string `json:"labels"`
	// 持续触发阈值，范围 0.0-1.0，表示在时间窗口内满足条件的指标比例
	// 例如：0.8 表示 80% 的指标满足条件时触发告警
	// 默认值为 1.0（100% 满足才触发）
	SatisfyThreshold float64       `json:"satisfy_threshold"`
	CreatedAt        time.Time     `json:"created_at"`
	UpdatedAt        time.Time     `json:"updated_at"`
}

// Condition 规则条件
type Condition struct {
	Metric    string            `json:"metric"`
	Operator  ConditionOperator `json:"operator"`
	Threshold float64           `json:"threshold"`
}

// RuleManager 规则管理器
type RuleManager struct {
	mu     sync.RWMutex
	rules  map[string]*Rule
	dir    string
	logger *logger.Logger
}

// NewRuleManager 创建规则管理器
func NewRuleManager(ruleDir string, log *logger.Logger) *RuleManager {
	if err := os.MkdirAll(ruleDir, 0755); err != nil {
		log.Warnf("创建规则目录 %s 失败: %v", ruleDir, err)
	}
	return &RuleManager{
		rules:  make(map[string]*Rule),
		dir:    ruleDir,
		logger: log,
	}
}

// LoadRules 加载规则
func (rm *RuleManager) LoadRules() error {
	entries, err := os.ReadDir(rm.dir)
	if err != nil {
		return fmt.Errorf("读取规则目录失败: %w", err)
	}

	for _, entry := range entries {
		if !entry.IsDir() && filepath.Ext(entry.Name()) == ".json" {
			path := filepath.Join(rm.dir, entry.Name())
			data, err := os.ReadFile(path)
			if err != nil {
				rm.logger.Warnf("读取规则文件 %s 失败: %v", path, err)
				continue
			}

			var rule Rule
			if err := json.Unmarshal(data, &rule); err != nil {
				rm.logger.Warnf("解析规则文件 %s 失败: %v", path, err)
				continue
			}

			rm.mu.Lock()
			rm.rules[rule.ID] = &rule
			rm.mu.Unlock()
		}
	}

	// 如果没有规则，创建默认规则
	if len(rm.rules) == 0 {
		rm.logger.Info("未找到规则文件，创建默认规则...")
		defaultRules := DefaultRules()
		for _, rule := range defaultRules {
			if err := rm.SaveRule(rule); err != nil {
				rm.logger.Warnf("保存默认规则 %s 失败: %v", rule.Name, err)
			}
		}
		rm.logger.Infof("创建了 %d 个默认规则", len(defaultRules))
	} else {
		rm.logger.Infof("加载了 %d 个规则", len(rm.rules))
	}

	return nil
}

// SaveRule 保存规则
func (rm *RuleManager) SaveRule(rule *Rule) error {
	rule.UpdatedAt = time.Now()
	if rule.CreatedAt.IsZero() {
		rule.CreatedAt = rule.UpdatedAt
	}

	path := filepath.Join(rm.dir, rule.ID+".json")
	data, err := json.MarshalIndent(rule, "", "  ")
	if err != nil {
		return fmt.Errorf("序列化规则失败: %w", err)
	}

	if err := os.WriteFile(path, data, 0644); err != nil {
		return fmt.Errorf("写入规则文件失败: %w", err)
	}

	rm.mu.Lock()
	rm.rules[rule.ID] = rule
	rm.mu.Unlock()

	rm.logger.Infof("保存规则: %s", rule.Name)
	return nil
}

// GetRules 获取所有规则
func (rm *RuleManager) GetRules() []*Rule {
	rm.mu.RLock()
	defer rm.mu.RUnlock()

	result := make([]*Rule, 0, len(rm.rules))
	for _, rule := range rm.rules {
		result = append(result, rule)
	}
	return result
}

// GetRuleByID 根据 ID 获取规则
func (rm *RuleManager) GetRuleByID(id string) *Rule {
	rm.mu.RLock()
	defer rm.mu.RUnlock()
	return rm.rules[id]
}

// DeleteRule 删除规则
func (rm *RuleManager) DeleteRule(id string) error {
	path := filepath.Join(rm.dir, id+".json")
	if err := os.Remove(path); err != nil {
		return fmt.Errorf("删除规则文件失败: %w", err)
	}

	rm.mu.Lock()
	delete(rm.rules, id)
	rm.mu.Unlock()

	rm.logger.Infof("删除规则: %s", id)
	return nil
}
