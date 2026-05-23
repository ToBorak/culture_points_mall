.PHONY: help bootstrap up down restart reset logs ps test test-int build migrate seed dev

help:
	@echo "culture-points-mall · 常用命令"
	@echo ""
	@echo "  make bootstrap   一键启动（首次推荐）：装依赖 → 起容器 → 重置 DB → 起服务"
	@echo "  make up          仅启动后端 + 前端进程"
	@echo "  make down        停止所有服务进程"
	@echo "  make restart     stop → up（保留 DB）"
	@echo "  make reset       重置数据库（drop + migrate + seed）"
	@echo "  make logs        tail 后端 + MCP + 前端日志"
	@echo "  make ps          查看 docker 容器状态"
	@echo "  make test        跑后端单元测试"
	@echo "  make test-int    跑集成测试（需 docker）"
	@echo "  make build       编译 server / mcp / migrate 到 bin/"
	@echo ""

bootstrap:
	./bootstrap.sh

up:
	./scripts/start-services.sh

down:
	./scripts/stop.sh

restart: down up

reset:
	./scripts/reset-db.sh

logs:
	@if [ ! -d .logs ]; then echo "no .logs/ yet, run make up first"; exit 1; fi
	tail -f .logs/*.log

ps:
	docker compose ps

# 后端开发独立目标
dev: migrate seed
	go run ./cmd/server

migrate:
	go run ./cmd/migrate -action=up

seed:
	go run ./cmd/migrate -action=seed

test:
	go test -race ./...

test-int:
	docker compose -f docker-compose.test.yml up -d
	@sleep 8
	go test -tags=integration -race ./...
	docker compose -f docker-compose.test.yml down

build:
	go build -o bin/server ./cmd/server
	go build -o bin/mcp ./cmd/mcp
	go build -o bin/migrate ./cmd/migrate
