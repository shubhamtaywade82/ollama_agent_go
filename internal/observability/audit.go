package observability

import (
	"context"
	"time"
)

// AuditEvent records a single auditable action taken by the agent runtime.
type AuditEvent struct {
	ID        string
	SessionID string
	Actor     string // "agent" | "user" | "system"
	Action    string // "tool_call", "approval_granted", "approval_denied", "session_start", "session_clear"
	Resource  string // tool name, file path, etc.
	Result    string // "ok" | "denied" | "error"
	Detail    string // free-form JSON or message
	At        time.Time
}

// AuditLogger persists and queries audit events.
type AuditLogger interface {
	Log(ctx context.Context, ev AuditEvent) error
	Query(ctx context.Context, sessionID string) ([]AuditEvent, error)
}

// noopAuditLogger silently drops all events.
type noopAuditLogger struct{}

func (noopAuditLogger) Log(_ context.Context, _ AuditEvent) error { return nil }
func (noopAuditLogger) Query(_ context.Context, _ string) ([]AuditEvent, error) {
	return nil, nil
}

// NoopAuditLogger is an AuditLogger that records nothing.
var NoopAuditLogger AuditLogger = noopAuditLogger{}
