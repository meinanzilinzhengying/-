# ==============================================================================
# CloudFlow Auth Service Dockerfile
# ==============================================================================
# 多阶段构建，优化镜像大小
# ==============================================================================

# 阶段1: 构建
FROM golang:1.22-alpine AS builder

RUN apk add --no-cache git ca-certificates tzdata

WORKDIR /build

# 设置 Go 代理
ENV GOPROXY=https://goproxy.cn,direct
ENV GO111MODULE=on

# 复制 go.mod 和 go.sum
# 注意：服务使用 cloud-flow/services/* 导入路径，需要从项目根目录构建
COPY pkg/ /build/pkg/
COPY proto/ /build/proto/
COPY services/proto/ /build/services/proto/
COPY services/shared/ /build/services/shared/
COPY services/auth-service/ /build/services/auth-service/

# 创建 go.mod
RUN echo 'module cloud-flow/services/auth-service' > go.mod && \
    echo 'go 1.22' >> go.mod && \
    echo '' >> go.mod && \
    echo 'require (' >> go.mod && \
    echo '	cloud-flow/pkg v0.0.0' >> go.mod && \
    echo '	cloud-flow/services/proto v0.0.0' >> go.mod && \
    echo '	github.com/go-sql-driver/mysql v1.8.1' >> go.mod && \
    echo '	github.com/golang-jwt/jtt/v5 v5.2.0' >> go.mod && \
    echo '	github.com/google/uuid v1.6.0' >> go.mod && \
    echo '	github.com/redis/go-redis/v9 v9.7.0' >> go.mod && \
    echo '	go.etcd.io/etcd/client/v3 v3.5.13' >> go.mod && \
    echo '	golang.org/x/crypto v0.47.0' >> go.mod && \
    echo '	google.golang.org/grpc v1.80.0' >> go.mod && \
    echo '	gorm.io/driver/mysql v1.5.6' >> go.mod && \
    echo '	gorm.io/gorm v1.25.10' >> go.mod && \
    echo ')' >> go.mod && \
    echo '' >> go.mod && \
    echo 'replace cloud-flow/pkg => ./pkg' >> go.mod && \
    echo 'replace cloud-flow/services/proto => ./services/proto' >> go.mod

# 复制 replace 指令
RUN echo 'replace cloud-flow/pkg => ./pkg' >> go.mod && \
    echo 'replace cloud-flow/services/proto => ./services/proto' >> go.mod

# 下载依赖
RUN go mod download

# 设置工作目录
WORKDIR /build/services/auth-service

# 构建
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 \
    go build -ldflags="-s -w" -o /build/auth-service ./cmd/main.go

# 阶段2: 运行
FROM alpine:3.20

RUN apk add --no-cache ca-certificates tzdata curl

ENV TZ=Asia/Shanghai
ENV SERVICE_NAME=auth-service

WORKDIR /app

# 创建非 root 用户
RUN addgroup -g 1000 -S appgroup && \
    adduser -u 1000 -S appuser -G appgroup

# 复制二进制文件
COPY --from=builder /build/auth-service /app/auth-service

# 复制配置文件（如果存在）
COPY --from=builder /build/services/auth-service/configs/ /app/configs/ 2>/dev/null || true

USER appuser

# 端口
EXPOSE 8006 9006

HEALTHCHECK --interval=30s --timeout=5s --start-period=10s --retries=3 \
    CMD curl -f http://localhost:8006/healthz || exit 1

ENTRYPOINT ["/app/auth-service"]
