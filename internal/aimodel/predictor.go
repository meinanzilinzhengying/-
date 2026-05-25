// Package aimodel 提供AI建模、正常区间预测、智能告警功能
// 基于时间序列分析实现异常检测
// Copyright (c) 2026 Cloud Flow Team
// Licensed under the MIT License.

package aimodel

import (
	"context"
	"fmt"
	"math"
	"sync"
	"sync/atomic"
	"time"
)

// ============================================================
// 配置定义
// ============================================================

// AIModelConfig AI建模配置
type AIModelConfig struct {
	Enabled              bool          `yaml:"enabled" json:"enabled"`
	ModelType            string        `yaml:"model_type" json:"model_type"`             // 模型类型: arima, lstm, prophet, ensemble
	TrainInterval        time.Duration `yaml:"train_interval" json:"train_interval"`     // 训练间隔
	PredictionWindow     int           `yaml:"prediction_window" json:"prediction_window"` // 预测窗口大小
	HistoryWindow        int           `yaml:"history_window" json:"history_window"`     // 历史数据窗口大小
	ConfidenceLevel      float64       `yaml:"confidence_level" json:"confidence_level"` // 置信区间 (0-1)
	AnomalyThreshold     float64       `yaml:"anomaly_threshold" json:"anomaly_threshold"` // 异常阈值 (标准差倍数)
	MinDataPoints        int           `yaml:"min_data_points" json:"min_data_points"`   // 最小数据点数
	SeasonalityPeriod    int           `yaml:"seasonality_period" json:"seasonality_period"` // 季节性周期
	EnableAutoTuning     bool          `yaml:"enable_auto_tuning" json:"enable_auto_tuning"` // 自动调参
	Metrics              []string      `yaml:"metrics" json:"metrics"`                   // 监控的指标列表
}

func DefaultAIModelConfig() *AIModelConfig {
	return &AIModelConfig{
		Enabled:           true,
		ModelType:         "ensemble", // 集成模型
		TrainInterval:     1 * time.Hour,
		PredictionWindow:  60,    // 预测60个点
		HistoryWindow:     1440,  // 使用1440个历史点（约24小时，每分钟一个点）
		ConfidenceLevel:   0.95,  // 95%置信区间
		AnomalyThreshold:  3.0,   // 3倍标准差
		MinDataPoints:     100,   // 最少100个数据点
		SeasonalityPeriod: 1440,  // 日周期（分钟级）
		EnableAutoTuning:  true,
		Metrics:           []string{"cpu_usage", "memory_usage", "network_bytes", "disk_io"},
	}
}

// ============================================================
// 数据模型
// ============================================================

// TimeSeriesPoint 时间序列数据点
type TimeSeriesPoint struct {
	Timestamp time.Time `json:"timestamp"`
	Value     float64   `json:"value"`
	Labels    map[string]string `json:"labels,omitempty"`
}

// PredictionResult 预测结果
type PredictionResult struct {
	Timestamp    time.Time `json:"timestamp"`
	Predicted    float64   `json:"predicted"`
	LowerBound   float64   `json:"lower_bound"`   // 置信区间下界
	UpperBound   float64   `json:"upper_bound"`   // 置信区间上界
	Actual       float64   `json:"actual"`        // 实际值（用于对比）
	Residual     float64   `json:"residual"`      // 残差
	IsAnomaly    bool      `json:"is_anomaly"`    // 是否异常
	AnomalyScore float64   `json:"anomaly_score"` // 异常分数 (0-1)
}

// NormalRange 正常区间
type NormalRange struct {
	MetricName  string    `json:"metric_name"`
	StartTime   time.Time `json:"start_time"`
	EndTime     time.Time `json:"end_time"`
	LowerBound  float64   `json:"lower_bound"`
	UpperBound  float64   `json:"upper_bound"`
	Mean        float64   `json:"mean"`
	StdDev      float64   `json:"std_dev"`
	Confidence  float64   `json:"confidence"`
	SampleCount int       `json:"sample_count"`
}

// AnomalyType 异常类型
type AnomalyType int

const (
	AnomalyTypeUnknown AnomalyType = iota
	AnomalyTypeSpike        // 尖峰
	AnomalyTypeDrop         // 骤降
	AnomalyTypeLevelShift   // 水平偏移
	AnomalyTypeTrendChange  // 趋势变化
	AnomalyTypeSeasonality  // 季节性异常
)

