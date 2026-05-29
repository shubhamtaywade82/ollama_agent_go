# Phase 02 — Memory Tiers

Tags: `#memory` `#shortterm` `#longterm` `#episodic` `#profile` `#sqlite` `#vectordb` `#p0` `#status/next`

Prerequisites: Phase 01 complete.

---

## Goal

Replace the bare `session.Conversation []types.Message` slice with a proper `memory.Manager`
interface that supports four tiers: short-term (rolling context), long-term (vector retrieval),
episodic (structured history), and profile (user/org preferences).

---

## Files to create (in order)

### Step 1 — Manager interface
**`internal/memory/manager.go`**

```go
package memory

type Manager interface {
    Short()    shortterm.Store
    Long()     longterm.Store
    Episodic() episodic.Store
    Profile()  profile.Store
}

type Config struct {
    TokenBudget int    // short-term sliding-window size
    RootDir     string // used by episodic/profile SQLite path
}
```

Tags: `#memory/interface`

---

### Step 2 — Short-term store
**`internal/memory/shortterm/store.go`**

Purpose: rolling conversation window with configurable token budget; replaces the
`session.Conversation` slice directly in `runtime.Engine`.

```go
package shortterm

type Store interface {
    Append(msg types.Message)
    Messages() []types.Message     // returns current window
    Reset()
    TokenCount() int
}

type RollingStore struct {
    budget  int
    msgs    []types.Message
}

func New(tokenBudget int) *RollingStore
func (s *RollingStore) Append(msg types.Message)   // calls tokens.Trim after append
func (s *RollingStore) Messages() []types.Message
func (s *RollingStore) Reset()
func (s *RollingStore) TokenCount() int
```

Tests: `internal/memory/shortterm/store_test.go`
- `TestRollingAppendTrimsBudget` — fill past budget, assert oldest non-system dropped
- `TestRollingResetClearsAll`
- `TestRollingPreservesSystemMessage`

Tags: `#memory/shortterm` `#tokens`

---

### Step 3 — Long-term store (stub)
**`internal/memory/longterm/store.go`**

Purpose: interface for vector-similarity retrieval. Phase 08 fills the real implementation;
this phase ships a no-op stub so the Manager compiles.

```go
package longterm

type Store interface {
    Insert(ctx context.Context, id string, text string, meta map[string]any) error
    Search(ctx context.Context, query string, topK int) ([]Result, error)
}

type Result struct {
    ID    string
    Text  string
    Score float32
    Meta  map[string]any
}

// NoopStore satisfies the interface with empty returns.
type NoopStore struct{}
```

Tags: `#memory/longterm` `#stub` `#rag`

---

### Step 4 — Episodic store
**`internal/memory/episodic/store.go`**

Purpose: persist named episodes (completed tasks, key decisions) to SQLite for later recall.

```go
package episodic

type Episode struct {
    ID        string
    SessionID string
    Summary   string
    Tags      []string
    CreatedAt time.Time
}

type Store interface {
    Save(ctx context.Context, ep Episode) error
    List(ctx context.Context, limit int) ([]Episode, error)
    Search(ctx context.Context, tag string) ([]Episode, error)
}
```

SQLite table to add to `internal/storage/sqlite/db.go` schema:

```sql
CREATE TABLE IF NOT EXISTS episodes (
    id         TEXT PRIMARY KEY,
    session_id TEXT NOT NULL,
    summary    TEXT NOT NULL,
    tags       TEXT NOT NULL DEFAULT '[]',  -- JSON array
    created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
);
```

Tags: `#memory/episodic` `#sqlite`

---

### Step 5 — Profile store
**`internal/memory/profile/store.go`**

Purpose: store user and org-level preferences that persist across sessions.

```go
package profile

type Profile struct {
    Key       string
    Value     string
    UpdatedAt time.Time
}

type Store interface {
    Set(ctx context.Context, key, value string) error
    Get(ctx context.Context, key string) (string, bool, error)
    All(ctx context.Context) ([]Profile, error)
}
```

SQLite table:

```sql
CREATE TABLE IF NOT EXISTS profile (
    key        TEXT PRIMARY KEY,
    value      TEXT NOT NULL,
    updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
);
```

Tags: `#memory/profile` `#sqlite`

---

### Step 6 — Default Manager implementation
**`internal/memory/default.go`**

```go
type DefaultManager struct {
    short    *shortterm.RollingStore
    long     longterm.Store
    episodic episodic.Store
    profile  profile.Store
}

func New(cfg Config, db *sql.DB) *DefaultManager
```

Tags: `#memory/manager`

---

### Step 7 — Wire into Engine
**`internal/runtime/engine.go`** — replace `session.Conversation []types.Message` usage:

- Add `Memory memory.Manager` field to `Engine`
- `Submit()`: call `e.Memory.Short().Append(userMsg)` instead of direct slice append
- Pass `e.Memory.Short().Messages()` to `agent.Run()`
- After run: `e.Memory.Short().Append(assistantMsg)`
- `ClearHistory()`: call `e.Memory.Short().Reset()` in addition to new session creation

**`internal/app/bootstrap.go`** — construct `memory.DefaultManager` and pass to `NewEngine`.

Tags: `#runtime/engine` `#memory/wire`

---

## Verification

```
go test ./internal/memory/...
go test ./internal/runtime/...   # existing engine tests still pass
go build ./...
```

After wiring:
- Multi-turn conversation maintains correct token budget (no unbounded growth)
- `ClearHistory` resets the short-term store
- Long-term and episodic stores compile with no-op implementations
