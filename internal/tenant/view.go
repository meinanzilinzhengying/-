package tenant

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"sync"
	"time"

	"cloud-flow-agent/pkg/logger"
)

// UserRole 用户角色
type UserRole string

const (
	RoleAdmin      UserRole = "admin"
	RoleTenantAdmin UserRole = "tenant_admin"
	RoleUser       UserRole = "user"
	RoleViewer     UserRole = "viewer"
)

// Permission 权限
type Permission string

const (
	PermViewCollectors   Permission = "collectors:view"
	PermManageCollectors Permission = "collectors:manage"
	PermViewTopology     Permission = "topology:view"
	PermManageTopology   Permission = "topology:manage"
	PermViewAlerts       Permission = "alerts:view"
	PermManageAlerts     Permission = "alerts:manage"
	PermViewReports      Permission = "reports:view"
	PermManageReports    Permission = "reports:manage"
	PermViewTenants      Permission = "tenants:view"
	PermManageTenants    Permission = "tenants:manage"
	PermViewUsers        Permission = "users:view"
	PermManageUsers      Permission = "users:manage"
	PermSystemConfig     Permission = "system:config"
)

// Tenant 租户信息
type Tenant struct {
	ID          string                 `json:"id" yaml:"id"`
	Name        string                 `json:"name" yaml:"name"`
	Description string                 `json:"description" yaml:"description"`
	Status      string                 `json:"status" yaml:"status"`
	Quota       *TenantQuota           `json:"quota" yaml:"quota"`
	Config      map[string]interface{} `json:"config" yaml:"config"`
	CreatedAt   time.Time              `json:"created_at" yaml:"created_at"`
	UpdatedAt   time.Time              `json:"updated_at" yaml:"updated_at"`
	ExpiredAt   *time.Time             `json:"expired_at,omitempty" yaml:"expired_at,omitempty"`
}

// TenantQuota 租户配额
type TenantQuota struct {
	MaxCollectors   int `json:"max_collectors" yaml:"max_collectors"`
	MaxUsers        int `json:"max_users" yaml:"max_users"`
	MaxAlerts       int `json:"max_alerts" yaml:"max_alerts"`
	MaxStorageGB    int `json:"max_storage_gb" yaml:"max_storage_gb"`
	MaxNetworkMbps  int `json:"max_network_mbps" yaml:"max_network_mbps"`
	RetentionDays   int `json:"retention_days" yaml:"retention_days"`
}

// DefaultTenantQuota 返回默认租户配额
func DefaultTenantQuota() *TenantQuota {
	return &TenantQuota{
		MaxCollectors:  50,
		MaxUsers:       10,
		MaxAlerts:      1000,
		MaxStorageGB:   100,
		MaxNetworkMbps: 100,
		RetentionDays:  30,
	}
}

// User 用户信息
type User struct {
	ID        string     `json:"id" yaml:"id"`
	Username  string     `json:"username" yaml:"username"`
	Email     string     `json:"email" yaml:"email"`
	Phone     string     `json:"phone,omitempty" yaml:"phone,omitempty"`
	Role      UserRole   `json:"role" yaml:"role"`
	TenantID  string     `json:"tenant_id" yaml:"tenant_id"`
	Status    string     `json:"status" yaml:"status"`
	LastLogin *time.Time `json:"last_login,omitempty" yaml:"last_login,omitempty"`
	CreatedAt time.Time  `json:"created_at" yaml:"created_at"`
	UpdatedAt time.Time  `json:"updated_at" yaml:"updated_at"`
}

// RolePermissions 角色权限映射
var RolePermissions = map[UserRole][]Permission{
	RoleAdmin: {
		PermViewCollectors, PermManageCollectors,
		PermViewTopology, PermManageTopology,
		PermViewAlerts, PermManageAlerts,
		PermViewReports, PermManageReports,
		PermViewTenants, PermManageTenants,
		PermViewUsers, PermManageUsers,
		PermSystemConfig,
	},
	RoleTenantAdmin: {
		PermViewCollectors, PermManageCollectors,
		PermViewTopology, PermManageTopology,
		PermViewAlerts, PermManageAlerts,
		PermViewReports, PermManageReports,
		PermViewUsers, PermManageUsers,
	},
	RoleUser: {
		PermViewCollectors,
		PermViewTopology,
		PermViewAlerts,
		PermViewReports,
	},
	RoleViewer: {
		PermViewCollectors,
		PermViewTopology,
		PermViewAlerts,
		PermViewReports,
	},
}

