package sqlite

import (
	"context"
	"database/sql"
	"time"

	"github.com/google/uuid"
	"ollama_agent_go/internal/observability"
)

// SQLiteTracer implements observability.Tracer backed by SQLite.
type SQLiteTracer struct {
	db *sql.DB
}

// NewTracer returns a Tracer that persists spans to db.
func NewTracer(db *sql.DB) *SQLiteTracer { return &SQLiteTracer{db: db} }

func (t *SQLiteTracer) Start(ctx context.Context, kind observability.SpanKind, sessionID string) (context.Context, *observability.Span) {
	parent := observability.SpanFromContext(ctx)
	s := &observability.Span{
		ID:        uuid.New().String(),
		Kind:      kind,
		SessionID: sessionID,
		StartedAt: time.Now(),
	}
	if parent != nil {
		s.ParentID = parent.ID
	}
	return observability.ContextWithSpan(ctx, s), s
}

func (t *SQLiteTracer) Finish(_ context.Context, span *observability.Span, err error) {
	if span == nil {
		return
	}
	span.EndedAt = time.Now()
	span.DurationMs = span.EndedAt.Sub(span.StartedAt).Milliseconds()
	if err != nil {
		span.Error = err.Error()
	}

	_, dbErr := t.db.Exec(
		`INSERT INTO spans
		    (id, parent_id, session_id, kind, model, tool, step, duration_ms, tokens, error, started_at, ended_at)
		 VALUES (?,?,?,?,?,?,?,?,?,?,?,?)`,
		span.ID, nullStr(span.ParentID), span.SessionID, string(span.Kind),
		nullStr(span.Model), nullStr(span.Tool), span.Step,
		span.DurationMs, span.Tokens, nullStr(span.Error),
		span.StartedAt.UTC(), span.EndedAt.UTC(),
	)
	_ = dbErr // span loss is non-fatal
}

func (t *SQLiteTracer) Export(ctx context.Context, sessionID string) ([]observability.Span, error) {
	rows, err := t.db.QueryContext(ctx,
		`SELECT id, COALESCE(parent_id,''), session_id, kind,
		        COALESCE(model,''), COALESCE(tool,''), step,
		        COALESCE(duration_ms,0), COALESCE(tokens,0), COALESCE(error,''),
		        started_at, ended_at
		 FROM spans WHERE session_id = ? ORDER BY started_at ASC`, sessionID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []observability.Span
	for rows.Next() {
		var s observability.Span
		var kind string
		if err := rows.Scan(
			&s.ID, &s.ParentID, &s.SessionID, &kind,
			&s.Model, &s.Tool, &s.Step,
			&s.DurationMs, &s.Tokens, &s.Error,
			&s.StartedAt, &s.EndedAt,
		); err != nil {
			return nil, err
		}
		s.Kind = observability.SpanKind(kind)
		out = append(out, s)
	}
	return out, rows.Err()
}

func nullStr(s string) any {
	if s == "" {
		return nil
	}
	return s
}
