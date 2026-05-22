// Package circuitbreaker 过载熔断保护
// 资源超限自动熔断、静默保护、自动恢复
package circuitbreaker

import (
	"fmt"
	"sync"
	"time"
)

// ============================================================
// 熔断器状态
// ============================================================

// CircuitState 熔断状态
type CircuitState int

const (
	StateClosed   CircuitState = iota // 关闭：正常请求
	StateOpen                       // 打开：拒绝请求
	StateHalfOpen                    // 半开：探测恢复
)

func (s CircuitState) String() string {
	switch s {
	case StateClosed:
		return "CLOSED"
	case StateOpen:
		return "OPEN"
	case StateHalfOpen:
		return "HALF_OPEN"
	default:
		return "UNKNOWN"
	}
}

// ============================================================
// 熔断配置
// ============================================================

// Config 熔断配置
type Config struct {
	// 触发条件
	CPUWarningThreshold    float64 `yaml:"cpu_warning_threshold"`    // CPU警告阈值 (%)
	CPUCriticalThreshold   float64 `yaml:"cpu_critical_threshold"`   // CPU熔断阈值 (%)
	MemoryWarningThreshold float64 `yaml:"memory_warning_threshold"` // 内存警告阈值 (%)
	MemoryCriticalThreshold float64 `yaml:"memory_critical_threshold"` // 内存熔断阈值 (%)
	
	// 触发方式
	FailureThreshold     int     `yaml:"failure_threshold"`      // 连续失败次数触发熔断
	SuccessThreshold    int     `yaml:"success_threshold"`      // 半开状态下连续成功次数恢复
	TimeoutThreshold     int     `yaml:"timeout_threshold"`      // 超时次数触发熔断
	LatencyP99Threshold  float64 `yaml:"latency_p99_threshold"`   // P99延迟阈值 (ms)
	
	// 时间窗口
	WindowDuration       int     `yaml:"window_duration"`        // 统计窗口（秒）
	CircuitOpenDuration  int     `yaml:"circuit_open_duration"`   // 熔断持续时间（秒）
	ProbeInterval        int     `yaml:"probe_interval"`          // 半开探测间隔（秒）
	
	// 保护策略
	DropPercentWhenOpen  int     `yaml:"drop_percent_when_open"` // 熔断时丢弃百分比 (0-100)
	SilentMode           bool    `yaml:"silent_mode"`            // 静默模式（不返回错误）
	QueueLimit           int     `yaml:"queue_limit"`            // 请求队列限制
}

// DefaultConfig 默认配置
func DefaultConfig() *Config {
	return &Config{
		CPUWarningThreshold:    60.0,
		CPUCriticalThreshold:   80.0,
		MemoryWarningThreshold: 70.0,
		MemoryCriticalThreshold: 90.0,
		FailureThreshold:      5,
		SuccessThreshold:       3,
		TimeoutThreshold:       10,
		LatencyP99Threshold:    1000.0,
		WindowDuration:         60,
		CircuitOpenDuration:    30,
		ProbeInterval:           5,
		DropPercentWhenOpen:    100,
		SilentMode:             true,
		QueueLimit:             1000,
	}
}

// ============================================================
// 熔断器
// ============================================================

// CircuitBreaker 熔断器
type CircuitBreaker struct {
	config   *Config
	name     string
	state    CircuitState
	mu       sync.RWMutex
	
	// 统计
	failures   int
	successes  int
	timeouts   int
	latencies  []float64
	totalReqs  int64
	totalDrops int64
	
	// 时间窗口
	windowStart   time.Time
	circuitOpenAt time.Time
	halfOpenAt    time.Time
	
	// 回调
	onStateChange func(name string, from, to CircuitState)
	onDrop       func(reason string)
	onRecover    func()
}

// NewCircuitBreaker 创建熔断器
func NewCircuitBreaker(name string, config *Config) *CircuitBreaker {
	if config == nil {
		config = DefaultConfig()
	}
	
	cb := &CircuitBreaker{
		config:      config,
		name:        name,
		state:       StateClosed,
		windowStart: time.Now(),
		latencies:   make([]float64, 0, 1000),
	}
	
	return cb
}

// Allow 检查是否允许请求
func (cb *CircuitBreaker) Allow() bool {
	cb.mu.Lock()
	defer cb.mu.Unlock()
	
	switch cb.state {
	case StateClosed:
		return true
		
	case StateOpen:
		// 检查是否可以进入半开状态
		if time.Since(cb.circuitOpenAt) >= time.Duration(cb.config.CircuitOpenDuration)*time.Second {
			cb.toHalfOpen()
			return true
		}
		return false
		
	case StateHalfOpen:
		return true
	}
	
	return true
}

