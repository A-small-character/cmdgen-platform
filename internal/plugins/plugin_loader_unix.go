//go:build !windows

package plugins

import (
	"fmt"
	"os"
	"path/filepath"
	"plugin"

	"github.com/cmdgen/platform/pkg/logger"
	"go.uber.org/zap"
)

// loadFromDirImpl Linux/macOS 实现：扫描目录加载 .so 插件
func (m *Manager) loadFromDirImpl() error {
	if m.pluginDir == "" {
		return nil
	}

	entries, err := os.ReadDir(m.pluginDir)
	if err != nil {
		if os.IsNotExist(err) {
			// 目录不存在不视为错误
			return nil
		}
		return fmt.Errorf("读取插件目录失败: %w", err)
	}

	loaded := 0
	for _, entry := range entries {
		if entry.IsDir() || filepath.Ext(entry.Name()) != ".so" {
			continue
		}

		soPath := filepath.Join(m.pluginDir, entry.Name())
		if loadErr := m.loadSoPlugin(soPath, nil); loadErr != nil {
			logger.Warn("加载插件失败",
				zap.String("file", entry.Name()),
				zap.Error(loadErr),
			)
			continue
		}
		loaded++
	}

	logger.Info("插件加载完成", zap.Int("loaded", loaded), zap.String("dir", m.pluginDir))
	return nil
}

// loadSoPlugin 加载单个 .so 插件文件
func (m *Manager) loadSoPlugin(path string, config map[string]interface{}) error {
	p, err := plugin.Open(path)
	if err != nil {
		return fmt.Errorf("打开插件失败: %w", err)
	}

	sym, err := p.Lookup("NewPlugin")
	if err != nil {
		return fmt.Errorf("插件缺少导出函数 NewPlugin: %w", err)
	}

	newPluginFn, ok := sym.(func() Plugin)
	if !ok {
		return fmt.Errorf("NewPlugin 函数签名应为 func() Plugin")
	}

	plug := newPluginFn()
	if initErr := plug.Initialize(config); initErr != nil {
		return fmt.Errorf("插件初始化失败: %w", initErr)
	}

	if genPlugin, ok := plug.(GeneratorPlugin); ok {
		m.RegisterGeneratorPlugin(genPlugin)
	}
	if mwPlugin, ok := plug.(MiddlewarePlugin); ok {
		m.RegisterMiddlewarePlugin(mwPlugin)
	}

	logger.Info("插件加载成功",
		zap.String("name", plug.Name()),
		zap.String("version", plug.Version()),
	)
	return nil
}
