//go:build linux

// Package middleware 提供全局中间件
package middleware

import (
	"fmt"
	"net/http"
	"runtime/debug"
	"time"

	"cloud-flow-agent/pkg/errors"
	"cloud-flow-agent/pkg/logger"
	"cloud-flow-agent/pkg/trace"
)

// RecoveryConfig panic 恢复配置
type RecoveryConfig struct {
	// 是否打印堆栈
	PrintStack bool
	// 堆栈大小
	StackSize int
	// 自定义错误处理函数
	ErrorHandler func(ctx *trace.Context, err interface{})
	// 是否上报错误（如发送到 Sentry）
	ReportError bool
}

// DefaultRecoveryConfig 默认配置
func DefaultRecoveryConfig() *RecoveryConfig {
	return &RecoveryConfig{
		PrintStack:  true,
		StackSize:   4096,
		ErrorHandler: nil,
		ReportError: false,
	}
}

// Recovery HTTP panic 恢复中间件
func Recovery(config ...*RecoveryConfig) func(http.Handler) http.Handler {
	cfg := DefaultRecoveryConfig()
	if len(config) > 0 && config[0] != nil {
		cfg = config[0]
	}

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// 获取 TraceID
			ctx := trace.FromHTTPRequest(r)

			defer func() {
				if err := recover(); err != nil {
					// 记录 panic 日志
					log := logger.WithContext(ctx)

					log.Error("panic recovered",
						"error", fmt.Sprintf("%v", err),
						"path", r.URL.Path,
						"method", r.Method,
						"remote_addr", r.RemoteAddr,
					)

					// 打印堆栈
					if cfg.PrintStack {
						stack := debug.Stack()
						if len(stack) > cfg.StackSize {
							stack = stack[:cfg.StackSize]
						}
						log.Error("panic stack", "stack", string(stack))
					}

					// 自定义错误处理
					if cfg.ErrorHandler != nil {
						cfg.ErrorHandler(ctx, err)
					}

					// 上报错误
					if cfg.ReportError {
						// TODO: 集成 Sentry 或其他错误上报服务
					}

					// 返回统一错误响应
					errors.WriteErrorResponse(w, errors.ErrInternalServer, ctx.TraceID)
				}
			}()

			next.ServeHTTP(w, r)
		})
	}
}

// RecoveryHandler 直接包装 HTTP Handler
func RecoveryHandler(handler http.HandlerFunc, config ...*RecoveryConfig) http.HandlerFunc {
	cfg := DefaultRecoveryConfig()
	if len(config) > 0 && config[0] != nil {
		cfg = config[0]
	}

	return func(w http.ResponseWriter, r *http.Request) {
		ctx := trace.FromHTTPRequest(r)

		defer func() {
			if err := recover(); err != nil {
				log := logger.WithContext(ctx)

				log.Error("panic recovered",
					"error", fmt.Sprintf("%v", err),
					"path", r.URL.Path,
					"method", r.Method,
				)

				if cfg.PrintStack {
					stack := debug.Stack()
					if len(stack) > cfg.StackSize {
						stack = stack[:cfg.StackSize]
					}
					log.Error("panic stack", "stack", string(stack))
				}

				if cfg.ErrorHandler != nil {
					cfg.ErrorHandler(ctx, err)
				}

				errors.WriteErrorResponse(w, errors.ErrInternalServer, ctx.TraceID)
			}
		}()

		handler(w, r)
	}
}

// GoSafe 安全地启动 goroutine，自动捕获 panic
func GoSafe(fn func(), config ...*RecoveryConfig) {
	cfg := DefaultRecoveryConfig()
	if len(config) > 0 && config[0] != nil {
		cfg = config[0]
	}

	go func() {
		defer func() {
			if err := recover(); err != nil {
				log := logger.Default()

				log.Error("goroutine panic recovered",
					"error", fmt.Sprintf("%v", err),
					"time", time.Now().Format(time.RFC3339),
				)

				if cfg.PrintStack {
					stack := debug.Stack()
					if len(stack) > cfg.StackSize {
						stack = stack[:cfg.StackSize]
					}
					log.Error("goroutine panic stack", "stack", string(stack))
				}

				if cfg.ErrorHandler != nil {
					cfg.ErrorHandler(trace.NewContext(), err)
				}
			}
		}()

		fn()
	}()
}

// SafeCall 安全地执行函数，返回是否发生 panic
func SafeCall(fn func()) (panicked bool) {
	defer func() {
		if err := recover(); err != nil {
			panicked = true

			log := logger.Default()
			log.Error("safe call panic recovered",
				"error", fmt.Sprintf("%v", err),
			)
		}
	}()

	fn()
	return false
}

// SafeCallWithResult 安全地执行函数，返回结果和是否发生 panic
func SafeCallWithResult(fn func() error) (err error, panicked bool) {
	defer func() {
		if r := recover(); r != nil {
			panicked = true
			err = fmt.Errorf("panic: %v", r)

			log := logger.Default()
			log.Error("safe call panic recovered",
				"error", fmt.Sprintf("%v", r),
			)
		}
	}()

	return fn(), false
}
