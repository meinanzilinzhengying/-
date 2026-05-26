// Package authservice Auth Service 服务
//
// 职责:
//   - JWT 认证签发与验证
//   - OIDC SSO 集成
//   - API Key 管理
//   - Casbin RBAC 鉴权
//   - tenant_id 全链路透传
//   - 权限模型: tenant → project → role → policy
//
// 端口:
//   - gRPC: 9006
//   - HTTP: 8006
//   - Metrics: 9106
package authservice

import (
	"context"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"strings"
	"sync"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/health"
	healthpb "google.golang.org/grpc/health/grpc_health_v1"

	"cloud-flow/services/auth-service/auth"
	"cloud-flow/services/auth-service/rbac"
	svcproto "cloud-flow/services/proto"
	"cloud-flow/services/shared/tenant"
)

// ============================================================================
// 配置
// ============================================================================

// Config 服务配置
type Config struct {
	ServiceName string
	Version     string
	GrpcAddr    string // :9006
	HttpAddr    string // :8006

	// JWT
	JWTSecret      string
	JWTIssuer      string
	JWTExpireSec   int64
	JWTRefreshSec  int64

	// OIDC (可选)
	OIDCIssuer       string
	OIDCClientID     string
	OIDCClientSecret string
	OIDCRedirectURL  string
	OIDCScopes       string

	// RBAC
	SuperAdminRole string

	// TiDB
	TiDBAddr string
}

// DefaultConfig 默认配置
func DefaultConfig() *Config {
	return &Config{
		ServiceName:    "auth-service",
		Version:        "1.0.0",
		GrpcAddr:       ":9006",
		HttpAddr:       ":8006",
		JWTIssuer:      "cloudflow",
		JWTExpireSec:   86400,   // 24h
		JWTRefreshSec:  604800,  // 7d
		SuperAdminRole: "super_admin",
	}
}

// ============================================================================
// 服务
// ============================================================================

// Service Auth Service
type Service struct {
	config *Config

	// 认证
	authenticator *auth.Authenticator

	// RBAC
	rbacEngine *rbac.RBACEngine

	// 用户缓存
	users sync.Map // userId -> *UserInfo

	// API Key 存储 (委托给 authenticator)

	// gRPC/HTTP
	grpcServer *grpc.Server
	health     *health.Server
	httpServer *http.Server

	startTime time.Time
}

// UserInfo 用户信息
type UserInfo struct {
	UserID   string
	Username string
	Password string // bcrypt hash
	TenantID string
	Role     string
}

// New 创建服务
func New(config *Config) (*Service, error) {
	if config == nil {
		config = DefaultConfig()
	}

	// 初始化认证器
	authenticator := auth.NewAuthenticator(
		config.JWTSecret,
		config.JWTIssuer,
		config.JWTExpireSec,
		config.JWTRefreshSec,
		nil, // OIDC 在 Start 中初始化
	)

	// 初始化 RBAC 引擎
	rbacEngine := rbac.NewRBACEngine(rbac.RBACConfig{
		SuperAdminRole:       config.SuperAdminRole,
		DefaultPoliciesEnabled: true,
	})

	s := &Service{
		config:        config,
		authenticator: authenticator,
		rbacEngine:    rbacEngine,
		startTime:     time.Now(),
		health:        health.NewServer(),
	}

	s.grpcServer = grpc.NewServer()
	RegisterAuthService(s.grpcServer, s)
	healthpb.RegisterHealthServer(s.grpcServer, s.health)

	return s, nil
}

