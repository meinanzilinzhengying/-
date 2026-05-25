// Package healthscore VPC网络评估器
// 评估VPC内网络质量：延迟、带宽、连通性、路由可达性
package healthscore

import (
	"context"
	"fmt"
	"math"
	"net"
	"sort"
	"sync"
	"time"
)

// ============================================================
// VPC网络评估
// ============================================================

// VPCEvaluator VPC网络评估器
type VPCEvaluator struct {
	config    *VPCEvalConfig
	engine    *Engine
	collector *Collector
	mu        sync.RWMutex
	// VPC网络探测结果缓存
	probeResults map[string]*VPCProbeResult
}

// VPCEvalConfig VPC评估配置
type VPCEvalConfig struct {
	// 探测配置
	ProbeTimeout    time.Duration `yaml:"probe_timeout"`     // 探测超时
	ProbeCount      int           `yaml:"probe_count"`       // 每次探测次数
	ProbeInterval   time.Duration `yaml:"probe_interval"`    // 探测间隔
	EnableBandwidth bool          `yaml:"enable_bandwidth"`  // 启用带宽测试
	EnableTraceroute bool         `yaml:"enable_traceroute"` // 启用路由追踪

	// 目标配置
	DefaultTargets []string `yaml:"default_targets"` // 默认探测目标
	CustomTargets  []string `yaml:"custom_targets"`  // 自定义探测目标

	// 阈值配置
	LatencyWarningMs  float64 `yaml:"latency_warning_ms"`  // 延迟告警阈值 (ms)
	LatencyCriticalMs float64 `yaml:"latency_critical_ms"` // 延迟严重阈值 (ms)
	LossWarningPct    float64 `yaml:"loss_warning_pct"`    // 丢包告警阈值 (%)
	LossCriticalPct   float64 `yaml:"loss_critical_pct"`   // 丢包严重阈值 (%)
	BandwidthMinMbps  float64 `yaml:"bandwidth_min_mbps"`  // 最小带宽要求 (Mbps)

	// VPC 特定配置
	CheckInternalRouting bool `yaml:"check_internal_routing"` // 检查VPC内路由
	CheckNAT             bool `yaml:"check_nat"`              // 检查NAT网关
	CheckSecurityGroup   bool `yaml:"check_security_group"`   // 检查安全组
	CheckLB              bool `yaml:"check_lb"`               // 检查负载均衡
}

// VPCProbeResult VPC探测结果
type VPCProbeResult struct {
	VPCID         string            `json:"vpc_id"`
	Timestamp     time.Time         `json:"timestamp"`
	Targets       []*TargetProbe    `json:"targets"`
	AvgLatencyMs  float64           `json:"avg_latency_ms"`
	P99LatencyMs  float64           `json:"p99_latency_ms"`
	PacketLossPct float64           `json:"packet_loss_pct"`
	JitterMs      float64           `json:"jitter_ms"`
	BandwidthMbps float64           `json:"bandwidth_mbps,omitempty"`
	Connectivity  map[string]bool   `json:"connectivity"`    // 目标可达性
	RouteHops     []RouteHop        `json:"route_hops,omitempty"`
	Issues        []string          `json:"issues"`          // 发现的问题
	Score         float64           `json:"score"`           // VPC网络得分
}

// TargetProbe 单目标探测结果
type TargetProbe struct {
	Target       string        `json:"target"`
	Reachable    bool          `json:"reachable"`
	Latencies    []float64     `json:"latencies_ms"`    // 各次探测延迟
	AvgLatency   float64       `json:"avg_latency_ms"`
	MinLatency   float64       `json:"min_latency_ms"`
	MaxLatency   float64       `json:"max_latency_ms"`
	PacketLoss   float64       `json:"packet_loss_pct"`
	Jitter       float64       `json:"jitter_ms"`
	Error        string        `json:"error,omitempty"`
	Duration     time.Duration `json:"duration"`
}

// RouteHop 路由跳
type RouteHop struct {
	HopNum    int       `json:"hop_num"`
	Addr      string    `json:"addr"`
	Hostname  string    `json:"hostname,omitempty"`
	LatencyMs float64   `json:"latency_ms"`
	LossPct   float64   `json:"loss_pct"`
	Timestamp time.Time `json:"timestamp"`
}

