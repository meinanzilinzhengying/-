//go:build linux

// Package ebpfcollector 提供基于 eBPF 的网络流量采集功能
package ebpfcollector

import (
	"encoding/binary"
	"fmt"
	"log"
	"net"
	"os"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	"github.com/cilium/ebpf"
	"github.com/cilium/ebpf/link"
	"github.com/cilium/ebpf/rlimit"

	"cloud-flow-agent/internal/ebpfcollector/bpf"
	"cloud-flow-agent/internal/ebpfcollector/parser"
	edge "cloud-flow/proto"
)

// 注意：eBPF 程序需要先编译，使用 `make ebpf` 命令

// Collector eBPF 采集器
type Collector struct {
	objs            *bpf.Objects
	tcpMetricsObjs  *bpf.TCPMetricsObjects
	httpMetricsObjs *bpf.HTTPMetricsObjects
	links           []link.Link
	tcpLinks        []link.Link
	httpLinks       []link.Link
	stopCh          chan struct{}
	collectCh       chan []*edge.MetricData
	enableTCPMetrics  bool
	enableHTTPMetrics bool
	mgmtIface       string // 管理网卡接口
}

// CollectorOptions 采集器配置选项
type CollectorOptions struct {
	EnableTCPMetrics  bool   // 启用TCP深度指标采集
	EnableHTTPMetrics bool  // 启用HTTP请求指标采集
	MgmtIface        string // 管理网卡接口名称
}

// New 创建 eBPF 采集器
func New() (*Collector, error) {
	return NewWithOptions(&CollectorOptions{
		EnableTCPMetrics:  true,
		EnableHTTPMetrics: true,
	})
}

// NewWithOptions 使用选项创建 eBPF 采集器
func NewWithOptions(opts *CollectorOptions) (*Collector, error) {
	if opts == nil {
		opts = &CollectorOptions{
			EnableTCPMetrics:  true,
			EnableHTTPMetrics: true,
		}
	}

	if err := rlimit.RemoveMemlock(); err != nil {
		return nil, fmt.Errorf("移除内存限制失败: %w", err)
	}

	objs, err := loadBPFObjects()
	if err != nil {
		return nil, fmt.Errorf("加载 eBPF 对象失败: %w", err)
	}

	links, err := attachProgram(objs.TCProg, opts.MgmtIface)
	if err != nil {
		objs.Close()
		return nil, fmt.Errorf("附加 eBPF 程序失败: %w", err)
	}

	collector := &Collector{
		objs:             objs,
		links:            links,
		stopCh:           make(chan struct{}),
		collectCh:        make(chan []*edge.MetricData, 10),
		enableTCPMetrics:  opts.EnableTCPMetrics,
		enableHTTPMetrics: opts.EnableHTTPMetrics,
		mgmtIface:        opts.MgmtIface,
	}

	// 加载TCP指标eBPF程序
	if opts.EnableTCPMetrics {
		tcpObjs, tcpLinks, err := loadTCPMetricsObjects()
		if err != nil {
			log.Printf("警告: 加载TCP指标eBPF程序失败: %v，将继续使用基础流量采集", err)
		} else {
			collector.tcpMetricsObjs = tcpObjs
			collector.tcpLinks = tcpLinks
			log.Printf("成功加载TCP指标eBPF程序")
		}
	}

	// 加载HTTP指标eBPF程序
	if opts.EnableHTTPMetrics {
		httpObjs, httpLinks, err := loadHTTPMetricsObjects()
		if err != nil {
			log.Printf("警告: 加载HTTP指标eBPF程序失败: %v，将继续使用基础流量采集", err)
		} else {
			collector.httpMetricsObjs = httpObjs
			collector.httpLinks = httpLinks
			log.Printf("成功加载HTTP指标eBPF程序")
		}
	}

	// 降权为普通用户运行
	if err := dropPrivileges(); err != nil {
		log.Printf("警告: 无法降权: %v，将继续以当前权限运行", err)
	}

	return collector, nil
}

