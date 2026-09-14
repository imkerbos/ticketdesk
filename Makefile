.PHONY: all build run dev test clean lint palette-check fmt swagger wire dev-docker-up dev-docker-up-d dev-docker-down dev-docker-logs dev-docker-rebuild prod prod-d prod-stop prod-logs prod-rebuild prod-ps helm-lint helm-template helm-install helm-upgrade helm-uninstall init migrate help

# 变量定义
APP_NAME := ticketdesk
BUILD_DIR := bin
MAIN_FILE := cmd/ticketdesk/main.go
CONFIG_FILE := configs/config-dev.yaml

# Go 相关
GOCMD := go
GOBUILD := $(GOCMD) build
GORUN := $(GOCMD) run
GOTEST := $(GOCMD) test
GOMOD := $(GOCMD) mod
GOFMT := gofmt
GOIMPORTS := goimports

# 构建标志
LDFLAGS := -ldflags "-s -w"

# 默认目标
all: lint test build

# 帮助信息
help:
	@echo "TicketDesk Makefile"
	@echo ""
	@echo "Usage:"
	@echo "  make init          - 初始化项目（安装依赖和工具）"
	@echo "  make build         - 构建项目"
	@echo "  make run           - 运行项目"
	@echo "  make dev           - 开发模式（前后端热更新）"
	@echo "  make dev-backend   - 仅启动后端（热更新）"
	@echo "  make dev-frontend  - 仅启动前端（热更新）"
	@echo "  make test          - 运行测试"
	@echo "  make lint          - 代码静态检查"
	@echo "  make fmt           - 格式化代码"
	@echo "  make swagger       - 生成 API 文档"
	@echo "  make wire          - 生成依赖注入代码"
	@echo "  make migrate       - 运行数据库迁移"
	@echo ""
	@echo "Docker Dev（开发环境）:"
	@echo "  make dev-docker-up      - Docker 开发模式（前后端热更新）"
	@echo "  make dev-docker-up-d    - Docker 开发模式（后台运行）"
	@echo "  make dev-docker-down    - 停止 Docker 开发容器"
	@echo "  make dev-docker-logs    - 查看 Docker 开发日志"
	@echo "  make dev-docker-rebuild - 重建 Docker 开发容器"
	@echo ""
	@echo "Production（生产部署）:"
	@echo "  make prod          - 生产环境启动（前台）"
	@echo "  make prod-d        - 生产环境启动（后台）"
	@echo "  make prod-stop     - 停止生产环境"
	@echo "  make prod-logs     - 查看生产环境日志"
	@echo "  make prod-rebuild  - 重建生产环境镜像并启动"
	@echo "  make prod-ps       - 查看生产环境容器状态"
	@echo ""
	@echo "Kubernetes（Helm 部署）:"
	@echo "  make helm-lint     - 校验 Helm chart"
	@echo "  make helm-template - 预览渲染结果"
	@echo "  make helm-install  - 首次安装"
	@echo "  make helm-upgrade  - 升级部署"
	@echo "  make helm-uninstall- 卸载"
	@echo ""
	@echo "  make clean         - 清理构建产物"
	@echo ""

# 初始化项目
init:
	@echo ">>> 安装后端依赖..."
	$(GOMOD) download
	$(GOMOD) tidy
	@echo ">>> 安装开发工具..."
	go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest
	go install github.com/swaggo/swag/cmd/swag@latest
	go install github.com/google/wire/cmd/wire@latest
	go install golang.org/x/tools/cmd/goimports@latest
	go install github.com/air-verse/air@latest
	@echo ">>> 安装前端依赖..."
	cd web && npm install
	@echo ">>> 初始化完成"

# 构建
build:
	@echo ">>> 构建项目..."
	@mkdir -p $(BUILD_DIR)
	$(GOBUILD) $(LDFLAGS) -o $(BUILD_DIR)/$(APP_NAME) $(MAIN_FILE)
	@echo ">>> 构建完成: $(BUILD_DIR)/$(APP_NAME)"

# 运行
run:
	@echo ">>> 运行项目..."
	$(GORUN) $(MAIN_FILE) -config $(CONFIG_FILE)

# 开发模式（热更新）- 同时启动前后端
dev:
	@echo ">>> 开发模式启动（前后端热更新）..."
	@bash scripts/dev.sh

# 仅启动后端（热更新）
dev-backend:
	@echo ">>> 后端开发模式启动（热更新）..."
	air -c .air.toml

# 仅启动前端（热更新）
dev-frontend:
	@echo ">>> 前端开发模式启动（热更新）..."
	cd web && npm run dev

# 测试
test:
	@echo ">>> 运行测试..."
	$(GOTEST) -v -race -cover ./...

# 测试覆盖率
test-coverage:
	@echo ">>> 生成测试覆盖率报告..."
	$(GOTEST) -v -race -coverprofile=coverage.out ./...
	$(GOCMD) tool cover -html=coverage.out -o coverage.html
	@echo ">>> 覆盖率报告: coverage.html"

# 代码检查
lint:
	@echo ">>> 代码静态检查..."
	golangci-lint run ./...

