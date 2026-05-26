# CloudFlow - 云内流量可观测平台

基于 eBPF 技术的云内网络流量采集、分析与可观测平台。从网络监控升级为全栈可观测，支持 L7 协议解析、服务拓扑自动发现、分布式追踪关联、多存储引擎路由。

## 系统架构

```
                              ┌─────────────────────────────────────────────────────┐
                              │                  微服务集群                          │
                              │                                                     │
  Agent (eBPF) ──gRPC──▶ Edge │  ┌──────────────┐  ┌──────────────┐  ┌──────────┐  │
                              │  │ control-plane│  │  data-plane  │  │  auth    │  │
                              │  │  (管理面)     │  │  (数据面)     │  │  service │  │
                              │  └──────┬───────┘  └──────┬───────┘  └──────────┘  │
                              │         │                  │                         │
                              │  ┌──────┴───────┐  ┌──────┴───────┐  ┌──────────┐  │
                              │  │ topology-eng │  │ query-service│  │  tenant  │  │
                              │  │ (拓扑引擎)   │  │  (查询面)     │  │  service │  │
                              │  └──────────────┘  └──────────────┘  └──────────┘  │
                              │         │                  │                         │
                              │  ┌──────┴──────────────────┴──────┐  ┌──────────┐  │
                              │  │         alert-engine          │  │          │  │
                              │  │         (告警引擎)             │  │          │  │
                              │  └───────────────────────────────┘  └──────────┘  │
                              └─────────────────────────────────────────────────────┘
                                              │
                              ┌───────────────┼───────────────┐
                              ▼               ▼               ▼
                        ┌──────────┐   ┌──────────┐   ┌──────────┐
                        │ClickHouse│   │VictoriaMs│   │   Loki   │
                        │  (Flow)  │   │(Metrics) │   │  (Logs)  │
                        └──────────┘   └──────────┘   └──────────┘
                              │
                              ▼
                        ┌──────────┐
                        │   TiDB   │
                        │(Metadata)│
                        └──────────┘
```

### 核心微服务

| 服务 | 端口 | 职责 |
|------|------|------|
| **control-plane** | gRPC:9000, HTTP:8000 | Agent/Edge 管理、配置下发、服务发现 |
| **data-plane** | gRPC:9001, HTTP:8001 | Flow 接收、L7 解析、多存储路由 |
| **query-service** | gRPC:9002, HTTP:8002 | Dashboard 查询、API 网关 |
| **topology-engine** | gRPC:9004, HTTP:8004 | 服务拓扑、依赖图、Heatmap、拓扑 Diff |
| **alert-engine** | gRPC:9005, HTTP:8005 | 告警规则评估、告警事件管理 |
| **auth-service** | gRPC:9006, HTTP:8006 | JWT 认证、RBAC 鉴权 |
| **tenant-service** | gRPC:9007, HTTP:8007 | 多租户管理、配额控制 |

### 数据面组件

| 组件 | 技术栈 | 说明 |
|------|--------|------|
| **Agent** | Go + eBPF | 探针采集端，网络流量 + 系统指标 + L7 协议解析 |
| **Edge** | Go + gRPC | 边缘节点，探针管理与数据转发 |
| **Frontend** | Vue 3 + Element Plus | Web 管理界面 |

### 存储引擎

| 存储 | 用途 | 技术特点 |
|------|------|---------|
| **ClickHouse** | Flow 数据 | MergeTree + SummingMergeTree, LowCardinality, Bloom Filter, Skip Index, Materialized View |
| **VictoriaMetrics** | 指标数据 | Prometheus 兼容，长期存储 |
| **Loki** | 日志数据 | JSON Push API, Stream 分组 |
| **TiDB** | 元数据 | Agent/Edge/Tenant/Config |
| **etcd** | 服务发现 | Lease 注册，健康检查 |

## 核心能力

### 1. L7 协议解析 (Protocol Parser Engine)

