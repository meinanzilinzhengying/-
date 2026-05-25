//go:build linux

// Package cluster 提供双中心全局查询与负载均衡能力
// 支持从任一中心入口查看全局数据
package cluster

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"sort"
	"sync"
	"sync/atomic"
	"time"
)

// ============================================================
// 全局查询代理
// ============================================================

// GlobalQueryConfig 全局查询配置
type GlobalQueryConfig struct {
	Enabled          bool          `json:"enabled"`
	Timeout          time.Duration `json:"timeout"`
	MaxConcurrent    int           `json:"max_concurrent"`
	FallbackOnSingle bool          `json:"fallback_on_single"`
	CacheTTL         time.Duration `json:"cache_ttl"`
}

// DefaultGlobalQueryConfig 默认全局查询配置
func DefaultGlobalQueryConfig() *GlobalQueryConfig {
	return &GlobalQueryConfig{
		Enabled:          true,
		Timeout:          10 * time.Second,
		MaxConcurrent:    5,
		FallbackOnSingle: true,
		CacheTTL:         30 * time.Second,
	}
}

// GlobalQueryProxy 全局查询代理
// 从任一中心入口查询全局数据，自动聚合多中心结果
type GlobalQueryProxy struct {
	config *GlobalQueryConfig
	sync   *DualCenterSync

	// 查询缓存
	cache     map[string]*cacheEntry
	cacheMu   sync.RWMutex

	// 统计
	stats *GlobalQueryStats

	// 负载均衡
	lb *DualCenterLoadBalancer

	mu sync.RWMutex
}

// cacheEntry 缓存条目
type cacheEntry struct {
	data      interface{}
	expiresAt time.Time
}

// GlobalQueryStats 全局查询统计
type GlobalQueryStats struct {
	TotalQueries   atomic.Int64 `json:"total_queries"`
	CacheHits      atomic.Int64 `json:"cache_hits"`
	CacheMisses    atomic.Int64 `json:"cache_misses"`
	MultiCenter    atomic.Int64 `json:"multi_center_queries"`
	SingleCenter   atomic.Int64 `json:"single_center_queries"`
	FailedQueries  atomic.Int64 `json:"failed_queries"`
	AvgLatencyMs   atomic.Int64 `json:"avg_latency_ms"`
}

// QueryResult 全局查询结果
type QueryResult struct {
	QueryID    string      `json:"query_id"`
	DataType   string      `json:"data_type"`
	TotalCount int         `json:"total_count"`
	FromCache  bool        `json:"from_cache"`
	Centers    []CenterData `json:"centers"`
	LatencyMs  int64       `json:"latency_ms"`
	Timestamp  time.Time   `json:"timestamp"`
}

// CenterData 单中心数据
type CenterData struct {
	CenterID  string          `json:"center_id"`
	Role      CenterRole      `json:"role"`
	Count     int             `json:"count"`
	LatencyMs int64           `json:"latency_ms"`
	Data      json.RawMessage `json:"data"`
	Error     string          `json:"error,omitempty"`
}

// NewGlobalQueryProxy 创建全局查询代理
func NewGlobalQueryProxy(cfg *GlobalQueryConfig, sync *DualCenterSync) *GlobalQueryProxy {
	if cfg == nil {
		cfg = DefaultGlobalQueryConfig()
	}

	gqp := &GlobalQueryProxy{
		config: cfg,
		sync:   sync,
		cache:  make(map[string]*cacheEntry),
		stats:  &GlobalQueryStats{},
	}

	// 创建负载均衡器
	gqp.lb = NewDualCenterLoadBalancer(sync)

	// 启动缓存清理
	go gqp.cacheCleanup()

	return gqp
}

