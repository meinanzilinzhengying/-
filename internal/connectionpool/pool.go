/*
 * Cloud Flow Agent - Connection Pool Manager
 *
 * 连接池管理器，支持大规模探针接入（5000+）
 * 实现连接复用、自适应限流、内存保护
 */

package connectionpool

import (
	"context"
	"errors"
	"fmt"
	"net"
	"sync"
	"sync/atomic"
	"time"
)

var (
	ErrPoolClosed    = errors.New("connection pool is closed")
	ErrConnBusy      = errors.New("connection is busy")
	ErrRateLimited   = errors.New("rate limited")
	ErrQueueFull     = errors.New("queue is full")
	ErrMemoryLimited = errors.New("memory limit reached")
)

// PoolConfig 连接池配置
type PoolConfig struct {
	// 基础配置
	MaxConnections    int           // 最大连接数
	InitialCap        int           // 初始连接数
	MaxIdle           int           // 最大空闲连接
	IdleTimeout       time.Duration // 空闲超时
	MaxLifetime       time.Duration // 最大生命周期

	// 限流配置
	EnableRateLimit   bool          // 启用限流
	RateLimitPerConn  int           // 每连接每秒限制
	BurstSize         int           // 突发流量大小
	GlobalRateLimit   int           // 全局每秒限制

	// 队列配置
	QueueSize         int           // 等待队列大小
	QueueTimeout      time.Duration // 队列等待超时

	// 内存保护
	MaxMemoryMB       int64         // 最大内存限制(MB)
	MemoryCheckInterval time.Duration // 内存检查间隔

	// 健康检查
	HealthCheckInterval time.Duration // 健康检查间隔
	HealthCheckTimeout  time.Duration // 健康检查超时

	// 自适应配置
	EnableAdaptive    bool          // 启用自适应调整
	ScaleUpThreshold  float64       // 扩容阈值(连接使用率)
	ScaleDownThreshold float64      // 缩容阈值
	ScaleUpFactor     int           // 扩容因子
	ScaleDownFactor   int           // 缩容因子
}

// DefaultPoolConfig 默认配置
func DefaultPoolConfig() *PoolConfig {
	return &PoolConfig{
		MaxConnections:      10000,
		InitialCap:          100,
		MaxIdle:             500,
		IdleTimeout:         300 * time.Second,
		MaxLifetime:         3600 * time.Second,
		EnableRateLimit:     true,
		RateLimitPerConn:    1000,
		BurstSize:           100,
		GlobalRateLimit:     1000000,
		QueueSize:           10000,
		QueueTimeout:        5 * time.Second,
		MaxMemoryMB:         4096,
		MemoryCheckInterval: 10 * time.Second,
		HealthCheckInterval: 30 * time.Second,
		HealthCheckTimeout:  5 * time.Second,
		EnableAdaptive:      true,
		ScaleUpThreshold:    0.8,
		ScaleDownThreshold:  0.3,
		ScaleUpFactor:       2,
		ScaleDownFactor:     2,
	}
}

// Conn 包装连接
type Conn struct {
	net.Conn
	pool        *Pool
	id          uint64
	createdAt   time.Time
	lastUsedAt  time.Time
	useCount    uint64
	inUse       int32
	closed      int32
	rateLimiter *RateLimiter
}

// Close 归还连接到池
func (c *Conn) Close() error {
	if atomic.CompareAndSwapInt32(&c.inUse, 1, 0) {
		c.pool.put(c)
	}
	return nil
}

// RealClose 真正关闭连接
func (c *Conn) RealClose() error {
	if atomic.CompareAndSwapInt32(&c.closed, 0, 1) {
		return c.Conn.Close()
	}
	return nil
}

// IsHealthy 检查连接健康
func (c *Conn) IsHealthy() bool {
	if atomic.LoadInt32(&c.closed) == 1 {
		return false
	}
	// 检查连接是否可用
	if c.Conn != nil {
		// 设置短超时检查
		c.Conn.SetReadDeadline(time.Now().Add(100 * time.Millisecond))
		buf := make([]byte, 1)
		n, err := c.Conn.Read(buf)
		c.Conn.SetReadDeadline(time.Time{})
		// 返回 0 且 err == nil 表示连接正常（无数据但连接有效）
		// 返回 n > 0 表示有数据可读
		// 返回 timeout err 表示连接正常
		if err != nil {
			if netErr, ok := err.(net.Error); ok && netErr.Timeout() {
				return true
			}
			return false
		}
		return n >= 0
	}
	return false
}