// RecordSuccess 记录成功
func (cb *CircuitBreaker) RecordSuccess(latencyMs float64) {
	cb.mu.Lock()
	defer cb.mu.Unlock()
	
	cb.successes++
	cb.totalReqs++
	cb.addLatency(latencyMs)
	
	// 半开状态下连续成功恢复
	if cb.state == StateHalfOpen {
		if cb.successes >= cb.config.SuccessThreshold {
			cb.toClosed()
			if cb.onRecover != nil {
				go cb.onRecover()
			}
		}
	}
	
	// 重置失败计数
	cb.failures = 0
	cb.timeouts = 0
}

// RecordFailure 记录失败
func (cb *CircuitBreaker) RecordFailure(reason string) {
	cb.mu.Lock()
	defer cb.mu.Unlock()
	
	cb.failures++
	cb.totalReqs++
	
	// 关闭状态下连续失败触发熔断
	if cb.state == StateClosed {
		if cb.failures >= cb.config.FailureThreshold {
			cb.toOpen()
		}
	}
	
	// 半开状态下失败重新打开
	if cb.state == StateHalfOpen {
		cb.toOpen()
	}
}

// RecordTimeout 记录超时
func (cb *CircuitBreaker) RecordTimeout() {
	cb.mu.Lock()
	defer cb.mu.Unlock()
	
	cb.timeouts++
	cb.totalReqs++
	
	if cb.state == StateClosed {
		if cb.timeouts >= cb.config.TimeoutThreshold {
			cb.toOpen()
		}
	}
}

// RecordLatency 记录延迟
func (cb *CircuitBreaker) RecordLatency(latencyMs float64) {
	cb.mu.Lock()
	defer cb.mu.Unlock()
	
	cb.addLatency(latencyMs)
	
	// 检查P99延迟
	if cb.state == StateClosed && len(cb.latencies) > 10 {
		p99 := cb.getP99()
		if p99 > cb.config.LatencyP99Threshold {
			cb.toOpen()
		}
	}
}

// RecordDrop 记录丢弃
func (cb *CircuitBreaker) RecordDrop() {
	cb.mu.Lock()
	defer cb.mu.Unlock()
	cb.totalDrops++
}

// CheckResource 检查资源使用
func (cb *CircuitBreaker) CheckResource(cpu, memory float64) {
	cb.mu.Lock()
	defer cb.mu.Unlock()
	
	// CPU检查
	if cpu >= cb.config.CPUCriticalThreshold {
		cb.toOpen()
		return
	}
	
	// 内存检查
	if memory >= cb.config.MemoryCriticalThreshold {
		cb.toOpen()
		return
	}
	
	// 警告级别但未达熔断
	if cpu >= cb.config.CPUWarningThreshold || memory >= cb.config.MemoryWarningThreshold {
		// 可以在这里触发告警
	}
}

// ShouldDrop 判断是否应该丢弃请求
func (cb *CircuitBreaker) ShouldDrop() bool {
	cb.mu.Lock()
	defer cb.mu.Unlock()
	
	if cb.state != StateOpen {
		return false
	}
	
	// 全量丢弃
	if cb.config.DropPercentWhenOpen >= 100 {
		cb.totalDrops++
		return true
	}
	
	// 百分比丢弃
	if cb.config.DropPercentWhenOpen > 0 {
		cb.totalDrops++
		return true
	}
	
	return false
}

// GetState 获取状态
func (cb *CircuitBreaker) GetState() CircuitState {
	cb.mu.RLock()
	defer cb.mu.RUnlock()
	return cb.state
}

// GetStats 获取统计
func (cb *CircuitBreaker) GetStats() *Stats {
	cb.mu.RLock()
	defer cb.mu.RUnlock()
	
	return &Stats{
		Name:             cb.name,
		State:            cb.state.String(),
		TotalRequests:    cb.totalReqs,
		TotalDrops:       cb.totalDrops,
		DropRate:         calcDropRate(cb.totalReqs, cb.totalDrops),
		CurrentFailures:  cb.failures,
		CurrentTimeouts:  cb.timeouts,
		AvgLatency:       cb.getAvg(),
		P99Latency:       cb.getP99(),
		CircuitOpenAt:    cb.circuitOpenAt,
		SecondsUntilRetry: cb.getSecondsUntilRetry(),
	}
}

// ============================================================
// 内部方法
// ============================================================

func (cb *CircuitBreaker) toOpen() {
	if cb.state == StateOpen {
		return
	}
	
	oldState := cb.state
	cb.state = StateOpen
	cb.circuitOpenAt = time.Now()
	cb.failures = 0
	cb.successes = 0
	
	if cb.onStateChange != nil {
		go cb.onStateChange(cb.name, oldState, StateOpen)
	}
}

func (cb *CircuitBreaker) toHalfOpen() {
	oldState := cb.state
	cb.state = StateHalfOpen
	cb.halfOpenAt = time.Now()
	cb.successes = 0
	
	if cb.onStateChange != nil {
		go cb.onStateChange(cb.name, oldState, StateHalfOpen)
	}
}

