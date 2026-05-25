// Package netmetrics 核心网络指标采集
// TCP建连时延、零窗口、队列溢出
package netmetrics

import (
	"bufio"
	"fmt"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"
)

// ============================================================
// TCP核心指标模型
// ============================================================

// TCPConnectionMetrics TCP连接指标
type TCPConnectionMetrics struct {
	Timestamp       time.Time `json:"timestamp"`
	LocalAddr       string    `json:"local_addr"`
	RemoteAddr      string    `json:"remote_addr"`
	LocalPort       uint16    `json:"local_port"`
	RemotePort      uint16    `json:"remote_port"`
	State           string    `json:"state"`
	
	// 建连时延
	ConnectLatency  float64   `json:"connect_latency_ms"`  // TCP建连时延 (ms)
	HandshakeTime   float64   `json:"handshake_time_ms"`   // 三次握手时间 (ms)
	
	// 窗口相关
	SendWindow      uint32    `json:"send_window"`         // 发送窗口大小
	RecvWindow      uint32    `json:"recv_window"`         // 接收窗口大小
	ZeroWindowCount int64     `json:"zero_window_count"`   // 零窗口事件次数
	WindowFullTime  float64   `json:"window_full_time_ms"` // 窗口满持续时间 (ms)
	
	// 队列相关
	SendQueue       uint32    `json:"send_queue"`          // 发送队列长度
	RecvQueue       uint32    `json:"recv_queue"`          // 接收队列长度
	SendQueueDrops  int64     `json:"send_queue_drops"`    // 发送队列丢包数
	RecvQueueDrops  int64     `json:"recv_queue_drops"`    // 接收队列丢包数
	OverflowCount   int64     `json:"overflow_count"`      // 队列溢出次数
	
	// 重传相关
	RetransCount    int64     `json:"retrans_count"`       // 重传次数
	FastRetrans     int64     `json:"fast_retrans"`        // 快速重传次数
	TimeoutRetrans  int64     `json:"timeout_retrans"`     // 超时重传次数
}

// TCPGlobalMetrics TCP全局指标
type TCPGlobalMetrics struct {
	Timestamp           time.Time `json:"timestamp"`
	
	// 连接统计
	ActiveOpens         int64     `json:"active_opens"`          // 主动打开次数
	PassiveOpens        int64     `json:"passive_opens"`         // 被动打开次数
	AttemptFails        int64     `json:"attempt_fails"`         // 连接尝试失败
	EstabResets         int64     `json:"estab_resets"`          // 已建立连接重置
	CurrEstab           int64     `json:"curr_estab"`            // 当前建立连接数
	
	// 建连时延统计
	ConnectLatencies    []float64 `json:"connect_latencies_ms,omitempty"`
	AvgConnectLatency   float64   `json:"avg_connect_latency_ms"`
	P99ConnectLatency   float64   `json:"p99_connect_latency_ms"`
	MaxConnectLatency   float64   `json:"max_connect_latency_ms"`
	
	// 零窗口统计
	ZeroWindowEvents    int64     `json:"zero_window_events"`    // 零窗口事件总数
	ZeroWindowDuration  float64   `json:"zero_window_duration_ms"` // 零窗口总持续时间
	
	// 队列溢出统计
	ListenOverflows     int64     `json:"listen_overflows"`      // 监听队列溢出
	ListenDrops         int64     `json:"listen_drops"`          // 监听队列丢包
	SyncookiesSent      int64     `json:"syncookies_sent"`       // SYN Cookies发送
	SyncookiesRecv      int64     `json:"syncookies_recv"`       // SYN Cookies接收
	SyncookiesFailed    int64     `json:"syncookies_failed"`     // SYN Cookies失败
	
	// 重传统计
	RetransSegs         int64     `json:"retrans_segs"`          // 重传段数
	RetransRatio        float64   `json:"retrans_ratio"`         // 重传率 (%)
}

// ============================================================
// TCP指标采集器
// ============================================================

// TCPMetricsCollector TCP指标采集器
type TCPMetricsCollector struct {
	mu              sync.RWMutex
	connections     map[string]*TCPConnectionMetrics
	history         []*TCPGlobalMetrics
	lastGlobalStats map[string]int64 // 上次全局统计，用于计算差值
	
	// 配置
	sampleInterval  time.Duration
	maxHistorySize  int
}

