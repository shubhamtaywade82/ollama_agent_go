// Package openai implements the Provider interface for OpenAI-compatible APIs
// (OpenAI, Groq, OpenRouter, and any /chat/completions compatible endpoint).
package openai

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"ollama_agent_go/internal/providers"
	"ollama_agent_go/internal/types"
)

// Client is an OpenAI-compatible chat provider.
type Client struct {
	ProviderName string
	BaseURL      string
	APIKey       string
	Model        string
	Price        providers.Pricing
	HTTPClient   *http.Client
}

// NewOpenAI builds an OpenAI provider.
func NewOpenAI(apiKey, model string) *Client {
	return &Client{ProviderName: "openai", BaseURL: "https://api.openai.com/v1", APIKey: apiKey, Model: model}
}

// NewGroq builds a Groq provider.
func NewGroq(apiKey, model string) *Client {
	return &Client{ProviderName: "groq", BaseURL: "https://api.groq.com/openai/v1", APIKey: apiKey, Model: model}
}

// NewOpenRouter builds an OpenRouter provider.
func NewOpenRouter(apiKey, model string) *Client {
	return &Client{ProviderName: "openrouter", BaseURL: "https://openrouter.ai/api/v1", APIKey: apiKey, Model: model}
}

func (c *Client) Name() string               { return c.ProviderName }
func (c *Client) SupportsTools() bool        { return true }
func (c *Client) SupportsStreaming() bool     { return false }
func (c *Client) Pricing() providers.Pricing { return c.Price }

func (c *Client) httpClient() *http.Client {
	if c.HTTPClient != nil {
		return c.HTTPClient
	}
	return &http.Client{Timeout: 120 * time.Second}
}

// wire types

type oaiMessage struct {
	Role       string       `json:"role"`
	Content    string       `json:"content"`
	ToolCalls  []oaiCall    `json:"tool_calls,omitempty"`
	ToolCallID string       `json:"tool_call_id,omitempty"`
	Name       string       `json:"name,omitempty"`
}

type oaiCall struct {
	ID       string      `json:"id"`
	Type     string      `json:"type"`
	Function oaiFunction `json:"function"`
}

type oaiFunction struct {
	Name      string `json:"name"`
	Arguments string `json:"arguments"` // JSON-encoded string
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
func (c *Client) Chat(ctx context.Context, req types.ChatRequest) (types.ChatResponse, error) {
	model := req.Model
	if model == "" {
		model = c.Model
	}

	oreq := oaiRequest{Model: model, Tools: req.Tools, Stream: false}
	for _, m := range req.Messages {
		oreq.Messages = append(oreq.Messages, toOAI(m))
	}

	body, err := json.Marshal(oreq)
	if err != nil {
		return types.ChatResponse{}, err
	}
	httpReq, err := http.NewRequestWithContext(ctx, "POST", c.BaseURL+"/chat/completions", bytes.NewReader(body))
	if err != nil {
		return types.ChatResponse{}, err
	}
	httpReq.Header.Set("Content-Type", "application/json")
	if c.APIKey != "" {
		httpReq.Header.Set("Authorization", "Bearer "+c.APIKey)
	}

	resp, err := c.httpClient().Do(httpReq)
	if err != nil {
		return types.ChatResponse{}, err
	}
	defer resp.Body.Close()
	raw, _ := io.ReadAll(resp.Body)

	var oresp oaiResponse
	if err := json.Unmarshal(raw, &oresp); err != nil {
		return types.ChatResponse{}, fmt.Errorf("%s: decode response (status %d): %w", c.ProviderName, resp.StatusCode, err)
	}
	if resp.StatusCode != http.StatusOK {
		msg := string(raw)
		if oresp.Error != nil {
			msg = oresp.Error.Message
		}
		return types.ChatResponse{}, fmt.Errorf("%s error (status %d): %s", c.ProviderName, resp.StatusCode, msg)
	}
	if len(oresp.Choices) == 0 {
		return types.ChatResponse{}, fmt.Errorf("%s: empty choices", c.ProviderName)
	}

	out := types.ChatResponse{
		Model:   model,
		Done:    true,
		Message: fromOAI(oresp.Choices[0].Message),
	}
	out.PromptTokens = oresp.Usage.PromptTokens
	out.CompletionTokens = oresp.Usage.CompletionTokens
	return out, nil
}

func toOAI(m types.Message) oaiMessage {
	om := oaiMessage{Role: m.Role, Content: m.Content, ToolCallID: m.ToolCallID, Name: m.ToolName}
	for _, tc := range m.ToolCalls {
		args, _ := json.Marshal(tc.Function.Arguments)
		om.ToolCalls = append(om.ToolCalls, oaiCall{
			ID:   tc.ID,
			Type: "function",
			Function: oaiFunction{
				Name:      tc.Function.Name,
				Arguments: string(args),
			},
		})
	}
	return om
}

func fromOAI(om oaiMessage) types.Message {
	m := types.Message{Role: om.Role, Content: om.Content}
	for _, tc := range om.ToolCalls {
		var args map[string]any
		if tc.Function.Arguments != "" {
			_ = json.Unmarshal([]byte(tc.Function.Arguments), &args)
		}
		if args == nil {
			args = map[string]any{}
		}
		m.ToolCalls = append(m.ToolCalls, types.ToolCall{
			ID:       tc.ID,
			Function: types.ToolCallFunction{Name: tc.Function.Name, Arguments: args},
		})
	}
	return m
}
