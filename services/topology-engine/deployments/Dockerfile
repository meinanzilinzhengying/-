# ==============================================================================
# CloudFlow Topology Engine Dockerfile
# ==============================================================================
# 多阶段构建，优化镜像大小
# ==============================================================================

FROM golang:1.22-alpine AS builder

RUN apk add --no-cache git ca-certificates tzdata

WORKDIR /build

ENV GOPROXY=https://goproxy.cn,direct
ENV GO111MODULE=on

COPY pkg/ /build/pkg/
COPY proto/ /build/proto/
COPY services/proto/ /build/services/proto/
COPY services/shared/ /build/services/shared/
COPY services/topology-engine/ /build/services/topology-engine/

RUN echo 'module cloud-flow/services/topology-engine' > go.mod && \
    echo 'go 1.22' >> go.mod && \
    echo '' >> go.mod && \
    echo 'require (' >> go.mod && \
    echo '	cloud-flow/pkg v0.0.0' >> go.mod && \
    echo '	cloud-flow/services/proto v0.0.0' >> go.mod && \
    echo '	github.com/ClickHouse/clickhouse-go/v2 v2.24.2' >> go.mod && \
    echo '	github.com/redis/go-redis/v9 v9.7.0' >> go.mod && \
    echo '	google.golang.org/grpc v1.80.0' >> go.mod && \
    echo ')' >> go.mod && \
    echo '' >> go.mod && \
    echo 'replace cloud-flow/pkg => ./pkg' >> go.mod && \
    echo 'replace cloud-flow/services/proto => ./services/proto' >> go.mod

WORKDIR /build/services/topology-engine

RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 \
    go build -ldflags="-s -w" -o /build/topology-engine ./cmd/main.go

FROM alpine:3.20

RUN apk add --no-cache ca-certificates tzdata curl

ENV TZ=Asia/Shanghai
ENV SERVICE_NAME=topology-engine

WORKDIR /app

RUN addgroup -g 1000 -S appgroup && \
    adduser -u 1000 -S appuser -G appgroup

COPY --from=builder /build/topology-engine /app/topology-engine
COPY --from=builder /build/services/topology-engine/configs/ /app/configs/ 2>/dev/null || true

USER appuser

EXPOSE 8008 9008

HEALTHCHECK --interval=30s --timeout=5s --start-period=10s --retries=3 \
    CMD curl -f http://localhost:8008/healthz || exit 1

ENTRYPOINT ["/app/topology-engine"]
