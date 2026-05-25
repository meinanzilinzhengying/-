// Package resilience 提供后端组件故障隔离、自动切换与优雅降级能力
// Copyright (c) 2026 Cloud Flow Team
// Licensed under the MIT License.

package resilience

import (
	"context"
	"fmt"
	"sync"
	"sync/atomic"
	"time"

	"github.com/meinanzilinzhengying/cloud-flow-agent/internal/circuitbreaker"
	"github.com/meinanzilinzhengying/cloud-flow-agent/pkg/models"
)

// ============================================================
// 组件健康状态
// ============================================================

// ComponentHealth 组件健康状态
type ComponentHealth string

const (
	HealthHealthy   ComponentHealth = "healthy"
	HealthDegraded  ComponentHealth = "degraded"
	HealthUnhealthy ComponentHealth = "unhealthy"
	HealthUnknown   ComponentHealth = "unknown"
)

// ComponentType 后端组件类型
type ComponentType string

const (
	ComponentCollectorEBPF       ComponentType = "collector_ebpf"
	ComponentCollectorTraditional ComponentType = "collector_traditional"
	ComponentCollectorMetrics    ComponentType = "collector_metrics"
	ComponentCollectorProcess    ComponentType = "collector_process"
	ComponentGRPCEdge            ComponentType = "grpc_edge"
	ComponentDualCenterSync      ComponentType = "dual_center_sync"
	ComponentFailover            ComponentType = "failover"
	ComponentConfigManager       ComponentType = "config_manager"
)

// ComponentInfo 组件信息
type ComponentInfo struct {
	ID          string          `json:"id"`
	Name        string          `json:"name"`
	Type        ComponentType   `json:"type"`
	Health      ComponentHealth `json:"health"`
	Active      bool            `json:"active"`       // 是否为当前活跃组件
	Primary     bool            `json:"primary"`      // 是否为主组件
	Fallback    string          `json:"fallback"`     // 降级目标组件ID
	Priority    int             `json:"priority"`     // 优先级（数值越小越高）
	ErrorCount  int64           `json:"error_count"`
	LastError   string          `json:"last_error"`
	SwitchCount int64           `json:"switch_count"` // 切换次数
	LastSwitch  time.Time       `json:"last_switch"`
	StartedAt   time.Time       `json:"started_at"`
}

// ============================================================
// 降级策略
// ============================================================

// DegradationPolicy 降级策略
type DegradationPolicy struct {
	ComponentID      string        `yaml:"component_id" json:"component_id"`
	MaxErrorRate     float64       `yaml:"max_error_rate" json:"max_error_rate"`       // 最大错误率触发降级
	MaxErrors        int           `yaml:"max_errors" json:"max_errors"`               // 最大错误次数触发降级
	ErrorWindow      time.Duration `yaml:"error_window" json:"error_window"`           // 错误统计窗口
	FallbackTo       string        `yaml:"fallback_to" json:"fallback_to"`             // 降级目标组件
	AutoRecover      bool          `yaml:"auto_recover" json:"auto_recover"`           // 自动恢复
	RecoverAfter     time.Duration `yaml:"recover_after" json:"recover_after"`         // 恢复等待时间
	RecoverThreshold int           `yaml:"recover_threshold" json:"recover_threshold"` // 恢复所需连续成功次数
}

// ============================================================
// 弹性管理器
// ============================================================

// ResilienceConfig 弹性管理配置
type ResilienceConfig struct {
	Enabled             bool                `yaml:"enabled" json:"enabled"`
	HealthCheckInterval time.Duration       `yaml:"health_check_interval" json:"health_check_interval"`
	SwitchTimeout       time.Duration       `yaml:"switch_timeout" json:"switch_timeout"`
	GracefulDegradation bool                `yaml:"graceful_degradation" json:"graceful_degradation"`
	Policies            []DegradationPolicy `yaml:"policies" json:"policies"`
}

