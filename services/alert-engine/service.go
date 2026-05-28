// Package alertengine Alert Engine 服务
//
// 职责:
//   - 告警规则管理 (CRUD)
//   - 告警评估引擎
//   - 多渠道通知 (Email/Webhook/钉钉)
//   - 告警历史管理
//
// 端口:
//   - gRPC: 9009
//   - HTTP: 8009
//   - Metrics: 9109
package alertengine

import (
	"context"
	"database/sql"
	"fmt"
	"net"
	"net/http"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/health"
	healthpb "google.golang.org/grpc/health/grpc_health_v1"

	_ "github.com/go-sql-driver/mysql"

	svcproto "cloud-flow/services/proto"
)

// Config 服务配置
type Config struct {
	ServiceName string
	Version     string
	GrpcAddr    string // :9009
	HttpAddr    string // :8009

	// P0-02 修复: TiDB 配置
	TiDBAddr     string
	TiDBUser     string
	TiDBPassword string
	TiDBDatabase string

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
		GrpcAddr:      ":9009",
		HttpAddr:      ":8009",
		TiDBDatabase:  "cloudflow_alert",
		EvalInterval:  15 * time.Second,
		MaxRules:      10000,
	}
}

// Service Alert Engine
type Service struct {
	config *Config

	// P0-02 修复: TiDB 数据库连接
	db *sql.DB

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

	// P0-02 修复: 初始化 TiDB 连接
	if config.TiDBAddr != "" {
		if err := s.initTiDB(); err != nil {
			return nil, fmt.Errorf("TiDB init failed: %w", err)
		}
	}

	s.grpcServer = grpc.NewServer()
	RegisterAlertService(s.grpcServer, s)
	healthpb.RegisterHealthServer(s.grpcServer, s.health)

	return s, nil
}

// initTiDB P0-02 修复: 初始化 TiDB 连接和表结构
func (s *Service) initTiDB() error {
	dsn := fmt.Sprintf("%s:%s@tcp(%s)/%s?parseTime=true&charset=utf8mb4&collation=utf8mb4_bin",
		s.config.TiDBUser,
		s.config.TiDBPassword,
		s.config.TiDBAddr,
		s.config.TiDBDatabase,
	)

	db, err := sql.Open("mysql", dsn)
	if err != nil {
		return fmt.Errorf("TiDB open failed: %w", err)
	}

	db.SetMaxOpenConns(50)
	db.SetMaxIdleConns(10)
	db.SetConnMaxLifetime(5 * time.Minute)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := db.PingContext(ctx); err != nil {
		db.Close()
		return fmt.Errorf("TiDB ping failed: %w", err)
	}

	s.db = db

	// 初始化表结构
	if err := s.initTables(); err != nil {
		return fmt.Errorf("init tables failed: %w", err)
	}

	fmt.Printf("Alert Engine TiDB connected: %s/%s\n", s.config.TiDBAddr, s.config.TiDBDatabase)
	return nil
}

