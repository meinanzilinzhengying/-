// Package aimodel 智能告警系统
// Copyright (c) 2026 Cloud Flow Team
// Licensed under the MIT License.

package aimodel

import (
	"context"
	"fmt"
	"sync"
	"sync/atomic"
	"time"
)

// ============================================================
// 智能告警系统
// ============================================================

// AlertConfig 告警配置
type AlertConfig struct {
	Enabled           bool          `yaml:"enabled" json:"enabled"`
	CooldownPeriod    time.Duration `yaml:"cooldown_period" json:"cooldown_period"`       // 告警冷却期
	AggregationWindow time.Duration `yaml:"aggregation_window" json:"aggregation_window"` // 聚合窗口
	MaxAlertsPerMin   int           `yaml:"max_alerts_per_min" json:"max_alerts_per_min"` // 每分钟最大告警数
	SuppressDuplicates bool         `yaml:"suppress_duplicates" json:"suppress_duplicates"` // 抑制重复告警
	NotificationChannels []string   `yaml:"notification_channels" json:"notification_channels"` // 通知渠道
}

func DefaultAlertConfig() *AlertConfig {
	return &AlertConfig{
		Enabled:            true,
		CooldownPeriod:     5 * time.Minute,
		AggregationWindow:  1 * time.Minute,
		MaxAlertsPerMin:    100,
		SuppressDuplicates: true,
		NotificationChannels: []string{"log", "webhook"},
	}
}

// Alert 告警
type Alert struct {
	ID           string            `json:"id"`
	Timestamp    time.Time         `json:"timestamp"`
	MetricName   string            `json:"metric_name"`
	AnomalyType  AnomalyType       `json:"anomaly_type"`
	Severity     string            `json:"severity"`
	Message      string            `json:"message"`
	ActualValue  float64           `json:"actual_value"`
	ExpectedMin  float64           `json:"expected_min"`
	ExpectedMax  float64           `json:"expected_max"`
	Deviation    float64           `json:"deviation"`
	Confidence   float64           `json:"confidence"`
	Context      map[string]string `json:"context"`
	Silenced     bool              `json:"silenced"`
	Acknowledged bool              `json:"acknowledged"`
}

// AlertManager 告警管理器
type AlertManager struct {
	config *AlertConfig

	// 告警历史
	alerts     []*Alert
	alertsMu   sync.RWMutex
	alertCount atomic.Uint64

	// 告警冷却
	cooldowns   map[string]time.Time // metric -> last alert time
	cooldownMu  sync.RWMutex

	// 告警聚合
	pendingAlerts   []*AnomalyEvent
	pendingMu       sync.Mutex
	lastAggregation time.Time

	// 速率限制
	alertsThisMin atomic.Int32
	lastReset     time.Time

	// 回调
	onAlert func(*Alert)

	// 生命周期
	ctx    context.Context
	cancel context.CancelFunc
	wg     sync.WaitGroup
	running atomic.Bool
}

// NewAlertManager 创建告警管理器
func NewAlertManager(cfg *AlertConfig) *AlertManager {
	if cfg == nil {
		cfg = DefaultAlertConfig()
	}

	ctx, cancel := context.WithCancel(context.Background())

	return &AlertManager{
		config:        cfg,
		alerts:        make([]*Alert, 0, 1000),
		cooldowns:     make(map[string]time.Time),
		pendingAlerts: make([]*AnomalyEvent, 0, 100),
		ctx:           ctx,
		cancel:        cancel,
		lastReset:     time.Now(),
	}
}

// Start 启动告警管理器
func (am *AlertManager) Start() error {
	if am.running.Load() {
		return fmt.Errorf("alert manager already running")
	}

	am.running.Store(true)

	// 启动聚合循环
	am.wg.Add(1)
	go am.aggregationLoop()

	return nil
}

// Stop 停止告警管理器
func (am *AlertManager) Stop() error {
	if !am.running.Load() {
		return nil
	}

	am.running.Store(false)
	am.cancel()
	am.wg.Wait()

	return nil
}

// ProcessAnomaly 处理异常事件
func (am *AlertManager) ProcessAnomaly(event *AnomalyEvent) {
	if !am.config.Enabled {
		return
	}

	// 检查冷却期
	if am.isInCooldown(event.MetricName) {
		return
	}

	// 速率限制
	if !am.checkRateLimit() {
		return
	}

	// 添加到待处理队列
	am.pendingMu.Lock()
	am.pendingAlerts = append(am.pendingAlerts, event)
	am.pendingMu.Unlock()
}

