//go:build linux

// Package storage 提供无状态存储层，所有数据存储到TiDB/PostgreSQL
package storage

import (
	"context"
	"database/sql"
	"fmt"
	"sync"
	"time"

	"cloud-flow-center/internal/config"
	"cloud-flow-agent/pkg/logger"
	_ "github.com/lib/pq" // PostgreSQL驱动
)

// Store TiDB/PostgreSQL存储层
type Store struct {
	db  *sql.DB
	cfg *config.DatabaseConfig
	log *logger.Logger

	// 用于实例标识
	instanceID string
	mu         sync.RWMutex
}

// NewStore 创建存储层
func NewStore(cfg *config.DatabaseConfig, log *logger.Logger) (*Store, error) {
	// 生成实例ID
	instanceID := fmt.Sprintf("center-%d", time.Now().UnixNano())

	db, err := sql.Open("postgres", cfg.GetDSN())
	if err != nil {
		return nil, fmt.Errorf("打开数据库连接失败: %w", err)
	}

	// 配置连接池
	db.SetMaxOpenConns(cfg.MaxOpenConns)
	db.SetMaxIdleConns(cfg.MaxIdleConns)
	db.SetConnMaxLifetime(cfg.ConnMaxLife)

	// 测试连接
	ctx, cancel := context.WithTimeout(context.Background(), cfg.ConnTimeout)
	defer cancel()

	if err := db.PingContext(ctx); err != nil {
		db.Close()
		return nil, fmt.Errorf("数据库连接失败: %w", err)
	}

	store := &Store{
		db:         db,
		cfg:        cfg,
		log:        log,
		instanceID: instanceID,
	}

	// 初始化表结构
	if err := store.initSchema(ctx); err != nil {
		db.Close()
		return nil, fmt.Errorf("初始化表结构失败: %w", err)
	}

	log.Infof("存储层初始化成功: instance=%s, addr=%s", instanceID, cfg.Addr)
	return store, nil
}

