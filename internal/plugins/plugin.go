// Package plugins 实现跨平台插件系统。
//
// 插件加载策略：
//   - Linux / macOS: 支持 .so 动态库插件（go build -buildmode=plugin）
//   - Windows:       不支持 .so，仅支持"内置注册"模式，
//                    即直接调用 RegisterGeneratorPlugin / RegisterMiddlewarePlugin
//                    在程序启动时注入自定义实现。
package plugins

import (
	"context"
	"fmt"
	"sync"

	"github.com/A-small-character/cmdgen-platform/internal/domain/command"
	"github.com/A-small-character/cmdgen-platform/pkg/logger"
	"go.uber.org/zap"
)

// ─── 插件接口定义 ────────────────────────────────────────────────────────────

// Plugin 所有插件必须实现的基础接口
type Plugin interface {
	Name() string
	Version() string
	Description() string
	Initialize(config map[string]interface{}) error
	Shutdown() error
}

// GeneratorPlugin 命令生成器插件
type GeneratorPlugin interface {
	Plugin
	command.Generator
}

// MiddlewarePlugin 前置/后置处理插件
type MiddlewarePlugin interface {
	Plugin
	PreProcess(ctx context.Context, req *command.GenerateRequest) error
	PostProcess(ctx context.Context, result *command.GenerateResult) error
}

// PluginManifest 插件元数据（用于 API 展示）
type PluginManifest struct {
	Name        string `json:"name"`
	Version     string `json:"version"`
	Description string `json:"description"`
	Category    string `json:"category"` // "generator" 或 "middleware"
}

// ─── 插件管理器 ───────────────────────────────────────────────────────────────

// Manager 插件管理器（线程安全）
type Manager struct {
	mu                sync.RWMutex
	generatorPlugins  map[string]GeneratorPlugin
	middlewarePlugins []MiddlewarePlugin
	pluginDir         string
}

func NewManager(pluginDir string) *Manager {
	return &Manager{
		generatorPlugins:  make(map[string]GeneratorPlugin),
		middlewarePlugins: make([]MiddlewarePlugin, 0),
		pluginDir:         pluginDir,
	}
}

// LoadFromDir 从目录加载插件（平台相关实现在 plugin_loader_unix.go / plugin_loader_windows.go）
func (m *Manager) LoadFromDir() error {
	return m.loadFromDirImpl()
}

// RegisterGeneratorPlugin 注册命令生成器插件（内置注册，全平台可用）
func (m *Manager) RegisterGeneratorPlugin(p GeneratorPlugin) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.generatorPlugins[string(p.SupportedCategory())] = p
	logger.Info("注册生成器插件",
		zap.String("name", p.Name()),
		zap.String("category", string(p.SupportedCategory())),
	)
}

// RegisterMiddlewarePlugin 注册中间件插件（全平台可用）
func (m *Manager) RegisterMiddlewarePlugin(p MiddlewarePlugin) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.middlewarePlugins = append(m.middlewarePlugins, p)
	logger.Info("注册中间件插件", zap.String("name", p.Name()))
}

// GetGeneratorPlugin 获取生成器插件
func (m *Manager) GetGeneratorPlugin(category command.Category) (GeneratorPlugin, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	p, ok := m.generatorPlugins[string(category)]
	return p, ok
}

// RunPreProcess 执行所有前置处理中间件
func (m *Manager) RunPreProcess(ctx context.Context, req *command.GenerateRequest) error {
	m.mu.RLock()
	defer m.mu.RUnlock()
	for _, mw := range m.middlewarePlugins {
		if err := mw.PreProcess(ctx, req); err != nil {
			return fmt.Errorf("插件 %s PreProcess 失败: %w", mw.Name(), err)
		}
	}
	return nil
}

// RunPostProcess 执行所有后置处理中间件（失败只记录日志，不中断流程）
func (m *Manager) RunPostProcess(ctx context.Context, result *command.GenerateResult) error {
	m.mu.RLock()
	defer m.mu.RUnlock()
	for _, mw := range m.middlewarePlugins {
		if err := mw.PostProcess(ctx, result); err != nil {
			logger.Warn("插件 PostProcess 失败（已跳过）",
				zap.String("plugin", mw.Name()),
				zap.Error(err),
			)
		}
	}
	return nil
}

// ListPlugins 列出所有已注册插件的元数据
func (m *Manager) ListPlugins() []PluginManifest {
	m.mu.RLock()
	defer m.mu.RUnlock()

	manifests := make([]PluginManifest, 0, len(m.generatorPlugins)+len(m.middlewarePlugins))
	for _, p := range m.generatorPlugins {
		manifests = append(manifests, PluginManifest{
			Name:        p.Name(),
			Version:     p.Version(),
			Description: p.Description(),
			Category:    "generator",
		})
	}
	for _, p := range m.middlewarePlugins {
		manifests = append(manifests, PluginManifest{
			Name:        p.Name(),
			Version:     p.Version(),
			Description: p.Description(),
			Category:    "middleware",
		})
	}
	return manifests
}
