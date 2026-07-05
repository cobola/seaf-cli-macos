package cmd

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/spf13/cobra"
	"github.com/your-username/seaf-cli-macos/internal/config"
)

var uploadCmd = &cobra.Command{
	Use:   "upload <本地目录> <库内路径>",
	Short: "上传文件到 Seafile 资料库",
	Long: `将本地目录的文件上传到指定资料库的指定路径

模式说明：
  merge   合并上传（默认），已存在文件覆盖，新文件追加
  skip    跳过已存在的文件，只上传新文件
  overwrite  清空目标目录后重新上传`,
	Args: cobra.ExactArgs(2),
	RunE: runUpload,
}

var uploadMode string

func init() {
	uploadCmd.Flags().StringVarP(&uploadMode, "mode", "m", "merge", "上传模式: merge/skip/overwrite")
	rootCmd.AddCommand(uploadCmd)
}

func runUpload(cmd *cobra.Command, args []string) error {
	localDir := args[0]
	remotePath := args[1]

	// 校验模式
	if uploadMode != "merge" && uploadMode != "skip" && uploadMode != "overwrite" {
		return fmt.Errorf("无效的模式: %s，可选: merge, skip, overwrite", uploadMode)
	}

	// 读取配置
	rootDir := initRootDir
	if rootDir == "" {
		rootDir = config.GetDefaultRootDir()
	}
	cfg := config.NewConfig(rootDir)
	if err := cfg.Load(); err != nil {
		return fmt.Errorf("未登录，请先执行 seaf-cli login")
	}
	if cfg.Server == "" || cfg.Token == "" {
		return fmt.Errorf("未登录，请先执行 seaf-cli login")
	}

	// 检查本地目录
	info, err := os.Stat(localDir)
	if err != nil {
		return fmt.Errorf("本地路径不存在: %s", localDir)
	}
	if !info.IsDir() {
		return fmt.Errorf("本地路径不是目录: %s", localDir)
	}

	// 获取资料库
	repoID, err := findRepo(cfg, remotePath)
	if err != nil {
		return err
	}

	// 提取库内目录路径
	remoteDir := "/" + remotePath
	parts := strings.SplitN(remotePath, "/", 2)
	if len(parts) > 1 {
		remoteDir = "/" + parts[1]
	}

	// overwrite 模式：先清空目标目录
	if uploadMode == "overwrite" {
		fmt.Printf("清空目标目录: %s\n", remoteDir)
		if err := deleteDir(cfg, repoID, remoteDir); err != nil {
			fmt.Printf("  清空失败（可能目录不存在）: %v\n", err)
		}
	}

	// 获取已有文件列表（skip 模式用）
	var existingFiles map[string]bool
	if uploadMode == "skip" {
		existingFiles, _ = listRemoteFiles(cfg, repoID, remoteDir)
		fmt.Printf("服务器已有 %d 个文件\n", len(existingFiles))
	}

	// 获取 upload link
	uploadLink, err := getUploadLink(cfg, repoID, remoteDir)
	if err != nil {
		return fmt.Errorf("获取上传链接失败: %w", err)
	}

	// 收集所有文件
	var files []string
	filepath.Walk(localDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return nil
		}
		if !info.IsDir() {
			files = append(files, path)
		}
		return nil
	})

	fmt.Printf("库: %s\n", remotePath)
	fmt.Printf("模式: %s\n", uploadMode)
	fmt.Printf("本地目录: %s\n", localDir)
	fmt.Printf("共 %d 个文件\n\n", len(files))

	// 收集需要创建的目录（按层级排序）
	var dirs []string
	dirSet := make(map[string]bool)
	for _, filePath := range files {
		relPath, _ := filepath.Rel(localDir, filePath)
		relDir := filepath.Dir(relPath)
		if relDir != "." {
			dir := remoteDir + "/" + filepath.ToSlash(relDir)
			dirSet[dir] = true
		}
	}
	for dir := range dirSet {
		dirs = append(dirs, dir)
	}
	sort.Slice(dirs, func(i, j int) bool {
		return strings.Count(dirs[i], "/") < strings.Count(dirs[j], "/")
	})

	// 创建目录
	createdSet := make(map[string]bool)
	createdSet[remoteDir] = true
	if len(dirs) > 0 {
		fmt.Printf("创建 %d 个目录...\n", len(dirs))
		for _, dir := range dirs {
			parent := filepath.Dir(dir)
			if !createdSet[parent] && parent != remoteDir {
				createDir(cfg, repoID, parent)
				createdSet[parent] = true
			}
			createDir(cfg, repoID, dir)
			createdSet[dir] = true
		}
		fmt.Println()
	}

	// 逐个上传
	success, skip, fail := 0, 0, 0
	for i, filePath := range files {
		relPath, _ := filepath.Rel(localDir, filePath)
		percent := float64(i+1) / float64(len(files)) * 100

		// skip 模式：跳过已存在的文件
		if uploadMode == "skip" && existingFiles[relPath] {
			skip++
			continue
		}

		fmt.Printf("[%d/%d %.0f%%] %s\n", i+1, len(files), percent, relPath)
		if err := uploadFile(cfg, uploadLink, remoteDir, localDir, filePath); err != nil {
			fmt.Printf("  ✗ 失败: %v\n", err)
			fail++
		} else {
			success++
		}
	}

	fmt.Printf("\n完成: %d 成功", success)
	if skip > 0 {
		fmt.Printf(", %d 跳过", skip)
	}
	if fail > 0 {
		fmt.Printf(", %d 失败", fail)
	}
	fmt.Println()
	return nil
}

