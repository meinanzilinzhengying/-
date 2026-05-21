# Cloud Flow Agent Makefile
# 支持多架构构建：x86_64 (海光), aarch64 (鲲鹏)

# 版本信息
VERSION ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo "dev")
BUILD_TIME ?= $(shell date -u +"%Y-%m-%dT%H:%M:%SZ")
GIT_COMMIT ?= $(shell git rev-parse --short HEAD 2>/dev/null || echo "unknown")

# Go 参数
GOCMD = go
GOBUILD = $(GOCMD) build
GOCLEAN = $(GOCMD) clean
GOTEST = $(GOCMD) test
GOGET = $(GOCMD) get
GOMOD = $(GOCMD) mod

# 构建参数
LDFLAGS = -ldflags "-s -w \
	-X main.version=$(VERSION) \
	-X main.buildTime=$(BUILD_TIME) \
	-X main.gitCommit=$(GIT_COMMIT)"

# 输出目录
BIN_DIR = bin
CMD_DIR = cmd/agent

# 二进制名称
BINARY_NAME = cloud-flow-agent

# 目标架构
ARCHS = amd64 arm64

# 默认目标
.PHONY: all
all: clean deps build

# 依赖
.PHONY: deps
deps:
	$(GOMOD) download
	$(GOMOD) verify

# 生成 protobuf
.PHONY: proto
proto:
	@which protoc > /dev/null || (echo "protoc not found, please install protobuf" && exit 1)
	protoc --go_out=. --go_opt=paths=source_relative \
		--go-grpc_out=. --go-grpc_opt=paths=source_relative \
		proto/agent.proto

# 生成 eBPF 字节码
.PHONY: bpf
bpf:
	@which clang > /dev/null || (echo "clang not found, please install llvm" && exit 1)
	cd bpf && clang -O2 -g -target bpf \
		-D__TARGET_ARCH_x86 \
		-c network.c -o network_x86.o
	cd bpf && clang -O2 -g -target bpf \
		-D__TARGET_ARCH_arm64 \
		-c network.c -o network_arm64.o

# 本地构建
.PHONY: build
build:
	CGO_ENABLED=0 GOOS=linux GOARCH=amd64 \
		$(GOBUILD) $(LDFLAGS) -o $(BIN_DIR)/$(BINARY_NAME)-amd64 ./$(CMD_DIR)
	CGO_ENABLED=0 GOOS=linux GOARCH=arm64 \
		$(GOBUILD) $(LDFLAGS) -o $(BIN_DIR)/$(BINARY_NAME)-arm64 ./$(CMD_DIR)

# 构建当前架构
.PHONY: build-local
build-local:
	CGO_ENABLED=0 $(GOBUILD) $(LDFLAGS) -o $(BIN_DIR)/$(BINARY_NAME) ./$(CMD_DIR)

# 多架构构建
.PHONY: build-all
build-all: build-amd64 build-arm64

.PHONY: build-amd64
build-amd64:
	CGO_ENABLED=0 GOOS=linux GOARCH=amd64 \
		$(GOBUILD) $(LDFLAGS) -o $(BIN_DIR)/$(BINARY_NAME)-amd64 ./$(CMD_DIR)

.PHONY: build-arm64
build-arm64:
	CGO_ENABLED=0 GOOS=linux GOARCH=arm64 \
		$(GOBUILD) $(LDFLAGS) -o $(BIN_DIR)/$(BINARY_NAME)-arm64 ./$(CMD_DIR)

# 使用 Docker 构建
.PHONY: docker-build
docker-build:
	docker build --build-arg VERSION=$(VERSION) \
		--build-arg BUILD_TIME=$(BUILD_TIME) \
		--build-arg GIT_COMMIT=$(GIT_COMMIT) \
		-t cloud-flow-agent:$(VERSION) .