基于插件的 L7 协议解析引擎，支持流式解析和部分重组：

| 协议 | 优先级 | 能力 |
|------|--------|------|
| **HTTP/1.x** | P0 | Method/Path/StatusCode/Headers 解析 |
| **HTTP/2** | P0 | Frame 解码、HPACK Header 解压、Stream 复用 |
| **gRPC** | P0 | Length-Prefixed Message、Service/Method 提取 |
| **MySQL** | P1 | Handshake、Query/Response 解析 |
| **Redis** | P1 | RESP 协议、Command/Key 提取 |
| **Kafka** | P1 | API Key/Version 识别、Topic 提取 |
| **DNS** | P1 | Query/Response 解析 |

- Plugin 架构 + Registry 注册
- 基于 Feature 的协议自动检测（非端口猜测）
- Streaming Parser + 部分重组（非全包重组）
- 独立 Worker 线程池 + Backpressure

### 2. 拓扑引擎 (Topology Engine)

自动从 UnifiedFlow 生成服务依赖关系图：

| 图类型 | 粒度 | 节点标识 |
|--------|------|---------|
| **Service Graph** | 服务级 | K8s Service > Deployment > IP |
| **Process Graph** | 进程级 | hostname:process:pid |
| **Pod Graph** | Pod 级 | namespace/pod |
| **Namespace Graph** | 命名空间级 | namespace |

- **实时拓扑**: 增量更新，5s 刷新周期
- **历史拓扑**: ClickHouse 时间范围查询
- **Latency Heatmap**: 边延迟分布热力图
- **Error Heatmap**: 边错误率分布热力图
- **Topology Diff**: 节点/边增删 + 指标变化检测
- **Graph Cache**: LRU + TTL 双重淘汰
- **Edge Weight**: `0.3*bytes + 0.3*latency + 0.2*errors + 0.2*requests`
- **Graph Pruning**: 低流量边自动裁剪
- 性能：百万级 edge，秒级更新

### 3. 统一流量模型 (UnifiedFlow)

```
UnifiedFlow (~4.5KB)
├── L3:   SrcIP, DstIP, IPVersion
├── L4:   SrcPort, DstPort, Protocol, TCPFlags
├── L7:   L7Protocol, Method, Path, StatusCode, ReqSize, RespSize
├── Process:   PID, ProcessName, Comm
├── Container: ContainerID, ContainerName, Image
├── K8s:   Pod, Namespace, Deployment, Service, Node
├── Trace: TraceID, SpanID, ParentID
├── Host:  HostID, Hostname
├── Tenant: TenantID
├── Metrics: Bytes, Packets, LatencyNs, Direction
└── Tags:  [16]Tag (Key-Value)
```

- 内存对齐，cache-line 友好
- FixedString 定长字符串（避免动态分配）
- Presence Bitmap（避免 nil 指针）
- 二进制序列化（非 JSON）

### 4. 多存储路由 (Storage Router)

```
Flow ──→ Router ──→ DataTypeFlow ──→ ClickHouse
                  ──→ DataTypeMetrics ──→ VictoriaMetrics
                  ──→ DataTypeLogs ──→ Loki
                  ──→ DataTypeMetadata ──→ TiDB
```

## 项目结构

