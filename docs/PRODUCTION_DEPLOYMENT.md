# CloudFlow 生产环境部署指南

## 概述

本指南提供 CloudFlow 平台的多种生产环境部署方案，从 Docker Compose 到 Kubernetes 集群，帮助你根据业务需求选择合适的部署架构。

## 部署方案对比

| 方案 | 适用场景 | 高可用 | 扩缩容 | 运维复杂度 |
|------|---------|--------|--------|-----------|
| Docker Compose (当前) | 开发/测试 | ❌ | ❌ | 低 |
| Docker Compose (生产) | 小规模生产 | ⚠️ 基础 | ⚠️ 手动 | 中 |
| Docker Swarm | 中等规模 | ✅ | ✅ | 中 |
| Kubernetes | 大规模生产 | ✅✅ | ✅✅ | 高 |

## 1. Docker Compose 生产部署

### 1.1 基础配置

使用 `docker-compose.prod.yml` 提供的基础高可用配置：

```bash
# 复制环境变量模板
cp .env.example .env
# 编辑 .env 填入实际的生产配置

# 启动生产环境
docker-compose -f docker-compose.prod.yml up -d

# 查看服务状态
docker-compose -f docker-compose.prod.yml ps

# 扩展特定服务
docker-compose -f docker-compose.prod.yml up -d --scale auth-service=3
```

### 1.2 高可用特性

**微服务多实例**：
- 每个微服务默认 2 个副本
- 支持滚动更新（rolling update）
- 自动故障恢复

**存储层 HA**：
- etcd：3 节点集群
- Kafka：3 节点 KRaft 集群
- Redis：主从复制（可选 Sentinel）

### 1.3 环境变量配置

```bash
# 必需的环境变量
export CLOUD_FLOW_DB_PASSWORD=your_strong_password
export REDIS_PASSWORD=your_redis_password
export CLICKHOUSE_PASSWORD=your_clickhouse_password
export JWT_PRIVATE_KEY=base64_encoded_rsa_private_key
export GRAFANA_ADMIN_PASSWORD=your_grafana_password
export ELASTIC_PASSWORD=your_elasticsearch_password

# 服务副本数配置
export AUTH_REPLICAS=3
export TENANT_REPLICAS=2
export CONTROL_REPLICAS=2
export DATA_REPLICAS=3
export QUERY_REPLICAS=3
export TOPOLOGY_REPLICAS=2
export ALERT_REPLICAS=2

# 存储地址配置
export TIDB_ADDR=tidb:4000
export CLICKHOUSE_ADDR=clickhouse:9000
export REDIS_ADDR=redis:6379
export ETCD_ENDPOINTS=etcd-1:2379,etcd-2:2379,etcd-3:2379
```

### 1.4 限制与注意事项

⚠️ **Docker Compose 部署的限制**：

1. **TiDB**：使用 mocktikv 存储引擎，不是真正的分布式数据库
2. **ClickHouse**：单节点，不支持分布式表和副本
3. **Redis**：主从复制，但无自动故障转移
4. **滚动更新**：需要手动协调

## 2. Docker Swarm 部署

对于需要比 Docker Compose 更多 HA 特性但不想运维 Kubernetes 的场景：

### 2.1 初始化 Swarm

```bash
# 初始化 Docker Swarm
docker swarm init

# 加入工作节点
docker swarm join-token worker
```

### 2.2 部署堆栈

```bash
# 创建配置
docker config create cloudflow-env < .env

# 部署堆栈
docker stack deploy -c docker-compose.prod.yml -c docker-compose.swarm.yml cloudflow

# 查看服务
docker service ls

# 查看服务日志
docker service logs cloudflow_auth-service
```

### 2.3 Swarm 高可用特性

- **服务副本**：自动在多个节点间分布
- **健康检查**：自动重启失败的容器
- **滚动更新**：零停机部署
- **负载均衡**：内置 DNS 和 VIP 负载均衡

## 3. Kubernetes 部署（推荐生产环境）

### 3.1 架构概览

