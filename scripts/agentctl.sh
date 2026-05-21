#!/bin/bash
# Cloud Flow Agent 运维脚本

set -e

# 颜色定义
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m'

SERVICE_NAME="cloud-flow-agent"
CONFIG_FILE="/etc/cloud-flow/agent.yaml"

# 显示帮助
show_help() {
    echo "Cloud Flow Agent 运维脚本"
    echo ""
    echo "用法: $0 <命令> [参数]"
    echo ""
    echo "命令:"
    echo "  start       启动服务"
    echo "  stop        停止服务（优雅关闭）"
    echo "  restart     重启服务"
    echo "  status      查看状态"
    echo "  logs        查看日志"
    echo "  reload      热加载配置"
    echo "  config      编辑配置"
    echo "  check       检查系统兼容性"
    echo "  version     查看版本"
    echo "  diagnose    诊断问题"
    echo ""
}

# 启动服务
do_start() {
    echo -e "${BLUE}启动服务...${NC}"
    systemctl start $SERVICE_NAME
    sleep 2
    if systemctl is-active --quiet $SERVICE_NAME; then
        echo -e "${GREEN}服务已启动${NC}"
    else
        echo -e "${RED}服务启动失败${NC}"
        exit 1
    fi
}

# 停止服务（优雅关闭）
do_stop() {
    echo -e "${BLUE}停止服务（优雅关闭）...${NC}"
    systemctl stop $SERVICE_NAME
    echo -e "${GREEN}服务已停止${NC}"
}

# 重启服务
do_restart() {
    echo -e "${BLUE}重启服务...${NC}"
    systemctl restart $SERVICE_NAME
    sleep 2
    if systemctl is-active --quiet $SERVICE_NAME; then
        echo -e "${GREEN}服务已重启${NC}"
    else
        echo -e "${RED}服务重启失败${NC}"
        exit 1
    fi
}

# 查看状态
do_status() {
    systemctl status $SERVICE_NAME --no-pager
}

# 查看日志
do_logs() {
    local lines=${1:-100}
    journalctl -u $SERVICE_NAME -n $lines --no-pager
}

# 热加载配置
do_reload() {
    echo -e "${BLUE}发送热加载信号...${NC}"
    systemctl kill -s HUP $SERVICE_NAME
    echo -e "${GREEN}已发送热加载信号${NC}"
    echo "配置将在下一个采集周期生效"
}

# 编辑配置
do_config() {
    ${EDITOR:-vi} $CONFIG_FILE
    echo ""
    read -p "是否热加载配置? [y/N] " -n 1 -r
    echo
    if [[ $REPLY =~ ^[Yy]$ ]]; then
        do_reload
    fi
}

# 检查系统兼容性
do_check() {
    echo "========================================"
    echo "系统兼容性检查"
    echo "========================================"
    echo ""

    # 检查架构
    echo -e "${BLUE}[1] 系统架构${NC}"
    local arch=$(uname -m)
    case $arch in
        x86_64)
            echo "  架构: $arch (海光/Intel/AMD 兼容)"
            ;;
        aarch64)
            echo "  架构: $arch (鲲鹏/ARM64)"
            ;;
        *)
            echo -e "  ${RED}架构: $arch (不支持)${NC}"
            ;;
    esac
    echo ""

    # 检查内核版本
    echo -e "${BLUE}[2] 内核版本${NC}"
    local kernel=$(uname -r)
    local major=$(echo $kernel | cut -d. -f1)
    local minor=$(echo $kernel | cut -d. -f2)

    echo "  内核: $kernel"

    if [ "$major" -lt 3 ] || ([ "$major" -eq 3 ] && [ "$minor" -lt 10 ]); then
        echo -e "  ${RED}状态: 不满足最低要求 (需要 >= 3.10)${NC}"
    elif [ "$major" -ge 5 ] || ([ "$major" -eq 4 ] && [ "$minor" -ge 4 ]); then
        echo -e "  ${GREEN}状态: 支持 eBPF 采集${NC}"
    else
        echo -e "  ${YELLOW}状态: 仅支持传统采集模式${NC}"
    fi
    echo ""

    # 检查 BTF 支持
    echo -e "${BLUE}[3] BTF 支持${NC}"
    if [ -f "/sys/kernel/btf/vmlinux" ]; then
        echo -e "  ${GREEN}BTF: 支持${NC}"
    else
        echo -e "  ${YELLOW}BTF: 不支持 (需要内核 >= 5.2)${NC}"
    fi
    echo ""

    # 检查权限
    echo -e "${BLUE}[4] 权限检查${NC}"
    if [ "$EUID" -eq 0 ]; then
        echo -e "  ${GREEN}权限: root (支持 eBPF)${NC}"
    else
        echo -e "  ${YELLOW}权限: 非 root (不支持 eBPF)${NC}"
    fi
    echo ""

    # 检查依赖
    echo -e "${BLUE}[5] 依赖检查${NC}"
    if command -v cloud-flow-agent &> /dev/null; then
        echo -e "  ${GREEN}cloud-flow-agent: 已安装${NC}"
    else
        echo -e "  ${RED}cloud-flow-agent: 未安装${NC}"
    fi
    echo ""

    echo "========================================"
}

# 查看版本
do_version() {
    cloud-flow-agent -version
}

# 诊断问题
do_diagnose() {
    echo "========================================"
    echo "诊断报告"
    echo "========================================"
    echo ""

    # 服务状态
    echo -e "${BLUE}[1] 服务状态${NC}"
    if systemctl is-active --quiet $SERVICE_NAME; then
        echo -e "  ${GREEN}运行中${NC}"
    else
        echo -e "  ${RED}未运行${NC}"
    fi
    echo ""

    # 最近日志
    echo -e "${BLUE}[2] 最近错误日志${NC}"
    journalctl -u $SERVICE_NAME -p err -n 10 --no-pager 2>/dev/null || echo "  无错误日志"
    echo ""

    # 资源使用
    echo -e "${BLUE}[3] 资源使用${NC}"
    if systemctl is-active --quiet $SERVICE_NAME; then
        ps aux | grep cloud-flow-agent | grep -v grep || echo "  无法获取进程信息"
    else
        echo "  服务未运行"
    fi
    echo ""

    # 配置检查
    echo -e "${BLUE}[4] 配置文件${NC}"
    if [ -f "$CONFIG_FILE" ]; then
        echo -e "  ${GREEN}存在: $CONFIG_FILE${NC}"
        # 检查配置语法
        if command -v yamllint &> /dev/null; then
            yamllint $CONFIG_FILE 2>&1 || true
        fi
    else
        echo -e "  ${RED}不存在: $CONFIG_FILE${NC}"
    fi
    echo ""

    echo "========================================"
}

# 主函数
case "$1" in
    start)
        do_start
        ;;
    stop)
        do_stop
        ;;
    restart)
        do_restart
        ;;
    status)
        do_status
        ;;
    logs)
        do_logs $2
        ;;
    reload)
        do_reload
        ;;
    config)
        do_config
        ;;
    check)
        do_check
        ;;
    version)
        do_version
        ;;
    diagnose)
        do_diagnose
        ;;
    *)
        show_help
        exit 1
        ;;
esac
