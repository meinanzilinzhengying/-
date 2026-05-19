# Cloud Flow Agent 开发文档

## 目录

- [架构概览](#架构概览)
- [项目结构](#项目结构)
- [内核兼容性](#内核兼容性)
- [国产芯片适配](#国产芯片适配)
- [模块说明](#模块说明)
- [数据格式](#数据格式)
- [零干扰运维](#零干扰运维)
- [开发指南](#开发指南)
- [部署指南](#部署指南)

---

## 架构概览

Cloud Flow Agent 是云内流量监测平台的边缘探针，部署在目标主机上采集系统指标和网络流量数据，通过 gRPC 上报至 Edge 节点。

```
                    ┌──────────────────┐
                    │   Cloud Center   │
                    └────────┬─────────┘
                             │ gRPC
                    ┌────────┴─────────┐
                    │   Cloud Edge     │
                    └────────┬─────────┘
                             │ gRPC
              ┌──────────────┼──────────────┐
              │              │              │
       ┌──────┴──────┐ ┌────┴────┐ ┌──────┴──────┐
       │   Agent-1   │ │ Agent-2 │ │   Agent-N   │
       │  (主机 A)   │ │(主机 B) │ │  (主机 N)   │
       └─────────────┘ └─────────┘ └─────────────┘
```

### 核心设计原则

1. **零干扰**: Agent 对宿主机的影响降到最低（CPU < 1%，内存 < 50MB）
2. **自适应降级**: eBPF 不可用时自动降级到传统采集模式
3. **安全优先**: 最小权限、降权运行、安全加固
4. **可观测性**: 完善的日志、指标和健康检查

---

## 项目结构

```
cloud-flow-agent/
├── cmd/
│   └── main.go                    # 主入口
├── internal/
│   ├── collector/
│   │   ├── collector.go           # 传统采集器（CPU/内存/网络/磁盘）
│   │   └── process/
│   │       └── collector.go       # 进程事件采集器（netlink/proc）
│   ├── config/
│   │   └── config.go              # 配置管理（Viper）
│   ├── ebpfcollector/
│   │   ├── collector.go           # eBPF 采集器
│   │   ├── collector_stub.go      # 非 Linux 平台桩
│   │   ├── bpf/
│   │   │   ├── tc.bpf.c           # eBPF C 程序
│   │   │   ├── tc_bpf.go          # Go 绑定
│   │   │   └── tc_bpf_o.go        # 编译后的 BPF 对象
│   │   └── parser/
│   │       └── parser.go          # 网络数据解析
│   ├── grpcclient/
│   │   └── client.go              # gRPC 客户端
│   ├── http/
│   │   └── health.go              # HTTP 健康检查
│   ├── kernel/
│   │   └── detector.go            # 内核能力检测
│   └── runtime/
│       └── manager.go             # 运行时管理（资源监控/熔断/限流）
├── pkg/
│   ├── logger/
│   │   └── logger.go              # 日志（zap）
│   └── metrics/
│       └── metrics.go             # Prometheus 指标
├── configs/
│   └── config.yaml                # 配置文件
├── deployments/
│   ├── Dockerfile                 # Docker 构建
│   ├── pm2/                       # PM2 部署
│   └── systemd/
│       └── cloud-flow-agent.service  # systemd 服务
├── scripts/
│   ├── install.sh                 # 安装脚本
│   ├── uninstall.sh               # 卸载脚本
│   └── agentctl.sh                # 运维脚本
├── docs/
│   └── agent-development.md       # 本文档
├── Makefile                       # 构建脚本
└── go.mod                         # Go 模块定义
```

---

## 内核兼容性

### 最低要求

| 内核版本 | 支持状态 | 说明 |
|---------|---------|------|
| < 4.14 | 有限支持 | 仅传统采集模式，无 eBPF |
| 4.14 - 4.19 | 基本支持 | eBPF 可用，无 BTF |
| 5.0 - 5.7 | 良好支持 | eBPF + BTF（手动编译） |
| 5.8+      | 完全支持 | eBPF + BTF + RingBuffer |
| 5.15+     | 推荐版本 | CO-RE 完全支持 |

### 内核能力检测

Agent 启动时通过 `internal/kernel` 模块自动检测内核能力：

```go
import "cloud-flow-agent/internal/kernel"

detector := kernel.NewDetector(log)
cap, err := detector.Detect()
if err != nil {
    log.Warnf("内核检测失败: %v", err)
}

// 查询具体能力
if cap.EBPFSupported {
    log.Info("eBPF 可用，启用网络流量采集")
}
if cap.BTFSupported {
    log.Info("BTF 可用，支持 CO-RE")
}
if cap.RingBufSupported {
    log.Info("RingBuffer 可用，使用高性能数据传输")
}
```

### 能力矩阵

| 能力 | 检测方式 | 最低版本 |
|------|---------|---------|
| eBPF | `/sys/fs/bpf` + 内核版本 | 4.10 |
| BTF | `/sys/kernel/btf/vmlinux` | 5.2 |
| RingBuffer | 内核版本 | 5.8 |
| kprobes | `/sys/kernel/debug/kprobes/enabled` | 4.1 |
| uprobes | `/sys/kernel/debug/uprobes/enabled` | 4.1 |
| cgroup BPF | 内核版本 | 4.10 |
| tracepoint | `/sys/kernel/debug/tracing/available_events` | 4.7 |

---

## 国产芯片适配

### 支持的国产芯片

| 芯片 | 架构 | 厂商 | 状态 |
|------|------|------|------|
| 海光 (Hygon) | x86_64 | 海光信息 | 完全支持 |
| 鲲鹏 (Kunpeng) | aarch64 | 华为 | 完全支持 |

### 海光 (Hygon) 适配

海光芯片基于 AMD Zen 架构授权，与 x86_64 完全兼容。Agent 对海光芯片无需特殊处理，所有功能开箱即用。

检测方式：
- `/proc/cpuinfo` 中 `vendor_id` 为 `HygonGenuine`
- `model name` 包含 `Hygon` 或 `Dhyana`

### 鲲鹏 (Kunpeng) 适配

鲲鹏芯片基于 ARMv8 架构，需要以下适配：

1. **eBPF 编译**: 在鲲鹏环境下重新编译 eBPF 程序
   ```bash
   make ebpf  # 在鲲鹏主机上执行
   ```

2. **性能优化**: 利用 ARM64 NEON 指令集加速数据处理

3. **内核要求**: 鲲鹏服务器通常运行 EulerOS 或 openEuler，内核版本 >= 4.19

检测方式：
- `/proc/cpuinfo` 中 `CPU implementer` 为 `0x48`（华为）
- `model name` 或 `Hardware` 包含 `Kunpeng`、`鲲鹏`、`HiSilicon`

### 国产操作系统兼容

| 操作系统 | 内核版本 | 兼容性 |
|---------|---------|--------|
| openEuler 20.03+ | 4.19+ | 完全兼容 |
| EulerOS 2.0 | 3.10 | 基本兼容（无 eBPF） |
| 麒麟 V10 | 4.19+ | 完全兼容 |
| UOS 20 | 4.19+ | 完全兼容 |

---

## 模块说明

### 内核检测模块 (`internal/kernel`)

负责在 Agent 启动时检测系统环境能力，为后续功能模块提供决策依据。

```go
// 创建检测器
detector := kernel.NewDetector(log)

// 执行检测
cap, err := detector.Detect()

// 使用检测结果
if cap.IsDomesticChip() {
    log.Info("国产芯片，启用兼容模式")
}
if cap.IsEBPFAvailable() {
    // 启动 eBPF 采集器
}
```

### 进程事件采集器 (`internal/collector/process`)

通过 netlink connector 实时监听进程事件（fork/exec/exit），当 netlink 不可用时自动降级到 /proc 扫描模式。

```go
// 创建采集器
procCollector := process.NewCollector(log, process.DefaultConfig())

// 启动采集
if err := procCollector.Start(); err != nil {
    log.Warnf("进程采集器启动失败: %v", err)
}
defer procCollector.Stop()

// 采集数据（返回 edge.MetricData 格式）
metrics := procCollector.Collect()

// 查看采集模式
switch procCollector.Mode() {
case process.ModeNetlink:
    log.Info("使用 netlink connector 实时模式")
case process.ModeProcScan:
    log.Info("使用 /proc 扫描模式（降级）")
}
```

### 运行时管理器 (`internal/runtime`)

提供资源监控、熔断器和速率限制器，确保 Agent 自身的稳定运行。

```go
// 创建运行时管理器
mgr := runtime.NewManager(log, runtime.DefaultManagerConfig())

// 启动资源监控
mgr.Start()
defer mgr.Stop()

// 添加熔断器（用于 gRPC 通信保护）
cb := mgr.AddCircuitBreaker(runtime.CircuitBreakerConfig{
    Name:         "grpc-edge",
    MaxFailures:  5,
    ResetTimeout: 30 * time.Second,
})

// 添加速率限制器
rl := mgr.AddRateLimiter(runtime.RateLimiterConfig{
    Name:  "metric-send",
    Rate:  100,  // 100 QPS
    Burst: 200,  // 突发 200
})

// 注册优雅关闭回调
mgr.RegisterShutdown("ebpf-collector", func(ctx context.Context) error {
    ebpfCollector.Stop()
    return nil
})

// 优雅关闭
ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
defer cancel()
mgr.GracefulShutdown(ctx)
```

---

## 数据格式

Agent 使用 `cloud-flow/proto` 中定义的 `edge.MetricData` 作为统一数据格式：

```go
type MetricData struct {
    ProbeId     string            `json:"probe_id,omitempty"`
    Timestamp   int64             `json:"timestamp,omitempty"`
    SrcIp       string            `json:"src_ip,omitempty"`
    DstIp       string            `json:"dst_ip,omitempty"`
    SrcPort     int32             `json:"src_port,omitempty"`
    DstPort     int32             `json:"dst_port,omitempty"`
    Protocol    string            `json:"protocol,omitempty"`
    Bytes       int64             `json:"bytes,omitempty"`
    Packets     int64             `json:"packets,omitempty"`
    Latency     int64             `json:"latency,omitempty"`
    CpuUsage    float64           `json:"cpu_usage,omitempty"`
    MemoryUsage float64           `json:"memory_usage,omitempty"`
    DiskUsage   float64           `json:"disk_usage,omitempty"`
    Tags        map[string]string `json:"tags,omitempty"`
}
```

### 各模块数据映射

| 模块 | SrcIp | DstIp | Protocol | Bytes | Packets | Latency | Tags.type |
|------|-------|-------|----------|-------|---------|---------|-----------|
| CPU 采集器 | localhost | cpu | cpu | CPU%*100 | - | - | cpu_percent |
| 内存采集器 | localhost | memory | memory | used_bytes | total_bytes | used%*100 | memory |
| 网络采集器 | localhost | network | network | tx_delta | rx_delta | - | network |
| 磁盘采集器 | localhost | disk | disk | writes_delta | reads_delta | - | disk_io |
| eBPF 采集器 | src_ip | dst_ip | tcp/udp/icmp | bytes | packets | - | network_flow |
| 进程采集器 | localhost | process | process | total_count | - | - | process_total |

---

## 零干扰运维

### 资源限制

Agent 通过 systemd 服务文件和运行时管理器双重保障资源限制：

**systemd 层面：**
```ini
MemoryMax=512M          # 最大内存 512MB
CPUQuota=50%            # 最大 CPU 50%
LimitNOFILE=65536       # 文件描述符限制
LimitNPROC=4096         # 进程数限制
```

**运行时层面：**
```go
// 资源监控，超限告警
monitor := runtime.NewResourceMonitor(log, runtime.ResourceMonitorConfig{
    Interval:     10 * time.Second,
    MaxGoroutine: 10000,
    MaxMemoryMB:  512,
})
```

### 安全加固

1. **降权运行**: eBPF 加载完成后降权为 `cloud-flow` 用户
2. **Capability 限制**: 仅保留必要的 capabilities
   ```
   CAP_BPF, CAP_PERFMON, CAP_DAC_READ_SEARCH, CAP_SYS_PTRACE
   ```
3. **文件系统隔离**: `ProtectSystem=strict`, `PrivateTmp=true`
4. **网络隔离**: 通过 systemd 的网络命名空间限制

### 优雅关闭

Agent 支持多级优雅关闭：

1. 收到 SIGTERM/SIGINT 信号
2. 停止采集协程
3. 发送剩余缓冲数据
4. 关闭 gRPC 连接
5. 关闭 HTTP 服务器

```go
// 30 秒优雅关闭超时
const gracefulShutdownTimeout = 30 * time.Second
```

### 熔断保护

当 Edge 节点不可用时，熔断器自动打开，避免无效请求浪费资源：

```
Closed (正常) → 连续失败 5 次 → Open (熔断 30s) → HalfOpen (试探) → Closed
```

---

## 开发指南

### 环境要求

- Go 1.24+
- Linux 内核 4.14+（eBPF 需要 5.8+）
- clang/llvm（编译 eBPF 程序）
- make

### 构建

```bash
# 编译 Agent
make build

# 编译 eBPF 程序（需要 clang）
make ebpf

# 运行测试
make test

# 交叉编译（ARM64）
GOOS=linux GOARCH=arm64 make build
```

### 代码规范

1. **日志**: 使用 `cloud-flow-agent/pkg/logger`，不使用 `fmt.Println`
2. **数据格式**: 使用 `edge.MetricData`（来自 `cloud-flow/proto`）
3. **注释**: 添加中文注释说明功能
4. **错误处理**: 使用 `fmt.Errorf("描述: %w", err)` 包装错误
5. **并发安全**: 使用 `sync.Mutex` 保护共享状态

### 添加新采集器

1. 在 `internal/collector/` 下创建新目录
2. 实现 `Collect() []*edge.MetricData` 方法
3. 在 `cmd/main.go` 中注册
4. 添加配置项到 `internal/config/config.go`

---

## 部署指南

### 使用安装脚本

```bash
# 构建
make build

# 安装（自动检测架构和芯片）
sudo ./scripts/install.sh

# 指定二进制文件
sudo ./scripts/install.sh --binary=./build/cloud-flow-agent-x86_64

# 指定安装前缀
sudo ./scripts/install.sh --prefix=/usr/local/cloud-flow-agent
```

### 使用运维脚本

```bash
# 启动/停止/重启
./scripts/agentctl.sh start
./scripts/agentctl.sh stop
./scripts/agentctl.sh restart

# 查看状态
./scripts/agentctl.sh status

# 查看日志
./scripts/agentctl.sh logs -f

# 环境检查
./scripts/agentctl.sh check

# 诊断报告
./scripts/agentctl.sh diagnose
```

### Docker 部署

```bash
docker build -t cloud-flow-agent -f deployments/Dockerfile .
docker run -d \
  --name cloud-flow-agent \
  --privileged \
  --network host \
  -v /etc/cloud-flow-agent:/etc/cloud-flow-agent \
  cloud-flow-agent
```

### 卸载

```bash
# 保留配置和数据
sudo ./scripts/uninstall.sh

# 完全清除
sudo ./scripts/uninstall.sh --purge
```
