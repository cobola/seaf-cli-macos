#!/bin/bash
# seaf-cli-macos 基本使用示例

set -euo pipefail

echo "=== seaf-cli-macos 基本使用示例 ==="

# 假设 seaf-cli 在当前目录
SEAF_CLI="./build/seaf-cli"

# 检查 seaf-cli 是否存在
if [[ ! -f "$SEAF_CLI" ]]; then
    echo "错误: 未找到 seaf-cli，请先运行 make build"
    exit 1
fi

# 1. 初始化
echo "1. 初始化配置目录..."
$SEAF_CLI init -d ~/SeafileData

# 2. 查看帮助
echo ""
echo "2. 查看帮助信息..."
$SEAF_CLI --help

# 3. 登录（需要服务器地址）
echo ""
echo "3. 登录示例..."
echo "请手动运行以下命令登录："
echo "  $SEAF_CLI login --web --server https://your-seafile.com"
echo ""
echo "或者使用账号密码登录："
echo "  $SEAF_CLI login --username user@example.com --password yourpassword"

# 4. 查看登录状态
echo ""
echo "4. 查看登录状态..."
$SEAF_CLI whoami

# 5. 启动守护进程
echo ""
echo "5. 启动守护进程..."
echo "请手动运行以下命令启动："
echo "  $SEAF_CLI start"

echo ""
echo "=== 示例完成 ==="
echo "更多命令请运行: $SEAF_CLI --help"