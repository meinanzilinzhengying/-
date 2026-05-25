// Package asset 指标聚合引擎
// 支持按资产维度聚合网络/应用指标
package asset

import (
	"context"
	"math"
	"sort"
	"sync"
	"time"
)

// AggregationType 聚合类型
type AggregationType string

const (
	AggregationAvg   AggregationType = "avg"
	AggregationSum   AggregationType = "sum"
	AggregationMax   AggregationType = "max"
	AggregationMin   AggregationType = "min"
	AggregationCount AggregationType = "count"
	AggregationP99   AggregationType = "p99"
	AggregationP95   AggregationType = "p95"
	AggregationP90   AggregationType = "p90"
)

// TimeGranularity 时间粒度
type TimeGranularity string

const (
	GranularityRaw   TimeGranularity = "raw"
	Granularity1s    TimeGranularity = "1s"
	Granularity10s   TimeGranularity = "10s"
	Granularity1m    TimeGranularity = "1m"
	Granularity5m    TimeGranularity = "5m"
	Granularity1h    TimeGranularity = "1h"
	Granularity1d    TimeGranularity = "1d"
)

// AggregatedMetrics 聚合指标
type AggregatedMetrics struct {
	AssetID       string                 `json:"asset_id"`
	AssetType     AssetType              `json:"asset_type"`
	AssetName     string                 `json:"asset_name"`
	Timestamp     time.Time              `json:"timestamp"`
	Granularity   TimeGranularity        `json:"granularity"`
	WindowStart   time.Time              `json:"window_start"`
	WindowEnd     time.Time              `json:"window_end"`
	SampleCount   int                    `json:"sample_count"`
	
	// 网络指标聚合
	Network       *AggregatedNetwork     `json:"network,omitempty"`
	
	// 应用指标聚合
	Application   *AggregatedApplication `json:"application,omitempty"`
	
	// 系统指标聚合
	System        *AggregatedSystem      `json:"system,omitempty"`
}

// AggregatedNetwork 聚合网络指标
type AggregatedNetwork struct {
	BytesSent      AggregatedValue `json:"bytes_sent"`
	BytesRecv      AggregatedValue `json:"bytes_recv"`
	PacketsSent    AggregatedValue `json:"packets_sent"`
	PacketsRecv    AggregatedValue `json:"packets_recv"`
	ErrorsIn       AggregatedValue `json:"errors_in"`
	ErrorsOut      AggregatedValue `json:"errors_out"`
	DropsIn        AggregatedValue `json:"drops_in"`
	DropsOut       AggregatedValue `json:"drops_out"`
	Connections    AggregatedValue `json:"connections"`
	LatencyMs      AggregatedValue `json:"latency_ms"`
	Retransmits    AggregatedValue `json:"retransmits"`
	
	// 速率指标
	BytesSentRate  float64         `json:"bytes_sent_rate"`
	BytesRecvRate  float64         `json:"bytes_recv_rate"`
	PacketsSentRate float64        `json:"packets_sent_rate"`
	PacketsRecvRate float64        `json:"packets_recv_rate"`
}

// AggregatedApplication 聚合应用指标
type AggregatedApplication struct {
	CPUUsage       AggregatedValue `json:"cpu_usage"`
	MemoryUsage    AggregatedValue `json:"memory_usage"`
	MemoryRSS      AggregatedValue `json:"memory_rss"`
	MemoryVMS      AggregatedValue `json:"memory_vms"`
	OpenFiles      AggregatedValue `json:"open_files"`
	Threads        AggregatedValue `json:"threads"`
	ResponseTime   AggregatedValue `json:"response_time"`
	ErrorRate      AggregatedValue `json:"error_rate"`
	Throughput     AggregatedValue `json:"throughput"`
	GCCount        AggregatedValue `json:"gc_count"`
	GCTime         AggregatedValue `json:"gc_time"`
}

// AggregatedSystem 聚合系统指标
type AggregatedSystem struct {
	CPUUsage       AggregatedValue `json:"cpu_usage"`
	MemoryUsage    AggregatedValue `json:"memory_usage"`
	MemoryUsed     AggregatedValue `json:"memory_used"`
	DiskUsage      AggregatedValue `json:"disk_usage"`
	DiskUsed       AggregatedValue `json:"disk_used"`
	Load1          AggregatedValue `json:"load1"`
	Load5          AggregatedValue `json:"load5"`
	Load15         AggregatedValue `json:"load15"`
}