// RateLimiter 限流器
type RateLimiter struct {
	tokens   chan struct{}
	interval time.Duration
	burst    int
	stopCh   chan struct{}
}

// NewRateLimiter 创建限流器
func NewRateLimiter(rate int, burst int) *RateLimiter {
	if burst <= 0 {
		burst = rate
	}
	rl := &RateLimiter{
		tokens:   make(chan struct{}, burst),
		interval: time.Second / time.Duration(rate),
		burst:    burst,
		stopCh:   make(chan struct{}),
	}
	// 填充初始令牌
	for i := 0; i < burst; i++ {
		rl.tokens <- struct{}{}
	}
	go rl.refill()
	return rl
}

// refill 补充令牌
func (rl *RateLimiter) refill() {
	ticker := time.NewTicker(rl.interval)
	defer ticker.Stop()
	for {
		select {
		case <-ticker.C:
			select {
			case rl.tokens <- struct{}{}:
			default:
			}
		case <-rl.stopCh:
			return
		}
	}
}

// Allow 检查是否允许通过
func (rl *RateLimiter) Allow() bool {
	select {
	case <-rl.tokens:
		return true
	default:
		return false
	}
}

// AllowWait 等待直到允许通过或超时
func (rl *RateLimiter) AllowWait(timeout time.Duration) bool {
	select {
	case <-rl.tokens:
		return true
	case <-time.After(timeout):
		return false
	case <-rl.stopCh:
		return false
	}
}

// Stop 停止限流器
func (rl *RateLimiter) Stop() {
	close(rl.stopCh)
}

// Pool 连接池
type Pool struct {
	mu sync.RWMutex

	config *PoolConfig

	// 工厂函数
	factory func() (net.Conn, error)

	// 连接存储
	conns     []*Conn
	available chan *Conn
	waiting   chan chan *Conn

	// 状态统计
	stats PoolStats

	// 控制
	ctx    context.Context
	cancel context.CancelFunc

	// 全局限流器
	globalLimiter *RateLimiter

	// 内存监控
	memoryMonitor *MemoryMonitor

	// 自适应控制器
	adaptiveController *AdaptiveController
}

// PoolStats 池统计
type PoolStats struct {
	TotalCreated    uint64
	TotalClosed     uint64
	TotalAcquired   uint64
	TotalReleased   uint64
	TotalWaited     uint64
	TotalTimedOut   uint64
	TotalRateLimited uint64
	CurrentActive   int32
	CurrentIdle     int32
	CurrentWaiting  int32
	QueueLength     int32
}

// MemoryMonitor 内存监控
type MemoryMonitor struct {
	maxMemoryMB int64
	interval    time.Duration
	stopCh      chan struct{}
	callbacks   []func(usage float64)
}

// NewMemoryMonitor 创建内存监控器
func NewMemoryMonitor(maxMB int64, interval time.Duration) *MemoryMonitor {
	return &MemoryMonitor{
		maxMemoryMB: maxMB,
		interval:    interval,
		stopCh:      make(chan struct{}),
		callbacks:   make([]func(usage float64), 0),
	}
}

// Start 启动监控
func (mm *MemoryMonitor) Start() {
	go mm.monitor()
}

// Stop 停止监控
func (mm *MemoryMonitor) Stop() {
	close(mm.stopCh)
}

// OnHighMemory 注册高内存回调
func (mm *MemoryMonitor) OnHighMemory(cb func(usage float64)) {
	mm.callbacks = append(mm.callbacks, cb)
}

// monitor 监控循环
func (mm *MemoryMonitor) monitor() {
	ticker := time.NewTicker(mm.interval)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			usage := mm.getMemoryUsage()
			if usage > 0.8 { // 80% 阈值
				for _, cb := range mm.callbacks {
					go cb(usage)
				}
			}
		case <-mm.stopCh:
			return
		}
	}
}

