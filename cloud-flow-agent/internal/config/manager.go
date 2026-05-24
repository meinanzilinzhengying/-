// Package config 提供远程配置管理功能
//
// 支持配置热更新，无需重启Agent
// 采用监听器模式，各模块可订阅配置变更事件
package config

import (
	"context"
	"fmt"
	"sync"
	"sync/atomic"
	"time"

	"cloud-flow-agent/pkg/logger"
	edge "cloud-flow/proto"
)

// ConfigListener 配置变更监听器接口
type ConfigListener interface {
	// OnConfigUpdate 配置更新回调
	// oldCfg: 旧配置（可能为nil）
	// newCfg: 新配置
	OnConfigUpdate(oldCfg, newCfg *CollectionConfig)
}

// ConfigListenerFunc 函数类型实现的监听器
type ConfigListenerFunc func(oldCfg, newCfg *CollectionConfig)

func (f ConfigListenerFunc) OnConfigUpdate(oldCfg, newCfg *CollectionConfig) {
	f(oldCfg, newCfg)
}

// Manager 配置管理器
type Manager struct {
	mu       sync.RWMutex
	log      *logger.Logger

	// 当前配置（原子操作保护指针）
	currentConfig atomic.Value // *CollectionConfig

	// 本地配置（作为fallback）
	localConfig *Config

	// 监听器列表
	listeners []ConfigListener

	// 版本号
	version int64

	// 组别ID
	groupID string

	// 探针ID
	probeID string

	// 更新通道（用于异步通知）
	updateCh chan *CollectionConfig

	// 停止信号
	stopCh chan struct{}

	// gRPC客户端（用于拉取配置）
	client ConfigClient
}

// ConfigClient 配置客户端接口
type ConfigClient interface {
	GetConfig(ctx context.Context, req *edge.GetConfigRequest) (*edge.GetConfigResponse, error)
}

// CollectionConfig 采集策略配置（本地定义，与proto保持一致）
type CollectionConfig struct {
	Version     int64
	GroupId     string
	UpdatedAt   int64
	UpdatedBy   string

	// 采样率配置
	SampleRate     float64
	TCPSampleRate  float64
	HTTPSampleRate float64

	// 采集项开关
	EnableTCPMetrics    bool
	EnableHTTPMetrics   bool
	EnableHTTPFull      bool
	EnableDNSFull       bool
	EnableMySQLFull     bool
	EnableSQLAggregator bool
	EnableCPUProfiler   bool

	// 资源限额
	MaxCPUCore    float64
	MaxMemoryMB   float64
	MaxGoroutines int

	// 采集间隔和批处理
	CollectInterval int
	BatchSize       int

	// 熔断配置
	CircuitBreakerEnabled bool
}

// NewConfigManager 创建配置管理器
func NewConfigManager(localCfg *Config, probeID, groupID string, log *logger.Logger) *Manager {
	m := &Manager{
		localConfig: localCfg,
		probeID:     probeID,
		groupID:     groupID,
		log:         log,
		listeners:   make([]ConfigListener, 0),
		updateCh:    make(chan *CollectionConfig, 10),
		stopCh:      make(chan struct{}),
	}

	// 初始化默认配置
	defaultCfg := m.buildDefaultConfig()
	m.currentConfig.Store(defaultCfg)
	m.version = defaultCfg.Version

	return m
}

// SetClient 设置gRPC客户端
func (m *Manager) SetClient(client ConfigClient) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.client = client
}

// Start 启动配置管理器
func (m *Manager) Start() {
	go m.updateLoop()
	m.log.Info("[配置管理器] 已启动")
}

// Stop 停止配置管理器
func (m *Manager) Stop() {
	close(m.stopCh)
	m.log.Info("[配置管理器] 已停止")
}

// GetConfig 获取当前配置（线程安全）
func (m *Manager) GetConfig() *CollectionConfig {
	cfg := m.currentConfig.Load()
	if cfg == nil {
		return m.buildDefaultConfig()
	}
	return cfg.(*CollectionConfig)
}

// GetVersion 获取当前配置版本
func (m *Manager) GetVersion() int64 {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.version
}

// AddListener 添加配置变更监听器
func (m *Manager) AddListener(listener ConfigListener) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.listeners = append(m.listeners, listener)
}

// RemoveListener 移除配置变更监听器
func (m *Manager) RemoveListener(listener ConfigListener) {
	m.mu.Lock()
	defer m.mu.Unlock()
	for i, l := range m.listeners {
		if l == listener {
			m.listeners = append(m.listeners[:i], m.listeners[i+1:]...)
			break
		}
	}
}

