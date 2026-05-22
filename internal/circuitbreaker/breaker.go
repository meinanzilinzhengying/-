// Package circuitbreaker 提供增强型熔断器，支持多级状态、滑动窗口和自适应恢复
// Copyright (c) 2026 Cloud Flow Team
// Licensed under the MIT License.

package circuitbreaker

import (
	"context"
	"fmt"
	"sync"
	"sync/atomic"
	"time"
)

// ============================================================
// 熔断器状态
// ============================================================

// State 熔断器状态
type State int32

const (
	StateClosed   State = iota // 关闭（正常）
	StateOpen                  // 打开（熔断）
	StateHalfOpen              // 半开（探测恢复）
)

func (s State) String() string {
	switch s {
	case StateClosed:
		return "closed"
	case StateOpen:
		return "open"
	case StateHalfOpen:
		return "half_open"
	default:
		return "unknown"
	}
}

// ============================================================
// 熔断器配置
// ============================================================

// Config 熔断器配置
type Config struct {
	// 基础配置
	Name          string        `yaml:"name" json:"name"`
	MaxFailures    int           `yaml:"max_failures" json:"max_failures"`         // 最大连续失败次数
	ResetTimeout   time.Duration `yaml:"reset_timeout" json:"reset_timeout"`       // 熔断恢复超时
	HalfOpenMax    int           `yaml:"half_open_max" json:"half_open_max"`       // 半开状态最大探测数

	// 滑动窗口配置
	WindowTime     time.Duration `yaml:"window_time" json:"window_time"`           // 滑动窗口时间
	WindowBuckets  int           `yaml:"window_buckets" json:"window_buckets"`     // 滑动窗口桶数

	// 自适应配置
	AdaptiveEnabled  bool          `yaml:"adaptive_enabled" json:"adaptive_enabled"`     // 启用自适应
	AdaptiveInterval time.Duration `yaml:"adaptive_interval" json:"adaptive_interval"` // 自适应调整间隔
	MinResetTimeout  time.Duration `yaml:"min_reset_timeout" json:"min_reset_timeout"` // 最小恢复超时
	MaxResetTimeout  time.Duration `yaml:"max_reset_timeout" json:"max_reset_timeout"` // 最大恢复超时

	// 超时控制
	Timeout         time.Duration `yaml:"timeout" json:"timeout"`                   // 单次执行超时
}

// DefaultConfig 默认配置
func DefaultConfig(name string) *Config {
	return &Config{
		Name:           name,
		MaxFailures:    5,
		ResetTimeout:   30 * time.Second,
		HalfOpenMax:    3,
		WindowTime:     60 * time.Second,
		WindowBuckets:  10,
		AdaptiveEnabled: true,
		AdaptiveInterval: 60 * time.Second,
		MinResetTimeout:  5 * time.Second,
		MaxResetTimeout:  5 * time.Minute,
		Timeout:        10 * time.Second,
	}
}

// ============================================================
// 增强型熔断器
// ============================================================

// CircuitBreaker 增强型熔断器
type CircuitBreaker struct {
	config *Config

	// 状态
	state       atomic.Int32 // State
	failures    atomic.Int64
	successes   atomic.Int64
	lastFailAt  atomic.Int64 // unix nano
	lastOpenAt  atomic.Int64 // unix nano

	// 半开状态探测计数
	halfOpenProbes atomic.Int64

	// 滑动窗口
	window *SlidingWindow

	// 自适应
	consecutiveOpens atomic.Int64

	// 事件回调
	onStateChange func(name string, from, to State)
	onTrip        func(name string, reason string)

	// 生命周期
	mu      sync.RWMutex
	running atomic.Bool
	stopCh  chan struct{}
}

// NewCircuitBreaker 创建增强型熔断器
func NewCircuitBreaker(cfg *Config) *CircuitBreaker {
	if cfg == nil {
		cfg = DefaultConfig("default")
	}

	cb := &CircuitBreaker{
		config: cfg,
		window: NewSlidingWindow(cfg.WindowTime, cfg.WindowBuckets),
		stopCh: make(chan struct{}),
	}
	cb.state.Store(int32(StateClosed))

	return cb
}

