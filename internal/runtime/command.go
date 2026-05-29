package runtime

import "time"

// ToolInvocation records the inputs of a single tool call.
type ToolInvocation struct {
	ID        string
	Tool      string
	Args      map[string]any
	StartedAt time.Time
}

// CommandResult records the outcome of a single tool call.
type CommandResult struct {
	Invocation ToolInvocation
	Output     string
	Error      error
	Duration   time.Duration
}
