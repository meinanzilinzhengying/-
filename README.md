# CloudFlow 微服务架构

> **版本:** v2.0  
> **架构:** 纯微服务架构（已废弃旧 Center）  
> **状态:** 生产就绪

---

## 一、快速开始

### 1.1 环境要求

- Docker 20.10+
- Docker Compose 2.0+
- 至少 8GB 内存（推荐 16GB）

### 1.2 启动步骤

```bash
# 1. 克隆代码
git clone https://github.com/meinanzilinzhengying/cloudflow.git
cd cloudflow

# 2. 配置环境变量
cp .env.example .env
vi .env  # 修改密码等配置

# 3. 启动所有服务
docker compose up -d

# 4. 查看服务状态
docker compose ps

# 5. 查看日志（以认证服务为例）
docker compose logs -f auth-service
```

### 1.3 停止服务

```bash
# 停止服务（保留数据）
docker compose down

# 停止并清除数据卷
docker compose down -v
```

---

## 二、服务架构

### 2.1 核心微服务

| 服务 | 端口 | 职责 |
|------|------|------|
| **auth-service** | 8006/9006 | 认证授权、JWT、RBAC |
| **tenant-service** | 8010/9010 | 租户管理、项目配额 |
| **control-plane** | 8001/9001 | 服务编排、配置下发 |
| **data-plane** | 9002/9102 | 数据采集、缓冲写入 |
| **query-service** | 8007/9007 | 数据查询、聚合报表 |
| **topology-engine** | 8008/9008 | 拓扑发现、流量分析 |
| **alert-engine** | 8009/9009 | 告警规则、通知发送 |

### 2.2 数据存储

| 存储 | 端口 | 用途 |
|------|------|------|
| **TiDB** | 4000 | 结构化数据（用户、租户、规则） |
| **ClickHouse** | 8123/9000 | 时序数据（流量、指标） |
| **etcd** | 2379 | 服务发现、配置管理 |
| **Redis** | 6379 | 缓存、状态管理 |
| **Kafka** | 9092 | 消息队列 |

### 2.3 监控栈

| 服务 | 端口 | 用途 |
|------|------|------|
| **Prometheus** | 9091 | 指标收集 |
| **Grafana** | 3001 | 可视化面板 |
| **Jaeger** | 16686 | 分布式追踪 |
| **Loki** | 3100 | 日志存储 |

---

## 三、API 端点

### 3.1 认证服务 (auth-service)

```bash
# 健康检查
curl http://localhost:8006/healthz

# 用户登录
curl -X POST http://localhost:8006/api/auth/login \
  -H "Content-Type: application/json" \
  -d '{"username":"admin","password":"password"}'

# 获取 JWKS（用于验证 Token）
curl http://localhost:8006/api/auth/jwks
```

### 3.2 控制面 (control-plane)

```bash
# 健康检查
curl http://localhost:8001/healthz

# 需要认证的接口（需携带 Token）
curl -H "Authorization: Bearer <token>" http://localhost:8001/api/agents
```

### 3.3 查询服务 (query-service)

```bash
# 健康检查
curl http://localhost:8007/healthz

# 查询流量（需认证）
curl -H "Authorization: Bearer <token>" \
  "http://localhost:8007/api/v1/query/flows?start_time=1234567890&end_time=1234567900"
```

---

## 四、架构图

