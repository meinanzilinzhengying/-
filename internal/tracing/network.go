// Package tracing 网络追踪器
// Copyright (c) 2026 Cloud Flow Team
// Licensed under the MIT License.

package tracing

import (
	"fmt"
	"net"
	"sync"
	"time"
)

// NetworkTracer 网络追踪器
type NetworkTracer struct {
	activeTraces map[string]*NetworkTrace // key: traceID:spanID
	mu           sync.RWMutex
}

// NetworkTrace 网络追踪
type NetworkTrace struct {
	TraceID    TraceID
	SpanID     SpanID
	StartTime  time.Time
	Events     []NetworkEvent
	mu         sync.Mutex
}

// NewNetworkTracer 创建网络追踪器
func NewNetworkTracer() *NetworkTracer {
	return &NetworkTracer{
		activeTraces: make(map[string]*NetworkTrace),
	}
}

// StartTracing 开始追踪
func (nt *NetworkTracer) StartTracing(traceID TraceID, spanID SpanID) {
	key := fmt.Sprintf("%s:%s", traceID, spanID)
	
	nt.mu.Lock()
	defer nt.mu.Unlock()
	
	nt.activeTraces[key] = &NetworkTrace{
		TraceID:   traceID,
		SpanID:    spanID,
		StartTime: time.Now(),
		Events:    make([]NetworkEvent, 0),
	}
}

// EndTracing 结束追踪
func (nt *NetworkTracer) EndTracing(traceID TraceID, spanID SpanID) []NetworkEvent {
	key := fmt.Sprintf("%s:%s", traceID, spanID)
	
	nt.mu.Lock()
	defer nt.mu.Unlock()
	
	trace, exists := nt.activeTraces[key]
	if !exists {
		return nil
	}
	
	events := make([]NetworkEvent, len(trace.Events))
	copy(events, trace.Events)
	
	delete(nt.activeTraces, key)
	
	return events
}

// RecordConnect 记录连接事件
func (nt *NetworkTracer) RecordConnect(traceID TraceID, spanID SpanID, localAddr, remoteAddr string, latency time.Duration, err error) {
	key := fmt.Sprintf("%s:%s", traceID, spanID)
	
	nt.mu.RLock()
	trace, exists := nt.activeTraces[key]
	nt.mu.RUnlock()
	
	if !exists {
		return
	}
	
	trace.mu.Lock()
	defer trace.mu.Unlock()
	
	event := NetworkEvent{
		Timestamp:  time.Now(),
		Type:       "connect",
		LocalAddr:  localAddr,
		RemoteAddr: remoteAddr,
		Latency:    latency,
	}
	
	if err != nil {
		event.Error = err.Error()
	}
	
	trace.Events = append(trace.Events, event)
}

// RecordSend 记录发送事件
func (nt *NetworkTracer) RecordSend(traceID TraceID, spanID SpanID, localAddr, remoteAddr string, bytes int64, latency time.Duration, err error) {
	key := fmt.Sprintf("%s:%s", traceID, spanID)
	
	nt.mu.RLock()
	trace, exists := nt.activeTraces[key]
	nt.mu.RUnlock()
	
	if !exists {
		return
	}
	
	trace.mu.Lock()
	defer trace.mu.Unlock()
	
	event := NetworkEvent{
		Timestamp:  time.Now(),
		Type:       "send",
		LocalAddr:  localAddr,
		RemoteAddr: remoteAddr,
		Bytes:      bytes,
		Latency:    latency,
	}
	
	if err != nil {
		event.Error = err.Error()
	}
	
	trace.Events = append(trace.Events, event)
}

// RecordRecv 记录接收事件
func (nt *NetworkTracer) RecordRecv(traceID TraceID, spanID SpanID, localAddr, remoteAddr string, bytes int64, latency time.Duration, err error) {
	key := fmt.Sprintf("%s:%s", traceID, spanID)
	
	nt.mu.RLock()
	trace, exists := nt.activeTraces[key]
	nt.mu.RUnlock()
	
	if !exists {
		return
	}
	
	trace.mu.Lock()
	defer trace.mu.Unlock()
	
	event := NetworkEvent{
		Timestamp:  time.Now(),
		Type:       "recv",
		LocalAddr:  localAddr,
		RemoteAddr: remoteAddr,
		Bytes:      bytes,
		Latency:    latency,
	}
	
	if err != nil {
		event.Error = err.Error()
	}
	
	trace.Events = append(trace.Events, event)
}

