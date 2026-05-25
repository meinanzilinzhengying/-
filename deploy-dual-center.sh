#!/bin/bash
# =============================================================================
# Cloud Flow - 同城双中心多活部署脚本
# =============================================================================
#
# 功能:
#   - 自动部署双中心（Zone-A + Zone-B）
#   - 数据实时同步（PostgreSQL 流复制 + Redis 复制）
#   - 单中心故障自动切换
#   - 全局查询代理（任一入口查看全局数据）
#   - 双中心负载均衡
#
# 使用方法:
#   ./deploy-dual-center.sh install          # 部署双中心
#   ./deploy-dual-center.sh install-a        # 仅部署中心A
#   ./deploy-dual-center.sh install-b        # 仅部署中心B
#   ./deploy-dual-center.sh status           # 查看双中心状态
#   ./deploy-dual-center.sh failover <zone>  # 手动故障切换
#   ./deploy-dual-center.sh recover <zone>   # 恢复故障中心
#   ./deploy-dual-center.sh update           # 滚动更新双中心
#   ./deploy-dual-center.sh delete           # 删除双中心
#   ./deploy-dual-center.sh sync-status      # 查看同步状态
#   ./deploy-dual-center.sh test-failover    # 测试故障切换
#
# 环境变量:
#   VERSION           - 镜像版本 (默认: latest)
#   REGISTRY          - 镜像仓库
#   DB_PASSWORD       - 数据库密码
#   REDIS_PASSWORD    - Redis密码
#   REPL_PASSWORD     - 复制用户密码
#   ZONE_A_CONTEXT    - 中心A K8s 上下文
#   ZONE_B_CONTEXT    - 中心B K8s 上下文
# =============================================================================

set -euo pipefail

# 颜色
readonly RED='\033[0;31m'
readonly GREEN='\033[0;32m'
readonly YELLOW='\033[1;33m'
readonly BLUE='\033[0;34m'
readonly CYAN='\033[0;36m'
readonly NC='\033[0m'

# 配置
readonly SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
readonly DUAL_CENTER_DIR="${SCRIPT_DIR}/deployments/k8s/dual-center"
readonly VERSION="${VERSION:-latest}"
readonly REGISTRY="${REGISTRY:-ghcr.io/meinanzilinzhengying/cloudflow}"

# 双中心命名空间
readonly NS_A="cloud-flow-zone-a"
readonly NS_B="cloud-flow-zone-b"

# K8s 上下文
readonly CTX_A="${ZONE_A_CONTEXT:-}"
readonly CTX_B="${ZONE_B_CONTEXT:-}"

# =============================================================================
# 工具函数
# =============================================================================

log_info()    { echo -e "${BLUE}[INFO]${NC} $1"; }
log_success() { echo -e "${GREEN}[OK]${NC} $1"; }
log_warn()    { echo -e "${YELLOW}[WARN]${NC} $1"; }
log_error()   { echo -e "${RED}[ERROR]${NC} $1"; }
log_zone_a()  { echo -e "${CYAN}[ZONE-A]${NC} $1"; }
log_zone_b()  { echo -e "${CYAN}[ZONE-B]${NC} $1"; }

# 在指定上下文执行 kubectl
k8s_a() {
    if [[ -n "${CTX_A}" ]]; then
        kubectl --context="${CTX_A}" "$@"
    else
        kubectl "$@"
    fi
}

k8s_b() {
    if [[ -n "${CTX_B}" ]]; then
        kubectl --context="${CTX_B}" "$@"
    else
        kubectl "$@"
    fi
}

