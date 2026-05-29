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
	// Create creates a new session and returns it.
	Create(ctx context.Context, id, model string) (*Session, error)
	// Get retrieves a session by ID, including its full conversation.
	Get(ctx context.Context, id string) (*Session, error)
	// SetState updates the session state.
	SetState(ctx context.Context, id string, state SessionState) error
	// AppendMessage appends a message to the session's conversation.
	AppendMessage(ctx context.Context, sessionID string, msg types.Message) error
	// RecordToolInvocation stores a completed tool call.
	RecordToolInvocation(ctx context.Context, rec ToolInvocationRecord) error
	// RecordTokens logs token usage for a session turn.
	RecordTokens(ctx context.Context, sessionID, model string, prompt, output int) error
	// Close releases underlying resources.
	Close() error
}
