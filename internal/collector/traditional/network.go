//go:build linux

// Package traditional 提供传统方式采集网络流量（用于低内核降级）
package traditional

import (
	"bufio"
	"context"
	"encoding/binary"
	"errors"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/vishvananda/netlink"
	"github.com/vishvananda/netlink/nl"

	"github.com/meinanzilinzhengying/cloud-flow-agent/pkg/models"
)

// NetworkCollector 传统网络采集器
type NetworkCollector struct {
	config    *models.TraditionalCollectorConfig
	status    models.CollectorStatus
	mu        sync.RWMutex
	ctx       context.Context
	cancel    context.CancelFunc

	// 数据通道
	flowChan   chan *models.NetworkFlow
	errorChan  chan error

	// 连接缓存
	connCache map[string]*connectionInfo

	// 统计
	eventsCount uint64
	dropCount   uint64
}

// connectionInfo 连接信息缓存
type connectionInfo struct {
	LastSeen   time.Time
	BytesSent  uint64
	BytesRecv  uint64
	PacketsSent uint64
	PacketsRecv uint64
}

// NewNetworkCollector 创建传统网络采集器
func NewNetworkCollector() *NetworkCollector {
	return &NetworkCollector{
		flowChan:  make(chan *models.NetworkFlow, 10000),
		errorChan: make(chan error, 100),
		connCache: make(map[string]*connectionInfo),
	}
}

// Name 返回采集器名称
func (c *NetworkCollector) Name() string {
	return "traditional-network"
}

// Type 返回采集器类型
func (c *NetworkCollector) Type() models.CollectorType {
	return models.CollectorTraditional
}

// Init 初始化采集器
func (c *NetworkCollector) Init(ctx context.Context, config interface{}) error {
	cfg, ok := config.(*models.TraditionalCollectorConfig)
	if !ok {
		return errors.New("invalid config type")
	}
	c.config = cfg

	c.ctx, c.cancel = context.WithCancel(ctx)

	c.status = models.CollectorStatus{
		Name:      c.Name(),
		Type:      c.Type(),
		Enabled:   cfg.Enabled,
		StartTime: time.Now(),
	}

	return nil
}

// Start 启动采集器
func (c *NetworkCollector) Start(ctx context.Context) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if !c.config.Enabled {
		return nil
	}

	// 启动 /proc/net 采集协程
	go c.pollProcNet()

	// 启动 netlink 监听协程
	go c.listenNetlink()

	c.status.Running = true
	c.status.StartTime = time.Now()

	return nil
}

// Stop 停止采集器
func (c *NetworkCollector) Stop(ctx context.Context) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.cancel != nil {
		c.cancel()
	}

	close(c.flowChan)
	close(c.errorChan)

	c.status.Running = false

	return nil
}

// Status 返回采集器状态
func (c *NetworkCollector) Status() models.CollectorStatus {
	c.mu.RLock()
	defer c.mu.RUnlock()

	status := c.status
	status.EventsCount = c.eventsCount
	status.DropCount = c.dropCount

	return status
}

// Flows 返回网络流量数据通道
func (c *NetworkCollector) Flows() <-chan *models.NetworkFlow {
	return c.flowChan
}

// Events 返回事件通道
func (c *NetworkCollector) Events() <-chan interface{} {
	ch := make(chan interface{})
	go func() {
		for flow := range c.flowChan {
			ch <- flow
		}
		close(ch)
	}()
	return ch
}

// Errors 返回错误通道
func (c *NetworkCollector) Errors() <-chan error {
	return c.errorChan
}

// pollProcNet 定期采集 /proc/net
func (c *NetworkCollector) pollProcNet() {
	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-c.ctx.Done():
			return
		case <-ticker.C:
			c.collectTCPConnections()
			c.collectUDPConnections()
		}
	}
}

// collectTCPConnections 采集 TCP 连接
func (c *NetworkCollector) collectTCPConnections() {
	// 读取 /proc/net/tcp 和 /proc/net/tcp6
	c.readProcNetTCP("/proc/net/tcp", false)
	c.readProcNetTCP("/proc/net/tcp6", true)
}

// readProcNetTCP 读取 /proc/net/tcp 文件
func (c *NetworkCollector) readProcNetTCP(path string, isIPv6 bool) {
	file, err := os.Open(path)
	if err != nil {
		return
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	// 跳过标题行
	scanner.Scan()

	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}

		flow := c.parseTCPLine(line, isIPv6)
		if flow != nil {
			select {
			case c.flowChan <- flow:
				c.mu.Lock()
				c.eventsCount++
				c.mu.Unlock()
			default:
				c.mu.Lock()
				c.dropCount++
				c.mu.Unlock()
			}
		}
	}
}

