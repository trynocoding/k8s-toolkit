# Makefile for k8s-toolkit

# 版本信息
VERSION ?= 0.1.0
BUILD_DATE := $(shell date -u '+%Y-%m-%d_%H:%M:%S')
GIT_COMMIT := $(shell git rev-parse --short HEAD 2>/dev/null || echo "unknown")

# Go构建参数
BINARY_NAME := k8s-toolkit
LDFLAGS := -X 'github.com/trynocoding/k8s-toolkit/cmd.Version=$(VERSION)' \
           -X 'github.com/trynocoding/k8s-toolkit/cmd.BuildDate=$(BUILD_DATE)' \
           -X 'github.com/trynocoding/k8s-toolkit/cmd.GitCommit=$(GIT_COMMIT)'

# 默认目标
.PHONY: all
all: build

# 构建
.PHONY: build
build:
	@echo "构建 $(BINARY_NAME) v$(VERSION)..."
	go build -ldflags "$(LDFLAGS)" -o $(BINARY_NAME)

# Linux构建
.PHONY: build-linux
build-linux:
	@echo "构建Linux版本..."
	GOOS=linux GOARCH=amd64 go build -ldflags "$(LDFLAGS)" -o $(BINARY_NAME)-linux-amd64

# 清理
.PHONY: clean
clean:
	@echo "清理构建文件..."
	rm -f $(BINARY_NAME) $(BINARY_NAME)-linux-amd64 $(BINARY_NAME).exe

# 测试
.PHONY: test
test:
	go test -v ./...

# 安装依赖
.PHONY: deps
deps:
	go mod download
	go mod tidy

# 运行
.PHONY: run
run:
	go run main.go

# 显示帮助
.PHONY: help
help:
	@echo "可用的make目标:"
	@echo "  make build         - 构建当前平台的二进制文件"
	@echo "  make build-linux   - 构建Linux amd64二进制文件"
	@echo "  make clean         - 清理构建文件"
	@echo "  make test          - 运行测试"
	@echo "  make deps          - 安装/更新依赖"
	@echo "  make run           - 直接运行(不构建)"
	@echo "  make help          - 显示此帮助信息"
