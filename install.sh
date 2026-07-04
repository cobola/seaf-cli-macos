#!/bin/bash
# seaf-cli-macos 安装脚本

set -euo pipefail

echo "=== seaf-cli-macos 安装脚本 ==="

# 检查是否为 root 用户
if [[ $EUID -eq 0 ]]; then
    echo "请不要以 root 用户运行此脚本"
    exit 1
fi

# 检查依赖
echo "检查依赖..."

# 检查 Go
if ! command -v go &> /dev/null; then
    echo "错误: 未找到 Go，请先安装 Go"
    echo "运行: brew install go"
    exit 1
fi

# 检查编译工具
if ! command -v make &> /dev/null; then
    echo "错误: 未找到 make，请先安装 Xcode 命令行工具"
    echo "运行: xcode-select --install"
    exit 1
fi

# 编译
echo "编译 seaf-cli..."
make build

# 安装到 ~/.local/bin
echo "安装到 ~/.local/bin..."
mkdir -p ~/.local/bin
cp build/seaf-cli ~/.local/bin/
chmod +x ~/.local/bin/seaf-cli

# 检查是否在 PATH 中
if [[ ":$PATH:" != *":$HOME/.local/bin:"* ]]; then
    echo ""
    echo "警告: ~/.local/bin 不在 PATH 中"
    echo "请将以下内容添加到 ~/.zshrc 或 ~/.bash_profile:"
    echo "  export PATH=\"\$HOME/.local/bin:\$PATH\""
fi

echo ""
echo "=== 安装完成 ==="
echo "已安装到: ~/.local/bin/seaf-cli"
echo ""
echo "使用方法:"
echo "  1. 初始化: seaf-cli init -d ~/SeafileData"
echo "  2. 启动: seaf-cli start"
echo "  3. 登录: seaf-cli login --web --server https://your-seafile.com"
echo ""
echo "查看帮助: seaf-cli --help"