// NewTCPMetricsCollector 创建TCP指标采集器
func NewTCPMetricsCollector() *TCPMetricsCollector {
	return &TCPMetricsCollector{
		connections:     make(map[string]*TCPConnectionMetrics),
		history:         make([]*TCPGlobalMetrics, 0),
		lastGlobalStats: make(map[string]int64),
		sampleInterval:  5 * time.Second,
		maxHistorySize:  1440, // 保留2小时数据 (5秒间隔)
	}
}

// CollectGlobal 采集全局TCP指标
func (c *TCPMetricsCollector) CollectGlobal() (*TCPGlobalMetrics, error) {
	metrics := &TCPGlobalMetrics{
		Timestamp: time.Now(),
	}
	
	// 读取 /proc/net/snmp
	snmpData, err := c.readSNMP()
	if err != nil {
		return nil, fmt.Errorf("读取SNMP失败: %w", err)
	}
	
	// 解析TCP统计
	c.parseTCPStats(metrics, snmpData)
	
	// 读取 /proc/net/netstat
	netstatData, err := c.readNetstat()
	if err == nil {
		c.parseNetstat(metrics, netstatData)
	}
	
	// 采集连接级指标
	c.collectConnectionMetrics(metrics)
	
	// 计算时延统计
	c.calculateLatencyStats(metrics)
	
	// 保存历史
	c.addToHistory(metrics)
	
	return metrics, nil
}

// readSNMP 读取SNMP数据
func (c *TCPMetricsCollector) readSNMP() (map[string]int64, error) {
	data, err := os.ReadFile("/proc/net/snmp")
	if err != nil {
		return nil, err
	}
	
	result := make(map[string]int64)
	lines := strings.Split(string(data), "\n")
	
	var tcpNames []string
	for i, line := range lines {
		if strings.HasPrefix(line, "Tcp:") {
			if i+1 < len(lines) {
				// 第一行是字段名
				fields := strings.Fields(line)
				if len(fields) > 1 {
					tcpNames = fields[1:]
				}
				
				// 第二行是数值
				values := strings.Fields(lines[i+1])
				if len(values) > 1 {
					values = values[1:]
					for j, name := range tcpNames {
						if j < len(values) {
							val, _ := strconv.ParseInt(values[j], 10, 64)
							result[name] = val
						}
					}
				}
			}
			break
		}
	}
	
	return result, nil
}

// parseTCPStats 解析TCP统计
func (c *TCPMetricsCollector) parseTCPStats(metrics *TCPGlobalMetrics, data map[string]int64) {
	// 连接统计
	metrics.ActiveOpens = data["ActiveOpens"]
	metrics.PassiveOpens = data["PassiveOpens"]
	metrics.AttemptFails = data["AttemptFails"]
	metrics.EstabResets = data["EstabResets"]
	metrics.CurrEstab = data["CurrEstab"]
	
	// 计算重传率
	outSegs := data["OutSegs"]
	retransSegs := data["RetransSegs"]
	metrics.RetransSegs = retransSegs
	if outSegs > 0 {
		metrics.RetransRatio = float64(retransSegs) / float64(outSegs) * 100
	}
	
	// 保存当前统计用于下次计算
	for k, v := range data {
		c.lastGlobalStats[k] = v
	}
}

// readNetstat 读取netstat数据
func (c *TCPMetricsCollector) readNetstat() (map[string]int64, error) {
	data, err := os.ReadFile("/proc/net/netstat")
	if err != nil {
		return nil, err
	}
	
	result := make(map[string]int64)
	lines := strings.Split(string(data), "\n")
	
	// 查找TcpExt行
	for i := 0; i < len(lines)-1; i++ {
		if strings.HasPrefix(lines[i], "TcpExt:") {
			names := strings.Fields(lines[i])
			values := strings.Fields(lines[i+1])
			
			if len(names) == len(values) {
				for j := 1; j < len(names); j++ {
					val, _ := strconv.ParseInt(values[j], 10, 64)
					result[names[j]] = val
				}
			}
		}
	}
	
	return result, nil
}

// parseNetstat 解析netstat数据
func (c *TCPMetricsCollector) parseNetstat(metrics *TCPGlobalMetrics, data map[string]int64) {
	// 零窗口统计
	metrics.ZeroWindowEvents = data["TCPZeroWindowDrop"]
	
	// 队列溢出统计
	metrics.ListenOverflows = data["ListenOverflows"]
	metrics.ListenDrops = data["ListenDrops"]
	metrics.SyncookiesSent = data["SyncookiesSent"]
	metrics.SyncookiesRecv = data["SyncookiesRecv"]
	metrics.SyncookiesFailed = data["SyncookiesFailed"]
}