// AggregatedValue 聚合值
type AggregatedValue struct {
	Min    float64 `json:"min"`
	Max    float64 `json:"max"`
	Avg    float64 `json:"avg"`
	Sum    float64 `json:"sum"`
	Count  int     `json:"count"`
	P99    float64 `json:"p99"`
	P95    float64 `json:"p95"`
	P90    float64 `json:"p90"`
	Last   float64 `json:"last"`
}

// AggregatorConfig 聚合器配置
type AggregatorConfig struct {
	Enabled           bool            `json:"enabled" yaml:"enabled"`
	Interval          time.Duration   `json:"interval" yaml:"interval"`
	DefaultGranularity TimeGranularity `json:"default_granularity" yaml:"default_granularity"`
	RetentionPeriod   time.Duration   `json:"retention_period" yaml:"retention_period"`
	MaxDataPoints     int             `json:"max_data_points" yaml:"max_data_points"`
	EnablePreAggregation bool         `json:"enable_pre_aggregation" yaml:"enable_pre_aggregation"`
	PreAggGranularities []TimeGranularity `json:"pre_agg_granularities" yaml:"pre_agg_granularities"`
}

// MetricsAggregator 指标聚合器
type MetricsAggregator struct {
	config      *AggregatorConfig
	collector   *MetricsCollector
	aggregated  map[string]map[TimeGranularity][]*AggregatedMetrics
	rawBuffer   map[string][]*AssetMetrics
	mu          sync.RWMutex
	ctx         context.Context
	cancel      context.CancelFunc
	wg          sync.WaitGroup
}

// NewMetricsAggregator 创建指标聚合器
func NewMetricsAggregator(config *AggregatorConfig, collector *MetricsCollector) *MetricsAggregator {
	ctx, cancel := context.WithCancel(context.Background())
	
	return &MetricsAggregator{
		config:     config,
		collector:  collector,
		aggregated: make(map[string]map[TimeGranularity][]*AggregatedMetrics),
		rawBuffer:  make(map[string][]*AssetMetrics),
		ctx:        ctx,
		cancel:     cancel,
	}
}

// Start 启动聚合器
func (a *MetricsAggregator) Start() error {
	// 注册采集器事件处理器
	if a.collector != nil {
		a.collector.RegisterHandler(&aggregatorMetricsHandler{aggregator: a})
	}
	
	// 启动预聚合协程
	if a.config.EnablePreAggregation {
		a.wg.Add(1)
		go a.preAggregationLoop()
	}
	
	return nil
}

// Stop 停止聚合器
func (a *MetricsAggregator) Stop() {
	a.cancel()
	a.wg.Wait()
}

// aggregatorMetricsHandler 指标事件处理器适配器
type aggregatorMetricsHandler struct {
	aggregator *MetricsAggregator
}

func (h *aggregatorMetricsHandler) OnMetricsCollected(metrics *AssetMetrics) {
	h.aggregator.bufferRawMetrics(metrics)
}

func (h *aggregatorMetricsHandler) OnMetricsUpdated(assetID string, metrics *AssetMetrics) {
	h.aggregator.bufferRawMetrics(metrics)
}

// bufferRawMetrics 缓冲原始指标
func (a *MetricsAggregator) bufferRawMetrics(metrics *AssetMetrics) {
	a.mu.Lock()
	defer a.mu.Unlock()
	
	buffer := a.rawBuffer[metrics.AssetID]
	buffer = append(buffer, metrics)
	
	// 限制缓冲区大小
	if len(buffer) > a.config.MaxDataPoints {
		buffer = buffer[len(buffer)-a.config.MaxDataPoints:]
	}
	
	a.rawBuffer[metrics.AssetID] = buffer
}

// preAggregationLoop 预聚合循环
func (a *MetricsAggregator) preAggregationLoop() {
	defer a.wg.Done()
	
	ticker := time.NewTicker(a.config.Interval)
	defer ticker.Stop()
	
	for {
		select {
		case <-a.ctx.Done():
			return
		case <-ticker.C:
			a.performPreAggregation()
		}
	}
}

// performPreAggregation 执行预聚合
func (a *MetricsAggregator) performPreAggregation() {
	a.mu.Lock()
	defer a.mu.Unlock()
	
	for assetID, buffer := range a.rawBuffer {
		if len(buffer) == 0 {
			continue
		}
		
		for _, granularity := range a.config.PreAggGranularities {
			aggregated := a.aggregateMetrics(assetID, buffer, granularity)
			if aggregated != nil {
				if a.aggregated[assetID] == nil {
					a.aggregated[assetID] = make(map[TimeGranularity][]*AggregatedMetrics)
				}
				a.aggregated[assetID][granularity] = append(a.aggregated[assetID][granularity], aggregated)
			}
		}
		
		// 清空缓冲区
		a.rawBuffer[assetID] = nil
	}
}

