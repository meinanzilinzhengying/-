// Package transport 管理网络传输
// 指定管理网卡，避免占用业务带宽
package transport

import (
	"fmt"
	"net"
	"net/url"
	"os"
	"runtime"
	"strings"
	"sync"
	"time"
)

// ============================================================
// 传输配置
// ============================================================

// TransportConfig 传输配置
type TransportConfig struct {
	// 传输模式
	Mode          string   `yaml:"mode" json:"mode"`                     // direct/relay/grpc
	Protocol      string   `yaml:"protocol" json:"protocol"`             // tcp/udp/quic
	
	// 管理网卡配置
	ManagementNetwork ManagementNetworkConfig `yaml:"management_network" json:"management_network"`
	
	// 连接配置
	BufferSize       int      `yaml:"buffer_size" json:"buffer_size"`          // 发送缓冲区大小
	MaxConnections   int      `yaml:"max_connections" json:"max_connections"`  // 最大连接数
	ConnTimeout      int      `yaml:"conn_timeout" json:"conn_timeout"`         // 连接超时（秒）
	WriteTimeout     int      `yaml:"write_timeout" json:"write_timeout"`       // 写超时（秒）
	KeepAlive        int      `yaml:"keepalive" json:"keepalive"`               // KeepAlive（秒）
	
	// 可靠性配置
	Reliability ReliabilityConfig `yaml:"reliability" json:"reliability"`
	
	// 负载均衡
	LoadBalancing LoadBalancingConfig `yaml:"load_balancing" json:"load_balancing"`
}

// ManagementNetworkConfig 管理网配置
type ManagementNetworkConfig struct {
	Enabled       bool     `yaml:"enabled" json:"enabled"`               // 启用管理网
	InterfaceName string   `yaml:"interface_name" json:"interface_name"` // 管理网卡名
	BindAddress   string   `yaml:"bind_address" json:"bind_address"`     // 绑定地址
	SourceIP      string   `yaml:"source_ip" json:"source_ip"`           // 指定源IP
	AllowedIPs    []string `yaml:"allowed_ips" json:"allowed_ips"`       // 允许的目标IP列表
	BlockedIPs    []string `yaml:"blocked_ips" json:"blocked_ips"`       // 禁止的目标IP列表
	AutoDetect    bool     `yaml:"auto_detect" json:"auto_detect"`        // 自动检测管理网
	DetectPatterns []string `yaml:"detect_patterns" json:"detect_patterns"` // 检测模式
}

// ReliabilityConfig 可靠性配置
type ReliabilityConfig struct {
	Enabled         bool    `yaml:"enabled" json:"enabled"`
	EnableACK       bool   `yaml:"enable_ack" json:"enable_ack"`           // 启用确认
	EnableRetry     bool   `yaml:"enable_retry" json:"enable_retry"`       // 启用重传
	EnableDedupe    bool   `yaml:"enable_dedupe" json:"enable_dedupe"`     // 启用去重
	RetryCount      int    `yaml:"retry_count" json:"retry_count"`         // 重传次数
	RetryInterval   int    `yaml:"retry_interval" json:"retry_interval"`   // 重传间隔（毫秒）
	AckTimeout      int    `yaml:"ack_timeout" json:"ack_timeout"`         // 确认超时（毫秒）
	WindowSize      int    `yaml:"window_size" json:"window_size"`           // 滑动窗口大小
	DedupeWindowSec int    `yaml:"dedupe_window_sec" json:"dedupe_window_sec"` // 去重窗口（秒）
}

// LoadBalancingConfig 负载均衡配置
type LoadBalancingConfig struct {
	Enabled       bool   `yaml:"enabled" json:"enabled"`
	Strategy      string `yaml:"strategy" json:"strategy"`             // round_robin/least_conn/hash
	HealthCheck   bool   `yaml:"health_check" json:"health_check"`     // 健康检查
	HealthCheckInterval int `yaml:"health_check_interval" json:"health_check_interval"` // 检查间隔（秒）
}

// DefaultConfig 默认配置
func DefaultConfig() *TransportConfig {
	return &TransportConfig{
		Mode:     "direct",
		Protocol: "tcp",
		ManagementNetwork: ManagementNetworkConfig{
			Enabled:       true,
			InterfaceName: "",
			AutoDetect:    true,
			DetectPatterns: []string{"mgmt", "management", "管理", "control"},
		},
		BufferSize:     64 * 1024, // 64KB
		MaxConnections: 10,
		ConnTimeout:   30,
		WriteTimeout:  10,
		KeepAlive:     60,
		Reliability: ReliabilityConfig{
			Enabled:        true,
			EnableACK:     true,
			EnableRetry:   true,
			EnableDedupe:  true,
			RetryCount:    3,
			RetryInterval: 100,
			AckTimeout:    5000,
			WindowSize:    256,
			DedupeWindowSec: 300,
		},
		LoadBalancing: LoadBalancingConfig{
			Enabled:       true,
			Strategy:     "round_robin",
			HealthCheck:  true,
			HealthCheckInterval: 10,
		},
	}
}