# 检查前置条件
check_prerequisites() {
    log_info "检查前置条件..."

    if ! command -v kubectl &> /dev/null; then
        log_error "kubectl 未安装"
        exit 1
    fi

    # 检查集群连接
    if [[ -n "${CTX_A}" ]]; then
        if ! k8s_a cluster-info &> /dev/null; then
            log_error "无法连接到中心A集群 (${CTX_A})"
            exit 1
        fi
        log_success "中心A集群连接正常"
    fi

    if [[ -n "${CTX_B}" ]]; then
        if ! k8s_b cluster-info &> /dev/null; then
            log_error "无法连接到中心B集群 (${CTX_B})"
            exit 1
        fi
        log_success "中心B集群连接正常"
    fi

    # 单集群双可用区模式
    if [[ -z "${CTX_A}" && -z "${CTX_B}" ]]; then
        log_info "检测到单集群模式（双可用区部署）"
        if ! kubectl cluster-info &> /dev/null; then
            log_error "无法连接到 Kubernetes 集群"
            exit 1
        fi
        log_success "集群连接正常"
    fi
}

# 等待 Deployment 就绪
wait_deployment() {
    local ctx_func=$1
    local ns=$2
    local deploy=$3
    local timeout=${4:-300}

    log_info "等待 ${ns}/${deploy} 就绪..."
    if ! ${ctx_func} rollout status "deployment/${deploy}" -n "${ns}" --timeout="${timeout}s" 2>/dev/null; then
        log_error "${ns}/${deploy} 未在 ${timeout}s 内就绪"
        return 1
    fi
    log_success "${ns}/${deploy} 已就绪"
}

# 等待 StatefulSet 就绪
wait_statefulset() {
    local ctx_func=$1
    local ns=$2
    local sts=$3
    local timeout=${4:-180}

    log_info "等待 ${ns}/${sts} 就绪..."
    if ! ${ctx_func} rollout status "statefulset/${sts}" -n "${ns}" --timeout="${timeout}s" 2>/dev/null; then
        log_error "${ns}/${sts} 未在 ${timeout}s 内就绪"
        return 1
    fi
    log_success "${ns}/${sts} 已就绪"
}

# =============================================================================
# 部署
# =============================================================================

# 创建 Secret
create_secrets() {
    local ctx_func=$1
    local ns=$2
    local secrets_name=$3

    local db_password="${DB_PASSWORD:-$(openssl rand -base64 32)}"
    local redis_password="${REDIS_PASSWORD:-}"
    local jwt_secret="${JWT_SECRET:-$(openssl rand -base64 64)}"
    local repl_password="${REPL_PASSWORD:-$(openssl rand -base64 32)}"

    ${ctx_func} create secret generic "${secrets_name}" \
        -n "${ns}" \
        --from-literal=db-password="${db_password}" \
        --from-literal=redis-password="${redis_password}" \
        --from-literal=jwt-secret="${jwt_secret}" \
        --from-literal=replication-password="${repl_password}" \
        --dry-run=client -o yaml | ${ctx_func} apply -f -

    log_success "${ns}/${secrets_name} 创建完成"
}

# 部署中心A
deploy_zone_a() {
    log_zone_a "开始部署中心A..."

    # 创建命名空间
    k8s_a create namespace "${NS_A}" --dry-run=client -o yaml | k8s_a apply -f -

    # 创建 RBAC
    k8s_a apply -f "${DUAL_CENTER_DIR}/zone-a.yaml" -l app.kubernetes.io/component=rbac

    # 创建 ConfigMap
    k8s_a apply -f "${DUAL_CENTER_DIR}/zone-a.yaml" -l app.kubernetes.io/component=config

    # 创建 Secret
    create_secrets k8s_a "${NS_A}" "dual-center-secrets-a"

    # 部署 etcd
    log_zone_a "部署 etcd..."
    k8s_a apply -f "${DUAL_CENTER_DIR}/zone-a.yaml" -l app=etcd
    wait_statefulset k8s_a "${NS_A}" "etcd-a" 120

    # 部署 PostgreSQL 主库
    log_zone_a "部署 PostgreSQL 主库..."
    k8s_a apply -f "${DUAL_CENTER_DIR}/zone-a.yaml" -l "app=postgres,site=zone-a"
    wait_statefulset k8s_a "${NS_A}" "postgres-a" 180

    # 部署 Redis 主节点
    log_zone_a "部署 Redis 主节点..."
    k8s_a apply -f "${DUAL_CENTER_DIR}/zone-a.yaml" -l "app=redis,site=zone-a"
    wait_statefulset k8s_a "${NS_A}" "redis-a" 120

    # 部署 Center
    log_zone_a "部署 Center..."
    k8s_a apply -f "${DUAL_CENTER_DIR}/zone-a.yaml" -l "app=center,site=zone-a"
    wait_deployment k8s_a "${NS_A}" "center-a" 300

    # 部署 Edge
    log_zone_a "部署 Edge..."
    k8s_a apply -f "${DUAL_CENTER_DIR}/zone-a.yaml" -l "app=edge,site=zone-a"
    wait_deployment k8s_a "${NS_A}" "edge-a" 300

    log_success "中心A 部署完成"
}

