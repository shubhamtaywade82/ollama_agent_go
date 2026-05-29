package memory

import (
	"ollama_agent_go/internal/memory/episodic"
	"ollama_agent_go/internal/memory/longterm"
	"ollama_agent_go/internal/memory/profile"
	"ollama_agent_go/internal/memory/shortterm"
)

// DefaultManager wires together the four memory tiers.
type DefaultManager struct {
	short    *shortterm.RollingStore
	long     longterm.Store
	episodic episodic.Store
	profile  profile.Store
}

// New constructs a DefaultManager from pre-created tier implementations.
// Pass nil for long/ep/prof to use the built-in no-op stubs.
func New(budget int, long longterm.Store, ep episodic.Store, prof profile.Store) *DefaultManager {
	if long == nil {
		long = longterm.NoopStore{}
	}
	if ep == nil {
		ep = episodic.NoopStore{}
	}
	if prof == nil {
		prof = profile.NoopStore{}
	}
	return &DefaultManager{
		short:    shortterm.New(budget),
		long:     long,
		episodic: ep,
		profile:  prof,
	}
}

// NewDefault creates a manager with only a rolling short-term store and
// no-op stubs for the other tiers. Suitable for tests and development.
func NewDefault(budget int) *DefaultManager {
	return New(budget, nil, nil, nil)
}

func (m *DefaultManager) Short() shortterm.Store    { return m.short }
func (m *DefaultManager) Long() longterm.Store      { return m.long }
func (m *DefaultManager) Episodic() episodic.Store  { return m.episodic }
func (m *DefaultManager) Profile() profile.Store    { return m.profile }