// getMemoryUsage 获取内存使用率（简化实现）
func (mm *MemoryMonitor) getMemoryUsage() float64 {
	// 实际实现应读取 /proc/meminfo 或使用 runtime.ReadMemStats
	// 这里返回模拟值
	return 0.5
}

// AdaptiveController 自适应控制器
type AdaptiveController struct {
	pool       *Pool
	config     *PoolConfig
	stopCh     chan struct{}
	lastScaleUp   time.Time
	lastScaleDown time.Time
}

// NewAdaptiveController 创建自适应控制器
func NewAdaptiveController(pool *Pool, config *PoolConfig) *AdaptiveController {
	return &AdaptiveController{
		pool:   pool,
		config: config,
		stopCh: make(chan struct{}),
	}
}

// Start 启动控制
func (ac *AdaptiveController) Start() {
	go ac.control()
}

// Stop 停止控制
func (ac *AdaptiveController) Stop() {
	close(ac.stopCh)
}

// control 控制循环
func (ac *AdaptiveController) control() {
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			ac.adjust()
		case <-ac.stopCh:
			return
		}
	}
}

// adjust 调整连接数
func (ac *AdaptiveController) adjust() {
	stats := ac.pool.GetStats()
	if stats.TotalCreated == 0 {
		return
	}

	// 计算使用率
	utilization := float64(stats.CurrentActive) / float64(ac.pool.config.MaxConnections)

	// 扩容检查
	if utilization > ac.config.ScaleUpThreshold {
		if time.Since(ac.lastScaleUp) > 60*time.Second {
			ac.pool.scaleUp()
			ac.lastScaleUp = time.Now()
		}
	}

	// 缩容检查
	if utilization < ac.config.ScaleDownThreshold && stats.CurrentIdle > int32(ac.config.MaxIdle) {
		if time.Since(ac.lastScaleDown) > 120*time.Second {
			ac.pool.scaleDown()
			ac.lastScaleDown = time.Now()
		}
	}
}

// NewPool 创建连接池
func NewPool(config *PoolConfig, factory func() (net.Conn, error)) (*Pool, error) {
	if config == nil {
		config = DefaultPoolConfig()
	}

	if factory == nil {
		return nil, errors.New("factory function is required")
	}

	ctx, cancel := context.WithCancel(context.Background())

	pool := &Pool{
		config:    config,
		factory:   factory,
		conns:     make([]*Conn, 0, config.MaxConnections),
		available: make(chan *Conn, config.MaxConnections),
		waiting:   make(chan chan *Conn, config.QueueSize),
		ctx:       ctx,
		cancel:    cancel,
	}

	// 创建全局限流器
	if config.EnableRateLimit {
		pool.globalLimiter = NewRateLimiter(config.GlobalRateLimit, config.BurstSize)
	}

	// 创建内存监控器
	pool.memoryMonitor = NewMemoryMonitor(config.MaxMemoryMB, config.MemoryCheckInterval)
	pool.memoryMonitor.OnHighMemory(func(usage float64) {
		pool.handleHighMemory(usage)
	})
	pool.memoryMonitor.Start()

	// 创建自适应控制器
	if config.EnableAdaptive {
		pool.adaptiveController = NewAdaptiveController(pool, config)
		pool.adaptiveController.Start()
	}

	// 初始化连接
	for i := 0; i < config.InitialCap; i++ {
		conn, err := pool.createConn()
		if err != nil {
			pool.Close()
			return nil, fmt.Errorf("failed to create initial connection: %w", err)
		}
		pool.conns = append(pool.conns, conn)
		pool.available <- conn
	}

	// 启动维护 goroutine
	go pool.maintain()

	return pool, nil
}

// createConn 创建新连接
func (p *Pool) createConn() (*Conn, error) {
	netConn, err := p.factory()
	if err != nil {
		return nil, err
	}

	conn := &Conn{
		Conn:       netConn,
		pool:       p,
		id:         uint64(len(p.conns) + 1),
		createdAt:  time.Now(),
		lastUsedAt: time.Now(),
	}

	// 创建连接级限流器
	if p.config.EnableRateLimit {
		conn.rateLimiter = NewRateLimiter(p.config.RateLimitPerConn, p.config.BurstSize)
	}

	atomic.AddUint64(&p.stats.TotalCreated, 1)
	return conn, nil
}

