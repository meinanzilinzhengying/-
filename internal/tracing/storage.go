// Package tracing 追踪数据存储与查询
// Copyright (c) 2026 Cloud Flow Team
// Licensed under the MIT License.

package tracing

import (
	"fmt"
	"sort"
	"sync"
	"time"
)

// TraceStorage 追踪数据存储
type TraceStorage struct {
	traces map[TraceID]*Trace
	spans  map[SpanID]*Span
	mu     sync.RWMutex
}

// NewTraceStorage 创建追踪数据存储
func NewTraceStorage() *TraceStorage {
	return &TraceStorage{
		traces: make(map[TraceID]*Trace),
		spans:  make(map[SpanID]*Span),
	}
}

// StoreTrace 存储追踪
func (s *TraceStorage) StoreTrace(trace *Trace) {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.traces[trace.TraceID] = trace

	// 存储所有Span
	for _, span := range trace.Spans {
		s.spans[span.SpanID] = span
	}
}

// StoreSpan 存储Span
func (s *TraceStorage) StoreSpan(span *Span) {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.spans[span.SpanID] = span
}

// GetTrace 获取追踪
func (s *TraceStorage) GetTrace(traceID TraceID) (*Trace, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	trace, exists := s.traces[traceID]
	if !exists {
		return nil, fmt.Errorf("trace not found: %s", traceID)
	}

	return trace, nil
}

// GetSpan 获取Span
func (s *TraceStorage) GetSpan(spanID SpanID) (*Span, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	span, exists := s.spans[spanID]
	if !exists {
		return nil, fmt.Errorf("span not found: %s", spanID)
	}

	return span, nil
}

// GetTraceBySpan 通过Span获取追踪
func (s *TraceStorage) GetTraceBySpan(spanID SpanID) (*Trace, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	span, exists := s.spans[spanID]
	if !exists {
		return nil, fmt.Errorf("span not found: %s", spanID)
	}

	trace, exists := s.traces[span.TraceID]
	if !exists {
		return nil, fmt.Errorf("trace not found: %s", span.TraceID)
	}

	return trace, nil
}

// QueryTraces 查询追踪
func (s *TraceStorage) QueryTraces(query *TraceQuery) ([]*Trace, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	var results []*Trace

	for _, trace := range s.traces {
		if s.matchTrace(trace, query) {
			results = append(results, trace)
		}
	}

	// 排序
	sort.Slice(results, func(i, j int) bool {
		return results[i].StartTime.After(results[j].StartTime)
	})

	// 限制数量
	if query.Limit > 0 && len(results) > query.Limit {
		results = results[:query.Limit]
	}

	return results, nil
}

// matchTrace 匹配追踪
func (s *TraceStorage) matchTrace(trace *Trace, query *TraceQuery) bool {
	// 时间范围
	if !query.StartTime.IsZero() && trace.StartTime.Before(query.StartTime) {
		return false
	}
	if !query.EndTime.IsZero() && trace.StartTime.After(query.EndTime) {
		return false
	}

	// 服务名
	if query.ServiceName != "" {
		found := false
		for _, span := range trace.Spans {
			if span.ServiceName == query.ServiceName {
				found = true
				break
			}
		}
		if !found {
			return false
		}
	}

	// 状态
	if query.Status != SpanStatusUnset {
		if trace.ErrorCount == 0 && query.Status == SpanStatusError {
			return false
		}
		if trace.ErrorCount > 0 && query.Status == SpanStatusOk {
			return false
		}
	}

	// 最小持续时间
	if query.MinDuration > 0 && trace.Duration < query.MinDuration {
		return false
	}

	// 最大持续时间
	if query.MaxDuration > 0 && trace.Duration > query.MaxDuration {
		return false
	}

	return true
}

// QuerySpans 查询Span
func (s *TraceStorage) QuerySpans(query *SpanQuery) ([]*Span, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	var results []*Span

	for _, span := range s.spans {
		if s.matchSpan(span, query) {
			results = append(results, span)
		}
	}

	// 排序
	sort.Slice(results, func(i, j int) bool {
		return results[i].StartTime.After(results[j].StartTime)
	})

	// 限制数量
	if query.Limit > 0 && len(results) > query.Limit {
		results = results[:query.Limit]
	}

	return results, nil
}

