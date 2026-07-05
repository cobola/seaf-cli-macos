# seaf-cli-macos

macOS 命令行 Seafile 客户端，基于官方 seaf-daemon 块同步协议，不依赖 Python。

## 安装

### 方式一：下载预编译包（推荐）

从 [GitHub Releases](https://github.com/cobola/seaf-cli-macos/releases) 下载对应架构的包：

```bash
# Apple Silicon (M1/M2/M3)
curl -L https://github.com/cobola/seaf-cli-macos/releases/latest/download/seaf-cli-macos-arm64.tar.gz -o seaf-cli.tar.gz

# Intel Mac
curl -L https://github.com/cobola/seaf-cli-macos/releases/latest/download/seaf-cli-macos-x86_64.tar.gz -o seaf-cli.tar.gz

# 解压并安装
tar -xzf seaf-cli.tar.gz
sudo cp seaf-cli*/bin/* /usr/local/bin/
```

### 方式二：源码编译

```bash
# 安装依赖
brew install autoconf automake libtool cmake pkg-config dylibbundler
brew install glib openssl@3 libevent sqlite jansson python@3.9 vala

# 克隆并编译
git clone https://github.com/cobola/seaf-cli-macos.git
cd seaf-cli-macos
chmod +x build.sh && ./build.sh
chmod +x package.sh && ./package.sh

# 安装
sudo cp dist/bin/* /usr/local/bin/
```

## 快速开始

```bash
# 1. 启动守护进程
seaf-cli start

# 2. 登录（打开浏览器获取 token）
seaf-cli login --web

# 3. 查看资料库
seaf-cli list

# 4. 同步资料库到本地
seaf-cli sync 公共软件 ~/SeafileData/公共软件

# 5. 查看同步状态
seaf-cli status

# 6. 停止守护进程
seaf-cli stop
```

## 命令一览

| 命令 | 说明 |
|------|------|
| `seaf-cli login --web` | 登录（浏览器获取 token） |
| `seaf-cli login` | 账号密码登录 |
| `seaf-cli logout` | 登出 |
| `seaf-cli whoami` | 查看当前登录信息 |
| `seaf-cli list` | 列出服务器所有资料库 |
| `seaf-cli ls <库名> [路径]` | 列出资料库中的文件 |
| `seaf-cli sync <库名> <本地目录>` | 同步资料库到本地 |
| `seaf-cli upload <本地目录> <库内路径>` | 上传文件到服务器 |
| `seaf-cli status` | 查看同步状态 |
| `seaf-cli list-local` | 列出已同步的本地资料库 |
| `seaf-cli start` | 启动 seaf-daemon |
| `seaf-cli stop` | 停止 seaf-daemon |
| `seaf-cli version` | 显示版本号 |

## 登录方式

### Token 登录（推荐）

```bash
seaf-cli login --web
```

1. 终端输入服务器地址（如 `https://pan.hep.com.cn`）
2. 浏览器自动打开服务器的 API Token 设置页
3. 登录后生成 token，复制粘贴到终端

### 账号密码登录

```bash
seaf-cli login
# 依次输入：服务器地址、邮箱、密码
```

### JSON 文件导入

```bash
# 创建 config.json
echo '{"server":"https://pan.hep.com.cn","email":"user@example.com","token":"xxx"}' > config.json

# 导入
seaf-cli login --config config.json
```

## 同步文件

```bash
# 同步整个资料库
seaf-cli sync 公共软件 ~/SeafileData/公共软件

# 同步后查看状态
seaf-cli status

# 列出已同步的资料库
seaf-cli list-local
```

sync 命令通过 seaf-daemon 的块同步协议工作，不走 REST API，速度快且不会触发 WAF 限流。

## 上传文件

```bash
# 上传目录
seaf-cli upload ~/Documents/my-project 公共软件/my-project

# 跳过已存在的文件
seaf-cli upload ~/Documents/my-project 公共软件/my-project -m skip

# 清空目标后重新上传
seaf-cli upload ~/Documents/my-project 公共软件/my-project -m overwrite
```

## 钥匙串集成

登录时 token 自动存入 macOS Keychain，配置文件作备份。

```bash
# 查看登录信息（token 从钥匙串读取）
seaf-cli whoami
```

## 架构说明

```
seaf-cli (Go)  ──RPC──>  seaf-daemon (C)  ──HTTP 块同步──>  Seafile 服务器
     │                        │
     ├─ 登录管理              ├─ 文件同步引擎
     ├─ 资料库浏览            ├─ 块存储管理
     └─ 配置管理              └─ 冲突处理
```

- `seaf-cli`：Go 实现的命令行工具，负责登录、配置、API 查询
- `seaf-daemon`：C 实现的同步守护进程，负责文件同步
- 两者通过 Unix socket (searpc) 通信

## 协议

GPLv3，与官方 Seafile 源码协议一致。
