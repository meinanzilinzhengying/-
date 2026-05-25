// Package protocol - Redis RESP 协议解析器
// Copyright (c) 2026 Cloud Flow Team
// Licensed under the MIT License.

package protocol

import (
	"bytes"
	"fmt"
	"strconv"
	"strings"
	"time"
)

// Redis 常量
const (
	RedisDefaultPort = 6379

	// RESP 类型前缀
	RESPStatus    = '+' // 简单字符串
	RESPError     = '-' // 错误
	RESPInteger   = ':' // 整数
	RESPBulk      = '$' // 批量字符串
	RESPArray     = '*' // 数组
)

// RedisParser Redis RESP 协议解析器
type RedisParser struct{}

// NewRedisParser 创建 Redis 解析器
func NewRedisParser() *RedisParser { return &RedisParser{} }

func (p *RedisParser) Name() string          { return "redis" }
func (p *RedisParser) Protocol() ProtocolType { return ProtocolRedis }
func (p *RedisParser) Ports() []uint16        { return []uint16{6379, 6380, 6381} }

// Detect 检测 Redis RESP 协议
func (p *RedisParser) Detect(srcPort, dstPort uint16, payload []byte) bool {
	portMatch := false
	for _, port := range p.Ports() {
		if srcPort == port || dstPort == port {
			portMatch = true
			break
		}
	}
	if !portMatch || len(payload) < 3 {
		return false
	}

	// RESP 协议特征：首字节为 + - : $ * 之一
	first := payload[0]
	switch first {
	case RESPStatus, RESPError, RESPInteger, RESPBulk, RESPArray:
		return true
	}

	// Redis 命令特征：PING\r\n, SET\r\n, GET\r\n 等
	upper := strings.ToUpper(string(payload[:min(len(payload), 20)]))
	for _, cmd := range []string{"PING", "SET ", "GET ", "DEL ", "HSET", "HGET", "LPUSH", "RPUSH", "SADD", "ZADD", "INCR", "DECR", "EXPIRE", "TTL", "KEYS", "MGET", "MSET", "SELECT", "AUTH", "INFO", "CONFIG", "FLUSH", "SUBSCRIBE", "PUBLISH"} {
		if strings.HasPrefix(upper, cmd) {
			return true
		}
	}

	return false
}

