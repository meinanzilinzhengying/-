// Package protocol - PostgreSQL 协议解析器
// Copyright (c) 2026 Cloud Flow Team
// Licensed under the MIT License.

package protocol

import (
	"encoding/binary"
	"fmt"
	"strings"
	"time"
)

// PostgreSQL 常量
const (
	PostgreSQLDefaultPort = 5432

	// 前端消息类型
	PGStartup    = 0x00 // Startup message (无类型字节，长度开头)
	PGCancel     = 80   // 'P' CancelRequest
	PGSSLRequest = 80   // 'P' SSLRequest
	PGPassword   = 'p'  // Password
	PGQuery      = 'Q'  // Query
	PGParse      = 'P'  // Parse
	PGBind       = 'B'  // Bind
	PGDescribe   = 'D'  // Describe
	PGExecute    = 'E'  // Execute
	PGSync       = 'S'  // Sync
	PGClose      = 'C'  // Close
	PGPrepare    = 'P'  // Prepare (extended query)
	PGTerminate  = 'X'  // Terminate

	// 后端消息类型
	PGBackendKeyData   = 'K'
	PGAuthentication  = 'R'
	PGParameterStatus = 'S'
	PGBackendReady    = 'Z'
	PGDataRow         = 'D'
	PGCommandComplete = 'C'
	PGErrorResponse   = 'E'
	PGNoticeResponse  = 'N'
	PGNotification    = 'A'
	PGParseComplete   = '1'
	PGBindComplete    = '2'
	PGCloseComplete   = '3'
)

// PostgreSQLParser PostgreSQL 协议解析器
type PostgreSQLParser struct{}

// NewPostgreSQLParser 创建 PostgreSQL 解析器
func NewPostgreSQLParser() *PostgreSQLParser { return &PostgreSQLParser{} }

func (p *PostgreSQLParser) Name() string          { return "postgresql" }
func (p *PostgreSQLParser) Protocol() ProtocolType { return ProtocolPostgreSQL }
func (p *PostgreSQLParser) Ports() []uint16        { return []uint16{5432, 5433, 5434} }

// Detect 检测 PostgreSQL 协议
func (p *PostgreSQLParser) Detect(srcPort, dstPort uint16, payload []byte) bool {
	portMatch := false
	for _, port := range p.Ports() {
		if srcPort == port || dstPort == port {
			portMatch = true
			break
		}
	}
	if !portMatch {
		return false
	}
	if len(payload) < 8 {
		return false
	}

	// Startup message: 长度(4) + 协议版本(196608=0x00030000)
	length := binary.BigEndian.Uint32(payload[0:4])
	if length < 8 || length > uint32(len(payload))+100 {
		return false
	}

	version := binary.BigEndian.Uint32(payload[4:8])
	// 3.0 协议版本: 196608 (0x00030000) 或 SSL Request: 80877103
	if version == 196608 || version == 80877103 {
		return true
	}

	// 常规消息：类型字节(1) + 长度(4)
	msgType := payload[0]
	if msgType >= 32 && msgType <= 122 {
		msgLen := binary.BigEndian.Uint32(payload[1:5])
		if msgLen >= 4 && msgLen <= uint32(len(payload))+100 {
			return true
		}
	}

	return false
}

