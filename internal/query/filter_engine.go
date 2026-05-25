// Package query 提供数据检索功能
// 支持网络/应用/SQL多条件筛选和历史趋势图
package query

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
	"sync"
	"time"
)

// FilterOperator 筛选操作符
type FilterOperator string

const (
	OpEqual      FilterOperator = "="
	OpNotEqual   FilterOperator = "!="
	OpGreaterThan FilterOperator = ">"
	OpLessThan    FilterOperator = "<"
	OpGreaterEqual FilterOperator = ">="
	OpLessEqual    FilterOperator = "<="
	OpLike        FilterOperator = "LIKE"
	OpIn          FilterOperator = "IN"
	OpNotIn       FilterOperator = "NOT IN"
	OpBetween     FilterOperator = "BETWEEN"
	OpIsNull      FilterOperator = "IS NULL"
	OpIsNotNull   FilterOperator = "IS NOT NULL"
	OpContains    FilterOperator = "CONTAINS"
	OpRegex       FilterOperator = "REGEX"
)

// LogicOperator 逻辑操作符
type LogicOperator string

const (
	LogicAnd LogicOperator = "AND"
	LogicOr  LogicOperator = "OR"
	LogicNot LogicOperator = "NOT"
)

// FilterCondition 筛选条件
type FilterCondition struct {
	Field    string         `json:"field"`              // 字段名
	Operator FilterOperator `json:"operator"`           // 操作符
	Value    interface{}    `json:"value"`              // 值
	Values   []interface{}  `json:"values,omitempty"`   // 多值（用于IN/BETWEEN）
}

// FilterGroup 筛选条件组
type FilterGroup struct {
	Logic     LogicOperator     `json:"logic"`               // 逻辑操作符
	Conditions []*FilterCondition `json:"conditions"`         // 条件列表
	Groups     []*FilterGroup     `json:"groups,omitempty"`   // 嵌套条件组
}

// SortOrder 排序
type SortOrder struct {
	Field string `json:"field"` // 排序字段
	Desc  bool   `json:"desc"`  // 是否降序
}

// Pagination 分页
type Pagination struct {
	Page     int `json:"page"`      // 页码（从1开始）
	PageSize int `json:"page_size"` // 每页大小
}

// QueryRequest 查询请求
type QueryRequest struct {
	// 数据源
	SourceType string   `json:"source_type"` // network/application/sql/system
	AssetIDs   []string `json:"asset_ids"`   // 资产ID列表
	AssetTypes []string `json:"asset_types"` // 资产类型列表

	// 时间范围
	StartTime time.Time `json:"start_time"`
	EndTime   time.Time `json:"end_time"`

	// 筛选条件
	Filter *FilterGroup `json:"filter"`

	// 聚合
	GroupBy    []string `json:"group_by"`    // 分组字段
	Aggregates []Aggregate `json:"aggregates"` // 聚合函数

	// 排序和分页
	OrderBy []SortOrder `json:"order_by"`
	Pagination *Pagination `json:"pagination"`

	// 结果配置
	Fields    []string `json:"fields"`    // 返回字段
	Distinct  bool     `json:"distinct"`  // 去重
	IncludeTotal bool  `json:"include_total"` // 包含总数
}

// Aggregate 聚合函数
type Aggregate struct {
	Field     string       `json:"field"`     // 字段
	Function  AggFunction  `json:"function"`  // 函数
	Alias     string       `json:"alias"`     // 别名
}

// AggFunction 聚合函数类型
type AggFunction string

const (
	AggSum   AggFunction = "SUM"
	AggAvg   AggFunction = "AVG"
	AggMax   AggFunction = "MAX"
	AggMin   AggFunction = "MIN"
	AggCount AggFunction = "COUNT"
	AggStdDev AggFunction = "STDDEV"
	AggVariance AggFunction = "VARIANCE"
)

// QueryResult 查询结果
type QueryResult struct {
	Data       []map[string]interface{} `json:"data"`
	Total      int64                    `json:"total"`
	Page       int                      `json:"page"`
	PageSize   int                      `json:"page_size"`
	Fields     []string                 `json:"fields"`
	ExecTime   time.Duration            `json:"exec_time"`
	SQL        string                   `json:"sql,omitempty"`
}

