-include .env

POSTGRES_DSN ?= postgres://$(POSTGRES_USER):$(POSTGRES_PASSWORD)@localhost:5432/$(POSTGRES_DB)?sslmode=disable

.PHONY: env-up env-down migrate-up migrate-down fmt vet test lint

env-up:
	docker-compose up -d postgres migrate

env-down:
	docker-compose down

migrate-up:
	migrate -path migrations -database "$(POSTGRES_DSN)" up

migrate-down:
	migrate -path migrations -database "$(POSTGRES_DSN)" down

fmt:
	gofmt -w $$(find . -name '*.go' -not -path './.cache/*')

vet:
	go vet ./...

test:
	go test ./...

lint:
	golangci-lint run ./...
