#!/bin/bash
# CloudFlow 生产环境部署检查清单
# ==================================
# 运行前请确保：
#   chmod +x scripts/prod-checklist.sh
#   ./scripts/prod-checklist.sh

set -e

RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

echo -e "${YELLOW}=========================================="
echo -e "  CloudFlow 生产环境部署检查清单"
echo -e "==========================================${NC}"
echo ""

checklist_items=(
    "[ ] 检查环境变量配置 (cp .env.example .env 并修改)"
    "[ ] 检查数据库密码强度 (至少 16 字符，包含大小写字母+数字+符号)"
    "[ ] 检查 JWT 密钥强度 (至少 32 字符，随机生成)"
    "[ ] 配置 TLS 证书 (Let's Encrypt 或自签名)"
    "[ ] 配置备份策略 (TiDB + ClickHouse + Redis)"
    "[ ] 配置监控告警 (Prometheus + Alertmanager)"
    "[ ] 配置日志收集 (Loki + Grafana)"
    "[ ] 配置分布式追踪 (Jaeger)"
    "[ ] 配置防火墙/安全组规则 (仅暴露必要端口)"
    "[ ] 配置资源限制 (Docker/K8s CPU/Memory)"
    "[ ] 配置自动扩展策略 (HPA)"
    "[ ] 配置健康检查和自动恢复"
    "[ ] 配置灾难恢复演练计划"
    "[ ] 测试核心业务流程"
    "[ ] 测试高可用故障切换"
    "[ ] 测试回滚流程"
    "[ ] 建立运维文档和 SOP"
    "[ ] 配置 24/7 监控告警通知"
)

echo -e "部署前检查清单："
echo ""
for item in "${checklist_items[@]}"; do
    echo -e "  ${item}"
done
echo ""

echo -e "${YELLOW}=========================================="
echo -e "  快速安全建议"
echo -e "==========================================${NC}"
echo ""
echo "1. 生成强密码（示例）："
echo "   openssl rand -base64 32"
echo ""
echo "2. 生成 JWT 密钥（示例）："
echo "   openssl rand -base64 64"
echo ""
echo "3. 生成自签名 TLS 证书（示例）："
echo "   openssl req -x509 -nodes -days 3650 -newkey rsa:2048 -keyout tls.key -out tls.crt"
echo ""
echo "4. 检查当前环境状态："
echo "   docker-compose -f docker-compose.prod.yml config"
echo ""
echo -e "${GREEN}=========================================="
echo -e "  检查清单执行完成"
echo -e "==========================================${NC}"
