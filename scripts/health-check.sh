#!/bin/bash
# CloudFlow 生产环境健康检查脚本
# ===================================
# 功能：检查所有服务的健康状态

RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m'

echo -e "${YELLOW}=========================================="
echo -e "  CloudFlow 生产环境健康检查"
echo -e "==========================================${NC}"
echo ""

# 检查 Docker 是否运行
echo "1. 检查 Docker 状态..."
if docker info > /dev/null 2>&1; then
    echo -e "   ${GREEN}[OK]${NC} Docker 正在运行"
else
    echo -e "   ${RED}[ERROR]${NC} Docker 未运行"
    exit 1
fi

# 检查容器状态
echo ""
echo "2. 检查容器状态..."

services=(
    "tidb"
    "clickhouse"
    "redis"
    "kafka"
    "etcd"
    "auth-service"
    "tenant-service"
    "control-plane"
    "data-plane"
    "query-service"
    "topology-engine"
    "alert-engine"
    "grafana"
    "prometheus"
)

all_healthy=true

for service in "${services[@]}"; do
    container_name="cloudflow-${service}"
    if docker inspect -f '{{.State.Running}}' "${container_name}" > /dev/null 2>&1; then
        health_status=$(docker inspect -f '{{.State.Health.Status}}' "${container_name}" 2>/dev/null || echo "unknown")
        if [ "$health_status" = "healthy" ]; then
            echo -e "   ${GREEN}[OK]${NC} ${service}"
        elif [ "$health_status" = "starting" ]; then
            echo -e "   ${YELLOW}[WARN]${NC} ${service} - 正在启动"
            all_healthy=false
        else
            echo -e "   ${RED}[ERROR]${NC} ${service} - 健康状态: ${health_status}"
            all_healthy=false
        fi
    else
        echo -e "   ${RED}[ERROR]${NC} ${service} - 容器未运行"
        all_healthy=false
    fi
done

# 检查端口可用性
echo ""
echo "3. 检查端口可用性..."

ports=(
    "8006:Auth HTTP"
    "8001:Control Plane HTTP"
    "8007:Query Service HTTP"
    "3000:Grafana"
    "9090:Prometheus"
)

for port_entry in "${ports[@]}"; do
    port=$(echo "$port_entry" | cut -d: -f1)
    desc=$(echo "$port_entry" | cut -d: -f2)
    if nc -z localhost "$port" > /dev/null 2>&1; then
        echo -e "   ${GREEN}[OK]${NC} 端口 ${port} (${desc})"
    else
        echo -e "   ${RED}[ERROR]${NC} 端口 ${port} (${desc}) - 不可访问"
        all_healthy=false
    fi
done

echo ""
echo -e "${YELLOW}=========================================="
if [ "$all_healthy" = true ]; then
    echo -e "  ${GREEN}所有检查通过！系统运行正常${NC}"
else
    echo -e "  ${RED}部分检查失败，请检查日志${NC}"
fi
echo -e "==========================================${NC}"

# 检查日志最近错误
echo ""
echo "4. 检查最近错误日志..."
for service in "${services[@]}"; do
    container_name="cloudflow-${service}"
    if docker ps -q -f name="${container_name}" > /dev/null; then
        errors=$(docker logs --tail=50 "${container_name}" 2>&1 | grep -i "error\|fatal\|panic" | head -5)
        if [ -n "$errors" ]; then
            echo -e "   ${YELLOW}[WARN]${NC} ${service} 发现错误:"
            echo "$errors" | sed 's/^/     /'
        fi
    fi
done
