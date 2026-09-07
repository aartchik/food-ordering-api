-include .env

POSTGRES_DB ?= food_ordering_api
POSTGRES_USER ?= foodapp
POSTGRES_PASSWORD ?= foodapp
POSTGRES_DSN ?= postgres://$(POSTGRES_USER):$(POSTGRES_PASSWORD)@localhost:5432/$(POSTGRES_DB)?sslmode=disable

.PHONY: env-up env-down db-create migrate-up migrate-down run fmt fmt-check vet test test-integration lint

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

fmt-check:
	@test -z "$$(git ls-files '*.go' | xargs gofmt -l)" || \
		{ git ls-files '*.go' | xargs gofmt -l; exit 1; }

test:
	go test ./...

test-integration:
	go test -tags=integration -race -count=1 -timeout=2m ./internal/models

lint:
	golangci-lint run ./...
