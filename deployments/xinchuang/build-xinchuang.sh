#!/bin/bash
# Cloud Flow Agent - 信创镜像构建脚本
# 支持多架构、多操作系统构建

set -e

# ==================== 配置 ====================
VERSION="${VERSION:-latest}"
REGISTRY="${REGISTRY:-registry.example.com}"
PROJECT="${PROJECT:-cloud-flow}"
IMAGE_NAME="${IMAGE_NAME:-cloud-flow-agent}"

# 颜色输出
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m'

log_info() {
    echo -e "${GREEN}[INFO]${NC} $1"
}

log_warn() {
    echo -e "${YELLOW}[WARN]${NC} $1"
}

log_error() {
    echo -e "${RED}[ERROR]${NC} $1"
}

# ==================== 构建函数 ====================

# 构建单架构镜像
build_single_arch() {
    local arch=$1
    local os_type=$2
    local dockerfile=$3
    local target=$4
    
    log_info "构建 ${os_type} ${arch} 镜像..."
    
    docker buildx build \
        --platform linux/${arch} \
        --target ${target} \
        --build-arg VERSION=${VERSION} \
        --build-arg BUILD_TIME=$(date -u +"%Y-%m-%dT%H:%M:%SZ") \
        --build-arg GIT_COMMIT=$(git rev-parse --short HEAD 2>/dev/null || echo "unknown") \
        --tag ${REGISTRY}/${PROJECT}/${IMAGE_NAME}:${VERSION}-${os_type}-${arch} \
        --tag ${REGISTRY}/${PROJECT}/${IMAGE_NAME}:${os_type}-${arch} \
        --file ${dockerfile} \
        --push \
        ../..
    
    log_info "构建完成: ${REGISTRY}/${PROJECT}/${IMAGE_NAME}:${VERSION}-${os_type}-${arch}"
}

# 构建多架构镜像
build_multi_arch() {
    local os_type=$1
    local target=$2
    local archs=$3
    
    log_info "构建 ${os_type} 多架构镜像 (架构: ${archs})..."
    
    docker buildx build \
        --platform ${archs} \
        --target ${target} \
        --build-arg VERSION=${VERSION} \
        --build-arg BUILD_TIME=$(date -u +"%Y-%m-%dT%H:%M:%SZ") \
        --build-arg GIT_COMMIT=$(git rev-parse --short HEAD 2>/dev/null || echo "unknown") \
        --tag ${REGISTRY}/${PROJECT}/${IMAGE_NAME}:${VERSION}-${os_type} \
        --tag ${REGISTRY}/${PROJECT}/${IMAGE_NAME}:${os_type} \
        --file deployments/xinchuang/Dockerfile.xinchuang \
        --push \
        .
    
    log_info "构建完成: ${REGISTRY}/${PROJECT}/${IMAGE_NAME}:${VERSION}-${os_type}"
}

# 创建并推送 manifest
create_manifest() {
    local os_type=$1
    local archs=$2
    
    log_info "创建 ${os_type} manifest..."
    
    local manifest_images=""
    for arch in ${archs}; do
        manifest_images="${manifest_images} ${REGISTRY}/${PROJECT}/${IMAGE_NAME}:${VERSION}-${os_type}-${arch}"
    done
    
    docker manifest create \
        ${REGISTRY}/${PROJECT}/${IMAGE_NAME}:${VERSION}-${os_type} \
        ${manifest_images}
    
    docker manifest push ${REGISTRY}/${PROJECT}/${IMAGE_NAME}:${VERSION}-${os_type}
    
    log_info "Manifest 创建完成"
}

# ==================== 主构建流程 ====================

# 检查 buildx
check_buildx() {
    if ! docker buildx version &> /dev/null; then
        log_error "Docker buildx 未安装，请先安装"
        exit 1
    fi
    
    # 创建 buildx builder
    if ! docker buildx inspect multiarch &> /dev/null; then
        log_info "创建 buildx builder..."
        docker buildx create --name multiarch --use
    else
        docker buildx use multiarch
    fi
}