// loadTCPMetricsObjects 加载TCP指标eBPF对象
func loadTCPMetricsObjects() (*bpf.TCPMetricsObjects, []link.Link, error) {
	opts := &ebpf.CollectionOptions{}

	tcpObjs, err := bpf.LoadTCPMetrics(opts)
	if err != nil {
		return nil, nil, fmt.Errorf("加载TCP指标eBPF对象失败: %w", err)
	}

	tcpLinks, err := bpf.AttachTCPMetricsProbes(tcpObjs)
	if err != nil {
		tcpObjs.Close()
		return nil, nil, fmt.Errorf("附加TCP指标kprobe失败: %w", err)
	}

	return tcpObjs, tcpLinks, nil
}

// loadHTTPMetricsObjects 加载HTTP指标eBPF对象
func loadHTTPMetricsObjects() (*bpf.HTTPMetricsObjects, []link.Link, error) {
	opts := &ebpf.CollectionOptions{}

	httpObjs, err := bpf.LoadHTTPMetrics(opts)
	if err != nil {
		return nil, nil, fmt.Errorf("加载HTTP指标eBPF对象失败: %w", err)
	}

	httpLinks, err := bpf.AttachHTTPMetricsProbes(httpObjs)
	if err != nil {
		httpObjs.Close()
		return nil, nil, fmt.Errorf("附加HTTP指标kprobe失败: %w", err)
	}

	return httpObjs, httpLinks, nil
}

// dropPrivileges 降权为普通用户
func dropPrivileges() error {
	// 尝试使用 "cloud-flow" 用户，如果不存在则使用 "nobody"
	targetUsers := []string{"cloud-flow", "nobody"}
	var pwd syscall.Passwd
	bufSize := 4096
	buf := make([]byte, bufSize)
	var targetUser string
	var err error
	var ptr *syscall.Passwd

	// 尝试获取用户信息
	for _, user := range targetUsers {
		ptr, err = syscall.Getpwnam_r(user, &pwd, buf, int32(len(buf)))
		if err == syscall.ERANGE {
			// 缓冲区不够大，增大后重试
			bufSize *= 2
			buf = make([]byte, bufSize)
			ptr, err = syscall.Getpwnam_r(user, &pwd, buf, int32(len(buf)))
		}
		if err == nil && ptr != nil {
			targetUser = user
			break
		}
	}

	if ptr == nil {
		log.Printf("警告: 目标用户 %v 均不存在，eBPF 采集器将以当前用户运行", targetUsers)
		return nil
	}

	// 先设置组 ID
	if err := syscall.Setgid(int(pwd.Gid)); err != nil {
		return fmt.Errorf("设置组 ID 失败: %w", err)
	}

	// 再设置用户 ID
	if err := syscall.Setuid(int(pwd.Uid)); err != nil {
		return fmt.Errorf("设置用户 ID 失败: %w", err)
	}

	// 验证权限是否已降
	if syscall.Getuid() != int(pwd.Uid) || syscall.Getgid() != int(pwd.Gid) {
		return fmt.Errorf("权限降权验证失败")
	}

	// 验证是否能够读取 /proc 目录
	if _, err := os.ReadDir("/proc"); err != nil {
		log.Printf("警告: 降权后无法读取 /proc 目录: %v，某些功能可能受限", err)
	}

	log.Printf("成功降权为用户 %s (UID: %d, GID: %d)", targetUser, pwd.Uid, pwd.Gid)
	return nil
}

// NewWithFallback 创建一个采集器，如果 eBPF 不可用则使用回退方案
func NewWithFallback() (*Collector, error) {
	collector, err := New()
	if err != nil {
		log.Printf("eBPF 采集器初始化失败: %v，将使用传统采集器作为回退", err)
		return nil, err
	}
	return collector, nil
}

