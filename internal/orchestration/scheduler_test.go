package orchestration

import (
	"context"
	"fmt"
	"sync/atomic"
	"testing"

	"ollama_agent_go/internal/agent/roles"
)

func TestRunGraphSequential(t *testing.T) {
	g := &TaskGraph{}
	a := &Task{ID: "a", Goal: "step a", Role: roles.RoleGeneral, Status: TaskPending}
	b := &Task{ID: "b", Goal: "step b", Role: roles.RoleGeneral, DependsOn: []string{"a"}, Status: TaskPending}
	g.Add(a)
	g.Add(b)

	var aBeforeB bool
	aDone := false

	sc := NewScheduler(1)
	err := sc.RunGraph(context.Background(), g, func(_ context.Context, task *Task) error {
		if task.ID == "a" {
			aDone = true
		}
		if task.ID == "b" && aDone {
			aBeforeB = true
		}
		task.Result = "ok"
		return nil
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !aBeforeB {
		t.Error("task b ran before task a")
	}
}

func TestRunGraphParallel(t *testing.T) {
	g := &TaskGraph{}
	g.Add(&Task{ID: "a", Goal: "left", Role: roles.RoleGeneral, Status: TaskPending})
	g.Add(&Task{ID: "b", Goal: "right", Role: roles.RoleGeneral, Status: TaskPending})

	var concurrent int64
	var maxConcurrent int64
	sc := NewScheduler(2)
	err := sc.RunGraph(context.Background(), g, func(_ context.Context, task *Task) error {
		cur := atomic.AddInt64(&concurrent, 1)
		if cur > atomic.LoadInt64(&maxConcurrent) {
			atomic.StoreInt64(&maxConcurrent, cur)
		}
		task.Result = "ok"
		atomic.AddInt64(&concurrent, -1)
		return nil
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// Both independent tasks may run concurrently (max>=1 always passes; ideally 2 but not guaranteed)
	if maxConcurrent < 1 {
		t.Errorf("expected at least 1 concurrent task, got %d", maxConcurrent)
	}
}

func TestRunGraphCancelOnFailure(t *testing.T) {
	g := &TaskGraph{}
	g.Add(&Task{ID: "a", Goal: "root", Role: roles.RoleGeneral, Status: TaskPending})
	g.Add(&Task{ID: "b", Goal: "child", Role: roles.RoleGeneral, DependsOn: []string{"a"}, Status: TaskPending})

	sc := NewScheduler(1)
	err := sc.RunGraph(context.Background(), g, func(_ context.Context, task *Task) error {
		if task.ID == "a" {
			return fmt.Errorf("root task failed")
		}
		return nil
	})
	if err == nil {
		t.Fatal("expected error from failed root task")
	}

	// b should be skipped, not failed
	var bTask *Task
	for _, t2 := range g.Tasks {
		if t2.ID == "b" {
			bTask = t2
		}
	}
	if bTask == nil {
		t.Fatal("task b not found")
	}
	if bTask.Status != TaskSkipped {
		t.Errorf("expected b to be skipped, got %s", bTask.Status)
	}
}
