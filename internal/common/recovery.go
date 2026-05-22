/*
 * Copyright (c) 2025 Yunlong Liao. All rights reserved.
 */

package common

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"runtime"
	"runtime/debug"
	"sync"
	"syscall"
	"time"

	"cloud-flow-agent/pkg/logger"
)

// RecoveryHandler 恢复处理器
type RecoveryHandler func(recover interface{}, stack []byte)

// PanicRecovery 全局Panic恢复
type PanicRecovery struct {
	handlers []RecoveryHandler
	mu       sync.RWMutex
}

// NewPanicRecovery 创建Panic恢复器
func NewPanicRecovery() *PanicRecovery {
	return &PanicRecovery{
		handlers: make([]RecoveryHandler, 0),
	}
}

// RegisterHandler 注册恢复处理器
func (r *PanicRecovery) RegisterHandler(handler RecoveryHandler) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.handlers = append(r.handlers, handler)
}

// Recover 执行恢复
func (r *PanicRecovery) Recover() {
	if rec := recover(); rec != nil {
		stack := debug.Stack()
		logger.Errorf("Panic recovered: %v\nStack:\n%s", rec, string(stack))

		r.mu.RLock()
		handlers := make([]RecoveryHandler, len(r.handlers))
		copy(handlers, r.handlers)
		r.mu.RUnlock()

		for _, handler := range handlers {
			func() {
				defer func() {
					if rec := recover(); rec != nil {
						logger.Errorf("Panic in recovery handler: %v", rec)
					}
				}()
				handler(rec, stack)
			}()
		}
	}
}

// Go 安全地启动goroutine
func (r *PanicRecovery) Go(f func()) {
	go func() {
		defer r.Recover()
		f()
	}()
}

// SafeFunc 包装函数为安全函数
func (r *PanicRecovery) SafeFunc(f func()) func() {
	return func() {
		defer r.Recover()
		f()
	}
}

// GlobalRecovery 全局恢复实例
var GlobalRecovery = NewPanicRecovery()

// SafeGo 使用全局恢复启动goroutine
func SafeGo(f func()) {
	GlobalRecovery.Go(f)
}

// SafeCall 安全调用函数
func SafeCall(f func()) (err error) {
	defer func() {
		if rec := recover(); rec != nil {
			stack := debug.Stack()
			err = fmt.Errorf("panic: %v\nstack: %s", rec, string(stack))
			logger.Errorf("SafeCall panic: %v", err)
		}
	}()
	f()
	return nil
}

// GracefulShutdown 优雅关闭
type GracefulShutdown struct {
	components []ShutdownComponent
	timeout    time.Duration
	mu         sync.RWMutex
}

// ShutdownComponent 关闭组件接口
type ShutdownComponent interface {
	Name() string
	Shutdown(ctx context.Context) error
}

// ShutdownFunc 关闭函数
type ShutdownFunc func(ctx context.Context) error

// Name 返回组件名
func (f ShutdownFunc) Name() string {
	return "anonymous"
}

// Shutdown 执行关闭
func (f ShutdownFunc) Shutdown(ctx context.Context) error {
	return f(ctx)
}

// NewGracefulShutdown 创建优雅关闭管理器
func NewGracefulShutdown(timeout time.Duration) *GracefulShutdown {
	if timeout == 0 {
		timeout = 30 * time.Second
	}
	return &GracefulShutdown{
		components: make([]ShutdownComponent, 0),
		timeout:    timeout,
	}
}

// Register 注册关闭组件
func (g *GracefulShutdown) Register(component ShutdownComponent) {
	g.mu.Lock()
	defer g.mu.Unlock()
	g.components = append(g.components, component)
	logger.Infof("Registered shutdown component: %s", component.Name())
}

// RegisterFunc 注册关闭函数
func (g *GracefulShutdown) RegisterFunc(name string, fn ShutdownFunc) {
	g.Register(&namedShutdownComponent{name: name, fn: fn})
}

// namedShutdownComponent 命名关闭组件
type namedShutdownComponent struct {
	name string
	fn   ShutdownFunc
}

