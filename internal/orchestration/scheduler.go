package orchestration

import (
	"context"
	"fmt"
	"sync"
)

// Scheduler executes a TaskGraph with bounded concurrency.
type Scheduler struct {
	maxConcurrent int
}

// NewScheduler creates a Scheduler. maxConcurrent<=0 defaults to 1 (sequential).
func NewScheduler(maxConcurrent int) *Scheduler {
	if maxConcurrent <= 0 {
		maxConcurrent = 1
	}
	return &Scheduler{maxConcurrent: maxConcurrent}
}

// Next returns tasks that are ready to run right now (all deps Done, status Pending).
func (s *Scheduler) Next(g *TaskGraph) []*Task { return g.Ready() }

// RunGraph executes the graph calling execFn for each task in dependency order.
// Up to s.maxConcurrent tasks may run concurrently.
// If a task fails, its downstream dependents are marked TaskSkipped.
func (s *Scheduler) RunGraph(ctx context.Context, g *TaskGraph, execFn func(context.Context, *Task) error) error {
	sem := make(chan struct{}, s.maxConcurrent)
	var mu sync.Mutex

	for !g.AllDone() {
		if ctx.Err() != nil {
			return ctx.Err()
		}

		mu.Lock()
		batch := g.Ready()
		if len(batch) == 0 {
			// Check for blocked state: all remaining tasks failed/skipped or stuck.
			stuck := true
			for _, t := range g.Tasks {
				if t.Status == TaskPending || t.Status == TaskRunning {
					stuck = false
					break
				}
			}
			mu.Unlock()
			if stuck {
				break
			}
			// Tasks are still running; yield CPU and retry.
			mu.Lock()
			mu.Unlock()
			continue
		}

		// Mark batch as Running while still holding mu.
		for _, t := range batch {
			t.Status = TaskRunning
		}
		mu.Unlock()

		var wg sync.WaitGroup
		for _, t := range batch {
			t := t
			sem <- struct{}{}
			wg.Add(1)
			go func() {
				defer func() { <-sem; wg.Done() }()
				err := execFn(ctx, t)
				mu.Lock()
				defer mu.Unlock()
				if err != nil {
					t.Status = TaskFailed
					t.Error = err
					skipDownstream(g, t.ID)
				} else {
					t.Status = TaskDone
				}
			}()
		}
		wg.Wait()
	}

	// Report first failure if any.
	for _, t := range g.Tasks {
		if t.Status == TaskFailed {
			return fmt.Errorf("task %q failed: %w", t.ID, t.Error)
		}
	}
	return nil
}

// skipDownstream marks all transitive dependents of failedID as TaskSkipped.
func skipDownstream(g *TaskGraph, failedID string) {
	for _, t := range g.Tasks {
		if t.Status != TaskPending {
			continue
		}
		for _, dep := range t.DependsOn {
			if dep == failedID {
				t.Status = TaskSkipped
				skipDownstream(g, t.ID)
				break
			}
		}
	}
}
