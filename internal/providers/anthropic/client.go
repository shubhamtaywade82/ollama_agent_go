// Package anthropic implements the Provider interface for the Anthropic Messages API.
package anthropic

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"ollama_agent_go/internal/providers"
	"ollama_agent_go/internal/types"
)

const (
	anthropicVersion    = "2023-06-01"
	anthropicMaxTokens  = 4096
	anthropicDefaultURL = "https://api.anthropic.com/v1"
)

// Client is the Anthropic Messages API adapter.
type Client struct {
	BaseURL    string
	APIKey     string
	Model      string
	MaxTokens  int
	Price      providers.Pricing
	HTTPClient *http.Client
}

// New builds an Anthropic client.
func New(apiKey, model string) *Client {
	return &Client{
		BaseURL:   anthropicDefaultURL,
		APIKey:    apiKey,
		Model:     model,
		MaxTokens: anthropicMaxTokens,
	}
}

func (c *Client) Name() string               { return "anthropic" }
func (c *Client) SupportsTools() bool        { return true }
func (c *Client) SupportsStreaming() bool     { return false }
func (c *Client) SupportsThinking() bool     { return true } // claude-3-7-sonnet+
func (c *Client) Pricing() providers.Pricing { return c.Price }

func (c *Client) httpClient() *http.Client {
	if c.HTTPClient != nil {
		return c.HTTPClient
	}
	return &http.Client{Timeout: 120 * time.Second}
}

// wire types

type antBlock struct {
	Type      string         `json:"type"`
	Text      string         `json:"text,omitempty"`
	ID        string         `json:"id,omitempty"`
	Name      string         `json:"name,omitempty"`
	Input     map[string]any `json:"input,omitempty"`
	ToolUseID string         `json:"tool_use_id,omitempty"`
	Content   string         `json:"content,omitempty"`
}

type antMessage struct {
	Role    string     `json:"role"`
	Content []antBlock `json:"content"`
}

type antTool struct {
	Name        string         `json:"name"`
	Description string         `json:"description"`
	InputSchema map[string]any `json:"input_schema"`
}

type antRequest struct {
	Model     string       `json:"model"`
	MaxTokens int          `json:"max_tokens"`
	System    string       `json:"system,omitempty"`
	Messages  []antMessage `json:"messages"`
	Tools     []antTool    `json:"tools,omitempty"`
}

type antResponse struct {
	Content []antBlock `json:"content"`
	Usage   struct {
		InputTokens  int `json:"input_tokens"`
		OutputTokens int `json:"output_tokens"`
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
	maxTokens := c.MaxTokens
	if maxTokens <= 0 {
		maxTokens = anthropicMaxTokens
	}

	system, messages := toAnthropicMessages(req.Messages)
	areq := antRequest{
		Model:     model,
		MaxTokens: maxTokens,
		System:    system,
		Messages:  messages,
		Tools:     toAnthropicTools(req.Tools),
	}

	body, err := json.Marshal(areq)
	if err != nil {
		return types.ChatResponse{}, err
	}
	httpReq, err := http.NewRequestWithContext(ctx, "POST", c.BaseURL+"/messages", bytes.NewReader(body))
	if err != nil {
		return types.ChatResponse{}, err
	}
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("x-api-key", c.APIKey)
	httpReq.Header.Set("anthropic-version", anthropicVersion)

	resp, err := c.httpClient().Do(httpReq)
	if err != nil {
		return types.ChatResponse{}, err
	}
	defer resp.Body.Close()
	raw, _ := io.ReadAll(resp.Body)

	var aresp antResponse
	if err := json.Unmarshal(raw, &aresp); err != nil {
		return types.ChatResponse{}, fmt.Errorf("anthropic: decode response (status %d): %w", resp.StatusCode, err)
	}
	if resp.StatusCode != http.StatusOK {
		msg := string(raw)
		if aresp.Error != nil {
			msg = aresp.Error.Message
		}
		return types.ChatResponse{}, fmt.Errorf("anthropic error (status %d): %s", resp.StatusCode, msg)
	}

	out := types.ChatResponse{Model: model, Done: true, Message: fromAnthropicBlocks(aresp.Content)}
	out.PromptTokens = aresp.Usage.InputTokens
	out.CompletionTokens = aresp.Usage.OutputTokens
	return out, nil
}

func toAnthropicMessages(msgs []types.Message) (system string, out []antMessage) {
	var sys strings.Builder
	for _, m := range msgs {
		switch m.Role {
		case "system":
			if sys.Len() > 0 {
				sys.WriteString("\n\n")
			}
			sys.WriteString(m.Content)
		case "tool":
			block := antBlock{Type: "tool_result", ToolUseID: m.ToolCallID, Content: m.Content}
			out = append(out, antMessage{Role: "user", Content: []antBlock{block}})
		case "assistant":
			var blocks []antBlock
			if m.Content != "" {
				blocks = append(blocks, antBlock{Type: "text", Text: m.Content})
			}
			for _, tc := range m.ToolCalls {
				blocks = append(blocks, antBlock{
					Type:  "tool_use",
					ID:    tc.ID,
					Name:  tc.Function.Name,
					Input: tc.Function.Arguments,
				})
			}
			out = append(out, antMessage{Role: "assistant", Content: blocks})
		default:
			out = append(out, antMessage{Role: "user", Content: []antBlock{{Type: "text", Text: m.Content}}})
		}
	}
	return sys.String(), out
}

func fromAnthropicBlocks(blocks []antBlock) types.Message {
	m := types.Message{Role: "assistant"}
	var text strings.Builder
	for _, b := range blocks {
		switch b.Type {
		case "text":
			text.WriteString(b.Text)
		case "tool_use":
			m.ToolCalls = append(m.ToolCalls, types.ToolCall{
				ID:       b.ID,
				Function: types.ToolCallFunction{Name: b.Name, Arguments: b.Input},
			})
		}
	}
	m.Content = text.String()
	return m
}

func toAnthropicTools(specs []any) []antTool {
	if len(specs) == 0 {
		return nil
	}
	var out []antTool
	for _, s := range specs {
		b, err := json.Marshal(s)
		if err != nil {
			continue
		}
		var fn struct {
			Function struct {
				Name        string         `json:"name"`
				Description string         `json:"description"`
				Parameters  map[string]any `json:"parameters"`
			} `json:"function"`
		}
		if err := json.Unmarshal(b, &fn); err != nil || fn.Function.Name == "" {
			continue
		}
		out = append(out, antTool{
			Name:        fn.Function.Name,
			Description: fn.Function.Description,
			InputSchema: fn.Function.Parameters,
		})
	}
	return out
}
