// Package protocol 增强协议解析器
// HTTP/DNS/MySQL 深度解析：路径/Cookie/事务ID/错误信息
package protocol

import (
	"encoding/binary"
	"encoding/hex"
	"fmt"
	"net"
	"net/http"
	"net/textproto"
	"regexp"
	"strconv"
	"strings"
	"time"
)

// ============================================================
// HTTP协议深度解析
// ============================================================

// HTTPParser HTTP协议解析器
type HTTPParser struct {
	// 解析配置
	parseBody       bool
	parseCookies    bool
	parseHeaders    bool
	maxBodySize     int
	
	// 统计
	stats *HTTPStats
}

// HTTPStats HTTP统计
type HTTPStats struct {
	TotalRequests   int64
	TotalResponses int64
	MethodStats    map[string]int64
	StatusCodes    map[int]int64
	Errors         int64
}

// HTTPRequest HTTP请求深度解析
type HTTPRequest struct {
	// 基础信息
	Method     string `json:"method"`
	Path       string `json:"path"`
	Query      string `json:"query,omitempty"`
	Version    string `json:"version"`
	Host       string `json:"host"`
	
	// 深度解析字段
	PathSegments []string `json:"path_segments,omitempty"` // 路径分段
	QueryParams map[string]string `json:"query_params,omitempty"` // 查询参数
	Cookies     map[string]string `json:"cookies,omitempty"`     // Cookies
	UserAgent  string `json:"user_agent,omitempty"`
	Referer    string `json:"referer,omitempty"`
	ContentType string `json:"content_type,omitempty"`
	ContentLen  int64  `json:"content_length,omitempty"`
	Accept      string `json:"accept,omitempty"`
	Auth        string `json:"auth,omitempty"` // Authorization header
	
	// 自定义头
	Headers    map[string]string `json:"headers,omitempty"`
	
	// 请求体关键词（用于分析）
	BodyKeywords []string `json:"body_keywords,omitempty"`
	
	// 关联信息
	TransactionID string `json:"transaction_id,omitempty"` // X-Request-ID
	TraceID       string `json:"trace_id,omitempty"`
	SpanID        string `json:"span_id,omitempty"`
}

// HTTPResponse HTTP响应深度解析
type HTTPResponse struct {
	// 基础信息
	StatusCode int    `json:"status_code"`
	StatusText string `json:"status_text"`
	Version    string `json:"version"`
	
	// 深度解析字段
	ContentType   string `json:"content_type,omitempty"`
	ContentLen    int64  `json:"content_length,omitempty"`
	Server        string `json:"server,omitempty"`
	CacheControl  string `json:"cache_control,omitempty"`
	Expires       string `json:"expires,omitempty"`
	ETag          string `json:"etag,omitempty"`
	Location      string `json:"location,omitempty"` // 重定向
	SetCookies    map[string]*Cookie `json:"set_cookies,omitempty"`
	
	// 自定义头
	Headers       map[string]string `json:"headers,omitempty"`
	
	// 错误信息
	ErrorMsg      string `json:"error_msg,omitempty"`
	ErrorCode     string `json:"error_code,omitempty"` // 业务错误码
	
	// 响应时间
	LatencyMs     float64 `json:"latency_ms,omitempty"`
}

// Cookie Cookie结构
type Cookie struct {
	Name     string `json:"name"`
	Value    string `json:"value"`
	Path     string `json:"path,omitempty"`
	Domain   string `json:"domain,omitempty"`
	Expires  string `json:"expires,omitempty"`
	HttpOnly bool   `json:"http_only,omitempty"`
	Secure   bool   `json:"secure,omitempty"`
}

// HTTPTransaction HTTP完整事务
type HTTPTransaction struct {
	Request  *HTTPRequest  `json:"request"`
	Response *HTTPResponse `json:"response,omitempty"`
	Duration time.Duration `json:"duration"`
	Error    string        `json:"error,omitempty"`
}

// NewHTTPParser 创建HTTP解析器
func NewHTTPParser() *HTTPParser {
	return &HTTPParser{
		parseBody:    false, // 默认不解析body
		parseCookies: true,
		parseHeaders: true,
		maxBodySize:  1024,
		stats: &HTTPStats{
			MethodStats: make(map[string]int64),
			StatusCodes: make(map[int]int64),
		},
	}
}

