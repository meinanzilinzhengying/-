// Package asset 指标存储模块
// 支持时序数据存储与查询
package asset

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"sync"
	"time"

	_ "github.com/mattn/go-sqlite3"
)

// AssetStorageConfig 资产指标存储配置
type AssetStorageConfig struct {
	Enabled           bool          `json:"enabled" yaml:"enabled"`
	Driver            string        `json:"driver" yaml:"driver"`                     // sqlite/mysql/postgres
	DSN               string        `json:"dsn" yaml:"dsn"`                           // 数据源
	MaxConnections    int           `json:"max_connections" yaml:"max_connections"`
	RetentionPeriod   time.Duration `json:"retention_period" yaml:"retention_period"` // 数据保留期
	CleanupInterval   time.Duration `json:"cleanup_interval" yaml:"cleanup_interval"` // 清理间隔
	EnableCompression bool          `json:"enable_compression" yaml:"enable_compression"` // 启用压缩
	BatchSize         int           `json:"batch_size" yaml:"batch_size"`             // 批量写入大小
	WriteInterval     time.Duration `json:"write_interval" yaml:"write_interval"`     // 写入间隔
}

// AssetStorage 资产指标存储
type AssetStorage struct {
	config       *AssetStorageConfig
	db           *sql.DB
	writeBuffer  map[string][]*AssetMetrics
	mu           sync.RWMutex
	ctx          context.Context
	cancel       context.CancelFunc
	wg           sync.WaitGroup
}

// NewAssetStorage 创建资产指标存储
func NewAssetStorage(config *AssetStorageConfig) (*AssetStorage, error) {
	ctx, cancel := context.WithCancel(context.Background())
	
	storage := &AssetStorage{
		config:      config,
		writeBuffer: make(map[string][]*AssetMetrics),
		ctx:         ctx,
		cancel:      cancel,
	}
	
	if config.Enabled {
		if err := storage.initDB(); err != nil {
			return nil, fmt.Errorf("failed to init database: %w", err)
		}
		
		// 启动写入协程
		storage.wg.Add(1)
		go storage.writeLoop()
		
		// 启动清理协程
		storage.wg.Add(1)
		go storage.cleanupLoop()
	}
	
	return storage, nil
}

// initDB 初始化数据库
func (s *AssetStorage) initDB() error {
	var err error
	
	s.db, err = sql.Open(s.config.Driver, s.config.DSN)
	if err != nil {
		return err
	}
	
	s.db.SetMaxOpenConns(s.config.MaxConnections)
	s.db.SetMaxIdleConns(s.config.MaxConnections / 2)
	s.db.SetConnMaxLifetime(time.Hour)
	
	// 创建表
	if err := s.createTables(); err != nil {
		return err
	}
	
	// 创建索引
	if err := s.createIndexes(); err != nil {
		return err
	}
	
	return nil
}

// createTables 创建表
func (s *AssetStorage) createTables() error {
	// 资产指标表
	_, err := s.db.Exec(`
		CREATE TABLE IF NOT EXISTS asset_metrics (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			asset_id TEXT NOT NULL,
			asset_type TEXT NOT NULL,
			asset_name TEXT,
			timestamp TIMESTAMP NOT NULL,
			network TEXT,
			application TEXT,
			system TEXT,
			labels TEXT
		)
	`)
	if err != nil {
		return err
	}
	
	// 聚合指标表
	_, err = s.db.Exec(`
		CREATE TABLE IF NOT EXISTS asset_metrics_aggregated (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			asset_id TEXT NOT NULL,
			asset_type TEXT NOT NULL,
			granularity TEXT NOT NULL,
			window_start TIMESTAMP NOT NULL,
			window_end TIMESTAMP NOT NULL,
			sample_count INTEGER,
			network TEXT,
			application TEXT,
			system TEXT
		)
	`)
	if err != nil {
		return err
	}
	
	// 资产元数据表
	_, err = s.db.Exec(`
		CREATE TABLE IF NOT EXISTS asset_metadata (
			asset_id TEXT PRIMARY KEY,
			asset_type TEXT NOT NULL,
			asset_name TEXT,
			ip_addresses TEXT,
			labels TEXT,
			first_seen TIMESTAMP,
			last_seen TIMESTAMP
		)
	`)
	
	return err
}

// createIndexes 创建索引
func (s *AssetStorage) createIndexes() error {
	indexes := []string{
		"CREATE INDEX IF NOT EXISTS idx_metrics_asset_id ON asset_metrics(asset_id)",
		"CREATE INDEX IF NOT EXISTS idx_metrics_timestamp ON asset_metrics(timestamp)",
		"CREATE INDEX IF NOT EXISTS idx_metrics_asset_time ON asset_metrics(asset_id, timestamp)",
		"CREATE INDEX IF NOT EXISTS idx_agg_asset_id ON asset_metrics_aggregated(asset_id)",
		"CREATE INDEX IF NOT EXISTS idx_agg_granularity ON asset_metrics_aggregated(granularity)",
		"CREATE INDEX IF NOT EXISTS idx_agg_window ON asset_metrics_aggregated(window_start, window_end)",
	}
	
	for _, idx := range indexes {
		if _, err := s.db.Exec(idx); err != nil {
			return err
		}
	}
	
	return nil
}

