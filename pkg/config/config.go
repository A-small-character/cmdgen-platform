package config

import (
	"os"
	"strings"
	"sync"
	"time"

	"github.com/spf13/viper"
)

var (
	once     sync.Once
	instance *Config
)

type Config struct {
	App      AppConfig      `mapstructure:"app"`
	Database DatabaseConfig `mapstructure:"database"`
	Redis    RedisConfig    `mapstructure:"redis"`
	AI       AIConfig       `mapstructure:"ai"`
	Vector   VectorConfig   `mapstructure:"vector"`
	Search   SearchConfig   `mapstructure:"search"`
	Crawler  CrawlerConfig  `mapstructure:"crawler"`
	JWT      JWTConfig      `mapstructure:"jwt"`
	Log      LogConfig      `mapstructure:"log"`
	Metrics  MetricsConfig  `mapstructure:"metrics"`
	Plugin   PluginConfig   `mapstructure:"plugin"`
}

type AppConfig struct {
	Name    string `mapstructure:"name"`
	Version string `mapstructure:"version"`
	Env     string `mapstructure:"env"`
	Port    int    `mapstructure:"port"`
	Debug   bool   `mapstructure:"debug"`
}

type DatabaseConfig struct {
	Host            string        `mapstructure:"host"`
	Port            int           `mapstructure:"port"`
	Name            string        `mapstructure:"name"`
	User            string        `mapstructure:"user"`
	Password        string        `mapstructure:"password"`
	SSLMode         string        `mapstructure:"sslmode"`
	MaxOpenConns    int           `mapstructure:"max_open_conns"`
	MaxIdleConns    int           `mapstructure:"max_idle_conns"`
	ConnMaxLifetime time.Duration `mapstructure:"conn_max_lifetime"`
}

type RedisConfig struct {
	Host     string `mapstructure:"host"`
	Port     int    `mapstructure:"port"`
	Password string `mapstructure:"password"`
	DB       int    `mapstructure:"db"`
	PoolSize int    `mapstructure:"pool_size"`
}

type AIConfig struct {
	DefaultProvider string         `mapstructure:"default_provider"`
	Timeout         int            `mapstructure:"timeout"`
	MaxTokens       int            `mapstructure:"max_tokens"`
	Temperature     float64        `mapstructure:"temperature"`
	OpenAI          OpenAIConfig   `mapstructure:"openai"`
	Claude          ClaudeConfig   `mapstructure:"claude"`
	DeepSeek        DeepSeekConfig `mapstructure:"deepseek"`
	Ollama          OllamaConfig   `mapstructure:"ollama"`
}

type OpenAIConfig struct {
	APIKey  string `mapstructure:"api_key"`
	BaseURL string `mapstructure:"base_url"`
	Model   string `mapstructure:"model"`
}

type ClaudeConfig struct {
	APIKey  string `mapstructure:"api_key"`
	BaseURL string `mapstructure:"base_url"`
	Model   string `mapstructure:"model"`
}

type DeepSeekConfig struct {
	APIKey  string `mapstructure:"api_key"`
	BaseURL string `mapstructure:"base_url"`
	Model   string `mapstructure:"model"`
}

type OllamaConfig struct {
	BaseURL string `mapstructure:"base_url"`
	Model   string `mapstructure:"model"`
}

type VectorConfig struct {
	Provider      string              `mapstructure:"provider"`
	Dimension     int                 `mapstructure:"dimension"`
	Elasticsearch ESVectorConfig      `mapstructure:"elasticsearch"`
	Qdrant        QdrantVectorConfig  `mapstructure:"qdrant"`
}

type ESVectorConfig struct {
	Addresses   []string `mapstructure:"addresses"`
	Username    string   `mapstructure:"username"`
	Password    string   `mapstructure:"password"`
	IndexPrefix string   `mapstructure:"index_prefix"`
}