// ParseRequest 解析HTTP请求
func (p *HTTPParser) ParseRequest(data []byte) (*HTTPRequest, error) {
	p.stats.TotalRequests++
	
	req := &HTTPRequest{
		Headers: make(map[string]string),
	}
	
	// 解析请求行
	lines := splitLines(data)
	if len(lines) < 1 {
		return nil, fmt.Errorf("无效的HTTP请求")
	}
	
	// 请求行: GET /path?query HTTP/1.1
	requestLine := lines[0]
	parts := strings.SplitN(requestLine, " ", 3)
	if len(parts) < 3 {
		return nil, fmt.Errorf("无效的请求行")
	}
	
	req.Method = parts[0]
	req.Path = parts[1]
	req.Version = parts[2]
	
	// 解析URL
	if idx := strings.Index(req.Path, "?"); idx >= 0 {
		req.Query = req.Path[idx+1:]
		req.Path = req.Path[:idx]
	}
	
	// 路径分段
	req.PathSegments = splitPath(req.Path)
	
	// 解析查询参数
	req.QueryParams = parseQueryParams(req.Query)
	
	// 解析请求头
	for _, line := range lines[1:] {
		if line == "" {
			break
		}
		
		idx := strings.Index(line, ":")
		if idx < 0 {
			continue
		}
		
		key := strings.TrimSpace(line[:idx])
		value := strings.TrimSpace(line[idx+1:])
		keyLower := strings.ToLower(key)
		
		req.Headers[key] = value
		
		switch keyLower {
		case "host":
			req.Host = value
		case "user-agent":
			req.UserAgent = value
		case "referer":
			req.Referer = value
		case "content-type":
			req.ContentType = value
		case "content-length":
			req.ContentLen, _ = strconv.ParseInt(value, 10, 64)
		case "accept":
			req.Accept = value
		case "authorization":
			req.Auth = maskAuth(value)
		case "cookie":
			if p.parseCookies {
				req.Cookies = parseCookies(value)
			}
		case "x-request-id", "x-correlation-id":
			req.TransactionID = value
		case "x-trace-id", "x-b3-traceid":
			req.TraceID = value
		case "x-span-id", "x-b3-spanid":
			req.SpanID = value
		}
	}
	
	p.stats.MethodStats[req.Method]++
	return req, nil
}

// ParseResponse 解析HTTP响应
func (p *HTTPParser) ParseResponse(data []byte) (*HTTPResponse, error) {
	p.stats.TotalResponses++
	
	resp := &HTTPResponse{
		Headers: make(map[string]string),
	}
	
	lines := splitLines(data)
	if len(lines) < 1 {
		return nil, fmt.Errorf("无效的HTTP响应")
	}
	
	// 状态行: HTTP/1.1 200 OK
	statusLine := lines[0]
	parts := strings.SplitN(statusLine, " ", 3)
	if len(parts) < 2 {
		return nil, fmt.Errorf("无效的状态行")
	}
	
	resp.Version = parts[0]
	resp.StatusCode, _ = strconv.Atoi(parts[1])
	if len(parts) >= 3 {
		resp.StatusText = parts[2]
	}
	
	// 解析响应头
	for _, line := range lines[1:] {
		if line == "" {
			break
		}
		
		idx := strings.Index(line, ":")
		if idx < 0 {
			continue
		}
		
		key := strings.TrimSpace(line[:idx])
		value := strings.TrimSpace(line[idx+1:])
		keyLower := strings.ToLower(key)
		
		resp.Headers[key] = value
		
		switch keyLower {
		case "content-type":
			resp.ContentType = value
		case "content-length":
			resp.ContentLen, _ = strconv.ParseInt(value, 10, 64)
		case "server":
			resp.Server = value
		case "cache-control":
			resp.CacheControl = value
		case "expires":
			resp.Expires = value
		case "etag":
			resp.ETag = value
		case "location":
			resp.Location = value
		case "set-cookie":
			if p.parseCookies {
				cookie := parseSetCookie(value)
				if resp.SetCookies == nil {
					resp.SetCookies = make(map[string]*Cookie)
				}
				resp.SetCookies[cookie.Name] = cookie
			}
		}
	}
	
	// 错误分类
	resp.ErrorCode, resp.ErrorMsg = classifyHTTPError(resp.StatusCode)
	
	p.stats.StatusCodes[resp.StatusCode]++
	return resp, nil
}

// GetStats 获取统计
func (p *HTTPParser) GetStats() *HTTPStats {
	return p.stats
}

// ============================================================
// DNS协议深度解析
// ============================================================

// DNSParser DNS协议解析器
type DNSParser struct {
	stats *DNSStats
}

