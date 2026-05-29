package orchestration

import "time"

// OrchestrationState tracks an orchestration run for resumability.
// In Phase 10 this is an in-memory snapshot; Phase 12 will persist it to SQLite.
type OrchestrationState struct {
	ID        string
	SessionID string
	Goal      string
	Graph     *TaskGraph
	StartedAt time.Time
	Status    string // "running", "done", "failed"
}
