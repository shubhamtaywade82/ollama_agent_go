package agent

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"ollama_agent_go/pkg/ollama"
	"ollama_agent_go/pkg/tools"
)

// fakeChatter returns queued responses in order, recording requests it saw.
type fakeChatter struct {
	responses []ollama.ChatResponse
	requests  []ollama.ChatRequest
	i         int
}

func (f *fakeChatter) Chat(_ context.Context, req ollama.ChatRequest) (ollama.ChatResponse, error) {
	f.requests = append(f.requests, req)
	r := f.responses[f.i]
	f.i++
	return r, nil
}

func TestRunFinalAnswerNoTools(t *testing.T) {
	fc := &fakeChatter{responses: []ollama.ChatResponse{
		{Message: ollama.Message{Role: "assistant", Content: "hello world"}},
	}}
	a := New(fc, tools.Default(t.TempDir()), "test")

	var events []Event
	final, err := a.Run(context.Background(), []ollama.Message{{Role: "user", Content: "hi"}}, func(e Event) {
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

	fc := &fakeChatter{responses: []ollama.ChatResponse{
		{Message: ollama.Message{
			Role: "assistant",
			ToolCalls: []ollama.ToolCall{{
				Function: ollama.ToolCallFunction{
					Name:      "read_file",
					Arguments: map[string]any{"path": "data.txt"},
				},
			}},
		}},
		{Message: ollama.Message{Role: "assistant", Content: "the file says secret"}},
	}}
	a := New(fc, tools.Default(root), "test")

	var toolResult string
	final, err := a.Run(context.Background(), []ollama.Message{{Role: "user", Content: "read data.txt"}}, func(e Event) {
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
	// Second request should carry the tool result message back to the model.
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

	fc := &fakeChatter{responses: []ollama.ChatResponse{
		{Message: ollama.Message{Role: "assistant", Content: "```json\n{\"name\":\"read_file\",\"arguments\":{\"path\":\"x.txt\"}}\n```"}},
		{Message: ollama.Message{Role: "assistant", Content: "done"}},
	}}
	a := New(fc, tools.Default(root), "test")

	var sawCall bool
	_, err := a.Run(context.Background(), []ollama.Message{{Role: "user", Content: "go"}}, func(e Event) {
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
	fc := &fakeChatter{responses: []ollama.ChatResponse{
		{Message: ollama.Message{Role: "assistant", Content: "ok"}},
	}}
	a := New(fc, tools.Default(t.TempDir()), "test")
	_, _ = a.Run(context.Background(), []ollama.Message{{Role: "user", Content: "hi"}}, nil)

	first := fc.requests[0].Messages[0]
	if first.Role != "system" || !strings.Contains(first.Content, "coding agent") {
		t.Errorf("system message not prepended: %+v", first)
	}
}

func TestRunMaxIterations(t *testing.T) {
	// Always returns a tool call -> never terminates -> hits the cap.
	loop := ollama.ChatResponse{Message: ollama.Message{
		ToolCalls: []ollama.ToolCall{{Function: ollama.ToolCallFunction{Name: "ls", Arguments: map[string]any{}}}},
	}}
	resps := make([]ollama.ChatResponse, 20)
	for i := range resps {
		resps[i] = loop
	}
	fc := &fakeChatter{responses: resps}
	a := New(fc, tools.Default(t.TempDir()), "test")
	a.MaxIterations = 3

	_, err := a.Run(context.Background(), []ollama.Message{{Role: "user", Content: "go"}}, nil)
	if err == nil {
		t.Error("expected max-iterations error")
	}
}