// Close 关闭存储
func (s *AssetStorage) Close() error {
	s.cancel()
	s.wg.Wait()
	
	// 刷新缓冲区
	s.flushBuffer()
	
	if s.db != nil {
		return s.db.Close()
	}
	return nil
}

// writeLoop 写入循环
func (s *AssetStorage) writeLoop() {
	defer s.wg.Done()
	
	ticker := time.NewTicker(s.config.WriteInterval)
	defer ticker.Stop()
	
	for {
		select {
		case <-s.ctx.Done():
			return
		case <-ticker.C:
			s.flushBuffer()
		}
	}
}

// cleanupLoop 清理循环
func (s *AssetStorage) cleanupLoop() {
	defer s.wg.Done()
	
	ticker := time.NewTicker(s.config.CleanupInterval)
	defer ticker.Stop()
	
	for {
		select {
		case <-s.ctx.Done():
			return
		case <-ticker.C:
			s.cleanup()
		}
	}
}

// SaveMetrics 保存指标（缓冲写入）
func (s *AssetStorage) SaveMetrics(metrics *AssetMetrics) error {
	if !s.config.Enabled {
		return nil
	}
	
	s.mu.Lock()
	defer s.mu.Unlock()
	
	buffer := s.writeBuffer[metrics.AssetID]
	buffer = append(buffer, metrics)
	
	// 如果缓冲区满了，立即写入
	if len(buffer) >= s.config.BatchSize {
		s.writeBuffer[metrics.AssetID] = nil
		s.mu.Unlock()
		s.batchWrite(metrics.AssetID, buffer)
		s.mu.Lock()
	} else {
		s.writeBuffer[metrics.AssetID] = buffer
	}
	
	return nil
}

// flushBuffer 刷新缓冲区
func (s *AssetStorage) flushBuffer() {
	s.mu.Lock()
	buffers := make(map[string][]*AssetMetrics)
	for id, buffer := range s.writeBuffer {
		if len(buffer) > 0 {
			buffers[id] = buffer
		}
	}
	s.writeBuffer = make(map[string][]*AssetMetrics)
	s.mu.Unlock()
	
	for id, buffer := range buffers {
		s.batchWrite(id, buffer)
	}
}

// batchWrite 批量写入
func (s *AssetStorage) batchWrite(assetID string, metrics []*AssetMetrics) error {
	if len(metrics) == 0 {
		return nil
	}
	
	tx, err := s.db.Begin()
	if err != nil {
		return err
	}
	
	stmt, err := tx.Prepare(`
		INSERT INTO asset_metrics 
		(asset_id, asset_type, asset_name, timestamp, network, application, system, labels)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?)
	`)
	if err != nil {
		tx.Rollback()
		return err
	}
	defer stmt.Close()
	
	for _, m := range metrics {
		network, _ := json.Marshal(m.Network)
		application, _ := json.Marshal(m.Application)
		system, _ := json.Marshal(m.System)
		labels, _ := json.Marshal(m.Labels)
		
		_, err := stmt.Exec(m.AssetID, m.AssetType, m.AssetName, m.Timestamp,
			string(network), string(application), string(system), string(labels))
		if err != nil {
			tx.Rollback()
			return err
		}
	}
	
	return tx.Commit()
}

// SaveAggregatedMetrics 保存聚合指标
func (s *AssetStorage) SaveAggregatedMetrics(metrics *AggregatedMetrics) error {
	if !s.config.Enabled {
		return nil
	}
	
	network, _ := json.Marshal(metrics.Network)
	application, _ := json.Marshal(metrics.Application)
	system, _ := json.Marshal(metrics.System)
	
	_, err := s.db.Exec(`
		INSERT INTO asset_metrics_aggregated 
		(asset_id, asset_type, granularity, window_start, window_end, sample_count, network, application, system)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)
	`, metrics.AssetID, metrics.AssetType, metrics.Granularity,
		metrics.WindowStart, metrics.WindowEnd, metrics.SampleCount,
		string(network), string(application), string(system))
	
	return err
}

