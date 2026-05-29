package retriever_test

import (
	"context"
	"strings"
	"testing"

	"ollama_agent_go/internal/memory/longterm"
	"ollama_agent_go/internal/retriever"
)

// memStore is an in-memory longterm.Store for tests.
type memStore struct {
	docs []longterm.Result
}

func (m *memStore) Insert(_ context.Context, id, text string, meta map[string]any) error {
	m.docs = append(m.docs, longterm.Result{ID: id, Text: text, Meta: meta, Score: 1.0})
	return nil
}

func (m *memStore) Search(_ context.Context, _ string, topK int) ([]longterm.Result, error) {
	if topK > len(m.docs) {
		topK = len(m.docs)
	}
	return m.docs[:topK], nil
}

func TestRetrieve(t *testing.T) {
	store := &memStore{}
	_ = store.Insert(context.Background(), "a", "chunk a text", map[string]any{"path": "a.md"})
	_ = store.Insert(context.Background(), "b", "chunk b text", map[string]any{"path": "b.md"})

	ret := retriever.New(store, 2)
	results, err := ret.Retrieve(context.Background(), "anything")
	if err != nil {
		t.Fatalf("retrieve: %v", err)
	}
	if len(results) != 2 {
		t.Errorf("want 2 results, got %d", len(results))
	}
}

func TestInjectContext(t *testing.T) {
	store := &memStore{}
	_ = store.Insert(context.Background(), "1", "content here", map[string]any{"path": "doc.md"})

	ret := retriever.New(store, 3)
	results, _ := ret.Retrieve(context.Background(), "query")
	ctx := ret.InjectContext(results)

	if !strings.Contains(ctx, "content here") {
		t.Error("injected context should contain chunk text")
	}
	if !strings.Contains(ctx, "doc.md") {
		t.Error("injected context should contain source path")
	}
	if !strings.HasPrefix(ctx, "## Retrieved Context") {
		t.Error("injected context should have section header")
	}
}

func TestInjectContextEmpty(t *testing.T) {
	ret := retriever.New(&memStore{}, 3)
	ctx := ret.InjectContext(nil)
	if ctx != "" {
		t.Errorf("empty results should produce empty context, got %q", ctx)
	}
}