// collectConnectionMetrics 采集连接级指标
func (c *TCPMetricsCollector) collectConnectionMetrics(global *TCPGlobalMetrics) {
	// 读取 /proc/net/tcp
	data, err := os.ReadFile("/proc/net/tcp")
	if err != nil {
		return
	}
	
	var latencies []float64
	now := time.Now()
	
	lines := strings.Split(string(data), "\n")
	for i, line := range lines {
		if i == 0 || line == "" {
			continue // 跳过标题行
		}
		
		fields := strings.Fields(line)
		if len(fields) < 10 {
			continue
		}
		
		// 解析连接信息
		conn := c.parseConnectionLine(fields)
		if conn == nil {
			continue
		}
		
		conn.Timestamp = now
		
		// 存储连接信息
		key := fmt.Sprintf("%s:%d-%s:%d", conn.LocalAddr, conn.LocalPort, conn.RemoteAddr, conn.RemotePort)
		c.mu.Lock()
		c.connections[key] = conn
		c.mu.Unlock()
		
		// 收集建连时延（仅ESTABLISHED状态）
		if conn.State == "ESTABLISHED" && conn.ConnectLatency > 0 {
			latencies = append(latencies, conn.ConnectLatency)
		}
	}
	
	global.ConnectLatencies = latencies
}

// parseConnectionLine 解析连接行
func (c *TCPMetricsCollector) parseConnectionLine(fields []string) *TCPConnectionMetrics {
	conn := &TCPConnectionMetrics{}
	
	// 本地地址
	localAddr := fields[1]
	if ip, port, err := parseAddr(localAddr); err == nil {
		conn.LocalAddr = ip
		conn.LocalPort = port
	}
	
	// 远程地址
	remoteAddr := fields[2]
	if ip, port, err := parseAddr(remoteAddr); err == nil {
		conn.RemoteAddr = ip
		conn.RemotePort = port
	}
	
	// 状态
	stateHex := fields[3]
	state, _ := strconv.ParseInt(stateHex, 16, 64)
	conn.State = tcpStateToString(int(state))
	
	// 发送/接收队列
	txQueue := fields[4]
	queues := strings.Split(txQueue, ":")
	if len(queues) == 2 {
		sendQ, _ := strconv.ParseUint(queues[0], 16, 32)
		recvQ, _ := strconv.ParseUint(queues[1], 16, 32)
		conn.SendQueue = uint32(sendQ)
		conn.RecvQueue = uint32(recvQ)
	}
	
	// 窗口大小
	if len(fields) > 6 {
		timerActive := fields[6]
		_ = timerActive
	}
	
	// 重传超时次数
	if len(fields) > 12 {
		retrans, _ := strconv.ParseInt(fields[12], 10, 64)
		conn.RetransCount = retrans
	}
	
	return conn
}

// calculateLatencyStats 计算时延统计
func (c *TCPMetricsCollector) calculateLatencyStats(metrics *TCPGlobalMetrics) {
	latencies := metrics.ConnectLatencies
	if len(latencies) == 0 {
		return
	}
	
	// 排序计算P99
	sortFloat64s(latencies)
	
	// 平均值
	sum := 0.0
	for _, v := range latencies {
		sum += v
	}
	metrics.AvgConnectLatency = sum / float64(len(latencies))
	
	// P99
	p99Idx := int(float64(len(latencies)) * 0.99)
	if p99Idx >= len(latencies) {
		p99Idx = len(latencies) - 1
	}
	metrics.P99ConnectLatency = latencies[p99Idx]
	
	// 最大值
	metrics.MaxConnectLatency = latencies[len(latencies)-1]
}

// addToHistory 添加到历史
func (c *TCPMetricsCollector) addToHistory(metrics *TCPGlobalMetrics) {
	c.mu.Lock()
	defer c.mu.Unlock()
	
	c.history = append(c.history, metrics)
	
	// 限制历史大小
	if len(c.history) > c.maxHistorySize {
		c.history = c.history[len(c.history)-c.maxHistorySize:]
	}
}

// GetHistory 获取历史数据
func (c *TCPMetricsCollector) GetHistory() []*TCPGlobalMetrics {
	c.mu.RLock()
	defer c.mu.RUnlock()
	
	result := make([]*TCPGlobalMetrics, len(c.history))
	copy(result, c.history)
	return result
}

