/*
 * Cloud Flow Agent - Offline Sync Manager
 *
 * 断网续传管理器，网络恢复后自动同步本地缓存数据
 */

package cache

import (
	"context"
	"errors"
	"sync"
	"sync/atomic"
	"time"
)

// OfflineSyncManager 断网续传管理器
type OfflineSyncManager struct {
	mu sync.RWMutex
	
	store *CacheStore
	
	config *SyncConfig
	
	// 网络状态
	networkState NetworkState
	offlineSince time.Time
	lastSyncTime time.Time
	
	// 待发送队列
	pendingQueue []*CacheItem
	pendingMu   sync.Mutex
	
	// 同步器
	syncer Syncer
	
	// 统计
	stats SyncStats
	
	// 控制
	ctx    context.Context
	cancel context.CancelFunc
}

// SyncConfig 同步配置
type SyncConfig struct {
	Enabled           bool          // 启用断网续传
	MaxRetries       int           // 最大重试次数
	RetryInterval    time.Duration // 重试间隔
	BatchSize        int           // 批量大小
	Parallelism      int           // 并行度
	NetworkTimeout   time.Duration // 网络超时
	HealthCheckInterval time.Duration // 健康检查间隔
	AutoSync        bool          // 网络恢复后自动同步
	SyncPriority    bool          // 优先同步高优先级数据
	DedupEnabled    bool          // 启用去重
	DedupWindow     time.Duration // 去重窗口
}

// DefaultSyncConfig 默认配置
func DefaultSyncConfig() *SyncConfig {
	return &SyncConfig{
		Enabled:             true,
		MaxRetries:          10,
		RetryInterval:       30 * time.Second,
		BatchSize:           1000,
		Parallelism:         4,
		NetworkTimeout:      30 * time.Second,
		HealthCheckInterval: 5 * time.Second,
		AutoSync:            true,
		SyncPriority:        true,
		DedupEnabled:        true,
		DedupWindow:         5 * time.Minute,
	}
}

// NetworkState 网络状态
type NetworkState int

const (
	NetworkStateOnline  NetworkState = iota
	NetworkStateOffline
	NetworkStateReconnecting
)

// Syncer 同步器接口
type Syncer interface {
	Send(data []*CacheItem) error
	CheckHealth() bool
	GetEndpoint() string
}

// SyncStats 同步统计
type SyncStats struct {
	TotalSynced     uint64
	TotalFailed     uint64
	TotalRetries    uint64
	OfflineDuration  int64 // 秒
	LastSyncTime    int64
	PendingCount    int32
	NetworkChanges  uint64
}

// NewOfflineSyncManager 创建断网续传管理器
func NewOfflineSyncManager(store *CacheStore, syncer Syncer, config *SyncConfig) *OfflineSyncManager {
	if config == nil {
		config = DefaultSyncConfig()
	}
	
	ctx, cancel := context.WithCancel(context.Background())
	
	mgr := &OfflineSyncManager{
		store:        store,
		syncer:       syncer,
		config:       config,
		networkState: NetworkStateOnline,
		pendingQueue: make([]*CacheItem, 0),
		ctx:          ctx,
		cancel:       cancel,
	}
	
	// 启动网络监控
	go mgr.monitorNetwork()
	
	// 启动同步协程
	go mgr.syncLoop()
	
	return mgr
}

// SubmitForSync 提交数据进行同步
func (m *OfflineSyncManager) SubmitForSync(key string, data []byte, metadata Metadata) error {
	// 先存入本地缓存
	if err := m.store.Put(key, data, metadata); err != nil {
		return err
	}
	
	// 如果网络正常，尝试直接发送
	if atomic.LoadInt32((*int32)(&m.networkState)) == int32(NetworkStateOnline) {
		item := &CacheItem{
			Key:      key,
			Data:     data,
			Metadata: metadata,
		}
		if err := m.trySync(item); err != nil {
			// 发送失败，加入待发送队列
			m.addToPending(item)
		}
	} else {
		// 网络断开，加入待发送队列
		item := &CacheItem{
			Key:      key,
			Data:     data,
			Metadata: metadata,
		}
		m.addToPending(item)
	}
	
	return nil
}

