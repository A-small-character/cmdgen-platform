package retriever

import (
	"context"
	"fmt"
	"sort"
	"time"

	"github.com/A-small-character/cmdgen-platform/internal/domain/knowledge"
	"github.com/A-small-character/cmdgen-platform/internal/infrastructure/ai"
)

// RAGService RAG检索增强生成服务
type RAGService struct {
	repo      knowledge.Repository
	embedder  *ai.Manager
	reranker  Reranker
}

// Reranker 重排序接口
type Reranker interface {
	Rerank(ctx context.Context, query string, docs []*knowledge.KnowledgeItem) ([]*knowledge.KnowledgeItem, error)
}

func NewRAGService(repo knowledge.Repository, embedder *ai.Manager) *RAGService {
	return &RAGService{
		repo:     repo,
		embedder: embedder,
	}
}

// Retrieve 混合检索：向量检索 + 关键词检索 + 重排序
func (s *RAGService) Retrieve(ctx context.Context, query *knowledge.SearchQuery) ([]*knowledge.KnowledgeItem, error) {
	start := time.Now()

	// 1. 并行执行向量检索和关键词检索
	type result struct {
		items []*knowledge.KnowledgeItem
		err   error
	}

	vectorCh := make(chan result, 1)
	keywordCh := make(chan result, 1)

	// 向量检索
	go func() {
		embeddings, err := s.embedder.Embed(ctx, []string{query.Query})
		if err != nil || len(embeddings) == 0 {
			vectorCh <- result{err: err}
			return
		}
		filter := make(map[string]string)
		if query.Type != "" {
			filter["type"] = string(query.Type)
		}
		if query.Vendor != "" {
			filter["vendor"] = query.Vendor
		}
		items, err := s.repo.VectorSearch(ctx, embeddings[0], query.TopK*2, filter)
		vectorCh <- result{items: items, err: err}
	}()

	// 关键词检索
	go func() {
		searchResult, err := s.repo.Search(ctx, query)
		if err != nil {
			keywordCh <- result{err: err}
			return
		}
		keywordCh <- result{items: searchResult.Items}
	}()

	vectorResult := <-vectorCh
	keywordResult := <-keywordCh

	// 2. 合并结果（RRF融合）
	merged := rrfMerge(vectorResult.items, keywordResult.items, query.TopK)

	// 3. 重排序（可选）
	if s.reranker != nil && len(merged) > 0 {
		reranked, err := s.reranker.Rerank(ctx, query.Query, merged)
		if err == nil {
			merged = reranked
		}
	}

	_ = start // 可用于metrics记录
	return merged, nil
}

// rrfMerge Reciprocal Rank Fusion 融合算法
func rrfMerge(vectorDocs, keywordDocs []*knowledge.KnowledgeItem, topK int) []*knowledge.KnowledgeItem {
	const k = 60.0
	scores := make(map[string]float64)
	itemMap := make(map[string]*knowledge.KnowledgeItem)

	for i, doc := range vectorDocs {
		scores[doc.ID] += 1.0 / (k + float64(i+1))
		itemMap[doc.ID] = doc
	}
	for i, doc := range keywordDocs {
		scores[doc.ID] += 1.0 / (k + float64(i+1))
		itemMap[doc.ID] = doc
	}

	type scored struct {
		id    string
		score float64
	}
	ranked := make([]scored, 0, len(scores))
	for id, score := range scores {
		ranked = append(ranked, scored{id, score})
	}
	sort.Slice(ranked, func(i, j int) bool {
		return ranked[i].score > ranked[j].score
	})

	result := make([]*knowledge.KnowledgeItem, 0, topK)
	for i, s := range ranked {
		if i >= topK {
			break
		}
		item := itemMap[s.id]
		item.Score = s.score
		result = append(result, item)
	}
	return result
}

// IndexService 知识库索引服务
type IndexService struct {
	repo    knowledge.Repository
	embedder *ai.Manager
}

func NewIndexService(repo knowledge.Repository, embedder *ai.Manager) *IndexService {
	return &IndexService{repo: repo, embedder: embedder}
}

// IndexItems 批量索引知识条目（生成向量并存储）
func (s *IndexService) IndexItems(ctx context.Context, items []*knowledge.KnowledgeItem) error {
	if len(items) == 0 {
		return nil
	}

	// 批量生成 embedding
	texts := make([]string, len(items))
	for i, item := range items {
		texts[i] = item.Title + "\n" + item.Content
	}

	embeddings, err := s.embedder.Embed(ctx, texts)
	if err != nil {
		return fmt.Errorf("生成向量失败: %w", err)
	}

	for i, item := range items {
		if i < len(embeddings) {
			item.Embedding = embeddings[i]
		}
	}

	return s.repo.BatchSave(ctx, items)
}
