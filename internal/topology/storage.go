// Package topology 拓扑存储模块
// 提供拓扑实体和路径的持久化存储
package topology

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"sync"
	"time"

	_ "github.com/mattn/go-sqlite3"
)

// StorageConfig 存储配置
type StorageConfig struct {
	Enabled         bool          `json:"enabled" yaml:"enabled"`
	Driver          string        `json:"driver" yaml:"driver"`                     // sqlite/mysql/postgres
	DSN             string        `json:"dsn" yaml:"dsn"`                           // 数据源
	MaxConnections  int           `json:"max_connections" yaml:"max_connections"`
	MaxEntities     int           `json:"max_entities" yaml:"max_entities"`         // 最大实体数
	MaxPaths        int           `json:"max_paths" yaml:"max_paths"`               // 最大路径数
	MaxRelations    int           `json:"max_relations" yaml:"max_relations"`       // 最大关系数
	RetentionPeriod time.Duration `json:"retention_period" yaml:"retention_period"` // 数据保留期
	CleanupInterval time.Duration `json:"cleanup_interval" yaml:"cleanup_interval"` // 清理间隔
	EnableIndex     bool          `json:"enable_index" yaml:"enable_index"`         // 启用索引
	EnableCache     bool          `json:"enable_cache" yaml:"enable_cache"`         // 启用缓存
	CacheSize       int           `json:"cache_size" yaml:"cache_size"`             // 缓存大小
}

// TopologyStorage 拓扑存储
type TopologyStorage struct {
	config    *StorageConfig
	db        *sql.DB
	entityCache map[string]*Entity
	relationCache map[string]*Relation
	pathCache  map[string]*TracePath
	mu        sync.RWMutex
	ctx       context.Context
	cancel    context.CancelFunc
	wg        sync.WaitGroup
}

// NewTopologyStorage 创建拓扑存储
func NewTopologyStorage(config *StorageConfig) (*TopologyStorage, error) {
	ctx, cancel := context.WithCancel(context.Background())
	
	storage := &TopologyStorage{
		config:       config,
		entityCache:  make(map[string]*Entity),
		relationCache: make(map[string]*Relation),
		pathCache:    make(map[string]*TracePath),
		ctx:          ctx,
		cancel:       cancel,
	}
	
	// 初始化数据库
	if config.Enabled {
		if err := storage.initDB(); err != nil {
			return nil, fmt.Errorf("failed to init database: %w", err)
		}
		
		// 启动清理协程
		storage.wg.Add(1)
		go storage.cleanupLoop()
	}
	
	return storage, nil
}

// initDB 初始化数据库
func (s *TopologyStorage) initDB() error {
	var err error
	
	// 打开数据库连接
	s.db, err = sql.Open(s.config.Driver, s.config.DSN)
	if err != nil {
		return err
	}
	
	// 设置连接池
	s.db.SetMaxOpenConns(s.config.MaxConnections)
	s.db.SetMaxIdleConns(s.config.MaxConnections / 2)
	s.db.SetConnMaxLifetime(time.Hour)
	
	// 创建表
	if err := s.createTables(); err != nil {
		return err
	}
	
	// 创建索引
	if s.config.EnableIndex {
		if err := s.createIndexes(); err != nil {
			return err
		}
	}
	
	return nil
}

