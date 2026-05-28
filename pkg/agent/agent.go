// Package agent implements the think-act loop: it sends the conversation plus
// available tool specs to the model, executes any tool calls the model
// requests (native JSON tool_calls or synthetic markdown code blocks), feeds
// results back, and repeats until the model produces a final answer.
package agent

import (
	"context"
	"fmt"

	"ollama_agent_go/pkg/ollama"
	"ollama_agent_go/pkg/tokens"
	"ollama_agent_go/pkg/tools"
)

// DefaultMaxIterations bounds the think-act loop so a misbehaving model cannot
// call tools forever.
const DefaultMaxIterations = 12

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

// Chatter is the model backend the agent talks to. *ollama.Client implements
// it; tests provide fakes.
type Chatter interface {
	Chat(ctx context.Context, req ollama.ChatRequest) (ollama.ChatResponse, error)
}

// Agent runs the think-act loop against a model backend with a tool registry.
type Agent struct {
	Client        Chatter
	Registry      *tools.Registry
	Model         string
	System        string
	MaxIterations int
	Options       map[string]any
	// ContextBudget caps the tokens sent per request via sliding-window
	// trimming. Zero disables trimming.
	ContextBudget int
}

// New builds an Agent with sensible defaults.
func New(client Chatter, registry *tools.Registry, model string) *Agent {
	return &Agent{
		Client:        client,
		Registry:      registry,
		Model:         model,
		System:        SystemPrompt(registry),
		MaxIterations: DefaultMaxIterations,
	}
}

// Run executes the loop over the given conversation history (user/assistant
// messages, no system message — the agent prepends its own). It returns the
// final assistant text. Events are emitted via emit as work progresses.
func (a *Agent) Run(ctx context.Context, history []ollama.Message, emit Emit) (string, error) {
	if emit == nil {
		emit = func(Event) {}
	}
	maxIters := a.MaxIterations
	if maxIters <= 0 {
		maxIters = DefaultMaxIterations
	}

	msgs := make([]ollama.Message, 0, len(history)+1)
	if a.System != "" {
		msgs = append(msgs, ollama.Message{Role: "system", Content: a.System})
	}
	msgs = append(msgs, history...)

	specs := toAnySpecs(a.Registry.Specs())

	for iter := 0; iter < maxIters; iter++ {
		if err := ctx.Err(); err != nil {
			return "", err
		}
		resp, err := a.Client.Chat(ctx, ollama.ChatRequest{
			Model:    a.Model,
			Messages: tokens.Trim(msgs, a.ContextBudget),
			Tools:    specs,
			Options:  a.Options,
		})
		if err != nil {
			emit(Event{Kind: EventError, Text: err.Error()})
			return "", err
		}

		msg := resp.Message
		calls := msg.ToolCalls
		if len(calls) == 0 {
			// Fall back to parsing synthetic tool calls from the content for
			// models that lack native tool support.
			if synthetic, rest := parseSyntheticCalls(msg.Content); len(synthetic) > 0 {
				calls = synthetic
				msg.Content = rest
			}
		}

		if len(calls) == 0 {
			emit(Event{Kind: EventToken, Text: msg.Content})
			emit(Event{Kind: EventDone})
			return msg.Content, nil
		}

		// Record the assistant turn that requested the tools.
		msgs = append(msgs, ollama.Message{
			Role:      "assistant",
			Content:   msg.Content,
			ToolCalls: calls,
		})

		for _, call := range calls {
			name := call.Function.Name
			args := call.Function.Arguments
			emit(Event{Kind: EventToolCall, Tool: name, Args: args})

			out, execErr := a.Registry.Execute(ctx, name, args)
			result := out
			if execErr != nil {
				result = "Error: " + execErr.Error()
			}
			emit(Event{Kind: EventToolResult, Tool: name, Text: result})

			msgs = append(msgs, ollama.Message{
				Role:     "tool",
				ToolName: name,
				Content:  result,
			})
		}
	}

	err := fmt.Errorf("reached max iterations (%d) without a final answer", maxIters)
	emit(Event{Kind: EventError, Text: err.Error()})
	return "", err
}

// toAnySpecs converts typed specs to the []any the request expects.
func toAnySpecs(specs []tools.Spec) []any {
	out := make([]any, len(specs))
	for i, s := range specs {
		out[i] = s
	}
	return out
}
