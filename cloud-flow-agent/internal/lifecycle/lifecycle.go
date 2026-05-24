// Package lifecycle 提供监控数据生命周期管理
// 本文件实现核心生命周期管理器，负责数据保留策略、过期数据清理与资源回收
// 支持：默认60天保留、自定义保留周期、凌晨自动清理、清理与查询隔离
package lifecycle

import (
	"context"
	"fmt"
	"sync"
	"time"
)

// ==================== 数据分类定义 ====================

// DataCategory 数据分类（用于精细化的保留策略）
type DataCategory string

const (
	// 通用分类
	CategoryMetric  DataCategory = "metric"   // 结构化指标数据
	CategoryLog     DataCategory = "log"      // 日志数据
	CategoryTrace   DataCategory = "trace"    // 链路追踪数据
	CategoryEvent   DataCategory = "event"    // 事件数据

	// 细分分类
	CategorySystemMetric   DataCategory = "system_metric"    // 系统指标（CPU/内存/磁盘/网络）
	CategoryAppMetric      DataCategory = "app_metric"       // 应用指标（QPS/延迟/错误率）
	CategoryDBMetric       DataCategory = "db_metric"        // 数据库指标（慢查询/连接数）
	CategorySQLAggregate   DataCategory = "sql_aggregate"    // SQL 聚合数据
	CategoryProfiling      DataCategory = "profiling"        // 性能剖析数据
	CategoryAlert          DataCategory = "alert"            // 告警数据
	CategoryTopology       DataCategory = "topology"         // 拓扑数据
	CategorySelfMonitor    DataCategory = "self_monitor"     // 自监控数据
	CategoryCustom         DataCategory = "custom"           // 自定义数据
)

// ==================== 清理任务定义 ====================

// CleanupTask 清理任务
type CleanupTask struct {
	ID            string        `json:"id"`              // 任务 ID
	Category      DataCategory  `json:"category"`        // 数据分类
	Source        string        `json:"source"`          // 数据源（可选）
	RetentionDays int           `json:"retention_days"`  // 保留天数
	CutoffTime    time.Time     `json:"cutoff_time"`     // 截止时间
	Status        TaskStatus    `json:"status"`          // 任务状态
	StartTime     time.Time     `json:"start_time"`      // 开始时间
	EndTime       time.Time     `json:"end_time"`        // 结束时间
	ScannedCount  int64         `json:"scanned_count"`   // 扫描条数
	DeletedCount  int64         `json:"deleted_count"`   // 删除条数
	DeletedBytes  int64         `json:"deleted_bytes"`   // 释放字节数
	Error         string        `json:"error,omitempty"` // 错误信息
	Duration      time.Duration `json:"duration"`        // 执行时长
}

// TaskStatus 任务状态
type TaskStatus string

const (
	TaskStatusPending   TaskStatus = "pending"    // 等待执行
	TaskStatusRunning   TaskStatus = "running"    // 执行中
	TaskStatusCompleted TaskStatus = "completed"  // 已完成
	TaskStatusFailed    TaskStatus = "failed"     // 执行失败
	TaskStatusSkipped   TaskStatus = "skipped"    // 跳过（无过期数据）
)

// ==================== 清理统计 ====================

// CleanupStats 清理统计
type CleanupStats struct {
	// 总体统计
	TotalTasks      int           `json:"total_tasks"`       // 总任务数
	SuccessTasks    int           `json:"success_tasks"`     // 成功任务数
	FailedTasks     int           `json:"failed_tasks"`      // 失败任务数
	SkippedTasks    int           `json:"skipped_tasks"`     // 跳过任务数
	TotalDuration   time.Duration `json:"total_duration"`    // 总执行时长

	// 数据统计
	TotalScanned    int64 `json:"total_scanned"`    // 总扫描条数
	TotalDeleted    int64 `json:"total_deleted"`    // 总删除条数
	TotalBytesFreed int64 `json:"total_bytes_freed"` // 总释放字节数

	// 分类统计
	CategoryStats map[DataCategory]*CategoryCleanupStat `json:"category_stats"`

	// 时间信息
	LastCleanupTime time.Time `json:"last_cleanup_time"` // 上次清理时间
	NextCleanupTime time.Time `json:"next_cleanup_time"` // 下次清理时间
}

// CategoryCleanupStat 分类清理统计
type CategoryCleanupStat struct {
	Category      DataCategory `json:"category"`
	RetentionDays int          `json:"retention_days"`
	ScannedCount  int64        `json:"scanned_count"`
	DeletedCount  int64        `json:"deleted_count"`
	DeletedBytes  int64        `json:"deleted_bytes"`
	Duration      time.Duration `json:"duration"`
}

// ==================== 数据扫描器接口 ====================