// ViewContext 视图上下文
type ViewContext struct {
	User      *User
	Tenant    *Tenant
	Role      UserRole
	Permissions []Permission
	IsAdmin   bool
	IsTenantAdmin bool
}

// TenantViewManager 租户视图管理器
type TenantViewManager struct {
	mu       sync.RWMutex
	tenants  map[string]*Tenant
	users    map[string]*User
	sessions map[string]*Session
	config   *ViewConfig
}

// ViewConfig 视图配置
type ViewConfig struct {
	DefaultTenantQuota *TenantQuota  `json:"default_quota" yaml:"default_quota"`
	SessionTimeout     time.Duration `json:"session_timeout" yaml:"session_timeout"`
	MaxUsersPerTenant  int           `json:"max_users_per_tenant" yaml:"max_users_per_tenant"`
	EnableRegistration bool          `json:"enable_registration" yaml:"enable_registration"`
	RequireApproval    bool          `json:"require_approval" yaml:"require_approval"`
}

// Session 用户会话
type Session struct {
	ID        string    `json:"id"`
	UserID    string    `json:"user_id"`
	TenantID  string    `json:"tenant_id"`
	Role      UserRole  `json:"role"`
	CreatedAt time.Time `json:"created_at"`
	ExpiresAt time.Time `json:"expires_at"`
	LastActivity time.Time `json:"last_activity"`
}

// DefaultViewConfig 返回默认视图配置
func DefaultViewConfig() *ViewConfig {
	return &ViewConfig{
		DefaultTenantQuota: DefaultTenantQuota(),
		SessionTimeout:     24 * time.Hour,
		MaxUsersPerTenant:  100,
		EnableRegistration: true,
		RequireApproval:    false,
	}
}

// NewTenantViewManager 创建新的租户视图管理器
func NewTenantViewManager(config *ViewConfig) *TenantViewManager {
	if config == nil {
		config = DefaultViewConfig()
	}

	return &TenantViewManager{
		tenants:  make(map[string]*Tenant),
		users:    make(map[string]*User),
		sessions: make(map[string]*Session),
		config:   config,
	}
}

// ==================== 租户管理 ====================

// CreateTenant 创建租户
func (tvm *TenantViewManager) CreateTenant(name, description string, quota *TenantQuota) (*Tenant, error) {
	if name == "" {
		return nil, fmt.Errorf("tenant name is required")
	}

	if quota == nil {
		quota = tvm.config.DefaultTenantQuota
	}

	tvm.mu.Lock()
	defer tvm.mu.Unlock()

	tenant := &Tenant{
		ID:          fmt.Sprintf("tenant_%d", time.Now().UnixNano()),
		Name:        name,
		Description: description,
		Status:      "active",
		Quota:       quota,
		Config:      make(map[string]interface{}),
		CreatedAt:   time.Now(),
		UpdatedAt:   time.Now(),
	}

	tvm.tenants[tenant.ID] = tenant

	logger.Info("Created tenant: %s (%s)", tenant.Name, tenant.ID)
	return tenant, nil
}

// GetTenant 获取租户信息
func (tvm *TenantViewManager) GetTenant(tenantID string) (*Tenant, error) {
	tvm.mu.RLock()
	defer tvm.mu.RUnlock()

	tenant, exists := tvm.tenants[tenantID]
	if !exists {
		return nil, fmt.Errorf("tenant not found: %s", tenantID)
	}

	return tenant, nil
}

