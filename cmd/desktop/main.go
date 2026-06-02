// Package main 桌面客户端入口
// 双击 EXE 自动启动本地服务 + 打开浏览器窗口，无需网络，无需额外安装
package main

import (
	"context"
	"embed"
	"encoding/json"
	"fmt"
	"io/fs"
	"net"
	"net/http"
	"os"
	"os/exec"
	"os/signal"
	"runtime"
	"time"

	"github.com/A-small-character/cmdgen-platform/internal/application/generator"
	"github.com/A-small-character/cmdgen-platform/internal/domain/command"
	"github.com/A-small-character/cmdgen-platform/internal/infrastructure/ai"
	"github.com/A-small-character/cmdgen-platform/internal/infrastructure/eshistory"
	appcfg "github.com/A-small-character/cmdgen-platform/pkg/config"
	"github.com/A-small-character/cmdgen-platform/pkg/logger"
	"go.uber.org/zap"
)

//go:embed all:webui
var webFS embed.FS

func main() {
	logger.Init("info", "console", "stdout", "")

	// ── 加载配置（可选，失败不退出）──────────────────────────────────────────
	cfg, _ := appcfg.Load(findConfigFile())

	// ── 初始化引擎 ────────────────────────────────────────────────────────────
	app := newApp(cfg)

	// ── 找一个空闲端口 ────────────────────────────────────────────────────────
	port, err := freePort()
	if err != nil {
		port = 17880
	}
	addr := fmt.Sprintf("127.0.0.1:%d", port)

	// ── 启动 HTTP 服务器 ──────────────────────────────────────────────────────
	mux := http.NewServeMux()
	app.registerRoutes(mux)

	// 静态文件（嵌入在二进制中）
	webRoot, _ := fs.Sub(webFS, "webui")
	mux.Handle("/", http.FileServer(http.FS(webRoot)))

	srv := &http.Server{Addr: addr, Handler: mux}
	go func() {
		logger.Info("桌面服务启动", zap.String("url", "http://"+addr))
		if e := srv.ListenAndServe(); e != nil && e != http.ErrServerClosed {
			logger.Error("服务异常", zap.Error(e))
			os.Exit(1)
		}
	}()

	// 等待服务就绪后打开浏览器
	waitReady(addr)
	openBrowser("http://" + addr)
	logger.Info("已打开浏览器窗口", zap.String("url", "http://"+addr))
	fmt.Printf("\n智能命令生成平台已启动\n地址：http://%s\n按 Ctrl+C 退出\n\n", addr)

	// ── 优雅退出 ──────────────────────────────────────────────────────────────
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, os.Interrupt)
	<-quit
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	srv.Shutdown(ctx)
}

// ─── 应用层 ────────────────────────────────────────────────────────────────────

type desktopApp struct {
	offline *generator.OfflineEngine
	online  *generator.Engine
	es      *eshistory.Store
	cfg     *appcfg.Config
}

func newApp(cfg *appcfg.Config) *desktopApp {
	app := &desktopApp{
		offline: generator.NewOfflineEngine(),
		cfg:     cfg,
	}
	if cfg != nil {
		mgr := ai.NewManager(&cfg.AI)
		if len(mgr.ListProviders()) > 0 {
			app.online = generator.NewEngine(mgr, nil, cfg)
		}
	}

	// 连接 ES 知识库（主集群 9200），不可用时自动降级
	esAddr := "http://localhost:9200"
	if cfg != nil && len(cfg.Vector.Elasticsearch.Addresses) > 0 && cfg.Vector.Elasticsearch.Addresses[0] != "" {
		esAddr = cfg.Vector.Elasticsearch.Addresses[0]
	}
	app.es = eshistory.New(esAddr)
	if app.es.Available() {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		_ = app.es.EnsureIndex(ctx)
		cancel()
		logger.Info("ES 知识库已连接", zap.String("addr", esAddr))
	} else {
		logger.Info("ES 知识库不可用，使用离线/AI 模式", zap.String("addr", esAddr))
	}

	return app
}

func (a *desktopApp) registerRoutes(mux *http.ServeMux) {
	// 命令生成
	mux.HandleFunc("/api/v1/generate", cors(a.handleGenerate))

	// AI 配置相关
	mux.HandleFunc("/api/v1/config/ai", cors(a.handleSaveAI))
	mux.HandleFunc("/api/v1/config/ai/status", cors(a.handleAIStatus))

	// ES 知识库状态
	mux.HandleFunc("/api/v1/stats", cors(a.handleStats))

	// 健康检查
	mux.HandleFunc("/health", func(w http.ResponseWriter, _ *http.Request) {
		json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
	})
}

// ─── Handler：命令生成 ────────────────────────────────────────────────────────

type generateReq struct {
	Input    string `json:"input"`
	Category string `json:"category"`
	Provider string `json:"provider"`
}

