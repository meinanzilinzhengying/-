#!/bin/bash
# CloudFlow 启动脚本 - 解决服务依赖顺序问题
#
# 使用方式:
#   ./start.sh          # 启动所有服务
#   ./start.sh -b       # 先启动基础服务，再启动业务服务
#   ./start.sh -c       # 先清理再启动
#   ./start.sh -h       # 查看帮助

set -eo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(dirname "$SCRIPT_DIR")"
cd "$PROJECT_ROOT"

# 颜色输出
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

# 配置选项
CLEAN_FIRST=false
BASE_ONLY=false

print_usage() {
    echo "CloudFlow 启动脚本 - 解决服务启动顺序依赖"
    echo ""
    echo "Usage: $0 [选项]"
    echo ""
    echo "选项:"
    echo "  -c, --clean       先清理所有数据再启动"
    echo "  -b, --base-only   只启动基础服务（数据库、消息队列等）"
    echo "  -h, --help        显示帮助信息"
    echo ""
    echo "示例:"
    echo "  $0              # 完整启动"
    echo "  $0 -c           # 清理后启动"
    echo "  $0 -b           # 仅启动基础服务"
    echo ""
}

parse_args() {
    while [[ $# -gt 0 ]]; do
        case "$1" in
            -c|--clean)
            CLEAN_FIRST=true
            shift
            ;;
            -b|--base-only)
            BASE_ONLY=true
            shift
            ;;
            -h|--help)
            print_usage
            exit 0
            ;;
            *)
            echo "Unknown option: $1" >&2
            print_usage
            exit 1
            ;;
        esac
    done
}

log_info() {
    echo -e "${BLUE}[INFO]${NC} $1"
}

log_success() {
    echo -e "${GREEN}[SUCCESS]${NC} $1"
}

log_warn() {
    echo -e "${YELLOW}[WARN]${NC} $1"
}

log_error() {
    echo -e "${RED}[ERROR]${NC} $1" >&2
}

check_dependencies() {
    log_info "检查 Docker Compose 是否可用..."
    
    if ! command -v docker &> /dev/null; then
        log_error "Docker 未安装或未启动"
        exit 1
    fi

    # 检测 Docker Compose 命令
    COMPOSE_CMD=""
    if docker compose version &> /dev/null; then
        COMPOSE_CMD="docker compose"
    elif command -v docker-compose &> /dev/null; then
        COMPOSE_CMD="docker-compose"
    else
        log_error "Docker Compose 未安装"
        exit 1
    fi
    
    log_success "使用命令: $COMPOSE_CMD"
    export COMPOSE_CMD
}

cleanup_services() {
    if [ "$CLEAN_FIRST" = true ]; then
        log_warn "清理现有服务和数据..."
        $COMPOSE_CMD down -v --remove-orphans
        log_success "清理完成"
    fi
}

start_base_services() {
    log_info "正在启动基础服务 (阶段 1/2)..."
    
    # 第一阶段：启动基础设施层（数据库、缓存等）
    local base_services=(
        "tidb"
        "redis"
        "etcd"
        "clickhouse"
        "zookeeper"
        "kafka"
    )
    
    for service in "${base_services[@]}"; do
        log_info "启动 $service..."
        $COMPOSE_CMD up -d "$service"
    done
    
    log_info "等待基础服务健康检查通过..."
    sleep 10
    
    # 等待所有基础服务就绪
    for service in "${base_services[@]}"; do
        wait_for_service "$service"
    done
    
    log_success "所有基础服务已就绪!"
}

wait_for_service() {
    local service_name=$1
    local max_attempts=30
    local attempt=0
    
    log_info "等待 $service_name 就绪..."
    
    while [ $attempt -lt $max_attempts ]; do
        if $COMPOSE_CMD ps --format "{{.Status}}" "$service_name" 2>/dev/null | grep -q "healthy"; then
            log_success "$service_name 已就绪!"
            return 0
        fi
        attempt=$((attempt + 1))
        sleep 3
    done
    
    log_error "$service_name 等待超时，继续尝试..."
    return 1
}

start_business_services() {
    log_info "正在启动业务服务 (阶段 2/2)..."
    
    # 第二阶段：启动业务服务
    local business_services=(
        "auth-service"
        "tenant-service"
        "control-plane"
        "data-plane"
        "query-service"
        "topology-engine"
        "alert-engine"
    )
    
    for service in "${business_services[@]}"; do
        log_info "启动 $service..."
        $COMPOSE_CMD up -d "$service"
        sleep 3  # 给服务一些时间
    done
    
    log_info "等待业务服务健康检查通过..."
    
    for service in "${business_services[@]}"; do
        wait_for_service "$service"
    done
    
    log_success "所有业务服务已就绪!"
}

start_monitoring_services() {
    log_info "正在启动监控服务 (可选)..."
    
    local monitor_services=(
        "prometheus"
        "grafana"
        "jaeger"
        "loki"
        "victoriametrics"
        "promtail"
    )
    
    for service in "${monitor_services[@]}"; do
        log_info "启动 $service..."
        $COMPOSE_CMD up -d "$service"
    done
    
    log_success "监控服务已启动!"
}

print_status() {
    log_info ""
    log_info "----------------------------------------------------------------------"
    log_info "服务状态概览:"
    log_info "----------------------------------------------------------------------"
    $COMPOSE_CMD ps --format "table {{.Names}}\t{{.Status}}\t{{.Ports}}"
    log_info "----------------------------------------------------------------------"
    log_info ""
    log_info "访问地址:"
    log_info "  Auth Service:  http://localhost:8006"
    log_info "  Control Plane: http://localhost:8001"
    log_info "  Query Service: http://localhost:8007"
    log_info ""
    log_info "监控面板:"
    log_info "  Grafana:    http://localhost:3001"
    log_info "  Prometheus: http://localhost:9091"
    log_info "  Jaeger:     http://localhost:16686"
    log_info ""
    log_info "查看日志:"
    log_info "  $COMPOSE_CMD logs -f [服务名]"
    log_info ""
}

main() {
    parse_args "$@"
    check_dependencies
    
    log_info "========================================="
    log_info "  CloudFlow 启动脚本"
    log_info "========================================="
    
    cleanup_services
    start_base_services
    
    if [ "$BASE_ONLY" = true ]; then
        log_success "基础服务启动完成!"
        print_status
        exit 0
    fi
    
    start_business_services
    start_monitoring_services
    
    log_success "所有服务启动完成!"
    print_status
}

main "$@"
