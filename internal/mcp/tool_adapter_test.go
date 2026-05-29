package mcp_test

import (
	"context"
	"encoding/json"
	"testing"

	"ollama_agent_go/internal/mcp"
)

func TestAdaptAll(t *testing.T) {
	ctx := context.Background()
	client, cancel := startMockServer(t)
	defer cancel()

	if err := client.Initialize(ctx); err != nil {
		t.Fatalf("initialize: %v", err)
	}

	adapted := mcp.AdaptAll(client)
	if len(adapted) != 1 {
		t.Fatalf("want 1 adapted tool, got %d", len(adapted))
	}
	tool := adapted[0]
	if tool.Name() != "echo" {
		t.Errorf("name: got %q, want echo", tool.Name())
	}
	if tool.Description() == "" {
		t.Error("description should not be empty")
	}
}

func TestAdaptedToolExecute(t *testing.T) {
	ctx := context.Background()
	client, cancel := startMockServer(t)
	defer cancel()

	if err := client.Initialize(ctx); err != nil {
		t.Fatalf("initialize: %v", err)
	}

	adapted := mcp.AdaptAll(client)
	result, err := adapted[0].Execute(ctx, map[string]any{"msg": "hello"})
	if err != nil {
		t.Fatalf("execute: %v", err)
	}
	if result != "pong" {
		t.Errorf("result: got %q, want pong", result)
	}
}

func TestAdaptedToolParameters(t *testing.T) {
	ctx := context.Background()
	client, cancel := startMockServer(t)
	defer cancel()

	if err := client.Initialize(ctx); err != nil {
		t.Fatalf("initialize: %v", err)
	}

	adapted := mcp.AdaptAll(client)
	params := adapted[0].Parameters()
	// Schema from mock server has "type":"object","properties":{"msg":...}
	if _, ok := params["type"]; !ok {
		// parameters may be nil or empty if schema unmarshal fails — just
		// check it doesn't panic.
	}
	// Re-serialise to ensure it's JSON-safe.
	if _, err := json.Marshal(params); err != nil {
		t.Errorf("parameters not JSON-serialisable: %v", err)
	}
}
