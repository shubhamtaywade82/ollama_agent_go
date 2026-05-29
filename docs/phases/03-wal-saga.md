# Phase 03 — WAL + Saga Engine

Tags: `#saga` `#wal` `#rollback` `#mutations` `#sqlite` `#reliability` `#p0` `#status/next`

Prerequisites: Phase 01 (storage layer).

---

## Goal

Any tool that mutates the filesystem (`write_file`, `edit_file`, `run_shell` with side-effects)
must be wrapped in a saga: capture before-state, execute, verify, commit or rollback.
This prevents partial writes from leaving the workspace corrupt if the agent loop crashes
mid-execution.

---

## State machine

```
Reserved → Locked → Applied → Verified → Committed
                                       ↘ RolledBack  (on verify failure or agent cancel)
```

---

## Files to create (in order)

### Step 1 — SQLite mutations table
**`internal/storage/sqlite/db.go`** — add to embedded schema:

```sql
CREATE TABLE IF NOT EXISTS mutations (
    id           TEXT PRIMARY KEY,
    session_id   TEXT NOT NULL,
    tool         TEXT NOT NULL,
    args         TEXT NOT NULL,    -- JSON
    before_state TEXT,             -- JSON snapshot (file content or null)
    after_state  TEXT,             -- JSON snapshot after apply
    status       TEXT NOT NULL DEFAULT 'reserved',
    created_at   DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at   DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
);
```

Tags: `#storage/sqlite` `#mutations`

---

### Step 2 — Saga types
**`internal/runtime/saga.go`**

```go
package runtime

type SagaStatus string

const (
    SagaReserved   SagaStatus = "reserved"
    SagaLocked     SagaStatus = "locked"
    SagaApplied    SagaStatus = "applied"
    SagaVerified   SagaStatus = "verified"
    SagaCommitted  SagaStatus = "committed"
    SagaRolledBack SagaStatus = "rolled_back"
)

type Mutation struct {
    ID          string
    SessionID   string
    Tool        string
    Args        map[string]any
    BeforeState []byte     // captured before execution
    AfterState  []byte     // captured after execution
    Status      SagaStatus
    CreatedAt   time.Time
    UpdatedAt   time.Time
}

type SagaStore interface {
    Reserve(ctx context.Context, m Mutation) error
    Transition(ctx context.Context, id string, to SagaStatus, after []byte) error
    Pending(ctx context.Context, sessionID string) ([]Mutation, error)
    RollbackAll(ctx context.Context, sessionID string) error
}
```

Tags: `#saga/types`

---

### Step 3 — SQLite saga store
**`internal/storage/sqlite/saga.go`**

Implements `SagaStore` using the `mutations` table.

```go
func (s *Store) Reserve(ctx context.Context, m runtime.Mutation) error
func (s *Store) Transition(ctx context.Context, id string, to runtime.SagaStatus, after []byte) error
func (s *Store) Pending(ctx context.Context, sessionID string) ([]runtime.Mutation, error)
func (s *Store) RollbackAll(ctx context.Context, sessionID string) error
```

Tags: `#storage/sqlite` `#saga`

---

### Step 4 — Compensation functions
**`internal/tools/compensation.go`**

Each mutating tool registers a compensation (undo) function:

```go
package tools

type CompensationFunc func(ctx context.Context, beforeState []byte) error

// Registry of compensations keyed by tool name.
var compensations = map[string]CompensationFunc{
    "write_file": compensateWriteFile,
    "edit_file":  compensateEditFile,
    "run_shell":  nil, // non-compensable; logs warning and marks manual review
}

// compensateWriteFile restores original content (or deletes if file was new).
func compensateWriteFile(ctx context.Context, before []byte) error

// compensateEditFile restores original file content.
func compensateEditFile(ctx context.Context, before []byte) error
```

Tags: `#tools/compensation` `#saga/rollback`

---

### Step 5 — CaptureState helper
**`internal/tools/snapshot.go`**

```go
// CaptureFileBefore reads path content before a mutating tool call.
// Returns nil if path does not exist (new file).
func CaptureFileBefore(root, path string) ([]byte, error)
```

Tags: `#tools/snapshot`

---

### Step 6 — Wrap Host.Execute with saga
**`internal/tools/host.go`** — update `Execute()`:

```go
func (h *Host) Execute(ctx context.Context, name string, args map[string]any) (string, error) {
    isMutating := h.isMutating(name)

    var mut runtime.Mutation
    if isMutating && h.Saga != nil {
        before, _ := CaptureFileBefore(h.Root, pathFromArgs(args))
        mut = runtime.Mutation{
            ID: uuid.New().String(), SessionID: h.SessionID,
            Tool: name, Args: args, BeforeState: before,
        }
        h.Saga.Reserve(ctx, mut)
        h.Saga.Transition(ctx, mut.ID, runtime.SagaLocked, nil)
    }

    // policy check
    // registry execute
    result, err := h.Registry.Execute(ctx, name, args)

    if isMutating && h.Saga != nil {
        after, _ := CaptureFileBefore(h.Root, pathFromArgs(args))
        if err != nil {
            h.Saga.Transition(ctx, mut.ID, runtime.SagaRolledBack, nil)
        } else {
            h.Saga.Transition(ctx, mut.ID, runtime.SagaApplied, after)
            h.Saga.Transition(ctx, mut.ID, runtime.SagaCommitted, nil)
        }
    }
    return result, err
}
```

Add `Saga SagaStore` and `Root string` fields to `Host`.

Tags: `#tools/host` `#saga/wire`

---

### Step 7 — Engine rollback on cancel
**`internal/runtime/engine.go`** — in `Submit()`, if context is cancelled mid-run:

```go
defer func() {
    if ctx.Err() != nil && e.Sessions != nil {
        e.Sessions.(storage.SagaStore).RollbackAll(ctx, e.session.ID)
    }
}()
```

Tags: `#runtime/engine` `#saga/cancel`

---

## Tests

**`internal/runtime/saga_test.go`**
- `TestSagaReserveAndCommit` — happy path through all states
- `TestSagaRollbackRestoresFile` — write_file compensation recreates original
- `TestSagaRollbackOnCancel` — cancel context triggers RollbackAll

**`internal/tools/compensation_test.go`**
- `TestCompensateWriteFileNewFile` — new file deleted on rollback
- `TestCompensateWriteFileExisting` — existing content restored

Tags: `#tests`

---

## Verification

```
go test ./internal/runtime/... ./internal/tools/... ./internal/storage/...
```

- Write a file via agent → cancel → file should be restored to pre-run state
- Check `mutations` table: status transitions logged correctly
- `run_shell` with no compensation: logged as non-compensable, saga status = `rolled_back` with warning
