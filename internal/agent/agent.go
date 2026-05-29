package agent

import (
	"context"
	"fmt"

	"ollama_agent_go/internal/tokens"
	"ollama_agent_go/internal/tools"
	"ollama_agent_go/internal/types"
)

const DefaultMaxIterations = 12

// Chatter is the model backend the agent talks to.
type Chatter interface {
	Chat(ctx context.Context, req types.ChatRequest) (types.ChatResponse, error)
}

// ToolRunner is the interface the agent uses to list and invoke tools.
// Both *tools.Host and *tools.ScopedHost satisfy this interface.
type ToolRunner interface {
	Specs() []tools.Spec
	Execute(ctx context.Context, name string, args map[string]any) (string, error)
}

// Runner is the interface the engine uses to invoke an agent run.
type Runner interface {
	Run(ctx context.Context, history []types.Message, emit Emit) (string, error)
}

// Agent runs the think-act loop against a model backend with a tool host.
type Agent struct {
	Client        Chatter
	ToolHost      ToolRunner // *tools.Host or *tools.ScopedHost
	Model         string
	ModelHint     string // optional provider-selection hint propagated to the router
	System        string
	MaxIterations int
	Options       map[string]any
	ContextBudget int

	// InterruptCheck is called at the start of each loop iteration.
	// If it returns false the run is aborted (HITL interrupt or policy enforcement).
	// Nil = no check.
	InterruptCheck func() bool
}

// New builds an Agent with sensible defaults.
// host is stored as-is; if it also satisfies ToolNamer, a tool list is
// appended to the system prompt automatically.
func New(client Chatter, host ToolRunner, model string) *Agent {
	ag := &Agent{
		Client:        client,
		ToolHost:      host,
		Model:         model,
		MaxIterations: DefaultMaxIterations,
	}
	if namer, ok := host.(ToolNamer); ok {
		ag.System = SystemPrompt(namer)
	}
	return ag
}

// Run executes the loop over the given conversation history. It returns the
// final assistant text and streams events via emit as work progresses.
func (a *Agent) Run(ctx context.Context, history []types.Message, emit Emit) (string, error) {
	if emit == nil {
		emit = func(Event) {}
	}
	maxIters := a.MaxIterations
	if maxIters <= 0 {
		maxIters = DefaultMaxIterations
	}

	msgs := make([]types.Message, 0, len(history)+1)
	if a.System != "" {
		msgs = append(msgs, types.Message{Role: "system", Content: a.System})
	}
	msgs = append(msgs, history...)

	specs := toAnySpecs(a.ToolHost.Specs())

	for iter := 0; iter < maxIters; iter++ {
		if err := ctx.Err(); err != nil {
			return "", err
		}
		if a.InterruptCheck != nil && !a.InterruptCheck() {
			err := fmt.Errorf("run aborted by interrupt check")
			emit(Event{Kind: EventError, Text: err.Error()})
			return "", err
		}
		resp, err := a.Client.Chat(ctx, types.ChatRequest{
			Model:     a.Model,
			ModelHint: a.ModelHint,
			Messages:  tokens.Trim(msgs, a.ContextBudget),
			Tools:     specs,
			Options:   a.Options,
		})
		if err != nil {
			emit(Event{Kind: EventError, Text: err.Error()})
			return "", err
		}

		msg := resp.Message
		calls := msg.ToolCalls
		if len(calls) == 0 {
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

		msgs = append(msgs, types.Message{
			Role:      "assistant",
			Content:   msg.Content,
			ToolCalls: calls,
		})

		for _, call := range calls {
			name := call.Function.Name
			args := call.Function.Arguments
			emit(Event{Kind: EventToolCall, Tool: name, Args: args})

			out, execErr := a.ToolHost.Execute(ctx, name, args)
			result := out
			if execErr != nil {
				result = "Error: " + execErr.Error()
			}
			emit(Event{Kind: EventToolResult, Tool: name, Text: result})

			msgs = append(msgs, types.Message{
				Role:       "tool",
				ToolName:   name,
				ToolCallID: call.ID,
				Content:    result,
			})
		}
	}

	err := fmt.Errorf("reached max iterations (%d) without a final answer", maxIters)
	emit(Event{Kind: EventError, Text: err.Error()})
	return "", err
}

func toAnySpecs(specs []tools.Spec) []any {
	out := make([]any, len(specs))
	for i, s := range specs {
		out[i] = s
	}
	return out
}
