#!/bin/bash
# cloud-flow-agent 运维脚本
# 提供 start/stop/restart/status/logs/reload/check/diagnose 功能
# 用法: ./agentctl.sh <command> [选项]

set -euo pipefail

# ============================================================================
# 配置
# ============================================================================

SERVICE_NAME="cloud-flow-agent"
CONFIG_FILE="/etc/cloud-flow-agent/config.yaml"
INSTALL_PREFIX="/opt/cloud-flow-agent"
BINARY="${INSTALL_PREFIX}/bin/cloud-flow-agent"
HEALTH_URL="http://localhost:8080/health"
METRICS_URL="http://localhost:9090/metrics"
LOG_DIR="/var/log/cloud-flow-agent"

# 颜色输出
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
CYAN='\033[0;36m'
NC='\033[0m'

info()    { echo -e "${GREEN}[INFO]${NC} $*"; }
warn()    { echo -e "${YELLOW}[WARN]${NC} $*"; }
error()   { echo -e "${RED}[ERROR]${NC} $*" >&2; }
step()    { echo -e "${BLUE}[STEP]${NC} $*"; }
header()  { echo -e "${CYAN}$*${NC}"; }

# ============================================================================
# 命令实现
# ============================================================================

cmd_start() {
    step "启动 ${SERVICE_NAME}..."
    systemctl start "${SERVICE_NAME}" 2>/dev/null || \
        error "启动失败，请检查服务状态: systemctl status ${SERVICE_NAME}"

    # 等待服务启动
    local retries=10
    while [[ ${retries} -gt 0 ]]; do
        if systemctl is-active --quiet "${SERVICE_NAME}" 2>/dev/null; then
            info "${SERVICE_NAME} 已启动"
            return 0
        fi
        sleep 1
        retries=$((retries - 1))
    done

    error "${SERVICE_NAME} 启动超时"
    return 1
}

cmd_stop() {
    step "停止 ${SERVICE_NAME}..."
    systemctl stop "${SERVICE_NAME}" 2>/dev/null || \
        warn "停止命令执行失败"

    # 等待服务停止
    local retries=15
    while [[ ${retries} -gt 0 ]]; do
        if ! systemctl is-active --quiet "${SERVICE_NAME}" 2>/dev/null; then
            info "${SERVICE_NAME} 已停止"
            return 0
        fi
        sleep 1
        retries=$((retries - 1))
    done

    warn "${SERVICE_NAME} 停止超时，可能仍在运行"
    return 1
}

cmd_restart() {
    step "重启 ${SERVICE_NAME}..."
    cmd_stop
    sleep 2
    cmd_start
}

cmd_status() {
    header "=== ${SERVICE_NAME} 服务状态 ==="
    echo ""

    # systemd 状态
    systemctl status "${SERVICE_NAME}" --no-pager -l 2>/dev/null || true

    echo ""
    header "=== 健康检查 ==="

    # 健康检查
    if command -v curl &>/dev/null; then
        local health_response
        health_response=$(curl -s -f "${HEALTH_URL}" 2>/dev/null) && \
            info "健康检查: ${health_response}" || \
            warn "健康检查失败（服务可能未启动或健康端口不正确）"
    else
        warn "curl 未安装，跳过健康检查"
    fi

    echo ""
    header "=== 资源使用 ==="

    # 进程信息
    local pid
    pid=$(pgrep -f "${BINARY}" 2>/dev/null | head -1) || true
    if [[ -n "${pid}" ]]; then
        info "PID: ${pid}"
        if [[ -f "/proc/${pid}/status" ]]; then
            local threads vms_rss
            threads=$(grep -i "^threads" /proc/${pid}/status 2>/dev/null | awk '{print $2}' || echo "N/A")
            vms_rss=$(grep -i "^vmrss" /proc/${pid}/status 2>/dev/null | awk '{print $2}' || echo "N/A")
            info "线程数: ${threads}"
            info "内存 (RSS): ${vms_rss} KB"
        fi
    else
        warn "进程未运行"
    fi

    echo ""
    header "=== 运行时间 ==="
    systemctl show "${SERVICE_NAME}" --property=ActiveEnterTimestamp 2>/dev/null || echo "N/A"
}

cmd_logs() {
    local lines="${1:-100}"
    local follow=""

    if [[ "${1:-}" == "-f" || "${1:-}" == "--follow" ]]; then
        follow="-f"
        lines="all"
    fi

    if [[ "${follow}" == "-f" ]]; then
        info "实时跟踪日志 (Ctrl+C 退出)..."
        journalctl -u "${SERVICE_NAME}" -f --no-pager
    else
        journalctl -u "${SERVICE_NAME}" --no-pager -n "${lines}"
    fi
}