// initSchema 初始化表结构
func (s *Store) initSchema(ctx context.Context) error {
	schema := `
	-- 租户表
	CREATE TABLE IF NOT EXISTS tenants (
		id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
		name VARCHAR(255) NOT NULL UNIQUE,
		display_name VARCHAR(255),
		status VARCHAR(50) DEFAULT 'active',
		quota JSONB DEFAULT '{}',
		created_at TIMESTAMP DEFAULT NOW(),
		updated_at TIMESTAMP DEFAULT NOW()
	);

	-- 配置表（支持多实例分布式锁）
	CREATE TABLE IF NOT EXISTS configs (
		key VARCHAR(255) PRIMARY KEY,
		value JSONB NOT NULL,
		version INT DEFAULT 1,
		updated_by VARCHAR(255),
		updated_at TIMESTAMP DEFAULT NOW()
	);

	-- 分布式锁表
	CREATE TABLE IF NOT EXISTS distributed_locks (
		lock_key VARCHAR(255) PRIMARY KEY,
		owner_id VARCHAR(255) NOT NULL,
		acquired_at TIMESTAMP NOT NULL,
		expires_at TIMESTAMP NOT NULL,
		INDEX idx_expires_at (expires_at)
	);

	-- 告警表
	CREATE TABLE IF NOT EXISTS alerts (
		id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
		tenant_id UUID REFERENCES tenants(id),
		rule_id VARCHAR(255) NOT NULL,
		rule_name VARCHAR(255) NOT NULL,
		level VARCHAR(50) NOT NULL,
		status VARCHAR(50) NOT NULL DEFAULT 'firing',
		fingerprint VARCHAR(255) NOT NULL,
		metric VARCHAR(255),
		value DOUBLE PRECISION,
		threshold DOUBLE PRECISION,
		labels JSONB DEFAULT '{}',
		annotations JSONB DEFAULT '{}',
		fired_at TIMESTAMP NOT NULL,
		resolved_at TIMESTAMP,
		duration BIGINT DEFAULT 0,
		instance_id VARCHAR(255),
		created_at TIMESTAMP DEFAULT NOW(),
		updated_at TIMESTAMP DEFAULT NOW(),
		INDEX idx_status (status),
		INDEX idx_fingerprint (fingerprint),
		INDEX idx_fired_at (fired_at),
		INDEX idx_tenant_status (tenant_id, status)
	);

	-- 指标数据表（分区表）
	CREATE TABLE IF NOT EXISTS metrics (
		id BIGSERIAL PRIMARY KEY,
		tenant_id UUID REFERENCES tenants(id),
		metric_name VARCHAR(255) NOT NULL,
		labels JSONB DEFAULT '{}',
		value DOUBLE PRECISION NOT NULL,
		timestamp TIMESTAMP NOT NULL,
		created_at TIMESTAMP DEFAULT NOW(),
		INDEX idx_metric_time (metric_name, timestamp),
		INDEX idx_tenant_metric (tenant_id, metric_name, timestamp)
	);

	-- 链路数据表
	CREATE TABLE IF NOT EXISTS traces (
		id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
		tenant_id UUID REFERENCES tenants(id),
		trace_id VARCHAR(255) NOT NULL,
		span_id VARCHAR(255) NOT NULL,
		parent_span_id VARCHAR(255),
		service_name VARCHAR(255) NOT NULL,
		operation_name VARCHAR(255) NOT NULL,
		start_time TIMESTAMP NOT NULL,
		duration BIGINT NOT NULL,
		status_code INT DEFAULT 0,
		tags JSONB DEFAULT '{}',
		instance_id VARCHAR(255),
		created_at TIMESTAMP DEFAULT NOW(),
		INDEX idx_trace_id (trace_id),
		INDEX idx_service_time (service_name, start_time)
	);

	-- 实例注册表（用于多实例协调）
	CREATE TABLE IF NOT EXISTS instances (
		instance_id VARCHAR(255) PRIMARY KEY,
		instance_type VARCHAR(50) NOT NULL,
		endpoint VARCHAR(255),
		status VARCHAR(50) DEFAULT 'active',
		capabilities JSONB DEFAULT '[]',
		registered_at TIMESTAMP DEFAULT NOW(),
		last_heartbeat TIMESTAMP DEFAULT NOW(),
		INDEX idx_status (status),
		INDEX idx_type_status (instance_type, status)
	);

	-- 会话状态表（用于无状态会话管理）
	CREATE TABLE IF NOT EXISTS sessions (
		session_id VARCHAR(255) PRIMARY KEY,
		tenant_id UUID REFERENCES tenants(id),
		user_id VARCHAR(255) NOT NULL,
		data JSONB DEFAULT '{}',
		expires_at TIMESTAMP NOT NULL,
		created_at TIMESTAMP DEFAULT NOW(),
		INDEX idx_expires_at (expires_at),
		INDEX idx_tenant_user (tenant_id, user_id)
	);
	`

	_, err := s.db.ExecContext(ctx, schema)
	return err
}

// Close 关闭存储层
func (s *Store) Close() error {
	return s.db.Close()
}

// Ping 检查连接
func (s *Store) Ping(ctx context.Context) error {
	return s.db.PingContext(ctx)
}

// GetDB 获取底层数据库连接
func (s *Store) GetDB() *sql.DB {
	return s.db
}

// GetInstanceID 获取实例ID
func (s *Store) GetInstanceID() string {
	return s.instanceID
}

// ==================== 通用操作 ====================

// Query 执行查询
func (s *Store) Query(ctx context.Context, query string, args ...interface{}) (*sql.Rows, error) {
	return s.db.QueryContext(ctx, query, args...)
}

// QueryRow 执行单行查询
func (s *Store) QueryRow(ctx context.Context, query string, args ...interface{}) *sql.Row {
	return s.db.QueryRowContext(ctx, query, args...)
}

// Exec 执行更新
func (s *Store) Exec(ctx context.Context, query string, args ...interface{}) (sql.Result, error) {
	return s.db.ExecContext(ctx, query, args...)
}

// ==================== 租户操作 ====================

