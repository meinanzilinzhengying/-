//go:build linux

// Package cache 提供Redis分布式缓存实现
package cache

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/redis/go-redis/v9"
	
	"cloud-flow-edge/edge/internal/config"
)

// RedisCache Redis分布式缓存
type RedisCache struct {
	client    *redis.Client
	config    *config.RedisConfig
	keyPrefix string
}

// CacheItem 缓存条目
type CacheItem struct {
	Key       string                 `json:"key"`
	Data      []byte                 `json:"data"`
	Timestamp int64                  `json:"timestamp"`
	Size      int                    `json:"size"`
	Priority  int                    `json:"priority"`
	Retries   int                    `json:"retries"`
	Metadata  map[string]string      `json:"metadata"`
}

// CacheStats 缓存统计
type CacheStats struct {
	TotalItems   uint64 `json:"total_items"`
	Hits         uint64 `json:"hits"`
	Misses       uint64 `json:"misses"`
	CurrentSize  int64  `json:"current_size"`
	CurrentItems int32  `json:"current_items"`
}

// NewRedisCache 创建Redis缓存
func NewRedisCache(cfg *config.RedisConfig) (*RedisCache, error) {
	if !cfg.Enabled {
		return nil, fmt.Errorf("redis is disabled")
	}

	client := redis.NewClient(&redis.Options{
		Addr:         cfg.Address,
		Password:     cfg.Password,
		DB:           cfg.DB,
		PoolSize:     cfg.PoolSize,
		MinIdleConns: cfg.MinIdleConns,
		MaxRetries:   cfg.MaxRetries,
		DialTimeout:  cfg.DialTimeout,
		ReadTimeout:  cfg.ReadTimeout,
		WriteTimeout: cfg.WriteTimeout,
	})

	// 测试连接
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	
	if err := client.Ping(ctx).Err(); err != nil {
		return nil, fmt.Errorf("failed to connect to redis: %w", err)
	}

	return &RedisCache{
		client:    client,
		config:    cfg,
		keyPrefix: cfg.KeyPrefix,
	}, nil
}

// ==================== 基础操作 ====================

// Get 获取缓存值
func (c *RedisCache) Get(ctx context.Context, key string) ([]byte, error) {
	fullKey := c.keyPrefix + key
	data, err := c.client.Get(ctx, fullKey).Bytes()
	if err == redis.Nil {
		return nil, fmt.Errorf("cache miss")
	}
	if err != nil {
		return nil, err
	}
	return data, nil
}

// Set 设置缓存值
func (c *RedisCache) Set(ctx context.Context, key string, value []byte, ttl time.Duration) error {
	fullKey := c.keyPrefix + key
	return c.client.Set(ctx, fullKey, value, ttl).Err()
}

// Delete 删除缓存
func (c *RedisCache) Delete(ctx context.Context, key string) error {
	fullKey := c.keyPrefix + key
	return c.client.Del(ctx, fullKey).Err()
}

// Exists 检查键是否存在
func (c *RedisCache) Exists(ctx context.Context, key string) (bool, error) {
	fullKey := c.keyPrefix + key
	n, err := c.client.Exists(ctx, fullKey).Result()
	return n > 0, err
}

// TTL 获取剩余过期时间
func (c *RedisCache) TTL(ctx context.Context, key string) (time.Duration, error) {
	fullKey := c.keyPrefix + key
	return c.client.TTL(ctx, fullKey).Result()
}

// ==================== 结构化数据操作 ====================

// GetJSON 获取JSON对象
func (c *RedisCache) GetJSON(ctx context.Context, key string, dest interface{}) error {
	data, err := c.Get(ctx, key)
	if err != nil {
		return err
	}
	return json.Unmarshal(data, dest)
}

// SetJSON 设置JSON对象
func (c *RedisCache) SetJSON(ctx context.Context, key string, value interface{}, ttl time.Duration) error {
	data, err := json.Marshal(value)
	if err != nil {
		return err
	}
	return c.Set(ctx, key, data, ttl)
}

// ==================== 缓存条目操作 ====================

// GetItem 获取缓存条目
func (c *RedisCache) GetItem(ctx context.Context, key string) (*CacheItem, error) {
	var item CacheItem
	if err := c.GetJSON(ctx, "item:"+key, &item); err != nil {
		return nil, err
	}
	return &item, nil
}

