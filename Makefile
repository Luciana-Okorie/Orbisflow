include .env
export

.PHONY: up down migrate run test

up:
	docker compose up -d

down:
	docker compose down

migrate:
	for f in migrations/*.sql; do psql "$(DATABASE_URL)" -f $$f; done

run:
	go run ./services/transaction

test:
	go test ./...
