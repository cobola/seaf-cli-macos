#!/bin/bash
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
BUILD_ROOT="$SCRIPT_DIR"
DIST_DIR="$BUILD_ROOT/dist"
PACKAGE_DIR="$BUILD_ROOT/package"
VERSION="0.1.0"
ARCH=$(uname -m)

echo "=== seaf-cli-macos 打包脚本 ==="
echo "版本: $VERSION  架构: $ARCH"

# 清理
rm -rf "$PACKAGE_DIR"
mkdir -p "$PACKAGE_DIR"

# 复制整个 dist 到 package
cp -a "$DIST_DIR" "$PACKAGE_DIR/dist"

# 进入 package/dist 执行 dylibbundler
cd "$PACKAGE_DIR/dist"

# 备份 Python 模块（dylibbundler 会清空 lib/）
echo "备份 Python 模块..."
cp -a lib/python3.9 /tmp/seafcli-python-backup

echo "收集动态库依赖..."
dylibbundler -od -b \
    -x bin/seaf-daemon \
    -d lib/ \
    -p @loader_path/../lib/ 2>&1 || true

# 恢复 Python 模块
echo "恢复 Python 模块..."
rm -rf lib/python3.9
cp -a /tmp/seafcli-python-backup lib/python3.9
rm -rf /tmp/seafcli-python-backup

echo "验证 seaf-daemon 依赖..."
otool -L bin/seaf-daemon | grep -v "System\|usr/lib" || true

# 创建启动脚本
cat > bin/seaf-cli-wrapper << 'WRAPPER'
#!/bin/bash
SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
ROOT_DIR="$(dirname "$SCRIPT_DIR")"
export PATH="$SCRIPT_DIR:$PATH"
export DYLD_LIBRARY_PATH="$ROOT_DIR/lib:${DYLD_LIBRARY_PATH:-}"
export PYTHONPATH="$ROOT_DIR/lib/python3.9/site-packages:${PYTHONPATH:-}"
exec "$SCRIPT_DIR/seaf-cli" "$@"
WRAPPER
chmod +x bin/seaf-cli-wrapper

# 复制 Go 包装器
if [[ -f "$BUILD_ROOT/build/seaf-cli" ]]; then
    cp "$BUILD_ROOT/build/seaf-cli" bin/seaf-cli-go
    chmod +x bin/seaf-cli-go
fi

# 创建 README
cat > README.md << 'README'
# seaf-cli-macos

macOS 版 Seafile CLI 命令行同步工具。

## 使用

```bash
# 初始化
./bin/seaf-cli-wrapper init -d ~/SeafileData

# 启动守护进程
./bin/seaf-cli-wrapper start

# 登录
./bin/seaf-cli-wrapper login --web --server https://your-seafile.com

# 查看帮助
./bin/seaf-cli-wrapper --help
```

## 注意

- 需要系统 Python 3.9+
- 依赖库已内嵌在 lib/ 目录
- 仅支持 macOS arm64
README

# 打包
PACKAGE_NAME="seaf-cli-macos-${VERSION}-${ARCH}"
cd "$PACKAGE_DIR"
mv dist "$PACKAGE_NAME"
tar -czf "${BUILD_ROOT}/${PACKAGE_NAME}.tar.gz" "$PACKAGE_NAME"
shasum -a 256 "${BUILD_ROOT}/${PACKAGE_NAME}.tar.gz" > "${BUILD_ROOT}/${PACKAGE_NAME}.tar.gz.sha256"

echo ""
echo "=== 打包完成 ==="
ls -lh "${BUILD_ROOT}/${PACKAGE_NAME}.tar.gz"
