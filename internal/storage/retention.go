/*
 * Copyright (c) 2025 Yunlong Liao. All rights reserved.
 */

package storage

import (
	"context"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"syscall"
	"time"

	"cloud-flow-agent/pkg/logger"
)

// RetentionPolicy 数据留存策略
type RetentionPolicy struct {
	MaxAge           time.Duration // 最大保留时间
	MaxSize          int64         // 最大磁盘使用量 (bytes)
	MinFreeSpace     int64         // 最小剩余空间 (bytes)
	CleanupInterval  time.Duration // 清理检查间隔
	ArchiveEnabled   bool          // 是否归档
	ArchivePath      string        // 归档路径
	ArchiveRetention time.Duration // 归档保留时间
	CompressionEnabled bool        // 是否启用压缩
	CompressionLevel int           // 压缩级别
}

// DefaultRetentionPolicy 默认留存策略 (60天)
func DefaultRetentionPolicy() *RetentionPolicy {
	return &RetentionPolicy{
		MaxAge:             60 * 24 * time.Hour, // 60天
		MaxSize:            100 * 1024 * 1024 * 1024, // 100GB
		MinFreeSpace:       10 * 1024 * 1024 * 1024,  // 10GB
		CleanupInterval:    1 * time.Hour,
		ArchiveEnabled:     true,
		ArchivePath:        "/var/lib/cloud-flow-agent/archive",
		ArchiveRetention:   180 * 24 * time.Hour, // 180天归档保留
		CompressionEnabled: true,
		CompressionLevel:   6,
	}
}

// DataCategory 数据类别
type DataCategory string

const (
	CategoryMetrics    DataCategory = "metrics"     // 指标数据
	CategoryLogs       DataCategory = "logs"        // 日志数据
	CategoryTraces     DataCategory = "traces"      // 链路数据
	CategoryProfiles   DataCategory = "profiles"    // 性能剖析数据
	CategoryPCAP       DataCategory = "pcap"        // 抓包数据
	CategoryDatabase   DataCategory = "database"    // 数据库观测数据
	CategoryEvents     DataCategory = "events"      // 事件数据
)

// CategoryConfig 类别配置
type CategoryConfig struct {
	Category       DataCategory
	Path           string
	MaxAge         time.Duration
	MaxSize        int64
	ArchiveEnabled bool
	Compress       bool // 是否压缩归档
}

// RetentionManager 留存管理器
type RetentionManager struct {
	policy     *RetentionPolicy
	categories map[DataCategory]*CategoryConfig
	stopCh     chan struct{}
	wg         sync.WaitGroup
	mu         sync.RWMutex
	stats      RetentionStats
}

// RetentionStats 留存统计
type RetentionStats struct {
	TotalCleanedFiles   uint64        `json:"total_cleaned_files"`
	TotalCleanedBytes   uint64        `json:"total_cleaned_bytes"`
	TotalArchivedFiles  uint64        `json:"total_archived_files"`
	TotalArchivedBytes  uint64        `json:"total_archived_bytes"`
	LastCleanupTime     time.Time     `json:"last_cleanup_time"`
	LastCleanupDuration time.Duration `json:"last_cleanup_duration"`
	CurrentDiskUsage    int64         `json:"current_disk_usage"`
	CurrentFreeSpace    int64         `json:"current_free_space"`
}

// FileInfo 文件信息
type FileInfo struct {
	Path      string
	Size      int64
	ModTime   time.Time
	Category  DataCategory
}

// NewRetentionManager 创建留存管理器
func NewRetentionManager(policy *RetentionPolicy) *RetentionManager {
	if policy == nil {
		policy = DefaultRetentionPolicy()
	}

	rm := &RetentionManager{
		policy:     policy,
		categories: make(map[DataCategory]*CategoryConfig),
		stopCh:     make(chan struct{}),
	}

	// 初始化默认类别
	rm.initDefaultCategories()

	return rm
}

