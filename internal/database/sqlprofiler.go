/*
 * Cloud Flow Agent - SQL Profiler
 *
 * 数据库 SQL 聚合与性能分析
 * 支持 MySQL/PostgreSQL/Redis 等数据库的 SQL 采集与分析
 */

package database

import (
	"context"
	"fmt"
	"hash/fnv"
	"regexp"
	"sort"
	"strings"
	"sync"
	"sync/atomic"
	"time"
)

// DBType 数据库类型
type DBType string

const (
	DBTypeMySQL      DBType = "mysql"
	DBTypePostgreSQL DBType = "postgresql"
	DBTypeRedis      DBType = "redis"
	DBTypeMongoDB    DBType = "mongodb"
	DBTypeOracle     DBType = "oracle"
	DBTypeSQLServer  DBType = "sqlserver"
)

// SQLProfilerConfig SQL 剖析器配置
type SQLProfilerConfig struct {
	Enabled           bool          // 启用 SQL 剖析
	DBTypes           []DBType      // 监控的数据库类型
	SampleRate        float64       // 采样率
	SlowQueryThreshold time.Duration // 慢查询阈值
	MaxSQLLength      int           // 最大 SQL 长度
	AggregationWindow time.Duration // 聚合窗口
	MaxQueries        int           // 最大保留查询数
	ExplainEnabled    bool          // 启用执行计划分析
	TopN              int           // Top N 查询
}

// DefaultSQLProfilerConfig 默认配置
func DefaultSQLProfilerConfig() *SQLProfilerConfig {
	return &SQLProfilerConfig{
		Enabled:            true,
		DBTypes:            []DBType{DBTypeMySQL, DBTypePostgreSQL, DBTypeRedis},
		SampleRate:         1.0,
		SlowQueryThreshold: 100 * time.Millisecond,
		MaxSQLLength:       4096,
		AggregationWindow:  60 * time.Second,
		MaxQueries:         10000,
		ExplainEnabled:     true,
		TopN:               20,
	}
}

// SQLQuery SQL 查询
type SQLQuery struct {
	ID          string            `json:"id"`
	Fingerprint string            `json:"fingerprint"` // SQL 指纹（规范化后的 SQL）
	RawSQL      string            `json:"raw_sql"`     // 原始 SQL
	DBType      DBType            `json:"db_type"`
	DBName      string            `json:"db_name"`
	Tables      []string          `json:"tables"`
	Operation   string            `json:"operation"` // SELECT/INSERT/UPDATE/DELETE
	Timestamp   time.Time         `json:"timestamp"`
	Duration    time.Duration     `json:"duration"`
	RowsAffected int64            `json:"rows_affected"`
	RowsReturned int64            `json:"rows_returned"`
	Error       string            `json:"error,omitempty"`
	ClientIP    string            `json:"client_ip"`
	SessionID   string            `json:"session_id"`
	TraceID     string            `json:"trace_id"`
	Tags        map[string]string `json:"tags"`
}

// SQLStats SQL 统计
type SQLStats struct {
	Fingerprint    string            `json:"fingerprint"`
	DBType         DBType            `json:"db_type"`
	Operation      string            `json:"operation"`
	Tables         []string          `json:"tables"`
	
	// 执行统计
	CallCount      uint64            `json:"call_count"`
	TotalTime      time.Duration     `json:"total_time"`
	AvgTime        time.Duration     `json:"avg_time"`
	MinTime        time.Duration     `json:"min_time"`
	MaxTime        time.Duration     `json:"max_time"`
	P95Time        time.Duration     `json:"p95_time"`
	P99Time        time.Duration     `json:"p99_time"`
	
	// 行数统计
	TotalRowsAffected uint64         `json:"total_rows_affected"`
	TotalRowsReturned uint64         `json:"total_rows_returned"`
	AvgRowsAffected   float64        `json:"avg_rows_affected"`
	AvgRowsReturned   float64        `json:"avg_rows_returned"`
	
	// 错误统计
	ErrorCount     uint64            `json:"error_count"`
	ErrorRate      float64           `json:"error_rate"`
	
	// 时间窗口
	FirstSeen      time.Time         `json:"first_seen"`
	LastSeen       time.Time         `json:"last_seen"`
	
	// 示例
	SampleQueries  []*SQLQuery       `json:"sample_queries,omitempty"`
}

