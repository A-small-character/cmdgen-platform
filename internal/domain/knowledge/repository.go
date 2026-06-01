package knowledge

import "context"

type Repository interface {
	Save(ctx context.Context, item *KnowledgeItem) error
	BatchSave(ctx context.Context, items []*KnowledgeItem) error
	FindByID(ctx context.Context, id string) (*KnowledgeItem, error)
	Search(ctx context.Context, query *SearchQuery) (*SearchResult, error)
	VectorSearch(ctx context.Context, embedding []float32, topK int, filter map[string]string) ([]*KnowledgeItem, error)
	Delete(ctx context.Context, id string) error
}

type Searcher interface {
	Search(ctx context.Context, query *SearchQuery) (*SearchResult, error)
}

type Embedder interface {
	Embed(ctx context.Context, text string) ([]float32, error)
	BatchEmbed(ctx context.Context, texts []string) ([][]float32, error)
}
