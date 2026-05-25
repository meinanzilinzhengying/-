#!/bin/bash
# =============================================================================
# Cloud Flow - 一键部署脚本
# =============================================================================
#
# 功能:
#   - 自动创建命名空间、ConfigMap、Secret
#   - 自动部署所有服务 (center, edge, agent)
#   - 检查服务运行状态
#   - 支持滚动更新和回滚
#
# 使用方法:
#   ./deploy.sh install              # 首次安装
#   ./deploy.sh update               # 滚动更新
#   ./deploy.sh rollback [REVISION]  # 回滚到指定版本
#   ./deploy.sh status               # 查看状态
#   ./deploy.sh delete               # 删除所有资源
#   ./deploy.sh logs [COMPONENT]     # 查看日志
#   ./deploy.sh scale [COMPONENT] [REPLICAS]  # 扩缩容
#
# 环境变量:
#   NAMESPACE      - 部署命名空间 (默认: cloud-flow)
#   VERSION        - 镜像版本 (默认: latest)
#   REGISTRY       - 镜像仓库 (默认: ghcr.io/meinanzilinzhengying/cloudflow)
#   DB_PASSWORD    - 数据库密码
#   REDIS_PASSWORD - Redis密码
#   JWT_SECRET     - JWT密钥
# =============================================================================

set -euo pipefail

# 颜色定义
readonly RED='\033[0;31m'
readonly GREEN='\033[0;32m'
readonly YELLOW='\033[1;33m'
readonly BLUE='\033[0;34m'
readonly NC='\033[0m' # No Color

# 配置
readonly SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
readonly K8S_DIR="${SCRIPT_DIR}/deployments/k8s"
readonly NAMESPACE="${NAMESPACE:-cloud-flow}"
readonly VERSION="${VERSION:-latest}"
readonly REGISTRY="${REGISTRY:-ghcr.io/meinanzilinzhengying/cloudflow}"

# 组件列表
readonly COMPONENTS=("center" "edge" "agent")

# =============================================================================
# 工具函数
# =============================================================================

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
    echo -e "${RED}[ERROR]${NC} $1"
}

# 检查 kubectl 是否可用
check_prerequisites() {
    log_info "检查前置条件..."
    
    if ! command -v kubectl &> /dev/null; then
        log_error "kubectl 未安装，请先安装 kubectl"
        exit 1
    fi
    
    if ! kubectl cluster-info &> /dev/null; then
        log_error "无法连接到 Kubernetes 集群，请检查 kubeconfig"
        exit 1
    fi
    
    log_success "前置条件检查通过"
}

# 等待资源就绪
wait_for_resource() {
    local resource_type=$1
    local resource_name=$2
    local namespace=$3
    local timeout=${4:-300}
    
    log_info "等待 ${resource_type}/${resource_name} 就绪..."
    
    if ! kubectl wait --for=condition=ready "${resource_type}/${resource_name}" \
        -n "${namespace}" --timeout="${timeout}s" &> /dev/null; then
        log_error "${resource_type}/${resource_name} 未在 ${timeout} 秒内就绪"
        return 1
    fi
    
    log_success "${resource_type}/${resource_name} 已就绪"
}

# 等待 Deployment 滚动更新完成
wait_for_rollout() {
    local deployment=$1
    local namespace=$2
    local timeout=${3:-300}
    
    log_info "等待 Deployment/${deployment} 滚动更新完成..."
    
    if ! kubectl rollout status "deployment/${deployment}" \
        -n "${namespace}" --timeout="${timeout}s" &> /dev/null; then
        log_error "Deployment/${deployment} 滚动更新失败"
        return 1
    fi
    
    log_success "Deployment/${deployment} 滚动更新完成"
}

# =============================================================================
# 核心功能
# =============================================================================

# 创建 Secret
create_secrets() {
    log_info "创建 Secret..."
    
    # 生成随机密码（如果未设置）
    local db_password="${DB_PASSWORD:-$(openssl rand -base64 32)}"
    local redis_password="${REDIS_PASSWORD:-}"
    local jwt_secret="${JWT_SECRET:-$(openssl rand -base64 64)}"
    local agent_token="${AGENT_TOKEN:-$(openssl rand -base64 32)}"
    local center_token="${CENTER_TOKEN:-$(openssl rand -base64 32)}"
    
    # 创建 Secret
    kubectl create secret generic cloud-flow-secrets \
        -n "${NAMESPACE}" \
        --from-literal=db-password="${db_password}" \
        --from-literal=redis-password="${redis_password}" \
        --from-literal=jwt-secret="${jwt_secret}" \
        --from-literal=agent-token="${agent_token}" \
        --from-literal=center-token="${center_token}" \
        --dry-run=client -o yaml | kubectl apply -f -
    
    log_success "Secret 创建完成"
    
    # 保存密码到文件（仅供参考）
    cat > "${SCRIPT_DIR}/.env.deploy" << EOF
# Cloud Flow 部署配置（自动生成的密码）
# 请妥善保管此文件，不要提交到版本控制
DB_PASSWORD=${db_password}
REDIS_PASSWORD=${redis_password}
JWT_SECRET=${jwt_secret}
AGENT_TOKEN=${agent_token}
CENTER_TOKEN=${center_token}
EOF
    
    log_info "密码已保存到 ${SCRIPT_DIR}/.env.deploy"
}