// ============================================================
// 网络接口管理
// ============================================================

// InterfaceManager 网卡管理器
type InterfaceManager struct {
	config   *ManagementNetworkConfig
	iface   *net.Interface
	ifaceAddrs []net.Addr
}

// NewInterfaceManager 创建网卡管理器
func NewInterfaceManager(config *ManagementNetworkConfig) (*InterfaceManager, error) {
	mgr := &InterfaceManager{
		config: config,
	}
	
	if config.Enabled {
		if err := mgr.detectOrSelect(); err != nil {
			return nil, err
		}
	}
	
	return mgr, nil
}

// detectOrSelect 检测或选择管理网卡
func (m *InterfaceManager) detectOrSelect() error {
	// 如果指定了网卡名，直接使用
	if m.config.InterfaceName != "" {
		return m.selectByName(m.config.InterfaceName)
	}
	
	// 如果指定了源IP，查找对应的网卡
	if m.config.SourceIP != "" {
		return m.selectByIP(m.config.SourceIP)
	}
	
	// 自动检测
	if m.config.AutoDetect {
		return m.autoDetect()
	}
	
	return fmt.Errorf("未配置管理网卡")
}

// selectByName 按名称选择网卡
func (m *InterfaceManager) selectByName(name string) error {
	iface, err := net.InterfaceByName(name)
	if err != nil {
		return fmt.Errorf("未找到网卡 %s: %w", name, err)
	}
	
	m.iface = iface
	m.ifaceAddrs, _ = iface.Addrs()
	return nil
}

// selectByIP 按IP地址选择网卡
func (m *InterfaceManager) selectByIP(ip string) error {
	ifaces, err := net.Interfaces()
	if err != nil {
		return err
	}
	
	for _, iface := range ifaces {
		addrs, err := iface.Addrs()
		if err != nil {
			continue
		}
		
		for _, addr := range addrs {
			if ipnet, ok := addr.(*net.IPNet); ok {
				if ipnet.IP.String() == ip {
					m.iface = &iface
					m.ifaceAddrs = addrs
					return nil
				}
			}
		}
	}
	
	return fmt.Errorf("未找到IP %s 对应的网卡", ip)
}

// autoDetect 自动检测管理网卡
func (m *InterfaceManager) autoDetect() error {
	ifaces, err := net.Interfaces()
	if err != nil {
		return err
	}
	
	// 优先选择匹配模式的网卡
	for _, pattern := range m.config.DetectPatterns {
		for _, iface := range ifaces {
			if strings.Contains(strings.ToLower(iface.Name), strings.ToLower(pattern)) {
				if err := m.selectByName(iface.Name); err == nil {
					return nil
				}
			}
		}
	}
	
	// 选择非默认路由的网卡（排除常见的业务网卡）
	for _, iface := range ifaces {
		// 跳过 lo、docker、veth、virbr 等
		skip := false
		skipNames := []string{"lo", "docker", "veth", "virbr", "flannel", "cni", "weave"}
		for _, skipName := range skipNames {
			if strings.HasPrefix(iface.Name, skipName) {
				skip = true
				break
			}
		}
		if skip {
			continue
		}
		
		// 跳过点对点链路
		if iface.Flags&net.FlagPointToPoint != 0 {
			continue
		}
		
		// 选择第一个UP的普通网卡
		if iface.Flags&net.FlagUp != 0 {
			if err := m.selectByName(iface.Name); err == nil {
				return nil
			}
		}
	}
	
	// 降级：使用系统默认路由的网卡
	return m.selectDefaultRouteIface()
}

// selectDefaultRouteIface 选择默认路由网卡
func (m *InterfaceManager) selectDefaultRouteIface() error {
	// 读取 /proc/net/route 找默认路由
	data, err := os.ReadFile("/proc/net/route")
	if err != nil {
		return err
	}
	
	lines := strings.Split(string(data), "\n")
	for _, line := range lines[1:] { // 跳过标题行
		fields := strings.Fields(line)
		if len(fields) < 3 {
			continue
		}
		
		// 默认路由: Destination = 00000000
		if fields[1] == "00000000" {
			ifaceName := fields[0]
			return m.selectByName(ifaceName)
		}
	}
	
	return fmt.Errorf("未找到默认路由网卡")
}

