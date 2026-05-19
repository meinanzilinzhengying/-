#!/bin/bash
# cloud-flow-agent 卸载脚本
# 用法: sudo ./uninstall.sh [选项]
#   --purge    同时删除配置文件和数据

set -euo pipefail

# ============================================================================
# 配置
# ============================================================================

INSTALL_PREFIX="${INSTALL_PREFIX:-/opt/cloud-flow-agent}"
CONFIG_DIR="/etc/cloud-flow-agent"
SERVICE_NAME="cloud-flow-agent"
USER_NAME="cloud-flow"
GROUP_NAME="cloud-flow"
LOG_DIR="/var/log/cloud-flow-agent"
DATA_DIR="/var/lib/cloud-flow-agent"
SYSTEMD_DIR="/etc/systemd/system"

# 颜色输出
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m'

info()  { echo -e "${GREEN}[INFO]${NC} $*"; }
warn()  { echo -e "${YELLOW}[WARN]${NC} $*"; }
error() { echo -e "${RED}[ERROR]${NC} $*" >&2; }

check_root() {
    if [[ $EUID -ne 0 ]]; then
        error "此脚本需要 root 权限运行，请使用 sudo"
        exit 1
    fi
}

# ============================================================================
# 卸载步骤
# ============================================================================

stop_service() {
    if systemctl is-active --quiet "${SERVICE_NAME}" 2>/dev/null; then
        echo "正在停止 ${SERVICE_NAME} 服务..."
        systemctl stop "${SERVICE_NAME}" || warn "停止服务失败"
    fi
}

disable_service() {
    if systemctl is-enabled --quiet "${SERVICE_NAME}" 2>/dev/null; then
        echo "正在禁用 ${SERVICE_NAME} 服务..."
        systemctl disable "${SERVICE_NAME}" || warn "禁用服务失败"
    fi
}

remove_systemd() {
    local service_file="${SYSTEMD_DIR}/${SERVICE_NAME}.service"
    if [[ -f "${service_file}" ]]; then
        rm -f "${service_file}"
        systemctl daemon-reload
        info "已移除 systemd 服务文件"
    fi
}

remove_binary() {
    if [[ -d "${INSTALL_PREFIX}" ]]; then
        rm -rf "${INSTALL_PREFIX}"
        info "已移除安装目录 ${INSTALL_PREFIX}"
    fi
}

remove_config() {
    local purge=false
    for arg in "$@"; do
        case "${arg}" in
            --purge) purge=true ;;
        esac
    done

    if [[ "${purge}" == "true" ]]; then
        if [[ -d "${CONFIG_DIR}" ]]; then
            rm -rf "${CONFIG_DIR}"
            info "已移除配置目录 ${CONFIG_DIR}"
        fi
    else
        info "保留配置目录 ${CONFIG_DIR}（使用 --purge 删除）"
    fi
}

remove_data() {
    local purge=false
    for arg in "$@"; do
        case "${arg}" in
            --purge) purge=true ;;
        esac
    done

    if [[ "${purge}" == "true" ]]; then
        if [[ -d "${DATA_DIR}" ]]; then
            rm -rf "${DATA_DIR}"
            info "已移除数据目录 ${DATA_DIR}"
        fi
        if [[ -d "${LOG_DIR}" ]]; then
            rm -rf "${LOG_DIR}"
            info "已移除日志目录 ${LOG_DIR}"
        fi
    else
        info "保留数据目录 ${DATA_DIR} 和日志目录 ${LOG_DIR}（使用 --purge 删除）"
    fi
}

remove_user() {
    if id "${USER_NAME}" &>/dev/null; then
        userdel -r "${USER_NAME}" 2>/dev/null || warn "移除用户 ${USER_NAME} 失败"
        info "已移除用户 ${USER_NAME}"
    fi

    if getent group "${GROUP_NAME}" &>/dev/null; then
        groupdel "${GROUP_NAME}" 2>/dev/null || warn "移除组 ${GROUP_NAME} 失败"
        info "已移除组 ${GROUP_NAME}"
    fi
}

# ============================================================================
# 主流程
# ============================================================================

main() {
    echo "============================================"
    echo "  Cloud Flow Agent 卸载程序"
    echo "============================================"
    echo ""

    check_root

    stop_service
    disable_service
    remove_systemd
    remove_binary
    remove_config "$@"
    remove_data "$@"
    remove_user

    echo ""
    echo "============================================"
    echo "  Cloud Flow Agent 已卸载"
    echo "============================================"
    echo ""
    info "如需完全清除，请运行: sudo $0 --purge"
}

main "$@"