// DNSStats DNS统计
type DNSStats struct {
	TotalQueries    int64
	TotalResponses int64
	QueryTypes     map[string]int64
	NXDomain       int64
	ServFail       int64
	Errors         int64
}

// DNSQuery DNS查询
type DNSQuery struct {
	// 基础信息
	TransactionID uint16 `json:"transaction_id"`
	Flags         uint16 `json:"flags"`
	
	// 查询详情
	QueryType  string `json:"query_type"`
	QueryClass string `json:"query_class"`
	Name       string `json:"name"`
	NameLabels []string `json:"name_labels,omitempty"`
	
	// 深度解析
	IsEDNS         bool              `json:"is_edns,omitempty"`
	EDNSVersion    uint8             `json:"edns_version,omitempty"`
	EDNSBufSize    uint16            `json:"edns_bufsize,omitempty"`
	EDNSOptions    map[string]string `json:"edns_options,omitempty"`
	
	// 客户端信息（从源IP推断）
	ClientSubnet   string `json:"client_subnet,omitempty"`
}

// DNSResponse DNS响应
type DNSResponse struct {
	// 基础信息
	TransactionID uint16 `json:"transaction_id"`
	Flags         uint16 `json:"flags"`
	
	// 响应详情
	ResponseCode string `json:"response_code"`
	IsAuthoritative bool `json:"is_authoritative,omitempty"`
	IsTruncated   bool   `json:"is_truncated,omitempty"`
	RecursionDesired bool `json:"recursion_desired,omitempty"`
	RecursionAvailable bool `json:"recursion_available,omitempty"`
	
	// 答案
	Answers     []DNSResourceRecord `json:"answers,omitempty"`
	Authorities []DNSResourceRecord `json:"authorities,omitempty"`
	Additionals []DNSResourceRecord `json:"additionals,omitempty"`
	
	// 错误信息
	ErrorMsg string `json:"error_msg,omitempty"`
	
	// 响应时间
	LatencyMs float64 `json:"latency_ms,omitempty"`
}

// DNSResourceRecord DNS资源记录
type DNSResourceRecord struct {
	Name     string `json:"name"`
	Type     string `json:"type"`
	Class    string `json:"class"`
	TTL      uint32 `json:"ttl"`
	RData    string `json:"rdata"`
	TypeSpecific map[string]string `json:"type_specific,omitempty"`
}

// DNSTransaction DNS完整事务
type DNSTransaction struct {
	ID              uint16     `json:"id"`
	Query           *DNSQuery  `json:"query"`
	Response        *DNSResponse `json:"response,omitempty"`
	Duration        time.Duration `json:"duration"`
	Error           string     `json:"error,omitempty"`
}

// NewDNSParser 创建DNS解析器
func NewDNSParser() *DNSParser {
	return &DNSParser{
		stats: &DNSStats{
			QueryTypes: make(map[string]int64),
		},
	}
}

// ParseQuery 解析DNS查询
func (p *DNSParser) ParseQuery(data []byte) (*DNSQuery, error) {
	if len(data) < 12 {
		return nil, fmt.Errorf("DNS数据包太短")
	}
	
	p.stats.TotalQueries++
	
	query := &DNSQuery{}
	
	// 事务ID
	query.TransactionID = binary.BigEndian.Uint16(data[0:2])
	query.Flags = binary.BigEndian.Uint16(data[2:4])
	
	// QDCOUNT
	qdcount := binary.BigEndian.Uint16(data[4:6])
	
	// 解析问题部分
	if qdcount > 0 {
		off := 12
		name, newOff, err := parseDNSName(data, off)
		if err != nil {
			return nil, err
		}
		query.Name = name
		query.NameLabels = splitDNSName(name)
		
		if len(data) >= newOff+4 {
			query.QueryType = dnsTypeToString(binary.BigEndian.Uint16(data[newOff : newOff+2]))
			query.QueryClass = dnsClassToString(binary.BigEndian.Uint16(data[newOff+2 : newOff+4]))
		}
	}
	
	// 检查EDNS
	if len(data) > 12 {
		// 检查是否有OPT记录
	}
	
	p.stats.QueryTypes[query.QueryType]++
	return query, nil
}

