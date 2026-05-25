//go:build linux

// Package cache 提供Redis缓存层
package cache

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"cloud-flow-center/internal/config"
	"cloud-flow-agent/pkg/logger"
	"github.com/redis/go-redis/v9"
)

// RedisCache Redis缓存层
type RedisCache struct {
	client *redis.Client
	cfg    *config.RedisConfig
	log    *logger.Logger
}

// NewRedisCache 创建Redis缓存
func NewRedisCache(cfg *config.RedisConfig, log *logger.Logger) (*RedisCache, error) {
	var client *redis.Client

	switch cfg.Mode {
	case "cluster":
		// 集群模式
		client = redis.NewClusterClient(&redis.ClusterOptions{
			Addrs:    cfg.GetAddrs(),
			Password: cfg.Password,
			PoolSize: cfg.PoolSize,
		})

	case "sentinel":
		// 哨兵模式
		client = redis.NewFailoverClient(&redis.FailoverOptions{
			MasterName:    cfg.MasterName,
			SentinelAddrs: cfg.GetAddrs(),
			Password:      cfg.Password,
			DB:            cfg.DB,
			PoolSize:      cfg.PoolSize,
		})

	default:
		// 单机模式
		client = redis.NewClient(&redis.Options{
			Addr:         cfg.Addr,
			Password:     cfg.Password,
			DB:           cfg.DB,
			PoolSize:     cfg.PoolSize,
			MinIdleConns: cfg.MinIdleConns,
			DialTimeout:  cfg.DialTimeout,
			ReadTimeout:  cfg.ReadTimeout,
			WriteTimeout: cfg.WriteTimeout,
		})
	}

	// 测试连接
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := client.Ping(ctx).Err(); err != nil {
		return nil, fmt.Errorf("Redis连接失败: %w", err)
	}

	log.Infof("Redis缓存初始化成功: addr=%s, mode=%s", cfg.Addr, cfg.Mode)
	return &RedisCache{
		client: client,
		cfg:    cfg,
		log:    log,
	}, nil
}

// Close 关闭连接
func (c *RedisCache) Close() error {
	return c.client.Close()
}

// Ping 检查连接
func (c *RedisCache) Ping(ctx context.Context) error {
	return c.client.Ping(ctx).Err()
}

// GetClient 获取底层客户端
func (c *RedisCache) GetClient() *redis.Client {
	return c.client
}

// ==================== 通用操作 ====================

// Get 获取值
func (c *RedisCache) Get(ctx context.Context, key string) (string, error) {
	return c.client.Get(ctx, key).Result()
}

// Set 设置值
func (c *RedisCache) Set(ctx context.Context, key string, value interface{}, ttl time.Duration) error {
	switch v := value.(type) {
	case string:
		return c.client.Set(ctx, key, v, ttl).Err()
	case []byte:
		return c.client.Set(ctx, key, v, ttl).Err()
	default:
		// JSON序列化
		data, err := json.Marshal(v)
		if err != nil {
			return err
		}
		return c.client.Set(ctx, key, data, ttl).Err()
	}
}

// Delete 删除键
func (c *RedisCache) Delete(ctx context.Context, keys ...string) error {
	return c.client.Del(ctx, keys...).Err()
}

// Exists 检查键是否存在
func (c *RedisCache) Exists(ctx context.Context, key string) (bool, error) {
	n, err := c.client.Exists(ctx, key).Result()
	return n > 0, err
}

// Expire 设置过期时间
func (c *RedisCache) Expire(ctx context.Context, key string, ttl time.Duration) error {
	return c.client.Expire(ctx, key, ttl).Err()
}

// TTL 获取剩余生存时间
func (c *RedisCache) TTL(ctx context.Context, key string) (time.Duration, error) {
	return c.client.TTL(ctx, key).Result()
}

// ==================== 分布式锁 ====================

// AcquireLock 获取分布式锁
func (c *RedisCache) AcquireLock(ctx context.Context, lockKey, ownerID string, ttl time.Duration) (bool, error) {
	// 使用SET NX EX
	key := fmt.Sprintf("lock:%s", lockKey)
	ok, err := c.client.SetNX(ctx, key, ownerID, ttl).Result()
	if err != nil {
		return false, err
	}
	return ok, nil
}

// ReleaseLock 释放分布式锁（只释放自己的锁）
func (c *RedisCache) ReleaseLock(ctx context.Context, lockKey, ownerID string) error {
	key := fmt.Sprintf("lock:%s", lockKey)

	// Lua脚本：只删除自己持有的锁
	script := `
		if redis.call("get", KEYS[1]) == ARGV[1] then
			return redis.call("del", KEYS[1])
		else
			return 0
		end
	`
	_, err := c.client.Eval(ctx, script, []string{key}, ownerID).Result()
	return err
}

// ==================== 告警缓存 ====================

// Alert 告警缓存结构
type Alert struct {
	ID          string    `json:"id"`
	TenantID    string    `json:"tenant_id"`
	RuleID      string    `json:"rule_id"`
	RuleName    string    `json:"rule_name"`
	Level       string    `json:"level"`
	Status      string    `json:"status"`
	Fingerprint string    `json:"fingerprint"`
	Metric      string    `json:"metric"`
	Value       float64   `json:"value"`
	Threshold   float64   `json:"threshold"`
	FiredAt     time.Time `json:"fired_at"`
}

// GetAlert 获取告警缓存
func (c *RedisCache) GetAlert(ctx context.Context, id string) (*Alert, error) {
	key := fmt.Sprintf("alert:%s", id)
	data, err := c.client.Get(ctx, key).Bytes()
	if err != nil {
		return nil, err
	}

	var alert Alert
	if err := json.Unmarshal(data, &alert); err != nil {
		return nil, err
	}
	return &alert, nil
}

