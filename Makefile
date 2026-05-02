.PHONY: help tidy build dev up down logs migrate test fmt vet api worker cli

help:
	@echo "Auditrail — Makefile targets"
	@echo "  make tidy      Download Go modules"
	@echo "  make build     Build all binaries"
	@echo "  make up        Start dev stack (docker-compose)"
	@echo "  make down      Stop dev stack"
	@echo "  make logs      Tail logs"
	@echo "  make api       Run API locally"
	@echo "  make worker    Run worker locally"
	@echo "  make migrate   Apply Postgres migrations locally"
	@echo "  make test      Run tests"

tidy:
	go mod tidy

build:
	go build -o bin/auditrail-api    ./cmd/api
	go build -o bin/auditrail-worker ./cmd/worker
	go build -o bin/auditrail        ./cmd/cli

up:
	docker compose up -d --build

down:
	docker compose down

logs:
	docker compose logs -f api worker

api:
	go run ./cmd/api

worker:
	go run ./cmd/worker

cli:
	go run ./cmd/cli $(ARGS)

migrate:
	go run ./cmd/cli migrate

test:
	go test ./... -race -count=1

fmt:
	gofmt -s -w .

vet:
	go vet ./...
