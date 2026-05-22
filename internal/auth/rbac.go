/*
 * Copyright (c) 2025 Yunlong Liao. All rights reserved.
 */

package auth

import (
	"context"
	"crypto/subtle"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"sync"
	"time"

	"cloud-flow-agent/pkg/logger"
)

// Permission 权限定义
type Permission string

const (
	// 系统管理权限
	PermSystemAdmin Permission = "system:admin"
	PermSystemRead  Permission = "system:read"
	PermSystemWrite Permission = "system:write"

	// 租户管理权限
	PermTenantCreate Permission = "tenant:create"
	PermTenantRead   Permission = "tenant:read"
	PermTenantUpdate Permission = "tenant:update"
	PermTenantDelete Permission = "tenant:delete"
	PermTenantManage Permission = "tenant:manage"

	// 用户管理权限
	PermUserCreate Permission = "user:create"
	PermUserRead   Permission = "user:read"
	PermUserUpdate Permission = "user:update"
	PermUserDelete Permission = "user:delete"
	PermUserManage Permission = "user:manage"

	// 用户组权限
	PermGroupCreate Permission = "group:create"
	PermGroupRead   Permission = "group:read"
	PermGroupUpdate Permission = "group:update"
	PermGroupDelete Permission = "group:delete"
	PermGroupManage Permission = "group:manage"

	// 资源权限
	PermResourceCreate Permission = "resource:create"
	PermResourceRead   Permission = "resource:read"
	PermResourceUpdate Permission = "resource:update"
	PermResourceDelete Permission = "resource:delete"
	PermResourceManage Permission = "resource:manage"

	// 数据采集权限
	PermDataCollect Permission = "data:collect"
	PermDataRead    Permission = "data:read"
	PermDataExport  Permission = "data:export"
	PermDataDelete  Permission = "data:delete"

	// 告警权限
	PermAlertCreate Permission = "alert:create"
	PermAlertRead   Permission = "alert:read"
	PermAlertUpdate Permission = "alert:update"
	PermAlertDelete Permission = "alert:delete"
	PermAlertManage Permission = "alert:manage"

	// 配置权限
	PermConfigRead  Permission = "config:read"
	PermConfigWrite Permission = "config:write"

	// 审计权限
	PermAuditRead Permission = "audit:read"
	PermAuditAll  Permission = "audit:all"
)

// Role 角色定义
type Role string

const (
	RoleSuperAdmin      Role = "super_admin"       // 超级管理员
	RoleTenantAdmin     Role = "tenant_admin"      // 租户管理员
	RoleTenantOperator  Role = "tenant_operator"   // 租户操作员
	RoleTenantViewer    Role = "tenant_viewer"     // 租户观察员
	RoleGroupAdmin      Role = "group_admin"       // 用户组管理员
	RoleGroupMember     Role = "group_member"      // 用户组成员
	RoleAuditor         Role = "auditor"           // 审计员
	RoleGuest           Role = "guest"             // 访客
)

