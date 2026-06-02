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
	"path/filepath"
	"runtime"
	"sync"
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

	cfg, _ := appcfg.Load(findConfigFile())
	app := newApp(cfg)

	port, err := freePort()
	if err != nil {
		port = 17880
	}
	addr := fmt.Sprintf("127.0.0.1:%d", port)

	mux := http.NewServeMux()
	app.registerRoutes(mux)
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

	waitReady(addr)
	openBrowser("http://" + addr)
	logger.Info("已打开浏览器窗口", zap.String("url", "http://"+addr))
	fmt.Printf("\n智能命令生成平台已启动\n地址：http://%s\n按 Ctrl+C 退出\n\n", addr)

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, os.Interrupt)
	<-quit
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	srv.Shutdown(ctx)
}

// ─── 用户设置（持久化到 用户配置目录/cmdgen/settings.json）────────────────────

type userSettings struct {
	ESHost     string `json:"es_host"`
	ESPort     string `json:"es_port"`
	ESUsername string `json:"es_username"`
	ESPassword string `json:"es_password"`
	AIProvider string `json:"ai_provider"`
	AIKey      string `json:"ai_key"`
}

func settingsPath() string {
	dir, err := os.UserConfigDir()
	if err != nil || dir == "" {
		dir = "."
	}
	return filepath.Join(dir, "cmdgen", "settings.json")
}

func loadSettings() *userSettings {
	s := &userSettings{ESHost: "localhost", ESPort: "9200"}
	data, err := os.ReadFile(settingsPath())
	if err == nil {
		_ = json.Unmarshal(data, s)
	}
	return s
}

func saveSettings(s *userSettings) error {
	p := settingsPath()
	if err := os.MkdirAll(filepath.Dir(p), 0755); err != nil {
		return err
	}
	data, _ := json.MarshalIndent(s, "", "  ")
	return os.WriteFile(p, data, 0600)
}

func (s *userSettings) esAddr() string {
	host := s.ESHost
	if host == "" {
		host = "localhost"
	}
	port := s.ESPort
	if port == "" {
		port = "9200"
	}
	// 允许 host 直接是完整 URL
	if len(host) > 7 && (host[:7] == "http://" || (len(host) > 8 && host[:8] == "https://")) {
		return host
	}
	return fmt.Sprintf("http://%s:%s", host, port)
}

// ─── 应用层 ────────────────────────────────────────────────────────────────────

type desktopApp struct {
	mu       sync.RWMutex
	offline  *generator.OfflineEngine
	online   *generator.Engine
	es       *eshistory.Store
	cfg      *appcfg.Config
	settings *userSettings
}

func newApp(cfg *appcfg.Config) *desktopApp {
	if cfg == nil {
		cfg = &appcfg.Config{}
		cfg.AI.MaxTokens = 4096
		cfg.AI.Temperature = 0.1
	}
	app := &desktopApp{
		offline:  generator.NewOfflineEngine(),
		cfg:      cfg,
		settings: loadSettings(),
	}

	// 用持久化的 AI 配置初始化在线引擎
	app.applyAIConfig(app.settings.AIProvider, app.settings.AIKey)

	// 用持久化的 ES 配置连接知识库
	app.reconnectES()

	return app
}

