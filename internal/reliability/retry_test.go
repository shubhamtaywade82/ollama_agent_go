package reliability_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"ollama_agent_go/internal/reliability"
)

func TestDoSucceedsOnSecondAttempt(t *testing.T) {
	attempts := 0
	err := reliability.Do(context.Background(), reliability.RetryConfig{
		MaxAttempts: 3,
		BaseDelay:   time.Millisecond,
		Multiplier:  2,
	}, func() error {
		attempts++
		if attempts < 2 {
			return errors.New("connection refused")
		}
		return nil
	})
	if err != nil {
		t.Fatalf("expected success on second attempt, got: %v", err)
	}
	if attempts != 2 {
		t.Errorf("expected 2 attempts, got %d", attempts)
	}
}

func TestDoRespectsMaxAttempts(t *testing.T) {
	attempts := 0
	err := reliability.Do(context.Background(), reliability.RetryConfig{
		MaxAttempts: 3,
		BaseDelay:   time.Millisecond,
		Multiplier:  1,
	}, func() error {
		attempts++
		return errors.New("503 service unavailable")
	})
	if err == nil {
		t.Fatal("expected error after max attempts")
	}
	if attempts != 3 {
		t.Errorf("expected 3 attempts, got %d", attempts)
	}
}

func TestDoNonRetryableErrorNotRetried(t *testing.T) {
	attempts := 0
	err := reliability.Do(context.Background(), reliability.RetryConfig{
		MaxAttempts: 5,
		BaseDelay:   time.Millisecond,
		Multiplier:  1,
	}, func() error {
		attempts++
		return errors.New("401 unauthorized")
	})
	if err == nil {
		t.Fatal("expected error")
	}
	if attempts != 1 {
		t.Errorf("non-retryable error should not be retried; got %d attempts", attempts)
	}
}

func TestDoContextCancellation(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel() // pre-cancel

	err := reliability.Do(ctx, reliability.RetryConfig{
		MaxAttempts: 5,
		BaseDelay:   time.Millisecond,
		Multiplier:  1,
	}, func() error {
		return errors.New("connection refused")
	})
	if !errors.Is(err, context.Canceled) {
		t.Errorf("expected context.Canceled, got %v", err)
	}
}
