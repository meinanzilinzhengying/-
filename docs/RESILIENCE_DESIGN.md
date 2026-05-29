# CloudFlow 微服务弹性容错设计

## 概述

本文档描述 CloudFlow 平台实现的微服务弹性容错机制，包括限流、熔断、降级、重试和隔离等策略，用于防止级联故障和雪崩效应。

## 目录

1. [架构设计](#架构设计)
2. [熔断器 (Circuit Breaker)](#熔断器-circuit-breaker)
3. [限流器 (Rate Limiter)](#限流器-rate-limiter)
4. [重试机制 (Retry)](#重试机制-retry)
5. [隔离舱 (Bulkhead)](#隔离舱-bulkhead)
6. [降级处理 (Fallback)](#降级处理-fallback)
7. [服务集成](#服务集成)
8. [配置指南](#配置指南)
9. [监控指标](#监控指标)

## 架构设计

```
┌─────────────────────────────────────────────────────────────────┐
│                        请求入口                                   │
└─────────────────────┬───────────────────────────────────────────┘
                      │
                      ▼
┌─────────────────────────────────────────────────────────────────┐
│                      Rate Limiter                                │
│                   (请求速率限制)                                   │
└─────────────────────┬───────────────────────────────────────────┘
                      │ 允许
                      ▼
┌─────────────────────────────────────────────────────────────────┐
│                     Circuit Breaker                              │
│                   (熔断保护)                                     │
└─────────────────────┬───────────────────────────────────────────┘
                      │ 允许
                      ▼
┌─────────────────────────────────────────────────────────────────┐
│                      Bulkhead                                    │
│                   (并发隔离)                                     │
└─────────────────────┬───────────────────────────────────────────┘
                      │ 允许
                      ▼
┌─────────────────────────────────────────────────────────────────┐
│                    Service Call                                  │
│                   (业务逻辑)                                     │
└─────────────────────┬───────────────────────────────────────────┘
                      │
                      ▼
        ┌─────────────┴─────────────┐
        │                           │
        ▼                           ▼
┌───────────────┐           ┌───────────────┐
│   成功        │           │    失败       │
│  (Record)     │           │  (Retry)      │
└───────────────┘           └───────────────┘
                                    │
                                    ▼
                          ┌───────────────┐
                          │  重试耗尽      │
                          │  (Fallback)   │
                          └───────────────┘
```

## 熔断器 (Circuit Breaker)

### 概念

熔断器模式防止级联故障，当下游服务持续失败时，快速失败而不是持续重试。

### 状态转换

```
           ┌──────────────────────────────────┐
           │                                  │
           ▼                                  │
     ┌──────────┐                       ┌─────────┐
     │  Closed  │ ──失败率>阈值────▶    │   Open  │
     │  (正常)   │                       │  (熔断)  │
     └──────────┘                       └────┬────┘
           │                                     │
           │ 连续成功>阈值                        │  超时
           ▼                                     ▼
     ┌──────────┐                       ┌─────────┐
     │  Closed  │ ◀────────────────────│HalfOpen  │
     │          │        成功           │  (半开)  │
     └──────────┘                       └─────────┘
```

### 配置参数

| 参数 | 默认值 | 说明 |
|------|--------|------|
| FailureThreshold | 0.5 | 失败率阈值 (50%) |
| SuccessThreshold | 3 | 熔断恢复需要的连续成功次数 |
| MinRequests | 10 | 最小请求数，用于计算失败率 |
| OpenTimeout | 30s | 熔断持续时间 |
| HalfOpenTimeout | 60s | 半开状态超时时间 |

### 使用示例

```go
import "cloud-flow/services/shared/resilience"

cb := resilience.NewCircuitBreaker(resilience.DefaultCircuitBreakerConfig("my-service"))

if cb.Allow() {
    // 执行请求
    err := callService()
    if err != nil {
        cb.RecordFailure()
    } else {
        cb.RecordSuccess()
    }
} else {
    // 熔断开启，快速失败
    return ErrCircuitOpen
}
```

### HTTP 中间件

```go
cb := resilience.NewCircuitBreaker(config)
handler = resilience.CircuitBreakerMiddleware(cb)(handler)
```

## 限流器 (Rate Limiter)

### 概念

限流器控制请求速率，防止系统过载。

### 算法

使用 Token Bucket 算法，支持突发流量。

### 配置参数

| 参数 | 默认值 | 说明 |
|------|--------|------|
| RequestsPerSecond | 100 | 每秒允许的请求数 |
| BurstSize | 200 | 允许的突发大小 |

### 多层限流

支持每秒、每分钟、每小时多层级限流。

```go
config := resilience.RateLimiterConfig{
    Name:                 "api",
    RequestsPerSecond:    100,
    RequestsPerMinute:    5000,
    RequestsPerHour:      20000,
    BlockOnLimitExceeded: true,
}

limiter := resilience.NewMultiTierRateLimiter(config)
```

### 使用示例

```go
limiter := resilience.NewRateLimiter(config)

if limiter.Allow() {
    // 处理请求
} else {
    // 返回 429 Too Many Requests
}
```

### HTTP 中间件

```go
limiter := resilience.NewRateLimiter(config)
handler = resilience.RateLimitMiddleware(limiter)(handler)
```

## 重试机制 (Retry)

### 概念

自动重试失败的请求，使用指数退避策略避免雪球效应。

### 退避策略

```
Attempt 1: 100ms
Attempt 2: 200ms
Attempt 3: 400ms
Attempt 4: 800ms
...
(max: 30s)
```

### 配置参数

| 参数 | 默认值 | 说明 |
|------|--------|------|
| MaxAttempts | 3 | 最大重试次数 |
| InitialInterval | 100ms | 初始间隔 |
| MaxInterval | 30s | 最大间隔 |
| Multiplier | 2.0 | 退避倍数 |
| Jitter | 0.1 | 抖动因子 (10%) |

### 使用示例

```go
config := resilience.DefaultRetryConfig()
config.MaxAttempts = 5
config.InitialInterval = 200 * time.Millisecond
config.OnRetry = func(attempt int, err error, next time.Duration) {
    log.Printf("Retry %d: %v, next in %v", attempt, err, next)
}

executor := resilience.NewRetryExecutor(config)
err := executor.Execute(func() error {
    return callService()
})
```

## 隔离舱 (Bulkhead)

### 概念

隔离舱模式限制并发调用数量，防止一个服务的故障影响其他服务。

### 配置参数

| 参数 | 默认值 | 说明 |
|------|--------|------|
| MaxConcurrent | 100 | 最大并发数 |
| MaxWaitTime | 5s | 最大等待时间 |

### 使用示例

```go
bulkhead := resilience.NewBulkhead(resilience.DefaultBulkheadConfig())
bulkhead.MaxConcurrent = 50

err := bulkhead.Execute(func() error {
    return callService()
})
```

## 降级处理 (Fallback)

### 概念

当服务不可用时，返回降级响应而不是错误。

### 降级级别

| 级别 | 说明 |
|------|------|
| None | 正常服务 |
| Graceful | 优雅降级（返回部分数据） |
| Minimal | 最小功能（返回缓存数据） |
| Fallback | 完全降级（返回错误消息） |

### 使用示例

```go
handler := resilience.NewFallbackHandler(resilience.DefaultFallbackConfig())

handler.Register("user-service", func(err error) interface{} {
    return map[string]interface{}{
        "status": "degraded",
        "data":   getCachedUsers(),
    }
})

result, _ := handler.Execute("user-service", err)
```

## 服务集成

### Data Plane

```go
import "cloud-flow/services/data-plane"

config := dataplane.DefaultResilienceConfig()
config.EnableCircuitBreaker = true
config.EnableRateLimiter = true
config.EnableBulkhead = true

middleware := dataplane.NewResilienceMiddleware(config)

// 应用中间件
http.Handle("/api/query/", middleware.RateLimit(
    middleware.CircuitBreaker(handler)))
```

### Query Service

```go
import "cloud-flow/services/queryservice"

config := queryservice.DefaultResilienceConfig()
config.RateLimiter.RequestsPerSecond = 500
config.Bulkhead.MaxConcurrent = 1000

client := queryservice.NewQueryResilienceClient(config)

// 查询降级
metrics, err := client.QueryMetrics(query)
if err != nil {
    // 使用缓存或降级响应
    return getCachedMetrics()
}
```

### Alert Engine

```go
import "cloud-flow/services/alertengine"

config := alertengine.DefaultResilienceConfig()
client := alertengine.NewAlertResilienceClient(config)

// 检查告警能力
if client.ShouldSkipEvaluation() {
    return ErrMetricsUnavailable
}
```

### Auth Service

```go
import "cloud-flow/services/authservice"

config := authservice.DefaultResilienceConfig()
client := authservice.NewAuthResilienceClient(config)

// 优雅降级：使用缓存的 token
degradation := authservice.NewGracefulDegradation(
    authservice.DefaultGracefulDegradationConfig())

userID, tenantID, ok := degradation.GetCachedToken(token)
if ok {
    return userID, tenantID, nil
}
```

## 配置指南

### 开发环境

```yaml
circuit_breaker:
  enabled: false

rate_limiter:
  enabled: true
  requests_per_second: 1000

bulkhead:
  enabled: false
```

### 生产环境

```yaml
circuit_breaker:
  enabled: true
  failure_threshold: 0.5
  success_threshold: 3
  min_requests: 10
  open_timeout: 30s

rate_limiter:
  enabled: true
  requests_per_second: 100
  burst_size: 200
  requests_per_minute: 5000

bulkhead:
  enabled: true
  max_concurrent: 100
  max_wait_time: 5s

retry:
  enabled: true
  max_attempts: 3
  initial_interval: 100ms
  max_interval: 30s
  multiplier: 2.0
```

## 监控指标

### 熔断器指标

```json
{
  "name": "data-plane",
  "state": "Closed",
  "total_requests": 10000,
  "success_count": 9500,
  "failure_count": 500,
  "rejection_count": 0,
  "success_rate": 95.0,
  "error_rate": 5.0,
  "average_latency_ms": 12.5
}
```

### 限流器指标

```json
{
  "name": "api",
  "total_requests": 50000,
  "allowed_requests": 48000,
  "rejected_requests": 2000,
  "rejection_rate": 4.0
}
```

### 健康检查端点

```
GET /health/resilience

{
  "circuit_breakers": {
    "data-plane": "Closed",
    "query-service": "HalfOpen",
    "alert-engine": "Closed"
  },
  "rate_limiters": {
    "default": {
      "available": true,
      "rejection_rate": 0.5
    }
  },
  "bulkhead": {
    "max_concurrent": 100,
    "active": 45,
    "utilization": 45.0
  }
}
```

## 最佳实践

1. **分层防护**
   - 第一层：限流器（防止流量突增）
   - 第二层：熔断器（防止下游故障）
   - 第三层：隔离舱（限制资源占用）
   - 第四层：降级（提供基本功能）

2. **配置调优**
   - 根据服务容量设置限流阈值
   - 根据 SLA 设置熔断阈值
   - 监控指标调整参数

3. **监控告警**
   - 熔断器状态变化时告警
   - 限流拒绝率过高时告警
   - 服务降级时告警

4. **测试验证**
   - 模拟服务故障测试熔断
   - 压力测试验证限流
   - 混沌测试验证降级

## 参考资源

- [Martin Fowler - Circuit Breaker](https://martinfowler.com/bliki/CircuitBreaker.html)
- [Microsoft - Bulkhead Isolation](https://docs.microsoft.com/en-us/architecture/patterns/bulkhead)
- [ Resilience4j 文档](https://resilience4j.readme.io/)
