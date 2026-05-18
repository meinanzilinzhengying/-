# Makefile for cloud-flow-agent

# 编译器设置
CLANG ?= clang
LLVM_OBJCOPY ?= llvm-objcopy
GO ?= go

# 目标文件
BPF_TARGET := internal/ebpfcollector/bpf/tc.bpf.o
BPF_SOURCE := internal/ebpfcollector/bpf/tc.bpf.c
BINARY := bin/agent

# 编译标志
BPF_CFLAGS := -O2 -g -Wall -Werror -target bpf -c
GOFLAGS := -ldflags "-s -w"

# 默认目标
.PHONY: all ebpf test lint build coverage clean help

all: ebpf build

# 编译 eBPF 程序
ebpf: $(BPF_TARGET)

$(BPF_TARGET): $(BPF_SOURCE)
	@echo "Compiling eBPF program..."
	@mkdir -p $(dir $@)
	@$(CLANG) $(BPF_CFLAGS) $< -o $@
	@echo "eBPF program compiled successfully: $@"

# 运行测试
test:
	@echo "Running tests..."
	@$(GO) test -race -short ./... -v
	@echo "Tests completed"

# 运行测试并生成覆盖率报告
coverage:
	@echo "Running tests with coverage..."
	@$(GO) test -race -short -coverprofile=coverage.out ./...
	@$(GO) tool cover -html=coverage.out -o coverage.html
	@echo "Coverage report generated: coverage.html"

# 代码检查
lint:
	@echo "Running linter..."
	@which golangci-lint > /dev/null 2>&1 && golangci-lint run || $(GO) vet ./...
	@echo "Lint completed"

# 构建二进制文件
build:
	@echo "Building agent binary..."
	@mkdir -p bin
	@$(GO) build $(GOFLAGS) -o $(BINARY) ./cmd/
	@echo "Binary built: $(BINARY)"

# 构建所有平台
build-all:
	@echo "Building for all platforms..."
	@mkdir -p bin
	GOOS=linux GOARCH=amd64 $(GO) build $(GOFLAGS) -o bin/agent-linux-amd64 ./cmd/
	GOOS=linux GOARCH=arm64 $(GO) build $(GOFLAGS) -o bin/agent-linux-arm64 ./cmd/
	@echo "Cross-platform build completed"

# 清理
clean:
	@echo "Cleaning..."
	@rm -f $(BPF_TARGET)
	@rm -f $(BINARY)
	@rm -f bin/*
	@rm -f coverage.out coverage.html
	@echo "Clean completed"

# 帮助信息
help:
	@echo "Makefile for cloud-flow-agent"
	@echo ""
	@echo "Targets:"
	@echo "  all       - Compile eBPF and build binary (default)"
	@echo "  ebpf      - Compile eBPF program"
	@echo "  test      - Run unit tests"
	@echo "  coverage  - Run tests with coverage report"
	@echo "  lint      - Run code linter"
	@echo "  build     - Build agent binary"
	@echo "  build-all - Build for all platforms (linux/amd64, linux/arm64)"
	@echo "  clean     - Clean compiled files"
	@echo "  help      - Show this help message"
	@echo ""
	@echo "Usage:"
	@echo "  make [target]"