// applyAIConfig 应用 AI 配置（无锁，仅启动期调用；运行期通过 handleSaveAI 加锁）
func (a *desktopApp) applyAIConfig(provider, key string) bool {
	if provider == "" || key == "" {
		return false
	}
	a.cfg.AI.DefaultProvider = provider
	switch provider {
	case "deepseek":
		a.cfg.AI.DeepSeek.APIKey = key
		a.cfg.AI.DeepSeek.BaseURL = "https://api.deepseek.com/v1"
		a.cfg.AI.DeepSeek.Model = "deepseek-chat"
	case "openai":
		a.cfg.AI.OpenAI.APIKey = key
		a.cfg.AI.OpenAI.BaseURL = "https://api.openai.com/v1"
		a.cfg.AI.OpenAI.Model = "gpt-4o"
	case "claude":
		a.cfg.AI.Claude.APIKey = key
		a.cfg.AI.Claude.BaseURL = "https://api.anthropic.com"
		a.cfg.AI.Claude.Model = "claude-opus-4-8"
	case "ollama":
		a.cfg.AI.Ollama.BaseURL = "http://localhost:11434"
		a.cfg.AI.Ollama.Model = key
	}
	mgr := ai.NewManager(&a.cfg.AI)
	if len(mgr.ListProviders()) > 0 {
		a.online = generator.NewEngine(mgr, nil, a.cfg)
		return true
	}
	return false
}

// reconnectES 根据当前 settings 重新连接 ES
func (a *desktopApp) reconnectES() {
	addr := a.settings.esAddr()
	es := eshistory.New(addr, a.settings.ESUsername, a.settings.ESPassword)
	if es.Available() {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		_ = es.EnsureIndex(ctx)
		cancel()
		logger.Info("ES 知识库已连接", zap.String("addr", addr))
	} else {
		logger.Info("ES 知识库不可用", zap.String("addr", addr))
	}
	a.es = es
}

func (a *desktopApp) getES() *eshistory.Store {
	a.mu.RLock()
	defer a.mu.RUnlock()
	return a.es
}

func (a *desktopApp) registerRoutes(mux *http.ServeMux) {
	mux.HandleFunc("/api/v1/generate", cors(a.handleGenerate))
	mux.HandleFunc("/api/v1/config/ai", cors(a.handleSaveAI))
	mux.HandleFunc("/api/v1/config/es", cors(a.handleSaveES))
	mux.HandleFunc("/api/v1/settings", cors(a.handleSettings))
	mux.HandleFunc("/api/v1/stats", cors(a.handleStats))
	mux.HandleFunc("/api/v1/history", cors(a.handleHistory))
	mux.HandleFunc("/health", func(w http.ResponseWriter, _ *http.Request) {
		json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
	})
}

// ─── 命令生成 ─────────────────────────────────────────────────────────────────

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
		writeJSON(w, 400, map[string]any{"code": 1001, "message": "参数错误"})
		return
	}
	if req.Input == "" {
		writeJSON(w, 400, map[string]any{"code": 1001, "message": "input 不能为空"})
		return
	}
	cat := command.Category(req.Category)
	if cat == "" {
		cat = command.CategoryLinux
	}

	es := a.getES()

	// ① ES 知识库命中
	if cached, ok := es.SearchHistory(r.Context(), req.Input, string(cat)); ok {
		writeJSON(w, 200, map[string]any{"code": 0, "data": cached, "source": "es-cache"})
		return
	}

	a.mu.RLock()
	online := a.online
	a.mu.RUnlock()

	var result *command.GenerateResult
	var err error

	// ② AI 在线生成
	if online != nil {
		genReq := command.NewGenerateRequest(req.Input, cat, command.GenerateOptions{})
		result, err = online.Generate(r.Context(), genReq)
	}
	// ③ 离线兜底
	if result == nil || err != nil {
		result, err = a.offline.Generate(r.Context(), req.Input, cat)
	}
	if err != nil {
		writeJSON(w, 500, map[string]any{"code": 5005, "message": err.Error()})
		return
	}

	// AI 结果沉淀到 ES
	if result.Metadata.AIProvider != "" && result.Metadata.AIProvider != "offline" {
		go es.SaveResult(context.Background(), req.Input, string(cat), result)
	}
	writeJSON(w, 200, map[string]any{"code": 0, "data": result})
}

// ─── 历史记录 ─────────────────────────────────────────────────────────────────

