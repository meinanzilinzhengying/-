// Package auth 提供基于角色的权限控制(RBAC)
// 本文件实现RBAC权限引擎核心
package auth

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"time"

	"cloud-flow-agent/pkg/logger"
)

// RBACEngine RBAC权限引擎
type RBACEngine struct {
	mu          sync.RWMutex
	log         *logger.Logger
	
	// 存储（实际应用中应使用数据库）
	users       map[string]*User
	groups      map[string]*Group
	roles       map[string]*Role
	permissions map[string]*Permission
	dataScopes  map[string]*DataScope
	tenants     map[string]*Tenant
	
	// 关系映射
	userRoles   map[string]map[string]bool   // userID -> roleID -> true
	userGroups  map[string]map[string]bool   // userID -> groupID -> true
	groupRoles  map[string]map[string]bool   // groupID -> roleID -> true
	rolePerms   map[string]map[string]bool   // roleID -> permissionID -> true
	roleScopes  map[string]map[string]bool   // roleID -> scopeID -> true
	
	// 缓存
	permCache   *PermissionCache
}

// PermissionCache 权限缓存
type PermissionCache struct {
	mu       sync.RWMutex
	entries  map[string]*CacheEntry
	ttl      time.Duration
}

// CacheEntry 缓存条目
type CacheEntry struct {
	Value      interface{}
	ExpireTime time.Time
}

// NewPermissionCache 创建权限缓存
func NewPermissionCache(ttl time.Duration) *PermissionCache {
	return &PermissionCache{
		entries: make(map[string]*CacheEntry),
		ttl:     ttl,
	}
}

// Get 获取缓存值
func (c *PermissionCache) Get(key string) (interface{}, bool) {
	c.mu.RLock()
	defer c.mu.RUnlock()
	
	entry, ok := c.entries[key]
	if !ok {
		return nil, false
	}
	
	if time.Now().After(entry.ExpireTime) {
		return nil, false
	}
	
	return entry.Value, true
}

// Set 设置缓存值
func (c *PermissionCache) Set(key string, value interface{}) {
	c.mu.Lock()
	defer c.mu.Unlock()
	
	c.entries[key] = &CacheEntry{
		Value:      value,
		ExpireTime: time.Now().Add(c.ttl),
	}
}

// Clear 清空缓存
func (c *PermissionCache) Clear() {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.entries = make(map[string]*CacheEntry)
}

// NewRBACEngine 创建RBAC引擎
func NewRBACEngine(log *logger.Logger) *RBACEngine {
	engine := &RBACEngine{
		log:         log,
		users:       make(map[string]*User),
		groups:      make(map[string]*Group),
		roles:       make(map[string]*Role),
		permissions: make(map[string]*Permission),
		dataScopes:  make(map[string]*DataScope),
		tenants:     make(map[string]*Tenant),
		userRoles:   make(map[string]map[string]bool),
		userGroups:  make(map[string]map[string]bool),
		groupRoles:  make(map[string]map[string]bool),
		rolePerms:   make(map[string]map[string]bool),
		roleScopes:  make(map[string]map[string]bool),
		permCache:   NewPermissionCache(5 * time.Minute),
	}
	
	// 初始化系统预设权限
	engine.initSystemPermissions()
	
	// 初始化系统预设角色
	engine.initSystemRoles()
	
	return engine
}

// ==================== 系统预设 ====================

