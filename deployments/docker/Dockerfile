# ============================================================
# 多阶段构建 - 智能命令生成平台
# ============================================================

# 构建阶段
FROM golang:1.24-alpine AS builder

WORKDIR /build

# 安装依赖
RUN apk add --no-cache git ca-certificates tzdata

# 复制依赖文件并下载
COPY go.mod go.sum ./
RUN go mod download

# 复制源码
COPY . .

# 编译（CGO禁用以获得静态链接二进制）
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build \
    -ldflags="-w -s -X main.BuildTime=$(date -u +%Y-%m-%dT%H:%M:%SZ) -X main.GitCommit=$(git rev-parse --short HEAD 2>/dev/null || echo unknown)" \
    -trimpath \
    -o cmdgen \
    ./cmd/server

# ============================================================
# 运行阶段
FROM alpine:3.20

WORKDIR /app

# 安装运行时依赖
RUN apk add --no-cache ca-certificates tzdata curl && \
    cp /usr/share/zoneinfo/Asia/Shanghai /etc/localtime && \
    echo "Asia/Shanghai" > /etc/timezone

# 创建非root用户
RUN addgroup -S cmdgen && adduser -S cmdgen -G cmdgen

# 从构建阶段复制二进制
COPY --from=builder /build/cmdgen /app/cmdgen
COPY --from=builder /build/configs /app/configs
COPY --from=builder /build/knowledge_base /app/knowledge_base

# 创建数据目录
RUN mkdir -p /app/logs /app/plugins /app/data && \
    chown -R cmdgen:cmdgen /app

USER cmdgen

EXPOSE 8080 9090

HEALTHCHECK --interval=30s --timeout=10s --start-period=10s --retries=3 \
    CMD curl -f http://localhost:8080/health || exit 1

ENTRYPOINT ["/app/cmdgen"]
CMD ["--config", "/app/configs/config.yaml"]