# 构建所有信创镜像
build_all_xinchuang() {
    log_info "开始构建所有信创镜像..."
    
    # 麒麟系统 (x86_64, arm64)
    log_info "========== 构建麒麟系统镜像 =========="
    build_multi_arch "kylin" "kylin" "linux/amd64,linux/arm64"
    
    # 统信UOS (x86_64, arm64)
    log_info "========== 构建统信UOS镜像 =========="
    build_multi_arch "uos" "uos" "linux/amd64,linux/arm64"
    
    # EulerOS (x86_64, arm64)
    log_info "========== 构建EulerOS镜像 =========="
    build_multi_arch "euler" "euler" "linux/amd64,linux/arm64"
    
    # 龙芯 (loongarch64)
    log_info "========== 构建龙芯镜像 =========="
    # 龙芯需要特殊处理，目前使用交叉编译
    build_single_arch "loongarch64" "loongarch" "deployments/xinchuang/Dockerfile.xinchuang" "loongarch"
    
    # 鲲鹏 (arm64)
    log_info "========== 构建鲲鹏镜像 =========="
    build_single_arch "arm64" "kunpeng" "deployments/xinchuang/Dockerfile.xinchuang" "kunpeng"
    
    # 海光 (x86_64)
    log_info "========== 构建海光镜像 =========="
    build_single_arch "amd64" "hygon" "deployments/xinchuang/Dockerfile.xinchuang" "hygon"
    
    log_info "所有信创镜像构建完成！"
}

# 构建默认多架构镜像
build_default() {
    log_info "构建默认多架构镜像..."
    
    docker buildx build \
        --platform linux/amd64,linux/arm64 \
        --target default \
        --build-arg VERSION=${VERSION} \
        --build-arg BUILD_TIME=$(date -u +"%Y-%m-%dT%H:%M:%SZ") \
        --build-arg GIT_COMMIT=$(git rev-parse --short HEAD 2>/dev/null || echo "unknown") \
        --tag ${REGISTRY}/${PROJECT}/${IMAGE_NAME}:${VERSION} \
        --tag ${REGISTRY}/${PROJECT}/${IMAGE_NAME}:latest \
        --file deployments/xinchuang/Dockerfile.xinchuang \
        --push \
        .
    
    log_info "默认镜像构建完成: ${REGISTRY}/${PROJECT}/${IMAGE_NAME}:${VERSION}"
}

# ==================== 命令行参数 ====================

show_help() {
    echo "Cloud Flow Agent 信创镜像构建脚本"
    echo ""
    echo "用法: $0 [命令] [选项]"
    echo ""
    echo "命令:"
    echo "  all         构建所有信创镜像"
    echo "  kylin       构建麒麟系统镜像"
    echo "  uos         构建统信UOS镜像"
    echo "  euler       构建EulerOS镜像"
    echo "  loongarch   构建龙芯镜像"
    echo "  kunpeng     构建鲲鹏镜像"
    echo "  hygon       构建海光镜像"
    echo "  default     构建默认多架构镜像"
    echo "  help        显示帮助信息"
    echo ""
    echo "环境变量:"
    echo "  VERSION     镜像版本 (默认: latest)"
    echo "  REGISTRY    镜像仓库地址 (默认: registry.example.com)"
    echo "  PROJECT     项目名称 (默认: cloud-flow)"
    echo ""
    echo "示例:"
    echo "  VERSION=v1.0.0 ./build-xinchuang.sh all"
    echo "  ./build-xinchuang.sh kylin"
}

# ==================== 主入口 ====================

main() {
    check_buildx
    
    case "${1:-all}" in
        all)
            build_all_xinchuang
            ;;
        kylin)
            build_multi_arch "kylin" "kylin" "linux/amd64,linux/arm64"
            ;;
        uos)
            build_multi_arch "uos" "uos" "linux/amd64,linux/arm64"
            ;;
        euler)
            build_multi_arch "euler" "euler" "linux/amd64,linux/arm64"
            ;;
        loongarch)
            build_single_arch "loongarch64" "loongarch" "deployments/xinchuang/Dockerfile.xinchuang" "loongarch"
            ;;
        kunpeng)
            build_single_arch "arm64" "kunpeng" "deployments/xinchuang/Dockerfile.xinchuang" "kunpeng"
            ;;
        hygon)
            build_single_arch "amd64" "hygon" "deployments/xinchuang/Dockerfile.xinchuang" "hygon"
            ;;
        default)
            build_default
            ;;
        help|--help|-h)
            show_help
            ;;
        *)
            log_error "未知命令: $1"
            show_help
            exit 1
            ;;
    esac
}

main "$@"
