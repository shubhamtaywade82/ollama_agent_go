package mcp_test

import (
	"context"
	"encoding/json"
	"testing"

	"ollama_agent_go/internal/mcp"
)

// mcpServerSim drives the server side of an in-proc transport pair,
// responding to initialize and tools/list in a goroutine.
func startMockServer(t *testing.T) (*mcp.Client, context.CancelFunc) {
	t.Helper()
	clientT, serverT := mcp.NewInProcPair()
	ctx, cancel := context.WithCancel(context.Background())

	fakeTool := mcp.ToolDef{
		Name:        "echo",
		Description: "echoes the input",
		InputSchema: json.RawMessage(`{"type":"object","properties":{"msg":{"type":"string"}}}`),
	}

	go func() {
		for {
			raw, err := serverT.Recv(ctx)
			if err != nil {
				return
			}
			var req mcp.Request
			if err := json.Unmarshal(raw, &req); err != nil {
				return
			}
			var resp mcp.Response
			switch req.Method {
			case "initialize":
				result := mcp.InitializeResult{
					ProtocolVersion: mcp.ProtocolVersion,
					Capabilities:    mcp.Capabilities{Tools: &mcp.ToolsCapability{}},
					ServerInfo:      mcp.ServerInfo{Name: "mock-server", Version: "1.0"},
				}
				b, _ := json.Marshal(result)
				resp = mcp.Response{JSONRPC: "2.0", ID: req.ID, Result: b}
			case "tools/list":
				result := mcp.ToolsListResult{Tools: []mcp.ToolDef{fakeTool}}
				b, _ := json.Marshal(result)
				resp = mcp.Response{JSONRPC: "2.0", ID: req.ID, Result: b}
			case "tools/call":
				result := mcp.CallToolResult{
					Content: []mcp.Content{{Type: "text", Text: "pong"}},
				}
				b, _ := json.Marshal(result)
				resp = mcp.Response{JSONRPC: "2.0", ID: req.ID, Result: b}
			default:
				continue
			}
			b, _ := json.Marshal(resp)
			_ = serverT.Send(ctx, b)
		}
	}()

	client := mcp.NewClient(clientT)
	return client, cancel
}

func TestClientInitialize(t *testing.T) {
	ctx := context.Background()
	client, cancel := startMockServer(t)
	defer cancel()

	if err := client.Initialize(ctx); err != nil {
		t.Fatalf("initialize: %v", err)
	}
	if client.Name() != "mock-server" {
		t.Errorf("server name: got %q, want mock-server", client.Name())
	}
}

func TestClientListTools(t *testing.T) {
	ctx := context.Background()
	client, cancel := startMockServer(t)
	defer cancel()

	if err := client.Initialize(ctx); err != nil {
		t.Fatalf("initialize: %v", err)
	}
	tools := client.Tools()
	if len(tools) != 1 {
		t.Fatalf("want 1 tool, got %d", len(tools))
	}
	if tools[0].Name != "echo" {
		t.Errorf("tool name: got %q, want echo", tools[0].Name)
	}
}

func TestClientCallTool(t *testing.T) {
	ctx := context.Background()
	client, cancel := startMockServer(t)
	defer cancel()

	if err := client.Initialize(ctx); err != nil {
		t.Fatalf("initialize: %v", err)
	}
	result, err := client.CallTool(ctx, mcp.CallToolRequest{
		Name:      "echo",
		Arguments: map[string]any{"msg": "hello"},
	})
	if err != nil {
		t.Fatalf("call tool: %v", err)
	}
	if len(result.Content) == 0 || result.Content[0].Text != "pong" {
		t.Errorf("unexpected result: %+v", result)
	}
}