// aggregateMetrics 聚合指标
func (a *MetricsAggregator) aggregateMetrics(assetID string, metrics []*AssetMetrics, granularity TimeGranularity) *AggregatedMetrics {
	if len(metrics) == 0 {
		return nil
	}
	
	first := metrics[0]
	last := metrics[len(metrics)-1]
	
	aggregated := &AggregatedMetrics{
		AssetID:     assetID,
		AssetType:   first.AssetType,
		AssetName:   first.AssetName,
		Timestamp:   time.Now(),
		Granularity: granularity,
		WindowStart: first.Timestamp,
		WindowEnd:   last.Timestamp,
		SampleCount: len(metrics),
	}
	
	// 聚合网络指标
	if first.Network != nil {
		aggregated.Network = a.aggregateNetworkMetrics(metrics)
	}
	
	// 聚合应用指标
	if first.Application != nil {
		aggregated.Application = a.aggregateApplicationMetrics(metrics)
	}
	
	// 聚合系统指标
	if first.System != nil {
		aggregated.System = a.aggregateSystemMetrics(metrics)
	}
	
	return aggregated
}

// aggregateNetworkMetrics 聚合网络指标
func (a *MetricsAggregator) aggregateNetworkMetrics(metrics []*AssetMetrics) *AggregatedNetwork {
	agg := &AggregatedNetwork{}
	
	var bytesSent, bytesRecv []float64
	var packetsSent, packetsRecv []float64
	var errorsIn, errorsOut []float64
	var dropsIn, dropsOut []float64
	var connections []float64
	var latency []float64
	var retransmits []float64
	
	for _, m := range metrics {
		if m.Network == nil {
			continue
		}
		
		bytesSent = append(bytesSent, float64(m.Network.BytesSent))
		bytesRecv = append(bytesRecv, float64(m.Network.BytesRecv))
		packetsSent = append(packetsSent, float64(m.Network.PacketsSent))
		packetsRecv = append(packetsRecv, float64(m.Network.PacketsRecv))
		errorsIn = append(errorsIn, float64(m.Network.ErrorsIn))
		errorsOut = append(errorsOut, float64(m.Network.ErrorsOut))
		dropsIn = append(dropsIn, float64(m.Network.DropsIn))
		dropsOut = append(dropsOut, float64(m.Network.DropsOut))
		connections = append(connections, float64(m.Network.Connections))
		latency = append(latency, m.Network.LatencyMs)
		retransmits = append(retransmits, float64(m.Network.Retransmits))
	}
	
	agg.BytesSent = calculateAggregatedValue(bytesSent)
	agg.BytesRecv = calculateAggregatedValue(bytesRecv)
	agg.PacketsSent = calculateAggregatedValue(packetsSent)
	agg.PacketsRecv = calculateAggregatedValue(packetsRecv)
	agg.ErrorsIn = calculateAggregatedValue(errorsIn)
	agg.ErrorsOut = calculateAggregatedValue(errorsOut)
	agg.DropsIn = calculateAggregatedValue(dropsIn)
	agg.DropsOut = calculateAggregatedValue(dropsOut)
	agg.Connections = calculateAggregatedValue(connections)
	agg.LatencyMs = calculateAggregatedValue(latency)
	agg.Retransmits = calculateAggregatedValue(retransmits)
	
	// 计算速率
	duration := float64(len(metrics)) * float64(a.config.Interval.Seconds())
	if duration > 0 {
		agg.BytesSentRate = agg.BytesSent.Sum / duration
		agg.BytesRecvRate = agg.BytesRecv.Sum / duration
		agg.PacketsSentRate = agg.PacketsSent.Sum / duration
		agg.PacketsRecvRate = agg.PacketsRecv.Sum / duration
	}
	
	return agg
}

