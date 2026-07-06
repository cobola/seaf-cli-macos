package config

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
)

// Config 表示配置结构
type Config struct {
	// 配置根目录
	RootDir string
	// 服务器地址
	Server string
	// 用户名
	Username string
	// Token
	Token string
}

// NewConfig 创建新的配置实例
func NewConfig(rootDir string) *Config {
	return &Config{
		RootDir: rootDir,
	}
}

// GetConfigDir 获取配置目录
func (c *Config) GetConfigDir() string {
	return filepath.Join(c.RootDir, "conf")
}

// GetConfFilePath 获取配置文件路径
func (c *Config) GetConfFilePath() string {
	return filepath.Join(c.GetConfigDir(), "seafile.conf")
}

// Load 从配置文件加载配置
func (c *Config) Load() error {
	configFile := c.GetConfFilePath()
	
	if _, err := os.Stat(configFile); os.IsNotExist(err) {
		// 配置文件不存在，返回空配置
		return nil
	}
	
	data, err := os.ReadFile(configFile)
	if err != nil {
		return fmt.Errorf("读取配置文件失败: %w", err)
	}
	
	// 简单的 ini 格式解析
	lines := strings.Split(string(data), "\n")
	section := ""
	
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") || strings.HasPrefix(line, ";") {
			continue
		}
		
		if strings.HasPrefix(line, "[") && strings.HasSuffix(line, "]") {
			section = line[1 : len(line)-1]
			continue
		}
		
		if strings.Contains(line, "=") {
			parts := strings.SplitN(line, "=", 2)
			if len(parts) == 2 {
				key := strings.TrimSpace(parts[0])
				value := strings.TrimSpace(parts[1])
				
				switch section {
				case "Account":
					switch key {
					case "server":
						c.Server = value
					case "username":
						c.Username = value
					case "token":
						c.Token = value
					}
				}
			}
		}
	}
	
	return nil
}

// Save 保存配置到文件
func (c *Config) Save() error {
	configDir := c.GetConfigDir()
	if err := os.MkdirAll(configDir, 0700); err != nil {
		return fmt.Errorf("创建配置目录失败: %w", err)
	}
	
	configFile := c.GetConfFilePath()
	
	var content strings.Builder
	content.WriteString("[Account]\n")
	content.WriteString(fmt.Sprintf("server = %s\n", c.Server))
	content.WriteString(fmt.Sprintf("username = %s\n", c.Username))
	content.WriteString(fmt.Sprintf("token = %s\n", c.Token))
	content.WriteString("\n")
	
	// 使用 0600 权限，只有当前用户可读写
	if err := os.WriteFile(configFile, []byte(content.String()), 0600); err != nil {
		return fmt.Errorf("写入配置文件失败: %w", err)
	}
	
	return nil
}

// Clear 清除配置
func (c *Config) Clear() error {
	c.Server = ""
	c.Username = ""
	c.Token = ""
	
	configFile := c.GetConfFilePath()
	if _, err := os.Stat(configFile); err == nil {
		if err := os.Remove(configFile); err != nil {
			return fmt.Errorf("删除配置文件失败: %w", err)
		}
	}
	
	return nil
}

// GetDefaultRootDir 获取默认的配置根目录
func GetDefaultRootDir() string {
	home, err := os.UserHomeDir()
	if err != nil {
		// 回退到当前目录
		return "SeafileData"
	}
	
	switch runtime.GOOS {
	case "darwin":
		return filepath.Join(home, "SeafileData")
	case "linux":
		return filepath.Join(home, "SeafileData")
	case "windows":
		return filepath.Join(home, "SeafileData")
	default:
		return filepath.Join(home, "SeafileData")
	}
}