```
┌─────────────────────────────────────────────────────────────────┐
│                         Kubernetes Cluster                       │
│                                                                  │
│  ┌─────────────┐  ┌─────────────┐  ┌─────────────┐               │
│  │   Node 1    │  │   Node 2    │  │   Node 3    │               │
│  │  ┌───────┐  │  │  ┌───────┐  │  │  ┌───────┐  │               │
│  │  │ auth  │  │  │  │ auth  │  │  │  │ auth  │  │               │
│  │  │tenant │  │  │  │tenant │  │  │  │tenant │  │               │
│  │  │ data  │  │  │  │ data  │  │  │  │ data  │  │               │
│  │  └───────┘  │  │  └───────┘  │  │  └───────┘  │               │
│  └─────────────┘  └─────────────┘  └─────────────┘               │
│                                                                  │
│  ┌─────────────────────────────────────────────────────┐         │
│  │              Managed Services (外部)                  │         │
│  │  TiDB Cloud │ ClickHouse Cloud │ Redis Cluster       │         │
│  └─────────────────────────────────────────────────────┘         │
└─────────────────────────────────────────────────────────────────┘
```

### 3.2 推荐托管服务

| 组件 | 推荐托管服务 | 优点 |
|------|------------|------|
| TiDB | TiDB Cloud (Serverless/Dedicated) | 全托管、自动扩缩容、跨区域复制 |
| ClickHouse | ClickHouse Cloud | 全托管、弹性扩缩容 |
| Redis | Redis Enterprise / ElastiCache | 高可用、自动故障转移 |
| Kafka | Confluent Cloud / MSK | 全托管、无限扩缩容 |
| Elasticsearch | Elastic Cloud | 全托管、搜索优化 |

### 3.3 Kubernetes 资源清单

需要创建以下 Kubernetes 资源：

```
kubernetes/
├── namespace.yaml
├── configmap/
│   └── nginx.conf.yaml
├── secret/
│   └── credentials.yaml (使用 Sealed Secrets)
├── deployment/
│   ├── auth-service.yaml
│   ├── tenant-service.yaml
│   ├── control-plane.yaml
│   ├── data-plane.yaml
│   ├── query-service.yaml
│   ├── topology-engine.yaml
│   └── alert-engine.yaml
├── service/
│   ├── auth-service.yaml
│   ├── tenant-service.yaml
│   └── ...
├── ingress/
│   └── nginx-ingress.yaml
├── hpa/
│   └── *.yaml (Horizontal Pod Autoscaler)
└── pdb/
    └── *.yaml (Pod Disruption Budget)
```

### 3.4 HPA (Horizontal Pod Autoscaler) 示例

```yaml
apiVersion: autoscaling/v2
kind: HorizontalPodAutoscaler
metadata:
  name: auth-service-hpa
spec:
  scaleTargetRef:
    apiVersion: apps/v1
    kind: Deployment
    name: auth-service
  minReplicas: 2
  maxReplicas: 10
  metrics:
  - type: Resource
    resource:
      name: cpu
      target:
        type: Utilization
        averageUtilization: 70
  - type: Resource
    resource:
      name: memory
      target:
        type: Utilization
        averageUtilization: 80
  behavior:
    scaleUp:
      stabilizationWindowSeconds: 60
      policies:
      - type: Percent
        value: 100
        periodSeconds: 15
    scaleDown:
      stabilizationWindowSeconds: 300
      policies:
      - type: Percent
        value: 10
        periodSeconds: 60
```

### 3.5 PDB (Pod Disruption Budget) 示例

```yaml
apiVersion: policy/v1
kind: PodDisruptionBudget
metadata:
  name: auth-service-pdb
spec:
  minAvailable: 1
  selector:
    matchLabels:
      app: auth-service
```

## 4. 数据层高可用配置

### 4.1 TiDB Cloud (推荐)

```bash
# 使用 TiDB Cloud Serverless
# 1. 在 TiDB Cloud 控制台创建集群
# 2. 获取连接字符串
export TIDB_ADDR=xxx.tidbcloud.com:4000
export TIDB_USER=your_user
export TIDB_PASSWORD=your_password
```

优势：
- 全球分布式部署
- 自动备份和恢复
- 无服务器架构，按需付费

### 4.2 ClickHouse 集群配置

```yaml
# clickhouse-cluster-config.yaml
apiVersion: clickhouse.operator.clickhouse.com/v1
kind: ClickHouseInstallation
metadata:
  name: cloudflow-ch
spec:
  configuration:
    clusters:
    - name: default
      shards:
      - replicaCount: 2  # 每分片 2 个副本
        templates:
          podTemplate: clickhouse-pod
      - replicaCount: 2
        templates:
          podTemplate: clickhouse-pod
    layouts:
    - name: default
      tableClusters:
      - clusterName: default
        tableNameOverride: flows
        createTableQuery: |
          CREATE TABLE flows (...)
          ENGINE = ReplicatedMergeTree(...)
```

