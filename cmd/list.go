package cmd

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/spf13/cobra"
)

var listCmd = &cobra.Command{
	Use:   "list",
	Short: "列出所有资料库",
	RunE:  runList,
}

func init() {
	rootCmd.AddCommand(listCmd)
}

func runList(cmd *cobra.Command, args []string) error {
	cfg, err := loadConfig()
	if err != nil {
		return err
	}

	client := &http.Client{Timeout: 15 * time.Second}
	req, _ := http.NewRequest("GET", cfg.Server+"/api2/repos/", nil)
	req.Header.Set("Authorization", "Token "+cfg.Token)
	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("连接服务器失败: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("获取资料库列表失败 (HTTP %d)", resp.StatusCode)
	}

	var repos []struct {
		ID       string `json:"id"`
		Name     string `json:"name"`
		Size     int64  `json:"size"`
		Perm     string `json:"permission"`
		Owner    string `json:"owner_name"`
		MTime    int64  `json:"mtime"`
		Encrypted bool  `json:"encrypted"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&repos); err != nil {
		return fmt.Errorf("解析响应失败: %w", err)
	}

	if len(repos) == 0 {
		fmt.Println("没有资料库")
		return nil
	}

	fmt.Printf("%-30s %-8s %-10s %-20s\n", "名称", "权限", "大小", "所有者")
	fmt.Println(strings.Repeat("-", 72))
	for _, r := range repos {
		size := formatSize(r.Size)
		perm := r.Perm
		if r.Encrypted {
			perm += " 🔒"
		}
		fmt.Printf("%-30s %-8s %-10s %-20s\n", r.Name, perm, size, r.Owner)
	}
	fmt.Printf("\n共 %d 个资料库\n", len(repos))
	return nil
}

func formatSize(bytes int64) string {
	const (
		KB = 1024
		MB = KB * 1024
		GB = MB * 1024
		TB = GB * 1024
	)
	switch {
	case bytes >= TB:
		return fmt.Sprintf("%.1f TB", float64(bytes)/float64(TB))
	case bytes >= GB:
		return fmt.Sprintf("%.1f GB", float64(bytes)/float64(GB))
	case bytes >= MB:
		return fmt.Sprintf("%.1f MB", float64(bytes)/float64(MB))
	case bytes >= KB:
		return fmt.Sprintf("%.1f KB", float64(bytes)/float64(KB))
	default:
		return fmt.Sprintf("%d B", bytes)
	}
}