// UpdateTenant 更新租户信息
func (tvm *TenantViewManager) UpdateTenant(tenantID string, updates map[string]interface{}) (*Tenant, error) {
	tvm.mu.Lock()
	defer tvm.mu.Unlock()

	tenant, exists := tvm.tenants[tenantID]
	if !exists {
		return nil, fmt.Errorf("tenant not found: %s", tenantID)
	}

	if name, ok := updates["name"].(string); ok && name != "" {
		tenant.Name = name
	}
	if desc, ok := updates["description"].(string); ok {
		tenant.Description = desc
	}
	if status, ok := updates["status"].(string); ok {
		tenant.Status = status
	}
	if quota, ok := updates["quota"].(map[string]interface{}); ok {
		if maxCollectors, ok := quota["max_collectors"].(float64); ok {
			tenant.Quota.MaxCollectors = int(maxCollectors)
		}
		if maxUsers, ok := quota["max_users"].(float64); ok {
			tenant.Quota.MaxUsers = int(maxUsers)
		}
	}

	tenant.UpdatedAt = time.Now()
	logger.Info("Updated tenant: %s (%s)", tenant.Name, tenant.ID)

	return tenant, nil
}

// DeleteTenant 删除租户
func (tvm *TenantViewManager) DeleteTenant(tenantID string) error {
	tvm.mu.Lock()
	defer tvm.mu.Unlock()

	_, exists := tvm.tenants[tenantID]
	if !exists {
		return fmt.Errorf("tenant not found: %s", tenantID)
	}

	delete(tvm.tenants, tenantID)
	logger.Info("Deleted tenant: %s", tenantID)
	return nil
}

// ListTenants 列出所有租户
func (tvm *TenantViewManager) ListTenants() []*Tenant {
	tvm.mu.RLock()
	defer tvm.mu.RUnlock()

	var result []*Tenant
	for _, tenant := range tvm.tenants {
		result = append(result, tenant)
	}
	return result
}

// ==================== 用户管理 ====================

// CreateUser 创建用户
func (tvm *TenantViewManager) CreateUser(username, email string, role UserRole, tenantID string) (*User, error) {
	if username == "" || email == "" {
		return nil, fmt.Errorf("username and email are required")
	}

	tvm.mu.Lock()
	defer tvm.mu.Unlock()

	// 检查租户是否存在
	if tenantID != "" {
		if _, exists := tvm.tenants[tenantID]; !exists {
			return nil, fmt.Errorf("tenant not found: %s", tenantID)
		}

		// 检查租户用户数量限制
		userCount := 0
		for _, user := range tvm.users {
			if user.TenantID == tenantID {
				userCount++
			}
		}
		if userCount >= tvm.config.MaxUsersPerTenant {
			return nil, fmt.Errorf("maximum users reached for tenant: %s", tenantID)
		}
	}

	user := &User{
		ID:        fmt.Sprintf("user_%d", time.Now().UnixNano()),
		Username:  username,
		Email:     email,
		Role:      role,
		TenantID:  tenantID,
		Status:    "active",
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}

	tvm.users[user.ID] = user
	logger.Info("Created user: %s (%s) with role %s", user.Username, user.ID, user.Role)

	return user, nil
}

// GetUser 获取用户信息
func (tvm *TenantViewManager) GetUser(userID string) (*User, error) {
	tvm.mu.RLock()
	defer tvm.mu.RUnlock()

	user, exists := tvm.users[userID]
	if !exists {
		return nil, fmt.Errorf("user not found: %s", userID)
	}

	return user, nil
}

// UpdateUser 更新用户信息
func (tvm *TenantViewManager) UpdateUser(userID string, updates map[string]interface{}) (*User, error) {
	tvm.mu.Lock()
	defer tvm.mu.Unlock()

	user, exists := tvm.users[userID]
	if !exists {
		return nil, fmt.Errorf("user not found: %s", userID)
	}

	if username, ok := updates["username"].(string); ok && username != "" {
		user.Username = username
	}
	if email, ok := updates["email"].(string); ok && email != "" {
		user.Email = email
	}
	if phone, ok := updates["phone"].(string); ok {
		user.Phone = phone
	}
	if role, ok := updates["role"].(string); ok {
		user.Role = UserRole(role)
	}
	if status, ok := updates["status"].(string); ok {
		user.Status = status
	}

	user.UpdatedAt = time.Now()
	logger.Info("Updated user: %s (%s)", user.Username, user.ID)

	return user, nil
}

