package cmd

import (
	"archive/zip"
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"syscall"
	"time"

	"github.com/spf13/cobra"
	"github.com/cobola/seaf-cli-macos/internal/config"
)

var uploadCmd = &cobra.Command{
	Use:   "upload <本地目录> <资料库名/子路径>",
	Short: "单向备份上传文件到服务器",
	Long: `将本地文件上传到 Seafile 服务器。

上传策略（--strategy）：
  auto     根据文件数量和大小自动选择（默认）
  zip      先压缩再上传（适合大量小文件）
  direct   逐文件直接上传（适合大文件）`,
	Args: cobra.ExactArgs(2),
	RunE: runUpload,
}

var strategy string

func init() {
	uploadCmd.Flags().StringVarP(&strategy, "strategy", "s", "auto", "上传策略: auto/zip/direct")
	rootCmd.AddCommand(uploadCmd)
}

// --- 分析 ---

type dirAnalysis struct {
	TotalSize  int64
	FileCount  int
	DirCount   int
	SmallFiles int // < 100KB
	LargeFiles int // > 1MB
	FileTypes  map[string]int
}

func scanDir(root string) *dirAnalysis {
	a := &dirAnalysis{FileTypes: make(map[string]int)}
	filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return nil
		}
		if info.IsDir() {
			a.DirCount++
			return nil
		}
		a.FileCount++
		a.TotalSize += info.Size()
		ext := strings.ToLower(filepath.Ext(path))
		if ext == "" {
			ext = "(无扩展名)"
		}
		a.FileTypes[ext]++
		if info.Size() < 100*1024 {
			a.SmallFiles++
		}
		if info.Size() > 1024*1024 {
			a.LargeFiles++
		}
		return nil
	})
	return a
}

func printAnalysis(a *dirAnalysis, localDir string) {
	fmt.Println("分析结果:")
	fmt.Printf("  总大小: %s | 文件: %d | 目录: %d\n", formatSize(a.TotalSize), a.FileCount, a.DirCount)

	// 大小分布
	var small, mid, large int
	filepath.Walk(localDir, func(path string, info os.FileInfo, err error) error {
		if err != nil || info.IsDir() {
			return nil
		}
		switch {
		case info.Size() < 100*1024:
			small++
		case info.Size() > 1024*1024:
			large++
		default:
			mid++
		}
		return nil
	})
	if a.FileCount > 0 {
		fmt.Printf("  大小分布: <100KB: %d (%.0f%%) | 100KB-1MB: %d (%.0f%%) | >1MB: %d (%.0f%%)\n",
			small, float64(small)*100/float64(a.FileCount),
			mid, float64(mid)*100/float64(a.FileCount),
			large, float64(large)*100/float64(a.FileCount))
	}

	// 文件类型
	type kv struct {
		Key   string
		Count int
	}
	var types []kv
	for k, v := range a.FileTypes {
		types = append(types, kv{k, v})
	}
	sort.Slice(types, func(i, j int) bool { return types[i].Count > types[j].Count })
	if len(types) > 5 {
		types = types[:5]
	}
	var parts []string
	for _, t := range types {
		parts = append(parts, fmt.Sprintf("%s(%d)", t.Key, t.Count))
	}
	fmt.Printf("  文件类型: %s\n", strings.Join(parts, ", "))
}

// --- 策略选择 ---

func chooseStrategy(a *dirAnalysis) string {
	if strategy != "auto" {
		return strategy
	}

	// 检查本地可用空间
	var stat syscall.Statfs_t
	syscall.Statfs(".", &stat)
	diskFree := stat.Bavail * uint64(stat.Bsize)

	// 空间不够压缩
	if uint64(a.TotalSize)*3/2 > diskFree {
		fmt.Println("  策略: direct（本地空间不足，无法压缩）")
		return "direct"
	}

	// 计算小文件占比
	if a.FileCount > 0 {
		ratio := float64(a.SmallFiles) / float64(a.FileCount)
		// 小文件占比高且文件多 → 压缩
		if ratio > 0.7 && a.FileCount > 50 {
			fmt.Printf("  策略: zip（小文件占比 %.0f%%，压缩减少请求）\n", ratio*100)
			return "zip"
		}
	}

	// 默认直传，慢速上传
	fmt.Println("  策略: direct（逐文件上传，自动限速避免限流）")
	return "direct"
}

