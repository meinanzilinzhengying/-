// Package tracing 提供TraceID跨服务/网络追踪，关联进程剖析能力
// 支持分布式调用链追踪、网络路径追踪、进程级性能关联
// Copyright (c) 2026 Cloud Flow Team
// Licensed under the MIT License.

package tracing

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"sync"
	"sync/atomic"
	"time"
)

// ============================================================
// 配置定义
// ============================================================

// TracingConfig 分布式追踪配置
type TracingConfig struct {
	Enabled              bool          `yaml:"enabled" json:"enabled"`
	SampleRate           float64       `yaml:"sample_rate" json:"sample_rate"`           // 采样率 (0-1)
	MaxSpansPerTrace     int           `yaml:"max_spans_per_trace" json:"max_spans_per_trace"` // 单个Trace最大Span数
	SpanBufferSize       int           `yaml:"span_buffer_size" json:"span_buffer_size"` // Span缓冲区大小
	FlushInterval        time.Duration `yaml:"flush_interval" json:"flush_interval"`     // 刷新间隔
	EnableNetworkTracing bool          `yaml:"enable_network_tracing" json:"enable_network_tracing"` // 启用网络追踪
	EnableProcessProfile bool          `yaml:"enable_process_profile" json:"enable_process_profile"` // 启用进程剖析
	ProfileSampleRate    float64       `yaml:"profile_sample_rate" json:"profile_sample_rate"` // 剖析采样率
	HeaderPropagation    []string      `yaml:"header_propagation" json:"header_propagation"` // 传播的Header
	BaggagePropagation   []string      `yaml:"baggage_propagation" json:"baggage_propagation"` // 传播的Baggage
}

func DefaultTracingConfig() *TracingConfig {
	return &TracingConfig{
		Enabled:              true,
		SampleRate:           0.1, // 10%采样
		MaxSpansPerTrace:     1000,
		SpanBufferSize:       10000,
		FlushInterval:        5 * time.Second,
		EnableNetworkTracing: true,
		EnableProcessProfile: true,
		ProfileSampleRate:    0.01, // 1%剖析采样
		HeaderPropagation:    []string{"traceparent", "tracestate", "x-request-id", "x-b3-traceid", "x-b3-spanid", "x-b3-parentspanid"},
		BaggagePropagation:   []string{"user-id", "tenant-id", "request-type"},
	}
}

// ============================================================
// TraceID和Span模型
// ============================================================

// TraceID 追踪ID
type TraceID string

// GenerateTraceID 生成新的TraceID
func GenerateTraceID() TraceID {
	b := make([]byte, 16)
	rand.Read(b)
	return TraceID(hex.EncodeToString(b))
}

// SpanID Span ID
type SpanID string

// GenerateSpanID 生成新的SpanID
func GenerateSpanID() SpanID {
	b := make([]byte, 8)
	rand.Read(b)
	return SpanID(hex.EncodeToString(b))
}

// SpanKind Span类型
type SpanKind int

const (
	SpanKindUnspecified SpanKind = iota
	SpanKindInternal
	SpanKindServer
	SpanKindClient
	SpanKindProducer
	SpanKindConsumer
)

func (k SpanKind) String() string {
	names := []string{"unspecified", "internal", "server", "client", "producer", "consumer"}
	if int(k) < len(names) {
		return names[k]
	}
	return "unknown"
}

// SpanStatus Span状态
type SpanStatus int

const (
	SpanStatusUnset SpanStatus = iota
	SpanStatusOk
	SpanStatusError
)