// DeleteUser 删除用户
func (tvm *TenantViewManager) DeleteUser(userID string) error {
	tvm.mu.Lock()
	defer tvm.mu.Unlock()

	_, exists := tvm.users[userID]
	if !exists {
		return fmt.Errorf("user not found: %s", userID)
	}

	delete(tvm.users, userID)
	logger.Info("Deleted user: %s", userID)
	return nil
}

// ListUsers 列出用户
func (tvm *TenantViewManager) ListUsers(tenantID string) []*User {
	tvm.mu.RLock()
	defer tvm.mu.RUnlock()

	var result []*User
	for _, user := range tvm.users {
		if tenantID == "" || user.TenantID == tenantID {
			result = append(result, user)
		}
	}
	return result
}

// ListUsersByTenant 按租户列出用户
func (tvm *TenantViewManager) ListUsersByTenant(tenantID string) []*User {
	return tvm.ListUsers(tenantID)
}

// ==================== 权限管理 ====================

// HasPermission 检查用户是否有权限
func (tvm *TenantViewManager) HasPermission(userID string, permission Permission) bool {
	user, err := tvm.GetUser(userID)
	if err != nil {
		return false
	}

	permissions, exists := RolePermissions[user.Role]
	if !exists {
		return false
	}

	for _, p := range permissions {
		if p == permission {
			return true
		}
	}

	return false
}

// GetUserPermissions 获取用户权限列表
func (tvm *TenantViewManager) GetUserPermissions(userID string) ([]Permission, error) {
	user, err := tvm.GetUser(userID)
	if err != nil {
		return nil, err
	}

	permissions, exists := RolePermissions[user.Role]
	if !exists {
		return []Permission{}, nil
	}

	return permissions, nil
}

// ==================== 视图上下文 ====================

// CreateViewContext 创建视图上下文
func (tvm *TenantViewManager) CreateViewContext(userID string) (*ViewContext, error) {
	user, err := tvm.GetUser(userID)
	if err != nil {
		return nil, err
	}

	ctx := &ViewContext{
		User:      user,
		Role:      user.Role,
		IsAdmin:   user.Role == RoleAdmin,
		IsTenantAdmin: user.Role == RoleTenantAdmin || user.Role == RoleAdmin,
	}

	if user.TenantID != "" {
		tenant, err := tvm.GetTenant(user.TenantID)
		if err == nil {
			ctx.Tenant = tenant
		}
	}

	permissions, err := tvm.GetUserPermissions(userID)
	if err == nil {
		ctx.Permissions = permissions
	}

	return ctx, nil
}

// FilterByPermission 根据权限过滤数据
func (ctx *ViewContext) FilterByPermission(permission Permission) bool {
	for _, p := range ctx.Permissions {
		if p == permission {
			return true
		}
	}
	return false
}

// CanManageTenant 是否可以管理租户
func (ctx *ViewContext) CanManageTenant() bool {
	return ctx.IsAdmin
}

// CanManageUsers 是否可以管理用户
func (ctx *ViewContext) CanManageUsers() bool {
	return ctx.IsTenantAdmin || ctx.FilterByPermission(PermManageUsers)
}

// CanManageCollectors 是否可以管理采集器
func (ctx *ViewContext) CanManageCollectors() bool {
	return ctx.FilterByPermission(PermManageCollectors)
}

// CanViewCollectors 是否可以查看采集器
func (ctx *ViewContext) CanViewCollectors() bool {
	return ctx.FilterByPermission(PermViewCollectors)
}

// ==================== Web API Handlers ====================

// TenantViewHandler 租户视图HTTP处理器
type TenantViewHandler struct {
	manager *TenantViewManager
}

// NewTenantViewHandler 创建新的处理器
func NewTenantViewHandler(manager *TenantViewManager) *TenantViewHandler {
	return &TenantViewHandler{manager: manager}
}

