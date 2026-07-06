package style

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"
)

var (
	// 标题
	Title = lipgloss.NewStyle().
		Bold(true).
		Foreground(lipgloss.Color("205")).
		MarginBottom(1)

	// 成功
	OK = lipgloss.NewStyle().
		Foreground(lipgloss.Color("46"))

	// 错误
	Err = lipgloss.NewStyle().
		Foreground(lipgloss.Color("196"))

	// 警告
	Warn = lipgloss.NewStyle().
		Foreground(lipgloss.Color("208"))

	// 信息
	Info = lipgloss.NewStyle().
		Foreground(lipgloss.Color("241"))

	// 文件/目录
	Dir = lipgloss.NewStyle().
		Foreground(lipgloss.Color("33"))

	File = lipgloss.NewStyle().
		Foreground(lipgloss.Color("252"))

	// 进度
	Progress = lipgloss.NewStyle().
		Foreground(lipgloss.Color("46"))

	// 数字
	Num = lipgloss.NewStyle().
		Bold(true).
		Foreground(lipgloss.Color("33"))
)

// 格式化输出
func Success(msg string) string {
	return OK.Render("✓") + " " + msg
}

func Error(msg string) string {
	return Err.Render("✗") + " " + msg
}

func Warning(msg string) string {
	return Warn.Render("⚠") + " " + msg
}

func InfoMsg(msg string) string {
	return Info.Render("ℹ") + " " + msg
}

func DirIcon(name string) string {
	return Dir.Render("📁") + " " + name + "/"
}

func FileIcon(name string) string {
	return File.Render("📄") + " " + name
}

func ProgressBar(current, total int, width int) string {
	if total == 0 {
		return ""
	}
	percent := float64(current) / float64(total)
	filled := int(percent * float64(width))
	empty := width - filled

	bar := strings.Repeat("█", filled) + strings.Repeat("░", empty)
	return fmt.Sprintf("[%s] %d/%d", Progress.Render(bar), current, total)
}

func init() {
	// 确保终端支持颜色
	lipgloss.SetColorProfile(1)
}
