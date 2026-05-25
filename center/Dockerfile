# Cloud Flow Center - 无状态中心控制服务
# 支持多实例部署，所有数据存储在TiDB/Redis

FROM golang:1.22-alpine AS builder

# 安装构建依赖
RUN apk add --no-cache git ca-certificates tzdata

WORKDIR /build

# 复制go mod文件
COPY go.mod go.sum ./
RUN go mod download

# 复制源代码
COPY . .

# 构建
ARG VERSION=dev
ARG BUILD_TIME=
ARG GIT_COMMIT=

RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 \
    go build -ldflags="-s -w \
    -X main.Version=${VERSION} \
    -X main.BuildTime=${BUILD_TIME} \
    -X main.GitCommit=${GIT_COMMIT}" \
    -o /cloud-flow-center \
    ./cmd/center

# 运行镜像
FROM alpine:3.19

# 安装运行时依赖
RUN apk add --no-cache ca-certificates tzdata curl

# 创建非root用户
RUN addgroup -g 1000 cloudflow && \
    adduser -u 1000 -G cloudflow -s /bin/sh -D cloudflow

WORKDIR /app

# 复制二进制文件
COPY --from=builder /cloud-flow-center .

# 设置权限
RUN chown -R cloudflow:cloudflow /app

# 切换用户
USER cloudflow

# 健康检查
HEALTHCHECK --interval=30s --timeout=10s --start-period=10s --retries=3 \
    CMD curl -f http://localhost:8081/health || exit 1

# 暴露端口
EXPOSE 8080 8081 50051

# 环境变量说明
ENV \
    CENTER_ID=center-1 \
    CENTER_MODE=standalone \
    HTTP_ADDR=:8080 \
    HEALTH_PORT=8081 \
    LOG_LEVEL=info \
    LOG_FORMAT=json

# 启动命令
ENTRYPOINT ["/app/cloud-flow-center"]
