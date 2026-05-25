// Package healthscore 实现资源池/VPC健康评分引擎
// 基于利用率、网络质量、SLA达标率三维加权计算综合健康分
package healthscore

import (
	"fmt"
	"math"
	"sort"
	"sync"
	"time"
)

// ============================================================
// 健康评分模型
// ============================================================

// HealthGrade 健康等级
type HealthGrade string

const (
	GradeExcellent HealthGrade = "excellent" // 优秀 (90-100)
	GradeGood      HealthGrade = "good"      // 良好 (75-89)
	GradeFair      HealthGrade = "fair"      // 一般 (60-74)
	GradePoor      HealthGrade = "poor"      // 较差 (40-59)
	GradeCritical  HealthGrade = "critical"  // 严重 (<40)
)

// GradeInfo 等级信息
type GradeInfo struct {
	Grade     HealthGrade `json:"grade"`
	Label     string      `json:"label"`
	Color     string      `json:"color"`     // 前端展示颜色
	MinScore  float64     `json:"min_score"`
	MaxScore  float64     `json:"max_score"`
	Suggestion string     `json:"suggestion"`
}

// 预定义等级
var GradeTable = []GradeInfo{
	{GradeExcellent, "优秀", "#52c41a", 90, 100, "资源池运行状态良好，无需干预"},
	{GradeGood, "良好", "#73d13d", 75, 89.99, "资源池状态正常，建议持续关注"},
	{GradeFair, "一般", "#faad14", 60, 74.99, "存在部分风险指标，建议排查优化"},
	{GradePoor, "较差", "#ff7a45", 40, 59.99, "多项指标异常，建议立即处理"},
	{GradeCritical, "严重", "#ff4d4f", 0, 39.99, "资源池状态危急，需紧急处置"},
}

// HealthScore 健康评分
type HealthScore struct {
	ID             string      `json:"id"`               // 资源池/VPC ID
	Name           string      `json:"name"`             // 资源池/VPC名称
	Type           string      `json:"type"`             // pool / vpc
	Score          float64     `json:"score"`            // 综合健康分 (0-100)
	Grade          HealthGrade `json:"grade"`            // 健康等级
	Utilization    float64     `json:"utilization"`     // 利用率得分 (0-100)
	Network        float64     `json:"network"`         // 网络质量得分 (0-100)
	SLA            float64     `json:"sla"`             // SLA达标率得分 (0-100)
	WeightConfig   WeightConfig `json:"weight_config"`  // 加权配置
	DimensionScores map[string]float64 `json:"dimension_scores,omitempty"` // 细分维度得分
	Timestamp      time.Time   `json:"timestamp"`
}

// WeightConfig 加权配置
type WeightConfig struct {
	UtilizationWeight float64 `json:"utilization_weight" yaml:"utilization_weight"` // 利用率权重
	NetworkWeight     float64 `json:"network_weight" yaml:"network_weight"`         // 网络权重
	SLAWeight         float64 `json:"sla_weight" yaml:"sla_weight"`                 // SLA权重
}

// Validate 校验权重配置
func (w *WeightConfig) Validate() error {
	total := w.UtilizationWeight + w.NetworkWeight + w.SLAWeight
	if math.Abs(total-1.0) > 0.001 {
		return fmt.Errorf("权重之和必须为1.0，当前为 %.3f", total)
	}
	if w.UtilizationWeight < 0 || w.NetworkWeight < 0 || w.SLAWeight < 0 {
		return fmt.Errorf("权重不能为负数")
	}
	return nil
}

// ============================================================
// 维度指标
// ============================================================

// UtilizationMetrics 利用率指标
type UtilizationMetrics struct {
	CPUUsage     float64 `json:"cpu_usage"`      // CPU使用率 (%)
	MemoryUsage  float64 `json:"memory_usage"`   // 内存使用率 (%)
	DiskUsage    float64 `json:"disk_usage"`     // 磁盘使用率 (%)
	BandwidthUsage float64 `json:"bandwidth_usage"` // 带宽使用率 (%)
	GoroutineCount  int    `json:"goroutine_count"`  // 协程数
	ProcessCount    int    `json:"process_count"`    // 进程数
}

