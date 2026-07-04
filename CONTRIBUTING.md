# 贡献指南

感谢你对 seaf-cli-macos 项目的关注！我们欢迎各种形式的贡献。

## 如何贡献

### 报告问题

1. 搜索现有的 Issue，避免重复报告
2. 创建新的 Issue，包含以下信息：
   - 问题描述
   - 复现步骤
   - 期望行为
   - 实际行为
   - 环境信息（macOS 版本、架构等）

### 提交代码

1. Fork 本仓库
2. 创建特性分支：`git checkout -b feature/your-feature`
3. 提交更改：`git commit -m 'Add some feature'`
4. 推送到分支：`git push origin feature/your-feature`
5. 创建 Pull Request

### 代码规范

1. **Go 代码**：
   - 遵循 Go 标准格式：`gofmt`
   - 添加必要的注释
   - 编写单元测试

2. **Shell 脚本**：
   - 使用 `shellcheck` 检查语法
   - 添加错误处理
   - 保持脚本简洁

3. **文档**：
   - 更新 README.md
   - 添加使用示例
   - 保持文档清晰

### 提交信息规范

使用 [Conventional Commits](https://www.conventionalcommits.org/) 规范：

```
<type>(<scope>): <subject>

<body>

<footer>
```

类型（type）：
- `feat`: 新功能
- `fix`: 修复 bug
- `docs`: 文档更新
- `style`: 代码格式调整
- `refactor`: 代码重构
- `test`: 测试相关
- `chore`: 构建/工具相关

示例：
```
feat(login): 添加 OAuth2 登录支持

实现了 OAuth2 本地回调登录功能，用户可以通过浏览器自动完成登录。

Closes #123
```

## 开发环境搭建

### 前置要求

1. macOS 12.0+
2. Xcode Command Line Tools
3. Go 1.21+
4. Homebrew

### 安装依赖

```bash
# 安装编译工具
brew install autoconf automake libtool cmake pkg-config dylibbundler

# 安装第三方库
brew install glib openssl@3 libevent sqlite jansson python@3.11 vala

# 安装 Go（如果未安装）
brew install go
```

### 本地开发

```bash
# 克隆仓库
git clone https://github.com/your-username/seaf-cli-macos.git
cd seaf-cli-macos

# 编译
make build

# 运行测试
make test

# 安装到本地
make install
```

### 项目结构

```
seaf-cli-macos/
├── cmd/                  # Go 命令实现
├── internal/             # 内部包
│   └── config/           # 配置管理
├── assets/               # 资源文件
├── patches/              # 源码补丁
├── build.sh              # C 程序编译脚本
├── package.sh            # 打包脚本
├── Makefile              # 构建自动化
└── main.go               # 程序入口
```

## 测试

### 单元测试

```bash
# 运行所有测试
make test

# 运行特定测试
go test -v ./cmd/...
```

### 集成测试

```bash
# 运行集成测试
./test.sh
```

### 手动测试

1. 编译项目：`make build`
2. 初始化：`./build/seaf-cli init -d ~/SeafileTest`
3. 登录：`./build/seaf-cli login --web --server https://your-seafile.com`
4. 同步：`./build/seaf-cli sync -l library-id -d ~/MyLib`

## 发布流程

1. 更新版本号
2. 更新 CHANGELOG.md
3. 创建发布分支：`git checkout -b release/v1.0.0`
4. 提交更改：`git commit -m 'chore: release v1.0.0'`
5. 打标签：`git tag v1.0.0`
6. 推送到远程：`git push origin v1.0.0`
7. 创建 GitHub Release

## 许可证

本项目基于 GPLv3 协议开源。贡献代码即表示你同意将代码以相同协议开源。

## 行为准则

请尊重所有参与者，保持友善和专业的态度。我们致力于为所有人提供开放、友好、包容的社区环境。

## 问题反馈

如有任何问题，请通过以下方式联系：

1. 创建 Issue
2. 发送邮件到：your-email@example.com
3. 加入讨论群：xxx

感谢你的贡献！