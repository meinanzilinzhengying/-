// Package protocol 提供统一的协议解析插件框架
// 支持 Oracle TNS、PostgreSQL、Redis RESP、Kafka、Dubbo 等协议深度解析
// Copyright (c) 2026 Cloud Flow Team
// Licensed under the MIT License.

package protocol

import (
	"context"
	"fmt"
	"sync"
	"sync/atomic"
	"time"
)

// ============================================================
// 协议解析统一接口
// ============================================================

// ProtocolType 协议类型
type ProtocolType string

const (
	ProtocolOracle    ProtocolType = "oracle"
	ProtocolPostgreSQL ProtocolType = "postgresql"
	ProtocolRedis     ProtocolType = "redis"
	ProtocolKafka     ProtocolType = "kafka"
	ProtocolDubbo     ProtocolType = "dubbo"
)

// MsgDirection 消息方向
type MsgDirection string

const (
	DirectionRequest  MsgDirection = "request"
	DirectionResponse MsgDirection = "response"
	DirectionError    MsgDirection = "error"
)

// Parser 协议解析器接口
type Parser interface {
	// Name 解析器名称
	Name() string

	// Protocol 协议类型
	Protocol() ProtocolType

	// Detect 检测是否为该协议（基于端口和报文特征）
	Detect(srcPort, dstPort uint16, payload []byte) bool

	// Parse 解析协议报文
	Parse(payload []byte, direction MsgDirection) (*ProtocolMessage, error)

	// Ports 返回默认识别端口
	Ports() []uint16
}

// ProtocolMessage 协议解析结果
type ProtocolMessage struct {
	Protocol   ProtocolType  `json:"protocol"`
	Direction  MsgDirection  `json:"direction"`
	Timestamp  time.Time     `json:"timestamp"`

	// 通用字段
	Operation  string        `json:"operation"`   // 操作类型 (SELECT/INSERT/GET/SET...)
	Resource   string        `json:"resource"`    // 资源标识 (表名/Key/Topic/Service...)
	Duration   time.Duration `json:"duration"`    // 请求耗时
	Success    bool          `json:"success"`     // 是否成功
	ErrorCode  string        `json:"error_code"`  // 错误码
	ErrorMsg   string        `json:"error_msg"`   // 错误信息

	// 协议特有字段
	Attributes map[string]interface{} `json:"attributes,omitempty"`

	// 原始数据摘要
	RawLength  int    `json:"raw_length"`
	RawPreview string `json:"raw_preview,omitempty"`
}

// ============================================================
// 插件管理器
// ============================================================

// Manager 协议解析插件管理器
type Manager struct {
	parsers map[ProtocolType]Parser
	portMap map[uint16][]ProtocolType // 端口 → 协议映射

	// 统计
	totalParsed  atomic.Uint64
	totalFailed  atomic.Uint64
	perProtocol  map[ProtocolType]*atomic.Uint64

	mu sync.RWMutex
}

// NewManager 创建插件管理器
func NewManager() *Manager {
	return &Manager{
		parsers:    make(map[ProtocolType]Parser),
		portMap:    make(map[uint16][]ProtocolType),
		perProtocol: make(map[ProtocolType]*atomic.Uint64),
	}
}

// Register 注册协议解析器
func (m *Manager) Register(p Parser) {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.parsers[p.Protocol()] = p
	m.perProtocol[p.Protocol()] = &atomic.Uint64{}

	// 建立端口映射
	for _, port := range p.Ports() {
		m.portMap[port] = append(m.portMap[port], p.Protocol())
	}
}

// Unregister 注销解析器
func (m *Manager) Unregister(pt ProtocolType) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if p, ok := m.parsers[pt]; ok {
		for _, port := range p.Ports() {
			var filtered []ProtocolType
			for _, proto := range m.portMap[port] {
				if proto != pt {
					filtered = append(filtered, proto)
				}
			}
			m.portMap[port] = filtered
		}
		delete(m.parsers, pt)
		delete(m.perProtocol, pt)
	}
}

// Parse 自动识别并解析协议报文
func (m *Manager) Parse(srcPort, dstPort uint16, payload []byte, direction MsgDirection) (*ProtocolMessage, error) {
	parser := m.detectParser(srcPort, dstPort, payload)
	if parser == nil {
		return nil, fmt.Errorf("no parser detected for ports %d->%d", srcPort, dstPort)
	}

	msg, err := parser.Parse(payload, direction)
	if err != nil {
		m.totalFailed.Add(1)
		return nil, err
	}

	m.totalParsed.Add(1)
	if counter, ok := m.perProtocol[parser.Protocol()]; ok {
		counter.Add(1)
	}

	return msg, nil
}

// Detect 检测协议类型
func (m *Manager) Detect(srcPort, dstPort uint16, payload []byte) ProtocolType {
	parser := m.detectParser(srcPort, dstPort, payload)
	if parser == nil {
		return ""
	}
	return parser.Protocol()
}