// Get 获取连接
func (p *Pool) Get() (*Conn, error) {
	return p.GetWithTimeout(p.config.QueueTimeout)
}

// GetWithTimeout 带超时的获取连接
func (p *Pool) GetWithTimeout(timeout time.Duration) (*Conn, error) {
	if p.ctx.Err() != nil {
		return nil, ErrPoolClosed
	}

	// 全局限流检查
	if p.globalLimiter != nil && !p.globalLimiter.Allow() {
		atomic.AddUint64(&p.stats.TotalRateLimited, 1)
		return nil, ErrRateLimited
	}

	// 尝试直接获取
	select {
	case conn := <-p.available:
		if conn.IsHealthy() {
			atomic.AddInt32(&conn.inUse, 1)
			conn.lastUsedAt = time.Now()
			atomic.AddUint64(&p.stats.TotalAcquired, 1)
			atomic.AddInt32(&p.stats.CurrentActive, 1)
			atomic.AddInt32(&p.stats.CurrentIdle, -1)
			return conn, nil
		}
		// 连接不健康，关闭并创建新连接
		conn.RealClose()
		atomic.AddUint64(&p.stats.TotalClosed, 1)
	default:
	}

	// 尝试创建新连接（未达上限）
	p.mu.Lock()
	if len(p.conns) < p.config.MaxConnections {
		conn, err := p.createConn()
		if err == nil {
			p.conns = append(p.conns, conn)
			atomic.AddInt32(&conn.inUse, 1)
			conn.lastUsedAt = time.Now()
			atomic.AddUint64(&p.stats.TotalAcquired, 1)
			atomic.AddInt32(&p.stats.CurrentActive, 1)
			p.mu.Unlock()
			return conn, nil
		}
	}
	p.mu.Unlock()

	// 进入等待队列
	atomic.AddInt32(&p.stats.CurrentWaiting, 1)
	atomic.AddUint64(&p.stats.TotalWaited, 1)
	defer atomic.AddInt32(&p.stats.CurrentWaiting, -1)

	waitCh := make(chan *Conn, 1)
	select {
	case p.waiting <- waitCh:
		select {
		case conn := <-waitCh:
			if conn != nil {
				atomic.AddInt32(&conn.inUse, 1)
				conn.lastUsedAt = time.Now()
				atomic.AddUint64(&p.stats.TotalAcquired, 1)
				atomic.AddInt32(&p.stats.CurrentActive, 1)
				return conn, nil
			}
		case <-time.After(timeout):
			atomic.AddUint64(&p.stats.TotalTimedOut, 1)
			return nil, ErrQueueFull
		case <-p.ctx.Done():
			return nil, ErrPoolClosed
		}
	case <-time.After(timeout):
		atomic.AddUint64(&p.stats.TotalTimedOut, 1)
		return nil, ErrQueueFull
	case <-p.ctx.Done():
		return nil, ErrPoolClosed
	}

	return nil, ErrQueueFull
}

// put 归还连接
func (p *Pool) put(conn *Conn) {
	if conn == nil || atomic.LoadInt32(&conn.closed) == 1 {
		return
	}

	atomic.AddUint64(&p.stats.TotalReleased, 1)
	atomic.AddInt32(&p.stats.CurrentActive, -1)
	atomic.AddInt32(&p.stats.CurrentIdle, 1)

	// 检查是否有等待者
	select {
	case waitCh := <-p.waiting:
		waitCh <- conn
		return
	default:
	}

	// 归还到可用队列
	select {
	case p.available <- conn:
	default:
		// 队列满，关闭连接
		conn.RealClose()
		atomic.AddUint64(&p.stats.TotalClosed, 1)
		atomic.AddInt32(&p.stats.CurrentIdle, -1)
	}
}

// maintain 维护循环
func (p *Pool) maintain() {
	ticker := time.NewTicker(p.config.HealthCheckInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			p.cleanup()
		case <-p.ctx.Done():
			return
		}
	}
}

