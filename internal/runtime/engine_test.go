package runtime

import (
	"context"
	"testing"

	"ollama_agent_go/internal/agent"
	"ollama_agent_go/internal/observability"
	"ollama_agent_go/internal/policy"
	sqstore "ollama_agent_go/internal/storage/sqlite"
	"ollama_agent_go/internal/tools"
	"ollama_agent_go/internal/types"
)

// fakeChatter is a test double that returns canned responses.
type fakeChatter struct {
	responses []types.ChatResponse
	i         int
}

func (f *fakeChatter) Chat(_ context.Context, req types.ChatRequest) (types.ChatResponse, error) {
	r := f.responses[f.i]
	f.i++
	return r, nil
}

func newTestEngine(t *testing.T) *Engine {
	t.Helper()
	root := t.TempDir()
	dbPath := root + "/test.db"

	db, err := sqstore.Open(dbPath)
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	t.Cleanup(func() { db.Close() })
	store := sqstore.NewStore(db)

	pol := policy.NewDefaultEngine(root, nil, true)
	reg := tools.Default(root)
	host := tools.NewHost(reg, pol, observability.Discard)

	fc := &fakeChatter{responses: []types.ChatResponse{
		{Message: types.Message{Role: "assistant", Content: "hello from engine"}},
	}}
	ag := agent.New(fc, host, "test-model")

	bus := NewInProcBus()
	return NewEngine(ag, host, nil, store, bus, observability.Discard, nil)
}

func TestEngineSubmitPersistsMessages(t *testing.T) {
	engine := newTestEngine(t)
	ctx := context.Background()

	if err := engine.Init(ctx); err != nil {
		t.Fatalf("init: %v", err)
	}

	var events []agent.Event
	err := engine.Submit(ctx, "hello", func(e agent.Event) {
		events = append(events, e)
	})
	if err != nil {
		t.Fatalf("submit: %v", err)
	}

	// Session should have user + assistant messages.
	if len(engine.session.Conversation) != 2 {
		t.Errorf("conversation length = %d, want 2", len(engine.session.Conversation))
	}
	if engine.session.Conversation[0].Role != "user" {
		t.Errorf("first message role = %q, want user", engine.session.Conversation[0].Role)
	}
	if engine.session.Conversation[1].Role != "assistant" {
		t.Errorf("second message role = %q, want assistant", engine.session.Conversation[1].Role)
	}

	// Events should include at least EventDone.
	var sawDone bool
	for _, e := range events {
		if e.Kind == agent.EventDone {
			sawDone = true
		}
	}
	if !sawDone {
		t.Error("EventDone not emitted")
	}
}

func TestEngineMultiTurnConversation(t *testing.T) {
	root := t.TempDir()
	dbPath := root + "/test.db"
	db, _ := sqstore.Open(dbPath)
	t.Cleanup(func() { db.Close() })
	store := sqstore.NewStore(db)

	pol := policy.NewDefaultEngine(root, nil, true)
	reg := tools.Default(root)
	host := tools.NewHost(reg, pol, observability.Discard)

	fc := &fakeChatter{responses: []types.ChatResponse{
		{Message: types.Message{Role: "assistant", Content: "turn 1 response"}},
		{Message: types.Message{Role: "assistant", Content: "turn 2 response"}},
	}}
	ag := agent.New(fc, host, "test-model")
	bus := NewInProcBus()
	engine := NewEngine(ag, host, nil, store, bus, observability.Discard, nil)

	ctx := context.Background()
	_ = engine.Init(ctx)
	_ = engine.Submit(ctx, "first message", func(agent.Event) {})
	_ = engine.Submit(ctx, "second message", func(agent.Event) {})

	// After two turns: user1, asst1, user2, asst2 = 4 messages.
	if len(engine.session.Conversation) != 4 {
		t.Errorf("conversation length after 2 turns = %d, want 4", len(engine.session.Conversation))
	}
}

func TestEngineClearHistory(t *testing.T) {
	engine := newTestEngine(t)
	ctx := context.Background()
	_ = engine.Init(ctx)
	firstID := engine.SessionID()

	_ = engine.Submit(ctx, "some input", func(agent.Event) {})
	if err := engine.ClearHistory(ctx); err != nil {
		t.Fatalf("clear: %v", err)
	}

	if engine.SessionID() == firstID {
		t.Error("session ID should change after clear")
	}
	if len(engine.session.Conversation) != 0 {
		t.Error("conversation should be empty after clear")
	}
}