// RegisterRoutes 注册路由
func (h *TenantViewHandler) RegisterRoutes(mux *http.ServeMux) {
	// 租户管理API
	mux.HandleFunc("/api/v1/tenants", h.handleTenants)
	mux.HandleFunc("/api/v1/tenants/", h.handleTenantDetail)
	mux.HandleFunc("/api/v1/tenants/quota", h.handleTenantQuota)

	// 用户管理API
	mux.HandleFunc("/api/v1/users", h.handleUsers)
	mux.HandleFunc("/api/v1/users/", h.handleUserDetail)
	mux.HandleFunc("/api/v1/users/permissions", h.handleUserPermissions)

	// 视图上下文API
	mux.HandleFunc("/api/v1/context", h.handleViewContext)
	mux.HandleFunc("/api/v1/dashboard", h.handleDashboard)
}

// handleTenants 处理租户列表请求
func (h *TenantViewHandler) handleTenants(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		h.listTenants(w, r)
	case http.MethodPost:
		h.createTenant(w, r)
	default:
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
	}
}

// listTenants 列出租户
func (h *TenantViewHandler) listTenants(w http.ResponseWriter, r *http.Request) {
	tenants := h.manager.ListTenants()

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"code":    0,
		"message": "success",
		"data":    tenants,
		"total":   len(tenants),
	})
}

// createTenant 创建租户
func (h *TenantViewHandler) createTenant(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Name        string                 `json:"name"`
		Description string                 `json:"description"`
		Quota       *TenantQuota           `json:"quota"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	tenant, err := h.manager.CreateTenant(req.Name, req.Description, req.Quota)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(map[string]interface{}{
		"code":    0,
		"message": "Tenant created successfully",
		"data":    tenant,
	})
}

// handleTenantDetail 处理单个租户详情
func (h *TenantViewHandler) handleTenantDetail(w http.ResponseWriter, r *http.Request) {
	tenantID := r.URL.Path[len("/api/v1/tenants/"):]
	if tenantID == "" {
		http.Error(w, "Tenant ID required", http.StatusBadRequest)
		return
	}

	switch r.Method {
	case http.MethodGet:
		h.getTenant(w, r, tenantID)
	case http.MethodPut:
		h.updateTenant(w, r, tenantID)
	case http.MethodDelete:
		h.deleteTenant(w, r, tenantID)
	default:
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
	}
}

// getTenant 获取租户详情
func (h *TenantViewHandler) getTenant(w http.ResponseWriter, r *http.Request, tenantID string) {
	tenant, err := h.manager.GetTenant(tenantID)
	if err != nil {
		http.Error(w, err.Error(), http.StatusNotFound)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"code":    0,
		"message": "success",
		"data":    tenant,
	})
}

// updateTenant 更新租户
func (h *TenantViewHandler) updateTenant(w http.ResponseWriter, r *http.Request, tenantID string) {
	var updates map[string]interface{}
	if err := json.NewDecoder(r.Body).Decode(&updates); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	tenant, err := h.manager.UpdateTenant(tenantID, updates)
	if err != nil {
		http.Error(w, err.Error(), http.StatusNotFound)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"code":    0,
		"message": "Tenant updated successfully",
		"data":    tenant,
	})
}

// deleteTenant 删除租户
func (h *TenantViewHandler) deleteTenant(w http.ResponseWriter, r *http.Request, tenantID string) {
	if err := h.manager.DeleteTenant(tenantID); err != nil {
		http.Error(w, err.Error(), http.StatusNotFound)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"code":    0,
		"message": "Tenant deleted successfully",
	})
}

// handleTenantQuota 处理租户配额
func (h *TenantViewHandler) handleTenantQuota(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	quota := DefaultTenantQuota()
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"code":    0,
		"message": "success",
		"data":    quota,
	})
}

// handleUsers 处理用户列表请求
func (h *TenantViewHandler) handleUsers(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		h.listUsers(w, r)
	case http.MethodPost:
		h.createUser(w, r)
	default:
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
	}
}

// listUsers 列出用户
func (h *TenantViewHandler) listUsers(w http.ResponseWriter, r *http.Request) {
	tenantID := r.URL.Query().Get("tenant_id")
	users := h.manager.ListUsers(tenantID)

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"code":    0,
		"message": "success",
		"data":    users,
		"total":   len(users),
	})
}

// createUser 创建用户
func (h *TenantViewHandler) createUser(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Username string   `json:"username"`
		Email    string   `json:"email"`
		Role     UserRole `json:"role"`
		TenantID string   `json:"tenant_id"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	user, err := h.manager.CreateUser(req.Username, req.Email, req.Role, req.TenantID)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(map[string]interface{}{
		"code":    0,
		"message": "User created successfully",
		"data":    user,
	})
}

