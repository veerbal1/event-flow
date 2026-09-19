.PHONY: run test migrate-up migrate-down lint

DATABASE_URL ?= postgres://eventflow:eventflow@localhost:5432/eventflow?sslmode=disable

run:
	go run ./cmd/api

test:
	go test ./...

migrate-up:
	goose -dir migrations postgres "$(DATABASE_URL)" up

migrate-down:
	goose -dir migrations postgres "$(DATABASE_URL)" down

lint:
	go vet ./...
	staticcheck ./...