// UpdateConfig 更新配置（外部调用，如gRPC推送）
func (m *Manager) UpdateConfig(newCfg *CollectionConfig) error {
	if newCfg == nil {
		return fmt.Errorf("配置不能为空")
	}

	// 版本号检查
	current := m.GetConfig()
	if newCfg.Version <= current.Version && !m.isForceUpdate(newCfg) {
		m.log.Debugf("[配置管理器] 忽略过期配置: current=%d, new=%d", current.Version, newCfg.Version)
		return nil
	}

	// 发送到更新通道
	select {
	case m.updateCh <- newCfg:
		return nil
	case <-m.stopCh:
		return fmt.Errorf("配置管理器已停止")
	default:
		return fmt.Errorf("更新通道已满")
	}
}

// FetchConfig 从远程拉取配置
func (m *Manager) FetchConfig(ctx context.Context) (*CollectionConfig, error) {
	m.mu.RLock()
	client := m.client
	m.mu.RUnlock()

	if client == nil {
		return nil, fmt.Errorf("gRPC客户端未初始化")
	}

	req := &edge.GetConfigRequest{
		ProbeId: m.probeID,
		GroupId: m.groupID,
		Version: m.GetVersion(),
	}

	resp, err := client.GetConfig(ctx, req)
	if err != nil {
		return nil, fmt.Errorf("拉取配置失败: %w", err)
	}

	if !resp.Success {
		return nil, fmt.Errorf("拉取配置被拒绝: %s", resp.Message)
	}

	if !resp.HasUpdate || resp.Config == nil {
		return nil, nil // 无更新
	}

	return m.protoToLocalConfig(resp.Config), nil
}

// updateLoop 配置更新循环
func (m *Manager) updateLoop() {
	for {
		select {
		case newCfg := <-m.updateCh:
			m.applyConfig(newCfg)
		case <-m.stopCh:
			return
		}
	}
}

// applyConfig 应用新配置
func (m *Manager) applyConfig(newCfg *CollectionConfig) {
	oldCfg := m.GetConfig()

	// 验证配置
	if err := m.validateConfig(newCfg); err != nil {
		m.log.Errorf("[配置管理器] 配置验证失败: %v", err)
		return
	}

	// 更新配置
	m.currentConfig.Store(newCfg)
	m.mu.Lock()
	m.version = newCfg.Version
	m.mu.Unlock()

	m.log.Infof("[配置管理器] 配置已更新: version=%d, group=%s", newCfg.Version, newCfg.GroupId)

	// 通知监听器
	m.notifyListeners(oldCfg, newCfg)
}

// notifyListeners 通知所有监听器
func (m *Manager) notifyListeners(oldCfg, newCfg *CollectionConfig) {
	m.mu.RLock()
	listeners := make([]ConfigListener, len(m.listeners))
	copy(listeners, m.listeners)
	m.mu.RUnlock()

	for _, listener := range listeners {
		go func(l ConfigListener) {
			defer func() {
				if r := recover(); r != nil {
					m.log.Errorf("[配置管理器] 监听器panic: %v", r)
				}
			}()
			l.OnConfigUpdate(oldCfg, newCfg)
		}(listener)
	}
}

// validateConfig 验证配置有效性
func (m *Manager) validateConfig(cfg *CollectionConfig) error {
	if cfg.SampleRate < 0 || cfg.SampleRate > 1 {
		return fmt.Errorf("采样率必须在0-1之间: %f", cfg.SampleRate)
	}
	if cfg.TCPSampleRate < 0 || cfg.TCPSampleRate > 1 {
		return fmt.Errorf("TCP采样率必须在0-1之间: %f", cfg.TCPSampleRate)
	}
	if cfg.HTTPSampleRate < 0 || cfg.HTTPSampleRate > 1 {
		return fmt.Errorf("HTTP采样率必须在0-1之间: %f", cfg.HTTPSampleRate)
	}
	if cfg.MaxCPUCore < 0 {
		return fmt.Errorf("CPU核心数不能为负: %f", cfg.MaxCPUCore)
	}
	if cfg.MaxMemoryMB < 0 {
		return fmt.Errorf("内存限制不能为负: %f", cfg.MaxMemoryMB)
	}
	if cfg.CollectInterval < 1 {
		return fmt.Errorf("采集间隔至少为1秒: %d", cfg.CollectInterval)
	}
	if cfg.BatchSize < 1 {
		return fmt.Errorf("批处理大小至少为1: %d", cfg.BatchSize)
	}
	return nil
}

