package cmd

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/spf13/cobra"
)

var lsCmd = &cobra.Command{
	Use:   "ls <资料库名或ID> [路径]",
	Short: "列出资料库中的文件和目录",
	Args:  cobra.RangeArgs(1, 2),
	RunE:  runLs,
}

func init() {
	rootCmd.AddCommand(lsCmd)
}

func runLs(cmd *cobra.Command, args []string) error {
	cfg, err := loadConfig()
	if err != nil {
		return err
	}

	libraryID := args[0]
	remotePath := "/"
	if len(args) > 1 {
		remotePath = args[1]
	}
	if !strings.HasPrefix(remotePath, "/") {
		remotePath = "/" + remotePath
	}

	// 如果传入的是名字而非 ID，先查找 ID
	if !strings.Contains(libraryID, "-") {
		id, err := findRepoIDByName(cfg, libraryID)
		if err != nil {
			return err
		}
		libraryID = id
	}

	client := &http.Client{Timeout: 15 * time.Second}
	url := fmt.Sprintf("%s/api2/repos/%s/dir/?p=%s", cfg.Server, libraryID, remotePath)
	req, _ := http.NewRequest("GET", url, nil)
	req.Header.Set("Authorization", "Token "+cfg.Token)
	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("连接服务器失败: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("获取目录失败 (HTTP %d)", resp.StatusCode)
	}

	var entries []dirEntry
	if err := json.NewDecoder(resp.Body).Decode(&entries); err != nil {
		return fmt.Errorf("解析响应失败: %w", err)
	}

	if len(entries) == 0 {
		fmt.Println("目录为空")
		return nil
	}

	// 目录排前面
	for _, e := range entries {
		if e.Type == "dir" {
			fmt.Printf("  📁 %s/\n", e.Name)
		}
	}
	for _, e := range entries {
		if e.Type == "file" {
			fmt.Printf("  📄 %s  %s\n", e.Name, formatSize(e.Size))
		}
	}
	fmt.Printf("\n%d 个目录, %d 个文件\n", countType(entries, "dir"), countType(entries, "file"))
	return nil
}

func countType(entries []dirEntry, t string) int {
	n := 0
	for _, e := range entries {
		if e.Type == t {
			n++
		}
	}
	return n
}