// NetworkFilter 网络筛选条件
type NetworkFilter struct {
	// 基础筛选
	SourceIP      []string `json:"source_ip"`
	DestIP        []string `json:"dest_ip"`
	SourcePort    []uint16 `json:"source_port"`
	DestPort      []uint16 `json:"dest_port"`
	Protocol      []string `json:"protocol"`      // TCP/UDP/HTTP

	// 性能筛选
	BytesSentMin    *uint64 `json:"bytes_sent_min"`
	BytesSentMax    *uint64 `json:"bytes_sent_max"`
	BytesRecvMin    *uint64 `json:"bytes_recv_min"`
	BytesRecvMax    *uint64 `json:"bytes_recv_max"`
	LatencyMin      *float64 `json:"latency_min"`
	LatencyMax      *float64 `json:"latency_max"`
	ConnectionsMin  *int `json:"connections_min"`
	ConnectionsMax  *int `json:"connections_max"`
	RetransmitMin   *uint64 `json:"retransmit_min"`
	RetransmitMax   *uint64 `json:"retransmit_max"`

	// 状态筛选
	TCPStates []string `json:"tcp_states"` // ESTABLISHED/TIME_WAIT/etc.
	Errors    *bool    `json:"errors"`     // 是否有错误
}

// ApplicationFilter 应用筛选条件
type ApplicationFilter struct {
	// 进程筛选
	ProcessName []string `json:"process_name"`
	ProcessPID  []uint32 `json:"process_pid"`
	ContainerID []string `json:"container_id"`
	PodName     []string `json:"pod_name"`
	Namespace   []string `json:"namespace"`

	// 性能筛选
	CPUUsageMin    *float64 `json:"cpu_usage_min"`
	CPUUsageMax    *float64 `json:"cpu_usage_max"`
	MemoryUsageMin *float64 `json:"memory_usage_min"`
	MemoryUsageMax *float64 `json:"memory_usage_max"`
	ThreadsMin     *int `json:"threads_min"`
	ThreadsMax     *int `json:"threads_max"`
	OpenFilesMin   *int `json:"open_files_min"`
	OpenFilesMax   *int `json:"open_files_max"`

	// 响应时间筛选
	ResponseTimeMin *float64 `json:"response_time_min"`
	ResponseTimeMax *float64 `json:"response_time_max"`
	ErrorRateMin    *float64 `json:"error_rate_min"`
	ErrorRateMax    *float64 `json:"error_rate_max"`
	ThroughputMin   *float64 `json:"throughput_min"`
	ThroughputMax   *float64 `json:"throughput_max"`
}

// SQLFilter SQL筛选条件
type SQLFilter struct {
	// SQL基础筛选
	Database    []string `json:"database"`
	Table       []string `json:"table"`
	Operation   []string `json:"operation"`   // SELECT/INSERT/UPDATE/DELETE
	QueryPattern string  `json:"query_pattern"` // SQL模式匹配

	// 性能筛选
	ExecTimeMin    *float64 `json:"exec_time_min"`    // 执行时间(ms)
	ExecTimeMax    *float64 `json:"exec_time_max"`
	RowsReadMin    *int64   `json:"rows_read_min"`
	RowsReadMax    *int64   `json:"rows_read_max"`
	RowsWrittenMin *int64   `json:"rows_written_min"`
	RowsWrittenMax *int64   `json:"rows_written_max"`

	// 错误筛选
	HasError   *bool    `json:"has_error"`
	ErrorCode  []string `json:"error_code"`
	HasLock    *bool    `json:"has_lock"`
	LockWaitMin *float64 `json:"lock_wait_min"` // 锁等待时间(ms)
	LockWaitMax *float64 `json:"lock_wait_max"`
}

// FilterEngine 筛选引擎
type FilterEngine struct {
	db         *sql.DB
	driver     string
	mu         sync.RWMutex
}

// FilterEngineConfig 筛选引擎配置
type FilterEngineConfig struct {
	Driver          string `yaml:"driver" json:"driver"`
	DSN             string `yaml:"dsn" json:"dsn"`
	MaxConnections  int    `yaml:"max_connections" json:"max_connections"`
	QueryTimeout    int    `yaml:"query_timeout" json:"query_timeout"`
	EnableCache     bool   `yaml:"enable_cache" json:"enable_cache"`
	CacheTTL        int    `yaml:"cache_ttl" json:"cache_ttl"`
}

