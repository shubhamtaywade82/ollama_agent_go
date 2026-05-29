// Package shortterm provides a token-budgeted rolling conversation window.
package shortterm

import (
	"sync"

	"ollama_agent_go/internal/tokens"
	"ollama_agent_go/internal/types"
)

// Store is the interface for the short-term conversation window.
type Store interface {
	Append(msg types.Message)
	Messages() []types.Message
	Reset()
	TokenCount() int
}

// RollingStore keeps messages within a token budget, dropping the oldest
// non-system messages when the budget is exceeded. Budget <= 0 disables trimming.
type RollingStore struct {
	mu     sync.Mutex
	budget int
	msgs   []types.Message
}

// New returns a RollingStore with the given token budget.
func New(budget int) *RollingStore {
	return &RollingStore{budget: budget}
}

// Append adds msg to the window and trims to budget if needed.
func (s *RollingStore) Append(msg types.Message) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.msgs = append(s.msgs, msg)
	if s.budget > 0 {
		s.msgs = tokens.Trim(s.msgs, s.budget)
	}
}

// Messages returns a copy of the current window.
func (s *RollingStore) Messages() []types.Message {
	s.mu.Lock()
	defer s.mu.Unlock()
	out := make([]types.Message, len(s.msgs))
	copy(out, s.msgs)
	return out
}

// Reset clears all messages.
func (s *RollingStore) Reset() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.msgs = nil
}

// TokenCount returns the estimated token count for the current window.
func (s *RollingStore) TokenCount() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return tokens.EstimateMessages(s.msgs)
}
