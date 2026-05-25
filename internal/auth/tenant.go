// Package auth 提供基于角色的权限控制(RBAC)
// 本文件实现租户隔离功能
package auth

import (
	"context"
	"fmt"
	"sync"
	"time"

	"cloud-flow-agent/pkg/logger"
)

// TenantManager 租户管理器
type TenantManager struct {
	mu       sync.RWMutex
	log      *logger.Logger
	tenants  map[string]*Tenant          // tenantID -> Tenant
	byCode   map[string]string           // tenantCode -> tenantID
	contexts map[string]*TenantContext   // tenantID -> TenantContext
}

// TenantContext 租户上下文
type TenantContext struct {
	Tenant      *Tenant                `json:"tenant"`
	Settings    map[string]interface{} `json:"settings"`
	Features    []string               `json:"features"`    // 启用的功能列表
	Limits      TenantLimits           `json:"limits"`
	Stats       TenantStats            `json:"stats"`
	LastUpdated time.Time              `json:"last_updated"`
}

// TenantLimits 租户配额限制
type TenantLimits struct {
	MaxUsers      int `json:"max_users"`
	MaxAssets     int `json:"max_assets"`
	MaxAlerts     int `json:"max_alerts"`
	MaxStorageGB  int `json:"max_storage_gb"`
	MaxAPIQPS     int `json:"max_api_qps"`
}

// TenantStats 租户统计
type TenantStats struct {
	UserCount   int `json:"user_count"`
	AssetCount  int `json:"asset_count"`
	AlertCount  int `json:"alert_count"`
	StorageUsed int64 `json:"storage_used"`
}

// NewTenantManager 创建租户管理器
func NewTenantManager(log *logger.Logger) *TenantManager {
	return &TenantManager{
		log:      log,
		tenants:  make(map[string]*Tenant),
		byCode:   make(map[string]string),
		contexts: make(map[string]*TenantContext),
	}
}

// CreateTenant 创建租户
func (m *TenantManager) CreateTenant(name, code, description string, limits *TenantLimits) (*Tenant, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	
	// 检查编码是否已存在
	if _, exists := m.byCode[code]; exists {
		return nil, fmt.Errorf("租户编码已存在: %s", code)
	}
	
	// 创建租户
	tenant := &Tenant{
		ID:          generateTenantID(),
		Name:        name,
		Code:        code,
		Description: description,
		Status:      1,
		CreatedAt:   time.Now(),
		UpdatedAt:   time.Now(),
	}
	
	// 设置默认配额
	if limits != nil {
		tenant.MaxUsers = limits.MaxUsers
		tenant.MaxAssets = limits.MaxAssets
		tenant.MaxAlerts = limits.MaxAlerts
	} else {
		tenant.MaxUsers = 100
		tenant.MaxAssets = 1000
		tenant.MaxAlerts = 10000
	}
	
	m.tenants[tenant.ID] = tenant
	m.byCode[code] = tenant.ID
	
	// 初始化租户上下文
	m.contexts[tenant.ID] = &TenantContext{
		Tenant:      tenant,
		Settings:    make(map[string]interface{}),
		Features:    []string{"alert", "asset", "dashboard", "report"},
		Limits:      m.getLimits(tenant),
		Stats:       TenantStats{},
		LastUpdated: time.Now(),
	}
	
	m.log.Infof("创建租户成功: %s (%s)", name, code)
	
	return tenant, nil
}

// GetTenant 获取租户
func (m *TenantManager) GetTenant(tenantID string) *Tenant {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.tenants[tenantID]
}

// GetTenantByCode 根据编码获取租户
func (m *TenantManager) GetTenantByCode(code string) *Tenant {
	m.mu.RLock()
	defer m.mu.RUnlock()
	
	if tenantID, ok := m.byCode[code]; ok {
		return m.tenants[tenantID]
	}
	return nil
}

