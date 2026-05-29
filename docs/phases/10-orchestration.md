# Phase 10 — Orchestration / Workflow Engine

Tags: `#orchestration` `#workflow` `#tasks` `#decomposition` `#dag` `#scheduling` `#planner` `#p2` `#status/planned`

Prerequisites: Phase 09 (agent roles + selector), Phase 06 (reliability/retry), Phase 03 (saga).

---

## Goal

Add a Control Plane that can decompose a high-level goal into a DAG of sub-tasks,
assign each to the best agent role, execute them (sequentially or in parallel),
and aggregate results. This maps to reference architecture Layer 02 (Orchestration/Control Plane).

---

## Core concepts

```
Goal (user input)
  └── Planner → TaskGraph (DAG of Task nodes)
        └── Scheduler → execution order (topological sort, respects dependencies)
              └── Executor → dispatches each Task to Selector.Select()
                    └── Aggregator → combines Task results into final response
```

---

## Files to create (in order)

### Step 1 — Task types
**`internal/orchestration/task.go`**

```go
package orchestration

type TaskStatus int

const (
    TaskPending  TaskStatus = iota
    TaskRunning
    TaskDone
    TaskFailed
    TaskSkipped
)

type Task struct {
    ID           string
    Goal         string           // the sub-goal for this task
    Role         roles.Role       // which agent role handles this
    DependsOn    []string         // task IDs that must complete first
    Status       TaskStatus
    Result       string
    Error        error
    StartedAt    time.Time
    CompletedAt  time.Time
}

type TaskGraph struct {
    Tasks  []*Task
    order  []string   // topological order
}

func (g *TaskGraph) Add(t *Task)
func (g *TaskGraph) TopologicalOrder() ([]*Task, error)
func (g *TaskGraph) Ready() []*Task   // tasks with all deps Done
```

Tags: `#orchestration/task`

---

### Step 2 — Planner (LLM-powered decomposition)
**`internal/orchestration/planner.go`**

```go
type Planner struct {
    provider providers.Provider   // uses reasoning model
    maxTasks int
}

func NewPlanner(p providers.Provider, maxTasks int) *Planner

// Plan decomposes a goal into a TaskGraph using an LLM call with structured output.
func (pl *Planner) Plan(ctx context.Context, goal string) (*TaskGraph, error)
```

System prompt for planning LLM call:

```
Decompose the following goal into at most {maxTasks} concrete sub-tasks.
Each sub-task must have: id, goal, role (research|reasoning|action|data|communication),
depends_on (list of ids).
Return JSON only: {"tasks": [...]}
```

Tags: `#orchestration/planner`

---

### Step 3 — Scheduler
**`internal/orchestration/scheduler.go`**

```go
type Scheduler struct {
    maxConcurrent int
}

func NewScheduler(maxConcurrent int) *Scheduler

// Next returns the next batch of tasks ready to execute (all deps Done).
func (s *Scheduler) Next(g *TaskGraph) []*Task

// RunGraph executes the graph, calling execFn for each task.
// Respects maxConcurrent: uses a semaphore to limit parallelism.
func (s *Scheduler) RunGraph(ctx context.Context, g *TaskGraph, execFn func(context.Context, *Task) error) error
```

Tags: `#orchestration/scheduler`

---

### Step 4 — Executor
**`internal/orchestration/executor.go`**

```go
type Executor struct {
    selector  *agent.Selector
    memory    memory.Manager
    obs       observability.Logger
}

func NewExecutor(sel *agent.Selector, mem memory.Manager, obs observability.Logger) *Executor

// ExecTask runs a single task using the selected agent.
func (e *Executor) ExecTask(ctx context.Context, task *Task) error
```

The executor:
1. Selects agent via `e.selector.Select(task.Goal)` (overridden by `task.Role` if set)
2. Prepends results of completed dependency tasks to the conversation history
3. Calls `agent.Run()`
4. Sets `task.Result`, `task.Status`, `task.CompletedAt`

Tags: `#orchestration/executor`

---

### Step 5 — Aggregator
**`internal/orchestration/aggregator.go`**

```go
type Aggregator struct {
    provider providers.Provider   // communication model
}

// Aggregate combines task results into a final coherent response.
func (a *Aggregator) Aggregate(ctx context.Context, goal string, tasks []*Task) (string, error)
```

Uses a communication-model LLM call with all task results in context.

Tags: `#orchestration/aggregator`

---

### Step 6 — Orchestrator (top-level coordinator)
**`internal/orchestration/orchestrator.go`**

```go
type Orchestrator struct {
    planner    *Planner
    scheduler  *Scheduler
    executor   *Executor
    aggregator *Aggregator
    obs        observability.Logger
}

func New(pl *Planner, sc *Scheduler, ex *Executor, ag *Aggregator, obs observability.Logger) *Orchestrator

// Run decomposes goal → plans → schedules → executes → aggregates.
func (o *Orchestrator) Run(ctx context.Context, goal string, emit agent.Emit) (string, error)
```

Tags: `#orchestration/coordinator`

---

### Step 7 — Engine integration
**`internal/runtime/engine.go`** — add `Orchestrator` as optional field:

```go
// If Orchestrator is set and input triggers multi-step detection, use it.
// Otherwise fall back to single-agent Submit().
func (e *Engine) Submit(ctx context.Context, input string, emit agent.Emit) error {
    if e.Orchestrator != nil && isComplexGoal(input) {
        result, err := e.Orchestrator.Run(ctx, input, emit)
        // persist result
        return err
    }
    // existing single-agent path
}

// isComplexGoal heuristics: multiple sentences, contains "and then", "after that",
// "first ... then ...", or input length > 200 chars.
func isComplexGoal(input string) bool
```

Tags: `#orchestration/engine`

---

### Step 8 — State & Context Manager
**`internal/orchestration/state.go`**

Tracks orchestration state for resumability (crash recovery):

```go
type OrchestrationState struct {
    ID        string
    SessionID string
    Goal      string
    Graph     *TaskGraph
    StartedAt time.Time
    Status    string
}
```

Persisted in SQLite `orchestrations` table. On engine restart with same session,
incomplete orchestrations are resumed from last completed task.

Tags: `#orchestration/state` `#storage/sqlite`

---

### Step 9 — `/plan` slash command
**`internal/ui/tui/model.go`** — add `/plan <goal>` to show task graph before execution:

```
/plan build a web scraper for hacker news
→ Shows planned tasks with roles and dependencies
→ User confirms before execution
```

Tags: `#ui/tui` `#orchestration/cli`

---

## Tests

**`internal/orchestration/task_test.go`**
- `TestTopologicalOrder` — correct ordering for chain A→B→C
- `TestTopologicalOrderParallel` — independent tasks returned in same batch
- `TestCycleDetection` — circular dependency returns error

**`internal/orchestration/scheduler_test.go`**
- `TestRunGraphSequential` — tasks with deps run after deps complete
- `TestRunGraphParallel` — independent tasks run concurrently up to maxConcurrent
- `TestRunGraphCancelOnFailure` — failed task cancels downstream dependents

Tags: `#tests`

---

## Verification

```
go test ./internal/orchestration/...
```

- "Research Go concurrency patterns, then write a summary document" →
  Planner produces 2 tasks (Research → Communication), Scheduler runs sequentially,
  aggregator produces final response
- Kill agent mid-orchestration → state persisted → restart resumes from last done task
- `/plan` shows task graph, `/plan confirm` executes it