# 部署中心B
deploy_zone_b() {
    log_zone_b "开始部署中心B..."

    # 创建命名空间
    k8s_b create namespace "${NS_B}" --dry-run=client -o yaml | k8s_b apply -f -

    # 创建 RBAC
    k8s_b apply -f "${DUAL_CENTER_DIR}/zone-b.yaml" -l app.kubernetes.io/component=rbac

    # 创建 ConfigMap
    k8s_b apply -f "${DUAL_CENTER_DIR}/zone-b.yaml" -l app.kubernetes.io/component=config

    # 创建 Secret
    create_secrets k8s_b "${NS_B}" "dual-center-secrets-b"

    # 部署 etcd
    log_zone_b "部署 etcd..."
    k8s_b apply -f "${DUAL_CENTER_DIR}/zone-b.yaml" -l app=etcd
    wait_statefulset k8s_b "${NS_B}" "etcd-b" 120

    # 部署 PostgreSQL 备库
    log_zone_b "部署 PostgreSQL 备库..."
    k8s_b apply -f "${DUAL_CENTER_DIR}/zone-b.yaml" -l "app=postgres,site=zone-b"
    wait_statefulset k8s_b "${NS_B}" "postgres-b" 180

    # 部署 Redis 从节点
    log_zone_b "部署 Redis 从节点..."
    k8s_b apply -f "${DUAL_CENTER_DIR}/zone-b.yaml" -l "app=redis,site=zone-b"
    wait_statefulset k8s_b "${NS_B}" "redis-b" 120

    # 部署 Center
    log_zone_b "部署 Center..."
    k8s_b apply -f "${DUAL_CENTER_DIR}/zone-b.yaml" -l "app=center,site=zone-b"
    wait_deployment k8s_b "${NS_B}" "center-b" 300

    # 部署 Edge
    log_zone_b "部署 Edge..."
    k8s_b apply -f "${DUAL_CENTER_DIR}/zone-b.yaml" -l "app=edge,site=zone-b"
    wait_deployment k8s_b "${NS_B}" "edge-b" 300

    log_success "中心B 部署完成"
}

# 完整部署
cmd_install() {
    log_info "=========================================="
    log_info "  Cloud Flow 同城双中心多活部署"
    log_info "=========================================="

    check_prerequisites

    # 部署中心A（先部署主中心）
    deploy_zone_a

    # 等待中心A完全就绪
    log_info "等待中心A完全就绪..."
    sleep 10

    # 部署中心B
    deploy_zone_b

    # 验证双中心连通性
    verify_connectivity

    log_info "=========================================="
    log_success "  双中心部署完成！"
    log_info "=========================================="
    echo ""
    cmd_status
}

# =============================================================================
# 状态检查
# =============================================================================

