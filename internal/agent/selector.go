package agent

import (
	"strings"

	"ollama_agent_go/internal/agent/roles"
	"ollama_agent_go/internal/tools"
)

// Selector picks the right Agent for a given user input.
type Selector struct {
	agents  map[roles.Role]*Agent
	pinned  roles.Role  // set by Pin(); RoleGeneral = auto-select
	hasPin  bool
}

// NewSelector builds a Selector from a base agent and a scoped tool host factory.
// roleAgents maps each role to a pre-built agent; missing roles fall back to base.
func NewSelector(base *Agent, host *tools.Host, roleAgents map[roles.Role]*Agent) *Selector {
	s := &Selector{
		agents: make(map[roles.Role]*Agent),
		pinned: roles.RoleGeneral,
	}
	// Populate with provided role agents, falling back to base for missing ones.
	for _, role := range []roles.Role{
		roles.RoleGeneral, roles.RoleResearch, roles.RoleReasoning,
		roles.RoleAction, roles.RoleData, roles.RoleCommunication,
		roles.RoleTrading,
	} {
		if ag, ok := roleAgents[role]; ok {
			s.agents[role] = ag
		} else {
			s.agents[role] = base
		}
	}
	return s
}

// Select returns the best agent for input, respecting any active pin.
func (s *Selector) Select(input string) *Agent {
	if s.hasPin && s.pinned != roles.RoleGeneral {
		return s.agentFor(s.pinned)
	}
	return s.agentFor(detectRole(input))
}

// SelectRole returns the agent configured for the given role directly.
// Falls back to the General agent if the role is not registered.
func (s *Selector) SelectRole(role roles.Role) *Agent {
	return s.agentFor(role)
}

// Pin locks the selector to role for all subsequent Select calls.
// Use RoleGeneral to reset to auto-select.
func (s *Selector) Pin(role roles.Role) {
	s.pinned = role
	s.hasPin = role != roles.RoleGeneral
}

// PinnedRole returns the currently pinned role (RoleGeneral = auto).
func (s *Selector) PinnedRole() roles.Role { return s.pinned }

func (s *Selector) agentFor(role roles.Role) *Agent {
	if ag, ok := s.agents[role]; ok {
		return ag
	}
	return s.agents[roles.RoleGeneral]
}

// detectRole maps input text to a role via simple keyword heuristics.
func detectRole(input string) roles.Role {
	lower := strings.ToLower(strings.TrimSpace(input))

	researchPrefixes := []string{"find", "search", "what is", "what are", "who is", "tell me about", "look up", "lookup"}
	for _, p := range researchPrefixes {
		if strings.HasPrefix(lower, p) {
			return roles.RoleResearch
		}
	}

	reasoningPrefixes := []string{"analyze", "explain why", "compare", "contrast", "reason", "think through", "evaluate", "assess"}
	for _, p := range reasoningPrefixes {
		if strings.HasPrefix(lower, p) {
			return roles.RoleReasoning
		}
	}

	if strings.Contains(lower, "summarize") || strings.Contains(lower, "draft") ||
		strings.HasPrefix(lower, "write email") || strings.HasPrefix(lower, "write a message") {
		return roles.RoleCommunication
	}

	dataKeywords := []string{"query", "sql", "csv", "table", "dataframe", "select ", "where "}
	for _, kw := range dataKeywords {
		if strings.Contains(lower, kw) {
			return roles.RoleData
		}
	}

	tradingKeywords := []string{"quote", "buy ", "sell ", "ticker", "stock", "trade", "portfolio", "signal", "chart", "rsi", "macd", "ohlcv", "market data", "earnings", "equity"}
	for _, kw := range tradingKeywords {
		if strings.Contains(lower, kw) {
			return roles.RoleTrading
		}
	}

	actionPrefixes := []string{"write", "create", "run", "execute", "fix", "refactor", "implement", "add", "delete", "remove", "edit"}
	for _, p := range actionPrefixes {
		if strings.HasPrefix(lower, p) {
			return roles.RoleAction
		}
	}

	return roles.RoleGeneral
}

// BuildRoleAgents constructs one Agent per role using a scoped host.
// base is used for the system client and model; its ToolHost is narrowed per role.
func BuildRoleAgents(base *Agent, host *tools.Host) map[roles.Role]*Agent {
	out := make(map[roles.Role]*Agent, len(roles.DefaultConfigs))
	for role, cfg := range roles.DefaultConfigs {
		ag := &Agent{
			Client:        base.Client,
			Model:         base.Model,
			ModelHint:     base.ModelHint,
			Options:       base.Options,
			ContextBudget: base.ContextBudget,
			MaxIterations: cfg.MaxIterations,
		}
		if cfg.SystemPrompt != "" {
			ag.System = cfg.SystemPrompt
		} else {
			ag.System = base.System
		}
		// Wire scoped host so the role can only see its allowed tools.
		scoped := tools.NewScopedHost(host, cfg.AllowedTools)
		ag.ToolHost = scoped
		out[role] = ag
	}
	return out
}
