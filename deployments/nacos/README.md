# Cloud Flow Nacos 配置中心

## 概述

Cloud Flow 使用 Nacos 作为配置中心，支持：

- **配置集中管理**：所有服务配置统一管理
- **配置热更新**：无需重启服务即可更新配置
- **环境隔离**：dev/test/prod 环境配置分离
- **版本控制**：配置变更历史可追溯

## 目录结构

```
deployments/nacos/
├── configs/                    # 配置文件模板
│   ├── center-service-dev.json
│   ├── center-service-test.json
│   ├── center-service-prod.json
│   ├── database-dev.json
│   ├── database-test.json
│   ├── database-prod.json
│   ├── redis-dev.json
│   ├── redis-test.json
│   ├── redis-prod.json
│   ├── logging-dev.json
│   ├── logging-test.json
│   └── logging-prod.json
├── init-configs.sh            # 配置初始化脚本
├── docker-compose.yml         # Nacos 本地部署
└── README.md                  # 本文档
```

## 快速开始

### 1. 启动 Nacos（本地开发）

```bash
cd deployments/nacos
docker-compose up -d
```

访问 Nacos 控制台：http://localhost:8848/nacos
- 默认用户名/密码：nacos/nacos

### 2. 初始化配置

```bash
# 初始化开发环境配置
./init-configs.sh dev

# 初始化测试环境配置
./init-configs.sh test

# 初始化生产环境配置
./init-configs.sh prod

# 初始化所有环境
./init-configs.sh all
```

### 3. 服务启动

设置环境变量启用 Nacos：

```bash
export NACOS_ENABLED=true
export NACOS_SERVER_ADDR=localhost:8848
export NACOS_NAMESPACE=public
export NACOS_GROUP=DEFAULT_GROUP
export NACOS_APP_ENV=dev  # dev/test/prod

# 启动服务
./cloud-flow-center
```

## 配置说明

### 环境变量

| 变量名 | 说明 | 默认值 |
|--------|------|--------|
| `NACOS_ENABLED` | 是否启用 Nacos | `false` |
| `NACOS_SERVER_ADDR` | Nacos 服务器地址 | `localhost:8848` |
| `NACOS_NAMESPACE` | 命名空间 | `public` |
| `NACOS_GROUP` | 配置分组 | `DEFAULT_GROUP` |
| `NACOS_USERNAME` | 用户名 | - |
| `NACOS_PASSWORD` | 密码 | - |
| `NACOS_APP_ENV` | 应用环境 | `dev` |
| `NACOS_APP_NAME` | 应用名称 | `cloud-flow` |

### 配置 Data ID 命名规则

```
{app-name}-{config-name}-{env}

例如：
- cloud-flow-center-service-dev
- cloud-flow-database-prod
- cloud-flow-redis-test
```

### 支持的配置项

| 配置名称 | 说明 | 格式 |
|----------|------|------|
| `center-service` | 中心服务配置 | JSON |
| `database` | 数据库配置 | JSON |
| `redis` | Redis 配置 | JSON |
| `logging` | 日志配置 | JSON |

## 配置热更新

### 自动热更新

以下配置支持热更新（无需重启服务）：

- ✅ 日志级别 (`logging.level`)
- ✅ 日志格式 (`logging.format`)
- ✅ 连接池大小（部分实现）

### 需要重启的配置

以下配置变更需要重启服务：

- ❌ 数据库连接地址
- ❌ 数据库用户名/密码
- ❌ Redis 连接地址
- ❌ 服务端口

## 环境配置差异

### 开发环境 (dev)

- 日志级别：debug
- 数据库：单实例
- SSL：禁用
- 连接池：较小

### 测试环境 (test)

- 日志级别：info
- 数据库：单实例
- SSL：启用
- 连接池：中等

### 生产环境 (prod)

- 日志级别：warn
- 数据库：集群
- SSL：强制验证
- 连接池：较大
- 超时：更短

## 手动管理配置

### 使用 Nacos 控制台

1. 访问 http://localhost:8848/nacos
2. 进入「配置管理」→「配置列表」
3. 选择对应的 Group 和 Namespace
4. 点击「+」新建配置或编辑现有配置

### 使用 API

```bash
# 获取配置
curl -X GET "http://localhost:8848/nacos/v1/cs/configs?dataId=cloud-flow-center-service-dev&group=DEFAULT_GROUP"

# 发布配置
curl -X POST "http://localhost:8848/nacos/v1/cs/configs" \
  -d "dataId=cloud-flow-center-service-dev" \
  -d "group=DEFAULT_GROUP" \
  -d "content={\"level\":\"debug\"}"

# 删除配置
curl -X DELETE "http://localhost:8848/nacos/v1/cs/configs?dataId=cloud-flow-center-service-dev&group=DEFAULT_GROUP"
```

## 生产部署建议

### Nacos 集群部署

生产环境建议使用 Nacos 集群（至少 3 节点）：

```yaml
# nacos-cluster.yaml
version: '3.8'
services:
  nacos1:
    image: nacos/nacos-server:v2.3.0
    environment:
      - MODE=cluster
      - NACOS_SERVERS=nacos1:8848 nacos2:8848 nacos3:8848
      # ... 其他配置
  nacos2:
    # ...
  nacos3:
    # ...
```

### 配置安全

1. **启用认证**：设置 `NACOS_AUTH_ENABLE=true`
2. **使用 HTTPS**：配置 TLS 证书
3. **敏感信息加密**：使用 Nacos 加密配置功能
4. **访问控制**：配置 Nacos 权限控制

### 备份策略

1. 定期导出配置：`nacos-cli config export`
2. 配置版本控制：将配置提交到 Git
3. 数据库备份：定期备份 Nacos MySQL 数据

## 故障排查

### 服务无法连接 Nacos

```bash
# 检查 Nacos 健康状态
curl http://localhost:8848/nacos/actuator/health

# 检查配置是否存在
curl "http://localhost:8848/nacos/v1/cs/configs?dataId=cloud-flow-center-service-dev&group=DEFAULT_GROUP"
```

### 配置未生效

1. 检查 `NACOS_ENABLED=true`
2. 检查 `NACOS_APP_ENV` 是否匹配
3. 查看服务日志中的配置加载信息
4. 确认配置 Data ID 命名正确

### 热更新不生效

- 确认配置项支持热更新
- 检查监听器是否正确注册
- 查看 Nacos 推送日志
