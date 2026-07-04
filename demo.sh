#!/bin/bash
# seaf-cli-macos 完整演示脚本

set -euo pipefail

echo "=== seaf-cli-macos 完整演示 ==="

SEAF_CLI="./build/seaf-cli"

# 检查 seaf-cli 是否存在
if [[ ! -f "$SEAF_CLI" ]]; then
    echo "错误: 未找到 seaf-cli，请先运行 make build"
    exit 1
fi

# 创建演示目录
DEMO_DIR=$(mktemp -d)
echo "演示目录: $DEMO_DIR"

# 1. 初始化
echo ""
echo "=== 1. 初始化配置目录 ==="
$SEAF_CLI init -d "$DEMO_DIR/seafile"

# 2. 查看帮助
echo ""
echo "=== 2. 查看帮助信息 ==="
$SEAF_CLI --help

# 3. 登录命令帮助
echo ""
echo "=== 3. 登录命令帮助 ==="
$SEAF_CLI login --help

# 4. 查看当前状态
echo ""
echo "=== 4. 查看当前状态 ==="
$SEAF_CLI whoami

# 5. 启动命令帮助
echo ""
echo "=== 5. 启动命令帮助 ==="
$SEAF_CLI start --help 2>/dev/null || echo "start 命令需要 seaf-daemon"

# 6. 停止命令帮助
echo ""
echo "=== 6. 停止命令帮助 ==="
$SEAF_CLI stop --help 2>/dev/null || echo "stop 命令需要 seaf-daemon"

# 7. 检查配置文件
echo ""
echo "=== 7. 检查配置文件 ==="
if [[ -f "$DEMO_DIR/seafile/conf/seafile.conf" ]]; then
    echo "配置文件内容:"
    cat "$DEMO_DIR/seafile/conf/seafile.conf"
else
    echo "配置文件未创建"
fi

# 8. 检查目录结构
echo ""
echo "=== 8. 检查目录结构 ==="
echo "目录结构:"
find "$DEMO_DIR/seafile" -type d | sort

# 清理
echo ""
echo "=== 9. 清理演示目录 ==="
rm -rf "$DEMO_DIR"

echo ""
echo "=== 演示完成 ==="
echo "seaf-cli-macos 基础功能演示完成"
echo ""
echo "实际使用需要:"
echo "1. 编译 seaf-daemon: ./build.sh"
echo "2. 登录服务器: $SEAF_CLI login --web --server https://your-seafile.com"
echo "3. 启动守护进程: $SEAF_CLI start"
echo "4. 同步文件: $SEAF_CLI sync -l library-id -d ~/MyLib"