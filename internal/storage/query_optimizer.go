/*
 * Copyright (c) 2025 Yunlong Liao. All rights reserved.
 */

package storage

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"sort"
	"strings"
	"sync"
	"time"

	"cloud-flow-agent/pkg/logger"
)

// QueryOptimizer 查询优化器
type QueryOptimizer struct {
	indexes      map[string]*IndexDefinition
	partitions   map[string]*PartitionConfig
	cache        *QueryCache
	stats        *QueryStats
	config       *QueryOptimizerConfig
	mu           sync.RWMutex
	stopCh       chan struct{}
}

// QueryOptimizerConfig 查询优化器配置
type QueryOptimizerConfig struct {
	Enabled           bool          `yaml:"enabled" json:"enabled"`
	MaxQueryTime      time.Duration `yaml:"max_query_time" json:"max_query_time"`
	CacheEnabled      bool          `yaml:"cache_enabled" json:"cache_enabled"`
	CacheTTL          time.Duration `yaml:"cache_ttl" json:"cache_ttl"`
	CacheMaxSize      int           `yaml:"cache_max_size" json:"cache_max_size"`
	AutoIndexEnabled  bool          `yaml:"auto_index_enabled" json:"auto_index_enabled"`
	PartitionEnabled  bool          `yaml:"partition_enabled" json:"partition_enabled"`
	MaxResultSize     int           `yaml:"max_result_size" json:"max_result_size"`
	QueryParallelism  int           `yaml:"query_parallelism" json:"query_parallelism"`
}

// DefaultQueryOptimizerConfig 默认配置
func DefaultQueryOptimizerConfig() *QueryOptimizerConfig {
	return &QueryOptimizerConfig{
		Enabled:          true,
		MaxQueryTime:     30 * time.Second,
		CacheEnabled:     true,
		CacheTTL:         5 * time.Minute,
		CacheMaxSize:     10000,
		AutoIndexEnabled: true,
		PartitionEnabled: true,
		MaxResultSize:    100000,
		QueryParallelism: 4,
	}
}

// IndexDefinition 索引定义
type IndexDefinition struct {
	ID          string    `json:"id"`
	TableName   string    `json:"table_name"`
	IndexName   string    `json:"index_name"`
	Columns     []string  `json:"columns"`
	IndexType   IndexType `json:"index_type"`
	Unique      bool      `json:"unique"`
	CreatedAt   time.Time `json:"created_at"`
	UsageCount  uint64    `json:"usage_count"`
	LastUsedAt  time.Time `json:"last_used_at"`
}

// IndexType 索引类型
type IndexType string

const (
	IndexTypeBTree   IndexType = "btree"
	IndexTypeHash    IndexType = "hash"
	IndexTypeBitmap  IndexType = "bitmap"
	IndexTypeLSM     IndexType = "lsm"
)

// PartitionConfig 分区配置
type PartitionConfig struct {
	TableName     string            `json:"table_name"`
	PartitionKey  string            `json:"partition_key"`
	PartitionType PartitionType     `json:"partition_type"`
	Partitions    []PartitionInfo   `json:"partitions"`
	CreatedAt     time.Time         `json:"created_at"`
}

// PartitionType 分区类型
type PartitionType string

const (
	PartitionTypeRange  PartitionType = "range"
	PartitionTypeHash   PartitionType = "hash"
	PartitionTypeList   PartitionType = "list"
	PartitionTypeTime   PartitionType = "time"
)

// PartitionInfo 分区信息
type PartitionInfo struct {
	ID        string    `json:"id"`
	Name      string    `json:"name"`
	Condition string    `json:"condition"`
	StartKey  string    `json:"start_key"`
	EndKey    string    `json:"end_key"`
	RowCount  uint64    `json:"row_count"`
	SizeBytes uint64    `json:"size_bytes"`
}

// QueryCache 查询缓存
type QueryCache struct {
	items    map[string]*CacheEntry
	maxSize  int
	ttl      time.Duration
	mu       sync.RWMutex
	hits     uint64
	misses   uint64
	evictions uint64
}

// CacheEntry 缓存条目
type CacheEntry struct {
	Key        string      `json:"key"`
	Value      interface{} `json:"value"`
	CreatedAt  time.Time   `json:"created_at"`
	ExpiresAt  time.Time   `json:"expires_at"`
	AccessCount uint64     `json:"access_count"`
	Size       int         `json:"size"`
}

