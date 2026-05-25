// Package protocol - Kafka 协议解析器
// Copyright (c) 2026 Cloud Flow Team
// Licensed under the MIT License.

package protocol

import (
	"encoding/binary"
	"fmt"
	"time"
)

// Kafka 常量
const (
	KafkaDefaultPort = 9092

	// API Keys
	KafkaProduce      int16 = 0
	KafkaFetch        int16 = 1
	KafkaListOffsets  int16 = 2
	KafkaMetadata     int16 = 3
	KafkaOffsetCommit int16 = 8
	KafkaOffsetFetch  int16 = 9
	KafkaFindCoord    int16 = 10
	KafkaJoinGroup    int16 = 11
	KafkaHeartbeat    int16 = 12
	KafkaLeaveGroup   int16 = 13
	KafkaSyncGroup    int16 = 14
	KafkaDescribe     int16 = 15
	KafkaCreateTopics int16 = 19
	KafkaAPIVersions  int16 = 18
)

// Kafka API Key 名称映射
var kafkaAPIKeyNames = map[int16]string{
	0:  "PRODUCE",
	1:  "FETCH",
	2:  "LIST_OFFSETS",
	3:  "METADATA",
	8:  "OFFSET_COMMIT",
	9:  "OFFSET_FETCH",
	10: "FIND_COORDINATOR",
	11: "JOIN_GROUP",
	12: "HEARTBEAT",
	13: "LEAVE_GROUP",
	14: "SYNC_GROUP",
	15: "DESCRIBE_GROUPS",
	18: "API_VERSIONS",
	19: "CREATE_TOPICS",
	20: "DELETE_TOPICS",
}

// KafkaParser Kafka 协议解析器
type KafkaParser struct{}

// NewKafkaParser 创建 Kafka 解析器
func NewKafkaParser() *KafkaParser { return &KafkaParser{} }

func (p *KafkaParser) Name() string          { return "kafka" }
func (p *KafkaParser) Protocol() ProtocolType { return ProtocolKafka }
func (p *KafkaParser) Ports() []uint16        { return []uint16{9092, 9093, 9094} }

// Detect 检测 Kafka 协议
func (p *KafkaParser) Detect(srcPort, dstPort uint16, payload []byte) bool {
	portMatch := false
	for _, port := range p.Ports() {
		if srcPort == port || dstPort == port {
			portMatch = true
			break
		}
	}
	if !portMatch || len(payload) < 10 {
		return false
	}

	// Kafka 请求格式：
	// int32: message_size (remaining bytes)
	// int16: api_key
	// int16: api_version
	// int32: correlation_id
	// int16: client_id_length
	// bytes: client_id

	msgSize := binary.BigEndian.Uint32(payload[0:4])
	if msgSize < 6 || msgSize > uint32(len(payload)) {
		return false
	}

	apiKey := int16(binary.BigEndian.Uint16(payload[4:6]))
	apiVersion := int16(binary.BigEndian.Uint16(payload[6:8]))

	// API Key 应在已知范围内
	if _, ok := kafkaAPIKeyNames[apiKey]; !ok {
		return false
	}

	// API Version 应合理（0-12）
	if apiVersion < 0 || apiVersion > 12 {
		return false
	}

	return true
}

// Parse 解析 Kafka 报文
func (p *KafkaParser) Parse(payload []byte, direction MsgDirection) (*ProtocolMessage, error) {
	if len(payload) < 10 {
		return nil, fmt.Errorf("payload too short for Kafka: %d bytes", len(payload))
	}

	msg := &ProtocolMessage{
		Protocol:  ProtocolKafka,
		Direction: direction,
		Timestamp: time.Now(),
		RawLength: len(payload),
		Attributes: make(map[string]interface{}),
	}

	if direction == DirectionRequest {
		p.parseRequest(payload, msg)
	} else {
		p.parseResponse(payload, msg)
	}

	msg.RawPreview = SafePreview(payload, 100)
	return msg, nil
}

