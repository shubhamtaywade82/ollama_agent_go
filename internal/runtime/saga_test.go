package runtime

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"ollama_agent_go/internal/agent"
	"ollama_agent_go/internal/observability"
	"ollama_agent_go/internal/policy"
	sqstore "ollama_agent_go/internal/storage/sqlite"
	"ollama_agent_go/internal/tools"
	"ollama_agent_go/internal/types"
)

// newSagaEngine wires a test engine that uses a real SQLite store and a Host
// with saga enabled, so we can assert on file state after rollback.
func newSagaEngine(t *testing.T, root string, responses []types.ChatResponse) *Engine {
	t.Helper()
	db, err := sqstore.Open(filepath.Join(root, "test.db"))
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	t.Cleanup(func() { db.Close() })
	store := sqstore.NewStore(db)

	pol := policy.NewDefaultEngine(root, nil, true) // no approval needed
	reg := tools.Default(root)
	host := tools.NewHost(reg, pol, observability.Discard)
	host.Saga = store // wire saga store

	fc := &fakeChatter{responses: responses}
	ag := agent.New(fc, host, "test-model")

	bus := NewInProcBus()
	return NewEngine(ag, host, nil, store, bus, observability.Discard, nil)
}

func TestSagaRollbackRestoresFile(t *testing.T) {
	root := t.TempDir()

	// Pre-populate a file so the agent overwrites it (existing content scenario).
	target := filepath.Join(root, "data.txt")
	original := []byte("original content")
	if err := os.WriteFile(target, original, 0o644); err != nil {
		t.Fatal(err)
	}

	// Agent response: write_file call that overwrites data.txt.
	responses := []types.ChatResponse{
		{Message: types.Message{
			Role: "assistant",
			ToolCalls: []types.ToolCall{{
				Function: types.ToolCallFunction{
					Name:      "write_file",
					Arguments: map[string]any{"path": "data.txt", "content": "overwritten"},
				},
			}},
		}},
		// The context will be cancelled before the second response is consumed.
		{Message: types.Message{Role: "assistant", Content: "done"}},
	}

	engine := newSagaEngine(t, root, responses)
	ctx := context.Background()
	_ = engine.Init(ctx)

	// Cancel context mid-run using a cancellable context.
	cancelCtx, cancel := context.WithCancel(ctx)
	cancel() // cancel immediately so Submit exits after write_file but triggers rollback

	_ = engine.Submit(cancelCtx, "overwrite data.txt", func(agent.Event) {})

	// After rollback, the file should be restored to original content.
	got, err := os.ReadFile(target)
	if err != nil {
		t.Fatalf("read after rollback: %v", err)
	}
	if string(got) != string(original) {
		t.Errorf("after rollback content = %q, want %q", got, original)
	}
}

func TestSagaNewFileDeletedOnRollback(t *testing.T) {
	root := t.TempDir()
	newFile := filepath.Join(root, "new.txt")

	responses := []types.ChatResponse{
		{Message: types.Message{
			Role: "assistant",
			ToolCalls: []types.ToolCall{{
				Function: types.ToolCallFunction{
					Name:      "write_file",
					Arguments: map[string]any{"path": "new.txt", "content": "hello"},
				},
			}},
		}},
		{Message: types.Message{Role: "assistant", Content: "done"}},
	}

	engine := newSagaEngine(t, root, responses)
	ctx := context.Background()
	_ = engine.Init(ctx)

	cancelCtx, cancel := context.WithCancel(ctx)
	cancel()
	_ = engine.Submit(cancelCtx, "create new.txt", func(agent.Event) {})

	if _, err := os.Stat(newFile); !os.IsNotExist(err) {
		t.Error("newly created file should be deleted on rollback")
	}
}
