package cmd

import (
	"fmt"
	"os/exec"
	"strings"
)

const keychainService = "com.cobola.seaf-cli"

func keychainSet(account, token string) error {
	// 先删除旧的
	cmd := exec.Command("security", "delete-generic-password",
		"-s", keychainService,
		"-a", account,
		"-w",
	)
	cmd.Run() // 忽略错误（可能不存在）

	// 添加新的
	cmd = exec.Command("security", "add-generic-password",
		"-s", keychainService,
		"-a", account,
		"-w", token,
		"-U", // 更新如果已存在
	)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("写入钥匙串失败: %w\n%s", err, string(out))
	}
	return nil
}

func keychainGet(account string) (string, error) {
	cmd := exec.Command("security", "find-generic-password",
		"-s", keychainService,
		"-a", account,
		"-w",
	)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("从钥匙串读取失败: %w", err)
	}
	return strings.TrimSpace(string(out)), nil
}

func keychainDelete(account string) error {
	cmd := exec.Command("security", "delete-generic-password",
		"-s", keychainService,
		"-a", account,
	)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("从钥匙串删除失败: %w\n%s", err, string(out))
	}
	return nil
}
