#!/bin/bash
# =============================================================================
# Cloud Flow - Nacos 配置初始化脚本
# =============================================================================
#
# 功能:
#   - 自动上传配置到 Nacos
#   - 支持 dev/test/prod 环境
#   - 支持配置模板渲染
#
# 使用方法:
#   ./init-configs.sh dev    # 初始化开发环境配置
#   ./init-configs.sh test   # 初始化测试环境配置
#   ./init-configs.sh prod   # 初始化生产环境配置
#   ./init-configs.sh all    # 初始化所有环境配置
#
# 环境变量:
#   NACOS_SERVER    Nacos 服务器地址 (默认: localhost:8848)
#   NACOS_NAMESPACE Nacos 命名空间 (默认: public)
#   NACOS_GROUP     Nacos 分组 (默认: DEFAULT_GROUP)
#   NACOS_USERNAME  Nacos 用户名
#   NACOS_PASSWORD  Nacos 密码
# =============================================================================

set -e

# 颜色
readonly RED='\033[0;31m'
readonly GREEN='\033[0;32m'
readonly YELLOW='\033[1;33m'
readonly BLUE='\033[0;34m'
readonly NC='\033[0m'

# 配置
readonly SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
readonly CONFIG_DIR="${SCRIPT_DIR}/configs"

# Nacos 配置
readonly NACOS_SERVER="${NACOS_SERVER:-localhost:8848}"
readonly NACOS_NAMESPACE="${NACOS_NAMESPACE:-public}"
readonly NACOS_GROUP="${NACOS_GROUP:-DEFAULT_GROUP}"
readonly NACOS_USERNAME="${NACOS_USERNAME:-}"
readonly NACOS_PASSWORD="${NACOS_PASSWORD:-}"

# 应用配置
readonly APP_NAME="cloud-flow"

log_info() { echo -e "${BLUE}[INFO]${NC} $1"; }
log_success() { echo -e "${GREEN}[OK]${NC} $1"; }
log_warn() { echo -e "${YELLOW}[WARN]${NC} $1"; }
log_error() { echo -e "${RED}[ERROR]${NC} $1"; }

# 上传配置到 Nacos
upload_config() {
    local env=$1
    local data_id=$2
    local file=$3

    log_info "Uploading ${data_id} (${env})..."

    # 构建 curl 命令
    local curl_cmd="curl -s -X POST"
    
    if [[ -n "${NACOS_USERNAME}" ]]; then
        curl_cmd="${curl_cmd} -u ${NACOS_USERNAME}:${NACOS_PASSWORD}"
    fi

    # 读取文件内容
    local content
    content=$(cat "${file}")

    # URL 编码
    local encoded_content
    encoded_content=$(python3 -c "import urllib.parse; print(urllib.parse.quote('''${content}'''))" 2>/dev/null || echo "${content}")

    # 上传
    local url="http://${NACOS_SERVER}/nacos/v1/cs/configs"
    local params="dataId=${data_id}&group=${NACOS_GROUP}&namespaceId=${NACOS_NAMESPACE}&content=${encoded_content}"

    local response
    response=$(curl -s -X POST "${url}?${params}" 2>/dev/null || echo "false")

    if [[ "${response}" == "true" ]]; then
        log_success "Uploaded ${data_id}"
        return 0
    else
        log_error "Failed to upload ${data_id}: ${response}"
        return 1
    fi
}

# 使用 nacos-cli 上传（如果安装了 nacos-cli）
upload_config_cli() {
    local env=$1
    local data_id=$2
    local file=$3

    if ! command -v nacos-cli &> /dev/null; then
        return 1
    fi

    log_info "Uploading ${data_id} (${env}) via nacos-cli..."

    if nacos-cli config publish \
        --server "${NACOS_SERVER}" \
        --namespace "${NACOS_NAMESPACE}" \
        --group "${NACOS_GROUP}" \
        --data-id "${data_id}" \
        --file "${file}" 2>/dev/null; then
        log_success "Uploaded ${data_id}"
        return 0
    else
        return 1
    fi
}

# 初始化环境配置
init_env() {
    local env=$1

    log_info "=========================================="
    log_info "Initializing ${env} environment configs"
    log_info "=========================================="

    local success=0
    local failed=0

    # 上传各个配置文件
    local configs=(
        "center-service:center-service-${env}.json"
        "database:database-${env}.json"
        "redis:redis-${env}.json"
        "logging:logging-${env}.json"
    )

    for config in "${configs[@]}"; do
        local data_id="${config%%:*}"
        local file="${config##*:}"
        local filepath="${CONFIG_DIR}/${file}"

        if [[ ! -f "${filepath}" ]]; then
            log_warn "Config file not found: ${filepath}"
            continue
        fi

        # 尝试使用 nacos-cli，失败则使用 curl
        if upload_config_cli "${env}" "${APP_NAME}-${data_id}-${env}" "${filepath}"; then
            ((success++))
        elif upload_config "${env}" "${APP_NAME}-${data_id}-${env}" "${filepath}"; then
            ((success++))
        else
            ((failed++))
        fi
    done

    echo ""
    log_success "Environment ${env}: ${success} configs uploaded, ${failed} failed"

    if [[ ${failed} -gt 0 ]]; then
        return 1
    fi
    return 0
}

# 显示帮助
show_help() {
    cat << 'EOF'
Cloud Flow Nacos 配置初始化脚本

使用方法:
  ./init-configs.sh <环境>

环境:
  dev     初始化开发环境配置
  test    初始化测试环境配置
  prod    初始化生产环境配置
  all     初始化所有环境配置

环境变量:
  NACOS_SERVER      Nacos 服务器地址 (默认: localhost:8848)
  NACOS_NAMESPACE   Nacos 命名空间 (默认: public)
  NACOS_GROUP       Nacos 分组 (默认: DEFAULT_GROUP)
  NACOS_USERNAME    Nacos 用户名
  NACOS_PASSWORD    Nacos 密码

示例:
  # 初始化开发环境
  ./init-configs.sh dev

  # 指定 Nacos 服务器
  NACOS_SERVER=nacos.example.com:8848 ./init-configs.sh prod

  # 带认证
  NACOS_USERNAME=admin NACOS_PASSWORD=secret ./init-configs.sh test
EOF
}

# 主函数
main() {
    local env=${1:-help}

    case "${env}" in
        dev|test|prod)
            init_env "${env}"
            ;;
        all)
            log_info "Initializing all environments..."
            init_env "dev" && init_env "test" && init_env "prod"
            ;;
        help|--help|-h)
            show_help
            ;;
        *)
            log_error "Unknown environment: ${env}"
            show_help
            exit 1
            ;;
    esac
}

main "$@"
