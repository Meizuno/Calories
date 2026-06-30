.PHONY: generate build run seed tidy tools

# Regenerate the sqlc db layer from queries + migrations.
generate:
	sqlc generate

build:
	go build -o bin/calories ./cmd/server

run:
	go run ./cmd/server

# Seed a dev user with sample foods + today's meals (local dev only; not shipped).
seed:
	go run ./cmd/seed

tidy:
	go mod tidy

# Install the codegen tool (sqlc).
tools:
	go install github.com/sqlc-dev/sqlc/cmd/sqlc@latest
