package shortterm

import (
	"testing"

	"ollama_agent_go/internal/tokens"
	"ollama_agent_go/internal/types"
)

func TestRollingAppendTrimsBudget(t *testing.T) {
	sys := types.Message{Role: "system", Content: "you are an agent"}
	old := types.Message{Role: "user", Content: "oldest message that should definitely be dropped when budget is tight"}
	newer := types.Message{Role: "assistant", Content: "older reply also dropped"}
	newest := types.Message{Role: "user", Content: "newest message"}

	// Budget: just enough for system + newest, not for old/newer.
	budget := tokens.EstimateMessage(sys) + tokens.EstimateMessage(newest) + 2
	s := New(budget)
	s.Append(sys)
	s.Append(old)
	s.Append(newer)
	s.Append(newest)

	msgs := s.Messages()
	if msgs[0].Role != "system" {
		t.Fatalf("system message not preserved: %+v", msgs)
	}
	if msgs[len(msgs)-1].Content != "newest message" {
		t.Errorf("most recent message dropped: %+v", msgs)
	}
	if len(msgs) >= 4 {
		t.Errorf("expected trimming, still have %d messages", len(msgs))
	}
}

func TestRollingResetClearsAll(t *testing.T) {
	s := New(0)
	s.Append(types.Message{Role: "user", Content: "a"})
	s.Append(types.Message{Role: "assistant", Content: "b"})
	s.Reset()
	if len(s.Messages()) != 0 {
		t.Errorf("expected empty after reset, got %d", len(s.Messages()))
	}
	if s.TokenCount() != 0 {
		t.Errorf("token count should be 0 after reset, got %d", s.TokenCount())
	}
}

func TestRollingPreservesSystemMessage(t *testing.T) {
	sys := types.Message{Role: "system", Content: "you are an agent"}
	// Tiny budget — only fits system message.
	budget := tokens.EstimateMessage(sys) + 1
	s := New(budget)
	s.Append(sys)
	s.Append(types.Message{Role: "user", Content: "hello"})

	msgs := s.Messages()
	if len(msgs) == 0 || msgs[0].Role != "system" {
		t.Errorf("system not preserved under tight budget: %+v", msgs)
	}
}

func TestRollingNoBudgetKeepsAll(t *testing.T) {
	s := New(0) // 0 = disabled
	for i := 0; i < 10; i++ {
		s.Append(types.Message{Role: "user", Content: "msg"})
	}
	if got := len(s.Messages()); got != 10 {
		t.Errorf("want 10 messages with no budget, got %d", got)
	}
}

func TestRollingTokenCount(t *testing.T) {
	s := New(0)
	msg := types.Message{Role: "user", Content: "hello world"}
	s.Append(msg)
	want := tokens.EstimateMessage(msg)
	if got := s.TokenCount(); got != want {
		t.Errorf("token count = %d, want %d", got, want)
	}
}
