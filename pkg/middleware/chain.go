//go:build linux

package middleware

import "net/http"

// Chain 中间件链
type Chain []func(http.Handler) http.Handler

// NewChain 创建中间件链
func NewChain(middlewares ...func(http.Handler) http.Handler) Chain {
	return Chain(middlewares)
}

// Then 应用中间件链到 handler
func (c Chain) Then(h http.Handler) http.Handler {
	if h == nil {
		h = http.DefaultServeMux
	}

	// 反向应用中间件（最后一个先执行）
	for i := len(c) - 1; i >= 0; i-- {
		h = c[i](h)
	}

	return h
}

// ThenFunc 应用中间件链到 handler func
func (c Chain) ThenFunc(fn http.HandlerFunc) http.Handler {
	return c.Then(fn)
}

// Append 添加中间件到链
func (c Chain) Append(middlewares ...func(http.Handler) http.Handler) Chain {
	return append(c, middlewares...)
}

// DefaultChain 默认中间件链（Recovery + TraceID + Logging）
func DefaultChain() Chain {
	return NewChain(
		Recovery(),           // panic 恢复
		trace.HTTPMiddleware, // TraceID
		Logging(),            // 请求日志
	)
}

// WrapHandler 包装 handler 使用默认中间件链
func WrapHandler(h http.HandlerFunc) http.Handler {
	return DefaultChain().ThenFunc(h)
}
