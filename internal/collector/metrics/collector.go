//go:build linux

// Package metrics 提供系统指标采集功能
package metrics

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"os"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/shirou/gopsutil/v3/cpu"
	"github.com/shirou/gopsutil/v3/disk"
	"github.com/shirou/gopsutil/v3/host"
	"github.com/shirou/gopsutil/v3/load"
	"github.com/shirou/gopsutil/v3/mem"
	"github.com/shirou/gopsutil/v3/net"

	"github.com/meinanzilinzhengying/cloud-flow-agent/pkg/models"
)

// Collector 系统指标采集器
type Collector struct {
	config    *models.MetricsCollectorConfig
	status    models.CollectorStatus
	mu        sync.RWMutex
	ctx       context.Context
	cancel    context.CancelFunc

	// 数据通道
	metricChan chan *models.SystemMetric
	errorChan  chan error

	// 主机信息
	hostIP   string
	hostname string

	// 统计
	eventsCount uint64
}

// NewCollector 创建系统指标采集器
func NewCollector() *Collector {
	return &Collector{
		metricChan: make(chan *models.SystemMetric, 1000),
		errorChan:  make(chan error, 100),
	}
}

// Name 返回采集器名称
func (c *Collector) Name() string {
	return "system-metrics"
}

// Type 返回采集器类型
func (c *Collector) Type() models.CollectorType {
	return models.CollectorMetrics
}

// Init 初始化采集器
func (c *Collector) Init(ctx context.Context, config interface{}) error {
	cfg, ok := config.(*models.MetricsCollectorConfig)
	if !ok {
		return errors.New("invalid config type")
	}
	c.config = cfg

	c.ctx, c.cancel = context.WithCancel(ctx)

	// 获取主机信息
	c.initHostInfo()

	c.status = models.CollectorStatus{
		Name:      c.Name(),
		Type:      c.Type(),
		Enabled:   cfg.Enabled,
		StartTime: time.Now(),
	}

	return nil
}

// Start 启动采集器
func (c *Collector) Start(ctx context.Context) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if !c.config.Enabled {
		return nil
	}

	// 启动采集协程
	go c.collectLoop()

	c.status.Running = true
	c.status.StartTime = time.Now()

	return nil
}

// Stop 停止采集器
func (c *Collector) Stop(ctx context.Context) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.cancel != nil {
		c.cancel()
	}

	close(c.metricChan)
	close(c.errorChan)

	c.status.Running = false

	return nil
}

// Status 返回采集器状态
func (c *Collector) Status() models.CollectorStatus {
	c.mu.RLock()
	defer c.mu.RUnlock()

	status := c.status
	status.EventsCount = c.eventsCount

	return status
}

// Metrics 返回系统指标数据通道
func (c *Collector) Metrics() <-chan *models.SystemMetric {
	return c.metricChan
}

// Events 返回事件通道
func (c *Collector) Events() <-chan interface{} {
	ch := make(chan interface{})
	go func() {
		for metric := range c.metricChan {
			ch <- metric
		}
		close(ch)
	}()
	return ch
}

// Errors 返回错误通道
func (c *Collector) Errors() <-chan error {
	return c.errorChan
}

// initHostInfo 初始化主机信息
func (c *Collector) initHostInfo() {
	// 获取主机名
	info, err := host.Info()
	if err == nil {
		c.hostname = info.Hostname
	}

	// 获取主机 IP
	c.hostIP = c.getHostIP()
}

// getHostIP 获取主机 IP
func (c *Collector) getHostIP() string {
	interfaces, err := net.Interfaces()
	if err != nil {
		return "127.0.0.1"
	}

	for _, iface := range interfaces {
		// 跳过回环接口
		if strings.HasPrefix(iface.Name, "lo") {
			continue
		}

		// 跳过未启用的接口
		if iface.Flags&net.FlagUp == 0 {
			continue
		}

		for _, addr := range iface.Addrs {
			// 优先返回 IPv4 地址
			if strings.Contains(addr.Addr, ".") {
				return strings.Split(addr.Addr, "/")[0]
			}
		}
	}

	return "127.0.0.1"
}

// collectLoop 采集循环
func (c *Collector) collectLoop() {
	interval := time.Duration(c.config.Interval) * time.Second
	if interval <= 0 {
		interval = 10 * time.Second
	}

	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	for {
		select {
		case <-c.ctx.Done():
			return
		case <-ticker.C:
			metric := c.collect()
			if metric != nil {
				select {
				case c.metricChan <- metric:
					c.mu.Lock()
					c.eventsCount++
					c.mu.Unlock()
				default:
					// 通道满，丢弃
				}
			}
		}
	}
}

// collect 采集所有指标
func (c *Collector) collect() *models.SystemMetric {
	metric := &models.SystemMetric{
		Timestamp: time.Now(),
		HostIP:    c.hostIP,
		Hostname:  c.hostname,
	}

	// 采集 CPU
	if c.config.CPU {
		c.collectCPU(metric)
	}

	// 采集内存
	if c.config.Memory {
		c.collectMemory(metric)
	}

	// 采集磁盘
	if c.config.Disk {
		c.collectDisk(metric)
	}

	// 采集网络
	if c.config.Network {
		c.collectNetwork(metric)
	}

	return metric
}

// collectCPU 采集 CPU 指标
func (c *Collector) collectCPU(metric *models.SystemMetric) {
	// CPU 使用率
	percent, err := cpu.Percent(time.Second, false)
	if err == nil && len(percent) > 0 {
		metric.CPUUsage = percent[0]
	}

	// CPU 时间统计
	times, err := cpu.Times(false)
	if err == nil && len(times) > 0 {
		total := times[0].User + times[0].System + times[0].Idle + times[0].Steal
		if total > 0 {
			metric.CPUUser = times[0].User / total * 100
			metric.CPUSystem = times[0].System / total * 100
			metric.CPUIdle = times[0].Idle / total * 100
			metric.CPUSteal = times[0].Steal / total * 100
		}
	}

	// 系统负载
	loadAvg, err := load.Avg()
	if err == nil {
		metric.Load1 = loadAvg.Load1
		metric.Load5 = loadAvg.Load5
		metric.Load15 = loadAvg.Load15
	}
}

