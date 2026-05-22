// Package timesync 提供NTP时钟自动校准能力
// Copyright (c) 2026 Cloud Flow Team
// Licensed under the MIT License.

package timesync

import (
	"context"
	"fmt"
	"net"
	"os/exec"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"
)

// ============================================================
// NTP 配置
// ============================================================

type NTPConfig struct {
	Enabled           bool          `yaml:"enabled" json:"enabled"`
	SyncInterval      time.Duration `yaml:"sync_interval" json:"sync_interval"`
	Servers           []string      `yaml:"servers" json:"servers"`
	Timeout           time.Duration `yaml:"timeout" json:"timeout"`
	MaxDrift          time.Duration `yaml:"max_drift" json:"max_drift"`             // 最大允许时钟偏差
	AutoCorrect       bool          `yaml:"auto_correct" json:"auto_correct"`       // 是否自动校准
	DriftThreshold    time.Duration `yaml:"drift_threshold" json:"drift_threshold"` // 偏差阈值，超过则告警
	RetryAttempts     int           `yaml:"retry_attempts" json:"retry_attempts"`
	RetryInterval     time.Duration `yaml:"retry_interval" json:"retry_interval"`
}

func DefaultNTPConfig() *NTPConfig {
	return &NTPConfig{
		Enabled:        true,
		SyncInterval:   1 * time.Hour,
		Servers:        []string{"pool.ntp.org", "time.windows.com", "time.apple.com", "cn.pool.ntp.org"},
		Timeout:        10 * time.Second,
		MaxDrift:       5 * time.Minute,
		AutoCorrect:    false, // 默认不自动修改系统时间，仅监控
		DriftThreshold: 1 * time.Second,
		RetryAttempts:  3,
		RetryInterval:  5 * time.Second,
	}
}

// ============================================================
// 时钟同步状态
// ============================================================

type SyncStatus int

const (
	SyncStatusUnknown SyncStatus = iota
	SyncStatusSynced
	SyncStatusDrifted
	SyncStatusError
	SyncStatusCorrecting
)

func (s SyncStatus) String() string {
	switch s {
	case SyncStatusSynced:
		return "synced"
	case SyncStatusDrifted:
		return "drifted"
	case SyncStatusError:
		return "error"
	case SyncStatusCorrecting:
		return "correcting"
	default:
		return "unknown"
	}
}

type TimeSyncInfo struct {
	LocalTime      time.Time     `json:"local_time"`
	ReferenceTime  time.Time     `json:"reference_time"`
	Drift          time.Duration `json:"drift"`
	DriftAbs       time.Duration `json:"drift_abs"`
	Server         string        `json:"server"`
	Status         SyncStatus    `json:"status"`
	LastSyncTime   time.Time     `json:"last_sync_time"`
	TotalSyncs     uint64        `json:"total_syncs"`
	SuccessSyncs   uint64        `json:"success_syncs"`
	FailedSyncs    uint64        `json:"failed_syncs"`
	CorrectionCount uint64       `json:"correction_count"`
}

// ============================================================
// NTP 客户端
// ============================================================

type NTPClient struct {
	config *NTPConfig

	// 状态
	currentInfo atomic.Pointer[TimeSyncInfo]
	status      atomic.Int32

	// 生命周期
	ctx    context.Context
	cancel context.CancelFunc
	wg     sync.WaitGroup
	running atomic.Bool

	// 回调
	onDrift    func(drift time.Duration, info *TimeSyncInfo)
	onCorrect  func(oldTime, newTime time.Time)
	onError    func(err error)
}

func NewNTPClient(cfg *NTPConfig) *NTPClient {
	if cfg == nil {
		cfg = DefaultNTPConfig()
	}

	ctx, cancel := context.WithCancel(context.Background())

	c := &NTPClient{
		config: cfg,
		ctx:    ctx,
		cancel: cancel,
	}

	// 初始化状态
	c.status.Store(int32(SyncStatusUnknown))
	c.currentInfo.Store(&TimeSyncInfo{
		Status: SyncStatusUnknown,
	})

	return c
}

func (c *NTPClient) Start() error {
	if c.running.Load() {
		return fmt.Errorf("NTP client already running")
	}

	c.running.Store(true)

	// 立即执行一次同步
	c.performSync()

	// 启动定时同步循环
	c.wg.Add(1)
	go c.syncLoop()

	return nil
}

func (c *NTPClient) Stop() error {
	if !c.running.Load() {
		return nil
	}

	c.running.Store(false)
	c.cancel()
	c.wg.Wait()

	return nil
}

func (c *NTPClient) syncLoop() {
	defer c.wg.Done()

	ticker := time.NewTicker(c.config.SyncInterval)
	defer ticker.Stop()

	for {
		select {
		case <-c.ctx.Done():
			return
		case <-ticker.C:
			c.performSync()
		}
	}
}

