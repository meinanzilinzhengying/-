//go:build linux

// Package storage 提供无状态存储管理器
package storage

import (
	"context"
	"fmt"
	"time"

	"cloud-flow-center/internal/cache"
	"cloud-flow-agent/pkg/logger"
)

// StatelessManager 无状态存储管理器
// 统一管理数据库和缓存，支持多实例无状态运行
type StatelessManager struct {
	store *Store
	cache *cache.RedisCache
	log   *logger.Logger
}

// NewStatelessManager 创建无状态管理器
func NewStatelessManager(store *Store, cache *cache.RedisCache, log *logger.Logger) *StatelessManager {
	return &StatelessManager{
		store: store,
		cache: cache,
		log:   log,
	}
}

// GetStore 获取存储层
func (m *StatelessManager) GetStore() *Store {
	return m.store
}

// GetCache 获取缓存层
func (m *StatelessManager) GetCache() *cache.RedisCache {
	return m.cache
}

// ==================== 告警操作（带缓存） ====================

// GetAlert 获取告警（先查缓存）
func (m *StatelessManager) GetAlert(ctx context.Context, id string) (*Alert, error) {
	// 先查缓存
	alert, err := m.cache.GetAlert(ctx, id)
	if err == nil && alert != nil {
		return alert, nil
	}

	// 缓存未命中，查数据库
	alert, err = m.store.GetAlertByID(ctx, id)
	if err != nil {
		return nil, err
	}

	// 回填缓存
	if alert != nil {
		_ = m.cache.SetAlert(ctx, alert, 5*time.Minute)
	}

	return alert, nil
}

// GetActiveAlerts 获取活跃告警（带缓存）
func (m *StatelessManager) GetActiveAlerts(ctx context.Context, tenantID string) ([]*Alert, error) {
	cacheKey := fmt.Sprintf("alerts:active:%s", tenantID)

	// 先查缓存
	alerts, err := m.cache.GetAlerts(ctx, cacheKey)
	if err == nil && len(alerts) > 0 {
		return alerts, nil
	}

	// 缓存未命中，查数据库
	alerts, err = m.store.GetActiveAlerts(ctx, tenantID)
	if err != nil {
		return nil, err
	}

	// 回填缓存
	if len(alerts) > 0 {
		_ = m.cache.SetAlerts(ctx, cacheKey, alerts, 30*time.Second)
	}

	return alerts, nil
}

// CreateAlert 创建告警（双写）
func (m *StatelessManager) CreateAlert(ctx context.Context, alert *Alert) error {
	// 写入数据库
	if err := m.store.CreateAlert(ctx, alert); err != nil {
		return err
	}

	// 更新缓存
	_ = m.cache.SetAlert(ctx, alert, 5*time.Minute)

	// 清除活跃告警缓存
	_ = m.cache.Delete(ctx, fmt.Sprintf("alerts:active:%s", alert.TenantID))

	return nil
}

// UpdateAlert 更新告警（双写）
func (m *StatelessManager) UpdateAlert(ctx context.Context, alert *Alert) error {
	// 写入数据库
	if err := m.store.UpdateAlert(ctx, alert); err != nil {
		return err
	}

	// 更新缓存
	_ = m.cache.SetAlert(ctx, alert, 5*time.Minute)

	// 清除活跃告警缓存
	_ = m.cache.Delete(ctx, fmt.Sprintf("alerts:active:%s", alert.TenantID))

	return nil
}

// ResolveAlert 解决告警
func (m *StatelessManager) ResolveAlert(ctx context.Context, id string) error {
	now := time.Now()

	// 获取告警
	alert, err := m.store.GetAlertByID(ctx, id)
	if err != nil {
		return err
	}
	if alert == nil {
		return fmt.Errorf("告警不存在: %s", id)
	}

	alert.Status = "resolved"
	alert.ResolvedAt = &now
	alert.Duration = int64(now.Sub(alert.FiredAt).Seconds())

	return m.UpdateAlert(ctx, alert)
}

// GetAlertByID 直接从数据库获取
func (s *Store) GetAlertByID(ctx context.Context, id string) (*Alert, error) {
	row := s.db.QueryRowContext(ctx,
		`SELECT id, tenant_id, rule_id, rule_name, level, status, fingerprint,
		metric, value, threshold, labels, annotations, fired_at, resolved_at, duration
		FROM alerts WHERE id = $1`, id)

	var a Alert
	err := row.Scan(&a.ID, &a.TenantID, &a.RuleID, &a.RuleName, &a.Level, &a.Status,
		&a.Fingerprint, &a.Metric, &a.Value, &a.Threshold, &a.Labels, &a.Annotations,
		&a.FiredAt, &a.ResolvedAt, &a.Duration)
	if err != nil {
		return nil, err
	}
	return &a, nil
}

// ==================== 分布式锁（多实例协调） ====================

