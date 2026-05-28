package provider

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"ollama_agent_go/pkg/ollama"
)

// OpenAICompatible is an adapter for any OpenAI /chat/completions compatible
// API — OpenAI itself, Groq, OpenRouter, and most local gateways. They differ
// only by base URL, API key, and model name.
type OpenAICompatible struct {
	ProviderName string
	BaseURL      string // e.g. https://api.openai.com/v1
	APIKey       string
	Model        string
	Price        Pricing
	HTTPClient   *http.Client
}

// NewOpenAI builds an OpenAI provider.
func NewOpenAI(apiKey, model string) *OpenAICompatible {
	return &OpenAICompatible{
		ProviderName: "openai",
		BaseURL:      "https://api.openai.com/v1",
		APIKey:       apiKey,
		Model:        model,
	}
}

// NewGroq builds a Groq provider (OpenAI-compatible).
func NewGroq(apiKey, model string) *OpenAICompatible {
	return &OpenAICompatible{
		ProviderName: "groq",
		BaseURL:      "https://api.groq.com/openai/v1",
		APIKey:       apiKey,
		Model:        model,
	}
}

// NewOpenRouter builds an OpenRouter provider (OpenAI-compatible).
func NewOpenRouter(apiKey, model string) *OpenAICompatible {
	return &OpenAICompatible{
		ProviderName: "openrouter",
		BaseURL:      "https://openrouter.ai/api/v1",
		APIKey:       apiKey,
		Model:        model,
	}
}

func (c *OpenAICompatible) Name() string     { return c.ProviderName }
func (c *OpenAICompatible) Pricing() Pricing { return c.Price }

func (c *OpenAICompatible) httpClient() *http.Client {
	if c.HTTPClient != nil {
		return c.HTTPClient
	}
	return &http.Client{Timeout: 120 * time.Second}
}

// --- wire types ---

type oaiMessage struct {
	Role       string          `json:"role"`
	Content    string          `json:"content"`
	ToolCalls  []oaiToolCall   `json:"tool_calls,omitempty"`
	ToolCallID string          `json:"tool_call_id,omitempty"`
	Name       string          `json:"name,omitempty"`
}

type oaiToolCall struct {
	ID       string          `json:"id"`
	Type     string          `json:"type"`
	Function oaiToolFunction `json:"function"`
}

type oaiToolFunction struct {
	Name      string `json:"name"`
	Arguments string `json:"arguments"` // OpenAI encodes arguments as a JSON string
}

type oaiRequest struct {
	Model    string       `json:"model"`
	Messages []oaiMessage `json:"messages"`
	Tools    []any        `json:"tools,omitempty"`
	Stream   bool         `json:"stream"`
}

type oaiResponse struct {
	Choices []struct {
		Message oaiMessage `json:"message"`
	} `json:"choices"`
	Usage struct {
		PromptTokens     int `json:"prompt_tokens"`
		CompletionTokens int `json:"completion_tokens"`
	} `json:"usage"`
	Error *struct {
		Message string `json:"message"`
	} `json:"error"`
}

// Chat implements Provider.
func (c *OpenAICompatible) Chat(ctx context.Context, req ollama.ChatRequest) (ollama.ChatResponse, error) {
	model := req.Model
	if model == "" {
		model = c.Model
	}

	oreq := oaiRequest{Model: model, Tools: req.Tools, Stream: false}
	for _, m := range req.Messages {
		oreq.Messages = append(oreq.Messages, toOAIMessage(m))
	}

	body, err := json.Marshal(oreq)
	if err != nil {
		return ollama.ChatResponse{}, err
	}
	httpReq, err := http.NewRequestWithContext(ctx, "POST", c.BaseURL+"/chat/completions", bytes.NewReader(body))
	if err != nil {
		return ollama.ChatResponse{}, err
	}
	httpReq.Header.Set("Content-Type", "application/json")
	if c.APIKey != "" {
		httpReq.Header.Set("Authorization", "Bearer "+c.APIKey)
	}

	resp, err := c.httpClient().Do(httpReq)
	if err != nil {
		return ollama.ChatResponse{}, err
	}
	defer resp.Body.Close()
	raw, _ := io.ReadAll(resp.Body)

	var oresp oaiResponse
	if err := json.Unmarshal(raw, &oresp); err != nil {
		return ollama.ChatResponse{}, fmt.Errorf("%s: decode response (status %d): %w", c.ProviderName, resp.StatusCode, err)
	}
	if resp.StatusCode != http.StatusOK {
		msg := string(raw)
		if oresp.Error != nil {
			msg = oresp.Error.Message
		}
		return ollama.ChatResponse{}, fmt.Errorf("%s error (status %d): %s", c.ProviderName, resp.StatusCode, msg)
	}
	if len(oresp.Choices) == 0 {
		return ollama.ChatResponse{}, fmt.Errorf("%s: empty choices", c.ProviderName)
	}

	out := ollama.ChatResponse{
		Model:   model,
		Done:    true,
		Message: fromOAIMessage(oresp.Choices[0].Message),
	}
	out.PromptTokens = oresp.Usage.PromptTokens
	out.CompletionTokens = oresp.Usage.CompletionTokens
	return out, nil
}

func toOAIMessage(m ollama.Message) oaiMessage {
	om := oaiMessage{Role: m.Role, Content: m.Content, ToolCallID: m.ToolCallID, Name: m.ToolName}
	for _, tc := range m.ToolCalls {
		args, _ := json.Marshal(tc.Function.Arguments)
		om.ToolCalls = append(om.ToolCalls, oaiToolCall{
			ID:   tc.ID,
			Type: "function",
			Function: oaiToolFunction{
				Name:      tc.Function.Name,
				Arguments: string(args),
			},
		})
	}
	return om
}

func fromOAIMessage(om oaiMessage) ollama.Message {
	m := ollama.Message{Role: om.Role, Content: om.Content}
	for _, tc := range om.ToolCalls {
		var args map[string]any
		if tc.Function.Arguments != "" {
			_ = json.Unmarshal([]byte(tc.Function.Arguments), &args)
		}
		if args == nil {
			args = map[string]any{}
		}
		m.ToolCalls = append(m.ToolCalls, ollama.ToolCall{
			ID:       tc.ID,
			Function: ollama.ToolCallFunction{Name: tc.Function.Name, Arguments: args},
		})
	}
	return m
}
