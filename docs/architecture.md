# 智能命令生成平台 - 系统架构文档

## 1. 系统架构总览

```
┌─────────────────────────────────────────────────────────────────────┐
│                         客户端层                                      │
│  Web(React/Next.js)  │  桌面(Wails)  │  移动端(Flutter)  │  API客户端  │
└─────────────────┬───────────────────────────────────────────────────┘
                  │ HTTP/WebSocket/SSE
┌─────────────────▼───────────────────────────────────────────────────┐
│                      API网关层 (Gin + 中间件)                          │
│   认证(JWT) │ 限流(令牌桶) │ CORS │ 日志 │ Metrics(Prometheus)        │
└─────────────────┬───────────────────────────────────────────────────┘
                  │
┌─────────────────▼───────────────────────────────────────────────────┐
│                       应用服务层                                       │
│                                                                       │
│  ┌─────────────────┐  ┌──────────────────┐  ┌───────────────────┐   │
│  │  命令生成引擎    │  │   Agent服务       │  │   RAG检索服务      │   │
│  │  (Engine)       │  │   (ReAct模式)     │  │   (混合检索)       │   │
│  └────────┬────────┘  └────────┬─────────┘  └────────┬──────────┘   │
│           │                    │                       │              │
│  ┌────────▼────────────────────▼───────────────────────▼──────────┐  │
│  │                    命令生成器(Generator)                          │  │
│  │  LinuxGenerator │ NetworkGenerator │ ESGenerator │ PluginGen    │  │
│  └─────────────────────────────────────────────────────────────────┘  │
└─────────────────┬───────────────────────────────────────────────────┘
                  │
┌─────────────────▼───────────────────────────────────────────────────┐
│                       基础设施层                                       │
│                                                                       │
│  ┌──────────┐  ┌──────────┐  ┌──────────┐  ┌──────────────────────┐ │
│  │ AI管理器  │  │ 向量存储  │  │ 知识库   │  │    文档爬取器          │ │
│  │ OpenAI   │  │   ES     │  │  Bleve   │  │  Elastic/Cisco/HW/   │ │
│  │ Claude   │  │  Qdrant  │  │ (全文索引)│  │  H3C/Linux ManPage  │ │
│  │ DeepSeek │  └──────────┘  └──────────┘  └──────────────────────┘ │
│  │ Ollama   │                                                         │
│  └──────────┘  ┌──────────┐  ┌──────────┐                           │
│                │PostgreSQL │  │  Redis   │                           │
│                │ (历史记录)│  │  (缓存)  │                           │
│                └──────────┘  └──────────┘                           │
└─────────────────────────────────────────────────────────────────────┘
```

## 2. Go项目目录结构

```
cmdgen-platform/
├── cmd/
│   └── server/
│       └── main.go                    # 程序入口，依赖注入，优雅关闭
│
├── internal/                          # 内部实现（不对外暴露）
│   ├── domain/                        # 领域层（DDD核心）
│   │   ├── command/
│   │   │   ├── entity.go              # 命令实体、值对象定义
│   │   │   └── repository.go          # 仓储接口、生成器接口
│   │   ├── device/                    # 设备领域模型
│   │   └── knowledge/
│   │       ├── entity.go              # 知识条目、搜索查询实体
│   │       └── repository.go          # 知识库仓储接口
│   │
│   ├── application/                   # 应用层（用例编排）
│   │   ├── generator/
│   │   │   ├── engine.go              # 命令生成引擎（门面）
│   │   │   ├── linux.go               # Linux/Network/ES生成器实现
│   │   │   └── prompts.go             # Prompt模板库
│   │   ├── agent/
│   │   │   └── agent.go               # ReAct Agent实现
│   │   └── retriever/
│   │       └── rag.go                 # RAG服务（RRF混合检索）
│   │
│   ├── infrastructure/                # 基础设施层
│   │   ├── ai/
│   │   │   ├── provider.go            # Provider接口定义
│   │   │   ├── openai.go              # OpenAI实现
│   │   │   ├── claude.go              # Claude实现
│   │   │   ├── deepseek.go            # DeepSeek实现
│   │   │   ├── ollama.go              # Ollama本地LLM实现
│   │   │   └── manager.go             # AI提供商管理器
│   │   ├── database/
│   │   │   └── postgres.go            # PostgreSQL连接、数据模型
│   │   ├── cache/
│   │   │   └── redis.go               # Redis缓存实现
│   │   ├── vector/
│   │   │   └── elasticsearch.go       # ES向量存储（kNN检索）
│   │   └── webcrawler/
│   │       └── crawler.go             # 官方文档爬取器
│   │
│   ├── interfaces/                    # 接口适配层
│   │   └── http/
│   │       ├── router.go              # 路由注册
│   │       ├── handler/
│   │       │   ├── generate.go        # 命令生成接口
│   │       │   └── knowledge.go       # 知识库接口
│   │       └── middleware/
│   │           └── middleware.go      # 通用中间件
│   │
│   └── plugins/
│       └── plugin.go                  # 插件系统（接口+管理器）
│
├── pkg/                               # 公共工具包
│   ├── config/config.go               # 配置加载（Viper）
│   ├── logger/logger.go               # 结构化日志（Zap）
│   └── errors/errors.go               # 统一错误定义
│
├── knowledge_base/                    # 离线知识库
│   ├── linux/commands.yaml
│   ├── elasticsearch/commands.yaml
│   └── network/huawei_switch.yaml
│
├── scripts/
│   └── init_knowledge.go              # 知识库初始化脚本
│
├── configs/
│   └── config.yaml                    # 应用配置
│
├── deployments/
│   ├── docker/
│   │   ├── Dockerfile                 # 多阶段构建
│   │   └── docker-compose.yml         # 本地开发全栈环境
│   └── k8s/
│       └── deployment.yaml            # K8s部署清单
│
└── go.mod
```