// NewFilterEngine 创建筛选引擎
func NewFilterEngine(config *FilterEngineConfig) (*FilterEngine, error) {
	db, err := sql.Open(config.Driver, config.DSN)
	if err != nil {
		return nil, err
	}

	db.SetMaxOpenConns(config.MaxConnections)
	db.SetMaxIdleConns(config.MaxConnections / 2)
	db.SetConnMaxLifetime(time.Hour)

	return &FilterEngine{
		db:     db,
		driver: config.Driver,
	}, nil
}

// Close 关闭筛选引擎
func (e *FilterEngine) Close() error {
	if e.db != nil {
		return e.db.Close()
	}
	return nil
}

// QueryNetwork 查询网络数据
func (e *FilterEngine) QueryNetwork(ctx context.Context, filter *NetworkFilter, start, end time.Time, pagination *Pagination) (*QueryResult, error) {
	query := &QueryRequest{
		SourceType: "network",
		StartTime:  start,
		EndTime:    end,
		Pagination: pagination,
	}

	// 构建筛选条件
	query.Filter = e.buildNetworkFilter(filter)

	return e.Execute(ctx, query)
}

// QueryApplication 查询应用数据
func (e *FilterEngine) QueryApplication(ctx context.Context, filter *ApplicationFilter, start, end time.Time, pagination *Pagination) (*QueryResult, error) {
	query := &QueryRequest{
		SourceType: "application",
		StartTime:  start,
		EndTime:    end,
		Pagination: pagination,
	}

	query.Filter = e.buildApplicationFilter(filter)

	return e.Execute(ctx, query)
}

// QuerySQL 查询SQL数据
func (e *FilterEngine) QuerySQL(ctx context.Context, filter *SQLFilter, start, end time.Time, pagination *Pagination) (*QueryResult, error) {
	query := &QueryRequest{
		SourceType: "sql",
		StartTime:  start,
		EndTime:    end,
		Pagination: pagination,
	}

	query.Filter = e.buildSQLFilter(filter)

	return e.Execute(ctx, query)
}

// buildNetworkFilter 构建网络筛选条件
func (e *FilterEngine) buildNetworkFilter(filter *NetworkFilter) *FilterGroup {
	if filter == nil {
		return nil
	}

	group := &FilterGroup{
		Logic:      LogicAnd,
		Conditions: make([]*FilterCondition, 0),
	}

	// IP筛选
	if len(filter.SourceIP) > 0 {
		group.Conditions = append(group.Conditions, &FilterCondition{
			Field:    "source_ip",
			Operator: OpIn,
			Values:   toInterfaceSlice(filter.SourceIP),
		})
	}

	if len(filter.DestIP) > 0 {
		group.Conditions = append(group.Conditions, &FilterCondition{
			Field:    "dest_ip",
			Operator: OpIn,
			Values:   toInterfaceSlice(filter.DestIP),
		})
	}

	// 端口筛选
	if len(filter.SourcePort) > 0 {
		group.Conditions = append(group.Conditions, &FilterCondition{
			Field:    "source_port",
			Operator: OpIn,
			Values:   toInterfaceSliceUint16(filter.SourcePort),
		})
	}

	if len(filter.DestPort) > 0 {
		group.Conditions = append(group.Conditions, &FilterCondition{
			Field:    "dest_port",
			Operator: OpIn,
			Values:   toInterfaceSliceUint16(filter.DestPort),
		})
	}

	// 协议筛选
	if len(filter.Protocol) > 0 {
		group.Conditions = append(group.Conditions, &FilterCondition{
			Field:    "protocol",
			Operator: OpIn,
			Values:   toInterfaceSlice(filter.Protocol),
		})
	}

	// 性能范围筛选
	if filter.BytesSentMin != nil {
		group.Conditions = append(group.Conditions, &FilterCondition{
			Field:    "bytes_sent",
			Operator: OpGreaterEqual,
			Value:    *filter.BytesSentMin,
		})
	}

	if filter.BytesSentMax != nil {
		group.Conditions = append(group.Conditions, &FilterCondition{
			Field:    "bytes_sent",
			Operator: OpLessEqual,
			Value:    *filter.BytesSentMax,
		})
	}

	if filter.LatencyMin != nil {
		group.Conditions = append(group.Conditions, &FilterCondition{
			Field:    "latency_ms",
			Operator: OpGreaterEqual,
			Value:    *filter.LatencyMin,
		})
	}

	if filter.LatencyMax != nil {
		group.Conditions = append(group.Conditions, &FilterCondition{
			Field:    "latency_ms",
			Operator: OpLessEqual,
			Value:    *filter.LatencyMax,
		})
	}

	// TCP状态筛选
	if len(filter.TCPStates) > 0 {
		group.Conditions = append(group.Conditions, &FilterCondition{
			Field:    "tcp_state",
			Operator: OpIn,
			Values:   toInterfaceSlice(filter.TCPStates),
		})
	}

	return group
}