// NetworkMetrics 网络质量指标
type NetworkMetrics struct {
	AvgLatency    float64 `json:"avg_latency_ms"`    // 平均延迟 (ms)
	P99Latency    float64 `json:"p99_latency_ms"`    // P99延迟 (ms)
	PacketLoss    float64 `json:"packet_loss"`       // 丢包率 (%)
	Jitter        float64 `json:"jitter_ms"`         // 抖动 (ms)
	BandwidthMbps float64 `json:"bandwidth_mbps"`    // 可用带宽 (Mbps)
	MaxBandwidth  float64 `json:"max_bandwidth_mbps"` // 最大带宽 (Mbps)
	TCPRetrans    float64 `json:"tcp_retrans"`       // TCP重传率 (%)
	ConnCount     int     `json:"conn_count"`        // 连接数
	ConnErrors    int     `json:"conn_errors"`       // 连接错误数
}

// SLAMetrics SLA指标
type SLAMetrics struct {
	UptimePercent    float64 `json:"uptime_percent"`     // 可用率 (%)
	RequestSuccess   float64 `json:"request_success"`    // 请求成功率 (%)
	ErrorRate        float64 `json:"error_rate"`         // 错误率 (%)
	AvgResponseTime  float64 `json:"avg_response_ms"`    // 平均响应时间 (ms)
	P99ResponseTime  float64 `json:"p99_response_ms"`    // P99响应时间 (ms)
	RecoveryTime     float64 `json:"recovery_time_min"`  // 平均恢复时间 (分钟)
	SLOTarget        float64 `json:"slo_target"`         // SLO目标 (%)
	BudgetRemaining  float64 `json:"budget_remaining"`   // 错误预算剩余 (%)
}

// ============================================================
// 健康评分引擎
// ============================================================

// Engine 健康评分引擎
type Engine struct {
	config     *EngineConfig
	history    map[string][]*HealthScore // 历史评分
	mu         sync.RWMutex
	alertCB    func(score *HealthScore) // 评分变化回调
}

// EngineConfig 评分引擎配置
type EngineConfig struct {
	Weights           WeightConfig `yaml:"weights"`
	EvalInterval      int          `yaml:"eval_interval"`       // 评估间隔（秒）
	HistoryRetention  int          `yaml:"history_retention"`   // 历史保留数量
	AlertThreshold    float64      `yaml:"alert_threshold"`     // 告警阈值
	DegradeThreshold  float64      `yaml:"degrade_threshold"`   // 降级阈值
	EnableTrend       bool         `yaml:"enable_trend"`        // 启用趋势分析
	TrendWindow       int          `yaml:"trend_window"`        // 趋势窗口大小
}

// NewEngine 创建健康评分引擎
func NewEngine(config *EngineConfig) *Engine {
	if config == nil {
		config = &EngineConfig{
			Weights: WeightConfig{
				UtilizationWeight: 0.4,
				NetworkWeight:     0.3,
				SLAWeight:         0.3,
			},
			EvalInterval:     60,
			HistoryRetention: 1440,
			AlertThreshold:   60,
			DegradeThreshold: 40,
			EnableTrend:      true,
			TrendWindow:      30,
		}
	}
	return &Engine{
		config:  config,
		history: make(map[string][]*HealthScore),
	}
}

// OnAlert 设置评分告警回调
func (e *Engine) OnAlert(cb func(score *HealthScore)) {
	e.alertCB = cb
}

