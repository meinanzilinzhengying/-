// Package protocol - Apache Dubbo 协议解析器
// Copyright (c) 2026 Cloud Flow Team
// Licensed under the MIT License.

package protocol

import (
	"encoding/binary"
	"fmt"
	"time"
)

// Dubbo 常量
const (
	DubboDefaultPort     = 20880
	DubboMagicHigh       = 0xda
	DubboMagicLow        = 0xbb
	DubboHeaderSize      = 16

	// 序列化类型
	DubboSerializationFastJSON = 6
	DubboSerializationHessian  = 2
	DubboSerializationJava     = 3
	DubboSerializationProtobuf = 7

	// 事件类型
	DubboEventHeartbeat = 20 // 0x14
	DubboEventRequest   = 0
	DubboEventResponse  = 1

	// 响应状态
	DubboResponseStatusOK       = 20
	DubboResponseStatusClientErr = 30
	DubboResponseStatusServerErr = 31
	DubboResponseStatusTimeout   = 32
)

// Dubbo 序列化名称映射
var dubboSerializationNames = map[byte]string{
	1: "fasterxml",
	2: "hessian2",
	3: "java",
	4: "compacted_java",
	5: "fastjson",
	6: "native_java",
	7: "protobuf",
	8: "kryo",
	9: "fst",
	10: "gson",
}

// DubboParser Dubbo 协议解析器
type DubboParser struct{}

// NewDubboParser 创建 Dubbo 解析器
func NewDubboParser() *DubboParser { return &DubboParser{} }

func (p *DubboParser) Name() string          { return "dubbo" }
func (p *DubboParser) Protocol() ProtocolType { return ProtocolDubbo }
func (p *DubboParser) Ports() []uint16        { return []uint16{20880, 20881, 20882} }

// Detect 检测 Dubbo 协议
func (p *DubboParser) Detect(srcPort, dstPort uint16, payload []byte) bool {
	portMatch := false
	for _, port := range p.Ports() {
		if srcPort == port || dstPort == port {
			portMatch = true
			break
		}
	}
	if !portMatch || len(payload) < DubboHeaderSize {
		return false
	}

	// Dubbo 魔数: 0xdabb
	if payload[0] != DubboMagicHigh || payload[1] != DubboMagicLow {
		return false
	}

	// 序列化类型检查 (byte 2)
	serialType := payload[2] & 0x1f
	if serialType > 15 {
		return false
	}

	return true
}

// Parse 解析 Dubbo 报文
func (p *DubboParser) Parse(payload []byte, direction MsgDirection) (*ProtocolMessage, error) {
	if len(payload) < DubboHeaderSize {
		return nil, fmt.Errorf("payload too short for Dubbo header: %d bytes", len(payload))
	}

	msg := &ProtocolMessage{
		Protocol:  ProtocolDubbo,
		Direction: direction,
		Timestamp: time.Now(),
		RawLength: len(payload),
		Attributes: make(map[string]interface{}),
	}

	// 解析 Dubbo 头部
	header := parseDubboHeader(payload)
	for k, v := range header {
		msg.Attributes[k] = v
	}

	isRequest := header["is_request"].(bool)
	eventType := header["event_type"].(int)

	if eventType == DubboEventHeartbeat {
		msg.Operation = "HEARTBEAT"
		msg.Success = true
		return msg, nil
	}

	if isRequest {
		p.parseRequest(payload, header, msg)
	} else {
		p.parseResponse(payload, header, msg)
	}

	msg.RawPreview = SafePreview(payload[DubboHeaderSize:], 100)
	return msg, nil
}

// parseDubboHeader 解析 Dubbo 头部
func parseDubboHeader(data []byte) map[string]interface{} {
	header := make(map[string]interface{})

	// Magic
	header["magic"] = fmt.Sprintf("0x%02x%02x", data[0], data[1])

	// Serialization type (低5位)
	serialType := data[2] & 0x1f
	header["serialization_type"] = serialType
	if name, ok := dubboSerializationNames[serialType]; ok {
		header["serialization_name"] = name
	}

	// 请求/响应标志 (bit 5 of byte 2)
	header["is_request"] = (data[2]&0x80) == 0

	// 双向标志 (bit 6 of byte 2)
	header["two_way"] = (data[2]&0x40) != 0

	// 事件类型 (bit 7 of byte 2)
	eventType := 0
	if (data[2] & 0x20) != 0 {
		eventType = DubboEventHeartbeat
	}
	header["event_type"] = eventType

	// 响应状态 (byte 3)
	if !((data[2] & 0x80) == 0) {
		status := int(data[3])
		header["status"] = status
		header["status_desc"] = dubboStatusDesc(status)
	}

	// Request ID (8 bytes, bytes 4-11)
	reqID := int64(binary.BigEndian.Uint64(data[4:12]))
	header["request_id"] = reqID

	// Data length (4 bytes, bytes 12-15)
	dataLen := binary.BigEndian.Uint32(data[12:16])
	header["data_length"] = dataLen

	return header
}