// GetInterface 获取选定的网卡
func (m *InterfaceManager) GetInterface() *net.Interface {
	return m.iface
}

// GetSourceIP 获取源IP地址
func (m *InterfaceManager) GetSourceIP() string {
	if m.config.SourceIP != "" {
		return m.config.SourceIP
	}
	
	// 从选定的网卡获取
	for _, addr := range m.ifaceAddrs {
		if ipnet, ok := addr.(*net.IPNet); ok {
			if ipnet.IP.To4() != nil {
				return ipnet.IP.String()
			}
		}
	}
	
	return ""
}

// IsAllowed 检查目标IP是否允许
func (m *InterfaceManager) IsAllowed(ip string) bool {
	if !m.config.Enabled {
		return true
	}
	
	// 检查禁止列表
	for _, blocked := range m.config.BlockedIPs {
		if matched, _ := matchIP(ip, blocked); matched {
			return false
		}
	}
	
	// 如果没有配置允许列表，允许所有
	if len(m.config.AllowedIPs) == 0 {
		return true
	}
	
	// 检查允许列表
	for _, allowed := range m.config.AllowedIPs {
		if matched, _ := matchIP(ip, allowed); matched {
			return true
		}
	}
	
	return false
}

// matchIP 匹配IP或CIDR
func matchIP(ip, pattern string) (bool, error) {
	// 精确匹配
	if ip == pattern {
		return true, nil
	}
	
	// CIDR匹配
	if strings.Contains(pattern, "/") {
		_, net, err := net.ParseCIDR(pattern)
		if err != nil {
			return false, err
		}
		return net.Contains(net.ParseIP(ip)), nil
	}
	
	return false, nil
}

// ============================================================
// 传输器
// ============================================================

// Transporter 传输器
type Transporter struct {
	config   *TransportConfig
	ifaceMgr *InterfaceManager
	conn     net.Conn
	dialer   *net.Dialer
	addrs    []*url.URL
	curIdx   int
	mu       sync.Mutex
}

// NewTransporter 创建传输器
func NewTransporter(config *TransportConfig) (*Transporter, error) {
	if config == nil {
		config = DefaultConfig()
	}
	
	t := &Transporter{
		config: config,
		dialer: &net.Dialer{
			Timeout:   time.Duration(config.ConnTimeout) * time.Second,
			KeepAlive: time.Duration(config.KeepAlive) * time.Second,
		},
	}
	
	// 初始化网卡管理器
	if config.ManagementNetwork.Enabled {
		ifaceMgr, err := NewInterfaceManager(&config.ManagementNetwork)
		if err != nil {
			return nil, fmt.Errorf("初始化管理网卡失败: %w", err)
		}
		t.ifaceMgr = ifaceMgr
	}
	
	return t, nil
}

// SetAddresses 设置目标地址列表
func (t *Transporter) SetAddresses(addrs []string) error {
	t.addrs = make([]*url.URL, 0, len(addrs))
	for _, addr := range addrs {
		u, err := url.Parse(addr)
		if err != nil {
			return fmt.Errorf("无效的地址 %s: %w", addr, err)
		}
		t.addrs = append(t.addrs, u)
	}
	return nil
}

// Dial 连接到目标
func (t *Transporter) Dial() error {
	t.mu.Lock()
	defer t.mu.Unlock()
	
	if len(t.addrs) == 0 {
		return fmt.Errorf("未设置目标地址")
	}
	
	// 选择地址（负载均衡）
	addr := t.selectAddr()
	
	// 检查是否允许
	if t.ifaceMgr != nil {
		host := addr.Hostname()
		if !t.ifaceMgr.IsAllowed(host) {
			return fmt.Errorf("目标IP %s 不在允许列表中", host)
		}
	}
	
	// 设置绑定地址
	if t.ifaceMgr != nil && t.ifaceMgr.GetSourceIP() != "" {
		t.dialer.LocalAddr = &net.TCPAddr{
			IP: net.ParseIP(t.ifaceMgr.GetSourceIP()),
		}
	}
	
	// 创建连接
	network := t.config.Protocol
	if network == "grpc" {
		network = "tcp"
	}
	
	conn, err := t.dialer.Dial(network, addr.Host)
	if err != nil {
		return fmt.Errorf("连接失败: %w", err)
	}
	
	t.conn = conn
	return nil
}