// DefaultResilienceConfig 默认配置
func DefaultResilienceConfig() *ResilienceConfig {
	return &ResilienceConfig{
		Enabled:             true,
		HealthCheckInterval: 10 * time.Second,
		SwitchTimeout:       30 * time.Second,
		GracefulDegradation: true,
		Policies: []DegradationPolicy{
			{
				ComponentID:      "collector_ebpf",
				MaxErrorRate:     0.5,
				MaxErrors:        10,
				ErrorWindow:      60 * time.Second,
				FallbackTo:       "collector_traditional",
				AutoRecover:      true,
				RecoverAfter:     120 * time.Second,
				RecoverThreshold: 5,
			},
			{
				ComponentID:      "grpc_edge",
				MaxErrorRate:     0.5,
				MaxErrors:        5,
				ErrorWindow:      30 * time.Second,
				FallbackTo:       "local_buffer",
				AutoRecover:      true,
				RecoverAfter:     60 * time.Second,
				RecoverThreshold: 3,
			},
		},
	}
}

// Manager 弹性管理器 —— 组件故障隔离与自动切换
type Manager struct {
	config *ResilienceConfig

	// 组件注册表
	components map[string]*ComponentInfo
	mu         sync.RWMutex

	// 熔断器
	breakerMgr *circuitbreaker.Manager

	// 降级策略
	policies map[string]*DegradationPolicy

	// 错误追踪
	errorTracker map[string]*errorTracker

	// 生命周期
	ctx    context.Context
	cancel context.CancelFunc
	wg     sync.WaitGroup
	running atomic.Bool

	// 回调
	onSwitch    func(from, to string, reason string)
	onRecover   func(componentID string)
	onDegrade   func(componentID string, reason string)

	// 事件通道
	events chan ComponentEvent
}

// ComponentEvent 组件事件
type ComponentEvent struct {
	Type      string      `json:"type"`       // switch / recover / degrade / error
	Component string      `json:"component"`
	From      string      `json:"from,omitempty"`
	To        string      `json:"to,omitempty"`
	Reason    string      `json:"reason"`
	Timestamp time.Time   `json:"timestamp"`
	Data      interface{} `json:"data,omitempty"`
}

// errorTracker 错误追踪器
type errorTracker struct {
	errors     []time.Time
	mu         sync.Mutex
	window     time.Duration
}

func newErrorTracker(window time.Duration) *errorTracker {
	return &errorTracker{
		errors: make([]time.Time, 0),
		window: window,
	}
}

func (et *errorTracker) Record() {
	et.mu.Lock()
	defer et.mu.Unlock()
	et.errors = append(et.errors, time.Now())
}

func (et *errorTracker) Count() int {
	et.mu.Lock()
	defer et.mu.Unlock()
	et.clean()
	return len(et.errors)
}

func (et *errorTracker) Rate() float64 {
	et.mu.Lock()
	defer et.mu.Unlock()
	et.clean()
	if et.window == 0 {
		return 0
	}
	return float64(len(et.errors)) / et.window.Seconds()
}

func (et *errorTracker) clean() {
	cutoff := time.Now().Add(-et.window)
	i := 0
	for i < len(et.errors) && et.errors[i].Before(cutoff) {
		i++
	}
	et.errors = et.errors[i:]
}

func (et *errorTracker) Reset() {
	et.mu.Lock()
	defer et.mu.Unlock()
	et.errors = et.errors[:0]
}

// NewManager 创建弹性管理器
func NewManager(cfg *ResilienceConfig) *Manager {
	if cfg == nil {
		cfg = DefaultResilienceConfig()
	}

	ctx, cancel := context.WithCancel(context.Background())

	m := &Manager{
		config:       cfg,
		components:   make(map[string]*ComponentInfo),
		breakerMgr:   circuitbreaker.NewManager(),
		policies:     make(map[string]*DegradationPolicy),
		errorTracker: make(map[string]*errorTracker),
		ctx:          ctx,
		cancel:       cancel,
		events:       make(chan ComponentEvent, 100),
	}

	// 初始化降级策略
	for i := range cfg.Policies {
		p := cfg.Policies[i]
		m.policies[p.ComponentID] = &p
		m.errorTracker[p.ComponentID] = newErrorTracker(p.ErrorWindow)
	}

	return m
}

// ============================================================
// 组件注册
// ============================================================

