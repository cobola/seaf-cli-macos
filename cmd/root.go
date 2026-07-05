package cmd

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
)

var (
	rootCmd = &cobra.Command{
		Use:   "seaf-cli",
		Short: "macOS 版 Seafile CLI 命令行同步工具",
		Long: `seaf-cli-macos 是基于 Seafile 官方源码编译的 macOS 命令行同步工具。
完全复用官方成熟的 seaf-daemon 同步引擎与 seaf-cli 命令行前端，
提供可移植的绿色包和 Homebrew 安装方式。`,
	}
	appVersion = "dev"
)

func SetVersion(v string) {
	appVersion = v
}

func Execute() error {
	return rootCmd.Execute()
}

func init() {
	cobra.OnInitialize(initConfig)
	rootCmd.AddCommand(&cobra.Command{
		Use:   "version",
		Short: "显示版本号",
		Run: func(cmd *cobra.Command, args []string) {
			fmt.Printf("seaf-cli %s\n", appVersion)
		},
	})
}

func initConfig() {
	// 配置初始化逻辑
}

func checkError(err error) {
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}