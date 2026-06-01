package generator

import (
	"context"
	"strings"
	"time"

	"github.com/cmdgen/platform/internal/domain/command"
	"github.com/cmdgen/platform/internal/infrastructure/ai"
	"github.com/cmdgen/platform/pkg/config"
)

// MySQLGenerator MySQL命令生成器
type MySQLGenerator struct {
	aiManager *ai.Manager
	cfg       *config.AIConfig
}

func NewMySQLGenerator(aiManager *ai.Manager, cfg *config.AIConfig) *MySQLGenerator {
	return &MySQLGenerator{aiManager: aiManager, cfg: cfg}
}

func (g *MySQLGenerator) SupportedCategory() command.Category {
	return command.CategoryMySQL
}

func (g *MySQLGenerator) Generate(ctx context.Context, req *command.GenerateRequest) (*command.GenerateResult, error) {
	start := time.Now()
	result := command.NewGenerateResult(req.ID, command.CategoryMySQL)

	mysqlVer := req.Options.MySQLVersion
	if mysqlVer == "" {
		mysqlVer = "8.0"
	}
	engine := req.Options.MySQLEngine
	if engine == "" {
		engine = "InnoDB"
	}

	prompt, err := renderTemplate(MySQLCommandPrompt, map[string]interface{}{
		"UserInput":    req.UserInput,
		"MySQLVersion": mysqlVer,
		"MySQLEngine":  engine,
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

	// 对高风险MySQL操作自动补充风险警告
	for i, cmd := range result.Commands {
		if isDangerousMySQL(cmd.Command) {
			result.Commands[i].Risk = command.RiskHigh
			result.Warnings = append(result.Warnings, command.Warning{
				Level:   command.RiskHigh,
				Message: "该操作不可逆，执行前请确认已备份数据库：mysqldump -u root -p --all-databases > backup.sql",
			})
		}
	}

	result.Metadata = command.ResultMetadata{
		AIProvider: provider,
		ModelName:  aiResp.Model,
		TokensUsed: aiResp.TokensUsed,
		Latency:    time.Since(start),
	}
	return result, nil
}

func isDangerousMySQL(cmd string) bool {
	lower := strings.ToLower(cmd)
	dangerous := []string{"drop table", "drop database", "truncate", "delete from", "drop user"}
	for _, d := range dangerous {
		if strings.Contains(lower, d) {
			return true
		}
	}
	return false
}