// DataScanner 数据扫描器接口
// 实现此接口以对接不同的存储后端
type DataScanner interface {
	// ScanExpired 扫描过期数据，通过回调返回待删除的数据标识
	// cutoffTime: 截止时间，早于此时间的数据视为过期
	// category: 数据分类
	// callback: 每批数据的回调，返回 false 可提前终止扫描
	ScanExpired(ctx context.Context, cutoffTime time.Time, category DataCategory, callback func(batch DataBatch) bool) error

	// DeleteBatch 删除一批数据
	DeleteBatch(ctx context.Context, batch DataBatch) (int64, int64, error)

	// GetCategoryStats 获取分类数据统计
	GetCategoryStats(ctx context.Context, category DataCategory) (*CategoryDataStats, error)
}

// DataBatch 数据批次（用于批量删除）
type DataBatch struct {
	IDs       []string // 数据标识列表（chunk ID / 文件路径等）
	Category  DataCategory
	StartTime int64 // 批次最早时间
	EndTime   int64 // 批次最晚时间
	Count     int64 // 数据条数
	Size      int64 // 数据大小（字节）
}

// CategoryDataStats 分类数据统计
type CategoryDataStats struct {
	Category     DataCategory `json:"category"`
	TotalCount   int64        `json:"total_count"`
	TotalSize    int64        `json:"total_size"`
	OldestTime   time.Time    `json:"oldest_time"`
	NewestTime   time.Time    `json:"newest_time"`
	ChunkCount   int          `json:"chunk_count"`
}

// ==================== 生命周期管理器 ====================

// LifecycleManager 生命周期管理器
type LifecycleManager struct {
	config    *LifecycleConfig
	policies  *RetentionPolicyManager
	scheduler *CleanupScheduler

	// 数据扫描器
	scanner DataScanner

	// 清理历史
	history    []*CleanupTask
	historyMu  sync.RWMutex
	maxHistory int

	// 运行状态
	running   bool
	mu        sync.RWMutex
	stopCh    chan struct{}
	wg        sync.WaitGroup

	// 回调
	onCleanupStart func(task *CleanupTask)
	onCleanupEnd   func(task *CleanupTask)
	onStatsUpdate  func(stats *CleanupStats)
}

// NewLifecycleManager 创建生命周期管理器
func NewLifecycleManager(cfg *LifecycleConfig) *LifecycleManager {
	if cfg == nil {
		cfg = DefaultLifecycleConfig()
	}

	mgr := &LifecycleManager{
		config:     cfg,
		policies:   NewRetentionPolicyManager(cfg),
		scheduler:  NewCleanupScheduler(cfg),
		history:    make([]*CleanupTask, 0),
		maxHistory: cfg.MaxHistoryRecords,
		stopCh:     make(chan struct{}),
	}

	// 设置清理执行器
	mgr.scheduler.SetExecutor(mgr.executeCleanup)

	return mgr
}

// SetScanner 设置数据扫描器
func (m *LifecycleManager) SetScanner(scanner DataScanner) {
	m.scanner = scanner
}

// SetCleanupStartCallback 设置清理开始回调
func (m *LifecycleManager) SetCleanupStartCallback(callback func(task *CleanupTask)) {
	m.onCleanupStart = callback
}

// SetCleanupEndCallback 设置清理结束回调
func (m *LifecycleManager) SetCleanupEndCallback(callback func(task *CleanupTask)) {
	m.onCleanupEnd = callback
}

// SetStatsUpdateCallback 设置统计更新回调
func (m *LifecycleManager) SetStatsUpdateCallback(callback func(stats *CleanupStats)) {
	m.onStatsUpdate = callback
}

// Start 启动生命周期管理器
func (m *LifecycleManager) Start(ctx context.Context) error {
	m.mu.Lock()
	if m.running {
		m.mu.Unlock()
		return fmt.Errorf("生命周期管理器已在运行")
	}
	m.running = true
	m.mu.Unlock()

	// 启动定时调度器
	if err := m.scheduler.Start(ctx); err != nil {
		return fmt.Errorf("启动调度器失败: %w", err)
	}

	return nil
}

// Stop 停止生命周期管理器
func (m *LifecycleManager) Stop() {
	m.mu.Lock()
	if !m.running {
		m.mu.Unlock()
		return
	}
	m.running = false
	close(m.stopCh)
	m.mu.Unlock()

	m.scheduler.Stop()
	m.wg.Wait()
}