// GetConnection 获取特定连接信息
func (c *TCPMetricsCollector) GetConnection(localAddr string, localPort uint16, 
	remoteAddr string, remotePort uint16) *TCPConnectionMetrics {
	c.mu.RLock()
	defer c.mu.RUnlock()
	
	key := fmt.Sprintf("%s:%d-%s:%d", localAddr, localPort, remoteAddr, remotePort)
	return c.connections[key]
}

// GetConnectionsByState 按状态获取连接
func (c *TCPMetricsCollector) GetConnectionsByState(state string) []*TCPConnectionMetrics {
	c.mu.RLock()
	defer c.mu.RUnlock()
	
	var result []*TCPConnectionMetrics
	for _, conn := range c.connections {
		if conn.State == state {
			result = append(result, conn)
		}
	}
	return result
}

// ============================================================
// 零窗口检测器
// ============================================================

// ZeroWindowDetector 零窗口检测器
type ZeroWindowDetector struct {
	mu          sync.RWMutex
	zeroWindows map[string]*ZeroWindowEvent
	onEvent     func(event *ZeroWindowEvent)
}

// ZeroWindowEvent 零窗口事件
type ZeroWindowEvent struct {
	ConnectionKey   string    `json:"connection_key"`
	LocalAddr       string    `json:"local_addr"`
	RemoteAddr      string    `json:"remote_addr"`
	StartTime       time.Time `json:"start_time"`
	EndTime         time.Time `json:"end_time,omitempty"`
	Duration        float64   `json:"duration_ms"`
	Resolved        bool      `json:"resolved"`
	ZeroWindowCount int       `json:"zero_window_count"`
}

// NewZeroWindowDetector 创建零窗口检测器
func NewZeroWindowDetector() *ZeroWindowDetector {
	return &ZeroWindowDetector{
		zeroWindows: make(map[string]*ZeroWindowEvent),
	}
}

// OnEvent 设置事件回调
func (z *ZeroWindowDetector) OnEvent(cb func(event *ZeroWindowEvent)) {
	z.onEvent = cb
}

// Check 检查零窗口
func (z *ZeroWindowDetector) Check(conn *TCPConnectionMetrics) {
	key := fmt.Sprintf("%s:%d-%s:%d", conn.LocalAddr, conn.LocalPort, conn.RemoteAddr, conn.RemotePort)
	
	z.mu.Lock()
	defer z.mu.Unlock()
	
	// 检查发送窗口是否为0
	if conn.SendWindow == 0 {
		event, exists := z.zeroWindows[key]
		if !exists {
			// 新零窗口事件
			event = &ZeroWindowEvent{
				ConnectionKey:   key,
				LocalAddr:       conn.LocalAddr,
				RemoteAddr:      conn.RemoteAddr,
				StartTime:       conn.Timestamp,
				ZeroWindowCount: 1,
			}
			z.zeroWindows[key] = event
			
			if z.onEvent != nil {
				go z.onEvent(event)
			}
		} else {
			event.ZeroWindowCount++
		}
	} else {
		// 零窗口恢复
		if event, exists := z.zeroWindows[key]; exists && !event.Resolved {
			event.EndTime = conn.Timestamp
			event.Duration = event.EndTime.Sub(event.StartTime).Seconds() * 1000
			event.Resolved = true
			
			if z.onEvent != nil {
				go z.onEvent(event)
			}
		}
	}
}

// GetActiveEvents 获取活跃的零窗口事件
func (z *ZeroWindowDetector) GetActiveEvents() []*ZeroWindowEvent {
	z.mu.RLock()
	defer z.mu.RUnlock()
	
	var result []*ZeroWindowEvent
	for _, event := range z.zeroWindows {
		if !event.Resolved {
			result = append(result, event)
		}
	}
	return result
}

// ============================================================
// 队列溢出检测器
// ============================================================

// QueueOverflowDetector 队列溢出检测器
type QueueOverflowDetector struct {
	mu           sync.RWMutex
	overflows    map[string]*QueueOverflowEvent
	onEvent      func(event *QueueOverflowEvent)
	lastDrops    int64
	lastOverflow int64
}

// QueueOverflowEvent 队列溢出事件
type QueueOverflowEvent struct {
	Timestamp   time.Time `json:"timestamp"`
	Type        string    `json:"type"`        // listen, send, recv
	Count       int64     `json:"count"`
	TotalDrops  int64     `json:"total_drops"`
	Severity    string    `json:"severity"`    // warning, critical
}