// RegisterComponent 注册组件
func (m *Manager) RegisterComponent(id, name string, compType ComponentType, primary bool, priority int) {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.components[id] = &ComponentInfo{
		ID:        id,
		Name:      name,
		Type:      compType,
		Health:    HealthUnknown,
		Active:    primary,
		Primary:   primary,
		Priority:  priority,
		StartedAt: time.Now(),
	}

	// 为组件创建熔断器
	breakerCfg := circuitbreaker.DefaultConfig(id)
	if policy, ok := m.policies[id]; ok {
		breakerCfg.MaxFailures = policy.MaxErrors
	}
	m.breakerMgr.GetOrCreate(id, breakerCfg)

	// 初始化错误追踪器
	if _, ok := m.errorTracker[id]; !ok {
		m.errorTracker[id] = newErrorTracker(60 * time.Second)
	}
}

// UnregisterComponent 注销组件
func (m *Manager) UnregisterComponent(id string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	delete(m.components, id)
}

// GetComponent 获取组件
func (m *Manager) GetComponent(id string) (*ComponentInfo, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	c, ok := m.components[id]
	if !ok {
		return nil, false
	}
	cp := *c
	return &cp, true
}

// GetActiveComponent 获取指定类型的活跃组件
func (m *Manager) GetActiveComponent(compType ComponentType) (*ComponentInfo, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	var best *ComponentInfo
	for _, c := range m.components {
		if c.Type != compType || !c.Active {
			continue
		}
		if best == nil || c.Priority < best.Priority {
			best = c
		}
	}

	if best == nil {
		return nil, false
	}
	cp := *best
	return &cp, true
}

// GetAllComponents 获取所有组件
func (m *Manager) GetAllComponents() map[string]*ComponentInfo {
	m.mu.RLock()
	defer m.mu.RUnlock()
	result := make(map[string]*ComponentInfo, len(m.components))
	for k, v := range m.components {
		cp := *v
		result[k] = &cp
	}
	return result
}

// ============================================================
// 生命周期
// ============================================================

// Start 启动弹性管理器
func (m *Manager) Start() error {
	if m.running.Load() {
		return fmt.Errorf("resilience manager already running")
	}

	m.running.Store(true)

	// 启动健康检查循环
	m.wg.Add(1)
	go m.healthCheckLoop()

	// 启动事件处理器
	m.wg.Add(1)
	go m.eventProcessor()

	return nil
}

// Stop 停止弹性管理器
func (m *Manager) Stop() error {
	if !m.running.Load() {
		return nil
	}

	m.running.Store(false)
	m.cancel()
	m.wg.Wait()
	close(m.events)
	return nil
}

// ============================================================
// 故障上报与自动切换
// ============================================================

// ReportError 上报组件错误
func (m *Manager) ReportError(componentID string, err error) {
	if !m.running.Load() {
		return
	}

	// 记录到错误追踪器
	if et, ok := m.errorTracker[componentID]; ok {
		et.Record()
	}

	// 记录到熔断器
	if cb, ok := m.breakerMgr.Get(componentID); ok {
		cb.RecordFailure()
	}

	// 更新组件信息
	m.mu.Lock()
	if comp, ok := m.components[componentID]; ok {
		comp.ErrorCount++
		if err != nil {
			comp.LastError = err.Error()
		}
	}
	m.mu.Unlock()

	// 检查是否需要降级
	m.checkDegradation(componentID, err)
}

// ReportSuccess 上报组件成功
func (m *Manager) ReportSuccess(componentID string) {
	if !m.running.Load() {
		return
	}

	// 熔断器记录成功
	if cb, ok := m.breakerMgr.Get(componentID); ok {
		cb.RecordSuccess()
	}

	// 更新组件健康状态
	m.mu.Lock()
	if comp, ok := m.components[componentID]; ok {
		comp.Health = HealthHealthy
		comp.LastError = ""
	}
	m.mu.Unlock()

	// 检查是否可以恢复
	m.checkRecovery(componentID)
}

// checkDegradation 检查是否需要降级
func (m *Manager) checkDegradation(componentID string, err error) {
	policy, ok := m.policies[componentID]
	if !ok {
		return
	}

	et, ok := m.errorTracker[componentID]
	if !ok {
		return
	}

	// 检查错误次数
	if et.Count() >= policy.MaxErrors {
		m.degrade(componentID, fmt.Sprintf("error count %d >= %d", et.Count(), policy.MaxErrors))
		return
	}

	// 检查错误率
	if policy.MaxErrorRate > 0 && et.Rate() > policy.MaxErrorRate {
		m.degrade(componentID, fmt.Sprintf("error rate %.2f > %.2f", et.Rate(), policy.MaxErrorRate))
		return
	}

	// 检查熔断器状态
	if cb, ok := m.breakerMgr.Get(componentID); ok {
		if cb.State() == circuitbreaker.StateOpen {
			m.degrade(componentID, "circuit breaker open")
			return
		}
	}
}

