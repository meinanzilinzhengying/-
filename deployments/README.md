# CloudFlow 部署配置

本目录包含 CloudFlow 平台的各种部署配置文件。

## 目录结构

```
deployments/
├── nginx/
│   ├── nginx.conf          # Nginx 负载均衡器配置
│   └── ssl/                # SSL 证书目录（需手动添加）
│       ├── cert.pem
│       └── key.pem
├── clickhouse/
│   └── config.d/           # ClickHouse 自定义配置
│       └── custom-config.xml
└── README.md
```

## Nginx 负载均衡器

Nginx 作为 API 网关和负载均衡器，提供以下功能：

### 主要功能

1. **服务路由**：将请求转发到对应的微服务
2. **负载均衡**：使用 least_conn 算法分配请求
3. **健康检查**：自动跳过故障后端
4. **Keep-Alive**：减少连接建立开销

### 路由规则

| 路径 | 目标服务 | 端口 |
|------|---------|------|
| `/api/auth/*` | auth-service | 8006 |
| `/api/tenants/*` | tenant-service | 8010 |
| `/api/query/*` | query-service | 8007 |
| `/api/topology/*` | topology-engine | 8008 |
| `/api/alerts/*` | alert-engine | 8009 |
| `/api/control/*` | control-plane | 8001 |
| `/health` | 健康检查 | - |

### 配置 SSL/TLS

1. 将 SSL 证书放入 `ssl/` 目录：
   ```bash
   cp your-cert.pem deployments/nginx/ssl/cert.pem
   cp your-key.pem deployments/nginx/ssl/key.pem
   ```

2. 取消 nginx.conf 中 HTTPS server 块的注释

3. 重启 Nginx：
   ```bash
   docker-compose -f docker-compose.prod.yml restart nginx
   ```

## ClickHouse 配置

自定义配置在 `clickhouse/config.d/custom-config.xml`。

### 默认配置

- 数据库名：cloudflow
- 用户：default
- 认证：使用环境变量 CLICKHOUSE_PASSWORD

## 环境变量

启动前确保设置以下必需环境变量：

```bash
# 数据库密码
export CLOUD_FLOW_DB_PASSWORD=your_strong_password

# Redis 密码
export REDIS_PASSWORD=your_redis_password

# ClickHouse 密码
export CLICKHOUSE_PASSWORD=your_clickhouse_password

# Grafana 管理员密码
export GRAFANA_ADMIN_PASSWORD=your_grafana_password

# Elasticsearch 密码（用于 Jaeger）
export ELASTIC_PASSWORD=your_elasticsearch_password

# JWT 私钥（Base64 编码）
export JWT_PRIVATE_KEY=your_base64_encoded_rsa_private_key

# Docker 镜像仓库（可选）
export DOCKER_REGISTRY=your-registry.example.com/
export VERSION=v1.0.0
```

## 快速启动

```bash
# 1. 复制并编辑环境变量
cp .env.example .env
vim .env

# 2. 构建镜像
docker-compose -f docker-compose.prod.yml build

# 3. 启动服务
docker-compose -f docker-compose.prod.yml up -d

# 4. 查看状态
docker-compose -f docker-compose.prod.yml ps

# 5. 查看日志
docker-compose -f docker-compose.prod.yml logs -f
```