// SetItem 设置缓存条目
func (c *RedisCache) SetItem(ctx context.Context, item *CacheItem, ttl time.Duration) error {
	return c.SetJSON(ctx, "item:"+item.Key, item, ttl)
}

// GetQueue 获取待发送队列（按优先级排序）
func (c *RedisCache) GetQueue(ctx context.Context, queueName string, limit int) ([]*CacheItem, error) {
	queueKey := c.keyPrefix + "queue:" + queueName
	
	// 使用Sorted Set按优先级排序
	members, err := c.client.ZRevRangeWithScores(ctx, queueKey, 0, int64(limit-1)).Result()
	if err != nil {
		return nil, err
	}

	items := make([]*CacheItem, 0, len(members))
	for _, member := range members {
		item, err := c.GetItem(ctx, member.Member.(string))
		if err != nil {
			continue
		}
		items = append(items, item)
	}

	return items, nil
}

// AddToQueue 添加到队列
func (c *RedisCache) AddToQueue(ctx context.Context, queueName string, item *CacheItem) error {
	queueKey := c.keyPrefix + "queue:" + queueName
	memberKey := item.Key
	
	// 先存储条目
	if err := c.SetItem(ctx, item, 24*time.Hour); err != nil {
		return err
	}

	// 添加到有序集合（分数为优先级）
	return c.client.ZAdd(ctx, queueKey, redis.Z{
		Score:  float64(item.Priority),
		Member: memberKey,
	}).Err()
}

// RemoveFromQueue 从队列移除
func (c *RedisCache) RemoveFromQueue(ctx context.Context, queueName string, keys []string) error {
	queueKey := c.keyPrefix + "queue:" + queueName
	
	// 从有序集合移除
	members := make([]interface{}, len(keys))
	for i, key := range keys {
		members[i] = key
	}
	
	if err := c.client.ZRem(ctx, queueKey, members...).Err(); err != nil {
		return err
	}

	// 删除条目
	for _, key := range keys {
		_ = c.Delete(ctx, "item:"+key)
	}

	return nil
}

// GetQueueLength 获取队列长度
func (c *RedisCache) GetQueueLength(ctx context.Context, queueName string) (int64, error) {
	queueKey := c.keyPrefix + "queue:" + queueName
	return c.client.ZCard(ctx, queueKey).Result()
}

// ==================== 分布式锁 ====================

// AcquireLock 获取分布式锁
func (c *RedisCache) AcquireLock(ctx context.Context, lockKey string, instanceID string, ttl time.Duration) (bool, error) {
	fullKey := c.keyPrefix + "lock:" + lockKey
	
	// 使用 SET NX EX 原子操作
	ok, err := c.client.SetNX(ctx, fullKey, instanceID, ttl).Result()
	if err != nil {
		return false, err
	}
	
	return ok, nil
}

// ReleaseLock 释放分布式锁
func (c *RedisCache) ReleaseLock(ctx context.Context, lockKey string, instanceID string) error {
	fullKey := c.keyPrefix + "lock:" + lockKey
	
	// 使用Lua脚本确保原子性检查并删除
	script := `
		if redis.call("get", KEYS[1]) == ARGV[1] then
			return redis.call("del", KEYS[1])
		else
			return 0
		end
	`
	
	return c.client.Eval(ctx, script, []string{fullKey}, instanceID).Err()
}

// RenewLock 续期分布式锁
func (c *RedisCache) RenewLock(ctx context.Context, lockKey string, instanceID string, ttl time.Duration) error {
	fullKey := c.keyPrefix + "lock:" + lockKey
	
	script := `
		if redis.call("get", KEYS[1]) == ARGV[1] then
			return redis.call("expire", KEYS[1], ARGV[2])
		else
			return 0
		end
	`
	
	return c.client.Eval(ctx, script, []string{fullKey}, instanceID, ttl.Seconds()).Err()
}

// ==================== 实例注册与发现 ====================

