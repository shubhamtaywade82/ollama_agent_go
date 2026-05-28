# Ollama Agent (Go)

High-performance, reactive terminal coding agent for Ollama (and, in time, cloud
providers). Go re-implementation of the Ruby `ollama_agent` gem. See
[PLAN.md](PLAN.md) for the full roadmap to feature parity.

> Status: **early development.** A reactive Bubble Tea TUI streams chat from a
> local Ollama server. The agent loop, tool calling, and Kernel runtime are
> under active construction (see the task list / PLAN.md phases).

## Requirements

- Go **1.24+** (`/usr/local/go` on this machine; the system `go` may be older).
- A running [Ollama](https://ollama.com) server (default `http://localhost:11434`).

## Build & run

```sh
make build      # -> bin/ollama_agent
make run        # build + launch the TUI
make test       # go test ./...
make ci         # tidy + vet + race tests + build
```

Or directly:

```sh
/usr/local/go/bin/go run ./cmd/ollama_agent
```

## Configuration

Environment variables:

| Variable             | Default                  | Meaning                  |
| -------------------- | ------------------------ | ------------------------ |
| `OLLAMA_AGENT_MODEL` | `qwen3.5:4b`   | Model name               |
| `OLLAMA_BASE_URL`    | `http://localhost:11434` | Ollama server base URL   |
| `OLLAMA_AGENT_ROOT`  | current working dir      | Sandbox root for tools   |

## Layout

```
cmd/ollama_agent   main entrypoint
pkg/config         configuration loading
pkg/ollama         Ollama HTTP/streaming client
pkg/tui            Bubble Tea terminal UI
pkg/agent          agent think-act loop + tool dispatch (WIP)
pkg/tools          tool registry + core tools (WIP)
```

## Development

CI (`.github/workflows/ci.yml`) runs vet, race tests, build, and golangci-lint
on every push/PR. Run `make lint` locally (install
[golangci-lint](https://golangci-lint.run) first).
