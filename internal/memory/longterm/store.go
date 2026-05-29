// Package longterm defines the interface for vector-similarity retrieval.
// The real implementation is added in Phase 08 (RAG). This package ships
// a NoopStore so the memory.Manager compiles without a vector DB dependency.
package longterm

import "context"

// Result holds a single similarity-search hit.
type Result struct {
	ID    string
	Text  string
	Score float32
	Meta  map[string]any
}

// Store is the interface for long-term semantic memory.
type Store interface {
	Insert(ctx context.Context, id string, text string, meta map[string]any) error
	Search(ctx context.Context, query string, topK int) ([]Result, error)
}

// NoopStore satisfies Store with empty returns. Used until Phase 08.
type NoopStore struct{}

func (NoopStore) Insert(_ context.Context, _ string, _ string, _ map[string]any) error {
	return nil
}

func (NoopStore) Search(_ context.Context, _ string, _ int) ([]Result, error) {
	return nil, nil
}
