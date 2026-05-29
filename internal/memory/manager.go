// Package memory defines the memory tier interfaces for the agent runtime.
// Tiers: short-term (rolling window), long-term (vector retrieval),
// episodic (structured history), and profile (user/org preferences).
package memory

import (
	"ollama_agent_go/internal/memory/episodic"
	"ollama_agent_go/internal/memory/longterm"
	"ollama_agent_go/internal/memory/profile"
	"ollama_agent_go/internal/memory/shortterm"
)

// Manager provides access to all memory tiers.
type Manager interface {
	Short() shortterm.Store
	Long() longterm.Store
	Episodic() episodic.Store
	Profile() profile.Store
}

// Config holds configuration for the memory layer.
type Config struct {
	// TokenBudget is the short-term sliding-window size in tokens.
	// 0 disables trimming (keep all messages).
	TokenBudget int
}
