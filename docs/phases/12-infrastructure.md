# Phase 12 — Infrastructure Layer

Tags: `#infrastructure` `#cache` `#queue` `#secrets-manager` `#docker` `#cicd` `#k8s` `#p3` `#status/done`

Prerequisites: Phases 01–11 complete and stable.

---

## Goal

Production-ready deployment: containerisation, cache layer (in-memory + Redis), async event
queue (in-proc + Kafka/SQS), secrets management, and CI/CD pipeline.

---

## Files to create (in order)

### Step 1 — Cache interface
**`internal/cache/cache.go`**

```go
package cache

type Cache interface {
    Get(ctx context.Context, key string) ([]byte, bool, error)
    Set(ctx context.Context, key string, value []byte, ttl time.Duration) error
    Delete(ctx context.Context, key string) error
    Flush(ctx context.Context) error
}

// InMemoryCache: sync.Map + TTL expiry goroutine. Zero dependencies.
type InMemoryCache struct { ... }

// RedisCache: wraps github.com/redis/go-redis/v9.
// Activated when REDIS_URL env var is set.
type RedisCache struct { ... }

func New(redisURL string) Cache   // returns Redis if URL set, InMemory otherwise
```

Use cases:
- Cache embedding vectors (key: sha256(text)+model) — avoids re-embedding unchanged docs
- Cache provider responses for identical requests (key: sha256(messages+model)) — optional, off by default
- Cache skill file contents (TTL 60s) — avoids re-reading on every prompt build

Tags: `#infrastructure/cache`

---

### Step 2 — Event queue interface
**`internal/queue/queue.go`**

```go
package queue

type Message struct {
    Topic   string
    Payload []byte
    ID      string
    At      time.Time
}

type Publisher interface {
    Publish(ctx context.Context, msg Message) error
}

type Subscriber interface {
    Subscribe(ctx context.Context, topic string, handler func(Message) error) error
    Close() error
}

// InProcQueue: channel-based, same process. Used in Phase 01 (EventBus).
// Wrap runtime.InProcBus to satisfy this interface.
type InProcQueue struct { ... }

// KafkaQueue: wraps github.com/segmentio/kafka-go.
// Activated when KAFKA_BROKERS env var is set.
type KafkaQueue struct { ... }

// SQSQueue: wraps github.com/aws/aws-sdk-go-v2/service/sqs.
// Activated when SQS_QUEUE_URL env var is set.
type SQSQueue struct { ... }

func New(cfg QueueConfig) (Publisher, Subscriber)
```

Use cases:
- Publish agent events (tool calls, completions) for downstream consumers
- Multi-instance coordination (multiple engine instances share a queue)
- Async task submission (submit job via queue, pick up in worker)

Tags: `#infrastructure/queue`

---

### Step 3 — Secrets manager
**`internal/secrets/manager.go`**

```go
package secrets

type Manager interface {
    Get(ctx context.Context, key string) (string, error)
    Set(ctx context.Context, key, value string) error
    Delete(ctx context.Context, key string) error
}

// EnvManager: reads from environment variables. Always available.
type EnvManager struct{}

// FileManager: reads from encrypted file at $OLLAMA_SECRETS_PATH.
// Encryption: AES-256-GCM with key derived from $OLLAMA_MASTER_KEY via PBKDF2.
type FileManager struct { path string; key []byte }

// VaultManager: wraps HashiCorp Vault HTTP API.
// Activated when VAULT_ADDR env var is set.
type VaultManager struct { addr, token string; client *http.Client }

func New(cfg SecretsConfig) Manager   // returns most secure available
```

Used in `bootstrap.go` to supply API keys to providers instead of reading from env directly.

Tags: `#infrastructure/secrets`

---

### Step 4 — Dockerfile
**`Dockerfile`**

```dockerfile
FROM golang:1.25-alpine AS builder
WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 go build -ldflags="-s -w" -o ollama-agent ./cmd/ollama_agent

FROM alpine:3.21
RUN apk add --no-cache ca-certificates tzdata
WORKDIR /app
COPY --from=builder /app/ollama-agent .
EXPOSE 8080
ENTRYPOINT ["./ollama-agent"]
```

Note: TUI mode requires a terminal; Docker is for `--api-server` mode (Phase 12.5).

Tags: `#infrastructure/docker`

---

### Step 5 — Docker Compose (local dev stack)
**`docker-compose.yml`**

```yaml
services:
  agent:
    build: .
    environment:
      - OLLAMA_BASE_URL=http://ollama:11434
      - REDIS_URL=redis://redis:6379
    depends_on: [ollama, redis]
    volumes:
      - ./data:/app/data

  ollama:
    image: ollama/ollama:latest
    ports: ["11434:11434"]
    volumes:
      - ollama_models:/root/.ollama

  redis:
    image: redis:7-alpine
    ports: ["6379:6379"]

volumes:
  ollama_models:
```

Tags: `#infrastructure/compose`

---

### Step 6 — Makefile targets
**`Makefile`**

```makefile
.PHONY: build test lint run docker-build docker-up index

build:
	go build ./...

test:
	go test ./...

lint:
	golangci-lint run

run:
	go run ./cmd/ollama_agent

docker-build:
	docker build -t ollama-agent:dev .

docker-up:
	docker compose up -d

index:
	go run ./cmd/ollama_agent --index ./docs
```

Tags: `#infrastructure/makefile`

---

### Step 7 — GitHub Actions CI
**`.github/workflows/ci.yml`**

```yaml
name: CI
on: [push, pull_request]
jobs:
  test:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4
      - uses: actions/setup-go@v5
        with: { go-version: "1.25" }
      - run: go mod download
      - run: go build ./...
      - run: go vet ./...
      - run: go test ./... -race -coverprofile=coverage.out
      - uses: codecov/codecov-action@v4

  lint:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4
      - uses: golangci/golangci-lint-action@v6
        with: { version: latest }
```

Tags: `#infrastructure/ci`

---

### Step 8 — API server mode (optional)
**`cmd/ollama_agent/main.go`** — add `--api-server` flag:

```go
// Starts an HTTP API server instead of TUI.
// Endpoints:
//   POST /v1/sessions         → create session
//   POST /v1/sessions/{id}/submit → submit message, SSE stream events
//   DELETE /v1/sessions/{id}  → clear history
//   GET  /v1/sessions/{id}/audit → export audit log
```

**`internal/ui/api/server.go`** — thin HTTP handler calling same `runtime.Engine`.

Tags: `#infrastructure/api-server`

---

### Step 9 — Kubernetes manifests (optional)
**`k8s/deployment.yaml`** — Deployment + Service + ConfigMap skeleton.

Tags: `#infrastructure/k8s`

---

## Tests

**`internal/cache/cache_test.go`**
- `TestInMemoryGetSet`
- `TestInMemoryTTLExpiry`

**`internal/queue/queue_test.go`**
- `TestInProcPublishSubscribe`

**`internal/secrets/manager_test.go`**
- `TestEnvManagerGet`
- `TestFileManagerEncryptDecrypt`

Tags: `#tests`

---

## Verification

```
make docker-build && make docker-up
curl -X POST http://localhost:8080/v1/sessions -d '{"model":"qwen3:4b"}'
curl -X POST http://localhost:8080/v1/sessions/{id}/submit -d '{"input":"hello"}' --no-buffer
```

- Embedding cache: second `/index` call skips re-embedding unchanged files (log: "cache hit")
- Redis cache: restart agent → cached embeddings survive (no re-embed on startup)
- CI pipeline passes on push to main
