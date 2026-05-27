#!/bin/bash
# =============================================================================
# CloudFlow 超级一键安装脚本
# =============================================================================
# 使用方式（一条命令搞定所有）：
#   curl -fsSL https://raw.githubusercontent.com/meinanzilinzhengying/cloudflow/main/scripts/install.sh | bash
#
# 或下载后执行：
#   chmod +x install.sh && ./install.sh
# =============================================================================

set -e

# 颜色定义
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
PURPLE='\033[0;35m'
CYAN='\033[0;36m'
BOLD='\033[1m'
NC='\033[0m' # No Color

# 配置
GITHUB_REPO="https://github.com/meinanzilinzhengying/cloudflow.git"
PROJECT_DIR="/opt/cloudflow"
INSTALL_LOG="/tmp/cloudflow-install.log"

# 打印函数
print_banner() {
    echo -e "${CYAN}"
    echo "╔════════════════════════════════════════════════════════════════════════╗"
    echo "║                                                                        ║"
    echo "║   ${BOLD}██████╗ ██████╗  ██████╗ ████████╗ ██████╗  ██████╗ ██████╗ ██╗    ${CYAN}║"
    echo "║   ${BOLD}██╔══██╗██╔══██╗██╔═══██╗╚══██╔══╝██╔═══██╗██╔════╝██╔═══██╗██║    ${CYAN}║"
    echo "║   ${BOLD}██████╔╝██████╔╝██║   ██║   ██║   ██║   ██║██║     ██║   ██║██║    ${CYAN}║"
    echo "║   ${BOLD}██╔═══╝ ██╔══██╗██║   ██║   ██║   ██║   ██║██║     ██║   ██║██║    ${CYAN}║"
    echo "║   ${BOLD}██║     ██║  ██║╚██████╔╝   ██║   ╚██████╔╝╚██████╗╚██████╔╝███████╗${CYAN}║"
    echo "║   ${BOLD}╚═╝     ╚═╝  ╚═╝ ╚═════╝    ╚═╝    ╚═════╝  ╚═════╝ ╚═════╝ ╚══════╝${CYAN}║"
    echo "║                                                                        ║"
    echo "║   ${BOLD}云内流量可观测平台 - 超级一键安装脚本 v1.0${NC}                           ║"
    echo "║                                                                        ║"
    echo "╚════════════════════════════════════════════════════════════════════════╝"
    echo -e "${NC}"
}

print_step() {
    echo -e "${BLUE}[$1/$2]${NC} ${BOLD}$3${NC}"
    echo "[$(date '+%Y-%m-%d %H:%M:%S')] $3" >> "$INSTALL_LOG"
}

print_success() {
    echo -e "${GREEN}✅ $1${NC}"
    echo "[$(date '+%Y-%m-%d %H:%M:%S')] ✅ $1" >> "$INSTALL_LOG"
}

print_warning() {
    echo -e "${YELLOW}⚠️  $1${NC}"
    echo "[$(date '+%Y-%m-%d %H:%M:%S')] ⚠️ $1" >> "$INSTALL_LOG"
}

print_error() {
    echo -e "${RED}❌ $1${NC}"
    echo "[$(date '+%Y-%m-%d %H:%M:%S')] ❌ $1" >> "$INSTALL_LOG"
}

print_info() {
    echo -e "${PURPLE}ℹ️  $1${NC}"
    echo "[$(date '+%Y-%m-%d %H:%M:%S')] ℹ️ $1" >> "$INSTALL_LOG"
}

# 检测是否为 root 用户
check_root() {
    if [ "$EUID" -ne 0 ]; then
        print_warning "非 root 用户，正在提升权限..."
        if ! sudo -n true 2>/dev/null; then
            print_info "请输入 sudo 密码："
        fi
        exec sudo bash "$0" "$@"
        exit 1
    fi
}