func (a AnomalyType) String() string {
	names := map[AnomalyType]string{
		AnomalyTypeSpike:       "spike",
		AnomalyTypeDrop:        "drop",
		AnomalyTypeLevelShift:  "level_shift",
		AnomalyTypeTrendChange: "trend_change",
		AnomalyTypeSeasonality: "seasonality",
	}
	if name, ok := names[a]; ok {
		return name
	}
	return "unknown"
}

// AnomalyEvent 异常事件
type AnomalyEvent struct {
	ID           string            `json:"id"`
	Timestamp    time.Time         `json:"timestamp"`
	MetricName   string            `json:"metric_name"`
	AnomalyType  AnomalyType       `json:"anomaly_type"`
	ActualValue  float64           `json:"actual_value"`
	ExpectedMin  float64           `json:"expected_min"`
	ExpectedMax  float64           `json:"expected_max"`
	Deviation    float64           `json:"deviation"`    // 偏离程度（标准差倍数）
	Severity     string            `json:"severity"`     // info, warning, critical
	Confidence   float64           `json:"confidence"`   // 置信度
	Context      map[string]string `json:"context"`      // 上下文信息
	RelatedAlerts []string         `json:"related_alerts,omitempty"`
}

// ModelMetrics 模型指标
type ModelMetrics struct {
	ModelType       string    `json:"model_type"`
	LastTrained     time.Time `json:"last_trained"`
	TrainDataPoints int       `json:"train_data_points"`
	RMSE            float64   `json:"rmse"`            // 均方根误差
	MAE             float64   `json:"mae"`             // 平均绝对误差
	MAPE            float64   `json:"mape"`            // 平均绝对百分比误差
	R2              float64   `json:"r2"`              // R平方
	PredictionCount uint64    `json:"prediction_count"`
	AnomalyCount    uint64    `json:"anomaly_count"`
}

// ============================================================
// 时间序列预测模型接口
// ============================================================

// TimeSeriesModel 时间序列模型接口
type TimeSeriesModel interface {
	// Train 训练模型
	Train(data []TimeSeriesPoint) error
	
	// Predict 预测未来值
	Predict(steps int) ([]PredictionResult, error)
	
	// PredictWithInterval 带置信区间的预测
	PredictWithInterval(steps int, confidence float64) ([]PredictionResult, error)
	
	// DetectAnomaly 检测异常
	DetectAnomaly(point TimeSeriesPoint) (*AnomalyEvent, error)
	
	// GetMetrics 获取模型指标
	GetMetrics() *ModelMetrics
	
	// Name 模型名称
	Name() string
}

// ============================================================
// 统计模型实现
// ============================================================

// StatisticalModel 统计模型（基于移动平均和标准差）
type StatisticalModel struct {
	config        *AIModelConfig
	metricName    string
	
	// 历史数据
	data       []TimeSeriesPoint
	dataMu     sync.RWMutex
	
	// 统计量
	mean       float64
	stdDev     float64
	min        float64
	max        float64
	trend      float64 // 趋势
	
	// 季节性
	seasonality []float64
	
	// 模型指标
	metrics    ModelMetrics
	
	// 状态
	lastTrainTime time.Time
	trained       bool
}

// NewStatisticalModel 创建统计模型
func NewStatisticalModel(metricName string, config *AIModelConfig) *StatisticalModel {
	return &StatisticalModel{
		config:     config,
		metricName: metricName,
		data:       make([]TimeSeriesPoint, 0, config.HistoryWindow),
		metrics: ModelMetrics{
			ModelType: "statistical",
		},
	}
}

// Train 训练统计模型
func (m *StatisticalModel) Train(data []TimeSeriesPoint) error {
	if len(data) < m.config.MinDataPoints {
		return fmt.Errorf("insufficient data points: %d < %d", len(data), m.config.MinDataPoints)
	}

	m.dataMu.Lock()
	defer m.dataMu.Unlock()

	// 存储数据
	m.data = make([]TimeSeriesPoint, len(data))
	copy(m.data, data)

	// 计算基本统计量
	var sum, sumSq float64
	for _, p := range data {
		sum += p.Value
		sumSq += p.Value * p.Value
		if p.Value < m.min || m.min == 0 {
			m.min = p.Value
		}
		if p.Value > m.max {
			m.max = p.Value
		}
	}

	n := float64(len(data))
	m.mean = sum / n
	variance := (sumSq / n) - (m.mean * m.mean)
	if variance < 0 {
		variance = 0
	}
	m.stdDev = math.Sqrt(variance)

	// 计算趋势（简单线性回归）
	m.calculateTrend(data)

	// 计算季节性
	if m.config.SeasonalityPeriod > 0 {
		m.calculateSeasonality(data)
	}

	// 更新模型指标
	m.metrics.LastTrained = time.Now()
	m.metrics.TrainDataPoints = len(data)
	m.lastTrainTime = time.Now()
	m.trained = true

	// 计算模型误差
	m.calculateMetrics(data)

	return nil
}

