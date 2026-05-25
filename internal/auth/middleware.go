// Package auth 提供基于角色的权限控制(RBAC)
// 本文件实现HTTP中间件集成
package auth

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	"cloud-flow-agent/pkg/logger"
)

// ==================== 认证中间件 ====================

// AuthMiddleware 认证中间件
type AuthMiddleware struct {
	rbacEngine     *RBACEngine
	tenantManager  *TenantManager
	log            *logger.Logger
	tokenValidator TokenValidator
	
	// 配置
	config AuthMiddlewareConfig
}

// AuthMiddlewareConfig 认证中间件配置
type AuthMiddlewareConfig struct {
	Enabled           bool     `json:"enabled"`
	TokenHeader       string   `json:"token_header"`
	TokenPrefix       string   `json:"token_prefix"`
	PublicPaths       []string `json:"public_paths"`
	SkipAuthPaths     []string `json:"skip_auth_paths"`
	EnableTenantCheck bool     `json:"enable_tenant_check"`
}

// DefaultAuthMiddlewareConfig 默认配置
func DefaultAuthMiddlewareConfig() AuthMiddlewareConfig {
	return AuthMiddlewareConfig{
		Enabled:           true,
		TokenHeader:       "Authorization",
		TokenPrefix:       "Bearer ",
		PublicPaths:       []string{"/health", "/api/v1/login", "/api/v1/register"},
		SkipAuthPaths:     []string{},
		EnableTenantCheck: true,
	}
}

// TokenValidator Token验证器接口
type TokenValidator interface {
	Validate(token string) (*TokenClaims, error)
}

// TokenClaims Token声明
type TokenClaims struct {
	UserID    string `json:"user_id"`
	Username  string `json:"username"`
	TenantID  string `json:"tenant_id"`
	Roles     []string `json:"roles"`
	ExpiresAt int64  `json:"exp"`
}

// IsExpired 检查Token是否过期
func (c *TokenClaims) IsExpired() bool {
	return time.Now().Unix() > c.ExpiresAt
}

// NewAuthMiddleware 创建认证中间件
func NewAuthMiddleware(rbacEngine *RBACEngine, tenantManager *TenantManager, log *logger.Logger, config AuthMiddlewareConfig) *AuthMiddleware {
	return &AuthMiddleware{
		rbacEngine:     rbacEngine,
		tenantManager:  tenantManager,
		log:            log,
		config:         config,
		tokenValidator: &DefaultTokenValidator{},
	}
}

// SetTokenValidator 设置Token验证器
func (m *AuthMiddleware) SetTokenValidator(validator TokenValidator) {
	m.tokenValidator = validator
}

// Middleware HTTP中间件
func (m *AuthMiddleware) Middleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !m.config.Enabled {
			next.ServeHTTP(w, r)
			return
		}
		
		// 检查是否是公开路径
		if m.isPublicPath(r.URL.Path) {
			next.ServeHTTP(w, r)
			return
		}
		
		// 提取Token
		token := m.extractToken(r)
		if token == "" {
			m.respondError(w, http.StatusUnauthorized, "缺少认证Token")
			return
		}
		
		// 验证Token
		claims, err := m.tokenValidator.Validate(token)
		if err != nil {
			m.log.Warnf("Token验证失败: %v", err)
			m.respondError(w, http.StatusUnauthorized, "无效的Token")
			return
		}
		
		// 检查Token是否过期
		if claims.IsExpired() {
			m.respondError(w, http.StatusUnauthorized, "Token已过期")
			return
		}
		
		// 检查用户是否存在且有效
		user := m.rbacEngine.GetUser(claims.UserID)
		if user == nil {
			m.respondError(w, http.StatusUnauthorized, "用户不存在")
			return
		}
		
		if !user.IsActive() {
			m.respondError(w, http.StatusForbidden, "用户已被禁用")
			return
		}
		
		// 租户检查
		if m.config.EnableTenantCheck && claims.TenantID != "" {
			// 检查用户是否属于该租户
			if user.TenantID != "" && user.TenantID != claims.TenantID {
				// 检查是否为超级管理员
				if !m.rbacEngine.IsSuperAdmin(claims.UserID) {
					m.respondError(w, http.StatusForbidden, "无权访问该租户")
					return
				}
			}
			
			// 检查租户是否有效
			tenant := m.tenantManager.GetTenant(claims.TenantID)
			if tenant == nil {
				m.respondError(w, http.StatusForbidden, "租户不存在")
				return
			}
			
			if !tenant.IsActive() {
				m.respondError(w, http.StatusForbidden, "租户未激活或已过期")
				return
			}
		}
		
		// 构建用户上下文
		userCtx, err := m.rbacEngine.BuildUserContext(claims.UserID)
		if err != nil {
			m.log.Errorf("构建用户上下文失败: %v", err)
			m.respondError(w, http.StatusInternalServerError, "内部错误")
			return
		}
		
		// 将用户上下文注入请求上下文
		ctx := WithUserContext(r.Context(), userCtx)
		ctx = WithTenantContext(ctx, claims.TenantID)
		
		// 继续处理
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

