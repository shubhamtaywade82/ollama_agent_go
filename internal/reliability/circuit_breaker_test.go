package reliability_test

import (
	"testing"
	"time"

	"ollama_agent_go/internal/reliability"
)

func TestBreakerOpensAfterThreshold(t *testing.T) {
	cb := reliability.NewCircuitBreaker("test", 3, 10*time.Second)

	if !cb.Allow() {
		t.Fatal("should allow initially")
	}

	for i := 0; i < 3; i++ {
		cb.RecordFailure()
	}

	if cb.CurrentState() != reliability.StateOpen {
		t.Errorf("expected Open after 3 failures, got %s", cb.CurrentState())
	}
	if cb.Allow() {
		t.Error("should not allow when open")
	}
}

func TestBreakerHalfOpenAllowsOneRequest(t *testing.T) {
	cb := reliability.NewCircuitBreaker("test", 1, 1*time.Millisecond)
	cb.RecordFailure() // trips the breaker

	// Wait for open timeout to expire.
	time.Sleep(5 * time.Millisecond)

	// First call should be allowed (probe).
	if !cb.Allow() {
		t.Error("should allow probe request in half-open state")
	}
	if cb.CurrentState() != reliability.StateHalfOpen {
		t.Errorf("expected HalfOpen, got %s", cb.CurrentState())
	}
}

func TestBreakerResetsOnSuccess(t *testing.T) {
	cb := reliability.NewCircuitBreaker("test", 2, 1*time.Millisecond)
	cb.RecordFailure()
	cb.RecordFailure() // trips
	if cb.CurrentState() != reliability.StateOpen {
		t.Fatal("should be Open")
	}

	time.Sleep(5 * time.Millisecond)
	cb.Allow() // transitions to HalfOpen
	cb.RecordSuccess()

	if cb.CurrentState() != reliability.StateClosed {
		t.Errorf("expected Closed after success in HalfOpen, got %s", cb.CurrentState())
	}
	if !cb.Allow() {
		t.Error("should allow requests when closed")
	}
}

func TestBreakerReopensOnHalfOpenFailure(t *testing.T) {
	cb := reliability.NewCircuitBreaker("test", 1, 1*time.Millisecond)
	cb.RecordFailure()       // trips
	time.Sleep(5 * time.Millisecond)
	cb.Allow()               // transitions to HalfOpen
	cb.RecordFailure()       // fails the probe → re-open

	if cb.CurrentState() != reliability.StateOpen {
		t.Errorf("expected Open after probe failure, got %s", cb.CurrentState())
	}
}

func TestBreakerRegistryCreatesDistinctBreakers(t *testing.T) {
	reg := reliability.NewBreakers(reliability.BreakerConfig{
		FailureThreshold: 2,
		OpenTimeout:      5 * time.Second,
	})

	b1 := reg.Get("provider-a")
	b2 := reg.Get("provider-b")
	b1same := reg.Get("provider-a")

	if b1 == b2 {
		t.Error("different providers should have different breakers")
	}
	if b1 != b1same {
		t.Error("same name should return same breaker")
	}
}