// buildApplicationFilter 构建应用筛选条件
func (e *FilterEngine) buildApplicationFilter(filter *ApplicationFilter) *FilterGroup {
	if filter == nil {
		return nil
	}

	group := &FilterGroup{
		Logic:      LogicAnd,
		Conditions: make([]*FilterCondition, 0),
	}

	// 进程筛选
	if len(filter.ProcessName) > 0 {
		group.Conditions = append(group.Conditions, &FilterCondition{
			Field:    "process_name",
			Operator: OpIn,
			Values:   toInterfaceSlice(filter.ProcessName),
		})
	}

	if len(filter.ProcessPID) > 0 {
		group.Conditions = append(group.Conditions, &FilterCondition{
			Field:    "pid",
			Operator: OpIn,
			Values:   toInterfaceSliceUint32(filter.ProcessPID),
		})
	}

	// 容器/Pod筛选
	if len(filter.ContainerID) > 0 {
		group.Conditions = append(group.Conditions, &FilterCondition{
			Field:    "container_id",
			Operator: OpIn,
			Values:   toInterfaceSlice(filter.ContainerID),
		})
	}

	if len(filter.PodName) > 0 {
		group.Conditions = append(group.Conditions, &FilterCondition{
			Field:    "pod_name",
			Operator: OpIn,
			Values:   toInterfaceSlice(filter.PodName),
		})
	}

	if len(filter.Namespace) > 0 {
		group.Conditions = append(group.Conditions, &FilterCondition{
			Field:    "namespace",
			Operator: OpIn,
			Values:   toInterfaceSlice(filter.Namespace),
		})
	}

	// CPU使用率筛选
	if filter.CPUUsageMin != nil {
		group.Conditions = append(group.Conditions, &FilterCondition{
			Field:    "cpu_usage",
			Operator: OpGreaterEqual,
			Value:    *filter.CPUUsageMin,
		})
	}

	if filter.CPUUsageMax != nil {
		group.Conditions = append(group.Conditions, &FilterCondition{
			Field:    "cpu_usage",
			Operator: OpLessEqual,
			Value:    *filter.CPUUsageMax,
		})
	}

	// 内存使用率筛选
	if filter.MemoryUsageMin != nil {
		group.Conditions = append(group.Conditions, &FilterCondition{
			Field:    "memory_usage",
			Operator: OpGreaterEqual,
			Value:    *filter.MemoryUsageMin,
		})
	}

	if filter.MemoryUsageMax != nil {
		group.Conditions = append(group.Conditions, &FilterCondition{
			Field:    "memory_usage",
			Operator: OpLessEqual,
			Value:    *filter.MemoryUsageMax,
		})
	}

	// 响应时间筛选
	if filter.ResponseTimeMin != nil {
		group.Conditions = append(group.Conditions, &FilterCondition{
			Field:    "response_time_ms",
			Operator: OpGreaterEqual,
			Value:    *filter.ResponseTimeMin,
		})
	}

	if filter.ResponseTimeMax != nil {
		group.Conditions = append(group.Conditions, &FilterCondition{
			Field:    "response_time_ms",
			Operator: OpLessEqual,
			Value:    *filter.ResponseTimeMax,
		})
	}

	return group
}