// IsAvailable 检查 eBPF 采集器是否可用
func (c *Collector) IsAvailable() bool {
	return c != nil && c.objs != nil && c.objs.NetworkMap != nil
}

// IsTCPMetricsAvailable 检查TCP指标采集是否可用
func (c *Collector) IsTCPMetricsAvailable() bool {
	return c != nil && c.tcpMetricsObjs != nil
}

// IsHTTPMetricsAvailable 检查HTTP指标采集是否可用
func (c *Collector) IsHTTPMetricsAvailable() bool {
	return c != nil && c.httpMetricsObjs != nil
}

// Start 启动采集器
func (c *Collector) Start() {
	go c.collectLoop()
}

// Stop 停止采集器
func (c *Collector) Stop() {
	close(c.stopCh)
	for _, l := range c.links {
		l.Close()
	}
	for _, l := range c.tcpLinks {
		l.Close()
	}
	for _, l := range c.httpLinks {
		l.Close()
	}
	if c.tcpMetricsObjs != nil {
		c.tcpMetricsObjs.Close()
	}
	if c.httpMetricsObjs != nil {
		c.httpMetricsObjs.Close()
	}
	c.objs.Close()
}

// Collect 采集网络流量数据
func (c *Collector) Collect() []*edge.MetricData {
	select {
	case metrics := <-c.collectCh:
		return metrics
	case <-time.After(1 * time.Second):
		return nil
	}
}

// collectLoop 采集循环
func (c *Collector) collectLoop() {
	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			metrics := c.collectData()
			if len(metrics) > 0 {
				select {
				case c.collectCh <- metrics:
				default:
				}
			}
		case <-c.stopCh:
			return
		}
	}
}

// collectData 从 eBPF map 中采集数据
func (c *Collector) collectData() []*edge.MetricData {
	var metrics []*edge.MetricData
	now := time.Now().Unix()

	// 采集基础流量数据
	iter := c.objs.NetworkMap.Iterate()
	var key, value []byte
	for iter.Next(&key, &value) {
		flow := parseNetworkData(key, value)
		if flow == nil {
			continue
		}

		parsedMetric := parser.ParseNetworkData(
			flow.SrcIP,
			flow.DstIP,
			flow.SrcPort,
			flow.DstPort,
			flow.Protocol,
			value,
		)

		parsedMetric.Timestamp = now
		parsedMetric.Bytes = flow.Bytes
		parsedMetric.Packets = flow.Packets

		metrics = append(metrics, parsedMetric)

		c.objs.NetworkMap.Delete(key)
	}

	// 采集TCP深度指标
	if c.tcpMetricsObjs != nil {
		tcpMetrics := c.collectTCPMetrics(now)
		metrics = append(metrics, tcpMetrics...)
	}

	// 采集HTTP请求指标
	if c.httpMetricsObjs != nil {
		httpMetrics := c.collectHTTPMetrics(now)
		metrics = append(metrics, httpMetrics...)
	}

	return metrics
}