// SetAlert 设置告警缓存
func (c *RedisCache) SetAlert(ctx context.Context, alert *Alert, ttl time.Duration) error {
	key := fmt.Sprintf("alert:%s", alert.ID)
	data, err := json.Marshal(alert)
	if err != nil {
		return err
	}
	return c.client.Set(ctx, key, data, ttl).Err()
}

// GetAlerts 获取告警列表缓存
func (c *RedisCache) GetAlerts(ctx context.Context, key string) ([]*Alert, error) {
	data, err := c.client.Get(ctx, key).Bytes()
	if err != nil {
		return nil, err
	}

	var alerts []*Alert
	if err := json.Unmarshal(data, &alerts); err != nil {
		return nil, err
	}
	return alerts, nil
}

// SetAlerts 设置告警列表缓存
func (c *RedisCache) SetAlerts(ctx context.Context, key string, alerts []*Alert, ttl time.Duration) error {
	data, err := json.Marshal(alerts)
	if err != nil {
		return err
	}
	return c.client.Set(ctx, key, data, ttl).Err()
}

// ==================== 租户缓存 ====================

// Tenant 租户缓存结构
type Tenant struct {
	ID          string    `json:"id"`
	Name        string    `json:"name"`
	DisplayName string    `json:"display_name"`
	Status      string    `json:"status"`
	Quota       string    `json:"quota"`
	CreatedAt   time.Time `json:"created_at"`
}

// GetTenant 获取租户缓存
func (c *RedisCache) GetTenant(ctx context.Context, key string) (*Tenant, error) {
	data, err := c.client.Get(ctx, key).Bytes()
	if err != nil {
		return nil, err
	}

	var tenant Tenant
	if err := json.Unmarshal(data, &tenant); err != nil {
		return nil, err
	}
	return &tenant, nil
}

// SetTenant 设置租户缓存
func (c *RedisCache) SetTenant(ctx context.Context, key string, tenant *Tenant, ttl time.Duration) error {
	data, err := json.Marshal(tenant)
	if err != nil {
		return err
	}
	return c.client.Set(ctx, key, data, ttl).Err()
}

// GetTenants 获取租户列表缓存
func (c *RedisCache) GetTenants(ctx context.Context, key string) ([]*Tenant, error) {
	data, err := c.client.Get(ctx, key).Bytes()
	if err != nil {
		return nil, err
	}

	var tenants []*Tenant
	if err := json.Unmarshal(data, &tenants); err != nil {
		return nil, err
	}
	return tenants, nil
}

// SetTenants 设置租户列表缓存
func (c *RedisCache) SetTenants(ctx context.Context, key string, tenants []*Tenant, ttl time.Duration) error {
	data, err := json.Marshal(tenants)
	if err != nil {
		return err
	}
	return c.client.Set(ctx, key, data, ttl).Err()
}

// ==================== 会话缓存 ====================

// Session 会话缓存结构
type Session struct {
	SessionID string                 `json:"session_id"`
	TenantID  string                 `json:"tenant_id"`
	UserID    string                 `json:"user_id"`
	Data      map[string]interface{} `json:"data"`
	ExpiresAt time.Time              `json:"expires_at"`
}

// GetSession 获取会话
func (c *RedisCache) GetSession(ctx context.Context, sessionID string) (*Session, error) {
	key := fmt.Sprintf("session:%s", sessionID)
	data, err := c.client.Get(ctx, key).Bytes()
	if err != nil {
		return nil, err
	}

	var session Session
	if err := json.Unmarshal(data, &session); err != nil {
		return nil, err
	}
	return &session, nil
}

// SetSession 设置会话
func (c *RedisCache) SetSession(ctx context.Context, session *Session) error {
	key := fmt.Sprintf("session:%s", session.SessionID)
	data, err := json.Marshal(session)
	if err != nil {
		return err
	}
	ttl := time.Until(session.ExpiresAt)
	if ttl <= 0 {
		return fmt.Errorf("会话已过期")
	}
	return c.client.Set(ctx, key, data, ttl).Err()
}

// DeleteSession 删除会话
func (c *RedisCache) DeleteSession(ctx context.Context, sessionID string) error {
	key := fmt.Sprintf("session:%s", sessionID)
	return c.client.Del(ctx, key).Err()
}

// RefreshSession 刷新会话过期时间
func (c *RedisCache) RefreshSession(ctx context.Context, sessionID string, ttl time.Duration) error {
	key := fmt.Sprintf("session:%s", sessionID)
	return c.client.Expire(ctx, key, ttl).Err()
}

// ==================== 限流 ====================

// RateLimit 限流
func (c *RedisCache) RateLimit(ctx context.Context, key string, limit int, window time.Duration) (bool, error) {
	luaScript := `
		local key = KEYS[1]
		local limit = tonumber(ARGV[1])
		local window = tonumber(ARGV[2])
		local current = tonumber(redis.call('GET', key) or '0')
		if current >= limit then
			return 0
		end
		current = redis.call('INCR', key)
		if current == 1 then
			redis.call('EXPIRE', key, window)
		end
		return 1
	`

	result, err := c.client.Eval(ctx, luaScript, []string{key}, limit, int(window.Seconds())).Int()
	if err != nil {
		return false, err
	}
	return result == 1, nil
}

// ==================== 发布订阅 ====================

// Publish 发布消息
func (c *RedisCache) Publish(ctx context.Context, channel string, message interface{}) error {
	data, err := json.Marshal(message)
	if err != nil {
		return err
	}
	return c.client.Publish(ctx, channel, data).Err()
}

// Subscribe 订阅频道
func (c *RedisCache) Subscribe(ctx context.Context, channel string) *redis.PubSub {
	return c.client.Subscribe(ctx, channel)
}
