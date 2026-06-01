package command

import (
	"time"

	"github.com/google/uuid"
)

// Category 命令分类
type Category string

const (
	CategoryLinux         Category = "linux"
	CategoryNetwork       Category = "network"
	CategoryElasticsearch Category = "elasticsearch"
	CategoryScript        Category = "script"
	CategoryDocker        Category = "docker"
	CategoryKubernetes    Category = "kubernetes"
	CategoryMySQL         Category = "mysql"
)

// OSFamily 操作系统家族
type OSFamily string

const (
	OSFamilyRHEL   OSFamily = "rhel"
	OSFamilyDebian OSFamily = "debian"
	OSFamilymacOS  OSFamily = "macos"
	OSFamilyOther  OSFamily = "other"
)

// NetworkVendor 网络设备厂商
type NetworkVendor string

const (
	VendorCisco    NetworkVendor = "cisco"
	VendorHuawei   NetworkVendor = "huawei"
	VendorH3C      NetworkVendor = "h3c"
	VendorJuniper  NetworkVendor = "juniper"
	VendorRuijie   NetworkVendor = "ruijie"
	VendorArista   NetworkVendor = "arista"
	VendorPaloAlto NetworkVendor = "paloalto"
	VendorFortinet NetworkVendor = "fortinet"
	VendorSangfor  NetworkVendor = "sangfor"
	VendorF5       NetworkVendor = "f5"
)

// ESVersion Elasticsearch版本
type ESVersion string

const (
	ESVersion6 ESVersion = "6.x"
	ESVersion7 ESVersion = "7.x"
	ESVersion8 ESVersion = "8.x"
	ESVersion9 ESVersion = "9.x"
)

// OutputFormat 输出格式
type OutputFormat string

const (
	FormatCurl       OutputFormat = "curl"
	FormatKibana     OutputFormat = "kibana"
	FormatSQL        OutputFormat = "sql"
	FormatCLI        OutputFormat = "cli"
	FormatConfig     OutputFormat = "config"
	FormatScript     OutputFormat = "script"
)

// RiskLevel 风险等级
type RiskLevel string

const (
	RiskLow    RiskLevel = "low"
	RiskMedium RiskLevel = "medium"
	RiskHigh   RiskLevel = "high"
)

// GenerateRequest 命令生成请求
type GenerateRequest struct {
	ID          string            `json:"id"`
	UserInput   string            `json:"user_input"`
	Category    Category          `json:"category"`
	Context     map[string]string `json:"context"`
	Options     GenerateOptions   `json:"options"`
	CreatedAt   time.Time         `json:"created_at"`
}

// GenerateOptions 生成选项
type GenerateOptions struct {
	// Linux 选项
	OSFamily   OSFamily `json:"os_family,omitempty"`
	OSVersion  string   `json:"os_version,omitempty"`

	// 网络设备选项
	Vendor      NetworkVendor `json:"vendor,omitempty"`
	DeviceModel string        `json:"device_model,omitempty"`
	OSVer       string        `json:"os_ver,omitempty"`

	// ES 选项
	ESVersion    ESVersion    `json:"es_version,omitempty"`
	OutputFormat OutputFormat `json:"output_format,omitempty"`

	// Docker 选项
	DockerVersion string `json:"docker_version,omitempty"` // "24", "25", "26", "27"
	ComposeVersion string `json:"compose_version,omitempty"` // "v2"
	Runtime       string `json:"runtime,omitempty"`        // "docker", "podman", "containerd"

	// Kubernetes 选项
	K8sVersion    string `json:"k8s_version,omitempty"`  // "1.28", "1.29", "1.30", "1.31"
	K8sResource   string `json:"k8s_resource,omitempty"` // "deployment", "service", "configmap"...
	OutputYAML    bool   `json:"output_yaml,omitempty"`

	// MySQL 选项
	MySQLVersion  string `json:"mysql_version,omitempty"` // "5.7", "8.0", "8.4"
	MySQLEngine   string `json:"mysql_engine,omitempty"`  // "InnoDB", "MyISAM"

	// 通用选项
	MultiVendor    bool `json:"multi_vendor,omitempty"`
	IncludeExplain bool `json:"include_explain,omitempty"`
	IncludeBackup  bool `json:"include_backup,omitempty"`
	UseRAG         bool `json:"use_rag,omitempty"`
	UseWebSearch   bool `json:"use_web_search,omitempty"`
	AIProvider     string `json:"ai_provider,omitempty"`
}

// GenerateResult 命令生成结果
type GenerateResult struct {
	ID          string           `json:"id"`
	RequestID   string           `json:"request_id"`
	Category    Category         `json:"category"`
	Commands    []CommandItem    `json:"commands"`
	Explanation string           `json:"explanation"`
	References  []Reference      `json:"references"`
	Warnings    []Warning        `json:"warnings"`
	Metadata    ResultMetadata   `json:"metadata"`
	CreatedAt   time.Time        `json:"created_at"`
}

// CommandItem 单条命令
type CommandItem struct {
	ID          string       `json:"id"`
	Title       string       `json:"title"`
	Command     string       `json:"command"`
	Format      OutputFormat `json:"format"`
	Vendor      string       `json:"vendor,omitempty"`
	OSTarget    string       `json:"os_target,omitempty"`
	Explanation string       `json:"explanation"`
	Risk        RiskLevel    `json:"risk"`
	BackupCmd   string       `json:"backup_cmd,omitempty"`
	RollbackCmd string       `json:"rollback_cmd,omitempty"`
	Example     string       `json:"example,omitempty"`
}

// Reference 参考来源
type Reference struct {
	Title   string `json:"title"`
	URL     string `json:"url"`
	Version string `json:"version,omitempty"`
	Date    string `json:"date,omitempty"`
	Snippet string `json:"snippet,omitempty"`
}

// Warning 风险警告
type Warning struct {
	Level   RiskLevel `json:"level"`
	Message string    `json:"message"`
}

// ResultMetadata 结果元数据
type ResultMetadata struct {
	AIProvider    string        `json:"ai_provider"`
	ModelName     string        `json:"model_name"`
	RAGUsed       bool          `json:"rag_used"`
	WebSearchUsed bool          `json:"web_search_used"`
	TokensUsed    int           `json:"tokens_used"`
	Latency       time.Duration `json:"latency"`
}

// VersionDiff 版本差异说明
type VersionDiff struct {
	Feature     string `json:"feature"`
	FromVersion string `json:"from_version"`
	ToVersion   string `json:"to_version"`
	Description string `json:"description"`
	Impact      string `json:"impact"`
}

func NewGenerateRequest(input string, category Category, opts GenerateOptions) *GenerateRequest {
	return &GenerateRequest{
		ID:        uuid.New().String(),
		UserInput: input,
		Category:  category,
		Options:   opts,
		Context:   make(map[string]string),
		CreatedAt: time.Now(),
	}
}

func NewGenerateResult(reqID string, category Category) *GenerateResult {
	return &GenerateResult{
		ID:        uuid.New().String(),
		RequestID: reqID,
		Category:  category,
		Commands:  make([]CommandItem, 0),
		CreatedAt: time.Now(),
	}
}
