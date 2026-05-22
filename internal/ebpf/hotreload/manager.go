/*
 * Cloud Flow Agent - Probe Hot Reload Manager
 *
 * 探针热更新管理器，支持运行时动态 attach/detach 探针
 * 无需重启 Agent 即可增删采集指标
 */

package hotreload

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/meinanzilinzhengying/cloud-flow-agent/internal/ebpf/libbpf"
)

// ProbeType 探针类型
type ProbeType string

const (
	ProbeTypeKprobe        ProbeType = "kprobe"
	ProbeTypeKretprobe     ProbeType = "kretprobe"
	ProbeTypeTracepoint    ProbeType = "tracepoint"
	ProbeTypeFentry        ProbeType = "fentry"
	ProbeTypeFexit         ProbeType = "fexit"
	ProbeTypeRawTracepoint ProbeType = "raw_tracepoint"
)

// ProbeState 探针状态
type ProbeState string

const (
	ProbeStateDetached ProbeState = "detached"
	ProbeStateAttaching ProbeState = "attaching"
	ProbeStateAttached  ProbeState = "attached"
	ProbeStateDetaching ProbeState = "detaching"
	ProbeStateError     ProbeState = "error"
)

// ProbeDefinition 探针定义
type ProbeDefinition struct {
	ID          string                 `json:"id" yaml:"id"`
	Name        string                 `json:"name" yaml:"name"`
	Type        ProbeType              `json:"type" yaml:"type"`
	Program     string                 `json:"program" yaml:"program"`
	Target      string                 `json:"target" yaml:"target"`
	Category    string                 `json:"category,omitempty" yaml:"category,omitempty"` // for tracepoint
	Enabled     bool                   `json:"enabled" yaml:"enabled"`
	Priority    int                    `json:"priority" yaml:"priority"`
	Description string                 `json:"description" yaml:"description"`
	Labels      map[string]string      `json:"labels,omitempty" yaml:"labels,omitempty"`
	Params      map[string]interface{} `json:"params,omitempty" yaml:"params,omitempty"`
}

// ProbeInstance 探针实例
type ProbeInstance struct {
	Definition ProbeDefinition `json:"definition"`
	State      ProbeState      `json:"state"`
	Error      string          `json:"error,omitempty"`
	AttachedAt *time.Time      `json:"attached_at,omitempty"`
	DetachedAt *time.Time      `json:"detached_at,omitempty"`
	AttachCount int            `json:"attach_count"`
	LastError  string          `json:"last_error,omitempty"`
}

// MetricDefinition 指标定义
type MetricDefinition struct {
	ID          string            `json:"id" yaml:"id"`
	Name        string            `json:"name" yaml:"name"`
	Description string            `json:"description" yaml:"description"`
	Unit        string            `json:"unit" yaml:"unit"`
	Type        string            `json:"type" yaml:"type"` // counter, gauge, histogram
	Probes      []string          `json:"probes" yaml:"probes"` // 依赖的探针ID
	Enabled     bool              `json:"enabled" yaml:"enabled"`
	Labels      map[string]string `json:"labels,omitempty" yaml:"labels,omitempty"`
}

// HotReloadManager 热更新管理器
type HotReloadManager struct {
	mu sync.RWMutex

	// libbpf 加载器
	loader *libbpf.Loader

	// 探针注册表
	probes map[string]*ProbeInstance

	// 指标注册表
	metrics map[string]*MetricDefinition

	// 依赖关系: metric_id -> []probe_id
	metricDeps map[string][]string

	// 反向依赖: probe_id -> []metric_id
	probeDependents map[string][]string

	// 配置
	config *HotReloadConfig

	// 事件通道
	eventCh chan ProbeEvent

	// 控制上下文
	ctx    context.Context
	cancel context.CancelFunc

	// 运行状态
	running bool
}