cmd_status() {
    echo ""
    echo "=========================================="
    echo "  Cloud Flow 双中心运行状态"
    echo "=========================================="
    echo ""

    # 中心A状态
    echo "--- 中心A (${NS_A}) ---"
    k8s_a get pods -n "${NS_A}" -o wide 2>/dev/null || echo "  不可达"
    echo ""

    # 中心B状态
    echo "--- 中心B (${NS_B}) ---"
    k8s_b get pods -n "${NS_B}" -o wide 2>/dev/null || echo "  不可达"
    echo ""

    # 健康检查
    echo "--- 健康检查 ---"
    check_center_health k8s_a "${NS_A}" "center-a" "Zone-A"
    check_center_health k8s_b "${NS_B}" "center-b" "Zone-B"
    echo ""

    # 数据同步状态
    echo "--- 数据同步 ---"
    check_sync_status
    echo ""

    # 负载均衡状态
    echo "--- 负载均衡 ---"
    check_load_balancer
    echo ""
}

check_center_health() {
    local ctx_func=$1
    local ns=$2
    local deploy=$3
    local label=$4

    local ready
    ready=$(${ctx_func} get deployment "${deploy}" -n "${ns}" -o jsonpath='{.status.readyReplicas}' 2>/dev/null || echo "0")
    local desired
    desired=$(${ctx_func} get deployment "${deploy}" -n "${ns}" -o jsonpath='{.spec.replicas}' 2>/dev/null || echo "0")

    if [[ "${ready}" == "${desired}" && "${ready}" != "0" ]]; then
        log_success "${label}: ${ready}/${desired} 就绪"
    else
        log_error "${label}: ${ready}/${desired} 就绪"
    fi
}

check_sync_status() {
    # PostgreSQL 复制状态
    local pg_repl
    pg_repl=$(k8s_a exec -n "${NS_A}" postgres-a-0 -- \
        psql -U cloudflow -d cloudflow -t -c "SELECT client_addr, state, sent_lag FROM pg_stat_replication" 2>/dev/null || echo "无法获取")

    if [[ -n "${pg_repl}" ]]; then
        log_success "PostgreSQL 复制: ${pg_repl}"
    else
        log_warn "PostgreSQL 复制状态未知"
    fi

    # Redis 复制状态
    local redis_repl
    redis_repl=$(k8s_b exec -n "${NS_B}" redis-b-0 -- \
        redis-cli info replication 2>/dev/null | grep "role\|master_link_status\|master_host" || echo "无法获取")

    if [[ -n "${redis_repl}" ]]; then
        log_success "Redis 复制: $(echo "${redis_repl}" | head -3 | tr '\n' ' ')"
    else
        log_warn "Redis 复制状态未知"
    fi
}

check_load_balancer() {
    local lb_a
    lb_a=$(k8s_a get svc cloud-flow-global -n "${NS_A}" -o jsonpath='{.status.loadBalancer.ingress[0].ip}' 2>/dev/null || echo "无外部IP")
    local lb_b
    lb_b=$(k8s_b get svc cloud-flow-global -n "${NS_B}" -o jsonpath='{.status.loadBalancer.ingress[0].ip}' 2>/dev/null || echo "无外部IP")

    echo "  Zone-A LB: ${lb_a}"
    echo "  Zone-B LB: ${lb_b}"
}

# =============================================================================
# 故障切换
# =============================================================================

