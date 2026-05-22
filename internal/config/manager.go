// Package config 提供配置管理功能
package config

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"

	"gopkg.in/yaml.v3"

	"github.com/meinanzilinzhengying/cloud-flow-agent/pkg/models"
)

// Manager 配置管理器
type Manager struct {
	configPath string
	config     *models.Config
	configHash string
	mu         sync.RWMutex

	// 热配置回调
	reloadCallbacks []func(*models.Config) error
}

// NewManager 创建配置管理器
func NewManager(configPath string) *Manager {
	return &Manager{
		configPath:      configPath,
		reloadCallbacks: make([]func(*models.Config) error, 0),
	}
}

// Load 加载配置
func (m *Manager) Load() (*models.Config, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	data, err := os.ReadFile(m.configPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read config file: %w", err)
	}

	// 计算配置哈希
	hash := sha256.Sum256(data)
	m.configHash = hex.EncodeToString(hash[:])

	config := &models.Config{}
	if err := yaml.Unmarshal(data, config); err != nil {
		return nil, fmt.Errorf("failed to parse config: %w", err)
	}

	// 设置默认值
	m.setDefaults(config)

	m.config = config
	return config, nil
}

// LoadFromBytes 从字节加载配置
func (m *Manager) LoadFromBytes(data []byte) (*models.Config, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	hash := sha256.Sum256(data)
	m.configHash = hex.EncodeToString(hash[:])

	config := &models.Config{}
	if err := yaml.Unmarshal(data, config); err != nil {
		return nil, fmt.Errorf("failed to parse config: %w", err)
	}

	m.setDefaults(config)
	m.config = config
	return config, nil
}

// Save 保存配置
func (m *Manager) Save(config *models.Config) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	data, err := yaml.Marshal(config)
	if err != nil {
		return fmt.Errorf("failed to marshal config: %w", err)
	}

	// 确保目录存在
	dir := filepath.Dir(m.configPath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("failed to create config directory: %w", err)
	}

	if err := os.WriteFile(m.configPath, data, 0644); err != nil {
		return fmt.Errorf("failed to write config file: %w", err)
	}

	// 更新哈希
	hash := sha256.Sum256(data)
	m.configHash = hex.EncodeToString(hash[:])
	m.config = config

	return nil
}

// Get 获取当前配置
func (m *Manager) Get() *models.Config {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.config
}

// GetHash 获取配置哈希
func (m *Manager) GetHash() string {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.configHash
}

// RegisterReloadCallback 注册热配置回调
func (m *Manager) RegisterReloadCallback(callback func(*models.Config) error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.reloadCallbacks = append(m.reloadCallbacks, callback)
}

// Reload 热加载配置
func (m *Manager) Reload(ctx context.Context) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	data, err := os.ReadFile(m.configPath)
	if err != nil {
		return fmt.Errorf("failed to read config file: %w", err)
	}

	hash := sha256.Sum256(data)
	newHash := hex.EncodeToString(hash[:])

	// 检查配置是否变化
	if newHash == m.configHash {
		return nil
	}

	config := &models.Config{}
	if err := yaml.Unmarshal(data, config); err != nil {
		return fmt.Errorf("failed to parse config: %w", err)
	}

	m.setDefaults(config)

	// 调用回调
	for _, callback := range m.reloadCallbacks {
		if err := callback(config); err != nil {
			return fmt.Errorf("reload callback failed: %w", err)
		}
	}

	m.config = config
	m.configHash = newHash

	return nil
}

// UpdateFromServer 从服务器更新配置
func (m *Manager) UpdateFromServer(data []byte, newHash string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	config := &models.Config{}
	if err := yaml.Unmarshal(data, config); err != nil {
		return fmt.Errorf("failed to parse config: %w", err)
	}

	m.setDefaults(config)

	// 调用回调
	for _, callback := range m.reloadCallbacks {
		if err := callback(config); err != nil {
			return fmt.Errorf("reload callback failed: %w", err)
		}
	}

	m.config = config
	m.configHash = newHash

	// 保存到文件
	if err := os.WriteFile(m.configPath, data, 0644); err != nil {
		return fmt.Errorf("failed to save config: %w", err)
	}

	return nil
}

// Watch 监听配置文件变化
func (m *Manager) Watch(ctx context.Context) <-chan error {
	errChan := make(chan error, 1)

	go func() {
		defer close(errChan)

		ticker := time.NewTicker(30 * time.Second)
		defer ticker.Stop()

		var lastModTime time.Time

		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				info, err := os.Stat(m.configPath)
				if err != nil {
					errChan <- fmt.Errorf("failed to stat config file: %w", err)
					continue
				}

				if info.ModTime().After(lastModTime) {
					lastModTime = info.ModTime()
					if err := m.Reload(ctx); err != nil {
						errChan <- err
					}
				}
			}
		}
	}()

	return errChan
}