# 安装/部署
cmd_install() {
    log_info "开始安装 Cloud Flow..."
    
    check_prerequisites
    
    # 创建命名空间
    log_info "创建命名空间 ${NAMESPACE}..."
    kubectl create namespace "${NAMESPACE}" --dry-run=client -o yaml | kubectl apply -f -
    
    # 创建 RBAC
    log_info "创建 RBAC 资源..."
    kubectl apply -f "${K8S_DIR}/base/99-rbac.yaml"
    
    # 创建 ConfigMap
    log_info "创建 ConfigMap..."
    kubectl apply -f "${K8S_DIR}/base/01-configmap.yaml"
    
    # 创建 Secret
    create_secrets
    
    # 部署依赖服务
    log_info "部署依赖服务 (Redis, PostgreSQL)..."
    kubectl apply -f "${K8S_DIR}/base/20-dependencies.yaml"
    
    # 等待依赖服务就绪
    wait_for_resource "statefulset" "redis" "${NAMESPACE}" 120
    wait_for_resource "statefulset" "postgres" "${NAMESPACE}" 120
    
    # 部署核心服务
    log_info "部署核心服务..."
    kubectl apply -k "${K8S_DIR}/base"
    
    # 等待服务就绪
    wait_for_rollout "center" "${NAMESPACE}" 300
    wait_for_rollout "edge" "${NAMESPACE}" 300
    
    # 检查 DaemonSet
    log_info "等待 Agent DaemonSet 就绪..."
    kubectl rollout status daemonset/agent -n "${NAMESPACE}" --timeout=300s
    
    log_success "Cloud Flow 安装完成！"
    cmd_status
}

# 更新
cmd_update() {
    log_info "开始更新 Cloud Flow..."
    
    check_prerequisites
    
    # 设置镜像版本
    log_info "更新镜像版本到 ${VERSION}..."
    kubectl set image deployment/center \
        center="${REGISTRY}/center:${VERSION}" \
        -n "${NAMESPACE}"
    
    kubectl set image deployment/edge \
        edge="${REGISTRY}/edge:${VERSION}" \
        -n "${NAMESPACE}"
    
    kubectl set image daemonset/agent \
        agent="${REGISTRY}/agent:${VERSION}" \
        -n "${NAMESPACE}"
    
    # 等待滚动更新完成
    wait_for_rollout "center" "${NAMESPACE}"
    wait_for_rollout "edge" "${NAMESPACE}"
    
    log_success "Cloud Flow 更新完成！"
    cmd_status
}

# 回滚
cmd_rollback() {
    local component=$1
    local revision=$2
    
    if [[ -z "${component}" ]]; then
        log_error "请指定要回滚的组件: center 或 edge"
        echo "用法: $0 rollback [center|edge] [REVISION]"
        exit 1
    fi
    
    log_info "回滚 ${component}..."
    
    if [[ -n "${revision}" ]]; then
        kubectl rollout undo "deployment/${component}" \
            -n "${NAMESPACE}" \
            --to-revision="${revision}"
    else
        kubectl rollout undo "deployment/${component}" \
            -n "${NAMESPACE}"
    fi
    
    wait_for_rollout "${component}" "${NAMESPACE}"
    log_success "${component} 回滚完成！"
}

# 查看状态
cmd_status() {
    log_info "Cloud Flow 运行状态:"
    echo ""
    
    echo "=== 命名空间 ==="
    kubectl get namespace "${NAMESPACE}" 2>/dev/null || echo "命名空间不存在"
    echo ""
    
    echo "=== Pods ==="
    kubectl get pods -n "${NAMESPACE}" -o wide
    echo ""
    
    echo "=== Services ==="
    kubectl get svc -n "${NAMESPACE}"
    echo ""
    
    echo "=== Deployments ==="
    kubectl get deployment -n "${NAMESPACE}"
    echo ""
    
    echo "=== DaemonSets ==="
    kubectl get daemonset -n "${NAMESPACE}"
    echo ""
    
    echo "=== StatefulSets ==="
    kubectl get statefulset -n "${NAMESPACE}"
    echo ""
    
    # 检查健康状态
    log_info "健康检查:"
    
    # Center 健康检查
    local center_pod
    center_pod=$(kubectl get pods -n "${NAMESPACE}" -l app.kubernetes.io/name=center -o jsonpath='{.items[0].metadata.name}' 2>/dev/null || true)
    if [[ -n "${center_pod}" ]]; then
        if kubectl exec "${center_pod}" -n "${NAMESPACE}" -- wget -qO- http://localhost:8081/health &> /dev/null; then
            log_success "Center: 健康"
        else
            log_error "Center: 不健康"
        fi
    fi
    
    # Edge 健康检查
    local edge_pod
    edge_pod=$(kubectl get pods -n "${NAMESPACE}" -l app.kubernetes.io/name=edge -o jsonpath='{.items[0].metadata.name}' 2>/dev/null || true)
    if [[ -n "${edge_pod}" ]]; then
        if kubectl exec "${edge_pod}" -n "${NAMESPACE}" -- wget -qO- http://localhost:8081/healthz &> /dev/null; then
            log_success "Edge: 健康"
        else
            log_error "Edge: 不健康"
        fi
    fi
}