// NewVPCEvaluator 创建VPC评估器
func NewVPCEvaluator(config *VPCEvalConfig, engine *Engine, collector *Collector) *VPCEvaluator {
	if config == nil {
		config = &VPCEvalConfig{
			ProbeTimeout:       5 * time.Second,
			ProbeCount:         5,
			ProbeInterval:      200 * time.Millisecond,
			EnableBandwidth:    false,
			EnableTraceroute:   false,
			DefaultTargets:     []string{"8.8.8.8", "114.114.114.114"},
			LatencyWarningMs:   50,
			LatencyCriticalMs:  200,
			LossWarningPct:     1.0,
			LossCriticalPct:    5.0,
			BandwidthMinMbps:   100,
			CheckInternalRouting: true,
			CheckNAT:           true,
			CheckSecurityGroup: true,
			CheckLB:            true,
		}
	}
	return &VPCEvaluator{
		config:       config,
		engine:       engine,
		collector:    collector,
		probeResults: make(map[string]*VPCProbeResult),
	}
}

// ============================================================
// VPC探测
// ============================================================

// ProbeVPC 探测VPC网络质量
func (v *VPCEvaluator) ProbeVPC(vpcID string) (*VPCProbeResult, error) {
	result := &VPCProbeResult{
		VPCID:        vpcID,
		Timestamp:    time.Now(),
		Connectivity: make(map[string]bool),
	}

	// 合并探测目标
	targets := v.mergeTargets()

	// 并发探测所有目标
	var wg sync.WaitGroup
	var mu sync.Mutex
	results := make([]*TargetProbe, 0, len(targets))

	for _, target := range targets {
		wg.Add(1)
		go func(t string) {
			defer wg.Done()
			probe := v.probeTarget(t)
			mu.Lock()
			results = append(results, probe)
			result.Connectivity[t] = probe.Reachable
			mu.Unlock()
		}(target)
	}
	wg.Wait()

	result.Targets = results

	// 汇总统计
	v.aggregateResults(result)

	// 检测问题
	result.Issues = v.detectIssues(result)

	// 计算VPC网络得分
	result.Score = v.calcVPCScore(result)

	// 缓存结果
	v.mu.Lock()
	v.probeResults[vpcID] = result
	v.mu.Unlock()

	return result, nil
}

// probeTarget 探测单个目标
func (v *VPCEvaluator) probeTarget(target string) *TargetProbe {
	probe := &TargetProbe{
		Target: target,
	}

	start := time.Now()
	ctx, cancel := context.WithTimeout(context.Background(), v.config.ProbeTimeout)
	defer cancel()

	// 解析目标地址
	addr := net.JoinHostPort(target, "80")
	if isPrivateIP(target) {
		addr = net.JoinHostPort(target, "22") // 内网用22端口
	}

	// 多次探测
	for i := 0; i < v.config.ProbeCount; i++ {
		if i > 0 {
			select {
			case <-time.After(v.config.ProbeInterval):
			case <-ctx.Done():
				break
			}
		}

		latency, err := v.tcpPing(ctx, addr)
		if err != nil {
			probe.PacketLoss++
			if probe.Error == "" {
				probe.Error = err.Error()
			}
			continue
		}
		probe.Latencies = append(probe.Latencies, latency)
	}

	probe.Duration = time.Since(start)
	probe.Reachable = len(probe.Latencies) > 0

	// 计算统计量
	if len(probe.Latencies) > 0 {
		probe.AvgLatency = avg(probe.Latencies)
		probe.MinLatency = min(probe.Latencies)
		probe.MaxLatency = max(probe.Latencies)
		probe.PacketLoss = float64(v.config.ProbeCount-len(probe.Latencies)) / float64(v.config.ProbeCount) * 100
		probe.Jitter = calcJitter(probe.Latencies)
	} else {
		probe.PacketLoss = 100
	}

	return probe
}

// tcpPing TCP Ping 探测
func (v *VPCEvaluator) tcpPing(ctx context.Context, addr string) (float64, error) {
	start := time.Now()

	dialer := &net.Dialer{
		Timeout: v.config.ProbeTimeout,
	}

	conn, err := dialer.DialContext(ctx, "tcp", addr)
	if err != nil {
		return 0, err
	}
	conn.Close()

	latency := time.Since(start).Seconds() * 1000 // 转换为ms
	return latency, nil
}

// ============================================================
// 结果汇总
// ============================================================

