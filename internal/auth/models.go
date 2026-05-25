// Package auth 提供基于角色的权限控制(RBAC)
// 支持用户、用户组、角色管理
// 按数据范围（资产、业务组）分配可见权限
// 按功能模块分配操作权限（查看/编辑/管理）
// 支持租户隔离
package auth

import (
	"fmt"
	"time"
)

// ==================== 租户模型 ====================

// Tenant 租户
type Tenant struct {
	ID          string    `json:"id" gorm:"primaryKey"`
	Name        string    `json:"name" gorm:"not null"`
	Code        string    `json:"code" gorm:"uniqueIndex;not null"` // 租户编码，用于隔离
	Description string    `json:"description"`
	Status      int       `json:"status"` // 0:禁用 1:启用
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
	ExpiredAt   *time.Time `json:"expired_at,omitempty"` // 过期时间
	
	// 配额限制
	MaxUsers   int `json:"max_users"`   // 最大用户数
	MaxAssets  int `json:"max_assets"`  // 最大资产数
	MaxAlerts  int `json:"max_alerts"`  // 最大告警数
}

// TableName 表名
func (Tenant) TableName() string {
	return "tenants"
}

// IsActive 检查租户是否活跃
func (t *Tenant) IsActive() bool {
	if t.Status != 1 {
		return false
	}
	if t.ExpiredAt != nil && time.Now().After(*t.ExpiredAt) {
		return false
	}
	return true
}

// ==================== 用户模型 ====================

// User 用户
type User struct {
	ID        string    `json:"id" gorm:"primaryKey"`
	TenantID  string    `json:"tenant_id" gorm:"index;not null"` // 租户ID，用于隔离
	Username  string    `json:"username" gorm:"index;not null"`
	Email     string    `json:"email" gorm:"index"`
	Phone     string    `json:"phone"`
	Password  string    `json:"-" gorm:"not null"` // 密码不序列化
	RealName  string    `json:"real_name"`
	Avatar    string    `json:"avatar"`
	Status    int       `json:"status"` // 0:禁用 1:启用 2:锁定
	LastLogin *time.Time `json:"last_login,omitempty"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
	
	// 关联
	Roles []Role `json:"roles,omitempty" gorm:"many2many:user_roles;"`
	Groups []Group `json:"groups,omitempty" gorm:"many2many:user_groups;"`
}

// TableName 表名
func (User) TableName() string {
	return "users"
}

// IsActive 检查用户是否活跃
func (u *User) IsActive() bool {
	return u.Status == 1
}

// GetTenantID 获取租户ID（实现TenantScoped接口）
func (u *User) GetTenantID() string {
	return u.TenantID
}

// ==================== 用户组模型 ====================

// Group 用户组
type Group struct {
	ID          string    `json:"id" gorm:"primaryKey"`
	TenantID    string    `json:"tenant_id" gorm:"index;not null"`
	Name        string    `json:"name" gorm:"not null"`
	Description string    `json:"description"`
	ParentID    *string   `json:"parent_id,omitempty"` // 父组ID，支持层级
	Status      int       `json:"status"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
	
	// 关联
	Users []User `json:"users,omitempty" gorm:"many2many:user_groups;"`
	Roles []Role `json:"roles,omitempty" gorm:"many2many:group_roles;"`
}

// TableName 表名
func (Group) TableName() string {
	return "groups"
}

// GetTenantID 获取租户ID
func (g *Group) GetTenantID() string {
	return g.TenantID
}

// ==================== 角色模型 ====================