// ParseResponse 解析DNS响应
func (p *DNSParser) ParseResponse(data []byte) (*DNSResponse, error) {
	if len(data) < 12 {
		return nil, fmt.Errorf("DNS数据包太短")
	}
	
	p.stats.TotalResponses++
	
	resp := &DNSResponse{}
	
	// 事务ID
	resp.TransactionID = binary.BigEndian.Uint16(data[0:2])
	resp.Flags = binary.BigEndian.Uint16(data[2:4])
	
	// RCODE
	rcode := resp.Flags & 0xF
	resp.ResponseCode = dnsRcodeToString(rcode)
	
	// 标志位
	resp.IsAuthoritative = (resp.Flags & 0x400) != 0
	resp.IsTruncated = (resp.Flags & 0x200) != 0
	resp.RecursionDesired = (resp.Flags & 0x100) != 0
	resp.RecursionAvailable = (resp.Flags & 0x80) != 0
	
	// 错误处理
	switch rcode {
	case 3:
		p.stats.NXDomain++
		resp.ErrorMsg = "NXDOMAIN - 域名不存在"
	case 2:
		p.stats.ServFail++
		resp.ErrorMsg = "SERVFAIL - 服务器失败"
	}
	
	// 解析答案
	offset := 12
	if qdcount := binary.BigEndian.Uint16(data[4:6]); qdcount > 0 {
		// 跳过问题部分
		_, offset, _ = parseDNSName(data, offset)
		offset += 4
	}
	
	// 解析资源记录
	answers := parseResourceRecords(data, offset, binary.BigEndian.Uint16(data[6:8]))
	resp.Answers = answers
	
	authorities := parseResourceRecords(data, offset, binary.BigEndian.Uint16(data[8:10]))
	resp.Authorities = authorities
	
	additionals := parseResourceRecords(data, offset, binary.BigEndian.Uint16(data[10:12]))
	resp.Additionals = additionals
	
	return resp, nil
}

// GetStats 获取统计
func (p *DNSParser) GetStats() *DNSStats {
	return p.stats
}

// ============================================================
// MySQL协议深度解析
// ============================================================

// MySQLParser MySQL协议解析器
type MySQLParser struct {
	stats *MySQLStats
}

// MySQLStats MySQL统计
type MySQLStats struct {
	TotalPackets   int64
	TotalQueries   int64
	TotalResponses int64
	Errors         int64
	Warnings       int64
	QueryTypes     map[string]int64
}

// MySQLPacket MySQL数据包
type MySQLPacket struct {
	// 基础信息
	Length    uint32 `json:"length"`
	PacketNum uint8  `json:"packet_num"`
	
	// 类型
	Type      string `json:"type"`
	
	// 深度解析字段
	Query     string `json:"query,omitempty"`
	QueryType string `json:"query_type,omitempty"` // SELECT/INSERT/UPDATE/DELETE
	
	// SQL解析
	SQLCommand string `json:"sql_command,omitempty"`
	Table      string `json:"table,omitempty"`
	Database   string `json:"database,omitempty"`
	
	// 参数绑定信息
	Params    int    `json:"params,omitempty"`
	
	// 错误信息
	ErrorCode    uint16 `json:"error_code,omitempty"`
	ErrorState   string `json:"error_state,omitempty"`
	ErrorMsg     string `json:"error_msg,omitempty"`
	
	// SQL错误详情
	SQLState  string `json:"sql_state,omitempty"`
	SQL errno string `json:"sql_errno,omitempty"`
	
	// 响应信息
	AffectedRows uint64 `json:"affected_rows,omitempty"`
	InsertID     uint64 `json:"insert_id,omitempty"`
	Warnings     uint16 `json:"warnings,omitempty"`
	StatusFlags  uint16 `json:"status_flags,omitempty"`
	
	// 性能指标
	ExecutionTime float64 `json:"execution_time_ms,omitempty"`
	RowsSent      uint64 `json:"rows_sent,omitempty"`
	RowsExamined  uint64 `json:"rows_examined,omitempty"`
}

// MySQLTransaction MySQL完整事务
type MySQLTransaction struct {
	Query    *MySQLPacket `json:"query"`
	Response *MySQLPacket `json:"response,omitempty"`
	Duration time.Duration `json:"duration"`
	Error    string        `json:"error,omitempty"`
}

// NewMySQLParser 创建MySQL解析器
func NewMySQLParser() *MySQLParser {
	return &MySQLParser{
		stats: &MySQLStats{
			QueryTypes: make(map[string]int64),
		},
	}
}

