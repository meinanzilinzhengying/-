//go:build linux

// Package process 提供进程事件采集功能
package process

import (
	"bufio"
	"context"
	"encoding/binary"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/vishvananda/netlink"

	"github.com/meinanzilinzhengying/cloud-flow-agent/pkg/models"
)

// Collector 进程事件采集器
type Collector struct {
	config    *models.ProcessCollectorConfig
	status    models.CollectorStatus
	mu        sync.RWMutex
	ctx       context.Context
	cancel    context.CancelFunc

	// 数据通道
	eventChan  chan *models.ProcessEvent
	errorChan  chan error

	// 进程缓存
	processCache map[uint32]*processInfo

	// 统计
	eventsCount uint64
	dropCount   uint64
}

// processInfo 进程信息缓存
type processInfo struct {
	PID     uint32
	PPID    uint32
	Comm    string
	Exe     string
	Cmdline string
	CWD     string
	UID     uint32
	GID     uint32
	Seen    time.Time
}

// NewCollector 创建进程事件采集器
func NewCollector() *Collector {
	return &Collector{
		eventChan:    make(chan *models.ProcessEvent, 10000),
		errorChan:    make(chan error, 100),
		processCache: make(map[uint32]*processInfo),
	}
}

// Name 返回采集器名称
func (c *Collector) Name() string {
	return "process-events"
}

// Type 返回采集器类型
func (c *Collector) Type() models.CollectorType {
	return models.CollectorProcess
}

// Init 初始化采集器
func (c *Collector) Init(ctx context.Context, config interface{}) error {
	cfg, ok := config.(*models.ProcessCollectorConfig)
	if !ok {
		return errors.New("invalid config type")
	}
	c.config = cfg

	c.ctx, c.cancel = context.WithCancel(ctx)

	// 初始化进程缓存
	c.initProcessCache()

	c.status = models.CollectorStatus{
		Name:      c.Name(),
		Type:      c.Type(),
		Enabled:   cfg.Enabled,
		StartTime: time.Now(),
	}

	return nil
}

// Start 启动采集器
func (c *Collector) Start(ctx context.Context) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if !c.config.Enabled {
		return nil
	}

	// 启动 netlink 监听协程
	go c.listenNetlink()

	// 启动定期扫描协程
	go c.scanLoop()

	c.status.Running = true
	c.status.StartTime = time.Now()

	return nil
}

// Stop 停止采集器
func (c *Collector) Stop(ctx context.Context) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.cancel != nil {
		c.cancel()
	}

	close(c.eventChan)
	close(c.errorChan)

	c.status.Running = false

	return nil
}

// Status 返回采集器状态
func (c *Collector) Status() models.CollectorStatus {
	c.mu.RLock()
	defer c.mu.RUnlock()

	status := c.status
	status.EventsCount = c.eventsCount
	status.DropCount = c.dropCount

	return status
}

// ProcessEvents 返回进程事件通道
func (c *Collector) ProcessEvents() <-chan *models.ProcessEvent {
	return c.eventChan
}

// Events 返回事件通道
func (c *Collector) Events() <-chan interface{} {
	ch := make(chan interface{})
	go func() {
		for event := range c.eventChan {
			ch <- event
		}
		close(ch)
	}()
	return ch
}

// Errors 返回错误通道
func (c *Collector) Errors() <-chan error {
	return c.errorChan
}

// initProcessCache 初始化进程缓存
func (c *Collector) initProcessCache() {
	procDir, err := os.Open("/proc")
	if err != nil {
		return
	}
	defer procDir.Close()

	entries, err := procDir.Readdirnames(0)
	if err != nil {
		return
	}

	for _, entry := range entries {
		pid, err := strconv.ParseUint(entry, 10, 32)
		if err != nil {
			continue
		}

		info := c.readProcessInfo(uint32(pid))
		if info != nil {
			c.processCache[uint32(pid)] = info
		}
	}
}