// isInCooldown 检查是否在冷却期
func (am *AlertManager) isInCooldown(metricName string) bool {
	if !am.config.SuppressDuplicates {
		return false
	}

	am.cooldownMu.RLock()
	lastAlert, exists := am.cooldowns[metricName]
	am.cooldownMu.RUnlock()

	if !exists {
		return false
	}

	return time.Since(lastAlert) < am.config.CooldownPeriod
}

// checkRateLimit 检查速率限制
func (am *AlertManager) checkRateLimit() bool {
	now := time.Now()

	// 每分钟重置计数器
	if now.Sub(am.lastReset) >= time.Minute {
		am.alertsThisMin.Store(0)
		am.lastReset = now
	}

	current := am.alertsThisMin.Load()
	if int(current) >= am.config.MaxAlertsPerMin {
		return false
	}

	am.alertsThisMin.Add(1)
	return true
}

// aggregationLoop 聚合循环
func (am *AlertManager) aggregationLoop() {
	defer am.wg.Done()

	ticker := time.NewTicker(am.config.AggregationWindow)
	defer ticker.Stop()

	for {
		select {
		case <-am.ctx.Done():
			return
		case <-ticker.C:
			am.aggregateAndSend()
		}
	}
}

// aggregateAndSend 聚合并发送告警
func (am *AlertManager) aggregateAndSend() {
	am.pendingMu.Lock()
	events := am.pendingAlerts
	am.pendingAlerts = make([]*AnomalyEvent, 0, 100)
	am.pendingMu.Unlock()

	if len(events) == 0 {
		return
	}

	// 按指标和严重程度分组
	groups := am.groupEvents(events)

	// 为每个组创建告警
	for key, groupEvents := range groups {
		alert := am.createAlert(key, groupEvents)
		if alert != nil {
			am.sendAlert(alert)
		}
	}
}

// groupEvents 分组事件
func (am *AlertManager) groupEvents(events []*AnomalyEvent) map[string][]*AnomalyEvent {
	groups := make(map[string][]*AnomalyEvent)

	for _, event := range events {
		key := fmt.Sprintf("%s-%s", event.MetricName, event.Severity)
		groups[key] = append(groups[key], event)
	}

	return groups
}

// createAlert 创建告警
func (am *AlertManager) createAlert(key string, events []*AnomalyEvent) *Alert {
	if len(events) == 0 {
		return nil
	}

	// 使用最新的事件
	latest := events[len(events)-1]

	alert := &Alert{
		ID:          fmt.Sprintf("alert-%d", am.alertCount.Add(1)),
		Timestamp:   time.Now(),
		MetricName:  latest.MetricName,
		AnomalyType: latest.AnomalyType,
		Severity:    latest.Severity,
		ActualValue: latest.ActualValue,
		ExpectedMin: latest.ExpectedMin,
		ExpectedMax: latest.ExpectedMax,
		Deviation:   latest.Deviation,
		Confidence:  latest.Confidence,
		Context:     latest.Context,
	}

	// 构建消息
	alert.Message = am.buildAlertMessage(alert, len(events))

	// 更新冷却时间
	am.cooldownMu.Lock()
	am.cooldowns[latest.MetricName] = time.Now()
	am.cooldownMu.Unlock()

	return alert
}

// buildAlertMessage 构建告警消息
func (am *AlertManager) buildAlertMessage(alert *Alert, count int) string {
	var direction string
	if alert.ActualValue > alert.ExpectedMax {
		direction = "高于"
	} else {
		direction = "低于"
	}

	msg := fmt.Sprintf("指标 %s 异常: 当前值 %.2f %s预期区间 [%.2f, %.2f], 偏离 %.1f 倍标准差",
		alert.MetricName,
		alert.ActualValue,
		direction,
		alert.ExpectedMin,
		alert.ExpectedMax,
		alert.Deviation,
	)

	if count > 1 {
		msg += fmt.Sprintf(" (聚合 %d 个事件)", count)
	}

	return msg
}

// sendAlert 发送告警
func (am *AlertManager) sendAlert(alert *Alert) {
	// 存储告警
	am.alertsMu.Lock()
	am.alerts = append(am.alerts, alert)
	// 保留最近1000条
	if len(am.alerts) > 1000 {
		am.alerts = am.alerts[len(am.alerts)-1000:]
	}
	am.alertsMu.Unlock()

	// 触发回调
	if am.onAlert != nil {
		go am.onAlert(alert)
	}
}

// OnAlert 设置告警回调
func (am *AlertManager) OnAlert(fn func(*Alert)) {
	am.onAlert = fn
}