cmd_failover() {
    local target_zone="${1:-}"

    if [[ -z "${target_zone}" ]]; then
        log_error "请指定切换目标: zone-a 或 zone-b"
        echo "用法: $0 failover <zone-a|zone-b>"
        exit 1
    fi

    log_warn "=========================================="
    log_warn "  手动故障切换到 ${target_zone}"
    log_warn "=========================================="
    read -p "确认执行故障切换? [y/N] " -n 1 -r
    echo
    if [[ ! $REPLY =~ ^[Yy]$ ]]; then
        log_info "取消故障切换"
        exit 0
    fi

    if [[ "${target_zone}" == "zone-b" ]]; then
        # 将中心B提升为主
        log_info "提升 PostgreSQL-B 为主库..."
        k8s_b exec -n "${NS_B}" postgres-b-0 -- touch /tmp/promote 2>/dev/null || \
            k8s_b exec -n "${NS_B}" postgres-b-0 -- pg_ctl promote -D /var/lib/postgresql/data/pgdata 2>/dev/null

        log_info "提升 Redis-B 为主节点..."
        k8s_b exec -n "${NS_B}" redis-b-0 -- redis-cli replicaof no one 2>/dev/null

        log_info "更新中心B角色为 active..."
        k8s_b patch configmap dual-center-config-b -n "${NS_B}" \
            --type merge -p '{"data":{"CENTER_ROLE":"active"}}'

        log_info "更新中心A角色为 standby..."
        k8s_a patch configmap dual-center-config-a -n "${NS_A}" \
            --type merge -p '{"data":{"CENTER_ROLE":"standby"}}'

        # 重启 Center 以应用新角色
        k8s_b rollout restart deployment/center-b -n "${NS_B}"
        k8s_a rollout restart deployment/center-a -n "${NS_A}"

        wait_deployment k8s_b "${NS_B}" "center-b"
        wait_deployment k8s_a "${NS_A}" "center-a"

    elif [[ "${target_zone}" == "zone-a" ]]; then
        log_info "中心A已经是主中心，无需切换"
    else
        log_error "无效的目标: ${target_zone}"
        exit 1
    fi

    log_success "故障切换完成！"
    cmd_status
}