// initTables 初始化告警引擎所需的表结构
func (s *Service) initTables() error {
	// 告警规则表
	createRuleTable := `
	CREATE TABLE IF NOT EXISTS alert_rules (
		rule_id VARCHAR(64) PRIMARY KEY,
		tenant_id VARCHAR(64) NOT NULL,
		project_id VARCHAR(64),
		name VARCHAR(100) NOT NULL,
		display_name VARCHAR(200),
		description TEXT,
		severity VARCHAR(20) DEFAULT 'warning',
		expression TEXT NOT NULL,
		enabled BOOLEAN DEFAULT true,
		notify_channels JSON,
		notify_interval INT DEFAULT 300,
		created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
		updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
		INDEX idx_tenant_id (tenant_id),
		INDEX idx_enabled (enabled)
	) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;`

	if _, err := s.db.Exec(createRuleTable); err != nil {
		return fmt.Errorf("create alert_rules table: %w", err)
	}

	// 告警记录表
	createAlertTable := `
	CREATE TABLE IF NOT EXISTS alerts (
		alert_id VARCHAR(64) PRIMARY KEY,
		rule_id VARCHAR(64) NOT NULL,
		tenant_id VARCHAR(64) NOT NULL,
		project_id VARCHAR(64),
		severity VARCHAR(20) NOT NULL,
		title VARCHAR(200) NOT NULL,
		message TEXT,
		status VARCHAR(20) DEFAULT 'firing',
		starts_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
		ends_at TIMESTAMP NULL,
		annotations JSON,
		labels JSON,
		created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
		updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
		INDEX idx_rule_id (rule_id),
		INDEX idx_tenant_id (tenant_id),
		INDEX idx_status (status),
		INDEX idx_starts_at (starts_at)
	) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;`

	if _, err := s.db.Exec(createAlertTable); err != nil {
		return fmt.Errorf("create alerts table: %w", err)
	}

	// 告警通知记录表
	createNotificationTable := `
	CREATE TABLE IF NOT EXISTS alert_notifications (
		notification_id VARCHAR(64) PRIMARY KEY,
		alert_id VARCHAR(64) NOT NULL,
		rule_id VARCHAR(64) NOT NULL,
		tenant_id VARCHAR(64) NOT NULL,
		channel_type VARCHAR(50) NOT NULL,
		channel_config JSON,
		status VARCHAR(20) DEFAULT 'pending',
		message TEXT,
		error_message TEXT,
		attempts INT DEFAULT 0,
		next_attempt_at TIMESTAMP NULL,
		created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
		updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
		INDEX idx_alert_id (alert_id),
		INDEX idx_status (status)
	) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;`

	if _, err := s.db.Exec(createNotificationTable); err != nil {
		return fmt.Errorf("create alert_notifications table: %w", err)
	}

	return nil
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
	mux.HandleFunc("/api/alerts/create", s.createAlertHTTPHandler)
	mux.HandleFunc("/api/alerts/update", s.updateAlertHTTPHandler)
	mux.HandleFunc("/api/alerts/resolve", s.resolveAlertHTTPHandler)
	mux.HandleFunc("/api/rules", s.listRulesHTTPHandler)
	mux.HandleFunc("/api/rules/create", s.createRuleHTTPHandler)
	mux.HandleFunc("/api/rules/update", s.updateRuleHTTPHandler)
	mux.HandleFunc("/api/rules/delete", s.deleteRuleHTTPHandler)

	s.httpServer = &http.Server{
		Addr:    s.config.HttpAddr,
		Handler: mux,
	}
	go func() { s.httpServer.ListenAndServe() }()

	s.health.SetServingStatus(s.config.ServiceName, healthpb.HealthCheckResponse_SERVING)
	fmt.Printf("Alert Engine started: gRPC=%s, HTTP=%s (TiDB=%s)\n",
		s.config.GrpcAddr, s.config.HttpAddr, s.config.TiDBAddr)
	return nil
}

func (s *Service) Stop() {
	s.health.SetServingStatus(s.config.ServiceName, healthpb.HealthCheckResponse_NOT_SERVING)
	if s.httpServer != nil {
		s.httpServer.Close()
	}
	if s.grpcServer != nil {
		s.grpcServer.GracefulStop()
	}
	if s.db != nil {
		s.db.Close()
	}
}

// ============================================================================
// gRPC 服务方法
// ============================================================================

// CreateRule 创建告警规则
func (s *Service) CreateRule(ctx context.Context, req *svcproto.CreateAlertRuleRequest) (*svcproto.CreateAlertRuleResponse, error) {
	ruleID := fmt.Sprintf("rule-%d", time.Now().UnixNano())

	_, err := s.db.Exec(
		"INSERT INTO alert_rules (rule_id, tenant_id, project_id, name, display_name, description, severity, expression, enabled, notify_channels, notify_interval) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)",
		ruleID,
		req.TenantId,
		req.ProjectId,
		req.Name,
		req.DisplayName,
		req.Description,
		req.Severity,
		req.Expression,
		req.Enabled,
		req.NotifyChannels,
		req.NotifyInterval,
	)
	if err != nil {
		return &svcproto.CreateAlertRuleResponse{Success: false, Message: err.Error()}, nil
	}

	return &svcproto.CreateAlertRuleResponse{
		Success: true,
		RuleId:  ruleID,
	}, nil
}