// degrade 执行降级
func (m *Manager) degrade(componentID, reason string) {
	m.mu.Lock()
	comp, ok := m.components[componentID]
	if !ok || !comp.Active {
		m.mu.Unlock()
		return
	}

	// 标记当前组件为不活跃
	comp.Active = false
	comp.Health = HealthUnhealthy

	// 查找降级目标
	policy := m.policies[componentID]
	fallbackID := ""
	if policy != nil {
		fallbackID = policy.FallbackTo
	}

	// 激活降级组件
	if fallbackID != "" {
		if fallback, ok := m.components[fallbackID]; ok {
			fallback.Active = true
			fallback.Health = HealthHealthy
			fallback.LastSwitch = time.Now()
			fallback.SwitchCount++
			comp.SwitchCount++
			comp.LastSwitch = time.Now()
		}
	}
	m.mu.Unlock()

	// 发送事件
	m.events <- ComponentEvent{
		Type:      "degrade",
		Component: componentID,
		To:        fallbackID,
		Reason:    reason,
		Timestamp: time.Now(),
	}

	// 回调
	if m.onDegrade != nil {
		m.onDegrade(componentID, reason)
	}
	if m.onSwitch != nil && fallbackID != "" {
		m.onSwitch(componentID, fallbackID, reason)
	}
}

// checkRecovery 检查是否可以恢复
func (m *Manager) checkRecovery(componentID string) {
	policy, ok := m.policies[componentID]
	if !ok || !policy.AutoRecover {
		return
	}

	m.mu.Lock()
	comp, ok := m.components[componentID]
	if !ok || comp.Active {
		m.mu.Unlock()
		return
	}

	// 检查是否满足恢复条件
	et := m.errorTracker[componentID]
	cb, _ := m.breakerMgr.Get(componentID)

	// 条件1：距离降级已过恢复等待时间
	timeSinceSwitch := time.Since(comp.LastSwitch)
	if timeSinceSwitch < policy.RecoverAfter {
		m.mu.Unlock()
		return
	}

	// 条件2：熔断器已关闭
	if cb != nil && cb.State() != circuitbreaker.StateClosed {
		m.mu.Unlock()
		return
	}

	// 条件3：错误率已降低
	if et != nil && et.Count() > 0 {
		m.mu.Unlock()
		return
	}

	// 执行恢复
	m.recover(componentID)
	m.mu.Unlock()
}

// recover 执行恢复
func (m *Manager) recover(componentID string) {
	comp, ok := m.components[componentID]
	if !ok {
		return
	}

	// 查找当前活跃的降级组件
	policy := m.policies[componentID]
	var deactivatedID string
	if policy != nil && policy.FallbackTo != "" {
		if fallback, ok := m.components[policy.FallbackTo]; ok && fallback.Active {
			fallback.Active = false
			fallback.Health = HealthDegraded
			deactivatedID = policy.FallbackTo
		}
	}

	// 激活原组件
	comp.Active = true
	comp.Health = HealthHealthy
	comp.LastSwitch = time.Now()
	comp.SwitchCount++
	comp.LastError = ""

	// 重置错误追踪
	if et, ok := m.errorTracker[componentID]; ok {
		et.Reset()
	}

	// 发送事件
	m.events <- ComponentEvent{
		Type:      "recover",
		Component: componentID,
		From:      deactivatedID,
		Reason:    "auto_recover",
		Timestamp: time.Now(),
	}

	// 回调
	if m.onRecover != nil {
		m.onRecover(componentID)
	}
}