// GetAlerts 获取告警列表
func (am *AlertManager) GetAlerts(limit int) []*Alert {
	am.alertsMu.RLock()
	defer am.alertsMu.RUnlock()

	if limit <= 0 || limit > len(am.alerts) {
		limit = len(am.alerts)
	}

	result := make([]*Alert, limit)
	copy(result, am.alerts[len(am.alerts)-limit:])
	return result
}

// GetAlertCount 获取告警数量
func (am *AlertManager) GetAlertCount() uint64 {
	return am.alertCount.Load()
}

// Silence 静默告警
func (am *AlertManager) Silence(metricName string, duration time.Duration) {
	am.cooldownMu.Lock()
	defer am.cooldownMu.Unlock()

	am.cooldowns[metricName] = time.Now().Add(duration)
}

// Acknowledge 确认告警
func (am *AlertManager) Acknowledge(alertID string) {
	am.alertsMu.Lock()
	defer am.alertsMu.Unlock()

	for _, alert := range am.alerts {
		if alert.ID == alertID {
			alert.Acknowledged = true
			break
		}
	}
}

// ============================================================
// 分析引擎
// ============================================================

// AnalysisEngine 分析引擎
type AnalysisEngine struct {
	config *AIModelConfig

	// 模型
	models    map[string]TimeSeriesModel // metric -> model
	modelsMu  sync.RWMutex

	// 数据缓冲
	dataBuffer map[string][]TimeSeriesPoint
	bufferMu   sync.Mutex

	// 告警管理器
	alertManager *AlertManager

	// 生命周期
	ctx    context.Context
	cancel context.CancelFunc
	wg     sync.WaitGroup
	running atomic.Bool

	// 回调
	onPrediction func(metric string, results []PredictionResult)
}

// NewAnalysisEngine 创建分析引擎
func NewAnalysisEngine(cfg *AIModelConfig, alertCfg *AlertConfig) *AnalysisEngine {
	if cfg == nil {
		cfg = DefaultAIModelConfig()
	}

	ctx, cancel := context.WithCancel(context.Background())

	return &AnalysisEngine{
		config:       cfg,
		models:       make(map[string]TimeSeriesModel),
		dataBuffer:   make(map[string][]TimeSeriesPoint),
		alertManager: NewAlertManager(alertCfg),
		ctx:          ctx,
		cancel:       cancel,
	}
}

// Start 启动分析引擎
func (e *AnalysisEngine) Start() error {
	if e.running.Load() {
		return fmt.Errorf("analysis engine already running")
	}

	// 启动告警管理器
	if err := e.alertManager.Start(); err != nil {
		return err
	}

	e.running.Store(true)

	// 启动训练循环
	e.wg.Add(1)
	go e.trainingLoop()

	// 启动预测循环
	e.wg.Add(1)
	go e.predictionLoop()

	return nil
}

// Stop 停止分析引擎
func (e *AnalysisEngine) Stop() error {
	if !e.running.Load() {
		return nil
	}

	e.running.Store(false)
	e.cancel()

	e.alertManager.Stop()
	e.wg.Wait()

	return nil
}

// IngestData 摄入数据
func (e *AnalysisEngine) IngestData(metric string, value float64, labels map[string]string) {
	point := TimeSeriesPoint{
		Timestamp: time.Now(),
		Value:     value,
		Labels:    labels,
	}

	e.bufferMu.Lock()
	e.dataBuffer[metric] = append(e.dataBuffer[metric], point)

	// 限制缓冲区大小
	if len(e.dataBuffer[metric]) > e.config.HistoryWindow*2 {
		e.dataBuffer[metric] = e.dataBuffer[metric][e.config.HistoryWindow:]
	}
	e.bufferMu.Unlock()

	// 实时异常检测
	e.detectAnomalyRealtime(metric, point)
}

// detectAnomalyRealtime 实时异常检测
func (e *AnalysisEngine) detectAnomalyRealtime(metric string, point TimeSeriesPoint) {
	e.modelsMu.RLock()
	model, exists := e.models[metric]
	e.modelsMu.RUnlock()

	if !exists {
		return
	}

	anomaly, err := model.DetectAnomaly(point)
	if err != nil || anomaly == nil {
		return
	}

	// 发送到告警管理器
	e.alertManager.ProcessAnomaly(anomaly)
}

// trainingLoop 训练循环
func (e *AnalysisEngine) trainingLoop() {
	defer e.wg.Done()

	ticker := time.NewTicker(e.config.TrainInterval)
	defer ticker.Stop()

	for {
		select {
		case <-e.ctx.Done():
			return
		case <-ticker.C:
			e.trainModels()
		}
	}
}