// trySync 尝试同步单条数据
func (m *OfflineSyncManager) trySync(item *CacheItem) error {
	ctx, cancel := context.WithTimeout(context.Background(), m.config.NetworkTimeout)
	defer cancel()
	
	for retry := 0; retry <= m.config.MaxRetries; retry++ {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}
		
		if err := m.syncer.Send([]*CacheItem{item}); err != nil {
			if retry < m.config.MaxRetries {
				atomic.AddUint64(&m.stats.TotalRetries, 1)
				time.Sleep(m.config.RetryInterval * time.Duration(retry+1))
				continue
			}
			return err
		}
		
		// 发送成功，标记
		m.store.MarkSent([]string{item.Key})
		atomic.AddUint64(&m.stats.TotalSynced, 1)
		m.lastSyncTime = time.Now()
		return nil
	}
	
	return ErrQueueEmpty
}

// addToPending 添加到待发送队列
func (m *OfflineSyncManager) addToPending(item *CacheItem) {
	m.pendingMu.Lock()
	defer m.pendingMu.Unlock()
	
	m.pendingQueue = append(m.pendingQueue, item)
	atomic.AddInt32(&m.stats.PendingCount, 1)
}

// monitorNetwork 监控网络状态
func (m *OfflineSyncManager) monitorNetwork() {
	ticker := time.NewTicker(m.config.HealthCheckInterval)
	defer ticker.Stop()
	
	for {
		select {
		case <-ticker.C:
			m.checkNetwork()
		case <-m.ctx.Done():
			return
		}
	}
}

// checkNetwork 检查网络状态
func (m *OfflineSyncManager) checkNetwork() {
	wasOffline := m.networkState == NetworkStateOffline
	
	isHealthy := m.syncer.CheckHealth()
	
	m.mu.Lock()
	if isHealthy {
		if wasOffline {
			// 网络恢复
			m.networkState = NetworkStateReconnecting
			m.mu.Unlock()
			m.handleNetworkRecovery()
		} else {
			m.networkState = NetworkStateOnline
			m.mu.Unlock()
		}
	} else {
		if !wasOffline {
			// 网络断开
			m.networkState = NetworkStateOffline
			m.offlineSince = time.Now()
			m.mu.Unlock()
			m.handleNetworkLoss()
		} else {
			m.mu.Unlock()
			// 更新离线时长
			duration := time.Since(m.offlineSince).Seconds()
			atomic.StoreInt64(&m.stats.OfflineDuration, int64(duration))
		}
	}
	
	atomic.AddUint64(&m.stats.NetworkChanges, 1)
}

// handleNetworkLoss 处理网络断开
func (m *OfflineSyncManager) handleNetworkLoss() {
	// 切换到离线模式，所有数据存入本地缓存
	// 已由 SubmitForSync 处理
}

// handleNetworkRecovery 处理网络恢复
func (m *OfflineSyncManager) handleNetworkRecovery() {
	if !m.config.AutoSync {
		return
	}
	
	// 网络恢复，开始同步待发送数据
	go m.syncPending()
}

// syncLoop 同步循环
func (m *OfflineSyncManager) syncLoop() {
	ticker := time.NewTicker(m.config.RetryInterval)
	defer ticker.Stop()
	
	for {
		select {
		case <-ticker.C:
			m.syncPending()
		case <-m.ctx.Done():
			return
		}
	}
}