// QueryStats 查询统计
type QueryStats struct {
	TotalQueries      uint64        `json:"total_queries"`
	CachedQueries     uint64        `json:"cached_queries"`
	SlowQueries       uint64        `json:"slow_queries"`
	TimeoutQueries    uint64        `json:"timeout_queries"`
	AvgQueryTime      time.Duration `json:"avg_query_time"`
	MaxQueryTime      time.Duration `json:"max_query_time"`
	IndexUsageCount   uint64        `json:"index_usage_count"`
	PartitionPrunes   uint64        `json:"partition_prunes"`
	mu                sync.RWMutex
}

// QueryRequest 查询请求
type QueryRequest struct {
	TableName   string                 `json:"table_name"`
	Columns     []string               `json:"columns"`
	Conditions  []QueryCondition       `json:"conditions"`
	OrderBy     []OrderByClause        `json:"order_by"`
	Limit       int                    `json:"limit"`
	Offset      int                    `json:"offset"`
	Aggregation []AggregationClause    `json:"aggregation"`
	GroupBy     []string               `json:"group_by"`
}

// QueryCondition 查询条件
type QueryCondition struct {
	Column   string      `json:"column"`
	Operator string      `json:"operator"`
	Value    interface{} `json:"value"`
	Logic    string      `json:"logic"` // AND, OR
}

// OrderByClause 排序子句
type OrderByClause struct {
	Column string `json:"column"`
	Desc   bool   `json:"desc"`
}

// AggregationClause 聚合子句
type AggregationClause struct {
	Function string `json:"function"` // COUNT, SUM, AVG, MAX, MIN
	Column   string `json:"column"`
	Alias    string `json:"alias"`
}

// QueryResult 查询结果
type QueryResult struct {
	Columns    []string        `json:"columns"`
	Rows       []map[string]interface{} `json:"rows"`
	RowCount   int             `json:"row_count"`
	TotalCount int64           `json:"total_count"`
	QueryTime  time.Duration   `json:"query_time"`
	FromCache  bool            `json:"from_cache"`
	IndexUsed  string          `json:"index_used,omitempty"`
	Partitions []string        `json:"partitions,omitempty"`
}

// NewQueryOptimizer 创建查询优化器
func NewQueryOptimizer(config *QueryOptimizerConfig) *QueryOptimizer {
	if config == nil {
		config = DefaultQueryOptimizerConfig()
	}

	o := &QueryOptimizer{
		indexes:    make(map[string]*IndexDefinition),
		partitions: make(map[string]*PartitionConfig),
		cache: &QueryCache{
			items:   make(map[string]*CacheEntry),
			maxSize: config.CacheMaxSize,
			ttl:     config.CacheTTL,
		},
		stats:  &QueryStats{},
		config: config,
		stopCh: make(chan struct{}),
	}

	// 启动缓存清理协程
	go o.cacheCleanupLoop()

	return o
}

// Stop 停止查询优化器
func (o *QueryOptimizer) Stop() {
	close(o.stopCh)
}

// ExecuteQuery 执行优化查询
func (o *QueryOptimizer) ExecuteQuery(ctx context.Context, req *QueryRequest) (*QueryResult, error) {
	startTime := time.Now()

	// 生成查询缓存键
	cacheKey := o.generateCacheKey(req)

	// 尝试从缓存获取
	if o.config.CacheEnabled {
		if cached := o.cache.Get(cacheKey); cached != nil {
			if result, ok := cached.(*QueryResult); ok {
				result.FromCache = true
				o.stats.mu.Lock()
				o.stats.CachedQueries++
				o.stats.mu.Unlock()
				return result, nil
			}
		}
	}

	// 设置查询超时
	ctx, cancel := context.WithTimeout(ctx, o.config.MaxQueryTime)
	defer cancel()

	// 执行查询（带超时控制）
	resultCh := make(chan *QueryResult, 1)
	errCh := make(chan error, 1)

	go func() {
		result, err := o.executeOptimizedQuery(req)
		if err != nil {
			errCh <- err
			return
		}
		resultCh <- result
	}()

	select {
	case <-ctx.Done():
		o.stats.mu.Lock()
		o.stats.TimeoutQueries++
		o.stats.mu.Unlock()
		return nil, fmt.Errorf("query timeout after %v", o.config.MaxQueryTime)
	case err := <-errCh:
		return nil, err
	case result := <-resultCh:
		result.QueryTime = time.Since(startTime)

		// 更新统计
		o.stats.mu.Lock()
		o.stats.TotalQueries++
		o.updateAvgQueryTime(result.QueryTime)
		if result.QueryTime > o.stats.MaxQueryTime {
			o.stats.MaxQueryTime = result.QueryTime
		}
		o.stats.mu.Unlock()

		// 慢查询检测
		if result.QueryTime > 5*time.Second {
			o.stats.mu.Lock()
			o.stats.SlowQueries++
			o.stats.mu.Unlock()
			logger.Warnf("Slow query detected: %v, table: %s", result.QueryTime, req.TableName)
		}

		// 缓存结果
		if o.config.CacheEnabled && result.RowCount < 1000 {
			o.cache.Set(cacheKey, result)
		}

		return result, nil
	}
}