// Start 启动服务
func (s *Service) Start() error {
	// 启动 RBAC 引擎
	s.rbacEngine.Start(context.Background())

	// 注册内置角色
	s.rbacEngine.AddBuiltinRoles()

	// 启动 gRPC (带租户拦截器)
	lis, err := net.Listen("tcp", s.config.GrpcAddr)
	if err != nil {
		return fmt.Errorf("gRPC listen failed: %w", err)
	}

	grpcServer := grpc.NewServer(
		grpc.UnaryInterceptor(tenant.GRPCUnaryServerInterceptor()),
	)
	RegisterAuthService(grpcServer, s)
	healthpb.RegisterHealthServer(grpcServer, s.health)
	s.grpcServer = grpcServer
	go func() { s.grpcServer.Serve(lis) }()

	// 启动 HTTP (带租户中间件)
	mux := http.NewServeMux()
	mux.HandleFunc("/healthz", s.healthzHandler)
	mux.HandleFunc("/api/login", s.loginHandler)
	mux.HandleFunc("/api/verify", s.verifyHandler)
	mux.HandleFunc("/api/refresh", s.refreshHandler)
	mux.HandleFunc("/api/oidc/callback", s.oidcCallbackHandler)
	mux.HandleFunc("/api/oidc/auth", s.oidcAuthHandler)
	mux.HandleFunc("/api/roles", s.rolesHandler)
	mux.HandleFunc("/api/permissions/check", s.checkPermissionHandler)
	mux.HandleFunc("/api/apikeys", s.apiKeyHandler)

	s.httpServer = &http.Server{
		Addr:    s.config.HttpAddr,
		Handler: tenant.HTTPMiddleware(mux),
	}
	go func() { s.httpServer.ListenAndServe() }()

	s.health.SetServingStatus(s.config.ServiceName, healthpb.HealthCheckResponse_SERVING)
	fmt.Printf("Auth Service started: gRPC=%s, HTTP=%s\n", s.config.GrpcAddr, s.config.HttpAddr)
	return nil
}

// Stop 停止服务
func (s *Service) Stop() {
	s.rbacEngine.Stop()
	s.health.SetServingStatus(s.config.ServiceName, healthpb.HealthCheckResponse_NOT_SERVING)
	if s.httpServer != nil {
		s.httpServer.Close()
	}
	if s.grpcServer != nil {
		s.grpcServer.GracefulStop()
	}
}

// ============================================================================
// gRPC 服务方法
// ============================================================================

// Authenticate 认证 (用户名+密码 → JWT)
func (s *Service) Authenticate(ctx context.Context, req *svcproto.AuthenticateRequest) (*svcproto.AuthenticateResponse, error) {
	// 查找用户
	user, ok := s.findUser(req.Username)
	if !ok {
		return nil, fmt.Errorf("invalid credentials")
	}

	// TODO: bcrypt.VerifyHash
	if user.Password != req.Password {
		return nil, fmt.Errorf("invalid credentials")
	}

	// 签发 JWT
	token, err := s.authenticator.JWTManager().GenerateToken(
		user.UserID, user.TenantID, user.Username, user.Role, "", nil, false,
	)
	if err != nil {
		return nil, fmt.Errorf("token generation failed: %w", err)
	}

	return &svcproto.AuthenticateResponse{
		Token:     token,
		ExpiresAt: time.Now().Add(time.Duration(s.config.JWTExpireSec) * time.Second).Unix(),
		UserId:    user.UserID,
		Role:      user.Role,
	}, nil
}

// ValidateToken 验证 Token
func (s *Service) ValidateToken(ctx context.Context, req *svcproto.ValidateTokenRequest) (*svcproto.ValidateTokenResponse, error) {
	claims, err := s.authenticator.Authenticate(ctx, req.Token)
	if err != nil {
		return &svcproto.ValidateTokenResponse{Valid: false}, nil
	}

	return &svcproto.ValidateTokenResponse{
		Valid:    true,
		UserId:   claims.Subject,
		Username: claims.Username,
		Role:     claims.Role,
		TenantId: claims.TenantID,
	}, nil
}

// Authorize 鉴权 (Casbin RBAC)
func (s *Service) Authorize(ctx context.Context, req *svcproto.AuthorizeRequest) (*svcproto.AuthorizeResponse, error) {
	// 从 context 获取租户信息
	tc, ok := tenant.FromContext(ctx)
	if !ok {
		return &svcproto.AuthorizeResponse{Allowed: false, Reason: "no tenant context"}, nil
	}

	// 使用 Casbin 检查权限
	allowed, reason := s.rbacEngine.CheckPermission(
		req.UserId, tc.TenantID, tc.ProjectID, req.Resource, req.Action,
	)

	return &svcproto.AuthorizeResponse{
		Allowed: allowed,
		Reason:  reason,
	}, nil
}

// CreateRole 创建角色
func (s *Service) CreateRole(ctx context.Context, req *svcproto.CreateRoleRequest) (*svcproto.CreateRoleResponse, error) {
	tc := tenant.MustFromContext(ctx)

	role := &svcproto.Role{
		Id:          fmt.Sprintf("role_%d", time.Now().UnixNano()),
		TenantId:    req.TenantId,
		ProjectId:   req.ProjectId,
		Name:        req.Name,
		DisplayName: req.DisplayName,
		Description: req.Description,
		Permissions: req.Permissions,
		IsBuiltin:   false,
		CreatedAt:   time.Now().Unix(),
	}

	// 为角色添加 Casbin 策略
	for _, perm := range req.Permissions {
		action, resource := parsePermission(perm)
		if action != "" {
			s.rbacEngine.AddPolicy(req.Name, tc.TenantID, req.ProjectId, resource, action, "allow")
		}
	}

	return &svcproto.CreateRoleResponse{Role: role}, nil
}

