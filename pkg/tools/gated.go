package tools

import (
	"context"
	"fmt"
)

// ApprovalFunc is called before a high-risk tool runs. Return true to allow,
// false to deny. The function receives the tool name and its arguments.
// A nil ApprovalFunc allows all tools unconditionally.
type ApprovalFunc func(name string, args map[string]any) bool

// GatedRegistry wraps a Registry and intercepts Execute calls for listed tools,
// routing them through an approval callback before execution.
type GatedRegistry struct {
	*Registry
	gated    map[string]bool
	approval ApprovalFunc
}

// NewGatedRegistry wraps r with an approval gate for the named tools.
func NewGatedRegistry(r *Registry, gatedTools []string, approval ApprovalFunc) *GatedRegistry {
	gated := make(map[string]bool, len(gatedTools))
	for _, name := range gatedTools {
		gated[name] = true
	}
	return &GatedRegistry{Registry: r, gated: gated, approval: approval}
}

// Execute runs the tool after approval (if required).
func (g *GatedRegistry) Execute(ctx context.Context, name string, args map[string]any) (string, error) {
	if g.gated[name] && g.approval != nil {
		if !g.approval(name, args) {
			return "", fmt.Errorf("tool %q was denied by user", name)
		}
	}
	return g.Registry.Execute(ctx, name, args)
}