// initDefaultCategories 初始化默认数据类别
func (rm *RetentionManager) initDefaultCategories() {
	basePath := "/var/lib/cloud-flow-agent"

	rm.RegisterCategory(&CategoryConfig{
		Category:       CategoryMetrics,
		Path:           filepath.Join(basePath, "metrics"),
		MaxAge:         rm.policy.MaxAge,
		MaxSize:        20 * 1024 * 1024 * 1024, // 20GB
		ArchiveEnabled: true,
		Compress:       true,
	})

	rm.RegisterCategory(&CategoryConfig{
		Category:       CategoryLogs,
		Path:           filepath.Join(basePath, "logs"),
		MaxAge:         30 * 24 * time.Hour, // 30天
		MaxSize:        10 * 1024 * 1024 * 1024, // 10GB
		ArchiveEnabled: true,
		Compress:       true,
	})

	rm.RegisterCategory(&CategoryConfig{
		Category:       CategoryTraces,
		Path:           filepath.Join(basePath, "traces"),
		MaxAge:         rm.policy.MaxAge,
		MaxSize:        30 * 1024 * 1024 * 1024, // 30GB
		ArchiveEnabled: true,
		Compress:       true,
	})

	rm.RegisterCategory(&CategoryConfig{
		Category:       CategoryProfiles,
		Path:           filepath.Join(basePath, "profiles"),
		MaxAge:         7 * 24 * time.Hour, // 7天
		MaxSize:        5 * 1024 * 1024 * 1024, // 5GB
		ArchiveEnabled: true,
		Compress:       true,
	})

	rm.RegisterCategory(&CategoryConfig{
		Category:       CategoryPCAP,
		Path:           filepath.Join(basePath, "pcap"),
		MaxAge:         3 * 24 * time.Hour, // 3天 (抓包数据通常很大)
		MaxSize:        50 * 1024 * 1024 * 1024, // 50GB
		ArchiveEnabled: true,
		Compress:       false, // PCAP已压缩
	})

	rm.RegisterCategory(&CategoryConfig{
		Category:       CategoryDatabase,
		Path:           filepath.Join(basePath, "database"),
		MaxAge:         rm.policy.MaxAge,
		MaxSize:        15 * 1024 * 1024 * 1024, // 15GB
		ArchiveEnabled: true,
		Compress:       true,
	})

	rm.RegisterCategory(&CategoryConfig{
		Category:       CategoryEvents,
		Path:           filepath.Join(basePath, "events"),
		MaxAge:         90 * 24 * time.Hour, // 90天
		MaxSize:        5 * 1024 * 1024 * 1024, // 5GB
		ArchiveEnabled: true,
		Compress:       true,
	})
}

// RegisterCategory 注册数据类别
func (rm *RetentionManager) RegisterCategory(config *CategoryConfig) {
	rm.mu.Lock()
	defer rm.mu.Unlock()

	rm.categories[config.Category] = config

	// 确保目录存在
	if err := os.MkdirAll(config.Path, 0755); err != nil {
		logger.Errorf("Failed to create category directory %s: %v", config.Path, err)
	}
}

// Start 启动留存管理器
func (rm *RetentionManager) Start(ctx context.Context) error {
	logger.Info("Starting retention manager")

	// 立即执行一次清理
	if err := rm.Cleanup(ctx); err != nil {
		logger.Warnf("Initial cleanup failed: %v", err)
	}

	// 启动定期清理任务
	rm.wg.Add(1)
	go rm.cleanupLoop(ctx)

	// 启动磁盘监控
	rm.wg.Add(1)
	go rm.diskMonitorLoop(ctx)

	logger.Info("Retention manager started")
	return nil
}

// Stop 停止留存管理器
func (rm *RetentionManager) Stop() error {
	logger.Info("Stopping retention manager")
	close(rm.stopCh)
	rm.wg.Wait()
	logger.Info("Retention manager stopped")
	return nil
}

// cleanupLoop 定期清理循环
func (rm *RetentionManager) cleanupLoop(ctx context.Context) {
	defer rm.wg.Done()

	ticker := time.NewTicker(rm.policy.CleanupInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-rm.stopCh:
			return
		case <-ticker.C:
			if err := rm.Cleanup(ctx); err != nil {
				logger.Errorf("Scheduled cleanup failed: %v", err)
			}
		}
	}
}