// QueryGlobal 全局查询
func (gqp *GlobalQueryProxy) QueryGlobal(ctx context.Context, dataType string, query string) (*QueryResult, error) {
	gqp.stats.TotalQueries.Add(1)

	// 检查缓存
	cacheKey := fmt.Sprintf("%s:%s", dataType, query)
	if entry := gqp.getCache(cacheKey); entry != nil {
		gqp.stats.CacheHits.Add(1)
		return entry.(*QueryResult), nil
	}
	gqp.stats.CacheMisses.Add(1)

	start := time.Now()

	// 获取所有可用中心
	centers := gqp.getAvailableCenters()
	if len(centers) == 0 {
		gqp.stats.FailedQueries.Add(1)
		return nil, fmt.Errorf("no available centers")
	}

	// 单中心降级
	if len(centers) == 1 && gqp.config.FallbackOnSingle {
		gqp.stats.SingleCenter.Add(1)
		result, err := gqp.querySingleCenter(ctx, centers[0], dataType, query)
		if err != nil {
			gqp.stats.FailedQueries.Add(1)
			return nil, err
		}
		gqp.setCache(cacheKey, result)
		return result, nil
	}

	// 多中心并发查询
	gqp.stats.MultiCenter.Add(1)
	result, err := gqp.queryMultiCenters(ctx, centers, dataType, query)
	if err != nil {
		gqp.stats.FailedQueries.Add(1)
		return nil, err
	}

	result.LatencyMs = time.Since(start).Milliseconds()
	gqp.updateAvgLatency(result.LatencyMs)

	gqp.setCache(cacheKey, result)
	return result, nil
}

// queryMultiCenters 多中心并发查询
func (gqp *GlobalQueryProxy) queryMultiCenters(ctx context.Context, centers []*CenterInfo, dataType string, query string) (*QueryResult, error) {
	result := &QueryResult{
		QueryID:   generateBatchID(),
		DataType:  dataType,
		Centers:   make([]CenterData, 0, len(centers)),
		Timestamp: time.Now(),
	}

	// 并发查询所有中心
	var wg sync.WaitGroup
	var mu sync.Mutex

	sem := make(chan struct{}, gqp.config.MaxConcurrent)

	for _, center := range centers {
		wg.Add(1)
		go func(c *CenterInfo) {
			defer wg.Done()
			sem <- struct{}{}
			defer func() { <-sem }()

			data, latency, err := gqp.queryCenter(ctx, c, dataType, query)

			mu.Lock()
			cd := CenterData{
				CenterID:  c.ID,
				Role:      c.Role,
				LatencyMs: latency,
			}

			if err != nil {
				cd.Error = err.Error()
			} else {
				cd.Data = data
				cd.Count = countDataItems(data)
				result.TotalCount += cd.Count
			}

			result.Centers = append(result.Centers, cd)
			mu.Unlock()
		}(center)
	}

	wg.Wait()

	// 按延迟排序
	sort.Slice(result.Centers, func(i, j int) bool {
		return result.Centers[i].LatencyMs < result.Centers[j].LatencyMs
	})

	return result, nil
}

// querySingleCenter 单中心查询
func (gqp *GlobalQueryProxy) querySingleCenter(ctx context.Context, center *CenterInfo, dataType string, query string) (*QueryResult, error) {
	data, latency, err := gqp.queryCenter(ctx, center, dataType, query)
	if err != nil {
		return nil, err
	}

	return &QueryResult{
		QueryID:   generateBatchID(),
		DataType:  dataType,
		TotalCount: countDataItems(data),
		Centers: []CenterData{{
			CenterID:  center.ID,
			Role:      center.Role,
			Count:     countDataItems(data),
			LatencyMs: latency,
			Data:      data,
		}},
		Timestamp: time.Now(),
	}, nil
}

// queryCenter 查询单个中心
func (gqp *GlobalQueryProxy) queryCenter(ctx context.Context, center *CenterInfo, dataType string, query string) (json.RawMessage, int64, error) {
	url := fmt.Sprintf("http://%s:%d/api/v1/query/%s?%s", center.Address, center.Port, dataType, query)

	queryCtx, cancel := context.WithTimeout(ctx, gqp.config.Timeout)
	defer cancel()

	req, err := http.NewRequestWithContext(queryCtx, "GET", url, nil)
	if err != nil {
		return nil, 0, err
	}
	req.Header.Set("X-Source-Center", gqp.sync.GetLocalCenter().ID)
	req.Header.Set("X-Query-Mode", "global")

	start := time.Now()
	resp, err := http.DefaultClient.Do(req)
	latency := time.Since(start).Milliseconds()

	if err != nil {
		return nil, latency, err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, latency, err
	}

	if resp.StatusCode != http.StatusOK {
		return nil, latency, fmt.Errorf("HTTP %d: %s", resp.StatusCode, string(body))
	}

	return json.RawMessage(body), latency, nil
}