func (c *NTPClient) performSync() {
	info := &TimeSyncInfo{
		LocalTime: time.Now(),
		Status:    SyncStatusError,
	}

	// 尝试所有服务器
	for _, server := range c.config.Servers {
		refTime, err := c.queryNTP(server)
		if err != nil {
			continue
		}

		info.ReferenceTime = refTime
		info.Server = server
		info.LastSyncTime = time.Now()

		// 计算时钟偏差
		localNow := time.Now()
		drift := localNow.Sub(refTime)
		info.Drift = drift
		if drift < 0 {
			info.DriftAbs = -drift
		} else {
			info.DriftAbs = drift
		}

		// 更新统计
		oldInfo := c.currentInfo.Load()
		if oldInfo != nil {
			info.TotalSyncs = oldInfo.TotalSyncs + 1
			info.SuccessSyncs = oldInfo.SuccessSyncs + 1
			info.FailedSyncs = oldInfo.FailedSyncs
			info.CorrectionCount = oldInfo.CorrectionCount
		} else {
			info.TotalSyncs = 1
			info.SuccessSyncs = 1
		}

		// 判断状态
		if info.DriftAbs <= c.config.DriftThreshold {
			info.Status = SyncStatusSynced
			c.status.Store(int32(SyncStatusSynced))
		} else if info.DriftAbs <= c.config.MaxDrift {
			info.Status = SyncStatusDrifted
			c.status.Store(int32(SyncStatusDrifted))

			// 触发漂移告警
			if c.onDrift != nil {
				go c.onDrift(drift, info)
			}

			// 自动校准
			if c.config.AutoCorrect {
				c.correctTime(drift, info)
			}
		} else {
			// 偏差过大，可能是系统时间被手动修改或其他问题
			info.Status = SyncStatusError
			c.status.Store(int32(SyncStatusError))

			if c.onError != nil {
				go c.onError(fmt.Errorf("clock drift too large: %v", info.DriftAbs))
			}
		}

		c.currentInfo.Store(info)
		return
	}

	// 所有服务器都失败
	oldInfo := c.currentInfo.Load()
	if oldInfo != nil {
		info.TotalSyncs = oldInfo.TotalSyncs + 1
		info.FailedSyncs = oldInfo.FailedSyncs + 1
		info.SuccessSyncs = oldInfo.SuccessSyncs
		info.CorrectionCount = oldInfo.CorrectionCount
	} else {
		info.TotalSyncs = 1
		info.FailedSyncs = 1
	}

	c.status.Store(int32(SyncStatusError))
	c.currentInfo.Store(info)

	if c.onError != nil {
		go c.onError(fmt.Errorf("all NTP servers unreachable"))
	}
}

func (c *NTPClient) queryNTP(server string) (time.Time, error) {
	// 使用 ntpdate 或 ntpd 命令查询
	// 首先尝试 ntpdate -q
	out, err := exec.Command("ntpdate", "-q", "-p", "1", "-t",
		strconv.FormatFloat(c.config.Timeout.Seconds(), 'f', 0, 64),
		server).Output()
	if err == nil {
		return c.parseNtpdateOutput(string(out))
	}

	// 回退：使用 sntp
	out, err = exec.Command("sntp", "-t", c.config.Timeout.String(), server).Output()
	if err == nil {
		return c.parseSntpOutput(string(out))
	}

	// 最后尝试：使用 chronyc
	out, err = exec.Command("chronyc", "-h", server, "tracking").Output()
	if err == nil {
		return c.parseChronycOutput(string(out))
	}

	return time.Time{}, fmt.Errorf("failed to query NTP server %s", server)
}

func (c *NTPClient) parseNtpdateOutput(output string) (time.Time, error) {
	// ntpdate -q 输出示例:
	// server 192.168.1.1, stratum 3, offset -0.001234, delay 0.00234
	lines := strings.Split(output, "\n")
	for _, line := range lines {
		if strings.Contains(line, "offset") {
			// 解析 offset
			parts := strings.Fields(line)
			for i, part := range parts {
				if part == "offset," && i+1 < len(parts) {
					offsetStr := strings.TrimSuffix(parts[i+1], ",")
					offset, err := strconv.ParseFloat(offsetStr, 64)
					if err == nil {
						// 根据 offset 计算参考时间
						return time.Now().Add(-time.Duration(offset * float64(time.Second))), nil
					}
				}
			}
		}
	}
	return time.Time{}, fmt.Errorf("failed to parse ntpdate output")
}

func (c *NTPClient) parseSntpOutput(output string) (time.Time, error) {
	// sntp 输出通常包含时间信息
	lines := strings.Split(output, "\n")
	for _, line := range lines {
		if strings.Contains(line, "time") || strings.Contains(line, "T") {
			// 尝试解析时间
			for _, layout := range []string{
				time.RFC3339,
				"2006-01-02 15:04:05",
				"Jan 02 15:04:05",
			} {
				if t, err := time.Parse(layout, strings.TrimSpace(line)); err == nil {
					return t, nil
				}
			}
		}
	}
	return time.Time{}, fmt.Errorf("failed to parse sntp output")
}