## 3. 数据库设计

### command_histories (命令历史)
| 字段 | 类型 | 说明 |
|------|------|------|
| id | uuid | 主键 |
| user_id | uuid | 用户ID |
| request_id | uuid | 请求ID |
| category | varchar | 命令类别(linux/network/elasticsearch) |
| user_input | text | 用户输入 |
| result_json | jsonb | 生成结果JSON |
| ai_provider | varchar | 使用的AI提供商 |
| tokens_used | int | Token消耗数 |
| latency_ms | bigint | 响应延迟毫秒 |
| created_at | timestamptz | 创建时间 |

### knowledge_items (知识条目)
| 字段 | 类型 | 说明 |
|------|------|------|
| id | uuid | 主键 |
| title | varchar | 标题 |
| content | text | 内容 |
| type | varchar | 类型(linux/network/elasticsearch) |
| tags | text | 标签（JSON数组） |
| source | varchar | 来源名称 |
| source_url | varchar | 来源URL |
| version | varchar | 版本 |
| vendor | varchar | 厂商 |
| created_at | timestamptz | 创建时间 |
| updated_at | timestamptz | 更新时间 |

### ES向量索引映射
```json
{
  "mappings": {
    "properties": {
      "id": { "type": "keyword" },
      "title": { "type": "text", "analyzer": "ik_max_word" },
      "content": { "type": "text", "analyzer": "ik_max_word" },
      "type": { "type": "keyword" },
      "vendor": { "type": "keyword" },
      "embedding": {
        "type": "dense_vector",
        "dims": 1536,
        "index": true,
        "similarity": "cosine"
      }
    }
  }
}
```

## 4. API设计

### POST /api/v1/generate
通用命令生成接口（自动识别类别）

**请求体**
```json
{
  "input": "查找7天前大于1GB的日志文件",
  "category": "linux",
  "provider": "openai",
  "use_rag": true,
  "web_search": false,
  "use_agent": false,
  "options": {
    "os_family": "rhel",
    "os_version": "8",
    "include_explain": true,
    "include_backup": true
  }
}
```

**响应**
```json
{
  "code": 0,
  "data": {
    "id": "uuid",
    "category": "linux",
    "commands": [
      {
        "id": "1",
        "title": "查找命令",
        "command": "find /var/log -type f -mtime +7 -size +1G",
        "format": "cli",
        "explanation": "...",
        "risk": "low",
        "backup_cmd": "",
        "rollback_cmd": ""
      }
    ],
    "explanation": "命令说明",
    "references": [],
    "warnings": [],
    "metadata": {
      "ai_provider": "openai",
      "model_name": "gpt-4o",
      "tokens_used": 350,
      "latency": 1500000000
    }
  }
}
```

