package orchestration

import (
	"context"
	"fmt"
	"strings"

	"ollama_agent_go/internal/agent"
	"ollama_agent_go/internal/observability"
)

// Orchestrator decomposes a goal into tasks, schedules them, executes each
// with the appropriate agent role, and aggregates the results.
type Orchestrator struct {
	planner    *Planner
	scheduler  *Scheduler
	executor   *Executor
	aggregator *Aggregator
	obs        observability.Logger
}

// New creates a fully wired Orchestrator.
func New(pl *Planner, sc *Scheduler, ex *Executor, ag *Aggregator, obs observability.Logger) *Orchestrator {
	return &Orchestrator{
		planner:    pl,
		scheduler:  sc,
		executor:   ex,
		aggregator: ag,
		obs:        obs,
	}
}

// Run decomposes goal → plans → schedules → executes → aggregates.
// emit receives streaming events (may be nil).
func (o *Orchestrator) Run(ctx context.Context, goal string, emit agent.Emit) (string, error) {
	o.obs.Info("orchestration start", "goal", goal)

	graph, err := o.planner.Plan(ctx, goal)
	if err != nil {
		return "", fmt.Errorf("orchestration plan: %w", err)
	}
	o.obs.Info("plan ready", "tasks", len(graph.Tasks))

	execFn := func(ctx context.Context, task *Task) error {
		// Inject results from completed dependencies before running.
		for _, depID := range task.DependsOn {
			dep := graph.byID(depID)
			if dep != nil && dep.Status == TaskDone {
				task.depResults = append(task.depResults, depResult{taskID: dep.ID, result: dep.Result})
			}
		}
		return o.executor.ExecTask(ctx, task)
	}

	if err := o.scheduler.RunGraph(ctx, graph, execFn); err != nil {
		// Non-fatal: some tasks may have failed but others succeeded.
		o.obs.Error("orchestration partial failure", err)
	}

	result, err := o.aggregator.Aggregate(ctx, goal, graph.Tasks)
	if err != nil {
		// Fall back to concatenating task results.
		result = concatenateResults(graph.Tasks)
	}
	o.obs.Info("orchestration done", "result_len", len(result))
	return result, nil
}

// Preview decomposes goal into a task graph and returns a formatted description
// without executing anything. Used by the /plan TUI command.
func (o *Orchestrator) Preview(ctx context.Context, goal string) (string, error) {
	graph, err := o.planner.Plan(ctx, goal)
	if err != nil {
		return "", err
	}
	var sb strings.Builder
	fmt.Fprintf(&sb, "Plan for: %s\n\n", goal)
	ordered, _ := graph.TopologicalOrder()
	for i, t := range ordered {
		deps := "(none)"
		if len(t.DependsOn) > 0 {
			deps = strings.Join(t.DependsOn, ", ")
		}
		fmt.Fprintf(&sb, "%d. [%s] %s  role=%s  deps=%s\n", i+1, t.ID, t.Goal, t.Role.String(), deps)
	}
	return sb.String(), nil
}

func concatenateResults(tasks []*Task) string {
	var out string
	for _, t := range tasks {
		if t.Status == TaskDone && t.Result != "" {
			out += fmt.Sprintf("[%s] %s\n\n", t.ID, t.Result)
		}
	}
	if out == "" {
		return "(no results)"
	}
	return out
}
