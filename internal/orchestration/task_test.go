package orchestration

import (
	"testing"

	"ollama_agent_go/internal/agent/roles"
)

func makeTask(id string, deps ...string) *Task {
	return &Task{ID: id, Goal: "do " + id, Role: roles.RoleGeneral, DependsOn: deps, Status: TaskPending}
}

func TestTopologicalOrder(t *testing.T) {
	g := &TaskGraph{}
	g.Add(makeTask("a"))
	g.Add(makeTask("b", "a"))
	g.Add(makeTask("c", "b"))

	order, err := g.TopologicalOrder()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(order) != 3 {
		t.Fatalf("expected 3 tasks, got %d", len(order))
	}
	idx := map[string]int{}
	for i, t := range order {
		idx[t.ID] = i
	}
	if idx["a"] >= idx["b"] || idx["b"] >= idx["c"] {
		t.Errorf("bad order: a=%d b=%d c=%d", idx["a"], idx["b"], idx["c"])
	}
}

func TestTopologicalOrderParallel(t *testing.T) {
	g := &TaskGraph{}
	g.Add(makeTask("root"))
	g.Add(makeTask("left", "root"))
	g.Add(makeTask("right", "root"))

	order, err := g.TopologicalOrder()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(order) != 3 {
		t.Fatalf("expected 3, got %d", len(order))
	}
	// root must come first
	if order[0].ID != "root" {
		t.Errorf("root should be first, got %s", order[0].ID)
	}
	// left and right may appear in any order
	rest := map[string]bool{order[1].ID: true, order[2].ID: true}
	if !rest["left"] || !rest["right"] {
		t.Errorf("expected left and right in positions 1-2, got %v", rest)
	}
}

func TestCycleDetection(t *testing.T) {
	g := &TaskGraph{}
	g.Add(makeTask("a", "b"))
	g.Add(makeTask("b", "a"))

	_, err := g.TopologicalOrder()
	if err == nil {
		t.Fatal("expected cycle detection error, got nil")
	}
}

func TestReady(t *testing.T) {
	g := &TaskGraph{}
	a := makeTask("a")
	b := makeTask("b", "a")
	g.Add(a)
	g.Add(b)

	ready := g.Ready()
	if len(ready) != 1 || ready[0].ID != "a" {
		t.Errorf("expected only 'a' ready, got %v", ready)
	}

	a.Status = TaskDone
	ready = g.Ready()
	if len(ready) != 1 || ready[0].ID != "b" {
		t.Errorf("expected only 'b' ready after a done, got %v", ready)
	}
}
