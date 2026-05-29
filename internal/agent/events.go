// Package agent implements the think-act loop.
package agent

// EventKind classifies an Event emitted during a run.
type EventKind int

const (
	// EventToken is a chunk of the model's final answer text.
	EventToken EventKind = iota
	// EventToolCall is emitted just before a tool runs.
	EventToolCall
	// EventToolResult is emitted with a tool's output (or error text).
	EventToolResult
	// EventError signals a fatal error; the run stops after it.
	EventError
	// EventDone signals the run finished successfully.
	EventDone
)

// Event is a reactive update from a run, consumed by the UI.
type Event struct {
	Kind EventKind
	Text string         // token text, tool output, or error message
	Tool string         // tool name (for tool events)
	Args map[string]any // tool arguments (for EventToolCall)
}

// Emit is the callback the agent uses to stream events to a consumer.
type Emit func(Event)
