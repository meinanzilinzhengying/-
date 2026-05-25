package cache

import (
	"bytes"
	"encoding/binary"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sync"
	"sync/atomic"
	"time"
)

var (
	ErrCacheFull    = errors.New("cache is full")
	ErrCacheMiss    = errors.New("cache miss")
	ErrQueueEmpty  = errors.New("queue is empty")
)

// CacheConfig 缓存配置
type CacheConfig struct {
	Enabled          bool          // 启用缓存
	CacheDir        string        // 缓存目录
	MaxSizeMB       int64         // 最大缓存大小 (MB)
	MaxItems        int           // 最大缓存条目数
	MaxAge          time.Duration // 数据最大保留时间
	SegmentSize     int           // 分段大小
	Compression     bool          // 启用压缩
	FlushInterval   time.Duration // 刷新间隔
	PreFetchEnabled bool          // 启用预取
	PreFetchCount   int           // 预取数量
}

// DefaultCacheConfig 默认配置
func DefaultCacheConfig() *CacheConfig {
	return &CacheConfig{
		Enabled:          true,
		CacheDir:        "/var/lib/cloud-flow/cache",
		MaxSizeMB:       10240, // 10GB
		MaxItems:        1000000,
		MaxAge:          7 * 24 * time.Hour,
		SegmentSize:     1000,
		Compression:     true,
		FlushInterval:   30 * time.Second,
		PreFetchEnabled: true,
		PreFetchCount:   100,
	}
}

// CacheItem 缓存条目
type CacheItem struct {
	Key       string    `json:"key"`
	Timestamp int64     `json:"timestamp"`
	Data      []byte    `json:"data"`
	Size      int       `json:"size"`
	Priority  int       `json:"priority"` // 优先级
	Retries   int       `json:"retries"`   // 发送失败重试次数
	Metadata  Metadata  `json:"metadata"`
}

// Metadata 元数据
type Metadata struct {
	Source     string            `json:"source"`
	Target     string            `json:"target"`
	Type       string            `json:"type"`
	Tags       map[string]string `json:"tags"`
	Checksum   uint64            `json:"checksum"`
}

// CacheStore 本地缓存存储
type CacheStore struct {
	mu sync.RWMutex
	
	config *CacheConfig
	
	// 内存索引
	index map[string]*CacheItem
	
	// 磁盘文件
	segmentFiles map[int]*os.File
	segmentIndex int
	
	// 统计
	stats CacheStats
	
	// 控制
	ctx    interface{ Done() <-struct{} }
	cancel func()
}

// CacheStats 缓存统计
type CacheStats struct {
	TotalItems      uint64
	TotalBytes     uint64
	Hits           uint64
	Misses         uint64
	Flushes        uint64
	Evictions      uint64
	CurrentSize    int64
	CurrentItems   int32
}

// NewCacheStore 创建缓存存储
func NewCacheStore(config *CacheConfig) (*CacheStore, error) {
	if config == nil {
		config = DefaultCacheConfig()
	}
	
	store := &CacheStore{
		config:       config,
		index:       make(map[string]*CacheItem),
		segmentFiles: make(map[int]*os.File),
	}
	
	// 确保目录存在
	if err := os.MkdirAll(config.CacheDir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create cache dir: %w", err)
	}
	
	// 加载现有缓存
	if err := store.loadIndex(); err != nil {
		// 忽略加载错误，从头开始
	}
	
	// 启动清理协程
	go store.cleanup()
	
	return store, nil
}

// Put 存入缓存
func (s *CacheStore) Put(key string, data []byte, metadata Metadata) error {
	if !s.config.Enabled {
		return nil
	}
	
	// 检查大小限制
	if s.stats.CurrentSize+int64(len(data)) > s.config.MaxSizeMB*1024*1024 {
		// 触发淘汰
		s.evict(len(data))
		if s.stats.CurrentSize+int64(len(data)) > s.config.MaxSizeMB*1024*1024 {
			return ErrCacheFull
		}
	}
	
	item := &CacheItem{
		Key:       key,
		Timestamp: time.Now().Unix(),
		Data:      data,
		Size:      len(data),
		Priority:  0,
		Metadata:  metadata,
	}
	
	// 压缩
	if s.config.Compression {
		compressed, err := s.compress(data)
		if err == nil {
			item.Data = compressed
			item.Size = len(compressed)
		}
	}
	
	s.mu.Lock()
	s.index[key] = item
	atomic.AddUint64(&s.stats.TotalItems, 1)
	atomic.AddInt64(&s.stats.CurrentSize, int64(item.Size))
	atomic.AddInt32(&s.stats.CurrentItems, 1)
	s.mu.Unlock()
	
	// 异步持久化
	go s.persistItem(item)
	
	return nil
}