// Parse 解析 PostgreSQL 报文
func (p *PostgreSQLParser) Parse(payload []byte, direction MsgDirection) (*ProtocolMessage, error) {
	if len(payload) < 4 {
		return nil, fmt.Errorf("payload too short: %d bytes", len(payload))
	}

	msg := &ProtocolMessage{
		Protocol:  ProtocolPostgreSQL,
		Direction: direction,
		Timestamp: time.Now(),
		RawLength: len(payload),
		Attributes: make(map[string]interface{}),
	}

	// 判断是否为 Startup message
	if len(payload) >= 8 {
		version := binary.BigEndian.Uint32(payload[4:8])
		if version == 196608 {
			p.parseStartup(payload, msg)
			return msg, nil
		}
		if version == 80877103 {
			msg.Operation = "SSL_REQUEST"
			msg.Success = true
			return msg, nil
		}
	}

	// 常规消息
	if len(payload) < 5 {
		return nil, fmt.Errorf("payload too short for PG message: %d bytes", len(payload))
	}

	msgType := payload[0]
	msgLen := binary.BigEndian.Uint32(payload[1:5])
	msg.Attributes["msg_type"] = string(msgType)
	msg.Attributes["msg_length"] = msgLen

	if direction == DirectionRequest {
		p.parseFrontend(payload, msgType, payload[5:], msg)
	} else {
		p.parseBackend(payload, msgType, payload[5:], msg)
	}

	msg.RawPreview = SafePreview(payload, 100)
	return msg, nil
}

func (p *PostgreSQLParser) parseStartup(data []byte, msg *ProtocolMessage) {
	msg.Operation = "STARTUP"
	if len(data) < 12 {
		return
	}

	// 解析参数（user, database 等）
	params := make(map[string]string)
	offset := 8
	for offset < len(data)-1 {
		// 读取 key
		keyEnd := offset
		for keyEnd < len(data) && data[keyEnd] != 0 {
			keyEnd++
		}
		key := string(data[offset:keyEnd])
		offset = keyEnd + 1

		if key == "" {
			break
		}

		// 读取 value
		valEnd := offset
		for valEnd < len(data) && data[valEnd] != 0 {
			valEnd++
		}
		value := string(data[offset:valEnd])
		offset = valEnd + 1

		params[key] = value
	}

	msg.Attributes["parameters"] = params
	if user, ok := params["user"]; ok {
		msg.Attributes["user"] = user
	}
	if db, ok := params["database"]; ok {
		msg.Resource = db
	}
	msg.Success = true
}

func (p *PostgreSQLParser) parseFrontend(data []byte, msgType byte, body []byte, msg *ProtocolMessage) {
	switch msgType {
	case PGQuery:
		msg.Operation = "QUERY"
		msg.Attributes["query"] = SafePreview(body, 500)
		msg.Operation = detectSQLOperation(SafePreview(body, 500))
		msg.Resource = extractTableFromSQL(SafePreview(body, 500))
		msg.Success = true

	case PGParse:
		msg.Operation = "PARSE"
		if len(body) > 4 {
			// Parse(string stmt, string query, int16 nParamTypes, int32[] paramTypes)
			stmt := readPGString(body, 0)
			query := readPGString(body, len(stmt)+1)
			msg.Attributes["stmt"] = stmt
			msg.Attributes["query"] = SafePreview([]byte(query), 500)
			msg.Operation = detectSQLOperation(query)
			msg.Resource = extractTableFromSQL(query)
		}
		msg.Success = true

	case PGBind:
		msg.Operation = "BIND"
		msg.Success = true

	case PGDescribe:
		msg.Operation = "DESCRIBE"
		if len(body) > 1 {
			msg.Attributes["describe_type"] = string(body[0])
			msg.Attributes["describe_name"] = SafePreview(body[1:], 100)
		}
		msg.Success = true

	case PGExecute:
		msg.Operation = "EXECUTE"
		if len(body) > 4 {
			portal := readPGString(body, 0)
			msg.Attributes["portal"] = portal
		}
		msg.Success = true

	case PGSync:
		msg.Operation = "SYNC"
		msg.Success = true

	case PGClose:
		msg.Operation = "CLOSE"
		msg.Success = true

	case PGPassword:
		msg.Operation = "PASSWORD"
		msg.Success = true

	case PGTerminate:
		msg.Operation = "TERMINATE"
		msg.Success = true

	default:
		msg.Operation = fmt.Sprintf("PG_FRONTEND_%c", msgType)
		msg.Success = true
	}
}

