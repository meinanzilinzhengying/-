# ==============================================================================
# CloudFlow Data Plane Dockerfile
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
COPY services/data-plane/ /build/services/data-plane/

RUN echo 'module cloud-flow/services/data-plane' > go.mod && \
    echo 'go 1.22' >> go.mod && \
    echo '' >> go.mod && \
    echo 'require (' >> go.mod && \
    echo '	cloud-flow/pkg v0.0.0' >> go.mod && \
    echo '	cloud-flow/services/proto v0.0.0' >> go.mod && \
    echo '	github.com/ClickHouse/clickhouse-go/v2 v2.24.2' >> go.mod && \
    echo '	google.golang.org/grpc v1.80.0' >> go.mod && \
    echo ')' >> go.mod && \
    echo '' >> go.mod && \
    echo 'replace cloud-flow/pkg => ./pkg' >> go.mod && \
    echo 'replace cloud-flow/services/proto => ./services/proto' >> go.mod

WORKDIR /build/services/data-plane

RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 \
    go build -ldflags="-s -w" -o /build/data-plane ./cmd/main.go

FROM alpine:3.20

RUN apk add --no-cache ca-certificates tzdata curl

ENV TZ=Asia/Shanghai
ENV SERVICE_NAME=data-plane

WORKDIR /app

RUN addgroup -g 1000 -S appgroup && \
    adduser -u 1000 -S appuser -G appgroup

COPY --from=builder /build/data-plane /app/data-plane
COPY --from=builder /build/services/data-plane/configs/ /app/configs/ 2>/dev/null || true

USER appuser

EXPOSE 8001 9001 9101

HEALTHCHECK --interval=30s --timeout=5s --start-period=10s --retries=3 \
    CMD curl -f http://localhost:8001/healthz || exit 1

ENTRYPOINT ["/app/data-plane"]