# 恢复故障中心
cmd_recover() {
    local zone="${1:-}"

    if [[ -z "${zone}" ]]; then
        log_error "请指定恢复目标: zone-a 或 zone-b"
        exit 1
    fi

    log_info "恢复 ${zone}..."

    if [[ "${zone}" == "zone-a" ]]; then
        # 将中心A恢复为备库
        log_info "重建 PostgreSQL-A 为备库..."
        k8s_a exec -n "${NS_A}" postgres-a-0 -- bash -c "
            pg_ctl stop -D /var/lib/postgresql/data/pgdata -m fast
            rm -rf /var/lib/postgresql/data/pgdata/*
            pg_basebackup -h postgres-b.cloud-flow-zone-b.svc.cluster.local \
                -U replicator -D /var/lib/postgresql/data/pgdata -Fp -Xs -P -R
            pg_ctl start -D /var/lib/postgresql/data/pgdata
        " 2>/dev/null || log_warn "PostgreSQL 恢复需要手动操作"

        log_info "重建 Redis-A 为从节点..."
        k8s_a exec -n "${NS_A}" redis-a-0 -- redis-cli replicaof \
            redis-b.cloud-flow-zone-b.svc.cluster.local 6379 2>/dev/null

        log_info "更新中心A角色为 standby..."
        k8s_a patch configmap dual-center-config-a -n "${NS_A}" \
            --type merge -p '{"data":{"CENTER_ROLE":"standby"}}'

        k8s_a rollout restart deployment/center-a -n "${NS_A}"
        wait_deployment k8s_a "${NS_A}" "center-a"

    elif [[ "${zone}" == "zone-b" ]]; then
        log_info "重建 PostgreSQL-B 为备库..."
        k8s_b exec -n "${NS_B}" postgres-b-0 -- bash -c "
            pg_ctl stop -D /var/lib/postgresql/data/pgdata -m fast
            rm -rf /var/lib/postgresql/data/pgdata/*
            pg_basebackup -h postgres-a.cloud-flow-zone-a.svc.cluster.local \
                -U replicator -D /var/lib/postgresql/data/pgdata -Fp -Xs -P -R
            pg_ctl start -D /var/lib/postgresql/data/pgdata
        " 2>/dev/null || log_warn "PostgreSQL 恢复需要手动操作"

        log_info "重建 Redis-B 为从节点..."
        k8s_b exec -n "${NS_B}" redis-b-0 -- redis-cli replicaof \
            redis-a.cloud-flow-zone-a.svc.cluster.local 6379 2>/dev/null

        log_info "更新中心B角色为 standby..."
        k8s_b patch configmap dual-center-config-b -n "${NS_B}" \
            --type merge -p '{"data":{"CENTER_ROLE":"standby"}}'

        k8s_b rollout restart deployment/center-b -n "${NS_B}"
        wait_deployment k8s_b "${NS_B}" "center-b"
    fi

    log_success "${zone} 恢复完成"
}

# 验证连通性
verify_connectivity() {
    log_info "验证双中心连通性..."

    # 检查跨中心网络
    local result
    result=$(k8s_a exec -n "${NS_A}" deployment/center-a -- \
        wget -qO- --timeout=5 "http://center-b.${NS_B}.svc.cluster.local:8081/health" 2>/dev/null || echo "FAIL")

    if [[ "${result}" == *"ok"* || "${result}" == *"SERVING"* ]]; then
        log_success "双中心网络连通"
    else
        log_warn "双中心网络可能不通，请检查网络策略"
    fi
}

# 滚动更新
cmd_update() {
    log_info "滚动更新双中心..."

    local version="${VERSION:-latest}"

    # 更新中心A
    log_zone_a "更新中心A..."
    k8s_a set image deployment/center-a center="${REGISTRY}/center:${version}" -n "${NS_A}"
    k8s_a set image deployment/edge-a edge="${REGISTRY}/edge:${version}" -n "${NS_A}"
    wait_deployment k8s_a "${NS_A}" "center-a"
    wait_deployment k8s_a "${NS_A}" "edge-a"

    # 更新中心B
    log_zone_b "更新中心B..."
    k8s_b set image deployment/center-b center="${REGISTRY}/center:${version}" -n "${NS_B}"
    k8s_b set image deployment/edge-b edge="${REGISTRY}/edge:${version}" -n "${NS_B}"
    wait_deployment k8s_b "${NS_B}" "center-b"
    wait_deployment k8s_b "${NS_B}" "edge-b"

    log_success "双中心更新完成"
}

# 同步状态
cmd_sync_status() {
    echo "=========================================="
    echo "  数据同步状态"
    echo "=========================================="
    echo ""

    echo "--- PostgreSQL 流复制 ---"
    k8s_a exec -n "${NS_A}" postgres-a-0 -- \
        psql -U cloudflow -d cloudflow -c "
        SELECT 
            client_addr as replica_host,
            state,
            sync_state,
            sent_lsn,
            replay_lsn,
            pg_wal_lsn_diff(sent_lsn, replay_lsn) as lag_bytes
        FROM pg_stat_replication;
    " 2>/dev/null || echo "无法获取 PostgreSQL 复制状态"

    echo ""
    echo "--- Redis 复制 ---"
    k8s_a exec -n "${NS_A}" redis-a-0 -- redis-cli info replication 2>/dev/null | \
        grep -E "role|connected_slaves|slave[0-9]|master_link_status" || echo "无法获取 Redis 复制状态"

    echo ""
    echo "--- etcd 集群 ---"
    k8s_a exec -n "${NS_A}" etcd-a-0 -- \
        etcdctl --endpoints=http://localhost:2379 endpoint health --write-out=table 2>/dev/null || \
        echo "无法获取 etcd 状态"
}

# 测试故障切换
cmd_test_failover() {
    log_warn "=========================================="
    log_warn "  故障切换测试"
    log_warn "=========================================="
    echo ""
    echo "此操作将:"
    echo "  1. 模拟中心A故障（缩容到0）"
    echo "  2. 验证中心B自动接管"
    echo "  3. 恢复中心A"
    echo ""
    read -p "确认执行? [y/N] " -n 1 -r
    echo
    if [[ ! $REPLY =~ ^[Yy]$ ]]; then
        exit 0
    fi

    log_info "步骤1: 模拟中心A故障..."
    k8s_a scale deployment/center-a --replicas=0 -n "${NS_A}"
    k8s_a scale deployment/edge-a --replicas=0 -n "${NS_A}"
    sleep 5

    log_info "步骤2: 验证中心B状态..."
    local ready
    ready=$(k8s_b get deployment center-b -n "${NS_B}" -o jsonpath='{.status.readyReplicas}' 2>/dev/null)
    if [[ "${ready}" -gt 0 ]]; then
        log_success "中心B 正常运行 (${ready} 副本)"
    else
        log_error "中心B 异常！"
    fi

    log_info "步骤3: 恢复中心A..."
    k8s_a scale deployment/center-a --replicas=2 -n "${NS_A}"
    k8s_a scale deployment/edge-a --replicas=3 -n "${NS_A}"
    wait_deployment k8s_a "${NS_A}" "center-a"

    log_success "故障切换测试完成"
}

# 删除
cmd_delete() {
    log_warn "即将删除双中心所有资源！"
    read -p "确认删除? [y/N] " -n 1 -r
    echo
    if [[ ! $REPLY =~ ^[Yy]$ ]]; then
        exit 0
    fi

    log_info "删除中心B..."
    k8s_b delete -f "${DUAL_CENTER_DIR}/zone-b.yaml" --ignore-not-found=true
    k8s_b delete namespace "${NS_B}" --ignore-not-found=true

    log_info "删除中心A..."
    k8s_a delete -f "${DUAL_CENTER_DIR}/zone-a.yaml" --ignore-not-found=true
    k8s_a delete namespace "${NS_A}" --ignore-not-found=true

    log_success "双中心已删除"
}

# 帮助
cmd_help() {
    cat << 'EOF'
Cloud Flow 同城双中心多活部署脚本

使用方法:
  ./deploy-dual-center.sh <命令> [参数]

命令:
  install              部署双中心（Zone-A + Zone-B）
  install-a            仅部署中心A
  install-b            仅部署中心B
  status               查看双中心运行状态
  failover <zone>      手动故障切换到指定中心
  recover <zone>       恢复指定中心
  update               滚动更新双中心
  sync-status          查看数据同步状态
  test-failover        测试故障切换
  delete               删除双中心
  help                 显示帮助

数据同步:
  PostgreSQL: 流复制 (主从)
  Redis: 异步复制 (主从)
  etcd: Raft 共识 (跨中心)

故障切换:
  - 自动: 基于 etcd 选举 + 健康检查
  - 手动: ./deploy-dual-center.sh failover zone-b

全局查询:
  任一中心入口可查询全局数据
  自动聚合多中心结果

环境变量:
  ZONE_A_CONTEXT    中心A K8s 上下文 (跨集群)
  ZONE_B_CONTEXT    中心B K8s 上下文 (跨集群)
  VERSION           镜像版本
  DB_PASSWORD       数据库密码
  REDIS_PASSWORD    Redis 密码
  REPL_PASSWORD     复制密码

示例:
  # 单集群双可用区部署
  ./deploy-dual-center.sh install

  # 跨集群部署
  ZONE_A_CONTEXT=cluster-a ZONE_B_CONTEXT=cluster-b ./deploy-dual-center.sh install

  # 手动故障切换
  ./deploy-dual-center.sh failover zone-b

  # 查看同步状态
  ./deploy-dual-center.sh sync-status
EOF
}

# =============================================================================
# 主函数
# =============================================================================

main() {
    local command=${1:-help}
    shift || true

    case "${command}" in
        install)         cmd_install ;;
        install-a)       check_prerequisites; deploy_zone_a ;;
        install-b)       check_prerequisites; deploy_zone_b ;;
        status)          cmd_status ;;
        failover)        cmd_failover "$@" ;;
        recover)         cmd_recover "$@" ;;
        update)          cmd_update ;;
        sync-status)     cmd_sync_status ;;
        test-failover)   cmd_test_failover ;;
        delete)          cmd_delete ;;
        help|--help|-h)  cmd_help ;;
        *)
            log_error "未知命令: ${command}"
            cmd_help
            exit 1
            ;;
    esac
}

main "$@"
