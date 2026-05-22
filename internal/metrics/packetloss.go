// Package metrics 提供丢包监控能力
// Copyright (c) 2026 Cloud Flow Team
// Licensed under the MIT License.

package metrics

import (
	"context"
	"fmt"
	"net"
	"os/exec"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"
)

// ============================================================
// 丢包监控配置
// ============================================================

type PacketLossConfig struct {
	Enabled           bool          `yaml:"enabled" json:"enabled"`
	CheckInterval     time.Duration `yaml:"check_interval" json:"check_interval"`
	Interface         string        `yaml:"interface" json:"interface"` // 空表示所有接口
	ThresholdPercent  float64       `yaml:"threshold_percent" json:"threshold_percent"`
	ThresholdPackets  uint64        `yaml:"threshold_packets" json:"threshold_packets"`
	AlertCooldown     time.Duration `yaml:"alert_cooldown" json:"alert_cooldown"`
	EnableTCPCheck    bool          `yaml:"enable_tcp_check" json:"enable_tcp_check"`
	TCPCheckTarget    string        `yaml:"tcp_check_target" json:"tcp_check_target"`
	TCPCheckPort      uint16        `yaml:"tcp_check_port" json:"tcp_check_port"`
}

func DefaultPacketLossConfig() *PacketLossConfig {
	return &PacketLossConfig{
		Enabled:          true,
		CheckInterval:    30 * time.Second,
		Interface:        "",
		ThresholdPercent: 1.0,  // 1% 丢包率触发告警
		ThresholdPackets: 100,   // 或100个丢包
		AlertCooldown:    5 * time.Minute,
		EnableTCPCheck:   true,
		TCPCheckTarget:   "8.8.8.8",
		TCPCheckPort:     53,
	}
}

// ============================================================
// 丢包统计
// ============================================================

type PacketLossStats struct {
	Interface       string    `json:"interface"`
	Timestamp       time.Time `json:"timestamp"`
	
	// 接收统计
	RxPackets       uint64  `json:"rx_packets"`
	RxDropped       uint64  `json:"rx_dropped"`
	RxErrors        uint64  `json:"rx_errors"`
	RxMissed        uint64  `json:"rx_missed"`     // 网卡缓冲区溢出
	
	// 发送统计
	TxPackets       uint64  `json:"tx_packets"`
	TxDropped       uint64  `json:"tx_dropped"`
	TxErrors        uint64  `json:"tx_errors"`
	TxCarrier       uint64  `json:"tx_carrier"`
	
	// 计算指标
	LossPercent     float64 `json:"loss_percent"`
	LossPackets     uint64  `json:"loss_packets"`
	
	// TCP 检测
	TCPLossPercent  float64 `json:"tcp_loss_percent"`
	TCPProbeCount   uint64  `json:"tcp_probe_count"`
	TCPSuccessCount uint64  `json:"tcp_success_count"`
}

// ============================================================
// 丢包监控器
// ============================================================

type PacketLossMonitor struct {
	config    *PacketLossConfig
	
	// 历史数据
	history   map[string]*PacketLossStats // interface -> stats
	historyMu sync.RWMutex
	
	// 告警状态
	lastAlert time.Time
	alerting  atomic.Bool
	
	// 统计
	totalChecks   atomic.Uint64
	totalAlerts   atomic.Uint64
	
	// 生命周期
	ctx    context.Context
	cancel context.CancelFunc
	wg     sync.WaitGroup
	running atomic.Bool
	
	// 回调
	onAlert func(stats *PacketLossStats)
}

func NewPacketLossMonitor(cfg *PacketLossConfig) *PacketLossMonitor {
	if cfg == nil {
		cfg = DefaultPacketLossConfig()
	}
	
	ctx, cancel := context.WithCancel(context.Background())
	
	return &PacketLossMonitor{
		config:  cfg,
		history: make(map[string]*PacketLossStats),
		ctx:     ctx,
		cancel:  cancel,
	}
}

func (m *PacketLossMonitor) Start() error {
	if m.running.Load() {
		return fmt.Errorf("packet loss monitor already running")
	}
	
	m.running.Store(true)
	
	// 启动监控循环
	m.wg.Add(1)
	go m.monitorLoop()
	
	// 启动 TCP 检测（如启用）
	if m.config.EnableTCPCheck {
		m.wg.Add(1)
		go m.tcpProbeLoop()
	}
	
	return nil
}

func (m *PacketLossMonitor) Stop() error {
	if !m.running.Load() {
		return nil
	}
	
	m.running.Store(false)
	m.cancel()
	m.wg.Wait()
	
	return nil
}

func (m *PacketLossMonitor) monitorLoop() {
	defer m.wg.Done()
	
	ticker := time.NewTicker(m.config.CheckInterval)
	defer ticker.Stop()
	
	// 立即执行一次
	m.checkAllInterfaces()
	
	for {
		select {
		case <-m.ctx.Done():
			return
		case <-ticker.C:
			m.checkAllInterfaces()
		}
	}
}

func (m *PacketLossMonitor) checkAllInterfaces() {
	m.totalChecks.Add(1)
	
	interfaces := m.getInterfaces()
	
	for _, iface := range interfaces {
		stats, err := m.readInterfaceStats(iface)
		if err != nil {
			continue
		}
		
		// 计算丢包率
		m.calculateLossRate(iface, stats)
		
		// 检查告警阈值
		m.checkThreshold(stats)
	}
}