```
┌─────────────────────────────────────────────────────────────────┐
│                    CloudFlow 微服务架构 v2.0                     │
├─────────────────────────────────────────────────────────────────┤
│                                                                  │
│  ┌─────────┐                                                  │
│  │ Browser │                                                  │
│  └────┬────┘                                                  │
│       │                                                        │
│       ▼                                                        │
│  ┌─────────────────────────────────────────────────────────┐   │
│  │                    API Gateway                         │   │
│  └─────────────────────────────────────────────────────────┘   │
│                           │                                    │
│     ┌─────────────────────┼─────────────────────┐              │
│     │                     │                     │              │
│     ▼                     ▼                     ▼              │
│  ┌─────────┐      ┌──────────┐      ┌────────────┐           │
│  │ Auth    │      │ Control  │      │ Query      │           │
│  │ Service │      │ Plane    │      │ Service    │           │
│  └────┬────┘      └────┬─────┘      └─────┬──────┘           │
│       │                │                  │                    │
│       │                │                  │                    │
│       ▼                ▼                  │                    │
│  ┌─────────┐      ┌──────────┐           │                    │
│  │ Tenant  │      │ Data     │           │                    │
│  │ Service │      │ Plane    │           │                    │
│  └─────────┘      └────┬─────┘           │                    │
│                        │                  │                    │
│                        ▼                  │                    │
│                 ┌────────────┐           │                    │
│                 │ Topology   │           │                    │
│                 │ Engine     │           │                    │
│                 └────┬───────┘           │                    │
│                      │                   │                    │
│                      ▼                   │                    │
│                 ┌────────────┐           │                    │
│                 │ Alert      │           │                    │
│                 │ Engine     │           │                    │
│                 └────────────┘           │                    │
│                                          │                    │
│                                          ▼                    │
│                 ┌──────────────────────────────────┐           │
│                 │         数据存储层                │           │
│                 │  TiDB / ClickHouse / etcd       │           │
│                 │  Redis / Kafka                  │           │
│                 └──────────────────────────────────┘           │
└─────────────────────────────────────────────────────────────────┘
```

---

## 五、安全注意事项

### 5.1 生产环境配置

1. **数据库密码**：务必修改 `.env` 中的密码
2. **JWT 密钥**：生产环境应使用 2048-bit RSA 密钥
3. **TLS/SSL**：生产环境应启用 HTTPS
4. **网络隔离**：内部服务不应暴露到公网

### 5.2 端口安全

| 端口 | 暴露 | 说明 |
|------|------|------|
| 8006 | 可暴露 | Auth API |
| 8001 | 可暴露 | Control API |
| 8007 | 可暴露 | Query API |
| 4000 | 不应暴露 | TiDB |
| 9000 | 不应暴露 | ClickHouse |
| 2379 | 不应暴露 | etcd |

---

## 六、故障排查

### 6.1 常见问题

**问题 1：服务无法启动**

```bash
# 查看日志
docker compose logs <service-name>

# 检查依赖服务状态
docker compose ps
```

**问题 2：认证失败**

```bash
# 检查 Auth Service 是否正常
curl http://localhost:8006/healthz

# 检查 Token 是否有效
curl -H "Authorization: Bearer <token>" http://localhost:8006/api/auth/validate
```

**问题 3：数据写入失败**

```bash
# 检查 ClickHouse 连接
curl http://localhost:8123/ping

# 检查 Data Plane 日志
docker compose logs -f data-plane
```

### 6.2 日志收集

```bash
# 收集所有服务日志
docker compose logs > cloudflow.log

# 收集特定服务日志
docker compose logs auth-service > auth.log
```

---

## 七、开发指南

### 7.1 本地开发

```bash
# 进入服务目录
cd services/auth-service

# 安装依赖
go mod download

# 运行服务（需配置环境变量）
go run cmd/main.go

# 构建 Docker 镜像
docker build -t cloudflow/auth-service .
```

### 7.2 测试

```bash
# 运行单元测试
go test ./...

# 运行集成测试
go test -v ./integration/...
```

---

## 八、版本历史

| 版本 | 日期 | 变更 |
|------|------|------|
| v2.0 | 2026-05-28 | 纯微服务架构，废弃旧 Center |
| v1.0 | 2026-05-01 | 初始版本，双架构并存 |

---

## 九、贡献指南

1. Fork 仓库
2. 创建功能分支
3. 提交代码
4. 发起 Pull Request

---

## 十、许可证

MIT License
