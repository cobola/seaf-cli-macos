#!/bin/bash
# seaf-cli-macos 卸载脚本

set -euo pipefail

echo "=== seaf-cli-macos 卸载脚本 ==="

# 检查是否为 root 用户
if [[ $EUID -eq 0 ]]; then
    echo "请不要以 root 用户运行此脚本"
    exit 1
fi

# 停止守护进程
echo "停止 seaf-daemon..."
if command -v seaf-cli &> /dev/null; then
    seaf-cli stop 2>/dev/null || true
fi

# 删除可执行文件
echo "删除可执行文件..."
rm -f ~/.local/bin/seaf-cli

# 询问是否删除配置
echo ""
read -p "是否删除配置目录 ~/SeafileData？(y/N): " confirm
if [[ $confirm == "y" || $confirm == "Y" ]]; then
    echo "删除配置目录..."
    rm -rf ~/SeafileData
    echo "配置目录已删除"
else
    echo "保留配置目录"
fi

# 清理构建文件
echo "清理构建文件..."
rm -rf build/

echo ""
echo "=== 卸载完成 ==="
echo "seaf-cli-macos 已卸载"