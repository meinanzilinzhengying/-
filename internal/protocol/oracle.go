// Package protocol - Oracle TNS 协议解析器
// Copyright (c) 2026 Cloud Flow Team
// Licensed under the MIT License.

package protocol

import (
	"encoding/binary"
	"fmt"
	"strings"
	"time"
)

// ============================================================
// Oracle TNS (Transparent Network Substrate) 协议解析
// ============================================================

const (
	OracleDefaultPort = 1521

	TNSTypeConnect     uint8 = 1
	TNSTypeAccept      uint8 = 2
	TNSTypeAcknowledge uint8 = 3
	TNSTypeRefuse      uint8 = 4
	TNSTypeRedirect    uint8 = 5
	TNSTypeData        uint8 = 6
	TNSTypeNull        uint8 = 7
	TNSTypeAbort       uint8 = 9
	TNSTypeResend      uint8 = 11
	TNSTypeMarker      uint8 = 12
	TNSTypeAttention   uint8 = 13
	TNSTypeControl     uint8 = 14
	TNSTypeError       uint8 = 15
)

// OracleParser Oracle TNS 协议解析器
type OracleParser struct{}

// NewOracleParser 创建 Oracle 解析器
func NewOracleParser() *OracleParser { return &OracleParser{} }

func (p *OracleParser) Name() string          { return "oracle" }
func (p *OracleParser) Protocol() ProtocolType { return ProtocolOracle }
func (p *OracleParser) Ports() []uint16        { return []uint16{1521, 1522, 1526, 1575} }

// Detect 检测 Oracle TNS 协议
func (p *OracleParser) Detect(srcPort, dstPort uint16, payload []byte) bool {
	// 端口检查
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

	pktLen := binary.BigEndian.Uint16(payload[0:2])
	if pktLen < 8 || pktLen > uint16(len(payload))+100 {
		return false
	}

	pktType := payload[2]
	if pktType < 1 || pktType > 19 {
		return false
	}

	return true
}

// Parse 解析 Oracle TNS 报文
func (p *OracleParser) Parse(payload []byte, direction MsgDirection) (*ProtocolMessage, error) {
	if len(payload) < 8 {
		return nil, fmt.Errorf("payload too short for TNS header: %d bytes", len(payload))
	}

	msg := &ProtocolMessage{
		Protocol:  ProtocolOracle,
		Direction: direction,
		Timestamp: time.Now(),
		RawLength: len(payload),
		Attributes: map[string]interface{}{
			"tns_length": binary.BigEndian.Uint16(payload[0:2]),
			"tns_type":   payload[2],
			"tns_flags":  payload[3] & 0x0F,
		},
	}

	switch payload[2] {
	case TNSTypeConnect:
		p.parseConnect(payload, msg)
	case TNSTypeData:
		p.parseData(payload, msg)
	case TNSTypeError:
		p.parseError(payload, msg)
	case TNSTypeRefuse:
		p.parseRefuse(payload, msg)
	case TNSTypeRedirect:
		p.parseRedirect(payload, msg)
	case TNSTypeAccept:
		p.parseAccept(payload, msg)
	default:
		msg.Operation = fmt.Sprintf("TNS_TYPE_%d", payload[2])
	}

	msg.RawPreview = SafePreview(payload, 100)
	return msg, nil
}

func (p *OracleParser) parseConnect(data []byte, msg *ProtocolMessage) {
	msg.Operation = "CONNECT"
	if len(data) < 34 {
		return
	}

	offset := 8
	if offset+2 <= len(data) {
		msg.Attributes["version"] = binary.BigEndian.Uint16(data[offset : offset+2])
		offset += 2
	}

	if offset+10 <= len(data) {
		serviceOffset := binary.BigEndian.Uint16(data[offset : offset+2])
		serviceLen := binary.BigEndian.Uint16(data[offset+2 : offset+4])
		if int(serviceOffset)+int(serviceLen) <= len(data) && serviceLen > 0 {
			msg.Resource = SafePreview(data[serviceOffset:serviceOffset+serviceLen], 200)
		}
	}
	msg.Success = true
}

func (p *OracleParser) parseData(data []byte, msg *ProtocolMessage) {
	msg.Operation = "DATA"
	if len(data) < 12 {
		return
	}

	dataFlags := data[8]
	msg.Attributes["data_flags"] = dataFlags

	offset := 10
	if dataFlags == 0 || dataFlags == 2 && offset < len(data) {
		sqlPreview := SafePreview(data[offset:], 500)
		msg.Operation = detectSQLOperation(sqlPreview)
		msg.Attributes["sql"] = sqlPreview
		msg.Resource = extractTableFromSQL(sqlPreview)
	}
	msg.Success = true
}

func (p *OracleParser) parseError(data []byte, msg *ProtocolMessage) {
	msg.Operation = "ERROR"
	msg.Success = false
	if len(data) >= 10 {
		errCode := binary.BigEndian.Uint16(data[8:10])
		msg.ErrorCode = fmt.Sprintf("TNS-%04d", errCode)
	}
	if len(data) > 10 {
		msg.ErrorMsg = SafePreview(data[10:], 200)
	}
}

func (p *OracleParser) parseRefuse(data []byte, msg *ProtocolMessage) {
	msg.Operation = "REFUSE"
	msg.Success = false
	if len(data) > 8 {
		reason := data[8]
		msg.ErrorCode = fmt.Sprintf("REFUSE_%d", reason)
		reasons := map[byte]string{0: "Version", 1: "Net error", 2: "Permission", 3: "Bad address", 4: "Service not found"}
		if desc, ok := reasons[reason]; ok {
			msg.ErrorMsg = desc
		}
	}
}

func (p *OracleParser) parseRedirect(data []byte, msg *ProtocolMessage) {
	msg.Operation = "REDIRECT"
	if len(data) > 8 {
		addrLen := int(data[8])
		if len(data) > 9+addrLen {
			msg.Attributes["redirect_address"] = SafePreview(data[9:9+addrLen], 200)
		}
	}
}

func (p *OracleParser) parseAccept(data []byte, msg *ProtocolMessage) {
	msg.Operation = "ACCEPT"
	msg.Success = true
	if len(data) >= 10 {
		msg.Attributes["accepted_version"] = binary.BigEndian.Uint16(data[8:10])
	}
}

// ============================================================
// SQL 辅助
// ============================================================

func detectSQLOperation(sql string) string {
	upper := strings.ToUpper(sql)
	for _, prefix := range []string{"SELECT", "INSERT", "UPDATE", "DELETE", "COMMIT", "ROLLBACK", "EXECUTE", "CALL", "ALTER", "CREATE", "DROP"} {
		if strings.HasPrefix(upper, prefix) {
			return prefix
		}
	}
	return "UNKNOWN_SQL"
}

func extractTableFromSQL(sql string) string {
	upper := strings.ToUpper(sql)
	idx := strings.Index(upper, " FROM ")
	if idx < 0 {
		return ""
	}
	table := strings.TrimSpace(sql[idx+6:])
	for _, sep := range []string{" WHERE", " GROUP", " ORDER", " HAVING", " LIMIT", ";"} {
		if si := strings.Index(strings.ToUpper(table), sep); si > 0 {
			table = table[:si]
		}
	}
	return strings.TrimSpace(table)
}