// calculateTrend 计算趋势
func (m *StatisticalModel) calculateTrend(data []TimeSeriesPoint) {
	if len(data) < 2 {
		m.trend = 0
		return
	}

	// 简单线性回归
	n := float64(len(data))
	var sumX, sumY, sumXY, sumX2 float64

	for i, p := range data {
		x := float64(i)
		y := p.Value
		sumX += x
		sumY += y
		sumXY += x * y
		sumX2 += x * x
	}

	denominator := n*sumX2 - sumX*sumX
	if denominator == 0 {
		m.trend = 0
		return
	}

	m.trend = (n*sumXY - sumX*sumY) / denominator
}

// calculateSeasonality 计算季节性
func (m *StatisticalModel) calculateSeasonality(data []TimeSeriesPoint) {
	period := m.config.SeasonalityPeriod
	if len(data) < period*2 {
		return
	}

	// 计算每个周期位置的平均值
	m.seasonality = make([]float64, period)
	counts := make([]int, period)

	for i, p := range data {
		pos := i % period
		m.seasonality[pos] += p.Value
		counts[pos]++
	}

	for i := 0; i < period; i++ {
		if counts[i] > 0 {
			m.seasonality[i] /= float64(counts[i])
		}
	}

	// 标准化季节性（去除均值）
	seasonMean := 0.0
	for _, s := range m.seasonality {
		seasonMean += s
	}
	seasonMean /= float64(period)

	for i := range m.seasonality {
		m.seasonality[i] -= seasonMean
	}
}

// calculateMetrics 计算模型指标
func (m *StatisticalModel) calculateMetrics(data []TimeSeriesPoint) {
	if len(data) < 2 {
		return
	}

	var sumSqErr, sumAbsErr, sumAbsPercErr float64
	var sumSqTotal float64

	for i, p := range data {
		predicted := m.mean
		if m.trend != 0 {
			predicted += m.trend * float64(i)
		}
		if len(m.seasonality) > 0 {
			predicted += m.seasonality[i%len(m.seasonality)]
		}

		err := p.Value - predicted
		sumSqErr += err * err
		sumAbsErr += math.Abs(err)
		if p.Value != 0 {
			sumAbsPercErr += math.Abs(err / p.Value)
		}
		sumSqTotal += (p.Value - m.mean) * (p.Value - m.mean)
	}

	n := float64(len(data))
	m.metrics.RMSE = math.Sqrt(sumSqErr / n)
	m.metrics.MAE = sumAbsErr / n
	m.metrics.MAPE = (sumAbsPercErr / n) * 100

	if sumSqTotal > 0 {
		m.metrics.R2 = 1 - (sumSqErr / sumSqTotal)
	}
}

// Predict 预测未来值
func (m *StatisticalModel) Predict(steps int) ([]PredictionResult, error) {
	return m.PredictWithInterval(steps, m.config.ConfidenceLevel)
}

// PredictWithInterval 带置信区间的预测
func (m *StatisticalModel) PredictWithInterval(steps int, confidence float64) ([]PredictionResult, error) {
	if !m.trained {
		return nil, fmt.Errorf("model not trained")
	}

	m.dataMu.RLock()
	lastIdx := len(m.data)
	m.dataMu.RUnlock()

	results := make([]PredictionResult, steps)
	
	// 计算置信区间因子（正态分布）
	zScore := getZScore(confidence)

	for i := 0; i < steps; i++ {
		futureIdx := lastIdx + i
		
		// 基础预测值
		predicted := m.mean
		
		// 加入趋势
		if m.trend != 0 {
			predicted += m.trend * float64(futureIdx)
		}
		
		// 加入季节性
		if len(m.seasonality) > 0 {
			predicted += m.seasonality[futureIdx%len(m.seasonality)]
		}

		// 置信区间
		margin := zScore * m.stdDev * math.Sqrt(1+float64(i)/float64(lastIdx))
		
		results[i] = PredictionResult{
			Timestamp:  time.Now().Add(time.Duration(i) * time.Minute),
			Predicted:  predicted,
			LowerBound: predicted - margin,
			UpperBound: predicted + margin,
		}
	}

	m.metrics.PredictionCount += uint64(steps)
	return results, nil
}