// Role 角色
type Role struct {
	ID          string    `json:"id" gorm:"primaryKey"`
	TenantID    string    `json:"tenant_id" gorm:"index;not null"`
	Name        string    `json:"name" gorm:"not null"`
	Code        string    `json:"code" gorm:"index;not null"` // 角色编码，如 admin, operator, viewer
	Description string    `json:"description"`
	Type        int       `json:"type"` // 0:系统预设 1:自定义
	Status      int       `json:"status"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
	
	// 关联
	Permissions []Permission `json:"permissions,omitempty" gorm:"many2many:role_permissions;"`
	DataScopes  []DataScope  `json:"data_scopes,omitempty" gorm:"many2many:role_data_scopes;"`
}

// TableName 表名
func (Role) TableName() string {
	return "roles"
}

// GetTenantID 获取租户ID
func (r *Role) GetTenantID() string {
	return r.TenantID
}

// IsSystemRole 检查是否为系统预设角色
func (r *Role) IsSystemRole() bool {
	return r.Type == 0
}

// 预设角色编码
const (
	RoleCodeSuperAdmin = "super_admin" // 超级管理员（跨租户）
	RoleCodeTenantAdmin = "tenant_admin" // 租户管理员
	RoleCodeOperator    = "operator"     // 运维人员
	RoleCodeViewer      = "viewer"       // 只读用户
	RoleCodeGuest       = "guest"        // 访客
)

// ==================== 权限模型 ====================

// Permission 权限
type Permission struct {
	ID          string    `json:"id" gorm:"primaryKey"`
	TenantID    string    `json:"tenant_id" gorm:"index"` // 空表示系统权限
	Name        string    `json:"name" gorm:"not null"`
	Code        string    `json:"code" gorm:"uniqueIndex;not null"` // 权限编码，如 alert:view, alert:edit
	Module      string    `json:"module" gorm:"index"` // 所属模块
	Action      string    `json:"action"` // 操作类型: view, edit, delete, manage
	Resource    string    `json:"resource"` // 资源类型
	Description string    `json:"description"`
	Status      int       `json:"status"`
	CreatedAt   time.Time `json:"created_at"`
}

// TableName 表名
func (Permission) TableName() string {
	return "permissions"
}

// 权限操作类型
const (
	ActionView   = "view"   // 查看
	ActionCreate = "create" // 创建
	ActionEdit   = "edit"   // 编辑
	ActionDelete = "delete" // 删除
	ActionManage = "manage" // 管理（全部权限）
	ActionExport = "export" // 导出
	ActionImport = "import" // 导入
)

// 功能模块
const (
	ModuleAlert     = "alert"     // 告警管理
	ModuleAsset     = "asset"     // 资产管理
	ModuleDashboard = "dashboard" // 仪表盘
	ModuleReport    = "report"    // 报表
	ModuleConfig    = "config"    // 系统配置
	ModuleUser      = "user"      // 用户管理
	ModuleTenant    = "tenant"    // 租户管理（仅超级管理员）
)

// ==================== 数据范围模型 ====================

// DataScope 数据范围
type DataScope struct {
	ID          string    `json:"id" gorm:"primaryKey"`
	TenantID    string    `json:"tenant_id" gorm:"index;not null"`
	Name        string    `json:"name" gorm:"not null"`
	Type        DataScopeType `json:"type" gorm:"not null"` // 范围类型
	ScopeValue  string    `json:"scope_value"` // 范围值，根据类型解析
	Description string    `json:"description"`
	Status      int       `json:"status"`
	CreatedAt   time.Time `json:"created_at"`
}

// TableName 表名
func (DataScope) TableName() string {
	return "data_scopes"
}

// DataScopeType 数据范围类型
type DataScopeType string

const (
	DataScopeTypeAll      DataScopeType = "all"      // 全部数据
	DataScopeTypeSelf     DataScopeType = "self"     // 仅自己
	DataScopeTypeGroup    DataScopeType = "group"    // 本组
	DataScopeTypeDept     DataScopeType = "dept"     // 本部门
	DataScopeTypeCustom   DataScopeType = "custom"   // 自定义
	DataScopeTypeBusiness DataScopeType = "business" // 业务组
	DataScopeTypeAsset    DataScopeType = "asset"    // 指定资产
)

// DataScopeValue 数据范围值（JSON解析）
type DataScopeValue struct {
	AssetIDs    []string `json:"asset_ids,omitempty"`    // 资产ID列表
	BusinessIDs []string `json:"business_ids,omitempty"` // 业务组ID列表
	GroupIDs    []string `json:"group_ids,omitempty"`    // 用户组ID列表
}

// ==================== 业务组模型 ====================

// BusinessGroup 业务组
type BusinessGroup struct {
	ID          string    `json:"id" gorm:"primaryKey"`
	TenantID    string    `json:"tenant_id" gorm:"index;not null"`
	Name        string    `json:"name" gorm:"not null"`
	Code        string    `json:"code" gorm:"index"`
	Description string    `json:"description"`
	ParentID    *string   `json:"parent_id,omitempty"`
	ManagerID   *string   `json:"manager_id,omitempty"` // 负责人
	Status      int       `json:"status"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}

// TableName 表名
func (BusinessGroup) TableName() string {
	return "business_groups"
}

// GetTenantID 获取租户ID
func (bg *BusinessGroup) GetTenantID() string {
	return bg.TenantID
}

// ==================== 资产模型 ====================

