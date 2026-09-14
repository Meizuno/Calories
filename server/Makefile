.PHONY: generate build run dev seed tidy tools

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

# Seed a dev user with sample foods + today's meals (local dev only; not shipped).
seed:
	go run ./cmd/seed

tidy:
	go mod tidy

# Install the dev tools: codegen (sqlc) and live reload (air).
tools:
	go install github.com/sqlc-dev/sqlc/cmd/sqlc@latest
	go install github.com/air-verse/air@latest
