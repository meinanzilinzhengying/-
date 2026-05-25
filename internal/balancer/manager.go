/*
 * Cloud Flow Agent - Edge Service Load Balancer
 *
 * 边缘服务负载均衡，支持多节点横向扩展
 */

package balancer

import (
	"context"
	"errors"
	"fmt"
	"math"
	"sync"
	"sync/atomic"
	"time"
)

var (
	ErrNoBackend       = errors.New("no available backend")
	ErrBackendUnhealthy = errors.New("backend is unhealthy")
)

// LBAlgorithm 负载均衡算法
type LBAlgorithm string

const (
	LBAlgorithmRoundRobin LBAlgorithm = "round_robin"
	LBAlgorithmWeighted   LBAlgorithm = "weighted"
	LBAlgorithmLeastConn  LBAlgorithm = "least_conn"
	LBAlgorithmIPHash     LBAlgorithm = "ip_hash"
	LBAlgorithmConsistent LBAlgorithm = "consistent_hash"
)

// LBConfig 负载均衡配置
type LBConfig struct {
	Enabled         bool          // 启用负载均衡
	Algorithm       LBAlgorithm   // 负载均衡算法
	HealthCheck     bool          // 启用健康检查
	CheckInterval   time.Duration // 健康检查间隔
	CheckTimeout    time.Duration // 健康检查超时
	CheckPath       string        // 健康检查路径
	CheckPort       int           // 健康检查端口
	
	// 熔断配置
	CircuitBreaker bool          // 启用熔断
	CBThreshold    int           // 熔断阈值（连续失败次数）
	CBTimeout      time.Duration // 熔断超时
	
	// 限流配置
	RateLimitEnabled bool         // 启用限流
	RateLimitPerSec  int          // 每秒请求限制
	BurstSize       int          // 突发大小
	
	// 连接池配置
	MaxConnPerBackend int         // 每个后端最大连接数
	ConnTimeout      time.Duration // 连接超时
	
	// 节点权重
	EnableWeighted  bool          // 启用权重
}

// DefaultLBConfig 默认配置
func DefaultLBConfig() *LBConfig {
	return &LBConfig{
		Enabled:          true,
		Algorithm:        LBAlgorithmWeighted,
		HealthCheck:      true,
		CheckInterval:    10 * time.Second,
		CheckTimeout:     5 * time.Second,
		CheckPath:        "/health",
		CheckPort:        8080,
		CircuitBreaker:   true,
		CBThreshold:      5,
		CBTimeout:        30 * time.Second,
		RateLimitEnabled: true,
		RateLimitPerSec:  100000,
		BurstSize:        1000,
		MaxConnPerBackend: 100,
		ConnTimeout:      10 * time.Second,
		EnableWeighted:   true,
	}
}

// Backend 后端节点
type Backend struct {
	ID          string        `json:"id"`
	Address     string        `json:"address"`
	Port        int           `json:"port"`
	Weight      int           `json:"weight"` // 权重 1-100
	Enabled     bool          `json:"enabled"`
	Health      bool          `json:"health"`
	
	// 统计
	Stats        BackendStats
	circuitState CircuitState
	lastCheck    time.Time
	failCount    int
	successCount int
	
	// 连接池
	connPool *BackendConnPool
}

// BackendStats 后端统计
type BackendStats struct {
	TotalRequests   uint64
	TotalResponses  uint64
	TotalErrors     uint64
	TotalLatency    uint64 // 微秒
	ActiveConns     int32
	MaxConns        int32
	BytesSent       uint64
	BytesReceived   uint64
}

// BackendConnPool 后端连接池
type BackendConnPool struct {
	mu    sync.Mutex
	conns []*Conn
}

// Conn 连接
type Conn struct {
	ID       uint64
	Backend  *Backend
	InUse    bool
	CreatedAt time.Time
}

// CircuitState 熔断状态
type CircuitState int

const (
	CircuitClosed CircuitState = iota
	CircuitOpen
	CircuitHalfOpen
)

// BackendManager 后端管理器
type BackendManager struct {
	mu       sync.RWMutex
	config   *LBConfig
	backends map[string]*Backend
	watchers []func([]*Backend) // 变更通知
	
	// 轮询索引
	rrIndex int
	
	// 限流器
	rateLimiter *GlobalRateLimiter
	
	// 统计
	stats LBStats
	
	// 控制
	ctx    context.Context
	cancel context.CancelFunc
}

