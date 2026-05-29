package orchestration

import (
	"context"
	"fmt"
	"strings"

	"ollama_agent_go/internal/providers"
	"ollama_agent_go/internal/types"
)

// Aggregator combines the results of all tasks into a final coherent response.
type Aggregator struct {
	provider providers.Provider
	model    string
}

// NewAggregator creates an Aggregator backed by the given provider/model.
func NewAggregator(p providers.Provider, model string) *Aggregator {
	return &Aggregator{provider: p, model: model}
}

const aggregatorSystemPrompt = `You are a synthesis assistant. Combine the provided sub-task
results into a clear, coherent final answer for the original goal. Be concise and structured.`

// Aggregate calls the LLM to synthesize task results into a final response.
func (a *Aggregator) Aggregate(ctx context.Context, goal string, tasks []*Task) (string, error) {
	var sb strings.Builder
	fmt.Fprintf(&sb, "Original goal: %s\n\nSub-task results:\n", goal)
	for _, t := range tasks {
		if t.Status == TaskDone && t.Result != "" {
			fmt.Fprintf(&sb, "\n### %s (%s)\n%s\n", t.ID, t.Role.String(), t.Result)
		}
	}

	req := types.ChatRequest{
		Model: a.model,
		Messages: []types.Message{
			{Role: "system", Content: aggregatorSystemPrompt},
			{Role: "user", Content: sb.String()},
		},
	}

	resp, err := a.provider.Chat(ctx, req)
	if err != nil {
		return "", fmt.Errorf("aggregator: %w", err)
	}
	return resp.Message.Content, nil
}
