// Package episodic defines the interface for structured episode persistence.
// Episodes capture named, summarised events (completed tasks, key decisions)
// that can be recalled across sessions.
package episodic

import (
	"context"
	"time"
)

// Episode is a named summary of a completed interaction or decision.
type Episode struct {
	ID        string
	SessionID string
	Summary   string
	Tags      []string
	CreatedAt time.Time
}

// Store persists and queries episodes.
type Store interface {
	Save(ctx context.Context, ep Episode) error
	List(ctx context.Context, limit int) ([]Episode, error)
	Search(ctx context.Context, tag string) ([]Episode, error)
}

// NoopStore satisfies Store with empty returns.
type NoopStore struct{}

func (NoopStore) Save(_ context.Context, _ Episode) error          { return nil }
func (NoopStore) List(_ context.Context, _ int) ([]Episode, error) { return nil, nil }
func (NoopStore) Search(_ context.Context, _ string) ([]Episode, error) {
	return nil, nil
}