// --- zip 上传 ---

func createZip(srcDir, zipPath string) error {
	zipFile, err := os.Create(zipPath)
	if err != nil {
		return err
	}
	defer zipFile.Close()

	w := zip.NewWriter(zipFile)
	defer w.Close()

	return filepath.Walk(srcDir, func(path string, info os.FileInfo, err error) error {
		if err != nil || info.IsDir() {
			return nil
		}
		relPath, _ := filepath.Rel(srcDir, path)
		f, err := w.Create(relPath)
		if err != nil {
			return err
		}
		srcFile, err := os.Open(path)
		if err != nil {
			return err
		}
		defer srcFile.Close()
		_, err = io.Copy(f, srcFile)
		return err
	})
}

func uploadAsZip(cfg *config.Config, repoID, remotePath, localDir string, a *dirAnalysis) error {
	tmpZip := filepath.Join(os.TempDir(), "seaf-cli-upload.zip")
	defer os.Remove(tmpZip)

	fmt.Printf("压缩中: %s → ", formatSize(a.TotalSize))
	start := time.Now()
	if err := createZip(localDir, tmpZip); err != nil {
		return fmt.Errorf("压缩失败: %w", err)
	}
	zipInfo, _ := os.Stat(tmpZip)
	fmt.Printf("%s (%.0f%% 压缩率, %s)\n",
		formatSize(zipInfo.Size()),
		float64(zipInfo.Size())*100/float64(a.TotalSize),
		time.Since(start).Round(time.Millisecond))

	// 上传 zip
	zipName := filepath.Base(tmpZip)
	remoteDir := "/" + remotePath
	if err := createRemoteDir(cfg, repoID, remoteDir); err != nil {
		fmt.Printf("创建目录: %v\n", err)
	}
	link, err := getUploadLink(cfg, repoID, remoteDir)
	if err != nil {
		return fmt.Errorf("获取上传链接失败: %w", err)
	}

	fmt.Printf("上传 zip: %s (%s)\n", zipName, formatSize(zipInfo.Size()))
	if err := uploadFile(cfg, link, remoteDir, localDir, tmpZip); err != nil {
		return fmt.Errorf("上传失败: %w", err)
	}

	fmt.Println("✓ 上传完成")
	return nil
}

// --- 直接上传 ---

func uploadFiles(cfg *config.Config, repoID, remotePath, localDir string, a *dirAnalysis) error {
	remoteDir := "/" + remotePath
	if err := createRemoteDir(cfg, repoID, remoteDir); err != nil {
		fmt.Printf("创建目录: %v\n", err)
	}

	var allFiles []string
	filepath.Walk(localDir, func(path string, info os.FileInfo, err error) error {
		if err != nil || info.IsDir() {
			return nil
		}
		allFiles = append(allFiles, path)
		return nil
	})

	existingFiles, _ := listRemoteFiles(cfg, repoID, remoteDir)

	linkCache := make(map[string]string)
	success, skip, fail := 0, 0, 0
	wafPause := false

	for i, filePath := range allFiles {
		relPath, _ := filepath.Rel(localDir, filePath)
		percent := float64(i+1) / float64(len(allFiles)) * 100

		if existingFiles[relPath] {
			skip++
			continue
		}

		relDir := filepath.Dir(relPath)
		fileRemoteDir := remoteDir
		if relDir != "." {
			fileRemoteDir = remoteDir + "/" + filepath.ToSlash(relDir)
		}

		link, ok := linkCache[fileRemoteDir]
		if !ok {
			var linkErr error
			link, linkErr = getUploadLink(cfg, repoID, fileRemoteDir)
			if linkErr != nil {
				// WAF 封禁检测
				if isWafBlocked(linkErr) {
					fmt.Printf("\n⚠ 检测到限流，暂停 60 秒...\n")
					time.Sleep(60 * time.Second)
					wafPause = true
					link, linkErr = getUploadLink(cfg, repoID, fileRemoteDir)
					if linkErr != nil {
						fmt.Printf("[%d/%d %.0f%%] %s\n  ✗ %v\n", i+1, len(allFiles), percent, relPath, linkErr)
						fail++
						continue
					}
				} else {
					fmt.Printf("[%d/%d %.0f%%] %s\n  ✗ %v\n", i+1, len(allFiles), percent, relPath, linkErr)
					fail++
					continue
				}
			}
			linkCache[fileRemoteDir] = link
			time.Sleep(1 * time.Second)
		}

		fmt.Printf("[%d/%d %.0f%%] %s\n", i+1, len(allFiles), percent, relPath)
		if err := uploadFile(cfg, link, fileRemoteDir, localDir, filePath); err != nil {
			// WAF 封禁检测
			if isWafBlocked(err) {
				fmt.Printf("\n⚠ 检测到限流，暂停 60 秒...\n")
				time.Sleep(60 * time.Second)
				wafPause = true
				// 重试
				if err := uploadFile(cfg, link, fileRemoteDir, localDir, filePath); err != nil {
					fmt.Printf("  ✗ %v\n", err)
					fail++
				} else {
					success++
				}
			} else {
				fmt.Printf("  ✗ %v\n", err)
				fail++
			}
		} else {
			success++
			wafPause = false
		}

		// 根据是否刚暂停过调整延迟
		if wafPause {
			time.Sleep(2 * time.Second)
		} else {
			time.Sleep(1 * time.Second)
		}
	}

	fmt.Printf("\n完成: %d 成功, %d 跳过, %d 失败\n", success, skip, fail)
	return nil
}

