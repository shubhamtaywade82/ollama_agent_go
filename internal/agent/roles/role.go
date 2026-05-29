// Package roles defines specialized agent role configurations.
package roles

// Role identifies a specialized agent persona.
type Role int

const (
	RoleGeneral       Role = iota
	RoleResearch
	RoleReasoning
	RoleAction
	RoleData
	RoleCommunication
	RoleTrading
)

func (r Role) String() string {
	switch r {
	case RoleResearch:
		return "research"
	case RoleReasoning:
		return "reasoning"
	case RoleAction:
		return "action"
	case RoleData:
		return "data"
	case RoleCommunication:
		return "communication"
	case RoleTrading:
		return "trading"
	default:
		return "general"
	}
}

// ParseRoleString converts a string to a Role, returning RoleGeneral on unknown values.
func ParseRoleString(s string) Role {
	r, _ := ParseRole(s)
	return r
}

// ParseRole converts a string to a Role (case-insensitive).
func ParseRole(s string) (Role, bool) {
	switch s {
	case "research":
		return RoleResearch, true
	case "reasoning":
		return RoleReasoning, true
	case "action":
		return RoleAction, true
	case "data":
		return RoleData, true
	case "communication", "comms":
		return RoleCommunication, true
	case "trading":
		return RoleTrading, true
	case "general", "reset", "":
		return RoleGeneral, true
	default:
		return RoleGeneral, false
	}
}

// RoleConfig binds a Role to its runtime parameters.
type RoleConfig struct {
	Role          Role
	SystemPrompt  string
	AllowedTools  []string // nil = all tools allowed; empty = no tools
	MaxIterations int
	PreferModel   string   // provider hint
	MemoryTiers   []string // "short", "long", "episodic", "profile"
}

// DefaultConfigs is the built-in role configuration table.
var DefaultConfigs = map[Role]RoleConfig{
	RoleGeneral: {
		Role:          RoleGeneral,
		MaxIterations: 12,
	},
	RoleResearch: {
		Role:          RoleResearch,
		SystemPrompt:  researchPrompt,
		AllowedTools:  []string{"search_knowledge", "read_file", "list_files", "web_search"},
		MaxIterations: 10,
		MemoryTiers:   []string{"short", "long"},
	},
	RoleReasoning: {
		Role:          RoleReasoning,
		SystemPrompt:  reasoningPrompt,
		AllowedTools:  []string{}, // no tools — pure reasoning
		MaxIterations: 5,
	},
	RoleAction: {
		Role:          RoleAction,
		SystemPrompt:  actionPrompt,
		MaxIterations: 20,
		MemoryTiers:   []string{"short", "episodic"},
	},
	RoleData: {
		Role:          RoleData,
		SystemPrompt:  dataPrompt,
		AllowedTools:  []string{"run_shell", "read_file", "list_files"},
		MaxIterations: 10,
	},
	RoleCommunication: {
		Role:          RoleCommunication,
		SystemPrompt:  commPrompt,
		AllowedTools:  []string{},
		MaxIterations: 3,
	},
	RoleTrading: {
		Role:          RoleTrading,
		SystemPrompt:  tradingPrompt,
		AllowedTools:  []string{"get_quote", "get_ohlcv", "get_signals", "get_portfolio", "place_order", "cancel_order", "get_news"},
		MaxIterations: 15,
	},
}
