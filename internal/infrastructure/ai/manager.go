package ai

import (
	"context"
	"fmt"
	"sync"

	"github.com/A-small-character/cmdgen-platform/pkg/config"
)

// Manager AI提供商管理器
type Manager struct {
	mu        sync.RWMutex
	providers map[string]Provider
	defaultP  string
}

func NewManager(cfg *config.AIConfig) *Manager {
	m := &Manager{
		providers: make(map[string]Provider),
		defaultP:  cfg.DefaultProvider,
	}

	if cfg.OpenAI.APIKey != "" {
		m.Register(NewOpenAIProvider(cfg.OpenAI.APIKey, cfg.OpenAI.BaseURL, cfg.OpenAI.Model))
	}
	if cfg.Claude.APIKey != "" {
		m.Register(NewClaudeProvider(cfg.Claude.APIKey, cfg.Claude.BaseURL, cfg.Claude.Model))
	}
	if cfg.DeepSeek.APIKey != "" {
		m.Register(NewDeepSeekProvider(cfg.DeepSeek.APIKey, cfg.DeepSeek.BaseURL, cfg.DeepSeek.Model))
	}
	if cfg.Ollama.BaseURL != "" {
		m.Register(NewOllamaProvider(cfg.Ollama.BaseURL, cfg.Ollama.Model))
	}

	return m
}

func (m *Manager) Register(p Provider) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.providers[p.Name()] = p
}

func (m *Manager) Get(name string) (Provider, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	if name == "" {
		name = m.defaultP
	}
	p, ok := m.providers[name]
	if !ok {
		return nil, fmt.Errorf("AI provider %q not found", name)
	}
	return p, nil
}

func (m *Manager) Complete(ctx context.Context, provider string, req *CompletionRequest) (*CompletionResponse, error) {
	p, err := m.Get(provider)
	if err != nil {
		return nil, err
	}
	return p.Complete(ctx, req)
}

func (m *Manager) Embed(ctx context.Context, texts []string) ([][]float32, error) {
	// 优先使用 openai embedding
	p, err := m.Get("openai")
	if err != nil {
		p, err = m.Get(m.defaultP)
		if err != nil {
			return nil, err
		}
	}
	resp, err := p.Embed(ctx, &EmbeddingRequest{Texts: texts})
	if err != nil {
		return nil, err
	}
	return resp.Embeddings, nil
}

func (m *Manager) ListProviders() []string {
	m.mu.RLock()
	defer m.mu.RUnlock()
	names := make([]string, 0, len(m.providers))
	for k := range m.providers {
		names = append(names, k)
	}
	return names
}
