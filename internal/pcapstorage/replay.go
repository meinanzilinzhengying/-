// Package pcapstorage 流量回放引擎
// 支持按时间检索、流量回放、倍速控制
package pcapstorage

import (
	"context"
	"fmt"
	"net"
	"sync"
	"time"
)

// ============================================================
// 回放模型
// ============================================================

// ReplaySession 回放会话
type ReplaySession struct {
	ID          string        `json:"id"`
	StartTime   time.Time     `json:"start_time"`
	EndTime     time.Time     `json:"end_time"`
	Query       *Query        `json:"query"`
	Speed       float64       `json:"speed"`        // 回放倍速 (0.5, 1.0, 2.0, 4.0, 8.0, 16.0)
	Status      ReplayStatus  `json:"status"`
	Progress    float64       `json:"progress"`     // 回放进度 (0-100)
	CurrentTime time.Time     `json:"current_time"` // 当前回放时间点
	PacketCount int64         `json:"packet_count"`
	ByteCount   int64         `json:"byte_count"`
	ErrorCount  int64         `json:"error_count"`
	CreatedAt   time.Time     `json:"created_at"`
	UpdatedAt   time.Time     `json:"updated_at"`
}

// ReplayStatus 回放状态
type ReplayStatus string

const (
	ReplayPending   ReplayStatus = "pending"   // 等待中
	ReplayRunning   ReplayStatus = "running"   // 回放中
	ReplayPaused    ReplayStatus = "paused"    // 已暂停
	ReplayCompleted ReplayStatus = "completed" // 已完成
	ReplayFailed    ReplayStatus = "failed"    // 失败
	ReplayCancelled ReplayStatus = "cancelled" // 已取消
)

// ReplayConfig 回放配置
type ReplayConfig struct {
	DefaultSpeed    float64       `json:"default_speed" yaml:"default_speed"`       // 默认倍速
	MinSpeed        float64       `json:"min_speed" yaml:"min_speed"`               // 最小倍速
	MaxSpeed        float64       `json:"max_speed" yaml:"max_speed"`               // 最大倍速
	BufferSize      int           `json:"buffer_size" yaml:"buffer_size"`           // 回放缓冲区大小
	LoopEnabled     bool          `json:"loop_enabled" yaml:"loop_enabled"`         // 循环回放
	FilterRewrite   bool          `json:"filter_rewrite" yaml:"filter_rewrite"`     // 重写过滤
	TargetInterface string        `json:"target_interface" yaml:"target_interface"` // 目标接口
	TargetMAC       string        `json:"target_mac" yaml:"target_mac"`             // 目标MAC
	PauseOnError    bool          `json:"pause_on_error" yaml:"pause_on_error"`     // 错误时暂停
	MaxConcurrent   int           `json:"max_concurrent" yaml:"max_concurrent"`     // 最大并发回放数
}

// DefaultReplayConfig 默认回放配置
func DefaultReplayConfig() *ReplayConfig {
	return &ReplayConfig{
		DefaultSpeed:    1.0,
		MinSpeed:        0.1,
		MaxSpeed:        16.0,
		BufferSize:      1000,
		LoopEnabled:     false,
		FilterRewrite:   false,
		PauseOnError:    false,
		MaxConcurrent:   5,
	}
}

// ReplayHandler 回放数据包处理回调
type ReplayHandler func(packet *PacketRecord) error

// ============================================================
// 回放引擎
// ============================================================

// ReplayEngine 流量回放引擎
type ReplayEngine struct {
	storage    *Engine
	config     *ReplayConfig
	sessions   map[string]*ReplaySession
	sessionMu  sync.RWMutex
	handlers   []ReplayHandler
	handlerMu  sync.RWMutex
	stopCh     map[string]chan struct{}
	pauseCh    map[string]chan struct{}
}

