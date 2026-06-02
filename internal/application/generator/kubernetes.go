package generator

import (
	"context"
	"time"

	"github.com/A-small-character/cmdgen-platform/internal/domain/command"
	"github.com/A-small-character/cmdgen-platform/internal/infrastructure/ai"
	"github.com/A-small-character/cmdgen-platform/pkg/config"
)

// KubernetesGenerator Kubernetes命令生成器
type KubernetesGenerator struct {
	aiManager *ai.Manager
	cfg       *config.AIConfig
}

func NewKubernetesGenerator(aiManager *ai.Manager, cfg *config.AIConfig) *KubernetesGenerator {
	return &KubernetesGenerator{aiManager: aiManager, cfg: cfg}
}

func (g *KubernetesGenerator) SupportedCategory() command.Category {
	return command.CategoryKubernetes
}

func (g *KubernetesGenerator) Generate(ctx context.Context, req *command.GenerateRequest) (*command.GenerateResult, error) {
	start := time.Now()
	result := command.NewGenerateResult(req.ID, command.CategoryKubernetes)

	k8sVer := req.Options.K8sVersion
	if k8sVer == "" {
		k8sVer = "1.31"
	}
	resource := req.Options.K8sResource
	if resource == "" {
		resource = "auto"
	}

	prompt, err := renderTemplate(KubernetesCommandPrompt, map[string]interface{}{
		"UserInput":   req.UserInput,
		"K8sVersion":  k8sVer,
		"K8sResource": resource,
		"OutputYAML":  req.Options.OutputYAML,
	})
	if err != nil {
		return nil, err
	}

	if ragCtx, ok := req.Context["rag_context"]; ok && ragCtx != "" {
		prompt = "参考以下官方文档片段：\n" + ragCtx + "\n\n" + prompt
	}

	provider := req.Options.AIProvider
	if provider == "" {
		provider = g.cfg.DefaultProvider
	}

	aiResp, err := g.aiManager.Complete(ctx, provider, &ai.CompletionRequest{
		Messages: []ai.Message{
			{Role: "system", Content: SystemPromptBase},
			{Role: "user", Content: prompt},
		},
		MaxTokens:   g.cfg.MaxTokens,
		Temperature: 0.05,
	})
	if err != nil {
		return nil, err
	}

	if err := parseAIResponse(aiResp.Content, result); err != nil {
		result.Explanation = aiResp.Content
	}

	result.Metadata = command.ResultMetadata{
		AIProvider: provider,
		ModelName:  aiResp.Model,
		TokensUsed: aiResp.TokensUsed,
		Latency:    time.Since(start),
	}
	return result, nil
}
