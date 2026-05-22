// Package asset 提供资产下钻功能
// 支持单资产网络/应用指标采集与查询
package asset

import (
	"context"
	"fmt"
	"net"
	"os"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/cloud-flow-agent/internal/topology"
)

// AssetType 资产类型
type AssetType string

const (
	AssetTypePod       AssetType = "pod"
	AssetTypeVM        AssetType = "vm"
	AssetTypePhysical  AssetType = "physical"
	AssetTypeContainer AssetType = "container"
	AssetTypeService   AssetType = "service"
	AssetTypeNode      AssetType = "node"
)

// MetricType 指标类型
type MetricType string

const (
	// 网络指标
	MetricNetBytesSent     MetricType = "net_bytes_sent"
	MetricNetBytesRecv     MetricType = "net_bytes_recv"
	MetricNetPacketsSent   MetricType = "net_packets_sent"
	MetricNetPacketsRecv   MetricType = "net_packets_recv"
	MetricNetErrorsIn      MetricType = "net_errors_in"
	MetricNetErrorsOut     MetricType = "net_errors_out"
	MetricNetDropsIn       MetricType = "net_drops_in"
	MetricNetDropsOut      MetricType = "net_drops_out"
	MetricNetLatency       MetricType = "net_latency"
	MetricNetRetransmit    MetricType = "net_retransmit"
	MetricNetConnections   MetricType = "net_connections"
	MetricNetTCPStates     MetricType = "net_tcp_states"

	// 应用指标
	MetricAppCPUUsage      MetricType = "app_cpu_usage"
	MetricAppMemoryUsage   MetricType = "app_memory_usage"
	MetricAppMemoryRSS     MetricType = "app_memory_rss"
	MetricAppMemoryVMS     MetricType = "app_memory_vms"
	MetricAppOpenFiles     MetricType = "app_open_files"
	MetricAppThreads       MetricType = "app_threads"
	MetricAppResponseTime  MetricType = "app_response_time"
	MetricAppErrorRate     MetricType = "app_error_rate"
	MetricAppThroughput    MetricType = "app_throughput"
	MetricAppGCCount       MetricType = "app_gc_count"
	MetricAppGCTime        MetricType = "app_gc_time"

	// 系统指标
	MetricSysCPUUsage      MetricType = "sys_cpu_usage"
	MetricSysMemoryUsage   MetricType = "sys_memory_usage"
	MetricSysDiskUsage     MetricType = "sys_disk_usage"
	MetricSysLoad1         MetricType = "sys_load1"
	MetricSysLoad5         MetricType = "sys_load5"
	MetricSysLoad15        MetricType = "sys_load15"
)

// AssetMetrics 资产指标
type AssetMetrics struct {
	AssetID       string                 `json:"asset_id"`
	AssetType     AssetType              `json:"asset_type"`
	AssetName     string                 `json:"asset_name"`
	Timestamp     time.Time              `json:"timestamp"`
	Network       *NetworkMetrics        `json:"network,omitempty"`
	Application   *ApplicationMetrics    `json:"application,omitempty"`
	System        *SystemMetrics         `json:"system,omitempty"`
	Labels        map[string]string      `json:"labels"`
}

// NetworkMetrics 网络指标
type NetworkMetrics struct {
	BytesSent      uint64                 `json:"bytes_sent"`
	BytesRecv      uint64                 `json:"bytes_recv"`
	PacketsSent    uint64                 `json:"packets_sent"`
	PacketsRecv    uint64                 `json:"packets_recv"`
	ErrorsIn       uint64                 `json:"errors_in"`
	ErrorsOut      uint64                 `json:"errors_out"`
	DropsIn        uint64                 `json:"drops_in"`
	DropsOut       uint64                 `json:"drops_out"`
	Connections    int                    `json:"connections"`
	Retransmits    uint64                 `json:"retransmits"`
	LatencyMs      float64                `json:"latency_ms"`
	TCPStates      map[string]int         `json:"tcp_states"`
	Interfaces     map[string]*IfaceMetrics `json:"interfaces"`
}

// IfaceMetrics 接口指标
type IfaceMetrics struct {
	Name         string  `json:"name"`
	BytesSent    uint64  `json:"bytes_sent"`
	BytesRecv    uint64  `json:"bytes_recv"`
	PacketsSent  uint64  `json:"packets_sent"`
	PacketsRecv  uint64  `json:"packets_recv"`
	Errors       uint64  `json:"errors"`
	Drops        uint64  `json:"drops"`
	Speed        uint64  `json:"speed"`
}