cmd_reload() {
    step "重新加载 ${SERVICE_NAME} 配置..."

    # 检查配置文件
    if [[ ! -f "${CONFIG_FILE}" ]]; then
        error "配置文件不存在: ${CONFIG_FILE}"
        return 1
    fi

    # 发送 SIGHUP 信号触发配置重载
    local pid
    pid=$(pgrep -f "${BINARY}" 2>/dev/null | head -1) || true
    if [[ -n "${pid}" ]]; then
        kill -HUP "${pid}" 2>/dev/null || true
        info "已发送 SIGHUP 信号 (PID: ${pid})"
    else
        warn "进程未运行，无法热重载配置"
        warn "请使用 restart 命令: $0 restart"
        return 1
    fi
}

cmd_check() {
    header "=== 系统环境检查 ==="
    echo ""

    # 架构检测
    local arch vendor kernel_version
    arch=$(uname -m)
    kernel_version=$(uname -r)

    info "架构: ${arch}"
    info "内核版本: ${kernel_version}"

    # 芯片厂商
    vendor="unknown"
    if [[ "${arch}" == "aarch64" ]]; then
        if grep -qi "kunpeng\|鲲鹏\|hisilicon" /proc/cpuinfo 2>/dev/null; then
            vendor="kunpeng (鲲鹏)"
        fi
    elif [[ "${arch}" == "x86_64" ]]; then
        if grep -qi "hygon\|dhyana\|海光" /proc/cpuinfo 2>/dev/null; then
            vendor="hygon (海光)"
        elif grep -q "GenuineIntel" /proc/cpuinfo 2>/dev/null; then
            vendor="intel"
        elif grep -q "AuthenticAMD" /proc/cpuinfo 2>/dev/null; then
            vendor="amd"
        fi
    fi
    info "芯片厂商: ${vendor}"

    echo ""
    header "=== 依赖检查 ==="

    # 二进制文件
    if [[ -x "${BINARY}" ]]; then
        info "二进制文件: ${BINARY} (OK)"
    else
        error "二进制文件: ${BINARY} (NOT FOUND)"
    fi

    # 配置文件
    if [[ -f "${CONFIG_FILE}" ]]; then
        info "配置文件: ${CONFIG_FILE} (OK)"
    else
        warn "配置文件: ${CONFIG_FILE} (NOT FOUND)"
    fi

    # eBPF 支持
    if [[ -d "/sys/fs/bpf" ]]; then
        info "eBPF 文件系统: /sys/fs/bpf (OK)"
    else
        warn "eBPF 文件系统: /sys/fs/bpf (NOT FOUND)"
    fi

    # BTF 支持
    if [[ -f "/sys/kernel/btf/vmlinux" ]]; then
        info "BTF: /sys/kernel/btf/vmlinux (OK)"
    else
        warn "BTF: /sys/kernel/btf/vmlinux (NOT FOUND, eBPF 可能使用 CO-RE 模式)"
    fi

    # 网络连通性
    echo ""
    header "=== 网络检查 ==="

    local edge_addr
    edge_addr=$(grep -E "^\s*edge_addr:" "${CONFIG_FILE}" 2>/dev/null | awk '{print $2}' | tr -d '"' || echo "")
    if [[ -n "${edge_addr}" ]]; then
        info "Edge 地址: ${edge_addr}"
        # 提取 host:port
        local edge_host edge_port
        edge_host=$(echo "${edge_addr}" | cut -d: -f1)
        edge_port=$(echo "${edge_addr}" | cut -d: -f2)

        if command -v nc &>/dev/null; then
            if nc -z -w 3 "${edge_host}" "${edge_port}" 2>/dev/null; then
                info "Edge 连通性: OK"
            else
                warn "Edge 连通性: FAILED (无法连接 ${edge_host}:${edge_port})"
            fi
        elif command -v timeout &>/dev/null; then
            if timeout 3 bash -c "echo > /dev/tcp/${edge_host}/${edge_port}" 2>/dev/null; then
                info "Edge 连通性: OK"
            else
                warn "Edge 连通性: FAILED (无法连接 ${edge_host}:${edge_port})"
            fi
        fi
    else
        warn "无法从配置文件获取 Edge 地址"
    fi

    echo ""
    header "=== 端口检查 ==="

    # 健康检查端口
    local health_port
    health_port=$(grep -E "^\s*health_port:" "${CONFIG_FILE}" 2>/dev/null | awk '{print $2}' | tr -d '"' || echo "8080")
    if ss -tlnp 2>/dev/null | grep -q ":${health_port} "; then
        info "健康检查端口 ${health_port}: 监听中"
    else
        warn "健康检查端口 ${health_port}: 未监听"
    fi

    # 指标端口
    local metrics_port
    metrics_port=$(grep -E "^\s*metrics_port:" "${CONFIG_FILE}" 2>/dev/null | awk '{print $2}' | tr -d '"' || echo "9090")
    if ss -tlnp 2>/dev/null | grep -q ":${metrics_port} "; then
        info "指标端口 ${metrics_port}: 监听中"
    else
        warn "指标端口 ${metrics_port}: 未监听"
    fi
}