// initSystemPermissions 初始化系统预设权限
func (e *RBACEngine) initSystemPermissions() {
	// 告警模块权限
	e.registerPermission(&Permission{
		ID:          "perm_alert_view",
		Code:        "alert:view",
		Name:        "查看告警",
		Module:      ModuleAlert,
		Action:      ActionView,
		Resource:    "alert",
		Description: "查看告警列表和详情",
		Status:      1,
		CreatedAt:   time.Now(),
	})
	
	e.registerPermission(&Permission{
		ID:          "perm_alert_edit",
		Code:        "alert:edit",
		Name:        "编辑告警",
		Module:      ModuleAlert,
		Action:      ActionEdit,
		Resource:    "alert",
		Description: "编辑告警配置和处理告警",
		Status:      1,
		CreatedAt:   time.Now(),
	})
	
	e.registerPermission(&Permission{
		ID:          "perm_alert_manage",
		Code:        "alert:manage",
		Name:        "管理告警",
		Module:      ModuleAlert,
		Action:      ActionManage,
		Resource:    "alert",
		Description: "管理告警规则、模板等",
		Status:      1,
		CreatedAt:   time.Now(),
	})
	
	// 资产模块权限
	e.registerPermission(&Permission{
		ID:          "perm_asset_view",
		Code:        "asset:view",
		Name:        "查看资产",
		Module:      ModuleAsset,
		Action:      ActionView,
		Resource:    "asset",
		Description: "查看资产列表和详情",
		Status:      1,
		CreatedAt:   time.Now(),
	})
	
	e.registerPermission(&Permission{
		ID:          "perm_asset_edit",
		Code:        "asset:edit",
		Name:        "编辑资产",
		Module:      ModuleAsset,
		Action:      ActionEdit,
		Resource:    "asset",
		Description: "添加、编辑、删除资产",
		Status:      1,
		CreatedAt:   time.Now(),
	})
	
	e.registerPermission(&Permission{
		ID:          "perm_asset_manage",
		Code:        "asset:manage",
		Name:        "管理资产",
		Module:      ModuleAsset,
		Action:      ActionManage,
		Resource:    "asset",
		Description: "管理资产分组、导入导出",
		Status:      1,
		CreatedAt:   time.Now(),
	})
	
	// 仪表盘权限
	e.registerPermission(&Permission{
		ID:          "perm_dashboard_view",
		Code:        "dashboard:view",
		Name:        "查看仪表盘",
		Module:      ModuleDashboard,
		Action:      ActionView,
		Resource:    "dashboard",
		Description: "查看仪表盘",
		Status:      1,
		CreatedAt:   time.Now(),
	})
	
	e.registerPermission(&Permission{
		ID:          "perm_dashboard_edit",
		Code:        "dashboard:edit",
		Name:        "编辑仪表盘",
		Module:      ModuleDashboard,
		Action:      ActionEdit,
		Resource:    "dashboard",
		Description: "编辑仪表盘配置",
		Status:      1,
		CreatedAt:   time.Now(),
	})
	
	// 报表权限
	e.registerPermission(&Permission{
		ID:          "perm_report_view",
		Code:        "report:view",
		Name:        "查看报表",
		Module:      ModuleReport,
		Action:      ActionView,
		Resource:    "report",
		Description: "查看报表",
		Status:      1,
		CreatedAt:   time.Now(),
	})
	
	e.registerPermission(&Permission{
		ID:          "perm_report_export",
		Code:        "report:export",
		Name:        "导出报表",
		Module:      ModuleReport,
		Action:      ActionExport,
		Resource:    "report",
		Description: "导出报表数据",
		Status:      1,
		CreatedAt:   time.Now(),
	})
	
	// 配置权限
	e.registerPermission(&Permission{
		ID:          "perm_config_view",
		Code:        "config:view",
		Name:        "查看配置",
		Module:      ModuleConfig,
		Action:      ActionView,
		Resource:    "config",
		Description: "查看系统配置",
		Status:      1,
		CreatedAt:   time.Now(),
	})
	
	e.registerPermission(&Permission{
		ID:          "perm_config_edit",
		Code:        "config:edit",
		Name:        "编辑配置",
		Module:      ModuleConfig,
		Action:      ActionEdit,
		Resource:    "config",
		Description: "编辑系统配置",
		Status:      1,
		CreatedAt:   time.Now(),
	})
	
	// 用户管理权限
	e.registerPermission(&Permission{
		ID:          "perm_user_view",
		Code:        "user:view",
		Name:        "查看用户",
		Module:      ModuleUser,
		Action:      ActionView,
		Resource:    "user",
		Description: "查看用户列表",
		Status:      1,
		CreatedAt:   time.Now(),
	})
	
	e.registerPermission(&Permission{
		ID:          "perm_user_edit",
		Code:        "user:edit",
		Name:        "编辑用户",
		Module:      ModuleUser,
		Action:      ActionEdit,
		Resource:    "user",
		Description: "添加、编辑、删除用户",
		Status:      1,
		CreatedAt:   time.Now(),
	})
	
	e.registerPermission(&Permission{
		ID:          "perm_user_manage",
		Code:        "user:manage",
		Name:        "管理用户",
		Module:      ModuleUser,
		Action:      ActionManage,
		Resource:    "user",
		Description: "管理用户组、角色分配",
		Status:      1,
		CreatedAt:   time.Now(),
	})
	
	// 租户管理权限（仅超级管理员）
	e.registerPermission(&Permission{
		ID:          "perm_tenant_manage",
		Code:        "tenant:manage",
		Name:        "管理租户",
		Module:      ModuleTenant,
		Action:      ActionManage,
		Resource:    "tenant",
		Description: "管理租户（仅超级管理员）",
		Status:      1,
		CreatedAt:   time.Now(),
	})
}

