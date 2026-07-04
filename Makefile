# seaf-cli-macos Makefile

.PHONY: all build clean install test

# 变量
APP_NAME := seaf-cli
VERSION := 0.1.0
BUILD_DIR := build
DIST_DIR := dist

# 构建目标
all: build

# 构建 Go 程序
build:
	@echo "构建 $(APP_NAME)..."
	@mkdir -p $(BUILD_DIR)
	go build -o $(BUILD_DIR)/$(APP_NAME) .

# 安装
install: build
	@echo "安装 $(APP_NAME)..."
	@mkdir -p $(HOME)/.local/bin
	cp $(BUILD_DIR)/$(APP_NAME) $(HOME)/.local/bin/
	@echo "已安装到 $(HOME)/.local/bin/$(APP_NAME)"

# 清理
clean:
	@echo "清理..."
	rm -rf $(BUILD_DIR) $(DIST_DIR)

# 测试
test:
	@echo "运行测试..."
	go test -v ./...

# 打包
package: build
	@echo "打包..."
	./package.sh

# 编译 C 程序（官方 seaf-daemon）
compile-daemon:
	@echo "编译 seaf-daemon..."
	./build.sh

# 完整构建（编译 C 程序 + Go 包装器）
full-build: compile-daemon build

# 帮助
help:
	@echo "可用命令:"
	@echo "  make build        - 构建 Go 包装器"
	@echo "  make install      - 安装到 ~/.local/bin"
	@echo "  make clean        - 清理构建文件"
	@echo "  make test         - 运行测试"
	@echo "  make package      - 打包分发"
	@echo "  make compile-daemon - 编译官方 seaf-daemon"
	@echo "  make full-build   - 完整构建"
	@echo "  make help         - 显示此帮助"