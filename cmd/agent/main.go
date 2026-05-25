// Package main Agent 主程序入口
package main

import (
	"context"
	"flag"
	"fmt"
	"net"
	"os"
	"os/signal"
	"runtime"
	"syscall"
	"time"

	"github.com/meinanzilinzhengying/cloud-flow-agent/internal/collector/ebpf"
	"github.com/meinanzilinzhengying/cloud-flow-agent/internal/collector/metrics"
	"github.com/meinanzilinzhengying/cloud-flow-agent/internal/collector/process"
	"github.com/meinanzilinzhengying/cloud-flow-agent/internal/collector/traditional"
	grpcClient "github.com/meinanzilinzhengying/cloud-flow-agent/internal/grpc"
	"github.com/meinanzilinzhengying/cloud-flow-agent/internal/config"
	"github.com/meinanzilinzhengying/cloud-flow-agent/internal/kernel"
	"github.com/meinanzilinzhengying/cloud-flow-agent/internal/runtime"
	"github.com/meinanzilinzhengying/cloud-flow-agent/pkg/api"
	"github.com/meinanzilinzhengying/cloud-flow-agent/pkg/models"
)

var (
	version   = "dev"
	buildTime = "unknown"
	gitCommit = "unknown"
)

// Agent 主结构
type Agent struct {
	config       *models.Config
	configMgr    *config.Manager
	runtimeMgr   *runtime.Manager
	kernelInfo   *models.KernelCapability
	grpcClient   *grpcClient.Client

	// 采集器
	ebpfCollector      *ebpf.NetworkCollector
	traditionalCollector *traditional.NetworkCollector
	metricsCollector   *metrics.Collector
	processCollector   *process.Collector

	// 上下文
	ctx    context.Context
	cancel context.CancelFunc
}

func main() {
	// 解析命令行参数
	configPath := flag.String("config", "/etc/cloud-flow/agent.yaml", "配置文件路径")
	showVersion := flag.Bool("version", false, "显示版本信息")
	flag.Parse()

	if *showVersion {
		fmt.Printf("Cloud Flow Agent %s\n", version)
		fmt.Printf("  Build Time: %s\n", buildTime)
		fmt.Printf("  Git Commit: %s\n", gitCommit)
		fmt.Printf("  Go Version: %s\n", runtime.Version())
		fmt.Printf("  OS/Arch: %s/%s\n", runtime.GOOS, runtime.GOARCH)
		os.Exit(0)
	}

	// 创建 Agent
	agent := &Agent{
		configMgr: config.NewManager(*configPath),
	}

	// 运行 Agent
	if err := agent.Run(); err != nil {
		fmt.Fprintf(os.Stderr, "Agent exited with error: %v\n", err)
		os.Exit(1)
	}
}

// Run 运行 Agent
func (a *Agent) Run() error {
	// 加载配置
	cfg, err := a.configMgr.Load()
	if err != nil {
		return fmt.Errorf("failed to load config: %w", err)
	}
	a.config = cfg

	// 检测内核能力
	detector := kernel.NewDetector()
	kernelInfo, err := detector.Detect()
	if err != nil {
		return fmt.Errorf("failed to detect kernel: %w", err)
	}
	a.kernelInfo = kernelInfo

	// 打印启动信息
	a.printStartupInfo()

	// 检查最低要求
	if !kernelInfo.MinRequired {
		return fmt.Errorf("kernel version %s does not meet minimum requirement (>=3.10)", kernelInfo.Version)
	}

	// 初始化运行时管理器
	a.runtimeMgr = runtime.NewManager(&cfg.Resources)
	if err := a.runtimeMgr.Init(context.Background()); err != nil {
		return fmt.Errorf("failed to init runtime manager: %w", err)
	}

	// 创建上下文
	a.ctx, a.cancel = context.WithCancel(context.Background())
	defer a.cancel()

	// 初始化采集器
	if err := a.initCollectors(); err != nil {
		return fmt.Errorf("failed to init collectors: %w", err)
	}

	// 初始化 gRPC 客户端
	if err := a.initGRPCClient(); err != nil {
		return fmt.Errorf("failed to init gRPC client: %w", err)
	}

	// 启动采集器
	if err := a.startCollectors(); err != nil {
		return fmt.Errorf("failed to start collectors: %w", err)
	}

	// 启动数据上报
	go a.reportLoop()

	// 启动配置热加载
	go a.configWatchLoop()

	// 启动运行时管理
	a.runtimeMgr.Start()

	// 注册关闭回调
	a.runtimeMgr.RegisterShutdownCallback(a.shutdown)

	// 等待信号
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	select {
	case <-a.ctx.Done():
		return nil
	case sig := <-sigChan:
		fmt.Printf("Received signal %v, shutting down...\n", sig)
		return a.runtimeMgr.GracefulShutdown(a.ctx)
	}
}

