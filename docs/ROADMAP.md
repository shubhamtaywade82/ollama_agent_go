# ollama_agent_go — Implementation Roadmap

Tags used throughout: `#layer/*` for architectural layer, `#p0–#p3` for priority,
`#status/*` for current state.

---

## Status legend

| Tag | Meaning |
|---|---|
| `#status/done` | Committed and tested |
| `#status/next` | Ready to start — all prerequisites met |
| `#status/planned` | Designed, not started |
| `#status/stub` | Interface defined, no implementation |

---

## Phase overview (ordered by dependency)

| # | Phase | Priority | Status | Doc |
|---|---|---|---|---|
| 01 | Runtime Kernel | #p0 | #status/done | — |
| 02 | Memory Tiers | #p0 | #status/next | [phases/02-memory-tiers.md](phases/02-memory-tiers.md) |
| 03 | WAL + Saga | #p0 | #status/next | [phases/03-wal-saga.md](phases/03-wal-saga.md) |
| 04 | Provider Router Enhancements | #p1 | #status/planned | [phases/04-provider-router.md](phases/04-provider-router.md) |
| 05 | Observability + Audit | #p1 | #status/planned | [phases/05-observability-audit.md](phases/05-observability-audit.md) |
| 06 | Reliability | #p1 | #status/planned | [phases/06-reliability.md](phases/06-reliability.md) |
| 07 | MCP Protocol | #p2 | #status/planned | [phases/07-mcp.md](phases/07-mcp.md) |
| 08 | RAG / Knowledge | #p2 | #status/planned | [phases/08-rag-knowledge.md](phases/08-rag-knowledge.md) |
| 09 | Agent Roles | #p2 | #status/planned | [phases/09-agent-roles.md](phases/09-agent-roles.md) |
| 10 | Orchestration / Workflow | #p2 | #status/planned | [phases/10-orchestration.md](phases/10-orchestration.md) |
| 11 | Governance & Security | #p2 | #status/planned | [phases/11-governance-security.md](phases/11-governance-security.md) |
| 12 | Infrastructure | #p3 | #status/planned | [phases/12-infrastructure.md](phases/12-infrastructure.md) |
| 13 | Trading Agent | #p3 | #status/planned | [phases/13-trading-agent.md](phases/13-trading-agent.md) |

---

## Dependency graph

```
Phase 01 (done)
  └── Phase 02 (memory)         ← needs engine.session
  └── Phase 03 (saga/WAL)       ← needs storage layer
        └── Phase 04 (router)   ← needs provider interface
        └── Phase 05 (observability) ← needs storage + logger
              └── Phase 06 (reliability) ← needs observability
              └── Phase 07 (MCP)         ← needs tools.Host
              └── Phase 08 (RAG)         ← needs memory.longterm + indexer
                    └── Phase 09 (agent roles)  ← needs RAG + providers
                          └── Phase 10 (orchestration) ← needs agent roles
                                └── Phase 11 (governance) ← needs all layers
                                      └── Phase 12 (infrastructure)
                                            └── Phase 13 (trading agent)
```

---

## Architecture layers → phase mapping

| Reference Arch Layer | Covered by Phase(s) |
|---|---|
| 01 User/Client Layer | Phase 01 (done) |
| 02 Orchestration/Control Plane | Phase 10 |
| 03 Agent Layer (Specialized) | Phase 09 |
| 04 Tools & Integrations | Phase 01 (done), Phase 07 (MCP) |
| 05 Memory & Knowledge | Phase 02, Phase 08 |
| 06 Monitoring & Observability | Phase 05 |
| 07 Reliability & Failure Mgmt | Phase 03 (saga), Phase 06 |
| 08 Governance & Security | Phase 11 |
| 09 Foundation / Infrastructure | Phase 12 |

---

## What Phase 01 already provides

```
internal/
  agent/          think-act loop, events, synthetic tool parsing
  app/            bootstrap wiring
  config/         env-based config with DBPath
  observability/  Logger interface, FileLogger, Discard
  policy/         Allow/Deny/RequireApproval, approval callback
  providers/      Ollama, Anthropic, OpenAI adapters + Router
  runtime/        Engine, Session, EventBus (InProcBus), Command types
  skills/         Markdown skill loading + system prompt injection
  storage/sqlite/ WAL SQLite: sessions, messages, tool_invocations, token_ledger
  tokens/         Heuristic token estimation + sliding-window trim
  tools/          Registry, Host, fs/shell/edit/search/sandbox
  types/          Neutral Message/ToolCall/ChatRequest/ChatResponse
  ui/tui/         Thin BubbleTea TUI calling engine.Submit() only
```