// aggregateApplicationMetrics 聚合应用指标
func (a *MetricsAggregator) aggregateApplicationMetrics(metrics []*AssetMetrics) *AggregatedApplication {
	agg := &AggregatedApplication{}
	
	var cpuUsage, memoryUsage []float64
	var memoryRSS, memoryVMS []float64
	var openFiles, threads []float64
	var responseTime, errorRate, throughput []float64
	var gcCount, gcTime []float64
	
	for _, m := range metrics {
		if m.Application == nil {
			continue
		}
		
		cpuUsage = append(cpuUsage, m.Application.CPUUsage)
		memoryUsage = append(memoryUsage, m.Application.MemoryUsage)
		memoryRSS = append(memoryRSS, float64(m.Application.MemoryRSS))
		memoryVMS = append(memoryVMS, float64(m.Application.MemoryVMS))
		openFiles = append(openFiles, float64(m.Application.OpenFiles))
		threads = append(threads, float64(m.Application.Threads))
		responseTime = append(responseTime, m.Application.ResponseTime)
		errorRate = append(errorRate, m.Application.ErrorRate)
		throughput = append(throughput, m.Application.Throughput)
		gcCount = append(gcCount, float64(m.Application.GCCount))
		gcTime = append(gcTime, m.Application.GCTime)
	}
	
	agg.CPUUsage = calculateAggregatedValue(cpuUsage)
	agg.MemoryUsage = calculateAggregatedValue(memoryUsage)
	agg.MemoryRSS = calculateAggregatedValue(memoryRSS)
	agg.MemoryVMS = calculateAggregatedValue(memoryVMS)
	agg.OpenFiles = calculateAggregatedValue(openFiles)
	agg.Threads = calculateAggregatedValue(threads)
	agg.ResponseTime = calculateAggregatedValue(responseTime)
	agg.ErrorRate = calculateAggregatedValue(errorRate)
	agg.Throughput = calculateAggregatedValue(throughput)
	agg.GCCount = calculateAggregatedValue(gcCount)
	agg.GCTime = calculateAggregatedValue(gcTime)
	
	return agg
}

// aggregateSystemMetrics 聚合系统指标
func (a *MetricsAggregator) aggregateSystemMetrics(metrics []*AssetMetrics) *AggregatedSystem {
	agg := &AggregatedSystem{}
	
	var cpuUsage, memoryUsage []float64
	var memoryUsed []float64
	var diskUsage, diskUsed []float64
	var load1, load5, load15 []float64
	
	for _, m := range metrics {
		if m.System == nil {
			continue
		}
		
		cpuUsage = append(cpuUsage, m.System.CPUUsage)
		memoryUsage = append(memoryUsage, m.System.MemoryUsage)
		memoryUsed = append(memoryUsed, float64(m.System.MemoryUsed))
		diskUsage = append(diskUsage, m.System.DiskUsage)
		diskUsed = append(diskUsed, float64(m.System.DiskUsed))
		load1 = append(load1, m.System.Load1)
		load5 = append(load5, m.System.Load5)
		load15 = append(load15, m.System.Load15)
	}
	
	agg.CPUUsage = calculateAggregatedValue(cpuUsage)
	agg.MemoryUsage = calculateAggregatedValue(memoryUsage)
	agg.MemoryUsed = calculateAggregatedValue(memoryUsed)
	agg.DiskUsage = calculateAggregatedValue(diskUsage)
	agg.DiskUsed = calculateAggregatedValue(diskUsed)
	agg.Load1 = calculateAggregatedValue(load1)
	agg.Load5 = calculateAggregatedValue(load5)
	agg.Load15 = calculateAggregatedValue(load15)
	
	return agg
}

// calculateAggregatedValue 计算聚合值
func calculateAggregatedValue(values []float64) AggregatedValue {
	if len(values) == 0 {
		return AggregatedValue{}
	}
	
	agg := AggregatedValue{
		Count: len(values),
		Min:   values[0],
		Max:   values[0],
		Sum:   0,
		Last:  values[len(values)-1],
	}
	
	for _, v := range values {
		agg.Sum += v
		if v < agg.Min {
			agg.Min = v
		}
		if v > agg.Max {
			agg.Max = v
		}
	}
	
	agg.Avg = agg.Sum / float64(agg.Count)
	
	// 计算百分位数
	sorted := make([]float64, len(values))
	copy(sorted, values)
	sort.Float64s(sorted)
	
	agg.P90 = calculatePercentile(sorted, 0.90)
	agg.P95 = calculatePercentile(sorted, 0.95)
	agg.P99 = calculatePercentile(sorted, 0.99)
	
	return agg
}

// calculatePercentile 计算百分位数
func calculatePercentile(sorted []float64, p float64) float64 {
	if len(sorted) == 0 {
		return 0
	}
	
	index := p * float64(len(sorted)-1)
	lower := int(math.Floor(index))
	upper := int(math.Ceil(index))
	
	if lower == upper {
		return sorted[lower]
	}
	
	weight := index - float64(lower)
	return sorted[lower]*(1-weight) + sorted[upper]*weight
}

