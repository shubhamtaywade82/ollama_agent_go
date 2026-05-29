package sqlite_test

import (
	"context"
	"testing"
	"time"

	"ollama_agent_go/internal/observability"
	sqstore "ollama_agent_go/internal/storage/sqlite"
)

func TestSQLiteAuditLogAndQuery(t *testing.T) {
	db := openTestRawDB(t)
	al := sqstore.NewAuditLogger(db)
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

	// Verify fields round-trip correctly.
	if got[0].Actor != "agent" {
		t.Errorf("actor: got %q, want agent", got[0].Actor)
	}
	if got[0].Resource != "read_file" {
		t.Errorf("resource: got %q, want read_file", got[0].Resource)
	}
}