// NewReplayEngine 创建回放引擎
func NewReplayEngine(storage *Engine, config *ReplayConfig) *ReplayEngine {
	if config == nil {
		config = DefaultReplayConfig()
	}
	return &ReplayEngine{
		storage:  storage,
		config:   config,
		sessions: make(map[string]*ReplaySession),
		stopCh:   make(map[string]chan struct{}),
		pauseCh:  make(map[string]chan struct{}),
	}
}

// RegisterHandler 注册回放处理器
func (r *ReplayEngine) RegisterHandler(handler ReplayHandler) {
	r.handlerMu.Lock()
	defer r.handlerMu.Unlock()
	r.handlers = append(r.handlers, handler)
}

// CreateSession 创建回放会话
func (r *ReplayEngine) CreateSession(query *Query, speed float64) (*ReplaySession, error) {
	// 校验倍速
	if speed < r.config.MinSpeed {
		speed = r.config.MinSpeed
	}
	if speed > r.config.MaxSpeed {
		speed = r.config.MaxSpeed
	}

	session := &ReplaySession{
		ID:        generateSessionID(),
		Query:     query,
		Speed:     speed,
		Status:    ReplayPending,
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}

	r.sessionMu.Lock()
	r.sessions[session.ID] = session
	r.stopCh[session.ID] = make(chan struct{})
	r.pauseCh[session.ID] = make(chan struct{})
	r.sessionMu.Unlock()

	return session, nil
}

// StartReplay 开始回放
func (r *ReplayEngine) StartReplay(sessionID string) error {
	r.sessionMu.Lock()
	session, ok := r.sessions[sessionID]
	if !ok {
		r.sessionMu.Unlock()
		return fmt.Errorf("会话不存在: %s", sessionID)
	}

	if session.Status == ReplayRunning {
		r.sessionMu.Unlock()
		return fmt.Errorf("会话已在运行中")
	}

	session.Status = ReplayRunning
	session.StartTime = time.Now()
	session.UpdatedAt = time.Now()
	stopCh := r.stopCh[sessionID]
	pauseCh := r.pauseCh[sessionID]
	r.sessionMu.Unlock()

	// 启动回放协程
	go r.replayLoop(session, stopCh, pauseCh)

	return nil
}

// PauseReplay 暂停回放
func (r *ReplayEngine) PauseReplay(sessionID string) error {
	r.sessionMu.Lock()
	defer r.sessionMu.Unlock()

	session, ok := r.sessions[sessionID]
	if !ok {
		return fmt.Errorf("会话不存在: %s", sessionID)
	}

	if session.Status != ReplayRunning {
		return fmt.Errorf("会话未在运行中")
	}

	session.Status = ReplayPaused
	session.UpdatedAt = time.Now()
	close(r.pauseCh[sessionID])
	r.pauseCh[sessionID] = make(chan struct{})

	return nil
}

// ResumeReplay 恢复回放
func (r *ReplayEngine) ResumeReplay(sessionID string) error {
	r.sessionMu.Lock()
	defer r.sessionMu.Unlock()

	session, ok := r.sessions[sessionID]
	if !ok {
		return fmt.Errorf("会话不存在: %s", sessionID)
	}

	if session.Status != ReplayPaused {
		return fmt.Errorf("会话未在暂停状态")
	}

	session.Status = ReplayRunning
	session.UpdatedAt = time.Now()

	return nil
}

// StopReplay 停止回放
func (r *ReplayEngine) StopReplay(sessionID string) error {
	r.sessionMu.Lock()
	defer r.sessionMu.Unlock()

	session, ok := r.sessions[sessionID]
	if !ok {
		return fmt.Errorf("会话不存在: %s", sessionID)
	}

	if session.Status == ReplayCompleted || session.Status == ReplayCancelled {
		return nil
	}

	session.Status = ReplayCancelled
	session.UpdatedAt = time.Now()
	close(r.stopCh[sessionID])

	return nil
}