// matchSpan 匹配Span
func (s *TraceStorage) matchSpan(span *Span, query *SpanQuery) bool {
	// TraceID
	if query.TraceID != "" && span.TraceID != query.TraceID {
		return false
	}

	// 时间范围
	if !query.StartTime.IsZero() && span.StartTime.Before(query.StartTime) {
		return false
	}
	if !query.EndTime.IsZero() && span.StartTime.After(query.EndTime) {
		return false
	}

	// 服务名
	if query.ServiceName != "" && span.ServiceName != query.ServiceName {
		return false
	}

	// Span名
	if query.SpanName != "" && span.Name != query.SpanName {
		return false
	}

	// 状态
	if query.Status != SpanStatusUnset && span.Status != query.Status {
		return false
	}

	// 最小持续时间
	if query.MinDuration > 0 && span.Duration < query.MinDuration {
		return false
	}

	// 最大持续时间
	if query.MaxDuration > 0 && span.Duration > query.MaxDuration {
		return false
	}

	// 属性
	for key, value := range query.Attributes {
		if span.Attributes[key] != value {
			return false
		}
	}

	return true
}

// DeleteOldTraces 删除过期追踪
func (s *TraceStorage) DeleteOldTraces(maxAge time.Duration) int {
	s.mu.Lock()
	defer s.mu.Unlock()

	now := time.Now()
	deleted := 0

	for traceID, trace := range s.traces {
		if now.Sub(trace.StartTime) > maxAge {
			// 删除关联的Span
			for _, span := range trace.Spans {
				delete(s.spans, span.SpanID)
			}
			delete(s.traces, traceID)
			deleted++
		}
	}

	return deleted
}

// GetTraceCount 获取追踪数量
func (s *TraceStorage) GetTraceCount() int {
	s.mu.RLock()
	defer s.mu.RUnlock()

	return len(s.traces)
}

// GetSpanCount 获取Span数量
func (s *TraceStorage) GetSpanCount() int {
	s.mu.RLock()
	defer s.mu.RUnlock()

	return len(s.spans)
}

// Clear 清空存储
func (s *TraceStorage) Clear() {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.traces = make(map[TraceID]*Trace)
	s.spans = make(map[SpanID]*Span)
}

// TraceQuery 追踪查询条件
type TraceQuery struct {
	StartTime   time.Time
	EndTime     time.Time
	ServiceName string
	Status      SpanStatus
	MinDuration time.Duration
	MaxDuration time.Duration
	Limit       int
}

// SpanQuery Span查询条件
type SpanQuery struct {
	TraceID     TraceID
	StartTime   time.Time
	EndTime     time.Time
	ServiceName string
	SpanName    string
	Status      SpanStatus
	MinDuration time.Duration
	MaxDuration time.Duration
	Attributes  map[string]interface{}
	Limit       int
}

// BuildTrace 构建完整追踪
func (s *TraceStorage) BuildTrace(spans []*Span) *Trace {
	if len(spans) == 0 {
		return nil
	}

	trace := &Trace{
		TraceID:   spans[0].TraceID,
		Spans:     spans,
		StartTime: spans[0].StartTime,
		EndTime:   spans[0].EndTime,
	}

	// 查找根Span
	for _, span := range spans {
		if span.ParentSpanID == "" {
			trace.RootSpan = span
		}
		if span.StartTime.Before(trace.StartTime) {
			trace.StartTime = span.StartTime
		}
		if span.EndTime.After(trace.EndTime) {
			trace.EndTime = span.EndTime
		}
		if span.Status == SpanStatusError {
			trace.ErrorCount++
		}
	}

	trace.Duration = trace.EndTime.Sub(trace.StartTime)
	trace.SpanCount = len(spans)

	// 统计服务数量
	services := make(map[string]bool)
	for _, span := range spans {
		services[span.ServiceName] = true
	}
	trace.ServiceCount = len(services)

	return trace
}

// GetSpanTree 获取Span树
func (s *TraceStorage) GetSpanTree(traceID TraceID) (*SpanNode, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	trace, exists := s.traces[traceID]
	if !exists {
		return nil, fmt.Errorf("trace not found: %s", traceID)
	}

	// 构建Span树
	spanMap := make(map[SpanID]*SpanNode)
	for _, span := range trace.Spans {
		spanMap[span.SpanID] = &SpanNode{
			Span:     span,
			Children: make([]*SpanNode, 0),
		}
	}

	var root *SpanNode
	for _, node := range spanMap {
		if node.Span.ParentSpanID == "" {
			root = node
		} else if parent, exists := spanMap[node.Span.ParentSpanID]; exists {
			parent.Children = append(parent.Children, node)
		}
	}

	return root, nil
}