// Allow 检查是否允许执行
func (cb *CircuitBreaker) Allow() bool {
	state := cb.currentState()

	switch state {
	case StateClosed:
		return true
	case StateOpen:
		// 检查是否可以进入半开状态
		if time.Since(cb.openTime()) > cb.config.ResetTimeout {
			cb.transitionTo(StateHalfOpen, "reset_timeout")
			return true
		}
		return false
	case StateHalfOpen:
		// 半开状态限制探测数
		if cb.halfOpenProbes.Load() < int64(cb.config.HalfOpenMax) {
			return true
		}
		return false
	}
	return false
}

// Execute 带熔断保护的执行
func (cb *CircuitBreaker) Execute(ctx context.Context, fn func(context.Context) error) error {
	if !cb.Allow() {
		return ErrCircuitOpen
	}

	// 超时控制
	if cb.config.Timeout > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, cb.config.Timeout)
		defer cancel()
	}

	// 半开状态递增探测计数
	if cb.currentState() == StateHalfOpen {
		cb.halfOpenProbes.Add(1)
	}

	err := fn(ctx)

	if err != nil {
		cb.RecordFailure()
		return err
	}

	cb.RecordSuccess()
	return nil
}

// RecordSuccess 记录成功
func (cb *CircuitBreaker) RecordSuccess() {
	cb.successes.Add(1)
	cb.window.Record(true)
	cb.failures.Store(0)

	state := cb.currentState()
	if state == StateHalfOpen {
		// 半开状态连续成功 → 恢复
		cb.transitionTo(StateClosed, "probe_success")
		cb.halfOpenProbes.Store(0)
		cb.consecutiveOpens.Store(0)
	}
}

// RecordFailure 记录失败
func (cb *CircuitBreaker) RecordFailure() {
	cb.failures.Add(1)
	cb.window.Record(false)
	cb.lastFailAt.Store(time.Now().UnixNano())

	state := cb.currentState()

	switch state {
	case StateClosed:
		// 检查滑动窗口失败率
		if cb.shouldTrip() {
			cb.transitionTo(StateOpen, "failure_threshold")
		}
	case StateHalfOpen:
		// 半开状态失败 → 重新打开
		cb.transitionTo(StateOpen, "probe_failure")
		cb.halfOpenProbes.Store(0)
	}
}

// shouldTrip 判断是否应该触发熔断
func (cb *CircuitBreaker) shouldTrip() bool {
	// 连续失败检查
	if cb.failures.Load() >= int64(cb.config.MaxFailures) {
		return true
	}

	// 滑动窗口失败率检查
	stats := cb.window.Stats()
	if stats.Total > 0 && float64(stats.Failures)/float64(stats.Total) > 0.5 {
		return true
	}

	return false
}

// transitionTo 状态转换
func (cb *CircuitBreaker) transitionTo(to State, reason string) {
	from := cb.currentState()
	if from == to {
		return
	}

	cb.state.Store(int32(to))

	if to == StateOpen {
		cb.lastOpenAt.Store(time.Now().UnixNano())
		cb.consecutiveOpens.Add(1)
		cb.adaptResetTimeout()
	}

	// 回调
	if cb.onStateChange != nil {
		cb.onStateChange(cb.config.Name, from, to)
	}
	if to == StateOpen && cb.onTrip != nil {
		cb.onTrip(cb.config.Name, reason)
	}
}

// adaptResetTimeout 自适应调整恢复超时
func (cb *CircuitBreaker) adaptResetTimeout() {
	if !cb.config.AdaptiveEnabled {
		return
	}

	opens := cb.consecutiveOpens.Load()
	// 指数退避：每次熔断，恢复超时翻倍
	newTimeout := cb.config.ResetTimeout
	for i := int64(1); i < opens; i++ {
		newTimeout *= 2
		if newTimeout >= cb.config.MaxResetTimeout {
			newTimeout = cb.config.MaxResetTimeout
			break
		}
	}

	cb.mu.Lock()
	cb.config.ResetTimeout = newTimeout
	cb.mu.Unlock()
}

