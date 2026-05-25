//go:build linux

package middleware

import (
	"net/http"
	"time"

	"cloud-flow-agent/pkg/logger"
	"cloud-flow-agent/pkg/trace"
)

// LoggingConfig 日志中间件配置
type LoggingConfig struct {
	// 是否记录请求体
	LogRequestBody bool
	// 是否记录响应体
	LogResponseBody bool
	// 请求体最大长度
	MaxRequestBodySize int
	// 响应体最大长度
	MaxResponseBodySize int
	// 慢请求阈值
	SlowRequestThreshold time.Duration
}

// DefaultLoggingConfig 默认日志配置
func DefaultLoggingConfig() *LoggingConfig {
	return &LoggingConfig{
		LogRequestBody:       false,
		LogResponseBody:      false,
		MaxRequestBodySize:   1024,
		MaxResponseBodySize:  1024,
		SlowRequestThreshold: time.Second,
	}
}

// Logging HTTP 日志中间件
func Logging(config ...*LoggingConfig) func(http.Handler) http.Handler {
	cfg := DefaultLoggingConfig()
	if len(config) > 0 && config[0] != nil {
		cfg = config[0]
	}

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			start := time.Now()

			// 获取或创建 TraceID
			ctx, traceCtx := trace.FromHTTPRequestWithContext(r)
			r = r.WithContext(ctx)

			// 包装 ResponseWriter 以捕获状态码
			wrapped := &responseWriter{
				ResponseWriter: w,
				statusCode:     http.StatusOK,
			}

			// 执行请求
			next.ServeHTTP(wrapped, r)

			// 计算耗时
			duration := time.Since(start)

			// 构建日志字段
			fields := logger.Fields{
				"trace_id":   traceCtx.TraceID,
				"method":     r.Method,
				"path":       r.URL.Path,
				"status":     wrapped.statusCode,
				"duration":   duration.Milliseconds(),
				"duration_ms": duration.Milliseconds(),
				"client_ip":  r.RemoteAddr,
				"user_agent": r.UserAgent(),
			}

			// 添加查询参数
			if r.URL.RawQuery != "" {
				fields["query"] = r.URL.RawQuery
			}

			// 判断日志级别
			log := logger.WithTraceID(traceCtx.TraceID)

			if wrapped.statusCode >= 500 {
				log.Error("HTTP request", fields)
			} else if wrapped.statusCode >= 400 {
				log.Warn("HTTP request", fields)
			} else if duration > cfg.SlowRequestThreshold {
				log.Warn("slow HTTP request", fields)
			} else {
				log.Info("HTTP request", fields)
			}
		})
	}
}

// responseWriter 包装 http.ResponseWriter 以捕获状态码
type responseWriter struct {
	http.ResponseWriter
	statusCode int
	written    bool
}

// WriteHeader 捕获状态码
func (rw *responseWriter) WriteHeader(code int) {
	if !rw.written {
		rw.statusCode = code
		rw.written = true
		rw.ResponseWriter.WriteHeader(code)
	}
}

// Write 写入响应
func (rw *responseWriter) Write(b []byte) (int, error) {
	if !rw.written {
		rw.WriteHeader(http.StatusOK)
	}
	return rw.ResponseWriter.Write(b)
}