// setDefaults 设置默认值
func (m *Manager) setDefaults(config *models.Config) {
	if config.Agent.Interval <= 0 {
		config.Agent.Interval = 10
	}

	if config.Edge.Port <= 0 {
		config.Edge.Port = 9090
	}

	if config.Edge.Timeout <= 0 {
		config.Edge.Timeout = 10
	}

	if config.Edge.RetryMax <= 0 {
		config.Edge.RetryMax = 5
	}

	if config.Edge.RetryDelay <= 0 {
		config.Edge.RetryDelay = 5
	}

	if config.Collectors.Metrics.Interval <= 0 {
		config.Collectors.Metrics.Interval = 10
	}

	if config.Collectors.EBPF.BufferSize <= 0 {
		config.Collectors.EBPF.BufferSize = 4096
	}

	if config.Collectors.EBPF.SampleRate <= 0 {
		config.Collectors.EBPF.SampleRate = 100
	}

	if config.Collectors.Traditional.ProcPath == "" {
		config.Collectors.Traditional.ProcPath = "/proc"
	}

	if config.Resources.CPUQuota <= 0 {
		config.Resources.CPUQuota = 1.0
	}

	if config.Resources.MemoryLimit <= 0 {
		config.Resources.MemoryLimit = 512
	}

	if config.Resources.BufferMaxSize <= 0 {
		config.Resources.BufferMaxSize = 100
	}

	if config.Resources.MaxGoroutines <= 0 {
		config.Resources.MaxGoroutines = 100
	}

	if config.Logging.Level == "" {
		config.Logging.Level = "info"
	}

	if config.Logging.Format == "" {
		config.Logging.Format = "json"
	}

	if config.Logging.Output == "" {
		config.Logging.Output = "stdout"
	}

	if config.Logging.MaxSize <= 0 {
		config.Logging.MaxSize = 100
	}

	if config.Logging.MaxBackups <= 0 {
		config.Logging.MaxBackups = 5
	}

	if config.Logging.MaxAge <= 0 {
		config.Logging.MaxAge = 30
	}

	// 双中心配置默认值
	if config.DualCenter.Enabled && config.DualCenter.LocalPort <= 0 {
		config.DualCenter.LocalPort = 7947
	}
	if config.DualCenter.Enabled && config.DualCenter.BatchSize <= 0 {
		config.DualCenter.BatchSize = 500
	}
	if config.DualCenter.Enabled && config.DualCenter.FlushInterval <= 0 {
		config.DualCenter.FlushInterval = 5
	}
	if config.DualCenter.Enabled && config.DualCenter.MaxRetries <= 0 {
		config.DualCenter.MaxRetries = 3
	}
	if config.DualCenter.Enabled && config.DualCenter.RetryDelay <= 0 {
		config.DualCenter.RetryDelay = 2
	}
	if config.DualCenter.Enabled && config.DualCenter.QueueSize <= 0 {
		config.DualCenter.QueueSize = 10000
	}
	if config.DualCenter.Enabled && config.DualCenter.QueueOverflow == "" {
		config.DualCenter.QueueOverflow = "drop_oldest"
	}
	if config.DualCenter.Enabled && config.DualCenter.SyncMode == "" {
		config.DualCenter.SyncMode = "async"
	}

	// 故障切换配置默认值
	if config.Failover.Enabled && config.Failover.HeartbeatInterval <= 0 {
		config.Failover.HeartbeatInterval = 3
	}
	if config.Failover.Enabled && config.Failover.HeartbeatTimeout <= 0 {
		config.Failover.HeartbeatTimeout = 2
	}
	if config.Failover.Enabled && config.Failover.FailureThreshold <= 0 {
		config.Failover.FailureThreshold = 3
	}
	if config.Failover.Enabled && config.Failover.SuccessThreshold <= 0 {
		config.Failover.SuccessThreshold = 5
	}
	if config.Failover.Enabled && config.Failover.SwitchTimeout <= 0 {
		config.Failover.SwitchTimeout = 30
	}
	if config.Failover.Enabled && config.Failover.PreSwitchDelay <= 0 {
		config.Failover.PreSwitchDelay = 2
	}
	if config.Failover.Enabled && config.Failover.DrainTimeout <= 0 {
		config.Failover.DrainTimeout = 60
	}
	if config.Failover.Enabled && config.Failover.RecoverDelay <= 0 {
		config.Failover.RecoverDelay = 30
	}
	if config.Failover.Enabled && config.Failover.FenceTimeout <= 0 {
		config.Failover.FenceTimeout = 10
	}
	if config.Failover.Enabled && config.Failover.Mode == "" {
		config.Failover.Mode = "auto"
	}

	// 弹性管理配置默认值
	if config.Resilience.Enabled && config.Resilience.HealthCheckInterval <= 0 {
		config.Resilience.HealthCheckInterval = 10
	}
	if config.Resilience.Enabled && config.Resilience.SwitchTimeoutSec <= 0 {
		config.Resilience.SwitchTimeoutSec = 30
	}
	if config.Resilience.Enabled && config.Resilience.CircuitBreaker.MaxFailures <= 0 {
		config.Resilience.CircuitBreaker.MaxFailures = 5
	}
	if config.Resilience.Enabled && config.Resilience.CircuitBreaker.ResetTimeoutSec <= 0 {
		config.Resilience.CircuitBreaker.ResetTimeoutSec = 30
	}
	if config.Resilience.Enabled && config.Resilience.CircuitBreaker.HalfOpenMax <= 0 {
		config.Resilience.CircuitBreaker.HalfOpenMax = 3
	}
	if config.Resilience.Enabled && config.Resilience.CircuitBreaker.WindowTimeSec <= 0 {
		config.Resilience.CircuitBreaker.WindowTimeSec = 60
	}
	if config.Resilience.Enabled && config.Resilience.CircuitBreaker.WindowBuckets <= 0 {
		config.Resilience.CircuitBreaker.WindowBuckets = 10
	}
	if config.Resilience.Enabled && config.Resilience.CircuitBreaker.TimeoutSec <= 0 {
		config.Resilience.CircuitBreaker.TimeoutSec = 10
	}

	// VXLAN 配置默认值
	if config.VXLAN.Enabled && config.VXLAN.ListenPort == 0 {
		config.VXLAN.ListenPort = 4789
	}
	if config.VXLAN.Enabled && config.VXLAN.BufferSize <= 0 {
		config.VXLAN.BufferSize = 65535
	}
	if config.VXLAN.Enabled && config.VXLAN.MaxPacketSize <= 0 {
		config.VXLAN.MaxPacketSize = 9000
	}

	// Mirror 配置默认值
	if config.Mirror.Enabled && config.Mirror.SourcePort == 0 {
		config.Mirror.SourcePort = 4789
	}
	if config.Mirror.Enabled && config.Mirror.QueueSize <= 0 {
		config.Mirror.QueueSize = 10000
	}
	if config.Mirror.Enabled && config.Mirror.BatchSize <= 0 {
		config.Mirror.BatchSize = 100
	}
	if config.Mirror.Enabled && config.Mirror.BatchTimeout <= 0 {
		config.Mirror.BatchTimeout = 100
	}
	if config.Mirror.Enabled && config.Mirror.Filter.SampleRate <= 0 {
		config.Mirror.Filter.SampleRate = 100
	}
}

