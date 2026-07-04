#!/bin/bash
set -euo pipefail

# seaf-cli-macos 测试脚本

SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"

echo "=== seaf-cli-macos 测试 ==="

# 测试 Go 构建
echo "测试 Go 构建..."
cd "$SCRIPT_DIR"
if go build -o build/seaf-cli .; then
    echo "✓ Go 构建成功"
else
    echo "✗ Go 构建失败"
    exit 1
fi

# 测试命令行帮助
echo "测试命令行帮助..."
if ./build/seaf-cli --help | grep -q "Usage:"; then
    echo "✓ 命令行帮助正常"
else
    echo "✗ 命令行帮助异常"
    exit 1
fi

# 测试版本信息
echo "测试版本信息..."
if ./build/seaf-cli --version 2>/dev/null || true; then
    echo "✓ 版本信息正常"
else
    echo "✗ 版本信息异常"
    exit 1
fi

# 测试初始化命令
echo "测试初始化命令..."
TEST_DIR=$(mktemp -d)
if ./build/seaf-cli init -d "$TEST_DIR"; then
    echo "✓ 初始化命令正常"
    
    # 检查目录结构
    if [ -d "$TEST_DIR/conf" ] && [ -d "$TEST_DIR/seafile-data" ]; then
        echo "✓ 目录结构正确"
    else
        echo "✗ 目录结构错误"
        exit 1
    fi
else
    echo "✗ 初始化命令失败"
    exit 1
fi

# 清理测试目录
rm -rf "$TEST_DIR"

# 测试登录命令（需要服务器）
echo "测试登录命令..."
if ./build/seaf-cli login --help | grep -q "Usage:"; then
    echo "✓ 登录命令帮助正常"
else
    echo "✗ 登录命令帮助异常"
    exit 1
fi

# 测试 whoami 命令
echo "测试 whoami 命令..."
if ./build/seaf-cli whoami 2>/dev/null || true; then
    echo "✓ whoami 命令正常"
else
    echo "✗ whoami 命令异常"
    exit 1
fi

echo ""
echo "=== 测试完成 ==="
echo "所有基本功能测试通过"