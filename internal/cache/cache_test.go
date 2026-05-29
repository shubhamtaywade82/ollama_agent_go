package cache_test

import (
	"context"
	"testing"
	"time"

	"ollama_agent_go/internal/cache"
)

func TestInMemoryGetSet(t *testing.T) {
	c := cache.NewInMemory()
	ctx := context.Background()

	if err := c.Set(ctx, "k", []byte("v"), 0); err != nil {
		t.Fatal(err)
	}
	val, ok, err := c.Get(ctx, "k")
	if err != nil || !ok {
		t.Fatalf("expected hit, got ok=%v err=%v", ok, err)
	}
	if string(val) != "v" {
		t.Errorf("expected 'v', got %q", val)
	}
}

func TestInMemoryTTLExpiry(t *testing.T) {
	c := cache.NewInMemory()
	ctx := context.Background()

	if err := c.Set(ctx, "k", []byte("v"), 10*time.Millisecond); err != nil {
		t.Fatal(err)
	}
	time.Sleep(20 * time.Millisecond)
	_, ok, err := c.Get(ctx, "k")
	if err != nil {
		t.Fatal(err)
	}
	if ok {
		t.Error("expected cache miss after TTL expiry, got hit")
	}
}

func TestInMemoryDelete(t *testing.T) {
	c := cache.NewInMemory()
	ctx := context.Background()

	_ = c.Set(ctx, "k", []byte("v"), 0)
	_ = c.Delete(ctx, "k")
	_, ok, _ := c.Get(ctx, "k")
	if ok {
		t.Error("expected miss after delete")
	}
}

func TestInMemoryFlush(t *testing.T) {
	c := cache.NewInMemory()
	ctx := context.Background()

	_ = c.Set(ctx, "a", []byte("1"), 0)
	_ = c.Set(ctx, "b", []byte("2"), 0)
	_ = c.Flush(ctx)
	_, ok, _ := c.Get(ctx, "a")
	if ok {
		t.Error("expected miss after flush")
	}
}
