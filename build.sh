#!/bin/bash
set -euo pipefail

# seaf-cli-macos 一键编译脚本
# 基于 Seafile 官方源码编译 macOS 版本

SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
BUILD_ROOT="$SCRIPT_DIR"
SRC_DIR="$BUILD_ROOT/src"
DIST_DIR="$BUILD_ROOT/dist"

# 检测架构
ARCH=$(uname -m)
if [[ "$ARCH" == "arm64" ]]; then
    HOMEBREW_PREFIX="/opt/homebrew"
else
    HOMEBREW_PREFIX="/usr/local"
fi

echo "=== seaf-cli-macos 编译脚本 ==="
echo "检测到架构: $ARCH"
echo "Homebrew 路径: $HOMEBREW_PREFIX"

# 检查依赖
check_dependencies() {
    echo "检查编译依赖..."
    
    # 检查必要的命令
    local required_commands=("autoconf" "automake" "libtool" "cmake" "pkg-config" "make" "gcc")
    for cmd in "${required_commands[@]}"; do
        if ! command -v "$cmd" &> /dev/null; then
            echo "错误: 未找到 $cmd，请先安装"
            echo "运行: brew install autoconf automake libtool cmake pkg-config"
            exit 1
        fi
    done
    
    # 检查必要的库
    local required_libs=("glib-2.0" "openssl" "libevent" "sqlite3" "jansson" "python3")
    for lib in "${required_libs[@]}"; do
        if ! pkg-config --exists "$lib" 2>/dev/null; then
            echo "警告: 未找到 $lib，可能需要安装"
            echo "运行: brew install glib openssl@3 libevent sqlite jansson python@3.11"
        fi
    done
    
    echo "依赖检查完成"
}

# 初始化工作目录
init_workspace() {
    echo "初始化工作目录..."
    
    mkdir -p "$SRC_DIR"
    mkdir -p "$DIST_DIR"
    
    cd "$SRC_DIR"
    
    # 检查是否已克隆源码
    if [[ ! -d "libsearpc" ]]; then
        echo "克隆 libsearpc..."
        git clone https://github.com/haiwen/libsearpc.git -b v3.3-latest
    else
        echo "libsearpc 已存在，跳过克隆"
    fi
    
    if [[ ! -d "seafile" ]]; then
        echo "克隆 seafile..."
        git clone https://github.com/haiwen/seafile.git -b v9.0.20
    else
        echo "seafile 已存在，跳过克隆"
    fi
}

# 设置编译环境
setup_env() {
    echo "设置编译环境..."
    
    # 设置 PKG_CONFIG_PATH
    export PKG_CONFIG_PATH="${PKG_CONFIG_PATH:-}"
    export PKG_CONFIG_PATH="$DIST_DIR/lib/pkgconfig:$HOMEBREW_PREFIX/opt/openssl@3/lib/pkgconfig:$HOMEBREW_PREFIX/lib/pkgconfig:$PKG_CONFIG_PATH"
    
    # 设置 Python 路径
    if command -v python3 &> /dev/null; then
        PYTHON_PATH=$(which python3)
        echo "使用 Python: $PYTHON_PATH"
    else
        echo "错误: 未找到 python3"
        exit 1
    fi
    
    # 设置编译标志
    export CFLAGS="-I$HOMEBREW_PREFIX/include"
    export LDFLAGS="-L$HOMEBREW_PREFIX/lib"
    export DYLD_LIBRARY_PATH="${DYLD_LIBRARY_PATH:-}"
    export DYLD_LIBRARY_PATH="$DIST_DIR/lib:$DYLD_LIBRARY_PATH"
}

# 编译 libsearpc
build_libsearpc() {
    echo "=== 编译 libsearpc ==="
    
    cd "$SRC_DIR/libsearpc"
    
    # 清理之前的编译
    make clean 2>/dev/null || true
    
    # 生成 configure 脚本
    echo "生成 configure 脚本..."
    ./autogen.sh
    
    # 配置
    echo "配置编译参数..."
    ./configure \
        --prefix="$DIST_DIR" \
        --with-python="$PYTHON_PATH" \
        --disable-static \
        --enable-shared
    
    # 编译
    echo "编译 libsearpc..."
    make -j$(sysctl -n hw.ncpu)
    
    # 安装
    echo "安装 libsearpc..."
    make install
    
    echo "libsearpc 编译完成"
}

# 编译 seafile
build_seafile() {
    echo "=== 编译 seafile ==="
    
    cd "$SRC_DIR/seafile"
    
    # 清理之前的编译
    make clean 2>/dev/null || true
    
    # 生成 configure 脚本
    echo "生成 configure 脚本..."
    ./autogen.sh
    
    # 配置
    echo "配置编译参数..."
    ./configure \
        --prefix="$DIST_DIR" \
        --disable-gui \
        --disable-server \
        --disable-console \
        --with-python="$PYTHON_PATH" \
        --enable-shared \
        --disable-static
    
    # 编译
    echo "编译 seafile..."
    make -j$(sysctl -n hw.ncpu)
    
    # 安装
    echo "安装 seafile..."
    make install
    
    echo "seafile 编译完成"
}

# 验证编译结果
verify_build() {
    echo "=== 验证编译结果 ==="
    
    # 检查关键文件
    local required_files=(
        "$DIST_DIR/bin/seaf-daemon"
        "$DIST_DIR/bin/seaf-cli"
        "$DIST_DIR/lib/libsearpc.dylib"
        "$DIST_DIR/lib/libseafile.dylib"
    )
    
    for file in "${required_files[@]}"; do
        if [[ -f "$file" ]]; then
            echo "✓ 找到: $file"
        else
            echo "✗ 缺失: $file"
            exit 1
        fi
    done
    
    # 测试 seaf-cli
    echo "测试 seaf-cli..."
    if "$DIST_DIR/bin/seaf-cli" --version; then
        echo "✓ seaf-cli 版本检查通过"
    else
        echo "✗ seaf-cli 版本检查失败"
        exit 1
    fi
    
    echo "编译验证完成"
}

# 编译 Go 包装器
build_go_wrapper() {
    echo "=== 编译 Go 包装器 ==="
    
    cd "$BUILD_ROOT"
    
    # 检查 Go 是否安装
    if ! command -v go &> /dev/null; then
        echo "错误: 未找到 go，请先安装 Go"
        echo "运行: brew install go"
        exit 1
    fi
    
    # 编译 Go 程序
    echo "编译 Go 包装器..."
    mkdir -p build
    go build -o build/seaf-cli .
    
    echo "Go 包装器编译完成"
}

# 主函数
main() {
    echo "开始编译 seaf-cli-macos..."
    
    check_dependencies
    init_workspace
    setup_env
    build_libsearpc
    build_seafile
    build_go_wrapper
    verify_build
    
    echo ""
    echo "=== 编译成功 ==="
    echo "编译产物位于: $DIST_DIR"
    echo "Go 包装器位于: $BUILD_ROOT/build/seaf-cli"
    echo "运行以下命令查看帮助:"
    echo "  $BUILD_ROOT/build/seaf-cli --help"
    echo ""
    echo "下一步: 运行 ./package.sh 进行打包"
}

# 运行主函数
main "$@"