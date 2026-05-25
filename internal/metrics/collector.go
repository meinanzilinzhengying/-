/*
 * Cloud Flow Agent - Metrics Collector
 *
 * 监控指标收集器，收集边缘服务状态并上报到中心端
 */

package metrics

import (
	"context"
	"runtime"
	"sync"
	"sync/atomic"
	"time"
)

// MetricType 指标类型
type MetricType string

const (
	MetricTypeGauge   MetricType = "gauge"
	MetricTypeCounter MetricType = "counter"
	MetricTypeHistogram MetricType = "histogram"
)

// Metric 指标定义
type Metric struct {
	Name      string            `json:"name"`
	Type      MetricType        `json:"type"`
	Value     float64           `json:"value"`
	Tags      map[string]string `json:"tags"`
	Timestamp time.Time         `json:"timestamp"`
}

// MetricCollector 指标收集器
type MetricCollector struct {
	mu sync.RWMutex

	// 指标存储
	metrics map[string]*Metric
	
	// 收集器配置
	config *CollectorConfig
	
	// 收集函数
	collectors []CollectorFunc
	
	// 上报通道
	reportCh chan []*Metric
	
	// 统计
	stats CollectorStats
	
	// 控制
	ctx    context.Context
	cancel context.CancelFunc
}

// CollectorConfig 收集器配置
type CollectorConfig struct {
	Enabled           bool          // 启用收集
	CollectionInterval time.Duration // 收集间隔
	ReportInterval    time.Duration // 上报间隔
	MaxMetrics        int           // 最大指标数
	BufferSize        int           // 缓冲区大小
}

// DefaultCollectorConfig 默认配置
func DefaultCollectorConfig() *CollectorConfig {
	return &CollectorConfig{
		Enabled:            true,
		CollectionInterval: 10 * time.Second,
		ReportInterval:     60 * time.Second,
		MaxMetrics:         10000,
		BufferSize:         1000,
	}
}

// CollectorFunc 收集函数类型
type CollectorFunc func() []*Metric

// CollectorStats 收集器统计
type CollectorStats struct {
	TotalCollected uint64
	TotalReported  uint64
	TotalDropped   uint64
	CurrentMetrics int32
}

// NewMetricCollector 创建指标收集器
func NewMetricCollector(config *CollectorConfig) *MetricCollector {
	if config == nil {
		config = DefaultCollectorConfig()
	}

	ctx, cancel := context.WithCancel(context.Background())

	mc := &MetricCollector{
		config:   config,
		metrics:  make(map[string]*Metric),
		reportCh: make(chan []*Metric, config.BufferSize),
		ctx:      ctx,
		cancel:   cancel,
	}

	// 注册默认收集器
	mc.RegisterCollector(mc.collectSystemMetrics)
	mc.RegisterCollector(mc.collectRuntimeMetrics)
	mc.RegisterCollector(mc.collectNetworkMetrics)

	// 启动收集循环
	go mc.collectionLoop()
	go mc.reportLoop()

	return mc
}

// RegisterCollector 注册收集函数
func (mc *MetricCollector) RegisterCollector(fn CollectorFunc) {
	mc.mu.Lock()
	defer mc.mu.Unlock()
	mc.collectors = append(mc.collectors, fn)
}

// collectionLoop 收集循环
func (mc *MetricCollector) collectionLoop() {
	ticker := time.NewTicker(mc.config.CollectionInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			mc.collect()
		case <-mc.ctx.Done():
			return
		}
	}
}

// collect 执行收集
func (mc *MetricCollector) collect() {
	mc.mu.RLock()
	collectors := make([]CollectorFunc, len(mc.collectors))
	copy(collectors, mc.collectors)
	mc.mu.RUnlock()

	allMetrics := make([]*Metric, 0)

	for _, fn := range collectors {
		metrics := fn()
		allMetrics = append(allMetrics, metrics...)
	}

	// 存储指标
	mc.mu.Lock()
	for _, m := range allMetrics {
		key := m.Name + serializeTags(m.Tags)
		mc.metrics[key] = m
	}
	mc.mu.Unlock()

	atomic.AddUint64(&mc.stats.TotalCollected, uint64(len(allMetrics)))
	atomic.AddInt32(&mc.stats.CurrentMetrics, int32(len(allMetrics)))
}

// reportLoop 上报循环
func (mc *MetricCollector) reportLoop() {
	ticker := time.NewTicker(mc.config.ReportInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			mc.flush()
		case <-mc.ctx.Done():
			return
		}
	}
}