// ParsePacket 解析MySQL数据包
func (p *MySQLParser) ParsePacket(data []byte) (*MySQLPacket, error) {
	if len(data) < 4 {
		return nil, fmt.Errorf("MySQL数据包太短")
	}
	
	p.stats.TotalPackets++
	
	packet := &MySQLPacket{}
	
	// 长度字段 (3 bytes)
	packet.Length = uint32(data[0]) | uint32(data[1])<<8 | uint32(data[2])<<16
	// 包序号 (1 byte)
	packet.PacketNum = data[3]
	
	// MySQL协议类型
	command := data[4]
	
	switch command {
	case 0x01: // COM_QUIT
		packet.Type = "QUIT"
		
	case 0x02: // COM_INIT_DB
		packet.Type = "INIT_DB"
		if len(data) > 5 {
			packet.Database = string(data[5:])
		}
		
	case 0x03: // COM_QUERY
		packet.Type = "QUERY"
		p.stats.TotalQueries++
		if len(data) > 5 {
			sql := string(data[5:])
			packet.Query = sql
			p.parseSQL(sql, packet)
		}
		
	case 0x04: // COM_FIELD_LIST
		packet.Type = "FIELD_LIST"
		if len(data) > 5 {
			idx := strings.Index(string(data[5:]), "\x00")
			if idx >= 0 {
				packet.Table = string(data[5 : 5+idx])
			}
		}
		
	case 0x05: // COM_CREATE_DB
		packet.Type = "CREATE_DB"
		if len(data) > 5 {
			packet.Database = string(data[5:])
		}
		
	case 0x06: // COM_DROP_DB
		packet.Type = "DROP_DB"
		if len(data) > 5 {
			packet.Database = string(data[5:])
		}
		
	case 0x07: // COM_REFRESH
		packet.Type = "REFRESH"
		
	case 0x08: // COM_SHUTDOWN
		packet.Type = "SHUTDOWN"
		
	case 0x09: // COM_STATISTICS
		packet.Type = "STATISTICS"
		
	case 0x0C: // COM_STMT_PREPARE
		packet.Type = "STMT_PREPARE"
		if len(data) > 5 {
			packet.Query = string(data[5:])
			p.parseSQL(packet.Query, packet)
		}
		
	case 0x0D: // COM_STMT_EXECUTE
		packet.Type = "STMT_EXECUTE"
		
	case 0x0E: // COM_STMT_CLOSE
		packet.Type = "STMT_CLOSE"
		
	case 0x16: // COM_STMT_SEND_LONG_DATA
		packet.Type = "STMT_SEND_LONG_DATA"
		
	case 0x1B: // COM_BINLOG_DUMP_GTID
		packet.Type = "BINLOG_DUMP_GTID"
		
	case 0xFE: // EOF
		packet.Type = "EOF"
		
	default:
		packet.Type = fmt.Sprintf("UNKNOWN(0x%02X)", command)
	}
	
	// 更新统计
	if packet.QueryType != "" {
		p.stats.QueryTypes[packet.QueryType]++
	}
	
	return packet, nil
}

// ParseResponsePacket 解析MySQL响应包
func (p *MySQLParser) ParseResponsePacket(data []byte) (*MySQLPacket, error) {
	if len(data) < 5 {
		return nil, fmt.Errorf("响应包太短")
	}
	
	p.stats.TotalResponses++
	
	packet := &MySQLPacket{}
	packet.Length = uint32(data[0]) | uint32(data[1])<<8 | uint32(data[2])<<16
	packet.PacketNum = data[3]
	
	// 检查是否是错误包
	if data[4] == 0xFF {
		packet.Type = "ERR"
		p.stats.Errors++
		return p.parseErrorPacket(data[4:], packet)
	}
	
	// OK包
	if data[4] == 0x00 {
		return p.parseOKPacket(data[4:], packet)
	}
	
	// EOF包
	if data[4] == 0xFE {
		packet.Type = "EOF"
		return packet, nil
	}
	
	// 结果集头
	packet.Type = "RESULT"
	return packet, nil
}