// isForceUpdate 检查是否为强制更新
func (m *Manager) isForceUpdate(cfg *CollectionConfig) bool {
	// 可以通过特定字段或版本号规则判断
	return cfg.Version < 0
}

// buildDefaultConfig 构建默认配置
func (m *Manager) buildDefaultConfig() *CollectionConfig {
	local := m.localConfig
	if local == nil {
		return &CollectionConfig{
			Version:     1,
			SampleRate:  1.0,
			MaxCPUCore:  1.0,
			MaxMemoryMB: 1024,
		}
	}

	return &CollectionConfig{
		Version:               1,
		SampleRate:            local.EBPF.PerfOptimizer.SampleRate,
		TCPSampleRate:         local.EBPF.PerfOptimizer.SampleRate,
		HTTPSampleRate:        local.EBPF.PerfOptimizer.SampleRate,
		EnableTCPMetrics:      local.EBPF.TCPMetrics.Enabled,
		EnableHTTPMetrics:     local.EBPF.HTTPMetrics.Enabled,
		EnableHTTPFull:        local.EBPF.ProtocolParsing.HTTPFull,
		EnableDNSFull:         local.EBPF.ProtocolParsing.DNSFull,
		EnableMySQLFull:       local.EBPF.ProtocolParsing.MySQLFull,
		EnableSQLAggregator:   local.EBPF.SQLAggregator.Enabled,
		EnableCPUProfiler:     local.EBPF.CPUProfiler.Enabled,
		MaxCPUCore:            local.EBPF.ResourceLimit.MaxCPUCore,
		MaxMemoryMB:           local.EBPF.ResourceLimit.MaxMemoryMB,
		MaxGoroutines:         local.EBPF.ResourceLimit.MaxGoroutines,
		CollectInterval:       local.CollectInterval,
		BatchSize:             local.BatchSize,
		CircuitBreakerEnabled: local.EBPF.CircuitBreaker.Enabled,
	}
}

// protoToLocalConfig 将proto配置转换为本地配置
func (m *Manager) protoToLocalConfig(protoCfg *edge.CollectionConfig) *CollectionConfig {
	return &CollectionConfig{
		Version:               protoCfg.Version,
		GroupId:               protoCfg.GroupId,
		UpdatedAt:             protoCfg.UpdatedAt,
		UpdatedBy:             protoCfg.UpdatedBy,
		SampleRate:            protoCfg.SampleRate,
		TCPSampleRate:         protoCfg.TCPSampleRate,
		HTTPSampleRate:        protoCfg.HTTPSampleRate,
		EnableTCPMetrics:      protoCfg.EnableTCPMetrics,
		EnableHTTPMetrics:     protoCfg.EnableHTTPMetrics,
		EnableHTTPFull:        protoCfg.EnableHTTPFull,
		EnableDNSFull:         protoCfg.EnableDNSFull,
		EnableMySQLFull:       protoCfg.EnableMySQLFull,
		EnableSQLAggregator:   protoCfg.EnableSQLAggregator,
		EnableCPUProfiler:     protoCfg.EnableCPUProfiler,
		MaxCPUCore:            protoCfg.MaxCPUCore,
		MaxMemoryMB:           protoCfg.MaxMemoryMB,
		MaxGoroutines:         protoCfg.MaxGoroutines,
		CollectInterval:       protoCfg.CollectInterval,
		BatchSize:             protoCfg.BatchSize,
		CircuitBreakerEnabled: protoCfg.CircuitBreakerEnabled,
	}
}

// StartPeriodicFetch 启动定期拉取配置
func (m *Manager) StartPeriodicFetch(interval time.Duration) {
	if interval <= 0 {
		interval = 60 * time.Second // 默认60秒
	}

	go func() {
		ticker := time.NewTicker(interval)
		defer ticker.Stop()

		for {
			select {
			case <-ticker.C:
				ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
				cfg, err := m.FetchConfig(ctx)
				cancel()

				if err != nil {
					m.log.Warnf("[配置管理器] 定期拉取配置失败: %v", err)
					continue
				}

				if cfg != nil {
					if err := m.UpdateConfig(cfg); err != nil {
						m.log.Warnf("[配置管理器] 更新配置失败: %v", err)
					}
				}
			case <-m.stopCh:
				return
			}
		}
	}()

	m.log.Infof("[配置管理器] 定期拉取已启动: 间隔=%v", interval)
}