// extractToken 从请求中提取Token
func (m *AuthMiddleware) extractToken(r *http.Request) string {
	// 从Header提取
	authHeader := r.Header.Get(m.config.TokenHeader)
	if authHeader != "" && strings.HasPrefix(authHeader, m.config.TokenPrefix) {
		return strings.TrimPrefix(authHeader, m.config.TokenPrefix)
	}
	
	// 从Query参数提取
	if token := r.URL.Query().Get("token"); token != "" {
		return token
	}
	
	// 从Cookie提取
	if cookie, err := r.Cookie("token"); err == nil {
		return cookie.Value
	}
	
	return ""
}

// isPublicPath 检查是否是公开路径
func (m *AuthMiddleware) isPublicPath(path string) bool {
	for _, publicPath := range m.config.PublicPaths {
		if strings.HasPrefix(path, publicPath) {
			return true
		}
	}
	return false
}

// respondError 返回错误响应
func (m *AuthMiddleware) respondError(w http.ResponseWriter, status int, message string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(map[string]interface{}{
		"code":    status,
		"message": message,
		"success": false,
	})
}

// ==================== 权限检查中间件 ====================

// PermissionMiddleware 权限检查中间件
type PermissionMiddleware struct {
	rbacEngine *RBACEngine
	log        *logger.Logger
	requiredPerm string
}

// NewPermissionMiddleware 创建权限检查中间件
func NewPermissionMiddleware(rbacEngine *RBACEngine, log *logger.Logger, permission string) *PermissionMiddleware {
	return &PermissionMiddleware{
		rbacEngine:   rbacEngine,
		log:          log,
		requiredPerm: permission,
	}
}

// Middleware HTTP中间件
func (m *PermissionMiddleware) Middleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// 从上下文获取用户
		userCtx := GetUserFromContext(r.Context())
		if userCtx == nil {
			m.respondError(w, http.StatusUnauthorized, "未认证")
			return
		}
		
		// 检查权限
		hasPerm, err := m.rbacEngine.CheckPermission(userCtx.UserID, m.requiredPerm)
		if err != nil {
			m.log.Errorf("权限检查失败: %v", err)
			m.respondError(w, http.StatusInternalServerError, "内部错误")
			return
		}
		
		if !hasPerm {
			m.log.Warnf("用户 %s 缺少权限: %s", userCtx.Username, m.requiredPerm)
			m.respondError(w, http.StatusForbidden, "缺少权限: "+m.requiredPerm)
			return
		}
		
		next.ServeHTTP(w, r)
	})
}

// respondError 返回错误响应
func (m *PermissionMiddleware) respondError(w http.ResponseWriter, status int, message string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(map[string]interface{}{
		"code":    status,
		"message": message,
		"success": false,
	})
}

// RequirePermission 快捷函数：创建权限检查中间件
func RequirePermission(rbacEngine *RBACEngine, log *logger.Logger, permission string) func(http.Handler) http.Handler {
	middleware := NewPermissionMiddleware(rbacEngine, log, permission)
	return middleware.Middleware
}

// ==================== 数据范围中间件 ====================

// DataScopeMiddleware 数据范围中间件
type DataScopeMiddleware struct {
	rbacEngine *RBACEngine
	log        *logger.Logger
	scopeType  DataScopeType
}

