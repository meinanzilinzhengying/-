// Package tenantservice Tenant Service 服务
//
// 职责:
//   - 租户 CRUD
//   - 配额管理
//   - 计费/Plan 管理
//   - 租户隔离
//
// 端口:
//   - gRPC: 9007
//   - HTTP: 8007
//   - Metrics: 9107
package tenantservice

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
	GrpcAddr    string // :9007
	HttpAddr    string // :8007

	TiDBAddr string // 租户数据存储

	DefaultRetentionDays int
	DefaultMaxAgents     int
}

func DefaultConfig() *Config {
	return &Config{
		ServiceName:          "tenant-service",
		Version:              "1.0.0",
		GrpcAddr:             ":9007",
		HttpAddr:             ":8007",
		DefaultRetentionDays: 30,
		DefaultMaxAgents:     100,
	}
}

// Service Tenant Service
type Service struct {
	config *Config

	// 租户存储
	tenants sync.Map // tenantId -> *svcproto.Tenant

	// gRPC/HTTP
	grpcServer *grpc.Server
	health     *health.Server
	httpServer *http.Server

	startTime time.Time
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
	RegisterTenantService(s.grpcServer, s)
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
	mux.HandleFunc("/api/tenants", s.listTenantsHTTPHandler)
	s.httpServer = &http.Server{Addr: s.config.HttpAddr, Handler: mux}
	go func() { s.httpServer.ListenAndServe() }()

	s.health.SetServingStatus(s.config.ServiceName, healthpb.HealthCheckResponse_SERVING)
	fmt.Printf("Tenant Service started: gRPC=%s, HTTP=%s\n", s.config.GrpcAddr, s.config.HttpAddr)
	return nil
}

func (s *Service) Stop() {
	s.health.SetServingStatus(s.config.ServiceName, healthpb.HealthCheckResponse_NOT_SERVING)
	if s.httpServer != nil { s.httpServer.Close() }
	if s.grpcServer != nil { s.grpcServer.GracefulStop() }
}

// CreateTenant 创建租户
func (s *Service) CreateTenant(ctx context.Context, req *svcproto.CreateTenantRequest) (*svcproto.CreateTenantResponse, error) {
	tenant := &svcproto.Tenant{
		Id:          generateID(),
		Name:        req.Name,
		DisplayName: req.DisplayName,
		Status:      "active",
		Plan:        req.Plan,
		Quota:       req.Quota,
		CreatedAt:   time.Now().Unix(),
		UpdatedAt:   time.Now().Unix(),
	}

	if tenant.Quota == nil {
		tenant.Quota = &svcproto.TenantQuota{
			MaxAgents:      int(s.config.DefaultMaxAgents),
			RetentionDays: s.config.DefaultRetentionDays,
		}
	}

	s.tenants.Store(tenant.Id, tenant)
	return &svcproto.CreateTenantResponse{Tenant: tenant}, nil
}

// GetTenant 获取租户
func (s *Service) GetTenant(ctx context.Context, req *svcproto.GetTenantRequest) (*svcproto.GetTenantResponse, error) {
	v, ok := s.tenants.Load(req.TenantId)
	if !ok {
		return nil, fmt.Errorf("tenant %s not found", req.TenantId)
	}
	return &svcproto.GetTenantResponse{Tenant: v.(*svcproto.Tenant)}, nil
}

// ListTenants 列出租户
func (s *Service) ListTenants(ctx context.Context, req *svcproto.ListTenantsRequest) (*svcproto.ListTenantsResponse, error) {
	var tenants []*svcproto.Tenant
	s.tenants.Range(func(_, v interface{}) bool {
		t := v.(*svcproto.Tenant)
		if req.Status != "" && t.Status != req.Status {
			return true
		}
		if req.Plan != "" && t.Plan != req.Plan {
			return true
		}
		tenants = append(tenants, t)
		return true
	})
	return &svcproto.ListTenantsResponse{Tenants: tenants, Total: len(tenants)}, nil
}

// UpdateQuota 更新配额
func (s *Service) UpdateQuota(ctx context.Context, req *svcproto.UpdateTenantQuotaRequest) (*svcproto.UpdateTenantQuotaResponse, error) {
	v, ok := s.tenants.Load(req.TenantId)
	if !ok {
		return &svcproto.UpdateTenantQuotaResponse{Success: false, Message: "tenant not found"}, nil
	}

	tenant := v.(*svcproto.Tenant)
	tenant.Quota = req.Quota
	tenant.UpdatedAt = time.Now().Unix()
	s.tenants.Store(req.TenantId, tenant)

	return &svcproto.UpdateTenantQuotaResponse{Success: true, Message: "quota updated"}, nil
}

func generateID() string {
	return fmt.Sprintf("tenant_%d", time.Now().UnixNano())
}

func (s *Service) healthzHandler(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusOK)
	fmt.Fprintf(w, `{"status":"healthy","service":"%s"}`, s.config.ServiceName)
}
func (s *Service) listTenantsHTTPHandler(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusOK)
	fmt.Fprint(w, `{"tenants":[],"total":0}`)
}