func (a *desktopApp) handleGenerate(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", 405)
		return
	}
	var req generateReq
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, 400, map[string]interface{}{"code": 1001, "message": "参数错误"})
		return
	}
	if req.Input == "" {
		writeJSON(w, 400, map[string]interface{}{"code": 1001, "message": "input 不能为空"})
		return
	}

	cat := command.Category(req.Category)
	if cat == "" {
		cat = command.CategoryLinux
	}

	// ── 三层查询策略 ──────────────────────────────────────────
	// 1) ES 知识库命中 → 直接复用（最快）
	if cached, ok := a.es.SearchHistory(r.Context(), req.Input, string(cat)); ok {
		writeJSON(w, 200, map[string]interface{}{"code": 0, "data": cached, "source": "es-cache"})
		return
	}

	var result *command.GenerateResult
	var err error

	// 2) AI 在线生成
	if a.online != nil {
		genReq := command.NewGenerateRequest(req.Input, cat, command.GenerateOptions{})
		result, err = a.online.Generate(r.Context(), genReq)
	}
	// 3) 离线内置规则库兜底
	if result == nil || err != nil {
		result, err = a.offline.Generate(r.Context(), req.Input, cat)
	}

	if err != nil {
		writeJSON(w, 500, map[string]interface{}{"code": 5005, "message": err.Error()})
		return
	}

	// AI 生成的结果沉淀到 ES（离线规则结果不沉淀，因为本身就在内置库）
	if result.Metadata.AIProvider != "" && result.Metadata.AIProvider != "offline" {
		go a.es.SaveResult(context.Background(), req.Input, string(cat), result)
	}

	writeJSON(w, 200, map[string]interface{}{"code": 0, "data": result})
}

// handleStats 返回 ES 知识库状态
func (a *desktopApp) handleStats(w http.ResponseWriter, r *http.Request) {
	count, _ := a.es.Stats(r.Context())
	writeJSON(w, 200, map[string]interface{}{
		"code":          0,
		"es_available":  a.es.Available(),
		"cached_count":  count,
		"ai_available":  a.online != nil,
	})
}

// ─── Handler：AI 配置 ─────────────────────────────────────────────────────────

type aiConfigReq struct {
	Provider string `json:"provider"`
	APIKey   string `json:"api_key"`
}

func (a *desktopApp) handleSaveAI(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", 405)
		return
	}
	var req aiConfigReq
	json.NewDecoder(r.Body).Decode(&req)

	if req.APIKey == "" || req.Provider == "" {
		writeJSON(w, 400, map[string]interface{}{"code": 1001, "message": "provider 和 api_key 不能为空"})
		return
	}

	if a.cfg == nil {
		a.cfg = &appcfg.Config{}
		a.cfg.AI.MaxTokens = 4096
		a.cfg.AI.Temperature = 0.1
	}
	a.cfg.AI.DefaultProvider = req.Provider
	switch req.Provider {
	case "deepseek":
		a.cfg.AI.DeepSeek.APIKey = req.APIKey
		a.cfg.AI.DeepSeek.BaseURL = "https://api.deepseek.com/v1"
		a.cfg.AI.DeepSeek.Model = "deepseek-chat"
	case "openai":
		a.cfg.AI.OpenAI.APIKey = req.APIKey
		a.cfg.AI.OpenAI.BaseURL = "https://api.openai.com/v1"
		a.cfg.AI.OpenAI.Model = "gpt-4o"
	case "claude":
		a.cfg.AI.Claude.APIKey = req.APIKey
		a.cfg.AI.Claude.BaseURL = "https://api.anthropic.com"
		a.cfg.AI.Claude.Model = "claude-opus-4-8"
	case "ollama":
		a.cfg.AI.Ollama.BaseURL = "http://localhost:11434"
		a.cfg.AI.Ollama.Model = req.APIKey // ollama 用 model name 代替 key
	}

	mgr := ai.NewManager(&a.cfg.AI)
	if len(mgr.ListProviders()) > 0 {
		a.online = generator.NewEngine(mgr, nil, a.cfg)
		writeJSON(w, 200, map[string]interface{}{"code": 0, "message": "AI 配置已保存"})
	} else {
		writeJSON(w, 500, map[string]interface{}{"code": 5001, "message": "AI 提供商初始化失败，Key 可能有误"})
	}
}

func (a *desktopApp) handleAIStatus(w http.ResponseWriter, _ *http.Request) {
	available := a.online != nil
	provider := ""
	if available && a.cfg != nil {
		provider = a.cfg.AI.DefaultProvider
	}
	writeJSON(w, 200, map[string]interface{}{
		"code":         0,
		"ai_available": available,
		"provider":     provider,
	})
}

// ─── 工具函数 ─────────────────────────────────────────────────────────────────

func writeJSON(w http.ResponseWriter, status int, v interface{}) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(v)
}

func cors(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "GET,POST,OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type")
		if r.Method == http.MethodOptions {
			w.WriteHeader(204)
			return
		}
		next(w, r)
	}
}

func freePort() (int, error) {
	l, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return 0, err
	}
	defer l.Close()
	return l.Addr().(*net.TCPAddr).Port, nil
}

func waitReady(addr string) {
	for i := 0; i < 30; i++ {
		if conn, err := net.Dial("tcp", addr); err == nil {
			conn.Close()
			return
		}
		time.Sleep(100 * time.Millisecond)
	}
}

func openBrowser(url string) {
	var cmd *exec.Cmd
	switch runtime.GOOS {
	case "windows":
		cmd = exec.Command("cmd", "/c", "start", url)
	case "darwin":
		cmd = exec.Command("open", url)
	default:
		cmd = exec.Command("xdg-open", url)
	}
	cmd.Start()
}

func findConfigFile() string {
	candidates := []string{
		"configs/config.yaml",
		"../configs/config.yaml",
	}
	for _, p := range candidates {
		if _, err := os.Stat(p); err == nil {
			return p
		}
	}
	return ""
}