func findRepo(cfg *config.Config, remotePath string) (string, error) {
	repoName := strings.Split(remotePath, "/")[0]

	client := &http.Client{Timeout: 15 * time.Second}
	req, _ := http.NewRequest("GET", cfg.Server+"/api2/repos/", nil)
	req.Header.Set("Authorization", "Token "+cfg.Token)
	resp, err := client.Do(req)
	if err != nil {
		return "", fmt.Errorf("连接服务器失败: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("获取资料库列表失败 (HTTP %d)", resp.StatusCode)
	}

	var repos []struct {
		ID   string `json:"id"`
		Name string `json:"name"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&repos); err != nil {
		return "", fmt.Errorf("解析响应失败: %w", err)
	}

	for _, r := range repos {
		if r.Name == repoName {
			return r.ID, nil
		}
	}
	return "", fmt.Errorf("未找到资料库: %s", repoName)
}

func createDir(cfg *config.Config, repoID, dir string) error {
	client := &http.Client{Timeout: 15 * time.Second}
	url := fmt.Sprintf("%s/api2/repos/%s/dir/?p=%s", cfg.Server, repoID, dir)
	req, _ := http.NewRequest("POST", url, strings.NewReader("operation=mkdir"))
	req.Header.Set("Authorization", "Token "+cfg.Token)
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("HTTP %d: %s", resp.StatusCode, string(body))
	}
	return nil
}

func deleteDir(cfg *config.Config, repoID, dir string) error {
	client := &http.Client{Timeout: 15 * time.Second}
	url := fmt.Sprintf("%s/api2/repos/%s/dir/?p=%s", cfg.Server, repoID, dir)
	req, _ := http.NewRequest("DELETE", url, nil)
	req.Header.Set("Authorization", "Token "+cfg.Token)
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusNoContent {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("HTTP %d: %s", resp.StatusCode, string(body))
	}
	return nil
}

func listRemoteFiles(cfg *config.Config, repoID, dir string) (map[string]bool, error) {
	client := &http.Client{Timeout: 30 * time.Second}
	url := fmt.Sprintf("%s/api2/repos/%s/dir/?p=%s", cfg.Server, repoID, dir)
	req, _ := http.NewRequest("GET", url, nil)
	req.Header.Set("Authorization", "Token "+cfg.Token)
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var entries []struct {
		Type string `json:"type"`
		Name string `json:"name"`
		Path string `json:"path"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&entries); err != nil {
		return nil, err
	}

	files := make(map[string]bool)
	for _, e := range entries {
		if e.Type == "file" {
			files[e.Name] = true
		}
	}
	return files, nil
}

func getUploadLink(cfg *config.Config, repoID, dir string) (string, error) {
	client := &http.Client{Timeout: 15 * time.Second}
	url := fmt.Sprintf("%s/api2/repos/%s/upload-link/?p=%s", cfg.Server, repoID, dir)
	req, _ := http.NewRequest("GET", url, nil)
	req.Header.Set("Authorization", "Token "+cfg.Token)
	resp, err := client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	var link string
	if err := json.NewDecoder(resp.Body).Decode(&link); err != nil {
		return "", err
	}
	link = strings.Trim(link, `"`)
	return link, nil
}

func uploadFile(cfg *config.Config, uploadLink, remoteDir, localDir, filePath string) error {
	file, err := os.Open(filePath)
	if err != nil {
		return err
	}
	defer file.Close()

	relPath, _ := filepath.Rel(localDir, filePath)
	relDir := filepath.Dir(relPath)
	uploadDir := remoteDir
	if relDir != "." {
		uploadDir = remoteDir + "/" + filepath.ToSlash(relDir)
	}

	var body bytes.Buffer
	writer := multipart.NewWriter(&body)
	writer.WriteField("parent_dir", uploadDir)
	writer.WriteField("replace", "1")

	part, err := writer.CreateFormFile("file", filepath.Base(filePath))
	if err != nil {
		return err
	}
	if _, err := io.Copy(part, file); err != nil {
		return err
	}
	writer.Close()

	client := &http.Client{Timeout: 5 * time.Minute}
	req, _ := http.NewRequest("POST", uploadLink, &body)
	req.Header.Set("Content-Type", writer.FormDataContentType())
	req.Header.Set("Authorization", "Token "+cfg.Token)

	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		respBody, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("HTTP %d: %s", resp.StatusCode, string(respBody))
	}
	return nil
}