// SQLAggregateResult SQL 聚合结果
type SQLAggregateResult struct {
	WindowStart    time.Time         `json:"window_start"`
	WindowEnd      time.Time         `json:"window_end"`
	DBType         DBType            `json:"db_type"`
	TotalQueries   uint64            `json:"total_queries"`
	SlowQueries    uint64            `json:"slow_queries"`
	ErrorQueries   uint64            `json:"error_queries"`
	TotalTime      time.Duration     `json:"total_time"`
	Stats          []*SQLStats       `json:"stats"`
	TopSlowQueries []*SQLQuery       `json:"top_slow_queries"`
	HotTables      []*TableStats     `json:"hot_tables"`
}

// TableStats 表统计
type TableStats struct {
	TableName      string            `json:"table_name"`
	DBType         DBType            `json:"db_type"`
	SelectCount    uint64            `json:"select_count"`
	InsertCount    uint64            `json:"insert_count"`
	UpdateCount    uint64            `json:"update_count"`
	DeleteCount    uint64            `json:"delete_count"`
	TotalTime      time.Duration     `json:"total_time"`
}

// SQLProfiler SQL 剖析器
type SQLProfiler struct {
	mu sync.RWMutex

	config *SQLProfilerConfig

	// 查询队列
	queryCh chan *SQLQuery

	// 聚合存储
	aggregates map[string]*SQLStats // key = fingerprint
	
	// 慢查询存储
	slowQueries []*SQLQuery

	// 表统计
	tableStats map[string]*TableStats // key = db_type:table_name

	// 统计
	stats ProfilerStats

	// 控制
	ctx    context.Context
	cancel context.CancelFunc
}

// ProfilerStats 剖析统计
type ProfilerStats struct {
	TotalQueries   uint64
	SlowQueries    uint64
	ErrorQueries   uint64
	AggregatedStats uint64
}

// NewSQLProfiler 创建 SQL 剖析器
func NewSQLProfiler(config *SQLProfilerConfig) (*SQLProfiler, error) {
	if config == nil {
		config = DefaultSQLProfilerConfig()
	}

	ctx, cancel := context.WithCancel(context.Background())

	profiler := &SQLProfiler{
		config:      config,
		queryCh:     make(chan *SQLQuery, 10000),
		aggregates:  make(map[string]*SQLStats),
		slowQueries: make([]*SQLQuery, 0),
		tableStats:  make(map[string]*TableStats),
		ctx:         ctx,
		cancel:      cancel,
	}

	// 启动处理协程
	go profiler.processLoop()
	go profiler.aggregationLoop()

	return profiler, nil
}

// RecordQuery 记录 SQL 查询
func (p *SQLProfiler) RecordQuery(query *SQLQuery) error {
	if !p.config.Enabled {
		return nil
	}

	// 采样检查
	if p.config.SampleRate < 1.0 {
		if time.Now().UnixNano()%1000000 > int64(p.config.SampleRate*1000000) {
			return nil
		}
	}

	// 截断过长的 SQL
	if len(query.RawSQL) > p.config.MaxSQLLength {
		query.RawSQL = query.RawSQL[:p.config.MaxSQLLength] + "..."
	}

	// 生成指纹
	query.Fingerprint = p.generateFingerprint(query.RawSQL)
	query.ID = p.generateQueryID(query)

	// 解析操作类型和表名
	query.Operation = p.parseOperation(query.RawSQL)
	query.Tables = p.parseTables(query.RawSQL)

	// 发送到处理队列
	select {
	case p.queryCh <- query:
		atomic.AddUint64(&p.stats.TotalQueries, 1)
		return nil
	default:
		return fmt.Errorf("query channel is full")
	}
}

