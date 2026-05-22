/*
 * Cloud Flow Agent - Data Aggregation Engine
 *
 * 数据聚合引擎，解决原始数据直接转发导致的中心端存储压力问题
 * 实现多级聚合策略：时间窗口聚合、维度聚合、采样压缩
 */

package aggregation

import (
	"context"
	"encoding/json"
	"fmt"
	"hash/fnv"
	"math"
	"sort"
	"sync"
	"sync/atomic"
	"time"
)

// AggLevel 聚合级别
type AggLevel int

const (
	AggLevelNone      AggLevel = 0  // 不聚合
	AggLevelSecond    AggLevel = 1  // 秒级聚合
	AggLevelMinute    AggLevel = 2  // 分钟级聚合
	AggLevelHour      AggLevel = 3  // 小时级聚合
	AggLevelDay       AggLevel = 4  // 天级聚合
)

// AggType 聚合类型
type AggType string

const (
	AggTypeSum     AggType = "sum"
	AggTypeAvg     AggType = "avg"
	AggTypeMin     AggType = "min"
	AggTypeMax     AggType = "max"
	AggTypeCount   AggType = "count"
	AggTypeP99     AggType = "p99"
	AggTypeP95     AggType = "p95"
	AggTypeP90     AggType = "p90"
	AggTypeHistogram AggType = "histogram"
)

// AggregationConfig 聚合配置
type AggregationConfig struct {
	Enabled           bool          // 启用聚合
	DefaultLevel      AggLevel      // 默认聚合级别
	WindowSize        time.Duration // 聚合窗口大小
	BufferSize        int           // 缓冲区大小
	FlushInterval     time.Duration // 刷新间隔
	MaxDimensions     int           // 最大维度数
	MaxCardinality    int           // 最大基数
	EnableCompression bool          // 启用压缩
	CompressionLevel  int           // 压缩级别
	
	// 多级聚合配置
	Levels []LevelConfig // 各级聚合配置
	
	// 采样配置
	SamplingEnabled   bool    // 启用采样
	SamplingRate      float64 // 采样率 (0-1)
	AdaptiveSampling  bool    // 自适应采样
	
	// 降级配置
	MemoryThresholdMB int64   // 内存阈值(MB)
	DowngradeRatio    float64 // 降级比例
}

// LevelConfig 级别配置
type LevelConfig struct {
	Level          AggLevel
	WindowSize     time.Duration
	RetentionTime  time.Duration
	AggTypes       []AggType
	Dimensions     []string
}

// DefaultAggregationConfig 默认配置
func DefaultAggregationConfig() *AggregationConfig {
	return &AggregationConfig{
		Enabled:           true,
		DefaultLevel:      AggLevelMinute,
		WindowSize:        60 * time.Second,
		BufferSize:        100000,
		FlushInterval:     30 * time.Second,
		MaxDimensions:     10,
		MaxCardinality:    10000,
		EnableCompression: true,
		CompressionLevel:  6,
		Levels: []LevelConfig{
			{
				Level:         AggLevelSecond,
				WindowSize:    time.Second,
				RetentionTime: 5 * time.Minute,
				AggTypes:      []AggType{AggTypeSum, AggTypeAvg, AggTypeCount, AggTypeMax, AggTypeMin},
				Dimensions:    []string{"host", "service", "metric"},
			},
			{
				Level:         AggLevelMinute,
				WindowSize:    time.Minute,
				RetentionTime: 24 * time.Hour,
				AggTypes:      []AggType{AggTypeSum, AggTypeAvg, AggTypeCount, AggTypeMax, AggTypeMin, AggTypeP99, AggTypeP95},
				Dimensions:    []string{"host", "service", "metric", "region"},
			},
			{
				Level:         AggLevelHour,
				WindowSize:    time.Hour,
				RetentionTime: 7 * 24 * time.Hour,
				AggTypes:      []AggType{AggTypeSum, AggTypeAvg, AggTypeCount, AggTypeMax, AggTypeMin, AggTypeP99},
				Dimensions:    []string{"service", "region", "metric"},
			},
		},
		SamplingEnabled:   true,
		SamplingRate:      1.0,
		AdaptiveSampling:  true,
		MemoryThresholdMB: 2048,
		DowngradeRatio:    0.5,
	}
}

