# Use the modern Go toolchain explicitly; the system PATH may resolve an old go.
GO ?= $(shell command -v /usr/local/go/bin/go 2>/dev/null || command -v go)
BIN := bin/ollama_agent
PKG := ./...
VERSION ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo dev)
LDFLAGS := -s -w -X main.version=$(VERSION)

.PHONY: all build run test test-race cover vet fmt lint tidy clean install ci docker-build docker-up index

all: fmt vet test build

build:
	$(GO) build -trimpath -ldflags '$(LDFLAGS)' -o $(BIN) ./cmd/ollama_agent

run: build
	./$(BIN)

test:
	$(GO) test $(PKG)

test-race:
	$(GO) test -race $(PKG)

cover:
	$(GO) test -coverprofile=coverage.out $(PKG)
	$(GO) tool cover -func=coverage.out | tail -1

vet:
	$(GO) vet $(PKG)

fmt:
	$(GO) fmt $(PKG)

lint:
	@command -v golangci-lint >/dev/null 2>&1 && golangci-lint run || echo "golangci-lint not installed; skipping"

tidy:
	$(GO) mod tidy

clean:
	rm -f $(BIN) coverage.out

install:
	$(GO) install -trimpath -ldflags '$(LDFLAGS)' ./cmd/ollama_agent

ci: tidy vet test-race build

docker-build:
	docker build -t ollama-agent:dev .

docker-up:
	docker compose up -d

index:
	$(GO) run ./cmd/ollama_agent --index ./docs