// parseTCPLine 解析 TCP 连接行
func (c *NetworkCollector) parseTCPLine(line string, isIPv6 bool) *models.NetworkFlow {
	fields := strings.Fields(line)
	if len(fields) < 10 {
		return nil
	}

	// 解析本地地址和端口
	localAddr, localPort := parseAddrPort(fields[1])
	// 解析远程地址和端口
	remoteAddr, remotePort := parseAddrPort(fields[2])

	// 解析状态
	state, _ := strconv.ParseInt(fields[3], 16, 32)

	// 解析 inode
	inode, _ := strconv.ParseInt(fields[9], 10, 64)

	// 查找进程
	pid, comm := c.findProcessByInode(inode)

	return &models.NetworkFlow{
		Timestamp:     time.Now(),
		Protocol:      "TCP",
		SourceIP:      localAddr,
		SourcePort:    localPort,
		DestIP:        remoteAddr,
		DestPort:      remotePort,
		ProcessName:   comm,
		ProcessPID:    uint32(pid),
		TCPState:      tcpState(int(state)),
		CollectorType: models.CollectorTraditional,
	}
}

// collectUDPConnections 采集 UDP 连接
func (c *NetworkCollector) collectUDPConnections() {
	c.readProcNetUDP("/proc/net/udp", false)
	c.readProcNetUDP("/proc/net/udp6", true)
}

// readProcNetUDP 读取 /proc/net/udp 文件
func (c *NetworkCollector) readProcNetUDP(path string, isIPv6 bool) {
	file, err := os.Open(path)
	if err != nil {
		return
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	scanner.Scan() // 跳过标题行

	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}

		flow := c.parseUDPLine(line, isIPv6)
		if flow != nil {
			select {
			case c.flowChan <- flow:
				c.mu.Lock()
				c.eventsCount++
				c.mu.Unlock()
			default:
				c.mu.Lock()
				c.dropCount++
				c.mu.Unlock()
			}
		}
	}
}

// parseUDPLine 解析 UDP 连接行
func (c *NetworkCollector) parseUDPLine(line string, isIPv6 bool) *models.NetworkFlow {
	fields := strings.Fields(line)
	if len(fields) < 10 {
		return nil
	}

	localAddr, localPort := parseAddrPort(fields[1])
	remoteAddr, remotePort := parseAddrPort(fields[2])
	inode, _ := strconv.ParseInt(fields[9], 10, 64)

	pid, comm := c.findProcessByInode(inode)

	return &models.NetworkFlow{
		Timestamp:     time.Now(),
		Protocol:      "UDP",
		SourceIP:      localAddr,
		SourcePort:    localPort,
		DestIP:        remoteAddr,
		DestPort:      remotePort,
		ProcessName:   comm,
		ProcessPID:    uint32(pid),
		CollectorType: models.CollectorTraditional,
	}
}

// parseAddrPort 解析地址和端口
func parseAddrPort(s string) (string, uint16) {
	parts := strings.Split(s, ":")
	if len(parts) != 2 {
		return "", 0
	}

	// 解析端口
	port, _ := strconv.ParseUint(parts[1], 16, 16)

	// 解析地址
	addr := parseHexIP(parts[0])

	return addr, uint16(port)
}

// parseHexIP 解析十六进制 IP 地址
func parseHexIP(hex string) string {
	if len(hex) == 8 {
		// IPv4
		b := make([]byte, 4)
		for i := 0; i < 4; i++ {
			v, _ := strconv.ParseUint(hex[6-2*i:8-2*i], 16, 8)
			b[i] = byte(v)
		}
		return net.IP(b).String()
	} else if len(hex) == 32 {
		// IPv6
		b := make([]byte, 16)
		for i := 0; i < 16; i++ {
			v, _ := strconv.ParseUint(hex[30-2*i:32-2*i], 16, 8)
			b[i] = byte(v)
		}
		return net.IP(b).String()
	}
	return ""
}

