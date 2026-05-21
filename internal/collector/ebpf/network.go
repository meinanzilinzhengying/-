//go:build linux

// Package ebpf 提供 eBPF 网络流量采集功能
package ebpf

import (
	"context"
	"encoding/binary"
	"errors"
	"fmt"
	"net"
	"sync"
	"time"
	"unsafe"

	"github.com/cilium/ebpf"
	"github.com/cilium/ebpf/link"
	"github.com/cilium/ebpf/perf"

	"github.com/meinanzilinzhengying/cloud-flow-agent/pkg/models"
)

// NetworkCollector eBPF 网络采集器
type NetworkCollector struct {
	config    *models.EBPFCollectorConfig
	status    models.CollectorStatus
	mu        sync.RWMutex
	ctx       context.Context
	cancel    context.CancelFunc

	// eBPF 相关
	collection *ebpf.Collection
	links      []link.Link
	reader     *perf.Reader

	// 数据通道
	flowChan   chan *models.NetworkFlow
	errorChan  chan error

	// 统计
	eventsCount uint64
	dropCount   uint64
}

// NetEvent C 结构对应的 Go 结构
type NetEvent struct {
	Timestamp  uint64
	EventType  uint32
	PID        uint32
	TID        uint32
	Comm       [16]byte
	Saddr      [4]byte
	Daddr      [4]byte
	Sport      uint16
	Dport      uint16
	Protocol   uint8
	TCPState   uint8
	Bytes      uint64
	Packets    uint64
	DurationNs uint64
}

// NewNetworkCollector 创建 eBPF 网络采集器
func NewNetworkCollector() *NetworkCollector {
	return &NetworkCollector{
		flowChan:  make(chan *models.NetworkFlow, 10000),
		errorChan: make(chan error, 100),
	}
}

// Name 返回采集器名称
func (c *NetworkCollector) Name() string {
	return "ebpf-network"
}

// Type 返回采集器类型
func (c *NetworkCollector) Type() models.CollectorType {
	return models.CollectorEBPF
}