// RolePermissions 角色权限映射
var RolePermissions = map[Role][]Permission{
	RoleSuperAdmin: {
		PermSystemAdmin, PermSystemRead, PermSystemWrite,
		PermTenantCreate, PermTenantRead, PermTenantUpdate, PermTenantDelete, PermTenantManage,
		PermUserCreate, PermUserRead, PermUserUpdate, PermUserDelete, PermUserManage,
		PermGroupCreate, PermGroupRead, PermGroupUpdate, PermGroupDelete, PermGroupManage,
		PermResourceCreate, PermResourceRead, PermResourceUpdate, PermResourceDelete, PermResourceManage,
		PermDataCollect, PermDataRead, PermDataExport, PermDataDelete,
		PermAlertCreate, PermAlertRead, PermAlertUpdate, PermAlertDelete, PermAlertManage,
		PermConfigRead, PermConfigWrite,
		PermAuditRead, PermAuditAll,
	},
	RoleTenantAdmin: {
		PermTenantRead, PermTenantUpdate, PermTenantManage,
		PermUserCreate, PermUserRead, PermUserUpdate, PermUserDelete, PermUserManage,
		PermGroupCreate, PermGroupRead, PermGroupUpdate, PermGroupDelete, PermGroupManage,
		PermResourceCreate, PermResourceRead, PermResourceUpdate, PermResourceDelete, PermResourceManage,
		PermDataCollect, PermDataRead, PermDataExport, PermDataDelete,
		PermAlertCreate, PermAlertRead, PermAlertUpdate, PermAlertDelete, PermAlertManage,
		PermConfigRead, PermConfigWrite,
		PermAuditRead,
	},
	RoleTenantOperator: {
		PermTenantRead,
		PermUserRead, PermUserUpdate,
		PermGroupRead,
		PermResourceCreate, PermResourceRead, PermResourceUpdate,
		PermDataCollect, PermDataRead, PermDataExport,
		PermAlertCreate, PermAlertRead, PermAlertUpdate,
		PermConfigRead,
	},
	RoleTenantViewer: {
		PermTenantRead,
		PermUserRead,
		PermGroupRead,
		PermResourceRead,
		PermDataRead,
		PermAlertRead,
		PermConfigRead,
	},
	RoleGroupAdmin: {
		PermGroupRead, PermGroupUpdate, PermGroupManage,
		PermUserRead, PermUserUpdate,
		PermResourceRead, PermResourceUpdate,
		PermDataRead, PermDataExport,
		PermAlertRead,
	},
	RoleGroupMember: {
		PermGroupRead,
		PermUserRead,
		PermResourceRead,
		PermDataRead,
		PermAlertRead,
	},
	RoleAuditor: {
		PermSystemRead,
		PermTenantRead,
		PermUserRead,
		PermGroupRead,
		PermResourceRead,
		PermDataRead,
		PermAlertRead,
		PermConfigRead,
		PermAuditRead, PermAuditAll,
	},
	RoleGuest: {
		PermResourceRead,
		PermDataRead,
	},
}

// Tenant 租户
type Tenant struct {
	ID          string       `json:"id"`
	Name        string       `json:"name"`
	Description string       `json:"description"`
	Status      TenantStatus `json:"status"`
	Quota       TenantQuota  `json:"quota"`
	Settings    map[string]string `json:"settings"`
	CreatedAt   time.Time    `json:"created_at"`
	UpdatedAt   time.Time    `json:"updated_at"`
	ExpiresAt   *time.Time   `json:"expires_at,omitempty"`
}

// TenantStatus 租户状态
type TenantStatus string

const (
	TenantStatusActive    TenantStatus = "active"
	TenantStatusSuspended TenantStatus = "suspended"
	TenantStatusDeleted   TenantStatus = "deleted"
)

// TenantQuota 租户配额
type TenantQuota struct {
	MaxUsers     int   `json:"max_users"`
	MaxGroups    int   `json:"max_groups"`
	MaxResources int   `json:"max_resources"`
	MaxDataSize  int64 `json:"max_data_size"`
	MaxBandwidth int64 `json:"max_bandwidth"`
	MaxAlerts    int   `json:"max_alerts"`
	MaxRetention int   `json:"max_retention"`
}

// DefaultTenantQuota 默认租户配额
func DefaultTenantQuota() TenantQuota {
	return TenantQuota{
		MaxUsers:     100,
		MaxGroups:    20,
		MaxResources: 1000,
		MaxDataSize:  100 * 1024 * 1024 * 1024,
		MaxBandwidth: 100 * 1024 * 1024,
		MaxAlerts:    1000,
		MaxRetention: 90,
	}
}

// User 用户
type User struct {
	ID           string       `json:"id"`
	Username     string       `json:"username"`
	PasswordHash string       `json:"-"`
	Email        string       `json:"email"`
	Phone        string       `json:"phone"`
	TenantID     string       `json:"tenant_id"`
	Roles        []Role       `json:"roles"`
	Groups       []string     `json:"groups"`
	Status       UserStatus   `json:"status"`
	LastLoginAt  *time.Time   `json:"last_login_at,omitempty"`
	CreatedAt    time.Time    `json:"created_at"`
	UpdatedAt    time.Time    `json:"updated_at"`
}

// UserStatus 用户状态
type UserStatus string

const (
	UserStatusActive   UserStatus = "active"
	UserStatusInactive UserStatus = "inactive"
	UserStatusLocked   UserStatus = "locked"
	UserStatusDeleted  UserStatus = "deleted"
)

