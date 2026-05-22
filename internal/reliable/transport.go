// Package reliable 可靠传输
// 重传/确认/去重机制，确保数据传输准确性
package reliable

import (
	"crypto/sha256"
	"encoding/binary"
	"fmt"
	"sync"
	"sync/atomic"
	"time"
)

// ============================================================
// 序列号
// ============================================================

// Sequence 序列号
type Sequence struct {
	SeqNum uint64
	TsMs   uint64
}

// NewSequence 创建新序列号
func NewSequence() Sequence {
	return Sequence{
		SeqNum: 0,
		TsMs:   uint64(time.Now().UnixMilli()),
	}
}

// Next 获取下一个序列号
func (s *Sequence) Next() uint64 {
	return atomic.AddUint64(&s.SeqNum, 1)
}

// ============================================================
// 数据包
// ============================================================

// Packet 数据包
type Packet struct {
	SeqNum     uint64    `json:"seq_num"`
	TsMs       uint64   `json:"ts_ms"`
	Data       []byte    `json:"data"`
	DataHash   []byte    `json:"data_hash"`    // 数据哈希
	IsLast     bool      `json:"is_last"`      // 是否最后一个包
	BatchID    uint64    `json:"batch_id"`     // 批次ID
	TotalPackets uint8   `json:"total_packets"` // 总包数
	CurrentIdx uint8     `json:"current_idx"`   // 当前索引
}

// ACK 确认包
type ACK struct {
	SeqNum    uint64 `json:"seq_num"`
	TsMs     uint64 `json:"ts_ms"`
	Status   ACKStatus `json:"status"` // 0=成功, 1=失败, 2=重传请求
	Info     string `json:"info,omitempty"`
}

// ACKStatus ACK状态
type ACKStatus uint8

const (
	ACKSuccess ACKStatus = iota
	ACKFail
	ACKRetry
)

// NewPacket 创建新数据包
func NewPacket(data []byte, batchID uint64) *Packet {
	hash := sha256.Sum256(data)
	
	return &Packet{
		SeqNum:   0,
		TsMs:     uint64(time.Now().UnixMilli()),
		Data:     data,
		DataHash: hash[:],
		BatchID:  batchID,
	}
}

// SetSeq 设置序列号
func (p *Packet) SetSeq(seq uint64) {
	p.SeqNum = seq
}

// GenerateID 生成唯一ID
func (p *Packet) GenerateID() []byte {
	id := make([]byte, 24)
	binary.BigEndian.PutUint64(id[0:8], p.SeqNum)
	binary.BigEndian.PutUint64(id[8:16], p.TsMs)
	binary.BigEndian.PutUint32(id[16:20], p.BatchID)
	hash := sha256.Sum256(p.Data)
	copy(id[20:24], hash[:4])
	return id
}

// ============================================================
// 发送窗口
// ============================================================

// SendWindow 发送窗口
type SendWindow struct {
	windowSize int
	packets    map[uint64]*Packet
	acked      map[uint64]bool
	pending    map[uint64]*PendingPacket
	mu         sync.RWMutex
	
	// 回调
	onTimeout func(seq uint64)
	onacked func(seq uint64)
	
	// 统计
	stats *SendStats
}

// PendingPacket 待确认包
type PendingPacket struct {
	Packet   *Packet
	SendTime time.Time
	Retries  int
}

// SendStats 发送统计
type SendStats struct {
	TotalSent     int64
	TotalAcked   int64
	TotalRetried int64
	TotalDropped int64
	AvgRttMs     float64
}

// NewSendWindow 创建发送窗口
func NewSendWindow(windowSize int) *SendWindow {
	if windowSize <= 0 {
		windowSize = 256
	}
	
	return &SendWindow{
		windowSize: windowSize,
		packets:   make(map[uint64]*Packet),
		acked:     make(map[uint64]bool),
		pending:   make(map[uint64]*PendingPacket),
		stats:     &SendStats{},
	}
}

