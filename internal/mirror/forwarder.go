// Package mirror 提供流量镜像与转发能力，支持将解封装后的流量镜像至云下环境
// Copyright (c) 2026 Cloud Flow Team
// Licensed under the MIT License.

package mirror

import (
	"context"
	"encoding/binary"
	"errors"
	"fmt"
	"net"
	"sync"
	"sync/atomic"
	"time"

	"github.com/meinanzilinzhengying/cloud-flow-agent/internal/vxlan"
)

// ============================================================
// 镜像模式
// ============================================================

// MirrorMode 镜像模式
type MirrorMode string

const (
	MirrorModeRaw      MirrorMode = "raw"       // 原始以太网帧
	MirrorModeGRE      MirrorMode = "gre"       // GRE 隧道封装
	MirrorModeVXLAN    MirrorMode = "vxlan"     // VXLAN 封装
	MirrorModeUDP      MirrorMode = "udp"       // UDP 封装
	MirrorModeERSPAN   MirrorMode = "erspan"    // ERSPAN (Encapsulated Remote Switch Port Analyzer)
)

// ============================================================
// 镜像目标配置
// ============================================================

// TargetConfig 镜像目标配置
type TargetConfig struct {
	Name        string     `yaml:"name" json:"name"`
	Mode        MirrorMode `yaml:"mode" json:"mode"`
	Address     string     `yaml:"address" json:"address"`         // 目标地址
	Port        uint16     `yaml:"port" json:"port"`               // 目标端口
	VNI         uint32     `yaml:"vni" json:"vni"`                 // VXLAN VNI (VXLAN模式)
	GREKey      uint32     `yaml:"gre_key" json:"gre_key"`         // GRE Key (GRE模式)
	ERSPANID    uint32     `yaml:"erspan_id" json:"erspan_id"`     // ERSPAN Session ID
	Enabled     bool       `yaml:"enabled" json:"enabled"`
	BufferSize  int        `yaml:"buffer_size" json:"buffer_size"` // 发送缓冲区大小
}

// ============================================================
// 镜像器配置
// ============================================================

// Config 镜像器配置
type Config struct {
	Enabled      bool           `yaml:"enabled" json:"enabled"`
	SourceIP     string         `yaml:"source_ip" json:"source_ip"`       // 源 IP（用于封装）
	SourcePort   uint16         `yaml:"source_port" json:"source_port"`   // 源端口
	QueueSize    int            `yaml:"queue_size" json:"queue_size"`     // 队列大小
	BatchSize    int            `yaml:"batch_size" json:"batch_size"`     // 批量发送大小
	BatchTimeout time.Duration  `yaml:"batch_timeout" json:"batch_timeout"` // 批量发送超时
	Targets      []TargetConfig `yaml:"targets" json:"targets"`           // 镜像目标列表
}

// DefaultConfig 默认配置
func DefaultConfig() *Config {
	return &Config{
		Enabled:      false,
		SourcePort:   4789,
		QueueSize:    10000,
		BatchSize:    100,
		BatchTimeout: 100 * time.Millisecond,
	}
}

// ============================================================
// 镜像目标
// ============================================================

// Target 镜像目标
type Target struct {
	config    *TargetConfig
	conn      net.Conn
	udpConn   *net.UDPConn
	encapsulator *vxlan.Encapsulator

	// 统计
	packetsSent atomic.Uint64
	bytesSent   atomic.Uint64
	errors      atomic.Uint64

	mu sync.Mutex
}

// NewTarget 创建镜像目标
func NewTarget(cfg *TargetConfig, sourceIP string, sourcePort uint16) (*Target, error) {
	t := &Target{
		config: cfg,
	}

	switch cfg.Mode {
	case MirrorModeVXLAN:
		t.encapsulator = vxlan.NewEncapsulator(
			net.ParseIP(sourceIP),
			net.ParseIP(cfg.Address),
			sourcePort,
			cfg.Port,
			cfg.VNI,
		)
	}

	return t, nil
}

