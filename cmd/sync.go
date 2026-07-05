package cmd

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/spf13/cobra"
	"github.com/cobola/seaf-cli-macos/internal/config"
)

func getDistDir() string {
	exe, _ := os.Executable()
	exeDir := filepath.Dir(exe)
	dir := exeDir
	for i := 0; i < 5; i++ {
		if _, err := os.Stat(filepath.Join(dir, "dist", "bin", "seaf-cli")); err == nil {
			return dir
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			break
		}
		dir = parent
	}
	return exeDir
}

func runSeafCli(args ...string) (string, error) {
	distDir := getDistDir()
	binDir := filepath.Join(distDir, "dist", "bin")
	seafCli := filepath.Join(binDir, "seaf-cli")

	python := "python3"
	if _, err := exec.LookPath(python); err != nil {
		python = "/usr/bin/python3"
	}

	cmd := exec.Command(python, append([]string{seafCli}, args...)...)
	cmd.Env = append(os.Environ(),
		"PATH="+binDir+":"+os.Getenv("PATH"),
		"PYTHONPATH="+filepath.Join(distDir, "dist", "lib", "python3.9", "site-packages"),
	)
	out, err := cmd.CombinedOutput()
	return strings.TrimSpace(string(out)), err
}

// hasSeafCli 检查 seaf-cli Python 是否可用
func hasSeafCli() bool {
	distDir := getDistDir()
	_, err := os.Stat(filepath.Join(distDir, "dist", "bin", "seaf-cli"))
	return err == nil
}

// --- list (local) via seaf-cli Python ---
var listLocalCmd = &cobra.Command{
	Use:   "list-local",
	Short: "列出已同步到本地的资料库",
	RunE: func(cmd *cobra.Command, args []string) error {
		if !hasSeafCli() {
			return fmt.Errorf("seaf-cli Python 不可用，请从项目目录运行或重新安装")
		}
		out, err := runSeafCli("list")
		if err != nil {
			return fmt.Errorf("获取本地资料库失败: %w\n%s", err, out)
		}
		if out == "" {
			fmt.Println("没有已同步的资料库")
			return nil
		}
		fmt.Println(out)
		return nil
	},
}

// --- status via seaf-cli Python ---
var statusCmd = &cobra.Command{
	Use:   "status",
	Short: "显示同步状态",
	RunE: func(cmd *cobra.Command, args []string) error {
		if !hasSeafCli() {
			return fmt.Errorf("seaf-cli Python 不可用，请从项目目录运行或重新安装")
		}
		out, err := runSeafCli("status")
		if err != nil {
			return fmt.Errorf("获取状态失败: %w\n%s", err, out)
		}
		if out == "" {
			fmt.Println("没有正在同步的任务")
			return nil
		}
		fmt.Println(out)
		return nil
	},
}

// --- sync via seaf-cli Python ---
var syncCmd = &cobra.Command{
	Use:   "sync <资料库名或ID> <本地目录>",
	Short: "同步资料库到本地目录",
	Args:  cobra.ExactArgs(2),
	RunE: func(cmd *cobra.Command, args []string) error {
		if !hasSeafCli() {
			return fmt.Errorf("seaf-cli Python 不可用，请从项目目录运行或重新安装")
		}

		libraryID := args[0]
		localDir := args[1]

		if err := os.MkdirAll(localDir, 0755); err != nil {
			return fmt.Errorf("创建本地目录失败: %w", err)
		}

		cfg, err := loadConfig()
		if err != nil {
			return err
		}

		if !strings.Contains(libraryID, "-") {
			id, err := findRepoIDByName(cfg, libraryID)
			if err != nil {
				return err
			}
			libraryID = id
		}

		if _, err := runSeafCli("config", "-k", "url", "-v", cfg.Server); err != nil {
			return fmt.Errorf("配置服务器失败: %w", err)
		}
		if _, err := runSeafCli("config", "-k", "token", "-v", cfg.Token); err != nil {
			return fmt.Errorf("配置 token 失败: %w", err)
		}

		out, err := runSeafCli("sync", libraryID, localDir)
		if err != nil {
			return fmt.Errorf("同步失败: %w\n%s", err, out)
		}

		fmt.Printf("✓ 已启动同步: %s → %s\n", libraryID, localDir)
		fmt.Println("使用 seaf-cli status 查看同步状态")
		return nil
	},
}

// --- desync ---
var desyncCmd = &cobra.Command{
	Use:   "desync <资料库ID>",
	Short: "取消同步资料库",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		if !hasSeafCli() {
			return fmt.Errorf("seaf-cli Python 不可用，请从项目目录运行或重新安装")
		}
		out, err := runSeafCli("desync", args[0])
		if err != nil {
			return fmt.Errorf("取消同步失败: %w\n%s", err, out)
		}
		fmt.Println("✓ 已取消同步")
		return nil
	},
}

func findRepoIDByName(cfg *config.Config, name string) (string, error) {
	client := &http.Client{Timeout: 15 * time.Second}
	req, _ := http.NewRequest("GET", cfg.Server+"/api2/repos/", nil)
	req.Header.Set("Authorization", "Token "+cfg.Token)
	resp, err := client.Do(req)
	if err != nil {
		return "", fmt.Errorf("连接服务器失败: %w", err)
	}
	defer resp.Body.Close()

	var repos []struct {
		ID   string `json:"id"`
		Name string `json:"name"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&repos); err != nil {
		return "", err
	}

	for _, r := range repos {
		if r.Name == name {
			return r.ID, nil
		}
	}
	return "", fmt.Errorf("未找到资料库: %s", name)
}

func init() {
	rootCmd.AddCommand(listLocalCmd)
	rootCmd.AddCommand(statusCmd)
	rootCmd.AddCommand(syncCmd)
	rootCmd.AddCommand(desyncCmd)
}