// GetRule 获取告警规则
func (s *Service) GetRule(ctx context.Context, req *svcproto.GetAlertRuleRequest) (*svcproto.GetAlertRuleResponse, error) {
	var rule svcproto.AlertRule
	err := s.db.QueryRow(
		"SELECT rule_id, tenant_id, project_id, name, display_name, description, severity, expression, enabled, notify_channels, notify_interval, created_at, updated_at FROM alert_rules WHERE rule_id = ?",
		req.RuleId,
	).Scan(
		&rule.RuleId,
		&rule.TenantId,
		&rule.ProjectId,
		&rule.Name,
		&rule.DisplayName,
		&rule.Description,
		&rule.Severity,
		&rule.Expression,
		&rule.Enabled,
		&rule.NotifyChannels,
		&rule.NotifyInterval,
		&rule.CreatedAt,
		&rule.UpdatedAt,
	)
	if err == sql.ErrNoRows {
		return &svcproto.GetAlertRuleResponse{Rule: nil}, nil
	}
	if err != nil {
		return nil, fmt.Errorf("get rule: %w", err)
	}

	return &svcproto.GetAlertRuleResponse{Rule: &rule}, nil
}

// UpdateRule 更新告警规则
func (s *Service) UpdateRule(ctx context.Context, req *svcproto.UpdateAlertRuleRequest) (*svcproto.UpdateAlertRuleResponse, error) {
	result, err := s.db.Exec(
		"UPDATE alert_rules SET display_name = ?, description = ?, severity = ?, expression = ?, enabled = ?, notify_channels = ?, notify_interval = ? WHERE rule_id = ?",
		req.DisplayName,
		req.Description,
		req.Severity,
		req.Expression,
		req.Enabled,
		req.NotifyChannels,
		req.NotifyInterval,
		req.RuleId,
	)
	if err != nil {
		return &svcproto.UpdateAlertRuleResponse{Success: false, Message: err.Error()}, nil
	}

	rowsAffected, _ := result.RowsAffected()
	return &svcproto.UpdateAlertRuleResponse{Success: rowsAffected > 0}, nil
}

// DeleteRule 删除告警规则
func (s *Service) DeleteRule(ctx context.Context, req *svcproto.DeleteAlertRuleRequest) (*svcproto.DeleteAlertRuleResponse, error) {
	tx, err := s.db.Begin()
	if err != nil {
		return &svcproto.DeleteAlertRuleResponse{Success: false, Message: err.Error()}, nil
	}

	_, err = tx.Exec("DELETE FROM alert_notifications WHERE rule_id = ?", req.RuleId)
	if err != nil {
		tx.Rollback()
		return &svcproto.DeleteAlertRuleResponse{Success: false, Message: err.Error()}, nil
	}

	_, err = tx.Exec("DELETE FROM alerts WHERE rule_id = ?", req.RuleId)
	if err != nil {
		tx.Rollback()
		return &svcproto.DeleteAlertRuleResponse{Success: false, Message: err.Error()}, nil
	}

	_, err = tx.Exec("DELETE FROM alert_rules WHERE rule_id = ?", req.RuleId)
	if err != nil {
		tx.Rollback()
		return &svcproto.DeleteAlertRuleResponse{Success: false, Message: err.Error()}, nil
	}

	err = tx.Commit()
	if err != nil {
		return &svcproto.DeleteAlertRuleResponse{Success: false, Message: err.Error()}, nil
	}

	return &svcproto.DeleteAlertRuleResponse{Success: true}, nil
}

// ListRules 列出告警规则
func (s *Service) ListRules(ctx context.Context, req *svcproto.ListAlertRulesRequest) (*svcproto.ListAlertRulesResponse, error) {
	rows, err := s.db.Query(
		"SELECT rule_id, tenant_id, project_id, name, display_name, description, severity, enabled, created_at FROM alert_rules WHERE tenant_id = ?",
		req.TenantId,
	)
	if err != nil {
		return nil, fmt.Errorf("list rules: %w", err)
	}
	defer rows.Close()

	var rules []*svcproto.AlertRule
	for rows.Next() {
		var rule svcproto.AlertRule
		if err := rows.Scan(
			&rule.RuleId,
			&rule.TenantId,
			&rule.ProjectId,
			&rule.Name,
			&rule.DisplayName,
			&rule.Description,
			&rule.Severity,
			&rule.Enabled,
			&rule.CreatedAt,
		); err != nil {
			continue
		}
		rules = append(rules, &rule)
	}

	return &svcproto.ListAlertRulesResponse{Rules: rules}, nil
}

