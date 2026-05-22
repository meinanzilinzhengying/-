// Package selfmonitor 采集器自监控
// 心跳/状态/资源占用上报，中心端感知探针异常
package selfmonitor

import (
	"encoding/json"
	"fmt"
	"os"
	"runtime"
	"sync"
	"sync/atomic"
	"time"
)

// ============================================================
// 探针状态
// ============================================================

// ProbeStatus 探针状态
type ProbeStatus string

const (
	StatusInitializing ProbeStatus = "initializing"
	StatusRunning      ProbeStatus = "running"
	StatusDegraded     ProbeStatus = "degraded"
	StatusError       ProbeStatus = "error"
	StatusStopped      ProbeStatus = "stopped"
)

// ============================================================
// 自监控配置
// ============================================================

// Config 自监控配置
type Config struct {
	Enabled        bool `yaml:"enabled" json:"enabled"`
	IntervalSec   int  `yaml:"interval_sec" json:"interval_sec"`     // 上报间隔（秒）
	HeartbeatEnabled bool `yaml:"heartbeat_enabled" json:"heartbeat_enabled"` // 启用心跳
	StatusEnabled    bool `yaml:"status_enabled" json:"status_enabled"`       // 启用状态上报
	ResourceEnabled  bool `yaml:"resource_enabled" json:"resource_enabled"`   // 启用资源上报
	HistoryEnabled   bool `yaml:"history_enabled" json:"history_enabled"`     // 启用历史记录
	
	// 上报目标
	ReportEndpoint string `yaml:"report_endpoint" json:"report_endpoint"`
	ReportTimeout  int    `yaml:"report_timeout" json:"report_timeout"` // 超时（秒）
	
	// 告警阈值
	CPUWarningThreshold float64 `yaml:"cpu_warning_threshold" json:"cpu_warning_threshold"`
	MemoryWarningThreshold float64 `yaml:"memory_warning_threshold" json:"memory_warning_threshold"`
	DiskWarningThreshold float64 `yaml:"disk_warning_threshold" json:"disk_warning_threshold"`
}

// DefaultConfig 默认配置
func DefaultConfig() *Config {
	return &Config{
		Enabled:         true,
		IntervalSec:    30,
		HeartbeatEnabled: true,
		StatusEnabled:    true,
		ResourceEnabled:  true,
		HistoryEnabled:   true,
		ReportTimeout:   10,
		CPUWarningThreshold: 70.0,
		MemoryWarningThreshold: 80.0,
		DiskWarningThreshold: 85.0,
	}
}

// ============================================================
// 自监控器
// ============================================================

// Monitor 自监控器
type Monitor struct {
	config    *Config
	probeID   string
	probeName string
	
	// 状态
	status    ProbeStatus
	startTime time.Time
	uptime    int64 // 秒
	
	// 统计
	stats *Stats
	
	// 资源使用
	resource *ResourceUsage
	
	// 心跳
	heartbeatSeq uint64
	
	// 上报回调
	reportFunc func(report *ProbeReport) error
	
	// 告警回调
	alertFunc func(alert *ProbeAlert)
	
	// 历史记录
	history    []*ProbeReport
	historyMu sync.RWMutex
	maxHistory int
	
	// 运行控制
	stopCh chan struct{}
	wg     sync.WaitGroup
}

// Stats 探针统计
type Stats struct {
	// 数据采集统计
	PacketsCollected  int64 `json:"packets_collected"`
	PacketsDropped     int64 `json:"packets_dropped"`
	BytesCollected    int64 `json:"bytes_collected"`
	Errors            int64 `json:"errors"`
	
	// 协议统计
	HTTPRequests     int64 `json:"http_requests"`
	DNSPackets        int64 `json:"dns_packets"`
	MySQLPackets      int64 `json:"mysql_packets"`
	
	// 传输统计
	PacketsSent       int64 `json:"packets_sent"`
	PacketsAcked      int64 `json:"packets_acked"`
	PacketsRetried    int64 `json:"packets_retried"`
	BytesSent         int64 `json:"bytes_sent"`
	
	// 健康统计
	SuccessRate       float64 `json:"success_rate"`
	AvgLatencyMs      float64 `json:"avg_latency_ms"`
	LastError         string  `json:"last_error,omitempty"`
}

