package sqlite

import (
	"context"
	"database/sql"

	"ollama_agent_go/internal/memory/profile"
)

// ProfileStore implements profile.Store using SQLite.
type ProfileStore struct{ db *sql.DB }

// NewProfileStore returns a ProfileStore backed by the given DB.
func NewProfileStore(db *DB) *ProfileStore {
	return &ProfileStore{db: db.db}
}

func (s *ProfileStore) Set(ctx context.Context, key, value string) error {
	_, err := s.db.ExecContext(ctx,
		`INSERT INTO profile (key, value) VALUES (?, ?)
		 ON CONFLICT(key) DO UPDATE SET value=excluded.value, updated_at=CURRENT_TIMESTAMP`,
		key, value,
	)
	return err
}

func (s *ProfileStore) Get(ctx context.Context, key string) (string, bool, error) {
	var value string
	err := s.db.QueryRowContext(ctx,
		`SELECT value FROM profile WHERE key = ?`, key,
	).Scan(&value)
	if err == sql.ErrNoRows {
		return "", false, nil
	}
	if err != nil {
		return "", false, err
	}
	return value, true, nil
}

func (s *ProfileStore) All(ctx context.Context) ([]profile.Entry, error) {
	rows, err := s.db.QueryContext(ctx,
		`SELECT key, value, updated_at FROM profile ORDER BY key`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var entries []profile.Entry
	for rows.Next() {
		var e profile.Entry
		if err := rows.Scan(&e.Key, &e.Value, &e.UpdatedAt); err != nil {
			return nil, err
		}
		entries = append(entries, e)
	}
	return entries, rows.Err()
}
