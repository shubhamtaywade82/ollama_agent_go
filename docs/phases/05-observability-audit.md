# Phase 05 — Observability + Audit

Tags: `#observability` `#tracing` `#spans` `#metrics` `#audit` `#sqlite` `#p1` `#status/done`

Prerequisites: Phase 01 (Logger interface, storage), Phase 03 (saga for mutation audit).

---

## Goal

Add per-run span timelines (LLM call, tool call, approval events) stored in SQLite.
Add counters/gauges for latency, token cost, error rates exportable as JSON.
Implement audit log that satisfies compliance requirements (who ran what, when, result).

---

## Files to create (in order)

### Step 1 — Span types
**`internal/observability/traces.go`**

```go
package observability

type SpanKind string

const (
    SpanLLMCall    SpanKind = "llm_call"
    SpanToolCall   SpanKind = "tool_call"
    SpanApproval   SpanKind = "approval"
    SpanAgentTurn  SpanKind = "agent_turn"
)

type Span struct {
    ID         string
    ParentID   string    // empty for root span
    SessionID  string
    Kind       SpanKind
    Model      string
    Tool       string
    Step       int
    DurationMs int64
    Tokens     int
    Error      string
    StartedAt  time.Time
    EndedAt    time.Time
}

type Tracer interface {
    Start(ctx context.Context, kind SpanKind, sessionID string) (context.Context, *Span)
    Finish(ctx context.Context, span *Span, err error)
    Export(ctx context.Context, sessionID string) ([]Span, error)
}
```

Tags: `#observability/spans`

---

### Step 2 — SQLite spans table
**`internal/storage/sqlite/db.go`** — add to schema:

```sql
CREATE TABLE IF NOT EXISTS spans (
    id          TEXT PRIMARY KEY,
    parent_id   TEXT,
    session_id  TEXT NOT NULL,
    kind        TEXT NOT NULL,
    model       TEXT,
    tool        TEXT,
    step        INTEGER,
    duration_ms INTEGER,
    tokens      INTEGER,
    error       TEXT,
    started_at  DATETIME NOT NULL,
    ended_at    DATETIME NOT NULL
);
CREATE INDEX IF NOT EXISTS idx_spans_session ON spans(session_id);
```

Tags: `#storage/sqlite/spans`

---

### Step 3 — SQLite tracer
**`internal/storage/sqlite/tracer.go`**

```go
type SQLiteTracer struct{ db *sql.DB }

func (t *SQLiteTracer) Start(ctx context.Context, kind SpanKind, sessionID string) (context.Context, *observability.Span)
func (t *SQLiteTracer) Finish(ctx context.Context, span *observability.Span, err error)
func (t *SQLiteTracer) Export(ctx context.Context, sessionID string) ([]observability.Span, error)
```

Spans stored synchronously (no goroutine) to keep it simple. If write fails, log and continue —
span loss is acceptable; correctness is not affected.

Tags: `#storage/sqlite/tracer`

---

### Step 4 — Metrics counters
**`internal/observability/metrics.go`**

```go
type Metrics interface {
    IncrToolCall(tool string, success bool)
    IncrLLMCall(model string, tokens int, success bool)
    IncrApproval(tool string, approved bool)
    Snapshot() MetricsSnapshot
}

type MetricsSnapshot struct {
    ToolCalls   map[string]ToolCallStats
    LLMCalls    map[string]LLMCallStats
    TotalTokens int
    Errors      int
    UpdatedAt   time.Time
}

// InMemoryMetrics: thread-safe counters, no external dependency.
type InMemoryMetrics struct { mu sync.RWMutex; ... }
```

Tags: `#observability/metrics`

---

### Step 5 — Audit log
**`internal/observability/audit.go`**

