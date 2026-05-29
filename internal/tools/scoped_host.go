package tools

import (
	"context"
	"fmt"
)

// ScopedHost wraps a Host and restricts Execute to an allowed set of tools.
// If allowed is nil, all tools are permitted (same as the underlying Host).
type ScopedHost struct {
	*Host
	allowed map[string]bool
}

// NewScopedHost creates a ScopedHost. allowedTools nil = all tools allowed.
func NewScopedHost(h *Host, allowedTools []string) *ScopedHost {
	var allowed map[string]bool
	if allowedTools != nil {
		allowed = make(map[string]bool, len(allowedTools))
		for _, t := range allowedTools {
			allowed[t] = true
		}
	}
	return &ScopedHost{Host: h, allowed: allowed}
}

// Execute checks the allowed list before delegating to the underlying Host.
func (s *ScopedHost) Execute(ctx context.Context, name string, args map[string]any) (string, error) {
	if s.allowed != nil && !s.allowed[name] {
		return "", fmt.Errorf("tool %q not available for this agent role", name)
	}
	return s.Host.Execute(ctx, name, args)
}

// Specs returns only the specs for allowed tools (nil allowed = all specs).
func (s *ScopedHost) Specs() []Spec {
	all := s.Host.Specs()
	if s.allowed == nil {
		return all
	}
	out := make([]Spec, 0, len(s.allowed))
	for _, sp := range all {
		if s.allowed[sp.Function.Name] {
			out = append(out, sp)
		}
	}
	return out
}