// HotReloadConfig 热更新配置
type HotReloadConfig struct {
	// 热更新总开关
	Enabled bool `json:"enabled" yaml:"enabled"`

	// 自动应用变更
	AutoApply bool `json:"auto_apply" yaml:"auto_apply"`

	// 变更检查间隔
	CheckInterval time.Duration `json:"check_interval" yaml:"check_interval"`

	// 探针 attach 超时
	AttachTimeout time.Duration `json:"attach_timeout" yaml:"attach_timeout"`

	// 探针 detach 超时
	DetachTimeout time.Duration `json:"detach_timeout" yaml:"detach_timeout"`

	// 失败重试次数
	RetryCount int `json:"retry_count" yaml:"retry_count"`

	// 重试间隔
	RetryInterval time.Duration `json:"retry_interval" yaml:"retry_interval"`

	// 是否允许部分失败
	AllowPartialFailure bool `json:"allow_partial_failure" yaml:"allow_partial_failure"`
}

// ProbeEvent 探针事件
type ProbeEvent struct {
	Timestamp time.Time       `json:"timestamp"`
	ProbeID   string          `json:"probe_id"`
	OldState  ProbeState      `json:"old_state"`
	NewState  ProbeState      `json:"new_state"`
	Error     string          `json:"error,omitempty"`
}

// DefaultHotReloadConfig 默认配置
func DefaultHotReloadConfig() *HotReloadConfig {
	return &HotReloadConfig{
		Enabled:             true,
		AutoApply:           true,
		CheckInterval:       30 * time.Second,
		AttachTimeout:       10 * time.Second,
		DetachTimeout:       5 * time.Second,
		RetryCount:          3,
		RetryInterval:       1 * time.Second,
		AllowPartialFailure: true,
	}
}

// NewHotReloadManager 创建热更新管理器
func NewHotReloadManager(loader *libbpf.Loader, config *HotReloadConfig) *HotReloadManager {
	if config == nil {
		config = DefaultHotReloadConfig()
	}

	ctx, cancel := context.WithCancel(context.Background())

	return &HotReloadManager{
		loader:          loader,
		probes:          make(map[string]*ProbeInstance),
		metrics:         make(map[string]*MetricDefinition),
		metricDeps:      make(map[string][]string),
		probeDependents: make(map[string][]string),
		config:          config,
		eventCh:         make(chan ProbeEvent, 100),
		ctx:             ctx,
		cancel:          cancel,
	}
}

// RegisterProbe 注册探针
func (m *HotReloadManager) RegisterProbe(def ProbeDefinition) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if _, exists := m.probes[def.ID]; exists {
		return fmt.Errorf("probe %s already registered", def.ID)
	}

	instance := &ProbeInstance{
		Definition: def,
		State:      ProbeStateDetached,
	}

	m.probes[def.ID] = instance

	// 如果启用自动应用且探针标记为启用，则自动 attach
	if m.config.AutoApply && def.Enabled {
		go m.attachProbe(def.ID)
	}

	return nil
}

// UnregisterProbe 注销探针
func (m *HotReloadManager) UnregisterProbe(probeID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	instance, exists := m.probes[probeID]
	if !exists {
		return fmt.Errorf("probe %s not found", probeID)
	}

	// 检查是否有指标依赖此探针
	dependents := m.probeDependents[probeID]
	if len(dependents) > 0 {
		return fmt.Errorf("probe %s has dependent metrics: %v", probeID, dependents)
	}

	// 如果已附加，先 detach
	if instance.State == ProbeStateAttached {
		m.mu.Unlock()
		if err := m.DetachProbe(probeID); err != nil {
			return err
		}
		m.mu.Lock()
	}

	delete(m.probes, probeID)
	return nil
}

// RegisterMetric 注册指标
func (m *HotReloadManager) RegisterMetric(def MetricDefinition) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if _, exists := m.metrics[def.ID]; exists {
		return fmt.Errorf("metric %s already registered", def.ID)
	}

	// 验证依赖的探针是否存在
	for _, probeID := range def.Probes {
		if _, exists := m.probes[probeID]; !exists {
			return fmt.Errorf("dependent probe %s not found", probeID)
		}
	}

	m.metrics[def.ID] = &def
	m.metricDeps[def.ID] = def.Probes

	// 建立反向依赖关系
	for _, probeID := range def.Probes {
		m.probeDependents[probeID] = append(m.probeDependents[probeID], def.ID)
	}

	// 如果启用自动应用且指标标记为启用，则启用依赖的探针
	if m.config.AutoApply && def.Enabled {
		for _, probeID := range def.Probes {
			if probe, exists := m.probes[probeID]; exists && !probe.Definition.Enabled {
				probe.Definition.Enabled = true
				go m.attachProbe(probeID)
			}
		}
	}

	return nil
}