// initSystemRoles 初始化系统预设角色
func (e *RBACEngine) initSystemRoles() {
	// 超级管理员
	superAdmin := &Role{
		ID:          "role_super_admin",
		TenantID:    "", // 系统级角色，无租户限制
		Name:        "超级管理员",
		Code:        RoleCodeSuperAdmin,
		Description: "系统超级管理员，拥有所有权限",
		Type:        0, // 系统预设
		Status:      1,
		CreatedAt:   time.Now(),
		UpdatedAt:   time.Now(),
	}
	e.roles[superAdmin.ID] = superAdmin
	
	// 为超级管理员分配所有权限
	e.rolePerms[superAdmin.ID] = make(map[string]bool)
	for permID := range e.permissions {
		e.rolePerms[superAdmin.ID][permID] = true
	}
	
	// 租户管理员
	tenantAdmin := &Role{
		ID:          "role_tenant_admin",
		TenantID:    "", // 模板角色，创建租户时复制
		Name:        "租户管理员",
		Code:        RoleCodeTenantAdmin,
		Description: "租户管理员，管理租户内所有资源",
		Type:        0,
		Status:      1,
		CreatedAt:   time.Now(),
		UpdatedAt:   time.Now(),
	}
	e.roles[tenantAdmin.ID] = tenantAdmin
	
	// 租户管理员权限（除租户管理外的所有权限）
	e.rolePerms[tenantAdmin.ID] = make(map[string]bool)
	for permID, perm := range e.permissions {
		if perm.Module != ModuleTenant {
			e.rolePerms[tenantAdmin.ID][permID] = true
		}
	}
	
	// 运维人员
	operator := &Role{
		ID:          "role_operator",
		TenantID:    "",
		Name:        "运维人员",
		Code:        RoleCodeOperator,
		Description: "日常运维人员",
		Type:        0,
		Status:      1,
		CreatedAt:   time.Now(),
		UpdatedAt:   time.Now(),
	}
	e.roles[operator.ID] = operator
	
	// 运维人员权限
	e.rolePerms[operator.ID] = make(map[string]bool)
	operatorPerms := []string{
		"perm_alert_view", "perm_alert_edit",
		"perm_asset_view", "perm_asset_edit",
		"perm_dashboard_view", "perm_dashboard_edit",
		"perm_report_view", "perm_report_export",
		"perm_config_view",
	}
	for _, permID := range operatorPerms {
		e.rolePerms[operator.ID][permID] = true
	}
	
	// 只读用户
	viewer := &Role{
		ID:          "role_viewer",
		TenantID:    "",
		Name:        "只读用户",
		Code:        RoleCodeViewer,
		Description: "只读访问权限",
		Type:        0,
		Status:      1,
		CreatedAt:   time.Now(),
		UpdatedAt:   time.Now(),
	}
	e.roles[viewer.ID] = viewer
	
	// 只读用户权限
	e.rolePerms[viewer.ID] = make(map[string]bool)
	viewerPerms := []string{
		"perm_alert_view",
		"perm_asset_view",
		"perm_dashboard_view",
		"perm_report_view",
	}
	for _, permID := range viewerPerms {
		e.rolePerms[viewer.ID][permID] = true
	}
	
	// 访客
	guest := &Role{
		ID:          "role_guest",
		TenantID:    "",
		Name:        "访客",
		Code:        RoleCodeGuest,
		Description: "访客权限，仅查看公开信息",
		Type:        0,
		Status:      1,
		CreatedAt:   time.Now(),
		UpdatedAt:   time.Now(),
	}
	e.roles[guest.ID] = guest
	
	// 访客权限
	e.rolePerms[guest.ID] = make(map[string]bool)
	e.rolePerms[guest.ID]["perm_dashboard_view"] = true
}