// NewDataScopeMiddleware 创建数据范围中间件
func NewDataScopeMiddleware(rbacEngine *RBACEngine, log *logger.Logger, scopeType DataScopeType) *DataScopeMiddleware {
	return &DataScopeMiddleware{
		rbacEngine: rbacEngine,
		log:        log,
		scopeType:  scopeType,
	}
}

// Middleware HTTP中间件
func (m *DataScopeMiddleware) Middleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		userCtx := GetUserFromContext(r.Context())
		if userCtx == nil {
			m.respondError(w, http.StatusUnauthorized, "未认证")
			return
		}
		
		// 检查数据范围权限
		hasScope := false
		for _, scope := range userCtx.DataScopes {
			if scope.Type == DataScopeTypeAll || scope.Type == m.scopeType {
				hasScope = true
				break
			}
		}
		
		if !hasScope {
			m.respondError(w, http.StatusForbidden, "缺少数据范围权限")
			return
		}
		
		next.ServeHTTP(w, r)
	})
}

// respondError 返回错误响应
func (m *DataScopeMiddleware) respondError(w http.ResponseWriter, status int, message string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(map[string]interface{}{
		"code":    status,
		"message": message,
		"success": false,
	})
}

// ==================== 默认Token验证器 ====================

// DefaultTokenValidator 默认Token验证器
// 实际应用中应使用JWT等标准实现
type DefaultTokenValidator struct {
	secret string
}

// NewDefaultTokenValidator 创建默认Token验证器
func NewDefaultTokenValidator(secret string) *DefaultTokenValidator {
	return &DefaultTokenValidator{secret: secret}
}

// Validate 验证Token
func (v *DefaultTokenValidator) Validate(token string) (*TokenClaims, error) {
	// 这里使用简化的实现，实际应使用JWT库解析
	// 示例: 解析格式为 "userID:tenantID:timestamp:signature"
	
	parts := strings.Split(token, ":")
	if len(parts) != 4 {
		return nil, fmt.Errorf("无效的Token格式")
	}
	
	// 验证签名（简化）
	expectedSig := v.generateSignature(parts[0], parts[1], parts[2])
	if parts[3] != expectedSig {
		return nil, fmt.Errorf("无效的Token签名")
	}
	
	// 解析时间戳
	timestamp := int64(0)
	fmt.Sscanf(parts[2], "%d", &timestamp)
	
	return &TokenClaims{
		UserID:    parts[0],
		TenantID:  parts[1],
		Username:  parts[0], // 简化处理
		ExpiresAt: timestamp + 3600, // 1小时过期
	}, nil
}

// GenerateToken 生成Token（用于测试）
func (v *DefaultTokenValidator) GenerateToken(userID, tenantID string) string {
	timestamp := time.Now().Unix()
	signature := v.generateSignature(userID, tenantID, fmt.Sprintf("%d", timestamp))
	return fmt.Sprintf("%s:%s:%d:%s", userID, tenantID, timestamp, signature)
}

// generateSignature 生成签名（简化实现）
func (v *DefaultTokenValidator) generateSignature(userID, tenantID, timestamp string) string {
	// 简化实现，实际应使用HMAC等
	data := userID + tenantID + timestamp + v.secret
	sum := 0
	for _, c := range data {
		sum += int(c)
	}
	return fmt.Sprintf("%x", sum%10000)
}

// ==================== 辅助函数 ====================

// GetCurrentUser 获取当前用户
func GetCurrentUser(ctx context.Context) *UserContext {
	return GetUserFromContext(ctx)
}

// GetCurrentTenantID 获取当前租户ID
func GetCurrentTenantID(ctx context.Context) string {
	return GetTenantFromContext(ctx)
}

// HasPermission 检查当前用户是否有权限
func HasPermission(ctx context.Context, permission string) bool {
	userCtx := GetUserFromContext(ctx)
	if userCtx == nil {
		return false
	}
	return userCtx.HasPermission(permission)
}

// RequireAuth 快捷函数：要求认证
func RequireAuth(rbacEngine *RBACEngine, tenantManager *TenantManager, log *logger.Logger) func(http.Handler) http.Handler {
	middleware := NewAuthMiddleware(rbacEngine, tenantManager, log, DefaultAuthMiddlewareConfig())
	return middleware.Middleware
}