// Get 获取缓存
func (s *CacheStore) Get(key string) (*CacheItem, error) {
	s.mu.RLock()
	item, exists := s.index[key]
	s.mu.RUnlock()
	
	if !exists {
		atomic.AddUint64(&s.stats.Misses, 1)
		return nil, ErrCacheMiss
	}
	
	// 检查过期
	if time.Since(time.Unix(item.Timestamp, 0)) > s.config.MaxAge {
		go s.Delete(key)
		atomic.AddUint64(&s.stats.Misses, 1)
		return nil, ErrCacheMiss
	}
	
	atomic.AddUint64(&s.stats.Hits, 1)
	
	// 解压缩
	if s.config.Compression {
		decompressed, err := s.decompress(item.Data)
		if err == nil {
			item.Data = decompressed
		}
	}
	
	return item, nil
}

// Delete 删除缓存
func (s *CacheStore) Delete(key string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	
	item, exists := s.index[key]
	if exists {
		delete(s.index, key)
		atomic.AddInt64(&s.stats.CurrentSize, -int64(item.Size))
		atomic.AddInt32(&s.stats.CurrentItems, -1)
	}
	
	return nil
}

// GetQueue 获取待发送队列（按优先级和年龄排序）
func (s *CacheStore) GetQueue(limit int) ([]*CacheItem, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	
	items := make([]*CacheItem, 0, limit)
	for _, item := range s.index {
		if len(items) >= limit {
			break
		}
		// 解压缩后添加
		data := item.Data
		if s.config.Compression {
			if decompressed, err := s.decompress(data); err == nil {
				data = decompressed
			}
		}
		itemCopy := &CacheItem{
			Key:       item.Key,
			Timestamp: item.Timestamp,
			Data:      data,
			Size:      item.Size,
			Priority:  item.Priority,
			Retries:   item.Retries,
			Metadata:  item.Metadata,
		}
		items = append(items, itemCopy)
	}
	
	// 按优先级和年龄排序
	// 优先级高的在前，年龄大的在前（LRU）
	sortCacheItems(items)
	
	return items, nil
}

// MarkSent 标记已发送
func (s *CacheStore) MarkSent(keys []string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	
	for _, key := range keys {
		if item, exists := s.index[key]; exists {
			delete(s.index, key)
			atomic.AddInt64(&s.stats.CurrentSize, -int64(item.Size))
			atomic.AddInt32(&s.stats.CurrentItems, -1)
			atomic.AddUint64(&s.stats.Flushes, 1)
		}
	}
}

// persistItem 持久化条目
func (s *CacheStore) persistItem(item *CacheItem) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	
	segment := s.getOrCreateSegment()
	if segment == nil {
		return errors.New("failed to create segment")
	}
	
	data, err := json.Marshal(item)
	if err != nil {
		return err
	}
	
	// 写入长度 + 数据
	header := make([]byte, 4)
	binary.LittleEndian.PutUint32(header, uint32(len(data)))
	
	if _, err := segment.Write(header); err != nil {
		return err
	}
	if _, err := segment.Write(data); err != nil {
		return err
	}
	segment.Sync()
	
	return nil
}

// getOrCreateSegment 获取或创建分段文件
func (s *CacheStore) getOrCreateSegment() *os.File {
	// 简单实现，每次创建新文件
	filename := filepath.Join(s.config.CacheDir, fmt.Sprintf("segment_%d_%d.dat", 
		time.Now().Unix(), s.segmentIndex))
	
	file, err := os.OpenFile(filename, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0644)
	if err != nil {
		return nil
	}
	
	return file
}

// compress 压缩数据
func (s *CacheStore) compress(data []byte) ([]byte, error) {
	var buf bytes.Buffer
	writer, err := gzip.NewWriterLevel(&buf, gzip.BestSpeed)
	if err != nil {
		return nil, err
	}
	writer.Write(data)
	writer.Close()
	return buf.Bytes(), nil
}

// decompress 解压缩数据
func (s *CacheStore) decompress(data []byte) ([]byte, error) {
	reader, err := gzip.NewReader(bytes.NewReader(data))
	if err != nil {
		return nil, err
	}
	return io.ReadAll(reader)
}

// evict 淘汰缓存
func (s *CacheStore) evict(requiredSpace int) {
	s.mu.Lock()
	defer s.mu.Unlock()
	
	// 按优先级和年龄排序，淘汰最低优先级的
	items := make([]*CacheItem, 0, len(s.index))
	for _, item := range s.index {
		items = append(items, item)
	}
	sortCacheItems(items)
	
	// 淘汰直到有足够空间
	freedSpace := 0
	for _, item := range items {
		if freedSpace >= requiredSpace {
			break
		}
		delete(s.index, item.Key)
		freedSpace += item.Size
		atomic.AddInt64(&s.stats.CurrentSize, -int64(item.Size))
		atomic.AddInt32(&s.stats.CurrentItems, -1)
		atomic.AddUint64(&s.stats.Evictions, 1)
	}
}

