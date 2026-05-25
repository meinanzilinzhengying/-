//go:build linux

// Package trace 提供 TraceID 生成和传播机制
package trace

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"net/http"
	"sync"
)

const (
	// TraceIDHeader TraceID HTTP 请求头
	TraceIDHeader = "X-Trace-ID"
	// TraceIDKey context key
	TraceIDKey = "trace_id"
)

// Context 追踪上下文
type Context struct {
	TraceID string
	SpanID  string
	ParentID string
}

// NewContext 创建新的追踪上下文
func NewContext() *Context {
	return &Context{
		TraceID: GenerateTraceID(),
		SpanID:  GenerateSpanID(),
	}
}

// NewContextWithParent 创建带父上下文的追踪上下文
func NewContextWithParent(traceID, parentID string) *Context {
	return &Context{
		TraceID:  traceID,
		SpanID:   GenerateSpanID(),
		ParentID: parentID,
	}
}

// ToHTTPHeader 将追踪上下文写入 HTTP Header
func (c *Context) ToHTTPHeader(header http.Header) {
	if c.TraceID != "" {
		header.Set(TraceIDHeader, c.TraceID)
	}
}

// GenerateTraceID 生成 TraceID (32位十六进制)
func GenerateTraceID() string {
	return generateID(16)
}

// GenerateSpanID 生成 SpanID (16位十六进制)
func GenerateSpanID() string {
	return generateID(8)
}

// generateID 生成指定长度的十六进制ID
func generateID(length int) string {
	bytes := make([]byte, length)
	if _, err := rand.Read(bytes); err != nil {
		// 回退到简单生成
		return fallbackID(length)
	}
	return hex.EncodeToString(bytes)
}

// fallbackID 简单的 ID 生成回退方案
func fallbackID(length int) string {
	const chars = "0123456789abcdef"
	result := make([]byte, length*2)
	for i := range result {
		result[i] = chars[i%16]
	}
	return string(result)
}

// ============================================================
// Context 集成
// ============================================================

// contextKey 私有类型避免冲突
type contextKey string

const traceContextKey contextKey = "trace_context"

// WithContext 将追踪上下文添加到 context
func WithContext(ctx context.Context, traceCtx *Context) context.Context {
	if ctx == nil {
		ctx = context.Background()
	}
	return context.WithValue(ctx, traceContextKey, traceCtx)
}

// FromContext 从 context 获取追踪上下文
func FromContext(ctx context.Context) string {
	if ctx == nil {
		return ""
	}

	if traceCtx, ok := ctx.Value(traceContextKey).(*Context); ok {
		return traceCtx.TraceID
	}

	// 尝试直接获取 string 类型的 trace_id
	if traceID, ok := ctx.Value(TraceIDKey).(string); ok {
		return traceID
	}

	return ""
}

// GetContext 从 context 获取完整的追踪上下文
func GetContext(ctx context.Context) *Context {
	if ctx == nil {
		return nil
	}

	if traceCtx, ok := ctx.Value(traceContextKey).(*Context); ok {
		return traceCtx
	}

	return nil
}

// EnsureContext 确保 context 中有 TraceID，如果没有则创建
func EnsureContext(ctx context.Context) context.Context {
	if ctx == nil {
		ctx = context.Background()
	}

	if FromContext(ctx) != "" {
		return ctx
	}

	return WithContext(ctx, NewContext())
}

// WithTraceID 将 TraceID 添加到 context
func WithTraceID(ctx context.Context, traceID string) context.Context {
	if ctx == nil {
		ctx = context.Background()
	}
	return context.WithValue(ctx, TraceIDKey, traceID)
}

// ============================================================
// HTTP 集成
// ============================================================

// FromHTTPRequest 从 HTTP 请求获取或创建追踪上下文
func FromHTTPRequest(r *http.Request) *Context {
	if r == nil {
		return NewContext()
	}

	// 从请求头获取 TraceID
	traceID := r.Header.Get(TraceIDHeader)
	if traceID == "" {
		// 创建新的 TraceID
		return NewContext()
	}

	return &Context{
		TraceID: traceID,
		SpanID:  GenerateSpanID(),
	}
}

// FromHTTPRequestWithContext 从 HTTP 请求获取或创建带 context 的追踪上下文
func FromHTTPRequestWithContext(r *http.Request) (context.Context, *Context) {
	traceCtx := FromHTTPRequest(r)
	ctx := WithContext(r.Context(), traceCtx)
	return ctx, traceCtx
}

// HTTPMiddleware TraceID HTTP 中间件
func HTTPMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ctx, traceCtx := FromHTTPRequestWithContext(r)

		// 将 TraceID 写入响应头
		w.Header().Set(TraceIDHeader, traceCtx.TraceID)

		// 使用新的 context 继续处理
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

// HTTPMiddlewareFunc TraceID HTTP 中间件（函数形式）
func HTTPMiddlewareFunc(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ctx, traceCtx := FromHTTPRequestWithContext(r)
		w.Header().Set(TraceIDHeader, traceCtx.TraceID)
		next(w, r.WithContext(ctx))
	}
}

// ============================================================
// gRPC 集成
// ============================================================

// TraceMetadataKey gRPC metadata key
const TraceMetadataKey = "x-trace-id"

// FromGRPCContext 从 gRPC context 获取 TraceID
// 注意：需要在导入 grpc 包后实现
func FromGRPCContext(ctx context.Context) string {
	// 先尝试从普通 context 获取
	if traceID := FromContext(ctx); traceID != "" {
		return traceID
	}

	// TODO: 从 gRPC metadata 获取
	// 需要在导入 google.golang.org/grpc/metadata 后实现
	// md, ok := metadata.FromIncomingContext(ctx)
	// if ok {
	//     if values := md.Get(TraceMetadataKey); len(values) > 0 {
	//         return values[0]
	//     }
	// }

	return ""
}

// ============================================================
// 便捷方法
// ============================================================

// StartSpan 开始新的 span
func StartSpan(ctx context.Context) (context.Context, *Context) {
	traceCtx := GetContext(ctx)
	if traceCtx == nil {
		traceCtx = NewContext()
		ctx = WithContext(ctx, traceCtx)
	} else {
		// 创建子 span
		newCtx := NewContextWithParent(traceCtx.TraceID, traceCtx.SpanID)
		ctx = WithContext(ctx, newCtx)
		traceCtx = newCtx
	}
	return ctx, traceCtx
}

// Span 用于创建 span 的辅助结构
type Span struct {
	ctx      context.Context
	traceCtx *Context
	once     sync.Once
}

// Finish 结束 span
func (s *Span) Finish() {
	s.once.Do(func() {
		// TODO: 上报 span 数据
	})
}

// Context 获取 span 的 context
func (s *Span) Context() context.Context {
	return s.ctx
}

// TraceID 获取 TraceID
func (s *Span) TraceID() string {
	if s.traceCtx != nil {
		return s.traceCtx.TraceID
	}
	return ""
}

// SpanID 获取 SpanID
func (s *Span) SpanID() string {
	if s.traceCtx != nil {
		return s.traceCtx.SpanID
	}
	return ""
}

// BeginSpan 开始一个 span
func BeginSpan(ctx context.Context) *Span {
	newCtx, traceCtx := StartSpan(ctx)
	return &Span{
		ctx:      newCtx,
		traceCtx: traceCtx,
	}
}