func isWafBlocked(err error) bool {
	if err == nil {
		return false
	}
	msg := err.Error()
	return strings.Contains(msg, "405") || strings.Contains(msg, "429") ||
		strings.Contains(msg, "blocked") || strings.Contains(msg, "WAF")
}

// --- 入口 ---

func runUpload(cmd *cobra.Command, args []string) error {
	localDir := args[0]
	remotePath := args[1]

	if _, err := os.Stat(localDir); err != nil {
		return fmt.Errorf("本地路径不存在: %s", localDir)
	}

	cfg, err := loadConfig()
	if err != nil {
		return err
	}

	parts := strings.SplitN(remotePath, "/", 2)
	repoName := parts[0]

	repoID, err := findRepoIDByName(cfg, repoName)
	if err != nil {
		return err
	}

	// 1. 分析
	fmt.Println("扫描目录...")
	a := scanDir(localDir)
	printAnalysis(a, localDir)

	// 2. 选择策略
	s := chooseStrategy(a)
	fmt.Println()

	// 3. 执行上传
	switch s {
	case "zip":
		return uploadAsZip(cfg, repoID, remotePath, localDir, a)
	default:
		return uploadFiles(cfg, repoID, remotePath, localDir, a)
	}
}

// --- 辅助函数 ---

func createRemoteDir(cfg *config.Config, repoID, dir string) error {
	parts := strings.Split(strings.TrimPrefix(dir, "/"), "/")
	current := ""
	for _, part := range parts {
		current += "/" + part
		encodedDir := (&url.URL{Path: current}).String()
		client := &http.Client{Timeout: 15 * time.Second}
		apiURL := fmt.Sprintf("%s/api2/repos/%s/dir/?p=%s", cfg.Server, repoID, encodedDir)
		req, _ := http.NewRequest("POST", apiURL, strings.NewReader("operation=mkdir"))
		req.Header.Set("Authorization", "Token "+cfg.Token)
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		resp, err := client.Do(req)
		if err != nil {
			return err
		}
		resp.Body.Close()
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

	// 目录不存在时返回空列表
	if resp.StatusCode == http.StatusNotFound || resp.StatusCode == http.StatusBadRequest {
		return make(map[string]bool), nil
	}

	var entries []struct {
		Type string `json:"type"`
		Name string `json:"name"`
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

	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("HTTP %d: %s", resp.StatusCode, string(body))
	}

	link := strings.Trim(string(body), `"`)
	return link, nil
}

func uploadFile(cfg *config.Config, uploadLink, remoteDir, localDir, filePath string) error {
	file, err := os.Open(filePath)
	if err != nil {
		return err
	}
	defer file.Close()

	var body bytes.Buffer
	writer := multipart.NewWriter(&body)
	writer.WriteField("parent_dir", remoteDir)
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
