// Package ollama implements the Provider interface for the local Ollama API.
package ollama

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"

	"ollama_agent_go/internal/providers"
	"ollama_agent_go/internal/types"
)

// Client is a combined HTTP client and Provider for Ollama.
type Client struct {
	BaseURL    string
	Model      string
	HTTPClient *http.Client
}

// NewClient creates an Ollama client.
func NewClient(baseURL, model string) *Client {
	if baseURL == "" {
		baseURL = "http://localhost:11434"
	}
	return &Client{
		BaseURL:    baseURL,
		Model:      model,
		HTTPClient: &http.Client{},
	}
}

func (c *Client) Name() string            { return "ollama" }
func (c *Client) SupportsTools() bool     { return true }
func (c *Client) SupportsStreaming() bool  { return true }
func (c *Client) Pricing() providers.Pricing { return providers.Pricing{} }

// Chat performs a single non-streaming chat completion.
func (c *Client) Chat(ctx context.Context, req types.ChatRequest) (types.ChatResponse, error) {
	if req.Model == "" {
		req.Model = c.Model
	}
	req.Stream = false
	body, err := json.Marshal(req)
	if err != nil {
		return types.ChatResponse{}, err
	}
	httpReq, err := http.NewRequestWithContext(ctx, "POST", c.BaseURL+"/api/chat", bytes.NewReader(body))
	if err != nil {
		return types.ChatResponse{}, err
	}
	httpReq.Header.Set("Content-Type", "application/json")

	resp, err := c.HTTPClient.Do(httpReq)
	if err != nil {
		return types.ChatResponse{}, err
	}
	defer resp.Body.Close()

	b, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusOK {
		return types.ChatResponse{}, fmt.Errorf("ollama error (status %d): %s", resp.StatusCode, string(b))
	}
	var chatResp types.ChatResponse
	if err := json.Unmarshal(b, &chatResp); err != nil {
		return types.ChatResponse{}, fmt.Errorf("decode response: %w", err)
	}
	return chatResp, nil
}

// ChatStream performs a streaming chat completion, calling callback for each chunk.
func (c *Client) ChatStream(ctx context.Context, req types.ChatRequest, callback func(types.ChatResponse)) error {
	if req.Model == "" {
		req.Model = c.Model
	}
	req.Stream = true
	body, err := json.Marshal(req)
	if err != nil {
		return err
	}
	httpReq, err := http.NewRequestWithContext(ctx, "POST", c.BaseURL+"/api/chat", bytes.NewReader(body))
	if err != nil {
		return err
	}
	httpReq.Header.Set("Content-Type", "application/json")

	resp, err := c.HTTPClient.Do(httpReq)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		b, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("ollama error (status %d): %s", resp.StatusCode, string(b))
	}

	scanner := bufio.NewScanner(resp.Body)
	for scanner.Scan() {
		var chatResp types.ChatResponse
		if err := json.Unmarshal(scanner.Bytes(), &chatResp); err != nil {
			continue
		}
		callback(chatResp)
		if chatResp.Done {
			break
		}
	}
	return scanner.Err()
}

// ListModels fetches the list of locally available Ollama models.
func (c *Client) ListModels(ctx context.Context) ([]types.ModelInfo, error) {
	httpReq, err := http.NewRequestWithContext(ctx, "GET", c.BaseURL+"/api/tags", nil)
	if err != nil {
		return nil, err
	}
	resp, err := c.HTTPClient.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("list models: %w", err)
	}
	defer resp.Body.Close()

	b, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("ollama error (status %d): %s", resp.StatusCode, string(b))
	}
	var result struct {
		Models []types.ModelInfo `json:"models"`
	}
	if err := json.Unmarshal(b, &result); err != nil {
		return nil, fmt.Errorf("decode models: %w", err)
	}
	return result.Models, nil
}
