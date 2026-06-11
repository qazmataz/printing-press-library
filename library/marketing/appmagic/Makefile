.PHONY: build test lint install clean

build:
	go build -o bin/appmagic-pp-cli ./cmd/appmagic-pp-cli

test:
	go test ./...

lint:
	golangci-lint run

install:
	go install ./cmd/appmagic-pp-cli

clean:
	rm -rf bin/

build-mcp:
	go build -o bin/appmagic-pp-mcp ./cmd/appmagic-pp-mcp

install-mcp:
	go install ./cmd/appmagic-pp-mcp

build-all: build build-mcp