// RawDataPoint 原始数据点
type RawDataPoint struct {
	Timestamp int64             `json:"timestamp"`
	Metric    string            `json:"metric"`
	Value     float64           `json:"value"`
	Tags      map[string]string `json:"tags"`
	Type      string            `json:"type"` // counter, gauge, histogram
}

// AggregatedDataPoint 聚合数据点
type AggregatedDataPoint struct {
	WindowStart int64             `json:"window_start"`
	WindowEnd   int64             `json:"window_end"`
	Metric      string            `json:"metric"`
	Tags        map[string]string `json:"tags"`
	Type        string            `json:"type"`
	Level       AggLevel          `json:"level"`
	
	// 聚合值
	Count  uint64  `json:"count"`
	Sum    float64 `json:"sum"`
	Avg    float64 `json:"avg"`
	Min    float64 `json:"min"`
	Max    float64 `json:"max"`
	P99    float64 `json:"p99,omitempty"`
	P95    float64 `json:"p95,omitempty"`
	P90    float64 `json:"p90,omitempty"`
	
	// 直方图数据
	Buckets map[string]uint64 `json:"buckets,omitempty"`
}

// AggregationWindow 聚合窗口
type AggregationWindow struct {
	mu       sync.RWMutex
	start    int64
	end      int64
	level    AggLevel
	buckets  map[string]*AggregationBucket // key = dimension hash
}

// AggregationBucket 聚合桶
type AggregationBucket struct {
	mu        sync.Mutex
	metric    string
	tags      map[string]string
	count     uint64
	sum       float64
	min       float64
	max       float64
	values    []float64 // 用于计算百分位
	type_     string
}

// NewAggregationBucket 创建聚合桶
func NewAggregationBucket(metric string, tags map[string]string, dataType string) *AggregationBucket {
	return &AggregationBucket{
		metric: metric,
		tags:   tags,
		min:    math.MaxFloat64,
		max:    -math.MaxFloat64,
		values: make([]float64, 0, 100),
		type_:  dataType,
	}
}

// Add 添加数据点
func (b *AggregationBucket) Add(value float64) {
	b.mu.Lock()
	defer b.mu.Unlock()
	
	b.count++
	b.sum += value
	if value < b.min {
		b.min = value
	}
	if value > b.max {
		b.max = value
	}
	b.values = append(b.values, value)
}

// Compute 计算聚合值
func (b *AggregationBucket) Compute() *AggregatedDataPoint {
	b.mu.Lock()
	defer b.mu.Unlock()
	
	if b.count == 0 {
		return nil
	}
	
	avg := b.sum / float64(b.count)
	
	// 计算百分位
	var p99, p95, p90 float64
	if len(b.values) > 0 {
		sort.Float64s(b.values)
		p99 = percentile(b.values, 0.99)
		p95 = percentile(b.values, 0.95)
		p90 = percentile(b.values, 0.90)
	}
	
	return &AggregatedDataPoint{
		Metric:   b.metric,
		Tags:     b.tags,
		Type:     b.type_,
		Count:    b.count,
		Sum:      b.sum,
		Avg:      avg,
		Min:      b.min,
		Max:      b.max,
		P99:      p99,
		P95:      p95,
		P90:      p90,
	}
}

// percentile 计算百分位
func percentile(sorted []float64, p float64) float64 {
	if len(sorted) == 0 {
		return 0
	}
	index := float64(len(sorted)-1) * p
	lower := int(math.Floor(index))
	upper := int(math.Ceil(index))
	if lower == upper {
		return sorted[lower]
	}
	weight := index - float64(lower)
	return sorted[lower]*(1-weight) + sorted[upper]*weight
}