// collectTCPMetrics 采集TCP深度指标
func (c *Collector) collectTCPMetrics(now int64) []*edge.MetricData {
	var metrics []*edge.MetricData

	// 1. 采集全局TCP指标
	globalMetrics, err := c.tcpMetricsObjs.GetGlobalTCPMetrics()
	if err == nil && globalMetrics != nil {
		metric := &edge.MetricData{
			Timestamp: now,
			Protocol:  "tcp_summary",
			Tags: map[string]string{
				"metric_type":            "global_tcp_metrics",
				"total_connections":      fmt.Sprintf("%d", globalMetrics.TotalConnections),
				"failed_connections":     fmt.Sprintf("%d", globalMetrics.FailedConnections),
				"total_retrans":          fmt.Sprintf("%d", globalMetrics.TotalRetrans),
				"zero_window_events":     fmt.Sprintf("%d", globalMetrics.ZeroWindowEvents),
				"queue_overflow_events":  fmt.Sprintf("%d", globalMetrics.QueueOverflowEvents),
				"avg_latency_ns":         fmt.Sprintf("%d", globalMetrics.AvgLatencyNs),
				"max_latency_ns":         fmt.Sprintf("%d", globalMetrics.MaxLatencyNs),
				"min_latency_ns":         fmt.Sprintf("%d", globalMetrics.MinLatencyNs),
				"latency_samples":        fmt.Sprintf("%d", globalMetrics.LatencySamples),
			},
		}
		metrics = append(metrics, metric)
	}

	// 2. 采集TCP时延数据
	latencyIter := c.tcpMetricsObjs.IterateTcpLatency()
	var latencyKey bpf.TcpConnKey
	var latencyValue bpf.TcpLatency
	for latencyIter.Next(&latencyKey, &latencyValue) {
		if latencyValue.Complete == 1 {
			metric := &edge.MetricData{
				Timestamp: now,
				SrcIp:     intToIP(latencyKey.Saddr).String(),
				DstIp:     intToIP(latencyKey.Daddr).String(),
				SrcPort:   int32(latencyKey.Sport),
				DstPort:   int32(latencyKey.Dport),
				Protocol:  "tcp",
				Latency:   int64(latencyValue.LatencyNs),
				Tags: map[string]string{
					"metric_type":    "tcp_connect_latency",
					"latency_us":      fmt.Sprintf("%d", latencyValue.LatencyNs/1000),
					"syn_sent_ns":     fmt.Sprintf("%d", latencyValue.SynSentNs),
					"established_ns":   fmt.Sprintf("%d", latencyValue.EstablishedNs),
					"pid":             fmt.Sprintf("%d", latencyKey.Pid),
				},
			}
			metrics = append(metrics, metric)
		}
	}

	// 3. 采集TCP统计指标(重传、零窗口等)
	statsIter := c.tcpMetricsObjs.IterateTcpStats()
	var statsKey bpf.TcpConnKey
	var statsValue bpf.TcpStats
	for statsIter.Next(&statsKey, &statsValue) {
		metric := &edge.MetricData{
			Timestamp: now,
			SrcIp:     intToIP(statsKey.Saddr).String(),
			DstIp:     intToIP(statsKey.Daddr).String(),
			SrcPort:   int32(statsKey.Sport),
			DstPort:   int32(statsKey.Dport),
			Protocol:  "tcp",
			Tags: map[string]string{
				"metric_type":           "tcp_connection_stats",
				"retrans_count":         fmt.Sprintf("%d", statsValue.RetransCount),
				"zero_window_count":     fmt.Sprintf("%d", statsValue.ZeroWindowCount),
				"queue_overflow_count":  fmt.Sprintf("%d", statsValue.QueueOverflowCount),
				"conn_fail_count":       fmt.Sprintf("%d", statsValue.ConnFailCount),
				"bytes_sent":            fmt.Sprintf("%d", statsValue.BytesSent),
				"bytes_recv":            fmt.Sprintf("%d", statsValue.BytesRecv),
				"pid":                   fmt.Sprintf("%d", statsKey.Pid),
			},
		}
		metrics = append(metrics, metric)
	}

	// 4. 采集零窗口事件
	zwIter := c.tcpMetricsObjs.IterateZeroWindow()
	var zwKey bpf.TcpConnKey
	var zwCount uint64
	for zwIter.Next(&zwKey, &zwCount) {
		metric := &edge.MetricData{
			Timestamp: now,
			SrcIp:     intToIP(zwKey.Saddr).String(),
			DstIp:     intToIP(zwKey.Daddr).String(),
			SrcPort:   int32(zwKey.Sport),
			DstPort:   int32(zwKey.Dport),
			Protocol:  "tcp",
			Tags: map[string]string{
				"metric_type": "zero_window_event",
				"event_count":  fmt.Sprintf("%d", zwCount),
				"pid":          fmt.Sprintf("%d", zwKey.Pid),
			},
		}
		metrics = append(metrics, metric)
	}

	// 5. 采集队列溢出事件
	qoIter := c.tcpMetricsObjs.IterateQueueOverflow()
	var qoPort uint16
	var qoCount uint64
	for qoIter.Next(&qoPort, &qoCount) {
		metric := &edge.MetricData{
			Timestamp: now,
			DstPort:   int32(qoPort),
			Protocol:  "tcp",
			Tags: map[string]string{
				"metric_type":    "queue_overflow_event",
				"listen_port":    fmt.Sprintf("%d", qoPort),
				"overflow_count":  fmt.Sprintf("%d", qoCount),
			},
		}
		metrics = append(metrics, metric)
	}

	// 6. 采集连接失败事件
	cfIter := c.tcpMetricsObjs.IterateConnFail()
	var cfKey bpf.TcpConnKey
	var cfCount uint64
	for cfIter.Next(&cfKey, &cfCount) {
		metric := &edge.MetricData{
			Timestamp: now,
			SrcIp:     intToIP(cfKey.Saddr).String(),
			DstIp:     intToIP(cfKey.Daddr).String(),
			SrcPort:   int32(cfKey.Sport),
			DstPort:   int32(cfKey.Dport),
			Protocol:  "tcp",
			Tags: map[string]string{
				"metric_type": "connection_fail_event",
				"fail_count":  fmt.Sprintf("%d", cfCount),
				"pid":         fmt.Sprintf("%d", cfKey.Pid),
			},
		}
		metrics = append(metrics, metric)
	}

	return metrics
}