// Span 追踪Span
type Span struct {
	TraceID       TraceID           `json:"trace_id"`
	SpanID        SpanID            `json:"span_id"`
	ParentSpanID  SpanID            `json:"parent_span_id,omitempty"`
	Name          string            `json:"name"`
	Kind          SpanKind          `json:"kind"`
	StartTime     time.Time         `json:"start_time"`
	EndTime       time.Time         `json:"end_time,omitempty"`
	Duration      time.Duration     `json:"duration"`
	Status        SpanStatus        `json:"status"`
	StatusMessage string            `json:"status_message,omitempty"`
	Attributes    map[string]interface{} `json:"attributes,omitempty"`
	Events        []SpanEvent       `json:"events,omitempty"`
	Links         []SpanLink        `json:"links,omitempty"`
	ServiceName   string            `json:"service_name"`
	ServiceVersion string           `json:"service_version,omitempty"`
	HostName      string            `json:"host_name"`
	PID           uint32            `json:"pid"`
	TID           uint32            `json:"tid"`
	
	// 进程剖析关联
	ProfileData   *ProcessProfile   `json:"profile_data,omitempty"`
	
	// 网络追踪关联
	NetworkEvents []NetworkEvent    `json:"network_events,omitempty"`
}

// SpanEvent Span事件
type SpanEvent struct {
	Timestamp time.Time              `json:"timestamp"`
	Name      string                 `json:"name"`
	Attributes map[string]interface{} `json:"attributes,omitempty"`
}

// SpanLink Span链接
type SpanLink struct {
	TraceID    TraceID                `json:"trace_id"`
	SpanID     SpanID                 `json:"span_id"`
	Attributes map[string]interface{} `json:"attributes,omitempty"`
}

// ProcessProfile 进程剖析数据
type ProcessProfile struct {
	CPUTime       time.Duration `json:"cpu_time"`
	MemoryAlloc   uint64        `json:"memory_alloc"`
	MemoryInUse   uint64        `json:"memory_in_use"`
	Goroutines    int           `json:"goroutines"`
	Syscalls      uint64        `json:"syscalls"`
	IOReadBytes   uint64        `json:"io_read_bytes"`
	IOWriteBytes  uint64        `json:"io_write_bytes"`
	StackTrace    []string      `json:"stack_trace,omitempty"`
}

// NetworkEvent 网络事件
type NetworkEvent struct {
	Timestamp  time.Time `json:"timestamp"`
	Type       string    `json:"type"` // connect, send, recv, close
	LocalAddr  string    `json:"local_addr"`
	RemoteAddr string    `json:"remote_addr"`
	Bytes      int64     `json:"bytes"`
	Latency    time.Duration `json:"latency"`
	Error      string    `json:"error,omitempty"`
}

// Trace 完整追踪
type Trace struct {
	TraceID      TraceID   `json:"trace_id"`
	Spans        []*Span   `json:"spans"`
	RootSpan     *Span     `json:"root_span,omitempty"`
	StartTime    time.Time `json:"start_time"`
	EndTime      time.Time `json:"end_time,omitempty"`
	Duration     time.Duration `json:"duration"`
	ServiceCount int       `json:"service_count"`
	SpanCount    int       `json:"span_count"`
	ErrorCount   int       `json:"error_count"`
}

// ============================================================
// SpanContext 传播上下文
// ============================================================

// SpanContext Span上下文
type SpanContext struct {
	TraceID  TraceID
	SpanID   SpanID
	TraceFlags byte
	TraceState string
	Baggage  map[string]string
}

// IsValid 检查是否有效
func (sc SpanContext) IsValid() bool {
	return sc.TraceID != "" && sc.SpanID != ""
}

// IsSampled 检查是否采样
func (sc SpanContext) IsSampled() bool {
	return sc.TraceFlags&0x01 == 0x01
}

// WithBaggage 添加Baggage
func (sc SpanContext) WithBaggage(key, value string) SpanContext {
	if sc.Baggage == nil {
		sc.Baggage = make(map[string]string)
	}
	sc.Baggage[key] = value
	return sc
}

// GetBaggage 获取Baggage
func (sc SpanContext) GetBaggage(key string) string {
	if sc.Baggage == nil {
		return ""
	}
	return sc.Baggage[key]
}

// ============================================================
// 上下文管理
// ============================================================

type contextKey struct{}

var spanContextKey = contextKey{}

// ContextWithSpanContext 将SpanContext注入context
func ContextWithSpanContext(ctx context.Context, sc SpanContext) context.Context {
	return context.WithValue(ctx, spanContextKey, sc)
}

