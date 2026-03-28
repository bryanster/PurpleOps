.PHONY: help dev-db dev-db-rm dev prod logs build seed test install-hooks check

help:
	@echo "Usage:"
	@echo "  make dev-db        Start MongoDB in Docker (exposes :27017)"
	@echo "  make dev-db-rm     Stop MongoDB container"
	@echo "  make build         Build Go binaries"
	@echo "  make seed          Run database seeder"
	@echo "  make dev           Run app locally against Dockerised MongoDB"
	@echo "  make prod          Run full production stack"
	@echo "  make logs          Tail all running container logs"
	@echo "  make test          Run Go unit tests"
	@echo "  make check         Run golangci-lint"

dev-db:
	docker-compose --profile dev up -d

dev-db-rm:
	docker-compose --profile dev down -v

build:
	go build -o purpleops .
	go build -o seed ./cmd/seed

seed: build
	MONGO_HOST=localhost ./seed

dev: dev-db build seed
	MONGO_HOST=localhost ./purpleops

prod:
	docker compose --profile prod up --build

logs:
	docker compose logs -f

test:
	go test -v ./...

check: 
	golangci-lint run