cmd_diagnose() {
    header "=== Cloud Flow Agent 诊断报告 ==="
    echo "生成时间: $(date '+%Y-%m-%d %H:%M:%S')"
    echo ""

    # 系统信息
    header "--- 系统信息 ---"
    echo "主机名: $(hostname)"
    echo "架构: $(uname -m)"
    echo "内核: $(uname -r)"
    echo "OS: $(cat /etc/os-release 2>/dev/null | grep PRETTY_NAME | cut -d= -f2 | tr -d '\"' || uname -s)"
    echo "运行时间: $(uptime -p 2>/dev/null || uptime)"
    echo ""

    # CPU 信息
    header "--- CPU 信息 ---"
    local cpu_model
    cpu_model=$(grep "model name" /proc/cpuinfo 2>/dev/null | head -1 | cut -d: -f2 | xargs || echo "N/A")
    echo "型号: ${cpu_model}"
    echo "核心数: $(nproc 2>/dev/null || echo 'N/A')"
    echo ""

    # 内存信息
    header "--- 内存信息 ---"
    if command -v free &>/dev/null; then
        free -h 2>/dev/null || echo "N/A"
    fi
    echo ""

    # 磁盘信息
    header "--- 磁盘信息 ---"
    if command -v df &>/dev/null; then
        df -h / /var /opt 2>/dev/null || df -h /
    fi
    echo ""

    # 服务状态
    header "--- 服务状态 ---"
    systemctl is-active "${SERVICE_NAME}" 2>/dev/null && echo "状态: 运行中" || echo "状态: 未运行"
    echo ""

    # 最近日志
    header "--- 最近日志 (最后 30 行) ---"
    journalctl -u "${SERVICE_NAME}" --no-pager -n 30 2>/dev/null || echo "无日志"
    echo ""

    # eBPF 状态
    header "--- eBPF 状态 ---"
    if [[ -d "/sys/fs/bpf" ]]; then
        echo "eBPF 挂载点: /sys/fs/bpf"
        ls -la /sys/fs/bpf/ 2>/dev/null || echo "  (空)"
    else
        echo "eBPF 挂载点: 不存在"
    fi
    echo ""

    # 网络连接
    header "--- 网络连接 ---"
    local pid
    pid=$(pgrep -f "${BINARY}" 2>/dev/null | head -1) || true
    if [[ -n "${pid}" ]]; then
        ss -tnp 2>/dev/null | grep "pid=${pid}" || echo "无活跃连接"
    else
        echo "进程未运行"
    fi
}

cmd_version() {
    if [[ -x "${BINARY}" ]]; then
        "${BINARY}" --version 2>/dev/null || \
            info "二进制文件: ${BINARY} (不支持 --version)"
    else
        warn "二进制文件未找到: ${BINARY}"
    fi
}

usage() {
    echo "Cloud Flow Agent 运维工具"
    echo ""
    echo "用法: $0 <command> [选项]"
    echo ""
    echo "命令:"
    echo "  start       启动服务"
    echo "  stop        停止服务"
    echo "  restart     重启服务"
    echo "  status      查看服务状态"
    echo "  logs [N]    查看最近 N 行日志 (默认 100)"
    echo "  logs -f     实时跟踪日志"
    echo "  reload      热重载配置 (发送 SIGHUP)"
    echo "  check       系统环境检查"
    echo "  diagnose    生成诊断报告"
    echo "  version     查看版本"
    echo "  help        显示帮助"
    echo ""
    echo "示例:"
    echo "  $0 start"
    echo "  $0 logs 200"
    echo "  $0 logs -f"
    echo "  $0 check"
    echo "  $0 diagnose"
}

# ============================================================================
# 主入口
# ============================================================================

case "${1:-}" in
    start)
        cmd_start
        ;;
    stop)
        cmd_stop
        ;;
    restart)
        cmd_restart
        ;;
    status)
        cmd_status
        ;;
    logs)
        cmd_logs "${2:-}"
        ;;
    reload)
        cmd_reload
        ;;
    check)
        cmd_check
        ;;
    diagnose)
        cmd_diagnose
        ;;
    version)
        cmd_version
        ;;
    help|--help|-h)
        usage
        ;;
    "")
        usage
        exit 1
        ;;
    *)
        error "未知命令: ${1}"
        echo ""
        usage
        exit 1
        ;;
esac
