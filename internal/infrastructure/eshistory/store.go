// Package eshistory 基于 Elasticsearch 的命令知识库 + AI 结果沉淀存储。
//
// 三层查询策略由上层编排：
//  1. ES 历史/知识库命中（本模块 SearchHistory）—— 最快，复用历史
//  2. AI 在线生成 —— 智能，结果通过 SaveResult 沉淀回 ES
//  3. 离线内置规则库 —— 兜底，ES 与 AI 均不可用时
//
// ES 不可用时所有方法安全降级（Available()=false，查询返回 miss，保存静默忽略），
// 不影响离线使用。
package eshistory

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/A-small-character/cmdgen-platform/internal/domain/command"
	"github.com/google/uuid"
)

const (
	historyIndex = "cmdgen_history"   // AI/离线生成的结果沉淀
	// 命中阈值：BM25 分数高于此值才视为有效复用，避免不相关结果误命中
	// 实测口语化查询命中相关知识条目分数 13~29，无关查询通常 <10
	hitScoreThreshold = 10.0
)

// Store ES 历史知识存储
type Store struct {
	addr     string
	username string
	password string
	client   *http.Client
	enabled  bool
}

// New 创建存储并探测 ES 连通性（探测失败则 enabled=false，自动降级）
// username/password 为空时不做认证（开发环境关闭 security 的集群）
func New(addr, username, password string) *Store {
	if addr == "" {
		addr = "http://localhost:9200"
	}
	s := &Store{
		addr:     strings.TrimRight(addr, "/"),
		username: username,
		password: password,
		client:   &http.Client{Timeout: 8 * time.Second},
	}
	s.probe()
	return s
}

// Addr 返回当前 ES 地址
func (s *Store) Addr() string {
	if s == nil {
		return ""
	}
	return s.addr
}

func (s *Store) Available() bool { return s != nil && s.enabled }

// probe 探测集群健康
func (s *Store) probe() {
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	resp, err := s.do(ctx, http.MethodGet, "/_cluster/health", nil)
	if err != nil {
		s.enabled = false
		return
	}
	defer resp.Body.Close()
	s.enabled = resp.StatusCode == http.StatusOK
}

// EnsureIndex 创建历史索引（已存在则忽略）
func (s *Store) EnsureIndex(ctx context.Context) error {
	if !s.enabled {
		return nil
	}
	mapping := map[string]any{
		"settings": map[string]any{
			"number_of_shards":   1,
			"number_of_replicas": 1,
		},
		"mappings": map[string]any{
			"properties": map[string]any{
				"query":       map[string]any{"type": "text"},
				"query_exact": map[string]any{"type": "keyword"},
				"category":    map[string]any{"type": "keyword"},
				"provider":    map[string]any{"type": "keyword"},
				"result_json": map[string]any{"type": "text", "index": false},
				"hit_count":   map[string]any{"type": "integer"},
				"created_at":  map[string]any{"type": "date"},
				"updated_at":  map[string]any{"type": "date"},
			},
		},
	}
	body, _ := json.Marshal(mapping)
	resp, err := s.do(ctx, http.MethodPut, "/"+historyIndex, body)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	// 400 通常是 resource_already_exists_exception，忽略
	return nil
}

