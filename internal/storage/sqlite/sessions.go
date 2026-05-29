package sqlite

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"time"

	"ollama_agent_go/internal/storage"
	"ollama_agent_go/internal/types"
)

// Store implements storage.SessionStore on top of a SQLite database.
type Store struct {
	db *DB
}

// NewStore creates a Store using an already-opened DB.
func NewStore(db *DB) *Store {
	return &Store{db: db}
}

func (s *Store) Close() error { return s.db.Close() }

// Create inserts a new session record.
func (s *Store) Create(ctx context.Context, id, model string) (*storage.Session, error) {
	now := time.Now().UTC()
	_, err := s.db.db.ExecContext(ctx,
		`INSERT INTO sessions(id, state, model, created_at, updated_at) VALUES(?,?,?,?,?)`,
		id, string(storage.StateIdle), model, now, now,
	)
	if err != nil {
		return nil, fmt.Errorf("create session: %w", err)
	}
	return &storage.Session{
		ID:        id,
		State:     storage.StateIdle,
		Model:     model,
		CreatedAt: now,
		UpdatedAt: now,
	}, nil
}

// Get loads a session and reconstructs its conversation from the messages table.
func (s *Store) Get(ctx context.Context, id string) (*storage.Session, error) {
	row := s.db.db.QueryRowContext(ctx,
		`SELECT id, state, model, created_at, updated_at FROM sessions WHERE id=?`, id,
	)
	var sess storage.Session
	var createdAt, updatedAt string
	if err := row.Scan(&sess.ID, (*string)(&sess.State), &sess.Model, &createdAt, &updatedAt); err != nil {
		if err == sql.ErrNoRows {
			return nil, fmt.Errorf("session %q not found", id)
		}
		return nil, fmt.Errorf("get session: %w", err)
	}
	sess.CreatedAt, _ = time.Parse("2006-01-02T15:04:05Z", createdAt)
	sess.UpdatedAt, _ = time.Parse("2006-01-02T15:04:05Z", updatedAt)

	rows, err := s.db.db.QueryContext(ctx,
		`SELECT role, content FROM messages WHERE session_id=? ORDER BY seq ASC`, id,
	)
	if err != nil {
		return nil, fmt.Errorf("load messages: %w", err)
	}
	defer rows.Close()
	for rows.Next() {
		var msg types.Message
		if err := rows.Scan(&msg.Role, &msg.Content); err != nil {
			return nil, fmt.Errorf("scan message: %w", err)
		}
		sess.Conversation = append(sess.Conversation, msg)
	}
	return &sess, rows.Err()
}

// SetState updates the session state and bumps updated_at.
func (s *Store) SetState(ctx context.Context, id string, state storage.SessionState) error {
	_, err := s.db.db.ExecContext(ctx,
		`UPDATE sessions SET state=?, updated_at=? WHERE id=?`,
		string(state), time.Now().UTC(), id,
	)
	return err
}

// AppendMessage inserts a single message, assigning the next sequence number.
func (s *Store) AppendMessage(ctx context.Context, sessionID string, msg types.Message) error {
	var seq int
	row := s.db.db.QueryRowContext(ctx,
		`SELECT COALESCE(MAX(seq),0)+1 FROM messages WHERE session_id=?`, sessionID,
	)
	if err := row.Scan(&seq); err != nil {
		return fmt.Errorf("next seq: %w", err)
	}
	_, err := s.db.db.ExecContext(ctx,
		`INSERT INTO messages(session_id, role, content, seq, created_at) VALUES(?,?,?,?,?)`,
		sessionID, msg.Role, msg.Content, seq, time.Now().UTC(),
	)
	if err != nil {
		return fmt.Errorf("append message: %w", err)
	}
	_, _ = s.db.db.ExecContext(ctx,
		`UPDATE sessions SET updated_at=? WHERE id=?`, time.Now().UTC(), sessionID,
	)
	return nil
}

// RecordToolInvocation inserts a completed tool invocation record.
func (s *Store) RecordToolInvocation(ctx context.Context, rec storage.ToolInvocationRecord) error {
	argsJSON, _ := json.Marshal(rec.Args)
	approved := 0
	if rec.Approved {
		approved = 1
	}
	_, err := s.db.db.ExecContext(ctx,
		`INSERT INTO tool_invocations(id, session_id, tool, args, output, error, duration_ms, approved, created_at)
		 VALUES(?,?,?,?,?,?,?,?,?)`,
		rec.ID, rec.SessionID, rec.Tool, string(argsJSON),
		rec.Output, rec.Error, rec.DurationMs, approved, rec.CreatedAt.UTC(),
	)
	return err
}

// RecordTokens appends a token usage row to the ledger.
func (s *Store) RecordTokens(ctx context.Context, sessionID, model string, prompt, output int) error {
	_, err := s.db.db.ExecContext(ctx,
		`INSERT INTO token_ledger(session_id, model, prompt_tokens, output_tokens, created_at) VALUES(?,?,?,?,?)`,
		sessionID, model, prompt, output, time.Now().UTC(),
	)
	return err
}
