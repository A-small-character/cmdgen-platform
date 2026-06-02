package handler

import (
	"net/http"
	"strconv"

	"github.com/A-small-character/cmdgen-platform/internal/application/retriever"
	"github.com/A-small-character/cmdgen-platform/internal/domain/knowledge"
	"github.com/A-small-character/cmdgen-platform/internal/infrastructure/webcrawler"
	"github.com/gin-gonic/gin"
)

type KnowledgeHandler struct {
	ragService   *retriever.RAGService
	indexService *retriever.IndexService
	crawler      *webcrawler.DocCrawler
}

func NewKnowledgeHandler(
	ragService *retriever.RAGService,
	indexService *retriever.IndexService,
	crawler *webcrawler.DocCrawler,
) *KnowledgeHandler {
	return &KnowledgeHandler{
		ragService:   ragService,
		indexService: indexService,
		crawler:      crawler,
	}
}

// Search 知识库检索
func (h *KnowledgeHandler) Search(c *gin.Context) {
	if h.ragService == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"code": 5010, "message": "知识库服务未初始化，请检查向量数据库配置"})
		return
	}
	query := c.Query("q")
	if query == "" {
		c.JSON(http.StatusBadRequest, gin.H{"code": 1001, "message": "搜索关键词不能为空"})
		return
	}

	topK := 10
	if k := c.Query("top_k"); k != "" {
		if n, err := strconv.Atoi(k); err == nil && n > 0 {
			topK = n
		}
	}

	items, err := h.ragService.Retrieve(c.Request.Context(), &knowledge.SearchQuery{
		Query:     query,
		Type:      knowledge.KnowledgeType(c.Query("type")),
		Vendor:    c.Query("vendor"),
		TopK:      topK,
		UseVector: true,
	})
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": 5002, "message": "检索失败", "detail": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"code":  0,
		"data":  items,
		"total": len(items),
	})
}

// IndexRequest 索引请求
type IndexRequest struct {
	Items []*knowledge.KnowledgeItem `json:"items" binding:"required"`
}

// BatchIndex 批量索引
func (h *KnowledgeHandler) BatchIndex(c *gin.Context) {
	if h.indexService == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"code": 5010, "message": "索引服务未初始化，请检查向量数据库配置"})
		return
	}
	var req IndexRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": 1001, "message": err.Error()})
		return
	}

	if err := h.indexService.IndexItems(c.Request.Context(), req.Items); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": 5002, "message": "索引失败", "detail": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"code": 0, "message": "索引成功", "count": len(req.Items)})
}

// CrawlRequest 文档爬取请求
type CrawlRequest struct {
	URL    string `json:"url" binding:"required,url"`
	Source string `json:"source" binding:"required"`
}

// CrawlAndIndex 爬取并索引官方文档
func (h *KnowledgeHandler) CrawlAndIndex(c *gin.Context) {
	if h.indexService == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"code": 5010, "message": "索引服务未初始化，请检查向量数据库配置"})
		return
	}
	var req CrawlRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": 1001, "message": err.Error()})
		return
	}

	item, err := h.crawler.FetchAndParse(c.Request.Context(), req.URL, req.Source)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": 5003, "message": "文档抓取失败", "detail": err.Error()})
		return
	}

	if err := h.indexService.IndexItems(c.Request.Context(), []*knowledge.KnowledgeItem{item}); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": 5002, "message": "索引失败", "detail": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"code": 0, "message": "抓取并索引成功", "data": item})
}

// SearchWebDocs 搜索官方在线文档
func (h *KnowledgeHandler) SearchWebDocs(c *gin.Context) {
	query := c.Query("q")
	source := c.DefaultQuery("source", "elastic_docs")

	if query == "" {
		c.JSON(http.StatusBadRequest, gin.H{"code": 1001, "message": "搜索关键词不能为空"})
		return
	}

	results, err := h.crawler.Search(c.Request.Context(), query, source)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": 5003, "message": "文档搜索失败", "detail": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"code":  0,
		"data":  results,
		"total": len(results),
	})
}