func (n *namedShutdownComponent) Name() string {
	return n.name
}

func (n *namedShutdownComponent) Shutdown(ctx context.Context) error {
	return n.fn(ctx)
}

// WaitForSignal 等待关闭信号
func (g *GracefulShutdown) WaitForSignal() {
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM, syscall.SIGQUIT)

	sig := <-sigCh
	logger.Infof("Received signal: %v, starting graceful shutdown...", sig)

	if err := g.Shutdown(); err != nil {
		logger.Errorf("Graceful shutdown failed: %v", err)
		os.Exit(1)
	}

	logger.Info("Graceful shutdown completed")
	os.Exit(0)
}

// Shutdown 执行优雅关闭
func (g *GracefulShutdown) Shutdown() error {
	g.mu.RLock()
	components := make([]ShutdownComponent, len(g.components))
	copy(components, g.components)
	g.mu.RUnlock()

	ctx, cancel := context.WithTimeout(context.Background(), g.timeout)
	defer cancel()

	var wg sync.WaitGroup
	errCh := make(chan error, len(components))

	for _, component := range components {
		wg.Add(1)
		go func(c ShutdownComponent) {
			defer wg.Done()
			defer func() {
				if rec := recover(); rec != nil {
					errCh <- fmt.Errorf("panic in shutdown %s: %v", c.Name(), rec)
				}
			}()

			logger.Infof("Shutting down component: %s", c.Name())
			if err := c.Shutdown(ctx); err != nil {
				errCh <- fmt.Errorf("failed to shutdown %s: %w", c.Name(), err)
			}
		}(component)
	}

	// 等待所有组件关闭或超时
	done := make(chan struct{})
	go func() {
		wg.Wait()
		close(done)
	}()

	select {
	case <-done:
		close(errCh)
	case <-ctx.Done():
		logger.Warnf("Shutdown timeout after %v", g.timeout)
		return fmt.Errorf("shutdown timeout")
	}

	// 收集错误
	var errs []error
	for err := range errCh {
		if err != nil {
			errs = append(errs, err)
		}
	}

	if len(errs) > 0 {
		return fmt.Errorf("shutdown errors: %v", errs)
	}

	return nil
}

// ErrorHandler 错误处理器
type ErrorHandler struct {
	handlers map[string]func(error)
	mu       sync.RWMutex
}

// NewErrorHandler 创建错误处理器
func NewErrorHandler() *ErrorHandler {
	return &ErrorHandler{
		handlers: make(map[string]func(error)),
	}
}

// Register 注册错误处理器
func (h *ErrorHandler) Register(errorType string, handler func(error)) {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.handlers[errorType] = handler
}

// Handle 处理错误
func (h *ErrorHandler) Handle(err error) {
	if err == nil {
		return
	}

	logger.Errorf("Error: %v", err)

	h.mu.RLock()
	handlers := make(map[string]func(error))
	for k, v := range h.handlers {
		handlers[k] = v
	}
	h.mu.RUnlock()

	// 根据错误类型调用对应处理器
	for errorType, handler := range handlers {
		if isErrorType(err, errorType) {
			handler(err)
			return
		}
	}

	// 默认处理
	logger.Errorf("Unhandled error: %v", err)
}

// isErrorType 检查错误类型
func isErrorType(err error, errorType string) bool {
	// 简化实现，实际应根据错误类型判断
	return true
}

// GlobalErrorHandler 全局错误处理器
var GlobalErrorHandler = NewErrorHandler()

// HandleError 使用全局处理器处理错误
func HandleError(err error) {
	GlobalErrorHandler.Handle(err)
}

// CircuitBreaker 熔断器
type CircuitBreaker struct {
	name          string
	maxFailures   int
	resetTimeout  time.Duration
	failures      int
	lastFailure   time.Time
	state         CircuitState
	mu            sync.RWMutex
}

// CircuitState 熔断状态
type CircuitState int

