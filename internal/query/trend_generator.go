// Package query 历史趋势生成器
// 支持生成历史趋势图数据
package query

import (
	"context"
	"database/sql"
	"fmt"
	"math"
	"sort"
	"strings"
	"sync"
	"time"
)

// TrendType 趋势类型
type TrendType string

const (
	TrendLine     TrendType = "line"     // 折线图
	TrendArea     TrendType = "area"     // 面积图
	TrendBar      TrendType = "bar"      // 柱状图
	TrendStacked  TrendType = "stacked"  // 堆叠图
	TrendStep     TrendType = "step"     // 阶梯图
)

// TrendRequest 趋势请求
type TrendRequest struct {
	// 数据源
	SourceType string   `json:"source_type"` // network/application/sql/system
	AssetIDs   []string `json:"asset_ids"`   // 资产ID列表

	// 时间范围
	StartTime time.Time `json:"start_time"`
	EndTime   time.Time `json:"end_time"`

	// 指标配置
	Metrics    []TrendMetric `json:"metrics"`    // 指标列表
	Granularity string        `json:"granularity"` // 时间粒度: 1s/10s/1m/5m/1h/1d

	// 趋势配置
	TrendType  TrendType `json:"trend_type"`  // 趋势类型
	Smooth     bool      `json:"smooth"`      // 平滑曲线
	FillGap    bool      `json:"fill_gap"`    // 填充数据间隙
	CompareEnabled bool  `json:"compare_enabled"` // 启用同比/环比

	// 筛选条件
	Filter *FilterGroup `json:"filter"`

	// 聚合方式
	Aggregation AggFunction `json:"aggregation"` // 聚合函数
}

// TrendMetric 趋势指标
type TrendMetric struct {
	Field       string `json:"field"`        // 字段名
	Alias       string `json:"alias"`        // 别名
	Color       string `json:"color"`        // 颜色
	YAxis       int    `json:"y_axis"`       // Y轴（0左/1右）
	Stack       string `json:"stack"`        // 堆叠组
	Transform   string `json:"transform"`    // 转换: none/rate/percent/diff
}

// TrendResult 趋势结果
type TrendResult struct {
	Timestamps  []time.Time         `json:"timestamps"`  // 时间戳列表
	Series      []*TrendSeries      `json:"series"`      // 数据系列
	Statistics  *TrendStatistics    `json:"statistics"`  // 统计信息
	Granularity string              `json:"granularity"` // 时间粒度
	StartTime   time.Time           `json:"start_time"`
	EndTime     time.Time           `json:"end_time"`
}

// TrendSeries 趋势数据系列
type TrendSeries struct {
	Name     string        `json:"name"`      // 系列名称
	Field    string        `json:"field"`     // 字段名
	Data     []float64     `json:"data"`      // 数据点
	Color    string        `json:"color"`     // 颜色
	YAxis    int           `json:"y_axis"`    // Y轴
	Stack    string        `json:"stack"`     // 堆叠组
	Unit     string        `json:"unit"`      // 单位
	Metadata map[string]string `json:"metadata"` // 元数据
}

// TrendStatistics 趋势统计
type TrendStatistics struct {
	Min      float64   `json:"min"`
	Max      float64   `json:"max"`
	Avg      float64   `json:"avg"`
	Sum      float64   `json:"sum"`
	Count    int       `json:"count"`
	StdDev   float64   `json:"std_dev"`
	P50      float64   `json:"p50"`
	P90      float64   `json:"p90"`
	P95      float64   `json:"p95"`
	P99      float64   `json:"p99"`
	Trend    string    `json:"trend"`      // up/down/stable
	Change   float64   `json:"change"`     // 变化率
	Compare  *CompareResult `json:"compare,omitempty"` // 同比/环比
}

// CompareResult 对比结果
type CompareResult struct {
	Type         string  `json:"type"`          // 同比/环比
	CurrentValue float64 `json:"current_value"` // 当前值
	CompareValue float64 `json:"compare_value"` // 对比值
	ChangeRate   float64 `json:"change_rate"`   // 变化率
	ChangeAbs    float64 `json:"change_abs"`    // 绝对变化
}

// TrendGenerator 趋势生成器
type TrendGenerator struct {
	db     *sql.DB
	driver string
	mu     sync.RWMutex
}

// NewTrendGenerator 创建趋势生成器
func NewTrendGenerator(db *sql.DB, driver string) *TrendGenerator {
	return &TrendGenerator{
		db:     db,
		driver: driver,
	}
}

