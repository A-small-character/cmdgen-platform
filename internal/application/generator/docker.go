package generator

import (
	"context"
	"strings"
	"time"

	"github.com/cmdgen/platform/internal/domain/command"
	"github.com/cmdgen/platform/internal/infrastructure/ai"
	"github.com/cmdgen/platform/pkg/config"
)

// DockerGenerator Docker命令生成器
type DockerGenerator struct {
	aiManager *ai.Manager
	cfg       *config.AIConfig
}

func NewDockerGenerator(aiManager *ai.Manager, cfg *config.AIConfig) *DockerGenerator {
	return &DockerGenerator{aiManager: aiManager, cfg: cfg}
}

func (g *DockerGenerator) SupportedCategory() command.Category {
	return command.CategoryDocker
}

func (g *DockerGenerator) Generate(ctx context.Context, req *command.GenerateRequest) (*command.GenerateResult, error) {
	start := time.Now()
	result := command.NewGenerateResult(req.ID, command.CategoryDocker)

	// 判断是 Dockerfile 生成还是 CLI 命令生成
	var promptTpl string
	if isDockerfileRequest(req.UserInput) {
		promptTpl = DockerfilePrompt
	} else {
		promptTpl = DockerCommandPrompt
	}

	ver := req.Options.DockerVersion
	if ver == "" {
		ver = "27"
	}
	runtime := req.Options.Runtime
	if runtime == "" {
		runtime = "docker"
	}

	prompt, err := renderTemplate(promptTpl, map[string]interface{}{
		"UserInput":     req.UserInput,
		"DockerVersion": ver,
		"Runtime":       runtime,
	})
	if err != nil {
		return nil, err
	}

	provider := req.Options.AIProvider
	if provider == "" {
		provider = g.cfg.DefaultProvider
	}

	// 注入 RAG 上下文
	if ragCtx, ok := req.Context["rag_context"]; ok && ragCtx != "" {
		prompt = "参考以下官方文档片段：\n" + ragCtx + "\n\n" + prompt
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

func isDockerfileRequest(input string) bool {
	keywords := []string{"dockerfile", "镜像构建", "build image", "多阶段", "multistage", "dockerignore"}
	lower := strings.ToLower(input)
	for _, kw := range keywords {
		if strings.Contains(lower, kw) {
			return true
		}
	}
	return false
}
