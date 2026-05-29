package kernel

import (
	"encoding/json"
	"os"
	"path/filepath"
	"time"
)

// MutationKind describes the type of change recorded in the WAL.
type MutationKind string

const (
	MutationWrite MutationKind = "write"
	MutationEdit  MutationKind = "edit"
	MutationDelete MutationKind = "delete"
)

// LogEntry is a single record in the Write-Ahead Log.
type LogEntry struct {
	ID        string       `json:"id"`
	Timestamp time.Time    `json:"timestamp"`
	Path      string       `json:"path"`
	Kind      MutationKind `json:"kind"`
	// Original is the file content BEFORE the mutation, used for rollback.
	Original []byte `json:"original,omitempty"`
	// Success marks if the mutation was successfully applied.
	Success bool `json:"success"`
}

// WAL manages the write-ahead log file.
type WAL struct {
	path string
}

func NewWAL(dir string) (*WAL, error) {
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return nil, err
	}
	return &WAL{path: filepath.Join(dir, "wal.log")}, nil
}

// Record appends a new entry to the log.
func (w *WAL) Record(entry LogEntry) error {
	f, err := os.OpenFile(w.path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
	if err != nil {
		return err
	}
	defer f.Close()

	b, err := json.Marshal(entry)
	if err != nil {
		return err
	}
	_, err = f.Write(append(b, '\n'))
	return err
}

// ReadAll returns all entries in the log.
func (w *WAL) ReadAll() ([]LogEntry, error) {
	b, err := os.ReadFile(w.path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}

	var entries []LogEntry
	lines := splitLines(string(b))
	for _, line := range lines {
		if line == "" {
			continue
		}
		var entry LogEntry
		if err := json.Unmarshal([]byte(line), &entry); err != nil {
			continue
		}
		entries = append(entries, entry)
	}
	return entries, nil
}

func splitLines(s string) []string {
	var lines []string
	start := 0
	for i := 0; i < len(s); i++ {
		if s[i] == '\n' {
			lines = append(lines, s[start:i])
			start = i + 1
		}
	}
	if start < len(s) {
		lines = append(lines, s[start:])
	}
	return lines
}