// buildSQLFilter 构建SQL筛选条件
func (e *FilterEngine) buildSQLFilter(filter *SQLFilter) *FilterGroup {
	if filter == nil {
		return nil
	}

	group := &FilterGroup{
		Logic:      LogicAnd,
		Conditions: make([]*FilterCondition, 0),
	}

	// 数据库/表筛选
	if len(filter.Database) > 0 {
		group.Conditions = append(group.Conditions, &FilterCondition{
			Field:    "database",
			Operator: OpIn,
			Values:   toInterfaceSlice(filter.Database),
		})
	}

	if len(filter.Table) > 0 {
		group.Conditions = append(group.Conditions, &FilterCondition{
			Field:    "table_name",
			Operator: OpIn,
			Values:   toInterfaceSlice(filter.Table),
		})
	}

	// 操作类型筛选
	if len(filter.Operation) > 0 {
		group.Conditions = append(group.Conditions, &FilterCondition{
			Field:    "operation",
			Operator: OpIn,
			Values:   toInterfaceSlice(filter.Operation),
		})
	}

	// SQL模式匹配
	if filter.QueryPattern != "" {
		group.Conditions = append(group.Conditions, &FilterCondition{
			Field:    "query",
			Operator: OpLike,
			Value:    "%" + filter.QueryPattern + "%",
		})
	}

	// 执行时间筛选
	if filter.ExecTimeMin != nil {
		group.Conditions = append(group.Conditions, &FilterCondition{
			Field:    "exec_time_ms",
			Operator: OpGreaterEqual,
			Value:    *filter.ExecTimeMin,
		})
	}

	if filter.ExecTimeMax != nil {
		group.Conditions = append(group.Conditions, &FilterCondition{
			Field:    "exec_time_ms",
			Operator: OpLessEqual,
			Value:    *filter.ExecTimeMax,
		})
	}

	// 错误筛选
	if filter.HasError != nil {
		if *filter.HasError {
			group.Conditions = append(group.Conditions, &FilterCondition{
				Field:    "error_code",
				Operator: OpIsNotNull,
			})
		} else {
			group.Conditions = append(group.Conditions, &FilterCondition{
				Field:    "error_code",
				Operator: OpIsNull,
			})
		}
	}

	if len(filter.ErrorCode) > 0 {
		group.Conditions = append(group.Conditions, &FilterCondition{
			Field:    "error_code",
			Operator: OpIn,
			Values:   toInterfaceSlice(filter.ErrorCode),
		})
	}

	return group
}

// Execute 执行查询
func (e *FilterEngine) Execute(ctx context.Context, query *QueryRequest) (*QueryResult, error) {
	startTime := time.Now()

	// 构建SQL
	sql, args := e.buildSQL(query)

	// 执行查询
	result := &QueryResult{
		SQL: sql,
	}

	// 查询总数
	if query.IncludeTotal {
		countSQL := e.buildCountSQL(query)
		var total int64
		err := e.db.QueryRowContext(ctx, countSQL, args...).Scan(&total)
		if err != nil {
			return nil, fmt.Errorf("count query failed: %w", err)
		}
		result.Total = total
	}

	// 查询数据
	rows, err := e.db.QueryContext(ctx, sql, args...)
	if err != nil {
		return nil, fmt.Errorf("query failed: %w", err)
	}
	defer rows.Close()

	// 获取列信息
	columns, err := rows.Columns()
	if err != nil {
		return nil, err
	}
	result.Fields = columns

	// 扫描数据
	data := make([]map[string]interface{}, 0)
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
		data = append(data, row)
	}

	result.Data = data
	result.ExecTime = time.Since(startTime)

	// 分页信息
	if query.Pagination != nil {
		result.Page = query.Pagination.Page
		result.PageSize = query.Pagination.PageSize
	}

	return result, nil
}

// buildSQL 构建SQL语句
func (e *FilterEngine) buildSQL(query *QueryRequest) (string, []interface{}) {
	var sql strings.Builder
	var args []interface{}

	// SELECT子句
	sql.WriteString("SELECT ")
	if query.Distinct {
		sql.WriteString("DISTINCT ")
	}

	if len(query.Fields) > 0 {
		sql.WriteString(strings.Join(query.Fields, ", "))
	} else {
		sql.WriteString("*")
	}

	// FROM子句
	tableName := e.getTableName(query.SourceType)
	sql.WriteString(" FROM ")
	sql.WriteString(tableName)

	// WHERE子句
	whereClause, whereArgs := e.buildWhereClause(query.Filter)
	if whereClause != "" {
		sql.WriteString(" WHERE ")
		sql.WriteString(whereClause)
		args = append(args, whereArgs...)
	}

	// 时间范围
	if !query.StartTime.IsZero() && !query.EndTime.IsZero() {
		if whereClause != "" {
			sql.WriteString(" AND ")
		} else {
			sql.WriteString(" WHERE ")
		}
		sql.WriteString("timestamp >= ? AND timestamp <= ?")
		args = append(args, query.StartTime, query.EndTime)
	}

	// GROUP BY子句
	if len(query.GroupBy) > 0 {
		sql.WriteString(" GROUP BY ")
		sql.WriteString(strings.Join(query.GroupBy, ", "))
	}

	// ORDER BY子句
	if len(query.OrderBy) > 0 {
		sql.WriteString(" ORDER BY ")
		orderParts := make([]string, len(query.OrderBy))
		for i, order := range query.OrderBy {
			orderParts[i] = order.Field
			if order.Desc {
				orderParts[i] += " DESC"
			}
		}
		sql.WriteString(strings.Join(orderParts, ", "))
	}

	// LIMIT/OFFSET子句
	if query.Pagination != nil {
		offset := (query.Pagination.Page - 1) * query.Pagination.PageSize
		sql.WriteString(" LIMIT ? OFFSET ?")
		args = append(args, query.Pagination.PageSize, offset)
	}

	return sql.String(), args
}

