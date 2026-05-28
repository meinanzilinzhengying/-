#!/bin/bash
# CloudFlow 一键启动脚本

set -e

echo "=========================================="
echo "  CloudFlow 微服务部署脚本"
echo "=========================================="
echo ""

# 检查 Docker
if ! command -v docker &> /dev/null; then
    echo "❌ 错误: Docker 未安装"
    exit 1
fi

if ! command -v docker compose &> /dev/null; then
    echo "❌ 错误: Docker Compose 未安装"
    exit 1
fi

echo "✅ Docker 和 Docker Compose 已安装"

# 检查环境变量文件
if [ ! -f .env ]; then
    echo "📝 创建环境变量文件..."
    cp .env.example .env
    echo "⚠️  请编辑 .env 文件设置数据库密码"
    echo "   默认密码: your_secure_password_here"
    echo ""
    read -p "按 Enter 继续..."
fi

# 检查数据库密码
source .env
if [ "$CLOUD_FLOW_DB_PASSWORD" = "your_secure_password_here" ]; then
    echo "⚠️  警告: 您仍在使用默认密码，请尽快修改 .env 文件"
fi

echo ""
echo "🚀 开始启动 CloudFlow 服务..."
echo ""

# 启动所有服务
docker compose up -d

echo ""
echo "⏳ 等待服务启动..."
sleep 5

# 检查服务状态
echo ""
echo "📊 服务状态:"
docker compose ps

echo ""
echo "✅ CloudFlow 已启动!"
echo ""
echo "访问地址:"
echo "  - API Gateway:    http://localhost:8007"
echo "  - Auth Service:   http://localhost:8006"
echo "  - Control Plane:  http://localhost:8001"
echo "  - Grafana:        http://localhost:3001  (admin/admin)"
echo "  - Prometheus:     http://localhost:9091"
echo "  - Jaeger:         http://localhost:16686"
echo ""
echo "查看日志: docker compose logs -f"
echo "停止服务: docker compose down"
echo ""
