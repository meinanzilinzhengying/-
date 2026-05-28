# CloudFlow 微服务部署指南

## 📋 概述

CloudFlow 采用纯微服务架构，所有组件都通过 Docker Compose 一键部署。

## 🏗️ 架构组件

### 核心微服务（7个）

| 服务 | 端口 | 说明 |
|------|------|------|
| **auth-service** | 8006/9006 | 用户认证、RBAC 授权 |
| **tenant-service** | 8010/9010 | 租户、项目、配额管理 |
| **control-plane** | 8001/9001 | Agent 注册、服务发现 |
| **data-plane** | 9002/9102 | 数据接收、采样、转发 |
| **query-service** | 8007/9007 | Dashboard 查询、API 网关 |
| **topology-engine** | 8008/9008 | 服务拓扑、依赖分析 |
| **alert-engine** | 8009/9009 | 告警规则、评估、通知 |

### 基础存储

| 组件 | 端口 | 用途 |
|------|------|------|
| **TiDB** | 4000/10080 | 结构化数据（用户、租户、告警） |
| **ClickHouse** | 8123/9000 | 时序数据（Flow、Metrics） |
| **Redis** | 6379 | 缓存、会话 |
| **etcd** | 2379 | 服务发现、配置中心 |
| **Kafka** | 9092 | 消息队列 |

### 可观测性

| 组件 | 端口 | 用途 |
|------|------|------|
| **VictoriaMetrics** | 8428 | 指标存储 |
| **Loki** | 3100 | 日志存储 |
| **Prometheus** | 9091 | 指标收集 |
| **Grafana** | 3001 | 可视化面板 |
| **Jaeger** | 16686 | 分布式追踪 |

## 🚀 快速开始

### 前置要求

- Docker 24.0+
- Docker Compose v2.20+

### 启动步骤

1. **克隆项目**
   ```bash
   git clone https://github.com/your-org/cloudflow.git
   cd cloudflow
   ```

2. **配置环境变量**
   ```bash
   cp .env.example .env
   # 编辑 .env 文件设置数据库密码
   ```

3. **启动所有服务**
   ```bash
   docker compose up -d
   ```

4. **查看服务状态**
   ```bash
   docker compose ps
   ```

5. **查看日志**
   ```bash
   # 查看所有服务
   docker compose logs -f

   # 查看特定服务
   docker compose logs -f auth-service

   # 查看实时错误
   docker compose logs --tail=100 | grep ERROR
   ```

## 📊 服务访问

启动后访问以下地址：

| 服务 | 地址 | 默认凭证 |
|------|------|----------|
| API Gateway | http://localhost:8007 | - |
| Auth API | http://localhost:8006 | - |
| Control Plane | http://localhost:8001 | - |
| Grafana | http://localhost:3001 | admin/admin |
| Prometheus | http://localhost:9091 | - |
| Jaeger | http://localhost:16686 | - |

## 🔧 常用操作

### 重启特定服务
```bash
docker compose restart auth-service
```

### 更新服务
```bash
docker compose pull auth-service
docker compose up -d auth-service
```

### 缩放服务
```bash
# 扩展到 3 个实例
docker compose up -d --scale data-plane=3
```

### 清理数据
```bash
# 停止并删除所有数据
docker compose down -v

# 仅删除数据卷
docker compose down -v --remove-orphans
```

## 🔐 TLS/mTLS 配置

### 生成测试证书
```bash
chmod +x scripts/generate-certs.sh
./scripts/generate-certs.sh ./certs
```

### 启用 TLS
编辑 `docker-compose.yml` 中的 control-plane 服务：
```yaml
environment:
  TLS_ENABLED: "true"
  TLS_CLIENT_AUTH: "true"  # 启用 mTLS
```

### 禁用 TLS（开发模式）
```yaml
environment:
  TLS_ENABLED: "false"
```

## 🛠️ 开发调试

### 进入服务容器
```bash
docker exec -it cloudflow-auth /bin/sh
```

### 查看服务健康状态
```bash
curl http://localhost:8006/healthz
curl http://localhost:8001/healthz
curl http://localhost:8007/healthz
```

### 查看 gRPC 服务列表
```bash
grpcurl -plaintext localhost:9006 list
grpcurl -plaintext localhost:9001 list
```

## 🐛 故障排查

### 服务启动失败

1. **检查依赖服务**
   ```bash
   docker compose ps
   ```

2. **查看详细日志**
   ```bash
   docker compose logs -f <service-name> --tail=50
   ```

3. **检查端口冲突**
   ```bash
   netstat -tulpn | grep <port>
   ```

### 数据库连接失败

1. **检查 TiDB 健康状态**
   ```bash
   docker compose exec tidb mysql -uroot -p -e "SHOW DATABASES;"
   ```

2. **检查连接字符串**
   ```bash
   docker compose exec auth-service env | grep TIDB
   ```

### 数据无法写入

1. **检查 ClickHouse**
   ```bash
   curl http://localhost:8123/ping
   ```

2. **查看 data-plane 日志**
   ```bash
   docker compose logs -f data-plane
   ```

## 📈 性能调优

### 调整 data-plane 批量写入
```yaml
environment:
  BATCH_SIZE: 20000        # 增加批量大小
  FLUSH_INTERVAL: "500ms"   # 减少刷新间隔
  WORKER_COUNT: 8           # 增加工作线程
```

### 调整 ClickHouse
```yaml
environment:
  CLICKHOUSE_MAXThreads: 16
  CLICKHOUSE_MAXMemoryUsage: 8589934592
```

## 🔒 安全建议

1. **生产环境必须启用 TLS**
2. **使用强密码**替换默认密码
3. **限制数据库端口**仅本地访问
4. **配置防火墙规则**
5. **定期备份数据卷**

## 📞 支持

- 文档: https://docs.cloudflow.example.com
- Issues: https://github.com/your-org/cloudflow/issues
- 社区: https://slack.cloudflow.example.com
