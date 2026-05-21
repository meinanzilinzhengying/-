#!/bin/bash
# Cloud Flow Agent 安装脚本
# 支持：x86_64 (海光), aarch64 (鲲鹏)

set -e

# 颜色定义
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

# 版本
VERSION="${VERSION:-latest}"
INSTALL_DIR="/usr/local/bin"
CONFIG_DIR="/etc/cloud-flow"
LOG_DIR="/var/log/cloud-flow"
DATA_DIR="/var/lib/cloud-flow"

# 检测架构
detect_arch() {
    local arch=$(uname -m)
    case $arch in
        x86_64)
            echo "amd64"
            ;;
        aarch64)
            echo "arm64"
            ;;
        *)
            echo -e "${RED}不支持的架构: $arch${NC}"
            exit 1
            ;;
    esac
}

# 检测内核版本
detect_kernel() {
    local kernel=$(uname -r)
    local major=$(echo $kernel | cut -d. -f1)
    local minor=$(echo $kernel | cut -d. -f2)

    echo "内核版本: $kernel"

    if [ "$major" -lt 3 ] || ([ "$major" -eq 3 ] && [ "$minor" -lt 10 ]); then
        echo -e "${RED}错误: 内核版本过低，需要 >= 3.10${NC}"
        exit 1
    fi

    if [ "$major" -ge 4 ] && [ "$minor" -ge 4 ]; then
        echo -e "${GREEN}支持 eBPF 采集${NC}"
    else
        echo -e "${YELLOW}不支持 eBPF，将使用传统采集模式${NC}"
    fi
}

# 检查 root 权限
check_root() {
    if [ "$EUID" -ne 0 ]; then
        echo -e "${RED}请使用 root 权限运行此脚本${NC}"
        exit 1
    fi
}

# 创建目录
create_dirs() {
    echo "创建目录..."
    mkdir -p "$CONFIG_DIR"
    mkdir -p "$LOG_DIR"
    mkdir -p "$DATA_DIR"
}

# 安装二进制
install_binary() {
    local arch=$(detect_arch)
    local binary_name="cloud-flow-agent-${arch}"

    echo "安装二进制文件 ($arch)..."

    if [ -f "./bin/$binary_name" ]; then
        cp "./bin/$binary_name" "$INSTALL_DIR/cloud-flow-agent"
    elif [ -f "./cloud-flow-agent" ]; then
        cp "./cloud-flow-agent" "$INSTALL_DIR/cloud-flow-agent"
    else
        echo -e "${RED}找不到二进制文件: $binary_name${NC}"
        exit 1
    fi

    chmod +x "$INSTALL_DIR/cloud-flow-agent"
}

# 安装配置文件
install_config() {
    echo "安装配置文件..."

    if [ -f "$CONFIG_DIR/agent.yaml" ]; then
        echo -e "${YELLOW}配置文件已存在，备份到 agent.yaml.bak${NC}"
        cp "$CONFIG_DIR/agent.yaml" "$CONFIG_DIR/agent.yaml.bak"
    fi

    if [ -f "./configs/agent.yaml" ]; then
        cp "./configs/agent.yaml" "$CONFIG_DIR/agent.yaml"
    else
        echo -e "${YELLOW}使用默认配置${NC}"
        cat > "$CONFIG_DIR/agent.yaml" << 'EOF'
agent:
  hostname: ""
  host_ip: ""
  interval: 10

edge:
  address: "edge.example.com"
  port: 9090
  tls_enabled: false
  timeout: 10
  retry_max: 5
  retry_delay: 5

collectors:
  ebpf:
    enabled: true
    events: ["tcp_connect", "tcp_accept", "tcp_close"]
    sample_rate: 100
    buffer_size: 4096
  traditional:
    enabled: true
    proc_path: "/proc"
  metrics:
    enabled: true
    interval: 10
    cpu: true
    memory: true
    disk: true
    network: true
  process:
    enabled: true
    events: ["exec", "fork", "exit"]

resources:
  cpu_quota: 1.0
  memory_limit: 512
  buffer_max_size: 100
  max_goroutines: 100

logging:
  level: "info"
  format: "json"
  output: "stdout"
EOF
    fi
}

# 安装 systemd 服务
install_systemd() {
    echo "安装 systemd 服务..."

    if [ -f "./deployments/systemd/cloud-flow-agent.service" ]; then
        cp "./deployments/systemd/cloud-flow-agent.service" /etc/systemd/system/
    else
        cat > /etc/systemd/system/cloud-flow-agent.service << 'EOF'
[Unit]
Description=Cloud Flow Agent
After=network-online.target
Wants=network-online.target

[Service]
Type=simple
User=root
ExecStart=/usr/local/bin/cloud-flow-agent -config /etc/cloud-flow/agent.yaml
ExecReload=/bin/kill -HUP $MAINPID
Restart=on-failure
RestartSec=5
LimitNOFILE=65535

[Install]
WantedBy=multi-user.target
EOF
    fi

    systemctl daemon-reload
    systemctl enable cloud-flow-agent
}

# 启动服务
start_service() {
    echo "启动服务..."
    systemctl start cloud-flow-agent

    sleep 2

    if systemctl is-active --quiet cloud-flow-agent; then
        echo -e "${GREEN}服务启动成功${NC}"
    else
        echo -e "${RED}服务启动失败，请检查日志${NC}"
        journalctl -u cloud-flow-agent -n 20
        exit 1
    fi
}

# 显示状态
show_status() {
    echo ""
    echo "========================================"
    echo -e "${GREEN}Cloud Flow Agent 安装完成${NC}"
    echo "========================================"
    echo ""
    echo "二进制: $INSTALL_DIR/cloud-flow-agent"
    echo "配置:   $CONFIG_DIR/agent.yaml"
    echo "日志:   $LOG_DIR/"
    echo ""
    echo "常用命令:"
    echo "  启动:   systemctl start cloud-flow-agent"
    echo "  停止:   systemctl stop cloud-flow-agent"
    echo "  重启:   systemctl restart cloud-flow-agent"
    echo "  状态:   systemctl status cloud-flow-agent"
    echo "  日志:   journalctl -u cloud-flow-agent -f"
    echo ""
}

# 主函数
main() {
    echo "========================================"
    echo "Cloud Flow Agent 安装脚本"
    echo "========================================"
    echo ""

    check_root
    detect_kernel
    create_dirs
    install_binary
    install_config
    install_systemd
    start_service
    show_status
}

# 运行
main "$@"
