# seaf-cli-macos

macOS 版 Seafile CLI 完整实现方案，基于官方源码编译适配，提供可移植的命令行同步工具。

## 项目说明

本项目基于 Seafile 官方源码（`libsearpc` + `seafile`）进行 macOS 平台适配编译，提供开箱即用的命令行同步工具。**不重写同步内核，只做官方源码的 macOS 适配编译 + 可移植打包 + 标准化分发**。

### 核心特性

- 完全复用官方成熟的 `seaf-daemon` 同步引擎与 `seaf-cli` 命令行前端
- 支持 Apple Silicon (arm64) 和 Intel (x86_64) 双架构
- 提供绿色便携包（tar.gz）和 Homebrew Tap 两种分发方式
- 支持 API Token 网页导入登录，兼容扫码/SSO/两步验证等所有登录方式
- 可选集成 macOS 钥匙串安全存储
- Go 语言实现的命令行包装器，提供更好的用户体验

## 项目结构

```
seaf-cli-macos/
├── build.sh              # C 程序编译脚本
├── package.sh            # 打包脚本
├── test.sh               # 测试脚本
├── Makefile              # 构建自动化
├── main.go               # Go 程序入口
├── cmd/                  # Go 命令实现
│   ├── root.go           # 根命令
│   ├── login.go          # 登录命令
│   ├── logout.go         # 登出命令
│   ├── whoami.go         # 用户信息命令
│   ├── init.go           # 初始化命令
│   ├── start.go          # 启动守护进程
│   └── stop.go           # 停止守护进程
├── internal/
│   └── config/           # 配置管理
│       └── config.go
├── assets/               # 资源文件
│   ├── seaf-cli-wrapper  # 启动包装脚本
│   └── homebrew-formula.rb
├── patches/              # 源码补丁
├── README.md
├── CHANGELOG.md
└── LICENSE               # GPLv3 协议
```

## 快速开始

### 1. 安装依赖

```bash
# 安装 Xcode 命令行工具
xcode-select --install

# 安装 Homebrew（如果未安装）
/bin/bash -c "$(curl -fsSL https://raw.githubusercontent.com/Homebrew/install/HEAD/install.sh)"

# 安装编译工具链
brew install autoconf automake libtool cmake pkg-config dylibbundler

# 安装第三方依赖库
brew install glib openssl@3 libevent sqlite jansson python@3.11 vala

# 安装 Go（如果未安装）
brew install go
```

### 2. 编译安装

```bash
# 克隆本项目
git clone https://github.com/cobola/seaf-cli-macos.git
cd seaf-cli-macos

# 编译 C 程序（seaf-daemon）
chmod +x build.sh
./build.sh

# 编译 Go 包装器
make build

# 打包
chmod +x package.sh
./package.sh
```

### 3. 使用方式

```bash
# 解压绿色包
tar -xzf seaf-cli-macos-*.tar.gz
cd seaf-cli-macos

# 初始化
./bin/seaf-cli init -d ~/SeafileData

# 启动守护进程
./bin/seaf-cli start

# 登录（支持网页 Token 导入）
./bin/seaf-cli login --web --server https://your-seafile.com

# 同步库
./bin/seaf-cli sync -l library-id -d ~/SeafileTest/MyLib -s server-url -u user -p passwd
```

## 命令说明

### 登录命令

```bash
# 网页 Token 登录（推荐）
seaf-cli login --web --server https://your-seafile.com

# 账号密码登录
seaf-cli login --username user@example.com --password yourpassword

# 查看当前登录状态
seaf-cli whoami

# 登出
seaf-cli logout
```

### 守护进程管理

```bash
# 启动守护进程
seaf-cli start

# 停止守护进程
seaf-cli stop

# 查看状态
seaf-cli status
```

### 同步操作

```bash
# 同步库
seaf-cli sync -l library-id -d ~/MyLib -s server-url -u user -p passwd

# 列出所有库
seaf-cli list

# 查看同步状态
seaf-cli status
```

## 项目结构

```
seaf-cli-macos/
├── build.sh              # 一键编译脚本
├── package.sh            # 打包脚本
├── patches/              # macOS 适配补丁
├── assets/
│   ├── seaf-cli-wrapper  # 启动包装脚本
│   └── homebrew-formula.rb
├── README.md
├── CHANGELOG.md
└── LICENSE               # GPLv3 协议
```

## 编译产物

编译完成后会在 `dist` 目录生成：

- `bin/seaf-daemon`：同步后台守护进程（C 语言二进制）
- `bin/seaf-cli`：命令行前端（Python 脚本）
- `lib/`：所有动态库（`libsearpc.dylib`、`libseafile.dylib` 等）
- `lib/python3.11/site-packages/`：Python RPC 绑定模块

## 登录方式

### 1. API Token 网页导入（推荐）

支持所有 Seafile 版本和登录方式（扫码/SSO/2FA）：

```bash
./bin/seaf-cli login --web --server https://your-seafile.com
```

按提示在浏览器完成登录，复制 API Token 粘贴回终端即可。

### 2. 账号密码登录

```bash
./bin/seaf-cli login --username user@example.com --password yourpassword
```