// ApplicationMetrics 应用指标
type ApplicationMetrics struct {
	CPUUsage      float64                `json:"cpu_usage"`
	MemoryUsage   float64                `json:"memory_usage"`
	MemoryRSS     uint64                 `json:"memory_rss"`
	MemoryVMS     uint64                 `json:"memory_vms"`
	OpenFiles     int                    `json:"open_files"`
	Threads       int                    `json:"threads"`
	ResponseTime  float64                `json:"response_time_ms"`
	ErrorRate     float64                `json:"error_rate"`
	Throughput    float64                `json:"throughput_rps"`
	GCCount       uint64                 `json:"gc_count"`
	GCTime        float64                `json:"gc_time_ms"`
	Processes     []*ProcessMetrics      `json:"processes,omitempty"`
}

// ProcessMetrics 进程指标
type ProcessMetrics struct {
	PID           uint32  `json:"pid"`
	Name          string  `json:"name"`
	CPUUsage      float64 `json:"cpu_usage"`
	MemoryUsage   float64 `json:"memory_usage"`
	MemoryRSS     uint64  `json:"memory_rss"`
	OpenFiles     int     `json:"open_files"`
	Threads       int     `json:"threads"`
	Connections   int     `json:"connections"`
}

// SystemMetrics 系统指标
type SystemMetrics struct {
	CPUUsage      float64                `json:"cpu_usage"`
	MemoryUsage   float64                `json:"memory_usage"`
	MemoryTotal   uint64                 `json:"memory_total"`
	MemoryUsed    uint64                 `json:"memory_used"`
	DiskUsage     float64                `json:"disk_usage"`
	DiskTotal     uint64                 `json:"disk_total"`
	DiskUsed      uint64                 `json:"disk_used"`
	Load1         float64                `json:"load1"`
	Load5         float64                `json:"load5"`
	Load15        float64                `json:"load15"`
}

// CollectorConfig 采集器配置
type CollectorConfig struct {
	Enabled           bool          `json:"enabled" yaml:"enabled"`
	Interval          time.Duration `json:"interval" yaml:"interval"`
	NetworkEnabled    bool          `json:"network_enabled" yaml:"network_enabled"`
	AppEnabled        bool          `json:"app_enabled" yaml:"app_enabled"`
	SystemEnabled     bool          `json:"system_enabled" yaml:"system_enabled"`
	ProcPath          string        `json:"proc_path" yaml:"proc_path"`
	SysPath           string        `json:"sys_path" yaml:"sys_path"`
	EnablePerProcess  bool          `json:"enable_per_process" yaml:"enable_per_process"`
	EnablePerInterface bool         `json:"enable_per_interface" yaml:"enable_per_interface"`
	MaxProcesses      int           `json:"max_processes" yaml:"max_processes"`
}

// MetricsCollector 指标采集器
type MetricsCollector struct {
	config        *CollectorConfig
	discovery     *topology.DiscoveryEngine
	metrics       map[string]*AssetMetrics
	history       map[string][]*AssetMetrics
	mu            sync.RWMutex
	ctx           context.Context
	cancel        context.CancelFunc
	wg            sync.WaitGroup
	handlers      []MetricsHandler
	lastNetStats  map[string]*NetDevStats
}

// NetDevStats 网络设备统计
type NetDevStats struct {
	BytesSent   uint64
	BytesRecv   uint64
	PacketsSent uint64
	PacketsRecv uint64
	Errors      uint64
	Drops       uint64
	Timestamp   time.Time
}

// MetricsHandler 指标事件处理器
type MetricsHandler interface {
	OnMetricsCollected(metrics *AssetMetrics)
	OnMetricsUpdated(assetID string, metrics *AssetMetrics)
}

// NewMetricsCollector 创建指标采集器
func NewMetricsCollector(config *CollectorConfig, discovery *topology.DiscoveryEngine) *MetricsCollector {
	ctx, cancel := context.WithCancel(context.Background())
	
	return &MetricsCollector{
		config:       config,
		discovery:    discovery,
		metrics:      make(map[string]*AssetMetrics),
		history:      make(map[string][]*AssetMetrics),
		ctx:          ctx,
		cancel:       cancel,
		lastNetStats: make(map[string]*NetDevStats),
	}
}