// syncPending 同步待发送数据
func (m *OfflineSyncManager) syncPending() {
	if m.networkState != NetworkStateOnline && m.networkState != NetworkStateReconnecting {
		return
	}
	
	m.pendingMu.Lock()
	queue := make([]*CacheItem, len(m.pendingQueue))
	copy(queue, m.pendingQueue)
	m.pendingMu.Unlock()
	
	if len(queue) == 0 {
		return
	}
	
	// 按优先级排序
	if m.config.SyncPriority {
		m.sortByPriority(queue)
	}
	
	// 批量同步
	batchSize := m.config.BatchSize
	for i := 0; i < len(queue); i += batchSize {
		end := i + batchSize
		if end > len(queue) {
			end = len(queue)
		}
		
		batch := queue[i:end]
		
		if err := m.syncBatch(batch); err != nil {
			// 批量失败，记录错误，继续下一个批次
			atomic.AddUint64(&m.stats.TotalFailed, uint64(len(batch)))
		}
	}
}

// syncBatch 同步批次
func (m *OfflineSyncManager) syncBatch(batch []*CacheItem) error {
	ctx, cancel := context.WithTimeout(context.Background(), m.config.NetworkTimeout*2)
	defer cancel()
	
	for retry := 0; retry <= m.config.MaxRetries; retry++ {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}
		
		if err := m.syncer.Send(batch); err != nil {
			if retry < m.config.MaxRetries {
				atomic.AddUint64(&m.stats.TotalRetries, 1)
				time.Sleep(m.config.RetryInterval * time.Duration(retry+1))
				continue
			}
			return err
		}
		
		// 发送成功，标记并从队列移除
		keys := make([]string, len(batch))
		for i, item := range batch {
			keys[i] = item.Key
		}
		m.store.MarkSent(keys)
		atomic.AddUint64(&m.stats.TotalSynced, uint64(len(batch)))
		m.lastSyncTime = time.Now()
		
		// 从待发送队列移除
		m.removeFromPending(keys)
		
		return nil
	}
	
	return ErrQueueEmpty
}

// removeFromPending 从待发送队列移除
func (m *OfflineSyncManager) removeFromPending(keys []string) {
	m.pendingMu.Lock()
	defer m.pendingMu.Unlock()
	
	keySet := make(map[string]bool)
	for _, k := range keys {
		keySet[k] = true
	}
	
	newQueue := make([]*CacheItem, 0, len(m.pendingQueue))
	for _, item := range m.pendingQueue {
		if !keySet[item.Key] {
			newQueue = append(newQueue, item)
		}
	}
	
	removed := len(m.pendingQueue) - len(newQueue)
	m.pendingQueue = newQueue
	atomic.AddInt32(&m.stats.PendingCount, -int32(removed))
}

// sortByPriority 按优先级排序
func (m *OfflineSyncManager) sortByPriority(items []*CacheItem) {
	// 优先级高的在前，年龄大的在前
	for i := 0; i < len(items)-1; i++ {
		for j := i + 1; j < len(items); j++ {
			if items[i].Priority < items[j].Priority ||
				(items[i].Priority == items[j].Priority && items[i].Timestamp > items[j].Timestamp) {
				items[i], items[j] = items[j], items[i]
			}
		}
	}
}

// GetNetworkState 获取网络状态
func (m *OfflineSyncManager) GetNetworkState() NetworkState {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.networkState
}

// GetPendingCount 获取待发送数量
func (m *OfflineSyncManager) GetPendingCount() int32 {
	return atomic.LoadInt32(&m.stats.PendingCount)
}

// GetStats 获取统计
func (m *OfflineSyncManager) GetStats() SyncStats {
	stats := m.stats
	stats.LastSyncTime = m.lastSyncTime.Unix()
	stats.PendingCount = atomic.LoadInt32(&m.stats.PendingCount)
	return stats
}

// ForceSync 强制同步
func (m *OfflineSyncManager) ForceSync() error {
	if m.networkState != NetworkStateOnline {
		return errors.New("network is not available")
	}
	m.syncPending()
	return nil
}

// Close 关闭管理器
func (m *OfflineSyncManager) Close() error {
	m.cancel()
	return nil
}
