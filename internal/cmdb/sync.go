//go:build linux

// Package cmdb 提供CMDB系统对接功能
// 本文件实现业务属性同步服务
package cmdb

import (
	"context"
	"fmt"
	"sync"
	"time"

	"cloud-flow-agent/pkg/logger"
)

// SyncConfig 同步配置
type SyncConfig struct {
	Enabled        bool          `yaml:"enabled" json:"enabled"`                       // 是否启用同步
	FullSyncInterval time.Duration `yaml:"full_sync_interval" json:"full_sync_interval"` // 全量同步间隔
	IncrementalInterval time.Duration `yaml:"incremental_interval" json:"incremental_interval"` // 增量同步间隔
	CacheTTL       time.Duration `yaml:"cache_ttl" json:"cache_ttl"`                   // 缓存TTL
	BatchSize      int           `yaml:"batch_size" json:"batch_size"`                 // 批量大小
}

// DefaultSyncConfig 默认同步配置
func DefaultSyncConfig() SyncConfig {
	return SyncConfig{
		Enabled:             true,
		FullSyncInterval:    1 * time.Hour,    // 每小时全量同步
		IncrementalInterval: 5 * time.Minute,  // 每5分钟增量同步
		CacheTTL:            2 * time.Hour,    // 缓存2小时
		BatchSize:           100,              // 每批100个
	}
}

// SyncService 同步服务
type SyncService struct {
	config    SyncConfig
	client    *Client
	log       *logger.Logger

	// 本地缓存
	systems     map[string]*BusinessSystem  // system_id -> BusinessSystem
	ciItems     map[string]*CIItem          // ci_id -> CIItem
	ipIndex     map[string]string           // ip -> ci_id
	hostnameIndex map[string]string         // hostname -> ci_id

	// 保护缓存的锁
	mu sync.RWMutex

	// 同步状态
	status    SyncStatus
	statusMu  sync.RWMutex

	// 控制
	ctx    context.Context
	cancel context.CancelFunc
	wg     sync.WaitGroup
}

// NewSyncService 创建同步服务
func NewSyncService(config SyncConfig, client *Client, log *logger.Logger) *SyncService {
	ctx, cancel := context.WithCancel(context.Background())

	return &SyncService{
		config:        config,
		client:        client,
		log:           log,
		systems:       make(map[string]*BusinessSystem),
		ciItems:       make(map[string]*CIItem),
		ipIndex:       make(map[string]string),
		hostnameIndex: make(map[string]string),
		ctx:           ctx,
		cancel:        cancel,
	}
}

// Start 启动同步服务
func (s *SyncService) Start() error {
	if !s.config.Enabled {
		s.log.Info("CMDB同步服务已禁用")
		return nil
	}

	s.log.Info("启动CMDB同步服务")

	// 立即执行一次全量同步
	if err := s.FullSync(); err != nil {
		s.log.Warnf("初始全量同步失败: %v", err)
	}

	// 启动定时同步
	s.wg.Add(2)
	go s.fullSyncLoop()
	go s.incrementalSyncLoop()

	return nil
}

// Stop 停止同步服务
func (s *SyncService) Stop() {
	s.log.Info("停止CMDB同步服务")
	s.cancel()
	s.wg.Wait()
}

// fullSyncLoop 全量同步循环
func (s *SyncService) fullSyncLoop() {
	defer s.wg.Done()

	ticker := time.NewTicker(s.config.FullSyncInterval)
	defer ticker.Stop()

	for {
		select {
		case <-s.ctx.Done():
			return
		case <-ticker.C:
			if err := s.FullSync(); err != nil {
				s.log.Errorf("全量同步失败: %v", err)
				s.updateStatus(false, err.Error())
			} else {
				s.updateStatus(true, "")
			}
		}
	}
}

// incrementalSyncLoop 增量同步循环
func (s *SyncService) incrementalSyncLoop() {
	defer s.wg.Done()

	ticker := time.NewTicker(s.config.IncrementalInterval)
	defer ticker.Stop()

	for {
		select {
		case <-s.ctx.Done():
			return
		case <-ticker.C:
			if err := s.IncrementalSync(); err != nil {
				s.log.Errorf("增量同步失败: %v", err)
			}
		}
	}
}

