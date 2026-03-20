.PHONY: help db db-stop dev prod logs build seed test e2e-seed e2e install-hooks

help:
	@echo "Usage:"
	@echo "  make db        Start MongoDB in Docker (exposes :27017)"
	@echo "  make db-stop   Stop MongoDB container"
	@echo "  make build     Build Go binaries"
	@echo "  make seed      Run database seeder"
	@echo "  make dev       Run app locally against Dockerised MongoDB"
	@echo "  make prod      Run full production stack"
	@echo "  make logs      Tail all running container logs"
	@echo "  make test      Run Go unit tests"
	@echo "  make e2e-seed  Seed E2E test database"
	@echo "  make e2e           Run Playwright E2E tests (requires: make db)"
	@echo "  make install-hooks Install pre-commit hooks (requires: pip install pre-commit)"

install-hooks:
	pre-commit install

db:
	docker-compose up mongodb -d

db-stop:
	docker-compose stop mongodb

db-rm:
	docker-compose down -v

build:
	go build -o purpleops .
	go build -o seed ./cmd/seed

seed: build
	MONGO_HOST=localhost ./seed

dev: db build seed
	MONGO_HOST=localhost ./purpleops

prod:
	docker compose --profile prod up --build

logs:
	docker compose logs -f

test:
	go test -v ./...

e2e-seed:
	go run ./cmd/e2e-seed

e2e: db e2e-seed
	cd e2e && npx playwright test