func (a *desktopApp) handleHistory(w http.ResponseWriter, r *http.Request) {
	cat := r.URL.Query().Get("category")
	items, err := a.getES().ListHistory(r.Context(), cat, 100)
	if err != nil {
		writeJSON(w, 200, map[string]any{"code": 0, "data": []any{}, "es_available": false})
		return
	}
	writeJSON(w, 200, map[string]any{"code": 0, "data": items, "es_available": a.getES().Available()})
}

// ─── 状态 ─────────────────────────────────────────────────────────────────────

func (a *desktopApp) handleStats(w http.ResponseWriter, r *http.Request) {
	es := a.getES()
	count, _ := es.Stats(r.Context())
	a.mu.RLock()
	aiOK := a.online != nil
	a.mu.RUnlock()
	writeJSON(w, 200, map[string]any{
		"code":         0,
		"es_available": es.Available(),
		"es_addr":      es.Addr(),
		"cached_count": count,
		"ai_available": aiOK,
	})
}

// handleSettings 返回当前设置（密码脱敏）
func (a *desktopApp) handleSettings(w http.ResponseWriter, _ *http.Request) {
	a.mu.RLock()
	s := *a.settings
	aiOK := a.online != nil
	a.mu.RUnlock()
	masked := func(v string) string {
		if v == "" {
			return ""
		}
		return "********"
	}
	writeJSON(w, 200, map[string]any{
		"code": 0,
		"data": map[string]any{
			"es_host":      s.ESHost,
			"es_port":      s.ESPort,
			"es_username":  s.ESUsername,
			"es_password":  masked(s.ESPassword),
			"ai_provider":  s.AIProvider,
			"ai_key":       masked(s.AIKey),
			"ai_available": aiOK,
			"es_available": a.getES().Available(),
		},
	})
}

// ─── AI 配置 ──────────────────────────────────────────────────────────────────

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
		writeJSON(w, 400, map[string]any{"code": 1001, "message": "provider 和 api_key 不能为空"})
		return
	}

	a.mu.Lock()
	ok := a.applyAIConfig(req.Provider, req.APIKey)
	if ok {
		a.settings.AIProvider = req.Provider
		a.settings.AIKey = req.APIKey
		_ = saveSettings(a.settings)
	}
	a.mu.Unlock()

	if ok {
		writeJSON(w, 200, map[string]any{"code": 0, "message": "AI 配置已保存"})
	} else {
		writeJSON(w, 500, map[string]any{"code": 5001, "message": "AI 提供商初始化失败，Key 可能有误"})
	}
}

// ─── ES 配置 ──────────────────────────────────────────────────────────────────

type esConfigReq struct {
	Host     string `json:"host"`
	Port     string `json:"port"`
	Username string `json:"username"`
	Password string `json:"password"`
}

func (a *desktopApp) handleSaveES(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", 405)
		return
	}
	var req esConfigReq
	json.NewDecoder(r.Body).Decode(&req)

	a.mu.Lock()
	if req.Host != "" {
		a.settings.ESHost = req.Host
	}
	if req.Port != "" {
		a.settings.ESPort = req.Port
	}
	a.settings.ESUsername = req.Username
	// 密码留空表示不修改（前端显示为 ******** 时不回传明文）
	if req.Password != "" && req.Password != "********" {
		a.settings.ESPassword = req.Password
	}
	a.reconnectES()
	_ = saveSettings(a.settings)
	available := a.es.Available()
	addr := a.es.Addr()
	a.mu.Unlock()

	if available {
		writeJSON(w, 200, map[string]any{"code": 0, "message": "ES 已连接", "addr": addr})
	} else {
		writeJSON(w, 200, map[string]any{"code": 5002, "message": "ES 连接失败，请检查地址/认证", "addr": addr})
	}
}

// ─── 工具 ─────────────────────────────────────────────────────────────────────

func writeJSON(w http.ResponseWriter, status int, v any) {
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
	for _, p := range []string{"configs/config.yaml", "../configs/config.yaml"} {
		if _, err := os.Stat(p); err == nil {
			return p
		}
	}
	return ""
}