// FullSync 全量同步
func (s *SyncService) FullSync() error {
	s.log.Info("开始CMDB全量同步")
	start := time.Now()

	// 同步业务系统
	systems, err := s.client.GetBusinessSystems(s.ctx)
	if err != nil {
		return fmt.Errorf("同步业务系统失败: %w", err)
	}

	s.mu.Lock()
	s.systems = make(map[string]*BusinessSystem)
	for i := range systems {
		s.systems[systems[i].ID] = &systems[i]
	}
	s.mu.Unlock()

	s.log.Infof("同步了 %d 个业务系统", len(systems))

	// 同步CI项（按类型分批）
	ciTypes := []string{CITypeHost, CITypeApp, CITypeService, CITypeDatabase}
	totalCI := 0

	for _, ciType := range ciTypes {
		items, err := s.client.GetCIItems(s.ctx, ciType)
		if err != nil {
			s.log.Warnf("同步CI类型 %s 失败: %v", ciType, err)
			continue
		}

		s.mu.Lock()
		for i := range items {
			item := &items[i]
			s.ciItems[item.ID] = item
			if item.IP != "" {
				s.ipIndex[item.IP] = item.ID
			}
			if item.Hostname != "" {
				s.hostnameIndex[item.Hostname] = item.ID
			}
		}
		s.mu.Unlock()

		totalCI += len(items)
		s.log.Infof("同步了 %d 个 %s 类型CI", len(items), ciType)
	}

	s.updateSyncStatus(len(systems), totalCI)
	s.log.Infof("CMDB全量同步完成，耗时 %v，系统: %d，CI: %d",
		time.Since(start), len(systems), totalCI)

	return nil
}

// IncrementalSync 增量同步
func (s *SyncService) IncrementalSync() error {
	// 获取上次同步时间后的变更
	// 实际实现应该调用CMDB的增量API
	// 这里简化为只更新有变化的CI

	s.log.Debug("开始CMDB增量同步")

	// 获取所有CI并检查更新时间
	// 简化实现：只同步主机类型的变更
	items, err := s.client.GetCIItems(s.ctx, CITypeHost)
	if err != nil {
		return fmt.Errorf("获取主机CI失败: %w", err)
	}

	updated := 0
	s.mu.Lock()
	for i := range items {
		item := &items[i]
		existing, exists := s.ciItems[item.ID]
		if !exists || existing.UpdateTime.Before(item.UpdateTime) {
			s.ciItems[item.ID] = item
			if item.IP != "" {
				s.ipIndex[item.IP] = item.ID
			}
			if item.Hostname != "" {
				s.hostnameIndex[item.Hostname] = item.ID
			}
			updated++
		}
	}
	s.mu.Unlock()

	if updated > 0 {
		s.log.Infof("增量同步更新了 %d 个CI", updated)
	}

	return nil
}

// ==================== 查询接口 ====================

// GetSystemByID 根据ID获取业务系统
func (s *SyncService) GetSystemByID(systemID string) (*BusinessSystem, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	system, exists := s.systems[systemID]
	return system, exists
}

// GetCIByID 根据ID获取CI
func (s *SyncService) GetCIByID(ciID string) (*CIItem, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	item, exists := s.ciItems[ciID]
	return item, exists
}

// GetCIByIP 根据IP获取CI
func (s *SyncService) GetCIByIP(ip string) (*CIItem, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	ciID, exists := s.ipIndex[ip]
	if !exists {
		return nil, false
	}

	item, exists := s.ciItems[ciID]
	return item, exists
}

// GetCIByHostname 根据主机名获取CI
func (s *SyncService) GetCIByHostname(hostname string) (*CIItem, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	ciID, exists := s.hostnameIndex[hostname]
	if !exists {
		return nil, false
	}

	item, exists := s.ciItems[ciID]
	return item, exists
}

