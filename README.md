# Cloud Flow - 云内流量监测系统

基于 eBPF 技术的云内网络流量采集、分析与可视化平台。采用探针-边缘-中心三层架构，支持网络流量实时监控、系统指标采集、告警管理和可视化仪表盘。

## 系统架构

```
Agent (eBPF探针) ──gRPC──▶ Edge (边缘节点) ──gRPC──▶ Center (中心服务)
                                                          │
Frontend (Vue 3) ◀──HTTP API──── Center Portal ◀── TiDB + Redis
```

### 核心组件

| 组件 | 技术栈 | 说明 |
|------|--------|------|
| **Agent** | Go + eBPF | 探针采集端，网络流量 + 系统指标 |
| **Edge** | Go + gRPC | 边缘节点，探针管理与数据转发 |
| **Center** | Go + gRPC | 中心服务，API 网关 + 数据存储 |
| **Frontend** | Vue 3 + Element Plus | Web 管理界面 |
| **TiDB** | MySQL 兼容 | 分布式数据库 |
| **Redis** | 缓存 | CSRF Token + 会话存储 |

### 监控栈

| 组件 | 说明 |
|------|------|
| **Prometheus** | 指标采集与存储 |
| **Grafana** | 可视化仪表盘 |
| **Alertmanager** | 告警管理 |
| **Loki** | 日志聚合 |

## 快速开始

### 环境要求

- Docker 20.10+
- Docker Compose v2+
- Linux (Ubuntu 20.04+ / CentOS 7+)
- 8GB+ 内存，50GB+ 磁盘

### 安装部署

```bash
# 1. 克隆代码
git clone https://github.com/meinanzilinzhengying/-.git
cd -

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

## 项目结构

```
├── cloud-flow-agent/        # Go 探针采集端
├── cloud-flow-center/       # Go 中心服务
├── cloud-flow-edge/         # Go 边缘节点
├── cloud-flow-frontend/     # Vue 3 前端
├── pkg/                     # Go 公共工具包
├── proto/                   # gRPC Protobuf 定义
├── monitoring/              # 监控配置
├── docs/                    # 项目文档
├── docker-compose.yml       # Docker Compose 部署
└── docker-bake.hcl          # 多架构构建配置
```

## 常用命令

```bash
# 启动服务
docker compose up -d

# 停止服务
docker compose down

# 查看日志
docker compose logs -f center
docker compose logs -f agent

# 重启单个服务
docker compose restart center

# 查看服务状态
docker compose ps
```

## Agent 分离部署

在被监控主机上单独部署 Agent：

```bash
docker run -d \
  --name agent \
  --privileged \
  --network host \
  -e EDGE_ADDR=center.example.com:9090 \
  cloud-flow-agent:latest
```

> Agent 需要 `--privileged` 权限以加载 eBPF 程序。

## 前端功能模块

- **仪表盘** - 系统概览
- **搜索中心** - 统一搜索入口
- **分析中心** - 网络/应用分析
- **日志中心** - 日志查询与分析
- **资产管理** - 计算资源、网络、存储、容器
- **告警管理** - 策略、端点、事件
- **指标中心** - 主机/容器指标
- **业务观测** - 业务定义与服务拓扑
- **系统管理** - 采集器、数据节点、账号

## 技术栈

### 后端
- Go 1.22+
- gRPC + Protobuf
- eBPF (cilium/ebpf)
- TiDB (MySQL 兼容)
- Redis
- JWT 认证

### 前端
- Vue 3
- TypeScript
- Element Plus
- Vite
- Pinia
- Vue Router

## License

MIT