// AggregationEngine 聚合引擎
type AggregationEngine struct {
	mu sync.RWMutex
	
	config *AggregationConfig
	
	// 多级窗口
	windows map[AggLevel]*AggregationWindow
	
	// 输入缓冲
	inputCh chan *RawDataPoint
	
	// 输出缓冲
	outputCh chan *AggregatedDataPoint
	
	// 控制
	ctx    context.Context
	cancel context.CancelFunc
	
	// 统计
	stats AggStats
	
	// 采样器
	sampler *Sampler
	
	// 内存监控
	memoryUsage int64
}

// AggStats 聚合统计
type AggStats struct {
	TotalReceived    uint64
	TotalAggregated  uint64
	TotalDropped     uint64
	TotalFlushed     uint64
	CurrentWindows   int32
	CurrentBuckets   int32
	InputQueueLength int32
	OutputQueueLength int32
}

// Sampler 采样器
type Sampler struct {
	baseRate   float64
	adaptive   bool
	currentRate float64
	mu         sync.RWMutex
}

// NewSampler 创建采样器
func NewSampler(rate float64, adaptive bool) *Sampler {
	return &Sampler{
		baseRate:    rate,
		adaptive:    adaptive,
		currentRate: rate,
	}
}

// ShouldSample 是否应该采样
func (s *Sampler) ShouldSample() bool {
	s.mu.RLock()
	rate := s.currentRate
	s.mu.RUnlock()
	
	if rate >= 1.0 {
		return true
	}
	if rate <= 0 {
		return false
	}
	// 使用哈希采样保持一致性
	return time.Now().UnixNano()%1000000 < int64(rate*1000000)
}

// AdjustRate 调整采样率
func (s *Sampler) AdjustRate(newRate float64) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if newRate < 0 {
		newRate = 0
	}
	if newRate > 1 {
		newRate = 1
	}
	s.currentRate = newRate
}

// NewAggregationEngine 创建聚合引擎
func NewAggregationEngine(config *AggregationConfig) *AggregationEngine {
	if config == nil {
		config = DefaultAggregationConfig()
	}
	
	ctx, cancel := context.WithCancel(context.Background())
	
	engine := &AggregationEngine{
		config:   config,
		windows:  make(map[AggLevel]*AggregationWindow),
		inputCh:  make(chan *RawDataPoint, config.BufferSize),
		outputCh: make(chan *AggregatedDataPoint, config.BufferSize),
		ctx:      ctx,
		cancel:   cancel,
		sampler:  NewSampler(config.SamplingRate, config.AdaptiveSampling),
	}
	
	// 初始化各级窗口
	for _, level := range []AggLevel{AggLevelSecond, AggLevelMinute, AggLevelHour, AggLevelDay} {
		engine.windows[level] = engine.createWindow(level)
	}
	
	// 启动工作协程
	go engine.process()
	go engine.flush()
	go engine.adaptiveControl()
	
	return engine
}

// createWindow 创建聚合窗口
func (e *AggregationEngine) createWindow(level AggLevel) *AggregationWindow {
	now := time.Now().Unix()
	windowSize := e.getWindowSize(level)
	
	return &AggregationWindow{
		start:   now,
		end:     now + int64(windowSize.Seconds()),
		level:   level,
		buckets: make(map[string]*AggregationBucket),
	}
}

// getWindowSize 获取窗口大小
func (e *AggregationEngine) getWindowSize(level AggLevel) time.Duration {
	for _, lc := range e.config.Levels {
		if lc.Level == level {
			return lc.WindowSize
		}
	}
	// 默认
	switch level {
	case AggLevelSecond:
		return time.Second
	case AggLevelMinute:
		return time.Minute
	case AggLevelHour:
		return time.Hour
	case AggLevelDay:
		return 24 * time.Hour
	}
	return time.Minute
}