// ResourceUsage 资源使用
type ResourceUsage struct {
	// CPU
	CPUPercent float64 `json:"cpu_percent"`
	CPUCores   int      `json:"cpu_cores"`
	
	// 内存
	MemoryUsedMB   uint64 `json:"memory_used_mb"`
	MemoryLimitMB  uint64 `json:"memory_limit_mb"`
	MemoryPercent   float64 `json:"memory_percent"`
	MemoryRssMB     uint64 `json:"memory_rss_mb"`
	MemoryHeapMB    uint64 `json:"memory_heap_mb"`
	MemoryStackMB   uint64 `json:"memory_stack_mb"`
	
	// Goroutine
	Goroutines int `json:"goroutines"`
	
	// 磁盘
	DiskUsedPercent float64 `json:"disk_used_percent"`
	DiskAvailableMB uint64 `json:"disk_available_mb"`
	
	// GC
	GCNum uint32 `json:"gc_num"`
	GCpauseMs float64 `json:"gc_pause_ms"`
	
	// 网络
	NetConnCount int `json:"net_conn_count"`
	
	// 采集器特有
	BufferUsagePercent float64 `json:"buffer_usage_percent"`
	QueueDepth         int     `json:"queue_depth"`
}

// ProbeReport 探针报告
type ProbeReport struct {
	// 基本信息
	Timestamp     time.Time `json:"timestamp"`
	ProbeID       string    `json:"probe_id"`
	ProbeName     string    `json:"probe_name"`
	Version       string    `json:"version"`
	Uptime        int64     `json:"uptime_sec"`
	
	// 状态
	Status        ProbeStatus `json:"status"`
	StatusReason string      `json:"status_reason,omitempty"`
	
	// 心跳序列
	HeartbeatSeq uint64 `json:"heartbeat_seq"`
	
	// 统计
	Stats *Stats `json:"stats"`
	
	// 资源
	Resource *ResourceUsage `json:"resource"`
	
	// 系统信息
	System *SystemInfo `json:"system"`
}

// SystemInfo 系统信息
type SystemInfo struct {
	Hostname    string `json:"hostname"`
	OS          string `json:"os"`
	Arch        string `json:"arch"`
	Kernel      string `json:"kernel"`
	GoVersion   string `json:"go_version"`
	ProcessID   int    `json:"process_id"`
}

// ProbeAlert 探针告警
type ProbeAlert struct {
	Timestamp  time.Time    `json:"timestamp"`
	ProbeID    string      `json:"probe_id"`
	AlertType  string      `json:"alert_type"`  // resource/high_error/critical
	Severity   string      `json:"severity"`     // warning/critical
	Message    string      `json:"message"`
	Metric     string      `json:"metric,omitempty"`
	Value      float64     `json:"value,omitempty"`
	Threshold  float64     `json:"threshold,omitempty"`
}

// NewMonitor 创建自监控器
func NewMonitor(probeID, probeName string, config *Config) *Monitor {
	if config == nil {
		config = DefaultConfig()
	}
	
	m := &Monitor{
		config:     config,
		probeID:   probeID,
		probeName: probeName,
		status:    StatusInitializing,
		startTime: time.Now(),
		stats:     &Stats{},
		resource:  &ResourceUsage{},
		maxHistory: 100,
		history:   make([]*ProbeReport, 0, 100),
		stopCh:    make(chan struct{}),
	}
	
	return m
}

// SetReportFunc 设置上报函数
func (m *Monitor) SetReportFunc(f func(report *ProbeReport) error) {
	m.reportFunc = f
}

// SetAlertFunc 设置告警函数
func (m *Monitor) SetAlertFunc(f func(alert *ProbeAlert)) {
	m.alertFunc = f
}

// Start 启动自监控
func (m *Monitor) Start() {
	if !m.config.Enabled {
		return
	}
	
	m.status = StatusRunning
	
	// 启动上报循环
	if m.config.HeartbeatEnabled || m.config.StatusEnabled || m.config.ResourceEnabled {
		m.wg.Add(1)
		go m.reportLoop()
	}
	
	// 启动资源监控
	if m.config.ResourceEnabled {
		m.wg.Add(1)
		go m.resourceLoop()
	}
}

// Stop 停止自监控
func (m *Monitor) Stop() {
	m.status = StatusStopped
	close(m.stopCh)
	m.wg.Wait()
}