// handleUserDetail 处理单个用户详情
func (h *TenantViewHandler) handleUserDetail(w http.ResponseWriter, r *http.Request) {
	userID := r.URL.Path[len("/api/v1/users/"):]
	if userID == "" {
		http.Error(w, "User ID required", http.StatusBadRequest)
		return
	}

	switch r.Method {
	case http.MethodGet:
		h.getUser(w, r, userID)
	case http.MethodPut:
		h.updateUser(w, r, userID)
	case http.MethodDelete:
		h.deleteUser(w, r, userID)
	default:
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
	}
}

// getUser 获取用户详情
func (h *TenantViewHandler) getUser(w http.ResponseWriter, r *http.Request, userID string) {
	user, err := h.manager.GetUser(userID)
	if err != nil {
		http.Error(w, err.Error(), http.StatusNotFound)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"code":    0,
		"message": "success",
		"data":    user,
	})
}

// updateUser 更新用户
func (h *TenantViewHandler) updateUser(w http.ResponseWriter, r *http.Request, userID string) {
	var updates map[string]interface{}
	if err := json.NewDecoder(r.Body).Decode(&updates); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	user, err := h.manager.UpdateUser(userID, updates)
	if err != nil {
		http.Error(w, err.Error(), http.StatusNotFound)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"code":    0,
		"message": "User updated successfully",
		"data":    user,
	})
}

// deleteUser 删除用户
func (h *TenantViewHandler) deleteUser(w http.ResponseWriter, r *http.Request, userID string) {
	if err := h.manager.DeleteUser(userID); err != nil {
		http.Error(w, err.Error(), http.StatusNotFound)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"code":    0,
		"message": "User deleted successfully",
	})
}

// handleUserPermissions 处理用户权限
func (h *TenantViewHandler) handleUserPermissions(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	userID := r.URL.Query().Get("user_id")
	if userID == "" {
		http.Error(w, "User ID required", http.StatusBadRequest)
		return
	}

	permissions, err := h.manager.GetUserPermissions(userID)
	if err != nil {
		http.Error(w, err.Error(), http.StatusNotFound)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"code":    0,
		"message": "success",
		"data": map[string]interface{}{
			"user_id":     userID,
			"permissions": permissions,
		},
	})
}

// handleViewContext 处理视图上下文请求
func (h *TenantViewHandler) handleViewContext(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	userID := r.URL.Query().Get("user_id")
	if userID == "" {
		http.Error(w, "User ID required", http.StatusBadRequest)
		return
	}

	ctx, err := h.manager.CreateViewContext(userID)
	if err != nil {
		http.Error(w, err.Error(), http.StatusNotFound)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"code":    0,
		"message": "success",
		"data": map[string]interface{}{
			"user":           ctx.User,
			"tenant":         ctx.Tenant,
			"role":           ctx.Role,
			"permissions":    ctx.Permissions,
			"is_admin":       ctx.IsAdmin,
			"is_tenant_admin": ctx.IsTenantAdmin,
		},
	})
}

// handleDashboard 处理仪表盘数据
func (h *TenantViewHandler) handleDashboard(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	userID := r.URL.Query().Get("user_id")
	if userID == "" {
		http.Error(w, "User ID required", http.StatusBadRequest)
		return
	}

	ctx, err := h.manager.CreateViewContext(userID)
	if err != nil {
		http.Error(w, err.Error(), http.StatusNotFound)
		return
	}

	// 根据角色返回不同的仪表盘数据
	dashboard := map[string]interface{}{
		"role":        ctx.Role,
		"permissions": ctx.Permissions,
		"widgets":     h.getDashboardWidgets(ctx),
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"code":    0,
		"message": "success",
		"data":    dashboard,
	})
}

