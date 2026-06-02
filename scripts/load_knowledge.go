//go:build ignore

// 知识库灌入脚本 - 将 knowledge_base/*.yaml 写入 ES cmdgen_history 索引
// 这样无 AI Key 时也能通过 ES 命中返回官方命令；ES 与 AI 结果共用一个库。
//
// 用法: go run scripts/load_knowledge.go [es地址，默认 http://localhost:9200]
package main

import (
	"bytes"
	"context"
	"crypto/md5"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"gopkg.in/yaml.v3"
)

// 与 eshistory.Store 的索引结构保持一致
const index = "cmdgen_history"

type kbFile struct {
	KnowledgeItems []kbItem `yaml:"knowledge_items"`
}

type kbItem struct {
	Title     string   `yaml:"title"`
	Type      string   `yaml:"type"`
	Tags      []string `yaml:"tags"`
	Source    string   `yaml:"source"`
	SourceURL string   `yaml:"source_url"`
	Version   string   `yaml:"version"`
	Vendor    string   `yaml:"vendor"`
	Content   string   `yaml:"content"`
}

// 与 command.GenerateResult / CommandItem 对齐的最小结构
type genResult struct {
	ID          string        `json:"id"`
	Category    string        `json:"category"`
	Commands    []commandItem `json:"commands"`
	Explanation string        `json:"explanation"`
	References  []reference   `json:"references"`
	Metadata    metadata      `json:"metadata"`
	CreatedAt   time.Time     `json:"created_at"`
}
type commandItem struct {
	ID          string `json:"id"`
	Title       string `json:"title"`
	Command     string `json:"command"`
	Format      string `json:"format"`
	Explanation string `json:"explanation"`
	Risk        string `json:"risk"`
}
type reference struct {
	Title string `json:"title"`
	URL   string `json:"url"`
}
type metadata struct {
	AIProvider string `json:"ai_provider"`
	ModelName  string `json:"model_name"`
}

// authTransport 给所有请求注入 Basic Auth
type authTransport struct {
	user, pass string
	base       http.RoundTripper
}

func (t authTransport) RoundTrip(r *http.Request) (*http.Response, error) {
	if t.user != "" {
		r.SetBasicAuth(t.user, t.pass)
	}
	return t.base.RoundTrip(r)
}

func main() {
	// 用法: go run scripts/load_knowledge.go [esAddr] [username] [password]
	esAddr := "http://localhost:9200"
	if len(os.Args) > 1 {
		esAddr = os.Args[1]
	}
	user, pass := "", ""
	if len(os.Args) > 3 {
		user, pass = os.Args[2], os.Args[3]
	}
	esAddr = strings.TrimRight(esAddr, "/")
	client := &http.Client{
		Timeout:   15 * time.Second,
		Transport: authTransport{user: user, pass: pass, base: http.DefaultTransport},
	}

	files := []string{
		"knowledge_base/linux/commands.yaml",
		"knowledge_base/elasticsearch/commands.yaml",
		"knowledge_base/network/huawei_switch.yaml",
		"knowledge_base/docker/commands.yaml",
		"knowledge_base/kubernetes/commands.yaml",
		"knowledge_base/mysql/commands.yaml",
	}

	total := 0
	var bulk bytes.Buffer

	for _, f := range files {
		data, err := os.ReadFile(f)
		if err != nil {
			fmt.Printf("跳过 %s: %v\n", f, err)
			continue
		}
		var kf kbFile
		if err := yaml.Unmarshal(data, &kf); err != nil {
			fmt.Printf("解析失败 %s: %v\n", f, err)
			continue
		}

		cat := categoryFromPath(f)
		for _, item := range kf.KnowledgeItems {
			itemType := item.Type
			if itemType == "" {
				itemType = cat
			}
			// 用标题作为可查询 query；content 拆为命令展示
			result := genResult{
				ID:       fmt.Sprintf("%x", md5.Sum([]byte(item.Title))),
				Category: itemType,
				Commands: []commandItem{{
					ID:          "1",
					Title:       item.Title,
					Command:     extractCommands(item.Content),
					Format:      formatFor(itemType),
					Explanation: item.Source,
					Risk:        "low",
				}},
				Explanation: buildExplanation(item),
				References: []reference{
					{Title: item.Source, URL: item.SourceURL},
				},
				Metadata: metadata{AIProvider: "knowledge-base", ModelName: "official-docs"},
				CreatedAt: time.Now(),
			}
			resultJSON, _ := json.Marshal(result)

			doc := map[string]any{
				"query":       buildQueryText(item),
				"query_exact": item.Title,
				"category":    itemType,
				"provider":    "knowledge-base",
				"result_json": string(resultJSON),
				"hit_count":   0,
				"created_at":  time.Now().Format(time.RFC3339),
				"updated_at":  time.Now().Format(time.RFC3339),
			}
			// 稳定 doc id：避免重复灌入产生重复文档
			docID := fmt.Sprintf("kb_%x", md5.Sum([]byte(itemType+"|"+item.Title)))
			meta, _ := json.Marshal(map[string]any{
				"index": map[string]any{"_index": index, "_id": docID},
			})
			docBytes, _ := json.Marshal(doc)
			bulk.Write(meta)
			bulk.WriteByte('\n')
			bulk.Write(docBytes)
			bulk.WriteByte('\n')
			total++
		}
		fmt.Printf("读取 %s: %d 条\n", f, len(kf.KnowledgeItems))
	}

	if total == 0 {
		fmt.Println("没有可灌入的数据")
		return
	}

	// 确保索引存在
	ensureIndex(client, esAddr)

	// Bulk 写入
	req, _ := http.NewRequestWithContext(context.Background(), http.MethodPost,
		esAddr+"/_bulk", bytes.NewReader(bulk.Bytes()))
	req.Header.Set("Content-Type", "application/x-ndjson")
	resp, err := client.Do(req)
	if err != nil {
		fmt.Printf("Bulk 写入失败: %v\n", err)
		os.Exit(1)
	}
	defer resp.Body.Close()

	var br struct {
		Errors bool `json:"errors"`
		Items  []any `json:"items"`
	}
	json.NewDecoder(resp.Body).Decode(&br)
	if br.Errors {
		fmt.Println("部分文档写入有错误（可能是 mapping 冲突）")
	}
	// 刷新使立即可查
	client.Post(esAddr+"/"+index+"/_refresh", "application/json", nil)

	fmt.Printf("\n✓ 知识库灌入完成：共 %d 条写入 ES (%s/%s)\n", total, esAddr, index)

	// 验证
	cnt := count(client, esAddr)
	fmt.Printf("✓ 当前索引文档总数：%d\n", cnt)
}

