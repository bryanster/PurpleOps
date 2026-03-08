.PHONY: help db db-stop dev prod logs

help:
	@echo "Usage:"
	@echo "  make db       Start MongoDB in Docker (exposes :27017)"
	@echo "  make db-stop  Stop MongoDB container"
	@echo "  make dev      Run Flask locally against Dockerised MongoDB"
	@echo "  make prod     Run full production stack (gunicorn + MongoDB)"
	@echo "  make logs     Tail all running container logs"

db:
	docker-compose up mongodb

db-stop:
	docker-compose stop mongodb

# Override MONGO_HOST so the local process hits localhost instead of the
# Docker service name defined in .env
dev: db
	MONGO_HOST=localhost python purpleops.py

prod:
	docker compose --profile prod up --build

logs:
	docker compose logs -f
