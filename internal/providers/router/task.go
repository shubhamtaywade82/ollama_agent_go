package router

import (
	"strings"

	"ollama_agent_go/internal/types"
)

// TaskType classifies the kind of work the agent is being asked to do.
// The router uses this to select the best-suited provider.
type TaskType int

const (
	TaskGeneral     TaskType = iota // default — use active provider
	TaskCoding                      // code generation/editing → prefer local coder model
	TaskReasoning                   // analysis, planning, explanation → prefer thinking model
	TaskFast                        // short/simple exchange → prefer cheapest/fastest
	TaskLongContext                 // large context required → prefer high-context model
)

// codingSignals are file extensions and code-related keywords.
var codingSignals = []string{
	"```", ".go", ".py", ".ts", ".js", ".rs", ".cpp", ".java", ".sh",
	"function", "func ", "class ", "import ", "package ", "def ",
	"refactor", "implement", "fix the bug", "write a test", "unit test",
}

// reasoningSignals indicate analytical or planning tasks.
var reasoningSignals = []string{
	"analyze", "analyse", "reason", "explain why", "compare", "evaluate",
	"what would happen", "pros and cons", "tradeoffs", "trade-offs",
	"architecture", "design", "plan", "strategy", "think through",
}

// Detect infers the TaskType from the last user message in msgs.
// Returns TaskGeneral when no clear signal is found.
func Detect(msgs []types.Message) TaskType {
	// Find the last user message.
	text := ""
	for i := len(msgs) - 1; i >= 0; i-- {
		if msgs[i].Role == "user" {
			text = strings.ToLower(msgs[i].Content)
			break
		}
	}
	if text == "" {
		return TaskGeneral
	}

	// Short single-word queries → fast.
	if len(strings.Fields(text)) <= 3 && !strings.Contains(text, "```") {
		return TaskFast
	}

	for _, sig := range codingSignals {
		if strings.Contains(text, sig) {
			return TaskCoding
		}
	}
	for _, sig := range reasoningSignals {
		if strings.Contains(text, sig) {
			return TaskReasoning
		}
	}
	return TaskGeneral
}