// QueryAggregated 查询聚合指标
func (a *MetricsAggregator) QueryAggregated(assetID string, granularity TimeGranularity, start, end time.Time) []*AggregatedMetrics {
	a.mu.RLock()
	defer a.mu.RUnlock()
	
	var result []*AggregatedMetrics
	
	if assetData, exists := a.aggregated[assetID]; exists {
		if metrics, exists := assetData[granularity]; exists {
			for _, m := range metrics {
				if m.WindowStart.After(start) && m.WindowEnd.Before(end) {
					result = append(result, m)
				}
			}
		}
	}
	
	return result
}

// QueryRaw 查询原始指标并实时聚合
func (a *MetricsAggregator) QueryRaw(assetID string, start, end time.Time, granularity TimeGranularity) *AggregatedMetrics {
	if a.collector == nil {
		return nil
	}
	
	// 获取原始指标历史
	history := a.collector.GetMetricsHistory(assetID, 0)
	
	// 过滤时间范围
	var filtered []*AssetMetrics
	for _, m := range history {
		if m.Timestamp.After(start) && m.Timestamp.Before(end) {
			filtered = append(filtered, m)
		}
	}
	
	if len(filtered) == 0 {
		return nil
	}
	
	return a.aggregateMetrics(assetID, filtered, granularity)
}

// GetAggregatedMetrics 获取预聚合指标
func (a *MetricsAggregator) GetAggregatedMetrics(assetID string, granularity TimeGranularity) []*AggregatedMetrics {
	a.mu.RLock()
	defer a.mu.RUnlock()
	
	if assetData, exists := a.aggregated[assetID]; exists {
		return assetData[granularity]
	}
	
	return nil
}

// GetLatestAggregated 获取最新聚合指标
func (a *MetricsAggregator) GetLatestAggregated(assetID string, granularity TimeGranularity) *AggregatedMetrics {
	a.mu.RLock()
	defer a.mu.RUnlock()
	
	if assetData, exists := a.aggregated[assetID]; exists {
		if metrics, exists := assetData[granularity]; exists && len(metrics) > 0 {
			return metrics[len(metrics)-1]
		}
	}
	
	return nil
}

// GetAllAggregated 获取所有资产的聚合指标
func (a *MetricsAggregator) GetAllAggregated(granularity TimeGranularity) map[string]*AggregatedMetrics {
	a.mu.RLock()
	defer a.mu.RUnlock()
	
	result := make(map[string]*AggregatedMetrics)
	
	for assetID, assetData := range a.aggregated {
		if metrics, exists := assetData[granularity]; exists && len(metrics) > 0 {
			result[assetID] = metrics[len(metrics)-1]
		}
	}
	
	return result
}

// Cleanup 清理过期数据
func (a *MetricsAggregator) Cleanup() {
	a.mu.Lock()
	defer a.mu.Unlock()
	
	cutoff := time.Now().Add(-a.config.RetentionPeriod)
	
	for assetID, assetData := range a.aggregated {
		for granularity, metrics := range assetData {
			var filtered []*AggregatedMetrics
			for _, m := range metrics {
				if m.Timestamp.After(cutoff) {
					filtered = append(filtered, m)
				}
			}
			a.aggregated[assetID][granularity] = filtered
		}
	}
}

// GetStats 获取聚合器统计
func (a *MetricsAggregator) GetStats() *AggregatorStats {
	a.mu.RLock()
	defer a.mu.RUnlock()
	
	stats := &AggregatorStats{
		AssetCount: len(a.aggregated),
	}
	
	for _, assetData := range a.aggregated {
		for _, metrics := range assetData {
			stats.TotalDataPoints += len(metrics)
		}
	}
	
	return stats
}

// AggregatorStats 聚合器统计
type AggregatorStats struct {
	AssetCount      int `json:"asset_count"`
	TotalDataPoints int `json:"total_data_points"`
}

// GranularityToDuration 将粒度转换为持续时间
func GranularityToDuration(g TimeGranularity) time.Duration {
	switch g {
	case Granularity1s:
		return time.Second
	case Granularity10s:
		return 10 * time.Second
	case Granularity1m:
		return time.Minute
	case Granularity5m:
		return 5 * time.Minute
	case Granularity1h:
		return time.Hour
	case Granularity1d:
		return 24 * time.Hour
	default:
		return time.Minute
	}
}

// DurationToGranularity 将持续时间转换为粒度
func DurationToGranularity(d time.Duration) TimeGranularity {
	switch {
	case d <= time.Second:
		return Granularity1s
	case d <= 10*time.Second:
		return Granularity10s
	case d <= time.Minute:
		return Granularity1m
	case d <= 5*time.Minute:
		return Granularity5m
	case d <= time.Hour:
		return Granularity1h
	default:
		return Granularity1d
	}
}