// AcquireLock 获取分布式锁
func (m *StatelessManager) AcquireLock(ctx context.Context, lockKey string, ttl time.Duration) (bool, error) {
	// 先尝试Redis分布式锁
	acquired, err := m.cache.AcquireLock(ctx, lockKey, m.store.GetInstanceID(), ttl)
	if err == nil && acquired {
		return true, nil
	}

	// Redis锁失败，回退到数据库锁
	return m.store.AcquireLock(ctx, lockKey, ttl)
}

// ReleaseLock 释放分布式锁
func (m *StatelessManager) ReleaseLock(ctx context.Context, lockKey string) error {
	// 释放Redis锁
	_ = m.cache.ReleaseLock(ctx, lockKey, m.store.GetInstanceID())

	// 释放数据库锁
	return m.store.ReleaseLock(ctx, lockKey)
}

// ==================== 租户操作 ====================

// GetTenant 获取租户
func (m *StatelessManager) GetTenant(ctx context.Context, name string) (*Tenant, error) {
	cacheKey := fmt.Sprintf("tenant:%s", name)

	// 先查缓存
	tenant, err := m.cache.GetTenant(ctx, cacheKey)
	if err == nil && tenant != nil {
		return tenant, nil
	}

	// 缓存未命中，查数据库
	tenant, err = m.store.GetTenant(ctx, name)
	if err != nil {
		return nil, err
	}

	// 回填缓存
	if tenant != nil {
		_ = m.cache.SetTenant(ctx, cacheKey, tenant, 10*time.Minute)
	}

	return tenant, nil
}

// ListTenants 列出所有租户
func (m *StatelessManager) ListTenants(ctx context.Context) ([]*Tenant, error) {
	cacheKey := "tenants:all"

	// 先查缓存
	tenants, err := m.cache.GetTenants(ctx, cacheKey)
	if err == nil && len(tenants) > 0 {
		return tenants, nil
	}

	// 缓存未命中，查数据库
	tenants, err = m.store.ListTenants(ctx)
	if err != nil {
		return nil, err
	}

	// 回填缓存
	if len(tenants) > 0 {
		_ = m.cache.SetTenants(ctx, cacheKey, tenants, 5*time.Minute)
	}

	return tenants, nil
}

// CreateTenant 创建租户
func (m *StatelessManager) CreateTenant(ctx context.Context, name, displayName string) (string, error) {
	// 创建租户
	id, err := m.store.CreateTenant(ctx, name, displayName)
	if err != nil {
		return "", err
	}

	// 清除租户列表缓存
	_ = m.cache.Delete(ctx, "tenants:all")

	return id, nil
}

// Tenant 租户
type Tenant struct {
	ID          string
	Name        string
	DisplayName string
	Status      string
	Quota       string
	CreatedAt   time.Time
	UpdatedAt   time.Time
}

// ==================== 实例协调 ====================

// RegisterInstance 注册实例
func (m *StatelessManager) RegisterInstance(ctx context.Context, instanceType, endpoint string) error {
	return m.store.RegisterInstance(ctx, instanceType, endpoint)
}

// Heartbeat 发送心跳
func (m *StatelessManager) Heartbeat(ctx context.Context) error {
	return m.store.Heartbeat(ctx)
}

// GetActiveInstances 获取活跃实例
func (m *StatelessManager) GetActiveInstances(ctx context.Context, instanceType string) ([]*Instance, error) {
	return m.store.GetActiveInstances(ctx, instanceType)
}

// ==================== 配置操作 ====================

// GetConfig 获取配置
func (m *StatelessManager) GetConfig(ctx context.Context, key string) (string, error) {
	// 先查缓存
	value, err := m.cache.Get(ctx, key)
	if err == nil && value != "" {
		return value, nil
	}

	// 缓存未命中，查数据库
	value, _, err = m.store.GetConfig(ctx, key)
	if err != nil {
		return "", err
	}

	// 回填缓存
	if value != "" {
		_ = m.cache.Set(ctx, key, value, 1*time.Minute)
	}

	return value, nil
}

// SetConfig 设置配置
func (m *StatelessManager) SetConfig(ctx context.Context, key, value string) error {
	// 更新数据库
	if err := m.store.SetConfig(ctx, key, value); err != nil {
		return err
	}

	// 更新缓存
	_ = m.cache.Set(ctx, key, value, 1*time.Minute)

	return nil
}

// ==================== 通用缓存操作 ====================

// Get 获取缓存值
func (m *StatelessManager) Get(ctx context.Context, key string) (string, error) {
	return m.cache.Get(ctx, key)
}

// Set 设置缓存值
func (m *StatelessManager) Set(ctx context.Context, key string, value interface{}, ttl time.Duration) error {
	return m.cache.Set(ctx, key, value, ttl)
}

// Delete 删除缓存
func (m *StatelessManager) Delete(ctx context.Context, key string) error {
	return m.cache.Delete(ctx, key)
}
