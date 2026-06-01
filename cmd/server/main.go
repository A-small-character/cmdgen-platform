package main

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"time"

	"github.com/cmdgen/platform/internal/application/agent"
	"github.com/cmdgen/platform/internal/application/generator"
	"github.com/cmdgen/platform/internal/application/retriever"
	"github.com/cmdgen/platform/internal/infrastructure/ai"
	"github.com/cmdgen/platform/internal/infrastructure/cache"
	"github.com/cmdgen/platform/internal/infrastructure/database"
	"github.com/cmdgen/platform/internal/infrastructure/vector"
	"github.com/cmdgen/platform/internal/infrastructure/webcrawler"
	httpInterface "github.com/cmdgen/platform/internal/interfaces/http"
	"github.com/cmdgen/platform/internal/interfaces/http/handler"
	"github.com/cmdgen/platform/internal/osutil"
	"github.com/cmdgen/platform/internal/plugins"
	"github.com/cmdgen/platform/pkg/config"
	"github.com/cmdgen/platform/pkg/logger"
	"github.com/gin-gonic/gin"
	"github.com/spf13/cobra"
	"go.uber.org/zap"
)

var cfgFile string

func main() {
	root := &cobra.Command{
		Use:   "cmdgen",
		Short: "智能命令生成平台",
		RunE:  run,
	}
	root.PersistentFlags().StringVar(&cfgFile, "config", "", "配置文件路径（默认读取 ./configs/config.yaml）")
	root.AddCommand(versionCmd())

	if err := root.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func run(_ *cobra.Command, _ []string) error {
	// ── 1. 加载配置 ──────────────────────────────────────────────────────────
	cfg, err := config.Load(cfgFile)
	if err != nil {
		return fmt.Errorf("加载配置失败: %w", err)
	}

	// ── 2. 初始化日志（确保日志目录存在，跨平台） ─────────────────────────────
	if cfg.Log.File != "" {
		logDir := filepath.Dir(cfg.Log.File)
		if mkErr := os.MkdirAll(logDir, 0755); mkErr != nil {
			fmt.Fprintf(os.Stderr, "创建日志目录失败: %v\n", mkErr)
		}
	}
	logger.Init(cfg.Log.Level, cfg.Log.Format, cfg.Log.Output, cfg.Log.File)
	logger.Info("启动智能命令生成平台",
		zap.String("version", cfg.App.Version),
		zap.String("env", cfg.App.Env),
		zap.String("os", osutil.OSName()),
	)

	// ── 3. AI 提供商管理器 ────────────────────────────────────────────────────
	aiManager := ai.NewManager(&cfg.AI)
	if len(aiManager.ListProviders()) == 0 {
		logger.Warn("未配置任何 AI 提供商，命令生成功能将不可用。请在配置文件或环境变量中设置 API Key")
	}

	// ── 4. PostgreSQL（可选） ─────────────────────────────────────────────────
	if cfg.Database.Host != "" {
		db, dbErr := database.NewPostgres(&cfg.Database)
		if dbErr != nil {
			logger.Warn("PostgreSQL 连接失败，历史记录功能不可用", zap.Error(dbErr))
		} else {
			if migrateErr := database.Migrate(db,
				&database.CommandHistoryModel{},
				&database.KnowledgeItemModel{},
				&database.UserModel{},
			); migrateErr != nil {
				logger.Warn("数据库迁移失败", zap.Error(migrateErr))
			} else {
				logger.Info("数据库迁移完成")
			}
		}
	} else {
		logger.Info("未配置 PostgreSQL，历史记录功能不可用")
	}

	// ── 5. Redis 缓存（可选） ─────────────────────────────────────────────────
	var cacheClient *cache.RedisCache
	if cfg.Redis.Host != "" {
		cacheClient, err = cache.NewRedisCache(&cfg.Redis)
		if err != nil {
			logger.Warn("Redis 连接失败，结果缓存不可用", zap.Error(err))
		} else {
			logger.Info("Redis 连接成功")
		}
	}
	_ = cacheClient

	// ── 6. 向量存储 + RAG 服务（可选） ───────────────────────────────────────
	var ragService *retriever.RAGService
	var indexService *retriever.IndexService

	if len(cfg.Vector.Elasticsearch.Addresses) > 0 && cfg.Vector.Elasticsearch.Addresses[0] != "" {
		esStore := vector.NewESVectorStore(&cfg.Vector.Elasticsearch)
		ensureCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		if ensureErr := esStore.EnsureIndex(ensureCtx, cfg.Vector.Elasticsearch.IndexPrefix+"_default", cfg.Vector.Dimension); ensureErr != nil {
			logger.Warn("向量索引初始化失败（可能已存在）", zap.Error(ensureErr))
		}
		cancel()
		ragService = retriever.NewRAGService(esStore, aiManager)
		indexService = retriever.NewIndexService(esStore, aiManager)
		logger.Info("RAG 服务初始化完成",
			zap.Strings("es_addresses", cfg.Vector.Elasticsearch.Addresses))
	} else {
		logger.Info("未配置向量数据库，RAG 检索功能不可用")
	}

	// ── 7. 应用服务层 ─────────────────────────────────────────────────────────
	eng := generator.NewEngine(aiManager, ragService, cfg)
	agentSvc := agent.NewAgent(aiManager, ragService, &cfg.AI)
	crawler := webcrawler.NewDocCrawler(&cfg.Crawler)

	// ── 8. 插件系统（可选，Windows 使用内置注册模式） ─────────────────────────
	if cfg.Plugin.Enabled && cfg.Plugin.Dir != "" {
		pluginMgr := plugins.NewManager(cfg.Plugin.Dir)
		if loadErr := pluginMgr.LoadFromDir(); loadErr != nil {
			logger.Warn("插件加载失败", zap.Error(loadErr))
		}
	}

	// ── 9. HTTP 服务器 ────────────────────────────────────────────────────────
	if cfg.App.Env == "production" {
		gin.SetMode(gin.ReleaseMode)
	}
	ginEngine := gin.New()

	generateHandler := handler.NewGenerateHandler(eng, agentSvc)
	knowledgeHandler := handler.NewKnowledgeHandler(ragService, indexService, crawler)

	router := httpInterface.NewRouter(generateHandler, knowledgeHandler)
	router.Setup(ginEngine)

	srv := &http.Server{
		Addr:         fmt.Sprintf(":%d", cfg.App.Port),
		Handler:      ginEngine,
		ReadTimeout:  30 * time.Second,
		WriteTimeout: 120 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	go func() {
		logger.Info("HTTP 服务器启动", zap.String("addr", srv.Addr))
		if srvErr := srv.ListenAndServe(); srvErr != nil && srvErr != http.ErrServerClosed {
			logger.Fatal("HTTP 服务器异常退出", zap.Error(srvErr))
		}
	}()

	// ── 10. 优雅关闭（跨平台信号处理） ───────────────────────────────────────
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, osutil.ShutdownSignals()...)
	sig := <-quit
	logger.Info("收到关闭信号，开始优雅退出", zap.String("signal", sig.String()))

	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer shutdownCancel()

	if shutdownErr := srv.Shutdown(shutdownCtx); shutdownErr != nil {
		logger.Error("HTTP 服务器关闭失败", zap.Error(shutdownErr))
		return shutdownErr
	}
	logger.Info("服务器已正常关闭")
	return nil
}

func versionCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "version",
		Short: "显示版本信息",
		Run: func(_ *cobra.Command, _ []string) {
			cfg := config.Get()
			if cfg != nil {
				fmt.Printf("智能命令生成平台 v%s (env: %s, os: %s)\n",
					cfg.App.Version, cfg.App.Env, osutil.OSName())
			} else {
				fmt.Printf("智能命令生成平台 v1.0.0 (os: %s)\n", osutil.OSName())
			}
		},
	}
}