// Connect 建立连接
func (t *Target) Connect() error {
	t.mu.Lock()
	defer t.mu.Unlock()

	switch t.config.Mode {
	case MirrorModeRaw, MirrorModeGRE, MirrorModeERSPAN:
		// TCP 连接
		addr := fmt.Sprintf("%s:%d", t.config.Address, t.config.Port)
		conn, err := net.DialTimeout("tcp", addr, 10*time.Second)
		if err != nil {
			return err
		}
		t.conn = conn

	case MirrorModeVXLAN, MirrorModeUDP:
		// UDP 连接
		dstAddr, err := net.ResolveUDPAddr("udp", fmt.Sprintf("%s:%d", t.config.Address, t.config.Port))
		if err != nil {
			return err
		}
		srcAddr := &net.UDPAddr{Port: int(t.config.Port)}
		conn, err := net.DialUDP("udp", srcAddr, dstAddr)
		if err != nil {
			return err
		}
		t.udpConn = conn
	}

	return nil
}

// Close 关闭连接
func (t *Target) Close() error {
	t.mu.Lock()
	defer t.mu.Unlock()

	if t.conn != nil {
		t.conn.Close()
	}
	if t.udpConn != nil {
		t.udpConn.Close()
	}
	return nil
}

// Send 发送数据
func (t *Target) Send(data []byte) error {
	t.mu.Lock()
	defer t.mu.Unlock()

	var err error
	var n int

	switch t.config.Mode {
	case MirrorModeRaw:
		n, err = t.conn.Write(data)

	case MirrorModeGRE:
		encapsulated := t.encapsulateGRE(data)
		n, err = t.conn.Write(encapsulated)

	case MirrorModeVXLAN:
		if t.encapsulator != nil {
			encapsulated, encErr := t.encapsulator.Encapsulate(data)
			if encErr != nil {
				return encErr
			}
			n, err = t.udpConn.Write(encapsulated)
		}

	case MirrorModeUDP:
		n, err = t.udpConn.Write(data)

	case MirrorModeERSPAN:
		encapsulated := t.encapsulateERSPAN(data)
		n, err = t.conn.Write(encapsulated)
	}

	if err != nil {
		t.errors.Add(1)
		return err
	}

	t.packetsSent.Add(1)
	t.bytesSent.Add(uint64(n))
	return nil
}

// encapsulateGRE GRE 封装
func (t *Target) encapsulateGRE(data []byte) []byte {
	// GRE 头 (4 bytes basic + 4 bytes key)
	header := make([]byte, 8)
	// Flags: set Key Present bit
	binary.BigEndian.PutUint16(header[0:2], 0x2000) // K bit set
	binary.BigEndian.PutUint16(header[2:4], 0x0800) // Protocol type: IPv4
	binary.BigEndian.PutUint32(header[4:8], t.config.GREKey)

	result := make([]byte, 0, len(header)+len(data))
	result = append(result, header...)
	result = append(result, data...)
	return result
}

// encapsulateERSPAN ERSPAN 封装
func (t *Target) encapsulateERSPAN(data []byte) []byte {
	// 简化实现：GRE + ERSPAN header
	greHeader := make([]byte, 8)
	binary.BigEndian.PutUint16(greHeader[0:2], 0x1000) // Checksum present
	binary.BigEndian.PutUint16(greHeader[2:4], 0x88BE) // ERSPAN type
	binary.BigEndian.PutUint32(greHeader[4:8], t.config.ERSPANID)

	// ERSPAN header (8 bytes)
	erspanHeader := make([]byte, 8)
	erspanHeader[0] = 0x10 // Version 1
	binary.BigEndian.PutUint16(erspanHeader[2:4], uint16(t.config.ERSPANID))

	result := make([]byte, 0, len(greHeader)+len(erspanHeader)+len(data))
	result = append(result, greHeader...)
	result = append(result, erspanHeader...)
	result = append(result, data...)
	return result
}

// Stats 获取统计
func (t *Target) Stats() TargetStats {
	return TargetStats{
		Name:        t.config.Name,
		PacketsSent: t.packetsSent.Load(),
		BytesSent:   t.bytesSent.Load(),
		Errors:      t.errors.Load(),
	}
}