// printStartupInfo 打印启动信息
func (a *Agent) printStartupInfo() {
	fmt.Println("========================================")
	fmt.Printf("Cloud Flow Agent %s\n", version)
	fmt.Println("========================================")
	fmt.Printf("Hostname:     %s\n", a.config.Agent.Hostname)
	fmt.Printf("Host IP:      %s\n", a.config.Agent.HostIP)
	fmt.Printf("Architecture: %s\n", a.kernelInfo.Arch)
	fmt.Printf("Kernel:       %s\n", a.kernelInfo.Version)
	fmt.Printf("eBPF Support: %v\n", a.kernelInfo.SupportsEBPF)
	fmt.Printf("BTF Support:  %v\n", a.kernelInfo.SupportsBTF)
	fmt.Printf("Edge Address: %s:%d\n", a.config.Edge.Address, a.config.Edge.Port)
	fmt.Println("========================================")
}

// initCollectors 初始化采集器
func (a *Agent) initCollectors() error {
	// 根据内核能力选择采集器
	if a.kernelInfo.SupportsEBPF && a.config.Collectors.EBPF.Enabled {
		// 使用 eBPF 采集器
		a.ebpfCollector = ebpf.NewNetworkCollector()
		if err := a.ebpfCollector.Init(a.ctx, &a.config.Collectors.EBPF); err != nil {
			fmt.Printf("Warning: failed to init eBPF collector: %v, falling back to traditional\n", err)
			a.ebpfCollector = nil
		} else {
			a.runtimeMgr.SetHealthStatus("ebpf-collector", true)
		}
	}

	// 初始化传统采集器（作为降级或补充）
	if a.config.Collectors.Traditional.Enabled || a.ebpfCollector == nil {
		a.traditionalCollector = traditional.NewNetworkCollector()
		if err := a.traditionalCollector.Init(a.ctx, &a.config.Collectors.Traditional); err != nil {
			return fmt.Errorf("failed to init traditional collector: %w", err)
		}
		a.runtimeMgr.SetHealthStatus("traditional-collector", true)
	}

	// 初始化系统指标采集器
	if a.config.Collectors.Metrics.Enabled {
		a.metricsCollector = metrics.NewCollector()
		if err := a.metricsCollector.Init(a.ctx, &a.config.Collectors.Metrics); err != nil {
			return fmt.Errorf("failed to init metrics collector: %w", err)
		}
		a.runtimeMgr.SetHealthStatus("metrics-collector", true)
	}

	// 初始化进程事件采集器
	if a.config.Collectors.Process.Enabled {
		a.processCollector = process.NewCollector()
		if err := a.processCollector.Init(a.ctx, &a.config.Collectors.Process); err != nil {
			return fmt.Errorf("failed to init process collector: %w", err)
		}
		a.runtimeMgr.SetHealthStatus("process-collector", true)
	}

	return nil
}

// initGRPCClient 初始化 gRPC 客户端
func (a *Agent) initGRPCClient() error {
	a.grpcClient = grpcClient.NewClient(&a.config.Edge)

	// 尝试连接
	if err := a.grpcClient.Connect(a.ctx); err != nil {
		fmt.Printf("Warning: failed to connect to edge: %v, will retry\n", err)
	}

	return nil
}

// startCollectors 启动采集器
func (a *Agent) startCollectors() error {
	if a.ebpfCollector != nil {
		if err := a.ebpfCollector.Start(a.ctx); err != nil {
			fmt.Printf("Warning: failed to start eBPF collector: %v\n", err)
			a.ebpfCollector = nil
		}
	}

	if a.traditionalCollector != nil {
		if err := a.traditionalCollector.Start(a.ctx); err != nil {
			return fmt.Errorf("failed to start traditional collector: %w", err)
		}
	}

	if a.metricsCollector != nil {
		if err := a.metricsCollector.Start(a.ctx); err != nil {
			return fmt.Errorf("failed to start metrics collector: %w", err)
		}
	}

	if a.processCollector != nil {
		if err := a.processCollector.Start(a.ctx); err != nil {
			return fmt.Errorf("failed to start process collector: %w", err)
		}
	}

	return nil
}

// reportLoop 数据上报循环
func (a *Agent) reportLoop() {
	ticker := time.NewTicker(time.Duration(a.config.Agent.Interval) * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-a.ctx.Done():
			return
		case <-ticker.C:
			a.reportData()
		}
	}
}

