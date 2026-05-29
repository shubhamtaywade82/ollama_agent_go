// Package storage defines the persistence interfaces used by the runtime engine.
package storage

import (
	"context"
	"time"

	"ollama_agent_go/internal/types"
)

// SessionState represents the lifecycle state of an agent session.
type SessionState string

const (
	StateIdle      SessionState = "idle"
	StateRunning   SessionState = "running"
	StateCancelled SessionState = "cancelled"
	StateFailed    SessionState = "failed"
)

// Session is a persisted agent session with its full conversation history.
type Session struct {
	ID           string
	State        SessionState
	Model        string
	Conversation []types.Message
	CreatedAt    time.Time
	UpdatedAt    time.Time
}

// ToolInvocationRecord is a persisted record of a single tool call.
type ToolInvocationRecord struct {
	ID         string
	SessionID  string
	Tool       string
	Args       map[string]any
	Output     string
	Error      string
	DurationMs int64
	Approved   bool
	CreatedAt  time.Time
}

// SessionStore persists sessions and their associated data.
type SessionStore interface {
	Create(ctx context.Context, id, model string) (*Session, error)
	Get(ctx context.Context, id string) (*Session, error)
	SetState(ctx context.Context, id string, state SessionState) error
	AppendMessage(ctx context.Context, sessionID string, msg types.Message) error
	RecordToolInvocation(ctx context.Context, rec ToolInvocationRecord) error
	RecordTokens(ctx context.Context, sessionID, model string, prompt, output int) error
	Close() error
}

// ─── Saga / WAL ────────────────────────────────────────────────────────────

// SagaStatus is the lifecycle state of a single mutation within a saga.
type SagaStatus string

const (
	SagaReserved   SagaStatus = "reserved"
	SagaLocked     SagaStatus = "locked"
	SagaApplied    SagaStatus = "applied"
	SagaCommitted  SagaStatus = "committed"
	SagaRolledBack SagaStatus = "rolled_back"
)

// Mutation records a single filesystem-mutating tool call and its before/after state.
type Mutation struct {
	ID          string
	SessionID   string
	Tool        string
	Args        map[string]any
	BeforeState []byte // file content before execution; nil if the file was new
	AfterState  []byte // file content after execution
	Status      SagaStatus
	CreatedAt   time.Time
	UpdatedAt   time.Time
}

// SagaStore persists mutation lifecycle records for WAL-style rollback.
type SagaStore interface {
	// Reserve creates a new mutation record in the reserved state.
	Reserve(ctx context.Context, m Mutation) error
	// Transition moves a mutation to the given state, optionally recording after-state.
	Transition(ctx context.Context, id string, to SagaStatus, after []byte) error
	// Compensable returns all applied or committed mutations for sessionID,
	// ordered oldest-first, suitable for reverse-order compensation.
	Compensable(ctx context.Context, sessionID string) ([]Mutation, error)
}