// processLoop 处理循环
func (p *SQLProfiler) processLoop() {
	for {
		select {
		case query := <-p.queryCh:
			p.processQuery(query)
		case <-p.ctx.Done():
			return
		}
	}
}

// processQuery 处理单个查询
func (p *SQLProfiler) processQuery(query *SQLQuery) {
	// 检查是否为慢查询
	isSlow := query.Duration >= p.config.SlowQueryThreshold
	if isSlow {
		atomic.AddUint64(&p.stats.SlowQueries, 1)
		p.addSlowQuery(query)
	}

	// 检查错误
	if query.Error != "" {
		atomic.AddUint64(&p.stats.ErrorQueries, 1)
	}

	// 更新聚合统计
	p.updateAggregate(query)

	// 更新表统计
	p.updateTableStats(query)
}

// updateAggregate 更新聚合统计
func (p *SQLProfiler) updateAggregate(query *SQLQuery) {
	p.mu.Lock()
	defer p.mu.Unlock()

	key := string(query.DBType) + ":" + query.Fingerprint
	stats, exists := p.aggregates[key]
	if !exists {
		stats = &SQLStats{
			Fingerprint:   query.Fingerprint,
			DBType:        query.DBType,
			Operation:     query.Operation,
			Tables:        query.Tables,
			FirstSeen:     query.Timestamp,
			MinTime:       query.Duration,
			MaxTime:       query.Duration,
			SampleQueries: make([]*SQLQuery, 0),
		}
		p.aggregates[key] = stats
		atomic.AddUint64(&p.stats.AggregatedStats, 1)
	}

	// 更新时间
	stats.LastSeen = query.Timestamp
	stats.CallCount++
	stats.TotalTime += query.Duration
	stats.TotalRowsAffected += uint64(query.RowsAffected)
	stats.TotalRowsReturned += uint64(query.RowsReturned)

	// 更新最值
	if query.Duration < stats.MinTime {
		stats.MinTime = query.Duration
	}
	if query.Duration > stats.MaxTime {
		stats.MaxTime = query.Duration
	}

	// 更新错误统计
	if query.Error != "" {
		stats.ErrorCount++
	}
	stats.ErrorRate = float64(stats.ErrorCount) / float64(stats.CallCount) * 100

	// 保存样本（保留最近 10 个不同执行时间的样本）
	if len(stats.SampleQueries) < 10 {
		stats.SampleQueries = append(stats.SampleQueries, query)
	}

	// 计算平均值
	stats.AvgTime = time.Duration(int64(stats.TotalTime) / int64(stats.CallCount))
	stats.AvgRowsAffected = float64(stats.TotalRowsAffected) / float64(stats.CallCount)
	stats.AvgRowsReturned = float64(stats.TotalRowsReturned) / float64(stats.CallCount)
}

// updateTableStats 更新表统计
func (p *SQLProfiler) updateTableStats(query *SQLQuery) {
	p.mu.Lock()
	defer p.mu.Unlock()

	for _, table := range query.Tables {
		key := string(query.DBType) + ":" + table
		stats, exists := p.tableStats[key]
		if !exists {
			stats = &TableStats{
				TableName: table,
				DBType:    query.DBType,
			}
			p.tableStats[key] = stats
		}

		stats.TotalTime += query.Duration

		switch query.Operation {
		case "SELECT":
			stats.SelectCount++
		case "INSERT":
			stats.InsertCount++
		case "UPDATE":
			stats.UpdateCount++
		case "DELETE":
			stats.DeleteCount++
		}
	}
}

