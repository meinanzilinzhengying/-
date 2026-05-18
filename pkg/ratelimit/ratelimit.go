package ratelimit

import (
	"sync"
	"time"
)

// TokenBucket 令牌桶限流器
type TokenBucket struct {
	mu         sync.Mutex
	bucketSize int
	refillRate int // 每秒填充的令牌数
	tokens     int
	lastRefill time.Time
}

// NewTokenBucket 创建令牌桶
func NewTokenBucket(bucketSize, refillRate int) *TokenBucket {
	return &TokenBucket{
		bucketSize: bucketSize,
		refillRate: refillRate,
		tokens:     bucketSize,
		lastRefill: time.Now(),
	}
}

// Allow 尝试消费一个令牌
func (tb *TokenBucket) Allow() bool {
	tb.mu.Lock()
	defer tb.mu.Unlock()

	now := time.Now()
	elapsed := now.Sub(tb.lastRefill)
	tb.lastRefill = now

	// 按时间比例填充令牌
	refillTokens := int(elapsed.Seconds()) * tb.refillRate
	if refillTokens > 0 {
		tb.tokens += refillTokens
		if tb.tokens > tb.bucketSize {
			tb.tokens = tb.bucketSize
		}
	}

	if tb.tokens > 0 {
		tb.tokens--
		return true
	}
	return false
}