// cleanup 清理过期数据
func (s *CacheStore) cleanup() {
	ticker := time.NewTicker(time.Hour)
	defer ticker.Stop()
	
	for {
		select {
		case <-ticker.C:
			s.cleanExpired()
			s.cleanOverflow()
		}
	}
}

// cleanExpired 清理过期数据
func (s *CacheStore) cleanExpired() {
	s.mu.Lock()
	defer s.mu.Unlock()
	
	cutoff := time.Now().Add(-s.config.MaxAge).Unix()
	
	for key, item := range s.index {
		if item.Timestamp < cutoff {
			delete(s.index, key)
			atomic.AddInt64(&s.stats.CurrentSize, -int64(item.Size))
			atomic.AddInt32(&s.stats.CurrentItems, -1)
			atomic.AddUint64(&s.stats.Evictions, 1)
		}
	}
}

// cleanOverflow 清理超出限制的数据
func (s *CacheStore) cleanOverflow() {
	s.mu.Lock()
	defer s.mu.Unlock()
	
	// 检查数量限制
	count := int32(len(s.index))
	if count <= int32(s.config.MaxItems) {
		return
	}
	
	// 淘汰超出部分
	items := make([]*CacheItem, 0, len(s.index))
	for _, item := range s.index {
		items = append(items, item)
	}
	sortCacheItems(items)
	
	toDelete := count - int32(s.config.MaxItems)
	for i := len(items) - 1; i >= 0 && toDelete > 0; i-- {
		delete(s.index, items[i].Key)
		atomic.AddInt64(&s.stats.CurrentSize, -int64(items[i].Size))
		atomic.AddInt32(&s.stats.CurrentItems, -1)
		atomic.AddUint64(&s.stats.Evictions, 1)
		toDelete--
	}
}

// loadIndex 加载索引
func (s *CacheStore) loadIndex() error {
	// 简化实现，扫描目录中的所有 segment 文件
	files, err := filepath.Glob(filepath.Join(s.config.CacheDir, "segment_*.dat"))
	if err != nil {
		return err
	}
	
	for _, file := range files {
		if err := s.loadSegment(file); err != nil {
			continue
		}
	}
	
	return nil
}

// loadSegment 加载分段
func (s *CacheStore) loadSegment(path string) error {
	file, err := os.Open(path)
	if err != nil {
		return err
	}
	defer file.Close()
	
	for {
		header := make([]byte, 4)
		n, err := file.Read(header)
		if err == io.EOF {
			break
		}
		if err != nil || n < 4 {
			break
		}
		
		length := binary.LittleEndian.Uint32(header)
		data := make([]byte, length)
		n, err = file.Read(data)
		if err != nil || n < int(length) {
			break
		}
		
		var item CacheItem
		if err := json.Unmarshal(data, &item); err != nil {
			continue
		}
		
		s.index[item.Key] = &item
		atomic.AddInt64(&s.stats.CurrentSize, int64(item.Size))
		atomic.AddInt32(&s.stats.CurrentItems, 1)
	}
	
	return nil
}

// sortCacheItems 排序缓存项
func sortCacheItems(items []*CacheItem) {
	// 按优先级降序，年龄降序排序
	for i := 0; i < len(items)-1; i++ {
		for j := i + 1; j < len(items); j++ {
			if items[i].Priority < items[j].Priority ||
				(items[i].Priority == items[j].Priority && items[i].Timestamp > items[j].Timestamp) {
				items[i], items[j] = items[j], items[i]
			}
		}
	}
}

// GetStats 获取统计
func (s *CacheStore) GetStats() CacheStats {
	return CacheStats{
		TotalItems:    atomic.LoadUint64(&s.stats.TotalItems),
		TotalBytes:   atomic.LoadUint64(&s.stats.TotalBytes),
		Hits:         atomic.LoadUint64(&s.stats.Hits),
		Misses:       atomic.LoadUint64(&s.stats.Misses),
		Flushes:      atomic.LoadUint64(&s.stats.Flushes),
		Evictions:    atomic.LoadUint64(&s.stats.Evictions),
		CurrentSize:  atomic.LoadInt64(&s.stats.CurrentSize),
		CurrentItems: atomic.LoadInt32(&s.stats.CurrentItems),
	}
}

// Close 关闭缓存
func (s *CacheStore) Close() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	
	for _, file := range s.segmentFiles {
		file.Close()
	}
	
	return nil
}
