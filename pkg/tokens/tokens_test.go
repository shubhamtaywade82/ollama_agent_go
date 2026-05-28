package tokens

import (
	"testing"

	"ollama_agent_go/pkg/ollama"
)

func TestEstimateMonotonic(t *testing.T) {
	short := Estimate("hello")
	long := Estimate("hello there, this is a much longer sentence with punctuation!")
	if short <= 0 {
		t.Errorf("short estimate = %d", short)
	}
	if long <= short {
		t.Errorf("longer text should estimate more tokens: %d vs %d", long, short)
	}
}

func TestEstimateEmpty(t *testing.T) {
	if Estimate("") != 0 {
		t.Errorf("empty = %d, want 0", Estimate(""))
	}
}

func TestTrimKeepsSystemAndRecent(t *testing.T) {
	msgs := []ollama.Message{
		{Role: "system", Content: "you are an agent"},
		{Role: "user", Content: "oldest message that should be dropped under a tight budget"},
		{Role: "assistant", Content: "older reply also dropped"},
		{Role: "user", Content: "newest"},
	}
	budget := EstimateMessage(msgs[0]) + EstimateMessage(msgs[3]) + 2
	got := Trim(msgs, budget)

	if got[0].Role != "system" {
		t.Fatalf("system not preserved: %+v", got)
	}
	if got[len(got)-1].Content != "newest" {
		t.Errorf("most recent message dropped: %+v", got)
	}
	if len(got) >= len(msgs) {
		t.Errorf("expected trimming, got %d of %d", len(got), len(msgs))
	}
}

func TestTrimDisabled(t *testing.T) {
	msgs := []ollama.Message{{Role: "user", Content: "a"}, {Role: "user", Content: "b"}}
	if got := Trim(msgs, 0); len(got) != 2 {
		t.Errorf("budget 0 should disable trimming, got %d", len(got))
	}
}

func TestTrimDropsOrphanTool(t *testing.T) {
	msgs := []ollama.Message{
		{Role: "system", Content: "sys"},
		{Role: "tool", ToolName: "ls", Content: "orphaned tool result with enough text to be dropped from the front"},
		{Role: "user", Content: "recent"},
	}
	budget := EstimateMessage(msgs[0]) + EstimateMessage(msgs[2]) + 2
	got := Trim(msgs, budget)
	for _, m := range got {
		if m.Role == "tool" {
			t.Errorf("orphan tool message should be dropped: %+v", got)
		}
	}
}