```go
type AuditEvent struct {
    ID        string
    SessionID string
    Actor     string       // "agent" | "user" | "system"
    Action    string       // "tool_call", "approval_granted", "approval_denied", "session_start", "session_clear"
    Resource  string       // tool name, file path, etc.
    Result    string       // "ok" | "denied" | "error"
    Detail    string       // free-form JSON
    At        time.Time
}

type AuditLogger interface {
    Log(ctx context.Context, ev AuditEvent) error
    Query(ctx context.Context, sessionID string) ([]AuditEvent, error)
}
```

SQLite table:

```sql
CREATE TABLE IF NOT EXISTS audit_log (
    id         TEXT PRIMARY KEY,
    session_id TEXT NOT NULL,
    actor      TEXT NOT NULL,
    action     TEXT NOT NULL,
    resource   TEXT,
    result     TEXT NOT NULL,
    detail     TEXT,
    at         DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
);
CREATE INDEX IF NOT EXISTS idx_audit_session ON audit_log(session_id);
```

Tags: `#observability/audit` `#compliance`

---

### Step 6 — Update Logger interface
**`internal/observability/logging.go`** — extend to include tracer + audit:

```go
type Logger interface {
    Info(msg string, fields ...any)
    Debug(msg string, fields ...any)
    Error(msg string, err error, fields ...any)
    RecordToolCall(tool string, duration time.Duration, err error)
    RecordTokens(session, model string, prompt, output int)

    // New in Phase 05
    Tracer() Tracer
    Audit() AuditLogger
    Metrics() Metrics
}
```

`FileLogger` wraps `SQLiteTracer` + `InMemoryMetrics` + `SQLiteAuditLogger`.
`Discard` returns no-op implementations.

Tags: `#observability/logger`

---

### Step 7 — Instrument Engine
**`internal/runtime/engine.go`** — add spans around key operations:

```go
func (e *Engine) Submit(ctx context.Context, input string, emit agent.Emit) error {
    ctx, span := e.Obs.Tracer().Start(ctx, observability.SpanAgentTurn, e.session.ID)
    defer func() { e.Obs.Tracer().Finish(ctx, span, err) }()
    // ... existing logic ...
}
```

**`internal/tools/host.go`** — wrap `Execute()` with `SpanToolCall` span:

```go
ctx, span := h.Obs.Tracer().Start(ctx, observability.SpanToolCall, h.SessionID)
defer func() { h.Obs.Tracer().Finish(ctx, span, err); h.Obs.Metrics().IncrToolCall(name, err==nil) }()
```

Tags: `#runtime/engine` `#tools/host` `#observability/wire`

---

### Step 8 — TUI /audit slash command
**`internal/ui/tui/model.go`** — add handler for `/audit`:

Display last N audit events for current session in a scrollable pane.

Tags: `#ui/tui` `#observability/ui`

---

### Step 9 — JSON export endpoint
**`internal/runtime/engine.go`** — add:

```go
func (e *Engine) ExportTrace(ctx context.Context) ([]observability.Span, error)
func (e *Engine) ExportAudit(ctx context.Context) ([]observability.AuditEvent, error)
func (e *Engine) MetricsSnapshot() observability.MetricsSnapshot
```

Tags: `#runtime/engine` `#observability/export`

---

## Tests

**`internal/observability/metrics_test.go`**
- `TestIncrAndSnapshot` — counter increments reflected in snapshot

**`internal/storage/sqlite/tracer_test.go`**
- `TestStartFinishSpan` — span persisted with correct duration
- `TestExportBySession` — only spans for given session returned

**`internal/observability/audit_test.go`**
- `TestAuditLogAndQuery`

Tags: `#tests`

---

## Verification

```
go test ./internal/observability/... ./internal/storage/...
```

- After a tool call: `spans` table has tool_call span with non-zero duration_ms
- After an approval: `audit_log` has approval_granted or approval_denied entry
- `/audit` in TUI renders last 10 events for session
- `ExportTrace(ctx)` returns JSON-serialisable slice with agent_turn > tool_call parent-child relationship
