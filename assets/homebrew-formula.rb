# typed: false
# frozen_string_literal: true

# seaf-cli-macos Homebrew Formula
# 基于 Seafile 官方源码编译的 macOS 命令行同步工具

class SeafCliMacos < Formula
  desc "macOS version of Seafile CLI - command line sync tool"
  homepage "https://github.com/cobola/seaf-cli-macos"
  version "0.1.0"

  # 架构检测
  if Hardware::CPU.arm?
    url "https://github.com/cobola/seaf-cli-macos/releases/download/v0.1.0/seaf-cli-macos-0.1.0-arm64.tar.gz"
    sha256 "YOUR_ARM64_SHA256_HERE"
  else
    url "https://github.com/cobola/seaf-cli-macos/releases/download/v0.1.0/seaf-cli-macos-0.1.0-x86_64.tar.gz"
    sha256 "YOUR_X86_64_SHA256_HERE"
  end

  # 依赖
  depends_on "autoconf" => :build
  depends_on "automake" => :build
  depends_on "cmake" => :build
  depends_on "libtool" => :build
  depends_on "pkg-config" => :build
  depends_on "glib"
  depends_on "openssl@3"
  depends_on "libevent"
  depends_on "sqlite"
  depends_on "jansson"
  depends_on "python@3.11"

  def install
    # 解压预编译包
    prefix.install Dir["*"]

    # 创建包装脚本
    (bin/"seaf-cli-wrapper").write <<~EOS
      #!/bin/bash
      exec "#{prefix}/bin/seaf-cli" "$@"
    EOS
    chmod "+x", bin/"seaf-cli-wrapper"

    # 创建符号链接
    bin.install_symlink "seaf-cli-wrapper" => "seaf-cli"

    # 安装配置模板
    (share/"seaf-cli-macos").install Dir["share/*"]
  end

  def post_install
    puts <<~EOS
      seaf-cli-macos 安装完成！
      
      快速开始:
        1. 初始化: seaf-cli init -d ~/SeafileData
        2. 启动: seaf-cli start
        3. 登录: seaf-cli login --web --server https://your-seafile.com
      
      查看帮助: seaf-cli --help
    EOS
  end

  test do
    # 测试版本
    assert_match version.to_s, shell_output("#{bin}/seaf-cli --version")
    
    # 测试帮助
    assert_match "Usage:", shell_output("#{bin}/seaf-cli --help")
  end
end