package provider

import (
	"context"

	"ollama_agent_go/pkg/ollama"
)

// Ollama is an adapter for the local Ollama API.
type Ollama struct {
	Client *ollama.Client
	Model  string
}

func NewOllama(baseURL, model string) *Ollama {
	return &Ollama{
		Client: ollama.NewClient(baseURL),
		Model:  model,
	}
}

func (o *Ollama) Name() string { return "ollama" }

func (o *Ollama) Chat(ctx context.Context, req ollama.ChatRequest) (ollama.ChatResponse, error) {
	if req.Model == "" {
		req.Model = o.Model
	}
	return o.Client.Chat(ctx, req)
}

func (o *Ollama) Pricing() Pricing {
	// Ollama is free (local).
	return Pricing{InputPerM: 0, OutputPerM: 0}
}