// BindUserRole 绑定用户角色
func (s *Service) BindUserRole(ctx context.Context, req *svcproto.BindUserRoleRequest) (*svcproto.BindUserRoleResponse, error) {
	tc := tenant.MustFromContext(ctx)

	err := s.rbacEngine.AddRoleForUser(req.UserId, req.RoleId, req.TenantId, req.ProjectId)
	if err != nil {
		return &svcproto.BindUserRoleResponse{Success: false, Message: err.Error()}, nil
	}

	return &svcproto.BindUserRoleResponse{Success: true, Message: "role bound"}, nil
}

// CreatePolicy 创建策略
func (s *Service) CreatePolicy(ctx context.Context, req *svcproto.CreatePolicyRequest) (*svcproto.CreatePolicyResponse, error) {
	tc := tenant.MustFromContext(ctx)

	policy := &svcproto.Policy{
		Id:          fmt.Sprintf("policy_%d", time.Now().UnixNano()),
		TenantId:    req.TenantId,
		ProjectId:   req.ProjectId,
		Name:        req.Name,
		Description: req.Description,
		Effect:      req.Effect,
		Actions:     req.Actions,
		Resources:   req.Resources,
		Conditions:  req.Conditions,
		Priority:    req.Priority,
		CreatedAt:   time.Now().Unix(),
		UpdatedAt:   time.Now().Unix(),
	}

	// 添加到 Casbin
	for _, action := range req.Actions {
		for _, resource := range req.Resources {
			s.rbacEngine.AddPolicy("*", tc.TenantID, req.ProjectId, resource, action, req.Effect)
		}
	}

	return &svcproto.CreatePolicyResponse{Policy: policy}, nil
}

// CheckPermission 检查权限
func (s *Service) CheckPermission(ctx context.Context, req *svcproto.CheckPermissionRequest) (*svcproto.CheckPermissionResponse, error) {
	tc, ok := tenant.FromContext(ctx)
	if !ok {
		return &svcproto.CheckPermissionResponse{Allowed: false, Reason: "no tenant context"}, nil
	}

	allowed, reason := s.rbacEngine.CheckPermission(
		req.UserId, req.TenantId, tc.ProjectID, req.Resource, req.Action,
	)

	return &svcproto.CheckPermissionResponse{Allowed: allowed, Reason: reason}, nil
}

// OIDCCallback OIDC 回调
func (s *Service) OIDCCallback(ctx context.Context, req *svcproto.OIDCCallbackRequest) (*svcproto.AuthenticateResponse, error) {
	claims, err := s.authenticator.Authenticate(ctx, req.Code)
	if err != nil {
		return nil, fmt.Errorf("OIDC callback failed: %w", err)
	}

	token, err := s.authenticator.JWTManager().GenerateToken(
		claims.Subject, claims.TenantID, claims.Username, claims.Role,
		claims.ProjectID, claims.Namespaces, claims.IsPlatformAdmin,
	)
	if err != nil {
		return nil, fmt.Errorf("token generation failed: %w", err)
	}

	return &svcproto.AuthenticateResponse{
		Token:     token,
		ExpiresAt: time.Now().Add(time.Duration(s.config.JWTExpireSec) * time.Second).Unix(),
		UserId:    claims.Subject,
		Role:      claims.Role,
	}, nil
}

// ============================================================================
// HTTP Handlers
// ============================================================================

func (s *Service) healthzHandler(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusOK)
	fmt.Fprintf(w, `{"status":"healthy","service":"%s","version":"%s"}`,
		s.config.ServiceName, s.config.Version)
}

func (s *Service) loginHandler(w http.ResponseWriter, r *http.Request) {
	var req svcproto.AuthenticateRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	resp, err := s.Authenticate(r.Context(), &req)
	if err != nil {
		http.Error(w, err.Error(), http.StatusUnauthorized)
		return
	}

	writeJSON(w, resp)
}