// Start 启动采集器
func (c *MetricsCollector) Start() error {
	c.wg.Add(1)
	go c.collectionLoop()
	return nil
}

// Stop 停止采集器
func (c *MetricsCollector) Stop() {
	c.cancel()
	c.wg.Wait()
}

// RegisterHandler 注册指标事件处理器
func (c *MetricsCollector) RegisterHandler(handler MetricsHandler) {
	c.handlers = append(c.handlers, handler)
}

// collectionLoop 采集循环
func (c *MetricsCollector) collectionLoop() {
	defer c.wg.Done()
	
	ticker := time.NewTicker(c.config.Interval)
	defer ticker.Stop()
	
	// 首次立即采集
	c.collectAll()
	
	for {
		select {
		case <-c.ctx.Done():
			return
		case <-ticker.C:
			c.collectAll()
		}
	}
}

// collectAll 采集所有资产指标
func (c *MetricsCollector) collectAll() {
	if c.discovery == nil {
		return
	}
	
	entities := c.discovery.GetEntities()
	
	var wg sync.WaitGroup
	semaphore := make(chan struct{}, 10) // 限制并发数
	
	for _, entity := range entities {
		wg.Add(1)
		semaphore <- struct{}{}
		
		go func(entity *topology.Entity) {
			defer wg.Done()
			defer func() { <-semaphore }()
			
			metrics := c.collectAssetMetrics(entity)
			if metrics != nil {
				c.storeMetrics(metrics)
			}
		}(entity)
	}
	
	wg.Wait()
}

// collectAssetMetrics 采集单个资产指标
func (c *MetricsCollector) collectAssetMetrics(entity *topology.Entity) *AssetMetrics {
	metrics := &AssetMetrics{
		AssetID:   entity.ID,
		AssetType: AssetType(entity.Type),
		AssetName: entity.Name,
		Timestamp: time.Now(),
		Labels:    entity.Labels,
	}
	
	// 采集网络指标
	if c.config.NetworkEnabled {
		metrics.Network = c.collectNetworkMetrics(entity)
	}
	
	// 采集应用指标
	if c.config.AppEnabled {
		metrics.Application = c.collectApplicationMetrics(entity)
	}
	
	// 采集系统指标
	if c.config.SystemEnabled {
		metrics.System = c.collectSystemMetrics(entity)
	}
	
	return metrics
}

// collectNetworkMetrics 采集网络指标
func (c *MetricsCollector) collectNetworkMetrics(entity *topology.Entity) *NetworkMetrics {
	metrics := &NetworkMetrics{
		TCPStates:  make(map[string]int),
		Interfaces: make(map[string]*IfaceMetrics),
	}
	
	// 读取网络接口统计
	if c.config.EnablePerInterface {
		ifaces, err := net.Interfaces()
		if err == nil {
			for _, iface := range ifaces {
				if iface.Flags&net.FlagLoopback != 0 {
					continue
				}
				
				ifaceMetrics := c.collectInterfaceMetrics(iface.Name)
				if ifaceMetrics != nil {
					metrics.Interfaces[iface.Name] = ifaceMetrics
					metrics.BytesSent += ifaceMetrics.BytesSent
					metrics.BytesRecv += ifaceMetrics.BytesRecv
					metrics.PacketsSent += ifaceMetrics.PacketsSent
					metrics.PacketsRecv += ifaceMetrics.PacketsRecv
					metrics.ErrorsIn += ifaceMetrics.Errors
					metrics.DropsIn += ifaceMetrics.Drops
				}
			}
		}
	}
	
	// 采集TCP连接状态
	metrics.TCPStates = c.collectTCPStates()
	metrics.Connections = sumMapValues(metrics.TCPStates)
	
	return metrics
}

