// Package policy implements the approval and guardrail layer. All tool
// executions pass through the policy engine before running, which allows
// centralised enforcement of rules independent of the TUI or tool executor.
package policy

import "context"

// Decision is the outcome of a policy check.
type Decision int

const (
	// Allow permits the tool to run immediately.
	Allow Decision = iota
	// Deny blocks the tool unconditionally.
	Deny
	// RequireApproval pauses execution and calls the registered callback.
	RequireApproval
)

// ApprovalFunc is called when a tool requires human approval. Return true to
// allow, false to deny.
type ApprovalFunc func(tool string, args map[string]any) bool

// Engine evaluates policy rules for tool executions.
type Engine interface {
	Check(ctx context.Context, tool string, args map[string]any) (Decision, error)
	// RegisterApprovalCallback sets the function invoked for RequireApproval decisions.
	RegisterApprovalCallback(cb ApprovalFunc)
}

// DefaultEngine checks a configurable set of gated tool names and delegates
// approval decisions to a registered callback.
type DefaultEngine struct {
	gatedTools  map[string]bool
	sandboxRoot string
	approval    ApprovalFunc
	noApproval  bool // when true, all tools are allowed without prompting
}

// NewDefaultEngine creates a policy engine that gates the listed tool names.
func NewDefaultEngine(sandboxRoot string, gatedTools []string, noApproval bool) *DefaultEngine {
	gated := make(map[string]bool, len(gatedTools))
	for _, name := range gatedTools {
		gated[name] = true
	}
	return &DefaultEngine{
		gatedTools:  gated,
		sandboxRoot: sandboxRoot,
		noApproval:  noApproval,
	}
}

// RegisterApprovalCallback wires the UI approval gate into the policy engine.
func (e *DefaultEngine) RegisterApprovalCallback(cb ApprovalFunc) {
	e.approval = cb
}

// Check evaluates the policy for a tool call and returns the decision.
func (e *DefaultEngine) Check(_ context.Context, tool string, args map[string]any) (Decision, error) {
	if e.noApproval {
		return Allow, nil
	}
	if e.gatedTools[tool] {
		return RequireApproval, nil
	}
	return Allow, nil
}

// Enforce calls Check and, for RequireApproval, invokes the registered callback.
// Returns true if the tool should proceed, false if it should be blocked.
func (e *DefaultEngine) Enforce(ctx context.Context, tool string, args map[string]any) bool {
	decision, _ := e.Check(ctx, tool, args)
	switch decision {
	case Allow:
		return true
	case Deny:
		return false
	case RequireApproval:
		if e.approval != nil {
			return e.approval(tool, args)
		}
		return false
	}
	return false
}
