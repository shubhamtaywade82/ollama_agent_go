package sqlite

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"ollama_agent_go/internal/storage"
)

// Reserve inserts a new mutation record in the reserved state.
func (s *Store) Reserve(ctx context.Context, m storage.Mutation) error {
	args, err := json.Marshal(m.Args)
	if err != nil {
		return fmt.Errorf("marshal args: %w", err)
	}
	now := time.Now().UTC()
	_, err = s.db.db.ExecContext(ctx,
		`INSERT INTO mutations(id, session_id, tool, args, before_state, status, created_at, updated_at)
		 VALUES(?,?,?,?,?,?,?,?)`,
		m.ID, m.SessionID, m.Tool, string(args), m.BeforeState,
		string(storage.SagaReserved), now, now,
	)
	return err
}

// Transition updates a mutation's status and, if after is non-nil, its after_state.
func (s *Store) Transition(ctx context.Context, id string, to storage.SagaStatus, after []byte) error {
	now := time.Now().UTC()
	if after != nil {
		_, err := s.db.db.ExecContext(ctx,
			`UPDATE mutations SET status=?, after_state=?, updated_at=? WHERE id=?`,
			string(to), after, now, id,
		)
		return err
	}
	_, err := s.db.db.ExecContext(ctx,
		`UPDATE mutations SET status=?, updated_at=? WHERE id=?`,
		string(to), now, id,
	)
	return err
}

// Compensable returns applied and committed mutations for a session, oldest first.
func (s *Store) Compensable(ctx context.Context, sessionID string) ([]storage.Mutation, error) {
	rows, err := s.db.db.QueryContext(ctx,
		`SELECT id, session_id, tool, args, before_state, after_state, status, created_at, updated_at
		 FROM mutations
		 WHERE session_id=? AND status IN ('applied','committed')
		 ORDER BY created_at ASC`,
		sessionID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []storage.Mutation
	for rows.Next() {
		var m storage.Mutation
		var argsJSON string
		var statusStr string
		if err := rows.Scan(
			&m.ID, &m.SessionID, &m.Tool, &argsJSON,
			&m.BeforeState, &m.AfterState,
			&statusStr, &m.CreatedAt, &m.UpdatedAt,
		); err != nil {
			return nil, err
		}
		_ = json.Unmarshal([]byte(argsJSON), &m.Args)
		m.Status = storage.SagaStatus(statusStr)
		out = append(out, m)
	}
	return out, rows.Err()
}