// collectInterfaceMetrics 采集接口指标
func (c *MetricsCollector) collectInterfaceMetrics(ifaceName string) *IfaceMetrics {
	// 读取 /proc/net/dev
	data, err := os.ReadFile(c.config.ProcPath + "/net/dev")
	if err != nil {
		return nil
	}
	
	lines := strings.Split(string(data), "\n")
	for _, line := range lines {
		if !strings.Contains(line, ifaceName+":") {
			continue
		}
		
		fields := strings.Fields(line)
		if len(fields) < 9 {
			continue
		}
		
		// 解析字段
		recvBytes, _ := strconv.ParseUint(fields[1], 10, 64)
		recvPackets, _ := strconv.ParseUint(fields[2], 10, 64)
		recvErrors, _ := strconv.ParseUint(fields[3], 10, 64)
		recvDrops, _ := strconv.ParseUint(fields[4], 10, 64)
		sentBytes, _ := strconv.ParseUint(fields[9], 10, 64)
		sentPackets, _ := strconv.ParseUint(fields[10], 10, 64)
		sentErrors, _ := strconv.ParseUint(fields[11], 10, 64)
		sentDrops, _ := strconv.ParseUint(fields[12], 10, 64)
		
		// 计算速率
		currentStats := &NetDevStats{
			BytesRecv:   recvBytes,
			BytesSent:   sentBytes,
			PacketsRecv: recvPackets,
			PacketsSent: sentPackets,
			Errors:      recvErrors + sentErrors,
			Drops:       recvDrops + sentDrops,
			Timestamp:   time.Now(),
		}
		
		ifaceMetrics := &IfaceMetrics{
			Name:        ifaceName,
			BytesSent:   sentBytes,
			BytesRecv:   recvBytes,
			PacketsSent: sentPackets,
			PacketsRecv: recvPackets,
			Errors:      recvErrors + sentErrors,
			Drops:       recvDrops + sentDrops,
		}
		
		// 计算速率（如果有上次数据）
		lastKey := ifaceName
		if lastStats, exists := c.lastNetStats[lastKey]; exists {
			duration := currentStats.Timestamp.Sub(lastStats.Timestamp).Seconds()
			if duration > 0 {
				// 这里可以计算速率
			}
		}
		
		c.lastNetStats[lastKey] = currentStats
		
		return ifaceMetrics
	}
	
	return nil
}

// collectTCPStates 采集TCP连接状态
func (c *MetricsCollector) collectTCPStates() map[string]int {
	states := make(map[string]int)
	
	// 读取 /proc/net/tcp
	data, err := os.ReadFile(c.config.ProcPath + "/net/tcp")
	if err != nil {
		return states
	}
	
	lines := strings.Split(string(data), "\n")
	for i, line := range lines {
		if i == 0 || line == "" {
			continue
		}
		
		fields := strings.Fields(line)
		if len(fields) < 4 {
			continue
		}
		
		// 解析状态
		stateCode := fields[3]
		state := parseTCPState(stateCode)
		states[state]++
	}
	
	return states
}

// parseTCPState 解析TCP状态码
func parseTCPState(code string) string {
	stateMap := map[string]string{
		"01": "ESTABLISHED",
		"02": "SYN_SENT",
		"03": "SYN_RECV",
		"04": "FIN_WAIT1",
		"05": "FIN_WAIT2",
		"06": "TIME_WAIT",
		"07": "CLOSE",
		"08": "CLOSE_WAIT",
		"09": "LAST_ACK",
		"0A": "LISTEN",
		"0B": "CLOSING",
	}
	
	if state, exists := stateMap[code]; exists {
		return state
	}
	return "UNKNOWN"
}

// collectApplicationMetrics 采集应用指标
func (c *MetricsCollector) collectApplicationMetrics(entity *topology.Entity) *ApplicationMetrics {
	metrics := &ApplicationMetrics{
		Processes: make([]*ProcessMetrics, 0),
	}
	
	// 如果有进程ID列表，采集进程指标
	if len(entity.ProcessIDs) > 0 {
		for _, pid := range entity.ProcessIDs {
			if len(metrics.Processes) >= c.config.MaxProcesses {
				break
			}
			
			procMetrics := c.collectProcessMetrics(pid)
			if procMetrics != nil {
				metrics.Processes = append(metrics.Processes, procMetrics)
				
				// 累加指标
				metrics.CPUUsage += procMetrics.CPUUsage
				metrics.MemoryRSS += procMetrics.MemoryRSS
				metrics.Threads += procMetrics.Threads
				metrics.OpenFiles += procMetrics.OpenFiles
			}
		}
	}
	
	// 采集容器指标
	if entity.ContainerID != "" {
		containerMetrics := c.collectContainerMetrics(entity.ContainerID)
		if containerMetrics != nil {
			metrics.CPUUsage = containerMetrics.CPUUsage
			metrics.MemoryUsage = containerMetrics.MemoryUsage
			metrics.MemoryRSS = containerMetrics.MemoryRSS
		}
	}
	
	// 计算内存使用率
	sysInfo := c.getSystemInfo()
	if sysInfo.MemoryTotal > 0 {
		metrics.MemoryUsage = float64(metrics.MemoryRSS) / float64(sysInfo.MemoryTotal) * 100
	}
	
	return metrics
}

