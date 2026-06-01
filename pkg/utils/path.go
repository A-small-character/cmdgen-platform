// Package utils 提供跨平台通用工具函数
package utils

import (
	"os"
	"path/filepath"
	"runtime"
)

// DataDir 返回平台适配的数据目录路径
// Windows: %APPDATA%\cmdgen  或  程序同级 data\
// Linux/macOS: ./data
func DataDir(subdir string) string {
	if runtime.GOOS == "windows" {
		appData := os.Getenv("APPDATA")
		if appData != "" {
			return filepath.Join(appData, "cmdgen", subdir)
		}
	}
	return filepath.Join("data", subdir)
}

// EnsureDir 确保目录存在，不存在则创建（跨平台）
func EnsureDir(path string) error {
	return os.MkdirAll(path, 0755)
}

// ExecutableDir 返回当前可执行文件所在目录
func ExecutableDir() string {
	exe, err := os.Executable()
	if err != nil {
		return "."
	}
	return filepath.Dir(exe)
}

// ToSlash 将路径中的反斜杠转为正斜杠（用于 URL/配置文件中的路径）
func ToSlash(path string) string {
	return filepath.ToSlash(path)
}
