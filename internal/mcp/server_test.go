package mcp_test

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"ollama_agent_go/internal/mcp"
	"ollama_agent_go/internal/observability"
	"ollama_agent_go/internal/policy"
	"ollama_agent_go/internal/tools"
)

func newTestHost(t *testing.T) *tools.Host {
	t.Helper()
	reg := tools.NewRegistry("/tmp")
	reg.Register(&echoTool{})
	pol := policy.NewDefaultEngine("/tmp", nil, true) // auto-approve
	return tools.NewHost(reg, pol, observability.Discard)
}

type echoTool struct{}

func (e *echoTool) Name() string               { return "echo" }
func (e *echoTool) Description() string        { return "echoes input" }
func (e *echoTool) Parameters() map[string]any { return map[string]any{} }
func (e *echoTool) Execute(_ context.Context, args map[string]any) (string, error) {
	if msg, ok := args["msg"].(string); ok {
		return msg, nil
	}
	return "", nil
}

func serverRequest(t *testing.T, srv *mcp.Server, req mcp.Request) mcp.Response {
	t.Helper()
	body, _ := json.Marshal(req)
	r := httptest.NewRequest(http.MethodPost, "/rpc", bytes.NewReader(body))
	r.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	srv.ServeHTTP(w, r)
	var resp mcp.Response
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	return resp
}

func TestServerToolsList(t *testing.T) {
	host := newTestHost(t)
	srv := mcp.NewServer(host, "test-server", "1.0")

	resp := serverRequest(t, srv, mcp.Request{
		JSONRPC: "2.0",
		ID:      1,
		Method:  "tools/list",
	})

	if resp.Error != nil {
		t.Fatalf("tools/list error: %s", resp.Error.Message)
	}

	var result mcp.ToolsListResult
	if err := json.Unmarshal(resp.Result, &result); err != nil {
		t.Fatalf("decode tools/list: %v", err)
	}
	if len(result.Tools) == 0 {
		t.Fatal("expected at least one tool")
	}
	if result.Tools[0].Name != "echo" {
		t.Errorf("tool name: got %q, want echo", result.Tools[0].Name)
	}
}

func TestServerToolsCall(t *testing.T) {
	host := newTestHost(t)
	srv := mcp.NewServer(host, "test-server", "1.0")

	params, _ := json.Marshal(mcp.CallToolRequest{
		Name:      "echo",
		Arguments: map[string]any{"msg": "hello from mcp"},
	})

	resp := serverRequest(t, srv, mcp.Request{
		JSONRPC: "2.0",
		ID:      2,
		Method:  "tools/call",
		Params:  params,
	})

	if resp.Error != nil {
		t.Fatalf("tools/call error: %s", resp.Error.Message)
	}

	var result mcp.CallToolResult
	if err := json.Unmarshal(resp.Result, &result); err != nil {
		t.Fatalf("decode tools/call: %v", err)
	}
	if result.IsError {
		t.Fatal("expected success")
	}
	if len(result.Content) == 0 || result.Content[0].Text != "hello from mcp" {
		t.Errorf("unexpected content: %+v", result.Content)
	}
}

func TestServerInitialize(t *testing.T) {
	host := newTestHost(t)
	srv := mcp.NewServer(host, "test-server", "1.0")

	params, _ := json.Marshal(mcp.InitializeParams{
		ProtocolVersion: mcp.ProtocolVersion,
		ClientInfo:      mcp.ServerInfo{Name: "test-client", Version: "1.0"},
	})
	resp := serverRequest(t, srv, mcp.Request{
		JSONRPC: "2.0",
		ID:      0,
		Method:  "initialize",
		Params:  params,
	})

	if resp.Error != nil {
		t.Fatalf("initialize error: %s", resp.Error.Message)
	}

	var result mcp.InitializeResult
	if err := json.Unmarshal(resp.Result, &result); err != nil {
		t.Fatalf("decode initialize: %v", err)
	}
	if result.ServerInfo.Name != "test-server" {
		t.Errorf("server name: got %q, want test-server", result.ServerInfo.Name)
	}
}