// UserGroup 用户组
type UserGroup struct {
	ID          string       `json:"id"`
	Name        string       `json:"name"`
	Description string       `json:"description"`
	TenantID    string       `json:"tenant_id"`
	ParentID    string       `json:"parent_id,omitempty"`
	Members     []string     `json:"members"`
	Roles       []Role       `json:"roles"`
	Status      GroupStatus  `json:"status"`
	CreatedAt   time.Time    `json:"created_at"`
	UpdatedAt   time.Time    `json:"updated_at"`
}

// GroupStatus 用户组状态
type GroupStatus string

const (
	GroupStatusActive  GroupStatus = "active"
	GroupStatusDeleted GroupStatus = "deleted"
)

// RBACManager RBAC权限管理器
type RBACManager struct {
	tenants map[string]*Tenant
	users   map[string]*User
	groups  map[string]*UserGroup
	mu      sync.RWMutex
	config  *RBACConfig
}

// RBACConfig RBAC配置
type RBACConfig struct {
	Enabled          bool   `yaml:"enabled" json:"enabled"`
	DefaultTenantID  string `yaml:"default_tenant_id" json:"default_tenant_id"`
	AllowSelfSignup bool   `yaml:"allow_self_signup" json:"allow_self_signup"`
	Require MFA     bool   `yaml:"require_mfa" json:"require_mfa"`
	SessionTimeout   int    `yaml:"session_timeout" json:"session_timeout"`
	MaxLoginAttempts int    `yaml:"max_login_attempts" json:"max_login_attempts"`
}

// DefaultRBACConfig 默认配置
func DefaultRBACConfig() *RBACConfig {
	return &RBACConfig{
		Enabled:          true,
		DefaultTenantID:  "default",
		AllowSelfSignup: false,
		RequireMFA:      false,
		SessionTimeout:  3600,
		MaxLoginAttempts: 5,
	}
}

// NewRBACManager 创建RBAC管理器
func NewRBACManager(config *RBACConfig) *RBACManager {
	if config == nil {
		config = DefaultRBACConfig()
	}

	return &RBACManager{
		tenants: make(map[string]*Tenant),
		users:   make(map[string]*User),
		groups:  make(map[string]*UserGroup),
		config:  config,
	}
}

// GetUserPermissions 获取用户所有权限
func (r *RBACManager) GetUserPermissions(userID string) ([]Permission, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	user, exists := r.users[userID]
	if !exists {
		return nil, fmt.Errorf("user not found: %s", userID)
	}

	perms := make([]Permission, 0)
	permSet := make(map[Permission]bool)

	// 添加用户角色权限
	for _, role := range user.Roles {
		if perms, ok := RolePermissions[role]; ok {
			for _, p := range perms {
				permSet[p] = true
			}
		}
	}

	// 添加用户组权限
	for _, groupID := range user.Groups {
		if group, exists := r.groups[groupID]; exists {
			for _, role := range group.Roles {
				if perms, ok := RolePermissions[role]; ok {
					for _, p := range perms {
						permSet[p] = true
					}
				}
			}
		}
	}

	for p := range permSet {
		perms = append(perms, p)
	}

	return perms, nil
}

// HasPermission 检查用户是否有权限
func (r *RBACManager) HasPermission(userID string, perm Permission) (bool, error) {
	perms, err := r.GetUserPermissions(userID)
	if err != nil {
		return false, err
	}

	for _, p := range perms {
		if p == perm {
			return true, nil
		}
	}

	return false, nil
}

// HasAnyPermission 检查是否有任一权限
func (r *RBACManager) HasAnyPermission(userID string, perms ...Permission) (bool, error) {
	for _, perm := range perms {
		has, err := r.HasPermission(userID, perm)
		if err != nil {
			return false, err
		}
		if has {
			return true, nil
		}
	}
	return false, nil
}

// HasAllPermissions 检查是否拥有所有权限
func (r *RBACManager) HasAllPermissions(userID string, perms ...Permission) (bool, error) {
	for _, perm := range perms {
		has, err := r.HasPermission(userID, perm)
		if err != nil {
			return false, err
		}
		if !has {
			return false, nil
		}
	}
	return true, nil
}

// CanAccessTenant 检查是否能访问租户
func (r *RBACManager) CanAccessTenant(userID, tenantID string) (bool, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	user, exists := r.users[userID]
	if !exists {
		return false, fmt.Errorf("user not found: %s", userID)
	}

	// 超级管理员可以访问所有租户
	for _, role := range user.Roles {
		if role == RoleSuperAdmin {
			return true, nil
		}
	}

	// 检查用户是否属于该租户
	return user.TenantID == tenantID, nil
}