// currentState 获取当前状态
func (cb *CircuitBreaker) currentState() State {
	return State(cb.state.Load())
}

// openTime 获取上次打开时间
func (cb *CircuitBreaker) openTime() time.Time {
	n := cb.lastOpenAt.Load()
	if n == 0 {
		return time.Time{}
	}
	return time.Unix(0, n)
}

// State 获取状态（公开方法）
func (cb *CircuitBreaker) State() State {
	return cb.currentState()
}

// Stats 获取熔断器统计
func (cb *CircuitBreaker) Stats() BreakerStats {
	ws := cb.window.Stats()
	return BreakerStats{
		Name:             cb.config.Name,
		State:            cb.currentState().String(),
		Failures:         cb.failures.Load(),
		Successes:        cb.successes.Load(),
		WindowFailures:   ws.Failures,
		WindowSuccesses:  ws.Successes,
		WindowTotal:      ws.Total,
		WindowFailRate:   ws.FailRate(),
		ResetTimeout:     cb.config.ResetTimeout.String(),
		ConsecutiveOpens: cb.consecutiveOpens.Load(),
	}
}

// OnStateChange 注册状态变更回调
func (cb *CircuitBreaker) OnStateChange(fn func(name string, from, to State)) {
	cb.onStateChange = fn
}

// OnTrip 注册熔断触发回调
func (cb *CircuitBreaker) OnTrip(fn func(name string, reason string)) {
	cb.onTrip = fn
}

// Reset 手动重置
func (cb *CircuitBreaker) Reset() {
	cb.failures.Store(0)
	cb.successes.Store(0)
	cb.halfOpenProbes.Store(0)
	cb.consecutiveOpens.Store(0)
	cb.window.Reset()
	cb.transitionTo(StateClosed, "manual_reset")
}

// Trip 手动触发熔断
func (cb *CircuitBreaker) Trip(reason string) {
	cb.transitionTo(StateOpen, reason)
}

// ============================================================
// 滑动窗口
// ============================================================

// SlidingWindow 滑动窗口统计
type SlidingWindow struct {
	buckets    []bucket
	bucketSize time.Duration
	bucketCount int
	currentIdx atomic.Int64
	lastRotate atomic.Int64 // unix nano
	mu         sync.Mutex
}

type bucket struct {
	successes int64
	failures  int64
}

// WindowStats 窗口统计
type WindowStats struct {
	Successes int64   `json:"successes"`
	Failures  int64   `json:"failures"`
	Total     int64   `json:"total"`
}

// FailRate 失败率
func (ws WindowStats) FailRate() float64 {
	if ws.Total == 0 {
		return 0
	}
	return float64(ws.Failures) / float64(ws.Total)
}

// NewSlidingWindow 创建滑动窗口
func NewSlidingWindow(windowTime time.Duration, bucketCount int) *SlidingWindow {
	if bucketCount <= 0 {
		bucketCount = 10
	}
	return &SlidingWindow{
		buckets:    make([]bucket, bucketCount),
		bucketSize: windowTime / time.Duration(bucketCount),
		bucketCount: bucketCount,
	}
}

// Record 记录结果
func (sw *SlidingWindow) Record(success bool) {
	sw.rotate()
	idx := sw.currentIdx.Load() % int64(sw.bucketCount)
	if success {
		sw.buckets[idx].successes++
	} else {
		sw.buckets[idx].failures++
	}
}

