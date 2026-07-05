package cmd

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"strings"
	"time"

	"github.com/spf13/cobra"
	"github.com/cobola/seaf-cli-macos/internal/config"
)

var loginCmd = &cobra.Command{
	Use:   "login",
	Short: "登录 Seafile 服务器",
	Long: `登录方式：
  --web                打开浏览器获取 token，终端输入验证
  --config <file.json> 直接导入 JSON 配置`,
	RunE: runLogin,
}

var (
	webMode    bool
	configFile string
	initRootDir string
)

func init() {
	loginCmd.Flags().BoolVar(&webMode, "web", false, "弹出浏览器获取 token")
	loginCmd.Flags().StringVar(&configFile, "config", "", "JSON 配置文件路径（含 server/email/token 字段）")
	rootCmd.AddCommand(loginCmd)
}

func runLogin(cmd *cobra.Command, args []string) error {
	rootDir := initRootDir
	if rootDir == "" {
		rootDir = config.GetDefaultRootDir()
	}

	switch {
	case configFile != "":
		return loginWithConfig(rootDir)
	case webMode:
		return loginWithWeb(rootDir)
	default:
		return loginWithTerminal(rootDir)
	}
}

// saveToken 保存 token 到配置文件和钥匙串
func saveToken(rootDir, server, email, token string) error {
	cfg := config.NewConfig(rootDir)
	if err := cfg.Load(); err != nil {
		return err
	}
	cfg.Server = server
	cfg.Username = email
	cfg.Token = token
	if err := cfg.Save(); err != nil {
		return err
	}
	// 尝试保存到钥匙串
	if err := keychainSet(server, token); err != nil {
		fmt.Printf("  提示: 钥匙串保存失败 (%v)，token 已存入配置文件\n", err)
	}
	return nil
}

// --config 模式：直接读取 JSON 文件导入
func loginWithConfig(rootDir string) error {
	data, err := os.ReadFile(configFile)
	if err != nil {
		return fmt.Errorf("读取文件失败: %w", err)
	}
	var cred struct {
		Server string `json:"server"`
		Email  string `json:"email"`
		Token  string `json:"token"`
	}
	if err := json.Unmarshal(data, &cred); err != nil {
		return fmt.Errorf("JSON 解析失败: %w\n格式示例: {\"server\":\"https://x.com\",\"email\":\"a@b.com\",\"token\":\"xxx\"}", err)
	}
	if cred.Server == "" || cred.Token == "" {
		return fmt.Errorf("server 和 token 字段必填")
	}
	server := normalizeServerURL(cred.Server)
	if err := validateToken(server, cred.Token); err != nil {
		return fmt.Errorf("token 验证失败: %w", err)
	}
	if err := saveToken(rootDir, server, cred.Email, cred.Token); err != nil {
		return err
	}
	fmt.Printf("✓ 登录成功  服务器: %s\n", server)
	if cred.Email != "" {
		fmt.Printf("  用户: %s\n", cred.Email)
	}
	return nil
}

// --web 模式：打开浏览器获取 token，终端输入验证
func loginWithWeb(rootDir string) error {
	reader := bufio.NewReader(os.Stdin)

	fmt.Println("请输入服务器地址（例如 https://pan.hep.com.cn）：")
	fmt.Print("> ")
	server, _ := reader.ReadString('\n')
	server = strings.TrimSpace(server)
	if server == "" {
		return fmt.Errorf("服务器地址不能为空")
	}
	server = normalizeServerURL(server)

	// 打开浏览器到 token 设置页
	tokenURL := server + "/profile"
	fmt.Printf("\n正在打开浏览器: %s\n", tokenURL)
	fmt.Println("请在浏览器中：")
	fmt.Println("  1. 登录（如未登录）")
	fmt.Println("  2. 找到 API Token / 个人令牌")
	fmt.Println("  3. 生成新令牌并复制\n")
	exec.Command("open", tokenURL).Start()

	fmt.Print("请粘贴 API Token:\n> ")
	token, _ := reader.ReadString('\n')
	token = strings.TrimSpace(token)
	if token == "" {
		return fmt.Errorf("token 不能为空")
	}

	fmt.Println("\n正在验证...")
	if err := validateToken(server, token); err != nil {
		return fmt.Errorf("验证失败: %w", err)
	}

	fmt.Print("请输入邮箱（可选，回车跳过）:\n> ")
	email, _ := reader.ReadString('\n')
	email = strings.TrimSpace(email)

	if err := saveToken(rootDir, server, email, token); err != nil {
		return err
	}

	fmt.Printf("\n✓ 登录成功\n  服务器: %s\n", server)
	if email != "" {
		fmt.Printf("  用户: %s\n", email)
	}
	return nil
}

