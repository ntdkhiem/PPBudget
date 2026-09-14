.PHONY: run build test test-integration migrate-up migrate-down

# Default TEST_DATABASE_URL for test-integration: the throwaway docker
# container on port 55432 -- never point this at the port-5432 dev/prod
# database. Override on the command line: make test-integration TEST_DATABASE_URL=...
TEST_DATABASE_URL ?= postgres://ppbudget:ppbudget_password@localhost:55432/ppbudget?sslmode=disable

run:
	go run cmd/api/main.go

build:
	go build -o bin/api cmd/api/main.go

test:
	go test ./...

# Integration tests hit a real Postgres instance, gated on TEST_DATABASE_URL.
test-integration:
	TEST_DATABASE_URL="$(TEST_DATABASE_URL)" go test -count=1 -run Integration ./internal/...

migrate-up:
	migrate -path internal/db/migrations -database "postgres://ppbudget:ppbudget_password@localhost:5432/ppbudget?sslmode=disable" up

migrate-down:
	migrate -path internal/db/migrations -database "postgres://ppbudget:ppbudget_password@localhost:5432/ppbudget?sslmode=disable" down 1
