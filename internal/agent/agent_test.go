package agent

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"ollama_agent_go/internal/observability"
	"ollama_agent_go/internal/policy"
	"ollama_agent_go/internal/tools"
	"ollama_agent_go/internal/types"
)

// fakeChatter returns queued responses in order, recording requests it saw.
type fakeChatter struct {
	responses []types.ChatResponse
	requests  []types.ChatRequest
	i         int
}

func (f *fakeChatter) Chat(_ context.Context, req types.ChatRequest) (types.ChatResponse, error) {
	f.requests = append(f.requests, req)
	r := f.responses[f.i]
	f.i++
	return r, nil
}

// newTestHost builds a tool host with policy set to allow all (no approval).
func newTestHost(root string) *tools.Host {
	reg := tools.Default(root)
	pol := policy.NewDefaultEngine(root, nil, true) // noApproval=true for tests
	return tools.NewHost(reg, pol, observability.Discard)
}

func TestRunFinalAnswerNoTools(t *testing.T) {
	fc := &fakeChatter{responses: []types.ChatResponse{
		{Message: types.Message{Role: "assistant", Content: "hello world"}},
	}}
	a := New(fc, newTestHost(t.TempDir()), "test")

	var events []Event
	final, err := a.Run(context.Background(), []types.Message{{Role: "user", Content: "hi"}}, func(e Event) {
		events = append(events, e)
	})
	if err != nil {
		t.Fatalf("run: %v", err)
	}
	if final != "hello world" {
		t.Errorf("final = %q", final)
	}
	if events[len(events)-1].Kind != EventDone {
		t.Errorf("last event = %v, want Done", events[len(events)-1].Kind)
	}
}

func TestRunExecutesNativeToolCall(t *testing.T) {
	root := t.TempDir()
	os.WriteFile(filepath.Join(root, "data.txt"), []byte("secret"), 0o644)

	fc := &fakeChatter{responses: []types.ChatResponse{
		{Message: types.Message{
			Role: "assistant",
			ToolCalls: []types.ToolCall{{
				Function: types.ToolCallFunction{
					Name:      "read_file",
					Arguments: map[string]any{"path": "data.txt"},
				},
			}},
		}},
		{Message: types.Message{Role: "assistant", Content: "the file says secret"}},
	}}
	a := New(fc, newTestHost(root), "test")

	var toolResult string
	final, err := a.Run(context.Background(), []types.Message{{Role: "user", Content: "read data.txt"}}, func(e Event) {
		if e.Kind == EventToolResult {
			toolResult = e.Text
		}
	})
	if err != nil {
		t.Fatalf("run: %v", err)
	}
	if toolResult != "secret" {
		t.Errorf("tool result = %q, want secret", toolResult)
	}
	if final != "the file says secret" {
		t.Errorf("final = %q", final)
	}
	last := fc.requests[1]
	foundTool := false
	for _, m := range last.Messages {
		if m.Role == "tool" && m.Content == "secret" {
			foundTool = true
		}
	}
	if !foundTool {
		t.Error("tool result not fed back into conversation")
	}
}

func TestRunExecutesSyntheticToolCall(t *testing.T) {
	root := t.TempDir()
	os.WriteFile(filepath.Join(root, "x.txt"), []byte("hi"), 0o644)

	fc := &fakeChatter{responses: []types.ChatResponse{
		{Message: types.Message{Role: "assistant", Content: "```json\n{\"name\":\"read_file\",\"arguments\":{\"path\":\"x.txt\"}}\n```"}},
		{Message: types.Message{Role: "assistant", Content: "done"}},
	}}
	a := New(fc, newTestHost(root), "test")

	var sawCall bool
	_, err := a.Run(context.Background(), []types.Message{{Role: "user", Content: "go"}}, func(e Event) {
		if e.Kind == EventToolCall && e.Tool == "read_file" {
			sawCall = true
		}
	})
	if err != nil {
		t.Fatalf("run: %v", err)
	}
	if !sawCall {
		t.Error("synthetic tool call not dispatched")
	}
}

func TestRunSystemPromptPrepended(t *testing.T) {
	fc := &fakeChatter{responses: []types.ChatResponse{
		{Message: types.Message{Role: "assistant", Content: "ok"}},
	}}
	a := New(fc, newTestHost(t.TempDir()), "test")
	_, _ = a.Run(context.Background(), []types.Message{{Role: "user", Content: "hi"}}, nil)

	first := fc.requests[0].Messages[0]
	if first.Role != "system" || !strings.Contains(first.Content, "coding agent") {
		t.Errorf("system message not prepended: %+v", first)
	}
}

func TestRunMaxIterations(t *testing.T) {
	loop := types.ChatResponse{Message: types.Message{
		ToolCalls: []types.ToolCall{{Function: types.ToolCallFunction{Name: "ls", Arguments: map[string]any{}}}},
	}}
	resps := make([]types.ChatResponse, 20)
	for i := range resps {
		resps[i] = loop
	}
	fc := &fakeChatter{responses: resps}
	a := New(fc, newTestHost(t.TempDir()), "test")
	a.MaxIterations = 3

	_, err := a.Run(context.Background(), []types.Message{{Role: "user", Content: "go"}}, nil)
	if err == nil {
		t.Error("expected max-iterations error")
	}
}