# 检测操作系统
detect_os() {
    print_step "1" "6" "检测操作系统..."

    if [ -f /etc/os-release ]; then
        . /etc/os-release
        OS_NAME="$NAME"
        OS_VERSION="$VERSION_ID"
    else
        OS_NAME="Unknown"
        OS_VERSION="Unknown"
    fi

    # 检测包管理器
    if command -v apt-get &>/dev/null; then
        PKG_MANAGER="apt-get"
    elif command -v yum &>/dev/null; then
        PKG_MANAGER="yum"
    elif command -v dnf &>/dev/null; then
        PKG_MANAGER="dnf"
    else
        print_error "无法检测支持的包管理器 (apt/yum/dnf)"
        exit 1
    fi

    print_success "检测到: $OS_NAME $OS_VERSION (使用 $PKG_MANAGER)"
}

# 安装基础依赖
install_dependencies() {
    print_step "2" "6" "安装系统基础依赖..."

    update_system() {
        print_info "更新软件包索引..."
        case $PKG_MANAGER in
            apt-get)
                apt-get update -qq
                ;;
            yum)
                yum makecache
                ;;
            dnf)
                dnf makecache
                ;;
        esac
        print_success "软件包索引已更新"
    }

    install_pkg() {
        print_info "安装基础工具包..."
        case $PKG_MANAGER in
            apt-get)
                apt-get install -y -qq curl wget git ca-certificates gnupg lsb-release lsof unzip net-tools
                ;;
            yum)
                yum install -y -q curl wget git ca-certificates redhat-lsb lsof unzip net-tools
                ;;
            dnf)
                dnf install -y -q curl wget git ca-certificates redhat-lsb lsof unzip net-tools
                ;;
        esac
        print_success "基础工具包安装完成"
    }

    update_system
    install_pkg
}

# 安装 Docker
install_docker() {
    print_step "3" "6" "安装 Docker..."

    if command -v docker &>/dev/null; then
        print_success "Docker 已安装: $(docker --version)"
        # 确保 Docker 服务运行
        systemctl start docker 2>/dev/null || true
        systemctl enable docker 2>/dev/null || true
        return
    fi

    print_info "正在安装 Docker..."

    case $PKG_MANAGER in
        apt-get)
            # 添加 Docker 官方 GPG 密钥
            mkdir -p /etc/apt/keyrings
            curl -fsSL https://download.docker.com/linux/ubuntu/gpg | gpg --dearmor -o /etc/apt/keyrings/docker.gpg

            # 添加 Docker 仓库
            echo "deb [arch=$(dpkg --print-architecture) signed-by=/etc/apt/keyrings/docker.gpg] https://download.docker.com/linux/ubuntu $(lsb_release -cs) stable" | tee /etc/apt/sources.list.d/docker.list > /dev/null

            # 安装 Docker
            apt-get update -qq
            apt-get install -y -qq docker-ce docker-ce-cli containerd.io docker-buildx-plugin docker-compose-plugin
            ;;
        yum)
            # 添加 Docker 仓库
            yum install -y -q yum-utils
            yum-config-manager --add-repo https://download.docker.com/linux/centos/docker-ce.repo

            # 安装 Docker
            yum install -y -q docker-ce docker-ce-cli containerd.io docker-buildx-plugin docker-compose-plugin
            ;;
        dnf)
            # 添加 Docker 仓库
            dnf config-manager --add-repo https://download.docker.com/linux/centos/docker-ce.repo

            # 安装 Docker
            dnf install -y -q docker-ce docker-ce-cli containerd.io docker-buildx-plugin docker-compose-plugin
            ;;
    esac

    # 启动 Docker
    systemctl start docker
    systemctl enable docker

    # 验证安装
    if command -v docker &>/dev/null; then
        print_success "Docker 安装成功: $(docker --version)"
    else
        print_error "Docker 安装失败"
        exit 1
    fi
}

# 拉取或更新代码
pull_code() {
    print_step "4" "6" "拉取 CloudFlow 代码..."

    if [ -d "$PROJECT_DIR" ]; then
        print_warning "项目目录已存在，正在更新..."
        cd "$PROJECT_DIR"
        git pull
        print_success "代码已更新"
    else
        print_info "正在克隆仓库..."
        git clone "$GITHUB_REPO" "$PROJECT_DIR"
        cd "$PROJECT_DIR"
        print_success "代码克隆完成"
    fi
}