// executeOptimizedQuery 执行优化后的查询
func (o *QueryOptimizer) executeOptimizedQuery(req *QueryRequest) (*QueryResult, error) {
	// 1. 分析查询条件，选择最优索引
	index := o.selectBestIndex(req)

	// 2. 确定需要扫描的分区
	partitions := o.selectPartitions(req)

	// 3. 构建优化后的查询计划
	plan := o.buildQueryPlan(req, index, partitions)

	// 4. 执行查询（这里简化处理，实际应调用底层存储）
	result := &QueryResult{
		Columns:    req.Columns,
		Rows:       make([]map[string]interface{}, 0),
		RowCount:   0,
		TotalCount: 0,
		IndexUsed:  "",
		Partitions: partitions,
	}

	if index != nil {
		result.IndexUsed = index.IndexName
		o.stats.mu.Lock()
		o.stats.IndexUsageCount++
		o.stats.mu.Unlock()
	}

	// 模拟查询执行
	logger.Debugf("Executing query plan: %+v", plan)

	return result, nil
}

// selectBestIndex 选择最优索引
func (o *QueryOptimizer) selectBestIndex(req *QueryRequest) *IndexDefinition {
	o.mu.RLock()
	defer o.mu.RUnlock()

	if !o.config.AutoIndexEnabled {
		return nil
	}

	// 提取查询条件中的列
	conditionCols := make(map[string]bool)
	for _, cond := range req.Conditions {
		conditionCols[cond.Column] = true
	}

	// 查找匹配的索引
	var bestIndex *IndexDefinition
	bestScore := 0

	for _, idx := range o.indexes {
		if idx.TableName != req.TableName {
			continue
		}

		// 计算索引匹配度
		score := 0
		matchedCols := 0
		for _, col := range idx.Columns {
			if conditionCols[col] {
				matchedCols++
				score += 10 - matchedCols // 前缀匹配得分更高
			}
		}

		// 考虑排序列
		for _, order := range req.OrderBy {
			for i, col := range idx.Columns {
				if col == order.Column && i == 0 {
					score += 5 // 索引有序，避免排序
				}
			}
		}

		if score > bestScore {
			bestScore = score
			bestIndex = idx
		}
	}

	return bestIndex
}

// selectPartitions 选择分区
func (o *QueryOptimizer) selectPartitions(req *QueryRequest) []string {
	o.mu.RLock()
	defer o.mu.RUnlock()

	if !o.config.PartitionEnabled {
		return nil
	}

	partitionConfig, exists := o.partitions[req.TableName]
	if !exists {
		return nil
	}

	// 根据查询条件进行分区裁剪
	selectedPartitions := make([]string, 0)

	for _, cond := range req.Conditions {
		if cond.Column == partitionConfig.PartitionKey {
			for _, part := range partitionConfig.Partitions {
				if o.conditionMatchesPartition(cond, part) {
					selectedPartitions = append(selectedPartitions, part.Name)
				}
			}
		}
	}

	if len(selectedPartitions) > 0 {
		o.stats.mu.Lock()
		o.stats.PartitionPrunes++
		o.stats.mu.Unlock()
	}

	return selectedPartitions
}

// conditionMatchesPartition 检查条件是否匹配分区
func (o *QueryOptimizer) conditionMatchesPartition(cond QueryCondition, part PartitionInfo) bool {
	switch cond.Operator {
	case "=":
		valueStr := fmt.Sprintf("%v", cond.Value)
		return valueStr >= part.StartKey && valueStr <= part.EndKey
	case ">=", ">":
		valueStr := fmt.Sprintf("%v", cond.Value)
		return valueStr <= part.EndKey
	case "<=", "<":
		valueStr := fmt.Sprintf("%v", cond.Value)
		return valueStr >= part.StartKey
	case "BETWEEN":
		// 处理范围查询
		return true
	default:
		return false
	}
}

