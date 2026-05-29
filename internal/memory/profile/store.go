// Package profile defines the interface for persistent user/org preferences.
package profile

import (
	"context"
	"time"
)

// Entry is a single key-value preference.
type Entry struct {
	Key       string
	Value     string
	UpdatedAt time.Time
}

// Store persists and retrieves profile preferences across sessions.
type Store interface {
	Set(ctx context.Context, key, value string) error
	Get(ctx context.Context, key string) (string, bool, error)
	All(ctx context.Context) ([]Entry, error)
}

// NoopStore satisfies Store with empty returns.
type NoopStore struct{}

func (NoopStore) Set(_ context.Context, _, _ string) error              { return nil }
func (NoopStore) Get(_ context.Context, _ string) (string, bool, error) { return "", false, nil }
func (NoopStore) All(_ context.Context) ([]Entry, error)                { return nil, nil }