// Push 添加包到窗口
func (w *SendWindow) Push(packet *Packet) {
	w.mu.Lock()
	defer w.mu.Unlock()
	
	seq := packet.SeqNum
	w.packets[seq] = packet
	w.pending[seq] = &PendingPacket{
		Packet:   packet,
		SendTime: time.Now(),
	}
	
	atomic.AddInt64(&w.stats.TotalSent, 1)
}

// Send 发送包
func (w *SendWindow) Send(seq uint64) *Packet {
	w.mu.RLock()
	defer w.mu.RUnlock()
	
	return w.packets[seq]
}

// IsSent 检查包是否已发送但未确认
func (w *SendWindow) IsSent(seq uint64) bool {
	w.mu.RLock()
	defer w.mu.RUnlock()
	
	_, pending := w.pending[seq]
	_, acked := w.acked[seq]
	return pending && !acked
}

// Ack 处理ACK
func (w *SendWindow) Ack(ack *ACK) bool {
	w.mu.Lock()
	defer w.mu.Unlock()
	
	seq := ack.SeqNum
	
	if ack.Status == ACKSuccess {
		// 成功确认
		delete(w.pending, seq)
		w.acked[seq] = true
		atomic.AddInt64(&w.stats.TotalAcked, 1)
		
		if w.onacked != nil {
			go w.onacked(seq)
		}
		
		return true
	}
	
	if ack.Status == ACKRetry {
		// 重传请求
		if pending, ok := w.pending[seq]; ok {
			pending.Retries++
			pending.SendTime = time.Now()
			atomic.AddInt64(&w.stats.TotalRetried, 1)
		}
		return false
	}
	
	return false
}

// GetPending 获取待确认包列表
func (w *SendWindow) GetPending() []*PendingPacket {
	w.mu.RLock()
	defer w.mu.RUnlock()
	
	result := make([]*PendingPacket, 0, len(w.pending))
	for _, p := range w.pending {
		result = append(result, p)
	}
	return result
}

// GetStats 获取统计
func (w *SendWindow) GetStats() *SendStats {
	return w.stats
}

// OnTimeout 设置超时回调
func (w *SendWindow) OnTimeout(f func(seq uint64)) {
	w.onTimeout = f
}

// OnAcked 设置确认回调
func (w *SendWindow) OnAcked(f func(seq uint64)) {
	w.onacked = f
}

// ============================================================
// 接收窗口
// ============================================================

// RecvWindow 接收窗口
type RecvWindow struct {
	windowSize int
	received   map[uint64]*Packet
	delivered  map[uint64]bool
	nextSeq    uint64
	mu         sync.RWMutex
	
	// 回调
	onDeliver func(packet *Packet)
	
	// 去重
	dedupe *DedupeCache
	
	// 统计
	stats *RecvStats
}

// RecvStats 接收统计
type RecvStats struct {
	TotalReceived   int64
	TotalDelivered int64
	TotalDuplicate int64
	TotalLost      int64
}

// NewRecvWindow 创建接收窗口
func NewRecvWindow(windowSize int) *RecvWindow {
	if windowSize <= 0 {
		windowSize = 256
	}
	
	return &RecvWindow{
		windowSize: windowSize,
		received:   make(map[uint64]*Packet),
		delivered:  make(map[uint64]bool),
		nextSeq:    1,
		dedupe:     NewDedupeCache(10000, 300), // 1万缓存，5分钟去重窗口
		stats:      &RecvStats{},
	}
}