// LBStats 负载均衡统计
type LBStats struct {
	TotalRequests     uint64
	TotalResponses    uint64
	TotalErrors       uint64
	TotalRetries      uint64
	BackendChanges    uint64
	HealthCheckCalls  uint64
	CircuitBreakerOpens uint64
}

// GlobalRateLimiter 全局限流器
type GlobalRateLimiter struct {
	tokens    int
	maxTokens int
	refillAt  time.Time
	mu        sync.Mutex
}

// NewGlobalRateLimiter 创建限流器
func NewGlobalRateLimiter(maxPerSec, burst int) *GlobalRateLimiter {
	return &GlobalRateLimiter{
		tokens:    burst,
		maxTokens: burst,
		refillAt:  time.Now().Add(time.Second),
	}
}

// Allow 是否允许
func (rl *GlobalRateLimiter) Allow() bool {
	rl.mu.Lock()
	defer rl.mu.Unlock()
	
	if time.Now().After(rl.refillAt) {
		rl.tokens = rl.maxTokens
		rl.refillAt = time.Now().Add(time.Second)
	}
	
	if rl.tokens > 0 {
		rl.tokens--
		return true
	}
	return false
}

// NewBackendManager 创建后端管理器
func NewBackendManager(config *LBConfig) (*BackendManager, error) {
	if config == nil {
		config = DefaultLBConfig()
	}
	
	ctx, cancel := context.WithCancel(context.Background())
	
	mgr := &BackendManager{
		config:     config,
		backends:  make(map[string]*Backend),
		rateLimiter: NewGlobalRateLimiter(config.RateLimitPerSec, config.BurstSize),
		ctx:        ctx,
		cancel:     cancel,
	}
	
	// 启动健康检查
	if config.HealthCheck {
		go mgr.healthCheckLoop()
	}
	
	// 启动熔断恢复
	if config.CircuitBreaker {
		go mgr.circuitBreakerLoop()
	}
	
	return mgr, nil
}

// RegisterBackend 注册后端
func (m *BackendManager) RegisterBackend(backend *Backend) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	
	if _, exists := m.backends[backend.ID]; exists {
		return fmt.Errorf("backend %s already exists", backend.ID)
	}
	
	// 设置默认值
	if backend.Weight == 0 {
		backend.Weight = 100
	}
	backend.Health = true
	backend.Enabled = true
	backend.lastCheck = time.Now()
	backend.circuitState = CircuitClosed
	
	// 创建连接池
	backend.connPool = &BackendConnPool{
		conns: make([]*Conn, 0),
	}
	
	m.backends[backend.ID] = backend
	atomic.AddUint64(&m.stats.BackendChanges, 1)
	
	// 通知观察者
	m.notifyWatchers()
	
	return nil
}

// UnregisterBackend 注销后端
func (m *BackendManager) UnregisterBackend(id string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	
	if _, exists := m.backends[id]; !exists {
		return fmt.Errorf("backend %s not found", id)
	}
	
	delete(m.backends, id)
	atomic.AddUint64(&m.stats.BackendChanges, 1)
	
	// 通知观察者
	m.notifyWatchers()
	
	return nil
}

// UpdateBackendWeight 更新后端权重
func (m *BackendManager) UpdateBackendWeight(id string, weight int) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	
	backend, exists := m.backends[id]
	if !exists {
		return fmt.Errorf("backend %s not found", id)
	}
	
	backend.Weight = weight
	atomic.AddUint64(&m.stats.BackendChanges, 1)
	return nil
}

// EnableBackend 启用后端
func (m *BackendManager) EnableBackend(id string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	
	backend, exists := m.backends[id]
	if !exists {
		return fmt.Errorf("backend %s not found", id)
	}
	
	backend.Enabled = true
	backend.Health = true
	backend.circuitState = CircuitClosed
	atomic.AddUint64(&m.stats.BackendChanges, 1)
	return nil
}

// DisableBackend 禁用后端
func (m *BackendManager) DisableBackend(id string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	
	backend, exists := m.backends[id]
	if !exists {
		return fmt.Errorf("backend %s not found", id)
	}
	
	backend.Enabled = false
	backend.Health = false
	atomic.AddUint64(&m.stats.BackendChanges, 1)
	return nil
}

