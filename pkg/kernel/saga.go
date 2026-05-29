package kernel

import (
	"os"
	"path/filepath"
	"time"

	"github.com/google/uuid"
)

// SagaState represents the current lifecycle phase of a saga.
type SagaState string

const (
	StateReserved  SagaState = "reserved"  // Saga created, no changes yet
	StateApplied   SagaState = "applied"   // Changes applied to disk
	StateVerified  SagaState = "verified"  // Changes verified (e.g. by tests)
	StateCommitted SagaState = "committed" // Finalized, WAL entry archived
	StateAborted   SagaState = "aborted"   // Rolled back
)

// Saga tracks a logical unit of work (e.g. a multi-file refactor).
type Saga struct {
	ID        string    `json:"id"`
	StartedAt time.Time `json:"started_at"`
	State     SagaState `json:"state"`
	Mutations []string  `json:"mutations"` // IDs of WAL entries
}

// Coordinator orchestrates sagas and mutations.
type Coordinator struct {
	WAL  *WAL
	Root string
}

func NewCoordinator(root string, walDir string) (*Coordinator, error) {
	wal, err := NewWAL(walDir)
	if err != nil {
		return nil, err
	}
	return &Coordinator{WAL: wal, Root: root}, nil
}

// Begin starts a new saga.
func (c *Coordinator) Begin() *Saga {
	return &Saga{
		ID:        uuid.New().String(),
		StartedAt: time.Now(),
		State:     StateReserved,
	}
}

// Apply records a mutation in the WAL and executes it.
func (c *Coordinator) Apply(s *Saga, path string, kind MutationKind, fn func() error) error {
	abs := filepath.Join(c.Root, path)
	var original []byte
	if b, err := os.ReadFile(abs); err == nil {
		original = b
	}

	entry := LogEntry{
		ID:        uuid.New().String(),
		Timestamp: time.Now(),
		Path:      path,
		Kind:      kind,
		Original:  original,
	}

	if err := fn(); err != nil {
		entry.Success = false
		c.WAL.Record(entry)
		return err
	}

	entry.Success = true
	if err := c.WAL.Record(entry); err != nil {
		return err
	}

	s.Mutations = append(s.Mutations, entry.ID)
	s.State = StateApplied
	return nil
}

// Rollback reverses all mutations in a saga using the WAL's original content.
func (c *Coordinator) Rollback(s *Saga) error {
	entries, err := c.WAL.ReadAll()
	if err != nil {
		return err
	}

	// Map entries by ID for fast lookup.
	entryMap := make(map[string]LogEntry)
	for _, e := range entries {
		entryMap[e.ID] = e
	}

	// Apply rollbacks in reverse order.
	for i := len(s.Mutations) - 1; i >= 0; i-- {
		id := s.Mutations[i]
		e, ok := entryMap[id]
		if !ok || !e.Success {
			continue
		}

		abs := filepath.Join(c.Root, e.Path)
		if e.Original == nil {
			// File was created; delete it.
			os.Remove(abs)
		} else {
			// Restore original content.
			os.WriteFile(abs, e.Original, 0o644)
		}
	}

	s.State = StateAborted
	return nil
}