// TargetStats 目标统计
type TargetStats struct {
	Name        string `json:"name"`
	PacketsSent uint64 `json:"packets_sent"`
	BytesSent   uint64 `json:"bytes_sent"`
	Errors      uint64 `json:"errors"`
}

// ============================================================
// 流量镜像器
// ============================================================

// Forwarder 流量镜像转发器
type Forwarder struct {
	config *Config

	// 镜像目标
	targets map[string]*Target
	mu      sync.RWMutex

	// 数据队列
	queue chan *vxlan.DecapsulatedFlow

	// 生命周期
	ctx    context.Context
	cancel context.CancelFunc
	wg     sync.WaitGroup
	running atomic.Bool

	// 统计
	totalReceived  atomic.Uint64
	totalForwarded atomic.Uint64
	totalDropped   atomic.Uint64

	// 回调
	onForward func(target string, flow *vxlan.DecapsulatedFlow)
	onError   func(target string, err error)
}

// NewForwarder 创建流量镜像转发器
func NewForwarder(cfg *Config) (*Forwarder, error) {
	if cfg == nil {
		cfg = DefaultConfig()
	}

	ctx, cancel := context.WithCancel(context.Background())

	f := &Forwarder{
		config:  cfg,
		targets: make(map[string]*Target),
		queue:   make(chan *vxlan.DecapsulatedFlow, cfg.QueueSize),
		ctx:     ctx,
		cancel:  cancel,
	}

	// 初始化目标
	for i := range cfg.Targets {
		tc := cfg.Targets[i]
		if !tc.Enabled {
			continue
		}
		target, err := NewTarget(&tc, cfg.SourceIP, cfg.SourcePort)
		if err != nil {
			continue
		}
		f.targets[tc.Name] = target
	}

	return f, nil
}

// Start 启动转发器
func (f *Forwarder) Start() error {
	if f.running.Load() {
		return errors.New("forwarder already running")
	}

	// 连接所有目标
	for name, target := range f.targets {
		if err := target.Connect(); err != nil {
			// 连接失败，移除目标
			delete(f.targets, name)
		}
	}

	f.running.Store(true)

	// 启动转发协程
	f.wg.Add(1)
	go f.forwardLoop()

	return nil
}

// Stop 停止转发器
func (f *Forwarder) Stop() error {
	if !f.running.Load() {
		return nil
	}

	f.running.Store(false)
	f.cancel()
	f.wg.Wait()

	// 关闭所有目标连接
	for _, target := range f.targets {
		target.Close()
	}

	close(f.queue)
	return nil
}

// Mirror 镜像流量
func (f *Forwarder) Mirror(flow *vxlan.DecapsulatedFlow) error {
	if !f.running.Load() {
		return errors.New("forwarder not running")
	}

	f.totalReceived.Add(1)

	select {
	case f.queue <- flow:
		return nil
	default:
		f.totalDropped.Add(1)
		return errors.New("queue full")
	}
}

// MirrorBatch 批量镜像
func (f *Forwarder) MirrorBatch(flows []*vxlan.DecapsulatedFlow) error {
	for _, flow := range flows {
		if err := f.Mirror(flow); err != nil {
			return err
		}
	}
	return nil
}

// forwardLoop 转发循环
func (f *Forwarder) forwardLoop() {
	defer f.wg.Done()

	batch := make([]*vxlan.DecapsulatedFlow, 0, f.config.BatchSize)
	ticker := time.NewTicker(f.config.BatchTimeout)
	defer ticker.Stop()

	for {
		select {
		case <-f.ctx.Done():
			// 发送剩余数据
			if len(batch) > 0 {
				f.sendBatch(batch)
			}
			return

		case flow, ok := <-f.queue:
			if !ok {
				return
			}
			batch = append(batch, flow)

			if len(batch) >= f.config.BatchSize {
				f.sendBatch(batch)
				batch = batch[:0]
			}

		case <-ticker.C:
			if len(batch) > 0 {
				f.sendBatch(batch)
				batch = batch[:0]
			}
		}
	}
}

