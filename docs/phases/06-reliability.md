# Phase 06 — Reliability & Failure Management

Tags: `#reliability` `#retry` `#backoff` `#circuit-breaker` `#fallback` `#hitl` `#error-detection` `#p1` `#status/done`

Prerequisites: Phase 04 (provider router fallback), Phase 05 (spans for error rate measurement).

---

## Goal

Make the agent loop robust against transient LLM failures, tool errors, and runaway loops.
Implements: error detection, retry with exponential backoff, circuit breaker per provider,
human-in-the-loop interrupt, and fallback agent selection.

---

## Files to create (in order)

### Step 1 — Retry + backoff
**`internal/reliability/retry.go`**

```go
package reliability

type RetryConfig struct {
    MaxAttempts int
    BaseDelay   time.Duration   // e.g. 500ms
    MaxDelay    time.Duration   // e.g. 30s
    Multiplier  float64         // e.g. 2.0
    Jitter      bool
}

var DefaultRetry = RetryConfig{
    MaxAttempts: 4,
    BaseDelay:   500 * time.Millisecond,
    MaxDelay:    30 * time.Second,
    Multiplier:  2.0,
    Jitter:      true,
}

// Do runs fn with exponential backoff. Only retries on retryable errors.
func Do(ctx context.Context, cfg RetryConfig, fn func() error) error

// IsRetryable returns true for network errors, HTTP 429/503.
func IsRetryable(err error) bool
```

Tags: `#reliability/retry`

---

### Step 2 — Circuit breaker
**`internal/reliability/circuit_breaker.go`**

States: `Closed → Open → HalfOpen → Closed`

```go
type State int

const (
    StateClosed   State = iota
    StateOpen
    StateHalfOpen
)

type CircuitBreaker struct {
    name           string
    failureThresh  int
    successThresh  int
    openTimeout    time.Duration
    failures       int
    successes      int
    state          State
    openedAt       time.Time
    mu             sync.Mutex
}

func New(name string, failureThresh int, openTimeout time.Duration) *CircuitBreaker

// Allow returns false if the circuit is open.
func (cb *CircuitBreaker) Allow() bool
func (cb *CircuitBreaker) RecordSuccess()
func (cb *CircuitBreaker) RecordFailure()
func (cb *CircuitBreaker) State() State
```

Tags: `#reliability/circuit-breaker`

---

### Step 3 — Circuit breaker registry
**`internal/reliability/registry.go`**

```go
// Breakers holds one CircuitBreaker per named provider.
type Breakers struct {
    breakers map[string]*CircuitBreaker
    mu       sync.RWMutex
    cfg      BreakerConfig
}

type BreakerConfig struct {
    FailureThreshold int
    OpenTimeout      time.Duration
}

func NewBreakers(cfg BreakerConfig) *Breakers
func (r *Breakers) Get(name string) *CircuitBreaker
```

Tags: `#reliability/registry`

---

### Step 4 — Wrap provider Chat with retry + circuit breaker
**`internal/providers/router/router.go`** — update `Chat()`:

```go
func (r *Router) Chat(ctx context.Context, req types.ChatRequest) (types.ChatResponse, error) {
    p := r.selectProvider(...)
    cb := r.breakers.Get(p.Name())
    if !cb.Allow() {
        return r.fallback(ctx, req, p.Name())
    }
    var resp types.ChatResponse
    err := reliability.Do(ctx, reliability.DefaultRetry, func() error {
        var e error
        resp, e = p.Chat(ctx, req)
        return e
    })
    if err != nil {
        cb.RecordFailure()
        return r.fallback(ctx, req, p.Name())
    }
    cb.RecordSuccess()
    return resp, nil
}
```

Add `breakers *reliability.Breakers` field to `Router`.

Tags: `#reliability/router`

---

### Step 5 — Human-in-the-loop interrupt
**`internal/runtime/engine.go`** — add `Interrupt(reason string)` method:

```go
// Interrupt signals the currently running Submit() to pause and surface a HITL checkpoint.
func (e *Engine) Interrupt(reason string)

type HITLCheckpoint struct {
    SessionID string
    Reason    string
    Pending   []types.Message   // conversation so far
    At        time.Time
}

// HITLHandler is called when engine.Interrupt() fires; returns true to continue, false to abort.
type HITLHandler func(cp HITLCheckpoint) bool
```

In `Submit()`, check a `interrupted` atomic bool at the top of each iteration of the think-act loop.
If set, call `e.hitlHandler(checkpoint)` — block until handler returns, then either resume or cancel.

**`internal/ui/tui/model.go`** — register HITL handler that renders a confirmation prompt
(reuse the existing `huh` approval form).

Tags: `#reliability/hitl` `#runtime/engine`

---

### Step 6 — Error classification
**`internal/reliability/errors.go`**

```go
type ErrorClass int

const (
    ErrClassTransient  ErrorClass = iota  // network, rate-limit → retry
    ErrClassPermanent                      // bad request, auth → no retry
    ErrClassToolFail                       // tool execution error → continue with error result
    ErrClassLoopLimit                      // MaxIterations exceeded → surface to user
    ErrClassCancelled                      // context cancelled → saga rollback
)

func Classify(err error) ErrorClass
```

Used by `Engine.Submit()` to decide: retry, skip, surface, or abort.

Tags: `#reliability/error-classification`

---

### Step 7 — Agent-level fallback
**`internal/runtime/engine.go`** — if primary agent fails `MaxFallbackAttempts` times,
switch to a simpler model (e.g., fall back from qwen3-coder to qwen3:4b):

```go
type FallbackConfig struct {
    MaxAttempts  int
    FallbackModel string
}
```

Tags: `#reliability/agent-fallback`

---

## Tests

**`internal/reliability/retry_test.go`**
- `TestDoSucceedsOnSecondAttempt`
- `TestDoRespectsMaxAttempts`
- `TestDoNonRetryableErrorNotRetried`
- `TestDoContextCancellation`

**`internal/reliability/circuit_breaker_test.go`**
- `TestBreakerOpensAfterThreshold`
- `TestBreakerHalfOpenAllowsOneRequest`
- `TestBreakerResetsOnSuccess`

Tags: `#tests`

---

## Verification

```
go test ./internal/reliability/...
```

- Simulate 5 consecutive LLM 503 errors → circuit opens → subsequent calls go to fallback provider
- Cancel a running Submit() → saga rolls back, HITL handler never called (cancelled, not interrupted)
- Call engine.Interrupt() during tool loop → HITL checkpoint shown in TUI → user can continue or abort