// Init 初始化采集器
func (c *NetworkCollector) Init(ctx context.Context, config interface{}) error {
	cfg, ok := config.(*models.EBPFCollectorConfig)
	if !ok {
		return errors.New("invalid config type")
	}
	c.config = cfg

	c.ctx, c.cancel = context.WithCancel(ctx)

	// 初始化状态
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

	// 加载 eBPF 程序
	if err := c.loadBPFProgram(); err != nil {
		c.status.LastError = err.Error()
		return fmt.Errorf("failed to load BPF program: %w", err)
	}

	// 附加 kprobes
	if err := c.attachKprobes(); err != nil {
		c.status.LastError = err.Error()
		return fmt.Errorf("failed to attach kprobes: %w", err)
	}

	// 初始化 perf reader
	if err := c.initPerfReader(); err != nil {
		c.status.LastError = err.Error()
		return fmt.Errorf("failed to init perf reader: %w", err)
	}

	// 启动事件处理协程
	go c.processEvents()

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

	// 关闭 perf reader
	if c.reader != nil {
		c.reader.Close()
	}

	// 分离所有链接
	for _, l := range c.links {
		if l != nil {
			l.Close()
		}
	}
	c.links = nil

	// 关闭 collection
	if c.collection != nil {
		c.collection.Close()
	}

	// 关闭通道
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

// Events 返回事件通道 (实现 Collector 接口)
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

// loadBPFProgram 加载 eBPF 程序
func (c *NetworkCollector) loadBPFProgram() error {
	// 在实际实现中，这里应该加载预编译的 eBPF 字节码
	// 使用 go:generate 和 bpf2go 工具生成

	// 这里提供一个框架实现
	// 实际部署时需要:
	// 1. 使用 //go:generate go run github.com/cilium/ebpf/cmd/bpf2go 生成
	// 2. 或者加载预编译的 .o 文件

	spec, err := ebpf.LoadCollectionSpec("bpf/network.o")
	if err != nil {
		// 如果文件不存在，返回一个提示
		return fmt.Errorf("BPF object file not found, please compile with 'make bpf': %w", err)
	}

	collection, err := ebpf.NewCollection(spec)
	if err != nil {
		return fmt.Errorf("failed to create collection: %w", err)
	}

	c.collection = collection
	return nil
}

// attachKprobes 附加 kprobe
func (c *NetworkCollector) attachKprobes() error {
	// 附加 tcp_connect
	if prog, ok := c.collection.Programs["kprobe__tcp_connect"]; ok {
		l, err := link.Kprobe("tcp_connect", prog, nil)
		if err != nil {
			return fmt.Errorf("failed to attach tcp_connect: %w", err)
		}
		c.links = append(c.links, l)
	}

	// 附加 inet_csk_accept (kretprobe)
	if prog, ok := c.collection.Programs["kretprobe__inet_csk_accept"]; ok {
		l, err := link.Kretprobe("inet_csk_accept", prog, nil)
		if err != nil {
			return fmt.Errorf("failed to attach inet_csk_accept: %w", err)
		}
		c.links = append(c.links, l)
	}

	// 附加 tcp_close
	if prog, ok := c.collection.Programs["kprobe__tcp_close"]; ok {
		l, err := link.Kprobe("tcp_close", prog, nil)
		if err != nil {
			return fmt.Errorf("failed to attach tcp_close: %w", err)
		}
		c.links = append(c.links, l)
	}

	// 附加 tcp_sendmsg
	if prog, ok := c.collection.Programs["kprobe__tcp_sendmsg"]; ok {
		l, err := link.Kprobe("tcp_sendmsg", prog, nil)
		if err != nil {
			return fmt.Errorf("failed to attach tcp_sendmsg: %w", err)
		}
		c.links = append(c.links, l)
	}

	// 附加 tcp_recvmsg
	if prog, ok := c.collection.Programs["kprobe__tcp_recvmsg"]; ok {
		l, err := link.Kprobe("tcp_recvmsg", prog, nil)
		if err != nil {
			return fmt.Errorf("failed to attach tcp_recvmsg: %w", err)
		}
		c.links = append(c.links, l)
	}

	return nil
}

// initPerfReader 初始化 perf reader
func (c *NetworkCollector) initPerfReader() error {
	eventsMap, ok := c.collection.Maps["events"]
	if !ok {
		return errors.New("events map not found")
	}

	reader, err := perf.NewReader(eventsMap, c.config.BufferSize)
	if err != nil {
		return fmt.Errorf("failed to create perf reader: %w", err)
	}

	c.reader = reader
	return nil
}

// processEvents 处理事件
func (c *NetworkCollector) processEvents() {
	for {
		select {
		case <-c.ctx.Done():
			return
		default:
			record, err := c.reader.Read()
			if err != nil {
				if errors.Is(err, perf.ErrClosed) {
					return
				}
				select {
				case c.errorChan <- err:
				default:
				}
				continue
			}

			// 解析事件
			if len(record.RawSample) > 0 {
				event := c.parseEvent(record.RawSample)
				if event != nil {
					select {
					case c.flowChan <- event:
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
	}
}

// parseEvent 解析事件
func (c *NetworkCollector) parseEvent(data []byte) *models.NetworkFlow {
	if len(data) < int(unsafe.Sizeof(NetEvent{})) {
		return nil
	}

	// 将原始数据转换为 NetEvent 结构
	var event NetEvent
	if err := binary.Read(
		unsafeReader{data: data[:unsafe.Sizeof(NetEvent{})]},
		binary.LittleEndian,
		&event,
	); err != nil {
		return nil
	}

	// 转换为 NetworkFlow
	flow := &models.NetworkFlow{
		Timestamp:     time.Unix(0, int64(event.Timestamp)),
		SourceIP:      ipToString(event.Saddr),
		SourcePort:    event.Sport,
		DestIP:        ipToString(event.Daddr),
		DestPort:      event.Dport,
		ProcessName:   string(bytesTrimNull(event.Comm[:])),
		ProcessPID:    event.PID,
		BytesSent:     event.Bytes,
		PacketsSent:   event.Packets,
		Duration:      event.DurationNs,
		CollectorType: models.CollectorEBPF,
	}

	// 设置协议
	switch event.Protocol {
	case 6:
		flow.Protocol = "TCP"
	case 17:
		flow.Protocol = "UDP"
	default:
		flow.Protocol = "UNKNOWN"
	}

	// 设置 TCP 状态
	flow.TCPState = tcpStateToString(event.TCPState)

	return flow
}

// unsafeReader 用于从字节切片读取
type unsafeReader struct {
	data []byte
	pos  int
}

func (r unsafeReader) Read(p []byte) (n int, err error) {
	if r.pos >= len(r.data) {
		return 0, errors.New("EOF")
	}
	n = copy(p, r.data[r.pos:])
	r.pos += n
	return n, nil
}

// ipToString 将 IP 字节数组转换为字符串
func ipToString(ip [4]byte) string {
	return net.IP(ip[:]).String()
}

// bytesTrimNull 去除字符串中的空字节
func bytesTrimNull(b []byte) string {
	for i, c := range b {
		if c == 0 {
			return string(b[:i])
		}
	}
	return string(b)
}

// tcpStateToString TCP 状态转换
func tcpStateToString(state uint8) string {
	states := map[uint8]string{
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
	}

	if s, ok := states[state]; ok {
		return s
	}
	return "UNKNOWN"
}