// ==================== 权限检查实现 ====================

// CheckPermission 检查用户是否有指定权限
func (e *RBACEngine) CheckPermission(userID, permissionCode string) (bool, error) {
	// 检查缓存
	cacheKey := fmt.Sprintf("perm:%s:%s", userID, permissionCode)
	if cached, ok := e.permCache.Get(cacheKey); ok {
		return cached.(bool), nil
	}
	
	// 获取用户
	user := e.GetUser(userID)
	if user == nil {
		return false, fmt.Errorf("用户不存在: %s", userID)
	}
	
	// 获取用户所有权限
	perms := e.GetUserPermissions(userID)
	
	// 检查权限
	hasPerm := false
	for _, perm := range perms {
		if perm.Code == permissionCode || perm.Code == "*" {
			hasPerm = true
			break
		}
	}
	
	// 缓存结果
	e.permCache.Set(cacheKey, hasPerm)
	
	return hasPerm, nil
}

// CheckPermissionInTenant 检查用户在指定租户中的权限
func (e *RBACEngine) CheckPermissionInTenant(userID, tenantID, permissionCode string) (bool, error) {
	// 获取用户
	user := e.GetUser(userID)
	if user == nil {
		return false, fmt.Errorf("用户不存在: %s", userID)
	}
	
	// 检查租户隔离
	if user.TenantID != "" && user.TenantID != tenantID {
		// 用户不属于该租户，检查是否为超级管理员
		if !e.IsSuperAdmin(userID) {
			return false, nil
		}
	}
	
	return e.CheckPermission(userID, permissionCode)
}

// CheckDataScope 检查用户是否有数据范围权限
func (e *RBACEngine) CheckDataScope(userID string, scope DataScopeType, resourceID string) (bool, error) {
	// 获取用户数据范围
	scopes := e.GetUserDataScopes(userID)
	
	for _, s := range scopes {
		// 全部数据权限
		if s.Type == DataScopeTypeAll {
			return true, nil
		}
		
		// 匹配范围类型
		if s.Type == scope {
			// 检查具体值
			if len(s.Values) == 0 {
				return true, nil
			}
			for _, v := range s.Values {
				if v == resourceID {
					return true, nil
				}
			}
		}
	}
	
	return false, nil
}

// IsSuperAdmin 检查用户是否为超级管理员
func (e *RBACEngine) IsSuperAdmin(userID string) bool {
	roles := e.GetUserRoles(userID)
	for _, role := range roles {
		if role.Code == RoleCodeSuperAdmin {
			return true
		}
	}
	return false
}

// IsTenantAdmin 检查用户是否为租户管理员
func (e *RBACEngine) IsTenantAdmin(userID string) bool {
	roles := e.GetUserRoles(userID)
	for _, role := range roles {
		if role.Code == RoleCodeTenantAdmin || role.Code == RoleCodeSuperAdmin {
			return true
		}
	}
	return false
}

// ==================== 查询方法 ====================

// GetUser 获取用户
func (e *RBACEngine) GetUser(userID string) *User {
	e.mu.RLock()
	defer e.mu.RUnlock()
	return e.users[userID]
}

// GetUserByUsername 根据用户名获取用户
func (e *RBACEngine) GetUserByUsername(tenantID, username string) *User {
	e.mu.RLock()
	defer e.mu.RUnlock()
	
	for _, user := range e.users {
		if user.TenantID == tenantID && user.Username == username {
			return user
		}
	}
	return nil
}