// SetSpeed 设置回放倍速
func (r *ReplayEngine) SetSpeed(sessionID string, speed float64) error {
	// 校验倍速
	if speed < r.config.MinSpeed {
		speed = r.config.MinSpeed
	}
	if speed > r.config.MaxSpeed {
		speed = r.config.MaxSpeed
	}

	r.sessionMu.Lock()
	defer r.sessionMu.Unlock()

	session, ok := r.sessions[sessionID]
	if !ok {
		return fmt.Errorf("会话不存在: %s", sessionID)
	}

	session.Speed = speed
	session.UpdatedAt = time.Now()

	return nil
}

// GetSession 获取会话信息
func (r *ReplayEngine) GetSession(sessionID string) (*ReplaySession, error) {
	r.sessionMu.RLock()
	defer r.sessionMu.RUnlock()

	session, ok := r.sessions[sessionID]
	if !ok {
		return nil, fmt.Errorf("会话不存在: %s", sessionID)
	}

	// 返回副本
	return r.cloneSession(session), nil
}

// ListSessions 列出所有会话
func (r *ReplayEngine) ListSessions() []*ReplaySession {
	r.sessionMu.RLock()
	defer r.sessionMu.RUnlock()

	sessions := make([]*ReplaySession, 0, len(r.sessions))
	for _, s := range r.sessions {
		sessions = append(sessions, r.cloneSession(s))
	}
	return sessions
}

// cloneSession 克隆会话
func (r *ReplayEngine) cloneSession(s *ReplaySession) *ReplaySession {
	return &ReplaySession{
		ID:          s.ID,
		StartTime:   s.StartTime,
		EndTime:     s.EndTime,
		Query:       s.Query,
		Speed:       s.Speed,
		Status:      s.Status,
		Progress:    s.Progress,
		CurrentTime: s.CurrentTime,
		PacketCount: s.PacketCount,
		ByteCount:   s.ByteCount,
		ErrorCount:  s.ErrorCount,
		CreatedAt:   s.CreatedAt,
		UpdatedAt:   s.UpdatedAt,
	}
}

// ============================================================
// 回放循环
// ============================================================

func (r *ReplayEngine) replayLoop(session *ReplaySession, stopCh, pauseCh chan struct{}) {
	defer func() {
		r.sessionMu.Lock()
		session.EndTime = time.Now()
		if session.Status == ReplayRunning {
			session.Status = ReplayCompleted
		}
		session.UpdatedAt = time.Now()
		r.sessionMu.Unlock()
	}()

	// 从存储查询数据包
	packets, err := r.storage.Query(session.Query)
	if err != nil {
		session.Status = ReplayFailed
		session.ErrorCount++
		return
	}

	if len(packets) == 0 {
		session.Status = ReplayCompleted
		session.Progress = 100
		return
	}

	totalPackets := int64(len(packets))
	baseTime := packets[0].Timestamp

	for i, packet := range packets {
		select {
		case <-stopCh:
			return

		case <-pauseCh:
			// 等待恢复
			for session.Status == ReplayPaused {
				select {
				case <-time.After(100 * time.Millisecond):
					r.sessionMu.RLock()
					status := session.Status
					r.sessionMu.RUnlock()
					if status != ReplayPaused {
						break
					}
				case <-stopCh:
					return
				}
			}

		default:
		}

		// 计算等待时间
		if i > 0 {
			timeDiff := packet.Timestamp.Sub(baseTime)
			waitDuration := time.Duration(float64(timeDiff) / session.Speed)
			time.Sleep(waitDuration)
		}

		// 更新当前时间点
		session.CurrentTime = packet.Timestamp

		// 执行回放
		if err := r.replayPacket(packet); err != nil {
			session.ErrorCount++
			if r.config.PauseOnError {
				r.PauseReplay(session.ID)
			}
		}

		session.PacketCount++
		session.ByteCount += int64(len(packet.Data))
		session.Progress = float64(i+1) / float64(totalPackets) * 100
		session.UpdatedAt = time.Now()
	}
}

