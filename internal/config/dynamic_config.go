/*
 * Copyright (c) 2025 Yunlong Liao. All rights reserved.
 */

package config

import (
	"context"
	"fmt"
	"os"
	"reflect"
	"regexp"
	"strings"
	"sync"
	"time"

	"cloud-flow-agent/pkg/logger"
	"github.com/fsnotify/fsnotify"
)

// ConfigSource 配置来源
type ConfigSource string

const (
	ConfigSourceFile   ConfigSource = "file"
	ConfigSourceEnv    ConfigSource = "env"
	ConfigSourceConsul ConfigSource = "consul"
	ConfigSourceEtcd   ConfigSource = "etcd"
	ConfigSourceNacos  ConfigSource = "nacos"
)

// DynamicConfigManager 动态配置管理器
type DynamicConfigManager struct {
	sources      map[ConfigSource]ConfigProvider
	values       map[string]interface{}
	watchers     []ConfigChangeHandler
	mu           sync.RWMutex
	stopCh       chan struct{}
}

// ConfigProvider 配置提供者接口
type ConfigProvider interface {
	Get(key string) (interface{}, error)
	GetString(key string) (string, error)
	GetInt(key string) (int, error)
	GetBool(key string) (bool, error)
	Watch(ctx context.Context, callback ConfigChangeCallback) error
	Close() error
}

// ConfigChangeCallback 配置变更回调
type ConfigChangeCallback func(key string, oldVal, newVal interface{})

// ConfigChangeHandler 配置变更处理器
type ConfigChangeHandler func(event *ConfigChangeEvent)

// ConfigChangeEvent 配置变更事件
type ConfigChangeEvent struct {
	Key       string
	OldValue  interface{}
	NewValue  interface{}
	Source    ConfigSource
	Timestamp time.Time
}

// NewDynamicConfigManager 创建动态配置管理器
func NewDynamicConfigManager() *DynamicConfigManager {
	return &DynamicConfigManager{
		sources:  make(map[ConfigSource]ConfigProvider),
		values:   make(map[string]interface{}),
		watchers: make([]ConfigChangeHandler, 0),
		stopCh:   make(chan struct{}),
	}
}

// RegisterProvider 注册配置提供者
func (m *DynamicConfigManager) RegisterProvider(source ConfigSource, provider ConfigProvider) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.sources[source] = provider
	logger.Infof("Registered config provider: %s", source)
}

// Get 获取配置值（按优先级：环境变量 > 配置中心 > 配置文件）
func (m *DynamicConfigManager) Get(key string) (interface{}, error) {
	// 1. 首先检查环境变量
	if val := os.Getenv(key); val != "" {
		return val, nil
	}

	// 2. 检查配置中心
	m.mu.RLock()
	defer m.mu.RUnlock()

	for _, source := range []ConfigSource{ConfigSourceConsul, ConfigSourceEtcd, ConfigSourceNacos} {
		if provider, exists := m.sources[source]; exists {
			if val, err := provider.Get(key); err == nil && val != nil {
				return val, nil
			}
		}
	}

	// 3. 检查本地缓存
	if val, exists := m.values[key]; exists {
		return val, nil
	}

	return nil, fmt.Errorf("config key not found: %s", key)
}

// GetString 获取字符串配置
func (m *DynamicConfigManager) GetString(key string) string {
	val, err := m.Get(key)
	if err != nil {
		return ""
	}
	return fmt.Sprintf("%v", val)
}

// GetInt 获取整数配置
func (m *DynamicConfigManager) GetInt(key string) int {
	val, err := m.Get(key)
	if err != nil {
		return 0
	}

	switch v := val.(type) {
	case int:
		return v
	case int64:
		return int(v)
	case float64:
		return int(v)
	case string:
		var result int
		fmt.Sscanf(v, "%d", &result)
		return result
	default:
		return 0
	}
}

// GetBool 获取布尔配置
func (m *DynamicConfigManager) GetBool(key string) bool {
	val, err := m.Get(key)
	if err != nil {
		return false
	}

	switch v := val.(type) {
	case bool:
		return v
	case string:
		return strings.ToLower(v) == "true" || v == "1"
	default:
		return false
	}
}

// Set 设置配置值
func (m *DynamicConfigManager) Set(key string, value interface{}) {
	m.mu.Lock()
	defer m.mu.Unlock()

	oldVal := m.values[key]
	m.values[key] = value

	// 触发变更事件
	if !reflect.DeepEqual(oldVal, value) {
		m.notifyWatchers(&ConfigChangeEvent{
			Key:       key,
			OldValue:  oldVal,
			NewValue:  value,
			Timestamp: time.Now(),
		})
	}
}

// Watch 监听配置变更
func (m *DynamicConfigManager) Watch(handler ConfigChangeHandler) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.watchers = append(m.watchers, handler)
}