// parseRequest 解析 Kafka 请求
func (p *KafkaParser) parseRequest(data []byte, msg *ProtocolMessage) {
	offset := 0

	// message_size (4 bytes)
	msgSize := binary.BigEndian.Uint32(data[offset : offset+4])
	msg.Attributes["message_size"] = msgSize
	offset += 4

	// api_key (2 bytes)
	apiKey := int16(binary.BigEndian.Uint16(data[offset : offset+2]))
	msg.Attributes["api_key"] = apiKey
	offset += 2

	// api_version (2 bytes)
	apiVersion := int16(binary.BigEndian.Uint16(data[offset : offset+2]))
	msg.Attributes["api_version"] = apiVersion
	offset += 2

	// correlation_id (4 bytes)
	correlationID := binary.BigEndian.Uint32(data[offset : offset+4])
	msg.Attributes["correlation_id"] = correlationID
	offset += 4

	// client_id (2 bytes length + string)
	if offset+2 <= len(data) {
		clientIDLen := int(binary.BigEndian.Uint16(data[offset : offset+2]))
		offset += 2
		if clientIDLen > 0 && offset+clientIDLen <= len(data) {
			msg.Attributes["client_id"] = string(data[offset : offset+clientIDLen])
			offset += clientIDLen
		}
	}

	// API 名称
	apiName := kafkaAPIKeyNames[apiKey]
	if apiName == "" {
		apiName = fmt.Sprintf("API_%d", apiKey)
	}
	msg.Operation = apiName

	// 根据不同 API 解析
	switch apiKey {
	case KafkaProduce:
		p.parseProduceRequest(data, offset, apiVersion, msg)
	case KafkaFetch:
		p.parseFetchRequest(data, offset, apiVersion, msg)
	case KafkaMetadata:
		p.parseMetadataRequest(data, offset, apiVersion, msg)
	case KafkaJoinGroup:
		p.parseGroupRequest(data, offset, apiVersion, msg, "JOIN_GROUP")
	case KafkaHeartbeat:
		msg.Resource = "heartbeat"
	case KafkaOffsetCommit:
		msg.Resource = "offset_commit"
	}

	msg.Success = true
}

// parseProduceRequest 解析 Produce 请求
func (p *KafkaParser) parseProduceRequest(data []byte, offset int, version int16, msg *ProtocolMessage) {
	if offset+6 > len(data) {
		return
	}

	// transactional_id_length (2 bytes)
	txnIDLen := int(binary.BigEndian.Uint16(data[offset : offset+2]))
	offset += 2 + txnIDLen

	// acks (2 bytes)
	if offset+2 <= len(data) {
		acks := int16(binary.BigEndian.Uint16(data[offset : offset+2]))
		msg.Attributes["acks"] = acks
		offset += 2
	}

	// timeout (4 bytes)
	if offset+4 <= len(data) {
		timeout := binary.BigEndian.Uint32(data[offset : offset+4])
		msg.Attributes["timeout_ms"] = timeout
		offset += 4
	}

	// num_topics (4 bytes)
	if offset+4 <= len(data) {
		numTopics := int(binary.BigEndian.Uint32(data[offset : offset+4]))
		msg.Attributes["topic_count"] = numTopics
		offset += 4

		// 提取第一个 topic name
		if numTopics > 0 && offset+2 <= len(data) {
			topicLen := int(binary.BigEndian.Uint16(data[offset : offset+2]))
			offset += 2
			if topicLen > 0 && offset+topicLen <= len(data) {
				msg.Resource = string(data[offset : offset+topicLen])
			}
		}
	}
}

// parseFetchRequest 解析 Fetch 请求
func (p *KafkaParser) parseFetchRequest(data []byte, offset int, version int16, msg *ProtocolMessage) {
	if offset+12 > len(data) {
		return
	}

	// replica_id, max_wait_ms, min_bytes
	replicaID := binary.BigEndian.Uint32(data[offset : offset+4])
	maxWait := binary.BigEndian.Uint32(data[offset+4 : offset+8])
	minBytes := binary.BigEndian.Uint32(data[offset+8 : offset+12])
	offset += 12

	msg.Attributes["replica_id"] = replicaID
	msg.Attributes["max_wait_ms"] = maxWait
	msg.Attributes["min_bytes"] = minBytes

	// num_topics
	if offset+4 <= len(data) {
		numTopics := int(binary.BigEndian.Uint32(data[offset : offset+4]))
		msg.Attributes["topic_count"] = numTopics
		offset += 4

		if numTopics > 0 && offset+2 <= len(data) {
			topicLen := int(binary.BigEndian.Uint16(data[offset : offset+2]))
			offset += 2
			if topicLen > 0 && offset+topicLen <= len(data) {
				msg.Resource = string(data[offset : offset+topicLen])
			}
		}
	}
}