func (p *PostgreSQLParser) parseBackend(data []byte, msgType byte, body []byte, msg *ProtocolMessage) {
	switch msgType {
	case PGCommandComplete:
		msg.Operation = "COMMAND_COMPLETE"
		tag := SafePreview(body, 200)
		msg.Attributes["tag"] = tag
		// 提取影响行数 (如 "INSERT 0 1" 或 "SELECT 5")
		msg.Success = true

	case PGDataRow:
		msg.Operation = "DATA_ROW"
		if len(body) >= 2 {
			colCount := binary.BigEndian.Uint16(body[0:2])
			msg.Attributes["column_count"] = colCount
		}
		msg.Success = true

	case PGErrorResponse:
		msg.Operation = "ERROR"
		msg.Success = false
		// 解析错误字段
		fields := parsePGErrorFields(body)
		if severity, ok := fields['S']; ok {
			msg.Attributes["severity"] = severity
		}
		if code, ok := fields['C']; ok {
			msg.ErrorCode = code
		}
		if message, ok := fields['M']; ok {
			msg.ErrorMsg = message
		}
		if detail, ok := fields['D']; ok {
			msg.Attributes["detail"] = detail
		}

	case PGNoticeResponse:
		msg.Operation = "NOTICE"
		fields := parsePGErrorFields(body)
		if message, ok := fields['M']; ok {
			msg.Attributes["message"] = message
		}
		msg.Success = true

	case PGBackendReady:
		msg.Operation = "READY"
		if len(body) > 0 {
			status := string(body[0])
			msg.Attributes["status"] = status // 'I'=idle, 'T'=in transaction, 'E'=error
		}
		msg.Success = true

	case PGAuthentication:
		msg.Operation = "AUTHENTICATION"
		if len(body) >= 4 {
			authType := binary.BigEndian.Uint32(body[0:4])
			msg.Attributes["auth_type"] = authType
			authTypes := map[uint32]string{
				0: "ok", 3: "cleartext_password", 5: "md5_password",
				10: "SASL", 11: "SASL_CONTINUE", 12: "SASL_FINAL",
			}
			if name, ok := authTypes[authType]; ok {
				msg.Attributes["auth_name"] = name
			}
		}
		msg.Success = true

	case PGParameterStatus:
		msg.Operation = "PARAMETER_STATUS"
		if len(body) > 0 {
			kv := strings.SplitN(SafePreview(body, 200), "\x00", 2)
			if len(kv) >= 2 {
				msg.Attributes["param"] = kv[0]
				msg.Attributes["value"] = kv[1]
			}
		}
		msg.Success = true

	case PGBackendKeyData:
		msg.Operation = "BACKEND_KEY"
		if len(body) >= 8 {
			pid := binary.BigEndian.Uint32(body[0:4])
			key := binary.BigEndian.Uint32(body[4:8])
			msg.Attributes["backend_pid"] = pid
			msg.Attributes["secret_key"] = key
		}
		msg.Success = true

	case PGNotification:
		msg.Operation = "NOTIFICATION"
		if len(body) > 0 {
			msg.Attributes["notification"] = SafePreview(body, 200)
		}
		msg.Success = true

	case PGParseComplete, PGBindComplete, PGCloseComplete:
		msg.Operation = fmt.Sprintf("PARSE_COMPLETE")
		msg.Success = true

	default:
		msg.Operation = fmt.Sprintf("PG_BACKEND_%c", msgType)
		msg.Success = true
	}
}

// parsePGErrorFields 解析 PG 错误字段
func parsePGErrorFields(data []byte) map[byte]string {
	fields := make(map[byte]string)
	i := 0
	for i < len(data) {
		if data[i] == 0 {
			break
		}
		fieldType := data[i]
		i++
		end := i
		for end < len(data) && data[end] != 0 {
			end++
		}
		fields[fieldType] = string(data[i:end])
		i = end + 1
	}
	return fields
}

// readPGString 读取 PG 风格的 null-terminated 字符串
func readPGString(data []byte, offset int) string {
	end := offset
	for end < len(data) && data[end] != 0 {
		end++
	}
	return string(data[offset:end])
}
