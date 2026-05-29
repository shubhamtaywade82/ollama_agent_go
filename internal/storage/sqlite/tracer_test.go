package sqlite_test

import (
	"context"
	"database/sql"
	"os"
	"testing"
	"time"

	"ollama_agent_go/internal/observability"
	sqstore "ollama_agent_go/internal/storage/sqlite"
)

func openTestRawDB(t *testing.T) *sql.DB {
	t.Helper()
	f, err := os.CreateTemp("", "tracer-test-*.db")
	if err != nil {
		t.Fatal(err)
	}
	f.Close()
	t.Cleanup(func() { os.Remove(f.Name()) })

	d, err := sqstore.Open(f.Name())
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { d.Close() })
	return d.Underlying()
}

func TestStartFinishSpan(t *testing.T) {
	db := openTestRawDB(t)
	tr := sqstore.NewTracer(db)

	ctx := context.Background()
	ctx, span := tr.Start(ctx, observability.SpanToolCall, "session-1")
	span.Tool = "write_file"

	time.Sleep(2 * time.Millisecond)
	tr.Finish(ctx, span, nil)

	if span.DurationMs <= 0 {
		t.Errorf("expected non-zero duration, got %d ms", span.DurationMs)
	}
	if span.EndedAt.IsZero() {
		t.Error("EndedAt should be set")
	}
}

func TestExportBySession(t *testing.T) {
	db := openTestRawDB(t)
	tr := sqstore.NewTracer(db)

	ctx := context.Background()
	for _, sid := range []string{"s1", "s1", "s2"} {
		_, span := tr.Start(ctx, observability.SpanToolCall, sid)
		tr.Finish(ctx, span, nil)
	}

	spans, err := tr.Export(ctx, "s1")
	if err != nil {
		t.Fatalf("export: %v", err)
	}
	if len(spans) != 2 {
		t.Errorf("want 2 spans for s1, got %d", len(spans))
	}

	spans2, err := tr.Export(ctx, "s2")
	if err != nil {
		t.Fatalf("export s2: %v", err)
	}
	if len(spans2) != 1 {
		t.Errorf("want 1 span for s2, got %d", len(spans2))
	}
}

func TestSpanParentLinking(t *testing.T) {
	db := openTestRawDB(t)
	tr := sqstore.NewTracer(db)

	ctx := context.Background()
	ctx, parent := tr.Start(ctx, observability.SpanAgentTurn, "sess")
	tr.Finish(ctx, parent, nil)

	_, child := tr.Start(ctx, observability.SpanToolCall, "sess")
	tr.Finish(ctx, child, nil)

	if child.ParentID != parent.ID {
		t.Errorf("child.ParentID = %q, want %q", child.ParentID, parent.ID)
	}
}
