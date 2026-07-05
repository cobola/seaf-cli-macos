package cmd

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/spf13/cobra"
	"github.com/cobola/seaf-cli-macos/internal/config"
)

// --- sync (通过 seaf-daemon RPC，不走 REST API) ---
var syncCmd = &cobra.Command{
	Use:   "sync <资料库名或ID> <本地目录>",
	Short: "同步资料库到本地目录（使用 seaf-daemon 块同步协议）",
	Args:  cobra.ExactArgs(2),
	RunE:  runSync,
}

func runSync(cmd *cobra.Command, args []string) error {
	cfg, err := loadConfig()
	if err != nil {
		return err
	}

	libraryID := args[0]
	localDir := args[1]

	// 如果传入的是名字而非 ID，先查找 ID
	if !strings.Contains(libraryID, "-") {
		id, err := findRepoIDByName(cfg, libraryID)
		if err != nil {
			return err
		}
		libraryID = id
	}

	// 确保本地目录存在
	if err := os.MkdirAll(localDir, 0755); err != nil {
		return fmt.Errorf("创建本地目录失败: %w", err)
	}

	// 从服务器获取 download info
	downloadInfo, err := getDownloadInfo(cfg, libraryID)
	if err != nil {
		return fmt.Errorf("获取下载信息失败: %w", err)
	}

	// 连接 seaf-daemon
	socketPath := findSearpcSocket()
	if socketPath == "" {
		return fmt.Errorf("未找到 seaf-daemon socket，请先运行 seaf-cli start")
	}

	client, err := newSearpcClient(socketPath)
	if err != nil {
		return err
	}
	defer client.close()

	// 配置服务器信息
	client.call("seafile_set_config", "url", cfg.Server)
	client.call("seafile_set_config", "token", cfg.Token)

	// 构造 more_info
	moreInfo := map[string]interface{}{
		"server_url":  cfg.Server,
		"is_readonly": 0,
	}
	if downloadInfo.RepoSalt != "" {
		moreInfo["repo_salt"] = downloadInfo.RepoSalt
	}
	moreInfoJSON, _ := json.Marshal(moreInfo)

	// 调用 seafile_download
	// enc_version 默认为 1（seafile-client 源码: dict.value("enc_version", 1).toInt()）
	encVersion := downloadInfo.EncVersion
	if encVersion == 0 {
		encVersion = 1
	}

	// 注意: passwd 必须传 nil（JSON null），不能传空字符串 ""
	// 空字符串在 C 里不是 NULL，会触发 "Incorrect password" 错误
	var passwd interface{} = nil
	if downloadInfo.Encrypted {
		passwd = "" // 加密库需要密码，这里暂不支持
	}

	_, err = client.call("seafile_download",
		libraryID,                // repo_id
		downloadInfo.RepoVersion, // repo_version
		downloadInfo.RepoName,    // name
		localDir,                 // wt_parent
		downloadInfo.Token,       // token
		passwd,                   // password (空=非加密)
		downloadInfo.Magic,       // magic
		downloadInfo.Email,       // email
		downloadInfo.RandomKey,   // random_key
		encVersion,               // enc_version
		string(moreInfoJSON),     // more_info
	)
	if err != nil {
		return fmt.Errorf("同步失败: %w", err)
	}

	// 保存同步记录
	saveSyncRecord(libraryID, downloadInfo.RepoName, localDir)

	fmt.Printf("✓ 已启动同步: %s → %s\n", downloadInfo.RepoName, localDir)
	fmt.Println("使用 seaf-cli status 查看同步状态")
	return nil
}

type downloadInfo struct {
	RepoName    string
	RepoVersion int
	Token       string
	Email       string
	Encrypted   bool
	Magic       string
	EncVersion  int
	RandomKey   string
	RepoSalt    string
	Permission  string
}