func (cb *CircuitBreaker) toClosed() {
	oldState := cb.state
	cb.state = StateClosed
	cb.failures = 0
	cb.successes = 0
	cb.timeouts = 0
	cb.windowStart = time.Now()
	cb.latencies = cb.latencies[:0]
	
	if cb.onStateChange != nil {
		go cb.onStateChange(cb.name, oldState, StateClosed)
	}
}

func (cb *CircuitBreaker) addLatency(latency float64) {
	// 保持滑动窗口
	if len(cb.latencies) >= 1000 {
		cb.latencies = cb.latencies[1:]
	}
	cb.latencies = append(cb.latencies, latency)
}

func (cb *CircuitBreaker) getAvg() float64 {
	if len(cb.latencies) == 0 {
		return 0
	}
	sum := 0.0
	for _, l := range cb.latencies {
		sum += l
	}
	return sum / float64(len(cb.latencies))
}

func (cb *CircuitBreaker) getP99() float64 {
	if len(cb.latencies) == 0 {
		return 0
	}
	
	// 简单实现：取最后99%的最大值
	idx := int(float64(len(cb.latencies)) * 0.99)
	if idx >= len(cb.latencies) {
		idx = len(cb.latencies) - 1
	}
	if idx < 0 {
		idx = 0
	}
	
	max := cb.latencies[idx]
	for i := idx + 1; i < len(cb.latencies); i++ {
		if cb.latencies[i] > max {
			max = cb.latencies[i]
		}
	}
	return max
}

func (cb *CircuitBreaker) getSecondsUntilRetry() int {
	if cb.state != StateOpen {
		return 0
	}
	
	elapsed := time.Since(cb.circuitOpenAt).Seconds()
	remaining := float64(cb.config.CircuitOpenDuration) - elapsed
	if remaining < 0 {
		return 0
	}
	return int(remaining)
}

func calcDropRate(total, drops int64) float64 {
	if total == 0 {
		return 0
	}
	return float64(drops) / float64(total) * 100
}

// OnStateChange 设置状态变化回调
func (cb *CircuitBreaker) OnStateChange(cb func(name string, from, to CircuitState)) {
	cb.mu.Lock()
	cb.onStateChange = cb.onStateChange
	cb.mu.Unlock()
}

// OnRecover 设置恢复回调
func (cb *CircuitBreaker) OnRecover(cb func()) {
	cb.mu.Lock()
	cb.onRecover = cb
	cb.mu.Unlock()
}

// OnDrop 设置丢弃回调
func (cb *CircuitBreaker) OnDrop(cb func(reason string)) {
	cb.mu.Lock()
	cb.onDrop = cb
	cb.mu.Unlock()
}

// ============================================================
// 统计信息
// ============================================================

// Stats 熔断器统计
type Stats struct {
	Name             string  `json:"name"`
	State            string  `json:"state"`
	TotalRequests    int64   `json:"total_requests"`
	TotalDrops       int64   `json:"total_drops"`
	DropRate         float64 `json:"drop_rate"`
	CurrentFailures  int     `json:"current_failures"`
	CurrentTimeouts  int     `json:"current_timeouts"`
	AvgLatency       float64 `json:"avg_latency_ms"`
	P99Latency       float64 `json:"p99_latency_ms"`
	CircuitOpenAt    time.Time `json:"circuit_open_at"`
	SecondsUntilRetry int     `json:"seconds_until_retry"`
}

// ============================================================
// 熔断器管理器
// ============================================================

// Manager 熔断器管理器
type Manager struct {
	breakers map[string]*CircuitBreaker
	config   *Config
	mu       sync.RWMutex
}

// NewManager 创建熔断器管理器
func NewManager(config *Config) *Manager {
	if config == nil {
		config = DefaultConfig()
	}
	
	return &Manager{
		breakers: make(map[string]*CircuitBreaker),
		config:   config,
	}
}

// GetOrCreate 获取或创建熔断器
func (m *Manager) GetOrCreate(name string) *CircuitBreaker {
	m.mu.RLock()
	cb, exists := m.breakers[name]
	m.mu.RUnlock()
	
	if exists {
		return cb
	}
	
	m.mu.Lock()
	defer m.mu.Unlock()
	
	// 双重检查
	if cb, exists = m.breakers[name]; exists {
		return cb
	}
	
	cb = NewCircuitBreaker(name, m.config)
	m.breakers[name] = cb
	return cb
}

// Remove 移除熔断器
func (m *Manager) Remove(name string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	delete(m.breakers, name)
}

// List 获取所有熔断器统计
func (m *Manager) List() []*Stats {
	m.mu.RLock()
	defer m.mu.RUnlock()
	
	stats := make([]*Stats, 0, len(m.breakers))
	for _, cb := range m.breakers {
		stats = append(stats, cb.GetStats())
	}
	return stats
}

// Reset 重置所有熔断器
func (m *Manager) Reset() {
	m.mu.Lock()
	defer m.mu.Unlock()
	
	for _, cb := range m.breakers {
		cb.mu.Lock()
		cb.toClosed()
		cb.mu.Unlock()
	}
}