// cleanup 清理过期连接
func (p *Pool) cleanup() {
	p.mu.Lock()
	defer p.mu.Unlock()

	now := time.Now()
	active := make([]*Conn, 0, len(p.conns))

	for _, conn := range p.conns {
		shouldClose := false

		// 检查是否已关闭
		if atomic.LoadInt32(&conn.closed) == 1 {
			shouldClose = true
		} else if atomic.LoadInt32(&conn.inUse) == 0 {
			// 空闲连接检查
			if now.Sub(conn.lastUsedAt) > p.config.IdleTimeout {
				shouldClose = true
			} else if now.Sub(conn.createdAt) > p.config.MaxLifetime {
				shouldClose = true
			} else if !conn.IsHealthy() {
				shouldClose = true
			}
		}

		if shouldClose && atomic.LoadInt32(&conn.inUse) == 0 {
			conn.RealClose()
			atomic.AddUint64(&p.stats.TotalClosed, 1)
			atomic.AddInt32(&p.stats.CurrentIdle, -1)
		} else {
			active = append(active, conn)
		}
	}

	p.conns = active
}

// scaleUp 扩容
func (p *Pool) scaleUp() {
	p.mu.Lock()
	defer p.mu.Unlock()

	target := len(p.conns) * p.config.ScaleUpFactor
	if target > p.config.MaxConnections {
		target = p.config.MaxConnections
	}

	for len(p.conns) < target {
		conn, err := p.createConn()
		if err != nil {
			break
		}
		p.conns = append(p.conns, conn)
		p.available <- conn
		atomic.AddInt32(&p.stats.CurrentIdle, 1)
	}
}

// scaleDown 缩容
func (p *Pool) scaleDown() {
	p.mu.Lock()
	defer p.mu.Unlock()

	target := len(p.conns) / p.config.ScaleDownFactor
	if target < p.config.InitialCap {
		target = p.config.InitialCap
	}

	// 从空闲队列中移除多余连接
	for len(p.conns) > target {
		select {
		case conn := <-p.available:
			conn.RealClose()
			atomic.AddUint64(&p.stats.TotalClosed, 1)
			atomic.AddInt32(&p.stats.CurrentIdle, -1)

			// 从列表中移除
			for i, c := range p.conns {
				if c == conn {
					p.conns = append(p.conns[:i], p.conns[i+1:]...)
					break
				}
			}
		default:
			return
		}
	}
}

// handleHighMemory 处理高内存
func (p *Pool) handleHighMemory(usage float64) {
	// 紧急缩容
	p.scaleDown()

	// 降低限流阈值
	if p.globalLimiter != nil {
		// 实际实现应动态调整限流器
	}
}

// GetStats 获取统计
func (p *Pool) GetStats() PoolStats {
	return PoolStats{
		TotalCreated:     atomic.LoadUint64(&p.stats.TotalCreated),
		TotalClosed:      atomic.LoadUint64(&p.stats.TotalClosed),
		TotalAcquired:    atomic.LoadUint64(&p.stats.TotalAcquired),
		TotalReleased:    atomic.LoadUint64(&p.stats.TotalReleased),
		TotalWaited:      atomic.LoadUint64(&p.stats.TotalWaited),
		TotalTimedOut:    atomic.LoadUint64(&p.stats.TotalTimedOut),
		TotalRateLimited: atomic.LoadUint64(&p.stats.TotalRateLimited),
		CurrentActive:    atomic.LoadInt32(&p.stats.CurrentActive),
		CurrentIdle:      atomic.LoadInt32(&p.stats.CurrentIdle),
		CurrentWaiting:   atomic.LoadInt32(&p.stats.CurrentWaiting),
		QueueLength:      int32(len(p.waiting)),
	}
}

// Close 关闭池
func (p *Pool) Close() error {
	p.cancel()

	if p.memoryMonitor != nil {
		p.memoryMonitor.Stop()
	}

	if p.adaptiveController != nil {
		p.adaptiveController.Stop()
	}

	if p.globalLimiter != nil {
		p.globalLimiter.Stop()
	}

	p.mu.Lock()
	defer p.mu.Unlock()

	// 关闭所有连接
	for _, conn := range p.conns {
		if conn.rateLimiter != nil {
			conn.rateLimiter.Stop()
		}
		conn.RealClose()
	}

	close(p.available)
	close(p.waiting)

	return nil
}
