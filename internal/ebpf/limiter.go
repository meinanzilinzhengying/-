// Package ebpf eBPF资源限制器
// CPU/内存上限控制，自适应采样，流量感知
package ebpf

import (
	"context"
	"fmt"
	"runtime"
	"sync"
	"time"
)

// ============================================================
// 资源限制配置
// ============================================================

// ResourceLimitConfig 资源限制配置
type ResourceLimitConfig struct {
	Enabled           bool    `yaml:"enabled" json:"enabled"`
	CPUMaxPercent     float64 `yaml:"cpu_max_percent" json:"cpu_max_percent"`         // CPU最大使用率 (%)
	MemoryMaxMB       int     `yaml:"memory_max_mb" json:"memory_max_mb"`             // 内存最大使用 (MB)
	SampleRateBase    int     `yaml:"sample_rate_base" json:"sample_rate_base"`       // 基础采样率 (Hz)
	SampleRateMin     int     `yaml:"sample_rate_min" json:"sample_rate_min"`         // 最小采样率 (Hz)
	SampleRateMax     int     `yaml:"sample_rate_max" json:"sample_rate_max"`         // 最大采样率 (Hz)
	AdaptiveEnabled   bool    `yaml:"adaptive_enabled" json:"adaptive_enabled"`       // 启用自适应采样
	TrafficThreshold  int64   `yaml:"traffic_threshold" json:"traffic_threshold"`     // 流量阈值 (pps)
	BackOffFactor     float64 `yaml:"back_off_factor" json:"back_off_factor"`         // 退避因子
	RecoveryFactor    float64 `yaml:"recovery_factor" json:"recovery_factor"`         // 恢复因子
	CheckIntervalSec  int     `yaml:"check_interval_sec" json:"check_interval_sec"`   // 检查间隔 (秒)
}

// DefaultResourceLimitConfig 默认配置
func DefaultResourceLimitConfig() *ResourceLimitConfig {
	return &ResourceLimitConfig{
		Enabled:          true,
		CPUMaxPercent:    20.0,   // 最多使用20% CPU
		MemoryMaxMB:      256,    // 最多使用256MB内存
		SampleRateBase:   1000,   // 基础1kHz采样
		SampleRateMin:    100,    // 最低100Hz
		SampleRateMax:    10000,  // 最高10kHz
		AdaptiveEnabled:  true,
		TrafficThreshold: 100000, // 10万pps阈值
		BackOffFactor:    0.8,    // 超限时采样率乘以0.8
		RecoveryFactor:   1.1,    // 正常时采样率乘以1.1
		CheckIntervalSec: 5,      // 每5秒检查一次
	}
}

// ============================================================
// 资源限制器
// ============================================================

// ResourceLimiter 资源限制器
type ResourceLimiter struct {
	config      *ResourceLimitConfig
	currentRate int
	mu          sync.RWMutex
	stopCh      chan struct{}
	wg          sync.WaitGroup
	
	// 统计信息
	stats       *LimiterStats
	statsMu     sync.RWMutex
}

// LimiterStats 限制器统计
type LimiterStats struct {
	CurrentSampleRate int       `json:"current_sample_rate"`
	CurrentCPUUsage   float64   `json:"current_cpu_usage"`
	CurrentMemoryMB   float64   `json:"current_memory_mb"`
	AdjustmentCount   int64     `json:"adjustment_count"`
	LastAdjustment    time.Time `json:"last_adjustment"`
	TotalPackets      int64     `json:"total_packets"`
	DroppedPackets    int64     `json:"dropped_packets"`
}

// NewResourceLimiter 创建资源限制器
func NewResourceLimiter(config *ResourceLimitConfig) *ResourceLimiter {
	if config == nil {
		config = DefaultResourceLimitConfig()
	}
	return &ResourceLimiter{
		config:      config,
		currentRate: config.SampleRateBase,
		stopCh:      make(chan struct{}),
		stats: &LimiterStats{
			CurrentSampleRate: config.SampleRateBase,
		},
	}
}

// Start 启动资源监控
func (r *ResourceLimiter) Start() {
	if !r.config.Enabled {
		return
	}
	
	r.wg.Add(1)
	go r.monitorLoop()
}