// SpanContextFromContext 从context提取SpanContext
func SpanContextFromContext(ctx context.Context) SpanContext {
	if sc, ok := ctx.Value(spanContextKey).(SpanContext); ok {
		return sc
	}
	return SpanContext{}
}

// ============================================================
// Tracer 追踪器
// ============================================================

// Tracer 分布式追踪器
type Tracer struct {
	config      *TracingConfig
	serviceName string
	serviceVersion string
	hostName    string
	
	// Span缓冲区
	spanBuffer []*Span
	bufferMu   sync.Mutex
	
	// 采样决策
	sampler Sampler
	
	// 导出器
	exporters []SpanExporter
	
	// 进程剖析器
	profiler *ProcessProfiler
	
	// 网络追踪器
	networkTracer *NetworkTracer
	
	// 统计
	spanCount atomic.Uint64
	dropCount atomic.Uint64
	
	// 生命周期
	flushTicker *time.Ticker
	stopCh      chan struct{}
	wg          sync.WaitGroup
}

// Sampler 采样器接口
type Sampler interface {
	ShouldSample(traceID TraceID, operation string) bool
}

// ProbabilitySampler 概率采样器
type ProbabilitySampler struct {
	rate float64
}

// ShouldSample 决定是否采样
func (s *ProbabilitySampler) ShouldSample(traceID TraceID, operation string) bool {
	if s.rate >= 1.0 {
		return true
	}
	if s.rate <= 0 {
		return false
	}
	// 基于TraceID的确定性采样
	return hashTraceID(traceID) < s.rate
}

func hashTraceID(traceID TraceID) float64 {
	// 简单哈希
	var sum uint64
	for _, c := range traceID {
		sum = sum*31 + uint64(c)
	}
	return float64(sum%10000) / 10000.0
}

// SpanExporter Span导出器接口
type SpanExporter interface {
	ExportSpans(spans []*Span) error
	Shutdown() error
}

// NewTracer 创建追踪器
func NewTracer(config *TracingConfig, serviceName, serviceVersion string) (*Tracer, error) {
	if config == nil {
		config = DefaultTracingConfig()
	}

	hostName, _ := getHostname()

	tracer := &Tracer{
		config:         config,
		serviceName:    serviceName,
		serviceVersion: serviceVersion,
		hostName:       hostName,
		spanBuffer:     make([]*Span, 0, config.SpanBufferSize),
		sampler:        &ProbabilitySampler{rate: config.SampleRate},
		exporters:      make([]SpanExporter, 0),
		stopCh:         make(chan struct{}),
	}

	// 初始化进程剖析器
	if config.EnableProcessProfile {
		tracer.profiler = NewProcessProfiler(config.ProfileSampleRate)
	}

	// 初始化网络追踪器
	if config.EnableNetworkTracing {
		tracer.networkTracer = NewNetworkTracer()
	}

	// 启动刷新循环
	tracer.flushTicker = time.NewTicker(config.FlushInterval)
	tracer.wg.Add(1)
	go tracer.flushLoop()

	return tracer, nil
}

