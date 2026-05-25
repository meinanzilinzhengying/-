//go:build linux

// Package alert 提供告警管理功能
// 本文件实现告警模块与权限系统的集成
package alert

import (
	"context"
	"fmt"

	"cloud-flow-agent/internal/auth"
)

// AuthIntegration 告警模块权限集成
type AuthIntegration struct {
	rbacEngine    *auth.RBACEngine
	tenantManager *auth.TenantManager
}

// NewAuthIntegration 创建权限集成
func NewAuthIntegration(rbacEngine *auth.RBACEngine, tenantManager *auth.TenantManager) *AuthIntegration {
	return &AuthIntegration{
		rbacEngine:    rbacEngine,
		tenantManager: tenantManager,
	}
}

// ==================== 告警权限检查 ====================

// CheckAlertViewPermission 检查告警查看权限
func (a *AuthIntegration) CheckAlertViewPermission(ctx context.Context) error {
	userCtx := auth.GetUserFromContext(ctx)
	if userCtx == nil {
		return fmt.Errorf("未认证")
	}

	if !userCtx.HasPermission("alert:view") {
		return fmt.Errorf("缺少告警查看权限")
	}

	return nil
}

// CheckAlertEditPermission 检查告警编辑权限
func (a *AuthIntegration) CheckAlertEditPermission(ctx context.Context) error {
	userCtx := auth.GetUserFromContext(ctx)
	if userCtx == nil {
		return fmt.Errorf("未认证")
	}

	if !userCtx.HasPermission("alert:edit") {
		return fmt.Errorf("缺少告警编辑权限")
	}

	return nil
}

// CheckAlertManagePermission 检查告警管理权限
func (a *AuthIntegration) CheckAlertManagePermission(ctx context.Context) error {
	userCtx := auth.GetUserFromContext(ctx)
	if userCtx == nil {
		return fmt.Errorf("未认证")
	}

	if !userCtx.HasPermission("alert:manage") {
		return fmt.Errorf("缺少告警管理权限")
	}

	return nil
}

// ==================== 数据范围过滤 ====================

// FilterAlertsByDataScope 按数据范围过滤告警
func (a *AuthIntegration) FilterAlertsByDataScope(ctx context.Context, alerts []*AlertEvent) ([]*AlertEvent, error) {
	userCtx := auth.GetUserFromContext(ctx)
	if userCtx == nil {
		return nil, fmt.Errorf("未认证")
	}

	// 超级管理员可以看到所有告警
	if userCtx.IsSuperAdmin() {
		return alerts, nil
	}

	// 按租户过滤
	filtered := make([]*AlertEvent, 0)
	for _, alert := range alerts {
		// 检查租户
		alertTenantID := alert.Labels["tenant_id"]
		if alertTenantID != "" && alertTenantID != userCtx.TenantID {
			continue
		}

		// 检查数据范围
		if a.checkAlertDataScope(userCtx, alert) {
			filtered = append(filtered, alert)
		}
	}

	return filtered, nil
}

// checkAlertDataScope 检查告警数据范围
func (a *AuthIntegration) checkAlertDataScope(userCtx *auth.UserContext, alert *AlertEvent) bool {
	for _, scope := range userCtx.DataScopes {
		switch scope.Type {
		case auth.DataScopeTypeAll:
			return true
		case auth.DataScopeTypeSelf:
			// 检查是否是自己的告警
			if alert.Labels["created_by"] == userCtx.UserID {
				return true
			}
		case auth.DataScopeTypeGroup:
			// 检查组权限
			for _, groupID := range scope.Values {
				if alert.Labels["group_id"] == groupID {
					return true
				}
			}
		case auth.DataScopeTypeBusiness:
			// 检查业务组权限
			for _, bizID := range scope.Values {
				if alert.Labels["business_group_id"] == bizID {
					return true
				}
			}
		case auth.DataScopeTypeAsset:
			// 检查资产权限
			for _, assetID := range scope.Values {
				if alert.Labels["asset_id"] == assetID || alert.Labels["instance"] == assetID {
					return true
				}
			}
		}
	}
	return false
}

// ==================== 租户隔离 ====================

// EnforceTenantIsolation 强制执行租户隔离
func (a *AuthIntegration) EnforceTenantIsolation(ctx context.Context, tenantID string) error {
	userCtx := auth.GetUserFromContext(ctx)
	if userCtx == nil {
		return fmt.Errorf("未认证")
	}

	// 超级管理员可以访问所有租户
	if userCtx.IsSuperAdmin() {
		return nil
	}

	// 检查用户是否属于该租户
	if userCtx.TenantID != tenantID {
		return fmt.Errorf("无权访问该租户数据")
	}

	// 检查租户是否有效
	tenant := a.tenantManager.GetTenant(tenantID)
	if tenant == nil {
		return fmt.Errorf("租户不存在")
	}

	if !tenant.IsActive() {
		return fmt.Errorf("租户未激活或已过期")
	}

	return nil
}

// GetUserTenantID 获取用户所属租户ID
func (a *AuthIntegration) GetUserTenantID(ctx context.Context) (string, error) {
	userCtx := auth.GetUserFromContext(ctx)
	if userCtx == nil {
		return "", fmt.Errorf("未认证")
	}
	return userCtx.TenantID, nil
}

// ==================== 告警操作权限 ====================