func (m *PacketLossMonitor) getInterfaces() []string {
	if m.config.Interface != "" {
		return []string{m.config.Interface}
	}
	
	// 读取所有网络接口
	out, err := exec.Command("ls", "/sys/class/net/").Output()
	if err != nil {
		return []string{"eth0", "ens33", "enp0s3"} // 默认回退
	}
	
	var interfaces []string
	for _, line := range strings.Split(string(out), "\n") {
		iface := strings.TrimSpace(line)
		if iface != "" && iface != "lo" {
			interfaces = append(interfaces, iface)
		}
	}
	
	return interfaces
}

func (m *PacketLossMonitor) readInterfaceStats(iface string) (*PacketLossStats, error) {
	stats := &PacketLossStats{
		Interface: iface,
		Timestamp: time.Now(),
	}
	
	// 读取 /sys/class/net/<iface>/statistics/
	basePath := fmt.Sprintf("/sys/class/net/%s/statistics/", iface)
	
	stats.RxPackets = readUint64File(basePath + "rx_packets")
	stats.RxDropped = readUint64File(basePath + "rx_dropped")
	stats.RxErrors = readUint64File(basePath + "rx_errors")
	stats.RxMissed = readUint64File(basePath + "rx_missed_errors")
	
	stats.TxPackets = readUint64File(basePath + "tx_packets")
	stats.TxDropped = readUint64File(basePath + "tx_dropped")
	stats.TxErrors = readUint64File(basePath + "tx_errors")
	stats.TxCarrier = readUint64File(basePath + "tx_carrier_errors")
	
	return stats, nil
}

func (m *PacketLossMonitor) calculateLossRate(iface string, current *PacketLossStats) {
	m.historyMu.Lock()
	defer m.historyMu.Unlock()
	
	prev, exists := m.history[iface]
	if !exists {
		m.history[iface] = current
		return
	}
	
	// 计算差值
	rxDelta := current.RxPackets - prev.RxPackets
	rxDropDelta := current.RxDropped - prev.RxDropped
	rxErrDelta := current.RxErrors - prev.RxErrors
	
	txDelta := current.TxPackets - prev.TxPackets
	txDropDelta := current.TxDropped - prev.TxDropped
	txErrDelta := current.TxErrors - prev.TxErrors
	
	// 计算丢包率
	if rxDelta > 0 {
		current.LossPercent = float64(rxDropDelta+rxErrDelta) / float64(rxDelta+rxDropDelta+rxErrDelta) * 100
	}
	current.LossPackets = rxDropDelta + rxErrDelta + txDropDelta + txErrDelta
	
	// 保存历史
	m.history[iface] = current
}

func (m *PacketLossMonitor) checkThreshold(stats *PacketLossStats) {
	// 检查是否超过阈值
	if stats.LossPercent < m.config.ThresholdPercent && 
	   stats.LossPackets < m.config.ThresholdPackets {
		return
	}
	
	// 检查告警冷却
	if time.Since(m.lastAlert) < m.config.AlertCooldown {
		return
	}
	
	// 触发告警
	m.lastAlert = time.Now()
	m.totalAlerts.Add(1)
	
	if m.onAlert != nil {
		m.onAlert(stats)
	}
}

// ============================================================
// TCP 丢包检测
// ============================================================

func (m *PacketLossMonitor) tcpProbeLoop() {
	defer m.wg.Done()
	
	ticker := time.NewTicker(m.config.CheckInterval)
	defer ticker.Stop()
	
	var (
		probeCount   atomic.Uint64
		successCount atomic.Uint64
	)
	
	for {
		select {
		case <-m.ctx.Done():
			return
		case <-ticker.C:
			// 执行 TCP 探测
			start := time.Now()
			conn, err := net.DialTimeout("tcp", 
				fmt.Sprintf("%s:%d", m.config.TCPCheckTarget, m.config.TCPCheckPort),
				5*time.Second)
			
			probeCount.Add(1)
			
			if err == nil {
				conn.Close()
				successCount.Add(1)
			}
			
			// 更新历史中的 TCP 统计
			m.historyMu.Lock()
			for _, stats := range m.history {
				stats.TCPProbeCount = probeCount.Load()
				stats.TCPSuccessCount = successCount.Load()
				if stats.TCPProbeCount > 0 {
					stats.TCPLossPercent = float64(stats.TCPProbeCount-stats.TCPSuccessCount) / 
						float64(stats.TCPProbeCount) * 100
				}
			}
			m.historyMu.Unlock()
			
			_ = start
		}
	}
}

// ============================================================
// 查询接口
// ============================================================

func (m *PacketLossMonitor) GetStats(iface string) (*PacketLossStats, bool) {
	m.historyMu.RLock()
	defer m.historyMu.RUnlock()
	stats, ok := m.history[iface]
	if !ok {
		return nil, false
	}
	cp := *stats
	return &cp, true
}

func (m *PacketLossMonitor) GetAllStats() map[string]*PacketLossStats {
	m.historyMu.RLock()
	defer m.historyMu.RUnlock()
	
	result := make(map[string]*PacketLossStats, len(m.history))
	for k, v := range m.history {
		cp := *v
		result[k] = &cp
	}
	return result
}

func (m *PacketLossMonitor) IsAlerting() bool {
	return m.alerting.Load()
}

func (m *PacketLossMonitor) OnAlert(fn func(stats *PacketLossStats)) {
	m.onAlert = fn
}

// ============================================================
// 辅助函数
// ============================================================

func readUint64File(path string) uint64 {
	data, err := exec.Command("cat", path).Output()
	if err != nil {
		return 0
	}
	
	val, err := strconv.ParseUint(strings.TrimSpace(string(data)), 10, 64)
	if err != nil {
		return 0
	}
	
	return val
}


