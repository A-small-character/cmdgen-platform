package http

import (
	"github.com/cmdgen/platform/internal/interfaces/http/handler"
	"github.com/cmdgen/platform/internal/interfaces/http/middleware"
	"github.com/gin-gonic/gin"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

type Router struct {
	generateHandler  *handler.GenerateHandler
	knowledgeHandler *handler.KnowledgeHandler
}

func NewRouter(
	generateHandler *handler.GenerateHandler,
	knowledgeHandler *handler.KnowledgeHandler,
) *Router {
	return &Router{
		generateHandler:  generateHandler,
		knowledgeHandler: knowledgeHandler,
	}
}

func (r *Router) Setup(engine *gin.Engine) {
	engine.Use(middleware.Recovery())
	engine.Use(middleware.RequestID())
	engine.Use(middleware.Logger())
	engine.Use(middleware.CORS())

	// 健康检查
	engine.GET("/health", func(c *gin.Context) {
		c.JSON(200, gin.H{"status": "ok", "service": "cmdgen-platform"})
	})

	// Prometheus指标
	engine.GET("/metrics", gin.WrapH(promhttp.Handler()))

	// API v1
	v1 := engine.Group("/api/v1")
	v1.Use(middleware.RateLimit(100))

	// 命令生成
	gen := v1.Group("/generate")
	{
		gen.POST("", r.generateHandler.Generate)
		gen.POST("/linux", r.generateHandler.GenerateLinux)
		gen.POST("/network", r.generateHandler.GenerateNetwork)
		gen.POST("/elasticsearch", r.generateHandler.GenerateES)
		gen.POST("/docker", r.generateHandler.GenerateDocker)
		gen.POST("/kubernetes", r.generateHandler.GenerateKubernetes)
		gen.POST("/mysql", r.generateHandler.GenerateMySQL)
		gen.POST("/stream", r.generateHandler.StreamGenerate)
	}

	// 知识库
	kb := v1.Group("/knowledge")
	{
		kb.GET("/search", r.knowledgeHandler.Search)
		kb.POST("/index", r.knowledgeHandler.BatchIndex)
		kb.POST("/crawl", r.knowledgeHandler.CrawlAndIndex)
		kb.GET("/docs/search", r.knowledgeHandler.SearchWebDocs)
	}
}