// Generate 生成趋势数据
func (g *TrendGenerator) Generate(ctx context.Context, request *TrendRequest) (*TrendResult, error) {
	// 计算时间间隔
	interval := g.parseGranularity(request.Granularity)
	if interval == 0 {
		interval = g.calculateInterval(request.StartTime, request.EndTime)
	}

	// 生成时间戳列表
	timestamps := g.generateTimestamps(request.StartTime, request.EndTime, interval)

	// 查询数据
	data, err := g.queryData(ctx, request, interval)
	if err != nil {
		return nil, err
	}

	// 构建趋势系列
	series := make([]*TrendSeries, 0, len(request.Metrics))
	for _, metric := range request.Metrics {
		seriesData := g.buildSeriesData(data, metric, timestamps, request.Aggregation)
		series = append(series, seriesData)
	}

	// 计算统计信息
	statistics := g.calculateStatistics(series, request)

	// 同比/环比对比
	if request.CompareEnabled {
		g.addComparison(ctx, request, statistics)
	}

	return &TrendResult{
		Timestamps:  timestamps,
		Series:      series,
		Statistics:  statistics,
		Granularity: request.Granularity,
		StartTime:   request.StartTime,
		EndTime:     request.EndTime,
	}, nil
}

// parseGranularity 解析时间粒度
func (g *TrendGenerator) parseGranularity(granularity string) time.Duration {
	switch granularity {
	case "1s":
		return time.Second
	case "10s":
		return 10 * time.Second
	case "1m":
		return time.Minute
	case "5m":
		return 5 * time.Minute
	case "15m":
		return 15 * time.Minute
	case "30m":
		return 30 * time.Minute
	case "1h":
		return time.Hour
	case "6h":
		return 6 * time.Hour
	case "12h":
		return 12 * time.Hour
	case "1d":
		return 24 * time.Hour
	case "1w":
		return 7 * 24 * time.Hour
	default:
		return 0
	}
}

// calculateInterval 计算合适的时间间隔
func (g *TrendGenerator) calculateInterval(start, end time.Time) time.Duration {
	duration := end.Sub(start)

	// 根据总时长选择合适的间隔
	switch {
	case duration <= time.Hour:
		return time.Minute
	case duration <= 6*time.Hour:
		return 5 * time.Minute
	case duration <= 24*time.Hour:
		return 15 * time.Minute
	case duration <= 7*24*time.Hour:
		return time.Hour
	case duration <= 30*24*time.Hour:
		return 6 * time.Hour
	default:
		return 24 * time.Hour
	}
}

// generateTimestamps 生成时间戳列表
func (g *TrendGenerator) generateTimestamps(start, end time.Time, interval time.Duration) []time.Time {
	var timestamps []time.Time

	// 对齐到间隔边界
	start = start.Truncate(interval)

	for t := start; t.Before(end) || t.Equal(end); t = t.Add(interval) {
		timestamps = append(timestamps, t)
	}

	return timestamps
}

// queryData 查询数据
func (g *TrendGenerator) queryData(ctx context.Context, request *TrendRequest, interval time.Duration) ([]map[string]interface{}, error) {
	// 构建查询
	tableName := g.getTableName(request.SourceType)

	// 构建字段列表
	var fields []string
	for _, metric := range request.Metrics {
		fields = append(fields, metric.Field)
	}

	// 构建时间分组
	timeGroup := g.buildTimeGroup(interval)

	query := fmt.Sprintf(`
		SELECT 
			%s as time_bucket,
			%s
		FROM %s
		WHERE timestamp >= ? AND timestamp <= ?
	`, timeGroup, strings.Join(fields, ", "), tableName)

	// 添加资产筛选
	var args []interface{}
	args = append(args, request.StartTime, request.EndTime)

	if len(request.AssetIDs) > 0 {
		query += " AND asset_id IN ("
		for i, id := range request.AssetIDs {
			if i > 0 {
				query += ", "
			}
			query += "?"
			args = append(args, id)
		}
		query += ")"
	}

	// 添加筛选条件
	if request.Filter != nil {
		filterClause, filterArgs := g.buildFilterClause(request.Filter)
		if filterClause != "" {
			query += " AND " + filterClause
			args = append(args, filterArgs...)
		}
	}

	// 分组
	query += fmt.Sprintf(" GROUP BY %s ORDER BY %s", timeGroup, timeGroup)

	// 执行查询
	rows, err := g.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	// 扫描结果
	columns, _ := rows.Columns()
	var results []map[string]interface{}

	for rows.Next() {
		row := make(map[string]interface{})
		values := make([]interface{}, len(columns))
		valuePtrs := make([]interface{}, len(columns))
		for i := range values {
			valuePtrs[i] = &values[i]
		}

		if err := rows.Scan(valuePtrs...); err != nil {
			return nil, err
		}

		for i, col := range columns {
			row[col] = values[i]
		}
		results = append(results, row)
	}

	return results, nil
}