```
├── cloud-flow-agent/              # eBPF 探针采集端
│   └── internal/l7parser/         #   L7 协议解析引擎 (16 files)
├── cloud-flow-center/             # 中心服务 (遗留单体，逐步废弃)
│   └── internal/storage/
│       ├── router.go              #   多存储路由
│       ├── clickhouse/            #   ClickHouse 存储
│       ├── victoriametrics/       #   VictoriaMetrics 写入
│       └── loki/                  #   Loki 写入
├── cloud-flow-edge/               # 边缘节点
├── cloud-flow-frontend/           # Vue 3 前端
├── pkg/                           # 公共工具包
│   └── flow/flow.go               #   UnifiedFlow 统一数据模型
├── proto/                         # gRPC Protobuf 定义
├── services/                      # 微服务集群
│   ├── proto/services.go          #   服务间通信协议定义
│   ├── shared/
│   │   ├── discovery/registry.go  #   etcd 服务发现
│   │   └── tracing/tracing.go     #   分布式追踪
│   ├── control-plane/             #   管理面
│   ├── data-plane/                #   数据面
│   ├── query-service/             #   查询面
│   ├── topology-engine/           #   拓扑引擎
│   │   ├── graph/graph.go         #     图数据结构
│   │   ├── builder/builder.go     #     4 种图构建器
│   │   ├── cache/cache.go         #     LRU+TTL 缓存
│   │   ├── updater/updater.go     #     增量更新引擎
│   │   ├── heatmap/heatmap.go     #     热力图引擎
│   │   └── historical/historical.go #   历史查询
│   ├── alert-engine/              #   告警引擎
│   ├── auth-service/              #   认证服务
│   └── tenant-service/            #   租户服务
├── monitoring/                    # Prometheus/Grafana/Alertmanager 配置
├── docs/                          # 项目文档
├── docker-compose.yml             # Docker Compose 部署
└── docker-bake.hcl                # 多架构构建配置
```

## 快速开始

### 环境要求

- Docker 20.10+
- Docker Compose v2+
- Linux (Ubuntu 20.04+ / CentOS 7+)
- 8GB+ 内存，50GB+ 磁盘

### 安装部署

```bash
# 1. 克隆代码
git clone https://github.com/meinanzilinzhengying/cloudflow.git
cd cloudflow

# 2. 配置环境变量
cp .env.example .env
```

编辑 `.env` 文件，设置以下必填项：

```env
CLOUD_FLOW_JWT_SECRET=your-jwt-secret-key
CLOUD_FLOW_ADMIN_PASSWORD=your-admin-password
REDIS_PASSWORD=your-redis-password
```

```bash
# 3. 一键启动
docker compose up -d

# 4. 查看服务状态
docker compose ps
```

### 服务访问

| 服务 | 地址 | 默认账号 |
|------|------|---------|
| **前端界面** | http://localhost:8888 | admin / (见 .env) |
| **Grafana** | http://localhost:3001 | admin / admin |
| **Prometheus** | http://localhost:9091 | - |
| **Alertmanager** | http://localhost:9094 | - |

## 技术栈

### 后端
- Go 1.24+
- gRPC + Protobuf（服务间通信）
- eBPF (cilium/ebpf)（内核级流量采集）
- ClickHouse（Flow 存储，MergeTree + LowCardinality + Bloom Filter）
- VictoriaMetrics（指标存储，Prometheus 兼容）
- Loki（日志聚合）
- TiDB（元数据，MySQL 兼容）
- etcd（服务发现 + Lease 注册）
- Redis（缓存 + 分布式锁）

### 前端
- Vue 3 + TypeScript
- Element Plus
- Vite + Pinia + Vue Router

## API 端点

### Topology Engine (HTTP:8004)

| 端点 | 说明 |
|------|------|
| `GET /api/topology/service` | 服务拓扑图 |
| `GET /api/topology/process` | 进程通信图 |
| `GET /api/topology/pod` | Pod 网络拓扑 |
| `GET /api/topology/namespace` | 命名空间流量图 |
| `GET /api/topology/heatmap/latency` | 延迟热力图 |
| `GET /api/topology/heatmap/error` | 错误热力图 |
| `GET /api/topology/diff` | 拓扑变更检测 |

### gRPC 服务

所有微服务同时暴露 gRPC 接口（端口 9000-9007），支持：
- 健康检查 (`HealthCheck`)
- 分布式追踪（OpenTelemetry 兼容拦截器）
- 服务发现（etcd Lease 注册）

## License

MIT
