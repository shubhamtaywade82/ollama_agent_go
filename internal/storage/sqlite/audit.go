package sqlite

import (
	"context"
	"database/sql"
	"time"

	"github.com/google/uuid"
	"ollama_agent_go/internal/observability"
)

// SQLiteAuditLogger implements observability.AuditLogger backed by SQLite.
type SQLiteAuditLogger struct {
	db *sql.DB
}

// NewAuditLogger returns an AuditLogger that persists events to db.
func NewAuditLogger(db *sql.DB) *SQLiteAuditLogger { return &SQLiteAuditLogger{db: db} }

func (a *SQLiteAuditLogger) Log(ctx context.Context, ev observability.AuditEvent) error {
	if ev.ID == "" {
		ev.ID = uuid.New().String()
	}
	if ev.At.IsZero() {
		ev.At = time.Now()
	}
	_, err := a.db.ExecContext(ctx,
		`INSERT INTO audit_log (id, session_id, actor, action, resource, result, detail, at)
		 VALUES (?,?,?,?,?,?,?,?)`,
		ev.ID, ev.SessionID, ev.Actor, ev.Action,
		nullStr(ev.Resource), ev.Result, nullStr(ev.Detail), ev.At.UTC(),
	)
	return err
}

func (a *SQLiteAuditLogger) Query(ctx context.Context, sessionID string) ([]observability.AuditEvent, error) {
	rows, err := a.db.QueryContext(ctx,
		`SELECT id, session_id, actor, action,
		        COALESCE(resource,''), result, COALESCE(detail,''), at
		 FROM audit_log WHERE session_id = ? ORDER BY at ASC`, sessionID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []observability.AuditEvent
	for rows.Next() {
		var ev observability.AuditEvent
		if err := rows.Scan(
			&ev.ID, &ev.SessionID, &ev.Actor, &ev.Action,
			&ev.Resource, &ev.Result, &ev.Detail, &ev.At,
		); err != nil {
			return nil, err
		}
		out = append(out, ev)
	}
	return out, rows.Err()
}