// notifyWatchers 通知所有监听者
func (m *DynamicConfigManager) notifyWatchers(event *ConfigChangeEvent) {
	m.mu.RLock()
	watchers := make([]ConfigChangeHandler, len(m.watchers))
	copy(watchers, m.watchers)
	m.mu.RUnlock()

	for _, handler := range watchers {
		go handler(event)
	}
}

// LoadFromEnv 从环境变量加载配置
func (m *DynamicConfigManager) LoadFromEnv(prefix string) {
	for _, env := range os.Environ() {
		pair := strings.SplitN(env, "=", 2)
		if len(pair) != 2 {
			continue
		}

		key := pair[0]
		value := pair[1]

		// 检查前缀
		if prefix != "" && !strings.HasPrefix(key, prefix) {
			continue
		}

		// 移除前缀并转换格式
		configKey := strings.TrimPrefix(key, prefix)
		configKey = strings.ToLower(configKey)
		configKey = strings.ReplaceAll(configKey, "_", ".")

		m.Set(configKey, value)
	}

	logger.Infof("Loaded config from environment variables with prefix: %s", prefix)
}

// ExpandEnvVars 展开配置中的环境变量
func ExpandEnvVars(input string) string {
	// 支持 ${VAR} 和 $VAR 格式
	re := regexp.MustCompile(`\$\{(\w+)\}|\$(\w+)`)
	return re.ReplaceAllStringFunc(input, func(match string) string {
		// 提取变量名
		var varName string
		if strings.HasPrefix(match, "${") {
			varName = match[2 : len(match)-1]
		} else {
			varName = match[1:]
		}

		// 获取环境变量值
		if val := os.Getenv(varName); val != "" {
			return val
		}
		return match
	})
}

// EnvConfigProvider 环境变量配置提供者
type EnvConfigProvider struct {
	prefix string
}

// NewEnvConfigProvider 创建环境变量配置提供者
func NewEnvConfigProvider(prefix string) *EnvConfigProvider {
	return &EnvConfigProvider{prefix: prefix}
}

// Get 获取配置
func (p *EnvConfigProvider) Get(key string) (interface{}, error) {
	envKey := p.toEnvKey(key)
	val := os.Getenv(envKey)
	if val == "" {
		return nil, fmt.Errorf("env var not found: %s", envKey)
	}
	return val, nil
}

// GetString 获取字符串
func (p *EnvConfigProvider) GetString(key string) (string, error) {
	val, err := p.Get(key)
	if err != nil {
		return "", err
	}
	return fmt.Sprintf("%v", val), nil
}

// GetInt 获取整数
func (p *EnvConfigProvider) GetInt(key string) (int, error) {
	val, err := p.GetString(key)
	if err != nil {
		return 0, err
	}

	var result int
	_, err = fmt.Sscanf(val, "%d", &result)
	return result, err
}

// GetBool 获取布尔值
func (p *EnvConfigProvider) GetBool(key string) (bool, error) {
	val, err := p.GetString(key)
	if err != nil {
		return false, err
	}
	return strings.ToLower(val) == "true" || val == "1", nil
}

// Watch 监听（环境变量不支持实时监听）
func (p *EnvConfigProvider) Watch(ctx context.Context, callback ConfigChangeCallback) error {
	// 环境变量不支持动态监听
	return nil
}

// Close 关闭
func (p *EnvConfigProvider) Close() error {
	return nil
}

// toEnvKey 转换为环境变量键
func (p *EnvConfigProvider) toEnvKey(key string) string {
	envKey := strings.ToUpper(key)
	envKey = strings.ReplaceAll(envKey, ".", "_")
	if p.prefix != "" {
		envKey = p.prefix + "_" + envKey
	}
	return envKey
}

// FileConfigProvider 文件配置提供者
type FileConfigProvider struct {
	path     string
	watcher  *fsnotify.Watcher
	stopCh   chan struct{}
}

// NewFileConfigProvider 创建文件配置提供者
func NewFileConfigProvider(path string) (*FileConfigProvider, error) {
	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		return nil, fmt.Errorf("failed to create watcher: %w", err)
	}

	return &FileConfigProvider{
		path:    path,
		watcher: watcher,
		stopCh:  make(chan struct{}),
	}, nil
}

// Get 获取配置（从文件读取）
func (p *FileConfigProvider) Get(key string) (interface{}, error) {
	// 这里应该解析YAML文件并返回对应值
	// 简化实现
	return nil, fmt.Errorf("not implemented")
}

// GetString 获取字符串
func (p *FileConfigProvider) GetString(key string) (string, error) {
	val, err := p.Get(key)
	if err != nil {
		return "", err
	}
	return fmt.Sprintf("%v", val), nil
}

// GetInt 获取整数
func (p *FileConfigProvider) GetInt(key string) (int, error) {
	return 0, fmt.Errorf("not implemented")
}

// GetBool 获取布尔值
func (p *FileConfigProvider) GetBool(key string) (bool, error) {
	return false, fmt.Errorf("not implemented")
}

