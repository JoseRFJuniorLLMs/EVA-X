.PHONY: help build run test docker-build docker-run clean

help:
	@echo "EVA-Mind - Comandos disponíveis:"
	@echo "  make build        - Build do binário"
	@echo "  make run          - Executa local"
	@echo "  make test         - Roda testes"
	@echo "  make docker-build - Build da imagem Docker"
	@echo "  make docker-run   - Executa no Docker"
	@echo "  make clean        - Limpa builds"

build:
	@echo "🔨 Building EVA-Mind..."
	go build -o bin/eva-mind ./cmd/server
	@echo "✓ Build complete: bin/eva-mind"

run:
	@echo "🚀 Starting EVA-Mind..."
	go run ./cmd/server/main.go

test:
	@echo "🧪 Running tests..."
	go test -v -race -coverprofile=coverage.out ./...
	go tool cover -html=coverage.out -o coverage.html
	@echo "✓ Coverage report: coverage.html"

docker-build:
	@echo "🐳 Building Docker image..."
	docker build -t eva-mind:latest .
	@echo "✓ Image built: eva-mind:latest"

docker-run:
	@echo "🐳 Running Docker container..."
	docker run -p 8080:8080 -p 9090:9090 --env-file .env eva-mind:latest

clean:
	@echo "🧹 Cleaning..."
	rm -rf bin/
	rm -f coverage.out coverage.html
	@echo "✓ Clean complete"

.DEFAULT_GOAL := help