// Evaluate 评估资源池/VPC健康分
func (e *Engine) Evaluate(id, name, typ string,
	util *UtilizationMetrics, net *NetworkMetrics, sla *SLAMetrics) (*HealthScore, error) {

	// 计算各维度得分
	utilScore := e.calcUtilizationScore(util)
	netScore := e.calcNetworkScore(net)
	slaScore := e.calcSLAScore(sla)

	// 细分维度得分
	dimScores := map[string]float64{
		"cpu_usage":      e.invertScore(util.CPUUsage),
		"memory_usage":   e.invertScore(util.MemoryUsage),
		"disk_usage":     e.invertScore(util.DiskUsage),
		"bandwidth_usage": e.invertScore(util.BandwidthUsage),
		"latency":        e.calcLatencyScore(net.AvgLatency),
		"packet_loss":    e.calcPacketLossScore(net.PacketLoss),
		"jitter":         e.calcJitterScore(net.Jitter),
		"uptime":         sla.UptimePercent,
		"request_success": sla.RequestSuccess,
		"error_budget":   sla.BudgetRemaining,
	}

	// 加权计算综合分
	totalScore := utilScore*e.config.Weights.UtilizationWeight +
		netScore*e.config.Weights.NetworkWeight +
		slaScore*e.config.Weights.SLAWeight

	// 限制在 0-100
	totalScore = math.Max(0, math.Min(100, totalScore))

	score := &HealthScore{
		ID:             id,
		Name:           name,
		Type:           typ,
		Score:          math.Round(totalScore*100) / 100,
		Grade:          GetGrade(totalScore),
		Utilization:    math.Round(utilScore*100) / 100,
		Network:        math.Round(netScore*100) / 100,
		SLA:            math.Round(slaScore*100) / 100,
		WeightConfig:   e.config.Weights,
		DimensionScores: dimScores,
		Timestamp:      time.Now(),
	}

	// 记录历史
	e.recordHistory(score)

	// 检查告警
	e.checkAlert(score)

	return score, nil
}

// ============================================================
// 利用率评分
// ============================================================

func (e *Engine) calcUtilizationScore(m *UtilizationMetrics) float64 {
	if m == nil {
		return 50.0 // 无数据默认中等
	}

	// 各项利用率得分（利用率越高，得分越低）
	cpuScore := e.invertScore(m.CPUUsage)
	memScore := e.invertScore(m.MemoryUsage)
	diskScore := e.invertScore(m.DiskUsage)
	bwScore := e.invertScore(m.BandwidthUsage)

	// 加权平均：CPU和内存权重更高
	score := cpuScore*0.35 + memScore*0.35 + diskScore*0.20 + bwScore*0.10
	return score
}

// invertScore 将利用率反转为得分
// 利用率 0% → 得分 100，利用率 100% → 得分 0
// 使用非线性映射，低利用率区间衰减更慢
func (e *Engine) invertScore(usage float64) float64 {
	if usage < 0 {
		usage = 0
	}
	if usage > 100 {
		usage = 100
	}
	// 使用 sigmoid 反转，在 70% 附近开始快速下降
	// score = 100 * (1 - sigmoid((usage - 70) / 10))
	x := (usage - 70.0) / 10.0
	sig := 1.0 / (1.0 + math.Exp(-x))
	return 100.0 * (1.0 - sig)
}

// ============================================================
// 网络质量评分
// ============================================================

func (e *Engine) calcNetworkScore(m *NetworkMetrics) float64 {
	if m == nil {
		return 50.0
	}

	latencyScore := e.calcLatencyScore(m.AvgLatency)
	p99Score := e.calcLatencyScore(m.P99Latency) * 0.8 // P99权重稍低
	lossScore := e.calcPacketLossScore(m.PacketLoss)
	jitterScore := e.calcJitterScore(m.Jitter)
	retransScore := e.calcRetransScore(m.TCPRetrans)

	// 加权平均
	score := latencyScore*0.25 + p99Score*0.20 + lossScore*0.25 +
		jitterScore*0.15 + retransScore*0.15
	return score
}

func (e *Engine) calcLatencyScore(latencyMs float64) float64 {
	// 延迟评分：< 1ms=100, 1-5ms=90, 5-10ms=75, 10-50ms=50, 50-100ms=25, >100ms=0
	if latencyMs <= 1 {
		return 100
	} else if latencyMs <= 5 {
		return 100 - (latencyMs-1)*2.5
	} else if latencyMs <= 10 {
		return 90 - (latencyMs-5)*3
	} else if latencyMs <= 50 {
		return 75 - (latencyMs-10)*0.625
	} else if latencyMs <= 100 {
		return 50 - (latencyMs-50)*0.5
	}
	return math.Max(0, 25-(latencyMs-100)*0.25)
}