func (s *Service) verifyHandler(w http.ResponseWriter, r *http.Request) {
	token := extractBearerToken(r)
	if token == "" {
		http.Error(w, "missing token", http.StatusUnauthorized)
		return
	}

	resp, err := s.ValidateToken(r.Context(), &svcproto.ValidateTokenRequest{Token: token})
	if err != nil {
		http.Error(w, err.Error(), http.StatusUnauthorized)
		return
	}
	writeJSON(w, resp)
}

func (s *Service) refreshHandler(w http.ResponseWriter, r *http.Request) {
	var body struct {
		RefreshToken string `json:"refresh_token"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	newToken, err := s.authenticator.JWTManager().RefreshToken(body.RefreshToken)
	if err != nil {
		http.Error(w, err.Error(), http.StatusUnauthorized)
		return
	}

	writeJSON(w, map[string]interface{}{
		"token":      newToken,
		"expires_at": time.Now().Add(time.Duration(s.config.JWTExpireSec) * time.Second).Unix(),
	})
}

func (s *Service) oidcCallbackHandler(w http.ResponseWriter, r *http.Request) {
	code := r.URL.Query().Get("code")
	state := r.URL.Query().Get("state")

	resp, err := s.OIDCCallback(r.Context(), &svcproto.OIDCCallbackRequest{Code: code, State: state})
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	writeJSON(w, resp)
}

func (s *Service) oidcAuthHandler(w http.ResponseWriter, r *http.Request) {
	// 返回 OIDC 授权 URL
	if s.config.OIDCIssuer == "" {
		http.Error(w, "OIDC not configured", http.StatusNotImplemented)
		return
	}
	writeJSON(w, map[string]string{
		"issuer":       s.config.OIDCIssuer,
		"authorization_url": s.config.OIDCIssuer + "/auth",
		"client_id":    s.config.OIDCClientID,
		"redirect_url": s.config.OIDCRedirectURL,
		"scopes":       s.config.OIDCScopes,
	})
}

func (s *Service) rolesHandler(w http.ResponseWriter, r *http.Request) {
	tc := tenant.MustFromContext(r.Context())
	roles, _ := s.rbacEngine.GetRolesForUser(tc.UserID, tc.TenantID, tc.ProjectID)
	writeJSON(w, map[string]interface{}{"roles": roles})
}

func (s *Service) checkPermissionHandler(w http.ResponseWriter, r *http.Request) {
	tc := tenant.MustFromContext(r.Context())
	action := r.URL.Query().Get("action")
	resource := r.URL.Query().Get("resource")

	allowed, reason := s.rbacEngine.CheckPermission(tc.UserID, tc.TenantID, tc.ProjectID, resource, action)
	writeJSON(w, map[string]interface{}{"allowed": allowed, "reason": reason})
}

func (s *Service) apiKeyHandler(w http.ResponseWriter, r *http.Request) {
	tc := tenant.MustFromContext(r.Context())

	switch r.Method {
	case http.MethodPost:
		var body struct {
			Name string `json:"name"`
		}
		json.NewDecoder(r.Body).Decode(&body)
		key, err := s.authenticator.GenerateAPIKey(tc.UserID, tc.TenantID, body.Name, 30*24*time.Hour)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		writeJSON(w, map[string]string{"api_key": key})

	case http.MethodDelete:
		key := r.URL.Query().Get("key")
		s.authenticator.RevokeAPIKey(key)
		writeJSON(w, map[string]string{"status": "revoked"})

	default:
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
	}
}

// ============================================================================
// 辅助函数
// ============================================================================

func (s *Service) findUser(username string) (*UserInfo, bool) {
	var found *UserInfo
	s.users.Range(func(_, v interface{}) bool {
		u := v.(*UserInfo)
		if u.Username == username {
			found = u
			return false
		}
		return true
	})
	if found == nil {
		return nil, false
	}
	return found, true
}

// parsePermission 解析权限字符串 "flow:read" → (action="read", resource="flow")
func parsePermission(perm string) (action, resource string) {
	parts := strings.SplitN(perm, ":", 2)
	if len(parts) == 2 {
		return parts[1], parts[0]
	}
	return perm, "*"
}

func extractBearerToken(r *http.Request) string {
	auth := r.Header.Get("Authorization")
	if strings.HasPrefix(auth, "Bearer ") {
		return strings.TrimPrefix(auth, "Bearer ")
	}
	return r.URL.Query().Get("token")
}

func writeJSON(w http.ResponseWriter, v interface{}) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(v)
}