// GetUserRoles 获取用户角色
func (e *RBACEngine) GetUserRoles(userID string) []*Role {
	e.mu.RLock()
	defer e.mu.RUnlock()
	
	roles := make([]*Role, 0)
	
	// 直接分配的角色
	if userRoles, ok := e.userRoles[userID]; ok {
		for roleID := range userRoles {
			if role := e.roles[roleID]; role != nil {
				roles = append(roles, role)
			}
		}
	}
	
	// 通过用户组继承的角色
	if userGroups, ok := e.userGroups[userID]; ok {
		for groupID := range userGroups {
			if groupRoles, ok := e.groupRoles[groupID]; ok {
				for roleID := range groupRoles {
					if role := e.roles[roleID]; role != nil {
						// 去重
						exists := false
						for _, r := range roles {
							if r.ID == role.ID {
								exists = true
								break
							}
						}
						if !exists {
							roles = append(roles, role)
						}
					}
				}
			}
		}
	}
	
	return roles
}

// GetUserPermissions 获取用户权限
func (e *RBACEngine) GetUserPermissions(userID string) []*Permission {
	roles := e.GetUserRoles(userID)
	
	permMap := make(map[string]*Permission)
	
	for _, role := range roles {
		if rolePerms, ok := e.rolePerms[role.ID]; ok {
			for permID := range rolePerms {
				if perm := e.permissions[permID]; perm != nil {
					permMap[permID] = perm
				}
			}
		}
	}
	
	perms := make([]*Permission, 0, len(permMap))
	for _, perm := range permMap {
		perms = append(perms, perm)
	}
	
	return perms
}

// GetUserDataScopes 获取用户数据范围
func (e *RBACEngine) GetUserDataScopes(userID string) []DataScopeInfo {
	roles := e.GetUserRoles(userID)
	
	scopes := make([]DataScopeInfo, 0)
	scopeMap := make(map[DataScopeType]bool)
	
	for _, role := range roles {
		if roleScopes, ok := e.roleScopes[role.ID]; ok {
			for scopeID := range roleScopes {
				if scope := e.dataScopes[scopeID]; scope != nil {
					info := DataScopeInfo{
						Type:   scope.Type,
						Values: parseScopeValues(scope.ScopeValue),
					}
					
					// 全部数据权限优先级最高
					if scope.Type == DataScopeTypeAll {
						return []DataScopeInfo{{Type: DataScopeTypeAll}}
					}
					
					if !scopeMap[scope.Type] {
						scopes = append(scopes, info)
						scopeMap[scope.Type] = true
					}
				}
			}
		}
	}
	
	return scopes
}

// BuildUserContext 构建用户上下文
func (e *RBACEngine) BuildUserContext(userID string) (*UserContext, error) {
	user := e.GetUser(userID)
	if user == nil {
		return nil, fmt.Errorf("用户不存在: %s", userID)
	}
	
	roles := e.GetUserRoles(userID)
	roleCodes := make([]string, 0, len(roles))
	for _, role := range roles {
		roleCodes = append(roleCodes, role.Code)
	}
	
	perms := e.GetUserPermissions(userID)
	permCodes := make([]string, 0, len(perms))
	for _, perm := range perms {
		permCodes = append(permCodes, perm.Code)
	}
	
	scopes := e.GetUserDataScopes(userID)
	
	return &UserContext{
		UserID:      userID,
		TenantID:    user.TenantID,
		Username:    user.Username,
		Roles:       roleCodes,
		Permissions: permCodes,
		DataScopes:  scopes,
	}, nil
}

// ==================== 注册方法 ====================

func (e *RBACEngine) registerPermission(perm *Permission) {
	e.permissions[perm.ID] = perm
}

func (e *RBACEngine) parseScopeValues(value string) []string {
	if value == "" {
		return nil
	}
	return strings.Split(value, ",")
}

// ClearCache 清空权限缓存
func (e *RBACEngine) ClearCache() {
	e.permCache.Clear()
}