// Asset 资产（简化模型，实际可能更复杂）
type Asset struct {
	ID             string    `json:"id" gorm:"primaryKey"`
	TenantID       string    `json:"tenant_id" gorm:"index;not null"`
	Name           string    `json:"name" gorm:"not null"`
	Type           string    `json:"type"` // 资产类型
	IP             string    `json:"ip"`
	BusinessGroupID *string  `json:"business_group_id,omitempty"`
	Status         int       `json:"status"`
	CreatedAt      time.Time `json:"created_at"`
	UpdatedAt      time.Time `json:"updated_at"`
}

// TableName 表名
func (Asset) TableName() string {
	return "assets"
}

// GetTenantID 获取租户ID
func (a *Asset) GetTenantID() string {
	return a.TenantID
}

// ==================== 接口定义 ====================

// TenantScoped 租户隔离接口
// 所有需要租户隔离的模型都应实现此接口
type TenantScoped interface {
	GetTenantID() string
}

// PermissionChecker 权限检查接口
type PermissionChecker interface {
	// CheckPermission 检查用户是否有指定权限
	CheckPermission(userID, permissionCode string) (bool, error)
	
	// CheckPermissionInTenant 检查用户在指定租户中的权限
	CheckPermissionInTenant(userID, tenantID, permissionCode string) (bool, error)
	
	// CheckDataScope 检查用户是否有数据范围权限
	CheckDataScope(userID string, scope DataScopeType, resourceID string) (bool, error)
}

// RoleAssigner 角色分配接口
type RoleAssigner interface {
	// AssignRoleToUser 给用户分配角色
	AssignRoleToUser(userID, roleID string) error
	
	// AssignRoleToGroup 给用户组分配角色
	AssignRoleToGroup(groupID, roleID string) error
	
	// RemoveRoleFromUser 移除用户角色
	RemoveRoleFromUser(userID, roleID string) error
	
	// RemoveRoleFromGroup 移除用户组角色
	RemoveRoleFromGroup(groupID, roleID string) error
}

// ==================== 请求上下文 ====================

// UserContext 用户上下文
type UserContext struct {
	UserID    string   `json:"user_id"`
	TenantID  string   `json:"tenant_id"`
	Username  string   `json:"username"`
	Roles     []string `json:"roles"`     // 角色编码列表
	Permissions []string `json:"permissions"` // 权限编码列表
	DataScopes []DataScopeInfo `json:"data_scopes"` // 数据范围
}

// DataScopeInfo 数据范围信息
type DataScopeInfo struct {
	Type   DataScopeType `json:"type"`
	Values []string      `json:"values,omitempty"` // 具体的范围值
}

// IsSuperAdmin 检查是否为超级管理员
func (uc *UserContext) IsSuperAdmin() bool {
	for _, role := range uc.Roles {
		if role == RoleCodeSuperAdmin {
			return true
		}
	}
	return false
}

// IsTenantAdmin 检查是否为租户管理员
func (uc *UserContext) IsTenantAdmin() bool {
	for _, role := range uc.Roles {
		if role == RoleCodeTenantAdmin || role == RoleCodeSuperAdmin {
			return true
		}
	}
	return false
}

// HasPermission 检查是否有指定权限
func (uc *UserContext) HasPermission(permissionCode string) bool {
	// 超级管理员拥有所有权限
	if uc.IsSuperAdmin() {
		return true
	}
	
	for _, p := range uc.Permissions {
		if p == permissionCode || p == "*" {
			return true
		}
	}
	return false
}

// HasAnyPermission 检查是否有任意一个权限
func (uc *UserContext) HasAnyPermission(permissionCodes ...string) bool {
	for _, code := range permissionCodes {
		if uc.HasPermission(code) {
			return true
		}
	}
	return false
}

// HasDataScope 检查是否有数据范围权限
func (uc *UserContext) HasDataScope(scopeType DataScopeType, resourceTenantID string) bool {
	// 超级管理员可以访问所有租户数据
	if uc.IsSuperAdmin() {
		return true
	}
	
	// 检查租户隔离
	if resourceTenantID != "" && resourceTenantID != uc.TenantID {
		return false
	}
	
	// 检查数据范围
	for _, scope := range uc.DataScopes {
		if scope.Type == DataScopeTypeAll {
			return true
		}
		if scope.Type == scopeType {
			return true
		}
	}
	return false
}

// ContextKey 上下文键类型
type ContextKey string

const (
	// ContextKeyUser 用户上下文键
	ContextKeyUser ContextKey = "user_context"
	// ContextKeyTenant 租户上下文键
	ContextKeyTenant ContextKey = "tenant_context"
)