// collectHTTPMetrics 采集HTTP请求指标
func (c *Collector) collectHTTPMetrics(now int64) []*edge.MetricData {
	var metrics []*edge.MetricData

	// 1. 采集全局HTTP指标(请求成功率、响应时延、异常比例)
	globalMetrics, err := c.httpMetricsObjs.GetGlobalHTTPMetrics()
	if err == nil && globalMetrics != nil {
		// 计算成功率
		successRate := float64(0)
		if globalMetrics.TotalResponses > 0 {
			successRate = float64(globalMetrics.SuccessResponses) / float64(globalMetrics.TotalResponses) * 100
		}

		// 计算异常比例
		errorRate := float64(0)
		if globalMetrics.TotalResponses > 0 {
			errorRate = float64(globalMetrics.ErrorResponses) / float64(globalMetrics.TotalResponses) * 100
		}

		metric := &edge.MetricData{
			Timestamp: now,
			Protocol:  "http",
			Tags: map[string]string{
				"metric_type":            "global_http_metrics",
				"total_requests":          fmt.Sprintf("%d", globalMetrics.TotalRequests),
				"total_responses":        fmt.Sprintf("%d", globalMetrics.TotalResponses),
				"success_count":          fmt.Sprintf("%d", globalMetrics.SuccessResponses),
				"error_count":            fmt.Sprintf("%d", globalMetrics.ErrorResponses),
				"success_rate":           fmt.Sprintf("%.2f", successRate),
				"error_rate":             fmt.Sprintf("%.2f", errorRate),
				"avg_latency_ns":         fmt.Sprintf("%d", globalMetrics.AvgLatencyNs),
				"avg_latency_us":         fmt.Sprintf("%.2f", float64(globalMetrics.AvgLatencyNs)/1000),
				"max_latency_ns":         fmt.Sprintf("%d", globalMetrics.MaxLatencyNs),
				"max_latency_us":         fmt.Sprintf("%.2f", float64(globalMetrics.MaxLatencyNs)/1000),
				"min_latency_ns":         fmt.Sprintf("%d", globalMetrics.MinLatencyNs),
				"min_latency_us":         fmt.Sprintf("%.2f", float64(globalMetrics.MinLatencyNs)/1000),
				"latency_samples":        fmt.Sprintf("%d", globalMetrics.LatencySamples),
			},
		}
		metrics = append(metrics, metric)
	}

	// 2. 采集HTTP统计指标(按连接维度)
	statsIter := c.httpMetricsObjs.IterateHttpStats()
	var statsKey bpf.HttpFlowKey
	var statsValue bpf.HttpStats
	for statsIter.Next(&statsKey, &statsValue) {
		// 计算成功率
		successRate := float64(0)
		if statsValue.ResponseCount > 0 {
			successRate = float64(statsValue.SuccessCount) / float64(statsValue.ResponseCount) * 100
		}

		// 计算异常比例
		errorRate := float64(0)
		if statsValue.ResponseCount > 0 {
			errorRate = float64(statsValue.ErrorCount) / float64(statsValue.ResponseCount) * 100
		}

		metric := &edge.MetricData{
			Timestamp: now,
			SrcIp:     intToIP(statsKey.Saddr).String(),
			DstIp:     intToIP(statsKey.Daddr).String(),
			SrcPort:   int32(statsKey.Sport),
			DstPort:   int32(statsKey.Dport),
			Protocol:  "http",
			Tags: map[string]string{
				"metric_type":   "http_connection_stats",
				"request_count":  fmt.Sprintf("%d", statsValue.RequestCount),
				"response_count": fmt.Sprintf("%d", statsValue.ResponseCount),
				"success_count":  fmt.Sprintf("%d", statsValue.SuccessCount),
				"error_count":    fmt.Sprintf("%d", statsValue.ErrorCount),
				"success_rate":   fmt.Sprintf("%.2f", successRate),
				"error_rate":     fmt.Sprintf("%.2f", errorRate),
				"avg_latency_ns": fmt.Sprintf("%d", statsValue.AvgLatencyNs),
				"avg_latency_us": fmt.Sprintf("%.2f", float64(statsValue.AvgLatencyNs)/1000),
				"max_latency_ns": fmt.Sprintf("%d", statsValue.MaxLatencyNs),
				"max_latency_us": fmt.Sprintf("%.2f", float64(statsValue.MaxLatencyNs)/1000),
				"min_latency_ns": fmt.Sprintf("%d", statsValue.MinLatencyNs),
				"min_latency_us": fmt.Sprintf("%.2f", float64(statsValue.MinLatencyNs)/1000),
				"total_request_bytes":  fmt.Sprintf("%d", statsValue.TotalRequestBytes),
				"total_response_bytes": fmt.Sprintf("%d", statsValue.TotalResponseBytes),
				"pid":           fmt.Sprintf("%d", statsKey.Pid),
			},
		}
		metrics = append(metrics, metric)
	}

	// 3. 采集异常状态码统计
	errorIter := c.httpMetricsObjs.IterateErrorStats()
	var statusCode uint16
	var errorCount uint64
	for errorIter.Next(&statusCode, &errorCount) {
		metric := &edge.MetricData{
			Timestamp: now,
			Protocol:  "http",
			Tags: map[string]string{
				"metric_type": "http_error_stats",
				"status_code": fmt.Sprintf("%d", statusCode),
				"error_count": fmt.Sprintf("%d", errorCount),
			},
		}
		metrics = append(metrics, metric)
	}

	return metrics
}

