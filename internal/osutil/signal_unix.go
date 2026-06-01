//go:build !windows

package osutil

import (
	"os"
	"syscall"
)

// ShutdownSignals 返回触发优雅关闭的信号列表（Unix/Linux/macOS）
func ShutdownSignals() []os.Signal {
	return []os.Signal{syscall.SIGINT, syscall.SIGTERM}
}
