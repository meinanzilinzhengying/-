# Cloud Flow Edge - 无状态边缘服务
# 支持多实例运行，所有数据存储在Redis

FROM golang:1.21-alpine AS builder

# 安装构建依赖
RUN apk add --no-cache git ca-certificates tzdata

WORKDIR /build

# 复制go.mod和go.sum
COPY go.mod go.sum ./
RUN go mod download

# 复制源代码
COPY . .

# 构建
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build \
    -ldflags='-w -s -extldflags "-static"' \
    -a -installsuffix cgo \
    -o /build/edge \
    ./cmd/edge

# 运行时镜像
FROM scratch

# 从builder复制证书
COPY --from=builder /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/
COPY --from=builder /usr/share/zoneinfo /usr/share/zoneinfo

# 复制二进制文件
COPY --from=builder /build/edge /edge

# 非root用户运行
USER 65534:65534

# 暴露端口
# 8080 - HTTP API
# 8081 - HTTP健康检查
# 50051 - gRPC服务
# 50052 - gRPC健康检查
EXPOSE 8080 8081 50051 50052

# 健康检查
HEALTHCHECK --interval=10s --timeout=5s --start-period=5s --retries=3 \
    CMD ["/edge", "-health-check"] || exit 1

# 入口点
ENTRYPOINT ["/edge"]
