.PHONY: build run test lint clean docker-build docker-up docker-down init-kb

# ============================================================
# 构建
# ============================================================
build:
	go build -ldflags="-w -s" -o bin/cmdgen ./cmd/server

# 跨平台编译
build-all:
	GOOS=linux   GOARCH=amd64 go build -o bin/cmdgen-linux-amd64    ./cmd/server
	GOOS=windows GOARCH=amd64 go build -o bin/cmdgen-windows-amd64.exe ./cmd/server
	GOOS=darwin  GOARCH=amd64 go build -o bin/cmdgen-darwin-amd64   ./cmd/server
	GOOS=darwin  GOARCH=arm64 go build -o bin/cmdgen-darwin-arm64   ./cmd/server

# ============================================================
# 运行
# ============================================================
run:
	go run ./cmd/server --config configs/config.yaml

run-dev:
	APP_ENV=development go run ./cmd/server --config configs/config.yaml

# ============================================================
# 测试
# ============================================================
test:
	go test ./... -v -race -timeout 60s

test-cover:
	go test ./... -coverprofile=coverage.out
	go tool cover -html=coverage.out -o coverage.html

# ============================================================
# 代码质量
# ============================================================
lint:
	golangci-lint run ./...

fmt:
	gofmt -w .
	goimports -w .

vet:
	go vet ./...

# ============================================================
# 依赖
# ============================================================
tidy:
	go mod tidy

deps:
	go mod download

# ============================================================
# Docker
# ============================================================
docker-build:
	docker build -f deployments/docker/Dockerfile -t cmdgen-platform:latest .

docker-up:
	docker compose -f deployments/docker/docker-compose.yml up -d

docker-up-monitoring:
	docker compose -f deployments/docker/docker-compose.yml --profile monitoring up -d

docker-down:
	docker compose -f deployments/docker/docker-compose.yml down

docker-logs:
	docker compose -f deployments/docker/docker-compose.yml logs -f cmdgen-api

# ============================================================
# K8s
# ============================================================
k8s-deploy:
	kubectl apply -f deployments/k8s/

k8s-delete:
	kubectl delete -f deployments/k8s/

k8s-status:
	kubectl get all -n cmdgen

# ============================================================
# 知识库
# ============================================================
init-kb:
	go run scripts/init_knowledge.go

# ============================================================
# 清理
# ============================================================
clean:
	rm -rf bin/ coverage.out coverage.html
