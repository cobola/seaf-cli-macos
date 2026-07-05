package cmd

import (
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/cobola/seaf-cli-macos/internal/config"
)

type dirEntry struct {
	Type  string `json:"type"`
	Name  string `json:"name"`
	Size  int64  `json:"size"`
	MTime int64  `json:"mtime"`
	Id    string `json:"id"`
}

func loadConfig() (*config.Config, error) {
	rootDir := initRootDir
	if rootDir == "" {
		rootDir = config.GetDefaultRootDir()
	}
	cfg := config.NewConfig(rootDir)
	if err := cfg.Load(); err != nil {
		return nil, fmt.Errorf("未登录，请先执行 seaf-cli login")
	}
	if cfg.Server == "" || cfg.Token == "" {
		return nil, fmt.Errorf("未登录，请先执行 seaf-cli login")
	}
	if keychainToken, err := keychainGet(cfg.Server); err == nil && keychainToken != "" {
		cfg.Token = keychainToken
	}
	return cfg, nil
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