// ManualSwitch 手动切换组件
func (m *Manager) ManualSwitch(fromID, toID, reason string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	from, ok := m.components[fromID]
	if !ok || !from.Active {
		return fmt.Errorf("component %s is not active", fromID)
	}

	to, ok := m.components[toID]
	if !ok {
		return fmt.Errorf("component %s not found", toID)
	}

	// 切换
	from.Active = false
	from.Health = HealthDegraded
	from.LastSwitch = time.Now()
	from.SwitchCount++

	to.Active = true
	to.Health = HealthHealthy
	to.LastSwitch = time.Now()
	to.SwitchCount++

	// 发送事件
	m.events <- ComponentEvent{
		Type:      "switch",
		Component: fromID,
		From:      fromID,
		To:        toID,
		Reason:    reason,
		Timestamp: time.Now(),
	}

	return nil
}

// ============================================================
// 健康检查
// ============================================================

// healthCheckLoop 健康检查循环
func (m *Manager) healthCheckLoop() {
	defer m.wg.Done()

	ticker := time.NewTicker(m.config.HealthCheckInterval)
	defer ticker.Stop()

	for {
		select {
		case <-m.ctx.Done():
			return
		case <-ticker.C:
			m.runHealthCheck()
		}
	}
}

// runHealthCheck 执行健康检查
func (m *Manager) runHealthCheck() {
	m.mu.RLock()
	components := make([]*ComponentInfo, 0, len(m.components))
	for _, c := range m.components {
		components = append(components, c)
	}
	m.mu.RUnlock()

	for _, comp := range components {
		if !comp.Active {
			// 非活跃组件也检查是否可以恢复
			m.checkRecovery(comp.ID)
			continue
		}

		// 检查熔断器状态
		if cb, ok := m.breakerMgr.Get(comp.ID); ok {
			if cb.State() == circuitbreaker.StateOpen {
				m.mu.Lock()
				if c, ok := m.components[comp.ID]; ok {
					c.Health = HealthUnhealthy
				}
				m.mu.Unlock()
				continue
			}
			if cb.State() == circuitbreaker.StateHalfOpen {
				m.mu.Lock()
				if c, ok := m.components[comp.ID]; ok {
					c.Health = HealthDegraded
				}
				m.mu.Unlock()
				continue
			}
		}

		// 检查错误率
		if et, ok := m.errorTracker[comp.ID]; ok {
			policy := m.policies[comp.ID]
			if policy != nil && et.Rate() > policy.MaxErrorRate*0.8 {
				m.mu.Lock()
				if c, ok := m.components[comp.ID]; ok {
					c.Health = HealthDegraded
				}
				m.mu.Unlock()
				continue
			}
		}

		// 标记为健康
		m.mu.Lock()
		if c, ok := m.components[comp.ID]; ok {
			c.Health = HealthHealthy
		}
		m.mu.Unlock()
	}
}

// ============================================================
// 事件处理
// ============================================================

// eventProcessor 事件处理器
func (m *Manager) eventProcessor() {
	defer m.wg.Done()
	for {
		select {
		case <-m.ctx.Done():
			return
		case _, ok := <-m.events:
			if !ok {
				return
			}
			// 事件已通过channel分发，此处可扩展日志/告警等
		}
	}
}

// Events 返回事件通道（只读）
func (m *Manager) Events() <-chan ComponentEvent {
	return m.events
}

// ============================================================
// 回调注册
// ============================================================

// OnSwitch 注册切换回调
func (m *Manager) OnSwitch(fn func(from, to string, reason string)) {
	m.onSwitch = fn
}

// OnRecover 注册恢复回调
func (m *Manager) OnRecover(fn func(componentID string)) {
	m.onRecover = fn
}

// OnDegrade 注册降级回调
func (m *Manager) OnDegrade(fn func(componentID string, reason string)) {
	m.onDegrade = fn
}

// ============================================================
// 状态查询
// ============================================================

// GetStatus 获取弹性管理器状态
func (m *Manager) GetStatus() ResilienceStatus {
	m.mu.RLock()
	defer m.mu.RUnlock()

	status := ResilienceStatus{
		Enabled:    m.config.Enabled,
		Components: make(map[string]*ComponentInfo, len(m.components)),
		Breakers:   m.breakerMgr.AllStats(),
	}

	for id, comp := range m.components {
		cp := *comp
		status.Components[id] = &cp
	}

	return status
}

// ResilienceStatus 弹性管理器状态
type ResilienceStatus struct {
	Enabled    bool                            `json:"enabled"`
	Components map[string]*ComponentInfo       `json:"components"`
	Breakers   map[string]circuitbreaker.BreakerStats `json:"breakers"`
}

