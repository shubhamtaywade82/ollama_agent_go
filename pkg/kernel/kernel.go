package kernel

import (
	"path/filepath"
	"time"

	"github.com/google/uuid"
	"ollama_agent_go/pkg/ollama"
	"ollama_agent_go/pkg/provider"
)

// Kernel is the top-level reliability and persistence layer.
type Kernel struct {
	DB          *DB
	Coordinator *Coordinator
	Gate        *Gate
	SessionID   string
}

func New(root string) (*Kernel, error) {
	dataDir := filepath.Join(root, ".ollama_agent", "data")
	db, err := OpenDB(dataDir)
	if err != nil {
		return nil, err
	}
	coord, err := NewCoordinator(root, dataDir)
	if err != nil {
		return nil, err
	}
	gate := NewGate(root)

	sessionID := uuid.New().String()
	_, err = db.conn.Exec("INSERT INTO sessions (id, started_at) VALUES (?, ?)", sessionID, time.Now())
	if err != nil {
		return nil, err
	}

	return &Kernel{
		DB:          db,
		Coordinator: coord,
		Gate:        gate,
		SessionID:   sessionID,
	}, nil
}

func (k *Kernel) IsAllowed(path string) bool {
	return k.Gate.IsAllowed(path)
}

func (k *Kernel) Close() error {
	_, _ = k.DB.conn.Exec("UPDATE sessions SET ended_at = ? WHERE id = ?", time.Now(), k.SessionID)
	return k.DB.Close()
}

func (k *Kernel) RecordMutation(id, path, kind string, success bool) error {
	_, err := k.DB.conn.Exec(
		"INSERT INTO mutations (id, session_id, path, kind, success, timestamp) VALUES (?, ?, ?, ?, ?, ?)",
		id, k.SessionID, path, kind, success, time.Now(),
	)
	return err
}

func (k *Kernel) RecordCost(p provider.Provider, resp ollama.ChatResponse) error {
	pricing := p.Pricing()
	cost := pricing.Cost(resp.PromptTokens, resp.CompletionTokens)
	_, err := k.DB.conn.Exec(
		"INSERT INTO cost_ledger (session_id, provider, model, input_tokens, output_tokens, cost, timestamp) VALUES (?, ?, ?, ?, ?, ?, ?)",
		k.SessionID, p.Name(), resp.Model, resp.PromptTokens, resp.CompletionTokens, cost, time.Now(),
	)
	return err
}

func (k *Kernel) GetTotalCost() (float64, error) {
	var total float64
	err := k.DB.conn.QueryRow("SELECT SUM(cost) FROM cost_ledger WHERE session_id = ?", k.SessionID).Scan(&total)
	return total, err
}
