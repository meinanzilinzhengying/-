// Package vxlan 提供 VXLAN 隧道解封装能力
// Copyright (c) 2026 Cloud Flow Team
// Licensed under the MIT License.

package vxlan

import (
	"encoding/binary"
	"errors"
	"fmt"
	"net"
	"sync"
	"sync/atomic"
	"time"
)

// ============================================================
// VXLAN 常量定义
// ============================================================

const (
	// VXLAN 默认端口 (RFC 7348)
	DefaultVXLANPort = 4789

	// VXLAN 头部大小
	VXLANHeaderSize = 8

	// VXLAN Flags
	VXLANFlagI = 0x08 // I 位表示 VNI 有效

	// 最大 VNI 值
	MaxVNI = 0xFFFFFF
)

// ============================================================
// VXLAN 头部结构
// ============================================================

// Header VXLAN 头部
type Header struct {
	Flags    uint8  // 标志位
	Reserved uint32 // 保留字段（包含 VNI）
}

// VNI 提取 VXLAN Network Identifier
func (h *Header) VNI() uint32 {
	return h.Reserved >> 8
}

// SetVNI 设置 VNI
func (h *Header) SetVNI(vni uint32) {
	h.Flags = VXLANFlagI
	h.Reserved = (vni << 8) & 0xFFFFFF00
}

// ============================================================
// 解封装后的流量
// ============================================================

// DecapsulatedFlow 解封装后的流量
type DecapsulatedFlow struct {
	// 外层信息（VXLAN 隧道）
	OuterSrcIP   net.IP
	OuterDstIP   net.IP
	OuterSrcPort uint16
	OuterDstPort uint16
	VNI          uint32

	// 内层信息（原始流量）
	InnerSrcMAC   net.HardwareAddr
	InnerDstMAC   net.HardwareAddr
	InnerEtherType uint16 // 0x0800=IPv4, 0x86DD=IPv6
	InnerSrcIP    net.IP
	InnerDstIP    net.IP
	InnerSrcPort  uint16
	InnerDstPort  uint16
	InnerProtocol uint8 // TCP=6, UDP=17, ICMP=1

	// 原始数据
	Payload      []byte // 内层完整以太网帧
	InnerPacket  []byte // 内层 IP 包
	Timestamp    time.Time
	PacketLength uint16
}

// String 返回流量描述
func (df *DecapsulatedFlow) String() string {
	return fmt.Sprintf("VXLAN[VNI=%d] %s:%d -> %s:%d (outer: %s:%d -> %s:%d)",
		df.VNI,
		df.InnerSrcIP, df.InnerSrcPort,
		df.InnerDstIP, df.InnerDstPort,
		df.OuterSrcIP, df.OuterSrcPort,
		df.OuterDstIP, df.OuterDstPort,
	)
}

// ============================================================
// VXLAN 解封装器配置
// ============================================================

// Config VXLAN 解封装器配置
type Config struct {
	Enabled       bool     `yaml:"enabled" json:"enabled"`
	ListenPort    uint16   `yaml:"listen_port" json:"listen_port"`       // 监听的 VXLAN 端口
	FilterVNI     []uint32 `yaml:"filter_vni" json:"filter_vni"`         // 过滤的 VNI 列表（空=全部）
	FilterSrcIP   []string `yaml:"filter_src_ip" json:"filter_src_ip"`   // 过滤的源 IP
	FilterDstIP   []string `yaml:"filter_dst_ip" json:"filter_dst_ip"`   // 过滤的目的 IP
	BufferSize    int      `yaml:"buffer_size" json:"buffer_size"`       // 缓冲区大小
	MaxPacketSize int      `yaml:"max_packet_size" json:"max_packet_size"` // 最大包大小
}

// DefaultConfig 默认配置
func DefaultConfig() *Config {
	return &Config{
		Enabled:       false,
		ListenPort:    DefaultVXLANPort,
		BufferSize:    65535,
		MaxPacketSize: 9000, // Jumbo frame
	}
}

// ============================================================
// VXLAN 解封装器
// ============================================================

// Decapsulator VXLAN 解封装器
type Decapsulator struct {
	config *Config

	// 流量输出通道
	flowChan chan *DecapsulatedFlow

	// 统计
	totalPackets   atomic.Uint64
	validPackets   atomic.Uint64
	invalidPackets atomic.Uint64
	filteredPackets atomic.Uint64
	bytesProcessed atomic.Uint64

	// 过滤器
	vniFilter    map[uint32]bool
	srcIPFilter  map[string]bool
	dstIPFilter  map[string]bool

	mu sync.RWMutex
}

