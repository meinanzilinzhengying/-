// =============================================================================
// Cloud Flow - 多架构镜像构建配置 (docker-bake.hcl)
// =============================================================================
//
// 支持架构:
//   - linux/amd64   (Intel/AMD/海光 Hygon)
//   - linux/arm64   (ARM/鲲鹏 Kunpeng/飞腾 Phytium)
//
// 基础镜像:
//   - alpine        轻量级 Alpine Linux
//   - openeuler     华为 openEuler 欧拉系统
//
// 使用方式:
//   docker bakex bake image-default              # 默认 Alpine 多架构
//   docker bakex bake image-openeuler            # openEuler 多架构
//   docker bakex bake image-agent-alpine         # 仅 Agent Alpine
//   docker bakex bake image-all                  # 全部构建
//   docker bakex bake image-kunpeng              # 仅鲲鹏 ARM64
//   docker bakex bake image-hygon                # 仅海光 x86_64
//   docker bakex bake --print                    # 查看构建计划
// =============================================================================

// ======================== 全局变量 ========================

variable "REGISTRY" {
    default = "ghcr.io/meinanzilinzhengying/cloudflow"
}

variable "VERSION" {
    default = "latest"
}

variable "BUILD_TIME" {
    default = ""
}

variable "GIT_COMMIT" {
    default = ""
}

variable "GO_VERSION" {
    default = "1.22"
}

// ======================== 通用函数 ========================

// 生成镜像标签列表
function "tags" {
    params = [component, base]
    result = [
        "${REGISTRY}/${component}:${VERSION}-${base}",
        "${REGISTRY}/${component}:${VERSION}",
    ]
}

// 生成平台列表
function "platform_amd64" {
    result = ["linux/amd64"]
}

function "platform_arm64" {
    result = ["linux/arm64"]
}

function "platform_multi" {
    result = ["linux/amd64", "linux/arm64"]
}

// ======================== 通用构建参数 ========================

group "default" {
    targets = ["image-agent-alpine"]
}

// ======================== Agent 镜像 ========================

group "agent" {
    targets = [
        "image-agent-alpine",
        "image-agent-openeuler",
    ]
}

// Agent - Alpine 多架构 (x86_64 + ARM64)
target "image-agent-alpine" {
    context    = "."
    dockerfile = "Dockerfile"
    platforms  = platform_multi()
    tags       = tags("agent", "alpine")
    args = {
        VERSION    = VERSION
        BUILD_TIME = BUILD_TIME
        GIT_COMMIT = GIT_COMMIT
    }
    labels = {
        "org.opencontainers.image.title"       = "Cloud Flow Agent"
        "org.opencontainers.image.description" = "Cloud Flow Agent - Alpine Multi-Arch"
        "org.opencontainers.image.version"     = VERSION
        "org.opencontainers.image.vendor"      = "Cloud Flow"
        "com.cloudflow.base"                   = "alpine"
        "com.cloudflow.architectures"          = "amd64,arm64"
    }
}

// Agent - openEuler 多架构 (海光 x86_64 + 鲲鹏 ARM64)
target "image-agent-openeuler" {
    context    = "."
    dockerfile = "deployments/xinchuang/Dockerfile.xinchuang"
    platforms  = platform_multi()
    target     = "euler"
    tags       = tags("agent", "openeuler")
    args = {
        VERSION    = VERSION
        BUILD_TIME = BUILD_TIME
        GIT_COMMIT = GIT_COMMIT
    }
    labels = {
        "org.opencontainers.image.title"       = "Cloud Flow Agent (openEuler)"
        "org.opencontainers.image.description" = "Cloud Flow Agent for openEuler - 海光/鲲鹏"
        "org.opencontainers.image.version"     = VERSION
        "org.opencontainers.image.vendor"      = "Cloud Flow"
        "com.cloudflow.base"                   = "openeuler"
        "com.cloudflow.chips"                  = "hygon,kunpeng"
        "com.openeuler.version"                = "22.03-lts"
    }
}

// Agent - 鲲鹏专用 (ARM64)
target "image-agent-kunpeng" {
    context    = "."
    dockerfile = "Dockerfile"
    platforms  = platform_arm64()
    tags = [
        "${REGISTRY}/agent:${VERSION}-kunpeng",
        "${REGISTRY}/agent:kunpeng",
    ]
    args = {
        VERSION    = VERSION
        BUILD_TIME = BUILD_TIME
        GIT_COMMIT = GIT_COMMIT
    }
    labels = {
        "org.opencontainers.image.title"       = "Cloud Flow Agent (Kunpeng)"
        "org.opencontainers.image.description" = "Cloud Flow Agent for Huawei Kunpeng ARM64"
        "org.opencontainers.image.version"     = VERSION
        "org.opencontainers.image.vendor"      = "Cloud Flow"
        "com.huawei.chip"                      = "kunpeng"
        "com.huawei.arch"                      = "arm64"
    }
}

// Agent - 海光专用 (x86_64)
target "image-agent-hygon" {
    context    = "."
    dockerfile = "Dockerfile"
    platforms  = platform_amd64()
    tags = [
        "${REGISTRY}/agent:${VERSION}-hygon",
        "${REGISTRY}/agent:hygon",
    ]
    args = {
        VERSION    = VERSION
        BUILD_TIME = BUILD_TIME
        GIT_COMMIT = GIT_COMMIT
    }
    labels = {
        "org.opencontainers.image.title"       = "Cloud Flow Agent (Hygon)"
        "org.opencontainers.image.description" = "Cloud Flow Agent for Hygon x86_64"
        "org.opencontainers.image.version"     = VERSION
        "org.opencontainers.image.vendor"      = "Cloud Flow"
        "com.hygon.chip"                       = "hygon"
        "com.hygon.arch"                       = "x86_64"
    }
}