// CanAccessResource 检查是否能访问资源
func (r *RBACManager) CanAccessResource(userID, resourceTenantID string, perm Permission) (bool, error) {
	// 先检查租户访问权限
	canAccess, err := r.CanAccessTenant(userID, resourceTenantID)
	if err != nil {
		return false, err
	}
	if !canAccess {
		return false, nil
	}

	// 检查具体权限
	return r.HasPermission(userID, perm)
}

// CreateTenant 创建租户
func (r *RBACManager) CreateTenant(tenant *Tenant) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	if _, exists := r.tenants[tenant.ID]; exists {
		return fmt.Errorf("tenant already exists: %s", tenant.ID)
	}

	if tenant.Quota.MaxUsers == 0 {
		tenant.Quota = DefaultTenantQuota()
	}
	tenant.Status = TenantStatusActive
	tenant.CreatedAt = time.Now()
	tenant.UpdatedAt = time.Now()

	r.tenants[tenant.ID] = tenant
	logger.Infof("Created tenant: %s", tenant.ID)
	return nil
}

// GetTenant 获取租户
func (r *RBACManager) GetTenant(tenantID string) (*Tenant, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	tenant, exists := r.tenants[tenantID]
	if !exists {
		return nil, fmt.Errorf("tenant not found: %s", tenantID)
	}
	return tenant, nil
}

// UpdateTenant 更新租户
func (r *RBACManager) UpdateTenant(tenant *Tenant) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	if _, exists := r.tenants[tenant.ID]; !exists {
		return fmt.Errorf("tenant not found: %s", tenant.ID)
	}

	tenant.UpdatedAt = time.Now()
	r.tenants[tenant.ID] = tenant
	return nil
}

// DeleteTenant 删除租户
func (r *RBACManager) DeleteTenant(tenantID string) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	if _, exists := r.tenants[tenantID]; !exists {
		return fmt.Errorf("tenant not found: %s", tenantID)
	}

	r.tenants[tenantID].Status = TenantStatusDeleted
	return nil
}

// CreateUser 创建用户
func (r *RBACManager) CreateUser(user *User) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	if _, exists := r.users[user.ID]; exists {
		return fmt.Errorf("user already exists: %s", user.ID)
	}

	// 检查租户是否存在
	if _, exists := r.tenants[user.TenantID]; !exists {
		return fmt.Errorf("tenant not found: %s", user.TenantID)
	}

	// 检查租户配额
	tenant := r.tenants[user.TenantID]
	userCount := r.countTenantUsers(user.TenantID)
	if userCount >= tenant.Quota.MaxUsers {
		return fmt.Errorf("tenant user quota exceeded: %d/%d", userCount, tenant.Quota.MaxUsers)
	}

	if user.Status == "" {
		user.Status = UserStatusActive
	}
	user.CreatedAt = time.Now()
	user.UpdatedAt = time.Now()

	r.users[user.ID] = user
	logger.Infof("Created user: %s in tenant: %s", user.ID, user.TenantID)
	return nil
}

// countTenantUsers 统计租户用户数
func (r *RBACManager) countTenantUsers(tenantID string) int {
	count := 0
	for _, user := range r.users {
		if user.TenantID == tenantID && user.Status != UserStatusDeleted {
			count++
		}
	}
	return count
}

// GetUser 获取用户
func (r *RBACManager) GetUser(userID string) (*User, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	user, exists := r.users[userID]
	if !exists {
		return nil, fmt.Errorf("user not found: %s", userID)
	}
	return user, nil
}

// UpdateUser 更新用户
func (r *RBACManager) UpdateUser(user *User) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	if _, exists := r.users[user.ID]; !exists {
		return fmt.Errorf("user not found: %s", user.ID)
	}

	user.UpdatedAt = time.Now()
	r.users[user.ID] = user
	return nil
}

// DeleteUser 删除用户
func (r *RBACManager) DeleteUser(userID string) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	if _, exists := r.users[userID]; !exists {
		return fmt.Errorf("user not found: %s", userID)
	}

	r.users[userID].Status = UserStatusDeleted
	return nil
}