// SelectBackend 选择后端
func (m *BackendManager) SelectBackend(clientIP string) (*Backend, error) {
	// 全局限流检查
	if m.config.RateLimitEnabled && !m.rateLimiter.Allow() {
		return nil, ErrRateLimited
	}
	
	m.mu.Lock()
	defer m.mu.Unlock()
	
	// 获取可用后端
	candidates := m.getAvailableBackends()
	if len(candidates) == 0 {
		return nil, ErrNoBackend
	}
	
	// 根据算法选择
	var selected *Backend
	switch m.config.Algorithm {
	case LBAlgorithmRoundRobin:
		selected = m.selectRoundRobin(candidates)
	case LBAlgorithmWeighted:
		selected = m.selectWeighted(candidates)
	case LBAlgorithmLeastConn:
		selected = m.selectLeastConn(candidates)
	case LBAlgorithmIPHash:
		selected = m.selectIPHash(candidates, clientIP)
	default:
		selected = m.selectWeighted(candidates)
	}
	
	if selected == nil {
		return nil, ErrNoBackend
	}
	
	atomic.AddUint64(&m.stats.TotalRequests, 1)
	atomic.AddUint64(&selected.Stats.TotalRequests, 1)
	
	return selected, nil
}

// getAvailableBackends 获取可用后端列表
func (m *BackendManager) getAvailableBackends() []*Backend {
	backends := make([]*Backend, 0, len(m.backends))
	
	for _, backend := range m.backends {
		if !backend.Enabled {
			continue
		}
		
		// 检查熔断状态
		if m.config.CircuitBreaker {
			switch backend.circuitState {
			case CircuitOpen:
				continue
			case CircuitHalfOpen:
				// 半开状态允许请求
			case CircuitClosed:
				if !backend.Health && m.config.HealthCheck {
					continue
				}
			}
		} else if !backend.Health && m.config.HealthCheck {
			continue
		}
		
		backends = append(backends, backend)
	}
	
	return backends
}

// selectRoundRobin 轮询选择
func (m *BackendManager) selectRoundRobin(backends []*Backend) *Backend {
	// 简单轮询
	index := m.rrIndex % len(backends)
	m.rrIndex++
	return backends[index]
}

// selectWeighted 加权选择
func (m *BackendManager) selectWeighted(backends []*Backend) *Backend {
	if !m.config.EnableWeighted {
		return m.selectRoundRobin(backends)
	}
	
	// 计算总权重
	totalWeight := 0
	for _, b := range backends {
		totalWeight += b.Weight
	}
	
	if totalWeight == 0 {
		return m.selectRoundRobin(backends)
	}
	
	// 随机选择
	r := time.Now().UnixNano() % int64(totalWeight)
	cumulative := 0
	
	for _, backend := range backends {
		cumulative += backend.Weight
		if int64(cumulative) > r {
			return backend
		}
	}
	
	return backends[0]
}

// selectLeastConn 最少连接选择
func (m *BackendManager) selectLeastConn(backends []*Backend) *Backend {
	var selected *Backend
	minConns := int32(math.MaxInt32)
	
	for _, backend := range backends {
		conns := atomic.LoadInt32(&backend.Stats.ActiveConns)
		if conns < minConns {
			minConns = conns
			selected = backend
		}
	}
	
	return selected
}

// selectIPHash IP哈希选择
func (m *BackendManager) selectIPHash(backends []*Backend, clientIP string) *Backend {
	if len(backends) == 0 {
		return nil
	}
	
	// 简单哈希
	hash := 0
	for _, c := range clientIP {
		hash = hash*31 + int(c)
	}
	
	index := hash % len(backends)
	return backends[index]
}

// RecordSuccess 记录成功
func (m *BackendManager) RecordSuccess(backendID string, latency time.Duration) {
	m.mu.RLock()
	backend, exists := m.backends[backendID]
	m.mu.RUnlock()
	
	if !exists {
		return
	}
	
	atomic.AddUint64(&m.stats.TotalResponses, 1)
	atomic.AddUint64(&backend.Stats.TotalResponses, 1)
	atomic.AddUint64(&backend.Stats.TotalLatency, uint64(latency.Microseconds()))
	atomic.AddInt32(&backend.Stats.ActiveConns, -1)
	
	// 更新熔断状态
	if m.config.CircuitBreaker {
		m.mu.Lock()
		backend.successCount++
		if backend.successCount >= 3 {
			backend.circuitState = CircuitClosed
			backend.failCount = 0
			backend.successCount = 0
		}
		m.mu.Unlock()
	}
}