// getDashboardWidgets 获取仪表盘组件
func (h *TenantViewHandler) getDashboardWidgets(ctx *ViewContext) []map[string]interface{} {
	var widgets []map[string]interface{}

	// 所有角色都能看到的基本组件
	if ctx.CanViewCollectors() {
		widgets = append(widgets, map[string]interface{}{
			"type":  "collector_status",
			"title": "采集器状态",
			"icon":  " collectors",
		})
	}

	if ctx.FilterByPermission(PermViewAlerts) {
		widgets = append(widgets, map[string]interface{}{
			"type":  "alert_summary",
			"title": "告警概览",
			"icon":  "alerts",
		})
	}

	if ctx.FilterByPermission(PermViewTopology) {
		widgets = append(widgets, map[string]interface{}{
			"type":  "topology_overview",
			"title": "拓扑概览",
			"icon":  "topology",
		})
	}

	// 管理员特有组件
	if ctx.IsAdmin {
		widgets = append(widgets, map[string]interface{}{
			"type":  "system_health",
			"title": "系统健康",
			"icon":  "system",
		})
		widgets = append(widgets, map[string]interface{}{
			"type":  "tenant_overview",
			"title": "租户概览",
			"icon":  "tenants",
		})
	}

	// 租户管理员特有组件
	if ctx.IsTenantAdmin {
		widgets = append(widgets, map[string]interface{}{
			"type":  "user_activity",
			"title": "用户活动",
			"icon":  "users",
		})
		widgets = append(widgets, map[string]interface{}{
			"type":  "quota_usage",
			"title": "配额使用",
			"icon":  "quota",
		})
	}

	return widgets
}

// ==================== 会话管理 ====================

// CreateSession 创建会话
func (tvm *TenantViewManager) CreateSession(userID string) (*Session, error) {
	user, err := tvm.GetUser(userID)
	if err != nil {
		return nil, err
	}

	tvm.mu.Lock()
	defer tvm.mu.Unlock()

	session := &Session{
		ID:           fmt.Sprintf("session_%d", time.Now().UnixNano()),
		UserID:       userID,
		TenantID:     user.TenantID,
		Role:         user.Role,
		CreatedAt:    time.Now(),
		ExpiresAt:    time.Now().Add(tvm.config.SessionTimeout),
		LastActivity: time.Now(),
	}

	tvm.sessions[session.ID] = session
	logger.Info("Created session: %s for user: %s", session.ID, userID)

	return session, nil
}

// GetSession 获取会话
func (tvm *TenantViewManager) GetSession(sessionID string) (*Session, error) {
	tvm.mu.RLock()
	defer tvm.mu.RUnlock()

	session, exists := tvm.sessions[sessionID]
	if !exists {
		return nil, fmt.Errorf("session not found: %s", sessionID)
	}

	if time.Now().After(session.ExpiresAt) {
		return nil, fmt.Errorf("session expired: %s", sessionID)
	}

	return session, nil
}

// RefreshSession 刷新会话
func (tvm *TenantViewManager) RefreshSession(sessionID string) error {
	tvm.mu.Lock()
	defer tvm.mu.Unlock()

	session, exists := tvm.sessions[sessionID]
	if !exists {
		return fmt.Errorf("session not found: %s", sessionID)
	}

	session.ExpiresAt = time.Now().Add(tvm.config.SessionTimeout)
	session.LastActivity = time.Now()

	return nil
}

// DestroySession 销毁会话
func (tvm *TenantViewManager) DestroySession(sessionID string) error {
	tvm.mu.Lock()
	defer tvm.mu.Unlock()

	delete(tvm.sessions, sessionID)
	logger.Info("Destroyed session: %s", sessionID)

	return nil
}

// CleanupExpiredSessions 清理过期会话
func (tvm *TenantViewManager) CleanupExpiredSessions() {
	tvm.mu.Lock()
	defer tvm.mu.Unlock()

	now := time.Now()
	for id, session := range tvm.sessions {
		if now.After(session.ExpiresAt) {
			delete(tvm.sessions, id)
			logger.Debug("Cleaned up expired session: %s", id)
		}
	}
}

// StartSessionCleanup 启动会话清理任务
func (tvm *TenantViewManager) StartSessionCleanup(ctx context.Context) {
	ticker := time.NewTicker(5 * time.Minute)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			tvm.CleanupExpiredSessions()
		}
	}
}
