// Package providers defines the chat backend interface and cost tracking types
// shared across all provider implementations.
package providers

import (
	"context"

	"ollama_agent_go/internal/types"
)

// Provider is a chat backend. All providers implement this interface so the
// agent loop and router can use them interchangeably.
type Provider interface {
	Name() string
	Chat(ctx context.Context, req types.ChatRequest) (types.ChatResponse, error)
	SupportsTools() bool
	SupportsStreaming() bool
	Pricing() Pricing
}

// Pricing is the cost per one million tokens, in USD.
type Pricing struct {
	InputPerM  float64
	OutputPerM float64
}

// Cost computes the dollar cost for a given token usage.
func (p Pricing) Cost(inputTokens, outputTokens int) float64 {
	return float64(inputTokens)/1e6*p.InputPerM + float64(outputTokens)/1e6*p.OutputPerM
}
