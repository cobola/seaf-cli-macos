package cmd

import (
	"fmt"

	"github.com/spf13/cobra"
	"github.com/cobola/seaf-cli-macos/internal/config"
)

var logoutCmd = &cobra.Command{
	Use:   "logout",
	Short: "登出当前账号",
	Long:  "清除本地保存的登录凭证",
	RunE:  runLogout,
}

func init() {
	rootCmd.AddCommand(logoutCmd)
}

func runLogout(cmd *cobra.Command, args []string) error {
	rootDir := config.GetDefaultRootDir()
	cfg := config.NewConfig(rootDir)
	
	if err := cfg.Load(); err != nil {
		return fmt.Errorf("加载配置失败: %w", err)
	}
	
	if cfg.Server == "" {
		fmt.Println("当前未登录")
		return nil
	}
	
	// 确认登出
	fmt.Printf("确定要登出服务器 %s 吗？(y/N): ", cfg.Server)
	var confirm string
	fmt.Scanln(&confirm)
	
	if confirm != "y" && confirm != "Y" {
		fmt.Println("已取消")
		return nil
	}
	
	// 清除配置
	if err := cfg.Clear(); err != nil {
		return fmt.Errorf("清除配置失败: %w", err)
	}
	
	fmt.Println("已登出")
	
	return nil
}