// collectProcessMetrics 采集进程指标
func (c *MetricsCollector) collectProcessMetrics(pid uint32) *ProcessMetrics {
	procPath := fmt.Sprintf("%s/%d", c.config.ProcPath, pid)
	
	// 检查进程是否存在
	if _, err := os.Stat(procPath); os.IsNotExist(err) {
		return nil
	}
	
	metrics := &ProcessMetrics{
		PID: pid,
	}
	
	// 读取进程状态
	statusData, err := os.ReadFile(fmt.Sprintf("%s/status", procPath))
	if err == nil {
		lines := strings.Split(string(statusData), "\n")
		for _, line := range lines {
			if strings.HasPrefix(line, "Name:") {
				metrics.Name = strings.TrimSpace(strings.TrimPrefix(line, "Name:"))
			} else if strings.HasPrefix(line, "VmRSS:") {
				fields := strings.Fields(line)
				if len(fields) >= 2 {
					rss, _ := strconv.ParseUint(fields[1], 10, 64)
					metrics.MemoryRSS = rss * 1024 // KB to bytes
				}
			} else if strings.HasPrefix(line, "Threads:") {
				fields := strings.Fields(line)
				if len(fields) >= 2 {
					metrics.Threads, _ = strconv.Atoi(fields[1])
				}
			}
		}
	}
	
	// 读取打开的文件数
	fdPath := fmt.Sprintf("%s/fd", procPath)
	if entries, err := os.ReadDir(fdPath); err == nil {
		metrics.OpenFiles = len(entries)
	}
	
	// 读取CPU使用率（需要计算）
	statData, err := os.ReadFile(fmt.Sprintf("%s/stat", procPath))
	if err == nil {
		fields := strings.Fields(string(statData))
		if len(fields) >= 15 {
			// utime + stime
			utime, _ := strconv.ParseUint(fields[13], 10, 64)
			stime, _ := strconv.ParseUint(fields[14], 10, 64)
			totalTime := utime + stime
			// 这里简化处理，实际需要计算差值
			_ = totalTime
		}
	}
	
	return metrics
}

// collectContainerMetrics 采集容器指标
func (c *MetricsCollector) collectContainerMetrics(containerID string) *ApplicationMetrics {
	// 从cgroup读取容器指标
	metrics := &ApplicationMetrics{}
	
	// 读取内存使用
	memPath := fmt.Sprintf("/sys/fs/cgroup/memory/docker/%s/memory.usage_in_bytes", containerID)
	if data, err := os.ReadFile(memPath); err == nil {
		memUsage, _ := strconv.ParseUint(strings.TrimSpace(string(data)), 10, 64)
		metrics.MemoryRSS = memUsage
	}
	
	// 读取CPU使用
	cpuPath := fmt.Sprintf("/sys/fs/cgroup/cpu/docker/%s/cpuacct.usage", containerID)
	if data, err := os.ReadFile(cpuPath); err == nil {
		// 纳秒转换为百分比需要更多计算
		_ = data
	}
	
	return metrics
}