// sendBatch 批量发送
func (f *Forwarder) sendBatch(flows []*vxlan.DecapsulatedFlow) {
	f.mu.RLock()
	targets := make([]*Target, 0, len(f.targets))
	for _, t := range f.targets {
		targets = append(targets, t)
	}
	f.mu.RUnlock()

	for _, flow := range flows {
		data := flow.Payload

		for _, target := range targets {
			if err := target.Send(data); err != nil {
				if f.onError != nil {
					f.onError(target.config.Name, err)
				}
			} else {
				f.totalForwarded.Add(1)
				if f.onForward != nil {
					f.onForward(target.config.Name, flow)
				}
			}
		}
	}
}

// ============================================================
// 目标管理
// ============================================================

// AddTarget 添加镜像目标
func (f *Forwarder) AddTarget(cfg *TargetConfig) error {
	f.mu.Lock()
	defer f.mu.Unlock()

	target, err := NewTarget(cfg, f.config.SourceIP, f.config.SourcePort)
	if err != nil {
		return err
	}

	if f.running.Load() {
		if err := target.Connect(); err != nil {
			return err
		}
	}

	f.targets[cfg.Name] = target
	return nil
}

// RemoveTarget 移除镜像目标
func (f *Forwarder) RemoveTarget(name string) error {
	f.mu.Lock()
	defer f.mu.Unlock()

	target, ok := f.targets[name]
	if !ok {
		return fmt.Errorf("target %s not found", name)
	}

	target.Close()
	delete(f.targets, name)
	return nil
}

// GetTarget 获取镜像目标
func (f *Forwarder) GetTarget(name string) (*Target, bool) {
	f.mu.RLock()
	defer f.mu.RUnlock()
	target, ok := f.targets[name]
	return target, ok
}

// ListTargets 列出所有目标
func (f *Forwarder) ListTargets() []string {
	f.mu.RLock()
	defer f.mu.RUnlock()

	names := make([]string, 0, len(f.targets))
	for name := range f.targets {
		names = append(names, name)
	}
	return names
}

// ============================================================
// 统计信息
// ============================================================

// Stats 获取统计信息
func (f *Forwarder) Stats() ForwarderStats {
	f.mu.RLock()
	targetStats := make(map[string]TargetStats, len(f.targets))
	for name, target := range f.targets {
		targetStats[name] = target.Stats()
	}
	f.mu.RUnlock()

	return ForwarderStats{
		Running:        f.running.Load(),
		TotalReceived:  f.totalReceived.Load(),
		TotalForwarded: f.totalForwarded.Load(),
		TotalDropped:   f.totalDropped.Load(),
		TargetCount:    len(f.targets),
		Targets:        targetStats,
	}
}

// ForwarderStats 转发器统计
type ForwarderStats struct {
	Running        bool                    `json:"running"`
	TotalReceived  uint64                  `json:"total_received"`
	TotalForwarded uint64                  `json:"total_forwarded"`
	TotalDropped   uint64                  `json:"total_dropped"`
	TargetCount    int                     `json:"target_count"`
	Targets        map[string]TargetStats  `json:"targets"`
}

// ============================================================
// 回调注册
// ============================================================

// OnForward 注册转发回调
func (f *Forwarder) OnForward(fn func(target string, flow *vxlan.DecapsulatedFlow)) {
	f.onForward = fn
}

// OnError 注册错误回调
func (f *Forwarder) OnError(fn func(target string, err error)) {
	f.onError = fn
}

// ============================================================
// 与 VXLAN 解封装器集成
// ============================================================

// ConnectToDecapsulator 连接到 VXLAN 解封装器
func (f *Forwarder) ConnectToDecapsulator(d *vxlan.Decapsulator) {
	go func() {
		for flow := range d.Flows() {
			f.Mirror(flow)
		}
	}()
}

// ============================================================
// 流量过滤与采样
// ============================================================

