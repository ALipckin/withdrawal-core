.PHONY: help docker-up docker-down docker-logs docker-clean shell test lint fmt build

help:
	@echo "Available commands:"
	@echo "  make docker-up    - Build and start app + db in Docker"
	@echo "  make docker-down  - Stop Docker services"
	@echo "  make docker-logs  - Show app logs"
	@echo "  make docker-clean - Stop services and remove volumes"
	@echo "  make shell        - Open shell in app container"
	@echo "  make build        - Build API binary in app container"
	@echo "  make test         - Run tests in app container"
	@echo "  make lint         - Run go vet in app container"
	@echo "  make fmt          - Run gofmt in app container"

docker-up:
	@docker compose up -d --build

docker-down:
	@docker compose down

docker-logs:
	@docker compose logs -f app

docker-clean:
	@docker compose down -v

shell:
	@docker compose exec app /bin/sh

build:
	@docker compose exec app go build -o /tmp/api ./cmd/api

test:
	@docker compose exec app go test ./... -v -count=1

lint:
	@docker compose exec app go vet ./...

fmt:
	@docker compose exec app sh -c 'find . -name "*.go" -print0 | xargs -0 gofmt -w'
