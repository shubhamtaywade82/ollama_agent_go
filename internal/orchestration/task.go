// Package orchestration provides the workflow engine: planner, DAG scheduler,
// per-task executor, and result aggregator.
package orchestration

import (
	"fmt"
	"time"

	"ollama_agent_go/internal/agent/roles"
)

// TaskStatus represents the lifecycle state of a single Task.
type TaskStatus int

const (
	TaskPending TaskStatus = iota
	TaskRunning
	TaskDone
	TaskFailed
	TaskSkipped
)

func (s TaskStatus) String() string {
	switch s {
	case TaskPending:
		return "pending"
	case TaskRunning:
		return "running"
	case TaskDone:
		return "done"
	case TaskFailed:
		return "failed"
	case TaskSkipped:
		return "skipped"
	default:
		return "unknown"
	}
}

// Task is one unit of work in an orchestrated goal.
type Task struct {
	ID          string
	Goal        string
	Role        roles.Role
	DependsOn   []string // IDs of tasks that must be Done before this runs
	Status      TaskStatus
	Result      string
	Error       error
	StartedAt   time.Time
	CompletedAt time.Time

	// depResults is populated by the Orchestrator before ExecTask is called.
	// It contains the results of completed dependency tasks for context injection.
	depResults []depResult
}

// depResult holds an upstream task result for context injection.
type depResult struct {
	taskID string
	result string
}

// TaskGraph is a directed acyclic graph of Tasks.
type TaskGraph struct {
	Tasks []*Task
	index map[string]*Task
}

// Add inserts a task into the graph. Duplicate IDs are silently ignored.
func (g *TaskGraph) Add(t *Task) {
	if g.index == nil {
		g.index = make(map[string]*Task)
	}
	if _, exists := g.index[t.ID]; exists {
		return
	}
	g.Tasks = append(g.Tasks, t)
	g.index[t.ID] = t
}

// byID returns the task with the given ID, or nil if not found.
func (g *TaskGraph) byID(id string) *Task {
	if g.index != nil {
		return g.index[id]
	}
	return nil
}

// TopologicalOrder returns tasks in a valid execution order (Kahn's algorithm).
// Returns an error if the graph contains a cycle.
func (g *TaskGraph) TopologicalOrder() ([]*Task, error) {
	inDegree := make(map[string]int, len(g.Tasks))
	for _, t := range g.Tasks {
		if _, ok := inDegree[t.ID]; !ok {
			inDegree[t.ID] = 0
		}
		for _, dep := range t.DependsOn {
			inDegree[t.ID]++
			_ = dep
		}
	}
	// rebuild with proper counting
	inDegree = make(map[string]int, len(g.Tasks))
	for _, t := range g.Tasks {
		inDegree[t.ID] = len(t.DependsOn)
	}

	queue := make([]*Task, 0, len(g.Tasks))
	for _, t := range g.Tasks {
		if inDegree[t.ID] == 0 {
			queue = append(queue, t)
		}
	}

	result := make([]*Task, 0, len(g.Tasks))
	for len(queue) > 0 {
		cur := queue[0]
		queue = queue[1:]
		result = append(result, cur)

		for _, candidate := range g.Tasks {
			for _, dep := range candidate.DependsOn {
				if dep == cur.ID {
					inDegree[candidate.ID]--
					if inDegree[candidate.ID] == 0 {
						queue = append(queue, candidate)
					}
				}
			}
		}
	}

	if len(result) != len(g.Tasks) {
		return nil, fmt.Errorf("orchestration: task graph contains a cycle")
	}
	return result, nil
}

// Ready returns all tasks whose dependencies are all Done and whose own status
// is still Pending — i.e. ready to start executing right now.
func (g *TaskGraph) Ready() []*Task {
	var out []*Task
	for _, t := range g.Tasks {
		if t.Status != TaskPending {
			continue
		}
		ready := true
		for _, depID := range t.DependsOn {
			dep := g.byID(depID)
			if dep == nil || dep.Status != TaskDone {
				ready = false
				break
			}
		}
		if ready {
			out = append(out, t)
		}
	}
	return out
}

// AllDone reports whether every task has a terminal status.
func (g *TaskGraph) AllDone() bool {
	for _, t := range g.Tasks {
		if t.Status == TaskPending || t.Status == TaskRunning {
			return false
		}
	}
	return true
}