// RecordFailure 记录失败
func (m *BackendManager) RecordFailure(backendID string) {
	m.mu.RLock()
	backend, exists := m.backends[backendID]
	m.mu.RUnlock()
	
	if !exists {
		return
	}
	
	atomic.AddUint64(&m.stats.TotalErrors, 1)
	atomic.AddUint64(&backend.Stats.TotalErrors, 1)
	atomic.AddInt32(&backend.Stats.ActiveConns, -1)
	
	// 更新熔断状态
	if m.config.CircuitBreaker {
		m.mu.Lock()
		backend.failCount++
		backend.successCount = 0
		if backend.failCount >= m.config.CBThreshold {
			backend.circuitState = CircuitOpen
			atomic.AddUint64(&m.stats.CircuitBreakerOpens, 1)
		}
		m.mu.Unlock()
	}
}

// healthCheckLoop 健康检查循环
func (m *BackendManager) healthCheckLoop() {
	ticker := time.NewTicker(m.config.CheckInterval)
	defer ticker.Stop()
	
	for {
		select {
		case <-ticker.C:
			m.doHealthCheck()
		case <-m.ctx.Done():
			return
		}
	}
}

// doHealthCheck 执行健康检查
func (m *BackendManager) doHealthCheck() {
	m.mu.RLock()
	backends := make([]*Backend, 0, len(m.backends))
	for _, b := range m.backends {
		backends = append(backends, b)
	}
	m.mu.RUnlock()
	
	for _, backend := range backends {
		go m.checkBackend(backend)
	}
}

// checkBackend 检查单个后端
func (m *BackendManager) checkBackend(backend *Backend) {
	atomic.AddUint64(&m.stats.HealthCheckCalls, 1)
	
	// 简化的健康检查，实际应发送 HTTP 请求
	isHealthy := m.pingBackend(backend)
	
	m.mu.Lock()
	defer m.mu.Unlock()
	
	backend.lastCheck = time.Now()
	backend.Health = isHealthy
	
	if !isHealthy && backend.circuitState == CircuitClosed {
		// 健康检查失败，触发熔断
		backend.failCount++
		if backend.failCount >= m.config.CBThreshold {
			backend.circuitState = CircuitOpen
		}
	}
}

// pingBackend Ping 后端
func (m *BackendManager) pingBackend(backend *Backend) bool {
	// 实际实现应使用 HTTP GET 或 TCP 连接检查
	// 这里模拟健康检查
	return backend.Enabled
}

// circuitBreakerLoop 熔断恢复循环
func (m *BackendManager) circuitBreakerLoop() {
	ticker := time.NewTicker(m.config.CBTimeout / 2)
	defer ticker.Stop()
	
	for {
		select {
		case <-ticker.C:
			m.checkCircuitBreaker()
		case <-m.ctx.Done():
			return
		}
	}
}

// checkCircuitBreaker 检查熔断状态
func (m *BackendManager) checkCircuitBreaker() {
	m.mu.Lock()
	defer m.mu.Unlock()
	
	for _, backend := range m.backends {
		if backend.circuitState == CircuitOpen {
			// 超时后进入半开状态
			backend.circuitState = CircuitHalfOpen
			backend.failCount = 0
		}
	}
}

// RegisterWatcher 注册变更观察者
func (m *BackendManager) RegisterWatcher(watcher func([]*Backend)) {
	m.watchers = append(m.watchers, watcher)
}

// notifyWatchers 通知观察者
func (m *BackendManager) notifyWatchers() {
	backends := make([]*Backend, 0, len(m.backends))
	for _, b := range m.backends {
		backends = append(backends, b)
	}
	
	for _, watcher := range m.watchers {
		go watcher(backends)
	}
}

// GetBackends 获取所有后端
func (m *BackendManager) GetBackends() []*Backend {
	m.mu.RLock()
	defer m.mu.RUnlock()
	
	backends := make([]*Backend, 0, len(m.backends))
	for _, b := range m.backends {
		copy := *b
		backends = append(backends, &copy)
	}
	return backends
}

// GetStats 获取统计
func (m *BackendManager) GetStats() LBStats {
	return LBStats{
		TotalRequests:       atomic.LoadUint64(&m.stats.TotalRequests),
		TotalResponses:      atomic.LoadUint64(&m.stats.TotalResponses),
		TotalErrors:         atomic.LoadUint64(&m.stats.TotalErrors),
		TotalRetries:        atomic.LoadUint64(&m.stats.TotalRetries),
		BackendChanges:      atomic.LoadUint64(&m.stats.BackendChanges),
		HealthCheckCalls:   atomic.LoadUint64(&m.stats.HealthCheckCalls),
		CircuitBreakerOpens: atomic.LoadUint64(&m.stats.CircuitBreakerOpens),
	}
}

// Close 关闭
func (m *BackendManager) Close() error {
	m.cancel()
	return nil
}