// diskMonitorLoop 磁盘监控循环
func (rm *RetentionManager) diskMonitorLoop(ctx context.Context) {
	defer rm.wg.Done()

	ticker := time.NewTicker(5 * time.Minute)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-rm.stopCh:
			return
		case <-ticker.C:
			if err := rm.checkDiskSpace(ctx); err != nil {
				logger.Errorf("Disk space check failed: %v", err)
			}
		}
	}
}

// Cleanup 执行清理
func (rm *RetentionManager) Cleanup(ctx context.Context) error {
	startTime := time.Now()
	logger.Info("Starting data retention cleanup")

	rm.mu.RLock()
	categories := make([]*CategoryConfig, 0, len(rm.categories))
	for _, cat := range rm.categories {
		categories = append(categories, cat)
	}
	rm.mu.RUnlock()

	var totalCleaned, totalArchived int64

	for _, cat := range categories {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		cleaned, archived, err := rm.cleanupCategory(ctx, cat)
		if err != nil {
			logger.Errorf("Cleanup category %s failed: %v", cat.Category, err)
			continue
		}

		totalCleaned += cleaned
		totalArchived += archived
	}

	// 清理归档目录
	if err := rm.cleanupArchive(ctx); err != nil {
		logger.Errorf("Archive cleanup failed: %v", err)
	}

	// 更新统计
	rm.mu.Lock()
	rm.stats.TotalCleanedFiles++
	rm.stats.TotalCleanedBytes += uint64(totalCleaned)
	rm.stats.TotalArchivedFiles++
	rm.stats.TotalArchivedBytes += uint64(totalArchived)
	rm.stats.LastCleanupTime = time.Now()
	rm.stats.LastCleanupDuration = time.Since(startTime)
	rm.mu.Unlock()

	logger.Infof("Cleanup completed: cleaned %d bytes, archived %d bytes in %v",
		totalCleaned, totalArchived, time.Since(startTime))

	return nil
}

// cleanupCategory 清理单个类别
func (rm *RetentionManager) cleanupCategory(ctx context.Context, config *CategoryConfig) (int64, int64, error) {
	files, err := rm.collectFiles(config.Path, config.Category)
	if err != nil {
		return 0, 0, err
	}

	// 按修改时间排序（最旧的在前）
	sort.Slice(files, func(i, j int) bool {
		return files[i].ModTime.Before(files[j].ModTime)
	})

	var cleaned, archived int64
	now := time.Now()
	cutoffTime := now.Add(-config.MaxAge)

	// 第一阶段：按时间清理
	for _, file := range files {
		select {
		case <-ctx.Done():
			return cleaned, archived, ctx.Err()
		default:
		}

		if file.ModTime.Before(cutoffTime) {
			if config.ArchiveEnabled && rm.policy.ArchiveEnabled {
				size, err := rm.archiveFile(file, config)
				if err != nil {
					logger.Warnf("Failed to archive file %s: %v", file.Path, err)
					// 归档失败则直接删除
					if err := os.Remove(file.Path); err != nil {
						logger.Warnf("Failed to remove file %s: %v", file.Path, err)
						continue
					}
					size = file.Size
				}
				archived += size
			} else {
				if err := os.Remove(file.Path); err != nil {
					logger.Warnf("Failed to remove file %s: %v", file.Path, err)
					continue
				}
			}
			cleaned += file.Size
		}
	}

	// 第二阶段：按大小清理（如果超过限制）
	currentSize := rm.calculateDirSize(config.Path)
	if config.MaxSize > 0 && currentSize > config.MaxSize {
		// 重新收集剩余文件
		files, _ = rm.collectFiles(config.Path, config.Category)
		sort.Slice(files, func(i, j int) bool {
			return files[i].ModTime.Before(files[j].ModTime)
		})

		for _, file := range files {
			if currentSize <= config.MaxSize {
				break
			}

			select {
			case <-ctx.Done():
				return cleaned, archived, ctx.Err()
			default:
			}

			if config.ArchiveEnabled && rm.policy.ArchiveEnabled {
				size, err := rm.archiveFile(file, config)
				if err != nil {
					if err := os.Remove(file.Path); err != nil {
						continue
					}
					size = file.Size
				}
				archived += size
			} else {
				if err := os.Remove(file.Path); err != nil {
					continue
				}
			}
			cleaned += file.Size
			currentSize -= file.Size
		}
	}

	return cleaned, archived, nil
}

