package cmd

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"

	"github.com/spf13/cobra"
	"github.com/cobola/seaf-cli-macos/internal/config"
)

var stopCmd = &cobra.Command{
	Use:   "stop",
	Short: "停止 seaf-daemon 守护进程",
	Long:  "停止 Seafile 同步守护进程",
	RunE:  runStop,
}

func init() {
	rootCmd.AddCommand(stopCmd)
}

func runStop(cmd *cobra.Command, args []string) error {
	rootDir := config.GetDefaultRootDir()
	pidFile := filepath.Join(rootDir, "seaf-daemon.pid")
	
	// 读取 PID 文件
	data, err := os.ReadFile(pidFile)
	if err != nil {
		fmt.Println("seaf-daemon 未运行")
		return nil
	}
	
	pidStr := strings.TrimSpace(string(data))
	pid, err := strconv.Atoi(pidStr)
	if err != nil {
		fmt.Println("PID 文件格式错误")
		return nil
	}
	
	// 查找进程
	process, err := os.FindProcess(pid)
	if err != nil {
		fmt.Println("seaf-daemon 未运行")
		return nil
	}
	
	// 发送终止信号
	fmt.Printf("停止 seaf-daemon (PID: %d)...\n", pid)
	
	if err := process.Signal(syscall.SIGTERM); err != nil {
		// 如果进程不存在，忽略错误
		fmt.Println("seaf-daemon 未运行")
	} else {
		fmt.Println("seaf-daemon 已停止")
	}
	
	// 删除 PID 文件
	if err := os.Remove(pidFile); err != nil && !os.IsNotExist(err) {
		fmt.Printf("警告: 无法删除 PID 文件: %v\n", err)
	}
	
	return nil
}