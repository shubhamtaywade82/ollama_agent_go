package runtime

import (
	"context"
	"time"
)

// SessionState tracks the current phase of a session.
type SessionState int

const (
	StateIdle      SessionState = iota
	StateRunning
	StateCancelled
	StateFailed
)

// Session is the in-memory state for the current agent session.
type Session struct {
	ID        string
	State     SessionState
	CreatedAt time.Time
	UpdatedAt time.Time
}

// RunContext bundles the context and cancellation for a single agent run.
type RunContext struct {
	Ctx     context.Context
	Cancel  context.CancelFunc
	Session *Session
	Input   string
}