// Watch 监听文件变更
func (p *FileConfigProvider) Watch(ctx context.Context, callback ConfigChangeCallback) error {
	if err := p.watcher.Add(p.path); err != nil {
		return fmt.Errorf("failed to watch file: %w", err)
	}

	go func() {
		for {
			select {
			case <-ctx.Done():
				return
			case <-p.stopCh:
				return
			case event, ok := <-p.watcher.Events:
				if !ok {
					return
				}
				if event.Op&fsnotify.Write == fsnotify.Write {
					logger.Infof("Config file modified: %s", event.Name)
					// 触发重新加载
					if callback != nil {
						callback("file", nil, nil)
					}
				}
			case err, ok := <-p.watcher.Errors:
				if !ok {
					return
				}
				logger.Errorf("Watcher error: %v", err)
			}
		}
	}()

	return nil
}

// Close 关闭
func (p *FileConfigProvider) Close() error {
	close(p.stopCh)
	return p.watcher.Close()
}

// ConsulConfigProvider Consul配置中心提供者
type ConsulConfigProvider struct {
	address string
	prefix  string
	client  interface{} // 简化，实际应为 consul.Client
}

// NewConsulConfigProvider 创建Consul配置提供者
func NewConsulConfigProvider(address, prefix string) *ConsulConfigProvider {
	return &ConsulConfigProvider{
		address: address,
		prefix:  prefix,
	}
}

// Get 获取配置
func (p *ConsulConfigProvider) Get(key string) (interface{}, error) {
	// 实际实现应调用Consul API
	return nil, fmt.Errorf("consul provider not fully implemented")
}

// GetString 获取字符串
func (p *ConsulConfigProvider) GetString(key string) (string, error) {
	val, err := p.Get(key)
	if err != nil {
		return "", err
	}
	return fmt.Sprintf("%v", val), nil
}

// GetInt 获取整数
func (p *ConsulConfigProvider) GetInt(key string) (int, error) {
	return 0, fmt.Errorf("not implemented")
}

// GetBool 获取布尔值
func (p *ConsulConfigProvider) GetBool(key string) (bool, error) {
	return false, fmt.Errorf("not implemented")
}

// Watch 监听Consul配置变更
func (p *ConsulConfigProvider) Watch(ctx context.Context, callback ConfigChangeCallback) error {
	// 实际实现应使用Consul的Watch机制
	return nil
}

// Close 关闭
func (p *ConsulConfigProvider) Close() error {
	return nil
}

// ConfigLoader 配置加载器
type ConfigLoader struct {
	manager *DynamicConfigManager
}

// NewConfigLoader 创建配置加载器
func NewConfigLoader() *ConfigLoader {
	return &ConfigLoader{
		manager: NewDynamicConfigManager(),
	}
}

// Load 加载配置
func (l *ConfigLoader) Load(configPath string) error {
	// 1. 加载配置文件
	if err := l.loadFromFile(configPath); err != nil {
		logger.Warnf("Failed to load config file: %v", err)
	}

	// 2. 加载环境变量
	l.manager.LoadFromEnv("CLOUD_FLOW")

	// 3. 尝试连接配置中心
	if err := l.loadFromConfigCenter(); err != nil {
		logger.Warnf("Failed to load from config center: %v", err)
	}

	return nil
}

// loadFromFile 从文件加载
func (l *ConfigLoader) loadFromFile(path string) error {
	provider, err := NewFileConfigProvider(path)
	if err != nil {
		return err
	}

	l.manager.RegisterProvider(ConfigSourceFile, provider)
	return nil
}

// loadFromConfigCenter 从配置中心加载
func (l *ConfigLoader) loadFromConfigCenter() error {
	// 检查环境变量确定使用哪个配置中心
	if consulAddr := os.Getenv("CONSUL_ADDR"); consulAddr != "" {
		provider := NewConsulConfigProvider(consulAddr, "/cloud-flow/config")
		l.manager.RegisterProvider(ConfigSourceConsul, provider)
		logger.Info("Registered Consul config provider")
	}

	return nil
}

// GetManager 获取配置管理器
func (l *ConfigLoader) GetManager() *DynamicConfigManager {
	return l.manager
}

// HotReloadConfig 热重载配置
type HotReloadConfig struct {
	Enabled      bool          `yaml:"enabled" json:"enabled"`
	Interval     time.Duration `yaml:"interval" json:"interval"`
	WatchFile    bool          `yaml:"watch_file" json:"watch_file"`
	WatchEnv     bool          `yaml:"watch_env" json:"watch_env"`
	WatchCenter  bool          `yaml:"watch_center" json:"watch_center"`
}

// DefaultHotReloadConfig 默认热重载配置
func DefaultHotReloadConfig() *HotReloadConfig {
	return &HotReloadConfig{
		Enabled:     true,
		Interval:    30 * time.Second,
		WatchFile:   true,
		WatchEnv:    false,
		WatchCenter: true,
	}
}
