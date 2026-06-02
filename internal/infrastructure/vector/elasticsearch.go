package vector

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/A-small-character/cmdgen-platform/internal/domain/knowledge"
	"github.com/A-small-character/cmdgen-platform/pkg/config"
	"github.com/google/uuid"
)

// ESVectorStore Elasticsearch向量存储
type ESVectorStore struct {
	client      *http.Client
	addresses   []string
	username    string
	password    string
	indexPrefix string
}

func NewESVectorStore(cfg *config.ESVectorConfig) *ESVectorStore {
	return &ESVectorStore{
		client:      &http.Client{Timeout: 30 * time.Second},
		addresses:   cfg.Addresses,
		username:    cfg.Username,
		password:    cfg.Password,
		indexPrefix: cfg.IndexPrefix,
	}
}

func (s *ESVectorStore) indexName(knowledgeType string) string {
	if knowledgeType == "" {
		return s.indexPrefix + "_default"
	}
	return s.indexPrefix + "_" + knowledgeType
}

// EnsureIndex 确保索引存在（带knn_vector映射）
func (s *ESVectorStore) EnsureIndex(ctx context.Context, indexName string, dimension int) error {
	mapping := map[string]interface{}{
		"mappings": map[string]interface{}{
			"properties": map[string]interface{}{
				"id":         map[string]string{"type": "keyword"},
				"title":      map[string]string{"type": "text", "analyzer": "ik_max_word"},
				"content":    map[string]string{"type": "text", "analyzer": "ik_max_word"},
				"type":       map[string]string{"type": "keyword"},
				"vendor":     map[string]string{"type": "keyword"},
				"version":    map[string]string{"type": "keyword"},
				"source":     map[string]string{"type": "keyword"},
				"source_url": map[string]string{"type": "keyword"},
				"tags":       map[string]string{"type": "keyword"},
				"embedding": map[string]interface{}{
					"type":       "dense_vector",
					"dims":       dimension,
					"index":      true,
					"similarity": "cosine",
				},
				"updated_at": map[string]string{"type": "date"},
			},
		},
	}

	body, _ := json.Marshal(mapping)
	resp, err := s.doRequest(ctx, "PUT", "/"+indexName, body)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	return nil
}

// Save 保存单个知识条目
func (s *ESVectorStore) Save(ctx context.Context, item *knowledge.KnowledgeItem) error {
	if item.ID == "" {
		item.ID = uuid.New().String()
	}
	indexName := s.indexName(string(item.Type))
	body, err := json.Marshal(item)
	if err != nil {
		return err
	}
	resp, err := s.doRequest(ctx, "PUT", fmt.Sprintf("/%s/_doc/%s", indexName, item.ID), body)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 400 {
		respBody, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("ES保存失败 %d: %s", resp.StatusCode, respBody)
	}
	return nil
}

// BatchSave 批量保存
func (s *ESVectorStore) BatchSave(ctx context.Context, items []*knowledge.KnowledgeItem) error {
	var buf bytes.Buffer
	for _, item := range items {
		if item.ID == "" {
			item.ID = uuid.New().String()
		}
		indexName := s.indexName(string(item.Type))
		meta, _ := json.Marshal(map[string]interface{}{
			"index": map[string]string{"_index": indexName, "_id": item.ID},
		})
		doc, _ := json.Marshal(item)
		buf.Write(meta)
		buf.WriteByte('\n')
		buf.Write(doc)
		buf.WriteByte('\n')
	}

	resp, err := s.doRequest(ctx, "POST", "/_bulk", buf.Bytes())
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	return nil
}

// VectorSearch kNN向量检索
func (s *ESVectorStore) VectorSearch(ctx context.Context, embedding []float32, topK int, filter map[string]string) ([]*knowledge.KnowledgeItem, error) {
	query := map[string]interface{}{
		"knn": map[string]interface{}{
			"field":          "embedding",
			"query_vector":   embedding,
			"k":              topK,
			"num_candidates": topK * 10,
		},
		"size": topK,
	}

	if len(filter) > 0 {
		terms := make([]map[string]interface{}, 0, len(filter))
		for k, v := range filter {
			terms = append(terms, map[string]interface{}{
				"term": map[string]string{k: v},
			})
		}
		query["knn"].(map[string]interface{})["filter"] = map[string]interface{}{
			"bool": map[string]interface{}{"must": terms},
		}
	}

	body, _ := json.Marshal(query)
	resp, err := s.doRequest(ctx, "POST", "/_search", body)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	return s.parseSearchResponse(resp.Body)
}

// Search 关键词检索
func (s *ESVectorStore) Search(ctx context.Context, q *knowledge.SearchQuery) (*knowledge.SearchResult, error) {
	query := map[string]interface{}{
		"query": map[string]interface{}{
			"multi_match": map[string]interface{}{
				"query":  q.Query,
				"fields": []string{"title^2", "content", "tags"},
			},
		},
		"size": q.TopK,
	}

	body, _ := json.Marshal(query)
	indexName := s.indexName(string(q.Type))
	resp, err := s.doRequest(ctx, "POST", fmt.Sprintf("/%s/_search", indexName), body)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	items, err := s.parseSearchResponse(resp.Body)
	if err != nil {
		return nil, err
	}
	return &knowledge.SearchResult{Items: items, Total: len(items)}, nil
}

func (s *ESVectorStore) FindByID(ctx context.Context, id string) (*knowledge.KnowledgeItem, error) {
	resp, err := s.doRequest(ctx, "GET", "/_all/_doc/"+id, nil)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var result struct {
		Source knowledge.KnowledgeItem `json:"_source"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, err
	}
	return &result.Source, nil
}

func (s *ESVectorStore) Delete(ctx context.Context, id string) error {
	resp, err := s.doRequest(ctx, "DELETE", "/_all/_doc/"+id, nil)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	return nil
}

func (s *ESVectorStore) doRequest(ctx context.Context, method, path string, body []byte) (*http.Response, error) {
	addr := s.addresses[0]
	url := strings.TrimRight(addr, "/") + path

	var bodyReader io.Reader
	if body != nil {
		bodyReader = bytes.NewReader(body)
	}

	req, err := http.NewRequestWithContext(ctx, method, url, bodyReader)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	if s.username != "" {
		req.SetBasicAuth(s.username, s.password)
	}

	return s.client.Do(req)
}

func (s *ESVectorStore) parseSearchResponse(body io.Reader) ([]*knowledge.KnowledgeItem, error) {
	var esResp struct {
		Hits struct {
			Hits []struct {
				ID     string                 `json:"_id"`
				Score  float64                `json:"_score"`
				Source knowledge.KnowledgeItem `json:"_source"`
			} `json:"hits"`
		} `json:"hits"`
	}

	if err := json.NewDecoder(body).Decode(&esResp); err != nil {
		return nil, err
	}

	items := make([]*knowledge.KnowledgeItem, 0, len(esResp.Hits.Hits))
	for _, h := range esResp.Hits.Hits {
		item := h.Source
		item.ID = h.ID
		item.Score = h.Score
		items = append(items, &item)
	}
	return items, nil
}