// parseSQL 解析SQL语句
func (p *MySQLParser) parseSQL(sql string, packet *MySQLPacket) {
	sql = strings.TrimSpace(sql)
	upper := strings.ToUpper(sql)
	
	// 识别SQL类型
	if strings.HasPrefix(upper, "SELECT") {
		packet.QueryType = "SELECT"
		packet.SQLCommand = "SELECT"
	} else if strings.HasPrefix(upper, "INSERT") {
		packet.QueryType = "INSERT"
		packet.SQLCommand = "INSERT"
	} else if strings.HasPrefix(upper, "UPDATE") {
		packet.QueryType = "UPDATE"
		packet.SQLCommand = "UPDATE"
	} else if strings.HasPrefix(upper, "DELETE") {
		packet.QueryType = "DELETE"
		packet.SQLCommand = "DELETE"
	} else if strings.HasPrefix(upper, "CREATE") {
		packet.QueryType = "CREATE"
		packet.SQLCommand = "CREATE"
	} else if strings.HasPrefix(upper, "DROP") {
		packet.QueryType = "DROP"
		packet.SQLCommand = "DROP"
	} else if strings.HasPrefix(upper, "ALTER") {
		packet.QueryType = "ALTER"
		packet.SQLCommand = "ALTER"
	} else if strings.HasPrefix(upper, "SET") {
		packet.QueryType = "SET"
		packet.SQLCommand = "SET"
	} else if strings.HasPrefix(upper, "SHOW") {
		packet.QueryType = "SHOW"
		packet.SQLCommand = "SHOW"
	} else if strings.HasPrefix(upper, "DESCRIBE") || strings.HasPrefix(upper, "DESC") {
		packet.QueryType = "DESCRIBE"
		packet.SQLCommand = "DESCRIBE"
	}
	
	// 提取表名（简单实现）
	packet.Table = extractTableName(sql)
}

// parseOKPacket 解析OK包
func (p *MySQLParser) parseOKPacket(data []byte, packet *MySQLPacket) (*MySQLPacket, error) {
	packet.Type = "OK"
	
	offset := 1 // 跳过0x00
	
	// Affected rows
	if len(data) > offset {
		packet.AffectedRows = decodeLengthEncodedInt(data[offset:])
		offset++
	}
	
	// Insert ID
	if len(data) > offset {
		packet.InsertID = decodeLengthEncodedInt(data[offset:])
		offset++
	}
	
	// Status flags
	if len(data) > offset+2 {
		packet.StatusFlags = binary.LittleEndian.Uint16(data[offset : offset+2])
		offset += 2
	}
	
	// Warnings
	if len(data) > offset+2 {
		packet.Warnings = binary.LittleEndian.Uint16(data[offset : offset+2])
		if packet.Warnings > 0 {
			p.stats.Warnings++
		}
	}
	
	return packet, nil
}

// parseErrorPacket 解析错误包
func (p *MySQLParser) parseErrorPacket(data []byte, packet *MySQLPacket) (*MySQLPacket, error) {
	packet.Type = "ERR"
	
	offset := 1 // 跳过 0xFF
	
	if len(data) < offset+2 {
		return packet, nil
	}
	
	packet.ErrorCode = binary.LittleEndian.Uint16(data[offset : offset+2])
	offset += 2
	
	// SQL state marker (#)
	if len(data) > offset && data[offset] == '#' {
		offset++
		packet.SQLState = string(data[offset : offset+5])
		offset += 5
	}
	
	if len(data) > offset {
		packet.ErrorMsg = string(data[offset:])
	}
	
	// 错误信息映射
	packet.ErrorState = mysqlErrorToState(packet.ErrorCode)
	
	return packet, nil
}

// GetStats 获取统计
func (p *MySQLParser) GetStats() *MySQLStats {
	return p.stats
}

// ============================================================
// 工具函数
// ============================================================

func splitLines(data []byte) []string {
	var lines []string
	start := 0
	for i, b := range data {
		if b == '\n' {
			line := string(data[start:i])
			if len(line) > 0 && line[len(line)-1] == '\r' {
				line = line[:len(line)-1]
			}
			lines = append(lines, line)
			start = i + 1
		}
	}
	// 最后一行（如果没有换行符）
	if start < len(data) {
		line := string(data[start:])
		if len(line) > 0 && line[len(line)-1] == '\r' {
			line = line[:len(line)-1]
		}
		lines = append(lines, line)
	}
	return lines
}

func splitPath(path string) []string {
	path = strings.Trim(path, "/")
	if path == "" {
		return nil
	}
	return strings.Split(path, "/")
}

func parseQueryParams(query string) map[string]string {
	params := make(map[string]string)
	if query == "" {
		return params
	}
	
	pairs := strings.Split(query, "&")
	for _, pair := range pairs {
		kv := strings.SplitN(pair, "=", 2)
		if len(kv) == 2 {
			params[kv[0]] = kv[1]
		}
	}
	return params
}

func parseCookies(cookieStr string) map[string]string {
	cookies := make(map[string]string)
	if cookieStr == "" {
		return cookies
	}
	
	pairs := strings.Split(cookieStr, ";")
	for _, pair := range pairs {
		kv := strings.SplitN(strings.TrimSpace(pair), "=", 2)
		if len(kv) == 2 {
			cookies[kv[0]] = kv[1]
		}
	}
	return cookies
}