const (
	StateClosed CircuitState = iota   // 关闭（正常）
	StateOpen                         // 打开（熔断）
	StateHalfOpen                     // 半开（尝试恢复）
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

// Call 执行带熔断保护的调用
func (cb *CircuitBreaker) Call(f func() error) error {
	cb.mu.Lock()
	state := cb.state
	cb.mu.Unlock()

	switch state {
	case StateOpen:
		if time.Since(cb.lastFailure) > cb.resetTimeout {
			cb.mu.Lock()
			cb.state = StateHalfOpen
			cb.mu.Unlock()
			logger.Infof("Circuit breaker %s entering half-open state", cb.name)
		} else {
			return fmt.Errorf("circuit breaker %s is open", cb.name)
		}
	}

	err := f()
	cb.recordResult(err)
	return err
}

// recordResult 记录调用结果
func (cb *CircuitBreaker) recordResult(err error) {
	cb.mu.Lock()
	defer cb.mu.Unlock()

	if err == nil {
		// 成功
		if cb.state == StateHalfOpen {
			cb.state = StateClosed
			cb.failures = 0
			logger.Infof("Circuit breaker %s closed", cb.name)
		}
		return
	}

	// 失败
	cb.failures++
	cb.lastFailure = time.Now()

	if cb.failures >= cb.maxFailures {
		cb.state = StateOpen
		logger.Warnf("Circuit breaker %s opened after %d failures", cb.name, cb.failures)
	}
}

// GetState 获取熔断器状态
func (cb *CircuitBreaker) GetState() CircuitState {
	cb.mu.RLock()
	defer cb.mu.RUnlock()
	return cb.state
}

// ResourceLimiter 资源限制器
type ResourceLimiter struct {
	maxGoroutines int
	semaphore     chan struct{}
}

// NewResourceLimiter 创建资源限制器
func NewResourceLimiter(maxGoroutines int) *ResourceLimiter {
	return &ResourceLimiter{
		maxGoroutines: maxGoroutines,
		semaphore:     make(chan struct{}, maxGoroutines),
	}
}

// Acquire 获取资源
func (rl *ResourceLimiter) Acquire(timeout time.Duration) bool {
	select {
	case rl.semaphore <- struct{}{}:
		return true
	case <-time.After(timeout):
		return false
	}
}

// Release 释放资源
func (rl *ResourceLimiter) Release() {
	select {
	case <-rl.semaphore:
	default:
	}
}

// Execute 执行受限资源操作
func (rl *ResourceLimiter) Execute(timeout time.Duration, f func()) bool {
	if !rl.Acquire(timeout) {
		return false
	}
	defer rl.Release()

	f()
	return true
}

// SystemMonitor 系统监控
type SystemMonitor struct {
	stopCh chan struct{}
	wg     sync.WaitGroup
}

// NewSystemMonitor 创建系统监控
func NewSystemMonitor() *SystemMonitor {
	return &SystemMonitor{
		stopCh: make(chan struct{}),
	}
}

// Start 启动监控
func (sm *SystemMonitor) Start() {
	sm.wg.Add(1)
	go sm.monitorLoop()
}

// Stop 停止监控
func (sm *SystemMonitor) Stop() {
	close(sm.stopCh)
	sm.wg.Wait()
}

// monitorLoop 监控循环
func (sm *SystemMonitor) monitorLoop() {
	defer sm.wg.Done()

	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-sm.stopCh:
			return
		case <-ticker.C:
			sm.checkSystemHealth()
		}
	}
}

// checkSystemHealth 检查系统健康
func (sm *SystemMonitor) checkSystemHealth() {
	var m runtime.MemStats
	runtime.ReadMemStats(&m)

	logger.Debugf("System Stats - Goroutines: %d, Memory: %d MB, GC: %d",
		runtime.NumGoroutine(),
		m.Alloc/1024/1024,
		m.NumGC,
	)

	// 检查goroutine泄漏
	if runtime.NumGoroutine() > 10000 {
		logger.Warnf("High goroutine count: %d", runtime.NumGoroutine())
	}

	// 检查内存使用
	if m.Alloc > 1024*1024*1024 { // 1GB
		logger.Warnf("High memory usage: %d MB", m.Alloc/1024/1024)
		runtime.GC() // 触发GC
	}
}