// intToIP 将uint32转换为net.IP
func intToIP(ipInt uint32) net.IP {
	ip := make(net.IP, 4)
	binary.BigEndian.PutUint32(ip, ipInt)
	return ip
}

// NetworkFlow 网络流量数据
type NetworkFlow struct {
	SrcIP     string
	DstIP     string
	SrcPort   uint16
	DstPort   uint16
	Protocol  string
	Bytes     int64
	Packets   int64
	Timestamp int64
}

// BPF 数据长度常量
const (
	bpfKeySize   = 12
	bpfValueSize = 31
)

// parseNetworkData 解析网络流量数据
func parseNetworkData(key, value []byte) *NetworkFlow {
	// BPF 实际数据结构:
	// key: 12 字节 (4-byte src_ip + 4-byte dst_ip + 2-byte src_port + 2-byte dst_port)
	// value: 31 字节 (4-byte dst_ip + 2-byte dst_port + 1-byte protocol + 8-byte bytes + 8-byte packets + 8-byte timestamp)

	var srcIP net.IP
	var srcPort uint16
	var dstIP net.IP
	var dstPort uint16
	var protocol string
	var bytes, packets, timestamp int64

	// 检查 key 长度（使用常量，便于版本兼容）
	if len(key) != bpfKeySize {
		log.Printf("警告: 无效的 key 长度: %d (期望 %d)，可能是 BPF 程序版本不匹配", len(key), bpfKeySize)
		return nil
	}

	// 解析源 IP 和端口
	// key 格式: src_ip(4) + dst_ip(4) + src_port(2) + dst_port(2)
	srcIP = net.IP(key[:4])
	srcPort = binary.BigEndian.Uint16(key[8:10])

	// 检查 value 长度（使用常量，便于版本兼容）
	if len(value) != bpfValueSize {
		log.Printf("警告: 无效的 value 长度: %d (期望 %d)，可能是 BPF 程序版本不匹配", len(value), bpfValueSize)
		return nil
	}

	// 解析目标 IP、端口、协议和统计数据
	dstIP = net.IP(value[:4])
	dstPort = binary.BigEndian.Uint16(value[4:6])

	// 解析协议
	protocol = "unknown"
	switch value[6] {
	case 6:
		protocol = "tcp"
	case 17:
		protocol = "udp"
	case 1:
		protocol = "icmp"
	}

	// 解析统计数据
	bytes = int64(binary.BigEndian.Uint64(value[7:15]))
	packets = int64(binary.BigEndian.Uint64(value[15:23]))
	timestamp = int64(binary.BigEndian.Uint64(value[23:31]))

	return &NetworkFlow{
		SrcIP:     srcIP.String(),
		DstIP:     dstIP.String(),
		SrcPort:   srcPort,
		DstPort:   dstPort,
		Protocol:  protocol,
		Bytes:     bytes,
		Packets:   packets,
		Timestamp: timestamp,
	}
}

