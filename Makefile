
.PHONY: help db-up db-down db-status db-logs db-shell \
        migrate-create migrate-up migrate-down \
        migrate-status migrate-version migrate-validate \
        api-run test vet

help:
	@echo "ScopeGuard Development Commands"
	@echo ""
	@echo "Database:"
	@echo "  make db-up"
	@echo "  make db-down"
	@echo "  make db-status"
	@echo "  make db-logs"
	@echo "  make db-shell"
	@echo ""
	@echo "Migrations:"
	@echo "  make migrate-create name=create_users"
	@echo "  make migrate-up"
	@echo "  make migrate-down"
	@echo "  make migrate-status"
	@echo "  make migrate-version"
	@echo "  make migrate-validate"
	@echo ""
	@echo "Application:"
	@echo "  make api-run"
	@echo "  make test"
	@echo "  make vet"

db-up:
	docker compose up -d --wait postgres

db-down:
	docker compose down

db-status:
	docker compose ps

db-logs:
	docker compose logs -f postgres

db-shell:
	docker compose exec postgres \
		psql -U scopeguard -d scopeguard

migrate-create:
	@test -n "$(name)" || \
		(echo "Usage: make migrate-create name=create_users"; exit 1)
	cd apps/api && goose create "$(name)" sql

migrate-up:
	cd apps/api && goose up

migrate-down:
	cd apps/api && goose down

migrate-status:
	cd apps/api && goose status

migrate-version:
	cd apps/api && goose version

migrate-validate:
	cd apps/api && goose validate

api-run:
	cd apps/api && go run ./cmd/api

test:
	cd apps/api && go test ./... -v

vet:
	cd apps/api && go vet ./...
