package sqlite

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"

	"ollama_agent_go/internal/memory/episodic"
)

// EpisodicStore implements episodic.Store using SQLite.
type EpisodicStore struct{ db *sql.DB }

// NewEpisodicStore returns an EpisodicStore backed by the given DB.
func NewEpisodicStore(db *DB) *EpisodicStore {
	return &EpisodicStore{db: db.db}
}

func (s *EpisodicStore) Save(ctx context.Context, ep episodic.Episode) error {
	tags, err := json.Marshal(ep.Tags)
	if err != nil {
		return fmt.Errorf("marshal tags: %w", err)
	}
	_, err = s.db.ExecContext(ctx,
		`INSERT INTO episodes (id, session_id, summary, tags) VALUES (?, ?, ?, ?)
		 ON CONFLICT(id) DO UPDATE SET summary=excluded.summary, tags=excluded.tags`,
		ep.ID, ep.SessionID, ep.Summary, string(tags),
	)
	return err
}

func (s *EpisodicStore) List(ctx context.Context, limit int) ([]episodic.Episode, error) {
	rows, err := s.db.QueryContext(ctx,
		`SELECT id, session_id, summary, tags, created_at FROM episodes
		 ORDER BY created_at DESC LIMIT ?`, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanEpisodes(rows)
}

func (s *EpisodicStore) Search(ctx context.Context, tag string) ([]episodic.Episode, error) {
	rows, err := s.db.QueryContext(ctx,
		`SELECT id, session_id, summary, tags, created_at FROM episodes
		 WHERE tags LIKE ? ORDER BY created_at DESC`, "%"+tag+"%")
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanEpisodes(rows)
}

func scanEpisodes(rows *sql.Rows) ([]episodic.Episode, error) {
	var eps []episodic.Episode
	for rows.Next() {
		var ep episodic.Episode
		var tagsJSON string
		if err := rows.Scan(&ep.ID, &ep.SessionID, &ep.Summary, &tagsJSON, &ep.CreatedAt); err != nil {
			return nil, err
		}
		_ = json.Unmarshal([]byte(tagsJSON), &ep.Tags)
		eps = append(eps, ep)
	}
	return eps, rows.Err()
}