// replayPacket 回放单个数据包
func (r *ReplayEngine) replayPacket(packet *PacketRecord) error {
	r.handlerMu.RLock()
	handlers := make([]ReplayHandler, len(r.handlers))
	copy(handlers, r.handlers)
	r.handlerMu.RUnlock()

	for _, handler := range handlers {
		if err := handler(packet); err != nil {
			return err
		}
	}
	return nil
}

// ============================================================
// 回放处理器
// ============================================================

// RawReplayHandler 原始数据包回放（输出到接口）
type RawReplayHandler struct {
	iface string
	conn  net.PacketConn
}

// NewRawReplayHandler 创建原始回放处理器
func NewRawReplayHandler(iface string) (*RawReplayHandler, error) {
	// 打开原始套接字
	conn, err := net.ListenPacket("ip4:icmp", "0.0.0.0")
	if err != nil {
		return nil, fmt.Errorf("打开原始套接字失败: %w", err)
	}

	return &RawReplayHandler{
		iface: iface,
		conn:  conn,
	}, nil
}

// Handle 处理数据包
func (h *RawReplayHandler) Handle(packet *PacketRecord) error {
	// 简化实现：仅打印信息
	// 实际生产环境需要写入原始套接字
	return nil
}

// Close 关闭处理器
func (h *RawReplayHandler) Close() error {
	if h.conn != nil {
		return h.conn.Close()
	}
	return nil
}

// AnalyzerReplayHandler 分析器回放处理器
type AnalyzerReplayHandler struct {
	analyzers []PacketAnalyzer
}

// PacketAnalyzer 数据包分析器接口
type PacketAnalyzer interface {
	Analyze(packet *PacketRecord) error
	Name() string
}

// NewAnalyzerReplayHandler 创建分析器回放处理器
func NewAnalyzerReplayHandler() *AnalyzerReplayHandler {
	return &AnalyzerReplayHandler{
		analyzers: make([]PacketAnalyzer, 0),
	}
}

// RegisterAnalyzer 注册分析器
func (h *AnalyzerReplayHandler) RegisterAnalyzer(a PacketAnalyzer) {
	h.analyzers = append(h.analyzers, a)
}

// Handle 处理数据包
func (h *AnalyzerReplayHandler) Handle(packet *PacketRecord) error {
	for _, analyzer := range h.analyzers {
		if err := analyzer.Analyze(packet); err != nil {
			return fmt.Errorf("分析器 %s 失败: %w", analyzer.Name(), err)
		}
	}
	return nil
}

// ============================================================
// 统计回放处理器
// ============================================================

// StatsReplayHandler 统计回放处理器
type StatsReplayHandler struct {
	mu          sync.RWMutex
	stats       *ReplayStats
	startTime   time.Time
}

// ReplayStats 回放统计
type ReplayStats struct {
	TotalPackets   int64            `json:"total_packets"`
	TotalBytes     int64            `json:"total_bytes"`
	PacketsPerSec  float64          `json:"packets_per_sec"`
	BytesPerSec    float64          `json:"bytes_per_sec"`
	AvgPacketSize  float64          `json:"avg_packet_size"`
	ProtocolDist   map[string]int64 `json:"protocol_dist"`
	SrcIPDist      map[string]int64 `json:"src_ip_dist"`
	DstIPDist      map[string]int64 `json:"dst_ip_dist"`
	PortDist       map[string]int64 `json:"port_dist"`
}

// NewStatsReplayHandler 创建统计回放处理器
func NewStatsReplayHandler() *StatsReplayHandler {
	return &StatsReplayHandler{
		stats: &ReplayStats{
			ProtocolDist: make(map[string]int64),
			SrcIPDist:    make(map[string]int64),
			DstIPDist:    make(map[string]int64),
			PortDist:     make(map[string]int64),
		},
		startTime: time.Now(),
	}
}

