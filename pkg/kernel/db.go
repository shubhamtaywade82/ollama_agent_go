package kernel

import (
	"database/sql"
	"fmt"
	"os"
	"path/filepath"

	_ "modernc.org/sqlite"
)

// DB manages the SQLite database for session and mutation persistence.
type DB struct {
	conn *sql.DB
}

func OpenDB(dir string) (*DB, error) {
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return nil, err
	}
	dbPath := filepath.Join(dir, "agent.db")
	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		return nil, err
	}

	if err := db.Ping(); err != nil {
		return nil, err
	}

	if err := migrate(db); err != nil {
		return nil, err
	}

	return &DB{conn: db}, nil
}

func (db *DB) Close() error {
	return db.conn.Close()
}

func migrate(db *sql.DB) error {
	schemas := []string{
		`CREATE TABLE IF NOT EXISTS sessions (
			id TEXT PRIMARY KEY,
			model TEXT,
			started_at DATETIME,
			ended_at DATETIME
		)`,
		`CREATE TABLE IF NOT EXISTS mutations (
			id TEXT PRIMARY KEY,
			session_id TEXT,
			path TEXT,
			kind TEXT,
			success BOOLEAN,
			timestamp DATETIME,
			FOREIGN KEY(session_id) REFERENCES sessions(id)
		)`,
		`CREATE TABLE IF NOT EXISTS cost_ledger (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			session_id TEXT,
			provider TEXT,
			model TEXT,
			input_tokens INTEGER,
			output_tokens INTEGER,
			cost REAL,
			timestamp DATETIME,
			FOREIGN KEY(session_id) REFERENCES sessions(id)
		)`,
	}

	for _, s := range schemas {
		if _, err := db.Exec(s); err != nil {
			return fmt.Errorf("migration failed: %w", err)
		}
	}
	return nil
}
