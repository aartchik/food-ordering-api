-include .env

POSTGRES_DB ?= food_ordering_api
POSTGRES_USER ?= foodapp
POSTGRES_PASSWORD ?= foodapp
POSTGRES_DSN ?= postgres://$(POSTGRES_USER):$(POSTGRES_PASSWORD)@localhost:5432/$(POSTGRES_DB)?sslmode=disable

.PHONY: env-up env-down db-create migrate-up migrate-down run fmt vet test lint

env-up:
	docker-compose up --build

env-down:
	docker-compose down

db-create:
	./scripts/create_local_database.sh

migrate-up:
	migrate -path migrations -database "$(POSTGRES_DSN)" up

migrate-down:
	migrate -path migrations -database "$(POSTGRES_DSN)" down

run:
	go run ./cmd/api -db-dsn="$(POSTGRES_DSN)"

fmt:
	gofmt -w $$(find . -name '*.go' -not -path './.cache/*')

vet:
	go vet ./...

test:
	go test ./...

lint:
	golangci-lint run ./...