### 3. OAuth2 自动登录（需服务端配置）

```bash
./bin/seaf-cli login --oauth --server https://your-seafile.com
```

## 安装方式

### 方式一：源码编译安装（推荐）

```bash
# 克隆项目
git clone https://github.com/cobola/seaf-cli-macos.git
cd seaf-cli-macos

# 一键安装
chmod +x install.sh
./install.sh
```

### 方式二：下载预编译包

1. 从 GitHub Releases 下载对应架构的压缩包
2. 解压到任意目录
3. 运行 `./bin/seaf-cli --help`

### 方式三：Homebrew 安装

```bash
# 添加 Tap
brew tap your-name/tap

# 安装
brew install seaf-cli-macos
```

## 使用示例

### 基本流程

```bash
# 1. 初始化配置目录
seaf-cli init -d ~/SeafileData

# 2. 启动守护进程
seaf-cli start

# 3. 登录服务器（网页 Token 方式）
seaf-cli login --web --server https://your-seafile.com

# 4. 查看登录状态
seaf-cli whoami

# 5. 同步库
seaf-cli sync -l library-id -d ~/MyLib -s server-url -u user -p passwd

# 6. 查看同步状态
seaf-cli status

# 7. 停止守护进程
seaf-cli stop
```

### 多账号管理

```bash
# 使用不同配置目录
seaf-cli init -d ~/SeafileWork
seaf-cli login --web --server https://work.seafile.com

seaf-cli init -d ~/SeafilePersonal
seaf-cli login --web --server https://personal.seafile.com
```

### 自动启动

```bash
# 创建 launchd 配置
cat > ~/Library/LaunchAgents/com.seafcli.daemon.plist << EOF
<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd">
<plist version="1.0">
<dict>
    <key>Label</key>
    <string>com.seafcli.daemon</string>
    <key>ProgramArguments</key>
    <array>
        <string>$HOME/.local/bin/seaf-cli</string>
        <string>start</string>
    </array>
    <key>RunAtLoad</key>
    <true/>
    <key>KeepAlive</key>
    <true/>
</dict>
</plist>
EOF

# 加载配置
launchctl load ~/Library/LaunchAgents/com.seafcli.daemon.plist
```

## 故障排除

### 编译问题

1. **找不到 glib/openssl 头文件**
   ```bash
   # 检查 PKG_CONFIG_PATH
   echo $PKG_CONFIG_PATH
   
   # 重新设置
   export PKG_CONFIG_PATH=/opt/homebrew/opt/openssl@3/lib/pkgconfig:/opt/homebrew/lib/pkgconfig
   ```

2. **Python 模块编译失败**
   ```bash
   # 检查 Python 版本
   python3 --version
   
   # 确保安装了 Python 开发头文件
   brew install python@3.11
   ```

3. **符号未定义报错**
   ```bash
   # 确保 libsearpc 先编译
   make clean
   ./build.sh
   ```

### 运行问题

1. **动态库找不到**
   ```bash
   # 检查动态库路径
   otool -L bin/seaf-daemon
   
   # 重新收集依赖
   dylibbundler -od -b -x bin/seaf-daemon -d lib/ -p @loader_path/../lib/
   ```

2. **Python 模块导入失败**
   ```bash
   # 检查 PYTHONPATH
   echo $PYTHONPATH
   
   # 手动设置
   export PYTHONPATH=/path/to/seaf-cli-macos/lib/python3.11/site-packages:$PYTHONPATH
   ```

3. **权限问题**
   ```bash
   # 检查文件权限
   ls -la bin/
   
   # 修复权限
   chmod +x bin/*
   ```

## 开源协议

本项目基于 GPLv3 协议开源，与官方 Seafile 源码协议保持一致。

### 合规要求

1. 保留官方源码的所有版权声明
2. 若分发二进制包，必须同时提供对应版本的完整源码
3. 若对官方源码有修改，需在文档中明确标注修改点与原因

## 项目状态

### 已完成

- ✅ Go 语言命令行包装器
- ✅ 基础命令实现（init, login, logout, whoami, start, stop）
- ✅ 配置管理
- ✅ 编译和打包脚本
- ✅ 文档和示例
- ✅ 单元测试

### 进行中

- 🔄 seaf-daemon 编译集成
- 🔄 真实服务器测试
- 🔄 多架构支持

### 待完成

- ⏳ 完整的同步命令实现
- ⏳ OAuth2 登录
- ⏳ macOS 钥匙串集成
- ⏳ Homebrew Tap 发布
- ⏳ CI/CD 流水线

详细状态请查看 [PROJECT-STATUS.md](PROJECT-STATUS.md)

## 贡献指南

欢迎提交 Issue 和 Pull Request。请确保：

1. 代码符合项目风格
2. 更新相关文档
3. 添加必要的测试
4. 遵循 GPLv3 协议

详细贡献指南请查看 [CONTRIBUTING.md](CONTRIBUTING.md)

## 致谢

- [Seafile](https://www.seafile.com/) - 官方同步引擎
- [haiwen](https://github.com/haiwen) - 官方源码维护
- 所有贡献者