// selectAddr 选择地址（负载均衡）
func (t *Transporter) selectAddr() *url.URL {
	if len(t.addrs) == 1 {
		return t.addrs[0]
	}
	
	switch t.config.LoadBalancing.Strategy {
	case "round_robin":
		addr := t.addrs[t.curIdx]
		t.curIdx = (t.curIdx + 1) % len(t.addrs)
		return addr
		
	case "least_conn":
		// 简化实现：返回第一个
		return t.addrs[0]
		
	case "hash":
		// 简化实现：使用一致性哈希
		return t.addrs[runtime.NumCPU()%len(t.addrs)]
		
	default:
		return t.addrs[0]
	}
}

// Write 写入数据
func (t *Transporter) Write(data []byte) (int, error) {
	t.mu.Lock()
	defer t.mu.Unlock()
	
	if t.conn == nil {
		return 0, fmt.Errorf("未建立连接")
	}
	
	// 设置写超时
	t.conn.SetWriteDeadline(time.Now().Add(time.Duration(t.config.WriteTimeout) * time.Second))
	
	n, err := t.conn.Write(data)
	if err != nil {
		return n, fmt.Errorf("写入失败: %w", err)
	}
	
	return n, nil
}

// Read 读取数据
func (t *Transporter) Read(buf []byte) (int, error) {
	t.mu.Lock()
	defer t.mu.Unlock()
	
	if t.conn == nil {
		return 0, fmt.Errorf("未建立连接")
	}
	
	n, err := t.conn.Read(buf)
	if err != nil {
		return n, fmt.Errorf("读取失败: %w", err)
	}
	
	return n, nil
}

// Close 关闭连接
func (t *Transporter) Close() error {
	t.mu.Lock()
	defer t.mu.Unlock()
	
	if t.conn != nil {
		err := t.conn.Close()
		t.conn = nil
		return err
	}
	return nil
}

// GetLocalAddr 获取本地地址
func (t *Transporter) GetLocalAddr() string {
	if t.conn != nil {
		return t.conn.LocalAddr().String()
	}
	if t.ifaceMgr != nil {
		return t.ifaceMgr.GetSourceIP()
	}
	return ""
}

// GetRemoteAddr 获取远程地址
func (t *Transporter) GetRemoteAddr() string {
	if t.conn != nil {
		return t.conn.RemoteAddr().String()
	}
	return ""
}

// GetInterfaceInfo 获取网卡信息
func (t *Transporter) GetInterfaceInfo() *InterfaceInfo {
	if t.ifaceMgr == nil {
		return nil
	}
	
	iface := t.ifaceMgr.GetInterface()
	if iface == nil {
		return nil
	}
	
	return &InterfaceInfo{
		Name:      iface.Name,
		MAC:       iface.HardwareAddr.String(),
		MTU:       iface.MTU,
		Flags:     iface.Flags.String(),
		IP:        t.ifaceMgr.GetSourceIP(),
	}
}

// InterfaceInfo 网卡信息
type InterfaceInfo struct {
	Name string `json:"name"`
	MAC  string `json:"mac"`
	MTU  int    `json:"mtu"`
	Flags string `json:"flags"`
	IP   string `json:"ip"`
}

// ============================================================
// 可用性检查
// ============================================================

// HealthChecker 健康检查器
type HealthChecker struct {
	config   *LoadBalancingConfig
	transporter *Transporter
	healthy  map[string]bool
	mu      sync.RWMutex
}

// NewHealthChecker 创建健康检查器
func NewHealthChecker(config *LoadBalancingConfig, transporter *Transporter) *HealthChecker {
	return &HealthChecker{
		config:     config,
		transporter: transporter,
		healthy:   make(map[string]bool),
	}
}

// Check 检查地址健康状态
func (h *HealthChecker) Check(addr string) bool {
	if !h.config.HealthCheck {
		return true
	}
	
	h.mu.RLock()
	isHealthy := h.healthy[addr]
	h.mu.RUnlock()
	
	return isHealthy
}

// SetHealthy 设置健康状态
func (h *HealthChecker) SetHealthy(addr string, healthy bool) {
	h.mu.Lock()
	h.healthy[addr] = healthy
	h.mu.Unlock()
}

// GetHealthyAddrs 获取健康地址列表
func (h *HealthChecker) GetHealthyAddrs(addrs []string) []string {
	if !h.config.HealthCheck {
		return addrs
	}
	
	h.mu.RLock()
	defer h.mu.RUnlock()
	
	var healthy []string
	for _, addr := range addrs {
		if h.healthy[addr] {
			healthy = append(healthy, addr)
		}
	}
	
	if len(healthy) == 0 {
		// 所有地址都不健康时，返回原始列表
		return addrs
	}
	
	return healthy
}
