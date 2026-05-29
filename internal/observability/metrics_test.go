package observability_test

import (
	"testing"

	"ollama_agent_go/internal/observability"
)

func TestIncrAndSnapshot(t *testing.T) {
	m := observability.NewInMemoryMetrics()

	m.IncrToolCall("write_file", true)
	m.IncrToolCall("write_file", false)
	m.IncrToolCall("read_file", true)

	m.IncrLLMCall("llama3", 100, true)
	m.IncrLLMCall("llama3", 200, false)

	m.IncrApproval("write_file", true)
	m.IncrApproval("write_file", false)

	snap := m.Snapshot()

	if snap.ToolCalls["write_file"].Total != 2 {
		t.Errorf("write_file total: got %d, want 2", snap.ToolCalls["write_file"].Total)
	}
	if snap.ToolCalls["write_file"].Errors != 1 {
		t.Errorf("write_file errors: got %d, want 1", snap.ToolCalls["write_file"].Errors)
	}
	if snap.ToolCalls["read_file"].Total != 1 {
		t.Errorf("read_file total: got %d, want 1", snap.ToolCalls["read_file"].Total)
	}

	if snap.LLMCalls["llama3"].Total != 2 {
		t.Errorf("llama3 total: got %d, want 2", snap.LLMCalls["llama3"].Total)
	}
	if snap.LLMCalls["llama3"].TotalTokens != 300 {
		t.Errorf("llama3 tokens: got %d, want 300", snap.LLMCalls["llama3"].TotalTokens)
	}

	if snap.TotalTokens != 300 {
		t.Errorf("total tokens: got %d, want 300", snap.TotalTokens)
	}
	if snap.Errors != 2 { // one tool error + one LLM error
		t.Errorf("total errors: got %d, want 2", snap.Errors)
	}
}

func TestSnapshotIsIsolated(t *testing.T) {
	m := observability.NewInMemoryMetrics()
	m.IncrToolCall("foo", true)

	s1 := m.Snapshot()
	m.IncrToolCall("foo", true)
	s2 := m.Snapshot()

	// Mutating s2 should not affect s1.
	if s1.ToolCalls["foo"].Total != 1 {
		t.Errorf("s1 contaminated: got %d", s1.ToolCalls["foo"].Total)
	}
	if s2.ToolCalls["foo"].Total != 2 {
		t.Errorf("s2 wrong: got %d", s2.ToolCalls["foo"].Total)
	}
}