// StartSpan 开始Span
func (t *Tracer) StartSpan(ctx context.Context, name string, opts ...SpanOption) (context.Context, *Span) {
	config := &SpanConfig{}
	for _, opt := range opts {
		opt(config)
	}

	// 获取父SpanContext
	parentSC := SpanContextFromContext(ctx)

	// 创建新的SpanContext
	var sc SpanContext
	if parentSC.IsValid() {
		sc.TraceID = parentSC.TraceID
		sc.TraceFlags = parentSC.TraceFlags
		sc.TraceState = parentSC.TraceState
		sc.Baggage = parentSC.Baggage
	} else {
		sc.TraceID = GenerateTraceID()
		// 采样决策
		if t.sampler.ShouldSample(sc.TraceID, name) {
			sc.TraceFlags = 0x01
		}
	}
	sc.SpanID = GenerateSpanID()

	// 创建Span
	span := &Span{
		TraceID:        sc.TraceID,
		SpanID:         sc.SpanID,
		Name:           name,
		Kind:           config.Kind,
		StartTime:      time.Now(),
		Attributes:     make(map[string]interface{}),
		ServiceName:    t.serviceName,
		ServiceVersion: t.serviceVersion,
		HostName:       t.hostName,
		PID:            getPID(),
		TID:            getTID(),
	}

	// 设置ParentSpanID
	if parentSC.IsValid() {
		span.ParentSpanID = parentSC.SpanID
	}

	// 应用配置
	if config.Parent != nil {
		span.ParentSpanID = config.Parent.SpanID
		span.TraceID = config.Parent.TraceID
	}

	// 开始进程剖析
	if t.profiler != nil && t.shouldProfile() {
		span.ProfileData = t.profiler.StartProfiling()
	}

	// 开始网络追踪
	if t.networkTracer != nil {
		t.networkTracer.StartTracing(span.TraceID, span.SpanID)
	}

	// 将SpanContext注入新的context
	newCtx := ContextWithSpanContext(ctx, sc)

	return newCtx, span
}

// EndSpan 结束Span
func (t *Tracer) EndSpan(span *Span, opts ...EndSpanOption) {
	if span == nil {
		return
	}

	span.EndTime = time.Now()
	span.Duration = span.EndTime.Sub(span.StartTime)

	// 结束进程剖析
	if t.profiler != nil && span.ProfileData != nil {
		t.profiler.EndProfiling(span.ProfileData)
	}

	// 结束网络追踪
	if t.networkTracer != nil {
		span.NetworkEvents = t.networkTracer.EndTracing(span.TraceID, span.SpanID)
	}

	// 应用结束选项
	for _, opt := range opts {
		opt(span)
	}

	// 添加到缓冲区
	t.addSpan(span)
}

// shouldProfile 决定是否剖析
func (t *Tracer) shouldProfile() bool {
	if t.config.ProfileSampleRate >= 1.0 {
		return true
	}
	// 简单随机采样
	return hashTraceID(GenerateTraceID()) < t.config.ProfileSampleRate
}

// addSpan 添加Span到缓冲区
func (t *Tracer) addSpan(span *Span) {
	t.bufferMu.Lock()
	defer t.bufferMu.Unlock()

	// 检查缓冲区是否已满
	if len(t.spanBuffer) >= t.config.SpanBufferSize {
		t.dropCount.Add(1)
		return
	}

	t.spanBuffer = append(t.spanBuffer, span)
	t.spanCount.Add(1)
}

// flushLoop 刷新循环
func (t *Tracer) flushLoop() {
	defer t.wg.Done()

	for {
		select {
		case <-t.stopCh:
			t.flush()
			return
		case <-t.flushTicker.C:
			t.flush()
		}
	}
}

// flush 刷新缓冲区
func (t *Tracer) flush() {
	t.bufferMu.Lock()
	if len(t.spanBuffer) == 0 {
		t.bufferMu.Unlock()
		return
	}

	spans := make([]*Span, len(t.spanBuffer))
	copy(spans, t.spanBuffer)
	t.spanBuffer = t.spanBuffer[:0]
	t.bufferMu.Unlock()

	// 导出Span
	for _, exporter := range t.exporters {
		go exporter.ExportSpans(spans)
	}
}

// Shutdown 关闭追踪器
func (t *Tracer) Shutdown() error {
	close(t.stopCh)
	t.flushTicker.Stop()
	t.wg.Wait()

	// 关闭导出器
	for _, exporter := range t.exporters {
		exporter.Shutdown()
	}

	return nil
}

// AddExporter 添加导出器
func (t *Tracer) AddExporter(exporter SpanExporter) {
	t.exporters = append(t.exporters, exporter)
}

// SpanConfig Span配置
type SpanConfig struct {
	Kind   SpanKind
	Parent *Span
}

// SpanOption Span选项
type SpanOption func(*SpanConfig)

// WithSpanKind 设置Span类型
func WithSpanKind(kind SpanKind) SpanOption {
	return func(c *SpanConfig) {
		c.Kind = kind
	}
}