// UpdateTenant 更新租户
func (m *TenantManager) UpdateTenant(tenantID string, updates map[string]interface{}) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	
	tenant, ok := m.tenants[tenantID]
	if !ok {
		return fmt.Errorf("租户不存在: %s", tenantID)
	}
	
	// 应用更新
	if name, ok := updates["name"].(string); ok {
		tenant.Name = name
	}
	if desc, ok := updates["description"].(string); ok {
		tenant.Description = desc
	}
	if status, ok := updates["status"].(int); ok {
		tenant.Status = status
	}
	
	tenant.UpdatedAt = time.Now()
	
	// 更新上下文
	if ctx, ok := m.contexts[tenantID]; ok {
		ctx.Tenant = tenant
		ctx.LastUpdated = time.Now()
	}
	
	return nil
}

// DeleteTenant 删除租户
func (m *TenantManager) DeleteTenant(tenantID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	
	tenant, ok := m.tenants[tenantID]
	if !ok {
		return fmt.Errorf("租户不存在: %s", tenantID)
	}
	
	delete(m.tenants, tenantID)
	delete(m.byCode, tenant.Code)
	delete(m.contexts, tenantID)
	
	m.log.Infof("删除租户: %s (%s)", tenant.Name, tenant.Code)
	
	return nil
}

// ListTenants 列出所有租户
func (m *TenantManager) ListTenants() []*Tenant {
	m.mu.RLock()
	defer m.mu.RUnlock()
	
	tenants := make([]*Tenant, 0, len(m.tenants))
	for _, t := range m.tenants {
		tenants = append(tenants, t)
	}
	
	return tenants
}

// GetTenantContext 获取租户上下文
func (m *TenantManager) GetTenantContext(tenantID string) *TenantContext {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.contexts[tenantID]
}

// UpdateTenantStats 更新租户统计
func (m *TenantManager) UpdateTenantStats(tenantID string, stats TenantStats) {
	m.mu.Lock()
	defer m.mu.Unlock()
	
	if ctx, ok := m.contexts[tenantID]; ok {
		ctx.Stats = stats
		ctx.LastUpdated = time.Now()
	}
}

// CheckTenantLimit 检查租户配额
func (m *TenantManager) CheckTenantLimit(tenantID string, limitType string, currentValue int) (bool, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	
	tenant, ok := m.tenants[tenantID]
	if !ok {
		return false, fmt.Errorf("租户不存在: %s", tenantID)
	}
	
	if !tenant.IsActive() {
		return false, fmt.Errorf("租户未激活或已过期")
	}
	
	var limit int
	switch limitType {
	case "users":
		limit = tenant.MaxUsers
	case "assets":
		limit = tenant.MaxAssets
	case "alerts":
		limit = tenant.MaxAlerts
	default:
		return true, nil
	}
	
	if limit <= 0 {
		return true, nil // 无限制
	}
	
	return currentValue < limit, nil
}

// getLimits 获取租户限制
func (m *TenantManager) getLimits(tenant *Tenant) TenantLimits {
	return TenantLimits{
		MaxUsers:  tenant.MaxUsers,
		MaxAssets: tenant.MaxAssets,
		MaxAlerts: tenant.MaxAlerts,
	}
}

// generateTenantID 生成租户ID
func generateTenantID() string {
	return fmt.Sprintf("tenant-%d", time.Now().UnixNano())
}

// ==================== 租户隔离过滤器 ====================

// TenantFilter 租户隔离过滤器
type TenantFilter struct {
	manager *TenantManager
}

// NewTenantFilter 创建租户过滤器
func NewTenantFilter(manager *TenantManager) *TenantFilter {
	return &TenantFilter{manager: manager}
}

// FilterByTenant 按租户过滤数据
func (f *TenantFilter) FilterByTenant(data []TenantScoped, tenantID string) []TenantScoped {
	result := make([]TenantScoped, 0)
	for _, item := range data {
		if item.GetTenantID() == tenantID {
			result = append(result, item)
		}
	}
	return result
}

// FilterByDataScope 按数据范围过滤
func (f *TenantFilter) FilterByDataScope(data []TenantScoped, userCtx *UserContext) []TenantScoped {
	// 超级管理员可以看到所有数据
	if userCtx.IsSuperAdmin() {
		return data
	}
	
	// 首先按租户过滤
	filtered := make([]TenantScoped, 0)
	for _, item := range data {
		if item.GetTenantID() == userCtx.TenantID {
			filtered = append(filtered, item)
		}
	}
	
	// 然后按数据范围过滤
	for _, scope := range userCtx.DataScopes {
		switch scope.Type {
		case DataScopeTypeAll:
			return filtered // 全部数据
		case DataScopeTypeSelf:
			// 仅自己的数据（需要数据项有用户ID字段，这里简化处理）
			return filtered
		default:
			// 其他范围类型需要具体实现
			return filtered
		}
	}
	
	return filtered
}