// DetectAnomaly 检测异常
func (m *StatisticalModel) DetectAnomaly(point TimeSeriesPoint) (*AnomalyEvent, error) {
	if !m.trained {
		return nil, fmt.Errorf("model not trained")
	}

	// 计算预期值
	m.dataMu.RLock()
	idx := len(m.data)
	m.dataMu.RUnlock()

	expected := m.mean
	if m.trend != 0 {
		expected += m.trend * float64(idx)
	}
	if len(m.seasonality) > 0 {
		expected += m.seasonality[idx%len(m.seasonality)]
	}

	// 计算偏差
	deviation := (point.Value - expected) / m.stdDev
	absDeviation := math.Abs(deviation)

	// 判断是否异常
	isAnomaly := absDeviation > m.config.AnomalyThreshold

	if !isAnomaly {
		return nil, nil
	}

	// 确定异常类型
	anomalyType := m.classifyAnomaly(point.Value, expected, deviation)

	// 计算异常分数
	anomalyScore := math.Min(1.0, absDeviation/10.0)

	// 确定严重程度
	severity := "warning"
	if absDeviation > m.config.AnomalyThreshold*2 {
		severity = "critical"
	}

	// 计算置信区间
	zScore := getZScore(m.config.ConfidenceLevel)
	lowerBound := expected - zScore*m.stdDev
	upperBound := expected + zScore*m.stdDev

	m.metrics.AnomalyCount++

	return &AnomalyEvent{
		ID:          fmt.Sprintf("%s-%d", m.metricName, point.Timestamp.Unix()),
		Timestamp:   point.Timestamp,
		MetricName:  m.metricName,
		AnomalyType: anomalyType,
		ActualValue: point.Value,
		ExpectedMin: lowerBound,
		ExpectedMax: upperBound,
		Deviation:   absDeviation,
		Severity:    severity,
		Confidence:  m.config.ConfidenceLevel,
		Context: map[string]string{
			"expected_value": fmt.Sprintf("%.2f", expected),
			"mean":           fmt.Sprintf("%.2f", m.mean),
			"std_dev":        fmt.Sprintf("%.2f", m.stdDev),
			"trend":          fmt.Sprintf("%.4f", m.trend),
		},
	}, nil
}

// classifyAnomaly 分类异常类型
func (m *StatisticalModel) classifyAnomaly(actual, expected, deviation float64) AnomalyType {
	// 简单分类逻辑
	if deviation > 0 {
		// 正向偏差
		if deviation > 5 {
			return AnomalyTypeSpike
		}
		return AnomalyTypeLevelShift
	} else {
		// 负向偏差
		if deviation < -5 {
			return AnomalyTypeDrop
		}
		return AnomalyTypeLevelShift
	}
}

// GetMetrics 获取模型指标
func (m *StatisticalModel) GetMetrics() *ModelMetrics {
	return &m.metrics
}

// Name 模型名称
func (m *StatisticalModel) Name() string {
	return "statistical"
}

// ============================================================
// 集成模型（多模型融合）
// ============================================================

// EnsembleModel 集成模型
type EnsembleModel struct {
	config     *AIModelConfig
	metricName string
	models     []TimeSeriesModel
	weights    []float64
	metrics    ModelMetrics
	trained    bool
}

// NewEnsembleModel 创建集成模型
func NewEnsembleModel(metricName string, config *AIModelConfig) *EnsembleModel {
	model := &EnsembleModel{
		config:     config,
		metricName: metricName,
		metrics: ModelMetrics{
			ModelType: "ensemble",
		},
	}

	// 添加多个子模型
	model.models = []TimeSeriesModel{
		NewStatisticalModel(metricName, config),
		NewMovingAverageModel(metricName, config),
		NewExponentialSmoothingModel(metricName, config),
	}

	// 初始权重（均等）
	model.weights = make([]float64, len(model.models))
	for i := range model.weights {
		model.weights[i] = 1.0 / float64(len(model.models))
	}

	return model
}