// NewDecapsulator 创建 VXLAN 解封装器
func NewDecapsulator(cfg *Config) *Decapsulator {
	if cfg == nil {
		cfg = DefaultConfig()
	}

	d := &Decapsulator{
		config:     cfg,
		flowChan:   make(chan *DecapsulatedFlow, cfg.BufferSize),
		vniFilter:  make(map[uint32]bool),
		srcIPFilter: make(map[string]bool),
		dstIPFilter: make(map[string]bool),
	}

	// 初始化过滤器
	for _, vni := range cfg.FilterVNI {
		d.vniFilter[vni] = true
	}
	for _, ip := range cfg.FilterSrcIP {
		d.srcIPFilter[ip] = true
	}
	for _, ip := range cfg.FilterDstIP {
		d.dstIPFilter[ip] = true
	}

	return d
}

// Flows 返回解封装流量通道
func (d *Decapsulator) Flows() <-chan *DecapsulatedFlow {
	return d.flowChan
}

// ProcessPacket 处理单个数据包
func (d *Decapsulator) ProcessPacket(data []byte, timestamp time.Time) error {
	d.totalPackets.Add(1)
	d.bytesProcessed.Add(uint64(len(data)))

	// 解析外层 IP 头
	outerIP, offset, err := d.parseOuterIP(data)
	if err != nil {
		d.invalidPackets.Add(1)
		return err
	}

	// 解析外层 UDP 头
	outerUDP, err := d.parseOuterUDP(data[offset:])
	if err != nil {
		d.invalidPackets.Add(1)
		return err
	}
	offset += 8

	// 验证 VXLAN 端口
	if outerUDP.DstPort != d.config.ListenPort {
		d.filteredPackets.Add(1)
		return nil
	}

	// 解析 VXLAN 头
	vxlanHeader, err := d.parseVXLANHeader(data[offset:])
	if err != nil {
		d.invalidPackets.Add(1)
		return err
	}
	offset += VXLANHeaderSize

	// VNI 过滤
	if len(d.vniFilter) > 0 && !d.vniFilter[vxlanHeader.VNI()] {
		d.filteredPackets.Add(1)
		return nil
	}

	// 解析内层以太网帧
	innerFrame := data[offset:]
	innerEth, innerOffset, err := d.parseInnerEthernet(innerFrame)
	if err != nil {
		d.invalidPackets.Add(1)
		return err
	}

	// 解析内层 IP 包
	innerIP, err := d.parseInnerIP(innerFrame[innerOffset:])
	if err != nil {
		d.invalidPackets.Add(1)
		return err
	}

	// IP 过滤
	if len(d.srcIPFilter) > 0 && !d.srcIPFilter[innerIP.SrcIP.String()] {
		d.filteredPackets.Add(1)
		return nil
	}
	if len(d.dstIPFilter) > 0 && !d.dstIPFilter[innerIP.DstIP.String()] {
		d.filteredPackets.Add(1)
		return nil
	}

	// 构建解封装流量
	flow := &DecapsulatedFlow{
		OuterSrcIP:    outerIP.SrcIP,
		OuterDstIP:    outerIP.DstIP,
		OuterSrcPort:  outerUDP.SrcPort,
		OuterDstPort:  outerUDP.DstPort,
		VNI:           vxlanHeader.VNI(),
		InnerSrcMAC:   innerEth.SrcMAC,
		InnerDstMAC:   innerEth.DstMAC,
		InnerEtherType: innerEth.EtherType,
		InnerSrcIP:    innerIP.SrcIP,
		InnerDstIP:    innerIP.DstIP,
		InnerSrcPort:  innerIP.SrcPort,
		InnerDstPort:  innerIP.DstPort,
		InnerProtocol: innerIP.Protocol,
		Payload:       innerFrame,
		InnerPacket:   innerFrame[innerOffset:],
		Timestamp:     timestamp,
		PacketLength:  uint16(len(innerFrame)),
	}

	d.validPackets.Add(1)

	// 发送到通道
	select {
	case d.flowChan <- flow:
	default:
		// 通道满，丢弃
	}

	return nil
}

// ============================================================
// 协议解析
// ============================================================

// outerIPInfo 外层 IP 信息
type outerIPInfo struct {
	SrcIP   net.IP
	DstIP   net.IP
	Protocol uint8
}