// SearchHistory 在历史库中查找相似查询，命中（分数达阈值）则返回缓存结果
func (s *Store) SearchHistory(ctx context.Context, query, category string) (*command.GenerateResult, bool) {
	if !s.enabled {
		return nil, false
	}

	must := []map[string]any{
		{"match": map[string]any{"query": query}},
	}
	filter := []map[string]any{}
	if category != "" {
		filter = append(filter, map[string]any{"term": map[string]any{"category": category}})
	}

	q := map[string]any{
		"size": 1,
		"query": map[string]any{
			"bool": map[string]any{"must": must, "filter": filter},
		},
	}
	body, _ := json.Marshal(q)
	resp, err := s.do(ctx, http.MethodPost, "/"+historyIndex+"/_search", body)
	if err != nil {
		return nil, false
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, false
	}

	var sr struct {
		Hits struct {
			Hits []struct {
				ID     string  `json:"_id"`
				Score  float64 `json:"_score"`
				Source struct {
					ResultJSON string `json:"result_json"`
				} `json:"_source"`
			} `json:"hits"`
		} `json:"hits"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&sr); err != nil {
		return nil, false
	}
	if len(sr.Hits.Hits) == 0 || sr.Hits.Hits[0].Score < hitScoreThreshold {
		return nil, false
	}

	var result command.GenerateResult
	if err := json.Unmarshal([]byte(sr.Hits.Hits[0].Source.ResultJSON), &result); err != nil {
		return nil, false
	}
	// 标记来源 + 异步增加命中计数
	result.Metadata.AIProvider = "es-cache"
	go s.incrHit(context.Background(), sr.Hits.Hits[0].ID)
	return &result, true
}

// SaveResult 将生成结果沉淀到 ES（失败静默，不影响主流程）
func (s *Store) SaveResult(ctx context.Context, query, category string, result *command.GenerateResult) {
	if !s.enabled || result == nil || len(result.Commands) == 0 {
		return
	}
	resultBytes, err := json.Marshal(result)
	if err != nil {
		return
	}
	doc := map[string]any{
		"query":       query,
		"query_exact": query,
		"category":    category,
		"provider":    result.Metadata.AIProvider,
		"result_json": string(resultBytes),
		"hit_count":   0,
		"created_at":  time.Now().Format(time.RFC3339),
		"updated_at":  time.Now().Format(time.RFC3339),
	}
	body, _ := json.Marshal(doc)
	resp, err := s.do(ctx, http.MethodPut, fmt.Sprintf("/%s/_doc/%s", historyIndex, uuid.New().String()), body)
	if err != nil {
		return
	}
	resp.Body.Close()
}

// incrHit 命中计数 +1
func (s *Store) incrHit(ctx context.Context, id string) {
	upd := map[string]any{
		"script": map[string]any{
			"source": "ctx._source.hit_count += 1; ctx._source.updated_at = params.now",
			"params": map[string]any{"now": time.Now().Format(time.RFC3339)},
		},
	}
	body, _ := json.Marshal(upd)
	ctx2, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	resp, err := s.do(ctx2, http.MethodPost, fmt.Sprintf("/%s/_update/%s", historyIndex, id), body)
	if err == nil {
		resp.Body.Close()
	}
}

// Stats 返回历史库统计（文档数）
func (s *Store) Stats(ctx context.Context) (int, error) {
	if !s.enabled {
		return 0, nil
	}
	resp, err := s.do(ctx, http.MethodGet, "/"+historyIndex+"/_count", nil)
	if err != nil {
		return 0, err
	}
	defer resp.Body.Close()
	var r struct {
		Count int `json:"count"`
	}
	json.NewDecoder(resp.Body).Decode(&r)
	return r.Count, nil
}

// HistoryItem 历史记录条目（用于回看列表）
type HistoryItem struct {
	ID         string `json:"id"`
	Query      string `json:"query"`
	Category   string `json:"category"`
	Provider   string `json:"provider"`
	HitCount   int    `json:"hit_count"`
	CreatedAt  string `json:"created_at"`
	ResultJSON string `json:"result_json"`
}

// ListHistory 返回历史记录列表（按时间倒序），category 为空则返回全部
func (s *Store) ListHistory(ctx context.Context, category string, size int) ([]HistoryItem, error) {
	if !s.enabled {
		return nil, nil
	}
	if size <= 0 || size > 200 {
		size = 50
	}
	var query map[string]any
	if category != "" {
		query = map[string]any{"term": map[string]any{"category": category}}
	} else {
		query = map[string]any{"match_all": map[string]any{}}
	}
	q := map[string]any{
		"size":  size,
		"query": query,
		"sort":  []any{map[string]any{"created_at": map[string]any{"order": "desc"}}},
	}
	body, _ := json.Marshal(q)
	resp, err := s.do(ctx, http.MethodPost, "/"+historyIndex+"/_search", body)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("list history: status %d", resp.StatusCode)
	}

	var sr struct {
		Hits struct {
			Hits []struct {
				ID     string `json:"_id"`
				Source struct {
					QueryExact string `json:"query_exact"`
					Category   string `json:"category"`
					Provider   string `json:"provider"`
					HitCount   int    `json:"hit_count"`
					CreatedAt  string `json:"created_at"`
					ResultJSON string `json:"result_json"`
				} `json:"_source"`
			} `json:"hits"`
		} `json:"hits"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&sr); err != nil {
		return nil, err
	}
	items := make([]HistoryItem, 0, len(sr.Hits.Hits))
	for _, h := range sr.Hits.Hits {
		items = append(items, HistoryItem{
			ID:         h.ID,
			Query:      h.Source.QueryExact,
			Category:   h.Source.Category,
			Provider:   h.Source.Provider,
			HitCount:   h.Source.HitCount,
			CreatedAt:  h.Source.CreatedAt,
			ResultJSON: h.Source.ResultJSON,
		})
	}
	return items, nil
}

// do 执行 HTTP 请求
func (s *Store) do(ctx context.Context, method, path string, body []byte) (*http.Response, error) {
	var r io.Reader
	if body != nil {
		r = bytes.NewReader(body)
	}
	req, err := http.NewRequestWithContext(ctx, method, s.addr+path, r)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	if s.username != "" {
		req.SetBasicAuth(s.username, s.password)
	}
	return s.client.Do(req)
}
