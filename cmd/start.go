package cmd

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"syscall"

	"github.com/spf13/cobra"
	"github.com/cobola/seaf-cli-macos/internal/config"
)

var startCmd = &cobra.Command{
	Use:   "start",
	Short: "启动 seaf-daemon 守护进程",
	Long:  "启动 Seafile 同步守护进程",
	RunE:  runStart,
}

func init() {
	rootCmd.AddCommand(startCmd)
}

func runStart(cmd *cobra.Command, args []string) error {
	rootDir := config.GetDefaultRootDir()
	cfg := config.NewConfig(rootDir)
	
	if err := cfg.Load(); err != nil {
		return fmt.Errorf("加载配置失败: %w", err)
	}
	
	// 检查是否已运行
	if isDaemonRunning() {
		fmt.Println("seaf-daemon 已在运行")
		return nil
	}
	
	// 获取 seaf-daemon 路径
	daemonPath := getDaemonPath()
	if _, err := os.Stat(daemonPath); os.IsNotExist(err) {
		return fmt.Errorf("未找到 seaf-daemon: %s", daemonPath)
	}
	
	// 设置环境变量
	distDir := filepath.Dir(filepath.Dir(getDaemonPath()))
	env := os.Environ()
	env = append(env, fmt.Sprintf("DYLD_LIBRARY_PATH=%s/dist/lib", distDir))
	env = append(env, fmt.Sprintf("PYTHONPATH=%s/dist/lib/python3.9/site-packages", distDir))
	
	// 启动守护进程
	fmt.Println("启动 seaf-daemon...")
	
	daemonCmd := exec.Command(daemonPath, "-d", rootDir)
	daemonCmd.Env = env
	daemonCmd.Stdout = nil
	daemonCmd.Stderr = nil
	
	// 设置进程组
	daemonCmd.SysProcAttr = &syscall.SysProcAttr{
		Setpgid: true,
	}
	
	if err := daemonCmd.Start(); err != nil {
		return fmt.Errorf("启动守护进程失败: %w", err)
	}
	
	fmt.Printf("seaf-daemon 已启动 (PID: %d)\n", daemonCmd.Process.Pid)
	
	// 保存 PID 到文件
	pidFile := filepath.Join(rootDir, "seaf-daemon.pid")
	if err := os.WriteFile(pidFile, []byte(fmt.Sprintf("%d", daemonCmd.Process.Pid)), 0644); err != nil {
		fmt.Printf("警告: 无法保存 PID 文件: %v\n", err)
	}
	
	return nil
}

func isDaemonRunning() bool {
	rootDir := config.GetDefaultRootDir()
	pidFile := filepath.Join(rootDir, "seaf-daemon.pid")
	
	data, err := os.ReadFile(pidFile)
	if err != nil {
		return false
	}
	
	var pid int
	if _, err := fmt.Sscanf(string(data), "%d", &pid); err != nil {
		return false
	}
	
	// 检查进程是否存在
	process, err := os.FindProcess(pid)
	if err != nil {
		return false
	}
	
	// 发送信号 0 检查进程是否存活
	err = process.Signal(syscall.Signal(0))
	return err == nil
}

func getDaemonPath() string {
	exe, _ := os.Executable()
	exeDir := filepath.Dir(exe)
	// 先看同目录
	p := filepath.Join(exeDir, "seaf-daemon")
	if _, err := os.Stat(p); err == nil {
		return p
	}
	// 向上查找 dist/bin/seaf-daemon
	dir := exeDir
	for i := 0; i < 5; i++ {
		p := filepath.Join(dir, "dist", "bin", "seaf-daemon")
		if _, err := os.Stat(p); err == nil {
			return p
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			break
		}
		dir = parent
	}
	return "seaf-daemon"
}

func getRootDir() string {
	// 获取可执行文件所在目录
	execPath, err := os.Executable()
	if err != nil {
		return "."
	}
	
	return filepath.Dir(filepath.Dir(execPath))
}