// parseOuterIP 解析外层 IP 头
func (d *Decapsulator) parseOuterIP(data []byte) (*outerIPInfo, int, error) {
	if len(data) < 20 {
		return nil, 0, errors.New("packet too short for IP header")
	}

	// IP 版本
	version := data[0] >> 4
	var ipLen int
	var srcIP, dstIP net.IP

	switch version {
	case 4:
		// IPv4
		ipLen = int(data[0]&0x0F) * 4
		if len(data) < ipLen {
			return nil, 0, errors.New("invalid IPv4 header length")
		}
		srcIP = net.IP(data[12:16])
		dstIP = net.IP(data[16:20])
	case 6:
		// IPv6
		ipLen = 40
		if len(data) < ipLen {
			return nil, 0, errors.New("invalid IPv6 header length")
		}
		srcIP = net.IP(data[8:24])
		dstIP = net.IP(data[24:40])
	default:
		return nil, 0, fmt.Errorf("unsupported IP version: %d", version)
	}

	return &outerIPInfo{
		SrcIP:   srcIP,
		DstIP:   dstIP,
		Protocol: data[9],
	}, ipLen, nil
}

// outerUDPInfo 外层 UDP 信息
type outerUDPInfo struct {
	SrcPort uint16
	DstPort uint16
	Length  uint16
}

// parseOuterUDP 解析外层 UDP 头
func (d *Decapsulator) parseOuterUDP(data []byte) (*outerUDPInfo, error) {
	if len(data) < 8 {
		return nil, errors.New("packet too short for UDP header")
	}

	return &outerUDPInfo{
		SrcPort: binary.BigEndian.Uint16(data[0:2]),
		DstPort: binary.BigEndian.Uint16(data[2:4]),
		Length:  binary.BigEndian.Uint16(data[4:6]),
	}, nil
}

// parseVXLANHeader 解析 VXLAN 头
func (d *Decapsulator) parseVXLANHeader(data []byte) (*Header, error) {
	if len(data) < VXLANHeaderSize {
		return nil, errors.New("packet too short for VXLAN header")
	}

	// 检查 I 标志位
	if data[0]&VXLANFlagI == 0 {
		return nil, errors.New("VXLAN I flag not set, invalid VNI")
	}

	// VNI 在 byte 1-3
	vni := uint32(data[1])<<16 | uint32(data[2])<<8 | uint32(data[3])

	return &Header{
		Flags:    data[0],
		Reserved: vni << 8,
	}, nil
}

// innerEthInfo 内层以太网信息
type innerEthInfo struct {
	DstMAC    net.HardwareAddr
	SrcMAC    net.HardwareAddr
	EtherType uint16
}

// parseInnerEthernet 解析内层以太网头
func (d *Decapsulator) parseInnerEthernet(data []byte) (*innerEthInfo, int, error) {
	if len(data) < 14 {
		return nil, 0, errors.New("packet too short for Ethernet header")
	}

	return &innerEthInfo{
		DstMAC:    net.HardwareAddr(data[0:6]),
		SrcMAC:    net.HardwareAddr(data[6:12]),
		EtherType: binary.BigEndian.Uint16(data[12:14]),
	}, 14, nil
}

// innerIPInfo 内层 IP 信息
type innerIPInfo struct {
	SrcIP    net.IP
	DstIP    net.IP
	Protocol uint8
	SrcPort  uint16
	DstPort  uint16
}

// parseInnerIP 解析内层 IP 包
func (d *Decapsulator) parseInnerIP(data []byte) (*innerIPInfo, error) {
	if len(data) < 20 {
		return nil, errors.New("packet too short for inner IP header")
	}

	version := data[0] >> 4
	var ipLen int
	var srcIP, dstIP net.IP
	var protocol uint8

	switch version {
	case 4:
		ipLen = int(data[0]&0x0F) * 4
		if len(data) < ipLen {
			return nil, errors.New("invalid inner IPv4 header length")
		}
		srcIP = net.IP(data[12:16])
		dstIP = net.IP(data[16:20])
		protocol = data[9]
	case 6:
		ipLen = 40
		if len(data) < ipLen {
			return nil, errors.New("invalid inner IPv6 header length")
		}
		srcIP = net.IP(data[8:24])
		dstIP = net.IP(data[24:40])
		protocol = data[6]
	default:
		return nil, fmt.Errorf("unsupported inner IP version: %d", version)
	}

	info := &innerIPInfo{
		SrcIP:    srcIP,
		DstIP:    dstIP,
		Protocol: protocol,
	}

	// 解析端口（仅 TCP/UDP）
	if protocol == 6 || protocol == 17 {
		if len(data) >= ipLen+4 {
			info.SrcPort = binary.BigEndian.Uint16(data[ipLen : ipLen+2])
			info.DstPort = binary.BigEndian.Uint16(data[ipLen+2 : ipLen+4])
		}
	}

	return info, nil
}

// ============================================================
// 统计信息
// ============================================================

// Stats 解封装统计
type Stats struct {
	TotalPackets    uint64 `json:"total_packets"`
	ValidPackets    uint64 `json:"valid_packets"`
	InvalidPackets  uint64 `json:"invalid_packets"`
	FilteredPackets uint64 `json:"filtered_packets"`
	BytesProcessed  uint64 `json:"bytes_processed"`
}