// buildCountSQL 构建计数SQL
func (e *FilterEngine) buildCountSQL(query *QueryRequest) string {
	var sql strings.Builder

	sql.WriteString("SELECT COUNT(*)")

	tableName := e.getTableName(query.SourceType)
	sql.WriteString(" FROM ")
	sql.WriteString(tableName)

	whereClause, _ := e.buildWhereClause(query.Filter)
	if whereClause != "" {
		sql.WriteString(" WHERE ")
		sql.WriteString(whereClause)
	}

	return sql.String()
}

// buildWhereClause 构建WHERE子句
func (e *FilterEngine) buildWhereClause(filter *FilterGroup) (string, []interface{}) {
	if filter == nil {
		return "", nil
	}

	var clauses []string
	var args []interface{}

	// 处理条件
	for _, cond := range filter.Conditions {
		clause, arg := e.buildCondition(cond)
		clauses = append(clauses, clause)
		args = append(args, arg...)
	}

	// 处理嵌套组
	for _, group := range filter.Groups {
		subClause, subArgs := e.buildWhereClause(group)
		if subClause != "" {
			clauses = append(clauses, "("+subClause+")")
			args = append(args, subArgs...)
		}
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

// buildCondition 构建条件
func (e *FilterEngine) buildCondition(cond *FilterCondition) (string, []interface{}) {
	var clause string
	var args []interface{}

	switch cond.Operator {
	case OpIn, OpNotIn:
		placeholders := make([]string, len(cond.Values))
		for i, v := range cond.Values {
			placeholders[i] = "?"
			args = append(args, v)
		}
		clause = fmt.Sprintf("%s %s (%s)", cond.Field, cond.Operator, strings.Join(placeholders, ", "))

	case OpBetween:
		if len(cond.Values) >= 2 {
			clause = fmt.Sprintf("%s BETWEEN ? AND ?", cond.Field)
			args = append(args, cond.Values[0], cond.Values[1])
		}

	case OpIsNull, OpIsNotNull:
		clause = fmt.Sprintf("%s %s", cond.Field, cond.Operator)

	case OpLike:
		clause = fmt.Sprintf("%s LIKE ?", cond.Field)
		args = append(args, cond.Value)

	case OpContains:
		clause = fmt.Sprintf("%s LIKE ?", cond.Field)
		args = append(args, "%"+fmt.Sprintf("%v", cond.Value)+"%")

	default:
		clause = fmt.Sprintf("%s %s ?", cond.Field, cond.Operator)
		args = append(args, cond.Value)
	}

	return clause, args
}

// getTableName 获取表名
func (e *FilterEngine) getTableName(sourceType string) string {
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

// MultiFilter 多条件筛选
func (e *FilterEngine) MultiFilter(ctx context.Context, request *MultiFilterRequest) (*MultiFilterResult, error) {
	result := &MultiFilterResult{
		Results: make(map[string]*QueryResult),
	}

	var wg sync.WaitGroup
	var mu sync.Mutex
	errCh := make(chan error, len(request.Filters))

	for name, filter := range request.Filters {
		wg.Add(1)
		go func(name string, filter *QueryRequest) {
			defer wg.Done()

			queryResult, err := e.Execute(ctx, filter)
			if err != nil {
				errCh <- fmt.Errorf("%s: %w", name, err)
				return
			}

			mu.Lock()
			result.Results[name] = queryResult
			mu.Unlock()
		}(name, filter)
	}

	wg.Wait()
	close(errCh)

	for err := range errCh {
		if err != nil {
			return nil, err
		}
	}

	return result, nil
}

// MultiFilterRequest 多条件筛选请求
type MultiFilterRequest struct {
	Filters map[string]*QueryRequest `json:"filters"`
}

// MultiFilterResult 多条件筛选结果
type MultiFilterResult struct {
	Results map[string]*QueryResult `json:"results"`
}

// 辅助函数
func toInterfaceSlice(slice []string) []interface{} {
	result := make([]interface{}, len(slice))
	for i, v := range slice {
		result[i] = v
	}
	return result
}

func toInterfaceSliceUint16(slice []uint16) []interface{} {
	result := make([]interface{}, len(slice))
	for i, v := range slice {
		result[i] = v
	}
	return result
}

func toInterfaceSliceUint32(slice []uint32) []interface{} {
	result := make([]interface{}, len(slice))
	for i, v := range slice {
		result[i] = v
	}
	return result
}

// SaveQuery 保存查询
func (e *FilterEngine) SaveQuery(name string, query *QueryRequest) error {
	queryJSON, err := json.Marshal(query)
	if err != nil {
		return err
	}

	_, err = e.db.Exec(`
		INSERT OR REPLACE INTO saved_queries (name, query, created_at)
		VALUES (?, ?, ?)
	`, name, string(queryJSON), time.Now())

	return err
}

// LoadQuery 加载查询
func (e *FilterEngine) LoadQuery(name string) (*QueryRequest, error) {
	var queryJSON string
	err := e.db.QueryRow(`SELECT query FROM saved_queries WHERE name = ?`, name).Scan(&queryJSON)
	if err != nil {
		return nil, err
	}

	var query QueryRequest
	if err := json.Unmarshal([]byte(queryJSON), &query); err != nil {
		return nil, err
	}

	return &query, nil
}

// ListSavedQueries 列出保存的查询
func (e *FilterEngine) ListSavedQueries() ([]string, error) {
	rows, err := e.db.Query(`SELECT name FROM saved_queries ORDER BY name`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var names []string
	for rows.Next() {
		var name string
		if err := rows.Scan(&name); err != nil {
			return nil, err
		}
		names = append(names, name)
	}

	return names, nil
}

// DeleteQuery 删除查询
func (e *FilterEngine) DeleteQuery(name string) error {
	_, err := e.db.Exec(`DELETE FROM saved_queries WHERE name = ?`, name)
	return err
}

// ParseFilterFromJSON 从JSON解析筛选条件
func ParseFilterFromJSON(jsonStr string) (*FilterGroup, error) {
	var filter FilterGroup
	if err := json.Unmarshal([]byte(jsonStr), &filter); err != nil {
		return nil, err
	}
	return &filter, nil
}

// ParseFilterFromMap 从Map解析筛选条件
func ParseFilterFromMap(m map[string]interface{}) (*FilterGroup, error) {
	group := &FilterGroup{
		Logic:      LogicAnd,
		Conditions: make([]*FilterCondition, 0),
	}

	for field, value := range m {
		switch v := value.(type) {
		case string:
			group.Conditions = append(group.Conditions, &FilterCondition{
				Field:    field,
				Operator: OpEqual,
				Value:    v,
			})
		case float64:
			group.Conditions = append(group.Conditions, &FilterCondition{
				Field:    field,
				Operator: OpEqual,
				Value:    v,
			})
		case bool:
			group.Conditions = append(group.Conditions, &FilterCondition{
				Field:    field,
				Operator: OpEqual,
				Value:    v,
			})
		case []interface{}:
			group.Conditions = append(group.Conditions, &FilterCondition{
				Field:    field,
				Operator: OpIn,
				Values:   v,
			})
		case map[string]interface{}:
			// 处理范围查询
			if min, ok := v["min"]; ok {
				group.Conditions = append(group.Conditions, &FilterCondition{
					Field:    field,
					Operator: OpGreaterEqual,
					Value:    min,
				})
			}
			if max, ok := v["max"]; ok {
				group.Conditions = append(group.Conditions, &FilterCondition{
					Field:    field,
					Operator: OpLessEqual,
					Value:    max,
				})
			}
		}
	}

	return group, nil
}

// parseIntOrDefault 解析整数或返回默认值
func parseIntOrDefault(s string, def int) int {
	val, err := strconv.Atoi(s)
	if err != nil {
		return def
	}
	return val
}