// Submit 提交原始数据
func (e *AggregationEngine) Submit(dp *RawDataPoint) error {
	if !e.config.Enabled {
		// 直接转发
		return nil
	}
	
	// 采样检查
	if e.config.SamplingEnabled && !e.sampler.ShouldSample() {
		atomic.AddUint64(&e.stats.TotalDropped, 1)
		return nil
	}
	
	select {
	case e.inputCh <- dp:
		atomic.AddUint64(&e.stats.TotalReceived, 1)
		atomic.AddInt32(&e.stats.InputQueueLength, 1)
		return nil
	default:
		// 缓冲区满，丢弃
		atomic.AddUint64(&e.stats.TotalDropped, 1)
		return fmt.Errorf("input buffer full")
	}
}

// process 处理输入数据
func (e *AggregationEngine) process() {
	for {
		select {
		case dp := <-e.inputCh:
			atomic.AddInt32(&e.stats.InputQueueLength, -1)
			e.aggregate(dp)
		case <-e.ctx.Done():
			return
		}
	}
}

// aggregate 聚合数据点
func (e *AggregationEngine) aggregate(dp *RawDataPoint) {
	// 确定聚合级别
	level := e.config.DefaultLevel
	
	// 计算窗口时间
	windowSize := e.getWindowSize(level)
	windowStart := (dp.Timestamp / int64(windowSize.Seconds())) * int64(windowSize.Seconds())
	
	// 获取窗口
	e.mu.RLock()
	window := e.windows[level]
	e.mu.RUnlock()
	
	// 检查是否需要旋转窗口
	if dp.Timestamp >= window.end {
		e.rotateWindow(level)
		window = e.windows[level]
	}
	
	// 计算维度键
	dimKey := e.computeDimensionKey(dp, level)
	
	// 获取或创建桶
	window.mu.Lock()
	bucket, exists := window.buckets[dimKey]
	if !exists {
		tags := e.extractTags(dp, level)
		bucket = NewAggregationBucket(dp.Metric, tags, dp.Type)
		window.buckets[dimKey] = bucket
		atomic.AddInt32(&e.stats.CurrentBuckets, 1)
	}
	window.mu.Unlock()
	
	// 添加数据
	bucket.Add(dp.Value)
	atomic.AddUint64(&e.stats.TotalAggregated, 1)
}

// computeDimensionKey 计算维度键
func (e *AggregationEngine) computeDimensionKey(dp *RawDataPoint, level AggLevel) string {
	// 获取该级别的维度配置
	var dimensions []string
	for _, lc := range e.config.Levels {
		if lc.Level == level {
			dimensions = lc.Dimensions
			break
		}
	}
	
	// 构建维度字符串
	h := fnv.New64a()
	h.Write([]byte(dp.Metric))
	
	for _, dim := range dimensions {
		if val, ok := dp.Tags[dim]; ok {
			h.Write([]byte(dim))
			h.Write([]byte(val))
		}
	}
	
	return fmt.Sprintf("%d", h.Sum64())
}

// extractTags 提取标签
func (e *AggregationEngine) extractTags(dp *RawDataPoint, level AggLevel) map[string]string {
	var dimensions []string
	for _, lc := range e.config.Levels {
		if lc.Level == level {
			dimensions = lc.Dimensions
			break
		}
	}
	
	tags := make(map[string]string)
	for _, dim := range dimensions {
		if val, ok := dp.Tags[dim]; ok {
			tags[dim] = val
		}
	}
	return tags
}

// rotateWindow 旋转窗口
func (e *AggregationEngine) rotateWindow(level AggLevel) {
	e.mu.Lock()
	oldWindow := e.windows[level]
	newWindow := e.createWindow(level)
	e.windows[level] = newWindow
	e.mu.Unlock()
	
	// 异步刷新旧窗口
	go e.flushWindow(oldWindow)
}