// Train 训练集成模型
func (m *EnsembleModel) Train(data []TimeSeriesPoint) error {
	var lastErr error
	
	for _, model := range m.models {
		if err := model.Train(data); err != nil {
			lastErr = err
		}
	}

	// 根据模型表现调整权重
	m.adjustWeights(data)

	m.metrics.LastTrained = time.Now()
	m.metrics.TrainDataPoints = len(data)
	m.trained = true

	return lastErr
}

// adjustWeights 调整模型权重
func (m *EnsembleModel) adjustWeights(data []TimeSeriesPoint) {
	if len(data) < 10 {
		return
	}

	// 使用交叉验证评估每个模型
	errors := make([]float64, len(m.models))
	
	for i, model := range m.models {
		metrics := model.GetMetrics()
		if metrics.RMSE > 0 {
			errors[i] = 1.0 / metrics.RMSE // RMSE越小权重越大
		}
	}

	// 归一化权重
	totalError := 0.0
	for _, e := range errors {
		totalError += e
	}

	if totalError > 0 {
		for i := range m.weights {
			m.weights[i] = errors[i] / totalError
		}
	}
}

// Predict 预测
func (m *EnsembleModel) Predict(steps int) ([]PredictionResult, error) {
	return m.PredictWithInterval(steps, m.config.ConfidenceLevel)
}

// PredictWithInterval 带置信区间预测
func (m *EnsembleModel) PredictWithInterval(steps int, confidence float64) ([]PredictionResult, error) {
	if !m.trained {
		return nil, fmt.Errorf("model not trained")
	}

	// 获取所有模型的预测
	allPredictions := make([][]PredictionResult, len(m.models))
	for i, model := range m.models {
		pred, err := model.PredictWithInterval(steps, confidence)
		if err != nil {
			continue
		}
		allPredictions[i] = pred
	}

	// 加权融合
	results := make([]PredictionResult, steps)
	for j := 0; j < steps; j++ {
		var predicted, lower, upper float64
		var totalWeight float64

		for i, pred := range allPredictions {
			if j < len(pred) {
				w := m.weights[i]
				predicted += w * pred[j].Predicted
				lower += w * pred[j].LowerBound
				upper += w * pred[j].UpperBound
				totalWeight += w
			}
		}

		if totalWeight > 0 {
			predicted /= totalWeight
			lower /= totalWeight
			upper /= totalWeight
		}

		results[j] = PredictionResult{
			Timestamp:  time.Now().Add(time.Duration(j) * time.Minute),
			Predicted:  predicted,
			LowerBound: lower,
			UpperBound: upper,
		}
	}

	m.metrics.PredictionCount += uint64(steps)
	return results, nil
}

// DetectAnomaly 检测异常
func (m *EnsembleModel) DetectAnomaly(point TimeSeriesPoint) (*AnomalyEvent, error) {
	if !m.trained {
		return nil, fmt.Errorf("model not trained")
	}

	// 收集所有模型的异常检测结果
	var anomalies []*AnomalyEvent
	var totalWeight float64

	for i, model := range m.models {
		anomaly, err := model.DetectAnomaly(point)
		if err != nil {
			continue
		}
		if anomaly != nil {
			anomalies = append(anomalies, anomaly)
			totalWeight += m.weights[i]
		}
	}

	// 如果超过一半的模型认为是异常，则判定为异常
	if totalWeight < 0.5 {
		return nil, nil
	}

	// 合并异常事件
	if len(anomalies) > 0 {
		// 使用权重最高的模型结果
		return anomalies[0], nil
	}

	return nil, nil
}

// GetMetrics 获取模型指标
func (m *EnsembleModel) GetMetrics() *ModelMetrics {
	return &m.metrics
}

// Name 模型名称
func (m *EnsembleModel) Name() string {
	return "ensemble"
}

// ============================================================
// 移动平均模型
// ============================================================

// MovingAverageModel 移动平均模型
type MovingAverageModel struct {
	config     *AIModelConfig
	metricName string
	data       []TimeSeriesPoint
	windowSize int
	mean       float64
	stdDev     float64
	metrics    ModelMetrics
	trained    bool
}