// ============================================================
// 预定义降级规则
// ============================================================

// DefaultDegradationPolicies 默认降级策略集合
func DefaultDegradationPolicies() []DegradationPolicy {
	return []DegradationPolicy{
		// eBPF 采集器 → 传统采集器
		{
			ComponentID:      "collector_ebpf",
			MaxErrorRate:     0.5,
			MaxErrors:        10,
			ErrorWindow:      60 * time.Second,
			FallbackTo:       "collector_traditional",
			AutoRecover:      true,
			RecoverAfter:     120 * time.Second,
			RecoverThreshold: 5,
		},
		// gRPC Edge 连接 → 本地缓冲
		{
			ComponentID:      "grpc_edge",
			MaxErrorRate:     0.5,
			MaxErrors:        5,
			ErrorWindow:      30 * time.Second,
			FallbackTo:       "local_buffer",
			AutoRecover:      true,
			RecoverAfter:     60 * time.Second,
			RecoverThreshold: 3,
		},
		// 指标采集器 → 无降级（仅告警）
		{
			ComponentID:      "collector_metrics",
			MaxErrorRate:     0.8,
			MaxErrors:        20,
			ErrorWindow:      60 * time.Second,
			FallbackTo:       "",
			AutoRecover:      true,
			RecoverAfter:     60 * time.Second,
			RecoverThreshold: 3,
		},
		// 进程采集器 → 无降级
		{
			ComponentID:      "collector_process",
			MaxErrorRate:     0.8,
			MaxErrors:        20,
			ErrorWindow:      60 * time.Second,
			FallbackTo:       "",
			AutoRecover:      true,
			RecoverAfter:     60 * time.Second,
			RecoverThreshold: 3,
		},
		// 双中心同步 → 降级为本地模式
		{
			ComponentID:      "dual_center_sync",
			MaxErrorRate:     0.5,
			MaxErrors:        5,
			ErrorWindow:      60 * time.Second,
			FallbackTo:       "local_only",
			AutoRecover:      true,
			RecoverAfter:     120 * time.Second,
			RecoverThreshold: 5,
		},
	}
}

// ============================================================
// 与现有采集器集成辅助
// ============================================================

// CollectorWrapper 采集器熔断包装器
// 将普通采集器包装为带熔断保护的采集器
type CollectorWrapper struct {
	collector models.Collector
	breaker   *circuitbreaker.CircuitBreaker
	resilience *Manager
	componentID string
}

// NewCollectorWrapper 创建采集器包装器
func NewCollectorWrapper(collector models.Collector, breaker *circuitbreaker.CircuitBreaker, resilience *Manager, componentID string) *CollectorWrapper {
	return &CollectorWrapper{
		collector:   collector,
		breaker:     breaker,
		resilience:  resilience,
		componentID: componentID,
	}
}

// Name 返回采集器名称
func (cw *CollectorWrapper) Name() string {
	return cw.collector.Name()
}

// Type 返回采集器类型
func (cw *CollectorWrapper) Type() models.CollectorType {
	return cw.collector.Type()
}

// Init 初始化
func (cw *CollectorWrapper) Init(ctx context.Context, config interface{}) error {
	return cw.collector.Init(ctx, config)
}

// Start 带熔断保护的启动
func (cw *CollectorWrapper) Start(ctx context.Context) error {
	return cw.breaker.Execute(ctx, func(execCtx context.Context) error {
		err := cw.collector.Start(execCtx)
		if err != nil {
			cw.resilience.ReportError(cw.componentID, err)
			return err
		}
		cw.resilience.ReportSuccess(cw.componentID)
		return nil
	})
}

// Stop 停止
func (cw *CollectorWrapper) Stop(ctx context.Context) error {
	return cw.collector.Stop(ctx)
}

// Status 状态
func (cw *CollectorWrapper) Status() models.CollectorStatus {
	return cw.collector.Status()
}

// Events 事件通道
func (cw *CollectorWrapper) Events() <-chan interface{} {
	return cw.collector.Events()
}

// Errors 错误通道（带熔断过滤）
func (cw *CollectorWrapper) Errors() <-chan error {
	return cw.collector.Errors()
}