// collectSystemMetrics 采集系统指标
func (c *MetricsCollector) collectSystemMetrics(entity *topology.Entity) *SystemMetrics {
	metrics := &SystemMetrics{}
	
	// 读取内存信息
	memInfo, err := os.ReadFile(c.config.ProcPath + "/meminfo")
	if err == nil {
		lines := strings.Split(string(memInfo), "\n")
		for _, line := range lines {
			if strings.HasPrefix(line, "MemTotal:") {
				fields := strings.Fields(line)
				if len(fields) >= 2 {
					metrics.MemoryTotal, _ = strconv.ParseUint(fields[1], 10, 64)
					metrics.MemoryTotal *= 1024 // KB to bytes
				}
			} else if strings.HasPrefix(line, "MemAvailable:") {
				fields := strings.Fields(line)
				if len(fields) >= 2 {
					memAvailable, _ := strconv.ParseUint(fields[1], 10, 64)
					memAvailable *= 1024
					if metrics.MemoryTotal > 0 {
						metrics.MemoryUsed = metrics.MemoryTotal - memAvailable
						metrics.MemoryUsage = float64(metrics.MemoryUsed) / float64(metrics.MemoryTotal) * 100
					}
				}
			}
		}
	}
	
	// 读取负载
	loadData, err := os.ReadFile(c.config.ProcPath + "/loadavg")
	if err == nil {
		fields := strings.Fields(string(loadData))
		if len(fields) >= 3 {
			metrics.Load1, _ = strconv.ParseFloat(fields[0], 64)
			metrics.Load5, _ = strconv.ParseFloat(fields[1], 64)
			metrics.Load15, _ = strconv.ParseFloat(fields[2], 64)
		}
	}
	
	// 读取CPU使用率
	metrics.CPUUsage = c.getCPUUsage()
	
	return metrics
}

// getCPUUsage 获取CPU使用率
func (c *MetricsCollector) getCPUUsage() float64 {
	// 读取 /proc/stat
	data, err := os.ReadFile(c.config.ProcPath + "/stat")
	if err != nil {
		return 0
	}
	
	lines := strings.Split(string(data), "\n")
	for _, line := range lines {
		if !strings.HasPrefix(line, "cpu ") {
			continue
		}
		
		fields := strings.Fields(line)
		if len(fields) < 5 {
			continue
		}
		
		// 解析CPU时间
		user, _ := strconv.ParseUint(fields[1], 10, 64)
		nice, _ := strconv.ParseUint(fields[2], 10, 64)
		system, _ := strconv.ParseUint(fields[3], 10, 64)
		idle, _ := strconv.ParseUint(fields[4], 10, 64)
		
		total := user + nice + system + idle
		if total > 0 {
			return float64(user+nice+system) / float64(total) * 100
		}
	}
	
	return 0
}

// getSystemInfo 获取系统信息
func (c *MetricsCollector) getSystemInfo() *SystemMetrics {
	return c.collectSystemMetrics(&topology.Entity{})
}

// storeMetrics 存储指标
func (c *MetricsCollector) storeMetrics(metrics *AssetMetrics) {
	c.mu.Lock()
	defer c.mu.Unlock()
	
	// 存储当前指标
	c.metrics[metrics.AssetID] = metrics
	
	// 存储历史
	history := c.history[metrics.AssetID]
	history = append(history, metrics)
	
	// 限制历史长度（保留最近100个）
	if len(history) > 100 {
		history = history[len(history)-100:]
	}
	c.history[metrics.AssetID] = history
	
	// 通知处理器
	for _, handler := range c.handlers {
		handler.OnMetricsCollected(metrics)
	}
}

// GetMetrics 获取资产当前指标
func (c *MetricsCollector) GetMetrics(assetID string) *AssetMetrics {
	c.mu.RLock()
	defer c.mu.RUnlock()
	
	return c.metrics[assetID]
}

// GetMetricsHistory 获取资产指标历史
func (c *MetricsCollector) GetMetricsHistory(assetID string, limit int) []*AssetMetrics {
	c.mu.RLock()
	defer c.mu.RUnlock()
	
	history := c.history[assetID]
	if limit <= 0 || limit > len(history) {
		return history
	}
	
	return history[len(history)-limit:]
}

// GetAllMetrics 获取所有资产指标
func (c *MetricsCollector) GetAllMetrics() map[string]*AssetMetrics {
	c.mu.RLock()
	defer c.mu.RUnlock()
	
	result := make(map[string]*AssetMetrics)
	for id, metrics := range c.metrics {
		result[id] = metrics
	}
	
	return result
}

// GetMetricsByType 按类型获取资产指标
func (c *MetricsCollector) GetMetricsByType(assetType AssetType) []*AssetMetrics {
	c.mu.RLock()
	defer c.mu.RUnlock()
	
	var result []*AssetMetrics
	for _, metrics := range c.metrics {
		if metrics.AssetType == assetType {
			result = append(result, metrics)
		}
	}
	
	return result
}

// sumMapValues 计算map值的总和
func sumMapValues(m map[string]int) int {
	sum := 0
	for _, v := range m {
		sum += v
	}
	return sum
}