// parseRequest 解析 Dubbo 请求
func (p *DubboParser) parseRequest(data []byte, header map[string]interface{}, msg *ProtocolMessage) {
	msg.Operation = "REQUEST"

	if len(data) <= DubboHeaderSize {
		return
	}

	body := data[DubboHeaderSize:]
	serialType := header["serialization_type"].(byte)

	// Dubbo 请求体编码格式因序列化方式不同而异
	// Hessian2: path (string) + version (string) + method (string) + parameterTypesDesc (string) + args
	switch serialType {
	case DubboSerializationHessian, DubboSerializationFastJSON:
		p.parseHessianRequest(body, msg)
	default:
		// 尝试提取可读信息
		msg.Attributes["body_preview"] = SafePreview(body, 200)
	}

	msg.Success = true
}

// parseHessianRequest 解析 Hessian 序列化的 Dubbo 请求
func (p *DubboParser) parseHessianRequest(body []byte, msg *ProtocolMessage) {
	offset := 0

	// Dubbo path (服务接口名)
	path, nextOffset := readHessianString(body, offset)
	if path != "" {
		msg.Resource = path
		msg.Attributes["service"] = path
		offset = nextOffset
	}

	// Dubbo version
	version, nextOffset := readHessianString(body, offset)
	if version != "" {
		msg.Attributes["version"] = version
		offset = nextOffset
	}

	// Method name
	method, nextOffset := readHessianString(body, offset)
	if method != "" {
		msg.Operation = method
		msg.Attributes["method"] = method
		offset = nextOffset
	}

	// Parameter types descriptor
	paramTypes, nextOffset := readHessianString(body, offset)
	if paramTypes != "" {
		msg.Attributes["param_types"] = paramTypes
		offset = nextOffset
	}

	// Arguments (简化，仅记录预览)
	if offset < len(body) {
		msg.Attributes["args_preview"] = SafePreview(body[offset:], 200)
	}
}

// parseResponse 解析 Dubbo 响应
func (p *DubboParser) parseResponse(data []byte, header map[string]interface{}, msg *ProtocolMessage) {
	msg.Operation = "RESPONSE"

	status, ok := header["status"].(int)
	if !ok {
		msg.Success = true
		return
	}

	switch {
	case status == DubboResponseStatusOK:
		msg.Success = true
		msg.Operation = "RESPONSE_OK"
	case status >= DubboResponseStatusClientErr && status <= DubboResponseStatusTimeout:
		msg.Success = false
		msg.Operation = "RESPONSE_ERROR"
		msg.ErrorCode = fmt.Sprintf("DUBBO_%d", status)
		msg.ErrorMsg = dubboStatusDesc(status)
	default:
		msg.Success = true
	}

	if len(data) > DubboHeaderSize {
		body := data[DubboHeaderSize:]
		msg.Attributes["body_preview"] = SafePreview(body, 200)
	}
}

// readHessianString 读取 Hessian 字符串
// Hessian2 字符串格式: type byte + length + data
// 'S' (0x53): 长字符串 (2字节长度)
// s (0x73): 短字符串 (1字节长度)
// 直接 UTF-8: 以长度开头的字符串
func readHessianString(data []byte, offset int) (string, int) {
	if offset >= len(data) {
		return "", offset
	}

	b := data[offset]

	switch {
	case b == 0x53: // 'S' - 长字符串
		if offset+3 > len(data) {
			return "", offset + 1
		}
		strLen := int(binary.BigEndian.Uint16(data[offset+1 : offset+3]))
		offset += 3
		if strLen > 0 && offset+strLen <= len(data) {
			return string(data[offset : offset+strLen]), offset + strLen
		}
		return "", offset

	case b == 0x73: // 's' - 短字符串
		if offset+2 > len(data) {
			return "", offset + 1
		}
		strLen := int(data[offset+1])
		offset += 2
		if strLen > 0 && offset+strLen <= len(data) {
			return string(data[offset : offset+strLen]), offset + strLen
		}
		return "", offset

	case b >= 0x00 && b <= 0x1f:
		// 直接 UTF-8 字符串（长度编码在低5位）
		strLen := int(b)
		offset++
		if strLen > 0 && offset+strLen <= len(data) {
			return string(data[offset : offset+strLen]), offset + strLen
		}
		return "", offset

	case b >= 0x20 && b <= 0x2f:
		// 短 ASCII 字符串（长度 = b - 0x20）
		strLen := int(b - 0x20)
		offset++
		if strLen > 0 && offset+strLen <= len(data) {
			return string(data[offset : offset+strLen]), offset + strLen
		}
		return "", offset

	default:
		// 未知类型，尝试作为普通字符串读取
		return "", offset + 1
	}
}

// dubboStatusDesc Dubbo 状态码描述
func dubboStatusDesc(status int) string {
	descs := map[int]string{
		20: "OK",
		30: "CLIENT_ERROR",
		31: "SERVER_ERROR",
		32: "TIMEOUT",
		33: "BAD_REQUEST",
		34: "FORBIDDEN",
		35: "LIMIT_EXCEEDED",
	}
	if desc, ok := descs[status]; ok {
		return desc
	}
	return fmt.Sprintf("UNKNOWN_%d", status)
}
