package cmd

import (
	"fmt"

	"github.com/spf13/cobra"
	"github.com/your-username/seaf-cli-macos/internal/config"
)

var whoamiCmd = &cobra.Command{
	Use:   "whoami",
	Short: "显示当前登录用户信息",
	Long:  "显示当前登录的 Seafile 服务器和用户信息",
	RunE:  runWhoami,
}

func init() {
	rootCmd.AddCommand(whoamiCmd)
}

func runWhoami(cmd *cobra.Command, args []string) error {
	rootDir := config.GetDefaultRootDir()
	cfg := config.NewConfig(rootDir)
	
	if err := cfg.Load(); err != nil {
		return fmt.Errorf("加载配置失败: %w", err)
	}
	
	if cfg.Server == "" {
		fmt.Println("未登录")
		fmt.Println("请先运行: seaf-cli login --web --server <服务器地址>")
		return nil
	}
	
	fmt.Println("当前登录信息：")
	fmt.Printf("服务器: %s\n", cfg.Server)
	
	if cfg.Username != "" {
		fmt.Printf("用户名: %s\n", cfg.Username)
	}
	
	if cfg.Token != "" {
		// 隐藏 Token 中间部分
		if len(cfg.Token) > 8 {
			maskedToken := cfg.Token[:4] + "****" + cfg.Token[len(cfg.Token)-4:]
			fmt.Printf("Token: %s\n", maskedToken)
		} else {
			fmt.Printf("Token: %s\n", cfg.Token)
		}
	}
	
	return nil
}