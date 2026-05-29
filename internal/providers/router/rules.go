package router

import "ollama_agent_go/internal/providers"

// Rule maps a TaskType to provider capability requirements and model preferences.
type Rule struct {
	TaskType    TaskType
	RequiresCap []string // "tools", "streaming", "thinking"
	PreferModel string   // preferred model name hint (matched against provider Name())
	MaxCostPer1K float64 // max input cost per 1K tokens, USD; 0 = no constraint
}

// DefaultRules is the out-of-the-box routing table.
var DefaultRules = []Rule{
	{TaskType: TaskCoding, RequiresCap: []string{"tools"}, PreferModel: "ollama"},
	{TaskType: TaskReasoning, RequiresCap: []string{"thinking"}},
	{TaskType: TaskFast, MaxCostPer1K: 0.001},
	{TaskType: TaskGeneral, RequiresCap: []string{"tools"}},
}

// satisfies returns true if p meets all of rule's capability requirements.
func (rule Rule) satisfies(p providers.Provider) bool {
	for _, cap := range rule.RequiresCap {
		switch cap {
		case "tools":
			if !p.SupportsTools() {
				return false
			}
		case "streaming":
			if !p.SupportsStreaming() {
				return false
			}
		case "thinking":
			if !p.SupportsThinking() {
				return false
			}
		}
	}
	if rule.MaxCostPer1K > 0 {
		costPer1K := p.Pricing().InputPerM / 1000.0
		if costPer1K > rule.MaxCostPer1K {
			return false
		}
	}
	return true
}
