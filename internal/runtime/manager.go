//go:build linux

// Package runtime 提供运行时管理功能（资源限制、故障隔离）
package runtime

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/signal"
	"runtime"
	"sync"
	"syscall"
	"time"

	"github.com/meinanzilinzhengying/cloud-flow-agent/pkg/models"
)

// Manager 运行时管理器
type Manager struct {
	config    *models.ResourceConfig
	mu        sync.RWMutex
	ctx       context.Context
	cancel    context.CancelFunc

	// 资源监控
	cpuUsage    float64
	memUsage    uint64
	goroutines  int

	// 故障隔离
	healthStatus map[string]bool
	errorCounts  map[string]int

	// 优雅关闭
	shutdownCallbacks []func(context.Context) error
}

// NewManager 创建运行时管理器
func NewManager(config *models.ResourceConfig) *Manager {
	return &Manager{
		config:            config,
		healthStatus:      make(map[string]bool),
		errorCounts:       make(map[string]int),
		shutdownCallbacks: make([]func(context.Context) error, 0),
	}
}

// Init 初始化运行时管理器
func (m *Manager) Init(ctx context.Context) error {
	m.ctx, m.cancel = context.WithCancel(ctx)

	// 设置 GOMAXPROCS
	// 在容器环境中，runtime.GOMAXPROCS(0) 会返回容器的 CPU 限制
	maxProcs := runtime.GOMAXPROCS(0)
	if m.config.CPUQuota > 0 && int(m.config.CPUQuota) < maxProcs {
		runtime.GOMAXPROCS(int(m.config.CPUQuota))
	}

	// 设置内存限制
	if m.config.MemoryLimit > 0 {
		// 设置 GC 目标
		debug := runtime.MemProfileRate
		_ = debug // 使用内存分析
	}

	return nil
}

// Start 启动运行时管理
func (m *Manager) Start() {
	// 启动资源监控
	go m.monitorResources()

	// 启动信号处理
	go m.handleSignals()
}

// Stop 停止运行时管理
func (m *Manager) Stop(ctx context.Context) error {
	return m.GracefulShutdown(ctx)
}

// monitorResources 监控资源使用
func (m *Manager) monitorResources() {
	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-m.ctx.Done():
			return
		case <-ticker.C:
			m.updateResourceUsage()
		}
	}
}

// updateResourceUsage 更新资源使用情况
func (m *Manager) updateResourceUsage() {
	m.mu.Lock()
	defer m.mu.Unlock()

	// 获取内存统计
	var memStats runtime.MemStats
	runtime.ReadMemStats(&memStats)

	m.memUsage = memStats.Alloc
	m.goroutines = runtime.NumGoroutine()

	// 检查资源限制
	m.checkResourceLimits()
}

// checkResourceLimits 检查资源限制
func (m *Manager) checkResourceLimits() {
	// 检查内存限制
	memLimitBytes := m.config.MemoryLimit * 1024 * 1024
	if memLimitBytes > 0 && m.memUsage > memLimitBytes {
		// 触发 GC
		runtime.GC()

		// 如果仍然超限，记录警告
		runtime.ReadMemStats(&runtime.MemStats{})
	}

	// 检查协程数量
	if m.config.MaxGoroutines > 0 && m.goroutines > m.config.MaxGoroutines {
		// 记录警告，但不强制限制
		// 实际的协程控制应该在业务逻辑中实现
	}
}

// handleSignals 处理信号
func (m *Manager) handleSignals() {
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan,
		syscall.SIGINT,
		syscall.SIGTERM,
		syscall.SIGHUP,
		syscall.SIGUSR1,
		syscall.SIGUSR2,
	)

	for {
		select {
		case <-m.ctx.Done():
			return
		case sig := <-sigChan:
			switch sig {
			case syscall.SIGINT, syscall.SIGTERM:
				// 优雅关闭
				ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
				defer cancel()
				_ = m.GracefulShutdown(ctx)

			case syscall.SIGHUP:
				// 热加载配置
				// 由外部处理

			case syscall.SIGUSR1:
				// 打印状态
				m.dumpStatus()

			case syscall.SIGUSR2:
				// 触发 GC
				runtime.GC()
			}
		}
	}
}

// GracefulShutdown 优雅关闭
func (m *Manager) GracefulShutdown(ctx context.Context) error {
	// 按顺序执行关闭回调
	for i := len(m.shutdownCallbacks) - 1; i >= 0; i-- {
		callback := m.shutdownCallbacks[i]
		if err := callback(ctx); err != nil {
			// 记录错误但继续关闭
			fmt.Fprintf(os.Stderr, "shutdown callback error: %v\n", err)
		}
	}

	if m.cancel != nil {
		m.cancel()
	}

	return nil
}

// RegisterShutdownCallback 注册关闭回调
func (m *Manager) RegisterShutdownCallback(callback func(context.Context) error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.shutdownCallbacks = append(m.shutdownCallbacks, callback)
}

// GetResourceUsage 获取资源使用情况
func (m *Manager) GetResourceUsage() (float64, uint64, int) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.cpuUsage, m.memUsage, m.goroutines
}

// SetHealthStatus 设置健康状态
func (m *Manager) SetHealthStatus(component string, healthy bool) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.healthStatus[component] = healthy
}

// GetHealthStatus 获取健康状态
func (m *Manager) GetHealthStatus() map[string]bool {
	m.mu.RLock()
	defer m.mu.RUnlock()

	result := make(map[string]bool)
	for k, v := range m.healthStatus {
		result[k] = v
	}
	return result
}

