package ai

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"time"

	"github.com/sashabaranov/go-openai"
)

type OpenAIProvider struct {
	client *openai.Client
	model  string
}

// NewOpenAIProvider 创建 OpenAI 提供商
// 自动读取 HTTP_PROXY / HTTPS_PROXY / ALL_PROXY 环境变量，支持国内代理
func NewOpenAIProvider(apiKey, baseURL, model string) *OpenAIProvider {
	cfg := openai.DefaultConfig(apiKey)
	if baseURL != "" {
		cfg.BaseURL = baseURL
	}

	// 构建支持代理的 HTTP 客户端
	cfg.HTTPClient = newProxyHTTPClient()

	return &OpenAIProvider{
		client: openai.NewClientWithConfig(cfg),
		model:  model,
	}
}

// newProxyHTTPClient 构建自动检测代理的 HTTP 客户端
// 优先级：HTTP_PROXY_URL 环境变量 > HTTP_PROXY/HTTPS_PROXY 系统变量 > 直连
func newProxyHTTPClient() *http.Client {
	transport := &http.Transport{
		Proxy:               proxyFromEnv(),
		TLSHandshakeTimeout: 10 * time.Second,
	}
	return &http.Client{
		Transport: transport,
		Timeout:   120 * time.Second,
	}
}

// proxyFromEnv 按优先级读取代理配置
func proxyFromEnv() func(*http.Request) (*url.URL, error) {
	// 1. 优先读取自定义变量 HTTP_PROXY_URL（精确控制，不影响其他程序）
	if proxyURL := os.Getenv("HTTP_PROXY_URL"); proxyURL != "" {
		u, err := url.Parse(proxyURL)
		if err == nil {
			return http.ProxyURL(u)
		}
	}
	// 2. 读取标准系统代理变量（Clash/V2Ray 等工具设置的）
	return http.ProxyFromEnvironment
}

func (p *OpenAIProvider) Name() string { return "openai" }

func (p *OpenAIProvider) Complete(ctx context.Context, req *CompletionRequest) (*CompletionResponse, error) {
	msgs := make([]openai.ChatCompletionMessage, len(req.Messages))
	for i, m := range req.Messages {
		msgs[i] = openai.ChatCompletionMessage{Role: m.Role, Content: m.Content}
	}

	resp, err := p.client.CreateChatCompletion(ctx, openai.ChatCompletionRequest{
		Model:       p.model,
		Messages:    msgs,
		MaxTokens:   req.MaxTokens,
		Temperature: float32(req.Temperature),
	})
	if err != nil {
		return nil, fmt.Errorf("openai complete: %w", err)
	}

	return &CompletionResponse{
		Content:    resp.Choices[0].Message.Content,
		TokensUsed: resp.Usage.TotalTokens,
		Model:      resp.Model,
	}, nil
}

func (p *OpenAIProvider) Embed(ctx context.Context, req *EmbeddingRequest) (*EmbeddingResponse, error) {
	model := openai.AdaEmbeddingV2
	if req.Model != "" {
		model = openai.EmbeddingModel(req.Model)
	}

	resp, err := p.client.CreateEmbeddings(ctx, openai.EmbeddingRequest{
		Input: req.Texts,
		Model: model,
	})
	if err != nil {
		return nil, fmt.Errorf("openai embed: %w", err)
	}

	embeddings := make([][]float32, len(resp.Data))
	for i, d := range resp.Data {
		embeddings[i] = d.Embedding
	}
	return &EmbeddingResponse{Embeddings: embeddings, Model: string(model)}, nil
}

func (p *OpenAIProvider) CompleteStream(ctx context.Context, req *CompletionRequest, handler StreamHandler) error {
	msgs := make([]openai.ChatCompletionMessage, len(req.Messages))
	for i, m := range req.Messages {
		msgs[i] = openai.ChatCompletionMessage{Role: m.Role, Content: m.Content}
	}

	stream, err := p.client.CreateChatCompletionStream(ctx, openai.ChatCompletionRequest{
		Model:       p.model,
		Messages:    msgs,
		MaxTokens:   req.MaxTokens,
		Temperature: float32(req.Temperature),
		Stream:      true,
	})
	if err != nil {
		return fmt.Errorf("openai stream: %w", err)
	}
	defer stream.Close()

	for {
		resp, err := stream.Recv()
		if err != nil {
			break
		}
		if len(resp.Choices) > 0 {
			if err := handler(resp.Choices[0].Delta.Content); err != nil {
				return err
			}
		}
	}
	return nil
}