// CanAcknowledgeAlert 是否可以确认告警
func (a *AuthIntegration) CanAcknowledgeAlert(ctx context.Context, alert *AlertEvent) bool {
	// 检查基本编辑权限
	if err := a.CheckAlertEditPermission(ctx); err != nil {
		return false
	}

	// 检查数据范围
	userCtx := auth.GetUserFromContext(ctx)
	if userCtx == nil {
		return false
	}

	return a.checkAlertDataScope(userCtx, alert)
}

// CanResolveAlert 是否可以解决告警
func (a *AuthIntegration) CanResolveAlert(ctx context.Context, alert *AlertEvent) bool {
	return a.CanAcknowledgeAlert(ctx, alert)
}

// CanModifyAlertRule 是否可以修改告警规则
func (a *AuthIntegration) CanModifyAlertRule(ctx context.Context) bool {
	return a.CheckAlertManagePermission(ctx) == nil
}

// CanDeleteAlert 是否可以删除告警
func (a *AuthIntegration) CanDeleteAlert(ctx context.Context, alert *AlertEvent) bool {
	userCtx := auth.GetUserFromContext(ctx)
	if userCtx == nil {
		return false
	}

	// 管理员可以删除
	if userCtx.IsTenantAdmin() {
		return a.checkAlertDataScope(userCtx, alert)
	}

	// 普通用户需要管理权限
	if err := a.CheckAlertManagePermission(ctx); err != nil {
		return false
	}

	return a.checkAlertDataScope(userCtx, alert)
}

// ==================== 告警API处理器（带权限检查） ====================

// AlertHandler 告警处理器
type AlertHandler struct {
	manager       *AlertManager
	authIntegration *AuthIntegration
}

// NewAlertHandler 创建告警处理器
func NewAlertHandler(manager *AlertManager, authIntegration *AuthIntegration) *AlertHandler {
	return &AlertHandler{
		manager:         manager,
		authIntegration: authIntegration,
	}
}

// GetActiveAlerts 获取活跃告警（带权限过滤）
func (h *AlertHandler) GetActiveAlerts(ctx context.Context) ([]*AlertEvent, error) {
	// 检查权限
	if err := h.authIntegration.CheckAlertViewPermission(ctx); err != nil {
		return nil, err
	}

	// 获取所有活跃告警
	alerts := h.manager.GetActiveAlerts()

	// 按数据范围过滤
	return h.authIntegration.FilterAlertsByDataScope(ctx, alerts)
}

// GetAlertHistory 获取告警历史（带权限过滤）
func (h *AlertHandler) GetAlertHistory(ctx context.Context, limit int) ([]*AlertEvent, error) {
	// 检查权限
	if err := h.authIntegration.CheckAlertViewPermission(ctx); err != nil {
		return nil, err
	}

	// 获取历史告警
	alerts := h.manager.GetAlertHistory(limit)

	// 按数据范围过滤
	return h.authIntegration.FilterAlertsByDataScope(ctx, alerts)
}

// AcknowledgeAlert 确认告警
func (h *AlertHandler) AcknowledgeAlert(ctx context.Context, fingerprint string) error {
	// 获取告警
	alert := h.manager.GetAlertByFingerprint(fingerprint)
	if alert == nil {
		return fmt.Errorf("告警不存在")
	}

	// 检查权限
	if !h.authIntegration.CanAcknowledgeAlert(ctx, alert) {
		return fmt.Errorf("无权确认此告警")
	}

	// 执行确认操作
	// 这里可以添加确认逻辑，如更新告警状态等
	return nil
}

// ResolveAlert 解决告警
func (h *AlertHandler) ResolveAlert(ctx context.Context, fingerprint string, reason string) error {
	// 获取告警
	alert := h.manager.GetAlertByFingerprint(fingerprint)
	if alert == nil {
		return fmt.Errorf("告警不存在")
	}

	// 检查权限
	if !h.authIntegration.CanResolveAlert(ctx, alert) {
		return fmt.Errorf("无权解决此告警")
	}

	// 执行解决操作
	return h.manager.ForceResolve(fingerprint, reason)
}

// GetAlertStats 获取告警统计（带租户隔离）
func (h *AlertHandler) GetAlertStats(ctx context.Context) (map[string]interface{}, error) {
	// 检查权限
	if err := h.authIntegration.CheckAlertViewPermission(ctx); err != nil {
		return nil, err
	}

	// 获取统计
	stats := h.manager.GetStats()

	// 添加租户信息
	userCtx := auth.GetUserFromContext(ctx)
	if userCtx != nil {
		stats["user_tenant_id"] = userCtx.TenantID
		stats["user_roles"] = userCtx.Roles
	}

	return stats, nil
}

// ==================== 权限辅助函数 ====================

// RequireAlertView 要求告警查看权限
func RequireAlertView(authIntegration *AuthIntegration) func(context.Context) error {
	return func(ctx context.Context) error {
		return authIntegration.CheckAlertViewPermission(ctx)
	}
}

// RequireAlertEdit 要求告警编辑权限
func RequireAlertEdit(authIntegration *AuthIntegration) func(context.Context) error {
	return func(ctx context.Context) error {
		return authIntegration.CheckAlertEditPermission(ctx)
	}
}

// RequireAlertManage 要求告警管理权限
func RequireAlertManage(authIntegration *AuthIntegration) func(context.Context) error {
	return func(ctx context.Context) error {
		return authIntegration.CheckAlertManagePermission(ctx)
	}
}