// Receive 接收包
func (w *RecvWindow) Receive(packet *Packet) (delivered bool, isDuplicate bool) {
	w.mu.Lock()
	defer w.mu.Unlock()
	
	seq := packet.SeqNum
	
	// 检查去重
	packetID := string(packet.GenerateID())
	if w.dedupe.IsDuplicate(packetID) {
		atomic.AddInt64(&w.stats.TotalDuplicate, 1)
		return false, true
	}
	
	// 检查是否已接收
	if w.delivered[seq] {
		atomic.AddInt64(&w.stats.TotalDuplicate, 1)
		return false, true
	}
	
	// 存储包
	w.received[seq] = packet
	
	// 按序交付
	delivered = false
	for {
		p, ok := w.received[w.nextSeq]
		if !ok {
			break
		}
		
		delete(w.received, w.nextSeq)
		w.delivered[w.nextSeq] = true
		w.nextSeq++
		delivered = true
		
		atomic.AddInt64(&w.stats.TotalDelivered, 1)
		
		if w.onDeliver != nil {
			go w.onDeliver(p)
		}
	}
	
	// 检查丢包
	w.checkLost()
	
	atomic.AddInt64(&w.stats.TotalReceived, 1)
	return delivered, false
}

// checkLost 检查丢包
func (w *RecvWindow) checkLost() {
	// 简化实现：只检查连续的丢包
	if len(w.received) == 0 {
		return
	}
	
	minSeq := uint64(0)
	for seq := range w.received {
		if minSeq == 0 || seq < minSeq {
			minSeq = seq
		}
	}
	
	if minSeq > w.nextSeq {
		lost := minSeq - w.nextSeq
		atomic.AddInt64(&w.stats.TotalLost, int64(lost))
	}
}

// GenerateACK 生成ACK
func (w *RecvWindow) GenerateACK(seq uint64, status ACKStatus, info string) *ACK {
	return &ACK{
		SeqNum:  seq,
		TsMs:   uint64(time.Now().UnixMilli()),
		Status: status,
		Info:   info,
	}
}

// GetStats 获取统计
func (w *RecvWindow) GetStats() *RecvStats {
	return w.stats
}

// OnDeliver 设置交付回调
func (w *RecvWindow) OnDeliver(f func(packet *Packet)) {
	w.onDeliver = f
}

// ============================================================
// 去重缓存
// ============================================================

// DedupeCache 去重缓存
type DedupeCache struct {
	items    map[string]*dedupeEntry
	mu       sync.RWMutex
	maxSize  int
	windowSec int
}

type dedupeEntry struct {
	ts time.Time
}

// NewDedupeCache 创建去重缓存
func NewDedupeCache(maxSize, windowSec int) *DedupeCache {
	return &DedupeCache{
		items:     make(map[string]*dedupeEntry),
		maxSize:  maxSize,
		windowSec: windowSec,
	}
}

// IsDuplicate 检查是否重复
func (c *DedupeCache) IsDuplicate(id string) bool {
	c.mu.RLock()
	_, exists := c.items[id]
	c.mu.RUnlock()
	
	if exists {
		return true
	}
	
	c.mu.Lock()
	defer c.mu.Unlock()
	
	// 再次检查
	if _, exists := c.items[id]; exists {
		return true
	}
	
	// 添加新条目
	c.items[id] = &dedupeEntry{ts: time.Now()}
	
	// 清理过期条目
	c.cleanup()
	
	return false
}

// cleanup 清理过期条目
func (c *DedupeCache) cleanup() {
	now := time.Now()
	expire := now.Add(-time.Duration(c.windowSec) * time.Second)
	
	for id, entry := range c.items {
		if entry.ts.Before(expire) {
			delete(c.items, id)
		}
	}
	
	// 如果还是太大，删除最老的
	if len(c.items) > c.maxSize {
		var oldestKey string
		var oldestTime time.Time
		for id, entry := range c.items {
			if oldestTime.IsZero() || entry.ts.Before(oldestTime) {
				oldestKey = id
				oldestTime = entry.ts
			}
		}
		if oldestKey != "" {
			delete(c.items, oldestKey)
		}
	}
}

// ============================================================
// 可靠传输器
// ============================================================

// ReliableTransport 可靠传输器
type ReliableTransport struct {
	config   *Config
	sendWin  *SendWindow
	recvWin  *RecvWindow
	seq      Sequence
	stopCh   chan struct{}
	wg       sync.WaitGroup
	
	// 回调
	onSend    func(packets []*Packet) error
	onReceive func(ack *ACK) error
}

