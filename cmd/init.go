package cmd

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"
	"github.com/cobola/seaf-cli-macos/internal/config"
)

var initCmd = &cobra.Command{
	Use:   "init",
	Short: "初始化配置目录",
	Long:  "创建 Seafile 配置目录和必要的子目录",
	RunE:  runInit,
}

func init() {
	initCmd.Flags().StringVarP(&initRootDir, "dir", "d", "", "配置目录路径（默认: ~/SeafileData）")
	rootCmd.AddCommand(initCmd)
}

func runInit(cmd *cobra.Command, args []string) error {
	rootDir := initRootDir
	if rootDir == "" {
		rootDir = config.GetDefaultRootDir()
	}
	
	fmt.Printf("初始化配置目录: %s\n", rootDir)
	
	// 创建配置目录
	configDir := filepath.Join(rootDir, "conf")
	if err := os.MkdirAll(configDir, 0755); err != nil {
		return fmt.Errorf("创建配置目录失败: %w", err)
	}
	
	// 创建数据目录
	dataDir := filepath.Join(rootDir, "seafile-data")
	if err := os.MkdirAll(dataDir, 0755); err != nil {
		return fmt.Errorf("创建数据目录失败: %w", err)
	}
	
	// 创建日志目录
	logDir := filepath.Join(rootDir, "logs")
	if err := os.MkdirAll(logDir, 0755); err != nil {
		return fmt.Errorf("创建日志目录失败: %w", err)
	}
	
	// 创建临时目录
	tmpDir := filepath.Join(rootDir, "tmp")
	if err := os.MkdirAll(tmpDir, 0755); err != nil {
		return fmt.Errorf("创建临时目录失败: %w", err)
	}
	
	// 创建配置文件
	cfg := config.NewConfig(rootDir)
	if err := cfg.Save(); err != nil {
		return fmt.Errorf("创建配置文件失败: %w", err)
	}
	
	fmt.Println("初始化完成！")
	fmt.Println()
	fmt.Println("目录结构：")
	fmt.Printf("  %s/\n", rootDir)
	fmt.Printf("  ├── conf/          # 配置文件\n")
	fmt.Printf("  ├── seafile-data/  # 数据文件\n")
	fmt.Printf("  ├── logs/          # 日志文件\n")
	fmt.Printf("  └── tmp/           # 临时文件\n")
	fmt.Println()
	fmt.Println("下一步：")
	fmt.Println("  1. 启动守护进程: seaf-cli start")
	fmt.Println("  2. 登录服务器: seaf-cli login --web --server <服务器地址>")
	
	return nil
}