// collectFiles 收集目录中的所有文件
func (rm *RetentionManager) collectFiles(dir string, category DataCategory) ([]*FileInfo, error) {
	var files []*FileInfo

	err := filepath.Walk(dir, func(path string, info fs.FileInfo, err error) error {
		if err != nil {
			return nil // 跳过无法访问的文件
		}

		if info.IsDir() {
			return nil
		}

		// 跳过临时文件和归档文件
		if strings.HasSuffix(path, ".tmp") ||
			strings.HasSuffix(path, ".archiving") ||
			strings.Contains(path, "/archive/") {
			return nil
		}

		files = append(files, &FileInfo{
			Path:     path,
			Size:     info.Size(),
			ModTime:  info.ModTime(),
			Category: category,
		})

		return nil
	})

	return files, err
}

// calculateDirSize 计算目录大小
func (rm *RetentionManager) calculateDirSize(dir string) int64 {
	var total int64

	filepath.Walk(dir, func(path string, info fs.FileInfo, err error) error {
		if err != nil {
			return nil
		}
		if !info.IsDir() {
			total += info.Size()
		}
		return nil
	})

	return total
}

// archiveFile 归档文件（支持压缩）
func (rm *RetentionManager) archiveFile(file *FileInfo, config *CategoryConfig) (int64, error) {
	// 创建归档目录
	archiveDir := filepath.Join(rm.policy.ArchivePath, string(config.Category))
	if err := os.MkdirAll(archiveDir, 0755); err != nil {
		return 0, fmt.Errorf("failed to create archive directory: %w", err)
	}

	// 按日期组织归档
	dateDir := filepath.Join(archiveDir, file.ModTime.Format("2006-01-02"))
	if err := os.MkdirAll(dateDir, 0755); err != nil {
		return 0, fmt.Errorf("failed to create date directory: %w", err)
	}

	filename := filepath.Base(file.Path)
	archivePath := filepath.Join(dateDir, filename)

	// 如果文件已存在，添加序号
	counter := 1
	originalPath := archivePath
	for {
		if _, err := os.Stat(archivePath); os.IsNotExist(err) {
			break
		}
		ext := filepath.Ext(originalPath)
		base := strings.TrimSuffix(originalPath, ext)
		archivePath = fmt.Sprintf("%s_%d%s", base, counter, ext)
		counter++
	}

	// 如果需要压缩
	if config.Compress && rm.policy.CompressionEnabled {
		return rm.archiveWithCompression(file, archivePath)
	}

	// 移动文件到归档（无压缩）
	if err := os.Rename(file.Path, archivePath); err != nil {
		// 如果跨设备移动失败，尝试复制后删除
		if err := rm.copyAndRemove(file.Path, archivePath); err != nil {
			return 0, fmt.Errorf("failed to archive file: %w", err)
		}
	}

	return file.Size, nil
}

