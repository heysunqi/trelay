# Makefile for Terminal Relay (trelay)
# 一个轻量级的终端远程连接工具

.PHONY: all build clean install uninstall run test vet fmt

# 项目名称
PROJECT := terminal-relay
BINARY := trelay

# Go源文件
SRCS := $(shell find . -name "*.go" -type f)
MAIN := ./cmd/rdm/main.go

# 编译标志
GOFLAGS := -v
LDFLAGS := -s -w

# 默认目标
all: build

# 构建二进制文件
build: $(BINARY)

$(BINARY): $(SRCS)
	@echo "正在构建 $(BINARY)..."
	CGO_ENABLED=0 go build $(GOFLAGS) -ldflags "$(LDFLAGS)" -o $(BINARY) $(MAIN)
	@echo "构建成功！可执行文件已生成: $(BINARY)"

# 清理构建产物
clean:
	@echo "正在清理..."
	rm -f $(BINARY)
	rm -f $(BINARY)-*.tar.gz
	rm -rf dist/
	@echo "清理完成"

# 安装到系统
install: $(BINARY)
	@echo "正在安装到系统..."
	cp $(BINARY) /usr/local/bin/$(BINARY)
	@echo "安装成功！可执行文件已安装到 /usr/local/bin/$(BINARY)"

# 从系统卸载
uninstall:
	@echo "正在卸载..."
	rm -f /usr/local/bin/$(BINARY)
	@echo "卸载完成"

# 运行程序
run:
	@echo "正在运行 $(BINARY)..."
	go run $(MAIN)

# 运行程序（调试模式）
run-debug:
	@echo "正在运行 $(BINARY) (调试模式)..."
	go run $(MAIN) --debug

# 运行测试
test:
	@echo "正在运行测试..."
	go test -v ./...

# 检查代码规范
vet:
	@echo "正在检查代码规范..."
	go vet ./...

# 格式化代码
fmt:
	@echo "正在格式化代码..."
	gofmt -w $(SRCS)

# 构建跨平台版本
.PHONY: build-all build-linux build-darwin

build-all: build-linux build-darwin

build-linux:
	@echo "正在构建 Linux 版本..."
	CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build $(GOFLAGS) -ldflags "$(LDFLAGS)" -o $(BINARY)-linux-amd64 $(MAIN)
	CGO_ENABLED=0 GOOS=linux GOARCH=arm64 go build $(GOFLAGS) -ldflags "$(LDFLAGS)" -o $(BINARY)-linux-arm64 $(MAIN)

build-darwin:
	@echo "正在构建 macOS 版本..."
	CGO_ENABLED=0 GOOS=darwin GOARCH=amd64 go build $(GOFLAGS) -ldflags "$(LDFLAGS)" -o $(BINARY)-darwin-amd64 $(MAIN)
	CGO_ENABLED=0 GOOS=darwin GOARCH=arm64 go build $(GOFLAGS) -ldflags "$(LDFLAGS)" -o $(BINARY)-darwin-arm64 $(MAIN)

# 打包分发
.PHONY: dist

dist: build-all
	@echo "正在创建分发包..."
	mkdir -p dist/
	cp $(BINARY)-linux-amd64 dist/
	cp $(BINARY)-linux-arm64 dist/
	cp $(BINARY)-darwin-amd64 dist/
	cp $(BINARY)-darwin-arm64 dist/
	@echo "分发包已创建在 dist/ 目录中"

# 显示帮助信息
.PHONY: help

help:
	@echo "Terminal Relay (trelay) 构建工具"
	@echo ""
	@echo "可用命令:"
	@echo "  make               构建项目（默认）"
	@echo "  make build         构建可执行文件"
	@echo "  make run           运行程序"
	@echo "  make run-debug     运行程序（调试模式）"
	@echo "  make install       安装到系统"
	@echo "  make uninstall     从系统卸载"
	@echo "  make clean         清理构建产物"
	@echo "  make test          运行测试"
	@echo "  make vet           检查代码规范"
	@echo "  make fmt           格式化代码"
	@echo "  make build-all     构建所有平台版本"
	@echo "  make build-linux   构建 Linux 版本"
	@echo "  make build-darwin  构建 macOS 版本"
	@echo "  make dist          打包分发"