### POST /api/v1/generate/linux
### POST /api/v1/generate/network
### POST /api/v1/generate/elasticsearch
### POST /api/v1/generate/stream (SSE流式)
### GET  /api/v1/knowledge/search?q=VLAN配置&type=network&top_k=5
### POST /api/v1/knowledge/index
### POST /api/v1/knowledge/crawl
### GET  /api/v1/knowledge/docs/search?q=ILM&source=elastic_docs

## 5. RAG设计

```
用户查询
    │
    ▼
向量化(OpenAI Embedding)
    │
    ├──► 向量检索(ES kNN)  ──────┐
    │                              │
    └──► 关键词检索(ES BM25) ──►  RRF融合排序
                                   │
                                   ▼
                              Top-K知识片段
                                   │
                                   ▼
                           注入Prompt上下文
                                   │
                                   ▼
                            AI生成最终命令
```

**RRF融合公式**: score(d) = Σ 1/(k + rank(d))，k=60

## 6. Agent设计（ReAct模式）

```
用户输入
    │
    ▼
[Thought] 分析意图
    │
    ▼
[Action] 选择工具
    ├── search_knowledge_base
    ├── search_web_docs
    ├── generate_linux_command
    ├── generate_network_command
    └── generate_es_command
    │
    ▼
[Observation] 工具返回结果
    │
    ▼
重复直到 Final Answer
```

## 7. 插件架构

```go
// 实现 GeneratorPlugin 接口即可注册为新命令类型
type MyVendorPlugin struct{}
func (p *MyVendorPlugin) Name() string { return "my-vendor" }
func (p *MyVendorPlugin) SupportedCategory() command.Category { return "my-vendor" }
func (p *MyVendorPlugin) Generate(ctx context.Context, req *command.GenerateRequest) (*command.GenerateResult, error) {
    // 厂商特定实现
}

// 编译为.so文件后放入plugins/目录自动加载
// go build -buildmode=plugin -o plugins/my-vendor.so ./plugins/my-vendor/
```

## 8. 多厂商适配设计

通过 `NetworkVendor` 枚举 + Prompt模板参数化实现：
- 每个厂商对应一套命令语法模板
- 支持 `multi_vendor=true` 同时生成多厂商配置并展示差异
- 可通过知识库条目扩展厂商特定知识

## 9. 版本兼容设计（ES示例）

| 特性 | ES 6.x | ES 7.x | ES 8.x |
|------|--------|--------|--------|
| Type | 支持，推荐_doc | 软废弃 | 完全移除 |
| Security | 需单独配置 | 基础包免费 | 默认开启 |
| SQL | 6.3起支持 | 支持 | 支持 |
| 向量检索 | 不支持 | 插件支持 | 原生dense_vector |
| kNN | 不支持 | 不支持 | 支持 |

## 10. MVP开发路线图

### Phase 1 - 核心功能 (4周)
- [x] 项目架构搭建
- [x] AI提供商接入(OpenAI/Claude/DeepSeek/Ollama)
- [x] Linux命令生成
- [x] ES命令生成
- [x] 网络设备命令生成
- [x] HTTP API

### Phase 2 - 智能增强 (4周)
- [ ] RAG知识检索集成
- [ ] Agent推理循环
- [ ] 官方文档爬取
- [ ] 流式响应(SSE)
- [ ] 缓存层

### Phase 3 - 生产就绪 (3周)
- [ ] 用户认证(JWT)
- [ ] 插件系统
- [ ] Prometheus监控
- [ ] Docker/K8s部署
- [ ] 知识库管理UI

### Phase 4 - 高级功能 (持续)
- [ ] 向量微调
- [ ] 多轮对话
- [ ] 命令执行验证
- [ ] 桌面客户端(Wails)
- [ ] 移动端(Flutter)

## 11. CI/CD方案

```yaml
# .github/workflows/ci.yml
stages:
  - lint (golangci-lint)
  - test (go test ./...)
  - build (go build, Docker buildx)
  - scan (trivy安全扫描)
  - deploy-staging
  - integration-test
  - deploy-prod (手动触发)
```

## 12. 安全设计

1. **API认证**: JWT Token，支持过期刷新
2. **输入验证**: Gin binding validation，防止注入
3. **速率限制**: 全局 + 用户级别限流
4. **AI Key保护**: 环境变量/K8s Secret，不落盘
5. **输出过滤**: 过滤高危命令（rm -rf /、dd if=/dev/zero等）给出明确风险标注
6. **审计日志**: 所有API调用记录到PostgreSQL
7. **TLS**: 生产环境强制HTTPS
