package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestNewConfig(t *testing.T) {
	cfg := NewConfig("/tmp/test")
	if cfg == nil {
		t.Fatal("NewConfig returned nil")
	}
	if cfg.RootDir != "/tmp/test" {
		t.Errorf("RootDir = %s, want /tmp/test", cfg.RootDir)
	}
}

func TestGetConfigDir(t *testing.T) {
	cfg := NewConfig("/tmp/test")
	got := cfg.GetConfigDir()
	want := filepath.Join("/tmp/test", "conf")
	if got != want {
		t.Errorf("GetConfigDir() = %s, want %s", got, want)
	}
}

func TestGetConfFilePath(t *testing.T) {
	cfg := NewConfig("/tmp/test")
	got := cfg.GetConfFilePath()
	want := filepath.Join("/tmp/test", "conf", "seafile.conf")
	if got != want {
		t.Errorf("GetConfFilePath() = %s, want %s", got, want)
	}
}

func TestSaveAndLoad(t *testing.T) {
	// 创建临时目录
	tmpDir, err := os.MkdirTemp("", "config-test")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tmpDir)

	// 创建配置
	cfg := NewConfig(tmpDir)
	cfg.Server = "https://test.seafile.com"
	cfg.Username = "testuser"
	cfg.Token = "test-token-123"

	// 保存配置
	if err := cfg.Save(); err != nil {
		t.Fatalf("Save() error = %v", err)
	}

	// 检查配置文件是否存在
	configFile := cfg.GetConfFilePath()
	if _, err := os.Stat(configFile); os.IsNotExist(err) {
		t.Fatal("配置文件未创建")
	}

	// 加载配置
	cfg2 := NewConfig(tmpDir)
	if err := cfg2.Load(); err != nil {
		t.Fatalf("Load() error = %v", err)
	}

	// 验证配置
	if cfg2.Server != cfg.Server {
		t.Errorf("Server = %s, want %s", cfg2.Server, cfg.Server)
	}
	if cfg2.Username != cfg.Username {
		t.Errorf("Username = %s, want %s", cfg2.Username, cfg.Username)
	}
	if cfg2.Token != cfg.Token {
		t.Errorf("Token = %s, want %s", cfg2.Token, cfg.Token)
	}
}

func TestClear(t *testing.T) {
	// 创建临时目录
	tmpDir, err := os.MkdirTemp("", "config-test")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tmpDir)

	// 创建配置
	cfg := NewConfig(tmpDir)
	cfg.Server = "https://test.seafile.com"
	cfg.Username = "testuser"
	cfg.Token = "test-token-123"

	// 保存配置
	if err := cfg.Save(); err != nil {
		t.Fatalf("Save() error = %v", err)
	}

	// 清除配置
	if err := cfg.Clear(); err != nil {
		t.Fatalf("Clear() error = %v", err)
	}

	// 验证配置已清除
	if cfg.Server != "" {
		t.Errorf("Server = %s, want empty", cfg.Server)
	}
	if cfg.Username != "" {
		t.Errorf("Username = %s, want empty", cfg.Username)
	}
	if cfg.Token != "" {
		t.Errorf("Token = %s, want empty", cfg.Token)
	}

	// 验证配置文件已删除
	configFile := cfg.GetConfFilePath()
	if _, err := os.Stat(configFile); !os.IsNotExist(err) {
		t.Error("配置文件未删除")
	}
}

func TestGetDefaultRootDir(t *testing.T) {
	got := GetDefaultRootDir()
	if got == "" {
		t.Error("GetDefaultRootDir() returned empty string")
	}
}