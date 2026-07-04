#!/bin/bash
# 测试登录功能的脚本

set -euo pipefail

echo "=== 测试登录功能 ==="

SEAF_CLI="./build/seaf-cli"

# 检查 seaf-cli 是否存在
if [[ ! -f "$SEAF_CLI" ]]; then
    echo "错误: 未找到 seaf-cli，请先运行 make build"
    exit 1
fi

# 测试初始化
echo "1. 测试初始化..."
TEST_DIR=$(mktemp -d)
$SEAF_CLI init -d "$TEST_DIR"

# 测试 whoami（未登录状态）
echo ""
echo "2. 测试 whoami（未登录状态）..."
$SEAF_CLI whoami

# 测试登录帮助
echo ""
echo "3. 测试登录帮助..."
$SEAF_CLI login --help

# 测试配置文件
echo ""
echo "4. 检查配置文件..."
if [[ -f "$TEST_DIR/conf/seafile.conf" ]]; then
    echo "配置文件已创建:"
    cat "$TEST_DIR/conf/seafile.conf"
else
    echo "警告: 配置文件未创建"
fi

# 清理
echo ""
echo "5. 清理测试目录..."
rm -rf "$TEST_DIR"

echo ""
echo "=== 登录功能测试完成 ==="
echo "注意: 由于没有真实的 Seafile 服务器，无法测试实际登录"
echo "请手动测试: $SEAF_CLI login --web --server https://your-seafile.com"