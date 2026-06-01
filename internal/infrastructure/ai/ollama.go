package ai

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
)

// OllamaProvider 本地LLM提供商
type OllamaProvider struct {
	baseURL string
	model   string
	client  *http.Client
}

func NewOllamaProvider(baseURL, model string) *OllamaProvider {
	if baseURL == "" {
		baseURL = "http://localhost:11434"
	}
	return &OllamaProvider{baseURL: baseURL, model: model, client: &http.Client{}}
}

func (p *OllamaProvider) Name() string { return "ollama" }

type ollamaGenerateReq struct {
	Model  string `json:"model"`
	Prompt string `json:"prompt"`
	Stream bool   `json:"stream"`
}

type ollamaGenerateResp struct {
	Response string `json:"response"`
	Done     bool   `json:"done"`
}

type ollamaEmbedReq struct {
	Model  string `json:"model"`
	Prompt string `json:"prompt"`
}

type ollamaEmbedResp struct {
	Embedding []float32 `json:"embedding"`
}

func (p *OllamaProvider) Complete(ctx context.Context, req *CompletionRequest) (*CompletionResponse, error) {
	prompt := buildPromptFromMessages(req.Messages)
	payload := ollamaGenerateReq{Model: p.model, Prompt: prompt, Stream: false}
	body, _ := json.Marshal(payload)

	httpReq, err := http.NewRequestWithContext(ctx, "POST", p.baseURL+"/api/generate", bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	httpReq.Header.Set("Content-Type", "application/json")

	resp, err := p.client.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("ollama request: %w", err)
	}
	defer resp.Body.Close()

	respBody, _ := io.ReadAll(resp.Body)
	var ollamaResp ollamaGenerateResp
	if err := json.Unmarshal(respBody, &ollamaResp); err != nil {
		return nil, err
	}

	return &CompletionResponse{Content: ollamaResp.Response, Model: p.model}, nil
}

func (p *OllamaProvider) Embed(ctx context.Context, req *EmbeddingRequest) (*EmbeddingResponse, error) {
	embeddings := make([][]float32, 0, len(req.Texts))
	for _, text := range req.Texts {
		emb, err := p.embedOne(ctx, text)
		if err != nil {
			return nil, err
		}
		embeddings = append(embeddings, emb)
	}
	return &EmbeddingResponse{Embeddings: embeddings, Model: p.model}, nil
}

// embedOne 单条文本嵌入，独立函数确保 resp.Body 及时关闭（避免 defer-in-loop）
func (p *OllamaProvider) embedOne(ctx context.Context, text string) ([]float32, error) {
	payload := ollamaEmbedReq{Model: p.model, Prompt: text}
	body, _ := json.Marshal(payload)

	httpReq, err := http.NewRequestWithContext(ctx, "POST", p.baseURL+"/api/embeddings", bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	httpReq.Header.Set("Content-Type", "application/json")

	resp, err := p.client.Do(httpReq)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close() // 函数返回时立即关闭，不在循环中积累

	respBody, _ := io.ReadAll(resp.Body)
	var ollamaResp ollamaEmbedResp
	if err := json.Unmarshal(respBody, &ollamaResp); err != nil {
		return nil, err
	}
	return ollamaResp.Embedding, nil
}

func buildPromptFromMessages(msgs []Message) string {
	var buf bytes.Buffer
	for _, m := range msgs {
		switch m.Role {
		case "system":
			buf.WriteString("System: " + m.Content + "\n\n")
		case "user":
			buf.WriteString("Human: " + m.Content + "\n\n")
		case "assistant":
			buf.WriteString("Assistant: " + m.Content + "\n\n")
		}
	}
	buf.WriteString("Assistant: ")
	return buf.String()
}