func ensureIndex(client *http.Client, addr string) {
	mapping := `{
      "settings":{"number_of_shards":1,"number_of_replicas":1},
      "mappings":{"properties":{
        "query":{"type":"text"},
        "query_exact":{"type":"keyword"},
        "category":{"type":"keyword"},
        "provider":{"type":"keyword"},
        "result_json":{"type":"text","index":false},
        "hit_count":{"type":"integer"},
        "created_at":{"type":"date"},
        "updated_at":{"type":"date"}
      }}}`
	req, _ := http.NewRequest(http.MethodPut, addr+"/"+index, strings.NewReader(mapping))
	req.Header.Set("Content-Type", "application/json")
	resp, err := client.Do(req)
	if err == nil {
		resp.Body.Close()
	}
}

func count(client *http.Client, addr string) int {
	resp, err := client.Get(addr + "/" + index + "/_count")
	if err != nil {
		return -1
	}
	defer resp.Body.Close()
	var r struct {
		Count int `json:"count"`
	}
	json.NewDecoder(resp.Body).Decode(&r)
	return r.Count
}

func categoryFromPath(p string) string {
	dir := filepath.Base(filepath.Dir(p))
	switch dir {
	case "linux":
		return "linux"
	case "elasticsearch":
		return "elasticsearch"
	case "network":
		return "network"
	case "docker":
		return "docker"
	case "kubernetes":
		return "kubernetes"
	case "mysql":
		return "mysql"
	default:
		return "general"
	}
}

func formatFor(cat string) string {
	if cat == "elasticsearch" {
		return "kibana"
	}
	return "cli"
}

// extractCommands 从知识 content 中保留完整内容（含注释和命令），作为命令块展示
func extractCommands(content string) string {
	return strings.TrimSpace(content)
}

// buildQueryText 构造可匹配文本：标题(加权重复) + 标签 + 正文前若干字符
// 让用户口语化查询能匹配到标题、标签或正文中的关键词
func buildQueryText(item kbItem) string {
	var sb strings.Builder
	// 标题重复2次提升权重
	sb.WriteString(item.Title)
	sb.WriteString(" ")
	sb.WriteString(item.Title)
	sb.WriteString(" ")
	sb.WriteString(strings.Join(item.Tags, " "))
	sb.WriteString(" ")
	sb.WriteString(item.Vendor)
	sb.WriteString(" ")
	// 正文前 600 个 rune（含命令名和说明关键词）
	runes := []rune(item.Content)
	if len(runes) > 600 {
		runes = runes[:600]
	}
	sb.WriteString(string(runes))
	return sb.String()
}

func buildExplanation(item kbItem) string {
	parts := []string{}
	if item.Source != "" {
		parts = append(parts, "来源："+item.Source)
	}
	if item.SourceURL != "" {
		parts = append(parts, "文档："+item.SourceURL)
	}
	if item.Version != "" {
		parts = append(parts, "版本："+item.Version)
	}
	return strings.Join(parts, "  |  ")
}