// createTables 创建表
func (s *TopologyStorage) createTables() error {
	// 实体表
	_, err := s.db.Exec(`
		CREATE TABLE IF NOT EXISTS topology_entities (
			id TEXT PRIMARY KEY,
			name TEXT NOT NULL,
			type TEXT NOT NULL,
			ip_addresses TEXT,
			mac_address TEXT,
			labels TEXT,
			annotations TEXT,
			parent_id TEXT,
			cluster_id TEXT,
			namespace TEXT,
			node_name TEXT,
			pod_name TEXT,
			container_id TEXT,
			process_ids TEXT,
			status TEXT,
			attributes TEXT,
			created_at TIMESTAMP,
			updated_at TIMESTAMP
		)
	`)
	if err != nil {
		return err
	}
	
	// 关系表
	_, err = s.db.Exec(`
		CREATE TABLE IF NOT EXISTS topology_relations (
			id TEXT PRIMARY KEY,
			source_id TEXT NOT NULL,
			target_id TEXT NOT NULL,
			type TEXT NOT NULL,
			properties TEXT,
			created_at TIMESTAMP
		)
	`)
	if err != nil {
		return err
	}
	
	// 路径表
	_, err = s.db.Exec(`
		CREATE TABLE IF NOT EXISTS topology_paths (
			id TEXT PRIMARY KEY,
			trace_id TEXT NOT NULL,
			start_time TIMESTAMP,
			end_time TIMESTAMP,
			duration BIGINT,
			source TEXT,
			destination TEXT,
			hops TEXT,
			status TEXT,
			error_code TEXT,
			error_message TEXT,
			protocol TEXT,
			method TEXT,
			path TEXT,
			status_code INTEGER,
			bytes_sent BIGINT,
			bytes_recv BIGINT,
			labels TEXT
		)
	`)
	if err != nil {
		return err
	}
	
	return nil
}

// createIndexes 创建索引
func (s *TopologyStorage) createIndexes() error {
	indexes := []string{
		"CREATE INDEX IF NOT EXISTS idx_entities_type ON topology_entities(type)",
		"CREATE INDEX IF NOT EXISTS idx_entities_cluster ON topology_entities(cluster_id)",
		"CREATE INDEX IF NOT EXISTS idx_entities_namespace ON topology_entities(namespace)",
		"CREATE INDEX IF NOT EXISTS idx_entities_parent ON topology_entities(parent_id)",
		"CREATE INDEX IF NOT EXISTS idx_relations_source ON topology_relations(source_id)",
		"CREATE INDEX IF NOT EXISTS idx_relations_target ON topology_relations(target_id)",
		"CREATE INDEX IF NOT EXISTS idx_relations_type ON topology_relations(type)",
		"CREATE INDEX IF NOT EXISTS idx_paths_trace_id ON topology_paths(trace_id)",
		"CREATE INDEX IF NOT EXISTS idx_paths_status ON topology_paths(status)",
		"CREATE INDEX IF NOT EXISTS idx_paths_start_time ON topology_paths(start_time)",
	}
	
	for _, idx := range indexes {
		if _, err := s.db.Exec(idx); err != nil {
			return err
		}
	}
	
	return nil
}

// Close 关闭存储
func (s *TopologyStorage) Close() error {
	s.cancel()
	s.wg.Wait()
	
	if s.db != nil {
		return s.db.Close()
	}
	return nil
}