// buildTimeGroup 构建时间分组表达式
func (g *TrendGenerator) buildTimeGroup(interval time.Duration) string {
	intervalSec := int64(interval.Seconds())

	switch g.driver {
	case "sqlite":
		return fmt.Sprintf("datetime(timestamp, 'unixepoch', 'localtime', '+' || (strftime('%%s', timestamp) %% %d) || ' seconds')", intervalSec)
	case "mysql":
		return fmt.Sprintf("FROM_UNIXTIME(FLOOR(UNIX_TIMESTAMP(timestamp) / %d) * %d)", intervalSec, intervalSec)
	case "postgres":
		return fmt.Sprintf("to_timestamp(FLOOR(EXTRACT(EPOCH FROM timestamp) / %d) * %d)", intervalSec, intervalSec)
	default:
		return "timestamp"
	}
}

// getTableName 获取表名
func (g *TrendGenerator) getTableName(sourceType string) string {
	switch sourceType {
	case "network":
		return "network_metrics"
	case "application":
		return "application_metrics"
	case "sql":
		return "sql_metrics"
	case "system":
		return "system_metrics"
	default:
		return sourceType
	}
}

// buildFilterClause 构建筛选条件
func (g *TrendGenerator) buildFilterClause(filter *FilterGroup) (string, []interface{}) {
	if filter == nil {
		return "", nil
	}

	var clauses []string
	var args []interface{}

	for _, cond := range filter.Conditions {
		clause := fmt.Sprintf("%s %s ?", cond.Field, cond.Operator)
		clauses = append(clauses, clause)
		args = append(args, cond.Value)
	}

	if len(clauses) == 0 {
		return "", nil
	}

	logic := string(filter.Logic)
	if logic == "" {
		logic = "AND"
	}

	return strings.Join(clauses, " "+logic+" "), args
}

// buildSeriesData 构建系列数据
func (g *TrendGenerator) buildSeriesData(data []map[string]interface{}, metric TrendMetric, timestamps []time.Time, aggregation AggFunction) *TrendSeries {
	// 创建时间到数据的映射
	dataMap := make(map[time.Time]float64)
	for _, row := range data {
		if timeVal, ok := row["time_bucket"]; ok {
			var t time.Time
			switch v := timeVal.(type) {
			case time.Time:
				t = v
			case string:
				t, _ = time.Parse(time.RFC3339, v)
			}

			if val, ok := row[metric.Field]; ok {
				dataMap[t] = toFloat64(val)
			}
		}
	}

	// 构建数据点
	points := make([]float64, len(timestamps))
	for i, ts := range timestamps {
		if val, ok := dataMap[ts]; ok {
			points[i] = val
		} else {
			points[i] = 0 // 填充0或使用插值
		}
	}

	return &TrendSeries{
		Name:  metric.Alias,
		Field: metric.Field,
		Data:  points,
		Color: metric.Color,
		YAxis: metric.YAxis,
		Stack: metric.Stack,
	}
}

// calculateStatistics 计算统计信息
func (g *TrendGenerator) calculateStatistics(series []*TrendSeries, request *TrendRequest) *TrendStatistics {
	if len(series) == 0 || len(series[0].Data) == 0 {
		return &TrendStatistics{}
	}

	// 合并所有数据点
	var allValues []float64
	for _, s := range series {
		for _, v := range s.Data {
			if v != 0 { // 排除填充的0值
				allValues = append(allValues, v)
			}
		}
	}

	if len(allValues) == 0 {
		return &TrendStatistics{}
	}

	// 排序用于百分位数计算
	sort.Float64s(allValues)

	stats := &TrendStatistics{
		Min:   allValues[0],
		Max:   allValues[len(allValues)-1],
		Count: len(allValues),
	}

	// 计算平均值和总和
	var sum float64
	for _, v := range allValues {
		sum += v
	}
	stats.Sum = sum
	stats.Avg = sum / float64(len(allValues))

	// 计算标准差
	var sumSq float64
	for _, v := range allValues {
		diff := v - stats.Avg
		sumSq += diff * diff
	}
	stats.StdDev = math.Sqrt(sumSq / float64(len(allValues)))

	// 计算百分位数
	stats.P50 = percentile(allValues, 0.50)
	stats.P90 = percentile(allValues, 0.90)
	stats.P95 = percentile(allValues, 0.95)
	stats.P99 = percentile(allValues, 0.99)

	// 计算趋势
	stats.Trend = g.calculateTrend(series[0].Data)
	stats.Change = g.calculateChange(series[0].Data)

	return stats
}