// ToJSON 转换为 JSON
func (m *Manager) ToJSON() (string, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	if m.config == nil {
		return "", errors.New("config not loaded")
	}

	data, err := json.MarshalIndent(m.config, "", "  ")
	if err != nil {
		return "", err
	}

	return string(data), nil
}

// Validate 验证配置
func (m *Manager) Validate(config *models.Config) error {
	if config.Edge.Address == "" {
		return errors.New("edge address is required")
	}

	return nil
}

// GenerateExampleConfig 生成示例配置
func GenerateExampleConfig() *models.Config {
	return &models.Config{
		Agent: models.AgentConfig{
			Hostname: "localhost",
			HostIP:   "127.0.0.1",
			Interval: 10,
		},
		Edge: models.EdgeConfig{
			Address:    "edge.example.com",
			Port:       9090,
			TLSEnabled: false,
			Timeout:    10,
			RetryMax:   5,
			RetryDelay: 5,
		},
		Collectors: models.CollectorsConfig{
			EBPF: models.EBPFCollectorConfig{
				Enabled:     true,
				Events:      []string{"tcp_connect", "tcp_accept", "tcp_close"},
				SampleRate:  100,
				BufferSize:  4096,
			},
			Traditional: models.TraditionalCollectorConfig{
				Enabled:  true,
				ProcPath: "/proc",
			},
			Metrics: models.MetricsCollectorConfig{
				Enabled:   true,
				Interval:  10,
				CPU:       true,
				Memory:    true,
				Disk:      true,
				DiskPaths: []string{"/"},
				Network:   true,
			},
			Process: models.ProcessCollectorConfig{
				Enabled: true,
				Events:  []string{"exec", "fork", "exit"},
			},
		},
		Resources: models.ResourceConfig{
			CPUQuota:      1.0,
			MemoryLimit:   512,
			BufferMaxSize: 100,
			MaxGoroutines: 100,
		},
		Logging: models.LoggingConfig{
			Level:      "info",
			Format:     "json",
			Output:     "stdout",
			MaxSize:    100,
			MaxBackups: 5,
			MaxAge:     30,
			Compress:   true,
		},
	}
}