// SpanNode Span树节点
type SpanNode struct {
	Span     *Span
	Children []*SpanNode
}

// GetCriticalPath 获取关键路径
func (s *TraceStorage) GetCriticalPath(traceID TraceID) ([]*Span, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	trace, exists := s.traces[traceID]
	if !exists {
		return nil, fmt.Errorf("trace not found: %s", traceID)
	}

	if len(trace.Spans) == 0 {
		return nil, nil
	}

	// 找到耗时最长的路径
	spanMap := make(map[SpanID]*Span)
	children := make(map[SpanID][]*Span)

	for _, span := range trace.Spans {
		spanMap[span.SpanID] = span
		if span.ParentSpanID != "" {
			children[span.ParentSpanID] = append(children[span.ParentSpanID], span)
		}
	}

	// 找到根Span
	var rootSpan *Span
	for _, span := range trace.Spans {
		if span.ParentSpanID == "" {
			rootSpan = span
			break
		}
	}

	if rootSpan == nil {
		return nil, fmt.Errorf("no root span found")
	}

	// DFS找到最长路径
	var criticalPath []*Span
	var dfs func(span *Span, path []*Span)

	dfs = func(span *Span, path []*Span) {
		currentPath := append(path, span)

		childSpans, exists := children[span.SpanID]
		if !exists || len(childSpans) == 0 {
			// 叶子节点
			if len(currentPath) > len(criticalPath) {
				criticalPath = currentPath
			}
			return
		}

		for _, child := range childSpans {
			dfs(child, currentPath)
		}
	}

	dfs(rootSpan, nil)

	return criticalPath, nil
}

// GetServiceDependencies 获取服务依赖关系
func (s *TraceStorage) GetServiceDependencies() map[string][]string {
	s.mu.RLock()
	defer s.mu.RUnlock()

	dependencies := make(map[string]map[string]bool)

	for _, trace := range s.traces {
		for _, span := range trace.Spans {
			if span.ParentSpanID != "" {
				parentSpan, exists := s.spans[span.ParentSpanID]
				if exists && parentSpan.ServiceName != span.ServiceName {
					if dependencies[parentSpan.ServiceName] == nil {
						dependencies[parentSpan.ServiceName] = make(map[string]bool)
					}
					dependencies[parentSpan.ServiceName][span.ServiceName] = true
				}
			}
		}
	}

	result := make(map[string][]string)
	for service, deps := range dependencies {
		for dep := range deps {
			result[service] = append(result[service], dep)
		}
	}

	return result
}

// GetServiceStats 获取服务统计
func (s *TraceStorage) GetServiceStats(serviceName string) *ServiceStats {
	s.mu.RLock()
	defer s.mu.RUnlock()

	stats := &ServiceStats{
		ServiceName: serviceName,
	}

	var totalDuration time.Duration
	var durations []time.Duration

	for _, span := range s.spans {
		if span.ServiceName == serviceName {
			stats.SpanCount++
			totalDuration += span.Duration
			durations = append(durations, span.Duration)

			if span.Status == SpanStatusError {
				stats.ErrorCount++
			}

			if span.Duration > stats.MaxDuration {
				stats.MaxDuration = span.Duration
			}
			if stats.MinDuration == 0 || span.Duration < stats.MinDuration {
				stats.MinDuration = span.Duration
			}
		}
	}

	if stats.SpanCount > 0 {
		stats.AvgDuration = totalDuration / time.Duration(stats.SpanCount)
		stats.ErrorRate = float64(stats.ErrorCount) / float64(stats.SpanCount)

		// 计算P99
		if len(durations) > 0 {
			sort.Slice(durations, func(i, j int) bool {
				return durations[i] < durations[j]
			})
			p99Index := int(float64(len(durations)) * 0.99)
			if p99Index >= len(durations) {
				p99Index = len(durations) - 1
			}
			stats.P99Duration = durations[p99Index]
		}
	}

	return stats
}

// ServiceStats 服务统计
type ServiceStats struct {
	ServiceName string
	SpanCount   int
	ErrorCount  int
	ErrorRate   float64
	AvgDuration time.Duration
	MinDuration time.Duration
	MaxDuration time.Duration
	P99Duration time.Duration
}
