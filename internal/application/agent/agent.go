package agent

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/A-small-character/cmdgen-platform/internal/application/retriever"
	"github.com/A-small-character/cmdgen-platform/internal/domain/command"
	"github.com/A-small-character/cmdgen-platform/internal/domain/knowledge"
	"github.com/A-small-character/cmdgen-platform/internal/infrastructure/ai"
	"github.com/A-small-character/cmdgen-platform/pkg/config"
	"github.com/A-small-character/cmdgen-platform/pkg/logger"
	"go.uber.org/zap"
)

// ToolCall Agent工具调用
type ToolCall struct {
	Thought     string          `json:"thought"`
	Action      string          `json:"action"`
	ActionInput json.RawMessage `json:"action_input"`
}

// Tool Agent可调用工具
type Tool interface {
	Name() string
	Description() string
	Execute(ctx context.Context, input json.RawMessage) (interface{}, error)
}

// Agent ReAct模式的智能Agent
type Agent struct {
	aiManager  *ai.Manager
	ragService *retriever.RAGService
	tools      map[string]Tool
	maxIter    int
	cfg        *config.AIConfig
}

func NewAgent(aiManager *ai.Manager, ragService *retriever.RAGService, cfg *config.AIConfig) *Agent {
	a := &Agent{
		aiManager:  aiManager,
		ragService: ragService,
		tools:      make(map[string]Tool),
		maxIter:    5,
		cfg:        cfg,
	}
	a.registerDefaultTools()
	return a
}

func (a *Agent) registerDefaultTools() {
	a.RegisterTool(&KnowledgeSearchTool{ragService: a.ragService})
	a.RegisterTool(&WebDocSearchTool{})
	a.RegisterTool(&CategoryDetectTool{})
}

func (a *Agent) RegisterTool(t Tool) {
	a.tools[t.Name()] = t
}

// Run 执行Agent推理循环
func (a *Agent) Run(ctx context.Context, userInput string, provider string) (*command.GenerateResult, error) {
	if provider == "" {
		provider = a.cfg.DefaultProvider
	}

	history := []ai.Message{
		{Role: "system", Content: buildAgentSystemPrompt(a.tools)},
		{Role: "user", Content: userInput},
	}

	result := &command.GenerateResult{
		CreatedAt: time.Now(),
	}

	for iter := 0; iter < a.maxIter; iter++ {
		resp, err := a.aiManager.Complete(ctx, provider, &ai.CompletionRequest{
			Messages:    history,
			MaxTokens:   a.cfg.MaxTokens,
			Temperature: 0.1,
		})
		if err != nil {
			return nil, fmt.Errorf("agent iteration %d: %w", iter, err)
		}

		history = append(history, ai.Message{Role: "assistant", Content: resp.Content})

		var call ToolCall
		if err := json.Unmarshal([]byte(extractJSON(resp.Content)), &call); err != nil {
			// 不是JSON，视为最终答案
			result.Explanation = resp.Content
			break
		}

		if call.Action == "Final Answer" {
			if err := json.Unmarshal(call.ActionInput, result); err != nil {
				result.Explanation = string(call.ActionInput)
			}
			break
		}

		// 执行工具
		tool, ok := a.tools[call.Action]
		if !ok {
			toolResult := fmt.Sprintf("工具 %q 不存在，可用工具: %s", call.Action, strings.Join(a.listToolNames(), ", "))
			history = append(history, ai.Message{Role: "user", Content: "Observation: " + toolResult})
			continue
		}

		logger.Debug("Agent执行工具", zap.String("tool", call.Action))
		toolOutput, err := tool.Execute(ctx, call.ActionInput)
		var observation string
		if err != nil {
			observation = fmt.Sprintf("工具执行失败: %v", err)
		} else {
			obsBytes, _ := json.Marshal(toolOutput)
			observation = string(obsBytes)
		}

		history = append(history, ai.Message{
			Role:    "user",
			Content: fmt.Sprintf("Observation: %s\n请继续分析并给出下一步行动或最终答案。", observation),
		})
	}

	return result, nil
}

func (a *Agent) listToolNames() []string {
	names := make([]string, 0, len(a.tools))
	for k := range a.tools {
		names = append(names, k)
	}
	return names
}

// --- 内置工具 ---

type KnowledgeSearchTool struct {
	ragService *retriever.RAGService
}

func (t *KnowledgeSearchTool) Name() string { return "search_knowledge_base" }
func (t *KnowledgeSearchTool) Description() string {
	return "搜索本地知识库，获取命令示例和最佳实践"
}
func (t *KnowledgeSearchTool) Execute(ctx context.Context, input json.RawMessage) (interface{}, error) {
	var params struct {
		Query string `json:"query"`
		Type  string `json:"type"`
	}
	if err := json.Unmarshal(input, &params); err != nil {
		return nil, err
	}
	if t.ragService == nil {
		return map[string]string{"result": "知识库服务未初始化"}, nil
	}
	return t.ragService.Retrieve(ctx, &knowledge.SearchQuery{
		Query: params.Query,
		Type:  knowledge.KnowledgeType(params.Type),
		TopK:  5,
	})
}

type WebDocSearchTool struct{}

func (t *WebDocSearchTool) Name() string        { return "search_web_docs" }
func (t *WebDocSearchTool) Description() string { return "搜索官方在线文档" }
func (t *WebDocSearchTool) Execute(ctx context.Context, input json.RawMessage) (interface{}, error) {
	var params struct {
		Query  string `json:"query"`
		Source string `json:"source"`
	}
	if err := json.Unmarshal(input, &params); err != nil {
		return nil, err
	}
	// 实际会调用 WebCrawler，这里返回占位
	return map[string]string{
		"result": fmt.Sprintf("已搜索 %s: %s，请查阅官方文档获取最新信息", params.Source, params.Query),
	}, nil
}

type CategoryDetectTool struct{}

func (t *CategoryDetectTool) Name() string        { return "detect_category" }
func (t *CategoryDetectTool) Description() string { return "识别用户意图和命令类别" }
func (t *CategoryDetectTool) Execute(ctx context.Context, input json.RawMessage) (interface{}, error) {
	var params struct {
		Input string `json:"input"`
	}
	json.Unmarshal(input, &params)
	return map[string]string{"category": "linux", "confidence": "0.9"}, nil
}

func buildAgentSystemPrompt(tools map[string]Tool) string {
	var toolsDesc strings.Builder
	for _, t := range tools {
		toolsDesc.WriteString(fmt.Sprintf("- %s: %s\n", t.Name(), t.Description()))
	}
	return fmt.Sprintf(`你是智能命令生成Agent。

可用工具:
%s

每次回复必须是以下JSON格式之一：

执行工具:
{"thought": "分析思路", "action": "工具名称", "action_input": {参数}}

最终答案:
{"thought": "总结", "action": "Final Answer", "action_input": {命令生成结果}}`, toolsDesc.String())
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
	return s
}
