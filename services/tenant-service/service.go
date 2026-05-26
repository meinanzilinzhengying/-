// Package tenantservice Tenant Service 服务
//
// 职责:
//   - 租户 CRUD
//   - 项目管理 (Project)
//   - 配额管理
//   - Plan 管理 (free/pro/enterprise)
//   - 租户隔离
//   - tenant_id 全链路透传
//
// 端口:
//   - gRPC: 9007
//   - HTTP: 8007
//   - Metrics: 9107
package tenantservice

import (
	"context"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"sync"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/health"
	healthpb "google.golang.org/grpc/health/grpc_health_v1"

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
	GrpcAddr    string // :9007
	HttpAddr    string // :8007

	TiDBAddr string

	DefaultRetentionDays int
	DefaultMaxAgents     int
	DefaultMaxFlowsPerDay int64
	DefaultMaxStorageGB  int
	DefaultMaxAlertRules int
}

// DefaultConfig 默认配置
func DefaultConfig() *Config {
	return &Config{
		ServiceName:          "tenant-service",
		Version:              "1.0.0",
		GrpcAddr:             ":9007",
		HttpAddr:             ":8007",
		DefaultRetentionDays: 30,
		DefaultMaxAgents:     100,
		DefaultMaxFlowsPerDay: 10_000_000,
		DefaultMaxStorageGB:  100,
		DefaultMaxAlertRules: 100,
	}
}

// ============================================================================
// 服务
// ============================================================================

// Service Tenant Service
type Service struct {
	config *Config

	// 存储
	tenants  sync.Map // tenantId -> *svcproto.Tenant
	projects sync.Map // projectId -> *svcproto.Project

	// gRPC/HTTP
	grpcServer *grpc.Server
	health     *health.Server
	httpServer *http.Server

	startTime time.Time
}

// New 创建服务
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

// Start 启动服务
func (s *Service) Start() error {
	// gRPC (带租户拦截器)
	lis, err := net.Listen("tcp", s.config.GrpcAddr)
	if err != nil {
		return fmt.Errorf("gRPC listen failed: %w", err)
	}

	grpcServer := grpc.NewServer(
		grpc.UnaryInterceptor(tenant.GRPCUnaryServerInterceptor()),
	)
	RegisterTenantService(grpcServer, s)
	healthpb.RegisterHealthServer(grpcServer, s.health)
	s.grpcServer = grpcServer
	go func() { s.grpcServer.Serve(lis) }()

	// HTTP (带租户中间件)
	mux := http.NewServeMux()
	mux.HandleFunc("/healthz", s.healthzHandler)
	mux.HandleFunc("/api/tenants", s.tenantsHTTPHandler)
	mux.HandleFunc("/api/projects", s.projectsHTTPHandler)
	mux.HandleFunc("/api/quotas", s.quotaHTTPHandler)

	s.httpServer = &http.Server{
		Addr:    s.config.HttpAddr,
		Handler: tenant.HTTPMiddleware(mux),
	}
	go func() { s.httpServer.ListenAndServe() }()

	s.health.SetServingStatus(s.config.ServiceName, healthpb.HealthCheckResponse_SERVING)
	fmt.Printf("Tenant Service started: gRPC=%s, HTTP=%s\n", s.config.GrpcAddr, s.config.HttpAddr)
	return nil
}

// Stop 停止服务
func (s *Service) Stop() {
	s.health.SetServingStatus(s.config.ServiceName, healthpb.HealthCheckResponse_NOT_SERVING)
	if s.httpServer != nil {
		s.httpServer.Close()
	}
	if s.grpcServer != nil {
		s.grpcServer.GracefulStop()
	}
}

// ============================================================================
// gRPC: 租户管理
// ============================================================================

// CreateTenant 创建租户
func (s *Service) CreateTenant(ctx context.Context, req *svcproto.CreateTenantRequest) (*svcproto.CreateTenantResponse, error) {
	tc := tenant.MustFromContext(ctx)

	tenant := &svcproto.Tenant{
		Id:          generateID("tenant"),
		Name:        req.Name,
		DisplayName: req.DisplayName,
		Status:      "active",
		Plan:        req.Plan,
		Quota:       s.defaultQuota(req.Quota),
		CreatedAt:   time.Now().Unix(),
		UpdatedAt:   time.Now().Unix(),
	}

	if tenant.Plan == "" {
		tenant.Plan = "free"
	}

	s.tenants.Store(tenant.Id, tenant)

	// 创建默认项目
	defaultProject := &svcproto.Project{
		Id:          generateID("proj"),
		TenantId:    tenant.Id,
		Name:        "default",
		DisplayName: "Default Project",
		Description: "Auto-created default project",
		Status:      "active",
		Namespaces:  []string{tenant.Name + "-default"},
		CreatedAt:   time.Now().Unix(),
		UpdatedAt:   time.Now().Unix(),
	}
	s.projects.Store(defaultProject.Id, defaultProject)

	return &svcproto.CreateTenantResponse{Tenant: tenant}, nil
}

// GetTenant 获取租户
func (s *Service) GetTenant(ctx context.Context, req *svcproto.GetTenantRequest) (*svcproto.GetTenantResponse, error) {
	tc := tenant.MustFromContext(ctx)

	// 租户隔离: 只能查询自己的租户
	if err := tenant.ValidateTenantAccess(ctx, req.TenantId); err != nil {
		return nil, err
	}

	v, ok := s.tenants.Load(req.TenantId)
	if !ok {
		return nil, fmt.Errorf("tenant %s not found", req.TenantId)
	}
	return &svcproto.GetTenantResponse{Tenant: v.(*svcproto.Tenant)}, nil
}