// executeCleanup 执行清理任务（由调度器调用）
func (m *LifecycleManager) executeCleanup(ctx context.Context, category DataCategory) (*CleanupTask, error) {
	if m.scanner == nil {
		return nil, fmt.Errorf("数据扫描器未设置")
	}

	// 获取保留策略
	policy := m.policies.GetPolicy(category)
	if policy == nil || !policy.Enabled {
		return nil, nil // 无策略或未启用，跳过
	}

	// 创建清理任务
	task := &CleanupTask{
		ID:            generateTaskID(),
		Category:      category,
		RetentionDays: policy.RetentionDays,
		CutoffTime:    time.Now().AddDate(0, 0, -policy.RetentionDays),
		Status:        TaskStatusPending,
	}

	// 通知清理开始
	if m.onCleanupStart != nil {
		m.onCleanupStart(task)
	}

	// 执行清理
	task.Status = TaskStatusRunning
	task.StartTime = time.Now()

	err := m.doCleanup(ctx, task)

	task.EndTime = time.Now()
	task.Duration = task.EndTime.Sub(task.StartTime)

	if err != nil {
		task.Status = TaskStatusFailed
		task.Error = err.Error()
	} else if task.DeletedCount == 0 {
		task.Status = TaskStatusSkipped
	} else {
		task.Status = TaskStatusCompleted
	}

	// 记录历史
	m.recordHistory(task)

	// 通知清理结束
	if m.onCleanupEnd != nil {
		m.onCleanupEnd(task)
	}

	// 更新统计
	if m.onStatsUpdate != nil {
		stats := m.GetStats()
		m.onStatsUpdate(stats)
	}

	return task, err
}

// doCleanup 执行实际清理逻辑
func (m *LifecycleManager) doCleanup(ctx context.Context, task *CleanupTask) error {
	// 检查上下文是否已取消
	select {
	case <-ctx.Done():
		return ctx.Err()
	default:
	}

	// 扫描过期数据并批量删除
	err := m.scanner.ScanExpired(ctx, task.CutoffTime, task.Category, func(batch DataBatch) bool {
		// 检查上下文
		select {
		case <-ctx.Done():
			return false
		default:
		}

		// 执行批量删除
		deleted, bytesFreed, delErr := m.scanner.DeleteBatch(ctx, batch)
		if delErr != nil {
			task.Error = delErr.Error()
			// 记录错误但继续清理
			return true
		}

		task.ScannedCount += batch.Count
		task.DeletedCount += deleted
		task.DeletedBytes += bytesFreed

		return true // 继续扫描
	})

	return err
}

// ManualCleanup 手动触发清理
func (m *LifecycleManager) ManualCleanup(ctx context.Context, categories ...DataCategory) ([]*CleanupTask, error) {
	if len(categories) == 0 {
		categories = m.policies.GetAllCategories()
	}

	tasks := make([]*CleanupTask, 0, len(categories))
	for _, cat := range categories {
		task, err := m.executeCleanup(ctx, cat)
		if err != nil {
			return tasks, fmt.Errorf("清理 %s 失败: %w", cat, err)
		}
		if task != nil {
			tasks = append(tasks, task)
		}
	}

	return tasks, nil
}

// GetStats 获取清理统计
func (m *LifecycleManager) GetStats() *CleanupStats {
	stats := &CleanupStats{
		CategoryStats: make(map[DataCategory]*CategoryCleanupStat),
	}

	m.historyMu.RLock()
	defer m.historyMu.RUnlock()

	for _, task := range m.history {
		switch task.Status {
		case TaskStatusCompleted:
			stats.SuccessTasks++
		case TaskStatusFailed:
			stats.FailedTasks++
		case TaskStatusSkipped:
			stats.SkippedTasks++
		}
		stats.TotalTasks++
		stats.TotalDuration += task.Duration
		stats.TotalScanned += task.ScannedCount
		stats.TotalDeleted += task.DeletedCount
		stats.TotalBytesFreed += task.DeletedBytes

		// 分类统计
		catStat, exists := stats.CategoryStats[task.Category]
		if !exists {
			catStat = &CategoryCleanupStat{
				Category:      task.Category,
				RetentionDays: task.RetentionDays,
			}
			stats.CategoryStats[task.Category] = catStat
		}
		catStat.ScannedCount += task.ScannedCount
		catStat.DeletedCount += task.DeletedCount
		catStat.DeletedBytes += task.DeletedBytes
		catStat.Duration += task.Duration
	}

	// 设置时间信息
	if len(m.history) > 0 {
		lastTask := m.history[len(m.history)-1]
		stats.LastCleanupTime = lastTask.StartTime
	}
	stats.NextCleanupTime = m.scheduler.GetNextCleanupTime()

	return stats
}

// GetHistory 获取清理历史
func (m *LifecycleManager) GetHistory(limit int) []*CleanupTask {
	m.historyMu.RLock()
	defer m.historyMu.RUnlock()

	if limit <= 0 || limit > len(m.history) {
		limit = len(m.history)
	}

	// 返回最近的记录（倒序）
	result := make([]*CleanupTask, limit)
	for i := 0; i < limit; i++ {
		result[i] = m.history[len(m.history)-1-i]
	}

	return result
}

// recordHistory 记录清理历史
func (m *LifecycleManager) recordHistory(task *CleanupTask) {
	m.historyMu.Lock()
	defer m.historyMu.Unlock()

	m.history = append(m.history, task)

	// 限制历史记录数量
	if len(m.history) > m.maxHistory {
		m.history = m.history[len(m.history)-m.maxHistory:]
	}
}

// ==================== 工具函数 ====================

// generateTaskID 生成任务 ID
func generateTaskID() string {
	return fmt.Sprintf("cleanup-%d", time.Now().UnixNano())
}