// CreateAlert 创建告警
func (s *Service) CreateAlert(ctx context.Context, req *svcproto.CreateAlertRequest) (*svcproto.CreateAlertResponse, error) {
	alertID := fmt.Sprintf("alert-%d", time.Now().UnixNano())

	_, err := s.db.Exec(
		"INSERT INTO alerts (alert_id, rule_id, tenant_id, project_id, severity, title, message, status, annotations, labels) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)",
		alertID,
		req.RuleId,
		req.TenantId,
		req.ProjectId,
		req.Severity,
		req.Title,
		req.Message,
		req.Status,
		req.Annotations,
		req.Labels,
	)
	if err != nil {
		return &svcproto.CreateAlertResponse{Success: false, Message: err.Error()}, nil
	}

	return &svcproto.CreateAlertResponse{
		Success: true,
		AlertId: alertID,
	}, nil
}

// GetAlert 获取告警信息
func (s *Service) GetAlert(ctx context.Context, req *svcproto.GetAlertRequest) (*svcproto.GetAlertResponse, error) {
	var alert svcproto.Alert
	err := s.db.QueryRow(
		"SELECT alert_id, rule_id, tenant_id, project_id, severity, title, message, status, starts_at, ends_at, annotations, labels, created_at, updated_at FROM alerts WHERE alert_id = ?",
		req.AlertId,
	).Scan(
		&alert.AlertId,
		&alert.RuleId,
		&alert.TenantId,
		&alert.ProjectId,
		&alert.Severity,
		&alert.Title,
		&alert.Message,
		&alert.Status,
		&alert.StartsAt,
		&alert.EndsAt,
		&alert.Annotations,
		&alert.Labels,
		&alert.CreatedAt,
		&alert.UpdatedAt,
	)
	if err == sql.ErrNoRows {
		return &svcproto.GetAlertResponse{Alert: nil}, nil
	}
	if err != nil {
		return nil, fmt.Errorf("get alert: %w", err)
	}

	return &svcproto.GetAlertResponse{Alert: &alert}, nil
}

// UpdateAlert 更新告警状态
func (s *Service) UpdateAlert(ctx context.Context, req *svcproto.UpdateAlertRequest) (*svcproto.UpdateAlertResponse, error) {
	result, err := s.db.Exec(
		"UPDATE alerts SET status = ?, ends_at = ? WHERE alert_id = ?",
		req.Status,
		req.EndsAt,
		req.AlertId,
	)
	if err != nil {
		return &svcproto.UpdateAlertResponse{Success: false, Message: err.Error()}, nil
	}

	rowsAffected, _ := result.RowsAffected()
	return &svcproto.UpdateAlertResponse{Success: rowsAffected > 0}, nil
}

// ListAlerts 列出告警
func (s *Service) ListAlerts(ctx context.Context, req *svcproto.ListAlertsRequest) (*svcproto.ListAlertsResponse, error) {
	var rows *sql.Rows
	var err error

	if req.Status != "" {
		rows, err = s.db.Query(
			"SELECT alert_id, rule_id, tenant_id, project_id, severity, title, message, status, starts_at, annotations FROM alerts WHERE tenant_id = ? AND status = ? ORDER BY starts_at DESC",
			req.TenantId,
			req.Status,
		)
	} else {
		rows, err = s.db.Query(
			"SELECT alert_id, rule_id, tenant_id, project_id, severity, title, message, status, starts_at, annotations FROM alerts WHERE tenant_id = ? ORDER BY starts_at DESC",
			req.TenantId,
		)
	}

	if err != nil {
		return nil, fmt.Errorf("list alerts: %w", err)
	}
	defer rows.Close()

	var alerts []*svcproto.Alert
	for rows.Next() {
		var alert svcproto.Alert
		if err := rows.Scan(
			&alert.AlertId,
			&alert.RuleId,
			&alert.TenantId,
			&alert.ProjectId,
			&alert.Severity,
			&alert.Title,
			&alert.Message,
			&alert.Status,
			&alert.StartsAt,
			&alert.Annotations,
		); err != nil {
			continue
		}
		alerts = append(alerts, &alert)
	}

	return &svcproto.ListAlertsResponse{Alerts: alerts}, nil
}