// collectMemory 采集内存指标
func (c *Collector) collectMemory(metric *models.SystemMetric) {
	memInfo, err := mem.VirtualMemory()
	if err == nil {
		metric.MemTotal = memInfo.Total
		metric.MemUsed = memInfo.Used
		metric.MemFree = memInfo.Free
		metric.MemBuffers = memInfo.Buffers
		metric.MemCached = memInfo.Cached
		metric.MemUsage = memInfo.UsedPercent
	}

	swapInfo, err := mem.SwapMemory()
	if err == nil {
		metric.SwapTotal = swapInfo.Total
		metric.SwapUsed = swapInfo.Used
		metric.SwapUsage = swapInfo.UsedPercent
	}
}

// collectDisk 采集磁盘指标
func (c *Collector) collectDisk(metric *models.SystemMetric) {
	// 磁盘使用率
	paths := c.config.DiskPaths
	if len(paths) == 0 {
		paths = []string{"/"}
	}

	var total, used, free uint64
	for _, path := range paths {
		usage, err := disk.Usage(path)
		if err == nil {
			total += usage.Total
			used += usage.Used
			free += usage.Free
		}
	}

	metric.DiskTotal = total
	metric.DiskUsed = used
	metric.DiskFree = free
	if total > 0 {
		metric.DiskUsage = float64(used) / float64(total) * 100
	}

	// 磁盘 IO
	ioCounters, err := disk.IOCounters()
	if err == nil {
		for _, io := range ioCounters {
			metric.DiskReadBytes += io.ReadBytes
			metric.DiskWriteBytes += io.WriteBytes
			metric.DiskReadOps += io.ReadCount
			metric.DiskWriteOps += io.WriteCount
		}
	}
}

// collectNetwork 采集网络指标
func (c *Collector) collectNetwork(metric *models.SystemMetric) {
	// 网络 IO
	ioCounters, err := net.IOCounters(true)
	if err == nil {
		for _, io := range ioCounters {
			// 跳过回环接口
			if io.Name == "lo" {
				continue
			}
			metric.NetBytesSent += io.BytesSent
			metric.NetBytesRecv += io.BytesRecv
			metric.NetPacketsSent += io.PacketsSent
			metric.NetPacketsRecv += io.PacketsRecv
		}
	}

	// 连接统计
	metric.NetTCPConns = c.countTCPConnections()
	metric.NetUDPConns = c.countUDPConnections()
}

// countTCPConnections 统计 TCP 连接数
func (c *Collector) countTCPConnections() uint64 {
	file, err := os.Open("/proc/net/tcp")
	if err != nil {
		return 0
	}
	defer file.Close()

	var count uint64
	scanner := bufio.NewScanner(file)
	scanner.Scan() // 跳过标题行

	for scanner.Scan() {
		count++
	}

	// 也统计 IPv6
	file6, err := os.Open("/proc/net/tcp6")
	if err == nil {
		defer file6.Close()
		scanner := bufio.NewScanner(file6)
		scanner.Scan()
		for scanner.Scan() {
			count++
		}
	}

	return count
}

// countUDPConnections 统计 UDP 连接数
func (c *Collector) countUDPConnections() uint64 {
	file, err := os.Open("/proc/net/udp")
	if err != nil {
		return 0
	}
	defer file.Close()

	var count uint64
	scanner := bufio.NewScanner(file)
	scanner.Scan()

	for scanner.Scan() {
		count++
	}

	file6, err := os.Open("/proc/net/udp6")
	if err == nil {
		defer file6.Close()
		scanner := bufio.NewScanner(file6)
		scanner.Scan()
		for scanner.Scan() {
			count++
		}
	}

	return count
}

// GetCPUCount 获取 CPU 核心数
func GetCPUCount() int {
	return runtime.NumCPU()
}

// GetUptime 获取系统运行时间
func GetUptime() (time.Duration, error) {
	file, err := os.Open("/proc/uptime")
	if err != nil {
		return 0, err
	}
	defer file.Close()

	var uptimeSeconds float64
	_, err = fmt.Fscanf(file, "%f", &uptimeSeconds)
	if err != nil {
		return 0, err
	}

	return time.Duration(uptimeSeconds * float64(time.Second)), nil
}

// GetLoadAvg 获取系统负载
func GetLoadAvg() (float64, float64, float64, error) {
	file, err := os.Open("/proc/loadavg")
	if err != nil {
		return 0, 0, 0, err
	}
	defer file.Close()

	var load1, load5, load15 float64
	_, err = fmt.Fscanf(file, "%f %f %f", &load1, &load5, &load15)
	if err != nil {
		return 0, 0, 0, err
	}

	return load1, load5, load15, nil
}

// GetMemInfo 获取内存信息
func GetMemInfo() (total, free, available uint64, err error) {
	file, err := os.Open("/proc/meminfo")
	if err != nil {
		return 0, 0, 0, err
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := scanner.Text()
		fields := strings.Fields(line)
		if len(fields) < 2 {
			continue
		}

		value, _ := strconv.ParseUint(fields[1], 10, 64)
		// 转换为字节 (原单位为 KB)
		value *= 1024

		switch fields[0] {
		case "MemTotal:":
			total = value
		case "MemFree:":
			free = value
		case "MemAvailable:":
			available = value
		}
	}

	return total, free, available, nil
}
