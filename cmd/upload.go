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
	"github.com/cobola/seaf-cli-macos/internal/style"
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

var (
	strategy  string
	excludeP  string
)

var defaultExclude = []string{".DS_Store", "desktop.ini", "Thumbs.db", "._*", "*.sbak"}

func init() {
	uploadCmd.Flags().StringVarP(&strategy, "strategy", "s", "auto", "上传策略: auto/zip/direct")
	uploadCmd.Flags().StringVarP(&excludeP, "exclude", "e", "", "排除文件模式（逗号分隔，默认排除 .DS_Store desktop.ini）")
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

func scanDir(root string, excludes []string) *dirAnalysis {
	a := &dirAnalysis{FileTypes: make(map[string]int)}
	filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return nil
		}
		if info.IsDir() {
			a.DirCount++
			return nil
		}
		// 检查排除列表
		if isExcluded(path, excludes) {
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

func isExcluded(path string, excludes []string) bool {
	name := filepath.Base(path)
	for _, pattern := range excludes {
		if strings.HasPrefix(pattern, ".*") {
			// 通配符匹配
			if matched, _ := filepath.Match(pattern, name); matched {
				return true
			}
		} else if name == pattern {
			return true
		}
	}
	return false
}

func getExcludes() []string {
	if excludeP != "" {
		return strings.Split(excludeP, ",")
	}
	return defaultExclude
}

func printAnalysis(a *dirAnalysis, localDir string, excludes []string) {
	fmt.Println(style.Title.Render("📊 分析结果"))
	fmt.Printf("  总大小: %s | 文件: %s | 目录: %s\n",
		style.Progress.Render(formatSize(a.TotalSize)),
		style.Num.Render(fmt.Sprintf("%d", a.FileCount)),
		style.Num.Render(fmt.Sprintf("%d", a.DirCount)))

	// 大小分布
	var small, mid, large int
	filepath.Walk(localDir, func(path string, info os.FileInfo, err error) error {
		if err != nil || info.IsDir() || isExcluded(path, excludes) {
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
		fmt.Printf("  大小分布: %s %d (%.0f%%) | %s %d (%.0f%%) | %s %d (%.0f%%)\n",
			style.Info.Render("<100KB"), small, float64(small)*100/float64(a.FileCount),
			style.Info.Render("100KB-1MB"), mid, float64(mid)*100/float64(a.FileCount),
			style.Info.Render(">1MB"), large, float64(large)*100/float64(a.FileCount))
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
	// 校验策略参数
	if strategy != "auto" && strategy != "zip" && strategy != "direct" {
		fmt.Println(style.Warning(fmt.Sprintf("未知策略 '%s'，使用 auto", strategy)))
		strategy = "auto"
	}

	if strategy != "auto" {
		return strategy
	}

	var stat syscall.Statfs_t
	syscall.Statfs(".", &stat)
	diskFree := stat.Bavail * uint64(stat.Bsize)

	if uint64(a.TotalSize)*3/2 > diskFree {
		fmt.Println(style.Warning("策略: direct（本地空间不足，无法压缩）"))
		return "direct"
	}

	if a.FileCount > 0 {
		ratio := float64(a.SmallFiles) / float64(a.FileCount)
		if ratio > 0.7 && a.FileCount > 50 {
			fmt.Printf("%s %.0f%%\n", style.OK.Render("策略: zip（小文件占比"), ratio*100)
			return "zip"
		}
	}

	fmt.Println(style.InfoMsg("策略: direct（逐文件上传，自动限速避免限流）"))
	return "direct"
}

// --- zip 上传 ---

func createZip(srcDir, zipPath string, excludes []string) error {
	zipFile, err := os.Create(zipPath)
	if err != nil {
		return err
	}
	defer zipFile.Close()

	w := zip.NewWriter(zipFile)
	defer w.Close()

	return filepath.Walk(srcDir, func(path string, info os.FileInfo, err error) error {
		if err != nil || info.IsDir() || isExcluded(path, excludes) {
			return nil
		}
		// 路径穿越检查
		relPath, _ := filepath.Rel(srcDir, path)
		if strings.Contains(relPath, "..") {
			return nil
		}
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

func uploadAsZip(cfg *config.Config, repoID, remotePath, localDir string, a *dirAnalysis, excludes []string) error {
	tmpFile, err := os.CreateTemp("", "seaf-cli-upload-*.zip")
	if err != nil {
		return fmt.Errorf("创建临时文件失败: %w", err)
	}
	tmpZip := tmpFile.Name()
	tmpFile.Close()
	defer os.Remove(tmpZip)

	fmt.Printf("压缩中: %s → ", formatSize(a.TotalSize))
	start := time.Now()
	if err := createZip(localDir, tmpZip, excludes); err != nil {
		return fmt.Errorf("压缩失败: %w", err)
	}
	zipInfo, _ := os.Stat(tmpZip)
	fmt.Printf("%s (%.0f%% 压缩率, %s)\n",
		formatSize(zipInfo.Size()),
		float64(zipInfo.Size())*100/float64(a.TotalSize),
		time.Since(start).Round(time.Millisecond))

	// 上传 zip
	zipName := filepath.Base(tmpZip)
	remoteDir := remotePath
	if !strings.HasPrefix(remoteDir, "/") {
		remoteDir = "/" + remoteDir
	}
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

func uploadFiles(cfg *config.Config, repoID, remotePath, localDir string, a *dirAnalysis, excludes []string) error {
	// remotePath 是库内路径，确保以 / 开头
	remoteDir := remotePath
	if !strings.HasPrefix(remoteDir, "/") {
		remoteDir = "/" + remoteDir
	}
	if err := createRemoteDir(cfg, repoID, remoteDir); err != nil {
		fmt.Printf("创建目录: %v\n", err)
	}

	var allFiles []string
	filepath.Walk(localDir, func(path string, info os.FileInfo, err error) error {
		if err != nil || info.IsDir() || isExcluded(path, excludes) {
			return nil
		}
		allFiles = append(allFiles, path)
		return nil
	})

	// 递归获取已存在文件（相对路径 -> size）
	existingFiles := make(map[string]int64)
	var checkDir func(dir string)
	checkDir = func(dir string) {
		encodedDir := url.PathEscape(dir)
		client := &http.Client{Timeout: 30 * time.Second}
		apiURL := fmt.Sprintf("%s/api2/repos/%s/dir/?p=%s", cfg.Server, repoID, encodedDir)
		req, _ := http.NewRequest("GET", apiURL, nil)
		req.Header.Set("Authorization", "Token "+cfg.Token)
		resp, err := client.Do(req)
		if err != nil {
			return
		}
		defer resp.Body.Close()
		if resp.StatusCode != http.StatusOK {
			return
		}
		var entries []struct {
			Type string `json:"type"`
			Name string `json:"name"`
			Size int64  `json:"size"`
		}
		json.NewDecoder(resp.Body).Decode(&entries)
		for _, e := range entries {
			rel := strings.TrimPrefix(dir+"/"+e.Name, remoteDir)
			if rel != "" && rel[0] == '/' {
				rel = rel[1:]
			}
			if e.Type == "file" {
				existingFiles[rel] = e.Size
			} else if e.Type == "dir" {
				checkDir(dir + "/" + e.Name)
			}
		}
	}
	checkDir(remoteDir)
	fmt.Printf("  服务器已有 %d 个文件\n", len(existingFiles))

	linkCache := make(map[string]string)
	success, skip, fail := 0, 0, 0
	wafRetries := 0
	maxRetries := 3

	// 快速列出已跳过的文件（不请求 API）
	if len(existingFiles) > 0 {
		fmt.Printf("\n%s 已跳过 %d 个文件：\n", style.Success("✓"), len(existingFiles))
		for name := range existingFiles {
			fmt.Printf("  ⏭ %s\n", name)
		}
		fmt.Println()
	}

	for i, filePath := range allFiles {
		relPath, _ := filepath.Rel(localDir, filePath)
		percent := float64(i+1) / float64(len(allFiles)) * 100

		// 跳过同名同大小的文件（断点续传）
		if remoteSize, ok := existingFiles[relPath]; ok {
			localInfo, _ := os.Stat(filePath)
			if localInfo != nil && localInfo.Size() == remoteSize {
				skip++
				continue
			}
		}

		relDir := filepath.Dir(relPath)
		fileRemoteDir := remoteDir
		if relDir != "." {
			fileRemoteDir = remoteDir + "/" + filepath.ToSlash(relDir)
			// 确保子目录存在
			if !dirExists(cfg, repoID, fileRemoteDir) {
				if err := createRemoteDir(cfg, repoID, fileRemoteDir); err != nil {
					fmt.Printf("[%d/%d %.0f%%] %s\n  ✗ 创建目录失败: %v\n", i+1, len(allFiles), percent, relPath, err)
					fail++
					continue
				}
			}
		}

		link, ok := linkCache[fileRemoteDir]
		if !ok {
			var linkErr error
			link, linkErr = getUploadLink(cfg, repoID, fileRemoteDir)
			if linkErr != nil {
				// WAF 封禁检测
				if isWafBlocked(linkErr) && wafRetries < maxRetries {
					delay := time.Duration(30*(1<<uint(wafRetries))) * time.Second
					if delay > 5*time.Minute {
						delay = 5 * time.Minute
					}
					fmt.Printf("\n%s 等待 %v...\n", style.Warning("检测到限流"), delay)
					time.Sleep(delay)
					wafRetries++
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

		fmt.Printf("%s %s\n", style.Progress.Render(fmt.Sprintf("[%d/%d %.0f%%]", i+1, len(allFiles), percent)), relPath)
		if err := uploadFile(cfg, link, fileRemoteDir, localDir, filePath); err != nil {
			// WAF 封禁检测 + 指数退避重试
			if isWafBlocked(err) && wafRetries < maxRetries {
				delay := time.Duration(30*(1<<uint(wafRetries))) * time.Second
				if delay > 5*time.Minute {
					delay = 5 * time.Minute
				}
				fmt.Printf("\n%s 等待 %v...\n", style.Warning("检测到限流"), delay)
				time.Sleep(delay)
				wafRetries++
				// 重试
				if err := uploadFile(cfg, link, fileRemoteDir, localDir, filePath); err != nil {
					fmt.Printf("  ✗ %v\n", err)
					fail++
				} else {
					success++
					wafRetries = 0
				}
			} else if isWafBlocked(err) {
				fmt.Printf("  ✗ 重试次数耗尽: %v\n", err)
				fail++
			} else {
				fmt.Printf("  ✗ %v\n", err)
				fail++
			}
		} else {
			success++
			wafRetries = 0
		}

		// 正常延迟
		time.Sleep(1 * time.Second)
	}

	fmt.Printf("\n%s 成功: %d, 跳过: %d, 失败: %d\n",
		style.Success("完成"), success, skip, fail)
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

func isRetryable(statusCode int) bool {
	return statusCode == 429 || statusCode == 503 || statusCode == 502
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

	// 分离库名和库内路径
	parts := strings.SplitN(remotePath, "/", 2)
	repoName := parts[0]
	repoPath := ""
	if len(parts) > 1 {
		repoPath = "/" + parts[1]
	}

	repoID, err := findRepoIDByName(cfg, repoName)
	if err != nil {
		return err
	}

	// 1. 分析
	excludes := getExcludes()
	fmt.Println("扫描目录...")
	a := scanDir(localDir, excludes)
	printAnalysis(a, localDir, excludes)

	// 2. 选择策略
	s := chooseStrategy(a)
	fmt.Println()

	// 3. 执行上传（repoPath 是库内路径，不含库名）
	switch s {
	case "zip":
		return uploadAsZip(cfg, repoID, repoPath, localDir, a, excludes)
	default:
		return uploadFiles(cfg, repoID, repoPath, localDir, a, excludes)
	}
}

// --- 辅助函数 ---

func createRemoteDir(cfg *config.Config, repoID, dir string) error {
	if dir == "" || dir == "/" {
		return nil
	}
	parts := strings.Split(strings.TrimPrefix(dir, "/"), "/")
	current := ""
	for _, part := range parts {
		current += "/" + part
		// 先检查目录是否已存在
		if dirExists(cfg, repoID, current) {
			continue
		}
		encodedDir := url.PathEscape(current)
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

func dirExists(cfg *config.Config, repoID, dir string) bool {
	encodedDir := url.PathEscape(dir)
	client := &http.Client{Timeout: 10 * time.Second}
	apiURL := fmt.Sprintf("%s/api2/repos/%s/dir/?p=%s", cfg.Server, repoID, encodedDir)
	req, _ := http.NewRequest("GET", apiURL, nil)
	req.Header.Set("Authorization", "Token "+cfg.Token)
	resp, err := client.Do(req)
	if err != nil {
		return false
	}
	defer resp.Body.Close()
	return resp.StatusCode == http.StatusOK
}

func listRemoteFilesWithSize(cfg *config.Config, repoID, dir string) map[string]int64 {
	encodedDir := url.PathEscape(dir)
	client := &http.Client{Timeout: 30 * time.Second}
	apiURL := fmt.Sprintf("%s/api2/repos/%s/dir/?p=%s", cfg.Server, repoID, encodedDir)
	req, _ := http.NewRequest("GET", apiURL, nil)
	req.Header.Set("Authorization", "Token "+cfg.Token)
	resp, err := client.Do(req)
	if err != nil {
		return make(map[string]int64)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return make(map[string]int64)
	}

	var entries []struct {
		Type string `json:"type"`
		Name string `json:"name"`
		Size int64  `json:"size"`
	}
	json.NewDecoder(resp.Body).Decode(&entries)

	files := make(map[string]int64)
	for _, e := range entries {
		if e.Type == "file" {
			files[e.Name] = e.Size
		}
	}
	return files
}

func listRemoteFiles(cfg *config.Config, repoID, dir string) (map[string]bool, error) {
	encodedDir := url.PathEscape(dir)
	client := &http.Client{Timeout: 30 * time.Second}
	apiURL := fmt.Sprintf("%s/api2/repos/%s/dir/?p=%s", cfg.Server, repoID, encodedDir)
	req, _ := http.NewRequest("GET", apiURL, nil)
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
	encodedDir := url.PathEscape(dir)
	client := &http.Client{Timeout: 15 * time.Second}
	apiURL := fmt.Sprintf("%s/api2/repos/%s/upload-link/?p=%s", cfg.Server, repoID, encodedDir)
	req, _ := http.NewRequest("GET", apiURL, nil)
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