// GetStats 获取统计信息
func (d *Decapsulator) GetStats() Stats {
	return Stats{
		TotalPackets:    d.totalPackets.Load(),
		ValidPackets:    d.validPackets.Load(),
		InvalidPackets:  d.invalidPackets.Load(),
		FilteredPackets: d.filteredPackets.Load(),
		BytesProcessed:  d.bytesProcessed.Load(),
	}
}

// ResetStats 重置统计
func (d *Decapsulator) ResetStats() {
	d.totalPackets.Store(0)
	d.validPackets.Store(0)
	d.invalidPackets.Store(0)
	d.filteredPackets.Store(0)
	d.bytesProcessed.Store(0)
}

// ============================================================
// VNI 过滤管理
// ============================================================

// AddVNIFilter 添加 VNI 过滤
func (d *Decapsulator) AddVNIFilter(vni uint32) {
	d.mu.Lock()
	defer d.mu.Unlock()
	d.vniFilter[vni] = true
}

// RemoveVNIFilter 移除 VNI 过滤
func (d *Decapsulator) RemoveVNIFilter(vni uint32) {
	d.mu.Lock()
	defer d.mu.Unlock()
	delete(d.vniFilter, vni)
}

// ClearVNIFilters 清除所有 VNI 过滤
func (d *Decapsulator) ClearVNIFilters() {
	d.mu.Lock()
	defer d.mu.Unlock()
	d.vniFilter = make(map[uint32]bool)
}

// ============================================================
// VXLAN 封装（用于测试或转发）
// ============================================================

// Encapsulator VXLAN 封装器
type Encapsulator struct {
	srcIP   net.IP
	dstIP   net.IP
	srcPort uint16
	dstPort uint16
	vni     uint32
}

// NewEncapsulator 创建 VXLAN 封装器
func NewEncapsulator(srcIP, dstIP net.IP, srcPort, dstPort uint16, vni uint32) *Encapsulator {
	return &Encapsulator{
		srcIP:   srcIP,
		dstIP:   dstIP,
		srcPort: srcPort,
		dstPort: dstPort,
		vni:     vni,
	}
}

// Encapsulate 封装以太网帧
func (e *Encapsulator) Encapsulate(ethernetFrame []byte) ([]byte, error) {
	// 构建 VXLAN 头 (8 bytes)
	vxlanHeader := make([]byte, VXLANHeaderSize)
	vxlanHeader[0] = VXLANFlagI
	vxlanHeader[1] = byte(e.vni >> 16)
	vxlanHeader[2] = byte(e.vni >> 8)
	vxlanHeader[3] = byte(e.vni)

	// 构建 UDP 头 (8 bytes)
	udpHeader := make([]byte, 8)
	binary.BigEndian.PutUint16(udpHeader[0:2], e.srcPort)
	binary.BigEndian.PutUint16(udpHeader[2:4], e.dstPort)
	udpLen := uint16(8 + VXLANHeaderSize + len(ethernetFrame))
	binary.BigEndian.PutUint16(udpHeader[4:6], udpLen)
	// Checksum 可选

	// 构建 IP 头 (简化，仅 IPv4)
	ipHeader := make([]byte, 20)
	ipHeader[0] = 0x45 // IPv4, 20 bytes header
	binary.BigEndian.PutUint16(ipHeader[2:4], uint16(20+8+VXLANHeaderSize+len(ethernetFrame)))
	ipHeader[8] = 64 // TTL
	ipHeader[9] = 17 // UDP
	copy(ipHeader[12:16], e.srcIP.To4())
	copy(ipHeader[16:20], e.dstIP.To4())

	// 组装完整包
	result := make([]byte, 0, len(ipHeader)+len(udpHeader)+len(vxlanHeader)+len(ethernetFrame))
	result = append(result, ipHeader...)
	result = append(result, udpHeader...)
	result = append(result, vxlanHeader...)
	result = append(result, ethernetFrame...)

	return result, nil
}

// ============================================================
// 辅助函数
// ============================================================

// IsVXLANPort 检查是否为 VXLAN 端口
func IsVXLANPort(port uint16) bool {
	return port == DefaultVXLANPort || port == 8472 // 8472 是一些实现使用的端口
}

// ParseVNI 从 VXLAN 头解析 VNI
func ParseVNI(data []byte) (uint32, error) {
	if len(data) < VXLANHeaderSize {
		return 0, errors.New("data too short")
	}
	return uint32(data[1])<<16 | uint32(data[2])<<8 | uint32(data[3]), nil
}