func (e *Engine) calcPacketLossScore(loss float64) float64 {
	// 丢包率评分：< 0.01%=100, 0.01-0.1%=90, 0.1-1%=60, 1-5%=20, >5%=0
	if loss <= 0.01 {
		return 100
	} else if loss <= 0.1 {
		return 100 - (loss-0.01)*111.11
	} else if loss <= 1 {
		return 90 - (loss-0.1)*33.33
	} else if loss <= 5 {
		return 60 - (loss-1)*10
	}
	return math.Max(0, 20-(loss-5)*4)
}

func (e *Engine) calcJitterScore(jitterMs float64) float64 {
	// 抖动评分：< 1ms=100, 1-5ms=80, 5-20ms=50, >20ms=10
	if jitterMs <= 1 {
		return 100
	} else if jitterMs <= 5 {
		return 100 - (jitterMs-1)*5
	} else if jitterMs <= 20 {
		return 80 - (jitterMs-5)*2
	}
	return math.Max(0, 50-(jitterMs-20)*2)
}

func (e *Engine) calcRetransScore(retrans float64) float64 {
	// TCP重传率评分：< 0.1%=100, 0.1-1%=70, 1-5%=30, >5%=0
	if retrans <= 0.1 {
		return 100
	} else if retrans <= 1 {
		return 100 - (retrans-0.1)*33.33
	} else if retrans <= 5 {
		return 70 - (retrans-1)*10
	}
	return math.Max(0, 30-(retrans-5)*6)
}

// ============================================================
// SLA评分
// ============================================================

func (e *Engine) calcSLAScore(m *SLAMetrics) float64 {
	if m == nil {
		return 50.0
	}

	uptimeScore := m.UptimePercent
	successScore := m.RequestSuccess
	errorBudgetScore := m.BudgetRemaining
	recoveryScore := e.calcRecoveryScore(m.RecoveryTime)

	// 加权平均
	score := uptimeScore*0.30 + successScore*0.30 +
		errorBudgetScore*0.25 + recoveryScore*0.15
	return score
}

func (e *Engine) calcRecoveryScore(recoveryMin float64) float64 {
	// 恢复时间评分：< 1min=100, 1-5min=90, 5-30min=60, >30min=20
	if recoveryMin <= 1 {
		return 100
	} else if recoveryMin <= 5 {
		return 100 - (recoveryMin-1)*2.5
	} else if recoveryMin <= 30 {
		return 90 - (recoveryMin-5)*1.2
	}
	return math.Max(0, 60-(recoveryMin-30)*1.33)
}

// ============================================================
// 等级判定
// ============================================================

// GetGrade 根据分数获取健康等级
func GetGrade(score float64) HealthGrade {
	for _, g := range GradeTable {
		if score >= g.MinScore && score <= g.MaxScore {
			return g.Grade
		}
	}
	return GradeCritical
}

// GetGradeInfo 获取等级详细信息
func GetGradeInfo(grade HealthGrade) *GradeInfo {
	for _, g := range GradeTable {
		if g.Grade == grade {
			return &g
		}
	}
	return &GradeTable[len(GradeTable)-1]
}

// ============================================================
// 历史记录与趋势分析
// ============================================================

func (e *Engine) recordHistory(score *HealthScore) {
	e.mu.Lock()
	defer e.mu.Unlock()

	key := score.ID
	e.history[key] = append(e.history[key], score)

	// 限制历史长度
	maxLen := e.config.HistoryRetention
	if maxLen <= 0 {
		maxLen = 1440
	}
	if len(e.history[key]) > maxLen {
		e.history[key] = e.history[key][len(e.history[key])-maxLen:]
	}
}

// GetHistory 获取历史评分
func (e *Engine) GetHistory(id string) []*HealthScore {
	e.mu.RLock()
	defer e.mu.RUnlock()

	h := e.history[id]
	result := make([]*HealthScore, len(h))
	copy(result, h)
	return result
}

// Trend 趋势方向
type Trend string

