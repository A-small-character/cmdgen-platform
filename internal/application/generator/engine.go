package generator

import (
	"context"
	"fmt"
	"time"

	"github.com/cmdgen/platform/internal/application/retriever"
	"github.com/cmdgen/platform/internal/domain/command"
	"github.com/cmdgen/platform/internal/domain/knowledge"
	"github.com/cmdgen/platform/internal/infrastructure/ai"
	"github.com/cmdgen/platform/pkg/config"
	"github.com/cmdgen/platform/pkg/logger"
	"go.uber.org/zap"
)

// Engine 命令生成引擎（门面）
type Engine struct {
	generators map[command.Category]command.Generator
	aiManager  *ai.Manager
	ragService *retriever.RAGService
	cfg        *config.Config
}

func NewEngine(
	aiManager *ai.Manager,
	ragService *retriever.RAGService,
	cfg *config.Config,
) *Engine {
	e := &Engine{
		generators: make(map[command.Category]command.Generator),
		aiManager:  aiManager,
		ragService: ragService,
		cfg:        cfg,
	}

	// 注册内置生成器
	linuxGen := NewLinuxGenerator(aiManager, &cfg.AI)
	networkGen := NewNetworkGenerator(aiManager, &cfg.AI)
	esGen := NewESGenerator(aiManager, &cfg.AI)
	dockerGen := NewDockerGenerator(aiManager, &cfg.AI)
	k8sGen := NewKubernetesGenerator(aiManager, &cfg.AI)
	mysqlGen := NewMySQLGenerator(aiManager, &cfg.AI)

	e.RegisterGenerator(linuxGen)
	e.RegisterGenerator(networkGen)
	e.RegisterGenerator(esGen)
	e.RegisterGenerator(dockerGen)
	e.RegisterGenerator(k8sGen)
	e.RegisterGenerator(mysqlGen)

	return e
}

func (e *Engine) RegisterGenerator(g command.Generator) {
	e.generators[g.SupportedCategory()] = g
}

func (e *Engine) Generate(ctx context.Context, req *command.GenerateRequest) (*command.GenerateResult, error) {
	start := time.Now()
	logger.Info("开始生成命令",
		zap.String("request_id", req.ID),
		zap.String("category", string(req.Category)),
		zap.String("input", req.UserInput),
	)

	// 如果未指定类别，自动识别
	if req.Category == "" {
		req.Category = e.detectCategory(req.UserInput)
	}

	// RAG 知识增强
	if req.Options.UseRAG && e.ragService != nil {
		ragDocs, err := e.ragService.Retrieve(ctx, &knowledge.SearchQuery{
			Query: req.UserInput,
			TopK:  5,
			Type:  knowledgeTypeFromCategory(req.Category),
		})
		if err == nil && len(ragDocs) > 0 {
			req.Context["rag_context"] = buildRAGContext(ragDocs)
		}
	}

	gen, ok := e.generators[req.Category]
	if !ok {
		return nil, fmt.Errorf("不支持的命令类别: %s", req.Category)
	}

	result, err := gen.Generate(ctx, req)
	if err != nil {
		logger.Error("命令生成失败", zap.String("request_id", req.ID), zap.Error(err))
		return nil, err
	}

	logger.Info("命令生成完成",
		zap.String("request_id", req.ID),
		zap.Duration("latency", time.Since(start)),
		zap.Int("commands", len(result.Commands)),
	)

	return result, nil
}

// detectCategory 自动识别命令类别
func (e *Engine) detectCategory(input string) command.Category {
	esKeywords := []string{"elasticsearch", "kibana", "index", "mapping", "shard", "cluster", "elastic", "_cat", "_cluster", "_search", "索引", "分片"}
	networkKeywords := []string{"vlan", "ospf", "bgp", "acl", "nat", "路由", "交换机", "防火墙", "cisco", "huawei", "h3c", "juniper", "firewall", "router", "switch", "ipsec", "mpls"}
	dockerKeywords := []string{"docker", "container", "容器", "dockerfile", "镜像", "image", "compose", "registry", "dockerhub", "podman", "containerd", "docker run", "docker build"}
	k8sKeywords := []string{"kubernetes", "k8s", "kubectl", "pod", "deployment", "service", "ingress", "namespace", "configmap", "secret", "helm", "statefulset", "daemonset", "节点", "集群扩容", "hpa", "pvc"}
	mysqlKeywords := []string{"mysql", "innodb", "mysqldump", "binlog", "主从", "复制", "slow query", "慢查询", "存储过程", "trigger", "索引优化", "explain", "事务", "deadlock", "死锁", "MGR", "组复制"}
	linuxKeywords := []string{"linux", "systemctl", "nginx", "ssh", "grep", "sed", "awk", "crontab", "find", "ps", "top", "df", "rpm", "yum", "apt", "chmod"}

	inputLower := []byte(input)
	// 优先级：ES > K8s > Docker > MySQL > Network > Linux
	for _, kw := range esKeywords {
		if containsCI(inputLower, kw) {
			return command.CategoryElasticsearch
		}
	}
	for _, kw := range k8sKeywords {
		if containsCI(inputLower, kw) {
			return command.CategoryKubernetes
		}
	}
	for _, kw := range dockerKeywords {
		if containsCI(inputLower, kw) {
			return command.CategoryDocker
		}
	}
	for _, kw := range mysqlKeywords {
		if containsCI(inputLower, kw) {
			return command.CategoryMySQL
		}
	}
	for _, kw := range networkKeywords {
		if containsCI(inputLower, kw) {
			return command.CategoryNetwork
		}
	}
	for _, kw := range linuxKeywords {
		if containsCI(inputLower, kw) {
			return command.CategoryLinux
		}
	}
	return command.CategoryLinux
}

func containsCI(s []byte, sub string) bool {
	lower := make([]byte, len(s))
	for i, c := range s {
		if c >= 'A' && c <= 'Z' {
			lower[i] = c + 32
		} else {
			lower[i] = c
		}
	}
	subLower := make([]byte, len(sub))
	for i, c := range []byte(sub) {
		if c >= 'A' && c <= 'Z' {
			subLower[i] = c + 32
		} else {
			subLower[i] = c
		}
	}
	for i := 0; i <= len(lower)-len(subLower); i++ {
		match := true
		for j := range subLower {
			if lower[i+j] != subLower[j] {
				match = false
				break
			}
		}
		if match {
			return true
		}
	}
	return false
}

func knowledgeTypeFromCategory(cat command.Category) knowledge.KnowledgeType {
	switch cat {
	case command.CategoryLinux:
		return knowledge.KnowledgeLinux
	case command.CategoryNetwork:
		return knowledge.KnowledgeNetwork
	case command.CategoryElasticsearch:
		return knowledge.KnowledgeElasticsearch
	case command.CategoryDocker:
		return knowledge.KnowledgeDocker
	case command.CategoryKubernetes:
		return knowledge.KnowledgeKubernetes
	case command.CategoryMySQL:
		return knowledge.KnowledgeMySQL
	default:
		return knowledge.KnowledgeGeneral
	}
}

func buildRAGContext(items []*knowledge.KnowledgeItem) string {
	var buf []byte
	for i, item := range items {
		buf = append(buf, []byte(fmt.Sprintf("--- 参考文档 %d ---\n标题: %s\n来源: %s\n内容:\n%s\n\n", i+1, item.Title, item.Source, item.Content))...)
	}
	return string(buf)
}
