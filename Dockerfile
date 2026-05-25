# Cloud Flow Agent Dockerfile
# 多架构支持：linux/amd64 (海光), linux/arm64 (鲲鹏)

# 构建阶段
FROM golang:1.22-alpine AS builder

# 安装构建依赖
RUN apk add --no-cache git make gcc musl-dev linux-headers

# 设置工作目录
WORKDIR /build

# 复制 go.mod 和 go.sum
COPY go.mod go.sum ./
RUN go mod download

# 复制源码
COPY . .

# 构建参数
ARG TARGETARCH
ARG VERSION=dev
ARG BUILD_TIME
ARG GIT_COMMIT

# 构建（TARGETARCH 由 Docker BuildKit 自动注入，支持多架构）
RUN CGO_ENABLED=0 GOOS=linux GOARCH=${TARGETARCH} go build \
    -ldflags "-s -w -X main.version=${VERSION} -X main.buildTime=${BUILD_TIME} -X main.gitCommit=${GIT_COMMIT}" \
    -o /build/cloud-flow-agent ./cmd/agent

# 运行阶段
FROM alpine:3.19

# 安装运行时依赖
RUN apk add --no-cache ca-certificates tzdata

# 创建非 root 用户（可选，eBPF 需要 root）
# RUN adduser -D -g '' appuser

# 设置工作目录
WORKDIR /app

# 复制二进制
COPY --from=builder /build/cloud-flow-agent /app/

# 复制默认配置
COPY configs/agent.yaml /etc/cloud-flow/agent.yaml

# 创建日志目录
RUN mkdir -p /var/log/cloud-flow

# 暴露端口（用于健康检查）
EXPOSE 9090

# 健康检查
HEALTHCHECK --interval=30s --timeout=10s --start-period=5s --retries=3 \
    CMD pgrep cloud-flow-agent || exit 1

# 设置环境变量
ENV CLOUD_FLOW_CONFIG=/etc/cloud-flow/agent.yaml

# 运行
ENTRYPOINT ["/app/cloud-flow-agent"]
CMD ["-config", "/etc/cloud-flow/agent.yaml"]