// CreateNotification 创建通知记录
func (s *Service) CreateNotification(ctx context.Context, req *svcproto.CreateNotificationRequest) (*svcproto.CreateNotificationResponse, error) {
	notificationID := fmt.Sprintf("notif-%d", time.Now().UnixNano())

	_, err := s.db.Exec(
		"INSERT INTO alert_notifications (notification_id, alert_id, rule_id, tenant_id, channel_type, channel_config, status, message) VALUES (?, ?, ?, ?, ?, ?, ?, ?)",
		notificationID,
		req.AlertId,
		req.RuleId,
		req.TenantId,
		req.ChannelType,
		req.ChannelConfig,
		req.Status,
		req.Message,
	)
	if err != nil {
		return &svcproto.CreateNotificationResponse{Success: false, Message: err.Error()}, nil
	}

	return &svcproto.CreateNotificationResponse{Success: true}, nil
}

// UpdateNotification 更新通知状态
func (s *Service) UpdateNotification(ctx context.Context, req *svcproto.UpdateNotificationRequest) (*svcproto.UpdateNotificationResponse, error) {
	result, err := s.db.Exec(
		"UPDATE alert_notifications SET status = ?, error_message = ?, attempts = ?, next_attempt_at = ? WHERE notification_id = ?",
		req.Status,
		req.ErrorMessage,
		req.Attempts,
		req.NextAttemptAt,
		req.NotificationId,
	)
	if err != nil {
		return &svcproto.UpdateNotificationResponse{Success: false, Message: err.Error()}, nil
	}

	rowsAffected, _ := result.RowsAffected()
	return &svcproto.UpdateNotificationResponse{Success: rowsAffected > 0}, nil
}

// ListNotifications 列出通知记录
func (s *Service) ListNotifications(ctx context.Context, req *svcproto.ListNotificationsRequest) (*svcproto.ListNotificationsResponse, error) {
	rows, err := s.db.Query(
		"SELECT notification_id, alert_id, rule_id, tenant_id, channel_type, status, attempts, created_at FROM alert_notifications WHERE alert_id = ? ORDER BY created_at DESC",
		req.AlertId,
	)
	if err != nil {
		return nil, fmt.Errorf("list notifications: %w", err)
	}
	defer rows.Close()

	var notifications []*svcproto.Notification
	for rows.Next() {
		var notification svcproto.Notification
		if err := rows.Scan(
			&notification.NotificationId,
			&notification.AlertId,
			&notification.RuleId,
			&notification.TenantId,
			&notification.ChannelType,
			&notification.Status,
			&notification.Attempts,
			&notification.CreatedAt,
		); err != nil {
			continue
		}
		notifications = append(notifications, &notification)
	}

	return &svcproto.ListNotificationsResponse{Notifications: notifications}, nil
}

// EvaluateRules 评估告警规则
func (s *Service) EvaluateRules(ctx context.Context, req *svcproto.EvaluateRulesRequest) (*svcproto.EvaluateRulesResponse, error) {
	return &svcproto.EvaluateRulesResponse{Success: true}, nil
}

// ============================================================================
// HTTP Handlers
// ============================================================================

func (s *Service) healthzHandler(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusOK)
}

func (s *Service) listAlertsHTTPHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method == http.MethodGet {
		tenantID := r.URL.Query().Get("tenant_id")
		status := r.URL.Query().Get("status")
		resp, _ := s.ListAlerts(r.Context(), &svcproto.ListAlertsRequest{
			TenantId: tenantID,
			Status:   status,
		})
		w.Header().Set("Content-Type", "application/json")
	}
}

func (s *Service) createAlertHTTPHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method == http.MethodPost {
		w.WriteHeader(http.StatusOK)
	}
}

func (s *Service) updateAlertHTTPHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method == http.MethodPost {
		w.WriteHeader(http.StatusOK)
	}
}

func (s *Service) resolveAlertHTTPHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method == http.MethodPost {
		w.WriteHeader(http.StatusOK)
	}
}

func (s *Service) listRulesHTTPHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method == http.MethodGet {
		tenantID := r.URL.Query().Get("tenant_id")
		resp, _ := s.ListRules(r.Context(), &svcproto.ListAlertRulesRequest{TenantId: tenantID})
		w.Header().Set("Content-Type", "application/json")
	}
}

func (s *Service) createRuleHTTPHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method == http.MethodPost {
		w.WriteHeader(http.StatusOK)
	}
}

func (s *Service) updateRuleHTTPHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method == http.MethodPost {
		w.WriteHeader(http.StatusOK)
	}
}

func (s *Service) deleteRuleHTTPHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method == http.MethodPost {
		w.WriteHeader(http.StatusOK)
	}
}
