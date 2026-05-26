// Package authservice Auth Service 服务
//
// 职责:
//   - 用户认证 (JWT)
//   - Token 验证
//   - RBAC 鉴权
//   - API Key 管理
//
// 端口:
//   - gRPC: 9006
//   - HTTP: 8006
//   - Metrics: 9106
package authservice

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"sync"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/health"
	healthpb "google.golang.org/grpc/health/grpc_health_v1"

	svcproto "cloud-flow/services/proto"
)

// Config 服务配置
type Config struct {
	ServiceName string
	Version     string
	GrpcAddr    string // :9006
	HttpAddr    string // :8006

	JWTSecret     string
	JWTExpireSec  int64
	TiDBAddr      string // 用户数据存储

	// RBAC
	SuperAdminRole string
}

func DefaultConfig() *Config {
	return &Config{
		ServiceName:    "auth-service",
		Version:        "1.0.0",
		GrpcAddr:       ":9006",
		HttpAddr:       ":8006",
		JWTExpireSec:   86400,
		SuperAdminRole: "super_admin",
	}
}

// Service Auth Service
type Service struct {
	config *Config

	// 用户缓存
	users sync.Map // userId -> *UserInfo

	// API Key 存储
	apiKeys sync.Map // apiKey -> *APIKeyInfo

	// gRPC/HTTP
	grpcServer *grpc.Server
	health     *health.Server
	httpServer *http.Server

	startTime time.Time
}

// UserInfo 用户信息
type UserInfo struct {
	UserId   string
	Username string
	Password string // bcrypt hash
	Role     string
	TenantId string
}

// APIKeyInfo API Key 信息
type APIKeyInfo struct {
	Key      string
	UserId   string
	Name     string
	TenantId string
	ExpiresAt int64
}

func New(config *Config) (*Service, error) {
	if config == nil {
		config = DefaultConfig()
	}

	s := &Service{
		config:    config,
		startTime: time.Now(),
		health:    health.NewServer(),
	}

	s.grpcServer = grpc.NewServer()
	RegisterAuthService(s.grpcServer, s)
	healthpb.RegisterHealthServer(s.grpcServer, s.health)

	return s, nil
}

func (s *Service) Start() error {
	lis, err := net.Listen("tcp", s.config.GrpcAddr)
	if err != nil {
		return err
	}
	go func() { s.grpcServer.Serve(lis) }()

	mux := http.NewServeMux()
	mux.HandleFunc("/healthz", s.healthzHandler)
	mux.HandleFunc("/api/login", s.loginHandler)
	mux.HandleFunc("/api/verify", s.verifyHandler)
	s.httpServer = &http.Server{Addr: s.config.HttpAddr, Handler: mux}
	go func() { s.httpServer.ListenAndServe() }()

	s.health.SetServingStatus(s.config.ServiceName, healthpb.HealthCheckResponse_SERVING)
	fmt.Printf("Auth Service started: gRPC=%s, HTTP=%s\n", s.config.GrpcAddr, s.config.HttpAddr)
	return nil
}

func (s *Service) Stop() {
	s.health.SetServingStatus(s.config.ServiceName, healthpb.HealthCheckResponse_NOT_SERVING)
	if s.httpServer != nil { s.httpServer.Close() }
	if s.grpcServer != nil { s.grpcServer.GracefulStop() }
}

// Authenticate 认证
func (s *Service) Authenticate(ctx context.Context, req *svcproto.AuthenticateRequest) (*svcproto.AuthenticateResponse, error) {
	// TODO: 查询用户并验证密码
	return &svcproto.AuthenticateResponse{
		Token:     "jwt_token_placeholder",
		ExpiresAt: time.Now().Add(time.Duration(s.config.JWTExpireSec) * time.Second).Unix(),
		UserId:    "user_1",
		Role:      "admin",
	}, nil
}

// ValidateToken 验证 Token
func (s *Service) ValidateToken(ctx context.Context, req *svcproto.ValidateTokenRequest) (*svcproto.ValidateTokenResponse, error) {
	// TODO: 验证 JWT
	return &svcproto.ValidateTokenResponse{
		Valid:    true,
		UserId:   "user_1",
		Username: "admin",
		Role:     "admin",
		TenantId: "tenant_1",
	}, nil
}

// Authorize 鉴权
func (s *Service) Authorize(ctx context.Context, req *svcproto.AuthorizeRequest) (*svcproto.AuthorizeResponse, error) {
	// TODO: RBAC 检查
	return &svcproto.AuthorizeResponse{Allowed: true}, nil
}

func (s *Service) healthzHandler(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusOK)
	fmt.Fprintf(w, `{"status":"healthy","service":"%s"}`, s.config.ServiceName)
}
func (s *Service) loginHandler(w http.ResponseWriter, r *http.Request)  { w.WriteHeader(http.StatusOK); fmt.Fprint(w, `{"token":"jwt_placeholder"}`) }
func (s *Service) verifyHandler(w http.ResponseWriter, r *http.Request) { w.WriteHeader(http.StatusOK); fmt.Fprint(w, `{"valid":true}`) }
