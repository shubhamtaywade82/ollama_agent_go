package provider

import (
	"context"
	"fmt"
	"strings"

	"ollama_agent_go/pkg/ollama"
)

// Router manages multiple providers and routes chat requests to the active one.
// It also supports fallback logic if a provider fails.
type Router struct {
	providers map[string]Provider
	active    string
}

func NewRouter() *Router {
	return &Router{
		providers: make(map[string]Provider),
	}
}

func (r *Router) Add(p Provider) {
	r.providers[strings.ToLower(p.Name())] = p
	if r.active == "" {
		r.active = strings.ToLower(p.Name())
	}
}

func (r *Router) SetActive(name string) error {
	name = strings.ToLower(name)
	if _, ok := r.providers[name]; !ok {
		return fmt.Errorf("provider %q not found", name)
	}
	r.active = name
	return nil
}

func (r *Router) ActiveName() string {
	return r.active
}

func (r *Router) Names() []string {
	var names []string
	for name := range r.providers {
		names = append(names, name)
	}
	return names
}

func (r *Router) Get(name string) (Provider, bool) {
	p, ok := r.providers[strings.ToLower(name)]
	return p, ok
}

// Chat satisfies agent.Chatter. It routes the request to the active provider.
func (r *Router) Chat(ctx context.Context, req ollama.ChatRequest) (ollama.ChatResponse, error) {
	p, ok := r.providers[r.active]
	if !ok {
		return ollama.ChatResponse{}, fmt.Errorf("no active provider")
	}
	return p.Chat(ctx, req)
}

// Pricing returns pricing for the active provider.
func (r *Router) Pricing() Pricing {
	if p, ok := r.providers[r.active]; ok {
		return p.Pricing()
	}
	return Pricing{}
}