// getAvailableCenters 获取可用中心列表
func (gqp *GlobalQueryProxy) getAvailableCenters() []*CenterInfo {
	centers := []*CenterInfo{gqp.sync.GetLocalCenter()}

	peers := gqp.sync.GetPeerCenters()
	for _, peer := range peers {
		if peer.IsActive {
			centers = append(centers, peer)
		}
	}

	return centers
}

// 缓存操作
func (gqp *GlobalQueryProxy) getCache(key string) interface{} {
	gqp.cacheMu.RLock()
	defer gqp.cacheMu.RUnlock()

	entry, ok := gqp.cache[key]
	if !ok || time.Now().After(entry.expiresAt) {
		return nil
	}
	return entry.data
}

func (gqp *GlobalQueryProxy) setCache(key string, data interface{}) {
	gqp.cacheMu.Lock()
	defer gqp.cacheMu.Unlock()

	gqp.cache[key] = &cacheEntry{
		data:      data,
		expiresAt: time.Now().Add(gqp.config.CacheTTL),
	}
}

func (gqp *GlobalQueryProxy) cacheCleanup() {
	ticker := time.NewTicker(1 * time.Minute)
	defer ticker.Stop()

	for range ticker.C {
		gqp.cacheMu.Lock()
		now := time.Now()
		for key, entry := range gqp.cache {
			if now.After(entry.expiresAt) {
				delete(gqp.cache, key)
			}
		}
		gqp.cacheMu.Unlock()
	}
}

func (gqp *GlobalQueryProxy) updateAvgLatency(latencyMs int64) {
	for {
		old := gqp.stats.AvgLatencyMs.Load()
		newAvg := (old*9 + latencyMs) / 10 // 指数移动平均
		if gqp.stats.AvgLatencyMs.CompareAndSwap(old, newAvg) {
			break
		}
	}
}

func countDataItems(data json.RawMessage) int {
	var items []json.RawMessage
	if json.Unmarshal(data, &items) == nil {
		return len(items)
	}
	if data != nil {
		return 1
	}
	return 0
}

// GetStats 获取全局查询统计
func (gqp *GlobalQueryProxy) GetStats() *GlobalQueryStats {
	return gqp.stats
}

// ============================================================
// 双中心负载均衡
// ============================================================

// DualCenterLoadBalancer 双中心负载均衡器
type DualCenterLoadBalancer struct {
	sync *DualCenterSync

	// 加权轮询
	counter atomic.Uint64

	// 权重（基于健康状态和延迟动态调整）
	weights map[string]int64
	mu      sync.RWMutex
}

// NewDualCenterLoadBalancer 创建双中心负载均衡器
func NewDualCenterLoadBalancer(sync *DualCenterSync) *DualCenterLoadBalancer {
	lb := &DualCenterLoadBalancer{
		sync:   sync,
		weights: make(map[string]int64),
	}

	// 启动权重更新
	go lb.updateWeights()

	return lb
}

// SelectCenter 选择中心
func (lb *DualCenterLoadBalancer) SelectCenter() *CenterInfo {
	centers := lb.getAvailableCenters()
	if len(centers) == 0 {
		return nil
	}
	if len(centers) == 1 {
		return centers[0]
	}

	// 加权轮询
	lb.mu.RLock()
	totalWeight := int64(0)
	for _, c := range centers {
		totalWeight += lb.weights[c.ID]
	}
	lb.mu.RUnlock()

	if totalWeight == 0 {
		// 均匀分布
		idx := lb.counter.Add(1) % uint64(len(centers))
		return centers[idx]
	}

	// 加权选择
	r := lb.counter.Add(1) % uint64(totalWeight)
	cumWeight := int64(0)
	for _, c := range centers {
		lb.mu.RLock()
		w := lb.weights[c.ID]
		lb.mu.RUnlock()

		cumWeight += w
		if r < uint64(cumWeight) {
			return c
		}
	}

	return centers[0]
}