// NewQueueOverflowDetector 创建队列溢出检测器
func NewQueueOverflowDetector() *QueueOverflowDetector {
	return &QueueOverflowDetector{
		overflows: make(map[string]*QueueOverflowEvent),
	}
}

// OnEvent 设置事件回调
func (q *QueueOverflowDetector) OnEvent(cb func(event *QueueOverflowEvent)) {
	q.onEvent = cb
}

// CheckGlobal 检查全局队列溢出
func (q *QueueOverflowDetector) CheckGlobal(metrics *TCPGlobalMetrics) {
	// 检查监听队列溢出
	if metrics.ListenOverflows > q.lastOverflow {
		event := &QueueOverflowEvent{
			Timestamp:  metrics.Timestamp,
			Type:       "listen",
			Count:      metrics.ListenOverflows - q.lastOverflow,
			TotalDrops: metrics.ListenOverflows,
			Severity:   "critical",
		}
		
		if q.onEvent != nil {
			go q.onEvent(event)
		}
		
		q.lastOverflow = metrics.ListenOverflows
	}
	
	// 检查监听队列丢包
	if metrics.ListenDrops > q.lastDrops {
		event := &QueueOverflowEvent{
			Timestamp:  metrics.Timestamp,
			Type:       "listen_drop",
			Count:      metrics.ListenDrops - q.lastDrops,
			TotalDrops: metrics.ListenDrops,
			Severity:   "warning",
		}
		
		if q.onEvent != nil {
			go q.onEvent(event)
		}
		
		q.lastDrops = metrics.ListenDrops
	}
}

// CheckConnection 检查连接队列
func (q *QueueOverflowDetector) CheckConnection(conn *TCPConnectionMetrics) {
	// 检查发送队列是否接近满
	if conn.SendQueue > 100000 { // 100KB
		event := &QueueOverflowEvent{
			Timestamp: conn.Timestamp,
			Type:      "send_queue_high",
			Count:     1,
			Severity:  "warning",
		}
		
		if q.onEvent != nil {
			go q.onEvent(event)
		}
	}
	
	// 检查接收队列是否接近满
	if conn.RecvQueue > 100000 {
		event := &QueueOverflowEvent{
			Timestamp: conn.Timestamp,
			Type:      "recv_queue_high",
			Count:     1,
			Severity:  "warning",
		}
		
		if q.onEvent != nil {
			go q.onEvent(event)
		}
	}
}

// ============================================================
// 工具函数
// ============================================================

// parseAddr 解析地址
func parseAddr(addr string) (string, uint16, error) {
	parts := strings.Split(addr, ":")
	if len(parts) != 2 {
		return "", 0, fmt.Errorf("无效地址格式")
	}
	
	// 解析IP (小端序十六进制)
	ipHex := parts[0]
	if len(ipHex) == 8 {
		// IPv4
		ipBytes := make([]byte, 4)
		for i := 0; i < 4; i++ {
			b, _ := strconv.ParseUint(ipHex[i*2:i*2+2], 16, 8)
			ipBytes[3-i] = byte(b) // 反转字节序
		}
		ip := fmt.Sprintf("%d.%d.%d.%d", ipBytes[0], ipBytes[1], ipBytes[2], ipBytes[3])
		
		// 解析端口
		port, _ := strconv.ParseUint(parts[1], 16, 16)
		
		return ip, uint16(port), nil
	}
	
	return "", 0, fmt.Errorf("不支持IPv6")
}

// tcpStateToString TCP状态码转字符串
func tcpStateToString(state int) string {
	states := map[int]string{
		1:  "ESTABLISHED",
		2:  "SYN_SENT",
		3:  "SYN_RECV",
		4:  "FIN_WAIT1",
		5:  "FIN_WAIT2",
		6:  "TIME_WAIT",
		7:  "CLOSE",
		8:  "CLOSE_WAIT",
		9:  "LAST_ACK",
		10: "LISTEN",
		11: "CLOSING",
		12: "NEW_SYN_RECV",
	}
	
	if s, ok := states[state]; ok {
		return s
	}
	return fmt.Sprintf("UNKNOWN(%d)", state)
}

// sortFloat64s 排序float64切片
func sortFloat64s(data []float64) {
	// 简单冒泡排序
	for i := 0; i < len(data); i++ {
		for j := i + 1; j < len(data); j++ {
			if data[i] > data[j] {
				data[i], data[j] = data[j], data[i]
			}
		}
	}
}
