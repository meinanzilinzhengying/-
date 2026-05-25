//go:build linux

// Package nacos 提供配置管理器，支持热更新和环境隔离
package nacos

import (
	"context"
	"encoding/json"
	"fmt"
	"reflect"
	"sync"
	"time"

	"gopkg.in/yaml.v3"
)

// ConfigManager 配置管理器
type ConfigManager struct {
	client      *Client
	env         string // dev/test/prod
	appName     string

	// 配置缓存
	configs     map[string]interface{}
	configsMu   sync.RWMutex

	// 热更新处理器
	handlers    map[string][]UpdateHandler
	handlersMu  sync.RWMutex

	// 结构化配置映射
	structCache map[string]interface{}
	structMu    sync.RWMutex
}

// UpdateHandler 配置更新处理器
type UpdateHandler func(oldVal, newVal interface{})

// ConfigMetadata 配置元数据
type ConfigMetadata struct {
	DataId      string            `json:"dataId"`
	Group       string            `json:"group"`
	Format      string            `json:"format"` // json/yaml/properties
	Description string            `json:"description"`
	Required    bool              `json:"required"`
	DefaultVal  string            `json:"defaultVal"`
	EnvOverride map[string]string `json:"envOverride"` // 环境特定覆盖值
}

// NewConfigManager 创建配置管理器
func NewConfigManager(client *Client) (*ConfigManager, error) {
	if client == nil {
		return nil, fmt.Errorf("nacos client is required")
	}

	cm := &ConfigManager{
		client:      client,
		env:         client.config.AppEnv,
		appName:     client.config.AppName,
		configs:     make(map[string]interface{}),
		handlers:    make(map[string][]UpdateHandler),
		structCache: make(map[string]interface{}),
	}

	return cm, nil
}

// ==================== 配置加载 ====================

// LoadConfig 加载配置到结构体
func (cm *ConfigManager) LoadConfig(dataId string, v interface{}) error {
	// 获取环境特定配置名
	envDataId := cm.getEnvConfigName(dataId)

	// 尝试加载环境特定配置
	content, err := cm.client.GetConfig(envDataId)
	if err != nil {
		// 回退到默认配置
		content, err = cm.client.GetConfig(dataId)
		if err != nil {
			return fmt.Errorf("failed to load config %s: %w", dataId, err)
		}
	}

	// 解析配置
	format := detectFormat(content)
	if err := parseConfig(content, format, v); err != nil {
		return fmt.Errorf("failed to parse config %s: %w", dataId, err)
	}

	// 缓存配置
	cm.structMu.Lock()
	cm.structCache[dataId] = v
	cm.structMu.Unlock()

	return nil
}

// LoadConfigWithDefault 加载配置，如果不存在则使用默认值
func (cm *ConfigManager) LoadConfigWithDefault(dataId string, v interface{}, defaultConfig string) error {
	err := cm.LoadConfig(dataId, v)
	if err != nil {
		// 使用默认值
		format := detectFormat(defaultConfig)
		if parseErr := parseConfig(defaultConfig, format, v); parseErr != nil {
			return parseErr
		}

		// 发布默认配置到 Nacos
		_ = cm.client.PublishConfig(dataId, defaultConfig)
	}
	return nil
}

// ==================== 热更新 ====================

// WatchConfig 监听配置变更
func (cm *ConfigManager) WatchConfig(dataId string, v interface{}, handler UpdateHandler) error {
	// 保存初始值用于比较
	cm.structMu.RLock()
	oldVal := cm.structCache[dataId]
	cm.structMu.RUnlock()

	// 注册监听器
	listener := func(dataId, group, content string) {
		// 解析新配置
		format := detectFormat(content)
		newVal := reflect.New(reflect.TypeOf(v).Elem()).Interface()
		if err := parseConfig(content, format, newVal); err != nil {
			fmt.Printf("[ConfigManager] Failed to parse updated config %s: %v\n", dataId, err)
			return
		}

		// 更新缓存
		cm.structMu.Lock()
		cm.structCache[dataId] = newVal
		cm.structMu.Unlock()

		// 调用处理器
		if handler != nil {
			handler(oldVal, newVal)
		}

		// 更新旧值
		oldVal = newVal

		fmt.Printf("[ConfigManager] Config %s updated successfully\n", dataId)
	}

	if err := cm.client.ListenConfig(dataId, listener); err != nil {
		return err
	}

	// 保存处理器
	cm.handlersMu.Lock()
	cm.handlers[dataId] = append(cm.handlers[dataId], handler)
	cm.handlersMu.Unlock()

	return nil
}

