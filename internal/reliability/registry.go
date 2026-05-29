package reliability

import (
	"sync"
	"time"
)

// BreakerConfig applies to every breaker created by a Breakers registry.
type BreakerConfig struct {
	FailureThreshold int
	OpenTimeout      time.Duration
}

// DefaultBreakerConfig is a sensible out-of-the-box configuration.
var DefaultBreakerConfig = BreakerConfig{
	FailureThreshold: 5,
	OpenTimeout:      30 * time.Second,
}

// Breakers holds one CircuitBreaker per named provider, creating on first access.
type Breakers struct {
	mu       sync.RWMutex
	breakers map[string]*CircuitBreaker
	cfg      BreakerConfig
}

// NewBreakers returns a Breakers registry using cfg for all breakers it creates.
func NewBreakers(cfg BreakerConfig) *Breakers {
	return &Breakers{
		breakers: make(map[string]*CircuitBreaker),
		cfg:      cfg,
	}
}

// Get returns (creating if necessary) the CircuitBreaker for name.
func (r *Breakers) Get(name string) *CircuitBreaker {
	r.mu.RLock()
	cb, ok := r.breakers[name]
	r.mu.RUnlock()
	if ok {
		return cb
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	// Double-check after lock upgrade.
	if cb, ok = r.breakers[name]; ok {
		return cb
	}
	cb = NewCircuitBreaker(name, r.cfg.FailureThreshold, r.cfg.OpenTimeout)
	r.breakers[name] = cb
	return cb
}
