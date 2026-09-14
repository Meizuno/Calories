.PHONY: generate build run dev seed test test-db tidy tools

# Regenerate the sqlc db layer from queries + migrations.
generate:
	sqlc generate

build:
	go build -o bin/calories ./cmd/server

run:
	go run ./cmd/server

# Live reload: rebuild + restart on every .go/.sql/.env save (see .air.toml).
# Install once with `make tools`.
dev:
	air

# Unit tests only — no database needed, safe anywhere.
test:
	go test ./...

# Everything, including the database-backed auth tests (rotation, replay
# detection, account linking). Uses a THROWAWAY database: it truncates.
#   make test-db TEST_DATABASE_URL=postgres://user:pass@localhost:5432/calories_test?sslmode=disable
TEST_DATABASE_URL ?= postgres://calories_user:password@localhost:5432/calories_test?sslmode=disable
test-db:
	TEST_DATABASE_URL="$(TEST_DATABASE_URL)" go test ./...

# Seed a dev user with sample foods + today's meals (local dev only; not shipped).
seed:
	go run ./cmd/seed

tidy:
	go mod tidy

# Install the dev tools: codegen (sqlc) and live reload (air).
tools:
	go install github.com/sqlc-dev/sqlc/cmd/sqlc@latest
	go install github.com/air-verse/air@latest