// CreateGroup 创建用户组
func (r *RBACManager) CreateGroup(group *UserGroup) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	if _, exists := r.groups[group.ID]; exists {
		return fmt.Errorf("group already exists: %s", group.ID)
	}

	// 检查租户是否存在
	if _, exists := r.tenants[group.TenantID]; !exists {
		return fmt.Errorf("tenant not found: %s", group.TenantID)
	}

	// 检查租户配额
	tenant := r.tenants[group.TenantID]
	groupCount := r.countTenantGroups(group.TenantID)
	if groupCount >= tenant.Quota.MaxGroups {
		return fmt.Errorf("tenant group quota exceeded: %d/%d", groupCount, tenant.Quota.MaxGroups)
	}

	group.Status = GroupStatusActive
	group.CreatedAt = time.Now()
	group.UpdatedAt = time.Now()

	r.groups[group.ID] = group
	logger.Infof("Created group: %s in tenant: %s", group.ID, group.TenantID)
	return nil
}

// countTenantGroups 统计租户组数
func (r *RBACManager) countTenantGroups(tenantID string) int {
	count := 0
	for _, group := range r.groups {
		if group.TenantID == tenantID && group.Status != GroupStatusDeleted {
			count++
		}
	}
	return count
}

// GetGroup 获取用户组
func (r *RBACManager) GetGroup(groupID string) (*UserGroup, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	group, exists := r.groups[groupID]
	if !exists {
		return nil, fmt.Errorf("group not found: %s", groupID)
	}
	return group, nil
}

// UpdateGroup 更新用户组
func (r *RBACManager) UpdateGroup(group *UserGroup) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	if _, exists := r.groups[group.ID]; !exists {
		return fmt.Errorf("group not found: %s", group.ID)
	}

	group.UpdatedAt = time.Now()
	r.groups[group.ID] = group
	return nil
}

// DeleteGroup 删除用户组
func (r *RBACManager) DeleteGroup(groupID string) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	if _, exists := r.groups[groupID]; !exists {
		return fmt.Errorf("group not found: %s", groupID)
	}

	r.groups[groupID].Status = GroupStatusDeleted
	return nil
}

// AddUserToGroup 添加用户到组
func (r *RBACManager) AddUserToGroup(userID, groupID string) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	user, uExists := r.users[userID]
	if !uExists {
		return fmt.Errorf("user not found: %s", userID)
	}

	group, gExists := r.groups[groupID]
	if !gExists {
		return fmt.Errorf("group not found: %s", groupID)
	}

	// 检查租户匹配
	if user.TenantID != group.TenantID {
		return fmt.Errorf("user and group must be in same tenant")
	}

	// 添加到组
	for _, m := range group.Members {
		if m == userID {
			return nil // 已在组中
		}
	}
	group.Members = append(group.Members, userID)

	// 添加组到用户
	for _, g := range user.Groups {
		if g == groupID {
			return nil
		}
	}
	user.Groups = append(user.Groups, groupID)

	group.UpdatedAt = time.Now()
	user.UpdatedAt = time.Now()

	return nil
}

// RemoveUserFromGroup 从组移除用户
func (r *RBACManager) RemoveUserFromGroup(userID, groupID string) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	user, uExists := r.users[userID]
	if !uExists {
		return fmt.Errorf("user not found: %s", userID)
	}

	group, gExists := r.groups[groupID]
	if !gExists {
		return fmt.Errorf("group not found: %s", groupID)
	}

	// 从组移除
	newMembers := make([]string, 0)
	for _, m := range group.Members {
		if m != userID {
			newMembers = append(newMembers, m)
		}
	}
	group.Members = newMembers

	// 从用户移除组
	newGroups := make([]string, 0)
	for _, g := range user.Groups {
		if g != groupID {
			newGroups = append(newGroups, g)
		}
	}
	user.Groups = newGroups

	group.UpdatedAt = time.Now()
	user.UpdatedAt = time.Now()

	return nil
}

// ListTenantUsers 列出租户用户
func (r *RBACManager) ListTenantUsers(tenantID string) ([]*User, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	users := make([]*User, 0)
	for _, user := range r.users {
		if user.TenantID == tenantID && user.Status != UserStatusDeleted {
			users = append(users, user)
		}
	}
	return users, nil
}

// ListTenantGroups 列出租户组
func (r *RBACManager) ListTenantGroups(tenantID string) ([]*UserGroup, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	groups := make([]*UserGroup, 0)
	for _, group := range r.groups {
		if group.TenantID == tenantID && group.Status != GroupStatusDeleted {
			groups = append(groups, group)
		}
	}
	return groups, nil
}