# 色板可辨性校验（六项：明度带 / 彩度下限 / 色觉障碍相邻分离度 /
# 正常视力分离度 / 对比度 / 亮暗一致性）。改了 --td-* 色值就必须跑。
palette-check:
	@echo ">>> 色板可辨性校验..."
	node scripts/palette-check.mjs

# 格式化代码
fmt:
	@echo ">>> 格式化代码..."
	$(GOFMT) -s -w .
	$(GOIMPORTS) -w .

# 生成 Swagger 文档
swagger:
	@echo ">>> 生成 API 文档..."
	swag init -g $(MAIN_FILE) -o docs/swagger
	@echo ">>> API 文档生成完成: docs/swagger"

# 生成 Wire 依赖注入代码
wire:
	@echo ">>> 生成依赖注入代码..."
	wire ./...

# 数据库迁移
migrate:
	@echo ">>> 运行数据库迁移..."
	$(GORUN) $(MAIN_FILE) -config $(CONFIG_FILE) -migrate

# Docker 开发模式（前后端热更新）
dev-docker-up:
	@echo ">>> Docker 开发模式启动（前后端热更新）..."
	docker-compose -f deploy/docker-compose.dev.yaml up --build

# Docker 开发模式（后台运行）
dev-docker-up-d:
	@echo ">>> Docker 开发模式启动（后台运行）..."
	docker-compose -f deploy/docker-compose.dev.yaml up --build -d

# Docker 开发模式停止
dev-docker-down:
	@echo ">>> 停止 Docker 开发容器..."
	docker-compose -f deploy/docker-compose.dev.yaml down

# Docker 开发模式日志
dev-docker-logs:
	@echo ">>> 查看 Docker 开发容器日志..."
	docker-compose -f deploy/docker-compose.dev.yaml logs -f

# Docker 开发模式重建
dev-docker-rebuild:
	@echo ">>> 重建 Docker 开发容器..."
	docker-compose -f deploy/docker-compose.dev.yaml up --build --force-recreate

# ============ 生产环境部署 ============

# 生产环境启动（前台，可看到日志）
prod:
	@echo ">>> 生产环境启动..."
	@if [ ! -f deploy/.env ]; then \
		echo "⚠️  未找到 deploy/.env 文件，正在从 .env.example 复制..."; \
		cp deploy/.env.example deploy/.env; \
		echo "📝 请修改 deploy/.env 中的配置（尤其是密码和 JWT_SECRET），然后重新运行"; \
		exit 1; \
	fi
	docker compose -f deploy/docker-compose.yaml --env-file deploy/.env up --build

# 生产环境启动（后台运行）
prod-d:
	@echo ">>> 生产环境启动（后台）..."
	@if [ ! -f deploy/.env ]; then \
		echo "⚠️  未找到 deploy/.env 文件，正在从 .env.example 复制..."; \
		cp deploy/.env.example deploy/.env; \
		echo "📝 请修改 deploy/.env 中的配置（尤其是密码和 JWT_SECRET），然后重新运行"; \
		exit 1; \
	fi
	docker compose -f deploy/docker-compose.yaml --env-file deploy/.env up --build -d
	@echo ">>> 生产环境已启动，访问 http://localhost:$$(grep WEB_PORT deploy/.env 2>/dev/null | cut -d= -f2 || echo 80)"

# 生产环境停止
prod-stop:
	@echo ">>> 停止生产环境..."
	docker compose -f deploy/docker-compose.yaml --env-file deploy/.env down

# 生产环境日志
prod-logs:
	docker compose -f deploy/docker-compose.yaml --env-file deploy/.env logs -f

# 生产环境重建
prod-rebuild:
	@echo ">>> 重建生产环境镜像并启动..."
	docker compose -f deploy/docker-compose.yaml --env-file deploy/.env up --build --force-recreate -d

# 生产环境容器状态
prod-ps:
	docker compose -f deploy/docker-compose.yaml --env-file deploy/.env ps

# ============ Kubernetes Helm 部署 ============

HELM_RELEASE := ticketdesk
HELM_CHART := deploy/helm
HELM_NAMESPACE := ticketdesk

# Helm 校验
helm-lint:
	helm lint $(HELM_CHART)

# Helm 预览渲染结果
helm-template:
	helm template $(HELM_RELEASE) $(HELM_CHART) -n $(HELM_NAMESPACE)

# Helm 首次安装
helm-install:
	helm install $(HELM_RELEASE) $(HELM_CHART) -n $(HELM_NAMESPACE) --create-namespace

# Helm 升级
helm-upgrade:
	helm upgrade $(HELM_RELEASE) $(HELM_CHART) -n $(HELM_NAMESPACE)

# Helm 卸载
helm-uninstall:
	helm uninstall $(HELM_RELEASE) -n $(HELM_NAMESPACE)

# 清理
clean:
	@echo ">>> 清理构建产物..."
	rm -rf $(BUILD_DIR)
	rm -rf tmp/
	rm -f coverage.out coverage.html build-errors.log
	@echo ">>> 清理完成"
