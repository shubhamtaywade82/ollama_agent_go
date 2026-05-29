package reliability

import (
	"context"
	"errors"
	"net"
	"strings"
)

// ErrorClass classifies an error for retry / surface / abort decisions.
type ErrorClass int

const (
	ErrClassTransient  ErrorClass = iota // network, rate-limit → retry
	ErrClassPermanent                    // bad request, auth → no retry
	ErrClassToolFail                     // tool execution error → continue with error result
	ErrClassLoopLimit                    // MaxIterations exceeded → surface to user
	ErrClassCancelled                    // context cancelled → saga rollback
)

// Classify maps an error to its class so callers can decide how to respond.
func Classify(err error) ErrorClass {
	if err == nil {
		return ErrClassTransient // shouldn't happen, but safe
	}
	if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
		return ErrClassCancelled
	}

	msg := strings.ToLower(err.Error())

	// Network-level transient signals.
	var netErr *net.OpError
	if errors.As(err, &netErr) {
		return ErrClassTransient
	}

	// HTTP rate-limiting / server overload.
	if strings.Contains(msg, "429") || strings.Contains(msg, "503") ||
		strings.Contains(msg, "rate limit") || strings.Contains(msg, "too many requests") ||
		strings.Contains(msg, "service unavailable") || strings.Contains(msg, "connection refused") ||
		strings.Contains(msg, "connection reset") || strings.Contains(msg, "timeout") ||
		strings.Contains(msg, "eof") {
		return ErrClassTransient
	}

	// Loop limit reached.
	if strings.Contains(msg, "max iterations") || strings.Contains(msg, "reached max") {
		return ErrClassLoopLimit
	}

	// Tool execution errors carry specific phrasing from the tool host.
	if strings.Contains(msg, "tool") && strings.Contains(msg, "error") {
		return ErrClassToolFail
	}

	// Auth / bad request signals → permanent.
	if strings.Contains(msg, "401") || strings.Contains(msg, "403") ||
		strings.Contains(msg, "unauthorized") || strings.Contains(msg, "forbidden") ||
		strings.Contains(msg, "invalid api key") || strings.Contains(msg, "400") {
		return ErrClassPermanent
	}

	return ErrClassPermanent
}

// IsRetryable returns true if err is worth retrying (transient).
func IsRetryable(err error) bool {
	return Classify(err) == ErrClassTransient
}