// Stop 停止资源监控
func (r *ResourceLimiter) Stop() {
	close(r.stopCh)
	r.wg.Wait()
}

// monitorLoop 监控循环
func (r *ResourceLimiter) monitorLoop() {
	defer r.wg.Done()
	
	ticker := time.NewTicker(time.Duration(r.config.CheckIntervalSec) * time.Second)
	defer ticker.Stop()
	
	for {
		select {
		case <-ticker.C:
			r.adjust()
		case <-r.stopCh:
			return
		}
	}
}

// adjust 调整采样率
func (r *ResourceLimiter) adjust() {
	// 获取当前资源使用
	cpuUsage := r.getCPUUsage()
	memUsage := r.getMemoryUsage()
	
	r.statsMu.Lock()
	r.stats.CurrentCPUUsage = cpuUsage
	r.stats.CurrentMemoryMB = memUsage
	r
	// 检查是否需要调整
	needAdjust := false
	newRate := r.currentRate
	
	// CPU超限 -> 降低采样率
	if cpuUsage > r.config.CPUMaxPercent {
		newRate = int(float64(r.currentRate) * r.config.BackOffFactor)
		needAdjust = true
	}
	
	// 内存超限 -> 降低采样率
	if memUsage > float64(r.config.MemoryMaxMB) {
		newRate = int(float64(r.currentRate) * r.config.BackOffFactor)
		needAdjust = true
	}
	
	// 资源正常 -> 尝试提高采样率
	if !needAdjust && r.config.AdaptiveEnabled {
		if cpuUsage < r.config.CPUMaxPercent*0.7 && memUsage < float64(r.config.MemoryMaxMB)*0.7 {
			newRate = int(float64(r.currentRate) * r.config.RecoveryFactor)
			needAdjust = true
		}
	}
	
	// 限制在有效范围内
	if newRate < r.config.SampleRateMin {
		newRate = r.config.SampleRateMin
	}
	if newRate > r.config.SampleRateMax {
		newRate = r.config.SampleRateMax
	}
	
	if needAdjust && newRate != r.currentRate {
		r.currentRate = newRate
		r.stats.CurrentSampleRate = newRate
		r.stats.AdjustmentCount++
		r.stats.LastAdjustment = time.Now()
	}
	r.statsMu.Unlock()
}

// GetSampleRate 获取当前采样率
func (r *ResourceLimiter) GetSampleRate() int {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.currentRate
}

// SetSampleRate 设置采样率
func (r *ResourceLimiter) SetSampleRate(rate int) {
	r.mu.Lock()
	defer r.mu.Unlock()
	
	if rate < r.config.SampleRateMin {
		rate = r.config.SampleRateMin
	}
	if rate > r.config.SampleRateMax {
		rate = r.config.SampleRateMax
	}
	
	r.currentRate = rate
}

// GetStats 获取统计信息
func (r *ResourceLimiter) GetStats() *LimiterStats {
	r.statsMu.RLock()
	defer r.statsMu.RUnlock()
	
	return &LimiterStats{
		CurrentSampleRate: r.stats.CurrentSampleRate,
		CurrentCPUUsage:   r.stats.CurrentCPUUsage,
		CurrentMemoryMB:   r.stats.CurrentMemoryMB,
		AdjustmentCount:   r.stats.AdjustmentCount,
		LastAdjustment:    r.stats.LastAdjustment,
		TotalPackets:      r.stats.TotalPackets,
		DroppedPackets:    r.stats.DroppedPackets,
	}
}

// getCPUUsage 获取CPU使用率
func (r *ResourceLimiter) getCPUUsage() float64 {
	// 简化实现，实际应读取/proc/stat
	// 这里返回一个模拟值
	return 10.0
}

// getMemoryUsage 获取内存使用 (MB)
func (r *ResourceLimiter) getMemoryUsage() float64 {
	var m runtime.MemStats
	runtime.ReadMemStats(&m)
	return float64(m.Alloc) / 1024 / 1024
}

// RecordPacket 记录数据包（用于流量感知）
func (r *ResourceLimiter) RecordPacket() {
	r.statsMu.Lock()
	r.stats.TotalPackets++
	r.statsMu.Unlock()
}