// buildQueryPlan 构建查询计划
func (o *QueryOptimizer) buildQueryPlan(req *QueryRequest, index *IndexDefinition, partitions []string) *QueryPlan {
	return &QueryPlan{
		TableName:   req.TableName,
		Index:       index,
		Partitions:  partitions,
		Conditions:  req.Conditions,
		OrderBy:     req.OrderBy,
		Limit:       req.Limit,
		Offset:      req.Offset,
		UseIndex:    index != nil,
		UseCache:    o.config.CacheEnabled,
	}
}

// QueryPlan 查询计划
type QueryPlan struct {
	TableName  string             `json:"table_name"`
	Index      *IndexDefinition   `json:"index,omitempty"`
	Partitions []string           `json:"partitions,omitempty"`
	Conditions []QueryCondition   `json:"conditions"`
	OrderBy    []OrderByClause    `json:"order_by"`
	Limit      int                `json:"limit"`
	Offset     int                `json:"offset"`
	UseIndex   bool               `json:"use_index"`
	UseCache   bool               `json:"use_cache"`
}

// generateCacheKey 生成缓存键
func (o *QueryOptimizer) generateCacheKey(req *QueryRequest) string {
	// 规范化查询请求
	normalized := &QueryRequest{
		TableName:  req.TableName,
		Columns:    normalizeColumns(req.Columns),
		Conditions: normalizeConditions(req.Conditions),
		OrderBy:    req.OrderBy,
		Limit:      req.Limit,
		Offset:     req.Offset,
	}

	data, _ := json.Marshal(normalized)
	hash := sha256.Sum256(data)
	return hex.EncodeToString(hash[:])
}

// normalizeColumns 规范化列名
func normalizeColumns(cols []string) []string {
	sorted := make([]string, len(cols))
	copy(sorted, cols)
	sort.Strings(sorted)
	return sorted
}

// normalizeConditions 规范化条件
func normalizeConditions(conds []QueryCondition) []QueryCondition {
	// 按列名排序
	sorted := make([]QueryCondition, len(conds))
	copy(sorted, conds)
	sort.Slice(sorted, func(i, j int) bool {
		return sorted[i].Column < sorted[j].Column
	})
	return sorted
}

// CreateIndex 创建索引
func (o *QueryOptimizer) CreateIndex(tableName, indexName string, columns []string, indexType IndexType, unique bool) (*IndexDefinition, error) {
	o.mu.Lock()
	defer o.mu.Unlock()

	indexID := fmt.Sprintf("%s.%s", tableName, indexName)
	if _, exists := o.indexes[indexID]; exists {
		return nil, fmt.Errorf("index already exists: %s", indexID)
	}

	index := &IndexDefinition{
		ID:         indexID,
		TableName:  tableName,
		IndexName:  indexName,
		Columns:    columns,
		IndexType:  indexType,
		Unique:     unique,
		CreatedAt:  time.Now(),
	}

	o.indexes[indexID] = index
	logger.Infof("Created index: %s on %s(%s)", indexName, tableName, strings.Join(columns, ", "))
	return index, nil
}

// DropIndex 删除索引
func (o *QueryOptimizer) DropIndex(tableName, indexName string) error {
	o.mu.Lock()
	defer o.mu.Unlock()

	indexID := fmt.Sprintf("%s.%s", tableName, indexName)
	if _, exists := o.indexes[indexID]; !exists {
		return fmt.Errorf("index not found: %s", indexID)
	}

	delete(o.indexes, indexID)
	logger.Infof("Dropped index: %s", indexID)
	return nil
}

// GetIndexes 获取表的所有索引
func (o *QueryOptimizer) GetIndexes(tableName string) []*IndexDefinition {
	o.mu.RLock()
	defer o.mu.RUnlock()

	indexes := make([]*IndexDefinition, 0)
	for _, idx := range o.indexes {
		if idx.TableName == tableName {
			indexes = append(indexes, idx)
		}
	}
	return indexes
}

// CreatePartition 创建分区
func (o *QueryOptimizer) CreatePartition(tableName string, config *PartitionConfig) error {
	o.mu.Lock()
	defer o.mu.Unlock()

	if _, exists := o.partitions[tableName]; exists {
		return fmt.Errorf("partition already exists for table: %s", tableName)
	}

	config.CreatedAt = time.Now()
	o.partitions[tableName] = config
	logger.Infof("Created partition for table: %s, key: %s", tableName, config.PartitionKey)
	return nil
}

