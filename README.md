# seaf-cli-macos

macOS 命令行 Seafile 客户端，基于官方 seaf-daemon 块同步协议，不依赖 Python。

## 30 秒上手

```bash
# 1. 安装
curl -L https://github.com/cobola/seaf-cli-macos/releases/latest/download/seaf-cli-macos-arm64.tar.gz | tar -xz
sudo cp seaf-cli-macos-arm64/bin/* /usr/local/bin/

# 2. 登录
seaf-cli login
# 按提示输入服务器地址、邮箱、密码

# 3. 同步
seaf-cli list          # 查看有哪些库
seaf-cli sync 库名 ~/同步目录  # 开始同步
```

## 安装

### 下载预编译包（推荐）

从 [GitHub Releases](https://github.com/cobola/seaf-cli-macos/releases) 下载最新版本。

**Apple Silicon (M1/M2/M3/M4)：**
```bash
curl -L https://github.com/cobola/seaf-cli-macos/releases/latest/download/seaf-cli-macos-arm64.tar.gz -o seaf-cli.tar.gz
tar -xzf seaf-cli.tar.gz
sudo cp seaf-cli-macos-arm64/bin/* /usr/local/bin/
```

**Intel Mac：**
```bash
curl -L https://github.com/cobola/seaf-cli-macos/releases/latest/download/seaf-cli-macos-x86_64.tar.gz -o seaf-cli.tar.gz
tar -xzf seaf-cli.tar.gz
sudo cp seaf-cli-macos-x86_64/bin/* /usr/local/bin/
```

> 💡 如果不想用 sudo，可以安装到用户目录：
> ```bash
> mkdir -p ~/.local/bin
> cp seaf-cli-macos-*/bin/* ~/.local/bin/
> # 然后确保 ~/.local/bin 在 PATH 中
> export PATH="$HOME/.local/bin:$PATH"
> ```

### 源码编译

需要先安装编译依赖：
```bash
brew install autoconf automake libtool cmake pkg-config dylibbundler
brew install glib openssl@3 libevent sqlite jansson python@3.9 vala
```

然后编译安装：
```bash
git clone https://github.com/cobola/seaf-cli-macos.git
cd seaf-cli-macos
chmod +x build.sh && ./build.sh
chmod +x package.sh && ./package.sh
sudo cp dist/bin/* /usr/local/bin/
```

## 使用指南

### 第一步：登录

```bash
seaf-cli login
```

按提示输入：
- 服务器地址（如 `https://cloud.seafile.com`）
- 邮箱
- 密码

> 💡 也可以用 token 登录：`seaf-cli login --web`

### 第二步：查看资料库

```bash
seaf-cli list
```

输出示例：
```
名称                 权限    大小       所有者
─────────────────────────────────────────────
私人资料库            rw     0 B       cobola
工作文件              rw     1.2 GB    cobola
```

### 第三步：浏览文件

```bash
seaf-cli ls 私人资料库           # 查看根目录
seaf-cli ls 私人资料库/文档      # 查看子目录
```

### 第四步：同步到本地

```bash
seaf-cli sync 私人资料库 ~/SeafileData/私人资料库
```

查看同步状态：
```bash
seaf-cli status
```

### 第五步：上传文件

```bash
# 上传整个目录
seaf-cli upload ~/Documents/my-project 私人资料库/my-project

# 上传时自动排除 .DS_Store 等系统文件
seaf-cli upload ~/Documents/my-project 私人资料库/my-project
```

### 停止同步

```bash
seaf-cli stop
```

## 命令一览

| 命令 | 说明 | 示例 |
|------|------|------|
| `seaf-cli login` | 登录 | `seaf-cli login` |
| `seaf-cli login --web` | Token 登录 | 适合 SSO/两步验证 |
| `seaf-cli logout` | 登出 | |
| `seaf-cli whoami` | 查看登录信息 | |
| `seaf-cli list` | 列出所有资料库 | |
| `seaf-cli ls <库> [路径]` | 浏览文件 | `seaf-cli ls 私人资料库/文档` |
| `seaf-cli sync <库> <目录>` | 同步到本地 | `seaf-cli sync 私人资料库 ~/sync` |
| `seaf-cli upload <本地> <远程>` | 上传文件 | `seaf-cli upload ./doc 私人资料库/doc` |
| `seaf-cli status` | 查看同步状态 | |
| `seaf-cli list-local` | 列出本地同步的库 | |
| `seaf-cli start` | 启动守护进程 | |
| `seaf-cli stop` | 停止守护进程 | |
| `seaf-cli version` | 版本号 | |

## 常见问题

### Q: 登录报错怎么办？

确认服务器地址、邮箱、密码正确。如果是 SSO 登录，使用 `seaf-cli login --web`。

### Q: 上传被限流？

大文件上传可能触发服务器限流。使用 zip 策略：
```bash
seaf-cli upload ~/大目录 私人资料库/大目录 -s zip
```

### Q: 同步的文件在哪里？

默认在 `~/SeafileData/` 目录下。查看同步状态：
```bash
seaf-cli status
seaf-cli list-local
```

### Q: 如何停止同步？

```bash
seaf-cli stop
```

## 架构

```
seaf-cli (Go)  ──RPC──>  seaf-daemon (C)  ──HTTP 块同步──>  Seafile 服务器
```

- `seaf-cli`：命令行工具，负责登录、配置、API 查询、文件上传
- `seaf-daemon`：同步守护进程，负责文件同步
- 两者通过 Unix socket (searpc) 通信

## 协议

GPLv3，与官方 Seafile 源码协议一致。