// readProcessInfo 读取进程信息
func (c *Collector) readProcessInfo(pid uint32) *processInfo {
	procPath := fmt.Sprintf("/proc/%d", pid)

	// 读取状态
	stat, err := os.ReadFile(filepath.Join(procPath, "stat"))
	if err != nil {
		return nil
	}

	// 解析 stat 文件
	fields := strings.Fields(string(stat))
	if len(fields) < 4 {
		return nil
	}

	ppid, _ := strconv.ParseUint(fields[3], 10, 32)

	// 读取 comm
	comm, _ := os.ReadFile(filepath.Join(procPath, "comm"))
	commStr := strings.TrimSpace(string(comm))

	// 读取 exe
	exe, _ := os.Readlink(filepath.Join(procPath, "exe"))
	exeStr := exe
	if strings.Contains(exeStr, " (deleted)") {
		exeStr = strings.TrimSuffix(exeStr, " (deleted)")
	}

	// 读取 cmdline
	cmdline, _ := os.ReadFile(filepath.Join(procPath, "cmdline"))
	cmdlineStr := strings.ReplaceAll(string(cmdline), "\x00", " ")

	// 读取 cwd
	cwd, _ := os.Readlink(filepath.Join(procPath, "cwd"))

	// 读取状态文件获取 UID/GID
	status, _ := os.ReadFile(filepath.Join(procPath, "status"))
	var uid, gid uint32
	lines := strings.Split(string(status), "\n")
	for _, line := range lines {
		if strings.HasPrefix(line, "Uid:") {
			fields := strings.Fields(line)
			if len(fields) >= 2 {
				u, _ := strconv.ParseUint(fields[1], 10, 32)
				uid = uint32(u)
			}
		}
		if strings.HasPrefix(line, "Gid:") {
			fields := strings.Fields(line)
			if len(fields) >= 2 {
				g, _ := strconv.ParseUint(fields[1], 10, 32)
				gid = uint32(g)
			}
		}
	}

	return &processInfo{
		PID:     pid,
		PPID:    uint32(ppid),
		Comm:    commStr,
		Exe:     exeStr,
		Cmdline: cmdlineStr,
		CWD:     cwd,
		UID:     uid,
		GID:     gid,
		Seen:    time.Now(),
	}
}

// listenNetlink 监听 netlink 事件
func (c *Collector) listenNetlink() {
	// 创建 netlink socket
	sock, err := netlink.GetNetlinkSocket(netlink.NETLINK_CONNECTOR)
	if err != nil {
		// 如果 connector 不可用，使用 proc 扫描
		return
	}
	defer sock.Close()

	// 订阅进程事件
	// CN_IDX_PROC = 0x1
	// PROC_CN_MCAST_LISTEN = 0x1
	msg := c.createSubscribeMsg()
	if err := sock.Send(msg); err != nil {
		select {
		case c.errorChan <- fmt.Errorf("failed to subscribe to process events: %w", err):
		default:
		}
		return
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
				continue
			}

			c.processNetlinkMessage(buf[:n])
		}
	}
}

// createSubscribeMsg 创建订阅消息
func (c *Collector) createSubscribeMsg() []byte {
	// 构造订阅消息
	// 参考: linux/connector.h, linux/cn_proc.h

	msg := make([]byte, 16)

	// nlmsghdr
	binary.LittleEndian.PutUint32(msg[0:4], 16)  // nlmsg_len
	binary.LittleEndian.PutUint16(msg[4:6], 1)   // nlmsg_type (NLMSG_DONE)
	binary.LittleEndian.PutUint16(msg[6:8], 0)   // nlmsg_flags
	binary.LittleEndian.PutUint32(msg[8:12], 0)  // nlmsg_seq
	binary.LittleEndian.PutUint32(msg[12:16], 0) // nlmsg_pid

	return msg
}

// processNetlinkMessage 处理 netlink 消息
func (c *Collector) processNetlinkMessage(data []byte) {
	// 解析 connector 消息
	// 参考: linux/cn_proc.h

	if len(data) < 32 {
		return
	}

	// 跳过 nlmsghdr (16 bytes)
	// 跳过 cn_msg (16 bytes)
	// 获取 proc_event

	eventType := binary.LittleEndian.Uint32(data[32:36])

	var event *models.ProcessEvent

	switch eventType {
	case 0x00000001: // PROC_EVENT_FORK
		event = c.parseForkEvent(data[36:])
	case 0x00000002: // PROC_EVENT_EXEC
		event = c.parseExecEvent(data[36:])
	case 0x00000003: // PROC_EVENT_EXIT
		event = c.parseExitEvent(data[36:])
	}

	if event != nil {
		c.sendEvent(event)
	}
}

// parseForkEvent 解析 fork 事件
func (c *Collector) parseForkEvent(data []byte) *models.ProcessEvent {
	if len(data) < 16 {
		return nil
	}

	parentPid := binary.LittleEndian.Uint32(data[0:4])
	parentTid := binary.LittleEndian.Uint32(data[4:8])
	childPid := binary.LittleEndian.Uint32(data[8:12])
	childTid := binary.LittleEndian.Uint32(data[12:16])

	// 获取父进程信息
	parentInfo := c.processCache[parentPid]

	event := &models.ProcessEvent{
		Timestamp: time.Now(),
		EventType: "fork",
		PID:       childPid,
		PPID:      parentPid,
		TID:       childTid,
	}

	if parentInfo != nil {
		event.Comm = parentInfo.Comm
	}

	// 更新缓存
	c.processCache[childPid] = &processInfo{
		PID:  childPid,
		PPID: parentPid,
		Seen: time.Now(),
	}

	return event
}

