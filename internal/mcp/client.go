package mcp

import (
	"context"
	"encoding/json"
	"fmt"
	"sync/atomic"
)

// Client connects to an MCP server and exposes its tools.
type Client struct {
	transport Transport
	name      string
	tools     []ToolDef
	nextID    atomic.Int64
}

// NewClient wraps transport in an MCP client.
func NewClient(transport Transport) *Client {
	return &Client{transport: transport}
}

// Initialize performs the MCP handshake and populates the tool list.
func (c *Client) Initialize(ctx context.Context) error {
	params := InitializeParams{
		ProtocolVersion: ProtocolVersion,
		Capabilities:    Capabilities{Tools: &ToolsCapability{}},
		ClientInfo:      ServerInfo{Name: "ollama-agent", Version: "1.0"},
	}
	resp, err := c.call(ctx, "initialize", params)
	if err != nil {
		return fmt.Errorf("mcp initialize: %w", err)
	}
	var result InitializeResult
	if err := json.Unmarshal(resp.Result, &result); err != nil {
		return fmt.Errorf("mcp initialize result: %w", err)
	}
	c.name = result.ServerInfo.Name

	// Acknowledge initialization.
	notif, _ := json.Marshal(Notification{JSONRPC: "2.0", Method: "notifications/initialized"})
	_ = c.transport.Send(ctx, notif)

	// Populate tool list.
	tools, err := c.ListTools(ctx)
	if err != nil {
		return fmt.Errorf("mcp list tools: %w", err)
	}
	c.tools = tools
	return nil
}

// ListTools fetches the tool list from the server.
func (c *Client) ListTools(ctx context.Context) ([]ToolDef, error) {
	resp, err := c.call(ctx, "tools/list", nil)
	if err != nil {
		return nil, err
	}
	var result ToolsListResult
	if err := json.Unmarshal(resp.Result, &result); err != nil {
		return nil, err
	}
	return result.Tools, nil
}

// CallTool invokes a tool on the MCP server.
func (c *Client) CallTool(ctx context.Context, req CallToolRequest) (CallToolResult, error) {
	resp, err := c.call(ctx, "tools/call", req)
	if err != nil {
		return CallToolResult{}, err
	}
	var result CallToolResult
	if err := json.Unmarshal(resp.Result, &result); err != nil {
		return CallToolResult{}, err
	}
	return result, nil
}

// Tools returns the cached tool definitions after Initialize().
func (c *Client) Tools() []ToolDef { return c.tools }

// Name returns the server's self-reported name.
func (c *Client) Name() string { return c.name }

// Close tears down the transport.
func (c *Client) Close() error { return c.transport.Close() }

// call sends a JSON-RPC request and waits for the matching response.
func (c *Client) call(ctx context.Context, method string, params any) (Response, error) {
	id := c.nextID.Add(1)
	data, err := marshalRequest(id, method, params)
	if err != nil {
		return Response{}, err
	}
	if err := c.transport.Send(ctx, data); err != nil {
		return Response{}, fmt.Errorf("mcp send %s: %w", method, err)
	}
	raw, err := c.transport.Recv(ctx)
	if err != nil {
		return Response{}, fmt.Errorf("mcp recv %s: %w", method, err)
	}
	var resp Response
	if err := json.Unmarshal(raw, &resp); err != nil {
		return Response{}, fmt.Errorf("mcp decode %s: %w", method, err)
	}
	if resp.Error != nil {
		return Response{}, resp.Error
	}
	return resp, nil
}
