package sqlite

import (
	"context"
	"path/filepath"
	"testing"

	"ollama_agent_go/internal/storage"
)

func newTestStore(t *testing.T) *Store {
	t.Helper()
	db, err := Open(filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	t.Cleanup(func() { db.Close() })
	return NewStore(db)
}

func TestSagaReserveAndCommit(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()

	m := storage.Mutation{
		ID:        "mut-1",
		SessionID: "sess-1",
		Tool:      "write_file",
		Args:      map[string]any{"path": "foo.txt"},
	}

	if err := s.Reserve(ctx, m); err != nil {
		t.Fatalf("reserve: %v", err)
	}
	if err := s.Transition(ctx, m.ID, storage.SagaLocked, nil); err != nil {
		t.Fatalf("lock: %v", err)
	}
	if err := s.Transition(ctx, m.ID, storage.SagaApplied, []byte("new content")); err != nil {
		t.Fatalf("apply: %v", err)
	}
	if err := s.Transition(ctx, m.ID, storage.SagaCommitted, nil); err != nil {
		t.Fatalf("commit: %v", err)
	}

	muts, err := s.Compensable(ctx, "sess-1")
	if err != nil {
		t.Fatalf("compensable: %v", err)
	}
	if len(muts) != 1 {
		t.Fatalf("compensable count = %d, want 1", len(muts))
	}
	if muts[0].Status != storage.SagaCommitted {
		t.Errorf("status = %q, want committed", muts[0].Status)
	}
	if string(muts[0].AfterState) != "new content" {
		t.Errorf("after_state = %q, want 'new content'", muts[0].AfterState)
	}
}

func TestSagaRolledBackExcludedFromCompensable(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()

	m := storage.Mutation{
		ID:        "mut-2",
		SessionID: "sess-2",
		Tool:      "write_file",
		Args:      map[string]any{"path": "bar.txt"},
	}
	_ = s.Reserve(ctx, m)
	_ = s.Transition(ctx, m.ID, storage.SagaRolledBack, nil)

	muts, err := s.Compensable(ctx, "sess-2")
	if err != nil {
		t.Fatalf("compensable: %v", err)
	}
	if len(muts) != 0 {
		t.Errorf("rolled_back mutation should not appear in compensable, got %d", len(muts))
	}
}

func TestSagaCompensableOrderedOldestFirst(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()

	for _, id := range []string{"a", "b", "c"} {
		m := storage.Mutation{
			ID: id, SessionID: "sess-3", Tool: "write_file",
			Args: map[string]any{"path": id + ".txt"},
		}
		_ = s.Reserve(ctx, m)
		_ = s.Transition(ctx, m.ID, storage.SagaApplied, nil)
	}

	muts, _ := s.Compensable(ctx, "sess-3")
	if len(muts) != 3 {
		t.Fatalf("want 3 compensable, got %d", len(muts))
	}
	if muts[0].ID != "a" || muts[1].ID != "b" || muts[2].ID != "c" {
		t.Errorf("wrong order: %v %v %v", muts[0].ID, muts[1].ID, muts[2].ID)
	}
}

func TestSagaBeforeStatePreserved(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()

	before := []byte("original file content")
	m := storage.Mutation{
		ID:          "mut-4",
		SessionID:   "sess-4",
		Tool:        "write_file",
		Args:        map[string]any{"path": "x.txt"},
		BeforeState: before,
	}
	_ = s.Reserve(ctx, m)
	_ = s.Transition(ctx, m.ID, storage.SagaApplied, []byte("new content"))

	muts, _ := s.Compensable(ctx, "sess-4")
	if len(muts) != 1 {
		t.Fatalf("want 1, got %d", len(muts))
	}
	if string(muts[0].BeforeState) != string(before) {
		t.Errorf("before_state = %q, want %q", muts[0].BeforeState, before)
	}
}