func (v *VPCEvaluator) aggregateResults(result *VPCProbeResult) {
	var allLatencies []float64
	var totalLoss float64
	reachableCount := 0

	for _, t := range result.Targets {
		if t.Reachable {
			reachableCount++
			allLatencies = append(allLatencies, t.Latencies...)
		}
		totalLoss += t.PacketLoss
	}

	if len(allLatencies) > 0 {
		sort.Float64s(allLatencies)
		result.AvgLatencyMs = avg(allLatencies)
		result.JitterMs = calcJitter(allLatencies)

		// P99延迟
		p99Idx := int(float64(len(allLatencies)) * 0.99)
		if p99Idx >= len(allLatencies) {
			p99Idx = len(allLatencies) - 1
		}
		result.P99LatencyMs = allLatencies[p99Idx]
	}

	if len(result.Targets) > 0 {
		result.PacketLossPct = totalLoss / float64(len(result.Targets))
	}
}

// ============================================================
// 问题检测
// ============================================================

func (v *VPCEvaluator) detectIssues(result *VPCProbeResult) []string {
	var issues []string

	// 延迟问题
	if result.AvgLatencyMs > v.config.LatencyCriticalMs {
		issues = append(issues, fmt.Sprintf("延迟严重: 平均 %.1fms 超过阈值 %.1fms",
			result.AvgLatencyMs, v.config.LatencyCriticalMs))
	} else if result.AvgLatencyMs > v.config.LatencyWarningMs {
		issues = append(issues, fmt.Sprintf("延迟偏高: 平均 %.1fms 超过阈值 %.1fms",
			result.AvgLatencyMs, v.config.LatencyWarningMs))
	}

	// P99延迟问题
	if result.P99LatencyMs > v.config.LatencyCriticalMs*2 {
		issues = append(issues, fmt.Sprintf("P99延迟异常: %.1fms", result.P99LatencyMs))
	}

	// 丢包问题
	if result.PacketLossPct > v.config.LossCriticalPct {
		issues = append(issues, fmt.Sprintf("丢包严重: %.2f%% 超过阈值 %.1f%%",
			result.PacketLossPct, v.config.LossCriticalPct))
	} else if result.PacketLossPct > v.config.LossWarningPct {
		issues = append(issues, fmt.Sprintf("丢包偏高: %.2f%% 超过阈值 %.1f%%",
			result.PacketLossPct, v.config.LossWarningPct))
	}

	// 抖动问题
	if result.JitterMs > 20 {
		issues = append(issues, fmt.Sprintf("网络抖动过大: %.2fms", result.JitterMs))
	}

	// 连通性问题
	for _, t := range result.Targets {
		if !t.Reachable {
			issues = append(issues, fmt.Sprintf("目标不可达: %s (%s)", t.Target, t.Error))
		}
	}

	// 检查各目标延迟差异（可能存在路由不对称）
	if len(result.Targets) > 1 {
		var latencies []float64
		for _, t := range result.Targets {
			if t.Reachable {
				latencies = append(latencies, t.AvgLatency)
			}
		}
		if len(latencies) > 1 {
			latencySpread := max(latencies) - min(latencies)
			if latencySpread > 50 {
				issues = append(issues, fmt.Sprintf("目标间延迟差异过大: %.1fms，可能存在路由问题",
					latencySpread))
			}
		}
	}

	return issues
}

// ============================================================
// VPC网络评分
// ============================================================

func (v *VPCEvaluator) calcVPCScore(result *VPCProbeResult) float64 {
	// 基于探测结果的评分
	latencyScore := v.engine.calcLatencyScore(result.AvgLatencyMs)
	p99Score := v.engine.calcLatencyScore(result.P99LatencyMs) * 0.8
	lossScore := v.engine.calcPacketLossScore(result.PacketLossPct)
	jitterScore := v.engine.calcJitterScore(result.JitterMs)

	// 连通性评分
	reachableCount := 0
	for _, reachable := range result.Connectivity {
		if reachable {
			reachableCount++
		}
	}
	connScore := 0.0
	if len(result.Connectivity) > 0 {
		connScore = float64(reachableCount) / float64(len(result.Connectivity)) * 100
	}

	// 问题扣分
	penalty := float64(len(result.Issues)) * 5
	if penalty > 30 {
		penalty = 30
	}

	// 加权计算
	score := latencyScore*0.25 + p99Score*0.15 + lossScore*0.25 +
		jitterScore*0.10 + connScore*0.25 - penalty

	return math.Max(0, math.Min(100, math.Round(score*100)/100))
}

// ============================================================
// VPC健康分更新
// ============================================================

