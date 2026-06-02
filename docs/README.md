# 智能命令生成平台 — 完整使用文档

> 版本：v1.0.0 | Go 1.24+ | Docker Engine 27+ | Kubernetes 1.28+
> **跨平台支持：Windows 10/11 ✓ | macOS 12+ ✓ | Linux ✓**

---

## 目录

1. [项目简介](#1-项目简介)
2. [系统要求](#2-系统要求)
2a. [Windows 快速开始](#windows-快速开始专项)
3. [快速开始（5分钟）](#3-快速开始5分钟)
4. [编译说明](#4-编译说明)
5. [配置说明](#5-配置说明)
6. [本地开发部署](#6-本地开发部署)
7. [Docker Compose 部署](#7-docker-compose-部署)
8. [Kubernetes 生产部署](#8-kubernetes-生产部署)
9. [知识库初始化](#9-知识库初始化)
10. [API 使用说明](#10-api-使用说明)
11. [功能模块说明](#11-功能模块说明)
12. [插件开发指南](#12-插件开发指南)
13. [监控与运维](#13-监控与运维)
14. [常见问题](#14-常见问题)
15. [已知 Bug 修复记录](#15-已知-bug-修复记录)

---

## 1. 项目简介

智能命令生成平台是一个基于 AI 大模型 + 规则引擎 + RAG 知识检索的命令生成工具。

**支持生成的命令类型：**

| 模块 | 覆盖内容 |
|------|---------|
| Linux | RHEL/CentOS/Rocky/AlmaLinux/Ubuntu/Debian/macOS，文件操作、用户管理、服务管理、网络诊断、进程管理、磁盘管理、Shell脚本 |
| 网络设备 | Cisco/Huawei/H3C/Juniper/Palo Alto/Fortinet/Sangfor/F5 等，VLAN/ACL/NAT/BGP/OSPF/IPSec/QoS |
| Elasticsearch | 6.x/7.x/8.x/9.x，索引/Mapping/ILM/快照/CCR/Security/SQL，含版本差异说明 |
| Docker | docker run/build/compose/network，Dockerfile 多阶段构建最佳实践 |
| Kubernetes | kubectl + 生产级 YAML，Deployment/Service/HPA/RBAC/PV 等，含 1.25-1.31 版本差异 |
| MySQL | 5.7/8.0/8.4，用户权限/备份恢复/主从复制/慢查询优化/InnoDB 调优/在线 DDL |

**核心能力：**
- 多 AI 提供商：OpenAI / Claude / DeepSeek / Ollama（本地 LLM）
- RAG 检索增强：ES 向量检索 + BM25 关键词检索，RRF 融合排序
- ReAct Agent：自动拆解复杂问题，调用工具完成多步推理
- 离线知识库：无需联网也可使用预置知识
- 官方文档爬取：Elastic / Docker / Kubernetes / MySQL 官方站点
- 插件系统：通过 `.so` 文件扩展新厂商支持
- 流式输出：SSE 实时推送生成结果

---

## 2. 系统要求

### 编译环境

| 依赖 | 版本 | 说明 |
|------|------|------|
| Go | 1.24+ | `go version` 确认 |
| Git | 任意 | 拉取代码 |
| Make | 可选 | 使用 Makefile 快捷命令 |

### 运行环境（最小配置）

| 服务 | 是否必需 | 说明 |
|------|---------|------|
| AI API Key | **必需** | OpenAI / Claude / DeepSeek 任意一个 |
| PostgreSQL 16+ | 可选 | 历史记录存储，不配置则跳过 |
| Redis 7+ | 可选 | 结果缓存，不配置则跳过 |
| Elasticsearch 8+ | 可选 | 向量检索 + RAG，不配置则 RAG 不可用 |
| Ollama | 可选 | 本地 LLM，无需 API Key |

> **最简运行**：只需一个 AI API Key 即可启动，ES/Redis/PG 均可选。

---

---

## Windows 快速开始（专项）

> 本节专门针对 Windows 10/11 开发者，**无需安装 make 工具**，使用 PowerShell 脚本替代。

### 第一步：安装环境

```powershell
# 安装 Go（推荐使用 winget，或从 https://go.dev/dl/ 下载 MSI 安装包）
winget install GoLang.Go

# 安装 Git
winget install Git.Git

# 安装 Docker Desktop（用于启动 ES/Redis/PG 依赖服务，可选）
# 下载地址: https://www.docker.com/products/docker-desktop/
# 安装后在设置中启用 WSL 2 后端（推荐）
winget install Docker.DockerDesktop

# 安装 Visual Studio Code（可选）
winget install Microsoft.VisualStudioCode
```

### 第二步：克隆并配置

```powershell
# 克隆项目
git clone https://github.com/A-small-character/cmdgen-platform.git
cd cmdgen-platform

# 复制配置文件
copy .env.example .env

# 用记事本编辑 .env，填写 AI API Key
notepad .env
# 或用 VSCode: code .env
```

`.env` 文件至少填写一个：
```
OPENAI_API_KEY=sk-your-openai-key
# 或
CLAUDE_API_KEY=sk-ant-your-claude-key
# 或
DEEPSEEK_API_KEY=sk-your-deepseek-key
```

### 第三步：一键启动

```powershell
# 设置 PowerShell 执行策略（首次使用，只需执行一次）
Set-ExecutionPolicy -ExecutionPolicy RemoteSigned -Scope CurrentUser

# 开发模式启动（自动加载 .env，启动 Docker 依赖，运行 go run）
.\scripts\start.ps1

# 或：跳过 Docker 依赖（最简模式，只需 AI Key）
.\scripts\start.ps1 -SkipDeps

# 或：生产模式（先编译再运行）
.\scripts\start.ps1 -Mode prod -BuildFirst
```

### 常用命令（PowerShell 版本）

```powershell
# 编译（当前平台 Windows）
.\scripts\build.ps1 -Target windows

# 编译所有平台
.\scripts\build.ps1 -Target all

# 初始化知识库（需先启动 ES）
.\scripts\init_kb.ps1

# 运行测试
go test ./... -v

# 清理编译产物
Remove-Item -Recurse -Force bin -ErrorAction SilentlyContinue

# 启动依赖服务（需要 Docker Desktop）
docker compose -f deployments/docker/docker-compose.yml up -d postgres redis elasticsearch

# 停止所有依赖服务
docker compose -f deployments/docker/docker-compose.yml down
```

### Windows 特别说明

| 功能 | Windows 支持情况 | 说明 |
|------|-----------------|------|
| 命令生成 API | ✅ 完全支持 | 所有 7 个模块正常工作 |
| RAG 检索 | ✅ 需要 ES | 通过 Docker Desktop 运行 ES |
| 流式输出 (SSE) | ✅ 完全支持 | |
| `.so` 动态插件 | ❌ 不支持 | 使用内置注册替代（见插件章节）|
| Docker 构建 | ✅ 需要 Docker Desktop | |
| K8s 部署 | ✅ 生成 YAML，部署到远程集群 | |
| 优雅关闭 | ✅ Ctrl+C | Windows 只响应 Interrupt 信号 |

### Windows 插件扩展方式

由于 Windows 不支持 Go `.so` 插件，使用**内置注册**方式扩展：

```go
// 1. 实现接口（新建 internal/plugins/my_vendor.go）
type MyVendorGenerator struct{}
func (g *MyVendorGenerator) Name() string { return "my-vendor" }
func (g *MyVendorGenerator) Version() string { return "1.0.0" }
func (g *MyVendorGenerator) Description() string { return "我的设备厂商" }
func (g *MyVendorGenerator) Initialize(map[string]interface{}) error { return nil }
func (g *MyVendorGenerator) Shutdown() error { return nil }
func (g *MyVendorGenerator) SupportedCategory() command.Category { return "my-vendor" }
func (g *MyVendorGenerator) Generate(ctx context.Context, req *command.GenerateRequest) (*command.GenerateResult, error) {
    // 实现生成逻辑
}

// 2. 在 cmd/server/main.go 的 run() 函数中注册
pluginMgr := plugins.NewManager(cfg.Plugin.Dir)
pluginMgr.RegisterGeneratorPlugin(&MyVendorGenerator{})
```

---

## 3. 快速开始（5分钟）

```bash
# 1. 克隆代码
git clone https://github.com/A-small-character/cmdgen-platform.git
cd cmdgen-platform

# 2. 复制并配置环境变量
cp .env.example .env
# 编辑 .env，至少填写一个 AI API Key：
#   OPENAI_API_KEY=sk-xxxx
# 或 CLAUDE_API_KEY=sk-ant-xxxx
# 或 DEEPSEEK_API_KEY=sk-xxxx

# 3. 编译
go build -o bin/cmdgen ./cmd/server

# 4. 运行（最简模式，无需 ES/Redis/PG）
./bin/cmdgen --config configs/config.yaml

# 5. 测试
curl -X POST http://localhost:8080/api/v1/generate \
  -H "Content-Type: application/json" \
  -d '{"input":"查找7天前大于1GB的日志文件"}'
```

---

## 4. 编译说明

### 4.1 本地编译（当前平台）

```bash
go build -o bin/cmdgen ./cmd/server
```

### 4.2 生产编译（缩小二进制体积）

```bash
go build \
  -ldflags="-w -s" \
  -trimpath \
  -o bin/cmdgen \
  ./cmd/server
```

参数说明：
- `-w`：去除 DWARF 调试信息
- `-s`：去除符号表
- `-trimpath`：去除源码路径（安全，防止泄露开发目录结构）

### 4.3 跨平台编译

```bash
# Linux AMD64（部署到服务器）
GOOS=linux GOARCH=amd64 go build -ldflags="-w -s" -o bin/cmdgen-linux-amd64 ./cmd/server

# Linux ARM64（部署到 ARM 服务器 / Apple Silicon Linux）
GOOS=linux GOARCH=arm64 go build -ldflags="-w -s" -o bin/cmdgen-linux-arm64 ./cmd/server

# Windows AMD64
GOOS=windows GOARCH=amd64 go build -ldflags="-w -s" -o bin/cmdgen-windows-amd64.exe ./cmd/server

# macOS Intel
GOOS=darwin GOARCH=amd64 go build -ldflags="-w -s" -o bin/cmdgen-darwin-amd64 ./cmd/server

# macOS Apple Silicon
GOOS=darwin GOARCH=arm64 go build -ldflags="-w -s" -o bin/cmdgen-darwin-arm64 ./cmd/server
```

或直接使用 Makefile：

```bash
make build       # 编译当前平台
make build-all   # 编译所有平台
```

### 4.4 Docker 镜像构建

```bash
# 标准构建
docker build -f deployments/docker/Dockerfile -t cmdgen-platform:latest .

# 多平台构建（需要 docker buildx）
docker buildx create --use
docker buildx build \
  --platform linux/amd64,linux/arm64 \
  -f deployments/docker/Dockerfile \
  -t ghcr.io/a-small-character/cmdgen-platform:1.0.0 \
  --push .
```

### 4.5 运行测试

```bash
go test ./... -v -race -timeout 60s

# 带覆盖率
go test ./... -coverprofile=coverage.out
go tool cover -html=coverage.out -o coverage.html
```

---

## 5. 配置说明

配置文件位于 `configs/config.yaml`，支持**环境变量覆盖**（格式：`SECTION_KEY`，如 `DATABASE_HOST`）。

### 5.1 最小配置（仅需 AI Key）

只需在 `configs/config.yaml` 中填写至少一个 AI 提供商的 API Key，或通过环境变量传入：

```bash
export OPENAI_API_KEY=sk-your-key-here
./bin/cmdgen
```

### 5.2 完整配置项说明

```yaml
app:
  name: "智能命令生成平台"
  version: "1.0.0"
  env: "development"   # development / production
  port: 8080
  debug: true          # production 环境设为 false

database:              # 可选，不配置则跳过
  host: "localhost"
  port: 5432
  name: "cmdgen"
  user: "postgres"
  password: "postgres"
  sslmode: "disable"   # 生产环境使用 require 或 verify-full
  max_open_conns: 25
  max_idle_conns: 5
  conn_max_lifetime: "5m"

redis:                 # 可选，不配置则跳过
  host: "localhost"
  port: 6379
  password: ""
  db: 0
  pool_size: 10

ai:
  default_provider: "openai"   # 默认提供商：openai / claude / deepseek / ollama
  timeout: 60                  # AI 调用超时秒数
  max_tokens: 4096             # 最大输出 Token
  temperature: 0.1             # 温度（0.0-1.0，越低越确定）

  openai:
    api_key: "${OPENAI_API_KEY}"          # 从环境变量读取
    base_url: "https://api.openai.com/v1" # 可替换为代理地址
    model: "gpt-4o"                       # 或 gpt-4o-mini, gpt-4-turbo 等

  claude:
    api_key: "${CLAUDE_API_KEY}"
    base_url: "https://api.anthropic.com"
    model: "claude-opus-4-8"

  deepseek:
    api_key: "${DEEPSEEK_API_KEY}"
    base_url: "https://api.deepseek.com/v1"
    model: "deepseek-chat"

  ollama:              # 本地 LLM，无需 API Key
    base_url: "http://localhost:11434"
    model: "llama3"    # 需提前 ollama pull llama3

vector:
  provider: "elasticsearch"
  dimension: 1536      # OpenAI text-embedding-ada-002 维度；Ollama 按模型调整

  elasticsearch:
    addresses:
      - "http://localhost:9200"
    username: ""       # ES 8.x 开启 Security 时填写
    password: ""
    index_prefix: "cmdgen_vectors"

log:
  level: "info"        # debug / info / warn / error
  format: "json"       # json / console
  output: "stdout"     # stdout / file
  file: "./logs/app.log"

plugin:
  dir: "./plugins"     # 插件目录
  enabled: true
```

### 5.3 环境变量优先级

环境变量格式：大写，`.` 替换为 `_`

```bash
# 示例：覆盖 ai.openai.api_key
export AI_OPENAI_API_KEY=sk-xxx

# 覆盖 database.host
export DATABASE_HOST=10.0.0.1

# 覆盖 app.env
export APP_ENV=production
```

---

## 6. 本地开发部署

### 6.1 仅启动依赖服务（推荐开发模式）

```bash
# 只启动 PostgreSQL + Redis + Elasticsearch
docker compose -f deployments/docker/docker-compose.yml \
  up -d postgres redis elasticsearch

# 等待 ES 健康（约30-60秒）
until curl -s http://localhost:9200/_cluster/health | grep -q '"status":"green"\|"status":"yellow"'; do
  echo "等待 Elasticsearch 就绪..."; sleep 5
done

# 在本地运行服务
export OPENAI_API_KEY=sk-your-key
go run ./cmd/server --config configs/config.yaml
```

### 6.2 初始化知识库（首次运行执行一次）

```bash
go run scripts/init_knowledge.go
```

输出示例：
```
索引成功: knowledge_base/linux/commands.yaml (4条)
索引成功: knowledge_base/elasticsearch/commands.yaml (5条)
索引成功: knowledge_base/network/huawei_switch.yaml (5条)
索引成功: knowledge_base/docker/commands.yaml (6条)
索引成功: knowledge_base/kubernetes/commands.yaml (7条)
索引成功: knowledge_base/mysql/commands.yaml (7条)

知识库初始化完成，共索引 34 条知识
```

### 6.3 热重载开发（推荐安装 air）

```bash
# 安装 air
go install github.com/air-verse/air@latest

# 创建 .air.toml
cat > .air.toml << 'EOF'
root = "."
tmp_dir = "tmp"
[build]
  cmd = "go build -o ./tmp/cmdgen ./cmd/server"
  bin = "./tmp/cmdgen --config configs/config.yaml"
  include_ext = ["go", "yaml"]
  exclude_dir = ["tmp", "bin", "vendor"]
EOF

# 启动热重载
air
```

---

## 7. Docker Compose 部署

### 7.1 准备环境变量

```bash
cp .env.example .env
# 编辑 .env 填写必要配置
vim .env
```

`.env` 内容：
```bash
OPENAI_API_KEY=sk-your-openai-key
CLAUDE_API_KEY=sk-ant-your-claude-key    # 可选
DEEPSEEK_API_KEY=sk-your-deepseek-key    # 可选
AI_DEFAULT_PROVIDER=openai
DB_PASSWORD=your-strong-password
JWT_SECRET=your-random-jwt-secret-min-32-chars
GRAFANA_PASSWORD=admin-password
```

### 7.2 启动核心服务

```bash
# 启动所有核心服务（API + PG + Redis + ES + Kibana）
docker compose -f deployments/docker/docker-compose.yml up -d

# 查看启动状态
docker compose -f deployments/docker/docker-compose.yml ps

# 查看 API 日志
docker compose -f deployments/docker/docker-compose.yml logs -f cmdgen-api
```

### 7.3 启动含监控的完整环境

```bash
# 额外启动 Prometheus + Grafana
docker compose -f deployments/docker/docker-compose.yml \
  --profile monitoring up -d

# 访问地址：
# API:       http://localhost:8080
# Kibana:    http://localhost:5601
# Prometheus: http://localhost:9091
# Grafana:   http://localhost:3001  (admin / GRAFANA_PASSWORD)
```

### 7.4 启动含本地 LLM 的环境

```bash
# 额外启动 Ollama（需要 NVIDIA GPU）
docker compose -f deployments/docker/docker-compose.yml \
  --profile local-llm up -d

# 拉取模型（首次需要，约数 GB）
docker exec cmdgen-ollama ollama pull llama3
docker exec cmdgen-ollama ollama pull nomic-embed-text  # 用于 Embedding
```

### 7.5 更新部署

```bash
# 重新构建并重启
docker compose -f deployments/docker/docker-compose.yml \
  up -d --build cmdgen-api

# 滚动更新（无停机）
docker compose -f deployments/docker/docker-compose.yml \
  up -d --no-deps --build cmdgen-api
```

### 7.6 数据持久化

所有数据存储在 Docker Volume 中，位于 Docker 数据目录：

```bash
# 查看 Volume
docker volume ls | grep cmdgen

# 备份 PostgreSQL
docker exec cmdgen-postgres pg_dump -U cmdgen cmdgen \
  | gzip > backup_$(date +%Y%m%d).sql.gz

# 备份 Elasticsearch
curl -X PUT "http://localhost:9200/_snapshot/backup/snap1" \
  -H 'Content-Type: application/json' \
  -d '{"indices": "cmdgen_*"}'
```

---

## 8. Kubernetes 生产部署

### 8.1 前提条件

```bash
# 确认 kubectl 可以连接集群
kubectl cluster-info

# 安装 cert-manager（TLS 证书自动签发）
kubectl apply -f https://github.com/cert-manager/cert-manager/releases/latest/download/cert-manager.yaml

# 安装 nginx-ingress（可选，按集群实际情况）
helm install ingress-nginx ingress-nginx/ingress-nginx \
  --namespace ingress-nginx --create-namespace
```

### 8.2 构建并推送镜像

```bash
# 替换为你的镜像仓库地址
export REGISTRY=your-registry.example.com
export IMAGE_TAG=1.0.0

docker build -f deployments/docker/Dockerfile \
  -t ${REGISTRY}/cmdgen-platform:${IMAGE_TAG} .

docker push ${REGISTRY}/cmdgen-platform:${IMAGE_TAG}
```

### 8.3 配置 Secret

```bash
# 修改 deployment.yaml 中的 Secret 值，或使用命令行创建
kubectl create namespace cmdgen

kubectl create secret generic cmdgen-secrets \
  --namespace cmdgen \
  --from-literal=db-password='your-strong-password' \
  --from-literal=openai-api-key='sk-your-key' \
  --from-literal=claude-api-key='sk-ant-your-key' \
  --from-literal=deepseek-api-key='sk-your-key' \
  --from-literal=jwt-secret='your-32-char-random-secret'
```

### 8.4 部署应用

```bash
# 修改 deployment.yaml 中的镜像地址
sed -i "s|ghcr.io/a-small-character/cmdgen-platform:1.0.0|${REGISTRY}/cmdgen-platform:${IMAGE_TAG}|g" \
  deployments/k8s/deployment.yaml

# 修改 Ingress 中的域名
sed -i 's/cmdgen.example.local/cmdgen.your-actual-domain.com/g' \
  deployments/k8s/deployment.yaml

# 部署
kubectl apply -f deployments/k8s/deployment.yaml

# 查看部署状态
kubectl get all -n cmdgen
kubectl rollout status deployment/cmdgen-api -n cmdgen
```

### 8.5 验证部署

```bash
# 查看 Pod 日志
kubectl logs -l app=cmdgen-api -n cmdgen --tail=50

# 端口转发测试（不通过 Ingress）
kubectl port-forward svc/cmdgen-api-service 8080:80 -n cmdgen

# 在另一个终端测试
curl http://localhost:8080/health
```

### 8.6 滚动更新

```bash
# 更新镜像
kubectl set image deployment/cmdgen-api \
  cmdgen-api=${REGISTRY}/cmdgen-platform:${NEW_TAG} \
  -n cmdgen

# 查看滚动更新状态
kubectl rollout status deployment/cmdgen-api -n cmdgen

# 回滚（如更新失败）
kubectl rollout undo deployment/cmdgen-api -n cmdgen
```

### 8.7 生产环境 Elasticsearch 部署建议

生产环境建议使用 Elastic Cloud 或自建 ECK（Elastic Cloud on Kubernetes）：

```bash
# 安装 ECK Operator
kubectl create -f https://download.elastic.co/downloads/eck/2.14.0/crds.yaml
kubectl apply -f https://download.elastic.co/downloads/eck/2.14.0/operator.yaml

# 创建 3 节点 ES 集群
cat <<EOF | kubectl apply -f -
apiVersion: elasticsearch.k8s.elastic.co/v1
kind: Elasticsearch
metadata:
  name: cmdgen-es
  namespace: cmdgen
spec:
  version: 8.16.0
  nodeSets:
  - name: default
    count: 3
    config:
      node.store.allow_mmap: false
    volumeClaimTemplates:
    - metadata:
        name: elasticsearch-data
      spec:
        accessModes: [ReadWriteOnce]
        storageClassName: fast-ssd
        resources:
          requests:
            storage: 100Gi
EOF
```

---

## 9. 知识库初始化

### 9.1 使用内置离线知识库

项目自带 34 条精选知识条目（来源于官方文档），首次启动后执行：

```bash
# 需要先启动 Elasticsearch
go run scripts/init_knowledge.go
```

### 9.2 从官方文档爬取

```bash
# 爬取并索引单个 URL（POST 请求）
curl -X POST http://localhost:8080/api/v1/knowledge/crawl \
  -H "Content-Type: application/json" \
  -d '{
    "url": "https://www.elastic.co/guide/en/elasticsearch/reference/current/cluster-health.html",
    "source": "elastic_docs"
  }'
```

支持的 `source` 值：
- `elastic_docs` — Elasticsearch 官方文档
- `docker_docs` — Docker 官方文档
- `kubernetes_docs` — Kubernetes 官方文档
- `mysql_docs` — MySQL 官方参考手册
- `cisco_docs` — Cisco 官方文档
- `huawei_docs` — 华为官方文档
- `linux_man` — Linux Man Pages

### 9.3 批量自定义索引

```bash
curl -X POST http://localhost:8080/api/v1/knowledge/index \
  -H "Content-Type: application/json" \
  -d '{
    "items": [
      {
        "title": "自定义知识条目",
        "content": "命令内容和说明...",
        "type": "linux",
        "tags": ["nginx", "config"],
        "source": "内部文档",
        "version": "1.0"
      }
    ]
  }'
```

---

## 10. API 使用说明

### 基础端点

```
GET  /health                          健康检查
GET  /metrics                         Prometheus 指标
```

### 命令生成

#### POST /api/v1/generate — 通用（自动识别类别）

```bash
curl -X POST http://localhost:8080/api/v1/generate \
  -H "Content-Type: application/json" \
  -d '{
    "input": "华为交换机创建VLAN100并配置Trunk口",
    "provider": "openai",
    "use_rag": true,
    "options": {
      "vendor": "huawei",
      "multi_vendor": true
    }
  }'
```

#### POST /api/v1/generate/linux — Linux 专用

```bash
curl -X POST http://localhost:8080/api/v1/generate/linux \
  -H "Content-Type: application/json" \
  -d '{
    "input": "查找7天前大于1GB的日志文件并删除",
    "options": {
      "os_family": "rhel",
      "os_version": "8",
      "include_explain": true,
      "include_backup": true
    }
  }'
```

#### POST /api/v1/generate/elasticsearch — ES 专用

```bash
curl -X POST http://localhost:8080/api/v1/generate/elasticsearch \
  -H "Content-Type: application/json" \
  -d '{
    "input": "创建一个带ILM策略的日志索引，热节点保留7天，冷节点保留30天",
    "options": {
      "es_version": "8.x",
      "output_format": "all"
    }
  }'
```

#### POST /api/v1/generate/docker — Docker 专用

```bash
# Docker CLI 命令
curl -X POST http://localhost:8080/api/v1/generate/docker \
  -H "Content-Type: application/json" \
  -d '{"input": "运行一个限制内存512M、CPU 0.5核的nginx容器，端口映射80:80"}'

# Dockerfile 生成
curl -X POST http://localhost:8080/api/v1/generate/docker \
  -H "Content-Type: application/json" \
  -d '{"input": "为Go 1.24应用写一个生产级多阶段构建Dockerfile，使用distroless基础镜像"}'
```

#### POST /api/v1/generate/kubernetes — K8s 专用

```bash
curl -X POST http://localhost:8080/api/v1/generate/kubernetes \
  -H "Content-Type: application/json" \
  -d '{
    "input": "部署一个3副本的Nginx，要求无损滚动更新、配置HPA（CPU 70%扩容）、反亲和性分散到不同节点",
    "options": {
      "k8s_version": "1.31",
      "output_yaml": true
    }
  }'
```

#### POST /api/v1/generate/mysql — MySQL 专用

```bash
curl -X POST http://localhost:8080/api/v1/generate/mysql \
  -H "Content-Type: application/json" \
  -d '{
    "input": "配置MySQL 8.0主从复制，使用GTID模式",
    "options": {
      "mysql_version": "8.0"
    }
  }'
```

#### POST /api/v1/generate/network — 网络设备专用

```bash
curl -X POST http://localhost:8080/api/v1/generate/network \
  -H "Content-Type: application/json" \
  -d '{
    "input": "配置OSPF Area 0，认证方式MD5",
    "options": {
      "vendor": "huawei",
      "multi_vendor": true
    }
  }'
```

#### POST /api/v1/generate/stream — 流式输出（SSE）

```bash
curl -N -X POST http://localhost:8080/api/v1/generate/stream \
  -H "Content-Type: application/json" \
  -d '{"input": "解释iptables的四表五链"}'
# 返回 SSE 事件流：
# data: {"chunk": "iptables..."}
# event: done
```

### 响应格式

**成功**
```json
{
  "code": 0,
  "data": {
    "id": "uuid",
    "category": "linux",
    "commands": [
      {
        "id": "1",
        "title": "查找并列出大文件",
        "command": "find /var/log -type f -mtime +7 -size +1G -exec ls -lh {} \\;",
        "format": "cli",
        "explanation": "find: 在/var/log中查找文件(-type f)，修改时间超过7天(-mtime +7)，大小超过1GB(-size +1G)",
        "risk": "low",
        "backup_cmd": "",
        "rollback_cmd": ""
      }
    ],
    "explanation": "以下命令用于查找满足条件的日志文件...",
    "warnings": [],
    "metadata": {
      "ai_provider": "openai",
      "model_name": "gpt-4o",
      "tokens_used": 380,
      "latency": 1823000000,
      "rag_used": false
    }
  }
}
```

**失败**
```json
{
  "code": 5005,
  "message": "命令生成失败",
  "detail": "AI provider timeout"
}
```

### 错误码说明

| 错误码 | HTTP状态 | 含义 |
|--------|---------|------|
| 1000 | 400 | 无效请求 |
| 1001 | 400 | 参数验证失败 |
| 2000 | 401 | 未授权 |
| 4290 | 429 | 请求过于频繁（限流）|
| 5000 | 500 | 服务器内部错误 |
| 5001 | 500 | AI 服务调用失败 |
| 5002 | 500 | 向量搜索失败 |
| 5003 | 500 | 文档抓取失败 |
| 5005 | 500 | 命令生成失败 |
| 5010 | 503 | 依赖服务未初始化 |

### 知识库 API

```bash
# 搜索知识库
GET /api/v1/knowledge/search?q=VLAN配置&type=network&top_k=5

# 搜索官方文档（联网）
GET /api/v1/knowledge/docs/search?q=ILM+policy&source=elastic_docs

# 批量索引
POST /api/v1/knowledge/index

# 爬取并索引官方文档
POST /api/v1/knowledge/crawl
```

---

## 11. 功能模块说明

### 11.1 自动类别识别

`POST /api/v1/generate` 接口无需指定 `category`，系统会根据关键词自动识别：

| 输入示例 | 识别类别 |
|---------|---------|
| "kubernetes 部署 nginx" | kubernetes |
| "docker run 限制内存" | docker |
| "mysql 主从复制" | mysql |
| "elasticsearch 集群健康" | elasticsearch |
| "华为交换机 VLAN" | network |
| "查找大文件" | linux（默认）|

### 11.2 RAG 检索增强

开启 RAG 后（`"use_rag": true`），系统执行：
1. 对用户输入生成向量（OpenAI Embedding）
2. ES kNN 向量检索（语义相似）
3. ES BM25 关键词检索（精确匹配）
4. RRF 融合排序（两路结果合并）
5. 将 Top-5 知识片段注入 Prompt

**前提**：需要配置 Elasticsearch 并执行知识库初始化。

### 11.3 多厂商对比

`"multi_vendor": true` 时，网络模块会同时生成 Huawei/Cisco/H3C 三家配置并标注差异：

```json
{
  "input": "配置VLAN100",
  "options": {"multi_vendor": true}
}
```

### 11.4 AI 提供商切换

每次请求可指定不同提供商：

```json
{"input": "...", "provider": "claude"}
{"input": "...", "provider": "deepseek"}
{"input": "...", "provider": "ollama"}
```

不指定时使用配置文件中的 `ai.default_provider`。

---

## 12. 插件开发指南

### 12.1 创建生成器插件

新建 `plugins/my-vendor/main.go`：

```go
package main

import (
    "context"
    "github.com/cmdgen/platform/internal/domain/command"
)

// MyVendorPlugin 示例：新增设备厂商插件
type MyVendorPlugin struct{}

func (p *MyVendorPlugin) Name() string        { return "my-vendor-plugin" }
func (p *MyVendorPlugin) Version() string     { return "1.0.0" }
func (p *MyVendorPlugin) Description() string { return "我的设备厂商命令生成器" }

func (p *MyVendorPlugin) Initialize(config map[string]interface{}) error {
    return nil
}

func (p *MyVendorPlugin) Shutdown() error { return nil }

func (p *MyVendorPlugin) SupportedCategory() command.Category {
    return "my-vendor"
}

func (p *MyVendorPlugin) Generate(ctx context.Context, req *command.GenerateRequest) (*command.GenerateResult, error) {
    result := command.NewGenerateResult(req.ID, "my-vendor")
    result.Commands = []command.CommandItem{
        {
            ID:          "1",
            Title:       "示例命令",
            Command:     "my-vendor-cli config vlan 100",
            Format:      command.FormatCLI,
            Explanation: "创建VLAN 100",
            Risk:        command.RiskLow,
        },
    }
    return result, nil
}

// NewPlugin 插件入口（必须导出此函数）
func NewPlugin() interface{} {
    return &MyVendorPlugin{}
}
```

### 12.2 编译插件

```bash
# Linux/macOS（Go plugin 只支持这两个平台）
go build -buildmode=plugin -o plugins/my-vendor.so ./plugins/my-vendor/

# 将 .so 文件放入 configs/config.yaml 中 plugin.dir 指定的目录
# 服务启动时自动加载
```

> **注意**：Go plugin 目前只支持 Linux 和 macOS，Windows 不支持 `.so` 插件。Windows 开发时建议直接在代码中注册生成器，或通过 gRPC 实现跨进程插件。

---

## 13. 监控与运维

### 13.1 健康检查

```bash
curl http://localhost:8080/health
# 返回：{"status":"ok","service":"cmdgen-platform"}
```

### 13.2 Prometheus 指标

```bash
curl http://localhost:9090/metrics
```

主要指标：
- `http_requests_total` — 请求总量（按路径、方法、状态码）
- `http_request_duration_seconds` — 请求延迟直方图
- `go_goroutines` — Goroutine 数量
- `go_memstats_alloc_bytes` — 内存使用

### 13.3 日志查看

```bash
# Docker Compose 环境
docker compose logs -f cmdgen-api

# Kubernetes 环境
kubectl logs -l app=cmdgen-api -n cmdgen -f --tail=100

# 本地运行（JSON 日志）
./bin/cmdgen 2>&1 | jq '.'
```

### 13.4 常用运维命令

```bash
# 查看当前配置（不含敏感信息）
curl http://localhost:8080/health

# 查看已加载的 AI 提供商（自定义实现，可通过日志确认）
# 启动时日志中会打印：INFO 启动智能命令生成平台

# 强制重新加载（重启服务）
kill -SIGTERM $(pidof cmdgen)
# 服务收到信号后优雅关闭（等待处理中的请求完成，最多30秒）
```

---

## 14. 常见问题

### Q1: 启动报错 "加载配置失败"

```
加载配置失败: open configs/config.yaml: no such file or directory
```

**解决**：确认在项目根目录运行，或指定配置文件路径：
```bash
./bin/cmdgen --config /absolute/path/to/config.yaml
```

---

### Q2: AI 调用失败 "AI provider not found"

**原因**：未配置任何 AI 提供商的 API Key。

**解决**：
```bash
# 方式1：环境变量
export OPENAI_API_KEY=sk-your-key
./bin/cmdgen

# 方式2：修改 configs/config.yaml
ai:
  openai:
    api_key: "sk-your-key"
```

---

### Q3: RAG 检索不可用

日志提示：`未配置向量数据库，RAG 检索功能不可用`

**解决**：
```bash
# 启动 Elasticsearch
docker run -d --name es \
  -e "discovery.type=single-node" \
  -e "xpack.security.enabled=false" \
  -e "ES_JAVA_OPTS=-Xms1g -Xmx1g" \
  -p 9200:9200 \
  docker.elastic.co/elasticsearch/elasticsearch:8.16.0

# 配置 ES 地址
# configs/config.yaml:
# vector.elasticsearch.addresses: ["http://localhost:9200"]

# 初始化知识库
go run scripts/init_knowledge.go
```

---

### Q4: ES 健康检查失败（Docker Compose 中）

**原因**：ES 启动慢，其他服务启动过快。

**解决**：等待约 60 秒让 ES 完全就绪，或手动检查：
```bash
docker compose logs elasticsearch | tail -20
curl http://localhost:9200/_cluster/health?pretty
```

---

### Q5: Kubernetes 部署后 Pod 一直 Pending

```bash
kubectl describe pod <pod-name> -n cmdgen
```

常见原因：
- `Insufficient memory/cpu`：调整 `resources.requests`
- `No nodes are available`：检查节点资源
- `ImagePullBackOff`：检查镜像仓库地址和 `imagePullSecrets`

---

### Q6: 跨平台编译出现 CGO 错误

```
# cgo: cannot load DWARF output
```

**解决**：禁用 CGO（大多数情况下不需要）：
```bash
CGO_ENABLED=0 GOOS=linux go build ./cmd/server
```

---

### Q7: Ollama 本地模型响应很慢

**原因**：模型体积大，CPU 推理慢。

**建议**：
```bash
# 使用量化版小模型
ollama pull llama3.2:3b      # 3B 参数，速度快
ollama pull qwen2.5:7b       # 中文能力较强

# 修改配置
ai:
  ollama:
    model: "llama3.2:3b"
```

---

### Q8: Windows 上插件无法加载

Go plugin 不支持 Windows。开发替代方案：

```go
// 在 cmd/server/main.go 中直接注册
eng.RegisterGenerator(&MyCustomGenerator{})
```

---

## 15. 已知 Bug 修复记录

以下 Bug 在 v1.0.1 中已修复：

| 编号 | 位置 | 问题描述 | 修复方案 |
|------|------|---------|---------|
| BUG-001 | `internal/infrastructure/ai/ollama.go` | `Embed()` 循环中使用 `defer resp.Body.Close()`，导致所有 HTTP 连接在函数返回前才关闭，高并发下造成连接泄漏 | 提取 `embedOne()` 独立函数，每次调用函数返回时立即关闭连接 |
| BUG-002 | `pkg/config/config.go` | `once.Do` 内部通过外部变量 `err` 传递错误，第二次调用 `Load()` 时 `once.Do` 不执行导致 `err` 永远为 nil，若首次失败则 `instance` 为 nil 但无错误返回 | 将 `err` 提升为包级变量 `loadErr`，在 `once.Do` 外部直接读取 |
| BUG-003 | `internal/interfaces/http/handler/knowledge.go` | `Search`/`BatchIndex`/`CrawlAndIndex` 未检查 `ragService`/`indexService` 是否为 nil，ES 未配置时调用导致 panic | 各 handler 方法入口添加 nil 检查，返回 503 ServiceUnavailable |
| BUG-004 | `internal/interfaces/http/middleware/middleware.go` | `RateLimit` 使用 Ticker channel 共享 token，高并发下行为不确定（Ticker 只保证每 1/N 秒产生一个 tick，不是真正的令牌桶限流）| 改为正确的令牌桶实现：带缓冲 channel 存放 tokens + goroutine 定时补充 |
| BUG-005 | `internal/application/generator/engine.go` | 变量名 `context_docs` 使用下划线命名，不符合 Go 惯例，且与 `context` 包名视觉混淆 | 重命名为 `ragDocs` |
| BUG-006 | `cmd/server/main.go` | `db`、`cacheClient`、`knowledgeRepo`、`pluginMgr` 初始化后均被 `_ =` 静默，数据库迁移、缓存集成等功能实际未接入 | 完整集成 DB 迁移逻辑，正确接入 RAG 服务，插件系统条件加载 |
| BUG-007 | `go.mod` | 声明了 `bleve`、`gin-contrib/cors`、`gin-contrib/requestid`、`golang-jwt`、`lib/pq` 等包但从未在代码中导入，造成无效依赖膨胀 | 移除未使用的声明，`go mod tidy` 清理 |
