# CloudFlow 服务启动依赖解决方案

## 问题

Docker Compose 的 `depends_on` 仅确保容器启动顺序，但不保证服务完全就绪，可能导致：

- 数据库未就绪时应用启动失败
- 服务反复 crash 重启
- 级联错误导致整个系统不可用

## 解决方案

### 1. 健康检查 + depends_on condition

我们已为所有服务添加了完善的健康检查和依赖条件。

#### 基础服务层（先启动）
- TiDB (数据库)
- Redis (缓存)
- etcd (服务发现)
- ClickHouse (时序数据)
- Zookeeper (Kafka 协调)
- Kafka (消息队列)

#### 业务服务层（后启动）
- auth-service
- tenant-service
- control-plane
- data-plane
- query-service
- topology-engine
- alert-engine

### 2. 启动脚本 `start.sh`

使用分阶段启动脚本确保依赖服务完全就绪。

#### 快速开始

```bash
# 1. 赋予执行权限
chmod +x scripts/start.sh
chmod +x scripts/wait-for-it.sh

# 2. 完整启动所有服务
./scripts/start.sh

# 3. 仅启动基础服务（数据库等）
./scripts/start.sh -b

# 4. 清理后重新启动
./scripts/start.sh -c
```

#### 脚本选项

| 选项 | 说明 |
|------|------|
| `-c, --clean` | 先清理再启动 |
| `-b, --base-only` | 仅启动基础服务 |
| `-h, --help` | 显示帮助 |

### 3. wait-for-it.sh 工具

在服务内部使用此工具确保依赖服务就绪后再启动。

```bash
# 在 Dockerfile 或 command 中使用
./wait-for-it.sh mysql:3306 -t 60 -- ./start-app.sh
```

## Docker Compose 配置说明

### 已配置的健康检查

```yaml
services:
  tidb:
    healthcheck:
      test: ["CMD", "curl", "-f", "http://localhost:10080/status"]
      interval: 10s
      timeout: 5s
      retries: 10
      start_period: 30s
```

### 服务依赖关系示例

```yaml
services:
  auth-service:
    depends_on:
      tidb:
        condition: service_healthy
      redis:
        condition: service_healthy
```

## 故障排查

### 服务反复重启

查看服务日志和健康状态：

```bash
# 查看状态
docker compose ps

# 查看特定服务日志
docker compose logs -f [service-name]

# 查看健康检查失败原因
docker inspect --format='{{.State.Health}}' [container-id]
```

### 手动检查服务健康

```bash
# 检查 TiDB
curl -f http://localhost:10080/status

# 检查 Redis
redis-cli ping

# 检查 ClickHouse
wget --spider -q http://localhost:8123/ping
```

## 扩展阅读

- Docker Compose 健康检查文档: https://docs.docker.com/compose/compose-file/05-services/#healthcheck
- wait-for-it 项目: https://github.com/vishnubob/wait-for-it