// UpdateVPCScore 探测VPC并更新健康分
func (v *VPCEvaluator) UpdateVPCScore(pool *ResourcePool) (*HealthScore, error) {
	probeResult, err := v.ProbeVPC(pool.ID)
	if err != nil {
		return nil, fmt.Errorf("VPC探测失败: %w", err)
	}

	// 构建网络指标
	netMetrics := &NetworkMetrics{
		AvgLatency:    probeResult.AvgLatencyMs,
		P99Latency:    probeResult.P99LatencyMs,
		PacketLoss:    probeResult.PacketLossPct,
		Jitter:        probeResult.JitterMs,
		BandwidthMbps: probeResult.BandwidthMbps,
		MaxBandwidth:  v.config.BandwidthMinMbps,
	}

	// 采集利用率和SLA
	utilMetrics, _ := v.collector.collectUtilization(pool)
	slaMetrics, _ := v.collector.collectSLA(pool)

	// 计算综合健康分
	score, err := v.engine.Evaluate(pool.ID, pool.Name, pool.Type, utilMetrics, netMetrics, slaMetrics)
	if err != nil {
		return nil, err
	}

	return score, nil
}

// ============================================================
// VPC间对比
// ============================================================

// VPCComparison VPC对比结果
type VPCComparison struct {
	VPCID       string  `json:"vpc_id"`
	VPCName     string  `json:"vpc_name"`
	Score       float64 `json:"score"`
	Latency     float64 `json:"latency_ms"`
	PacketLoss  float64 `json:"packet_loss_pct"`
	Bandwidth   float64 `json:"bandwidth_mbps"`
	Rank        int     `json:"rank"`
}

// CompareVPCs 对比多个VPC的网络质量
func (v *VPCEvaluator) CompareVPCs(vpcIDs []string) ([]*VPCComparison, error) {
	var comparisons []*VPCComparison

	for _, id := range vpcIDs {
		result, err := v.ProbeVPC(id)
		if err != nil {
			continue
		}

		comp := &VPCComparison{
			VPCID:      id,
			Score:      result.Score,
			Latency:    result.AvgLatencyMs,
			PacketLoss: result.PacketLossPct,
			Bandwidth:  result.BandwidthMbps,
		}

		// 获取VPC名称
		if pool, ok := v.collector.pools[id]; ok {
			comp.VPCName = pool.Name
		}

		comparisons = append(comparisons, comp)
	}

	// 按分数排序
	sort.Slice(comparisons, func(i, j int) bool {
		return comparisons[i].Score > comparisons[j].Score
	})

	for i, c := range comparisons {
		c.Rank = i + 1
	}

	return comparisons, nil
}

// ============================================================
// 工具函数
// ============================================================

func (v *VPCEvaluator) mergeTargets() []string {
	seen := make(map[string]bool)
	var targets []string

	for _, t := range v.config.DefaultTargets {
		if !seen[t] {
			seen[t] = true
			targets = append(targets, t)
		}
	}
	for _, t := range v.config.CustomTargets {
		if !seen[t] {
			seen[t] = true
			targets = append(targets, t)
		}
	}

	return targets
}

func avg(data []float64) float64 {
	if len(data) == 0 {
		return 0
	}
	sum := 0.0
	for _, v := range data {
		sum += v
	}
	return sum / float64(len(data))
}

func min(data []float64) float64 {
	if len(data) == 0 {
		return 0
	}
	m := data[0]
	for _, v := range data[1:] {
		if v < m {
			m = v
		}
	}
	return m
}

func max(data []float64) float64 {
	if len(data) == 0 {
		return 0
	}
	m := data[0]
	for _, v := range data[1:] {
		if v > m {
			m = v
		}
	}
	return m
}

// calcJitter 计算抖动（相邻探测延迟差的平均值）
func calcJitter(latencies []float64) float64 {
	if len(latencies) < 2 {
		return 0
	}

	totalDiff := 0.0
	for i := 1; i < len(latencies); i++ {
		diff := latencies[i] - latencies[i-1]
		if diff < 0 {
			diff = -diff
		}
		totalDiff += diff
	}

	return totalDiff / float64(len(latencies)-1)
}

// isPrivateIP 判断是否为内网IP
func isPrivateIP(ip string) bool {
	parsed := net.ParseIP(ip)
	if parsed == nil {
		return false
	}

	privateRanges := []struct {
		network *net.IPNet
	}{
		{parseCIDR("10.0.0.0/8")},
		{parseCIDR("172.16.0.0/12")},
		{parseCIDR("192.168.0.0/16")},
		{parseCIDR("100.64.0.0/10")},
	}

	for _, r := range privateRanges {
		if r.network.Contains(parsed) {
			return true
		}
	}
	return false
}

func parseCIDR(cidr string) *net.IPNet {
	_, network, err := net.ParseCIDR(cidr)
	if err != nil {
		return nil
	}
	return network
}