// GetPartition 获取分区配置
func (o *QueryOptimizer) GetPartition(tableName string) *PartitionConfig {
	o.mu.RLock()
	defer o.mu.RUnlock()
	return o.partitions[tableName]
}

// Get 从缓存获取
func (c *QueryCache) Get(key string) interface{} {
	c.mu.RLock()
	defer c.mu.RUnlock()

	entry, exists := c.items[key]
	if !exists {
		c.misses++
		return nil
	}

	if time.Now().After(entry.ExpiresAt) {
		c.misses++
		return nil
	}

	entry.AccessCount++
	c.hits++
	return entry.Value
}

// Set 设置缓存
func (c *QueryCache) Set(key string, value interface{}) {
	c.mu.Lock()
	defer c.mu.Unlock()

	// 检查是否需要淘汰
	if len(c.items) >= c.maxSize {
		c.evict()
	}

	c.items[key] = &CacheEntry{
		Key:         key,
		Value:       value,
		CreatedAt:   time.Now(),
		ExpiresAt:   time.Now().Add(c.ttl),
		AccessCount: 1,
		Size:        estimateSize(value),
	}
}

// evict 淘汰缓存
func (c *QueryCache) evict() {
	// LRU淘汰策略
	var oldestKey string
	var oldestTime time.Time

	for key, entry := range c.items {
		if oldestKey == "" || entry.LastUsed().Before(oldestTime) {
			oldestKey = key
			oldestTime = entry.LastUsed()
		}
	}

	if oldestKey != "" {
		delete(c.items, oldestKey)
		c.evictions++
	}
}

// LastUsed 获取最后使用时间
func (e *CacheEntry) LastUsed() time.Time {
	// 简化：使用创建时间
	return e.CreatedAt
}

// estimateSize 估算对象大小
func estimateSize(v interface{}) int {
	data, _ := json.Marshal(v)
	return len(data)
}

// cacheCleanupLoop 缓存清理循环
func (o *QueryOptimizer) cacheCleanupLoop() {
	ticker := time.NewTicker(1 * time.Minute)
	defer ticker.Stop()

	for {
		select {
		case <-o.stopCh:
			return
		case <-ticker.C:
			o.cache.Cleanup()
		}
	}
}

// Cleanup 清理过期缓存
func (c *QueryCache) Cleanup() {
	c.mu.Lock()
	defer c.mu.Unlock()

	now := time.Now()
	for key, entry := range c.items {
		if now.After(entry.ExpiresAt) {
			delete(c.items, key)
		}
	}
}

// GetStats 获取统计信息
func (o *QueryOptimizer) GetStats() QueryStats {
	o.stats.mu.RLock()
	defer o.stats.mu.RUnlock()
	return *o.stats
}

// GetCacheStats 获取缓存统计
func (o *QueryOptimizer) GetCacheStats() map[string]interface{} {
	o.cache.mu.RLock()
	defer o.cache.mu.RUnlock()

	total := o.cache.hits + o.cache.misses
	hitRate := float64(0)
	if total > 0 {
		hitRate = float64(o.cache.hits) / float64(total) * 100
	}

	return map[string]interface{}{
		"size":       len(o.cache.items),
		"max_size":   o.cache.maxSize,
		"hits":       o.cache.hits,
		"misses":     o.cache.misses,
		"hit_rate":   hitRate,
		"evictions":  o.cache.evictions,
	}
}

// updateAvgQueryTime 更新平均查询时间
func (s *QueryStats) updateAvgQueryTime(duration time.Duration) {
	if s.TotalQueries == 1 {
		s.AvgQueryTime = duration
	} else {
		s.AvgQueryTime = (s.AvgQueryTime*time.Duration(s.TotalQueries-1) + duration) / time.Duration(s.TotalQueries)
	}
}