// UnregisterMetric 注销指标
func (m *HotReloadManager) UnregisterMetric(metricID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	metric, exists := m.metrics[metricID]
	if !exists {
		return fmt.Errorf("metric %s not found", metricID)
	}

	// 清理依赖关系
	for _, probeID := range metric.Probes {
		dependents := m.probeDependents[probeID]
		for i, id := range dependents {
			if id == metricID {
				m.probeDependents[probeID] = append(dependents[:i], dependents[i+1:]...)
				break
			}
		}
	}

	delete(m.metrics, metricID)
	delete(m.metricDeps, metricID)

	return nil
}

// AttachProbe 附加探针
func (m *HotReloadManager) AttachProbe(probeID string) error {
	m.mu.Lock()
	instance, exists := m.probes[probeID]
	if !exists {
		m.mu.Unlock()
		return fmt.Errorf("probe %s not found", probeID)
	}

	if instance.State == ProbeStateAttached || instance.State == ProbeStateAttaching {
		m.mu.Unlock()
		return nil // 已经附加或正在附加
	}

	instance.State = ProbeStateAttaching
	m.mu.Unlock()

	return m.attachProbe(probeID)
}

// attachProbe 内部 attach 实现
func (m *HotReloadManager) attachProbe(probeID string) error {
	m.mu.RLock()
	instance := m.probes[probeID]
	m.mu.RUnlock()

	if instance == nil {
		return fmt.Errorf("probe %s not found", probeID)
	}

	def := instance.Definition
	var err error

	// 根据探针类型选择 attach 方法
	switch def.Type {
	case ProbeTypeKprobe:
		err = m.loader.AttachKprobe(def.Program, def.Target)
	case ProbeTypeKretprobe:
		err = m.loader.AttachKretprobe(def.Program, def.Target)
	case ProbeTypeTracepoint:
		err = m.loader.AttachTracepoint(def.Program, def.Category, def.Target)
	case ProbeTypeFentry:
		err = m.loader.AttachFentry(def.Program, def.Target)
	case ProbeTypeFexit:
		err = m.loader.AttachFexit(def.Program, def.Target)
	case ProbeTypeRawTracepoint:
		err = m.loader.AttachRawTracepoint(def.Program, def.Target)
	default:
		err = fmt.Errorf("unsupported probe type: %s", def.Type)
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	now := time.Now()
	if err != nil {
		instance.State = ProbeStateError
		instance.Error = err.Error()
		instance.LastError = err.Error()
		m.emitEvent(probeID, ProbeStateAttaching, ProbeStateError, err.Error())
		return err
	}

	instance.State = ProbeStateAttached
	instance.AttachedAt = &now
	instance.AttachCount++
	instance.Error = ""
	m.emitEvent(probeID, ProbeStateAttaching, ProbeStateAttached, "")

	return nil
}

// DetachProbe 分离探针
func (m *HotReloadManager) DetachProbe(probeID string) error {
	m.mu.Lock()
	instance, exists := m.probes[probeID]
	if !exists {
		m.mu.Unlock()
		return fmt.Errorf("probe %s not found", probeID)
	}

	// 检查是否有启用的指标依赖此探针
	dependents := m.probeDependents[probeID]
	for _, metricID := range dependents {
		if metric, exists := m.metrics[metricID]; exists && metric.Enabled {
			m.mu.Unlock()
			return fmt.Errorf("cannot detach probe %s, metric %s is still enabled", probeID, metricID)
		}
	}

	if instance.State == ProbeStateDetached || instance.State == ProbeStateDetaching {
		m.mu.Unlock()
		return nil // 已经分离或正在分离
	}

	oldState := instance.State
	instance.State = ProbeStateDetaching
	m.mu.Unlock()

	err := m.loader.Detach(probeID)

	m.mu.Lock()
	defer m.mu.Unlock()

	now := time.Now()
	if err != nil {
		instance.State = ProbeStateError
		instance.Error = err.Error()
		instance.LastError = err.Error()
		m.emitEvent(probeID, oldState, ProbeStateError, err.Error())
		return err
	}

	instance.State = ProbeStateDetached
	instance.DetachedAt = &now
	instance.Error = ""
	m.emitEvent(probeID, oldState, ProbeStateDetached, "")

	return nil
}

// EnableMetric 启用指标（自动 attach 依赖的探针）
func (m *HotReloadManager) EnableMetric(metricID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	metric, exists := m.metrics[metricID]
	if !exists {
		return fmt.Errorf("metric %s not found", metricID)
	}

	metric.Enabled = true

	var errors []error
	for _, probeID := range metric.Probes {
		instance, exists := m.probes[probeID]
		if !exists {
			errors = append(errors, fmt.Errorf("probe %s not found", probeID))
			continue
		}

		if instance.State == ProbeStateAttached {
			continue // 已经附加
		}

		instance.Definition.Enabled = true

		// 异步 attach
		go m.attachProbe(probeID)
	}

	if len(errors) > 0 && !m.config.AllowPartialFailure {
		return fmt.Errorf("failed to enable metric %s: %v", metricID, errors)
	}

	return nil
}

// DisableMetric 禁用指标（自动 detach 不再需要的探针）
func (m *HotReloadManager) DisableMetric(metricID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	metric, exists := m.metrics[metricID]
	if !exists {
		return fmt.Errorf("metric %s not found", metricID)
	}

	metric.Enabled = false

	// 检查依赖的探针是否可以 detach
	for _, probeID := range metric.Probes {
		canDetach := true

		// 检查是否有其他启用的指标依赖此探针
		for _, dependentID := range m.probeDependents[probeID] {
			if dependentID == metricID {
				continue
			}
			if depMetric, exists := m.metrics[dependentID]; exists && depMetric.Enabled {
				canDetach = false
				break
			}
		}

		if canDetach {
			instance := m.probes[probeID]
			if instance != nil {
				instance.Definition.Enabled = false
				go m.detachProbe(probeID)
			}
		}
	}

	return nil
}

// detachProbe 内部 detach 实现
func (m *HotReloadManager) detachProbe(probeID string) error {
	err := m.loader.Detach(probeID)

	m.mu.Lock()
	defer m.mu.Unlock()

	instance := m.probes[probeID]
	if instance == nil {
		return nil
	}

	now := time.Now()
	if err != nil {
		instance.State = ProbeStateError
		instance.Error = err.Error()
		instance.LastError = err.Error()
		m.emitEvent(probeID, ProbeStateDetaching, ProbeStateError, err.Error())
		return err
	}

	instance.State = ProbeStateDetached
	instance.DetachedAt = &now
	instance.Error = ""
	m.emitEvent(probeID, ProbeStateDetaching, ProbeStateDetached, "")

	return nil
}

// GetProbe 获取探针实例
func (m *HotReloadManager) GetProbe(probeID string) (*ProbeInstance, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	instance, exists := m.probes[probeID]
	if !exists {
		return nil, fmt.Errorf("probe %s not found", probeID)
	}

	// 返回副本
	copy := *instance
	return &copy, nil
}

// GetMetric 获取指标定义
func (m *HotReloadManager) GetMetric(metricID string) (*MetricDefinition, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	metric, exists := m.metrics[metricID]
	if !exists {
		return nil, fmt.Errorf("metric %s not found", metricID)
	}

	// 返回副本
	copy := *metric
	return &copy, nil
}

// ListProbes 列出所有探针
func (m *HotReloadManager) ListProbes() []*ProbeInstance {
	m.mu.RLock()
	defer m.mu.RUnlock()

	result := make([]*ProbeInstance, 0, len(m.probes))
	for _, instance := range m.probes {
		copy := *instance
		result = append(result, &copy)
	}
	return result
}

// ListMetrics 列出所有指标
func (m *HotReloadManager) ListMetrics() []*MetricDefinition {
	m.mu.RLock()
	defer m.mu.RUnlock()

	result := make([]*MetricDefinition, 0, len(m.metrics))
	for _, metric := range m.metrics {
		copy := *metric
		result = append(result, &copy)
	}
	return result
}

// GetProbesByState 按状态获取探针
func (m *HotReloadManager) GetProbesByState(state ProbeState) []*ProbeInstance {
	m.mu.RLock()
	defer m.mu.RUnlock()

	result := make([]*ProbeInstance, 0)
	for _, instance := range m.probes {
		if instance.State == state {
			copy := *instance
			result = append(result, &copy)
		}
	}
	return result
}

// UpdateProbeConfig 更新探针配置（热更新）
func (m *HotReloadManager) UpdateProbeConfig(probeID string, updates map[string]interface{}) error {
	m.mu.Lock()
	instance, exists := m.probes[probeID]
	if !exists {
		m.mu.Unlock()
		return fmt.Errorf("probe %s not found", probeID)
	}

	wasEnabled := instance.Definition.Enabled
	m.mu.Unlock()

	// 应用更新
	if enabled, ok := updates["enabled"].(bool); ok {
		if enabled != wasEnabled {
			if enabled {
				return m.AttachProbe(probeID)
			} else {
				return m.DetachProbe(probeID)
			}
		}
	}

	// 其他配置更新
	m.mu.Lock()
	if priority, ok := updates["priority"].(int); ok {
		instance.Definition.Priority = priority
	}
	if labels, ok := updates["labels"].(map[string]string); ok {
		instance.Definition.Labels = labels
	}
	m.mu.Unlock()

	return nil
}

// ReloadProbe 重新加载探针（detach + attach）
func (m *HotReloadManager) ReloadProbe(probeID string) error {
	// 先 detach
	if err := m.DetachProbe(probeID); err != nil {
		return fmt.Errorf("detach failed: %w", err)
	}

	// 等待一小段时间确保完全分离
	time.Sleep(100 * time.Millisecond)

	// 再 attach
	if err := m.AttachProbe(probeID); err != nil {
		return fmt.Errorf("attach failed: %w", err)
	}

	return nil
}

// BatchUpdate 批量更新探针状态
func (m *HotReloadManager) BatchUpdate(updates map[string]bool) error {
	var errors []error

	for probeID, enabled := range updates {
		if enabled {
			if err := m.AttachProbe(probeID); err != nil {
				errors = append(errors, fmt.Errorf("attach %s: %w", probeID, err))
			}
		} else {
			if err := m.DetachProbe(probeID); err != nil {
				errors = append(errors, fmt.Errorf("detach %s: %w", probeID, err))
			}
		}
	}

	if len(errors) > 0 && !m.config.AllowPartialFailure {
		return fmt.Errorf("batch update failed: %v", errors)
	}

	return nil
}

// SubscribeEvents 订阅探针事件
func (m *HotReloadManager) SubscribeEvents() <-chan ProbeEvent {
	return m.eventCh
}

// emitEvent 发送事件
func (m *HotReloadManager) emitEvent(probeID string, oldState, newState ProbeState, err string) {
	select {
	case m.eventCh <- ProbeEvent{
		Timestamp: time.Now(),
		ProbeID:   probeID,
		OldState:  oldState,
		NewState:  newState,
		Error:     err,
	}:
	default:
		// 通道满，丢弃事件
	}
}

// GetStats 获取统计信息
func (m *HotReloadManager) GetStats() map[string]interface{} {
	m.mu.RLock()
	defer m.mu.RUnlock()

	stats := map[string]interface{}{
		"total_probes":  len(m.probes),
		"total_metrics": len(m.metrics),
		"probes_by_state": map[string]int{
			"detached":  0,
			"attaching": 0,
			"attached":  0,
			"detaching": 0,
			"error":     0,
		},
	}

	for _, instance := range m.probes {
		stateMap := stats["probes_by_state"].(map[string]int)
		stateMap[string(instance.State)]++
	}

	return stats
}

// Close 关闭管理器
func (m *HotReloadManager) Close() error {
	m.cancel()

	// Detach 所有探针
	m.mu.RLock()
	probeIDs := make([]string, 0, len(m.probes))
	for id := range m.probes {
		probeIDs = append(probeIDs, id)
	}
	m.mu.RUnlock()

	for _, id := range probeIDs {
		m.DetachProbe(id)
	}

	close(m.eventCh)
	return nil
}

// HealthCheck 健康检查
func (m *HotReloadManager) HealthCheck() error {
	m.mu.RLock()
	defer m.mu.RUnlock()

	// 检查是否有处于 error 状态的探针
	errorProbes := 0
	for _, instance := range m.probes {
		if instance.State == ProbeStateError {
			errorProbes++
		}
	}

	if errorProbes > 0 {
		return fmt.Errorf("%d probes in error state", errorProbes)
	}

	return nil
}