// cleanupLoop 定期清理过期数据
func (s *TopologyStorage) cleanupLoop() {
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

// cleanup 清理过期数据
func (s *TopologyStorage) cleanup() {
	cutoff := time.Now().Add(-s.config.RetentionPeriod)
	
	// 清理过期路径
	_, _ = s.db.Exec("DELETE FROM topology_paths WHERE start_time < ?", cutoff)
	
	// 清理孤立实体（没有关联关系的）
	_, _ = s.db.Exec(`
		DELETE FROM topology_entities 
		WHERE id NOT IN (
			SELECT DISTINCT source_id FROM topology_relations
			UNION
			SELECT DISTINCT target_id FROM topology_relations
		) AND updated_at < ?
	`, cutoff)
}

// SaveEntity 保存实体
func (s *TopologyStorage) SaveEntity(entity *Entity) error {
	// 更新缓存
	if s.config.EnableCache {
		s.mu.Lock()
		s.entityCache[entity.ID] = entity
		// 限制缓存大小
		if len(s.entityCache) > s.config.CacheSize {
			// 删除最旧的
			var oldestID string
			var oldestTime time.Time
			for id, e := range s.entityCache {
				if oldestID == "" || e.UpdatedAt.Before(oldestTime) {
					oldestID = id
					oldestTime = e.UpdatedAt
				}
			}
			delete(s.entityCache, oldestID)
		}
		s.mu.Unlock()
	}
	
	if !s.config.Enabled {
		return nil
	}
	
	// 序列化字段
	ipAddresses, _ := json.Marshal(entity.IPAddresses)
	labels, _ := json.Marshal(entity.Labels)
	annotations, _ := json.Marshal(entity.Annotations)
	processIDs, _ := json.Marshal(entity.ProcessIDs)
	attributes, _ := json.Marshal(entity.Attributes)
	
	_, err := s.db.Exec(`
		INSERT OR REPLACE INTO topology_entities 
		(id, name, type, ip_addresses, mac_address, labels, annotations, 
		 parent_id, cluster_id, namespace, node_name, pod_name, container_id, 
		 process_ids, status, attributes, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`, entity.ID, entity.Name, entity.Type, string(ipAddresses), entity.MACAddress,
		string(labels), string(annotations), entity.ParentID, entity.ClusterID,
		entity.Namespace, entity.NodeName, entity.PodName, entity.ContainerID,
		string(processIDs), entity.Status, string(attributes),
		entity.CreatedAt, entity.UpdatedAt)
	
	return err
}

// GetEntity 获取实体
func (s *TopologyStorage) GetEntity(id string) (*Entity, error) {
	// 先查缓存
	if s.config.EnableCache {
		s.mu.RLock()
		if entity, exists := s.entityCache[id]; exists {
			s.mu.RUnlock()
			return entity, nil
		}
		s.mu.RUnlock()
	}
	
	if !s.config.Enabled {
		return nil, fmt.Errorf("storage not enabled")
	}
	
	entity := &Entity{}
	var ipAddresses, labels, annotations, processIDs, attributes string
	
	err := s.db.QueryRow(`
		SELECT id, name, type, ip_addresses, mac_address, labels, annotations,
		       parent_id, cluster_id, namespace, node_name, pod_name, container_id,
		       process_ids, status, attributes, created_at, updated_at
		FROM topology_entities WHERE id = ?
	`, id).Scan(&entity.ID, &entity.Name, &entity.Type, &ipAddresses, &entity.MACAddress,
		&labels, &annotations, &entity.ParentID, &entity.ClusterID,
		&entity.Namespace, &entity.NodeName, &entity.PodName, &entity.ContainerID,
		&processIDs, &entity.Status, &attributes, &entity.CreatedAt, &entity.UpdatedAt)
	
	if err != nil {
		return nil, err
	}
	
	// 反序列化
	json.Unmarshal([]byte(ipAddresses), &entity.IPAddresses)
	json.Unmarshal([]byte(labels), &entity.Labels)
	json.Unmarshal([]byte(annotations), &entity.Annotations)
	json.Unmarshal([]byte(processIDs), &entity.ProcessIDs)
	json.Unmarshal([]byte(attributes), &entity.Attributes)
	
	return entity, nil
}

// GetEntities 获取所有实体
func (s *TopologyStorage) GetEntities(limit, offset int) ([]*Entity, error) {
	if !s.config.Enabled {
		return nil, fmt.Errorf("storage not enabled")
	}
	
	rows, err := s.db.Query(`
		SELECT id, name, type, ip_addresses, mac_address, labels, annotations,
		       parent_id, cluster_id, namespace, node_name, pod_name, container_id,
		       process_ids, status, attributes, created_at, updated_at
		FROM topology_entities ORDER BY updated_at DESC LIMIT ? OFFSET ?
	`, limit, offset)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	
	return s.scanEntities(rows)
}

// GetEntitiesByType 按类型获取实体
func (s *TopologyStorage) GetEntitiesByType(entityType EntityType, limit, offset int) ([]*Entity, error) {
	if !s.config.Enabled {
		return nil, fmt.Errorf("storage not enabled")
	}
	
	rows, err := s.db.Query(`
		SELECT id, name, type, ip_addresses, mac_address, labels, annotations,
		       parent_id, cluster_id, namespace, node_name, pod_name, container_id,
		       process_ids, status, attributes, created_at, updated_at
		FROM topology_entities WHERE type = ? ORDER BY updated_at DESC LIMIT ? OFFSET ?
	`, entityType, limit, offset)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	
	return s.scanEntities(rows)
}

// DeleteEntity 删除实体
func (s *TopologyStorage) DeleteEntity(id string) error {
	// 删除缓存
	if s.config.EnableCache {
		s.mu.Lock()
		delete(s.entityCache, id)
		s.mu.Unlock()
	}
	
	if !s.config.Enabled {
		return nil
	}
	
	_, err := s.db.Exec("DELETE FROM topology_entities WHERE id = ?", id)
	return err
}

// scanEntities 扫描实体行
func (s *TopologyStorage) scanEntities(rows *sql.Rows) ([]*Entity, error) {
	var entities []*Entity
	
	for rows.Next() {
		entity := &Entity{}
		var ipAddresses, labels, annotations, processIDs, attributes string
		
		err := rows.Scan(&entity.ID, &entity.Name, &entity.Type, &ipAddresses, &entity.MACAddress,
			&labels, &annotations, &entity.ParentID, &entity.ClusterID,
			&entity.Namespace, &entity.NodeName, &entity.PodName, &entity.ContainerID,
			&processIDs, &entity.Status, &attributes, &entity.CreatedAt, &entity.UpdatedAt)
		if err != nil {
			return nil, err
		}
		
		json.Unmarshal([]byte(ipAddresses), &entity.IPAddresses)
		json.Unmarshal([]byte(labels), &entity.Labels)
		json.Unmarshal([]byte(annotations), &entity.Annotations)
		json.Unmarshal([]byte(processIDs), &entity.ProcessIDs)
		json.Unmarshal([]byte(attributes), &entity.Attributes)
		
		entities = append(entities, entity)
	}
	
	return entities, nil
}

// SaveRelation 保存关系
func (s *TopologyStorage) SaveRelation(relation *Relation) error {
	if s.config.EnableCache {
		s.mu.Lock()
		s.relationCache[relation.ID] = relation
		s.mu.Unlock()
	}
	
	if !s.config.Enabled {
		return nil
	}
	
	properties, _ := json.Marshal(relation.Properties)
	
	_, err := s.db.Exec(`
		INSERT OR REPLACE INTO topology_relations 
		(id, source_id, target_id, type, properties, created_at)
		VALUES (?, ?, ?, ?, ?, ?)
	`, relation.ID, relation.SourceID, relation.TargetID, relation.Type,
		string(properties), relation.CreatedAt)
	
	return err
}

// GetRelations 获取所有关系
func (s *TopologyStorage) GetRelations(limit, offset int) ([]*Relation, error) {
	if !s.config.Enabled {
		return nil, fmt.Errorf("storage not enabled")
	}
	
	rows, err := s.db.Query(`
		SELECT id, source_id, target_id, type, properties, created_at
		FROM topology_relations ORDER BY created_at DESC LIMIT ? OFFSET ?
	`, limit, offset)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	
	var relations []*Relation
	
	for rows.Next() {
		relation := &Relation{}
		var properties string
		
		err := rows.Scan(&relation.ID, &relation.SourceID, &relation.TargetID,
			&relation.Type, &properties, &relation.CreatedAt)
		if err != nil {
			return nil, err
		}
		
		json.Unmarshal([]byte(properties), &relation.Properties)
		relations = append(relations, relation)
	}
	
	return relations, nil
}

// GetEntityRelations 获取实体的关系
func (s *TopologyStorage) GetEntityRelations(entityID string) ([]*Relation, error) {
	if !s.config.Enabled {
		return nil, fmt.Errorf("storage not enabled")
	}
	
	rows, err := s.db.Query(`
		SELECT id, source_id, target_id, type, properties, created_at
		FROM topology_relations WHERE source_id = ? OR target_id = ?
	`, entityID, entityID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	
	var relations []*Relation
	
	for rows.Next() {
		relation := &Relation{}
		var properties string
		
		err := rows.Scan(&relation.ID, &relation.SourceID, &relation.TargetID,
			&relation.Type, &properties, &relation.CreatedAt)
		if err != nil {
			return nil, err
		}
		
		json.Unmarshal([]byte(properties), &relation.Properties)
		relations = append(relations, relation)
	}
	
	return relations, nil
}

// DeleteRelation 删除关系
func (s *TopologyStorage) DeleteRelation(id string) error {
	if s.config.EnableCache {
		s.mu.Lock()
		delete(s.relationCache, id)
		s.mu.Unlock()
	}
	
	if !s.config.Enabled {
		return nil
	}
	
	_, err := s.db.Exec("DELETE FROM topology_relations WHERE id = ?", id)
	return err
}

// SavePath 保存路径
func (s *TopologyStorage) SavePath(path *TracePath) error {
	if s.config.EnableCache {
		s.mu.Lock()
		s.pathCache[path.ID] = path
		s.mu.Unlock()
	}
	
	if !s.config.Enabled {
		return nil
	}
	
	source, _ := json.Marshal(path.Source)
	destination, _ := json.Marshal(path.Destination)
	hops, _ := json.Marshal(path.Hops)
	labels, _ := json.Marshal(path.Labels)
	
	_, err := s.db.Exec(`
		INSERT OR REPLACE INTO topology_paths 
		(id, trace_id, start_time, end_time, duration, source, destination, hops,
		 status, error_code, error_message, protocol, method, path, status_code,
		 bytes_sent, bytes_recv, labels)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`, path.ID, path.TraceID, path.StartTime, path.EndTime, path.Duration.Nanoseconds(),
		string(source), string(destination), string(hops),
		path.Status, path.ErrorCode, path.ErrorMessage, path.Protocol, path.Method,
		path.Path, path.StatusCode, path.BytesSent, path.BytesRecv, string(labels))
	
	return err
}

// GetPath 获取路径
func (s *TopologyStorage) GetPath(id string) (*TracePath, error) {
	if s.config.EnableCache {
		s.mu.RLock()
		if path, exists := s.pathCache[id]; exists {
			s.mu.RUnlock()
			return path, nil
		}
		s.mu.RUnlock()
	}
	
	if !s.config.Enabled {
		return nil, fmt.Errorf("storage not enabled")
	}
	
	path := &TracePath{}
	var source, destination, hops, labels string
	var durationNS int64
	
	err := s.db.QueryRow(`
		SELECT id, trace_id, start_time, end_time, duration, source, destination, hops,
		       status, error_code, error_message, protocol, method, path, status_code,
		       bytes_sent, bytes_recv, labels
		FROM topology_paths WHERE id = ?
	`, id).Scan(&path.ID, &path.TraceID, &path.StartTime, &path.EndTime, &durationNS,
		&source, &destination, &hops, &path.Status, &path.ErrorCode, &path.ErrorMessage,
		&path.Protocol, &path.Method, &path.Path, &path.StatusCode,
		&path.BytesSent, &path.BytesRecv, &labels)
	
	if err != nil {
		return nil, err
	}
	
	path.Duration = time.Duration(durationNS)
	json.Unmarshal([]byte(source), &path.Source)
	json.Unmarshal([]byte(destination), &path.Destination)
	json.Unmarshal([]byte(hops), &path.Hops)
	json.Unmarshal([]byte(labels), &path.Labels)
	
	return path, nil
}

// GetPaths 获取路径列表
func (s *TopologyStorage) GetPaths(limit, offset int) ([]*TracePath, error) {
	if !s.config.Enabled {
		return nil, fmt.Errorf("storage not enabled")
	}
	
	rows, err := s.db.Query(`
		SELECT id, trace_id, start_time, end_time, duration, source, destination, hops,
		       status, error_code, error_message, protocol, method, path, status_code,
		       bytes_sent, bytes_recv, labels
		FROM topology_paths ORDER BY start_time DESC LIMIT ? OFFSET ?
	`, limit, offset)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	
	return s.scanPaths(rows)
}

// GetPathsByTraceID 通过TraceID获取路径
func (s *TopologyStorage) GetPathsByTraceID(traceID string) ([]*TracePath, error) {
	if !s.config.Enabled {
		return nil, fmt.Errorf("storage not enabled")
	}
	
	rows, err := s.db.Query(`
		SELECT id, trace_id, start_time, end_time, duration, source, destination, hops,
		       status, error_code, error_message, protocol, method, path, status_code,
		       bytes_sent, bytes_recv, labels
		FROM topology_paths WHERE trace_id = ?
	`, traceID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	
	return s.scanPaths(rows)
}

// GetPathsByTimeRange 按时间范围获取路径
func (s *TopologyStorage) GetPathsByTimeRange(start, end time.Time, limit, offset int) ([]*TracePath, error) {
	if !s.config.Enabled {
		return nil, fmt.Errorf("storage not enabled")
	}
	
	rows, err := s.db.Query(`
		SELECT id, trace_id, start_time, end_time, duration, source, destination, hops,
		       status, error_code, error_message, protocol, method, path, status_code,
		       bytes_sent, bytes_recv, labels
		FROM topology_paths WHERE start_time >= ? AND start_time <= ?
		ORDER BY start_time DESC LIMIT ? OFFSET ?
	`, start, end, limit, offset)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	
	return s.scanPaths(rows)
}

// scanPaths 扫描路径行
func (s *TopologyStorage) scanPaths(rows *sql.Rows) ([]*TracePath, error) {
	var paths []*TracePath
	
	for rows.Next() {
		path := &TracePath{}
		var source, destination, hops, labels string
		var durationNS int64
		
		err := rows.Scan(&path.ID, &path.TraceID, &path.StartTime, &path.EndTime, &durationNS,
			&source, &destination, &hops, &path.Status, &path.ErrorCode, &path.ErrorMessage,
			&path.Protocol, &path.Method, &path.Path, &path.StatusCode,
			&path.BytesSent, &path.BytesRecv, &labels)
		if err != nil {
			return nil, err
		}
		
		path.Duration = time.Duration(durationNS)
		json.Unmarshal([]byte(source), &path.Source)
		json.Unmarshal([]byte(destination), &path.Destination)
		json.Unmarshal([]byte(hops), &path.Hops)
		json.Unmarshal([]byte(labels), &path.Labels)
		
		paths = append(paths, path)
	}
	
	return paths, nil
}

// DeletePath 删除路径
func (s *TopologyStorage) DeletePath(id string) error {
	if s.config.EnableCache {
		s.mu.Lock()
		delete(s.pathCache, id)
		s.mu.Unlock()
	}
	
	if !s.config.Enabled {
		return nil
	}
	
	_, err := s.db.Exec("DELETE FROM topology_paths WHERE id = ?", id)
	return err
}

// GetStats 获取存储统计
func (s *TopologyStorage) GetStats() (*StorageStats, error) {
	stats := &StorageStats{}
	
	if s.config.Enabled {
		s.db.QueryRow("SELECT COUNT(*) FROM topology_entities").Scan(&stats.EntityCount)
		s.db.QueryRow("SELECT COUNT(*) FROM topology_relations").Scan(&stats.RelationCount)
		s.db.QueryRow("SELECT COUNT(*) FROM topology_paths").Scan(&stats.PathCount)
	}
	
	if s.config.EnableCache {
		s.mu.RLock()
		stats.CachedEntities = len(s.entityCache)
		stats.CachedRelations = len(s.relationCache)
		stats.CachedPaths = len(s.pathCache)
		s.mu.RUnlock()
	}
	
	return stats, nil
}

// StorageStats 存储统计
type StorageStats struct {
	EntityCount     int `json:"entity_count"`
	RelationCount   int `json:"relation_count"`
	PathCount       int `json:"path_count"`
	CachedEntities  int `json:"cached_entities"`
	CachedRelations int `json:"cached_relations"`
	CachedPaths     int `json:"cached_paths"`
}

// BatchSaveEntities 批量保存实体
func (s *TopologyStorage) BatchSaveEntities(entities []*Entity) error {
	if !s.config.Enabled {
		return nil
	}
	
	tx, err := s.db.Begin()
	if err != nil {
		return err
	}
	
	stmt, err := tx.Prepare(`
		INSERT OR REPLACE INTO topology_entities 
		(id, name, type, ip_addresses, mac_address, labels, annotations, 
		 parent_id, cluster_id, namespace, node_name, pod_name, container_id, 
		 process_ids, status, attributes, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`)
	if err != nil {
		tx.Rollback()
		return err
	}
	defer stmt.Close()
	
	for _, entity := range entities {
		ipAddresses, _ := json.Marshal(entity.IPAddresses)
		labels, _ := json.Marshal(entity.Labels)
		annotations, _ := json.Marshal(entity.Annotations)
		processIDs, _ := json.Marshal(entity.ProcessIDs)
		attributes, _ := json.Marshal(entity.Attributes)
		
		_, err := stmt.Exec(entity.ID, entity.Name, entity.Type, string(ipAddresses),
			entity.MACAddress, string(labels), string(annotations), entity.ParentID,
			entity.ClusterID, entity.Namespace, entity.NodeName, entity.PodName,
			entity.ContainerID, string(processIDs), entity.Status, string(attributes),
			entity.CreatedAt, entity.UpdatedAt)
		if err != nil {
			tx.Rollback()
			return err
		}
	}
	
	return tx.Commit()
}

// BatchSaveRelations 批量保存关系
func (s *TopologyStorage) BatchSaveRelations(relations []*Relation) error {
	if !s.config.Enabled {
		return nil
	}
	
	tx, err := s.db.Begin()
	if err != nil {
		return err
	}
	
	stmt, err := tx.Prepare(`
		INSERT OR REPLACE INTO topology_relations 
		(id, source_id, target_id, type, properties, created_at)
		VALUES (?, ?, ?, ?, ?, ?)
	`)
	if err != nil {
		tx.Rollback()
		return err
	}
	defer stmt.Close()
	
	for _, relation := range relations {
		properties, _ := json.Marshal(relation.Properties)
		
		_, err := stmt.Exec(relation.ID, relation.SourceID, relation.TargetID,
			relation.Type, string(properties), relation.CreatedAt)
		if err != nil {
			tx.Rollback()
			return err
		}
	}
	
	return tx.Commit()
}

// SearchEntities 搜索实体
func (s *TopologyStorage) SearchEntities(query string, limit int) ([]*Entity, error) {
	if !s.config.Enabled {
		return nil, fmt.Errorf("storage not enabled")
	}
	
	searchPattern := "%" + query + "%"
	
	rows, err := s.db.Query(`
		SELECT id, name, type, ip_addresses, mac_address, labels, annotations,
		       parent_id, cluster_id, namespace, node_name, pod_name, container_id,
		       process_ids, status, attributes, created_at, updated_at
		FROM topology_entities 
		WHERE name LIKE ? OR id LIKE ? OR namespace LIKE ? OR pod_name LIKE ?
		ORDER BY updated_at DESC LIMIT ?
	`, searchPattern, searchPattern, searchPattern, searchPattern, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	
	return s.scanEntities(rows)
}