// SuggestIndexes 索引建议
func (o *QueryOptimizer) SuggestIndexes(tableName string, queryHistory []QueryRequest) []*IndexSuggestion {
	columnFreq := make(map[string]int)
	columnCombo := make(map[string]int)

	for _, req := range queryHistory {
		if req.TableName != tableName {
			continue
		}

		// 统计单列频率
		for _, cond := range req.Conditions {
			columnFreq[cond.Column]++
		}

		// 统计组合频率
		if len(req.Conditions) > 1 {
			cols := make([]string, 0, len(req.Conditions))
			for _, cond := range req.Conditions {
				cols = append(cols, cond.Column)
			}
			sort.Strings(cols)
			combo := strings.Join(cols, ",")
			columnCombo[combo]++
		}
	}

	// 生成建议
	suggestions := make([]*IndexSuggestion, 0)

	// 单列索引建议
	for col, freq := range columnFreq {
		if freq >= 10 {
			suggestions = append(suggestions, &IndexSuggestion{
				TableName: tableName,
				Columns:   []string{col},
				Reason:    fmt.Sprintf("Column '%s' used in %d queries", col, freq),
				Priority:  "medium",
			})
		}
	}

	// 组合索引建议
	for combo, freq := range columnCombo {
		if freq >= 5 {
			cols := strings.Split(combo, ",")
			suggestions = append(suggestions, &IndexSuggestion{
				TableName: tableName,
				Columns:   cols,
				Reason:    fmt.Sprintf("Column combination used in %d queries", freq),
				Priority:  "high",
			})
		}
	}

	return suggestions
}

// IndexSuggestion 索引建议
type IndexSuggestion struct {
	TableName string   `json:"table_name"`
	Columns   []string `json:"columns"`
	Reason    string   `json:"reason"`
	Priority  string   `json:"priority"`
}

// AnalyzeQuery 分析查询性能
func (o *QueryOptimizer) AnalyzeQuery(req *QueryRequest) *QueryAnalysis {
	analysis := &QueryAnalysis{
		Query:       req,
		Suggestions: make([]string, 0),
		Warnings:    make([]string, 0),
	}

	// 检查是否有索引可用
	index := o.selectBestIndex(req)
	if index == nil && len(req.Conditions) > 0 {
		analysis.Warnings = append(analysis.Warnings, "No suitable index found for query conditions")
		analysis.Suggestions = append(analysis.Suggestions, 
			fmt.Sprintf("Consider creating index on columns: %v", getConditionColumns(req.Conditions)))
	} else if index != nil {
		analysis.IndexUsed = index.IndexName
	}

	// 检查分区裁剪
	partitions := o.selectPartitions(req)
	if len(partitions) == 0 && o.config.PartitionEnabled {
		if _, hasPartition := o.partitions[req.TableName]; hasPartition {
			analysis.Warnings = append(analysis.Warnings, "Partition pruning not effective, consider adding partition key to WHERE clause")
		}
	} else {
		analysis.PartitionsScanned = partitions
	}

	// 检查大结果集
	if req.Limit == 0 || req.Limit > o.config.MaxResultSize {
		analysis.Warnings = append(analysis.Warnings, 
			fmt.Sprintf("Query may return large result set, consider adding LIMIT clause (max: %d)", o.config.MaxResultSize))
	}

	// 检查排序
	if len(req.OrderBy) > 0 && index == nil {
		analysis.Warnings = append(analysis.Warnings, "Query requires sorting without index, may cause performance issue")
	}

	return analysis
}

// QueryAnalysis 查询分析结果
type QueryAnalysis struct {
	Query             *QueryRequest `json:"query"`
	IndexUsed         string        `json:"index_used,omitempty"`
	PartitionsScanned []string      `json:"partitions_scanned,omitempty"`
	Suggestions       []string      `json:"suggestions"`
	Warnings          []string      `json:"warnings"`
}

// getConditionColumns 获取条件列
func getConditionColumns(conds []QueryCondition) []string {
	cols := make([]string, 0, len(conds))
	for _, cond := range conds {
		cols = append(cols, cond.Column)
	}
	return cols
}

// PreloadCache 预加载缓存
func (o *QueryOptimizer) PreloadCache(req *QueryRequest, result *QueryResult) {
	if !o.config.CacheEnabled {
		return
	}

	cacheKey := o.generateCacheKey(req)
	o.cache.Set(cacheKey, result)
	logger.Debugf("Preloaded cache for query: %s", cacheKey)
}

// InvalidateCache 使缓存失效
func (o *QueryOptimizer) InvalidateCache(tableName string) {
	o.cache.mu.Lock()
	defer o.cache.mu.Unlock()

	// 删除与表相关的缓存
	for key := range o.cache.items {
		// 简化：检查key是否包含表名
		if strings.Contains(key, tableName) {
			delete(o.cache.items, key)
		}
	}

	logger.Infof("Invalidated cache for table: %s", tableName)
}
