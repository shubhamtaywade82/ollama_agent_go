// Package cache provides a simple key-value cache used by the agent runtime
// to avoid redundant LLM calls and embedding computations.
package cache

import (
	"context"
	"sync"
	"time"
)

// Cache is the interface for all cache implementations.
type Cache interface {
	Get(ctx context.Context, key string) ([]byte, bool, error)
	Set(ctx context.Context, key string, value []byte, ttl time.Duration) error
	Delete(ctx context.Context, key string) error
	Flush(ctx context.Context) error
}

// entry is one item stored in the in-memory cache.
type entry struct {
	value   []byte
	expires time.Time // zero = no expiry
}

func (e *entry) alive() bool {
	return e.expires.IsZero() || time.Now().Before(e.expires)
}

// InMemoryCache is a thread-safe in-process cache with optional TTL.
type InMemoryCache struct {
	mu    sync.RWMutex
	items map[string]*entry
}

// NewInMemory creates an InMemoryCache and starts the background eviction loop.
func NewInMemory() *InMemoryCache {
	c := &InMemoryCache{items: make(map[string]*entry)}
	go c.evict()
	return c
}

func (c *InMemoryCache) Get(_ context.Context, key string) ([]byte, bool, error) {
	c.mu.RLock()
	e, ok := c.items[key]
	c.mu.RUnlock()
	if !ok || !e.alive() {
		return nil, false, nil
	}
	return e.value, true, nil
}

func (c *InMemoryCache) Set(_ context.Context, key string, value []byte, ttl time.Duration) error {
	e := &entry{value: value}
	if ttl > 0 {
		e.expires = time.Now().Add(ttl)
	}
	c.mu.Lock()
	c.items[key] = e
	c.mu.Unlock()
	return nil
}

func (c *InMemoryCache) Delete(_ context.Context, key string) error {
	c.mu.Lock()
	delete(c.items, key)
	c.mu.Unlock()
	return nil
}

func (c *InMemoryCache) Flush(_ context.Context) error {
	c.mu.Lock()
	c.items = make(map[string]*entry)
	c.mu.Unlock()
	return nil
}

// evict runs every 30 seconds to remove expired entries.
func (c *InMemoryCache) evict() {
	t := time.NewTicker(30 * time.Second)
	defer t.Stop()
	for range t.C {
		c.mu.Lock()
		for k, e := range c.items {
			if !e.alive() {
				delete(c.items, k)
			}
		}
		c.mu.Unlock()
	}
}

// New returns an InMemoryCache (Redis support can be added when REDIS_URL is set).
func New(_ string) Cache {
	return NewInMemory()
}
