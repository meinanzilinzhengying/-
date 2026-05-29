# ==============================================================================
# CloudFlow Auth Service Dockerfile
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
COPY services/auth-service/ /build/services/auth-service/

RUN go mod download

WORKDIR /build/services/auth-service

RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 \
    go build -ldflags="-s -w" -o /build/auth-service ./cmd/main.go

FROM alpine:3.20

RUN apk add --no-cache ca-certificates tzdata curl

ENV TZ=Asia/Shanghai
ENV SERVICE_NAME=auth-service

WORKDIR /app

RUN addgroup -g 1000 -S appgroup && \
    adduser -u 1000 -S appuser -G appgroup

COPY --from=builder /build/auth-service /app/auth-service
COPY --from=builder /build/services/auth-service/configs/ /app/configs/ 2>/dev/null || true

USER appuser

EXPOSE 8006 9006

HEALTHCHECK --interval=30s --timeout=5s --start-period=10s --retries=3 \
    CMD curl -f http://localhost:8006/healthz || exit 1

ENTRYPOINT ["/app/auth-service"]