const (
	TrendImproving  Trend = "improving"  // 改善中
	TrendStable     Trend = "stable"     // 稳定
	TrendDegrading  Trend = "degrading"  // 恶化中
	TrendFluctuating Trend = "fluctuating" // 波动
)

// TrendResult 趋势分析结果
type TrendResult struct {
	Direction     Trend   `json:"direction"`
	Slope         float64 `json:"slope"`          // 变化斜率
	AvgScore      float64 `json:"avg_score"`      // 平均分
	MinScore      float64 `json:"min_score"`      // 最低分
	MaxScore      float64 `json:"max_score"`      // 最高分
	Variance      float64 `json:"variance"`       // 方差
	PredictedScore float64 `json:"predicted_score"` // 预测下一周期得分
}

// AnalyzeTrend 分析健康分趋势
func (e *Engine) AnalyzeTrend(id string) *TrendResult {
	history := e.GetHistory(id)
	if len(history) < 3 {
		return nil
	}

	window := e.config.TrendWindow
	if window <= 0 {
		window = 30
	}
	if window > len(history) {
		window = len(history)
	}

	recent := history[len(history)-window:]

	// 计算统计量
	scores := make([]float64, len(recent))
	for i, s := range recent {
		scores[i] = s.Score
	}

	avg, variance, minS, maxS := calcStats(scores)
	slope := calcSlope(scores)

	// 判断趋势
	direction := TrendStable
	if slope > 0.5 {
		direction = TrendImproving
	} else if slope < -0.5 {
		direction = TrendDegrading
	} else if variance > 25 {
		direction = TrendFluctuating
	}

	// 简单线性预测
	predicted := avg + slope

	return &TrendResult{
		Direction:      direction,
		Slope:          math.Round(slope*100) / 100,
		AvgScore:       math.Round(avg*100) / 100,
		MinScore:       math.Round(minS*100) / 100,
		MaxScore:       math.Round(maxS*100) / 100,
		Variance:       math.Round(variance*100) / 100,
		PredictedScore: math.Round(math.Max(0, math.Min(100, predicted))*100) / 100,
	}
}

// calcStats 计算基本统计量
func calcStats(data []float64) (avg, variance, min, max float64) {
	if len(data) == 0 {
		return 0, 0, 0, 0
	}

	sum := 0.0
	min = data[0]
	max = data[0]
	for _, v := range data {
		sum += v
		if v < min {
			min = v
		}
		if v > max {
			max = v
		}
	}
	avg = sum / float64(len(data))

	varSum := 0.0
	for _, v := range data {
		diff := v - avg
		varSum += diff * diff
	}
	variance = varSum / float64(len(data))
	return
}

// calcSlope 计算线性回归斜率
func calcSlope(data []float64) float64 {
	n := float64(len(data))
	if n < 2 {
		return 0
	}

	sumX := 0.0
	sumY := 0.0
	sumXY := 0.0
	sumX2 := 0.0

	for i, y := range data {
		x := float64(i)
		sumX += x
		sumY += y
		sumXY += x * y
		sumX2 += x * x
	}

	denom := n*sumX2 - sumX*sumX
	if denom == 0 {
		return 0
	}

	slope := (n*sumXY - sumX*sumY) / denom
	return slope
}

// ============================================================
// 告警检查
// ============================================================

func (e *Engine) checkAlert(score *HealthScore) {
	if e.alertCB == nil {
		return
	}

	// 检查是否低于告警阈值
	if score.Score < e.config.AlertThreshold {
		e.alertCB(score)
		return
	}

	// 检查趋势恶化
	if e.config.EnableTrend {
		trend := e.AnalyzeTrend(score.ID)
		if trend != nil && trend.Direction == TrendDegrading && trend.Slope < -2 {
			e.alertCB(score)
		}
	}
}

// ============================================================
// 批量评估与排名
// ============================================================

// RankResult 排名结果
type RankResult struct {
	ID       string      `json:"id"`
	Name     string      `json:"name"`
	Type     string      `json:"type"`
	Score    float64     `json:"score"`
	Grade    HealthGrade `json:"grade"`
	Rank     int         `json:"rank"`
}