# 删除
cmd_delete() {
    log_warn "即将删除 Cloud Flow 所有资源！"
    read -p "确认删除? [y/N] " -n 1 -r
    echo
    
    if [[ ! $REPLY =~ ^[Yy]$ ]]; then
        log_info "取消删除"
        exit 0
    fi
    
    log_info "删除 Cloud Flow 资源..."
    kubectl delete -k "${K8S_DIR}/base" --ignore-not-found=true
    kubectl delete -f "${K8S_DIR}/base/20-dependencies.yaml" --ignore-not-found=true
    kubectl delete -f "${K8S_DIR}/base/02-secret.yaml" --ignore-not-found=true
    kubectl delete -f "${K8S_DIR}/base/01-configmap.yaml" --ignore-not-found=true
    kubectl delete -f "${K8S_DIR}/base/99-rbac.yaml" --ignore-not-found=true
    kubectl delete namespace "${NAMESPACE}" --ignore-not-found=true
    
    log_success "Cloud Flow 已删除"
}

# 查看日志
cmd_logs() {
    local component=$1
    
    if [[ -z "${component}" ]]; then
        log_error "请指定组件: center, edge, agent, redis, postgres"
        exit 1
    fi
    
    local pod
    pod=$(kubectl get pods -n "${NAMESPACE}" -l app.kubernetes.io/name="${component}" -o jsonpath='{.items[0].metadata.name}' 2>/dev/null || true)
    
    if [[ -z "${pod}" ]]; then
        log_error "未找到 ${component} 的 Pod"
        exit 1
    fi
    
    log_info "查看 ${component} (${pod}) 日志..."
    kubectl logs -f "${pod}" -n "${NAMESPACE}" --tail=100
}

# 扩缩容
cmd_scale() {
    local component=$1
    local replicas=$2
    
    if [[ -z "${component}" || -z "${replicas}" ]]; then
        log_error "请指定组件和副本数"
        echo "用法: $0 scale [center|edge] [REPLICAS]"
        exit 1
    fi
    
    log_info "扩缩容 ${component} 到 ${replicas} 个副本..."
    kubectl scale "deployment/${component}" --replicas="${replicas}" -n "${NAMESPACE}"
    
    wait_for_rollout "${component}" "${NAMESPACE}"
    log_success "${component} 扩缩容完成"
}

# 显示帮助
cmd_help() {
    cat << EOF
Cloud Flow 一键部署脚本

使用方法:
  $0 <命令> [参数]

命令:
  install                    首次安装 Cloud Flow
  update                     滚动更新到最新版本
  rollback <组件> [版本]     回滚到指定版本
  status                     查看运行状态
  delete                     删除所有资源
  logs <组件>                查看组件日志
  scale <组件> <副本数>      扩缩容
  help                       显示帮助信息

组件:
  center    控制中心服务
  edge      边缘服务
  agent     Agent 守护进程
  redis     Redis 缓存
  postgres  PostgreSQL 数据库

环境变量:
  NAMESPACE       部署命名空间 (默认: cloud-flow)
  VERSION         镜像版本 (默认: latest)
  REGISTRY        镜像仓库
  DB_PASSWORD     数据库密码
  REDIS_PASSWORD  Redis 密码
  JWT_SECRET      JWT 密钥

示例:
  # 安装
  $0 install

  # 更新到 v1.0.0
  VERSION=v1.0.0 $0 update

  # 回滚 center
  $0 rollback center

  # 扩缩容 edge 到 5 个副本
  $0 scale edge 5

  # 查看 center 日志
  $0 logs center
EOF
}

# =============================================================================
# 主函数
# =============================================================================

main() {
    local command=${1:-help}
    shift || true
    
    case "${command}" in
        install)
            cmd_install
            ;;
        update)
            cmd_update
            ;;
        rollback)
            cmd_rollback "$@"
            ;;
        status)
            cmd_status
            ;;
        delete)
            cmd_delete
            ;;
        logs)
            cmd_logs "$@"
            ;;
        scale)
            cmd_scale "$@"
            ;;
        help|--help|-h)
            cmd_help
            ;;
        *)
            log_error "未知命令: ${command}"
            cmd_help
            exit 1
            ;;
    esac
}

main "$@"