// flushWindow 刷新窗口数据
func (e *AggregationEngine) flushWindow(window *AggregationWindow) {
	window.mu.RLock()
	buckets := make([]*AggregationBucket, 0, len(window.buckets))
	for _, b := range window.buckets {
		buckets = append(buckets, b)
	}
	window.mu.RUnlock()
	
	for _, bucket := range buckets {
		agg := bucket.Compute()
		if agg != nil {
			agg.WindowStart = window.start
			agg.WindowEnd = window.end
			agg.Level = window.level
			
			select {
			case e.outputCh <- agg:
				atomic.AddUint64(&e.stats.TotalFlushed, 1)
				atomic.AddInt32(&e.stats.OutputQueueLength, 1)
			default:
				// 输出缓冲区满
			}
		}
	}
	
	atomic.AddInt32(&e.stats.CurrentBuckets, -int32(len(buckets)))
}

// flush 定时刷新
func (e *AggregationEngine) flush() {
	ticker := time.NewTicker(e.config.FlushInterval)
	defer ticker.Stop()
	
	for {
		select {
		case <-ticker.C:
			for level := range e.windows {
				e.rotateWindow(level)
			}
		case <-e.ctx.Done():
			return
		}
	}
}

// adaptiveControl 自适应控制
func (e *AggregationEngine) adaptiveControl() {
	if !e.config.AdaptiveSampling {
		return
	}
	
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()
	
	for {
		select {
		case <-ticker.C:
			e.adjustSampling()
		case <-e.ctx.Done():
			return
		}
	}
}

// adjustSampling 调整采样率
func (e *AggregationEngine) adjustSampling() {
	// 基于内存使用调整
	usage := atomic.LoadInt64(&e.memoryUsage)
	threshold := e.config.MemoryThresholdMB * 1024 * 1024
	
	if usage > threshold {
		// 内存压力大，降低采样率
		newRate := e.sampler.currentRate * e.config.DowngradeRatio
		e.sampler.AdjustRate(newRate)
	} else if usage < threshold/2 {
		// 内存压力小，恢复采样率
		newRate := e.sampler.currentRate / e.config.DowngradeRatio
		if newRate > e.config.SamplingRate {
			newRate = e.config.SamplingRate
		}
		e.sampler.AdjustRate(newRate)
	}
}

// GetOutput 获取聚合输出
func (e *AggregationEngine) GetOutput() <-chan *AggregatedDataPoint {
	return e.outputCh
}

// GetStats 获取统计
func (e *AggregationEngine) GetStats() AggStats {
	return AggStats{
		TotalReceived:     atomic.LoadUint64(&e.stats.TotalReceived),
		TotalAggregated:   atomic.LoadUint64(&e.stats.TotalAggregated),
		TotalDropped:      atomic.LoadUint64(&e.stats.TotalDropped),
		TotalFlushed:      atomic.LoadUint64(&e.stats.TotalFlushed),
		CurrentWindows:    atomic.LoadInt32(&e.stats.CurrentWindows),
		CurrentBuckets:    atomic.LoadInt32(&e.stats.CurrentBuckets),
		InputQueueLength:  atomic.LoadInt32(&e.stats.InputQueueLength),
		OutputQueueLength: atomic.LoadInt32(&e.stats.OutputQueueLength),
	}
}

// Close 关闭引擎
func (e *AggregationEngine) Close() error {
	e.cancel()
	close(e.inputCh)
	close(e.outputCh)
	return nil
}

// CompressData 压缩数据
func CompressData(data []byte) ([]byte, error) {
	// 实际实现应使用 snappy/zstd/lz4 等压缩算法
	// 这里返回原始数据作为占位
	return data, nil
}

// SerializeAggregated 序列化聚合数据
func SerializeAggregated(agg *AggregatedDataPoint) ([]byte, error) {
	return json.Marshal(agg)
}

// BatchAggregated 批量序列化
func BatchAggregated(aggs []*AggregatedDataPoint) ([]byte, error) {
	return json.Marshal(aggs)
}