// Rank 对多个资源池/VPC按健康分排名
func (e *Engine) Rank(scores []*HealthScore) []*RankResult {
	results := make([]*RankResult, len(scores))
	for i, s := range scores {
		results[i] = &RankResult{
			ID:    s.ID,
			Name:  s.Name,
			Type:  s.Type,
			Score: s.Score,
			Grade: s.Grade,
		}
	}

	// 按分数降序排序
	sort.Slice(results, func(i, j int) bool {
		return results[i].Score > results[j].Score
	})

	// 分配排名
	for i, r := range results {
		r.Rank = i + 1
	}

	return results
}

// GetTopN 获取健康分最高的N个
func (e *Engine) GetTopN(scores []*HealthScore, n int) []*RankResult {
	ranks := e.Rank(scores)
	if n > len(ranks) {
		n = len(ranks)
	}
	return ranks[:n]
}

// GetBottomN 获取健康分最低的N个
func (e *Engine) GetBottomN(scores []*HealthScore, n int) []*RankResult {
	ranks := e.Rank(scores)
	total := len(ranks)
	if n > total {
		n = total
	}
	return ranks[total-n:]
}

// ============================================================
// 综合报告
// ============================================================

// HealthReport 健康报告
type HealthReport struct {
	GeneratedAt   time.Time      `json:"generated_at"`
	TotalPools    int            `json:"total_pools"`
	TotalVPCs     int            `json:"total_vpcs"`
	AvgScore      float64        `json:"avg_score"`
	GradeDistribution map[HealthGrade]int `json:"grade_distribution"`
	TopRisks      []*RankResult  `json:"top_risks"`
	TrendSummary  map[string]*TrendResult `json:"trend_summary,omitempty"`
	Recommendations []string     `json:"recommendations"`
}

// GenerateReport 生成综合健康报告
func (e *Engine) GenerateReport(scores []*HealthScore) *HealthReport {
	report := &HealthReport{
		GeneratedAt:       time.Now(),
		GradeDistribution: make(map[HealthGrade]int),
		TrendSummary:      make(map[string]*TrendResult),
	}

	totalScore := 0.0
	for _, s := range scores {
		totalScore += s.Score
		report.GradeDistribution[s.Grade]++

		if s.Type == "pool" {
			report.TotalPools++
		} else if s.Type == "vpc" {
			report.TotalVPCs++
		}

		// 趋势分析
		if e.config.EnableTrend {
			trend := e.AnalyzeTrend(s.ID)
			if trend != nil {
				report.TrendSummary[s.ID] = trend
			}
		}
	}

	if len(scores) > 0 {
		report.AvgScore = math.Round(totalScore/float64(len(scores))*100) / 100
	}

	// 获取风险最高的资源
	report.TopRisks = e.GetBottomN(scores, 5)

	// 生成建议
	report.Recommendations = e.generateRecommendations(scores)

	return report
}

func (e *Engine) generateRecommendations(scores []*HealthScore) []string {
	var recs []string

	for _, s := range scores {
		if s.Score >= 75 {
			continue
		}

		prefix := fmt.Sprintf("[%s] ", s.Name)

		if s.Utilization < 50 {
			recs = append(recs, prefix+"资源利用率过高，建议扩容或优化负载分配")
		}
		if s.Network < 50 {
			recs = append(recs, prefix+"网络质量较差，建议排查网络瓶颈和丢包原因")
		}
		if s.SLA < 50 {
			recs = append(recs, prefix+"SLA达标率偏低，建议排查错误率和恢复时间")
		}

		// 细分维度建议
		if ds := s.DimensionScores; ds != nil {
			if ds["disk_usage"] < 40 {
				recs = append(recs, prefix+"磁盘使用率超过60%，建议清理或扩容")
			}
			if ds["packet_loss"] < 60 {
				recs = append(recs, prefix+"丢包率偏高，建议检查网络设备和链路质量")
			}
			if ds["error_budget"] < 30 {
				recs = append(recs, prefix+"错误预算即将耗尽，需优先保障稳定性")
			}
		}
	}

	return recs
}