func parseSetCookie(cookieStr string) *Cookie {
	cookie := &Cookie{}
	
	parts := strings.Split(cookieStr, ";")
	for i, part := range parts {
		part = strings.TrimSpace(part)
		kv := strings.SplitN(part, "=", 2)
		if len(kv) != 2 {
			continue
		}
		
		key := strings.ToLower(strings.TrimSpace(kv[0]))
		value := strings.TrimSpace(kv[1])
		
		if i == 0 {
			cookie.Name = value
			continue
		}
		
		switch key {
		case "path":
			cookie.Path = value
		case "domain":
			cookie.Domain = value
		case "expires":
			cookie.Expires = value
		case "httponly":
			cookie.HttpOnly = true
		case "secure":
			cookie.Secure = true
		}
	}
	
	return cookie
}

func maskAuth(auth string) string {
	if len(auth) < 8 {
		return "***"
	}
	
	parts := strings.SplitN(auth, " ", 2)
	if len(parts) != 2 {
		return "***"
	}
	
	scheme := strings.ToUpper(parts[0])
	switch scheme {
	case "BASIC":
		// 只显示用户名，不显示密码
		creds := parts[1]
		if idx := strings.Index(creds, ":"); idx >= 0 {
			return scheme + " " + creds[:idx] + ":***"
		}
		return scheme + " ***"
	case "BEARER", "TOKEN":
		return scheme + " ***"
	}
	
	return "***"
}

func classifyHTTPError(statusCode int) (string, string) {
	switch {
	case statusCode >= 400 && statusCode < 500:
		return "CLIENT_ERROR", http.StatusText(statusCode)
	case statusCode >= 500 && statusCode < 600:
		return "SERVER_ERROR", http.StatusText(statusCode)
	case statusCode == 301, statusCode == 302, statusCode == 303, statusCode == 307, statusCode == 308:
		return "REDIRECT", http.StatusText(statusCode)
	}
	return "", ""
}

// DNS协议解析辅助函数
func parseDNSName(data []byte, offset int) (string, int, error) {
	var name []string
	origOffset := offset
	
	for {
		if offset >= len(data) {
			return "", offset, fmt.Errorf("DNS名称解析超出数据范围")
		}
		
		length := int(data[offset])
		
		if length == 0 {
			offset++
			break
		}
		
		// 压缩指针
		if length&0xC0 == 0xC0 {
			if offset+1 >= len(data) {
				return "", offset, fmt.Errorf("DNS压缩指针不完整")
			}
			ptr := int(uint16(data[offset])&0x3F)<<8 | int(data[offset+1])
			ptrName, _, _ := parseDNSName(data, ptr)
			name = append(name, ptrName)
			offset += 2
			return strings.Join(name, "."), offset, nil
		}
		
		if offset+1+length > len(data) {
			return "", offset, fmt.Errorf("DNS名称标签超出范围")
		}
		
		name = append(name, string(data[offset+1:offset+1+length]))
		offset += 1 + length
	}
	
	return strings.Join(name, "."), offset, nil
}

func splitDNSName(name string) []string {
	if name == "" {
		return nil
	}
	return strings.Split(name, ".")
}

var dnsTypes = map[uint16]string{
	1:   "A",
	2:   "NS",
	5:   "CNAME",
	6:   "SOA",
	12:  "PTR",
	15:  "MX",
	16:  "TXT",
	28:  "AAAA",
	33:  "SRV",
	35:  "NAPTR",
	252: "AXFR",
	255: "ANY",
}

var dnsClasses = map[uint16]string{
	1: "IN",
	2: "CS",
	3: "CH",
	4: "HS",
}

var dnsRcodes = map[uint16]string{
	0:  "NOERROR",
	1:  "FORMERR",
	2:  "SERVFAIL",
	3:  "NXDOMAIN",
	4:  "NOTIMP",
	5:  "REFUSED",
	6:  "YXDOMAIN",
	7:  "YXRRSET",
	8:  "NXRRSET",
	9:  "NOTAUTH",
	10: "NOTZONE",
}

func dnsTypeToString(t uint16) string {
	if s, ok := dnsTypes[t]; ok {
		return s
	}
	return fmt.Sprintf("TYPE%d", t)
}

func dnsClassToString(c uint16) string {
	if s, ok := dnsClasses[c]; ok {
		return s
	}
	return fmt.Sprintf("CLASS%d", c)
}

func dnsRcodeToString(r uint16) string {
	if s, ok := dnsRcodes[r]; ok {
		return s
	}
	return fmt.Sprintf("RCODE%d", r)
}