// RecordDrop 记录丢包
func (r *ResourceLimiter) RecordDrop() {
	r.statsMu.Lock()
	r.stats.DroppedPackets++
	r.statsMu.Unlock()
}

// ============================================================
// 流量感知采样器
// ============================================================

// TrafficAwareSampler 流量感知采样器
type TrafficAwareSampler struct {
	config         *ResourceLimitConfig
	limiter        *ResourceLimiter
	trafficCounter *TrafficCounter
	mu             sync.RWMutex
}

// TrafficCounter 流量计数器
type TrafficCounter struct {
	packets   int64
	bytes     int64
	lastReset time.Time
	mu        sync.RWMutex
}

// NewTrafficAwareSampler 创建流量感知采样器
func NewTrafficAwareSampler(config *ResourceLimitConfig) *TrafficAwareSampler {
	return &TrafficAwareSampler{
		config:         config,
		limiter:        NewResourceLimiter(config),
		trafficCounter: &TrafficCounter{lastReset: time.Now()},
	}
}

// Start 启动采样器
func (t *TrafficAwareSampler) Start() {
	t.limiter.Start()
}

// Stop 停止采样器
func (t *TrafficAwareSampler) Stop() {
	t.limiter.Stop()
}

// ShouldSample 判断是否采样
func (t *TrafficAwareSampler) ShouldSample() bool {
	if !t.config.Enabled {
		return true
	}
	
	// 基于当前采样率决定是否采样
	rate := t.limiter.GetSampleRate()
	
	// 简化实现：使用随机数
	// 实际应使用更精确的采样算法
	return true
}

// Record 记录流量
func (t *TrafficAwareSampler) Record(bytes int64) {
	t.trafficCounter.mu.Lock()
	t.trafficCounter.packets++
	t.trafficCounter.bytes += bytes
	t.trafficCounter.mu.Unlock()
	
	t.limiter.RecordPacket()
}

// GetTrafficRate 获取当前流量速率 (pps)
func (t *TrafficAwareSampler) GetTrafficRate() int64 {
	t.trafficCounter.mu.RLock()
	packets := t.trafficCounter.packets
	lastReset := t.trafficCounter.lastReset
	t.trafficCounter.mu.RUnlock()
	
	elapsed := time.Since(lastReset).Seconds()
	if elapsed > 0 {
		return int64(float64(packets) / elapsed)
	}
	return 0
}

// ResetCounter 重置计数器
func (t *TrafficAwareSampler) ResetCounter() {
	t.trafficCounter.mu.Lock()
	t.trafficCounter.packets = 0
	t.trafficCounter.bytes = 0
	t.trafficCounter.lastReset = time.Now()
	t.trafficCounter.mu.Unlock()
}

// ============================================================
// 自适应采样控制器
// ============================================================

// AdaptiveController 自适应采样控制器
type AdaptiveController struct {
	config      *ResourceLimitConfig
	limiter     *ResourceLimiter
	targetCPU   float64
	targetMem   float64
}

// NewAdaptiveController 创建自适应控制器
func NewAdaptiveController(config *ResourceLimitConfig) *AdaptiveController {
	if config == nil {
		config = DefaultResourceLimitConfig()
	}
	return &AdaptiveController{
		config:    config,
		limiter:   NewResourceLimiter(config),
		targetCPU: config.CPUMaxPercent * 0.8,  // 目标80%的CPU限制
		targetMem: float64(config.MemoryMaxMB) * 0.8, // 目标80%的内存限制
	}
}

// Start 启动控制器
func (a *AdaptiveController) Start() {
	a.limiter.Start()
}

// Stop 停止控制器
func (a *AdaptiveController) Stop() {
	a.limiter.Stop()
}

// GetOptimalSampleRate 获取最优采样率
func (a *AdaptiveController) GetOptimalSampleRate() int {
	return a.limiter.GetSampleRate()
}

// ReportLoad 报告负载情况
func (a *AdaptiveController) ReportLoad(cpuUsage float64, memUsage float64) {
	// 控制器内部会自动调整，这里可以添加额外的逻辑
}
