# CloudFlow 生产环境安全部署指南

## 概述

本指南提供 CloudFlow 在生产环境中的安全部署最佳实践。

## 1. 密钥管理（最重要）

### 1.1 不要使用 .env 文件存储生产密钥

⚠️ **风险警告**：将密钥存储在 .env 文件中存在严重安全隐患：

- 容器被入侵后密钥全裸奔
- 意外提交到 Git 仓库
- 备份文件中可能泄露
- 共享环境中其他用户可访问

### 1.2 推荐方案：使用密钥管理服务

| 环境 | 推荐方案 |
|------|---------|
| Kubernetes | K8s Secrets / External Secrets Operator |
| AWS | AWS Secrets Manager / AWS Parameter Store |
| Azure | Azure Key Vault |
| GCP | Google Cloud Secret Manager |
| 自托管 | HashiCorp Vault |
| Docker Swarm | Docker Secrets |

### 1.3 Docker Secrets 示例

创建 secrets：

```bash
# 创建数据库密码
echo "your_strong_password_here" | docker secret create cloudflow_db_password -

# 创建 JWT 密钥
openssl rand -base64 32 | docker secret create cloudflow_jwt_secret -

# 创建 Redis 密码
openssl rand -base64 16 | docker secret create cloudflow_redis_password -
```

在 Docker Compose 中使用：

```yaml
services:
  auth-service:
    # ...
    secrets:
      - cloudflow_db_password
      - cloudflow_jwt_secret
    environment:
      - TIDB_PASSWORD_FILE=/run/secrets/cloudflow_db_password
      - JWT_SECRET_FILE=/run/secrets/cloudflow_jwt_secret

secrets:
  cloudflow_db_password:
    external: true
  cloudflow_jwt_secret:
    external: true
```

### 1.4 Kubernetes Secrets 示例

创建 secrets：

```bash
# 创建 secret
kubectl create secret generic cloudflow-secrets \
  --from-literal=db-password=$(openssl rand -base64 16) \
  --from-literal=jwt-secret=$(openssl rand -base64 32) \
  --from-literal=redis-password=$(openssl rand -base64 16)
```

在 Deployment 中使用：

```yaml
apiVersion: apps/v1
kind: Deployment
metadata:
  name: auth-service
spec:
  template:
    spec:
      containers:
      - name: auth-service
        env:
        - name: TIDB_PASSWORD
          valueFrom:
            secretKeyRef:
              name: cloudflow-secrets
              key: db-password
        - name: JWT_SECRET
          valueFrom:
            secretKeyRef:
              name: cloudflow-secrets
              key: jwt-secret
```

## 2. 生成强密钥

### 2.1 使用 OpenSSL 生成安全密钥

```bash
# 生成 16 字节密码（适合数据库）
openssl rand -base64 16

# 生成 32 字节密钥（适合 JWT）
openssl rand -base64 32

# 生成 RSA 密钥对
openssl genrsa -out private.pem 2048
openssl rsa -in private.pem -pubout -out public.pem

# Base64 编码 RSA 私钥
base64 -w 0 private.pem
```

### 2.2 密钥强度要求

| 用途 | 最小长度 | 推荐字符集 |
|------|---------|-----------|
| 数据库密码 | 16 字节 | 字母+数字+符号 |
| JWT 密钥 | 32 字节 | 字母+数字+符号 |
| API 密钥 | 32 字节 | 字母+数字 |
| 管理员密码 | 12 字节 | 字母+数字+符号 |

## 3. 环境变量配置

### 3.1 必需配置项

```bash
# 数据库密码（所有使用 TiDB 的服务）
CLOUD_FLOW_DB_PASSWORD=REPLACE_WITH_STRONG_PASSWORD

# JWT 密钥（Auth Service）
JWT_PRIVATE_KEY=REPLACE_WITH_BASE64_ENCODED_RSA_PRIVATE_KEY

# Redis 密码（缓存/会话存储）
REDIS_PASSWORD=REPLACE_WITH_REDIS_PASSWORD

# API 通信密钥
CLOUD_FLOW_API_KEY=REPLACE_WITH_API_KEY

# ClickHouse 密码
CLICKHOUSE_PASSWORD=REPLACE_WITH_CLICKHOUSE_PASSWORD

# Grafana 管理员密码
GRAFANA_ADMIN_PASSWORD=REPLACE_WITH_GRAFANA_PASSWORD
```