// loadBPFObjects 加载 eBPF 对象
func loadBPFObjects() (*bpf.Objects, error) {
	// 1. 首先尝试从文件系统加载（优先使用本地编译的版本）
	ebpfFile := findEBPFFile()
	if ebpfFile != "" {
		spec, err := ebpf.LoadCollectionSpec(ebpfFile)
		if err != nil {
			log.Printf("从文件 %s 加载 eBPF spec 失败: %v，尝试使用嵌入版本", ebpfFile, err)
		} else {
			objs := &bpf.Objects{}
			if err := spec.LoadAndAssign(objs, nil); err != nil {
				log.Printf("从文件 %s 加载 eBPF 对象失败: %v，尝试使用嵌入版本", ebpfFile, err)
			} else {
				log.Printf("成功从文件 %s 加载 eBPF 对象", ebpfFile)
				return objs, nil
			}
		}
	}

	// 2. 检查嵌入的 BPF 程序是否可用
	if err := bpf.CheckRequiredGoFiles(); err != nil {
		return nil, fmt.Errorf("eBPF 程序未嵌入，请运行 'make ebpf' 命令编译 tc.bpf.c: %w", err)
	}

	// 3. 尝试加载嵌入的 BPF 对象
	objs := &bpf.Objects{}
	if err := objs.Load(nil); err != nil {
		// 检查是否是内核兼容性问题
		if strings.Contains(err.Error(), "invalid bpf program") ||
			strings.Contains(err.Error(), "kernel version") ||
			strings.Contains(err.Error(), "BTF") {
			return nil, fmt.Errorf("eBPF 程序与当前内核不兼容: %w\n"+
				"请在目标系统上运行 'make ebpf' 重新编译 BPF 程序，确保与当前内核版本匹配", err)
		}
		return nil, fmt.Errorf("加载 eBPF 对象失败: %w", err)
	}
	log.Printf("成功加载嵌入的 eBPF 对象")
	return objs, nil
}