func parseResourceRecords(data []byte, offset int, count uint16) []DNSResourceRecord {
	var records []DNSResourceRecord
	
	for i := uint16(0); i < count && offset < len(data); i++ {
		rr := DNSResourceRecord{}
		
		// 解析名称
		name, newOffset, err := parseDNSName(data, offset)
		if err != nil {
			break
		}
		rr.Name = name
		offset = newOffset
		
		// 解析类型和类
		if offset+10 > len(data) {
			break
		}
		rr.Type = dnsTypeToString(binary.BigEndian.Uint16(data[offset:]))
		offset += 2
		offset += 2 // class
		rr.TTL = binary.BigEndian.Uint32(data[offset:])
		offset += 4
		rdlen := binary.BigEndian.Uint16(data[offset:])
		offset += 2
		
		// 解析RData
		if offset+int(rdlen) <= len(data) {
			rr.RData = parseRData(data[offset:offset+int(rdlen)], rr.Type)
		}
		offset += int(rdlen)
		
		records = append(records, rr)
	}
	
	return records
}

func parseRData(data []byte, rtype string) string {
	switch rtype {
	case "A":
		if len(data) >= 4 {
			return fmt.Sprintf("%d.%d.%d.%d", data[0], data[1], data[2], data[3])
		}
	case "AAAA":
		if len(data) >= 16 {
			var parts []string
			for i := 0; i < 16; i += 2 {
				parts = append(parts, fmt.Sprintf("%x", binary.BigEndian.Uint16(data[i:])))
			}
			return strings.Join(parts, ":")
		}
	case "TXT", "SPF":
		if len(data) > 0 {
			return string(data[1:])
		}
	case "CNAME", "NS", "PTR":
		name, _, _ := parseDNSName(data, 0)
		return name
	}
	
	return hex.EncodeToString(data)
}

// MySQL协议解析辅助函数
func decodeLengthEncodedInt(data []byte) uint64 {
	if len(data) == 0 {
		return 0
	}
	
	first := data[0]
	
	switch {
	case first < 0xFB:
		return uint64(first)
	case first == 0xFC:
		if len(data) >= 3 {
			return uint64(data[1]) | uint64(data[2])<<8
		}
	case first == 0xFD:
		if len(data) >= 4 {
			return uint64(data[1]) | uint64(data[2])<<8 | uint64(data[3])<<16
		}
	case first == 0xFE:
		if len(data) >= 9 {
			return binary.LittleEndian.Uint64(data[1:])
		}
	}
	
	return 0
}

func extractTableName(sql string) string {
	// 简单实现：提取FROM/JOIN/INTO/UPDATE后第一个词
	upper := strings.ToUpper(sql)
	
	patterns := []struct {
		keyword string
		skip    int
	}{
		{"FROM", 1},
		{"JOIN", 1},
		{"INTO", 1},
		{"UPDATE", 1},
		{"TABLE", 1},
	}
	
	for _, p := range patterns {
		idx := strings.Index(upper, p.keyword)
		if idx >= 0 {
			remaining := strings.Trim(sql[idx+len(p.keyword):], " \t\n\r(")
			parts := strings.Fields(remaining)
			if len(parts) > p.skip {
				// 去除可能的AS别名
				name := parts[p.skip]
				if idx := strings.Index(name, ","); idx >= 0 {
					name = name[:idx]
				}
				if idx := strings.Index(strings.ToUpper(name), " AS "); idx >= 0 {
					name = name[:idx]
				}
				return strings.Trim(name, "`\"'")
			}
		}
	}
	
	return ""
}

func mysqlErrorToState(errCode uint16) string {
	errors := map[uint16]string{
		1045: "28000", // Access denied
		1062: "23000", // Duplicate entry
		1146: "42S02", // Table doesn't exist
		1451: "23000", // Foreign key constraint
		1452: "23000", // Foreign key constraint
		2006: "08000", // MySQL server has gone away
		2013: "08000", // Lost connection during query
	}
	
	if s, ok := errors[errCode]; ok {
		return s
	}
	return "HY000"
}

// 正则表达式缓存
var (
	pathPattern     = regexp.MustCompile(`^([A-Z]+)\s+([^\s?]+)(?:\?(.+))?\s+HTTP`)
	statusPattern   = regexp.MustCompile(`^HTTP/[\d.]+\s+(\d+)\s*(.*)`)
)

// 确保textproto已导入（用于标准库兼容）
var _ = textproto.Errorf
var _ = net.Dial