// RegisterInstance 注册实例
func (c *RedisCache) RegisterInstance(ctx context.Context, instanceType string, instanceID string, endpoint string, ttl time.Duration) error {
	key := fmt.Sprintf("%sinstance:%s:%s", c.keyPrefix, instanceType, instanceID)
	
	instanceInfo := map[string]interface{}{
		"id":        instanceID,
		"type":      instanceType,
		"endpoint":  endpoint,
		"timestamp": time.Now().Unix(),
	}
	
	data, _ := json.Marshal(instanceInfo)
	
	pipe := c.client.Pipeline()
	pipe.Set(ctx, key, data, ttl)
	pipe.SAdd(ctx, c.keyPrefix+"instances:"+instanceType, instanceID)
	
	_, err := pipe.Exec(ctx)
	return err
}

// UnregisterInstance 注销实例
func (c *RedisCache) UnregisterInstance(ctx context.Context, instanceType string, instanceID string) error {
	key := fmt.Sprintf("%sinstance:%s:%s", c.keyPrefix, instanceType, instanceID)
	
	pipe := c.client.Pipeline()
	pipe.Del(ctx, key)
	pipe.SRem(ctx, c.keyPrefix+"instances:"+instanceType, instanceID)
	
	_, err := pipe.Exec(ctx)
	return err
}

// GetInstances 获取活跃实例列表
func (c *RedisCache) GetInstances(ctx context.Context, instanceType string) ([]map[string]string, error) {
	setKey := c.keyPrefix + "instances:" + instanceType
	
	instanceIDs, err := c.client.SMembers(ctx, setKey).Result()
	if err != nil {
		return nil, err
	}

	instances := make([]map[string]string, 0, len(instanceIDs))
	for _, id := range instanceIDs {
		key := fmt.Sprintf("%sinstance:%s:%s", c.keyPrefix, instanceType, id)
		data, err := c.client.Get(ctx, key).Bytes()
		if err != nil {
			continue
		}

		var info map[string]string
		if err := json.Unmarshal(data, &info); err != nil {
			continue
		}
		instances = append(instances, info)
	}

	return instances, nil
}

// Heartbeat 发送心跳
func (c *RedisCache) Heartbeat(ctx context.Context, instanceType string, instanceID string, ttl time.Duration) error {
	key := fmt.Sprintf("%sinstance:%s:%s", c.keyPrefix, instanceType, instanceID)
	return c.client.Expire(ctx, key, ttl).Err()
}

// ==================== Agent会话管理 ====================

// RegisterAgentSession 注册Agent会话
func (c *RedisCache) RegisterAgentSession(ctx context.Context, agentID string, edgeID string, ttl time.Duration) error {
	key := c.keyPrefix + "agent:" + agentID
	return c.client.Set(ctx, key, edgeID, ttl).Err()
}

// GetAgentSession 获取Agent会话
func (c *RedisCache) GetAgentSession(ctx context.Context, agentID string) (string, error) {
	key := c.keyPrefix + "agent:" + agentID
	return c.client.Get(ctx, key).Result()
}

// RemoveAgentSession 移除Agent会话
func (c *RedisCache) RemoveAgentSession(ctx context.Context, agentID string) error {
	key := c.keyPrefix + "agent:" + agentID
	return c.client.Del(ctx, key).Err()
}

// GetAgentCount 获取连接的Agent数量
func (c *RedisCache) GetAgentCount(ctx context.Context) (int64, error) {
	pattern := c.keyPrefix + "agent:*"
	keys, err := c.client.Keys(ctx, pattern).Result()
	if err != nil {
		return 0, err
	}
	return int64(len(keys)), nil
}

// ==================== 统计与监控 ====================

// GetStats 获取缓存统计
func (c *RedisCache) GetStats(ctx context.Context) (*CacheStats, error) {
	info, err := c.client.Info(ctx, "stats").Result()
	if err != nil {
		return nil, err
	}

	// 解析Redis INFO统计
	stats := &CacheStats{}
	_ = info // 简化实现

	return stats, nil
}

// Ping 检查连接
func (c *RedisCache) Ping(ctx context.Context) error {
	return c.client.Ping(ctx).Err()
}

// Close 关闭连接
func (c *RedisCache) Close() error {
	return c.client.Close()
}

// Client 获取原始Redis客户端
func (c *RedisCache) Client() *redis.Client {
	return c.client
}