// Config 可靠传输配置
type Config struct {
	WindowSize    int  `yaml:"window_size" json:"window_size"`       // 滑动窗口大小
	RetryCount    int  `yaml:"retry_count" json:"retry_count"`       // 最大重传次数
	RetryInterval int  `yaml:"retry_interval" json:"retry_interval"` // 重传间隔（毫秒）
	AckTimeout    int  `yaml:"ack_timeout" json:"ack_timeout"`       // ACK超时（毫秒）
	DedupeEnabled bool `yaml:"dedupe_enabled" json:"dedupe_enabled"` // 启用去重
	DedupeSize    int  `yaml:"dedupe_size" json:"dedupe_size"`       // 去重缓存大小
	DedupeWindow  int  `yaml:"dedupe_window" json:"dedupe_window"`    // 去重窗口（秒）
}

// DefaultConfig 默认配置
func DefaultConfig() *Config {
	return &Config{
		WindowSize:    256,
		RetryCount:   3,
		RetryInterval: 100,
		AckTimeout:   5000,
		DedupeEnabled: true,
		DedupeSize:   10000,
		DedupeWindow: 300,
	}
}

// NewReliableTransport 创建可靠传输器
func NewReliableTransport(config *Config) *ReliableTransport {
	if config == nil {
		config = DefaultConfig()
	}
	
	t := &ReliableTransport{
		config:  config,
		sendWin: NewSendWindow(config.WindowSize),
		recvWin: NewRecvWindow(config.WindowSize),
		seq:     NewSequence(),
		stopCh:  make(chan struct{}),
	}
	
	// 设置回调
	t.sendWin.OnAcked(func(seq uint64) {
		t.handleAcked(seq)
	})
	
	return t
}

// SetSendFunc 设置发送函数
func (t *ReliableTransport) SetSendFunc(f func(packets []*Packet) error) {
	t.onSend = f
}

// SetReceiveFunc 设置接收函数
func (t *ReliableTransport) SetReceiveFunc(f func(ack *ACK) error) {
	t.onReceive = f
}

// Send 发送数据（可靠）
func (t *ReliableTransport) Send(data []byte) error {
	seq := t.seq.Next()
	
	packet := NewPacket(data, 0)
	packet.SetSeq(seq)
	
	// 添加到发送窗口
	t.sendWin.Push(packet)
	
	// 发送
	if t.onSend != nil {
		if err := t.onSend([]*Packet{packet}); err != nil {
			return fmt.Errorf("发送失败: %w", err)
		}
	}
	
	return nil
}

// HandleACK 处理ACK
func (t *ReliableTransport) HandleACK(ack *ACK) {
	t.sendWin.Ack(ack)
}

// HandlePacket 处理接收到的包
func (t *ReliableTransport) HandlePacket(packet *Packet) {
	delivered, isDup := t.recvWin.Receive(packet)
	
	// 发送ACK
	var status ACKStatus
	var info string
	
	if isDup {
		status = ACKSuccess // 重复包不需要重传
		info = "duplicate"
	} else if delivered {
		status = ACKSuccess
		info = "ok"
	} else {
		status = ACKSuccess // 缓存后等待
		info = "buffered"
	}
	
	ack := &ACK{
		SeqNum:  packet.SeqNum,
		TsMs:    uint64(time.Now().UnixMilli()),
		Status:  status,
		Info:    info,
	}
	
	if t.onReceive != nil {
		t.onReceive(ack)
	}
}

// handleAcked 处理确认
func (t *ReliableTransport) handleAcked(seq uint64) {
	// 回调通知上层
}

// GetSendStats 获取发送统计
func (t *ReliableTransport) GetSendStats() *SendStats {
	return t.sendWin.GetStats()
}

// GetRecvStats 获取接收统计
func (t *ReliableTransport) GetRecvStats() *RecvStats {
	return t.recvWin.GetStats()
}

// Close 关闭
func (t *ReliableTransport) Close() {
	close(t.stopCh)
	t.wg.Wait()
}
