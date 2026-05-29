package observability_test

import (
	"context"
	"testing"
	"time"

	"ollama_agent_go/internal/observability"
)

// inMemAudit is a simple in-memory AuditLogger for unit tests that don't need SQLite.
type inMemAudit struct {
	events []observability.AuditEvent
}

func (a *inMemAudit) Log(_ context.Context, ev observability.AuditEvent) error {
	a.events = append(a.events, ev)
	return nil
}

func (a *inMemAudit) Query(_ context.Context, sessionID string) ([]observability.AuditEvent, error) {
	var out []observability.AuditEvent
	for _, ev := range a.events {
		if ev.SessionID == sessionID {
			out = append(out, ev)
		}
	}
	return out, nil
}

func TestAuditLogAndQuery(t *testing.T) {
	al := &inMemAudit{}
	ctx := context.Background()

	events := []observability.AuditEvent{
		{SessionID: "s1", Actor: "agent", Action: "tool_call", Resource: "read_file", Result: "ok", At: time.Now()},
		{SessionID: "s1", Actor: "system", Action: "approval_granted", Resource: "write_file", Result: "ok", At: time.Now()},
		{SessionID: "s2", Actor: "user", Action: "submit", Result: "ok", At: time.Now()},
	}

	for _, ev := range events {
		if err := al.Log(ctx, ev); err != nil {
			t.Fatalf("log: %v", err)
		}
	}

	got, err := al.Query(ctx, "s1")
	if err != nil {
		t.Fatalf("query: %v", err)
	}
	if len(got) != 2 {
		t.Errorf("want 2 events for s1, got %d", len(got))
	}

	got2, err := al.Query(ctx, "s2")
	if err != nil {
		t.Fatalf("query s2: %v", err)
	}
	if len(got2) != 1 {
		t.Errorf("want 1 event for s2, got %d", len(got2))
	}

	// Noop logger should not fail.
	if err := observability.NoopAuditLogger.Log(ctx, events[0]); err != nil {
		t.Errorf("noop log: %v", err)
	}
}