// 无参数模式：终端交互式登录
func loginWithTerminal(rootDir string) error {
	reader := bufio.NewReader(os.Stdin)

	fmt.Println("=== Seafile 登录 ===\n")

	fmt.Print("服务器地址（例如 https://pan.hep.com.cn）:\n> ")
	server, _ := reader.ReadString('\n')
	server = strings.TrimSpace(server)
	if server == "" {
		return fmt.Errorf("服务器地址不能为空")
	}
	server = normalizeServerURL(server)

	fmt.Print("邮箱/用户名:\n> ")
	username, _ := reader.ReadString('\n')
	username = strings.TrimSpace(username)

	fmt.Print("密码:\n> ")
	password, _ := reader.ReadString('\n')
	password = strings.TrimSpace(password)

	if username == "" || password == "" {
		return fmt.Errorf("用户名和密码不能为空")
	}

	fmt.Println("\n正在登录...")
	token, err := fetchAuthToken(server, username, password)
	if err != nil {
		return fmt.Errorf("登录失败: %w", err)
	}

	if err := saveToken(rootDir, server, username, token); err != nil {
		return err
	}

	fmt.Printf("\n✓ 登录成功\n  服务器: %s\n  用户: %s\n", server, username)
	return nil
}

func fetchAuthToken(server, username, password string) (string, error) {
	client := &http.Client{Timeout: 15 * time.Second}
	form := url.Values{"username": {username}, "password": {password}}
	resp, err := client.Post(server+"/api2/auth-token/", "application/x-www-form-urlencoded", strings.NewReader(form.Encode()))
	if err != nil {
		return "", fmt.Errorf("连接服务器失败: %w", err)
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusOK {
		var seaErr struct {
			ErrorMsg string `json:"error_msg"`
			Detail   string `json:"detail"`
		}
		if json.Unmarshal(body, &seaErr) == nil && seaErr.ErrorMsg != "" {
			return "", fmt.Errorf("%s", seaErr.ErrorMsg)
		}
		if seaErr.Detail != "" {
			return "", fmt.Errorf("%s", seaErr.Detail)
		}
		return "", fmt.Errorf("登录失败 (HTTP %d)", resp.StatusCode)
	}
	var result struct {
		Token string `json:"token"`
		Key   string `json:"key"`
	}
	if err := json.Unmarshal(body, &result); err != nil {
		return "", fmt.Errorf("解析响应失败: %w", err)
	}
	token := result.Token
	if token == "" {
		token = result.Key
	}
	if token == "" {
		return "", fmt.Errorf("服务器未返回 token")
	}
	return token, nil
}

func validateToken(server, token string) error {
	req, err := http.NewRequest("GET", server+"/api2/auth/ping/", nil)
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", "Token "+token)
	client := &http.Client{Timeout: 15 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("连接失败: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("token 无效 (HTTP %d)", resp.StatusCode)
	}
	return nil
}

func normalizeServerURL(s string) string {
	s = strings.TrimRight(s, "/")
	if !strings.HasPrefix(s, "http://") && !strings.HasPrefix(s, "https://") {
		s = "https://" + s
	}
	return s
}