// addSlowQuery 添加慢查询
func (p *SQLProfiler) addSlowQuery(query *SQLQuery) {
	p.mu.Lock()
	defer p.mu.Unlock()

	p.slowQueries = append(p.slowQueries, query)

	// 只保留最近的 1000 条慢查询
	if len(p.slowQueries) > 1000 {
		p.slowQueries = p.slowQueries[len(p.slowQueries)-1000:]
	}
}

// aggregationLoop 聚合循环
func (p *SQLProfiler) aggregationLoop() {
	ticker := time.NewTicker(p.config.AggregationWindow)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			p.flushAggregates()
		case <-p.ctx.Done():
			return
		}
	}
}

// flushAggregates 刷新聚合结果
func (p *SQLProfiler) flushAggregates() *SQLAggregateResult {
	p.mu.Lock()
	defer p.mu.Unlock()

	if len(p.aggregates) == 0 {
		return nil
	}

	now := time.Now()
	result := &SQLAggregateResult{
		WindowEnd: now,
		Stats:     make([]*SQLStats, 0, len(p.aggregates)),
	}

	// 复制统计
	for _, stats := range p.aggregates {
		result.TotalQueries += stats.CallCount
		result.TotalTime += stats.TotalTime
		
		if stats.MaxTime >= p.config.SlowQueryThreshold {
			result.SlowQueries += stats.CallCount
		}
		result.ErrorQueries += stats.ErrorCount

		// 复制统计对象
		statsCopy := *stats
		result.Stats = append(result.Stats, &statsCopy)
	}

	// 按调用次数排序
	sort.Slice(result.Stats, func(i, j int) bool {
		return result.Stats[i].CallCount > result.Stats[j].CallCount
	})

	// 取 Top N
	if len(result.Stats) > p.config.TopN {
		result.Stats = result.Stats[:p.config.TopN]
	}

	// 获取慢查询 Top N
	if len(p.slowQueries) > 0 {
		sort.Slice(p.slowQueries, func(i, j int) bool {
			return p.slowQueries[i].Duration > p.slowQueries[j].Duration
		})
		topN := p.config.TopN
		if len(p.slowQueries) < topN {
			topN = len(p.slowQueries)
		}
		result.TopSlowQueries = p.slowQueries[:topN]
	}

	// 获取热点表
	tables := make([]*TableStats, 0, len(p.tableStats))
	for _, stats := range p.tableStats {
		tables = append(tables, stats)
	}
	sort.Slice(tables, func(i, j int) bool {
		return tables[i].TotalTime > tables[j].TotalTime
	})
	if len(tables) > 10 {
		tables = tables[:10]
	}
	result.HotTables = tables

	// 清空聚合数据
	p.aggregates = make(map[string]*SQLStats)
	p.slowQueries = make([]*SQLQuery, 0)
	p.tableStats = make(map[string]*TableStats)

	return result
}

// generateFingerprint 生成 SQL 指纹
func (p *SQLProfiler) generateFingerprint(sql string) string {
	// 规范化 SQL
	fingerprint := sql

	// 替换字符串常量
	fingerprint = regexp.MustCompile(`'[^']*'`).ReplaceAllString(fingerprint, "'?'")
	fingerprint = regexp.MustCompile(`"[^"]*"`).ReplaceAllString(fingerprint, `"?"`)

	// 替换数字
	fingerprint = regexp.MustCompile(`\b\d+\b`).ReplaceAllString(fingerprint, "?")

	// 替换 IN 列表
	fingerprint = regexp.MustCompile(`\bin\s*\([^)]+\)`).ReplaceAllString(fingerprint, "IN (?)")

	// 统一空白字符
	fingerprint = regexp.MustCompile(`\s+`).ReplaceAllString(fingerprint, " ")
	fingerprint = strings.TrimSpace(fingerprint)

	// 转大写
	fingerprint = strings.ToUpper(fingerprint)

	return fingerprint
}

