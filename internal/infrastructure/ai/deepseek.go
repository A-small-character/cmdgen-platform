package ai

import (
	"context"
)

// DeepSeekProvider 使用 OpenAI 兼容接口
type DeepSeekProvider struct {
	inner *OpenAIProvider
}

func NewDeepSeekProvider(apiKey, baseURL, model string) *DeepSeekProvider {
	if baseURL == "" {
		baseURL = "https://api.deepseek.com/v1"
	}
	return &DeepSeekProvider{inner: NewOpenAIProvider(apiKey, baseURL, model)}
}

func (p *DeepSeekProvider) Name() string { return "deepseek" }

func (p *DeepSeekProvider) Complete(ctx context.Context, req *CompletionRequest) (*CompletionResponse, error) {
	return p.inner.Complete(ctx, req)
}

func (p *DeepSeekProvider) Embed(ctx context.Context, req *EmbeddingRequest) (*EmbeddingResponse, error) {
	// DeepSeek暂不提供公开Embedding接口，回退到本地处理
	return nil, nil
}
