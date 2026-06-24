.PHONY: run build test migrate-up migrate-down

run:
	go run cmd/api/main.go

build:
	go build -o bin/api cmd/api/main.go

migrate-up:
	migrate -path internal/db/migrations -database "postgres://ppbudget:ppbudget_password@localhost:5432/ppbudget?sslmode=disable" up

migrate-down:
	migrate -path internal/db/migrations -database "postgres://ppbudget:ppbudget_password@localhost:5432/ppbudget?sslmode=disable" down 1