// flush 刷新指标到上报通道
func (mc *MetricCollector) flush() {
	mc.mu.Lock()
	metrics := make([]*Metric, 0, len(mc.metrics))
	for _, m := range mc.metrics {
		metrics = append(metrics, m)
	}
	// 清空已收集的指标
	mc.metrics = make(map[string]*Metric)
	mc.mu.Unlock()

	if len(metrics) > 0 {
		select {
		case mc.reportCh <- metrics:
			atomic.AddUint64(&mc.stats.TotalReported, uint64(len(metrics)))
			atomic.AddInt32(&mc.stats.CurrentMetrics, -int32(len(metrics)))
		default:
			// 通道满，丢弃
			atomic.AddUint64(&mc.stats.TotalDropped, uint64(len(metrics)))
		}
	}
}

// GetReportChannel 获取上报通道
func (mc *MetricCollector) GetReportChannel() <-chan []*Metric {
	return mc.reportCh
}

// GetMetrics 获取当前所有指标
func (mc *MetricCollector) GetMetrics() []*Metric {
	mc.mu.RLock()
	defer mc.mu.RUnlock()

	metrics := make([]*Metric, 0, len(mc.metrics))
	for _, m := range mc.metrics {
		metrics = append(metrics, m)
	}
	return metrics
}

// GetStats 获取统计
func (mc *MetricCollector) GetStats() CollectorStats {
	return CollectorStats{
		TotalCollected: atomic.LoadUint64(&mc.stats.TotalCollected),
		TotalReported:  atomic.LoadUint64(&mc.stats.TotalReported),
		TotalDropped:   atomic.LoadUint64(&mc.stats.TotalDropped),
		CurrentMetrics: atomic.LoadInt32(&mc.stats.CurrentMetrics),
	}
}

// collectSystemMetrics 收集系统指标
func (mc *MetricCollector) collectSystemMetrics() []*Metric {
	metrics := make([]*Metric, 0)
	now := time.Now()

	// CPU 使用率
	metrics = append(metrics, &Metric{
		Name:      "system.cpu.usage",
		Type:      MetricTypeGauge,
		Value:     getCPUUsage(),
		Tags:      map[string]string{"type": "system"},
		Timestamp: now,
	})

	// 内存使用
	var m runtime.MemStats
	runtime.ReadMemStats(&m)
	metrics = append(metrics, &Metric{
		Name:      "system.memory.used",
		Type:      MetricTypeGauge,
		Value:     float64(m.Sys),
		Tags:      map[string]string{"type": "system"},
		Timestamp: now,
	})

	// Goroutine 数量
	metrics = append(metrics, &Metric{
		Name:      "system.goroutines",
		Type:      MetricTypeGauge,
		Value:     float64(runtime.NumGoroutine()),
		Tags:      map[string]string{"type": "system"},
		Timestamp: now,
	})

	return metrics
}

// collectRuntimeMetrics 收集运行时指标
func (mc *MetricCollector) collectRuntimeMetrics() []*Metric {
	metrics := make([]*Metric, 0)
	now := time.Now()
	var m runtime.MemStats
	runtime.ReadMemStats(&m)

	// GC 统计
	metrics = append(metrics, &Metric{
		Name:      "runtime.gc.count",
		Type:      MetricTypeCounter,
		Value:     float64(m.NumGC),
		Tags:      map[string]string{},
		Timestamp: now,
	})

	metrics = append(metrics, &Metric{
		Name:      "runtime.gc.pause_ns",
		Type:      MetricTypeGauge,
		Value:     float64(m.PauseNs[(m.NumGC+255)%256]),
		Tags:      map[string]string{},
		Timestamp: now,
	})

	// 堆内存
	metrics = append(metrics, &Metric{
		Name:      "runtime.heap.alloc",
		Type:      MetricTypeGauge,
		Value:     float64(m.HeapAlloc),
		Tags:      map[string]string{},
		Timestamp: now,
	})

	metrics = append(metrics, &Metric{
		Name:      "runtime.heap.sys",
		Type:      MetricTypeGauge,
		Value:     float64(m.HeapSys),
		Tags:      map[string]string{},
		Timestamp: now,
	})

	return metrics
}

// collectNetworkMetrics 收集网络指标
func (mc *MetricCollector) collectNetworkMetrics() []*Metric {
	metrics := make([]*Metric, 0)
	now := time.Now()

	// 连接数（模拟）
	metrics = append(metrics, &Metric{
		Name:      "network.connections.active",
		Type:      MetricTypeGauge,
		Value:     0, // 实际实现需要读取 /proc/net/tcp
		Tags:      map[string]string{},
		Timestamp: now,
	})

	return metrics
}

// getCPUUsage 获取 CPU 使用率（简化实现）
func getCPUUsage() float64 {
	// 实际实现应读取 /proc/stat
	return 0.0
}

// serializeTags 序列化标签
func serializeTags(tags map[string]string) string {
	result := ""
	for k, v := range tags {
		result += k + "=" + v + ";"
	}
	return result
}

// Close 关闭收集器
func (mc *MetricCollector) Close() error {
	mc.cancel()
	close(mc.reportCh)
	return nil
}