// reportLoop 上报循环
func (m *Monitor) reportLoop() {
	defer m.wg.Done()
	
	ticker := time.NewTicker(time.Duration(m.config.IntervalSec) * time.Second)
	defer ticker.Stop()
	
	for {
		select {
		case <-ticker.C:
			report := m.GenerateReport()
			
			// 存储历史
			if m.config.HistoryEnabled {
				m.addToHistory(report)
			}
			
			// 上报
			if m.reportFunc != nil {
				if err := m.reportFunc(report); err != nil {
					m.stats.Errors++
					m.stats.LastError = err.Error()
				}
			}
			
			// 检查告警
			m.checkAlerts(report)
			
		case <-m.stopCh:
			return
		}
	}
}

// resourceLoop 资源监控循环
func (m *Monitor) resourceLoop() {
	defer m.wg.Done()
	
	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()
	
	for {
		select {
		case <-ticker.C:
			m.collectResource()
			
		case <-m.stopCh:
			return
		}
	}
}

// GenerateReport 生成报告
func (m *Monitor) GenerateReport() *ProbeReport {
	m.uptime = int64(time.Since(m.startTime).Seconds())
	
	report := &ProbeReport{
		Timestamp:     time.Now(),
		ProbeID:       m.probeID,
		ProbeName:     m.probeName,
		Version:       "1.0.0",
		Uptime:        m.uptime,
		Status:        m.status,
		HeartbeatSeq:  atomic.AddUint64(&m.heartbeatSeq, 1),
		Stats:         m.getStatsCopy(),
		Resource:      m.getResourceCopy(),
		System:        m.getSystemInfo(),
	}
	
	return report
}

// collectResource 采集资源使用
func (m *Monitor) collectResource() {
	var memStats runtime.MemStats
	runtime.ReadMemStats(&memStats)
	
	m.resource.MemoryHeapMB = memStats.Alloc / 1024 / 1024
	m.resource.MemoryStackMB = memStats.StackInuse / 1024 / 1024
	m.resource.GCNum = memStats.NumGC
	m.resource.GCpauseMs = float64(memStats.PauseTotalNs) / 1000000
	
	// 估算内存使用
	totalMem := uint64(memStats.Alloc + memStats.StackInuse + memStats.MSpanInuse + memStats.MCacheInuse)
	m.resource.MemoryUsedMB = totalMem / 1024 / 1024
	
	// GC暂停时间（最近一次）
	if memStats.NumGC > 0 {
		lastGC := memStats.PauseEnd[memStats.NumGC%256]
		pause := time.Duration(lastGC).Seconds() * 1000
		m.resource.GCpauseMs = pause
	}
	
	// CPU（简化实现）
	m.resource.CPUPercent = m.estimateCPU()
	m.resource.CPUCores = runtime.NumCPU()
	
	// 磁盘
	m.collectDiskUsage()
}

// estimateCPU 估算CPU使用
func (m *Monitor) estimateCPU() float64 {
	// 简化实现，实际应读取/proc/stat
	return 0.0
}

// collectDiskUsage 采集磁盘使用
func (m *Monitor) collectDiskUsage() {
	// 简化实现
	m.resource.DiskUsedPercent = 0
	m.resource.DiskAvailableMB = 0
}

// checkAlerts 检查告警
func (m *Monitor) checkAlerts(report *ProbeReport) {
	if m.alertFunc == nil {
		return
	}
	
	// CPU告警
	if report.Resource.CPUPercent > m.config.CPUWarningThreshold {
		m.alertFunc(&ProbeAlert{
			Timestamp: time.Now(),
			ProbeID:  m.probeID,
			AlertType: "resource",
			Severity: "warning",
			Message:  fmt.Sprintf("CPU使用率 %.1f%% 超过阈值 %.1f%%", 
				report.Resource.CPUPercent, m.config.CPUWarningThreshold),
			Metric:    "cpu_percent",
			Value:     report.Resource.CPUPercent,
			Threshold: m.config.CPUWarningThreshold,
		})
	}
	
	// 内存告警
	if report.Resource.MemoryPercent > m.config.MemoryWarningThreshold {
		m.alertFunc(&ProbeAlert{
			Timestamp: time.Now(),
			ProbeID:  m.probeID,
			AlertType: "resource",
			Severity: "warning",
			Message:  fmt.Sprintf("内存使用率 %.1f%% 超过阈值 %.1f%%", 
				report.Resource.MemoryPercent, m.config.MemoryWarningThreshold),
			Metric:    "memory_percent",
			Value:     report.Resource.MemoryPercent,
			Threshold: m.config.MemoryWarningThreshold,
		})
	}
	
	// 错误率告警
	if report.Stats.PacketsCollected > 0 {
		errorRate := float64(report.Stats.Errors) / float64(report.Stats.PacketsCollected) * 100
		if errorRate > 5 {
			m.alertFunc(&ProbeAlert{
				Timestamp: time.Now(),
				ProbeID:  m.probeID,
				AlertType: "high_error",
				Severity: "critical",
				Message:  fmt.Sprintf("错误率 %.2f%% 过高", errorRate),
				Metric:   "error_rate",
				Value:    errorRate,
			})
		}
	}
}

