.PHONY: build test vet fmt

build:
	go build -o bin/opensre ./cmd/opensre

test:
	go test -race ./...

vet:
	go vet ./...

fmt:
	gofmt -w .
