# =============================================================================
# 云内流量监测系统 (Cloud Flow Frontend) - 优化版多阶段构建 Dockerfile
# 优化点:
#   - BuildKit 缓存挂载加速 npm install
#   - 更小的基础镜像 (nginx:alpine-slim)
#   - 移除不必要的包
#   - 安全加固 (非 root 运行)
# =============================================================================

# syntax=docker/dockerfile:1.7

# ---------------------------------------------------------------------------
# Stage 1: 依赖安装 - 利用缓存层避免重复安装
# ---------------------------------------------------------------------------
FROM node:20-alpine AS deps

WORKDIR /app

# 启用 Corepack 管理 pnpm/yarn
RUN corepack enable

# 利用 BuildKit 缓存挂载加速 npm ci
# 挂载 npm 缓存到主机，避免每次构建都重新下载
COPY package.json package-lock.json ./
RUN --mount=type=cache,target=/root/.npm,sharing=locked \
    npm ci --prefer-offline --no-audit --no-fund

# ---------------------------------------------------------------------------
# Stage 2: 构建阶段
# ---------------------------------------------------------------------------
FROM node:20-alpine AS builder

WORKDIR /app

# 构建参数
ARG VITE_API_BASE_URL=/api
ENV VITE_API_BASE_URL=${VITE_API_BASE_URL}

# 从 deps 阶段复制 node_modules
COPY --from=deps /app/node_modules ./node_modules
COPY . .

# 构建生产版本
RUN npm run build

# ---------------------------------------------------------------------------
# Stage 3: 部署阶段 - 使用最小化的 nginx 镜像
# ---------------------------------------------------------------------------
FROM nginx:alpine-slim AS production

# 设置时区
ENV TZ=Asia/Shanghai

# 安装必要依赖并清理
RUN apk add --no-cache tzdata curl && \
    rm -rf /var/cache/apk/* /tmp/*

# 移除默认配置
RUN rm -rf /usr/share/nginx/html/* /etc/nginx/conf.d/default.conf

# 复制构建产物
COPY --from=builder /app/dist /usr/share/nginx/html

# 复制 Nginx 配置
COPY deployments/nginx.conf /etc/nginx/conf.d/default.conf

# 创建非 root 用户并设置权限
RUN addgroup -g 1000 -S nginx && \
    adduser -u 1000 -S nginx -G nginx && \
    chown -R nginx:nginx /usr/share/nginx/html && \
    chown -R nginx:nginx /var/cache/nginx && \
    chown -R nginx:nginx /var/log/nginx && \
    touch /var/run/nginx.pid && \
    chown nginx:nginx /var/run/nginx.pid

# 健康检查
HEALTHCHECK --interval=30s --timeout=3s --start-period=5s --retries=3 \
    CMD curl -f http://localhost:80/health || exit 1

USER nginx

EXPOSE 80

CMD ["nginx", "-g", "daemon off;"]