// ======================== Center 镜像 ========================

group "center" {
    targets = [
        "image-center-alpine",
        "image-center-openeuler",
    ]
}

// Center - Alpine 多架构
target "image-center-alpine" {
    context    = "."
    dockerfile = "center/Dockerfile"
    platforms  = platform_multi()
    tags       = tags("center", "alpine")
    args = {
        VERSION    = VERSION
        BUILD_TIME = BUILD_TIME
        GIT_COMMIT = GIT_COMMIT
    }
    labels = {
        "org.opencontainers.image.title"       = "Cloud Flow Center"
        "org.opencontainers.image.description" = "Cloud Flow Center - Alpine Multi-Arch"
        "org.opencontainers.image.version"     = VERSION
        "org.opencontainers.image.vendor"      = "Cloud Flow"
        "com.cloudflow.base"                   = "alpine"
        "com.cloudflow.architectures"          = "amd64,arm64"
    }
}

// Center - openEuler 多架构
target "image-center-openeuler" {
    context    = "."
    dockerfile = "center/Dockerfile.openeuler"
    platforms  = platform_multi()
    tags       = tags("center", "openeuler")
    args = {
        VERSION    = VERSION
        BUILD_TIME = BUILD_TIME
        GIT_COMMIT = GIT_COMMIT
    }
    labels = {
        "org.opencontainers.image.title"       = "Cloud Flow Center (openEuler)"
        "org.opencontainers.image.description" = "Cloud Flow Center for openEuler - 海光/鲲鹏"
        "org.opencontainers.image.version"     = VERSION
        "org.opencontainers.image.vendor"      = "Cloud Flow"
        "com.cloudflow.base"                   = "openeuler"
        "com.cloudflow.chips"                  = "hygon,kunpeng"
    }
}

// ======================== Edge 镜像 ========================

group "edge" {
    targets = [
        "image-edge-alpine",
        "image-edge-openeuler",
    ]
}

// Edge - Alpine 多架构
target "image-edge-alpine" {
    context    = "."
    dockerfile = "edge/Dockerfile"
    platforms  = platform_multi()
    tags       = tags("edge", "alpine")
    labels = {
        "org.opencontainers.image.title"       = "Cloud Flow Edge"
        "org.opencontainers.image.description" = "Cloud Flow Edge - Alpine Multi-Arch"
        "org.opencontainers.image.version"     = VERSION
        "org.opencontainers.image.vendor"      = "Cloud Flow"
        "com.cloudflow.base"                   = "alpine"
        "com.cloudflow.architectures"          = "amd64,arm64"
    }
}

// Edge - openEuler 多架构
target "image-edge-openeuler" {
    context    = "."
    dockerfile = "edge/Dockerfile.openeuler"
    platforms  = platform_multi()
    tags       = tags("edge", "openeuler")
    labels = {
        "org.opencontainers.image.title"       = "Cloud Flow Edge (openEuler)"
        "org.opencontainers.image.description" = "Cloud Flow Edge for openEuler - 海光/鲲鹏"
        "org.opencontainers.image.version"     = VERSION
        "org.opencontainers.image.vendor"      = "Cloud Flow"
        "com.cloudflow.base"                   = "openeuler"
        "com.cloudflow.chips"                  = "hygon,kunpeng"
    }
}

// ======================== 国产芯片专用构建组 ========================

group "kunpeng" {
    targets = [
        "image-agent-kunpeng",
    ]
    description = "鲲鹏 ARM64 专用构建"
}

group "hygon" {
    targets = [
        "image-agent-hygon",
    ]
    description = "海光 x86_64 专用构建"
}

// ======================== 基础镜像分组 ========================

group "image-alpine" {
    targets = [
        "image-agent-alpine",
        "image-center-alpine",
        "image-edge-alpine",
    ]
    description = "全部组件 - Alpine 基础镜像"
}

group "image-openeuler" {
    targets = [
        "image-agent-openeuler",
        "image-center-openeuler",
        "image-edge-openeuler",
    ]
    description = "全部组件 - openEuler 欧拉基础镜像"
}

group "image-default" {
    targets = [
        "image-agent-alpine",
        "image-center-alpine",
        "image-edge-alpine",
    ]
    description = "默认构建 - Alpine 多架构"
}

group "image-domestic" {
    targets = [
        "image-agent-openeuler",
        "image-center-openeuler",
        "image-edge-openeuler",
    ]
    description = "国产化构建 - openEuler 多架构"
}

group "image-all" {
    targets = [
        "image-agent-alpine",
        "image-agent-openeuler",
        "image-center-alpine",
        "image-center-openeuler",
        "image-edge-alpine",
        "image-edge-openeuler",
    ]
    description = "全部组件 x 全部基础镜像"
}

// ======================== CI/CD 专用 ========================

group "ci" {
    targets = [
        "image-agent-alpine",
        "image-center-alpine",
        "image-edge-alpine",
    ]
    description = "CI 流水线默认构建目标"
}

// 仅构建不推送（本地验证用）
target "image-validate" {
    context    = "."
    dockerfile = "Dockerfile"
    platforms  = platform_multi()
    output     = ["type=cacheonly"]
    args = {
        VERSION    = VERSION
        BUILD_TIME = BUILD_TIME
        GIT_COMMIT = GIT_COMMIT
    }
}