### 3.2 配置验证

启动前验证所有必需密钥已设置：

```bash
#!/bin/bash
REQUIRED_VARS=(
  "CLOUD_FLOW_DB_PASSWORD"
  "REDIS_PASSWORD"
  "JWT_PRIVATE_KEY"
)

for var in "${REQUIRED_VARS[@]}"; do
  if [ -z "${!var}" ] || [[ "${!var}" == "REPLACE_WITH_"* ]]; then
    echo "ERROR: $var is not set or still using placeholder"
    exit 1
  fi
done

echo "All secrets configured properly"
```

## 4. Docker Compose 安全配置

### 4.1 配置安全最佳实践

```yaml
services:
  # 所有服务启用 security_opt
  auth-service:
    # ...
    security_opt:
      - no-new-privileges:true
    read_only: true  # 只读文件系统
    tmpfs:
      - /tmp:rw,noexec,nosuid,size=100m
    cap_drop:
      - ALL
    cap_add:
      - NET_BIND_SERVICE
```

### 4.2 网络安全

```yaml
networks:
  cloudflow-network:
    internal: true  # 仅内部网络访问

# 需要外部访问的服务单独配置
  frontend:
    networks:
      - cloudflow-network
      - public
```

## 5. 密钥轮换策略

### 5.1 定期轮换

| 密钥类型 | 推荐轮换周期 |
|---------|------------|
| 数据库密码 | 每 90 天 |
| JWT 签名密钥 | 每 180 天 |
| API 密钥 | 每 90 天 |
| 管理员密码 | 每 90 天 |
| TLS 证书 | 每 90 天（Let's Encrypt） |

### 5.2 轮换流程示例

```bash
# 1. 生成新密钥
NEW_DB_PASS=$(openssl rand -base64 16)

# 2. 更新密钥管理服务
docker secret create cloudflow_db_password_v2 - <<<"$NEW_DB_PASS"

# 3. 更新应用配置，同时支持旧密钥和新密钥
# （具体取决于应用）

# 4. 滚动更新服务
docker service update --secret-add cloudflow_db_password_v2 auth-service

# 5. 验证新密钥正常工作

# 6. 删除旧密钥
docker secret rm cloudflow_db_password_v1
```

## 6. 审计和监控

### 6.1 密钥访问审计

- 启用密钥管理服务的访问日志
- 监控异常访问模式
- 定期审查密钥访问日志

### 6.2 安全事件响应

建立安全事件响应流程：

1. 检测到密钥泄露
2. 立即撤销受影响的密钥
3. 轮换所有相关密钥
4. 调查泄露原因
5. 实施加固措施
6. 通知相关方

## 7. 开发与生产环境隔离

### 7.1 环境分离

- 使用不同的密钥管理实例
- 生产密钥绝不用于开发
- 开发环境使用独立的数据库/缓存

### 7.2 CI/CD 安全

- CI/CD 中使用临时密钥
- 构建产物中不包含密钥
- 使用加密变量存储 CI/CD 密钥

## 8. 检查清单

部署前确认：

- [ ] 所有密钥使用强随机值
- [ ] 未使用 .env 文件存储生产密钥
- [ ] 使用专业密钥管理服务
- [ ] .gitignore 包含所有敏感文件
- [ ] 密钥已配置轮换策略
- [ ] 访问日志和监控已启用
- [ ] TLS 已正确配置
- [ ] 开发与生产环境完全隔离
- [ ] 安全事件响应流程已建立

## 9. 参考资源

- [Docker Secrets 文档](https://docs.docker.com/engine/swarm/secrets/)
- [Kubernetes Secrets 文档](https://kubernetes.io/docs/concepts/configuration/secret/)
- [HashiCorp Vault](https://www.vaultproject.io/)
- [OWASP 密钥管理指南](https://cheatsheetseries.owasp.org/cheatsheets/Key_Management_Cheat_Sheet.html)
