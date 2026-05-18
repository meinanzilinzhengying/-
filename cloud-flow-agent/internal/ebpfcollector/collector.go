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
	objs      *bpf.Objects
	links     []link.Link
	stopCh    chan struct{}
	collectCh chan []*edge.MetricData
}

// New 创建 eBPF 采集器
func New() (*Collector, error) {
	if err := rlimit.RemoveMemlock(); err != nil {
		return nil, fmt.Errorf("移除内存限制失败: %w", err)
	}

	objs, err := loadBPFObjects()
	if err != nil {
		return nil, fmt.Errorf("加载 eBPF 对象失败: %w", err)
	}

	links, err := attachProgram(objs.TCProg)
	if err != nil {
		objs.Close()
		return nil, fmt.Errorf("附加 eBPF 程序失败: %w", err)
	}

	// 降权为普通用户运行
	if err := dropPrivileges(); err != nil {
		log.Printf("警告: 无法降权: %v，将继续以当前权限运行", err)
	}

	return &Collector{
		objs:      objs,
		links:     links,
		stopCh:    make(chan struct{}),
		collectCh: make(chan []*edge.MetricData, 10),
	}, nil
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

	return metrics
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
func attachProgram(prog *ebpf.Program) ([]link.Link, error) {
	devices, err := net.Interfaces()
	if err != nil {
		return nil, fmt.Errorf("获取网络设备失败: %w", err)
	}

	var links []link.Link
	for _, dev := range devices {
		if dev.Name == "lo" {
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
	default:
		return nil
	}
}