// NewMovingAverageModel 创建移动平均模型
func NewMovingAverageModel(metricName string, config *AIModelConfig) *MovingAverageModel {
	windowSize := config.HistoryWindow / 10
	if windowSize < 10 {
		windowSize = 10
	}
	
	return &MovingAverageModel{
		config:     config,
		metricName: metricName,
		windowSize: windowSize,
		metrics: ModelMetrics{
			ModelType: "moving_average",
		},
	}
}

// Train 训练
func (m *MovingAverageModel) Train(data []TimeSeriesPoint) error {
	if len(data) < m.windowSize {
		return fmt.Errorf("insufficient data")
	}

	m.data = make([]TimeSeriesPoint, len(data))
	copy(m.data, data)

	// 计算移动平均的统计量
	var sum, sumSq float64
	start := len(data) - m.windowSize
	for i := start; i < len(data); i++ {
		sum += data[i].Value
		sumSq += data[i].Value * data[i].Value
	}

	n := float64(m.windowSize)
	m.mean = sum / n
	variance := (sumSq / n) - (m.mean * m.mean)
	if variance < 0 {
		variance = 0
	}
	m.stdDev = math.Sqrt(variance)

	m.metrics.LastTrained = time.Now()
	m.metrics.TrainDataPoints = len(data)
	m.trained = true

	return nil
}

// Predict 预测
func (m *MovingAverageModel) Predict(steps int) ([]PredictionResult, error) {
	return m.PredictWithInterval(steps, m.config.ConfidenceLevel)
}

// PredictWithInterval 带置信区间预测
func (m *MovingAverageModel) PredictWithInterval(steps int, confidence float64) ([]PredictionResult, error) {
	if !m.trained {
		return nil, fmt.Errorf("model not trained")
	}

	zScore := getZScore(confidence)
	margin := zScore * m.stdDev

	results := make([]PredictionResult, steps)
	for i := 0; i < steps; i++ {
		results[i] = PredictionResult{
			Timestamp:  time.Now().Add(time.Duration(i) * time.Minute),
			Predicted:  m.mean,
			LowerBound: m.mean - margin,
			UpperBound: m.mean + margin,
		}
	}

	return results, nil
}

// DetectAnomaly 检测异常
func (m *MovingAverageModel) DetectAnomaly(point TimeSeriesPoint) (*AnomalyEvent, error) {
	if !m.trained {
		return nil, fmt.Errorf("model not trained")
	}

	deviation := (point.Value - m.mean) / m.stdDev
	absDeviation := math.Abs(deviation)

	if absDeviation <= m.config.AnomalyThreshold {
		return nil, nil
	}

	severity := "warning"
	if absDeviation > m.config.AnomalyThreshold*2 {
		severity = "critical"
	}

	anomalyType := AnomalyTypeSpike
	if deviation < 0 {
		anomalyType = AnomalyTypeDrop
	}

	return &AnomalyEvent{
		ID:          fmt.Sprintf("%s-ma-%d", m.metricName, point.Timestamp.Unix()),
		Timestamp:   point.Timestamp,
		MetricName:  m.metricName,
		AnomalyType: anomalyType,
		ActualValue: point.Value,
		ExpectedMin: m.mean - getZScore(m.config.ConfidenceLevel)*m.stdDev,
		ExpectedMax: m.mean + getZScore(m.config.ConfidenceLevel)*m.stdDev,
		Deviation:   absDeviation,
		Severity:    severity,
		Confidence:  m.config.ConfidenceLevel,
	}, nil
}

// GetMetrics 获取模型指标
func (m *MovingAverageModel) GetMetrics() *ModelMetrics {
	return &m.metrics
}

// Name 模型名称
func (m *MovingAverageModel) Name() string {
	return "moving_average"
}

// ============================================================
// 指数平滑模型
// ============================================================

// ExponentialSmoothingModel 指数平滑模型
type ExponentialSmoothingModel struct {
	config     *AIModelConfig
	metricName string
	alpha      float64 // 平滑系数
	level      float64
	trend      float64
	stdDev     float64
	data       []TimeSeriesPoint
	metrics    ModelMetrics
	trained    bool
}

// NewExponentialSmoothingModel 创建指数平滑模型
func NewExponentialSmoothingModel(metricName string, config *AIModelConfig) *ExponentialSmoothingModel {
	return &ExponentialSmoothingModel{
		config:     config,
		metricName: metricName,
		alpha:      0.3, // 默认平滑系数
		metrics: ModelMetrics{
			ModelType: "exponential_smoothing",
		},
	}
}