// WatchConfigs 批量监听配置
func (cm *ConfigManager) WatchConfigs(configs map[string]interface{}, handler UpdateHandler) error {
	for dataId, v := range configs {
		if err := cm.WatchConfig(dataId, v, handler); err != nil {
			return err
		}
	}
	return nil
}

// UnwatchConfig 停止监听配置
func (cm *ConfigManager) UnwatchConfig(dataId string) error {
	cm.handlersMu.Lock()
	delete(cm.handlers, dataId)
	cm.handlersMu.Unlock()

	return cm.client.StopListenConfig(dataId)
}

// ==================== 配置获取 ====================

// GetConfig 获取配置值
func (cm *ConfigManager) GetConfig(key string) (interface{}, bool) {
	cm.configsMu.RLock()
	defer cm.configsMu.RUnlock()
	val, ok := cm.configs[key]
	return val, ok
}

// GetConfigString 获取字符串配置
func (cm *ConfigManager) GetConfigString(key string) string {
	val, ok := cm.GetConfig(key)
	if !ok {
		return ""
	}
	switch v := val.(type) {
	case string:
		return v
	default:
		return fmt.Sprintf("%v", v)
	}
}

// GetConfigInt 获取整数配置
func (cm *ConfigManager) GetConfigInt(key string) int {
	val, ok := cm.GetConfig(key)
	if !ok {
		return 0
	}
	switch v := val.(type) {
	case int:
		return v
	case int64:
		return int(v)
	case float64:
		return int(v)
	default:
		return 0
	}
}

// GetConfigBool 获取布尔配置
func (cm *ConfigManager) GetConfigBool(key string) bool {
	val, ok := cm.GetConfig(key)
	if !ok {
		return false
	}
	switch v := val.(type) {
	case bool:
		return v
	case string:
		return v == "true" || v == "1" || v == "yes"
	default:
		return false
	}
}

// SetConfig 设置配置值（本地缓存，不持久化）
func (cm *ConfigManager) SetConfig(key string, value interface{}) {
	cm.configsMu.Lock()
	cm.configs[key] = value
	cm.configsMu.Unlock()
}

// ==================== 结构化配置 ====================

// GetStructConfig 获取结构化配置
func (cm *ConfigManager) GetStructConfig(dataId string) (interface{}, bool) {
	cm.structMu.RLock()
	defer cm.structMu.RUnlock()
	val, ok := cm.structCache[dataId]
	return val, ok
}

// RefreshConfig 刷新配置
func (cm *ConfigManager) RefreshConfig(dataId string, v interface{}) error {
	return cm.LoadConfig(dataId, v)
}

// RefreshAll 刷新所有配置
func (cm *ConfigManager) RefreshAll() error {
	cm.structMu.RLock()
	configs := make(map[string]interface{})
	for k, v := range cm.structCache {
		configs[k] = v
	}
	cm.structMu.RUnlock()

	var errs []error
	for dataId, v := range configs {
		if err := cm.RefreshConfig(dataId, v); err != nil {
			errs = append(errs, fmt.Errorf("refresh %s failed: %w", dataId, err))
		}
	}

	if len(errs) > 0 {
		return fmt.Errorf("refresh errors: %v", errs)
	}
	return nil
}

// ==================== 环境相关 ====================

// GetEnv 获取当前环境
func (cm *ConfigManager) GetEnv() string {
	return cm.env
}

// IsDev 是否是开发环境
func (cm *ConfigManager) IsDev() bool {
	return cm.env == "dev" || cm.env == "development"
}

// IsTest 是否是测试环境
func (cm *ConfigManager) IsTest() bool {
	return cm.env == "test" || cm.env == "testing"
}

// IsProd 是否是生产环境
func (cm *ConfigManager) IsProd() bool {
	return cm.env == "prod" || cm.env == "production"
}

// getEnvConfigName 获取环境特定配置名
func (cm *ConfigManager) getEnvConfigName(dataId string) string {
	if cm.env == "" || cm.env == "default" {
		return dataId
	}
	// 格式: appName-dataId-env
	return fmt.Sprintf("%s-%s-%s", cm.appName, dataId, cm.env)
}

// ==================== 配置发布 ====================

// PublishEnvConfig 发布环境特定配置
func (cm *ConfigManager) PublishEnvConfig(dataId string, v interface{}) error {
	envDataId := cm.getEnvConfigName(dataId)
	return cm.client.PublishConfigJSON(envDataId, v)
}