// parseExecEvent 解析 exec 事件
func (c *Collector) parseExecEvent(data []byte) *models.ProcessEvent {
	if len(data) < 8 {
		return nil
	}

	pid := binary.LittleEndian.Uint32(data[0:4])
	tid := binary.LittleEndian.Uint32(data[4:8])

	// 读取新的进程信息
	info := c.readProcessInfo(pid)
	if info == nil {
		return nil
	}

	// 更新缓存
	c.processCache[pid] = info

	return &models.ProcessEvent{
		Timestamp: time.Now(),
		EventType: "exec",
		PID:       pid,
		PPID:      info.PPID,
		TID:       tid,
		Comm:      info.Comm,
		Exe:       info.Exe,
		Cmdline:   info.Cmdline,
		CWD:       info.CWD,
		UID:       info.UID,
		GID:       info.GID,
	}
}

// parseExitEvent 解析 exit 事件
func (c *Collector) parseExitEvent(data []byte) *models.ProcessEvent {
	if len(data) < 16 {
		return nil
	}

	pid := binary.LittleEndian.Uint32(data[0:4])
	tid := binary.LittleEndian.Uint32(data[4:8])
	exitCode := binary.LittleEndian.Uint32(data[8:12])
	signal := binary.LittleEndian.Uint32(data[12:16])

	// 获取进程信息
	info := c.processCache[pid]

	event := &models.ProcessEvent{
		Timestamp: time.Now(),
		EventType: "exit",
		PID:       pid,
		TID:       tid,
		ExitCode:  int32(exitCode),
		Signal:    signal,
	}

	if info != nil {
		event.Comm = info.Comm
		event.Exe = info.Exe
	}

	// 从缓存中删除
	delete(c.processCache, pid)

	return event
}

// scanLoop 定期扫描进程
func (c *Collector) scanLoop() {
	ticker := time.NewTicker(10 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-c.ctx.Done():
			return
		case <-ticker.C:
			c.scanProcesses()
		}
	}
}

// scanProcesses 扫描进程
func (c *Collector) scanProcesses() {
	procDir, err := os.Open("/proc")
	if err != nil {
		return
	}
	defer procDir.Close()

	entries, err := procDir.Readdirnames(0)
	if err != nil {
		return
	}

	currentPids := make(map[uint32]bool)

	for _, entry := range entries {
		pid, err := strconv.ParseUint(entry, 10, 32)
		if err != nil {
			continue
		}

		currentPids[uint32(pid)] = true

		// 检查是否是新进程
		if _, exists := c.processCache[uint32(pid)]; !exists {
			// 新进程，读取信息并发送事件
			info := c.readProcessInfo(uint32(pid))
			if info != nil {
				c.processCache[uint32(pid)] = info

				// 检查过滤器
				if c.filterProcess(info) {
					event := &models.ProcessEvent{
						Timestamp: time.Now(),
						EventType: "exec",
						PID:       info.PID,
						PPID:      info.PPID,
						Comm:      info.Comm,
						Exe:       info.Exe,
						Cmdline:   info.Cmdline,
						CWD:       info.CWD,
						UID:       info.UID,
						GID:       info.GID,
					}
					c.sendEvent(event)
				}
			}
		}
	}

	// 检查已退出的进程
	for pid := range c.processCache {
		if !currentPids[pid] {
			info := c.processCache[pid]
			delete(c.processCache, pid)

			if info != nil && c.filterProcess(info) {
				event := &models.ProcessEvent{
					Timestamp: time.Now(),
					EventType: "exit",
					PID:       pid,
					Comm:      info.Comm,
					Exe:       info.Exe,
				}
				c.sendEvent(event)
			}
		}
	}
}

// filterProcess 过滤进程
func (c *Collector) filterProcess(info *processInfo) bool {
	// 用户过滤
	if len(c.config.FilterUsers) > 0 {
		found := false
		for _, uid := range c.config.FilterUsers {
			if info.UID == uid {
				found = true
				break
			}
		}
		if !found {
			return false
		}
	}

	// 进程名过滤
	if len(c.config.FilterComms) > 0 {
		found := false
		for _, comm := range c.config.FilterComms {
			if info.Comm == comm {
				found = true
				break
			}
		}
		if !found {
			return false
		}
	}

	return true
}

// sendEvent 发送事件
func (c *Collector) sendEvent(event *models.ProcessEvent) {
	select {
	case c.eventChan <- event:
		c.mu.Lock()
		c.eventsCount++
		c.mu.Unlock()
	default:
		c.mu.Lock()
		c.dropCount++
		c.mu.Unlock()
	}
}