func (c *NTPClient) parseChronycOutput(output string) (time.Time, error) {
	// chronyc tracking 输出包含 Reference time
	lines := strings.Split(output, "\n")
	for _, line := range lines {
		if strings.HasPrefix(line, "Reference time") {
			// 解析时间字符串
			parts := strings.SplitN(line, ":", 2)
			if len(parts) == 2 {
				timeStr := strings.TrimSpace(parts[1])
				for _, layout := range []string{
					time.RFC3339,
					"Mon Jan 02 15:04:05 2006",
					"2006-01-02 15:04:05",
				} {
					if t, err := time.Parse(layout, timeStr); err == nil {
						return t, nil
					}
				}
			}
		}
	}
	return time.Time{}, fmt.Errorf("failed to parse chronyc output")
}

func (c *NTPClient) correctTime(drift time.Duration, info *TimeSyncInfo) {
	c.status.Store(int32(SyncStatusCorrecting))

	oldTime := time.Now()

	// 使用 date 或 hwclock 命令修改系统时间
	newTime := info.ReferenceTime
	timeStr := newTime.Format("2006-01-02 15:04:05")

	// 尝试 date 命令
	_, err := exec.Command("date", "-s", timeStr).Output()
	if err != nil {
		// 尝试 timedatectl
		_, err = exec.Command("timedatectl", "set-time", timeStr).Output()
	}

	if err != nil {
		if c.onError != nil {
			go c.onError(fmt.Errorf("failed to correct time: %v", err))
		}
		return
	}

	// 更新校正计数
	info.CorrectionCount++
	c.currentInfo.Store(info)

	if c.onCorrect != nil {
		go c.onCorrect(oldTime, newTime)
	}
}

// ============================================================
// 查询接口
// ============================================================

func (c *NTPClient) GetInfo() *TimeSyncInfo {
	info := c.currentInfo.Load()
	if info == nil {
		return &TimeSyncInfo{Status: SyncStatusUnknown}
	}
	cp := *info
	return &cp
}

func (c *NTPClient) GetStatus() SyncStatus {
	return SyncStatus(c.status.Load())
}

func (c *NTPClient) ForceSync() error {
	if !c.running.Load() {
		return fmt.Errorf("NTP client not running")
	}

	c.performSync()
	return nil
}

func (c *NTPClient) OnDrift(fn func(drift time.Duration, info *TimeSyncInfo)) {
	c.onDrift = fn
}

func (c *NTPClient) OnCorrect(fn func(oldTime, newTime time.Time)) {
	c.onCorrect = fn
}

func (c *NTPClient) OnError(fn func(err error)) {
	c.onError = fn
}

// ============================================================
// 简单 NTP 协议实现（备用）
// ============================================================

// SimpleNTPQuery 使用简单 NTP 协议查询时间（不依赖外部命令）
func SimpleNTPQuery(server string, timeout time.Duration) (time.Time, error) {
	// NTP 协议参考: RFC 5905
	// 构造 NTP 请求包
	req := make([]byte, 48)
	req[0] = 0x1B // LI=0, VN=3, Mode=3 (client)

	addr, err := net.ResolveUDPAddr("udp", net.JoinHostPort(server, "123"))
	if err != nil {
		return time.Time{}, err
	}

	conn, err := net.DialUDP("udp", nil, addr)
	if err != nil {
		return time.Time{}, err
	}
	defer conn.Close()

	conn.SetDeadline(time.Now().Add(timeout))

	// 发送请求
	_, err = conn.Write(req)
	if err != nil {
		return time.Time{}, err
	}

	// 接收响应
	resp := make([]byte, 48)
	_, err = conn.Read(resp)
	if err != nil {
		return time.Time{}, err
	}

	// 解析 NTP 时间戳（从第40字节开始，64位）
	// NTP 时间从 1900-01-01 开始，Unix 时间从 1970-01-01 开始
	// 差值为 2208988800 秒
	seconds := uint64(resp[40])<<24 | uint64(resp[41])<<16 | uint64(resp[42])<<8 | uint64(resp[43])
	fraction := uint64(resp[44])<<24 | uint64(resp[45])<<16 | uint64(resp[46])<<8 | uint64(resp[47])

	ntpEpoch := time.Date(1900, 1, 1, 0, 0, 0, 0, time.UTC)
	unixEpoch := time.Date(1970, 1, 1, 0, 0, 0, 0, time.UTC)
	offset := unixEpoch.Sub(ntpEpoch).Seconds()

	unixSeconds := float64(seconds) - offset
	unixNanos := float64(fraction) * 1e9 / (1 << 32)

	return time.Unix(int64(unixSeconds), int64(unixNanos)).UTC(), nil
}