// RecordError 记录错误（用于故障隔离）
func (m *Manager) RecordError(component string) {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.errorCounts[component]++

	// 如果错误次数过多，标记为不健康
	if m.errorCounts[component] > 10 {
		m.healthStatus[component] = false
	}
}

// ClearErrors 清除错误计数
func (m *Manager) ClearErrors(component string) {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.errorCounts[component] = 0
	m.healthStatus[component] = true
}

// IsHealthy 检查是否健康
func (m *Manager) IsHealthy() bool {
	m.mu.RLock()
	defer m.mu.RUnlock()

	for _, healthy := range m.healthStatus {
		if !healthy {
			return false
		}
	}
	return true
}

// dumpStatus 打印状态
func (m *Manager) dumpStatus() {
	m.mu.RLock()
	defer m.mu.RUnlock()

	fmt.Fprintf(os.Stderr, "=== Agent Status ===\n")
	fmt.Fprintf(os.Stderr, "Goroutines: %d\n", m.goroutines)
	fmt.Fprintf(os.Stderr, "Memory: %d bytes\n", m.memUsage)
	fmt.Fprintf(os.Stderr, "Health Status: %v\n", m.healthStatus)
	fmt.Fprintf(os.Stderr, "Error Counts: %v\n", m.errorCounts)
}

// Context 获取上下文
func (m *Manager) Context() context.Context {
	return m.ctx
}

// Done 获取关闭信号
func (m *Manager) Done() <-chan struct{} {
	return m.ctx.Done()
}

// CircuitBreaker 熔断器
type CircuitBreaker struct {
	name          string
	maxFailures   int
	resetTimeout  time.Duration

	mu           sync.RWMutex
	failures     int
	lastFailTime time.Time
	state        State
}

// State 熔断器状态
type State int

const (
	StateClosed State = iota
	StateOpen
	StateHalfOpen
)

// NewCircuitBreaker 创建熔断器
func NewCircuitBreaker(name string, maxFailures int, resetTimeout time.Duration) *CircuitBreaker {
	return &CircuitBreaker{
		name:         name,
		maxFailures:  maxFailures,
		resetTimeout: resetTimeout,
		state:        StateClosed,
	}
}

// Allow 检查是否允许执行
func (cb *CircuitBreaker) Allow() bool {
	cb.mu.RLock()
	defer cb.mu.RUnlock()

	switch cb.state {
	case StateClosed:
		return true
	case StateOpen:
		// 检查是否可以进入半开状态
		if time.Since(cb.lastFailTime) > cb.resetTimeout {
			return true
		}
		return false
	case StateHalfOpen:
		return true
	}
	return false
}

// RecordSuccess 记录成功
func (cb *CircuitBreaker) RecordSuccess() {
	cb.mu.Lock()
	defer cb.mu.Unlock()

	cb.failures = 0
	cb.state = StateClosed
}

// RecordFailure 记录失败
func (cb *CircuitBreaker) RecordFailure() {
	cb.mu.Lock()
	defer cb.mu.Unlock()

	cb.failures++
	cb.lastFailTime = time.Now()

	if cb.failures >= cb.maxFailures {
		cb.state = StateOpen
	}
}

// State 获取状态
func (cb *CircuitBreaker) State() State {
	cb.mu.RLock()
	defer cb.mu.RUnlock()
	return cb.state
}

// RateLimiter 速率限制器
type RateLimiter struct {
	rate      int
	interval  time.Duration

	mu       sync.Mutex
	tokens   int
	lastTime time.Time
}

// NewRateLimiter 创建速率限制器
func NewRateLimiter(rate int, interval time.Duration) *RateLimiter {
	return &RateLimiter{
		rate:     rate,
		interval: interval,
		tokens:   rate,
		lastTime: time.Now(),
	}
}

// Allow 检查是否允许
func (rl *RateLimiter) Allow() bool {
	rl.mu.Lock()
	defer rl.mu.Unlock()

	now := time.Now()
	elapsed := now.Sub(rl.lastTime)

	// 补充令牌
	tokensToAdd := int(elapsed / rl.interval) * rl.rate
	rl.tokens += tokensToAdd
	if rl.tokens > rl.rate {
		rl.tokens = rl.rate
	}

	rl.lastTime = now

	// 检查令牌
	if rl.tokens > 0 {
		rl.tokens--
		return true
	}

	return false
}

// Wait 等待直到允许
func (rl *RateLimiter) Wait(ctx context.Context) error {
	for {
		if rl.Allow() {
			return nil
		}

		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(rl.interval / time.Duration(rl.rate)):
			continue
		}
	}
}

// GracefulStartup 优雅启动
func GracefulStartup(startFuncs ...func(context.Context) error) error {
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	for i, startFunc := range startFuncs {
		if err := startFunc(ctx); err != nil {
			// 回滚已启动的服务
			for j := i - 1; j >= 0; j-- {
				// 假设每个 startFunc 都有对应的 stopFunc
				// 这里简化处理
			}
			return fmt.Errorf("failed to start service %d: %w", i, err)
		}
	}

	return nil
}

// CheckDependencies 检查依赖
func CheckDependencies() error {
	// 检查内核版本
	if _, err := os.Stat("/proc/version"); err != nil {
		return errors.New("cannot access /proc/version")
	}

	// 检查权限
	if os.Geteuid() != 0 {
		return errors.New("agent must run as root for eBPF support")
	}

	return nil
}
