package generator

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"text/template"
	"time"

	"github.com/cmdgen/platform/internal/domain/command"
	"github.com/cmdgen/platform/internal/infrastructure/ai"
	"github.com/cmdgen/platform/pkg/config"
)

// LinuxGenerator Linux命令生成器
type LinuxGenerator struct {
	aiManager *ai.Manager
	cfg       *config.AIConfig
}

func NewLinuxGenerator(aiManager *ai.Manager, cfg *config.AIConfig) *LinuxGenerator {
	return &LinuxGenerator{aiManager: aiManager, cfg: cfg}
}

func (g *LinuxGenerator) SupportedCategory() command.Category {
	return command.CategoryLinux
}

func (g *LinuxGenerator) Generate(ctx context.Context, req *command.GenerateRequest) (*command.GenerateResult, error) {
	start := time.Now()
	result := command.NewGenerateResult(req.ID, command.CategoryLinux)

	var promptTpl string
	if isFileModifyRequest(req.UserInput) {
		promptTpl = LinuxFileModifyPrompt
	} else {
		promptTpl = LinuxCommandPrompt
	}

	prompt, err := renderTemplate(promptTpl, map[string]interface{}{
		"UserInput": req.UserInput,
		"OSFamily":  string(req.Options.OSFamily),
		"OSVersion": req.Options.OSVersion,
	})
	if err != nil {
		return nil, fmt.Errorf("render template: %w", err)
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
		Temperature: g.cfg.Temperature,
	})
	if err != nil {
		return nil, fmt.Errorf("AI completion: %w", err)
	}

	if err := parseAIResponse(aiResp.Content, result); err != nil {
		// 如果JSON解析失败，将原始内容作为说明
		result.Explanation = aiResp.Content
		result.Commands = []command.CommandItem{{
			ID:          "1",
			Title:       "生成结果",
			Command:     extractCommandFromText(aiResp.Content),
			Format:      command.FormatCLI,
			Explanation: aiResp.Content,
			Risk:        command.RiskLow,
		}}
	}

	result.Metadata = command.ResultMetadata{
		AIProvider: provider,
		ModelName:  aiResp.Model,
		TokensUsed: aiResp.TokensUsed,
		Latency:    time.Since(start),
	}

	return result, nil
}

// NetworkGenerator 网络设备命令生成器
type NetworkGenerator struct {
	aiManager *ai.Manager
	cfg       *config.AIConfig
}

func NewNetworkGenerator(aiManager *ai.Manager, cfg *config.AIConfig) *NetworkGenerator {
	return &NetworkGenerator{aiManager: aiManager, cfg: cfg}
}

func (g *NetworkGenerator) SupportedCategory() command.Category {
	return command.CategoryNetwork
}

func (g *NetworkGenerator) Generate(ctx context.Context, req *command.GenerateRequest) (*command.GenerateResult, error) {
	start := time.Now()
	result := command.NewGenerateResult(req.ID, command.CategoryNetwork)

	prompt, err := renderTemplate(NetworkCommandPrompt, map[string]interface{}{
		"UserInput":   req.UserInput,
		"Vendor":      string(req.Options.Vendor),
		"DeviceModel": req.Options.DeviceModel,
		"OSVersion":   req.Options.OSVersion,
		"MultiVendor": req.Options.MultiVendor,
	})
	if err != nil {
		return nil, fmt.Errorf("render template: %w", err)
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
		return nil, fmt.Errorf("AI completion: %w", err)
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

// ESGenerator Elasticsearch命令生成器
type ESGenerator struct {
	aiManager *ai.Manager
	cfg       *config.AIConfig
}

func NewESGenerator(aiManager *ai.Manager, cfg *config.AIConfig) *ESGenerator {
	return &ESGenerator{aiManager: aiManager, cfg: cfg}
}

func (g *ESGenerator) SupportedCategory() command.Category {
	return command.CategoryElasticsearch
}

func (g *ESGenerator) Generate(ctx context.Context, req *command.GenerateRequest) (*command.GenerateResult, error) {
	start := time.Now()
	result := command.NewGenerateResult(req.ID, command.CategoryElasticsearch)

	esVer := string(req.Options.ESVersion)
	if esVer == "" {
		esVer = "7.x"
	}
	outputFmt := string(req.Options.OutputFormat)
	if outputFmt == "" {
		outputFmt = "all"
	}

	prompt, err := renderTemplate(ESCommandPrompt, map[string]interface{}{
		"UserInput":    req.UserInput,
		"ESVersion":    esVer,
		"OutputFormat": outputFmt,
	})
	if err != nil {
		return nil, fmt.Errorf("render template: %w", err)
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
		return nil, fmt.Errorf("AI completion: %w", err)
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

// --- 辅助函数 ---

func renderTemplate(tpl string, data map[string]interface{}) (string, error) {
	t, err := template.New("").Parse(tpl)
	if err != nil {
		return "", err
	}
	var buf bytes.Buffer
	if err := t.Execute(&buf, data); err != nil {
		return "", err
	}
	return buf.String(), nil
}

type aiResultJSON struct {
	Commands    []command.CommandItem `json:"commands"`
	Explanation string                `json:"explanation"`
	Warnings    []command.Warning     `json:"warnings"`
	ConfigExample string              `json:"config_example,omitempty"`
	VendorDiff  string                `json:"vendor_diff,omitempty"`
}

func parseAIResponse(content string, result *command.GenerateResult) error {
	// 尝试从Markdown代码块中提取JSON
	jsonStr := extractJSON(content)
	if jsonStr == "" {
		jsonStr = content
	}

	var parsed aiResultJSON
	if err := json.Unmarshal([]byte(jsonStr), &parsed); err != nil {
		return err
	}

	for i := range parsed.Commands {
		if parsed.Commands[i].ID == "" {
			parsed.Commands[i].ID = fmt.Sprintf("%d", i+1)
		}
	}

	result.Commands = parsed.Commands
	result.Explanation = parsed.Explanation
	result.Warnings = parsed.Warnings
	return nil
}

func extractJSON(s string) string {
	start := -1
	depth := 0
	for i, c := range s {
		if c == '{' {
			if depth == 0 {
				start = i
			}
			depth++
		} else if c == '}' {
			depth--
			if depth == 0 && start >= 0 {
				return s[start : i+1]
			}
		}
	}
	return ""
}

func extractCommandFromText(text string) string {
	// 简单提取代码块中的内容
	for _, delim := range []string{"```bash\n", "```shell\n", "```\n"} {
		if idx := bytes.Index([]byte(text), []byte(delim)); idx >= 0 {
			start := idx + len(delim)
			end := bytes.Index([]byte(text[start:]), []byte("```"))
			if end >= 0 {
				return text[start : start+end]
			}
		}
	}
	return text
}

func isFileModifyRequest(input string) bool {
	keywords := []string{"修改", "配置文件", "config", "conf", "nginx", "sshd", "sysctl", "limits"}
	for _, kw := range keywords {
		if bytes.Contains([]byte(input), []byte(kw)) {
			return true
		}
	}
	return false
}