// ListTenants 列出租户
func (s *Service) ListTenants(ctx context.Context, req *svcproto.ListTenantsRequest) (*svcproto.ListTenantsResponse, error) {
	tc := tenant.MustFromContext(ctx)

	var tenants []*svcproto.Tenant
	s.tenants.Range(func(_, v interface{}) bool {
		t := v.(*svcproto.Tenant)
		// 租户隔离: 非平台管理员只能看到自己的租户
		if !tc.IsPlatformAdmin && t.Id != tc.TenantID {
			return true
		}
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
	tenant.MustFromContext(ctx)

	v, ok := s.tenants.Load(req.TenantId)
	if !ok {
		return &svcproto.UpdateTenantQuotaResponse{Success: false, Message: "tenant not found"}, nil
	}

	t := v.(*svcproto.Tenant)
	t.Quota = req.Quota
	t.UpdatedAt = time.Now().Unix()
	s.tenants.Store(req.TenantId, t)

	return &svcproto.UpdateTenantQuotaResponse{Success: true, Message: "quota updated"}, nil
}

// ============================================================================
// gRPC: 项目管理
// ============================================================================

// CreateProject 创建项目
func (s *Service) CreateProject(ctx context.Context, req *svcproto.CreateProjectRequest) (*svcproto.CreateProjectResponse, error) {
	tc := tenant.MustFromContext(ctx)

	project := &svcproto.Project{
		Id:          generateID("proj"),
		TenantId:    tc.TenantID,
		Name:        req.Name,
		DisplayName: req.DisplayName,
		Description: req.Description,
		Status:      "active",
		Namespaces:  req.Namespaces,
		CreatedAt:   time.Now().Unix(),
		UpdatedAt:   time.Now().Unix(),
	}

	if project.DisplayName == "" {
		project.DisplayName = project.Name
	}
	if project.Namespaces == nil {
		project.Namespaces = []string{tc.TenantID + "-" + project.Name}
	}

	s.projects.Store(project.Id, project)
	return &svcproto.CreateProjectResponse{Project: project}, nil
}

// ListProjects 列出项目
func (s *Service) ListProjects(ctx context.Context, req *svcproto.ListProjectsRequest) (*svcproto.ListProjectsResponse, error) {
	tc := tenant.MustFromContext(ctx)

	var projects []*svcproto.Project
	s.projects.Range(func(_, v interface{}) bool {
		p := v.(*svcproto.Project)
		// 租户隔离
		if p.TenantId != tc.TenantID {
			return true
		}
		if req.Status != "" && p.Status != req.Status {
			return true
		}
		projects = append(projects, p)
		return true
	})
	return &svcproto.ListProjectsResponse{Projects: projects, Total: len(projects)}, nil
}

// ============================================================================
// HTTP Handlers
// ============================================================================

func (s *Service) healthzHandler(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusOK)
	fmt.Fprintf(w, `{"status":"healthy","service":"%s","version":"%s"}`,
		s.config.ServiceName, s.config.Version)
}

func (s *Service) tenantsHTTPHandler(w http.ResponseWriter, r *http.Request) {
	tc, ok := tenant.FromContext(r.Context())
	if !ok {
		http.Error(w, "tenant context required", http.StatusForbidden)
		return
	}

	switch r.Method {
	case http.MethodGet:
		resp, _ := s.ListTenants(r.Context(), &svcproto.ListTenantsRequest{})
		writeJSON(w, resp)
	case http.MethodPost:
		var req svcproto.CreateTenantRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		resp, err := s.CreateTenant(r.Context(), &req)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		writeJSON(w, resp)
	default:
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
	}
}

func (s *Service) projectsHTTPHandler(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		resp, _ := s.ListProjects(r.Context(), &svcproto.ListProjectsRequest{})
		writeJSON(w, resp)
	case http.MethodPost:
		var req svcproto.CreateProjectRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		resp, err := s.CreateProject(r.Context(), &req)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		writeJSON(w, resp)
	default:
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
	}
}

func (s *Service) quotaHTTPHandler(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodPut:
		var req svcproto.UpdateTenantQuotaRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		resp, _ := s.UpdateQuota(r.Context(), &req)
		writeJSON(w, resp)
	default:
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
	}
}

// ============================================================================
// 辅助函数
// ============================================================================

func (s *Service) defaultQuota(custom *svcproto.TenantQuota) *svcproto.TenantQuota {
	if custom != nil {
		return custom
	}
	return &svcproto.TenantQuota{
		MaxAgents:       s.config.DefaultMaxAgents,
		MaxFlowsPerDay:  s.config.DefaultMaxFlowsPerDay,
		MaxStorageGB:    s.config.DefaultMaxStorageGB,
		MaxAlertRules:   s.config.DefaultMaxAlertRules,
		RetentionDays:   s.config.DefaultRetentionDays,
	}
}

func generateID(prefix string) string {
	return fmt.Sprintf("%s_%d", prefix, time.Now().UnixNano())
}

func writeJSON(w http.ResponseWriter, v interface{}) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(v)
}