// CreateTenant 创建租户
func (s *Store) CreateTenant(ctx context.Context, name, displayName string) (string, error) {
	var id string
	err := s.db.QueryRowContext(ctx,
		`INSERT INTO tenants (name, display_name) VALUES ($1, $2) RETURNING id`,
		name, displayName,
	).Scan(&id)
	return id, err
}

// GetTenant 获取租户
func (s *Store) GetTenant(ctx context.Context, name string) (*Tenant, error) {
	row := s.db.QueryRowContext(ctx,
		`SELECT id, name, display_name, status, quota, created_at, updated_at 
		FROM tenants WHERE name = $1`, name)

	var t Tenant
	err := row.Scan(&t.ID, &t.Name, &t.DisplayName, &t.Status, &t.Quota, &t.CreatedAt, &t.UpdatedAt)
	if err != nil {
		return nil, err
	}
	return &t, nil
}

// ListTenants 列出所有租户
func (s *Store) ListTenants(ctx context.Context) ([]*Tenant, error) {
	rows, err := s.db.QueryContext(ctx,
		`SELECT id, name, display_name, status, quota, created_at, updated_at FROM tenants`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var tenants []*Tenant
	for rows.Next() {
		var t Tenant
		if err := rows.Scan(&t.ID, &t.Name, &t.DisplayName, &t.Status, &t.Quota, &t.CreatedAt, &t.UpdatedAt); err != nil {
			return nil, err
		}
		tenants = append(tenants, &t)
	}
	return tenants, rows.Err()
}

// Tenant 租户结构
type Tenant struct {
	ID          string
	Name        string
	DisplayName string
	Status      string
	Quota       string
	CreatedAt   time.Time
	UpdatedAt   time.Time
}

// ==================== 配置操作 ====================

// GetConfig 获取配置
func (s *Store) GetConfig(ctx context.Context, key string) (string, int, error) {
	var value string
	var version int
	err := s.db.QueryRowContext(ctx,
		`SELECT value, version FROM configs WHERE key = $1`, key,
	).Scan(&value, &version)
	return value, version, err
}

// SetConfig 设置配置
func (s *Store) SetConfig(ctx context.Context, key, value string) error {
	_, err := s.db.ExecContext(ctx,
		`INSERT INTO configs (key, value, version, updated_by, updated_at) 
		VALUES ($1, $2, 1, $3, NOW())
		ON CONFLICT (key) DO UPDATE SET 
			value = EXCLUDED.value,
			version = configs.version + 1,
			updated_by = EXCLUDED.updated_by,
			updated_at = NOW()`,
		key, value, s.instanceID)
	return err
}

// ==================== 分布式锁 ====================

// AcquireLock 获取分布式锁
func (s *Store) AcquireLock(ctx context.Context, lockKey string, ttl time.Duration) (bool, error) {
	now := time.Now()
	expiresAt := now.Add(ttl)

	result, err := s.db.ExecContext(ctx,
		`INSERT INTO distributed_locks (lock_key, owner_id, acquired_at, expires_at)
		VALUES ($1, $2, $3, $4)
		ON CONFLICT (lock_key) DO UPDATE SET
			owner_id = EXCLUDED.owner_id,
			acquired_at = EXCLUDED.acquired_at,
			expires_at = EXCLUDED.expires_at
		WHERE distributed_locks.expires_at < $3
			OR distributed_locks.owner_id = $2`,
		lockKey, s.instanceID, now, expiresAt,
	)
	if err != nil {
		return false, err
	}

	affected, err := result.RowsAffected()
	return affected > 0, err
}

// ReleaseLock 释放分布式锁
func (s *Store) ReleaseLock(ctx context.Context, lockKey string) error {
	_, err := s.db.ExecContext(ctx,
		`DELETE FROM distributed_locks WHERE lock_key = $1 AND owner_id = $2`,
		lockKey, s.instanceID)
	return err
}

// ==================== 告警操作 ====================

// CreateAlert 创建告警
func (s *Store) CreateAlert(ctx context.Context, alert *Alert) error {
	_, err := s.db.ExecContext(ctx,
		`INSERT INTO alerts (id, tenant_id, rule_id, rule_name, level, status, fingerprint,
		metric, value, threshold, labels, annotations, fired_at, instance_id)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14)`,
		alert.ID, alert.TenantID, alert.RuleID, alert.RuleName, alert.Level, alert.Status,
		alert.Fingerprint, alert.Metric, alert.Value, alert.Threshold, alert.Labels,
		alert.Annotations, alert.FiredAt, s.instanceID)
	return err
}

// UpdateAlert 更新告警
func (s *Store) UpdateAlert(ctx context.Context, alert *Alert) error {
	_, err := s.db.ExecContext(ctx,
		`UPDATE alerts SET 
			status = $2, resolved_at = $3, duration = $4, updated_at = NOW()
		WHERE id = $1`,
		alert.ID, alert.Status, alert.ResolvedAt, alert.Duration)
	return err
}

// GetActiveAlerts 获取活跃告警
func (s *Store) GetActiveAlerts(ctx context.Context, tenantID string) ([]*Alert, error) {
	query := `SELECT id, tenant_id, rule_id, rule_name, level, status, fingerprint,
		metric, value, threshold, labels, annotations, fired_at, resolved_at, duration
		FROM alerts WHERE status = 'firing'`
	args := []interface{}{}

	if tenantID != "" {
		query += " AND tenant_id = $1"
		args = append(args, tenantID)
	}

	rows, err := s.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var alerts []*Alert
	for rows.Next() {
		var a Alert
		if err := rows.Scan(&a.ID, &a.TenantID, &a.RuleID, &a.RuleName, &a.Level, &a.Status,
			&a.Fingerprint, &a.Metric, &a.Value, &a.Threshold, &a.Labels, &a.Annotations,
			&a.FiredAt, &a.ResolvedAt, &a.Duration); err != nil {
			return nil, err
		}
		alerts = append(alerts, &a)
	}
	return alerts, rows.Err()
}

// Alert 告警结构
type Alert struct {
	ID           string
	TenantID     string
	RuleID       string
	RuleName     string
	Level        string
	Status       string
	Fingerprint  string
	Metric       string
	Value        float64
	Threshold    float64
	Labels       string
	Annotations   string
	FiredAt      time.Time
	ResolvedAt   *time.Time
	Duration     int64
}

// ==================== 实例操作 ====================

// RegisterInstance 注册实例
func (s *Store) RegisterInstance(ctx context.Context, instanceType, endpoint string) error {
	_, err := s.db.ExecContext(ctx,
		`INSERT INTO instances (instance_id, instance_type, endpoint, status, registered_at, last_heartbeat)
		VALUES ($1, $2, $3, 'active', NOW(), NOW())
		ON CONFLICT (instance_id) DO UPDATE SET
			endpoint = EXCLUDED.endpoint,
			last_heartbeat = NOW()`,
		s.instanceID, instanceType, endpoint)
	return err
}

// Heartbeat 实例心跳
func (s *Store) Heartbeat(ctx context.Context) error {
	_, err := s.db.ExecContext(ctx,
		`UPDATE instances SET last_heartbeat = NOW() WHERE instance_id = $1`,
		s.instanceID)
	return err
}

// GetActiveInstances 获取活跃实例
func (s *Store) GetActiveInstances(ctx context.Context, instanceType string) ([]*Instance, error) {
	rows, err := s.db.QueryContext(ctx,
		`SELECT instance_id, instance_type, endpoint, status, capabilities, last_heartbeat
		FROM instances 
		WHERE instance_type = $1 AND status = 'active' 
			AND last_heartbeat > NOW() - INTERVAL '5 minutes'`,
		instanceType)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var instances []*Instance
	for rows.Next() {
		var i Instance
		if err := rows.Scan(&i.InstanceID, &i.InstanceType, &i.Endpoint, &i.Status, &i.Capabilities, &i.LastHeartbeat); err != nil {
			return nil, err
		}
		instances = append(instances, &i)
	}
	return instances, rows.Err()
}

// Instance 实例结构
type Instance struct {
	InstanceID    string
	InstanceType  string
	Endpoint      string
	Status        string
	Capabilities  string
	LastHeartbeat time.Time
}
