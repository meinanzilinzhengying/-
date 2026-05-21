#!/bin/bash
# Cloud Flow Agent 卸载脚本

set -e

# 颜色定义
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m'

INSTALL_DIR="/usr/local/bin"
CONFIG_DIR="/etc/cloud-flow"
LOG_DIR="/var/log/cloud-flow"
DATA_DIR="/var/lib/cloud-flow"

# 检查 root 权限
if [ "$EUID" -ne 0 ]; then
    echo -e "${RED}请使用 root 权限运行此脚本${NC}"
    exit 1
fi

echo "========================================"
echo "Cloud Flow Agent 卸载脚本"
echo "========================================"
echo ""

# 停止服务
echo "停止服务..."
systemctl stop cloud-flow-agent 2>/dev/null || true
systemctl disable cloud-flow-agent 2>/dev/null || true

# 删除 systemd 服务
echo "删除 systemd 服务..."
rm -f /etc/systemd/system/cloud-flow-agent.service
systemctl daemon-reload

# 删除二进制
echo "删除二进制文件..."
rm -f "$INSTALL_DIR/cloud-flow-agent"

# 询问是否删除配置和数据
read -p "是否删除配置文件? [y/N] " -n 1 -r
echo
if [[ $REPLY =~ ^[Yy]$ ]]; then
    echo "删除配置文件..."
    rm -rf "$CONFIG_DIR"
fi

read -p "是否删除日志文件? [y/N] " -n 1 -r
echo
if [[ $REPLY =~ ^[Yy]$ ]]; then
    echo "删除日志文件..."
    rm -rf "$LOG_DIR"
fi

read -p "是否删除数据文件? [y/N] " -n 1 -r
echo
if [[ $REPLY =~ ^[Yy]$ ]]; then
    echo "删除数据文件..."
    rm -rf "$DATA_DIR"
fi

echo ""
echo -e "${GREEN}卸载完成${NC}"