// SelectCenterForAgent 为 Agent 选择最优中心
func (lb *DualCenterLoadBalancer) SelectCenterForAgent(agentID string) *CenterInfo {
	// 优先选择 Agent 已连接的中心
	centers := lb.getAvailableCenters()
	if len(centers) == 0 {
		return nil
	}

	// 基于连接亲和性选择
	local := lb.sync.GetLocalCenter()
	for _, c := range centers {
		if c.ID == local.ID {
			return c // 优先本地
		}
	}

	// 选择延迟最低的中心
	var best *CenterInfo
	var minLag int64 = 1<<63 - 1
	for _, c := range centers {
		if c.LagMs < minLag {
			minLag = c.LagMs
			best = c
		}
	}

	return best
}

func (lb *DualCenterLoadBalancer) getAvailableCenters() []*CenterInfo {
	centers := []*CenterInfo{lb.sync.GetLocalCenter()}
	peers := lb.sync.GetPeerCenters()
	for _, peer := range peers {
		if peer.IsActive {
			centers = append(centers, peer)
		}
	}
	return centers
}

// updateWeights 动态更新权重
func (lb *DualCenterLoadBalancer) updateWeights() {
	ticker := time.NewTicker(10 * time.Second)
	defer ticker.Stop()

	for range ticker.C {
		lb.mu.Lock()
		peers := lb.sync.GetPeerCenters()

		// 本地中心权重
		lb.weights[lb.sync.GetLocalCenter().ID] = 100

		for _, peer := range peers {
			if peer.IsActive {
				// 基于延迟计算权重：延迟越低权重越高
				lag := peer.LagMs
				if lag <= 10 {
					lb.weights[peer.ID] = 100
				} else if lag <= 50 {
					lb.weights[peer.ID] = 80
				} else if lag <= 100 {
					lb.weights[peer.ID] = 60
				} else if lag <= 500 {
					lb.weights[peer.ID] = 30
				} else {
					lb.weights[peer.ID] = 10
				}
			} else {
				lb.weights[peer.ID] = 0
			}
		}
		lb.mu.Unlock()
	}
}

// GetWeights 获取当前权重
func (lb *DualCenterLoadBalancer) GetWeights() map[string]int64 {
	lb.mu.RLock()
	defer lb.mu.RUnlock()

	result := make(map[string]int64, len(lb.weights))
	for k, v := range lb.weights {
		result[k] = v
	}
	return result
}

// ============================================================
// 双中心状态总览
// ============================================================

// DualCenterOverview 双中心状态总览
type DualCenterOverview struct {
	LocalCenter  *CenterInfo              `json:"local_center"`
	PeerCenters  map[string]*CenterInfo  `json:"peer_centers"`
	SyncStats    *SyncStats              `json:"sync_stats"`
	QueryStats   *GlobalQueryStats       `json:"query_stats"`
	Failover     *FailoverStatus         `json:"failover"`
	LoadBalancer map[string]int64        `json:"load_balancer_weights"`
	Timestamp    time.Time               `json:"timestamp"`
}

// GetOverview 获取双中心总览
func (gqp *GlobalQueryProxy) GetOverview(failoverMgr *FailoverManager) *DualCenterOverview {
	overview := &DualCenterOverview{
		LocalCenter: gqp.sync.GetLocalCenter(),
		PeerCenters: gqp.sync.GetPeerCenters(),
		SyncStats:   gqp.sync.GetStats(),
		QueryStats:  gqp.stats,
		Timestamp:   time.Now(),
	}

	if failoverMgr != nil {
		overview.Failover = failoverMgr.GetStatus()
	}

	if gqp.lb != nil {
		overview.LoadBalancer = gqp.lb.GetWeights()
	}

	return overview
}
