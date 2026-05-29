package mcp

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"ollama_agent_go/internal/tools"
)

// MCPToolAdapter wraps an MCP ToolDef + Client as a tools.Tool.
type MCPToolAdapter struct {
	client  *Client
	toolDef ToolDef
}

func (a *MCPToolAdapter) Name() string              { return a.toolDef.Name }
func (a *MCPToolAdapter) Description() string        { return a.toolDef.Description }
func (a *MCPToolAdapter) Parameters() map[string]any { return a.rawSchema() }

func (a *MCPToolAdapter) Execute(ctx context.Context, args map[string]any) (string, error) {
	result, err := a.client.CallTool(ctx, CallToolRequest{
		Name:      a.toolDef.Name,
		Arguments: args,
	})
	if err != nil {
		return "", err
	}
	if result.IsError {
		var parts []string
		for _, c := range result.Content {
			if c.Text != "" {
				parts = append(parts, c.Text)
			}
		}
		return "", fmt.Errorf("mcp tool error: %s", strings.Join(parts, "; "))
	}
	var texts []string
	for _, c := range result.Content {
		if c.Type == "text" && c.Text != "" {
			texts = append(texts, c.Text)
		}
	}
	return strings.Join(texts, "\n"), nil
}

func (a *MCPToolAdapter) rawSchema() map[string]any {
	var m map[string]any
	_ = json.Unmarshal(a.toolDef.InputSchema, &m)
	return m
}

// AdaptAll returns one MCPToolAdapter per tool the MCP server exposes.
// The client must have been initialized before calling AdaptAll.
func AdaptAll(client *Client) []tools.Tool {
	defs := client.Tools()
	out := make([]tools.Tool, 0, len(defs))
	for i := range defs {
		out = append(out, &MCPToolAdapter{client: client, toolDef: defs[i]})
	}
	return out
}

// ConnectServer connects to an MCP server described by cfg and initializes it.
// Returns the ready-to-use Client.
func ConnectServer(ctx context.Context, cfg ServerConfig) (*Client, error) {
	var transport Transport
	switch cfg.Transport {
	case "stdio":
		t, err := NewStdioTransport(cfg.Command, cfg.Args...)
		if err != nil {
			return nil, err
		}
		transport = t
	case "http":
		transport = NewHTTPTransport(cfg.URL)
	default:
		return nil, fmt.Errorf("mcp: unknown transport %q", cfg.Transport)
	}
	c := NewClient(transport)
	if err := c.Initialize(ctx); err != nil {
		_ = c.Close()
		return nil, err
	}
	return c, nil
}

// ServerConfig describes an external MCP server to connect to.
type ServerConfig struct {
	Name      string   `json:"name"`
	Transport string   `json:"transport"` // "stdio" | "http"
	Command   string   `json:"command"`   // for stdio
	Args      []string `json:"args"`
	URL       string   `json:"url"` // for http
}
