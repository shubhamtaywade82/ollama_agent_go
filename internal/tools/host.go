package tools

import (
	"context"
	"fmt"
	"time"

	"ollama_agent_go/internal/observability"
	"ollama_agent_go/internal/policy"
)

// Host wraps a Registry with policy enforcement and observability. It is the
// single execution path for all tool calls; neither the agent nor the TUI
// should call Registry.Execute directly.
type Host struct {
	Registry *Registry
	Policy   *policy.DefaultEngine
	Obs      observability.Logger
	// SessionID is set by the runtime engine before each agent run so that
	// tool observability records are associated with the correct session.
	SessionID string
}

// NewHost builds a Host with the given components.
func NewHost(reg *Registry, pol *policy.DefaultEngine, obs observability.Logger) *Host {
	return &Host{Registry: reg, Policy: pol, Obs: obs}
}

// Names delegates to the underlying registry.
func (h *Host) Names() []string { return h.Registry.Names() }

// Get delegates to the underlying registry.
func (h *Host) Get(name string) (Tool, bool) { return h.Registry.Get(name) }

// Specs delegates to the underlying registry.
func (h *Host) Specs() []Spec { return h.Registry.Specs() }

// Execute enforces policy, records observability, and delegates to the registry.
func (h *Host) Execute(ctx context.Context, name string, args map[string]any) (string, error) {
	if !h.Policy.Enforce(ctx, name, args) {
		return "", fmt.Errorf("tool %q was denied", name)
	}

	start := time.Now()
	out, err := h.Registry.Execute(ctx, name, args)
	dur := time.Since(start)

	h.Obs.RecordToolCall(h.SessionID, name, dur, err)
	return out, err
}