// GetMetrics 查询指标
func (s *AssetStorage) GetMetrics(assetID string, start, end time.Time, limit int) ([]*AssetMetrics, error) {
	if !s.config.Enabled {
		return nil, fmt.Errorf("storage not enabled")
	}
	
	query := `
		SELECT asset_id, asset_type, asset_name, timestamp, network, application, system, labels
		FROM asset_metrics 
		WHERE asset_id = ? AND timestamp >= ? AND timestamp <= ?
		ORDER BY timestamp DESC
	`
	if limit > 0 {
		query += fmt.Sprintf(" LIMIT %d", limit)
	}
	
	rows, err := s.db.Query(query, assetID, start, end)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	
	return s.scanMetrics(rows)
}

// GetLatestMetrics 获取最新指标
func (s *AssetStorage) GetLatestMetrics(assetID string) (*AssetMetrics, error) {
	if !s.config.Enabled {
		return nil, fmt.Errorf("storage not enabled")
	}
	
	row := s.db.QueryRow(`
		SELECT asset_id, asset_type, asset_name, timestamp, network, application, system, labels
		FROM asset_metrics 
		WHERE asset_id = ?
		ORDER BY timestamp DESC
		LIMIT 1
	`, assetID)
	
	return s.scanMetric(row)
}

// GetMetricsByType 按类型查询指标
func (s *AssetStorage) GetMetricsByType(assetType AssetType, start, end time.Time, limit int) ([]*AssetMetrics, error) {
	if !s.config.Enabled {
		return nil, fmt.Errorf("storage not enabled")
	}
	
	rows, err := s.db.Query(`
		SELECT asset_id, asset_type, asset_name, timestamp, network, application, system, labels
		FROM asset_metrics 
		WHERE asset_type = ? AND timestamp >= ? AND timestamp <= ?
		ORDER BY timestamp DESC
		LIMIT ?
	`, assetType, start, end, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	
	return s.scanMetrics(rows)
}

// GetAggregatedMetrics 查询聚合指标
func (s *AssetStorage) GetAggregatedMetrics(assetID string, granularity TimeGranularity, start, end time.Time) ([]*AggregatedMetrics, error) {
	if !s.config.Enabled {
		return nil, fmt.Errorf("storage not enabled")
	}
	
	rows, err := s.db.Query(`
		SELECT asset_id, asset_type, granularity, window_start, window_end, sample_count, network, application, system
		FROM asset_metrics_aggregated 
		WHERE asset_id = ? AND granularity = ? AND window_start >= ? AND window_end <= ?
		ORDER BY window_start DESC
	`, assetID, granularity, start, end)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	
	return s.scanAggregatedMetrics(rows)
}

// GetAssetList 获取资产列表
func (s *AssetStorage) GetAssetList(assetType AssetType, limit int) ([]string, error) {
	if !s.config.Enabled {
		return nil, fmt.Errorf("storage not enabled")
	}
	
	var query string
	var args []interface{}
	
	if assetType != "" {
		query = `SELECT DISTINCT asset_id FROM asset_metrics WHERE asset_type = ? ORDER BY asset_id LIMIT ?`
		args = []interface{}{assetType, limit}
	} else {
		query = `SELECT DISTINCT asset_id FROM asset_metrics ORDER BY asset_id LIMIT ?`
		args = []interface{}{limit}
	}
	
	rows, err := s.db.Query(query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	
	var assets []string
	for rows.Next() {
		var assetID string
		if err := rows.Scan(&assetID); err == nil {
			assets = append(assets, assetID)
		}
	}
	
	return assets, nil
}

// GetTimeRange 获取数据时间范围
func (s *AssetStorage) GetTimeRange(assetID string) (time.Time, time.Time, error) {
	if !s.config.Enabled {
		return time.Time{}, time.Time{}, fmt.Errorf("storage not enabled")
	}
	
	var minTime, maxTime time.Time
	
	err := s.db.QueryRow(`
		SELECT MIN(timestamp), MAX(timestamp) FROM asset_metrics WHERE asset_id = ?
	`, assetID).Scan(&minTime, &maxTime)
	
	return minTime, maxTime, err
}

// cleanup 清理过期数据
func (s *AssetStorage) cleanup() {
	cutoff := time.Now().Add(-s.config.RetentionPeriod)
	
	_, _ = s.db.Exec("DELETE FROM asset_metrics WHERE timestamp < ?", cutoff)
	_, _ = s.db.Exec("DELETE FROM asset_metrics_aggregated WHERE window_end < ?", cutoff)
}

// scanMetrics 扫描指标行
func (s *AssetStorage) scanMetrics(rows *sql.Rows) ([]*AssetMetrics, error) {
	var metrics []*AssetMetrics
	
	for rows.Next() {
		m := &AssetMetrics{}
		var network, application, system, labels string
		
		err := rows.Scan(&m.AssetID, &m.AssetType, &m.AssetName, &m.Timestamp,
			&network, &application, &system, &labels)
		if err != nil {
			return nil, err
		}
		
		json.Unmarshal([]byte(network), &m.Network)
		json.Unmarshal([]byte(application), &m.Application)
		json.Unmarshal([]byte(system), &m.System)
		json.Unmarshal([]byte(labels), &m.Labels)
		
		metrics = append(metrics, m)
	}
	
	return metrics, nil
}

// scanMetric 扫描单行指标
func (s *AssetStorage) scanMetric(row *sql.Row) (*AssetMetrics, error) {
	m := &AssetMetrics{}
	var network, application, system, labels string
	
	err := row.Scan(&m.AssetID, &m.AssetType, &m.AssetName, &m.Timestamp,
		&network, &application, &system, &labels)
	if err != nil {
		return nil, err
	}
	
	json.Unmarshal([]byte(network), &m.Network)
	json.Unmarshal([]byte(application), &m.Application)
	json.Unmarshal([]byte(system), &m.System)
	json.Unmarshal([]byte(labels), &m.Labels)
	
	return m, nil
}

// scanAggregatedMetrics 扫描聚合指标行
func (s *AssetStorage) scanAggregatedMetrics(rows *sql.Rows) ([]*AggregatedMetrics, error) {
	var metrics []*AggregatedMetrics
	
	for rows.Next() {
		m := &AggregatedMetrics{}
		var network, application, system string
		
		err := rows.Scan(&m.AssetID, &m.AssetType, &m.Granularity, &m.WindowStart, &m.WindowEnd,
			&m.SampleCount, &network, &application, &system)
		if err != nil {
			return nil, err
		}
		
		json.Unmarshal([]byte(network), &m.Network)
		json.Unmarshal([]byte(application), &m.Application)
		json.Unmarshal([]byte(system), &m.System)
		
		metrics = append(metrics, m)
	}
	
	return metrics, nil
}

// GetStats 获取存储统计
func (s *AssetStorage) GetStats() (*AssetStorageStats, error) {
	stats := &AssetStorageStats{}
	
	if s.config.Enabled {
		s.db.QueryRow("SELECT COUNT(*) FROM asset_metrics").Scan(&stats.TotalMetrics)
		s.db.QueryRow("SELECT COUNT(DISTINCT asset_id) FROM asset_metrics").Scan(&stats.AssetCount)
		s.db.QueryRow("SELECT COUNT(*) FROM asset_metrics_aggregated").Scan(&stats.AggregatedMetrics)
	}
	
	return stats, nil
}

// AssetStorageStats 存储统计
type AssetStorageStats struct {
	TotalMetrics      int `json:"total_metrics"`
	AssetCount        int `json:"asset_count"`
	AggregatedMetrics int `json:"aggregated_metrics"`
}

// QueryMetricsWithDownsampling 查询指标（支持降采样）
func (s *AssetStorage) QueryMetricsWithDownsampling(assetID string, start, end time.Time, points int) ([]*AssetMetrics, error) {
	if !s.config.Enabled {
		return nil, fmt.Errorf("storage not enabled")
	}
	
	// 计算时间间隔
	duration := end.Sub(start)
	interval := duration / time.Duration(points)
	
	// 使用聚合查询
	query := `
		SELECT 
			asset_id,
			asset_type,
			asset_name,
			MAX(timestamp) as timestamp,
			AVG(CAST(json_extract(network, '$.bytes_sent') AS REAL)) as bytes_sent,
			AVG(CAST(json_extract(network, '$.bytes_recv') AS REAL)) as bytes_recv,
			AVG(CAST(json_extract(application, '$.cpu_usage') AS REAL)) as cpu_usage,
			AVG(CAST(json_extract(application, '$.memory_usage') AS REAL)) as memory_usage
		FROM asset_metrics 
		WHERE asset_id = ? AND timestamp >= ? AND timestamp <= ?
		GROUP BY strftime('%s', timestamp) / ?
		ORDER BY timestamp DESC
	`
	
	intervalSec := int64(interval.Seconds())
	if intervalSec < 1 {
		intervalSec = 1
	}
	
	rows, err := s.db.Query(query, assetID, start, end, intervalSec)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	
	var metrics []*AssetMetrics
	for rows.Next() {
		m := &AssetMetrics{
			AssetID: assetID,
			Network: &NetworkMetrics{},
			Application: &ApplicationMetrics{},
		}
		
		var bytesSent, bytesRecv, cpuUsage, memoryUsage float64
		err := rows.Scan(&m.AssetID, &m.AssetType, &m.AssetName, &m.Timestamp,
			&bytesSent, &bytesRecv, &cpuUsage, &memoryUsage)
		if err != nil {
			continue
		}
		
		m.Network.BytesSent = uint64(bytesSent)
		m.Network.BytesRecv = uint64(bytesRecv)
		m.Application.CPUUsage = cpuUsage
		m.Application.MemoryUsage = memoryUsage
		
		metrics = append(metrics, m)
	}
	
	return metrics, nil
}
