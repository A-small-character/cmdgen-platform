//go:build windows

package osutil

import "os"

// ShutdownSignals 返回触发优雅关闭的信号列表（Windows 只支持 os.Interrupt）
func ShutdownSignals() []os.Signal {
	return []os.Signal{os.Interrupt}
}
