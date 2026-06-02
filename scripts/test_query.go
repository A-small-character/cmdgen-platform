//go:build ignore

// 测试 ES 知识库中文查询命中情况（Go 字符串原生 UTF-8，避免命令行编码问题）
// 用法: go run scripts/test_query.go
package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
)

func main() {
	queries := []string{
		"修改SSH端口为2222",
		"查找7天前大于1GB的日志文件",
		"华为交换机创建VLAN100",
		"查看ES集群健康状态",
		"docker运行nginx限制内存",
		"mysql备份数据库",
		"k8s滚动重启",
	}
	for _, q := range queries {
		body := map[string]any{
			"size":    1,
			"_source": []string{"query_exact", "category"},
			"query": map[string]any{
				"match": map[string]any{"query": q},
			},
		}
		b, _ := json.Marshal(body)
		resp, err := http.Post("http://localhost:9200/cmdgen_history/_search",
			"application/json", bytes.NewReader(b))
		if err != nil {
			fmt.Printf("查询失败: %v\n", err)
			continue
		}
		var r struct {
			Hits struct {
				Hits []struct {
					Score  float64 `json:"_score"`
					Source struct {
						QueryExact string `json:"query_exact"`
						Category   string `json:"category"`
					} `json:"_source"`
				} `json:"hits"`
			} `json:"hits"`
		}
		json.NewDecoder(resp.Body).Decode(&r)
		resp.Body.Close()
		if len(r.Hits.Hits) == 0 {
			fmt.Printf("[未命中] %s\n", q)
			continue
		}
		h := r.Hits.Hits[0]
		fmt.Printf("[分数 %5.1f] 查询「%s」-> 命中「%s」(%s)\n",
			h.Score, q, h.Source.QueryExact, h.Source.Category)
	}
}