// reportData 上报数据
func (a *Agent) reportData() {
	if !a.grpcClient.IsConnected() {
		// 尝试重连
		if err := a.grpcClient.Connect(a.ctx); err != nil {
			return
		}
	}

	// 上报网络流量
	a.reportNetworkFlows()

	// 上报系统指标
	a.reportSystemMetrics()

	// 上报进程事件
	a.reportProcessEvents()

	// 发送心跳
	a.sendHeartbeat()
}

// reportNetworkFlows 上报网络流量
func (a *Agent) reportNetworkFlows() {
	var flows []*api.NetworkFlow

	// 从 eBPF 采集器获取
	if a.ebpfCollector != nil {
		for flow := range a.ebpfCollector.Flows() {
			flows = append(flows, convertNetworkFlow(flow))
			if len(flows) >= 1000 {
				break
			}
		}
	}

	// 从传统采集器获取
	if a.traditionalCollector != nil {
		for flow := range a.traditionalCollector.Flows() {
			flows = append(flows, convertNetworkFlow(flow))
			if len(flows) >= 1000 {
				break
			}
		}
	}

	if len(flows) > 0 {
		if err := a.grpcClient.ReportNetworkFlow(a.ctx, flows); err != nil {
			a.runtimeMgr.RecordError("grpc-report")
		}
	}
}

// reportSystemMetrics 上报系统指标
func (a *Agent) reportSystemMetrics() {
	if a.metricsCollector == nil {
		return
	}

	var metrics []*api.SystemMetric
	for metric := range a.metricsCollector.Metrics() {
		metrics = append(metrics, convertSystemMetric(metric))
		if len(metrics) >= 100 {
			break
		}
	}

	if len(metrics) > 0 {
		if err := a.grpcClient.ReportSystemMetrics(a.ctx, metrics); err != nil {
			a.runtimeMgr.RecordError("grpc-report")
		}
	}
}

// reportProcessEvents 上报进程事件
func (a *Agent) reportProcessEvents() {
	if a.processCollector == nil {
		return
	}

	var events []*api.ProcessEvent
	for event := range a.processCollector.ProcessEvents() {
		events = append(events, convertProcessEvent(event))
		if len(events) >= 1000 {
			break
		}
	}

	if len(events) > 0 {
		if err := a.grpcClient.ReportProcessEvents(a.ctx, events); err != nil {
			a.runtimeMgr.RecordError("grpc-report")
		}
	}
}

// sendHeartbeat 发送心跳
func (a *Agent) sendHeartbeat() {
	status := a.getStatus()
	req := &api.HeartbeatRequest{
		AgentId:   a.config.Agent.Hostname,
		Hostname:  a.config.Agent.Hostname,
		HostIp:    a.config.Agent.HostIP,
		Version:   version,
		Uptime:    0, // TODO: 计算运行时间
		Status:    status,
	}

	_, err := a.grpcClient.Heartbeat(a.ctx, req)
	if err != nil {
		a.runtimeMgr.RecordError("grpc-heartbeat")
	} else {
		a.runtimeMgr.ClearErrors("grpc-heartbeat")
	}
}

// getStatus 获取状态
func (a *Agent) getStatus() *api.AgentStatus {
	status := &api.AgentStatus{
		Collectors: make([]*api.CollectorStatus, 0),
	}

	if a.ebpfCollector != nil {
		s := a.ebpfCollector.Status()
		status.Collectors = append(status.Collectors, &api.CollectorStatus{
			Name:        s.Name,
			Type:        string(s.Type),
			Running:     s.Running,
			EventsCount: int64(s.EventsCount),
			DropCount:   int64(s.DropCount),
			LastError:   s.LastError,
		})
	}

	if a.traditionalCollector != nil {
		s := a.traditionalCollector.Status()
		status.Collectors = append(status.Collectors, &api.CollectorStatus{
			Name:        s.Name,
			Type:        string(s.Type),
			Running:     s.Running,
			EventsCount: int64(s.EventsCount),
			DropCount:   int64(s.DropCount),
			LastError:   s.LastError,
		})
	}

	cpu, mem, goroutines := a.runtimeMgr.GetResourceUsage()
	status.ResourceUsage = &api.ResourceUsage{
		CpuPercent: cpu,
		MemoryBytes: int64(mem),
		Goroutines: int32(goroutines),
	}

	return status
}

// configWatchLoop 配置热加载循环
func (a *Agent) configWatchLoop() {
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-a.ctx.Done():
			return
		case <-ticker.C:
			// 检查本地配置文件变化
			if err := a.configMgr.Reload(a.ctx); err != nil {
				continue
			}

			// 检查服务器配置更新
			if a.grpcClient.IsConnected() {
				data, hash, err := a.grpcClient.GetConfig(a.ctx, a.config.Agent.Hostname, a.configMgr.GetHash())
				if err == nil && len(data) > 0 {
					if err := a.configMgr.UpdateFromServer(data, hash); err != nil {
						fmt.Printf("Failed to update config: %v\n", err)
					} else {
						fmt.Println("Config updated from server")
						a.config = a.configMgr.Get()
					}
				}
			}
		}
	}
}

