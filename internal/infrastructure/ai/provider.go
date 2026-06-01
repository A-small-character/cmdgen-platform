package ai

import "context"

// Message AI对话消息
type Message struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

// CompletionRequest 补全请求
type CompletionRequest struct {
	Messages    []Message `json:"messages"`
	MaxTokens   int       `json:"max_tokens"`
	Temperature float64   `json:"temperature"`
	Stream      bool      `json:"stream"`
}

// CompletionResponse 补全响应
type CompletionResponse struct {
	Content    string `json:"content"`
	TokensUsed int    `json:"tokens_used"`
	Model      string `json:"model"`
}

// EmbeddingRequest 向量嵌入请求
type EmbeddingRequest struct {
	Texts []string `json:"texts"`
	Model string   `json:"model"`
}

// EmbeddingResponse 向量嵌入响应
type EmbeddingResponse struct {
	Embeddings [][]float32 `json:"embeddings"`
	Model      string      `json:"model"`
}

// Provider AI提供商接口
type Provider interface {
	Name() string
	Complete(ctx context.Context, req *CompletionRequest) (*CompletionResponse, error)
	Embed(ctx context.Context, req *EmbeddingRequest) (*EmbeddingResponse, error)
}

// StreamHandler 流式响应处理器
type StreamHandler func(chunk string) error

// StreamProvider 支持流式响应的提供商
type StreamProvider interface {
	Provider
	CompleteStream(ctx context.Context, req *CompletionRequest, handler StreamHandler) error
}