// rotate 旋转桶
func (sw *SlidingWindow) rotate() {
	now := time.Now().UnixNano()
	last := sw.lastRotate.Load()

	if last == 0 {
		sw.lastRotate.Store(now)
		return
	}

	elapsed := time.Duration(now - last)
	bucketsToRotate := int(elapsed / sw.bucketSize)

	if bucketsToRotate > 0 {
		sw.mu.Lock()
		for i := 0; i < bucketsToRotate && i < sw.bucketCount; i++ {
			sw.currentIdx.Add(1)
			idx := sw.currentIdx.Load() % int64(sw.bucketCount)
			sw.buckets[idx] = bucket{}
		}
		sw.lastRotate.Store(now)
		sw.mu.Unlock()
	}
}

// Stats 获取窗口统计
func (sw *SlidingWindow) Stats() WindowStats {
	sw.rotate()

	sw.mu.Lock()
	defer sw.mu.Unlock()

	var stats WindowStats
	for _, b := range sw.buckets {
		stats.Successes += b.successes
		stats.Failures += b.failures
	}
	stats.Total = stats.Successes + stats.Failures
	return stats
}

// Reset 重置窗口
func (sw *SlidingWindow) Reset() {
	sw.mu.Lock()
	defer sw.mu.Unlock()
	for i := range sw.buckets {
		sw.buckets[i] = bucket{}
	}
	sw.currentIdx.Store(0)
	sw.lastRotate.Store(0)
}

// ============================================================
// 熔断器统计
// ============================================================

// BreakerStats 熔断器统计信息
type BreakerStats struct {
	Name             string `json:"name"`
	State            string `json:"state"`
	Failures         int64  `json:"failures"`
	Successes        int64  `json:"successes"`
	WindowFailures   int64  `json:"window_failures"`
	WindowSuccesses  int64  `json:"window_successes"`
	WindowTotal      int64  `json:"window_total"`
	WindowFailRate   float64 `json:"window_fail_rate"`
	ResetTimeout     string `json:"reset_timeout"`
	ConsecutiveOpens int64  `json:"consecutive_opens"`
}

// ============================================================
// 熔断器管理器
// ============================================================

// Manager 熔断器管理器
type Manager struct {
	breakers map[string]*CircuitBreaker
	mu       sync.RWMutex
}

// NewManager 创建熔断器管理器
func NewManager() *Manager {
	return &Manager{
		breakers: make(map[string]*CircuitBreaker),
	}
}

// GetOrCreate 获取或创建熔断器
func (m *Manager) GetOrCreate(name string, cfg *Config) *CircuitBreaker {
	m.mu.RLock()
	cb, ok := m.breakers[name]
	m.mu.RUnlock()

	if ok {
		return cb
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	// 双重检查
	if cb, ok = m.breakers[name]; ok {
		return cb
	}

	if cfg == nil {
		cfg = DefaultConfig(name)
	}
	cfg.Name = name

	cb = NewCircuitBreaker(cfg)
	m.breakers[name] = cb
	return cb
}

// Get 获取熔断器
func (m *Manager) Get(name string) (*CircuitBreaker, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	cb, ok := m.breakers[name]
	return cb, ok
}

// GetAll 获取所有熔断器
func (m *Manager) GetAll() map[string]*CircuitBreaker {
	m.mu.RLock()
	defer m.mu.RUnlock()
	result := make(map[string]*CircuitBreaker, len(m.breakers))
	for k, v := range m.breakers {
		result[k] = v
	}
	return result
}

// AllStats 获取所有熔断器统计
func (m *Manager) AllStats() map[string]BreakerStats {
	m.mu.RLock()
	defer m.mu.RUnlock()
	result := make(map[string]BreakerStats, len(m.breakers))
	for name, cb := range m.breakers {
		result[name] = cb.Stats()
	}
	return result
}

// ============================================================
// 错误定义
// ============================================================

// ErrCircuitOpen 熔断器打开错误
var ErrCircuitOpen = fmt.Errorf("circuit breaker is open")

// IsErrCircuitOpen 判断是否为熔断打开错误
func IsErrCircuitOpen(err error) bool {
	return err == ErrCircuitOpen
}