// archiveWithCompression 压缩归档文件
func (rm *RetentionManager) archiveWithCompression(file *FileInfo, archivePath string) (int64, error) {
	// 添加压缩扩展名
	compressedPath := archivePath + ".gz"

	// 如果压缩文件已存在，添加序号
	counter := 1
	basePath := compressedPath
	for {
		if _, err := os.Stat(compressedPath); os.IsNotExist(err) {
			break
		}
		compressedPath = fmt.Sprintf("%s_%d.gz", strings.TrimSuffix(basePath, ".gz"), counter)
		counter++
	}

	// 创建压缩配置
	compConfig := &CompressionConfig{
		Enabled: true,
		Level:   CompressionLevel(rm.policy.CompressionLevel),
	}

	// 读取并压缩文件
	data, err := os.ReadFile(file.Path)
	if err != nil {
		return 0, fmt.Errorf("failed to read file: %w", err)
	}

	// 创建压缩器
	factory := NewCompressorFactory(compConfig)
	compressor := factory.GetCompressor()

	// 压缩数据
	compressed, err := compressor.Compress(data)
	if err != nil {
		logger.Warnf("Compression failed for %s, storing uncompressed: %v", file.Path, err)
		// 压缩失败则存储原文件
		if err := os.Rename(file.Path, archivePath); err != nil {
			return 0, fmt.Errorf("failed to archive uncompressed: %w", err)
		}
		return file.Size, nil
	}

	// 检查压缩率
	origSize := int64(len(data))
	compSize := int64(len(compressed))
	ratio := float64(compSize) / float64(origSize)

	if ratio > 0.8 {
		// 压缩率太低，存储原文件
		logger.Debugf("Compression ratio %.2f too low for %s, storing uncompressed", ratio, file.Path)
		if err := os.Rename(file.Path, archivePath); err != nil {
			return 0, fmt.Errorf("failed to archive uncompressed: %w", err)
		}
		return origSize, nil
	}

	// 写入压缩文件
	if err := os.WriteFile(compressedPath, compressed, 0644); err != nil {
		return 0, fmt.Errorf("failed to write compressed file: %w", err)
	}

	// 删除原文件
	if err := os.Remove(file.Path); err != nil {
		logger.Warnf("Failed to remove original file %s: %v", file.Path, err)
	}

	logger.Infof("Compressed %s: %d -> %d bytes (%.1f%%)", file.Path, origSize, compSize, ratio*100)

	return compSize, nil
}

// copyAndRemove 复制文件后删除原文件
func (rm *RetentionManager) copyAndRemove(src, dst string) error {
	data, err := os.ReadFile(src)
	if err != nil {
		return err
	}

	if err := os.WriteFile(dst, data, 0644); err != nil {
		return err
	}

	return os.Remove(src)
}

// cleanupArchive 清理过期归档
func (rm *RetentionManager) cleanupArchive(ctx context.Context) error {
	if !rm.policy.ArchiveEnabled {
		return nil
	}

	archivePath := rm.policy.ArchivePath
	if _, err := os.Stat(archivePath); os.IsNotExist(err) {
		return nil
	}

	cutoffTime := time.Now().Add(-rm.policy.ArchiveRetention)

	return filepath.Walk(archivePath, func(path string, info fs.FileInfo, err error) error {
		if err != nil {
			return nil
		}

		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		// 检查目录名是否为日期格式
		if info.IsDir() && path != archivePath {
			date, err := time.Parse("2006-01-02", filepath.Base(path))
			if err == nil && date.Before(cutoffTime) {
				logger.Infof("Removing expired archive directory: %s", path)
				if err := os.RemoveAll(path); err != nil {
					logger.Warnf("Failed to remove archive directory %s: %v", path, err)
				}
				return filepath.SkipDir
			}
		}

		return nil
	})
}

// checkDiskSpace 检查磁盘空间
func (rm *RetentionManager) checkDiskSpace(ctx context.Context) error {
	// 获取数据目录的磁盘使用情况
	var stat syscall.Statfs_t

	// 使用第一个类别的路径作为基准
	rm.mu.RLock()
	var basePath string
	for _, cat := range rm.categories {
		basePath = cat.Path
		break
	}
	rm.mu.RUnlock()

	if basePath == "" {
		return nil
	}

	if err := syscall.Statfs(basePath, &stat); err != nil {
		return fmt.Errorf("failed to get disk stats: %w", err)
	}

	// 计算可用空间
	freeSpace := int64(stat.Bavail) * int64(stat.Bsize)
	totalSpace := int64(stat.Blocks) * int64(stat.Bsize)
	usedSpace := totalSpace - freeSpace

	rm.mu.Lock()
	rm.stats.CurrentDiskUsage = usedSpace
	rm.stats.CurrentFreeSpace = freeSpace
	rm.mu.Unlock()

	// 如果可用空间低于阈值，触发紧急清理
	if freeSpace < rm.policy.MinFreeSpace {
		logger.Warnf("Low disk space detected: %d GB free, triggering emergency cleanup",
			freeSpace/(1024*1024*1024))
		return rm.emergencyCleanup(ctx)
	}

	return nil
}