### 4.3 Redis Cluster/Sentinel

```bash
# 使用 Redis Sentinel 高可用
# 1. 部署 Redis Sentinel
# 2. 配置应用使用 Sentinel 自动发现

REDIS_SENTINEL_ADDRS=sentinel1:26379,sentinel2:26379,sentinel3:26379
REDIS_MASTER_NAME=mymaster
```

### 4.4 Kafka 集群

```yaml
# Kafka KRaft 模式配置
# docker-compose.prod.yml 已包含 3 节点配置
KAFKA_BROKERS=kafka-1:9092,kafka-2:9092,kafka-3:9092

# 推荐 Topic 配置
# 副本因子 = 3
# 最小 ISR = 2
```

## 5. 部署检查清单

### 5.1 部署前检查

- [ ] 所有环境变量已配置（无占位符）
- [ ] 密钥已从密钥管理服务加载
- [ ] TLS 证书已配置
- [ ] 数据库连接已测试
- [ ] 健康检查端点可访问
- [ ] 监控告警已配置

### 5.2 高可用验证

- [ ] 每个关键服务至少 2 个副本
- [ ] 模拟单节点故障测试
- [ ] 滚动更新测试
- [ ] 网络分区测试
- [ ] 数据持久化验证

### 5.3 监控验证

- [ ] Prometheus 可采集所有指标
- [ ] Grafana 仪表板可访问
- [ ] 告警规则已配置并测试
- [ ] 日志收集已配置
- [ ] 分布式追踪已配置

## 6. 故障恢复流程

### 6.1 服务故障

```bash
# 1. 查看故障服务日志
docker-compose -f docker-compose.prod.yml logs auth-service

# 2. 重启故障服务
docker-compose -f docker-compose.prod.yml restart auth-service

# 3. 如果需要扩展
docker-compose -f docker-compose.prod.yml up -d --scale auth-service=3
```

### 6.2 数据层故障

```bash
# TiDB Cloud: 使用控制台进行故障转移
# ClickHouse: 检查副本同步状态
docker exec cloudflow-clickhouse clickhouse-client --query "SYSTEM TABLETS cloudflow.flows"

# Redis: 检查主从状态
docker exec cloudflow-redis redis-cli -a $REDIS_PASSWORD info replication
```

## 7. 扩展指南

### 7.1 水平扩展

```bash
# 增加服务副本
docker-compose -f docker-compose.prod.yml up -d --scale auth-service=5

# Kubernetes HPA 自动扩展
kubectl autoscale deployment auth-service --cpu-percent=70 --min=2 --max=20
```

### 7.2 垂直扩展

```bash
# Docker Compose: 修改资源限制后重新部署
# Kubernetes: 修改资源请求和限制
kubectl patch deployment auth-service -p '{"spec":{"template":{"spec":{"containers":[{"name":"auth-service","resources":{"limits":{"cpu":"2","memory":"2Gi"}}}]}}}}'
```

### 7.3 滚动更新

```bash
# Docker Compose
docker-compose -f docker-compose.prod.yml up -d

# Kubernetes
kubectl set image deployment/auth-service auth-service=cloudflow-auth:v1.2.0
kubectl rollout status deployment/auth-service
```

## 8. 监控和告警

### 8.1 关键指标

| 指标 | 告警阈值 | 说明 |
|------|---------|------|
| 服务副本数 | < 最小副本数 | 服务不可用风险 |
| CPU 使用率 | > 80% | 性能下降 |
| 内存使用率 | > 85% | OOM 风险 |
| 请求延迟 P99 | > 500ms | 用户体验下降 |
| 错误率 | > 1% | 服务异常 |

### 8.2 Grafana 仪表板

建议创建以下仪表板：
- 服务健康概览
- 请求延迟和吞吐量
- 错误率和成功率
- 资源使用率
- 存储容量和增长趋势

## 9. 参考资源

- [TiDB Cloud 文档](https://docs.pingcap.com/tidbcloud)
- [ClickHouse Cloud 文档](https://clickhouse.com/docs/cloud)
- [Redis 高可用指南](https://redis.io/docs/management/scaling/)
- [Kafka KRaft 模式](https://developer.confluent.io/learn/apache-kafka-raft/)
- [Kubernetes 生产实践](https://kubernetes.io/docs/setup/production-environment/)