type QdrantVectorConfig struct {
	Host       string `mapstructure:"host"`
	Port       int    `mapstructure:"port"`
	Collection string `mapstructure:"collection"`
}

type SearchConfig struct {
	BlevePath string `mapstructure:"bleve_path"`
}

type CrawlerConfig struct {
	Timeout   int             `mapstructure:"timeout"`
	MaxRetries int            `mapstructure:"max_retries"`
	UserAgent string          `mapstructure:"user_agent"`
	RateLimit float64         `mapstructure:"rate_limit"`
	Sources   []CrawlerSource `mapstructure:"sources"`
}

type CrawlerSource struct {
	Name     string `mapstructure:"name"`
	BaseURL  string `mapstructure:"base_url"`
	Priority int    `mapstructure:"priority"`
}

type JWTConfig struct {
	Secret      string `mapstructure:"secret"`
	ExpireHours int    `mapstructure:"expire_hours"`
}

type LogConfig struct {
	Level  string `mapstructure:"level"`
	Format string `mapstructure:"format"`
	Output string `mapstructure:"output"`
	File   string `mapstructure:"file"`
}

type MetricsConfig struct {
	Enabled bool `mapstructure:"enabled"`
	Port    int  `mapstructure:"port"`
}

type PluginConfig struct {
	Dir     string `mapstructure:"dir"`
	Enabled bool   `mapstructure:"enabled"`
}

// loadErr 保存首次加载的错误，once.Do 外部可安全读取
var loadErr error

func Load(cfgFile string) (*Config, error) {
	once.Do(func() {
		v := viper.New()
		if cfgFile != "" {
			v.SetConfigFile(cfgFile)
		} else {
			v.AddConfigPath("./configs")
			v.AddConfigPath(".")
			v.SetConfigName("config")
			v.SetConfigType("yaml")
		}
		v.AutomaticEnv()
		v.SetEnvKeyReplacer(strings.NewReplacer(".", "_"))

		if e := v.ReadInConfig(); e != nil {
			loadErr = e
			return
		}
		cfg := &Config{}
		if e := v.Unmarshal(cfg); e != nil {
			loadErr = e
			return
		}
		// 展开 config 中 "${VAR_NAME}" 格式的环境变量引用
		// Viper 不自动展开 yaml 值中的 ${} 占位符，需手动处理
		expandConfigEnvVars(cfg)
		instance = cfg
	})
	return instance, loadErr
}

// expandConfigEnvVars 将 config 中所有 "${VAR}" 格式的字段替换为实际环境变量值。
// 若环境变量未设置，则保留空字符串（而非错误的占位符字符串发送到 API）。
func expandConfigEnvVars(cfg *Config) {
	expand := func(s string) string {
		return os.ExpandEnv(s) // 展开 $VAR 和 ${VAR} 两种格式
	}
	// AI 提供商 Key
	cfg.AI.OpenAI.APIKey   = expand(cfg.AI.OpenAI.APIKey)
	cfg.AI.OpenAI.BaseURL  = expand(cfg.AI.OpenAI.BaseURL)
	cfg.AI.Claude.APIKey   = expand(cfg.AI.Claude.APIKey)
	cfg.AI.Claude.BaseURL  = expand(cfg.AI.Claude.BaseURL)
	cfg.AI.DeepSeek.APIKey = expand(cfg.AI.DeepSeek.APIKey)
	cfg.AI.DeepSeek.BaseURL = expand(cfg.AI.DeepSeek.BaseURL)
	cfg.AI.Ollama.BaseURL  = expand(cfg.AI.Ollama.BaseURL)
	// 数据库
	cfg.Database.Password  = expand(cfg.Database.Password)
	cfg.Database.Host      = expand(cfg.Database.Host)
	// Redis
	cfg.Redis.Password     = expand(cfg.Redis.Password)
	// JWT
	cfg.JWT.Secret         = expand(cfg.JWT.Secret)
}

func Get() *Config {
	return instance
}
