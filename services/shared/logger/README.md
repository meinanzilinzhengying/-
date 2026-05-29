# CloudFlow 日志系统

## 概述

全新的日志系统提供：
- **结构化日志** - JSON 格式，方便日志解析和分析
- **TraceId 全链路透传** - gRPC 和 HTTP 自动传递
- **错误堆栈标准化** - 统一的堆栈格式
- **审计日志** - 关键操作独立记录

## 快速开始

### 1. 初始化 Logger

```go
package main

import (
	"cloudflow/services/shared/logger"
)

func main() {
	cfg := logger.Config{
		Level:            "info",
		Format:           "json",
		Output:           "both",
		LogDir:           "/var/log/cloudflow",
		MaxSize:          100,
		MaxBackups:       10,
		MaxAge:           7,
		ServiceName:      "auth-service",
		EnableAudit:      true,
		AuditLogDir:      "/var/log/cloudflow/audit",
		EnableStackTrace: true,
	}

	l := logger.New(cfg)
	logger.SetGlobalLogger(l)
	defer l.Sync()
}
```

### 2. 基础日志使用

```go
// 简单日志
l.Info("service_started", "port", 8080)

// 带 traceId 的日志
ctx := logger.WithTraceID(context.Background())
l.WithContext(ctx).Info("request_received", "path", "/api/v1/users")

// 带错误和堆栈的日志
if err := someFunc(); err != nil {
	l.ErrorWithStack(err, "operation_failed", "resource", "user")
}
```

### 3. TraceId 全链路透传

#### gRPC 服务端
```go
import (
	"cloudflow/services/shared/logger"
	mw "cloudflow/services/shared/logger"
)

grpcServer := grpc.NewServer(
	grpc.ChainUnaryInterceptor(
		mw.GrpcUnaryServerInterceptor(l),
		// 其他拦截器
	),
	grpc.ChainStreamInterceptor(
		mw.GrpcStreamServerInterceptor(l),
	),
)
```

#### gRPC 客户端
```go
conn, err := grpc.Dial(addr,
	grpc.WithUnaryInterceptor(mw.GrpcUnaryClientInterceptor(l)),
)
```

#### HTTP 服务
```go
import mw "cloudflow/services/shared/logger"

mw := mw.NewHTTPMiddleware(l)
http.Handle("/api", mw.TraceIDMiddleware(http.HandlerFunc(handler)))
```

### 4. 审计日志

```go
// 记录审计事件
l.Audit(ctx, logger.AuditTypeLogin, "user", "login", "success",
	map[string]interface{}{"user_id": userID},
)

// 使用审计函数包装
err := mw.AuditFunc(ctx, l, logger.AuditTypeCreate, "user", "create",
	func() error {
		// 业务逻辑
		return createUser(user)
	},
)
```

## 环境变量配置

可以通过环境变量配置：

```bash
LOG_LEVEL=info
LOG_FORMAT=json
LOG_OUTPUT=both
LOG_DIR=/var/log/cloudflow
AUDIT_LOG_ENABLED=true
AUDIT_LOG_DIR=/var/log/cloudflow/audit
```

## 日志结构

### 普通日志示例 (JSON)
```json
{
  "timestamp": "2025-01-01T00:00:00Z",
  "level": "info",
  "service_name": "auth-service",
  "trace_id": "550e8400-e29b-41d4-a716-446655440000",
  "message": "request_received",
  "path": "/api/v1/users",
  "caller": "service.go:55"
}
```

### 错误日志示例
```json
{
  "timestamp": "2025-01-01T00:00:00Z",
  "level": "error",
  "service_name": "auth-service",
  "trace_id": "550e8400-e29b-41d4-a716-446655440000",
  "message": "database_error",
  "error": "connection refused",
  "stack_trace": "main.func1 (main.go:20)\n...",
  "caller": "service.go:100"
}
```

### 审计日志示例
```json
{
  "timestamp": "2025-01-01T00:00:00Z",
  "trace_id": "550e8400-e29b-41d4-a716-446655440000",
  "event_type": "login",
  "service_name": "auth-service",
  "user_id": "123",
  "tenant_id": "456",
  "resource": "user",
  "action": "login",
  "result": "success",
  "duration_ms": 50,
  "details": {}
}
```
