package knowledge

import "time"

// KnowledgeType 知识类型
type KnowledgeType string

const (
	KnowledgeLinux         KnowledgeType = "linux"
	KnowledgeNetwork       KnowledgeType = "network"
	KnowledgeElasticsearch KnowledgeType = "elasticsearch"
	KnowledgeDocker        KnowledgeType = "docker"
	KnowledgeKubernetes    KnowledgeType = "kubernetes"
	KnowledgeMySQL         KnowledgeType = "mysql"
	KnowledgeGeneral       KnowledgeType = "general"
)

// KnowledgeItem 知识条目
type KnowledgeItem struct {
	ID          string        `json:"id" gorm:"primaryKey"`
	Title       string        `json:"title"`
	Content     string        `json:"content"`
	Type        KnowledgeType `json:"type"`
	Tags        []string      `json:"tags" gorm:"serializer:json"`
	Source      string        `json:"source"`
	SourceURL   string        `json:"source_url"`
	Version     string        `json:"version"`
	Vendor      string        `json:"vendor"`
	Embedding   []float32     `json:"embedding,omitempty" gorm:"-"`
	Score       float64       `json:"score,omitempty" gorm:"-"`
	UpdatedAt   time.Time     `json:"updated_at"`
	CreatedAt   time.Time     `json:"created_at"`
}

// SearchQuery 知识检索查询
type SearchQuery struct {
	Query    string        `json:"query"`
	Type     KnowledgeType `json:"type,omitempty"`
	Vendor   string        `json:"vendor,omitempty"`
	Version  string        `json:"version,omitempty"`
	TopK     int           `json:"top_k"`
	UseVector bool         `json:"use_vector"`
}

// SearchResult 知识检索结果
type SearchResult struct {
	Items   []*KnowledgeItem `json:"items"`
	Total   int              `json:"total"`
	Latency int64            `json:"latency_ms"`
}

// DocSource 官方文档源
type DocSource struct {
	Name     string `json:"name"`
	BaseURL  string `json:"base_url"`
	Priority int    `json:"priority"`
}