// trainModels 训练模型
func (e *AnalysisEngine) trainModels() {
	for _, metric := range e.config.Metrics {
		e.trainModel(metric)
	}
}

// trainModel 训练单个模型
func (e *AnalysisEngine) trainModel(metric string) {
	e.bufferMu.Lock()
	data := e.dataBuffer[metric]
	e.bufferMu.Unlock()

	if len(data) < e.config.MinDataPoints {
		return
	}

	// 创建或获取模型
	e.modelsMu.Lock()
	model, exists := e.models[metric]
	if !exists {
		model = e.createModel(metric)
		e.models[metric] = model
	}
	e.modelsMu.Unlock()

	// 训练模型
	_ = model.Train(data)
}

// createModel 创建模型
func (e *AnalysisEngine) createModel(metric string) TimeSeriesModel {
	switch e.config.ModelType {
	case "statistical":
		return NewStatisticalModel(metric, e.config)
	case "moving_average":
		return NewMovingAverageModel(metric, e.config)
	case "exponential_smoothing":
		return NewExponentialSmoothingModel(metric, e.config)
	case "ensemble":
		return NewEnsembleModel(metric, e.config)
	default:
		return NewEnsembleModel(metric, e.config)
	}
}

// predictionLoop 预测循环
func (e *AnalysisEngine) predictionLoop() {
	defer e.wg.Done()

	ticker := time.NewTicker(1 * time.Minute)
	defer ticker.Stop()

	for {
		select {
		case <-e.ctx.Done():
			return
		case <-ticker.C:
			e.generatePredictions()
		}
	}
}

// generatePredictions 生成预测
func (e *AnalysisEngine) generatePredictions() {
	e.modelsMu.RLock()
	defer e.modelsMu.RUnlock()

	for metric, model := range e.models {
		results, err := model.Predict(e.config.PredictionWindow)
		if err != nil {
			continue
		}

		if e.onPrediction != nil {
			go e.onPrediction(metric, results)
		}
	}
}

// GetNormalRange 获取正常区间
func (e *AnalysisEngine) GetNormalRange(metric string) (*NormalRange, error) {
	e.modelsMu.RLock()
	model, exists := e.models[metric]
	e.modelsMu.RUnlock()

	if !exists {
		return nil, fmt.Errorf("model not found for metric: %s", metric)
	}

	// 获取预测结果
	results, err := model.Predict(1)
	if err != nil {
		return nil, err
	}

	if len(results) == 0 {
		return nil, fmt.Errorf("no prediction results")
	}

	metrics := model.GetMetrics()

	return &NormalRange{
		MetricName:  metric,
		StartTime:   time.Now(),
		EndTime:     results[0].Timestamp,
		LowerBound:  results[0].LowerBound,
		UpperBound:  results[0].UpperBound,
		Mean:        results[0].Predicted,
		StdDev:      (results[0].UpperBound - results[0].LowerBound) / (2 * getZScore(e.config.ConfidenceLevel)),
		Confidence:  e.config.ConfidenceLevel,
		SampleCount: metrics.TrainDataPoints,
	}, nil
}

// Predict 预测
func (e *AnalysisEngine) Predict(metric string, steps int) ([]PredictionResult, error) {
	e.modelsMu.RLock()
	model, exists := e.models[metric]
	e.modelsMu.RUnlock()

	if !exists {
		return nil, fmt.Errorf("model not found for metric: %s", metric)
	}

	return model.Predict(steps)
}

// GetModelMetrics 获取模型指标
func (e *AnalysisEngine) GetModelMetrics(metric string) (*ModelMetrics, error) {
	e.modelsMu.RLock()
	model, exists := e.models[metric]
	e.modelsMu.RUnlock()

	if !exists {
		return nil, fmt.Errorf("model not found for metric: %s", metric)
	}

	return model.GetMetrics(), nil
}

// OnPrediction 设置预测回调
func (e *AnalysisEngine) OnPrediction(fn func(metric string, results []PredictionResult)) {
	e.onPrediction = fn
}

// OnAlert 设置告警回调
func (e *AnalysisEngine) OnAlert(fn func(*Alert)) {
	e.alertManager.OnAlert(fn)
}

// GetAlerts 获取告警
func (e *AnalysisEngine) GetAlerts(limit int) []*Alert {
	return e.alertManager.GetAlerts(limit)
}

// ForceTrain 强制训练
func (e *AnalysisEngine) ForceTrain(metric string) error {
	e.trainModel(metric)
	return nil
}
