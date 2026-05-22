.PHONY: up down migrate seed run test test-int build

up:
	docker compose up -d

down:
	docker compose down

migrate:
	go run ./cmd/migrate -action=up

run:
	go run ./cmd/server

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