// percentile 计算百分位数
func percentile(sorted []float64, p float64) float64 {
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

// calculateTrend 计算趋势方向
func (g *TrendGenerator) calculateTrend(data []float64) string {
	if len(data) < 2 {
		return "stable"
	}

	// 计算前半部分和后半部分的平均值
	mid := len(data) / 2
	var firstHalf, secondHalf float64
	var firstCount, secondCount int

	for i, v := range data {
		if i < mid {
			firstHalf += v
			firstCount++
		} else {
			secondHalf += v
			secondCount++
		}
	}

	if firstCount == 0 || secondCount == 0 {
		return "stable"
	}

	firstAvg := firstHalf / float64(firstCount)
	secondAvg := secondHalf / float64(secondCount)

	// 判断趋势
	threshold := 0.05 // 5%变化阈值
	change := (secondAvg - firstAvg) / firstAvg

	if change > threshold {
		return "up"
	} else if change < -threshold {
		return "down"
	}
	return "stable"
}

// calculateChange 计算变化率
func (g *TrendGenerator) calculateChange(data []float64) float64 {
	if len(data) < 2 {
		return 0
	}

	first := data[0]
	last := data[len(data)-1]

	if first == 0 {
		return 0
	}

	return (last - first) / first * 100
}

// addComparison 添加同比/环比对比
func (g *TrendGenerator) addComparison(ctx context.Context, request *TrendRequest, stats *TrendStatistics) {
	// 计算环比（上一周期）
	duration := request.EndTime.Sub(request.StartTime)
	compareStart := request.StartTime.Add(-duration)
	compareEnd := request.StartTime

	// 查询对比数据
	compareRequest := &TrendRequest{
		SourceType:  request.SourceType,
		AssetIDs:    request.AssetIDs,
		StartTime:   compareStart,
		EndTime:     compareEnd,
		Metrics:     request.Metrics,
		Granularity: request.Granularity,
		Filter:      request.Filter,
	}

	compareResult, err := g.Generate(ctx, compareRequest)
	if err != nil {
		return
	}

	if compareResult.Statistics != nil && compareResult.Statistics.Avg > 0 {
		stats.Compare = &CompareResult{
			Type:         "环比",
			CurrentValue: stats.Avg,
			CompareValue: compareResult.Statistics.Avg,
			ChangeRate:   (stats.Avg - compareResult.Statistics.Avg) / compareResult.Statistics.Avg * 100,
			ChangeAbs:    stats.Avg - compareResult.Statistics.Avg,
		}
	}
}

// GenerateMultiTrend 生成多指标趋势
func (g *TrendGenerator) GenerateMultiTrend(ctx context.Context, requests []*TrendRequest) (map[string]*TrendResult, error) {
	results := make(map[string]*TrendResult)
	var mu sync.Mutex
	var wg sync.WaitGroup
	errCh := make(chan error, len(requests))

	for i, req := range requests {
		wg.Add(1)
		go func(index int, request *TrendRequest) {
			defer wg.Done()

			result, err := g.Generate(ctx, request)
			if err != nil {
				errCh <- err
				return
			}

			mu.Lock()
			results[fmt.Sprintf("series_%d", index)] = result
			mu.Unlock()
		}(i, req)
	}

	wg.Wait()
	close(errCh)

	for err := range errCh {
		if err != nil {
			return nil, err
		}
	}

	return results, nil
}

// GenerateHeatmap 生成热力图数据
func (g *TrendGenerator) GenerateHeatmap(ctx context.Context, request *TrendRequest, xDimension, yDimension string) (*HeatmapResult, error) {
	// 查询数据
	interval := g.parseGranularity(request.Granularity)
	if interval == 0 {
		interval = g.calculateInterval(request.StartTime, request.EndTime)
	}

	data, err := g.queryData(ctx, request, interval)
	if err != nil {
		return nil, err
	}

	// 构建热力图
	result := &HeatmapResult{
		XLabels: make([]string, 0),
		YLabels: make([]string, 0),
		Data:    make([][]float64, 0),
	}

	// 提取X轴标签（时间）
	xMap := make(map[string]bool)
	yMap := make(map[string]bool)
	valueMap := make(map[string]map[string]float64)

	for _, row := range data {
		xVal := fmt.Sprintf("%v", row["time_bucket"])
		yVal := fmt.Sprintf("%v", row[yDimension])

		xMap[xVal] = true
		yMap[yVal] = true

		if valueMap[yVal] == nil {
			valueMap[yVal] = make(map[string]float64)
		}

		if val, ok := row[request.Metrics[0].Field]; ok {
			valueMap[yVal][xVal] = toFloat64(val)
		}
	}

	// 排序标签
	for x := range xMap {
		result.XLabels = append(result.XLabels, x)
	}
	sort.Strings(result.XLabels)

	for y := range yMap {
		result.YLabels = append(result.YLabels, y)
	}
	sort.Strings(result.YLabels)

	// 构建数据矩阵
	for _, y := range result.YLabels {
		row := make([]float64, len(result.XLabels))
		for i, x := range result.XLabels {
			row[i] = valueMap[y][x]
		}
		result.Data = append(result.Data, row)
	}

	return result, nil
}

// HeatmapResult 热力图结果
type HeatmapResult struct {
	XLabels []string     `json:"x_labels"` // X轴标签
	YLabels []string     `json:"y_labels"` // Y轴标签
	Data    [][]float64  `json:"data"`     // 数据矩阵
}

// GenerateSparkline 生成迷你图数据
func (g *TrendGenerator) GenerateSparkline(ctx context.Context, request *TrendRequest) (*SparklineResult, error) {
	result, err := g.Generate(ctx, request)
	if err != nil {
		return nil, err
	}

	sparkline := &SparklineResult{
		Data:       result.Series[0].Data,
		Min:        result.Statistics.Min,
		Max:        result.Statistics.Max,
		Avg:        result.Statistics.Avg,
		Trend:      result.Statistics.Trend,
		Change:     result.Statistics.Change,
		LastValue:  result.Series[0].Data[len(result.Series[0].Data)-1],
	}

	return sparkline, nil
}

// SparklineResult 迷你图结果
type SparklineResult struct {
	Data      []float64 `json:"data"`       // 数据点
	Min       float64   `json:"min"`        // 最小值
	Max       float64   `json:"max"`        // 最大值
	Avg       float64   `json:"avg"`        // 平均值
	Trend     string    `json:"trend"`      // 趋势
	Change    float64   `json:"change"`     // 变化率
	LastValue float64   `json:"last_value"` // 最后一个值
}

// GenerateGauge 生成仪表盘数据
func (g *TrendGenerator) GenerateGauge(ctx context.Context, request *TrendRequest) (*GaugeResult, error) {
	result, err := g.Generate(ctx, request)
	if err != nil {
		return nil, err
	}

	gauge := &GaugeResult{
		Value:     result.Statistics.Avg,
		Min:       result.Statistics.Min,
		Max:       result.Statistics.Max,
		Threshold: make(map[string]float64),
	}

	// 设置阈值
	if len(request.Metrics) > 0 {
		// 根据指标类型设置默认阈值
		field := request.Metrics[0].Field
		switch {
		case strings.Contains(field, "cpu") || strings.Contains(field, "memory"):
			gauge.Threshold["warning"] = 70
			gauge.Threshold["critical"] = 90
		case strings.Contains(field, "latency") || strings.Contains(field, "response"):
			gauge.Threshold["warning"] = 100
			gauge.Threshold["critical"] = 500
		default:
			gauge.Threshold["warning"] = result.Statistics.Avg * 1.5
			gauge.Threshold["critical"] = result.Statistics.Avg * 2
		}
	}

	return gauge, nil
}

// GaugeResult 仪表盘结果
type GaugeResult struct {
	Value     float64            `json:"value"`     // 当前值
	Min       float64            `json:"min"`       // 最小值
	Max       float64            `json:"max"`       // 最大值
	Threshold map[string]float64 `json:"threshold"` // 阈值
}

// toFloat64 转换为float64
func toFloat64(val interface{}) float64 {
	switch v := val.(type) {
	case float64:
		return v
	case float32:
		return float64(v)
	case int:
		return float64(v)
	case int64:
		return float64(v)
	case int32:
		return float64(v)
	case uint:
		return float64(v)
	case uint64:
		return float64(v)
	case uint32:
		return float64(v)
	default:
		return 0
	}
}
