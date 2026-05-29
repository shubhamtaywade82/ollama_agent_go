// Package router manages multiple providers, routing requests to the best
// available provider based on task type, capability requirements, and cost.
package router

import (
	"context"
	"fmt"
	"strings"
	"sync"

	"ollama_agent_go/internal/providers"
	"ollama_agent_go/internal/reliability"
	"ollama_agent_go/internal/types"
)

// Router manages a pool of providers and routes chat requests intelligently.
type Router struct {
	mu       sync.RWMutex
	pool     []providers.Provider // ordered: first added = highest priority
	index    map[string]providers.Provider
	active   string // explicit override set by Switch(); empty = auto-select
	rules    []Rule
	breakers *reliability.Breakers
	retry    reliability.RetryConfig
}

// New creates a Router using the given rules. Pass no rules to use DefaultRules.
func New(rules ...Rule) *Router {
	if len(rules) == 0 {
		rules = DefaultRules
	}
	return &Router{
		index:    make(map[string]providers.Provider),
		rules:    rules,
		breakers: reliability.NewBreakers(reliability.DefaultBreakerConfig),
		retry:    reliability.DefaultRetry,
	}
}

// Add registers a provider. The first added becomes the default active provider.
func (r *Router) Add(p providers.Provider) {
	r.mu.Lock()
	defer r.mu.Unlock()
	key := strings.ToLower(p.Name())
	r.index[key] = p
	r.pool = append(r.pool, p)
	if r.active == "" {
		r.active = key
	}
}

// Switch sets the active provider by name, overriding task-type selection.
// Returns an error if the provider is not registered.
func (r *Router) Switch(name string) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	key := strings.ToLower(name)
	if _, ok := r.index[key]; !ok {
		return fmt.Errorf("provider %q not registered", name)
	}
	r.active = key
	return nil
}

// SetActive is an alias for Switch (backwards compatibility).
func (r *Router) SetActive(name string) error { return r.Switch(name) }

// Active returns the current active provider name.
func (r *Router) Active() string {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.active
}

// Names returns the names of all registered providers.
func (r *Router) Names() []string {
	r.mu.RLock()
	defer r.mu.RUnlock()
	out := make([]string, 0, len(r.pool))
	for _, p := range r.pool {
		out = append(out, p.Name())
	}
	return out
}

// Get returns a provider by name.
func (r *Router) Get(name string) (providers.Provider, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	p, ok := r.index[strings.ToLower(name)]
	return p, ok
}

// Chat routes the request to the best available provider. On error it
// attempts a fallback to the next qualifying provider before giving up.
// Retry with exponential backoff and per-provider circuit breaking are applied.
func (r *Router) Chat(ctx context.Context, req types.ChatRequest) (types.ChatResponse, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	if len(r.pool) == 0 {
		return types.ChatResponse{}, fmt.Errorf("router: no providers registered")
	}

	// 1. ModelHint: caller explicitly asked for a provider by model/name prefix.
	if req.ModelHint != "" {
		if p := r.findByHint(req.ModelHint); p != nil {
			resp, err := r.callWithReliability(ctx, p, req)
			if err == nil {
				return resp, nil
			}
			// hint provider failed — fall through to task selection
		}
	}

	// 2. Task-based selection.
	selected := r.selectForTask(Detect(req.Messages), "")
	if selected == nil {
		selected = r.pool[0]
	}

	resp, err := r.callWithReliability(ctx, selected, req)
	if err == nil {
		return resp, nil
	}

	// 3. Fallback: try remaining providers in pool order.
	return r.fallback(ctx, req, selected.Name())
}

// callWithReliability wraps a single provider call with circuit breaker + retry.
func (r *Router) callWithReliability(ctx context.Context, p providers.Provider, req types.ChatRequest) (types.ChatResponse, error) {
	cb := r.breakers.Get(p.Name())
	if !cb.Allow() {
		return types.ChatResponse{}, fmt.Errorf("circuit open for provider %q", p.Name())
	}

	var resp types.ChatResponse
	err := reliability.Do(ctx, r.retry, func() error {
		var e error
		resp, e = p.Chat(ctx, req)
		return e
	})
	if err != nil {
		cb.RecordFailure()
		return types.ChatResponse{}, err
	}
	cb.RecordSuccess()
	return resp, nil
}

// Pricing returns the pricing of the active provider.
func (r *Router) Pricing() providers.Pricing {
	r.mu.RLock()
	defer r.mu.RUnlock()
	if p, ok := r.index[r.active]; ok {
		return p.Pricing()
	}
	return providers.Pricing{}
}

// ─── internal helpers ────────────────────────────────────────────────────────

// modelPrefixMap maps well-known model name prefixes to provider names.
// Used by findByHint so that e.g. "claude-3-5-sonnet" resolves to "anthropic".
var modelPrefixMap = map[string]string{
	"claude":   "anthropic",
	"gpt":      "openai",
	"o1":       "openai",
	"o3":       "openai",
	"llama":    "ollama",
	"qwen":     "ollama",
	"mistral":  "ollama",
	"gemma":    "ollama",
	"phi":      "ollama",
	"deepseek": "ollama",
	"codellama": "ollama",
}

// findByHint returns the best provider for the given model/provider hint.
// Checks: (1) exact provider name match, (2) model-prefix map, (3) substring.
func (r *Router) findByHint(hint string) providers.Provider {
	h := strings.ToLower(hint)

	// 1. Exact or direct substring of provider name.
	for _, p := range r.pool {
		pn := strings.ToLower(p.Name())
		if pn == h || strings.Contains(pn, h) || strings.Contains(h, pn) {
			return p
		}
	}

	// 2. Known model-prefix lookup.
	for prefix, providerName := range modelPrefixMap {
		if strings.HasPrefix(h, prefix) {
			if p, ok := r.index[providerName]; ok {
				return p
			}
		}
	}

	return nil
}

// selectForTask finds the best provider for the given task type, excluding any
// provider whose name equals exclude (empty = exclude none).
func (r *Router) selectForTask(task TaskType, exclude string) providers.Provider {
	excludeKey := strings.ToLower(exclude)

	// Find the matching rule for this task type.
	var rule *Rule
	for i := range r.rules {
		if r.rules[i].TaskType == task {
			rule = &r.rules[i]
			break
		}
	}

	if rule != nil {
		// 1. Try preferred model first.
		if rule.PreferModel != "" {
			key := strings.ToLower(rule.PreferModel)
			if p, ok := r.index[key]; ok && key != excludeKey && rule.satisfies(p) {
				return p
			}
		}
		// 2. First pool provider that satisfies the rule.
		for _, p := range r.pool {
			key := strings.ToLower(p.Name())
			if key != excludeKey && rule.satisfies(p) {
				return p
			}
		}
	}

	// 3. Fall back to active provider if it's not excluded.
	if r.active != excludeKey {
		if p, ok := r.index[r.active]; ok {
			return p
		}
	}

	// 4. Any provider that's not excluded.
	for _, p := range r.pool {
		if strings.ToLower(p.Name()) != excludeKey {
			return p
		}
	}
	return nil
}

// fallback tries each provider in pool order, skipping the one that already failed.
func (r *Router) fallback(ctx context.Context, req types.ChatRequest, excludeName string) (types.ChatResponse, error) {
	for _, p := range r.pool {
		if strings.EqualFold(p.Name(), excludeName) {
			continue
		}
		resp, err := p.Chat(ctx, req)
		if err == nil {
			return resp, nil
		}
	}
	return types.ChatResponse{}, fmt.Errorf("router: all providers failed for request")
}