// detectParser 检测解析器
func (m *Manager) detectParser(srcPort, dstPort uint16, payload []byte) Parser {
	m.mu.RLock()
	defer m.mu.RUnlock()

	// 1. 先按端口匹配
	candidates := m.portMap[srcPort]
	if len(candidates) == 0 {
		candidates = m.portMap[dstPort]
	}

	if len(candidates) > 0 {
		for _, pt := range candidates {
			if p, ok := m.parsers[pt]; ok {
				if p.Detect(srcPort, dstPort, payload) {
					return p
				}
			}
		}
	}

	// 2. 全量特征匹配
	for _, p := range m.parsers {
		if p.Detect(srcPort, dstPort, payload) {
			return p
		}
	}

	return nil
}

// GetParser 获取指定协议解析器
func (m *Manager) GetParser(pt ProtocolType) (Parser, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	p, ok := m.parsers[pt]
	return p, ok
}

// ListParsers 列出所有解析器
func (m *Manager) ListParsers() []Parser {
	m.mu.RLock()
	defer m.mu.RUnlock()
	result := make([]Parser, 0, len(m.parsers))
	for _, p := range m.parsers {
		result = append(result, p)
	}
	return result
}

// Stats 获取统计
func (m *Manager) Stats() map[string]uint64 {
	m.mu.RLock()
	defer m.mu.RUnlock()

	stats := map[string]uint64{
		"total_parsed": m.totalParsed.Load(),
		"total_failed": m.totalFailed.Load(),
	}
	for pt, counter := range m.perProtocol {
		stats[string(pt)] = counter.Load()
	}
	return stats
}

// ============================================================
// 辅助函数
// ============================================================

// ReadBytes 读取指定长度的字节
func ReadBytes(data []byte, offset, length int) []byte {
	if offset+length > len(data) {
		return data[offset:]
	}
	return data[offset : offset+length]
}

// ReadUint16 读取大端 uint16
func ReadUint16(data []byte, offset int) uint16 {
	if offset+2 > len(data) {
		return 0
	}
	return uint16(data[offset])<<8 | uint16(data[offset+1])
}

// ReadUint32 读取大端 uint32
func ReadUint32(data []byte, offset int) uint32 {
	if offset+4 > len(data) {
		return 0
	}
	return uint32(data[offset])<<24 | uint32(data[offset+1])<<16 |
		uint32(data[offset+2])<<8 | uint32(data[offset+3])
}

// ReadInt32 读取大端 int32
func ReadInt32(data []byte, offset int) int32 {
	return int32(ReadUint32(data, offset))
}

// ReadString 读取 C 风格字符串
func ReadCString(data []byte, offset int) (string, int) {
	end := offset
	for end < len(data) && data[end] != 0 {
		end++
	}
	return string(data[offset:end]), end + 1
}

// ReadStringN 读取指定长度字符串
func ReadStringN(data []byte, offset, length int) string {
	if offset+length > len(data) {
		return string(data[offset:])
	}
	return string(data[offset : offset+length])
}

// SafePreview 安全预览（截取前N个字符，非打印字符替换）
func SafePreview(data []byte, maxLen int) string {
	if len(data) == 0 {
		return ""
	}
	if maxLen <= 0 || maxLen > len(data) {
		maxLen = len(data)
	}
	if maxLen > 200 {
		maxLen = 200
	}

	result := make([]byte, 0, maxLen)
	for i := 0; i < maxLen; i++ {
		b := data[i]
		if b >= 32 && b < 127 {
			result = append(result, b)
		} else {
			result = append(result, '.')
		}
	}
	return string(result)
}

// ============================================================
// 协议解析上下文
// ============================================================

// ParseContext 解析上下文（用于关联请求和响应）
type ParseContext struct {
	ctx        context.Context
	Connection string // 连接标识
	StartTime  time.Time
	Requests   map[string]*ProtocolMessage // 请求ID → 请求消息
	mu         sync.RWMutex
}

// NewParseContext 创建解析上下文
func NewParseContext(connID string) *ParseContext {
	return &ParseContext{
		ctx:      context.Background(),
		Connection: connID,
		Requests: make(map[string]*ProtocolMessage),
	}
}

// StoreRequest 存储请求
func (pc *ParseContext) StoreRequest(reqID string, msg *ProtocolMessage) {
	pc.mu.Lock()
	defer pc.mu.Unlock()
	msg.Timestamp = time.Now()
	pc.Requests[reqID] = msg
}

// MatchResponse 匹配响应
func (pc *ParseContext) MatchResponse(reqID string) (*ProtocolMessage, bool) {
	pc.mu.RLock()
	defer pc.mu.RUnlock()
	req, ok := pc.Requests[reqID]
	return req, ok
}

// RemoveRequest 移除请求
func (pc *ParseContext) RemoveRequest(reqID string) {
	pc.mu.Lock()
	defer pc.mu.Unlock()
	delete(pc.Requests, reqID)
}

// ============================================================
// RegisterAllParsers 注册所有内置解析器
// ============================================================

// RegisterAllParsers 注册所有内置协议解析器
func RegisterAllParsers(m *Manager) {
	m.Register(NewOracleParser())
	m.Register(NewPostgreSQLParser())
	m.Register(NewRedisParser())
	m.Register(NewKafkaParser())
	m.Register(NewDubboParser())
}
