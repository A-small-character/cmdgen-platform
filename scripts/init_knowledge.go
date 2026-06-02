//go:build ignore

// 知识库初始化脚本
// 用法: go run scripts/init_knowledge.go --config configs/config.yaml
package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"time"

	"github.com/A-small-character/cmdgen-platform/internal/domain/knowledge"
	"github.com/A-small-character/cmdgen-platform/internal/infrastructure/ai"
	"github.com/A-small-character/cmdgen-platform/internal/infrastructure/vector"
	"github.com/A-small-character/cmdgen-platform/pkg/config"
	"gopkg.in/yaml.v3"
)

type KnowledgeFile struct {
	KnowledgeItems []knowledge.KnowledgeItem `yaml:"knowledge_items"`
}

func main() {
	cfg, err := config.Load("configs/config.yaml")
	if err != nil {
		fmt.Fprintf(os.Stderr, "加载配置失败: %v\n", err)
		os.Exit(1)
	}

	aiManager := ai.NewManager(&cfg.AI)
	esStore := vector.NewESVectorStore(&cfg.Vector.Elasticsearch)

	ctx := context.Background()

	// 确保索引存在
	if err := esStore.EnsureIndex(ctx, cfg.Vector.Elasticsearch.IndexPrefix+"_default", cfg.Vector.Dimension); err != nil {
		fmt.Printf("创建索引失败（可能已存在）: %v\n", err)
	}

	// 加载知识库文件
	kbFiles := []struct {
		path     string
		itemType knowledge.KnowledgeType
	}{
		{"knowledge_base/linux/commands.yaml", knowledge.KnowledgeLinux},
		{"knowledge_base/elasticsearch/commands.yaml", knowledge.KnowledgeElasticsearch},
		{"knowledge_base/network/huawei_switch.yaml", knowledge.KnowledgeNetwork},
		{"knowledge_base/docker/commands.yaml", knowledge.KnowledgeDocker},
		{"knowledge_base/kubernetes/commands.yaml", knowledge.KnowledgeKubernetes},
		{"knowledge_base/mysql/commands.yaml", knowledge.KnowledgeMySQL},
	}

	totalIndexed := 0
	for _, kbFile := range kbFiles {
		data, err := os.ReadFile(kbFile.path)
		if err != nil {
			fmt.Printf("读取文件失败 %s: %v\n", kbFile.path, err)
			continue
		}

		var kf KnowledgeFile
		if err := yaml.Unmarshal(data, &kf); err != nil {
			fmt.Printf("解析文件失败 %s: %v\n", kbFile.path, err)
			continue
		}

		items := make([]*knowledge.KnowledgeItem, 0, len(kf.KnowledgeItems))
		for i := range kf.KnowledgeItems {
			item := &kf.KnowledgeItems[i]
			if item.Type == "" {
				item.Type = kbFile.itemType
			}
			item.UpdatedAt = time.Now()
			items = append(items, item)
		}

		// 生成向量
		texts := make([]string, len(items))
		for i, item := range items {
			texts[i] = item.Title + "\n" + item.Content
		}

		embeddings, err := aiManager.Embed(ctx, texts)
		if err != nil {
			fmt.Printf("生成向量失败: %v，跳过向量索引\n", err)
		} else {
			for i, item := range items {
				if i < len(embeddings) {
					item.Embedding = embeddings[i]
				}
			}
		}

		// 批量保存
		if err := esStore.BatchSave(ctx, items); err != nil {
			fmt.Printf("保存失败 %s: %v\n", kbFile.path, err)
			continue
		}

		fmt.Printf("索引成功: %s (%d条)\n", kbFile.path, len(items))
		totalIndexed += len(items)
	}

	fmt.Printf("\n知识库初始化完成，共索引 %d 条知识\n", totalIndexed)

	// 验证
	result, err := esStore.Search(ctx, &knowledge.SearchQuery{
		Query: "VLAN配置",
		TopK:  3,
	})
	if err != nil {
		fmt.Printf("验证搜索失败: %v\n", err)
		return
	}
	fmt.Printf("\n验证搜索 'VLAN配置' 结果:\n")
	for _, item := range result.Items {
		out, _ := json.Marshal(map[string]string{
			"title": item.Title,
			"type":  string(item.Type),
		})
		fmt.Println(string(out))
	}
}