func getDownloadInfo(cfg *config.Config, repoID string) (*downloadInfo, error) {
	client := &http.Client{Timeout: 15 * time.Second}
	url := fmt.Sprintf("%s/api2/repos/%s/download-info/", cfg.Server, repoID)
	req, _ := http.NewRequest("GET", url, nil)
	req.Header.Set("Authorization", "Token "+cfg.Token)
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("HTTP %d", resp.StatusCode)
	}

	var info downloadInfo
	var raw struct {
		RepoName    string `json:"repo_name"`
		RepoVersion int    `json:"repo_version"`
		Token       string `json:"token"`
		Email       string `json:"email"`
		Encrypted   interface{} `json:"encrypted"`
		Magic       string `json:"magic"`
		EncVersion  int    `json:"enc_version"`
		RandomKey   string `json:"random_key"`
		Salt        string `json:"salt"`
		Permission  string `json:"permission"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&raw); err != nil {
		return nil, err
	}

	info.RepoName = raw.RepoName
	info.RepoVersion = raw.RepoVersion
	info.Token = raw.Token
	info.Email = raw.Email
	if v, ok := raw.Encrypted.(bool); ok {
		info.Encrypted = v
	} else if v, ok := raw.Encrypted.(float64); ok {
		info.Encrypted = v != 0
	}
	info.Magic = raw.Magic
	info.EncVersion = raw.EncVersion
	info.RandomKey = raw.RandomKey
	info.RepoSalt = raw.Salt
	info.Permission = raw.Permission
	return &info, nil
}

func saveSyncRecord(id, name, path string) error {
	rootDir := initRootDir
	if rootDir == "" {
		rootDir = config.GetDefaultRootDir()
	}
	syncFile := filepath.Join(rootDir, "synced-repos.json")

	var repos []struct {
		ID   string `json:"id"`
		Name string `json:"name"`
		Path string `json:"path"`
	}
	data, _ := os.ReadFile(syncFile)
	json.Unmarshal(data, &repos)

	// 去重
	for _, r := range repos {
		if r.ID == id {
			r.Path = path
			out, _ := json.MarshalIndent(repos, "", "  ")
			return os.WriteFile(syncFile, out, 0644)
		}
	}

	repos = append(repos, struct {
		ID   string `json:"id"`
		Name string `json:"name"`
		Path string `json:"path"`
	}{id, name, path})
	out, _ := json.MarshalIndent(repos, "", "  ")
	return os.WriteFile(syncFile, out, 0644)
}

func downloadDir(cfg *config.Config, repoID, remotePath, localDir string) error {
	// 列出远程目录
	entries, err := listDir(cfg, repoID, remotePath)
	if err != nil {
		return fmt.Errorf("列出目录失败: %w", err)
	}

	for _, e := range entries {
		localPath := filepath.Join(localDir, e.Name)
		remotePath2 := remotePath + e.Name
		if remotePath2 != "/" && !strings.HasSuffix(remotePath2, "/") {
			remotePath2 += "/"
		}

		if e.Type == "dir" {
			if err := os.MkdirAll(localPath, 0755); err != nil {
				fmt.Printf("  ✗ 创建目录失败 %s: %v\n", e.Name, err)
				continue
			}
			fmt.Printf("  📁 %s/\n", e.Name)
			if err := downloadDir(cfg, repoID, remotePath2, localPath); err != nil {
				fmt.Printf("  ✗ 同步子目录失败 %s: %v\n", e.Name, err)
			}
		} else {
			// 跳过已存在的文件
			if info, err := os.Stat(localPath); err == nil && info.Size() == e.Size {
				continue
			}
			fmt.Printf("  📄 %s (%s)\n", e.Name, formatSize(e.Size))
			if err := downloadFile(cfg, repoID, remotePath2, localPath); err != nil {
				fmt.Printf("  ✗ 下载失败 %s: %v\n", e.Name, err)
			}
		}
	}
	return nil
}

func listDir(cfg *config.Config, repoID, dir string) ([]dirEntry, error) {
	client := &http.Client{Timeout: 30 * time.Second}
	encodedDir := url.PathEscape(dir)
	url := fmt.Sprintf("%s/api2/repos/%s/dir/?p=%s", cfg.Server, repoID, encodedDir)
	req, _ := http.NewRequest("GET", url, nil)
	req.Header.Set("Authorization", "Token "+cfg.Token)
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("HTTP %d", resp.StatusCode)
	}
	var entries []dirEntry
	if err := json.NewDecoder(resp.Body).Decode(&entries); err != nil {
		return nil, err
	}
	return entries, nil
}

func downloadFile(cfg *config.Config, repoID, remotePath, localPath string) error {
	client := &http.Client{Timeout: 10 * time.Minute}
	encodedPath := url.PathEscape(remotePath)
	url := fmt.Sprintf("%s/api2/repos/%s/file/?p=%s", cfg.Server, repoID, encodedPath)
	req, _ := http.NewRequest("GET", url, nil)
	req.Header.Set("Authorization", "Token "+cfg.Token)
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("HTTP %d", resp.StatusCode)
	}

	f, err := os.Create(localPath)
	if err != nil {
		return err
	}
	defer f.Close()
	_, err = io.Copy(f, resp.Body)
	return err
}

// --- status (纯 Go，不依赖 Python) ---
var statusCmd = &cobra.Command{
	Use:   "status",
	Short: "显示同步状态",
	RunE:  runStatus,
}

func runStatus(cmd *cobra.Command, args []string) error {
	if isDaemonRunning() {
		fmt.Println("seaf-daemon: 运行中")
	} else {
		fmt.Println("seaf-daemon: 未运行")
		fmt.Println("使用 seaf-cli start 启动")
	}
	return nil
}

// --- list-local ---
var listLocalCmd = &cobra.Command{
	Use:   "list-local",
	Short: "列出已同步到本地的资料库",
	RunE:  runListLocal,
}

func runListLocal(cmd *cobra.Command, args []string) error {
	// 扫描配置目录中的同步记录
	rootDir := initRootDir
	if rootDir == "" {
		rootDir = config.GetDefaultRootDir()
	}

	syncFile := filepath.Join(rootDir, "synced-repos.json")
	data, err := os.ReadFile(syncFile)
	if err != nil {
		fmt.Println("没有已同步的资料库")
		return nil
	}

	var repos []struct {
		ID   string `json:"id"`
		Name string `json:"name"`
		Path string `json:"path"`
	}
	if err := json.Unmarshal(data, &repos); err != nil {
		return err
	}

	if len(repos) == 0 {
		fmt.Println("没有已同步的资料库")
		return nil
	}

	fmt.Printf("%-30s %-40s\n", "名称", "本地路径")
	fmt.Println(strings.Repeat("-", 72))
	for _, r := range repos {
		fmt.Printf("%-30s %-40s\n", r.Name, r.Path)
	}
	return nil
}

// --- desync ---
var desyncCmd = &cobra.Command{
	Use:   "desync <资料库名或ID>",
	Short: "取消同步资料库",
	Args:  cobra.ExactArgs(1),
	RunE:  runDesync,
}

func runDesync(cmd *cobra.Command, args []string) error {
	cfg, err := loadConfig()
	if err != nil {
		return err
	}

	name := args[0]
	if !strings.Contains(name, "-") {
		_, err := findRepoIDByName(cfg, name)
		if err != nil {
			return err
		}
	}

	rootDir := initRootDir
	if rootDir == "" {
		rootDir = config.GetDefaultRootDir()
	}

	syncFile := filepath.Join(rootDir, "synced-repos.json")
	data, err := os.ReadFile(syncFile)
	if err != nil {
		return fmt.Errorf("没有同步记录")
	}

	var repos []struct {
		ID   string `json:"id"`
		Name string `json:"name"`
		Path string `json:"path"`
	}
	if err := json.Unmarshal(data, &repos); err != nil {
		return err
	}

	var filtered []struct {
		ID   string `json:"id"`
		Name string `json:"name"`
		Path string `json:"path"`
	}
	found := false
	for _, r := range repos {
		if r.ID == name || r.Name == name {
			found = true
			fmt.Printf("✓ 已取消同步: %s\n", r.Name)
			continue
		}
		filtered = append(filtered, r)
	}

	if !found {
		return fmt.Errorf("未找到同步记录: %s", name)
	}

	out, _ := json.MarshalIndent(filtered, "", "  ")
	os.WriteFile(syncFile, out, 0644)
	return nil
}

func init() {
	rootCmd.AddCommand(syncCmd)
	rootCmd.AddCommand(statusCmd)
	rootCmd.AddCommand(listLocalCmd)
	rootCmd.AddCommand(desyncCmd)
}