// findEBPFFile 查找 eBPF 目标文件
func findEBPFFile() string {
	searchPaths := []string{
		"bpf/tc.bpf.o",
		"internal/ebpfcollector/bpf/tc.bpf.o",
		"/etc/cloud-flow-agent/bpf/tc.bpf.o",
		"/usr/share/cloud-flow-agent/bpf/tc.bpf.o",
	}

	for _, path := range searchPaths {
		if _, err := os.Stat(path); err == nil {
			return path
		}
	}

	execPath, err := os.Executable()
	if err == nil {
		dir := filepath.Dir(execPath)
		ebpfFile := filepath.Join(dir, "bpf", "tc.bpf.o")
		if _, err := os.Stat(ebpfFile); err == nil {
			return ebpfFile
		}
	}

	return ""
}

// attachProgram 附加 eBPF 程序到网络设备
func attachProgram(prog *ebpf.Program, mgmtIface string) ([]link.Link, error) {
	devices, err := net.Interfaces()
	if err != nil {
		return nil, fmt.Errorf("获取网络设备失败: %w", err)
	}

	var links []link.Link
	for _, dev := range devices {
		// 跳过回环接口
		if dev.Name == "lo" {
			continue
		}

		// 如果指定了管理网卡,只附加到管理网卡
		if mgmtIface != "" && dev.Name != mgmtIface {
			log.Printf("跳过非管理网卡 %s", dev.Name)
			continue
		}

		// 检查设备是否有有效的索引
		if dev.Index <= 0 {
			log.Printf("设备 %s 索引无效: %d", dev.Name, dev.Index)
			continue
		}

		// 尝试附加到设备的入站方向
		l, err := link.AttachTC(link.TCOptions{
			Interface: dev.Index,
			Direction: link.TCIngress,
			Program:   prog,
		})
		if err != nil {
			log.Printf("附加到设备 %s 入站方向失败: %v", dev.Name, err)
			continue
		}
		links = append(links, l)

		// 尝试附加到设备的出站方向
		l, err = link.AttachTC(link.TCOptions{
			Interface: dev.Index,
			Direction: link.TCEgress,
			Program:   prog,
		})
		if err != nil {
			log.Printf("附加到设备 %s 出站方向失败: %v", dev.Name, err)
			continue
		}
		links = append(links, l)
	}

	if len(links) == 0 {
		log.Printf("警告: 未能附加到任何网络设备")
	}

	return links, nil
}

// GetMap 获取指定的 eBPF map
func (c *Collector) GetMap(name string) *ebpf.Map {
	switch name {
	case "network_map":
		return c.objs.NetworkMap
	case "tcp_latency_map":
		if c.tcpMetricsObjs != nil {
			return c.tcpMetricsObjs.TcpLatencyMap
		}
	case "tcp_stats_map":
		if c.tcpMetricsObjs != nil {
			return c.tcpMetricsObjs.TcpStatsMap
		}
	case "global_tcp_metrics_map":
		if c.tcpMetricsObjs != nil {
			return c.tcpMetricsObjs.GlobalTcpMetricsMap
		}
	case "http_stats_map":
		if c.httpMetricsObjs != nil {
			return c.httpMetricsObjs.HttpStatsMap
		}
	case "global_http_metrics_map":
		if c.httpMetricsObjs != nil {
			return c.httpMetricsObjs.GlobalHttpMetricsMap
		}
	}
	return nil
}

// GetTCPMetrics 获取TCP指标采集器对象
func (c *Collector) GetTCPMetrics() *bpf.TCPMetricsObjects {
	return c.tcpMetricsObjs
}

// GetHTTPMetrics 获取HTTP指标采集器对象
func (c *Collector) GetHTTPMetrics() *bpf.HTTPMetricsObjects {
	return c.httpMetricsObjs
}