// GetStatus 获取当前状态
func (m *Monitor) GetStatus() ProbeStatus {
	return m.status
}

// SetStatus 设置状态
func (m *Monitor) SetStatus(status ProbeStatus, reason string) {
	m.status = status
	_ = reason // 可用于记录日志
}

// UpdateStats 更新统计
func (m *Monitor) UpdateStats(fn func(*Stats)) {
	fn(m.stats)
}

// GetHistory 获取历史报告
func (m *Monitor) GetHistory() []*ProbeReport {
	m.historyMu.RLock()
	defer m.historyMu.RUnlock()
	
	result := make([]*ProbeReport, len(m.history))
	copy(result, m.history)
	return result
}

func (m *Monitor) addToHistory(report *ProbeReport) {
	m.historyMu.Lock()
	defer m.historyMu.Unlock()
	
	m.history = append(m.history, report)
	if len(m.history) > m.maxHistory {
		m.history = m.history[len(m.history)-m.maxHistory:]
	}
}

func (m *Monitor) getStatsCopy() *Stats {
	return &Stats{
		PacketsCollected: atomic.LoadInt64(&m.stats.PacketsCollected),
		PacketsDropped:    atomic.LoadInt64(&m.stats.PacketsDropped),
		BytesCollected:    atomic.LoadInt64(&m.stats.BytesCollected),
		Errors:            atomic.LoadInt64(&m.stats.Errors),
		HTTPRequests:      atomic.LoadInt64(&m.stats.HTTPRequests),
		DNSPackets:        atomic.LoadInt64(&m.stats.DNSPackets),
		MySQLPackets:      atomic.LoadInt64(&m.stats.MySQLPackets),
		PacketsSent:       atomic.LoadInt64(&m.stats.PacketsSent),
		PacketsAcked:      atomic.LoadInt64(&m.stats.PacketsAcked),
		PacketsRetried:    atomic.LoadInt64(&m.stats.PacketsRetried),
		BytesSent:        atomic.LoadInt64(&m.stats.BytesSent),
		SuccessRate:      m.stats.SuccessRate,
		AvgLatencyMs:     m.stats.AvgLatencyMs,
		LastError:        m.stats.LastError,
	}
}

func (m *Monitor) getResourceCopy() *ResourceUsage {
	return &ResourceUsage{
		CPUPercent:         m.resource.CPUPercent,
		CPUCores:           m.resource.CPUCores,
		MemoryUsedMB:       m.resource.MemoryUsedMB,
		MemoryLimitMB:      m.resource.MemoryLimitMB,
		MemoryPercent:      m.resource.MemoryPercent,
		MemoryRssMB:       m.resource.MemoryRssMB,
		MemoryHeapMB:      m.resource.MemoryHeapMB,
		MemoryStackMB:     m.resource.MemoryStackMB,
		Goroutines:        m.resource.Goroutines,
		DiskUsedPercent:   m.resource.DiskUsedPercent,
		DiskAvailableMB:   m.resource.DiskAvailableMB,
		GCNum:             m.resource.GCNum,
		GCpauseMs:        m.resource.GCpauseMs,
		NetConnCount:      m.resource.NetConnCount,
		BufferUsagePercent: m.resource.BufferUsagePercent,
		QueueDepth:        m.resource.QueueDepth,
	}
}

func (m *Monitor) getSystemInfo() *SystemInfo {
	hostname, _ := os.Hostname()
	
	return &SystemInfo{
		Hostname:  hostname,
		OS:        runtime.GOOS,
		Arch:      runtime.GOARCH,
		Kernel:    "linux", // 简化
		GoVersion: runtime.Version(),
		ProcessID: os.Getpid(),
	}
}

// ToJSON 序列化为JSON
func (r *ProbeReport) ToJSON() ([]byte, error) {
	return json.MarshalIndent(r, "", "  ")
}
