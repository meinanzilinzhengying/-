//go:build linux

// Package balancer 提供负载均衡和多实例协调
package balancer

import (
	"context"
	"fmt"
	"hash/crc32"
	"sync"
	"sync/atomic"
	"time"

	"cloud-flow-edge/edge/internal/cache"
	"cloud-flow-edge/edge/internal/config"
)

// LoadBalancer 负载均衡器
type LoadBalancer struct {
	config *config.LoadBalancerConfig
	cache  *cache.RedisCache
	
	// 实例状态
	mu        sync.RWMutex
	instances map[string]*Instance
	
	// 策略
	strategy Strategy
	
	// 轮询计数器
	roundRobinCounter atomic.Uint64
	
	// 停止信号
	stopCh chan struct{}
}

// Instance 后端实例
type Instance struct {
	ID         string            `json:"id"`
	Endpoint   string            `json:"endpoint"`
	Weight     int               `json:"weight"`
	Healthy    bool              `json:"healthy"`
	ConnCount  int64             `json:"conn_count"`
	Metadata   map[string]string `json:"metadata"`
	LastActive time.Time         `json:"last_active"`
	FailCount  int               `json:"fail_count"`
	SuccessCount int             `json:"success_count"`
}

// Strategy 负载均衡策略
type Strategy interface {
	Select(instances []*Instance, key string) (*Instance, error)
	Name() string
}

// NewLoadBalancer 创建负载均衡器
func NewLoadBalancer(cfg *config.LoadBalancerConfig, redisCache *cache.RedisCache) *LoadBalancer {
	lb := &LoadBalancer{
		config:    cfg,
		cache:     redisCache,
		instances: make(map[string]*Instance),
		stopCh:    make(chan struct{}),
	}
	
	// 设置策略
	switch cfg.Strategy {
	case "round_robin":
		lb.strategy = &RoundRobinStrategy{}
	case "consistent_hash":
		lb.strategy = &ConsistentHashStrategy{}
	default:
		lb.strategy = &LeastConnStrategy{}
	}
	
	return lb
}

// Start 启动负载均衡器
func (lb *LoadBalancer) Start(ctx context.Context) error {
	// 启动健康检查
	go lb.healthCheckLoop(ctx)
	
	// 启动实例同步
	go lb.syncInstancesLoop(ctx)
	
	return nil
}

// Stop 停止负载均衡器
func (lb *LoadBalancer) Stop() {
	close(lb.stopCh)
}

// RegisterInstance 注册本地实例到Redis
func (lb *LoadBalancer) RegisterInstance(ctx context.Context, instanceID, endpoint string) error {
	if lb.cache == nil {
		return fmt.Errorf("cache not available")
	}
	
	return lb.cache.RegisterInstance(ctx, "edge", instanceID, endpoint, 30*time.Second)
}

// UnregisterInstance 注销实例
func (lb *LoadBalancer) UnregisterInstance(ctx context.Context, instanceID string) error {
	if lb.cache == nil {
		return nil
	}
	
	return lb.cache.UnregisterInstance(ctx, "edge", instanceID)
}

// Heartbeat 发送心跳
func (lb *LoadBalancer) Heartbeat(ctx context.Context, instanceID string) error {
	if lb.cache == nil {
		return nil
	}
	
	return lb.cache.Heartbeat(ctx, "edge", instanceID, 30*time.Second)
}

// SelectInstance 选择实例（用于Agent连接）
func (lb *LoadBalancer) SelectInstance(agentID string) (*Instance, error) {
	lb.mu.RLock()
	instances := make([]*Instance, 0, len(lb.instances))
	for _, inst := range lb.instances {
		if inst.Healthy {
			instances = append(instances, inst)
		}
	}
	lb.mu.RUnlock()
	
	if len(instances) == 0 {
		return nil, fmt.Errorf("no healthy instances available")
	}
	
	return lb.strategy.Select(instances, agentID)
}

// GetInstance 获取指定实例
func (lb *LoadBalancer) GetInstance(instanceID string) (*Instance, bool) {
	lb.mu.RLock()
	defer lb.mu.RUnlock()
	inst, ok := lb.instances[instanceID]
	return inst, ok
}

// GetAllInstances 获取所有实例
func (lb *LoadBalancer) GetAllInstances() []*Instance {
	lb.mu.RLock()
	defer lb.mu.RUnlock()
	
	instances := make([]*Instance, 0, len(lb.instances))
	for _, inst := range lb.instances {
		instances = append(instances, inst)
	}
	return instances
}

// UpdateInstanceStatus 更新实例状态
func (lb *LoadBalancer) UpdateInstanceStatus(instanceID string, healthy bool) {
	lb.mu.Lock()
	defer lb.mu.Unlock()
	
	if inst, ok := lb.instances[instanceID]; ok {
		inst.Healthy = healthy
		if healthy {
			inst.SuccessCount++
			inst.FailCount = 0
		} else {
			inst.FailCount++
		}
	}
}

// IncrementConnCount 增加连接计数
func (lb *LoadBalancer) IncrementConnCount(instanceID string) {
	lb.mu.RLock()
	if inst, ok := lb.instances[instanceID]; ok {
		atomic.AddInt64(&inst.ConnCount, 1)
	}
	lb.mu.RUnlock()
}

// DecrementConnCount 减少连接计数
func (lb *LoadBalancer) DecrementConnCount(instanceID string) {
	lb.mu.RLock()
	if inst, ok := lb.instances[instanceID]; ok {
		atomic.AddInt64(&inst.ConnCount, -1)
	}
	lb.mu.RUnlock()
}