// WithParent 设置父Span
func WithParent(parent *Span) SpanOption {
	return func(c *SpanConfig) {
		c.Parent = parent
	}
}

// EndSpanOption 结束Span选项
type EndSpanOption func(*Span)

// WithStatus 设置状态
func WithStatus(status SpanStatus, message string) EndSpanOption {
	return func(s *Span) {
		s.Status = status
		s.StatusMessage = message
	}
}

// WithAttribute 添加属性
func WithAttribute(key string, value interface{}) EndSpanOption {
	return func(s *Span) {
		s.Attributes[key] = value
	}
}

// ============================================================
// HTTP传播
// ============================================================

// HTTPPropagator HTTP传播器
type HTTPPropagator struct {
	config *TracingConfig
}

// NewHTTPPropagator 创建HTTP传播器
func NewHTTPPropagator(config *TracingConfig) *HTTPPropagator {
	return &HTTPPropagator{config: config}
}

// Inject 注入SpanContext到HTTP Header
func (p *HTTPPropagator) Inject(ctx context.Context, header http.Header) {
	sc := SpanContextFromContext(ctx)
	if !sc.IsValid() {
		return
	}

	// W3C Trace Context
	header.Set("traceparent", fmt.Sprintf("00-%s-%s-%02x", sc.TraceID, sc.SpanID, sc.TraceFlags))
	if sc.TraceState != "" {
		header.Set("tracestate", sc.TraceState)
	}

	// Zipkin B3 Propagation
	header.Set("X-B3-TraceId", string(sc.TraceID))
	header.Set("X-B3-SpanId", string(sc.SpanID))
	if sc.IsSampled() {
		header.Set("X-B3-Sampled", "1")
	}

	// Baggage
	for key, value := range sc.Baggage {
		header.Set("baggage-"+key, value)
	}
}

// Extract 从HTTP Header提取SpanContext
func (p *HTTPPropagator) Extract(header http.Header) SpanContext {
	var sc SpanContext

	// 尝试W3C Trace Context
	traceParent := header.Get("traceparent")
	if traceParent != "" {
		sc = p.parseTraceParent(traceParent)
	}

	// 尝试Zipkin B3
	if !sc.IsValid() {
		sc = p.parseB3Headers(header)
	}

	// 提取Baggage
	sc.Baggage = make(map[string]string)
	for key, values := range header {
		if strings.HasPrefix(key, "baggage-") {
			baggageKey := strings.TrimPrefix(key, "baggage-")
			if len(values) > 0 {
				sc.Baggage[baggageKey] = values[0]
			}
		}
	}

	return sc
}

// parseTraceParent 解析W3C traceparent
func (p *HTTPPropagator) parseTraceParent(value string) SpanContext {
	// 格式: 00-traceid-spanid-flags
	parts := strings.Split(value, "-")
	if len(parts) != 4 {
		return SpanContext{}
	}

	var flags byte
	fmt.Sscanf(parts[3], "%02x", &flags)

	return SpanContext{
		TraceID:    TraceID(parts[1]),
		SpanID:     SpanID(parts[2]),
		TraceFlags: flags,
	}
}

// parseB3Headers 解析B3 headers
func (p *HTTPPropagator) parseB3Headers(header http.Header) SpanContext {
	traceID := header.Get("X-B3-TraceId")
	spanID := header.Get("X-B3-SpanId")

	if traceID == "" || spanID == "" {
		return SpanContext{}
	}

	var flags byte
	if header.Get("X-B3-Sampled") == "1" {
		flags = 0x01
	}

	return SpanContext{
		TraceID:    TraceID(traceID),
		SpanID:     SpanID(spanID),
		TraceFlags: flags,
	}
}

// ============================================================
// 辅助函数
// ============================================================

func getHostname() (string, error) {
	return "localhost", nil // 简化实现
}

func getPID() uint32 {
	return uint32(0) // 简化实现
}

func getTID() uint32 {
	return uint32(0) // 简化实现
}