// Train 训练
func (m *ExponentialSmoothingModel) Train(data []TimeSeriesPoint) error {
	if len(data) < 2 {
		return fmt.Errorf("insufficient data")
	}

	m.data = make([]TimeSeriesPoint, len(data))
	copy(m.data, data)

	// 初始化
	m.level = data[0].Value
	m.trend = data[1].Value - data[0].Value

	// Holt双参数指数平滑
	var sumSqErr float64
	for i := 1; i < len(data); i++ {
		prevLevel := m.level
		prevTrend := m.trend

		m.level = m.alpha*data[i].Value + (1-m.alpha)*(prevLevel+prevTrend)
		m.trend = m.alpha*(m.level-prevLevel) + (1-m.alpha)*prevTrend

		predicted := prevLevel + prevTrend
		err := data[i].Value - predicted
		sumSqErr += err * err
	}

	m.stdDev = math.Sqrt(sumSqErr / float64(len(data)-1))

	m.metrics.LastTrained = time.Now()
	m.metrics.TrainDataPoints = len(data)
	m.trained = true

	return nil
}

// Predict 预测
func (m *ExponentialSmoothingModel) Predict(steps int) ([]PredictionResult, error) {
	return m.PredictWithInterval(steps, m.config.ConfidenceLevel)
}

// PredictWithInterval 带置信区间预测
func (m *ExponentialSmoothingModel) PredictWithInterval(steps int, confidence float64) ([]PredictionResult, error) {
	if !m.trained {
		return nil, fmt.Errorf("model not trained")
	}

	zScore := getZScore(confidence)
	results := make([]PredictionResult, steps)

	for i := 0; i < steps; i++ {
		predicted := m.level + float64(i+1)*m.trend
		margin := zScore * m.stdDev * math.Sqrt(1+float64(i)*0.1)

		results[i] = PredictionResult{
			Timestamp:  time.Now().Add(time.Duration(i) * time.Minute),
			Predicted:  predicted,
			LowerBound: predicted - margin,
			UpperBound: predicted + margin,
		}
	}

	return results, nil
}

// DetectAnomaly 检测异常
func (m *ExponentialSmoothingModel) DetectAnomaly(point TimeSeriesPoint) (*AnomalyEvent, error) {
	if !m.trained {
		return nil, fmt.Errorf("model not trained")
	}

	expected := m.level + m.trend
	deviation := (point.Value - expected) / m.stdDev
	absDeviation := math.Abs(deviation)

	if absDeviation <= m.config.AnomalyThreshold {
		return nil, nil
	}

	severity := "warning"
	if absDeviation > m.config.AnomalyThreshold*2 {
		severity = "critical"
	}

	anomalyType := AnomalyTypeLevelShift
	if absDeviation > 5 {
		if deviation > 0 {
			anomalyType = AnomalyTypeSpike
		} else {
			anomalyType = AnomalyTypeDrop
		}
	}

	return &AnomalyEvent{
		ID:          fmt.Sprintf("%s-es-%d", m.metricName, point.Timestamp.Unix()),
		Timestamp:   point.Timestamp,
		MetricName:  m.metricName,
		AnomalyType: anomalyType,
		ActualValue: point.Value,
		ExpectedMin: expected - getZScore(m.config.ConfidenceLevel)*m.stdDev,
		ExpectedMax: expected + getZScore(m.config.ConfidenceLevel)*m.stdDev,
		Deviation:   absDeviation,
		Severity:    severity,
		Confidence:  m.config.ConfidenceLevel,
	}, nil
}

// GetMetrics 获取模型指标
func (m *ExponentialSmoothingModel) GetMetrics() *ModelMetrics {
	return &m.metrics
}

// Name 模型名称
func (m *ExponentialSmoothingModel) Name() string {
	return "exponential_smoothing"
}

// ============================================================
// 辅助函数
// ============================================================

// getZScore 获取正态分布Z分数
func getZScore(confidence float64) float64 {
	// 常用置信区间对应的Z分数
	zScores := map[float64]float64{
		0.90: 1.645,
		0.95: 1.96,
		0.99: 2.576,
	}

	if z, ok := zScores[confidence]; ok {
		return z
	}

	// 默认返回95%置信区间
	return 1.96
}