// emergencyCleanup 紧急清理
func (rm *RetentionManager) emergencyCleanup(ctx context.Context) error {
	logger.Warn("Starting emergency cleanup due to low disk space")

	rm.mu.RLock()
	categories := make([]*CategoryConfig, 0, len(rm.categories))
	for _, cat := range rm.categories {
		categories = append(categories, cat)
	}
	rm.mu.RUnlock()

	// 按优先级排序（先清理大的、旧的）
	sort.Slice(categories, func(i, j int) bool {
		// PCAP > Metrics > Traces > Database > Logs > Profiles > Events
		priority := map[DataCategory]int{
			CategoryPCAP:     1,
			CategoryMetrics:  2,
			CategoryTraces:   3,
			CategoryDatabase: 4,
			CategoryLogs:     5,
			CategoryProfiles: 6,
			CategoryEvents:   7,
		}
		return priority[categories[i].Category] < priority[categories[j].Category]
	})

	targetFree := rm.policy.MinFreeSpace + 5*1024*1024*1024 // 目标：最小空间 + 5GB缓冲

	for _, cat := range categories {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		var stat syscall.Statfs_t
		syscall.Statfs(cat.Path, &stat)
		freeSpace := int64(stat.Bavail) * int64(stat.Bsize)

		if freeSpace >= targetFree {
			break
		}

		files, err := rm.collectFiles(cat.Path, cat.Category)
		if err != nil {
			continue
		}

		// 按时间排序，优先删除最旧的
		sort.Slice(files, func(i, j int) bool {
			return files[i].ModTime.Before(files[j].ModTime)
		})

		// 删除50%最旧的文件
		deleteCount := len(files) / 2
		for i := 0; i < deleteCount && i < len(files); i++ {
			if err := os.Remove(files[i].Path); err != nil {
				logger.Warnf("Failed to remove file %s: %v", files[i].Path, err)
			} else {
				rm.mu.Lock()
				rm.stats.TotalCleanedBytes += uint64(files[i].Size)
				rm.stats.TotalCleanedFiles++
				rm.mu.Unlock()
			}
		}
	}

	return nil
}

// GetStats 获取统计信息
func (rm *RetentionManager) GetStats() RetentionStats {
	rm.mu.RLock()
	defer rm.mu.RUnlock()
	return rm.stats
}

// GetDiskUsage 获取磁盘使用情况
func (rm *RetentionManager) GetDiskUsage() (used, free, total int64, usagePercent float64) {
	var stat syscall.Statfs_t

	rm.mu.RLock()
	var basePath string
	for _, cat := range rm.categories {
		basePath = cat.Path
		break
	}
	rm.mu.RUnlock()

	if basePath == "" {
		return 0, 0, 0, 0
	}

	if err := syscall.Statfs(basePath, &stat); err != nil {
		return 0, 0, 0, 0
	}

	blockSize := int64(stat.Bsize)
	total = int64(stat.Blocks) * blockSize
	free = int64(stat.Bavail) * blockSize
	used = total - free

	if total > 0 {
		usagePercent = float64(used) / float64(total) * 100
	}

	return
}

// ForceCleanup 强制清理指定类别的数据
func (rm *RetentionManager) ForceCleanup(category DataCategory, keepLast time.Duration) error {
	rm.mu.RLock()
	config, exists := rm.categories[category]
	rm.mu.RUnlock()

	if !exists {
		return fmt.Errorf("category %s not found", category)
	}

	files, err := rm.collectFiles(config.Path, category)
	if err != nil {
		return err
	}

	cutoffTime := time.Now().Add(-keepLast)
	var cleaned int64

	for _, file := range files {
		if file.ModTime.Before(cutoffTime) {
			if err := os.Remove(file.Path); err != nil {
				logger.Warnf("Failed to remove file %s: %v", file.Path, err)
			} else {
				cleaned += file.Size
			}
		}
	}

	logger.Infof("Force cleanup category %s: removed %d bytes", category, cleaned)
	return nil
}
