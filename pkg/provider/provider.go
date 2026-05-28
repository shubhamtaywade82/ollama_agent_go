// Package provider implements chat backends for multiple LLM vendors behind a
// single interface, plus a cost-aware router that falls back across them.
//
// The neutral request/response contract reuses the types in pkg/ollama, which
// already mirror the OpenAI chat schema (roles, tool_calls, tool results).
// Adapters translate to and from each vendor's wire format.
package provider

import (
	"context"

	"ollama_agent_go/pkg/ollama"
)

// Provider is a chat backend. It satisfies agent.Chatter via the Chat method,
// so any Provider can drive the agent loop.
type Provider interface {
	// Name is a short identifier, e.g. "ollama", "openai", "anthropic".
	Name() string
	// Chat performs one non-streaming completion.
	Chat(ctx context.Context, req ollama.ChatRequest) (ollama.ChatResponse, error)
	// Pricing returns the per-million-token cost for the active model, used for
	// cost tracking. Zero means free/unknown.
	Pricing() Pricing
}

// Pricing is the cost per one million tokens, in USD.
type Pricing struct {
	InputPerM  float64
	OutputPerM float64
}

// Cost computes the dollar cost for a token usage given this pricing.
func (p Pricing) Cost(inputTokens, outputTokens int) float64 {
	return float64(inputTokens)/1e6*p.InputPerM + float64(outputTokens)/1e6*p.OutputPerM
}