// Parse 解析 Redis RESP 报文
func (p *RedisParser) Parse(payload []byte, direction MsgDirection) (*ProtocolMessage, error) {
	if len(payload) < 2 {
		return nil, fmt.Errorf("payload too short: %d bytes", len(payload))
	}

	msg := &ProtocolMessage{
		Protocol:  ProtocolRedis,
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

// parseRequest 解析 Redis 请求
func (p *RedisParser) parseRequest(data []byte, msg *ProtocolMessage) {
	// RESP 数组格式：*N\r\n$len\r\ncmd\r\n$len\r\nkey\r\n...
	if data[0] == RESPArray {
		p.parseRESPArray(data, msg)
		return
	}

	// 内联命令格式：CMD arg1 arg2\r\n
	line := strings.TrimRight(string(data), "\r\n")
	parts := strings.Fields(line)
	if len(parts) == 0 {
		msg.Operation = "UNKNOWN"
		return
	}

	cmd := strings.ToUpper(parts[0])
	msg.Operation = cmd
	msg.Success = true

	// 提取 Key
	if len(parts) >= 2 {
		msg.Resource = parts[1]
	}

	// 解析不同命令
	switch cmd {
	case "GET", "MGET":
		msg.Attributes["keys"] = parts[1:]
	case "SET", "MSET":
		if len(parts) >= 3 {
			msg.Attributes["key"] = parts[1]
			msg.Attributes["value_preview"] = SafePreview([]byte(parts[2]), 100)
		}
	case "DEL", "UNLINK":
		msg.Attributes["keys"] = parts[1:]
	case "HGET", "HSET", "HDEL":
		if len(parts) >= 2 {
			msg.Attributes["hash_key"] = parts[1]
		}
		if len(parts) >= 3 {
			msg.Attributes["field"] = parts[2]
		}
	case "LPUSH", "RPUSH", "LPOP", "RPOP":
		msg.Attributes["list_key"] = parts[1]
	case "SADD", "SREM", "SMEMBERS", "SCARD":
		msg.Attributes["set_key"] = parts[1]
	case "ZADD", "ZREM", "ZRANGE", "ZSCORE":
		msg.Attributes["zset_key"] = parts[1]
	case "EXPIRE", "TTL", "PTTL":
		msg.Attributes["key"] = parts[1]
	case "INCR", "DECR", "INCRBY", "DECRBY":
		msg.Attributes["key"] = parts[1]
	case "SUBSCRIBE", "UNSUBSCRIBE":
		msg.Attributes["channel"] = parts[1]
	case "PUBLISH":
		msg.Attributes["channel"] = parts[1]
		if len(parts) >= 3 {
			msg.Attributes["message_preview"] = SafePreview([]byte(parts[2]), 100)
		}
	case "SELECT":
		msg.Attributes["db"] = parts[1]
	case "AUTH":
		msg.Attributes["user"] = parts[1]
	case "INFO", "CONFIG", "KEYS", "SCAN", "CLUSTER":
		msg.Attributes["args"] = parts[1:]
	case "PING":
		msg.Attributes["message"] = ""
		if len(parts) > 1 {
			msg.Attributes["message"] = parts[1]
		}
	}
}

// parseRESPArray 解析 RESP 数组（标准请求格式）
func (p *RedisParser) parseRESPArray(data []byte, msg *ProtocolMessage) {
	// *N\r\n
	count, offset := parseRESPLine(data, 1)
	if count <= 0 || offset < 0 {
		msg.Operation = "UNKNOWN"
		return
	}

	// 读取命令
	args := make([]string, 0, count)
	for i := 0; i < count && offset < len(data); i++ {
		if offset >= len(data) || data[offset] != RESPBulk {
			break
		}
		str, nextOffset := parseRESPBulkString(data, offset)
		args = append(args, str)
		offset = nextOffset
	}

	if len(args) == 0 {
		msg.Operation = "UNKNOWN"
		return
	}

	cmd := strings.ToUpper(args[0])
	msg.Operation = cmd
	msg.Success = true

	if len(args) >= 2 {
		msg.Resource = args[1]
		msg.Attributes["args"] = args[1:]
	}
}

// parseResponse 解析 Redis 响应
func (p *RedisParser) parseResponse(data []byte, msg *ProtocolMessage) {
	if len(data) == 0 {
		msg.Operation = "EMPTY"
		return
	}

	switch data[0] {
	case RESPStatus:
		// +OK\r\n, +PONG\r\n
		line := strings.TrimRight(string(data[1:]), "\r\n")
		msg.Operation = "STATUS"
		msg.Attributes["status"] = line
		msg.Success = true

	case RESPError:
		// -ERR message\r\n
		line := strings.TrimRight(string(data[1:]), "\r\n")
		msg.Operation = "ERROR"
		msg.Success = false
		// 提取错误码
		parts := strings.SplitN(line, " ", 2)
		if len(parts) >= 1 {
			msg.ErrorCode = parts[0]
		}
		if len(parts) >= 2 {
			msg.ErrorMsg = parts[1]
		}

	case RESPInteger:
		// :123\r\n
		line := strings.TrimRight(string(data[1:]), "\r\n")
		msg.Operation = "INTEGER"
		msg.Attributes["value"] = line
		msg.Success = true

	case RESPBulk:
		// $6\r\nfoobar\r\n
		str, _ := parseRESPBulkString(data, 0)
		msg.Operation = "BULK_STRING"
		msg.Attributes["value_preview"] = SafePreview([]byte(str), 200)
		msg.Success = true

	case RESPArray:
		// *3\r\n...
		count, _ := parseRESPLine(data, 1)
		msg.Operation = "ARRAY"
		msg.Attributes["element_count"] = count
		msg.Success = true

	default:
		// 内联响应
		line := strings.TrimRight(string(data), "\r\n")
		msg.Operation = "INLINE"
		msg.Attributes["value"] = line
		msg.Success = true
	}
}

// parseRESPLine 解析 RESP 行（整数）
func parseRESPLine(data []byte, start int) (int, int) {
	end := bytes.IndexByte(data[start:], '\n')
	if end < 0 {
		return -1, -1
	}
	line := string(data[start : start+end])
	line = strings.TrimRight(line, "\r")
	val, err := strconv.Atoi(line)
	if err != nil {
		return -1, start + end + 1
	}
	return val, start + end + 1
}

// parseRESPBulkString 解析 RESP 批量字符串
func parseRESPBulkString(data []byte, offset int) (string, int) {
	if offset >= len(data) || data[offset] != RESPBulk {
		return "", offset
	}

	length, nextOffset := parseRESPLine(data, offset+1)
	if length < 0 {
		return "(null)", nextOffset
	}
	if length == 0 {
		return "", nextOffset
	}

	if nextOffset+length > len(data) {
		return string(data[nextOffset:]), len(data)
	}

	str := string(data[nextOffset : nextOffset+length])
	return str, nextOffset + length + 2 // +2 for \r\n
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