// shutdown 关闭
func (a *Agent) shutdown(ctx context.Context) error {
	fmt.Println("Shutting down agent...")

	// 停止采集器
	if a.ebpfCollector != nil {
		_ = a.ebpfCollector.Stop(ctx)
	}
	if a.traditionalCollector != nil {
		_ = a.traditionalCollector.Stop(ctx)
	}
	if a.metricsCollector != nil {
		_ = a.metricsCollector.Stop(ctx)
	}
	if a.processCollector != nil {
		_ = a.processCollector.Stop(ctx)
	}

	// 关闭 gRPC 连接
	if a.grpcClient != nil {
		_ = a.grpcClient.Close()
	}

	fmt.Println("Agent shutdown complete")
	return nil
}

// 转换函数
func convertNetworkFlow(f *models.NetworkFlow) *api.NetworkFlow {
	return &api.NetworkFlow{
		Timestamp:     f.Timestamp.UnixNano(),
		Protocol:      f.Protocol,
		SourceIp:      f.SourceIP,
		SourcePort:    uint32(f.SourcePort),
		DestIp:        f.DestIP,
		DestPort:      uint32(f.DestPort),
		ProcessName:   f.ProcessName,
		ProcessPid:    f.ProcessPID,
		BytesSent:     f.BytesSent,
		BytesRecv:     f.BytesRecv,
		PacketsSent:   f.PacketsSent,
		PacketsRecv:   f.PacketsRecv,
		DurationNs:    f.Duration,
		TcpState:      f.TCPState,
		CollectorType: string(f.CollectorType),
	}
}

func convertSystemMetric(m *models.SystemMetric) *api.SystemMetric {
	return &api.SystemMetric{
		Timestamp:       m.Timestamp.UnixNano(),
		HostIp:          m.HostIP,
		Hostname:        m.Hostname,
		CpuUsage:        m.CPUUsage,
		CpuUser:         m.CPUUser,
		CpuSystem:       m.CPUSystem,
		CpuIdle:         m.CPUIdle,
		CpuSteal:        m.CPUSteal,
		Load_1:          m.Load1,
		Load_5:          m.Load5,
		Load_15:         m.Load15,
		MemTotal:        m.MemTotal,
		MemUsed:         m.MemUsed,
		MemFree:         m.MemFree,
		MemBuffers:      m.MemBuffers,
		MemCached:       m.MemCached,
		MemUsage:        m.MemUsage,
		SwapTotal:       m.SwapTotal,
		SwapUsed:        m.SwapUsed,
		SwapUsage:       m.SwapUsage,
		DiskTotal:       m.DiskTotal,
		DiskUsed:        m.DiskUsed,
		DiskFree:        m.DiskFree,
		DiskUsage:       m.DiskUsage,
		DiskReadBytes:   m.DiskReadBytes,
		DiskWriteBytes:  m.DiskWriteBytes,
		DiskReadOps:     m.DiskReadOps,
		DiskWriteOps:    m.DiskWriteOps,
		NetBytesSent:    m.NetBytesSent,
		NetBytesRecv:    m.NetBytesRecv,
		NetPacketsSent:  m.NetPacketsSent,
		NetPacketsRecv:  m.NetPacketsRecv,
		NetTcpConns:     m.NetTCPConns,
		NetUdpConns:     m.NetUDPConns,
	}
}

func convertProcessEvent(e *models.ProcessEvent) *api.ProcessEvent {
	return &api.ProcessEvent{
		Timestamp: e.Timestamp.UnixNano(),
		EventType: e.EventType,
		Pid:       e.PID,
		Ppid:      e.PPID,
		Tid:       e.TID,
		Comm:      e.Comm,
		Exe:       e.Exe,
		Cmdline:   e.Cmdline,
		Cwd:       e.CWD,
		Uid:       e.UID,
		Gid:       e.GID,
		ExitCode:  e.ExitCode,
		Signal:    e.Signal,
	}
}

// 获取主机 IP
func getHostIP() string {
	addrs, err := net.InterfaceAddrs()
	if err != nil {
		return "127.0.0.1"
	}

	for _, addr := range addrs {
		if ipnet, ok := addr.(*net.IPNet); ok && !ipnet.IP.IsLoopback() {
			if ipnet.IP.To4() != nil {
				return ipnet.IP.String()
			}
		}
	}

	return "127.0.0.1"
}