// Handle 处理数据包
func (h *StatsReplayHandler) Handle(packet *PacketRecord) error {
	h.mu.Lock()
	defer h.mu.Unlock()

	h.stats.TotalPackets++
	h.stats.TotalBytes += int64(len(packet.Data))

	// 协议分布
	proto := fmt.Sprintf("%d", packet.Proto)
	h.stats.ProtocolDist[proto]++

	// IP分布
	if packet.SrcIP != "" {
		h.stats.SrcIPDist[packet.SrcIP]++
	}
	if packet.DstIP != "" {
		h.stats.DstIPDist[packet.DstIP]++
	}

	// 端口分布
	if packet.SrcPort != 0 {
		port := fmt.Sprintf("%d", packet.SrcPort)
		h.stats.PortDist[port]++
	}
	if packet.DstPort != 0 {
		port := fmt.Sprintf("%d", packet.DstPort)
		h.stats.PortDist[port]++
	}

	// 计算速率
	elapsed := time.Since(h.startTime).Seconds()
	if elapsed > 0 {
		h.stats.PacketsPerSec = float64(h.stats.TotalPackets) / elapsed
		h.stats.BytesPerSec = float64(h.stats.TotalBytes) / elapsed
	}
	if h.stats.TotalPackets > 0 {
		h.stats.AvgPacketSize = float64(h.stats.TotalBytes) / float64(h.stats.TotalPackets)
	}

	return nil
}

// GetStats 获取统计信息
func (h *StatsReplayHandler) GetStats() *ReplayStats {
	h.mu.RLock()
	defer h.mu.RUnlock()

	// 返回副本
	stats := &ReplayStats{
		TotalPackets:  h.stats.TotalPackets,
		TotalBytes:    h.stats.TotalBytes,
		PacketsPerSec: h.stats.PacketsPerSec,
		BytesPerSec:   h.stats.BytesPerSec,
		AvgPacketSize: h.stats.AvgPacketSize,
		ProtocolDist:  make(map[string]int64),
		SrcIPDist:     make(map[string]int64),
		DstIPDist:     make(map[string]int64),
		PortDist:      make(map[string]int64),
	}

	for k, v := range h.stats.ProtocolDist {
		stats.ProtocolDist[k] = v
	}
	for k, v := range h.stats.SrcIPDist {
		stats.SrcIPDist[k] = v
	}
	for k, v := range h.stats.DstIPDist {
		stats.DstIPDist[k] = v
	}
	for k, v := range h.stats.PortDist {
		stats.PortDist[k] = v
	}

	return stats
}

// Reset 重置统计
func (h *StatsReplayHandler) Reset() {
	h.mu.Lock()
	defer h.mu.Unlock()

	h.stats = &ReplayStats{
		ProtocolDist: make(map[string]int64),
		SrcIPDist:    make(map[string]int64),
		DstIPDist:    make(map[string]int64),
		PortDist:     make(map[string]int64),
	}
	h.startTime = time.Now()
}

// ============================================================
// 工具函数
// ============================================================

var sessionIDCounter uint64
var sessionIDMu sync.Mutex

func generateSessionID() string {
	sessionIDMu.Lock()
	defer sessionIDMu.Unlock()
	sessionIDCounter++
	return fmt.Sprintf("replay_%d_%d", time.Now().Unix(), sessionIDCounter)
}

// ValidateSpeed 校验倍速
func ValidateSpeed(speed float64) bool {
	validSpeeds := []float64{0.1, 0.25, 0.5, 1.0, 2.0, 4.0, 8.0, 10.0, 16.0}
	for _, s := range validSpeeds {
		if speed == s {
			return true
		}
	}
	return false
}

// GetValidSpeeds 获取有效倍速列表
func GetValidSpeeds() []float64 {
	return []float64{0.1, 0.25, 0.5, 1.0, 2.0, 4.0, 8.0, 10.0, 16.0}
}
