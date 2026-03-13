.PHONY: build install test lint

build:
	go build -o bin/claudemux ./cmd/claudemux

install:
	go install ./cmd/claudemux

test:
	go test ./...

lint:
	go vet ./...
