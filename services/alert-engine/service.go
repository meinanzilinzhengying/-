// Package alertengine Alert Engine 服务
//
// 职责:
//   - 告警规则管理 (CRUD)
//   - 告警评估引擎
//   - 多渠道通知 (Email/Webhook/钉钉)
//   - 告警历史管理
//
// 端口:
//   - gRPC: 9005
//   - HTTP: 8005
//   - Metrics: 9105
package alertengine

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
	GrpcAddr    string // :9005
	HttpAddr    string // :8005

	DataPlaneAddr string
	TenantAddr    string

	// 评估配置
	EvalInterval time.Duration
	MaxRules     int
}

func DefaultConfig() *Config {
	return &Config{
		ServiceName:   "alert-engine",
		Version:       "1.0.0",
		GrpcAddr:      ":9005",
		HttpAddr:      ":8005",
		EvalInterval:  15 * time.Second,
		MaxRules:      10000,
	}
}

// Service Alert Engine
type Service struct {
	config *Config

	// 规则存储
	rules   sync.Map // ruleId -> *svcproto.AlertRule
	alerts  sync.Map // alertId -> *svcproto.Alert

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
	RegisterAlertService(s.grpcServer, s)
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
	mux.HandleFunc("/api/alerts", s.listAlertsHTTPHandler)
	mux.HandleFunc("/api/rules", s.listRulesHTTPHandler)
	mux.HandleFunc("/v1/alerts/webhook", s.webhookHandler)
	s.httpServer = &http.Server{Addr: s.config.HttpAddr, Handler: mux}
	go func() { s.httpServer.ListenAndServe() }()

	s.health.SetServingStatus(s.config.ServiceName, healthpb.HealthCheckResponse_SERVING)
	fmt.Printf("Alert Engine started: gRPC=%s, HTTP=%s\n", s.config.GrpcAddr, s.config.HttpAddr)

	go s.evalLoop()
	return nil
}

func (s *Service) Stop() {
	s.health.SetServingStatus(s.config.ServiceName, healthpb.HealthCheckResponse_NOT_SERVING)
	if s.httpServer != nil { s.httpServer.Close() }
	if s.grpcServer != nil { s.grpcServer.GracefulStop() }
}

// CreateRule 创建规则
func (s *Service) CreateRule(rule *svcproto.AlertRule) (*svcproto.AlertRule, error) {
	s.rules.Store(rule.Id, rule)
	return rule, nil
}

// GetRule 获取规则
func (s *Service) GetRule(rule *svcproto.AlertRule) (*svcproto.AlertRule, error) {
	v, ok := s.rules.Load(rule.Id)
	if !ok {
		return nil, fmt.Errorf("rule %s not found", rule.Id)
	}
	return v.(*svcproto.AlertRule), nil
}

// ListRules 列出规则
func (s *Service) ListRules() ([]*svcproto.AlertRule, error) {
	var rules []*svcproto.AlertRule
	s.rules.Range(func(_, v interface{}) bool {
		rules = append(rules, v.(*svcproto.AlertRule))
		return true
	})
	return rules, nil
}

// DeleteRule 删除规则
func (s *Service) DeleteRule(rule *svcproto.AlertRule) (*svcproto.AlertRule, error) {
	v, ok := s.rules.LoadAndDelete(rule.Id)
	if !ok {
		return nil, fmt.Errorf("rule %s not found", rule.Id)
	}
	return v.(*svcproto.AlertRule), nil
}

// ListAlerts 列出告警
func (s *Service) ListAlerts() ([]*svcproto.Alert, error) {
	var alerts []*svcproto.Alert
	s.alerts.Range(func(_, v interface{}) bool {
		alerts = append(alerts, v.(*svcproto.Alert))
		return true
	})
	return alerts, nil
}

// EvaluateAlerts 评估告警
func (s *Service) EvaluateAlerts(ctx context.Context, req *svcproto.EvaluateAlertsRequest) (*svcproto.EvaluateAlertsResponse, error) {
	var fired []*svcproto.Alert
	s.rules.Range(func(_, v interface{}) bool {
		rule := v.(*svcproto.AlertRule)
		if !rule.Enabled {
			return true
		}
		// TODO: 评估规则
		_ = req.Metrics
		_ = rule
		return true
	})
	return &svcproto.EvaluateAlertsResponse{Alerts: fired}, nil
}

// evalLoop 评估循环
func (s *Service) evalLoop() {
	ticker := time.NewTicker(s.config.EvalInterval)
	defer ticker.Stop()
	for range ticker.C {
		s.EvaluateAlerts(context.Background(), &svcproto.EvaluateAlertsRequest{})
	}
}

func (s *Service) healthzHandler(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusOK)
	fmt.Fprintf(w, `{"status":"healthy","service":"%s"}`, s.config.ServiceName)
}
func (s *Service) listAlertsHTTPHandler(w http.ResponseWriter, r *http.Request) { w.WriteHeader(http.StatusOK); fmt.Fprint(w, `{"alerts":[]}`) }
func (s *Service) listRulesHTTPHandler(w http.ResponseWriter, r *http.Request)   { w.WriteHeader(http.StatusOK); fmt.Fprint(w, `{"rules":[]}`) }
func (s *Service) webhookHandler(w http.ResponseWriter, r *http.Request)         { w.WriteHeader(http.StatusNoContent) }
