// Package router manages multiple providers and routes requests to the active one.
package router

import (
	"context"
	"fmt"
	"strings"

	"ollama_agent_go/internal/providers"
	"ollama_agent_go/internal/types"
)

// Router manages multiple providers and routes chat requests to the active one.
type Router struct {
	registered map[string]providers.Provider
	active     string
}

// New creates an empty router.
func New() *Router {
	return &Router{registered: make(map[string]providers.Provider)}
}

// Add registers a provider. The first added becomes the active provider.
func (r *Router) Add(p providers.Provider) {
	key := strings.ToLower(p.Name())
	r.registered[key] = p
	if r.active == "" {
		r.active = key
	}
}

// SetActive switches the active provider by name. Returns an error if the name
// is not registered.
func (r *Router) SetActive(name string) error {
	name = strings.ToLower(name)
	if _, ok := r.registered[name]; !ok {
		return fmt.Errorf("provider %q not found", name)
	}
	r.active = name
	return nil
}

// Active returns the name of the currently active provider.
func (r *Router) Active() string { return r.active }

// Names returns the names of all registered providers.
func (r *Router) Names() []string {
	out := make([]string, 0, len(r.registered))
	for name := range r.registered {
		out = append(out, name)
	}
	return out
}

// Get returns a provider by name.
func (r *Router) Get(name string) (providers.Provider, bool) {
	p, ok := r.registered[strings.ToLower(name)]
	return p, ok
}

// Chat satisfies the agent.Chatter interface. Routes to the active provider.
func (r *Router) Chat(ctx context.Context, req types.ChatRequest) (types.ChatResponse, error) {
	p, ok := r.registered[r.active]
	if !ok {
		return types.ChatResponse{}, fmt.Errorf("no active provider")
	}
	return p.Chat(ctx, req)
}

// Pricing returns pricing for the active provider.
func (r *Router) Pricing() providers.Pricing {
	if p, ok := r.registered[r.active]; ok {
		return p.Pricing()
	}
	return providers.Pricing{}
}