# 多架构 Docker 构建
.PHONY: docker-buildx
docker-buildx:
	docker buildx build --platform linux/amd64,linux/arm64 \
		--build-arg VERSION=$(VERSION) \
		--build-arg BUILD_TIME=$(BUILD_TIME) \
		--build-arg GIT_COMMIT=$(GIT_COMMIT) \
		-t cloud-flow-agent:$(VERSION) \
		--push .

# 测试
.PHONY: test
test:
	$(GOTEST) -v -race -coverprofile=coverage.out ./...

# 测试覆盖率
.PHONY: coverage
coverage: test
	$(GOCMD) tool cover -html=coverage.out -o coverage.html

# 代码检查
.PHONY: lint
lint:
	@which golangci-lint > /dev/null || (echo "golangci-lint not found" && exit 1)
	golangci-lint run ./...

# 清理
.PHONY: clean
clean:
	$(GOCLEAN)
	rm -rf $(BIN_DIR)
	rm -f coverage.out coverage.html

# 安装
.PHONY: install
install: build
	install -D -m 755 $(BIN_DIR)/$(BINARY_NAME)-$(shell uname -m | sed 's/x86_64/amd64/' | sed 's/aarch64/arm64/') /usr/local/bin/$(BINARY_NAME)
	install -D -m 644 configs/agent.yaml /etc/cloud-flow/agent.yaml
	install -D -m 644 deployments/systemd/cloud-flow-agent.service /etc/systemd/system/

# 打包
.PHONY: package
package: build-all
	mkdir -p dist
	cp $(BIN_DIR)/$(BINARY_NAME)-amd64 dist/
	cp $(BIN_DIR)/$(BINARY_NAME)-arm64 dist/
	cp -r configs dist/
	cp -r deployments dist/
	cp README.md dist/
	tar -czvf cloud-flow-agent-$(VERSION).tar.gz -C dist .

# 发布包（海光 x86_64）
.PHONY: release-amd64
release-amd64: build-amd64
	mkdir -p release
	cp $(BIN_DIR)/$(BINARY_NAME)-amd64 release/cloud-flow-agent
	cp -r configs release/
	cp -r scripts release/
	tar -czvf release/cloud-flow-agent-$(VERSION)-amd64.tar.gz -C release cloud-flow-agent configs scripts

# 发布包（鲲鹏 arm64）
.PHONY: release-arm64
release-arm64: build-arm64
	mkdir -p release
	cp $(BIN_DIR)/$(BINARY_NAME)-arm64 release/cloud-flow-agent
	cp -r configs release/
	cp -r scripts release/
	tar -czvf release/cloud-flow-agent-$(VERSION)-arm64.tar.gz -C release cloud-flow-agent configs scripts

# 帮助
.PHONY: help
help:
	@echo "Cloud Flow Agent Build System"
	@echo ""
	@echo "Usage: make [target]"
	@echo ""
	@echo "Targets:"
	@echo "  all           构建所有架构 (默认)"
	@echo "  deps          下载依赖"
	@echo "  proto         生成 protobuf 代码"
	@echo "  bpf           编译 eBPF 字节码"
	@echo "  build         构建所有架构"
	@echo "  build-local   构建当前架构"
	@echo "  build-amd64   构建 x86_64 (海光)"
	@echo "  build-arm64   构建 aarch64 (鲲鹏)"
	@echo "  docker-build  Docker 构建"
	@echo "  docker-buildx 多架构 Docker 构建"
	@echo "  test          运行测试"
	@echo "  coverage      生成测试覆盖率报告"
	@echo "  lint          代码检查"
	@echo "  clean         清理构建产物"
	@echo "  install       安装到系统"
	@echo "  package       打包发布"
	@echo "  release-amd64 发布 x86_64 包"
	@echo "  release-arm64 发布 arm64 包"
	@echo ""
	@echo "Variables:"
	@echo "  VERSION       版本号 (default: $(VERSION))"
	@echo "  BUILD_TIME    构建时间 (default: $(BUILD_TIME))"
	@echo "  GIT_COMMIT    Git 提交 (default: $(GIT_COMMIT))"