# 创建精简配置
create_light_config() {
    print_step "5" "6" "创建精简版部署配置..."

    cd "$PROJECT_DIR"

    cat > docker-compose.light.yml << 'EOF'
# =============================================================================
# CloudFlow 精简部署配置（适合 4GB 以下内存的服务器）
# 警告：仅用于开发/测试环境！
# =============================================================================

services:
  # ---------------------------------------------------------------------------
  # TiDB 数据库（轻量模式）
  # ---------------------------------------------------------------------------
  tidb:
    image: pingcap/tidb:v8.4.0
    container_name: cloud-flow-tidb
    restart: unless-stopped
    command: ["tidb-server", "--skip-grant-table", "--instance-id=tidb1"]
    ports:
      - "127.0.0.1:4000:4000"
      - "127.0.0.1:10080:10080"
    environment:
      TZ: Asia/Shanghai
    volumes:
      - tidb-data:/var/lib/tidb
    healthcheck:
      test: ["CMD", "curl", "-f", "http://localhost:10080/status"]
      interval: 10s
      timeout: 5s
      retries: 10
      start_period: 30s
    networks:
      - cloud-flow-net
    deploy:
      resources:
        limits:
          memory: 512M
        reservations:
          memory: 256M

  # ---------------------------------------------------------------------------
  # Redis (轻量模式)
  # ---------------------------------------------------------------------------
  redis:
    image: redis:7-alpine
    container_name: cloud-flow-redis
    restart: unless-stopped
    ports:
      - "127.0.0.1:6379:6379"
    volumes:
      - redis-data:/data
    command: redis-server --maxmemory 128mb --maxmemory-policy allkeys-lru
    healthcheck:
      test: ["CMD", "redis-cli", "ping"]
      interval: 10s
      timeout: 5s
      retries: 5
      start_period: 5s
    networks:
      - cloud-flow-net
    deploy:
      resources:
        limits:
          memory: 256M

  # ---------------------------------------------------------------------------
  # 中心服务 (Center)
  # ---------------------------------------------------------------------------
  center:
    build:
      context: ./cloud-flow-center
      dockerfile: deployments/Dockerfile
    container_name: cloud-flow-center
    restart: unless-stopped
    depends_on:
      tidb:
        condition: service_healthy
      redis:
        condition: service_healthy
    ports:
      - "9090:8080"
      - "9999:9090"
    environment:
      TZ: Asia/Shanghai
      CLOUD_FLOW_DSN: "root:@tcp(tidb:4000)/cloud_flow?parseTime=true&charset=utf8mb4&collation=utf8mb4_bin"
      CLOUD_FLOW_JWT_SECRET: "cloudflow-dev-jwt-secret-key-$(date +%s)"
      CLOUD_FLOW_ADMIN_PASSWORD: "admin123"
      METRICS_PORT: "9191"
      CLOUD_FLOW_REDIS_ADDR: "redis://redis:6379"
    volumes:
      - center-config:/app/configs
      - center-data:/app/data
    healthcheck:
      test: ["CMD", "wget", "--spider", "-q", "http://localhost:8080/health"]
      interval: 15s
      timeout: 5s
      retries: 5
      start_period: 20s
    networks:
      - cloud-flow-net
    deploy:
      resources:
        limits:
          memory: 1G

  # ---------------------------------------------------------------------------
  # 前端 (Frontend)
  # ---------------------------------------------------------------------------
  frontend:
    build:
      context: ./cloud-flow-frontend
      dockerfile: deployments/Dockerfile
      args:
        VITE_API_BASE_URL: "/api"
    container_name: cloud-flow-frontend
    restart: unless-stopped
    depends_on:
      center:
        condition: service_healthy
    ports:
      - "8888:80"
    healthcheck:
      test: ["CMD", "wget", "--spider", "-q", "http://localhost:80/health"]
      interval: 15s
      timeout: 5s
      retries: 3
      start_period: 10s
    networks:
      - cloud-flow-net

volumes:
  tidb-data:
  redis-data:
  center-config:
  center-data:

networks:
  cloud-flow-net:
    driver: bridge
EOF

    # 创建环境变量文件
    cat > .env << 'EOF'
# CloudFlow 环境变量（自动生成）
CLOUD_FLOW_JWT_SECRET=cloudflow-dev-jwt-secret-key
CLOUD_FLOW_ADMIN_PASSWORD=admin123
REDIS_PASSWORD=
EOF

    # 给脚本添加执行权限
    chmod +x scripts/deploy.sh 2>/dev/null || true

    print_success "精简配置已创建"
}

