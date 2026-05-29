package reliability

import (
	"context"
	"math/rand/v2"
	"time"
)

// RetryConfig controls exponential back-off behaviour.
type RetryConfig struct {
	MaxAttempts int
	BaseDelay   time.Duration
	MaxDelay    time.Duration
	Multiplier  float64
	Jitter      bool
}

// DefaultRetry is suitable for LLM API calls.
var DefaultRetry = RetryConfig{
	MaxAttempts: 4,
	BaseDelay:   500 * time.Millisecond,
	MaxDelay:    30 * time.Second,
	Multiplier:  2.0,
	Jitter:      true,
}

// Do calls fn up to cfg.MaxAttempts times with exponential back-off.
// Only retries when IsRetryable(err) returns true. Context cancellation
// stops the loop immediately.
func Do(ctx context.Context, cfg RetryConfig, fn func() error) error {
	delay := cfg.BaseDelay
	var lastErr error
	for attempt := 0; attempt < cfg.MaxAttempts; attempt++ {
		if err := ctx.Err(); err != nil {
			return err
		}
		lastErr = fn()
		if lastErr == nil {
			return nil
		}
		if !IsRetryable(lastErr) {
			return lastErr
		}
		if attempt == cfg.MaxAttempts-1 {
			break
		}
		sleep := delay
		if cfg.Jitter {
			// ±25% jitter
			jitter := time.Duration(float64(delay) * 0.25 * (rand.Float64()*2 - 1))
			sleep += jitter
			if sleep < 0 {
				sleep = delay / 2
			}
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(sleep):
		}
		delay = time.Duration(float64(delay) * cfg.Multiplier)
		if delay > cfg.MaxDelay {
			delay = cfg.MaxDelay
		}
	}
	return lastErr
}
