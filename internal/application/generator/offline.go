package generator

// 离线规则引擎 - 无需 AI Key、无需 ES 即可生成命令。
// 知识库（59 条官方命令）通过 //go:embed 编译进二进制，完全离线可用，与 ES 同一份数据。
import (
	"context"
	"embed"
	"fmt"
	"sort"
	"strings"
	"unicode"

	"github.com/A-small-character/cmdgen-platform/internal/domain/command"
	"gopkg.in/yaml.v3"
)

//go:embed kb/*.yaml
var kbFS embed.FS

// OfflineEngine 基于内置知识库 + 规则库的离线命令生成器
type OfflineEngine struct {
	entries []kbEntry // 来自 embed YAML 的官方知识库
	rules   []offlineRule
}

// kbEntry 知识库条目（embed 自 YAML）
type kbEntry struct {
	Title      string
	Category   command.Category
	Tags       []string
	Source     string
	SourceURL  string
	Version    string
	Vendor     string
	Content    string
	searchText string // 预处理的可搜索文本（小写，去标点）
}

type kbFile struct {
	KnowledgeItems []struct {
		Title     string   `yaml:"title"`
		Type      string   `yaml:"type"`
		Tags      []string `yaml:"tags"`
		Source    string   `yaml:"source"`
		SourceURL string   `yaml:"source_url"`
		Version   string   `yaml:"version"`
		Vendor    string   `yaml:"vendor"`
		Content   string   `yaml:"content"`
	} `yaml:"knowledge_items"`
}

type offlineRule struct {
	keywords []string
	category command.Category
	commands []command.CommandItem
	explain  string
}

func NewOfflineEngine() *OfflineEngine {
	e := &OfflineEngine{}
	e.loadEmbeddedKB()
	e.loadRules()
	return e
}

// loadEmbeddedKB 加载 embed 的 YAML 知识库
func (e *OfflineEngine) loadEmbeddedKB() {
	files, _ := kbFS.ReadDir("kb")
	for _, f := range files {
		data, err := kbFS.ReadFile("kb/" + f.Name())
		if err != nil {
			continue
		}
		var kf kbFile
		if err := yaml.Unmarshal(data, &kf); err != nil {
			continue
		}
		defaultCat := command.Category(strings.TrimSuffix(f.Name(), ".yaml"))
		for _, item := range kf.KnowledgeItems {
			cat := command.Category(item.Type)
			if cat == "" {
				cat = defaultCat
			}
			entry := kbEntry{
				Title:     item.Title,
				Category:  cat,
				Tags:      item.Tags,
				Source:    item.Source,
				SourceURL: item.SourceURL,
				Version:   item.Version,
				Vendor:    item.Vendor,
				Content:   item.Content,
			}
			// 可搜索文本：标题(权重×3) + 标签 + 厂商 + 正文
			entry.searchText = normalize(
				item.Title + " " + item.Title + " " + item.Title + " " +
					strings.Join(item.Tags, " ") + " " + item.Vendor + " " + item.Content,
			)
			e.entries = append(e.entries, entry)
		}
	}
}

// KnowledgeCount 返回内置知识库条目数（用于状态展示）
func (e *OfflineEngine) KnowledgeCount() int { return len(e.entries) }

// Generate 离线生成：优先匹配 embed 知识库（59条官方命令），再回退硬编码规则
func (e *OfflineEngine) Generate(_ context.Context, input string, cat command.Category) (*command.GenerateResult, error) {
	if entry, score := e.bestMatch(input, cat); entry != nil && score > 0 {
		return e.entryToResult(entry), nil
	}
	if result := e.matchRules(input, cat); result != nil {
		return result, nil
	}
	return nil, fmt.Errorf("离线知识库未找到匹配，请尝试更具体的关键词，或在设置中配置 AI/ES")
}

// bestMatch 在知识库中按词频评分，返回最佳匹配
func (e *OfflineEngine) bestMatch(input string, cat command.Category) (*kbEntry, int) {
	tokens := tokenize(normalize(input))
	if len(tokens) == 0 {
		return nil, 0
	}
	type scored struct {
		idx   int
		score int
	}
	var ranked []scored
	for i := range e.entries {
		ent := &e.entries[i]
		catBonus := 0
		if cat != "" && ent.Category == cat {
			catBonus = 3
		}
		score := 0
		for _, tk := range tokens {
			score += strings.Count(ent.searchText, tk)
		}
		if score > 0 {
			ranked = append(ranked, scored{i, score + catBonus})
		}
	}
	if len(ranked) == 0 {
		return nil, 0
	}
	sort.Slice(ranked, func(a, b int) bool { return ranked[a].score > ranked[b].score })
	best := ranked[0]
	if best.score < 2 { // 阈值：避免只命中 1 个常用字就返回
		return nil, 0
	}
	return &e.entries[best.idx], best.score
}

