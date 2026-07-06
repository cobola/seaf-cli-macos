package cmd

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"
	"github.com/cobola/seaf-cli-macos/internal/config"
)

var uploadCmd = &cobra.Command{
	Use:   "upload <本地目录> <库内路径>",
	Short: "上传文件到 Seafile 资料库",
	Long: `将本地目录的文件上传到指定资料库。

通过 seaf-daemon 块同步协议上传，不走 REST API。
原理：设置同步关系 → 复制文件到本地同步目录 → seaf-daemon 自动上传。`,
	Args: cobra.ExactArgs(2),
	RunE: runUpload,
}

func init() {
	rootCmd.AddCommand(uploadCmd)
}

func runUpload(cmd *cobra.Command, args []string) error {
	localDir := args[0]
	remotePath := args[1]

	// 校验本地目录
	info, err := os.Stat(localDir)
	if err != nil {
		return fmt.Errorf("本地路径不存在: %s", localDir)
	}
	if !info.IsDir() {
		return fmt.Errorf("本地路径不是目录: %s", localDir)
	}

	cfg, err := loadConfig()
	if err != nil {
		return err
	}

	// 解析远程路径: "公共软件" 或 "公共软件/子目录"
	parts := strings.SplitN(remotePath, "/", 2)
	repoName := parts[0]
	subPath := ""
	if len(parts) > 1 {
		subPath = parts[1]
	}

	// 查找资料库 ID
	repoID, err := findRepoIDByName(cfg, repoName)
	if err != nil {
		return err
	}

	// 查找本地同步目录
	syncDir := findLocalSyncDir(repoID)
	if syncDir == "" {
		// 没有同步关系，先设置同步
		fmt.Printf("设置同步关系: %s\n", repoName)
		syncDir, err = setupSync(cfg, repoID, repoName)
		if err != nil {
			return fmt.Errorf("设置同步失败: %w", err)
		}
	}

	// 目标目录：同步根目录/库名/子路径
	targetDir := filepath.Join(syncDir, repoName, subPath)

	// 确保目标目录存在
	if err := os.MkdirAll(targetDir, 0755); err != nil {
		return fmt.Errorf("创建目标目录失败: %w", err)
	}

	// 复制文件到同步目录
	fmt.Printf("复制文件: %s → %s\n", localDir, targetDir)
	if err := copyDir(localDir, targetDir); err != nil {
		return fmt.Errorf("复制文件失败: %w", err)
	}

	fmt.Printf("✓ 文件已复制到同步目录\n")

	// 触发 seaf-daemon 立即同步
	fmt.Println("触发同步...")
	if err := triggerSync(repoID); err != nil {
		fmt.Printf("  触发同步失败: %v（seaf-daemon 会自动重试）\n", err)
	}

	fmt.Println("seaf-daemon 将自动上传变更，使用 seaf-cli status 查看进度")
	return nil
}

func triggerSync(repoID string) error {
	socketPath := findSearpcSocket()
	if socketPath == "" {
		return fmt.Errorf("seaf-daemon 未运行")
	}

	client, err := newSearpcClient(socketPath)
	if err != nil {
		return err
	}
	defer client.close()

	// 启用自动同步
	client.call("seafile_set_config", "auto_sync", "true")

	// 触发立即同步
	_, err = client.call("seafile_sync", repoID, "")
	return err
}

func findLocalSyncDir(repoID string) string {
	rootDir := initRootDir
	if rootDir == "" {
		rootDir = config.GetDefaultRootDir()
	}

	syncFile := filepath.Join(rootDir, "synced-repos.json")
	data, err := os.ReadFile(syncFile)
	if err != nil {
		return ""
	}

	var repos []struct {
		ID   string `json:"id"`
		Name string `json:"name"`
		Path string `json:"path"`
	}
	if err := json.Unmarshal(data, &repos); err != nil {
		return ""
	}

	for _, r := range repos {
		if r.ID == repoID {
			return r.Path
		}
	}
	return ""
}

func setupSync(cfg *config.Config, repoID, repoName string) (string, error) {
	// 确定同步目录
	syncDir := filepath.Join(config.GetDefaultRootDir(), "synced", repoName)
	if err := os.MkdirAll(syncDir, 0755); err != nil {
		return "", err
	}

	// 通过 RPC 设置同步
	socketPath := findSearpcSocket()
	if socketPath == "" {
		return "", fmt.Errorf("seaf-daemon 未运行，请先执行 seaf-cli start")
	}

	client, err := newSearpcClient(socketPath)
	if err != nil {
		return "", err
	}
	defer client.close()

	client.call("seafile_set_config", "url", cfg.Server)
	client.call("seafile_set_config", "token", cfg.Token)

	// 获取 download info
	downloadInfo, err := getDownloadInfo(cfg, repoID)
	if err != nil {
		return "", err
	}

	encVersion := downloadInfo.EncVersion
	if encVersion == 0 {
		encVersion = 1
	}

	var passwd interface{} = nil
	moreInfo := map[string]interface{}{
		"server_url":  cfg.Server,
		"is_readonly": 0,
	}
	if downloadInfo.RepoSalt != "" {
		moreInfo["repo_salt"] = downloadInfo.RepoSalt
	}
	moreInfoJSON, _ := json.Marshal(moreInfo)

	_, err = client.call("seafile_download",
		repoID,
		downloadInfo.RepoVersion,
		downloadInfo.RepoName,
		syncDir,
		downloadInfo.Token,
		passwd,
		downloadInfo.Magic,
		downloadInfo.Email,
		downloadInfo.RandomKey,
		encVersion,
		string(moreInfoJSON),
	)
	if err != nil {
		return "", err
	}

	// 保存同步记录
	saveSyncRecord(repoID, repoName, syncDir)
	return syncDir, nil
}

func copyDir(src, dst string) error {
	return filepath.Walk(src, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		relPath, _ := filepath.Rel(src, path)
		dstPath := filepath.Join(dst, relPath)

		if info.IsDir() {
			return os.MkdirAll(dstPath, 0755)
		}

		// 跳过已存在且大小相同的文件
		if dstInfo, err := os.Stat(dstPath); err == nil && dstInfo.Size() == info.Size() {
			return nil
		}

		return copyFile(path, dstPath)
	})
}

func copyFile(src, dst string) error {
	// 优先使用硬链接（不占额外空间，同文件系统有效）
	if err := os.Link(src, dst); err == nil {
		return nil
	}
	// 硬链接失败（跨文件系统），回退到复制
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()

	out, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer out.Close()

	_, err = io.Copy(out, in)
	return err
}
