package handler

import (
	"net/http"
	"time"

	"github.com/cmdgen/platform/internal/application/agent"
	"github.com/cmdgen/platform/internal/application/generator"
	"github.com/cmdgen/platform/internal/domain/command"
	"github.com/cmdgen/platform/pkg/errors"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

type GenerateHandler struct {
	engine *generator.Engine
	agent  *agent.Agent
}

func NewGenerateHandler(engine *generator.Engine, ag *agent.Agent) *GenerateHandler {
	return &GenerateHandler{engine: engine, agent: ag}
}

// GenerateRequest HTTP请求体
type GenerateRequest struct {
	Input      string                 `json:"input" binding:"required,min=2,max=2000"`
	Category   string                 `json:"category"`
	Provider   string                 `json:"provider"`
	UseAgent   bool                   `json:"use_agent"`
	UseRAG     bool                   `json:"use_rag"`
	WebSearch  bool                   `json:"web_search"`
	Options    command.GenerateOptions `json:"options"`
}

// Generate godoc
// @Summary 生成命令
// @Tags generate
// @Accept json
// @Produce json
// @Param request body GenerateRequest true "生成请求"
// @Success 200 {object} command.GenerateResult
// @Router /api/v1/generate [post]
func (h *GenerateHandler) Generate(c *gin.Context) {
	var req GenerateRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"code":    1001,
			"message": "参数验证失败",
			"detail":  err.Error(),
		})
		return
	}

	req.Options.AIProvider = req.Provider
	req.Options.UseRAG = req.UseRAG
	req.Options.UseWebSearch = req.WebSearch

	genReq := &command.GenerateRequest{
		ID:        uuid.New().String(),
		UserInput: req.Input,
		Category:  command.Category(req.Category),
		Options:   req.Options,
		Context:   make(map[string]string),
		CreatedAt: time.Now(),
	}

	var result *command.GenerateResult
	var err error

	if req.UseAgent {
		result, err = h.agent.Run(c.Request.Context(), req.Input, req.Provider)
	} else {
		result, err = h.engine.Generate(c.Request.Context(), genReq)
	}

	if err != nil {
		appErr, ok := err.(*errors.AppError)
		if !ok {
			appErr = errors.Wrap(err, 5005, "命令生成失败")
		}
		c.JSON(appErr.HTTPStatus(), gin.H{
			"code":    appErr.Code,
			"message": appErr.Message,
			"detail":  appErr.Detail,
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "success",
		"data":    result,
	})
}

// GenerateLinux 专用Linux命令生成
func (h *GenerateHandler) GenerateLinux(c *gin.Context) {
	var req GenerateRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": 1001, "message": err.Error()})
		return
	}
	req.Category = string(command.CategoryLinux)
	h.doGenerate(c, req)
}

// GenerateNetwork 专用网络命令生成
func (h *GenerateHandler) GenerateNetwork(c *gin.Context) {
	var req GenerateRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": 1001, "message": err.Error()})
		return
	}
	req.Category = string(command.CategoryNetwork)
	h.doGenerate(c, req)
}

// GenerateES Elasticsearch命令生成
func (h *GenerateHandler) GenerateES(c *gin.Context) {
	var req GenerateRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": 1001, "message": err.Error()})
		return
	}
	req.Category = string(command.CategoryElasticsearch)
	h.doGenerate(c, req)
}

// GenerateDocker Docker命令生成
func (h *GenerateHandler) GenerateDocker(c *gin.Context) {
	var req GenerateRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": 1001, "message": err.Error()})
		return
	}
	req.Category = string(command.CategoryDocker)
	h.doGenerate(c, req)
}

// GenerateKubernetes Kubernetes命令生成
func (h *GenerateHandler) GenerateKubernetes(c *gin.Context) {
	var req GenerateRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": 1001, "message": err.Error()})
		return
	}
	req.Category = string(command.CategoryKubernetes)
	h.doGenerate(c, req)
}

// GenerateMySQL MySQL命令生成
func (h *GenerateHandler) GenerateMySQL(c *gin.Context) {
	var req GenerateRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": 1001, "message": err.Error()})
		return
	}
	req.Category = string(command.CategoryMySQL)
	h.doGenerate(c, req)
}

// StreamGenerate 流式生成（SSE）
func (h *GenerateHandler) StreamGenerate(c *gin.Context) {
	var req GenerateRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": 1001, "message": err.Error()})
		return
	}

	c.Header("Content-Type", "text/event-stream")
	c.Header("Cache-Control", "no-cache")
	c.Header("Connection", "keep-alive")
	c.Header("X-Accel-Buffering", "no")

	flusher, ok := c.Writer.(http.Flusher)
	if !ok {
		c.JSON(http.StatusInternalServerError, gin.H{"code": 5000, "message": "不支持流式响应"})
		return
	}

	// 异步生成并推送SSE事件
	resultCh := make(chan string, 100)
	errCh := make(chan error, 1)

	go func() {
		genReq := &command.GenerateRequest{
			ID:        uuid.New().String(),
			UserInput: req.Input,
			Category:  command.Category(req.Category),
			Options:   req.Options,
			Context:   make(map[string]string),
			CreatedAt: time.Now(),
		}
		result, err := h.engine.Generate(c.Request.Context(), genReq)
		if err != nil {
			errCh <- err
			return
		}
		// 模拟流式推送命令
		for _, cmd := range result.Commands {
			resultCh <- cmd.Command
		}
		close(resultCh)
	}()

	for {
		select {
		case chunk, ok := <-resultCh:
			if !ok {
				c.SSEvent("done", gin.H{"status": "completed"})
				flusher.Flush()
				return
			}
			c.SSEvent("data", gin.H{"chunk": chunk})
			flusher.Flush()
		case err := <-errCh:
			c.SSEvent("error", gin.H{"message": err.Error()})
			flusher.Flush()
			return
		case <-c.Request.Context().Done():
			return
		}
	}
}

func (h *GenerateHandler) doGenerate(c *gin.Context, req GenerateRequest) {
	genReq := &command.GenerateRequest{
		ID:        uuid.New().String(),
		UserInput: req.Input,
		Category:  command.Category(req.Category),
		Options:   req.Options,
		Context:   make(map[string]string),
		CreatedAt: time.Now(),
	}

	result, err := h.engine.Generate(c.Request.Context(), genReq)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": 5005, "message": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"code": 0, "data": result})
}
