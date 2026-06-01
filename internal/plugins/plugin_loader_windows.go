//go:build windows

package plugins

import (
	"github.com/cmdgen/platform/pkg/logger"
	"go.uber.org/zap"
)

// loadFromDirImpl Windows 实现：不支持 .so 插件，仅使用内置注册模式
//
// 在 Windows 上扩展新命令类型的方式：
//  1. 实现 GeneratorPlugin 接口
//  2. 在 cmd/server/main.go 中调用 pluginMgr.RegisterGeneratorPlugin(yourPlugin)
func (m *Manager) loadFromDirImpl() error {
	logger.Info("Windows 平台：.so 插件不可用，请使用 RegisterGeneratorPlugin 进行内置注册",
		zap.String("plugin_dir", m.pluginDir),
	)
	return nil
}
