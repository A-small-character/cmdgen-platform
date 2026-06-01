// Package osutil 提供跨平台操作系统工具函数
package osutil

import "runtime"

// OSName 返回当前操作系统名称
func OSName() string {
	return runtime.GOOS
}

// IsWindows 判断是否运行在 Windows 上
func IsWindows() bool {
	return runtime.GOOS == "windows"
}

// IsLinux 判断是否运行在 Linux 上
func IsLinux() bool {
	return runtime.GOOS == "linux"
}

// IsMacOS 判断是否运行在 macOS 上
func IsMacOS() bool {
	return runtime.GOOS == "darwin"
}