// findProcessByInode 通过 inode 查找进程
func (c *NetworkCollector) findProcessByInode(inode int64) (int, string) {
	// 遍历 /proc/[pid]/fd 查找对应的 socket
	procDir, err := os.Open("/proc")
	if err != nil {
		return 0, ""
	}
	defer procDir.Close()

	entries, err := procDir.Readdirnames(0)
	if err != nil {
		return 0, ""
	}

	for _, entry := range entries {
		pid, err := strconv.Atoi(entry)
		if err != nil {
			continue
		}

		fdDir := fmt.Sprintf("/proc/%d/fd", pid)
		fdEntries, err := os.ReadDir(fdDir)
		if err != nil {
			continue
		}

		for _, fdEntry := range fdEntries {
			link, err := os.Readlink(filepath.Join(fdDir, fdEntry.Name()))
			if err != nil {
				continue
			}

			// 检查是否是 socket
			if strings.HasPrefix(link, "socket:[") {
				socketInode, _ := strconv.ParseInt(link[8:len(link)-1], 10, 64)
				if socketInode == inode {
					// 读取进程名
					comm, _ := os.ReadFile(fmt.Sprintf("/proc/%d/comm", pid))
					return pid, strings.TrimSpace(string(comm))
				}
			}
		}
	}

	return 0, ""
}

// listenNetlink 监听 netlink 事件
func (c *NetworkCollector) listenNetlink() {
	// 创建 netlink socket
	sock, err := nl.GetNetlinkSocket(nl.NETLINK_INET_DIAG)
	if err != nil {
		select {
		case c.errorChan <- fmt.Errorf("failed to create netlink socket: %w", err):
		default:
		}
		return
	}
	defer sock.Close()

	// 订阅事件
	groups := uint32(0)
	for _, g := range c.config.NetlinkGroups {
		groups |= g
	}

	if groups != 0 {
		if err := sock.SetReadTimeout(1 * time.Second); err != nil {
			select {
			case c.errorChan <- err:
			default:
			}
		}
	}

	// 读取消息
	buf := make([]byte, os.Getpagesize())
	for {
		select {
		case <-c.ctx.Done():
			return
		default:
			n, _, err := sock.ReceiveFrom(buf)
			if err != nil {
				if errors.Is(err, netlink.ErrInterrupt) {
					continue
				}
				select {
				case c.errorChan <- err:
				default:
				}
				continue
			}

			// 处理消息
			c.processNetlinkMessage(buf[:n])
		}
	}
}

// processNetlinkMessage 处理 netlink 消息
func (c *NetworkCollector) processNetlinkMessage(data []byte) {
	// 解析 netlink 消息头
	if len(data) < nl.SizeofNlMsghdr {
		return
	}

	h := nl.DeserializeNlMsghdr(data)

	// 处理不同类型的消息
	switch h.Type {
	case nl.NLMSG_DONE:
		return
	case nl.NLMSG_ERROR:
		return
	default:
		// 处理 inet_diag 消息
		c.processInetDiagMsg(data[nl.SizeofNlMsghdr:])
	}
}

// processInetDiagMsg 处理 inet_diag 消息
func (c *NetworkCollector) processInetDiagMsg(data []byte) {
	// inet_diag_msg 结构
	// 参考: https://man7.org/linux/man-pages/man7/sock_diag.7.html
	if len(data) < 72 {
		return
	}

	// 解析消息
	// family: 1 byte
	// state: 1 byte
	// timer: 1 byte
	// retrans: 1 byte
	// id: 16 bytes (src_port, dst_port, src_addr, dst_addr)
	// expires: 4 bytes
	// rqueue: 4 bytes
	// wqueue: 4 bytes
	// uid: 4 bytes
	// inode: 4 bytes

	family := data[0]
	state := data[1]

	// 解析端口
	srcPort := binary.BigEndian.Uint16(data[4:6])
	dstPort := binary.BigEndian.Uint16(data[6:8])

	var srcIP, dstIP string
	var protocol string

	switch family {
	case 2: // AF_INET (IPv4)
		srcIP = net.IP(data[8:12]).String()
		dstIP = net.IP(data[12:16]).String()
		protocol = "TCP"
	case 10: // AF_INET6 (IPv6)
		srcIP = net.IP(data[8:24]).String()
		dstIP = net.IP(data[24:40]).String()
		protocol = "TCP"
	}

	// 解析 inode
	inode := binary.LittleEndian.Uint32(data[64:68])
	pid, comm := c.findProcessByInode(int64(inode))

	flow := &models.NetworkFlow{
		Timestamp:     time.Now(),
		Protocol:      protocol,
		SourceIP:      srcIP,
		SourcePort:    srcPort,
		DestIP:        dstIP,
		DestPort:      dstPort,
		ProcessName:   comm,
		ProcessPID:    uint32(pid),
		TCPState:      tcpState(int(state)),
		CollectorType: models.CollectorTraditional,
	}

	select {
	case c.flowChan <- flow:
		c.mu.Lock()
		c.eventsCount++
		c.mu.Unlock()
	default:
		c.mu.Lock()
		c.dropCount++
		c.mu.Unlock()
	}
}

// tcpState TCP 状态转换
func tcpState(state int) string {
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
	return "UNKNOWN"
}