// RecordClose 记录关闭事件
func (nt *NetworkTracer) RecordClose(traceID TraceID, spanID SpanID, localAddr, remoteAddr string) {
	key := fmt.Sprintf("%s:%s", traceID, spanID)
	
	nt.mu.RLock()
	trace, exists := nt.activeTraces[key]
	nt.mu.RUnlock()
	
	if !exists {
		return
	}
	
	trace.mu.Lock()
	defer trace.mu.Unlock()
	
	event := NetworkEvent{
		Timestamp:  time.Now(),
		Type:       "close",
		LocalAddr:  localAddr,
		RemoteAddr: remoteAddr,
	}
	
	trace.Events = append(trace.Events, event)
}

// GetActiveTraces 获取活跃追踪
func (nt *NetworkTracer) GetActiveTraces() map[string]*NetworkTrace {
	nt.mu.RLock()
	defer nt.mu.RUnlock()
	
	result := make(map[string]*NetworkTrace)
	for k, v := range nt.activeTraces {
		result[k] = v
	}
	return result
}

// ClearOldTraces 清除过期追踪
func (nt *NetworkTracer) ClearOldTraces(maxAge time.Duration) {
	nt.mu.Lock()
	defer nt.mu.Unlock()
	
	now := time.Now()
	for key, trace := range nt.activeTraces {
		if now.Sub(trace.StartTime) > maxAge {
			delete(nt.activeTraces, key)
		}
	}
}

// NetworkPath 网络路径
type NetworkPath struct {
	Source      string    `json:"source"`
	Destination string    `json:"destination"`
	Hops        []NetHop  `json:"hops"`
	TotalLatency time.Duration `json:"total_latency"`
	PacketLoss  float64   `json:"packet_loss"`
}

// NetHop 网络跳
type NetHop struct {
	Index    int       `json:"index"`
	Address  string    `json:"address"`
	Latency  time.Duration `json:"latency"`
	LossRate float64   `json:"loss_rate"`
}

// TraceNetworkPath 追踪网络路径
func (nt *NetworkTracer) TraceNetworkPath(source, destination string) (*NetworkPath, error) {
	// 简化实现：使用traceroute或类似工具
	path := &NetworkPath{
		Source:      source,
		Destination: destination,
		Hops:        make([]NetHop, 0),
	}
	
	// 解析目标地址
	_, err := net.ResolveTCPAddr("tcp", destination)
	if err != nil {
		return nil, fmt.Errorf("failed to resolve destination: %w", err)
	}
	
	// 实际实现应该调用traceroute命令或使用原始套接字
	// 这里仅作为框架示例
	
	return path, nil
}

// GetNetworkStats 获取网络统计
func (nt *NetworkTracer) GetNetworkStats(traceID TraceID, spanID SpanID) *NetworkStats {
	key := fmt.Sprintf("%s:%s", traceID, spanID)
	
	nt.mu.RLock()
	trace, exists := nt.activeTraces[key]
	nt.mu.RUnlock()
	
	if !exists {
		return nil
	}
	
	trace.mu.Lock()
	defer trace.mu.Unlock()
	
	stats := &NetworkStats{
		ConnectCount: 0,
		SendCount:    0,
		RecvCount:    0,
		CloseCount:   0,
		TotalBytes:   0,
		TotalLatency: 0,
		ErrorCount:   0,
	}
	
	for _, event := range trace.Events {
		switch event.Type {
		case "connect":
			stats.ConnectCount++
			stats.TotalLatency += event.Latency
		case "send":
			stats.SendCount++
			stats.TotalBytes += event.Bytes
			stats.TotalLatency += event.Latency
		case "recv":
			stats.RecvCount++
			stats.TotalBytes += event.Bytes
			stats.TotalLatency += event.Latency
		case "close":
			stats.CloseCount++
		}
		
		if event.Error != "" {
			stats.ErrorCount++
		}
	}
	
	if len(trace.Events) > 0 {
		stats.AvgLatency = stats.TotalLatency / time.Duration(len(trace.Events))
	}
	
	return stats
}

// NetworkStats 网络统计
type NetworkStats struct {
	ConnectCount int           `json:"connect_count"`
	SendCount    int           `json:"send_count"`
	RecvCount    int           `json:"recv_count"`
	CloseCount   int           `json:"close_count"`
	TotalBytes   int64         `json:"total_bytes"`
	TotalLatency time.Duration `json:"total_latency"`
	AvgLatency   time.Duration `json:"avg_latency"`
	ErrorCount   int           `json:"error_count"`
}