// PublishSharedConfig 发布共享配置
func (cm *ConfigManager) PublishSharedConfig(dataId string, v interface{}) error {
	return cm.client.PublishConfigJSON(dataId, v)
}

// ==================== 批量操作 ====================

// BatchLoad 批量加载配置
func (cm *ConfigManager) BatchLoad(configs map[string]interface{}) error {
	var errs []error
	for dataId, v := range configs {
		if err := cm.LoadConfig(dataId, v); err != nil {
			errs = append(errs, fmt.Errorf("load %s failed: %w", dataId, err))
		}
	}
	if len(errs) > 0 {
		return fmt.Errorf("batch load errors: %v", errs)
	}
	return nil
}

// BatchWatch 批量监听配置
func (cm *ConfigManager) BatchWatch(configs map[string]interface{}, handler UpdateHandler) error {
	var errs []error
	for dataId, v := range configs {
		if err := cm.WatchConfig(dataId, v, handler); err != nil {
			errs = append(errs, fmt.Errorf("watch %s failed: %w", dataId, err))
		}
	}
	if len(errs) > 0 {
		return fmt.Errorf("batch watch errors: %v", errs)
	}
	return nil
}

// ==================== 工具函数 ====================

func detectFormat(content string) string {
	content = trimBOM(content)
	trimmed := trimSpace(content)

	if len(trimmed) == 0 {
		return "properties"
	}

	// 检测 JSON
	if (trimmed[0] == '{' && trimmed[len(trimmed)-1] == '}') ||
		(trimmed[0] == '[' && trimmed[len(trimmed)-1] == ']') {
		return "json"
	}

	// 检测 YAML
	if trimmed[0] == '-' || contains(trimmed, ":\n") || contains(trimmed, ": ") {
		return "yaml"
	}

	return "properties"
}

func parseConfig(content, format string, v interface{}) error {
	switch format {
	case "json":
		return json.Unmarshal([]byte(content), v)
	case "yaml", "yml":
		return yaml.Unmarshal([]byte(content), v)
	case "properties":
		// 简化的 properties 解析
		return parseProperties(content, v)
	default:
		return json.Unmarshal([]byte(content), v)
	}
}

func parseProperties(content string, v interface{}) error {
	// 将 properties 转换为 map
	result := make(map[string]string)
	lines := splitLines(content)
	for _, line := range lines {
		line = trimSpace(line)
		if line == "" || startsWith(line, "#") {
			continue
		}
		parts := splitN(line, "=", 2)
		if len(parts) == 2 {
			key := trimSpace(parts[0])
			value := trimSpace(parts[1])
			value = trim(value, `"'`)
			result[key] = value
		}
	}

	// 将 map 转换为 JSON 再解析到结构体
	jsonData, err := json.Marshal(result)
	if err != nil {
		return err
	}
	return json.Unmarshal(jsonData, v)
}

// 简单的字符串处理函数（避免依赖）
func trimSpace(s string) string {
	return trimLeft(trimRight(s, " \t\n\r"), " \t\n\r")
}

func trimLeft(s, cutset string) string {
	for len(s) > 0 && containsChar(cutset, s[0]) {
		s = s[1:]
	}
	return s
}

func trimRight(s, cutset string) string {
	for len(s) > 0 && containsChar(cutset, s[len(s)-1]) {
		s = s[:len(s)-1]
	}
	return s
}

func trim(s, cutset string) string {
	return trimLeft(trimRight(s, cutset), cutset)
}

func contains(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}

func containsChar(s string, c byte) bool {
	for i := 0; i < len(s); i++ {
		if s[i] == c {
			return true
		}
	}
	return false
}

func startsWith(s, prefix string) bool {
	return len(s) >= len(prefix) && s[:len(prefix)] == prefix
}

func splitLines(s string) []string {
	var result []string
	start := 0
	for i := 0; i < len(s); i++ {
		if s[i] == '\n' {
			result = append(result, s[start:i])
			start = i + 1
		}
	}
	if start < len(s) {
		result = append(result, s[start:])
	}
	return result
}

func splitN(s, sep string, n int) []string {
	if n <= 0 {
		return []string{s}
	}
	var result []string
	start := 0
	for i := 0; i < len(s)-len(sep)+1 && len(result) < n-1; i++ {
		if s[i:i+len(sep)] == sep {
			result = append(result, s[start:i])
			start = i + len(sep)
		}
	}
	result = append(result, s[start:])
	return result
}

func trimBOM(s string) string {
	if len(s) >= 3 && s[0] == 0xEF && s[1] == 0xBB && s[2] == 0xBF {
		return s[3:]
	}
	return s
}
