package orchestration

import (
	"context"
	"fmt"
	"strings"
	"time"

	"ollama_agent_go/internal/agent"
	"ollama_agent_go/internal/memory"
	"ollama_agent_go/internal/observability"
	"ollama_agent_go/internal/types"
)

// Executor runs individual tasks by routing them through the agent Selector.
type Executor struct {
	selector *agent.Selector
	memory   memory.Manager
	obs      observability.Logger
}

// NewExecutor creates an Executor.
func NewExecutor(sel *agent.Selector, mem memory.Manager, obs observability.Logger) *Executor {
	return &Executor{selector: sel, memory: mem, obs: obs}
}

// ExecTask selects the right agent for task.Role/Goal, builds context from
// dependency results, runs the agent, and populates task.Result.
func (e *Executor) ExecTask(ctx context.Context, task *Task) error {
	task.StartedAt = time.Now()

	// Build history: start with short-term conversation window then prepend
	// results from completed dependency tasks.
	history := e.memory.Short().Messages()

	// Inject dependency results as system context before the task goal.
	depContext := buildDepContext(task)
	if depContext != "" {
		history = append([]types.Message{{Role: "system", Content: depContext}}, history...)
	}
	history = append(history, types.Message{Role: "user", Content: task.Goal})

	ag := e.selector.SelectRole(task.Role)
	result, err := ag.Run(ctx, history, nil)
	task.CompletedAt = time.Now()
	if err != nil {
		return fmt.Errorf("task %q: %w", task.ID, err)
	}
	task.Result = result
	e.obs.Info("task done", "id", task.ID, "role", task.Role.String(), "duration",
		task.CompletedAt.Sub(task.StartedAt).String())
	return nil
}

// buildDepContext formats completed dependency results as a system message.
func buildDepContext(task *Task) string {
	// dependency results are populated on the task graph by the Orchestrator
	// before ExecTask is called; they're embedded via task.depResults.
	// For this phase we use the task's own depResults slice (set by Orchestrator).
	if len(task.depResults) == 0 {
		return ""
	}
	var sb strings.Builder
	sb.WriteString("Context from prior tasks:\n")
	for _, r := range task.depResults {
		fmt.Fprintf(&sb, "- [%s] %s\n", r.taskID, r.result)
	}
	return sb.String()
}

