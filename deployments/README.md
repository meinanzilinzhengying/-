# Cloud Flow Agent 部署文档

## 目录

1. [部署架构](#部署架构)
2. [单节点部署](#单节点部署)
3. [高可用部署](#高可用部署)
4. [同城双中心部署](#同城双中心部署)
5. [信创环境部署](#信创环境部署)
6. [配置说明](#配置说明)
7. [运维指南](#运维指南)

---

## 部署架构

### 架构概览

```
                    ┌─────────────────────────────────────────┐
                    │              负载均衡 (VIP)              │
                    │         192.168.1.100 (Keepalived)       │
                    └───────────────────┬─────────────────────┘
                                        │
            ┌───────────────────────────┼───────────────────────────┐
            │                           │                           │
            ▼                           ▼                           ▼
    ┌───────────────┐           ┌───────────────┐           ┌───────────────┐
    │   Center-A    │           │   Center-B    │           │   Center-C    │
    │   (主中心)     │           │   (备中心)     │           │   (仲裁节点)   │
    │  192.168.1.10 │           │  192.168.1.11 │           │  192.168.1.12 │
    └───────┬───────┘           └───────┬───────┘           └───────────────┘
            │                           │
    ┌───────┴───────┐           ┌───────┴───────┐
    │               │           │               │
    ▼               ▼           ▼               ▼
┌───────┐     ┌───────┐   ┌───────┐     ┌───────┐
│Edge-A1│     │Edge-A2│   │Edge-B1│     │Edge-B2│
└───────┘     └───────┘   └───────┘     └───────┘
    │               │           │               │
    └───────┬───────┴───────────┴───────┬───────┘
            │                           │
            ▼                           ▼
    ┌───────────────┐           ┌───────────────┐
    │   Agent 集群   │           │   Agent 集群   │
    │   (中心A区域)  │           │   (中心B区域)  │
    └───────────────┘           └───────────────┘
```

### 组件说明

| 组件 | 说明 | 端口 |
|------|------|------|
| Center | 控制中心，负责全局配置、策略管理 | 8080(HTTP), 50051(gRPC) |
| Edge | 边缘节点，数据聚合与转发 | 9090(HTTP), 9091(gRPC) |
| Agent | 采集探针，数据采集与上报 | 9090(HTTP) |
| Redis | 缓存/消息队列 | 6379 |
| PostgreSQL | 元数据存储 | 5432 |
| etcd | 分布式协调/选主 | 2379, 2380 |
| Keepalived | VIP管理 | - |

---

## 单节点部署

### 前置条件

- Docker 20.10+
- Docker Compose 2.0+
- 至少 8GB 内存
- 至少 50GB 磁盘空间

### 快速部署

```bash
# 1. 克隆代码
git clone https://github.com/meinanzilinzhengying/cloud-flow-agent.git
cd cloud-flow-agent

# 2. 配置环境变量
cp .env.example .env
vi .env

# 3. 启动服务
docker-compose -f deployments/docker/docker-compose.yml up -d

# 4. 查看状态
docker-compose -f deployments/docker/docker-compose.yml ps

# 5. 查看日志
docker-compose -f deployments/docker/docker-compose.yml logs -f agent
```

### 验证部署

```bash
# 检查服务健康
curl http://localhost:8080/health

# 检查 Agent 状态
curl http://localhost:9090/api/v1/status

# 访问 Grafana
open http://localhost:3000
# 默认账号: admin / admin123
```

---

## 高可用部署

### 架构要求

- 至少 3 个节点（奇数个，用于 etcd 选举）
- 节点间网络延迟 < 10ms
- 共享存储（可选，用于数据持久化）

### 部署步骤

```bash
# 在每个节点上执行

# 1. 初始化集群
docker swarm init --advertise-addr <节点IP>

# 2. 加入集群（其他节点）
docker swarm join --token <TOKEN> <MANAGER_IP>:2377

# 3. 创建 overlay 网络
docker network create --driver overlay --attachable cloud-flow-ha

# 4. 部署服务栈
docker stack deploy -c deployments/docker/docker-compose.yml cloud-flow

# 5. 查看服务状态
docker service ls
docker service ps cloud-flow_agent
```

### 服务扩缩容

```bash
# 扩展 Agent 副本
docker service scale cloud-flow_agent=5

# 扩展 Edge 副本
docker service scale cloud-flow_edge=3
```

---

## 同城双中心部署

### 架构要求

- 两个数据中心（中心A、中心B）
- 数据中心间网络延迟 < 5ms
- 带宽 >= 1Gbps
- 每个中心至少 2 个节点

### 部署步骤

#### 中心A（主中心）

```bash
# 1. 进入部署目录
cd deployments/ha

# 2. 配置环境变量
cat > .env.center-a <<EOF
CENTER_ID=center-a
CENTER_ROLE=primary
VIP=192.168.1.100
PEER_B=192.168.1.11
REGION=center-a
ZONE=zone-a
DB_USER=cloudflow
DB_PASSWORD=your-secure-password
REPL_PASSWORD=your-repl-password
JWT_SECRET=your-jwt-secret
EOF

# 3. 启动服务
docker-compose -f docker-compose-center-a.yml --env-file .env.center-a up -d

# 4. 验证服务
docker-compose -f docker-compose-center-a.yml ps
```

#### 中心B（备中心）

```bash
# 1. 进入部署目录
cd deployments/ha

# 2. 配置环境变量
cat > .env.center-b <<EOF
CENTER_ID=center-b
CENTER_ROLE=standby
VIP=192.168.1.100
PEER_A=192.168.1.10
REGION=center-b
ZONE=zone-b
DB_USER=cloudflow
DB_PASSWORD=your-secure-password
REPL_PASSWORD=your-repl-password
JWT_SECRET=your-jwt-secret
EOF

# 3. 启动服务
docker-compose -f docker-compose-center-b.yml --env-file .env.center-b up -d

# 4. 验证服务
docker-compose -f docker-compose-center-b.yml ps
```

### 故障切换测试

```bash
# 1. 模拟主中心故障
docker-compose -f docker-compose-center-a.yml down

# 2. 观察备中心自动接管
docker-compose -f docker-compose-center-b.yml logs -f center-b

# 3. 恢复主中心
docker-compose -f docker-compose-center-a.yml up -d

# 4. 验证主中心恢复为主
docker-compose -f docker-compose-center-a.yml logs -f center-a
```

### 数据同步验证

```bash
# PostgreSQL 主从同步
docker exec -it cloud-flow-postgres-a psql -U cloudflow -c "SELECT * FROM pg_stat_replication;"

# Redis 主从同步
docker exec -it cloud-flow-redis-a redis-cli INFO replication

# etcd 集群状态
docker exec -it cloud-flow-etcd-a etcdctl member list
docker exec -it cloud-flow-etcd-a etcdctl endpoint status --cluster
```

---

## 信创环境部署

### 支持的信创平台

| 平台 | 架构 | CPU | 操作系统 |
|------|------|-----|----------|
| 鲲鹏 | arm64 | 华为鲲鹏 | EulerOS, 麒麟, 统信 |
| 海光 | x86_64 | 海光 | 麒麟, 统信 |
| 飞腾 | arm64 | 飞腾 | 麒麟, 统信 |
| 龙芯 | loongarch64 | 龙芯3A5000 | 龙芯操作系统 |
| 兆芯 | x86_64 | 兆芯 | 麒麟, 统信 |

### 构建信创镜像

```bash
# 进入部署目录
cd deployments/xinchuang

# 构建所有信创镜像
./build-xinchuang.sh all

# 构建特定平台镜像
./build-xinchuang.sh kylin      # 麒麟系统
./build-xinchuang.sh uos        # 统信UOS
./build-xinchuang.sh euler      # EulerOS
./build-xinchuang.sh kunpeng    # 鲲鹏
./build-xinchuang.sh hygon      # 海光
./build-xinchuang.sh loongarch  # 龙芯
```

### 部署到信创环境

#### 麒麟系统

```bash
# 拉取麒麟镜像
docker pull registry.example.com/cloud-flow/cloud-flow-agent:latest-kylin-arm64

# 运行容器
docker run -d \
    --name cloud-flow-agent \
    --privileged \
    --pid=host \
    --network=host \
    -v /etc/cloud-flow/agent.yaml:/etc/cloud-flow/agent.yaml:ro \
    -v /var/lib/cloud-flow:/var/lib/cloud-flow \
    -v /sys/kernel/debug:/sys/kernel/debug:ro \
    -v /proc:/proc:ro \
    registry.example.com/cloud-flow/cloud-flow-agent:latest-kylin-arm64
```

#### 统信UOS

```bash
# 拉取UOS镜像
docker pull registry.example.com/cloud-flow/cloud-flow-agent:latest-uos-arm64

# 运行容器
docker run -d \
    --name cloud-flow-agent \
    --privileged \
    --pid=host \
    --network=host \
    -v /etc/cloud-flow/agent.yaml:/etc/cloud-flow/agent.yaml:ro \
    -v /var/lib/cloud-flow:/var/lib/cloud-flow \
    -v /sys/kernel/debug:/sys/kernel/debug:ro \
    -v /proc:/proc:ro \
    registry.example.com/cloud-flow/cloud-flow-agent:latest-uos-arm64
```

#### EulerOS

```bash
# 拉取EulerOS镜像
docker pull registry.example.com/cloud-flow/cloud-flow-agent:latest-euler-arm64

# 运行容器
docker run -d \
    --name cloud-flow-agent \
    --privileged \
    --pid=host \
    --network=host \
    -v /etc/cloud-flow/agent.yaml:/etc/cloud-flow/agent.yaml:ro \
    -v /var/lib/cloud-flow:/var/lib/cloud-flow \
    -v /sys/kernel/debug:/sys/kernel/debug:ro \
    -v /proc:/proc:ro \
    registry.example.com/cloud-flow/cloud-flow-agent:latest-euler-arm64
```

---

## 配置说明

### 环境变量

| 变量 | 说明 | 默认值 |
|------|------|--------|
| VERSION | 镜像版本 | latest |
| CENTER_URL | 控制中心地址 | http://center:8080 |
| EDGE_URL | 边缘节点地址 | http://edge:9090 |
| AGENT_ID | Agent唯一标识 | 自动生成 |
| TENANT_ID | 租户ID | default |
| REGION | 区域标识 | default |
| ZONE | 可用区标识 | default |
| DB_USER | 数据库用户名 | cloudflow |
| DB_PASSWORD | 数据库密码 | cloudflow123 |
| JWT_SECRET | JWT密钥 | - |

### 配置文件

```yaml
# agent.yaml 示例
agent:
  id: ""
  name: "cloud-flow-agent"
  log_level: info
  log_format: json

center:
  url: "http://center:8080"
  timeout: 30s
  retry: 3

collectors:
  network:
    enabled: true
    interval: 15s
  system:
    enabled: true
    interval: 30s
  process:
    enabled: true
    interval: 10s

ebpf:
  enabled: true
  kernel_version: ""
```

---

## 运维指南

### 日常运维

```bash
# 查看服务状态
docker-compose ps

# 查看日志
docker-compose logs -f agent

# 重启服务
docker-compose restart agent

# 更新服务
docker-compose pull agent
docker-compose up -d agent
```

### 监控告警

```bash
# Prometheus 指标
curl http://localhost:9090/metrics

# Grafana 仪表盘
open http://localhost:3000

# 告警规则
cat config/prometheus/alerts.yml
```

### 备份恢复

```bash
# 备份 PostgreSQL
docker exec cloud-flow-postgres pg_dump -U cloudflow cloudflow > backup.sql

# 备份 Redis
docker exec cloud-flow-redis redis-cli BGSAVE
docker cp cloud-flow-redis:/data/dump.rdb ./redis-backup.rdb

# 恢复 PostgreSQL
cat backup.sql | docker exec -i cloud-flow-postgres psql -U cloudflow cloudflow

# 恢复 Redis
docker cp ./redis-backup.rdb cloud-flow-redis:/data/dump.rdb
docker restart cloud-flow-redis
```

### 故障排查

```bash
# 检查容器状态
docker inspect cloud-flow-agent

# 查看容器日志
docker logs --tail 100 cloud-flow-agent

# 进入容器调试
docker exec -it cloud-flow-agent sh

# 检查网络连通性
docker exec cloud-flow-agent ping center
docker exec cloud-flow-agent curl http://center:8080/health

# 检查资源使用
docker stats cloud-flow-agent
```

### 性能调优

```bash
# 调整 Agent 资源限制
# 编辑 docker-compose.yml
deploy:
  resources:
    limits:
      cpus: '8'
      memory: 8G

# 调整采集间隔
# 编辑 configs/agent.yaml
collectors:
  network:
    interval: 30s  # 增大间隔降低CPU使用

# 调整数据缓冲
# 编辑 configs/agent.yaml
buffer:
  size: 10000
  flush_interval: 10s
```

---

## 常见问题

### Q1: Agent 无法连接 Center

```bash
# 检查网络
docker network ls
docker network inspect cloud-flow-network

# 检查 Center 服务
curl http://center:8080/health

# 检查防火墙
iptables -L -n
```

### Q2: eBPF 加载失败

```bash
# 检查内核版本 (需要 >= 4.18)
uname -r

# 检查 BTF 支持
ls /sys/kernel/btf/

# 检查权限
docker inspect cloud-flow-agent | grep Privileged
```

### Q3: 数据库连接失败

```bash
# 检查 PostgreSQL 状态
docker exec cloud-flow-postgres pg_isready

# 检查连接数
docker exec cloud-flow-postgres psql -c "SELECT count(*) FROM pg_stat_activity;"

# 检查日志
docker logs cloud-flow-postgres
```

### Q4: 内存占用过高

```bash
# 查看内存使用
docker stats

# 调整内存限制
docker update --memory 4g cloud-flow-agent

# 检查内存泄漏
curl http://localhost:9090/debug/pprof/heap > heap.out
go tool pprof heap.out
```

---

## 联系支持

- GitHub Issues: https://github.com/meinanzilinzhengying/cloud-flow-agent/issues
- 文档: https://docs.cloud-flow.io
- 邮箱: support@cloud-flow.io