// ListTenants 列出所有租户
func (r *RBACManager) ListTenants() []*Tenant {
	r.mu.RLock()
	defer r.mu.RUnlock()

	tenants := make([]*Tenant, 0, len(r.tenants))
	for _, tenant := range r.tenants {
		if tenant.Status != TenantStatusDeleted {
			tenants = append(tenants, tenant)
		}
	}
	return tenants
}

// Authenticate 用户认证
func (r *RBACManager) Authenticate(username, password string) (*User, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	for _, user := range r.users {
		if user.Status == UserStatusDeleted {
			continue
		}

		// 支持用户名或邮箱登录
		if user.Username == username || user.Email == username {
			if user.Status != UserStatusActive {
				return nil, fmt.Errorf("user not active: %s", user.Status)
			}

			// 验证密码
			if err := bcryptVerify(password, user.PasswordHash); err != nil {
				return nil, fmt.Errorf("invalid password")
			}

			return user, nil
		}
	}

	return nil, fmt.Errorf("user not found")
}

// bcryptVerify 验证密码 (简化实现)
func bcryptVerify(password, hash string) error {
	// 实际应使用 golang.org/x/crypto/bcrypt
	// 这里简化处理
	if subtle.ConstantTimeCompare([]byte(password), []byte(hash)) != 1 {
		// 尝试验证 bcrypt 格式
		if strings.HasPrefix(hash, "$2") {
			return fmt.Errorf("bcrypt hash mismatch")
		}
		// 开发模式下直接比较
		if password == hash {
			return nil
		}
		return fmt.Errorf("password mismatch")
	}
	return nil
}

// AuthorizeRequest 授权请求中间件
func (r *RBACManager) AuthorizeRequest(userID string, perm Permission, resourceTenantID string) error {
	// 检查租户访问
	canAccess, err := r.CanAccessTenant(userID, resourceTenantID)
	if err != nil {
		return fmt.Errorf("tenant access denied: %v", err)
	}
	if !canAccess {
		return fmt.Errorf("access denied to tenant: %s", resourceTenantID)
	}

	// 检查权限
	has, err := r.HasPermission(userID, perm)
	if err != nil {
		return fmt.Errorf("permission check failed: %v", err)
	}
	if !has {
		return fmt.Errorf("permission denied: %s", perm)
	}

	return nil
}

// AuditLog 审计日志
type AuditLog struct {
	ID        string    `json:"id"`
	UserID    string    `json:"user_id"`
	TenantID  string    `json:"tenant_id"`
	Action    string    `json:"action"`
	Resource  string    `json:"resource"`
	Result    string    `json:"result"`
	IP        string    `json:"ip"`
	UserAgent string    `json:"user_agent"`
	Timestamp time.Time `json:"timestamp"`
}

// LogAction 记录操作日志
func (r *RBACManager) LogAction(userID, tenantID, action, resource, result, ip, userAgent string) {
	log := AuditLog{
		ID:        fmt.Sprintf("audit_%d", time.Now().UnixNano()),
		UserID:    userID,
		TenantID:  tenantID,
		Action:    action,
		Resource:  resource,
		Result:    result,
		IP:        ip,
		UserAgent: userAgent,
		Timestamp: time.Now(),
	}

	// 异步写入日志
	go func() {
		data, _ := json.Marshal(log)
		logger.Infof("Audit: %s", string(data))
	}()
}

// MiddlewareFunc 中间件函数类型
type MiddlewareFunc func(http.Handler) http.Handler

// RequirePermission 权限检查中间件
func (r *RBACManager) RequirePermission(perm Permission, resourceTenantID string) MiddlewareFunc {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
			// 从请求中获取用户ID (通过JWT或其他认证机制)
			userID := req.Header.Get("X-User-ID")
			if userID == "" {
				http.Error(w, "Unauthorized", http.StatusUnauthorized)
				return
			}

			// 检查权限
			if err := r.AuthorizeRequest(userID, perm, resourceTenantID); err != nil {
				logger.Warnf("Permission denied: %v", err)
				http.Error(w, "Forbidden", http.StatusForbidden)
				return
			}

			next.ServeHTTP(w, req)
		})
	}
}