// ==================== 后台任务 ====================

// healthCheckLoop 健康检查循环
func (lb *LoadBalancer) healthCheckLoop(ctx context.Context) {
	ticker := time.NewTicker(lb.config.HealthCheckInterval)
	defer ticker.Stop()
	
	for {
		select {
		case <-ctx.Done():
			return
		case <-lb.stopCh:
			return
		case <-ticker.C:
			lb.performHealthChecks(ctx)
		}
	}
}

// performHealthChecks 执行健康检查
func (lb *LoadBalancer) performHealthChecks(ctx context.Context) {
	instances := lb.GetAllInstances()
	
	for _, inst := range instances {
		// 简化实现：检查最后活跃时间
		if time.Since(inst.LastActive) > lb.config.HealthCheckInterval*3 {
			lb.UpdateInstanceStatus(inst.ID, false)
		}
	}
}

// syncInstancesLoop 同步实例列表循环
func (lb *LoadBalancer) syncInstancesLoop(ctx context.Context) {
	ticker := time.NewTicker(10 * time.Second)
	defer ticker.Stop()
	
	// 立即执行一次
	lb.syncInstances(ctx)
	
	for {
		select {
		case <-ctx.Done():
			return
		case <-lb.stopCh:
			return
		case <-ticker.C:
			lb.syncInstances(ctx)
		}
	}
}

// syncInstances 同步实例列表
func (lb *LoadBalancer) syncInstances(ctx context.Context) {
	if lb.cache == nil {
		return
	}
	
	instances, err := lb.cache.GetInstances(ctx, "edge")
	if err != nil {
		return
	}
	
	lb.mu.Lock()
	defer lb.mu.Unlock()
	
	// 更新实例列表
	currentIDs := make(map[string]bool)
	for _, info := range instances {
		id := info["id"]
		currentIDs[id] = true
		
		if existing, ok := lb.instances[id]; ok {
			// 更新现有实例
			existing.Endpoint = info["endpoint"]
			existing.LastActive = time.Now()
		} else {
			// 添加新实例
			lb.instances[id] = &Instance{
				ID:         id,
				Endpoint:   info["endpoint"],
				Weight:     1,
				Healthy:    true,
				Metadata:   info,
				LastActive: time.Now(),
			}
		}
	}
	
	// 移除不存在的实例
	for id := range lb.instances {
		if !currentIDs[id] {
			delete(lb.instances, id)
		}
	}
}

// ==================== 负载均衡策略 ====================

// RoundRobinStrategy 轮询策略
type RoundRobinStrategy struct {
	counter atomic.Uint64
}

// Name 返回策略名称
func (s *RoundRobinStrategy) Name() string {
	return "round_robin"
}

// Select 选择实例
func (s *RoundRobinStrategy) Select(instances []*Instance, key string) (*Instance, error) {
	if len(instances) == 0 {
		return nil, fmt.Errorf("no instances available")
	}
	
	idx := s.counter.Add(1) % uint64(len(instances))
	return instances[idx], nil
}

// LeastConnStrategy 最少连接策略
type LeastConnStrategy struct{}

// Name 返回策略名称
func (s *LeastConnStrategy) Name() string {
	return "least_conn"
}

// Select 选择实例
func (s *LeastConnStrategy) Select(instances []*Instance, key string) (*Instance, error) {
	if len(instances) == 0 {
		return nil, fmt.Errorf("no instances available")
	}
	
	var selected *Instance
	minConn := int64(^uint64(0) >> 1) // MaxInt64
	
	for _, inst := range instances {
		connCount := atomic.LoadInt64(&inst.ConnCount)
		if connCount < minConn {
			minConn = connCount
			selected = inst
		}
	}
	
	return selected, nil
}

// ConsistentHashStrategy 一致性哈希策略
type ConsistentHashStrategy struct{}

// Name 返回策略名称
func (s *ConsistentHashStrategy) Name() string {
	return "consistent_hash"
}

// Select 选择实例
func (s *ConsistentHashStrategy) Select(instances []*Instance, key string) (*Instance, error) {
	if len(instances) == 0 {
		return nil, fmt.Errorf("no instances available")
	}
	
	// 使用CRC32计算哈希
	hash := crc32.ChecksumIEEE([]byte(key))
	idx := hash % uint32(len(instances))
	
	return instances[idx], nil
}

// ==================== Agent会话亲和性 ====================

// SessionAffinity 会话亲和性管理器
type SessionAffinity struct {
	cache *cache.RedisCache
}

// NewSessionAffinity 创建会话亲和性管理器
func NewSessionAffinity(cache *cache.RedisCache) *SessionAffinity {
	return &SessionAffinity{cache: cache}
}

// BindAgentToInstance 绑定Agent到实例
func (sa *SessionAffinity) BindAgentToInstance(ctx context.Context, agentID, instanceID string, ttl time.Duration) error {
	if sa.cache == nil {
		return nil
	}
	return sa.cache.RegisterAgentSession(ctx, agentID, instanceID, ttl)
}

// GetAgentInstance 获取Agent绑定的实例
func (sa *SessionAffinity) GetAgentInstance(ctx context.Context, agentID string) (string, error) {
	if sa.cache == nil {
		return "", fmt.Errorf("cache not available")
	}
	return sa.cache.GetAgentSession(ctx, agentID)
}

// RemoveAgentBinding 移除Agent绑定
func (sa *SessionAffinity) RemoveAgentBinding(ctx context.Context, agentID string) error {
	if sa.cache == nil {
		return nil
	}
	return sa.cache.RemoveAgentSession(ctx, agentID)
}