// parseMetadataRequest 解析 Metadata 请求
func (p *KafkaParser) parseMetadataRequest(data []byte, offset int, version int16, msg *ProtocolMessage) {
	if offset+4 > len(data) {
		return
	}

	numTopics := int(binary.BigEndian.Uint32(data[offset : offset+4]))
	msg.Attributes["topic_count"] = numTopics
	offset += 4

	if numTopics > 0 && offset+2 <= len(data) {
		topicLen := int(binary.BigEndian.Uint16(data[offset : offset+2]))
		offset += 2
		if topicLen > 0 && offset+topicLen <= len(data) {
			msg.Resource = string(data[offset : offset+topicLen])
		}
	}
}

// parseGroupRequest 解析组请求
func (p *KafkaParser) parseGroupRequest(data []byte, offset int, version int16, msg *ProtocolMessage, opName string) {
	if offset+4 > len(data) {
		return
	}

	// group_id
	groupIDLen := int(binary.BigEndian.Uint16(data[offset : offset+2]))
	offset += 2
	if groupIDLen > 0 && offset+groupIDLen <= len(data) {
		msg.Resource = string(data[offset : offset+groupIDLen])
		offset += groupIDLen
	}

	// session_timeout_ms
	if offset+4 <= len(data) {
		msg.Attributes["session_timeout_ms"] = binary.BigEndian.Uint32(data[offset : offset+4])
	}
}

// parseResponse 解析 Kafka 响应
func (p *KafkaParser) parseResponse(data []byte, msg *ProtocolMessage) {
	if len(data) < 8 {
		msg.Operation = "UNKNOWN"
		return
	}

	// correlation_id (4 bytes)
	correlationID := binary.BigEndian.Uint32(data[0:4])
	msg.Attributes["correlation_id"] = correlationID

	// 尝试判断响应类型（基于内容特征）
	// Kafka 响应没有 API Key，需要根据 correlation_id 关联
	msg.Operation = "RESPONSE"
	msg.Success = true

	// 检查错误
	if len(data) > 4 {
		// 检查是否有错误码（很多响应的第5-6字节是 error_code）
		if len(data) >= 6 {
			errorCode := int16(binary.BigEndian.Uint16(data[4:6]))
			if errorCode != 0 {
				msg.Success = false
				msg.ErrorCode = fmt.Sprintf("KAFKA_%d", errorCode)
				msg.ErrorMsg = kafkaErrorCode(errorCode)
			}
		}
	}
}

// kafkaErrorCode Kafka 错误码描述
func kafkaErrorCode(code int16) string {
	codes := map[int16]string{
		-1:  "UNKNOWN_SERVER_ERROR",
		3:   "UNKNOWN_TOPIC_OR_PARTITION",
		4:   "LEADER_NOT_AVAILABLE",
		5:   "NOT_LEADER_OR_FOLLOWER",
		6:   "REQUEST_TIMED_OUT",
		7:   "BROKER_NOT_AVAILABLE",
		8:   "REPLICA_NOT_AVAILABLE",
		9:   "MESSAGE_TOO_LARGE",
		10:  "UNSUPPORTED_VERSION",
		16:  "OFFSET_OUT_OF_RANGE",
		17:  "TOO_MANY_REQUESTS",
		18:  "ILLEGAL_GENERATION",
		19:  "INCONSISTENT_GROUP_PROTOCOL",
		22:  "UNKNOWN_MEMBER_ID",
		25:  "REBALANCE_IN_PROGRESS",
		29:  "INVALID_COMMIT_OFFSET_SIZE",
		36:  "CONCURRENT_TRANSACTIONS",
		47:  "INVALID_PRODUCER_EPOCH",
	}
	if desc, ok := codes[code]; ok {
		return desc
	}
	return fmt.Sprintf("ERROR_CODE_%d", code)
}