// generateQueryID 生成查询 ID
func (p *SQLProfiler) generateQueryID(query *SQLQuery) string {
	h := fnv.New64a()
	h.Write([]byte(query.Fingerprint))
	h.Write([]byte(query.Timestamp.String()))
	return fmt.Sprintf("%x", h.Sum64())
}

// parseOperation 解析操作类型
func (p *SQLProfiler) parseOperation(sql string) string {
	sqlUpper := strings.ToUpper(strings.TrimSpace(sql))
	
	if strings.HasPrefix(sqlUpper, "SELECT") {
		return "SELECT"
	} else if strings.HasPrefix(sqlUpper, "INSERT") {
		return "INSERT"
	} else if strings.HasPrefix(sqlUpper, "UPDATE") {
		return "UPDATE"
	} else if strings.HasPrefix(sqlUpper, "DELETE") {
		return "DELETE"
	} else if strings.HasPrefix(sqlUpper, "CREATE") {
		return "CREATE"
	} else if strings.HasPrefix(sqlUpper, "DROP") {
		return "DROP"
	} else if strings.HasPrefix(sqlUpper, "ALTER") {
		return "ALTER"
	}
	
	return "OTHER"
}

// parseTables 解析表名
func (p *SQLProfiler) parseTables(sql string) []string {
	tables := make([]string, 0)
	sqlUpper := strings.ToUpper(sql)

	// FROM 子句
	fromRegex := regexp.MustCompile(`\bFROM\s+(\w+)`)
	matches := fromRegex.FindAllStringSubmatch(sqlUpper, -1)
	for _, match := range matches {
		if len(match) > 1 {
			tables = append(tables, match[1])
		}
	}

	// JOIN 子句
	joinRegex := regexp.MustCompile(`\bJOIN\s+(\w+)`)
	matches = joinRegex.FindAllStringSubmatch(sqlUpper, -1)
	for _, match := range matches {
		if len(match) > 1 {
			tables = append(tables, match[1])
		}
	}

	// INTO 子句 (INSERT)
	intoRegex := regexp.MustCompile(`\bINTO\s+(\w+)`)
	matches = intoRegex.FindAllStringSubmatch(sqlUpper, -1)
	for _, match := range matches {
		if len(match) > 1 {
			tables = append(tables, match[1])
		}
	}

	// UPDATE 子句
	updateRegex := regexp.MustCompile(`\bUPDATE\s+(\w+)`)
	matches = updateRegex.FindAllStringSubmatch(sqlUpper, -1)
	for _, match := range matches {
		if len(match) > 1 {
			tables = append(tables, match[1])
		}
	}

	// 去重
	seen := make(map[string]bool)
	unique := make([]string, 0)
	for _, table := range tables {
		if !seen[table] {
			seen[table] = true
			unique = append(unique, table)
		}
	}

	return unique
}

// GetAggregateResult 获取聚合结果
func (p *SQLProfiler) GetAggregateResult() *SQLAggregateResult {
	return p.flushAggregates()
}

// GetSlowQueries 获取慢查询
func (p *SQLProfiler) GetSlowQueries(limit int) []*SQLQuery {
	p.mu.RLock()
	defer p.mu.RUnlock()

	if limit > len(p.slowQueries) {
		limit = len(p.slowQueries)
	}

	result := make([]*SQLQuery, limit)
	copy(result, p.slowQueries[:limit])
	return result
}

// GetStats 获取统计
func (p *SQLProfiler) GetStats() ProfilerStats {
	return ProfilerStats{
		TotalQueries:    atomic.LoadUint64(&p.stats.TotalQueries),
		SlowQueries:     atomic.LoadUint64(&p.stats.SlowQueries),
		ErrorQueries:    atomic.LoadUint64(&p.stats.ErrorQueries),
		AggregatedStats: atomic.LoadUint64(&p.stats.AggregatedStats),
	}
}

// Close 关闭剖析器
func (p *SQLProfiler) Close() error {
	p.cancel()
	close(p.queryCh)
	return nil
}
