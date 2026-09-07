.PHONY: fmt vet test lint

fmt:
	gofmt -w $$(find . -name '*.go' -not -path './.cache/*')

vet:
	go vet ./...

test:
	go test ./...

lint:
	golangci-lint run ./...