// FilterConfig 流量过滤配置
type FilterConfig struct {
	SrcIPs      []string `yaml:"src_ips" json:"src_ips"`
	DstIPs      []string `yaml:"dst_ips" json:"dst_ips"`
	SrcPorts    []uint16 `yaml:"src_ports" json:"src_ports"`
	DstPorts    []uint16 `yaml:"dst_ports" json:"dst_ports"`
	Protocols   []uint8  `yaml:"protocols" json:"protocols"`     // 6=TCP, 17=UDP
	VNIs        []uint32 `yaml:"vnis" json:"vnis"`
	SampleRate  int      `yaml:"sample_rate" json:"sample_rate"` // 1-100
}

// Filter 流量过滤器
type Filter struct {
	config    *FilterConfig
	srcIPs    map[string]bool
	dstIPs    map[string]bool
	srcPorts  map[uint16]bool
	dstPorts  map[uint16]bool
	protocols map[uint8]bool
	vnis      map[uint32]bool
	counter   atomic.Uint64
}

// NewFilter 创建过滤器
func NewFilter(cfg *FilterConfig) *Filter {
	f := &Filter{
		config:    cfg,
		srcIPs:    make(map[string]bool),
		dstIPs:    make(map[string]bool),
		srcPorts:  make(map[uint16]bool),
		dstPorts:  make(map[uint16]bool),
		protocols: make(map[uint8]bool),
		vnis:      make(map[uint32]bool),
	}

	for _, ip := range cfg.SrcIPs {
		f.srcIPs[ip] = true
	}
	for _, ip := range cfg.DstIPs {
		f.dstIPs[ip] = true
	}
	for _, port := range cfg.SrcPorts {
		f.srcPorts[port] = true
	}
	for _, port := range cfg.DstPorts {
		f.dstPorts[port] = true
	}
	for _, proto := range cfg.Protocols {
		f.protocols[proto] = true
	}
	for _, vni := range cfg.VNIs {
		f.vnis[vni] = true
	}

	return f
}

// Match 检查流量是否匹配
func (f *Filter) Match(flow *vxlan.DecapsulatedFlow) bool {
	// 采样检查
	if f.config.SampleRate > 0 && f.config.SampleRate < 100 {
		f.counter.Add(1)
		if f.counter.Load()%uint64(100/f.config.SampleRate) != 0 {
			return false
		}
	}

	// 源 IP 过滤
	if len(f.srcIPs) > 0 && !f.srcIPs[flow.InnerSrcIP.String()] {
		return false
	}

	// 目的 IP 过滤
	if len(f.dstIPs) > 0 && !f.dstIPs[flow.InnerDstIP.String()] {
		return false
	}

	// 源端口过滤
	if len(f.srcPorts) > 0 && !f.srcPorts[flow.InnerSrcPort] {
		return false
	}

	// 目的端口过滤
	if len(f.dstPorts) > 0 && !f.dstPorts[flow.InnerDstPort] {
		return false
	}

	// 协议过滤
	if len(f.protocols) > 0 && !f.protocols[flow.InnerProtocol] {
		return false
	}

	// VNI 过滤
	if len(f.vnis) > 0 && !f.vnis[flow.VNI] {
		return false
	}

	return true
}

// FilteredForwarder 带过滤的转发器
type FilteredForwarder struct {
	forwarder *Forwarder
	filter    *Filter
}

// NewFilteredForwarder 创建带过滤的转发器
func NewFilteredForwarder(forwarder *Forwarder, filter *Filter) *FilteredForwarder {
	return &FilteredForwarder{
		forwarder: forwarder,
		filter:    filter,
	}
}

// Mirror 带过滤的镜像
func (ff *FilteredForwarder) Mirror(flow *vxlan.DecapsulatedFlow) error {
	if !ff.filter.Match(flow) {
		return nil
	}
	return ff.forwarder.Mirror(flow)
}

// ConnectToDecapsulator 连接到解封装器
func (ff *FilteredForwarder) ConnectToDecapsulator(d *vxlan.Decapsulator) {
	go func() {
		for flow := range d.Flows() {
			ff.Mirror(flow)
		}
	}()
}