// ==================== 租户上下文管理 ====================

// TenantContextManager 租户上下文管理器
type TenantContextManager struct {
	mu       sync.RWMutex
	contexts map[string]context.Context // userID -> context
}

// NewTenantContextManager 创建租户上下文管理器
func NewTenantContextManager() *TenantContextManager {
	return &TenantContextManager{
		contexts: make(map[string]context.Context),
	}
}

// SetContext 设置用户上下文
func (m *TenantContextManager) SetContext(userID string, ctx context.Context) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.contexts[userID] = ctx
}

// GetContext 获取用户上下文
func (m *TenantContextManager) GetContext(userID string) (context.Context, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	ctx, ok := m.contexts[userID]
	return ctx, ok
}

// RemoveContext 移除用户上下文
func (m *TenantContextManager) RemoveContext(userID string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	delete(m.contexts, userID)
}

// WithTenantContext 将租户信息注入context
func WithTenantContext(ctx context.Context, tenantID string) context.Context {
	return context.WithValue(ctx, ContextKeyTenant, tenantID)
}

// GetTenantFromContext 从context获取租户ID
func GetTenantFromContext(ctx context.Context) string {
	if tenantID, ok := ctx.Value(ContextKeyTenant).(string); ok {
		return tenantID
	}
	return ""
}

// WithUserContext 将用户信息注入context
func WithUserContext(ctx context.Context, userCtx *UserContext) context.Context {
	return context.WithValue(ctx, ContextKeyUser, userCtx)
}

// GetUserFromContext 从context获取用户上下文
func GetUserFromContext(ctx context.Context) *UserContext {
	if userCtx, ok := ctx.Value(ContextKeyUser).(*UserContext); ok {
		return userCtx
	}
	return nil
}

// ==================== 多租户资源管理 ====================

// TenantResourceManager 租户资源管理器
// 用于管理租户隔离的资源
type TenantResourceManager struct {
	mu        sync.RWMutex
	resources map[string]map[string]interface{} // tenantID -> resourceID -> resource
}

// NewTenantResourceManager 创建租户资源管理器
func NewTenantResourceManager() *TenantResourceManager {
	return &TenantResourceManager{
		resources: make(map[string]map[string]interface{}),
	}
}

// AddResource 添加资源
func (m *TenantResourceManager) AddResource(tenantID string, resourceID string, resource interface{}) {
	m.mu.Lock()
	defer m.mu.Unlock()
	
	if _, ok := m.resources[tenantID]; !ok {
		m.resources[tenantID] = make(map[string]interface{})
	}
	
	m.resources[tenantID][resourceID] = resource
}

// GetResource 获取资源
func (m *TenantResourceManager) GetResource(tenantID string, resourceID string) (interface{}, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	
	if tenantResources, ok := m.resources[tenantID]; ok {
		resource, exists := tenantResources[resourceID]
		return resource, exists
	}
	
	return nil, false
}

// RemoveResource 移除资源
func (m *TenantResourceManager) RemoveResource(tenantID string, resourceID string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	
	if tenantResources, ok := m.resources[tenantID]; ok {
		delete(tenantResources, resourceID)
	}
}

// ListResources 列出租户的所有资源
func (m *TenantResourceManager) ListResources(tenantID string) []interface{} {
	m.mu.RLock()
	defer m.mu.RUnlock()
	
	if tenantResources, ok := m.resources[tenantID]; ok {
		resources := make([]interface{}, 0, len(tenantResources))
		for _, r := range tenantResources {
			resources = append(resources, r)
		}
		return resources
	}
	
	return nil
}

// ClearTenantResources 清空租户的所有资源
func (m *TenantResourceManager) ClearTenantResources(tenantID string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	delete(m.resources, tenantID)
}
