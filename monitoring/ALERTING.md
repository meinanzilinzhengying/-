# CloudFlow 监控告警系统

## 概述

CloudFlow 具备完整的监控告警体系，涵盖服务可用性、系统资源、业务指标等多个维度。

## 核心组件

| 组件 | 功能 | 访问地址 |
|------|------|----------|
| Prometheus | 指标采集与存储 | http://localhost:9091 |
| Alertmanager | 告警管理与通知 | http://localhost:9093 |
| Grafana | 可视化面板 | http://localhost:3001 (admin/admin) |
| Blackbox Exporter | 服务健康检查 | http://localhost:9115 |
| Node Exporter | 宿主机指标 | http://localhost:9100 |
| cAdvisor | 容器指标 | http://localhost:9101 |
| Kafka Exporter | Kafka 指标 | http://localhost:9308 |
| Jaeger | 分布式追踪 | http://localhost:16686 |
| Loki | 日志收集 | http://localhost:3100 |

## 告警规则

### 核心告警规则

#### 1. 服务可用性告警 (Critical)
- **ServiceDown**: 服务离线超过1分钟
- **TiDBDown**: TiDB 数据库不可用
- **RedisDown**: Redis 缓存不可用
- **KafkaBrokerDown**: Kafka 消息队列不可用
- **ClickHouseDown**: ClickHouse 数据库不可用

#### 2. 系统资源告警
- **HighMemoryUsage (Warning)**: 内存使用率 > 85%
- **CriticalMemoryUsage (Critical)**: 内存使用率 > 95%
- **HighCPUUsage (Warning)**: CPU 使用率 > 85%
- **DiskSpaceLow (Warning)**: 磁盘使用率 > 85%
- **DiskSpaceCritical (Critical)**: 磁盘使用率 > 95%
- **FilesystemFillPredict (Warning)**: 预测24小时内磁盘耗尽

#### 3. Kafka 告警
- **KafkaLagHigh (Warning)**: 消费者组滞后 > 10000条
- **KafkaLagCritical (Critical)**: 消费者组滞后 > 100000条
- **KafkaMessageRateHigh (Info)**: 消息生产速率高

#### 4. 容器告警
- **ContainerOOMKilled (Critical)**: 容器发生OOM Kill
- **HighContainerRestartCount (Warning)**: 容器1小时内重启 > 3次
- **ContainerCpuUsageHigh (Warning)**: 容器CPU使用率 > 80%
- **ContainerMemoryUsageHigh (Warning)**: 容器内存使用率 > 85%

#### 5. 业务服务告警
- **AuthServiceHighErrorRate**: 认证服务错误率高
- **AuthServiceHighLatency**: 认证服务延迟高
- **ControlPlaneHighErrorRate**: 控制平面错误率高
- **DataPlaneHighErrorRate**: 数据平面错误率高
- **DataPlaneBacklogHigh**: 数据平面队列积压

#### 6. 业务指标告警
- **FlowProcessingDelayHigh**: 流量处理延迟过高
- **FlowsIngestionRateDrop**: 流量采集速率骤降50%

## 告警流程

```
Prometheus 采集指标
        ↓
    告警评估
        ↓
  触发告警规则
        ↓
 Alertmanager
        ↓
  路由到接收器
        ↓
  Alert Engine
```

## 配置告警通知

### 1. Webhook 通知
已配置 Alertmanager 自动发送告警到 Alert Engine 的 `/api/v1/alerts/webhook` 端点。

### 2. 邮件通知 (可选)
编辑 `monitoring/alertmanager/alertmanager.yml`，取消邮件通知注释：

```yaml
email_configs:
  - to: 'your-email@example.com'
    send_resolved: true
```

配置环境变量：
```bash
SMTP_HOST=smtp.example.com:587
SMTP_FROM=alertmanager@cloudflow.local
SMTP_USER=your-smtp-user
SMTP_PASSWORD=your-smtp-password
ALERT_EMAIL_TO=admin@example.com
```

## 常用操作

### 查看告警
```bash
# 访问 Prometheus Alertmanager
open http://localhost:9093

# 查看所有告警规则
open http://localhost:9091/graph?g0.expr=ALERTS
```

### 查看服务健康
```bash
# 查看 Prometheus 目标状态
open http://localhost:9091/targets

# 查看 Grafana 面板
open http://localhost:3001
```

### 静默特定告警
1. 访问 Alertmanager
2. 点击 "New Silence"
3. 选择要静默的告警
4. 设置静默时长

## PromQL 查询示例

### 1. 服务可用性
```promql
up{job=~"auth-service|control-plane|data-plane"}
```

### 2. 内存使用率
```promql
(1 - node_memory_MemAvailable_bytes / node_memory_MemTotal_bytes) * 100
```

### 3. CPU 使用率
```promql
100 - (avg by(instance) (rate(node_cpu_seconds_total{mode="idle"}[5m])) * 100)
```

### 4. 磁盘使用率
```promql
(1 - node_filesystem_avail_bytes{fstype!~"tmpfs|overlay"} / node_filesystem_size_bytes{fstype!~"tmpfs|overlay"}) * 100
```

### 5. Kafka 消费者组滞后
```promql
kafka_consumergroup_lag
```

### 6. 容器重启次数
```promql
increase(container_restarts_total[1h]) > 3
```

### 7. 活跃告警
```promql
ALERTS{alertstate="firing"}
```

## Grafana 面板

Grafana 已经配置好 Prometheus 数据源，你可以：
1. 访问 http://localhost:3001
2. 使用 admin/admin 登录
3. 创建自定义面板
4. 导入官方面板（如 Node Exporter 等）

## 故障排查

### Prometheus 目标显示 Down
1. 检查目标服务是否启动
2. 查看服务日志
3. 验证 Prometheus 配置

### Alertmanager 未发送告警
1. 检查 Alertmanager 配置
2. 查看 Alertmanager 日志
3. 验证接收器地址可访问

### 告警触发但未通知
1. 检查告警抑制规则
2. 检查告警路由配置
3. 检查通知接收器配置

## 自定义告警

编辑 `monitoring/prometheus/rules/alerts.yml` 文件：

```yaml
groups:
  - name: custom-rules
    rules:
      - alert: MyCustomAlert
        expr: metric > 100
        for: 5m
        labels:
          severity: warning
        annotations:
          summary: "Custom alert summary"
          description: "Custom alert description"
```

修改后重启 Prometheus：
```bash
docker compose restart prometheus
```