// GetLabelsByIP 根据IP获取CMDB标签
func (s *SyncService) GetLabelsByIP(ip string) *CMDBLabels {
	item, exists := s.GetCIByIP(ip)
	if !exists {
		return nil
	}

	return s.buildLabels(item)
}

// GetLabelsByHostname 根据主机名获取CMDB标签
func (s *SyncService) GetLabelsByHostname(hostname string) *CMDBLabels {
	item, exists := s.GetCIByHostname(hostname)
	if !exists {
		return nil
	}

	return s.buildLabels(item)
}

// buildLabels 构建CMDB标签
func (s *SyncService) buildLabels(item *CIItem) *CMDBLabels {
	labels := &CMDBLabels{
		SystemID:    item.SystemID,
		SystemName:  item.SystemName,
		Environment: item.Environment,
		CIType:      item.Type,
		CIID:        item.ID,
		Labels:      make(map[string]string),
	}

	// 复制扩展标签
	for k, v := range item.Labels {
		labels.Labels[k] = v
	}

	// 获取业务系统信息补充标签
	s.mu.RLock()
	if system, exists := s.systems[item.SystemID]; exists {
		labels.SystemCode = system.Code
		labels.SystemLevel = system.Level.String()
		labels.Owner = system.Owner
		labels.Department = system.Department
	}
	s.mu.RUnlock()

	return labels
}

// GetAllSystems 获取所有业务系统
func (s *SyncService) GetAllSystems() []*BusinessSystem {
	s.mu.RLock()
	defer s.mu.RUnlock()

	result := make([]*BusinessSystem, 0, len(s.systems))
	for _, system := range s.systems {
		result = append(result, system)
	}
	return result
}

// GetSystemsByLevel 根据等级获取业务系统
func (s *SyncService) GetSystemsByLevel(level SystemLevel) []*BusinessSystem {
	s.mu.RLock()
	defer s.mu.RUnlock()

	result := make([]*BusinessSystem, 0)
	for _, system := range s.systems {
		if system.Level == level {
			result = append(result, system)
		}
	}
	return result
}

// GetCIItemsBySystem 根据业务系统获取CI列表
func (s *SyncService) GetCIItemsBySystem(systemID string) []*CIItem {
	s.mu.RLock()
	defer s.mu.RUnlock()

	result := make([]*CIItem, 0)
	for _, item := range s.ciItems {
		if item.SystemID == systemID {
			result = append(result, item)
		}
	}
	return result
}

// ==================== 状态管理 ====================

// updateStatus 更新同步状态
func (s *SyncService) updateStatus(success bool, errMsg string) {
	s.statusMu.Lock()
	defer s.statusMu.Unlock()

	s.status.LastSyncSuccess = success
	s.status.LastSyncTime = time.Now()
	s.status.NextSyncTime = time.Now().Add(s.config.FullSyncInterval)
	if !success {
		s.status.LastError = errMsg
	} else {
		s.status.LastError = ""
	}
}

// updateSyncStatus 更新同步统计
func (s *SyncService) updateSyncStatus(systemCount, ciCount int) {
	s.statusMu.Lock()
	defer s.statusMu.Unlock()

	s.status.SystemCount = systemCount
	s.status.CIItemCount = ciCount
}

// GetStatus 获取同步状态
func (s *SyncService) GetStatus() SyncStatus {
	s.statusMu.RLock()
	defer s.statusMu.RUnlock()
	return s.status
}

// GetStats 获取统计信息
func (s *SyncService) GetStats() map[string]interface{} {
	s.mu.RLock()
	s.statusMu.RLock()
	defer s.mu.RUnlock()
	defer s.statusMu.RUnlock()

	return map[string]interface{}{
		"systems_count":       len(s.systems),
		"ci_items_count":      len(s.ciItems),
		"ip_index_count":      len(s.ipIndex),
		"hostname_index_count": len(s.hostnameIndex),
		"last_sync_time":      s.status.LastSyncTime,
		"last_sync_success":   s.status.LastSyncSuccess,
		"next_sync_time":      s.status.NextSyncTime,
	}
}