# 一键部署
deploy() {
    print_step "6" "6" "开始一键部署..."

    cd "$PROJECT_DIR"

    print_info "正在构建 Docker 镜像（首次可能需要 5-10 分钟）..."
    docker compose -f docker-compose.light.yml build --no-cache

    print_info "正在启动所有服务..."
    docker compose -f docker-compose.light.yml up -d

    print_success "部署命令已执行"
}

# 等待服务就绪
wait_for_services() {
    echo ""
    print_info "正在等待服务启动，请稍候..."

    local max_wait=120
    local waited=0

    while [ $waited -lt $max_wait ]; do
        # 检查容器是否都在运行
        running=$(docker compose -f docker-compose.light.yml ps --format json 2>/dev/null | grep -c "running" || echo 0)

        if [ "$running" -ge 3 ]; then
            break
        fi

        echo -ne "${YELLOW}  等待中... ${waited}s/${max_wait}s\r${NC}"
        sleep 5
        waited=$((waited + 5))
    done

    echo ""
}

# 打印部署结果
print_result() {
    echo ""
    echo -e "${GREEN}"
    echo "╔════════════════════════════════════════════════════════════════════════╗"
    echo "║                      🎉 部署完成！                                      ║"
    echo "╚════════════════════════════════════════════════════════════════════════╝"
    echo -e "${NC}"

    # 获取服务器 IP
    SERVER_IP=$(hostname -I | awk '{print $1}')
    if [ -z "$SERVER_IP" ]; then
        SERVER_IP="localhost"
    fi

    echo -e "${BOLD}📋 服务访问地址：${NC}"
    echo ""
    echo -e "  ${CYAN}━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━${NC}"
    echo -e "  ${GREEN}Center Portal${NC}:  ${BOLD}http://${SERVER_IP}:9090${NC}"
    echo -e "  ${GREEN}Frontend${NC}:       ${BOLD}http://${SERVER_IP}:8888${NC}"
    echo -e "  ${CYAN}━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━${NC}"
    echo ""
    echo -e "${BOLD}🔐 默认登录账号：${NC}"
    echo ""
    echo -e "  用户名: ${CYAN}admin${NC}"
    echo -e "  密码:   ${CYAN}admin123${NC}"
    echo ""
    echo -e "${BOLD}📝 常用命令：${NC}"
    echo ""
    echo -e "  ${YELLOW}# 进入项目目录${NC}"
    echo -e "  cd $PROJECT_DIR"
    echo ""
    echo -e "  ${YELLOW}# 查看服务状态${NC}"
    echo -e "  docker compose -f docker-compose.light.yml ps"
    echo ""
    echo -e "  ${YELLOW}# 查看日志${NC}"
    echo -e "  docker compose -f docker-compose.light.yml logs -f"
    echo ""
    echo -e "  ${YELLOW}# 停止服务${NC}"
    echo -e "  docker compose -f docker-compose.light.yml stop"
    echo ""
    echo -e "  ${YELLOW}# 启动服务${NC}"
    echo -e "  docker compose -f docker-compose.light.yml start"
    echo ""
    echo -e "  ${YELLOW}# 清理所有数据${NC}"
    echo -e "  docker compose -f docker-compose.light.yml down -v"
    echo ""
    echo -e "${RED}⚠️  注意：此配置仅用于开发/测试环境！${NC}"
    echo ""
}

# 主函数
main() {
    # 初始化日志
    mkdir -p "$(dirname "$INSTALL_LOG")"
    echo "CloudFlow 安装日志 - $(date)" > "$INSTALL_LOG"

    print_banner

    echo -e "${BOLD}${PURPLE}开始自动安装 CloudFlow...${NC}"
    echo ""

    check_root "$@"
    detect_os
    install_dependencies
    install_docker
    pull_code
    create_light_config
    deploy
    wait_for_services
    print_result

    echo ""
    print_info "安装日志已保存到: $INSTALL_LOG"
    echo ""
}

# 运行主函数
main "$@"