// entryToResult 将知识库条目转为生成结果
func (e *OfflineEngine) entryToResult(ent *kbEntry) *command.GenerateResult {
	result := command.NewGenerateResult("offline-kb", ent.Category)
	format := command.FormatCLI
	if ent.Category == command.CategoryElasticsearch {
		format = command.FormatKibana
	}
	result.Commands = []command.CommandItem{{
		ID:          "1",
		Title:       ent.Title,
		Command:     strings.TrimSpace(ent.Content),
		Format:      format,
		Explanation: ent.Source,
		Risk:        command.RiskLow,
	}}
	parts := []string{}
	if ent.Source != "" {
		parts = append(parts, "来源："+ent.Source)
	}
	if ent.SourceURL != "" {
		parts = append(parts, "文档："+ent.SourceURL)
	}
	if ent.Version != "" {
		parts = append(parts, "版本："+ent.Version)
	}
	result.Explanation = strings.Join(parts, "  |  ")
	if ent.SourceURL != "" {
		result.References = []command.Reference{{Title: ent.Source, URL: ent.SourceURL}}
	}
	result.Metadata = command.ResultMetadata{AIProvider: "offline-kb", ModelName: "embedded-knowledge"}
	return result
}

// matchRules 硬编码规则匹配（知识库未命中时的补充）
func (e *OfflineEngine) matchRules(input string, cat command.Category) *command.GenerateResult {
	lower := normalize(input)
	var best []command.CommandItem
	var bestExplain string
	bestScore := 0
	for _, rule := range e.rules {
		if cat != "" && rule.category != cat {
			continue
		}
		score := 0
		for _, kw := range rule.keywords {
			if strings.Contains(lower, kw) {
				score++
			}
		}
		if score > bestScore {
			bestScore = score
			best = rule.commands
			bestExplain = rule.explain
		}
	}
	if bestScore == 0 {
		return nil
	}
	result := command.NewGenerateResult("offline-rule", cat)
	result.Commands = best
	result.Explanation = bestExplain
	result.Metadata = command.ResultMetadata{AIProvider: "offline-rule", ModelName: "rule-engine-v1"}
	return result
}

// ─── 文本处理 ───

// normalize 转小写 + 标点/空白转空格
func normalize(s string) string {
	return strings.Map(func(r rune) rune {
		if unicode.IsPunct(r) || unicode.IsSpace(r) {
			return ' '
		}
		return unicode.ToLower(r)
	}, s)
}

// tokenize 分词：ASCII 按空白切词，中文按单字切（适配无分词器场景）
func tokenize(s string) []string {
	var tokens []string
	var buf strings.Builder
	flush := func() {
		if buf.Len() > 0 {
			tokens = append(tokens, buf.String())
			buf.Reset()
		}
	}
	for _, r := range s {
		switch {
		case r == ' ':
			flush()
		case r > 127:
			flush()
			tokens = append(tokens, string(r))
		default:
			buf.WriteRune(r)
		}
	}
	flush()
	// 过滤单个 ASCII 字符（噪音），保留中文单字和多字符英文词
	out := tokens[:0]
	for _, t := range tokens {
		if len([]rune(t)) == 1 && t[0] < 128 {
			continue
		}
		out = append(out, t)
	}
	return out
}

// loadRules 硬编码规则（知识库的口语化补充兜底）
func (e *OfflineEngine) loadRules() {
	e.rules = []offlineRule{
		{
			keywords: []string{"端口", "占用", "kill", "杀", "port"},
			category: command.CategoryLinux,
			explain:  "查找并终止占用端口的进程",
			commands: []command.CommandItem{
				{ID: "1", Title: "查看端口占用进程", Command: "ss -tlnp | grep :8080", Format: command.FormatCLI, Risk: command.RiskLow, Explanation: "把 8080 换成目标端口"},
				{ID: "2", Title: "终止占用进程", Command: "kill -9 $(lsof -t -i:8080)", Format: command.FormatCLI, Risk: command.RiskHigh, Explanation: "强制终止占用 8080 端口的进程"},
			},
		},
	}
}
