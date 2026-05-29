// Package config loads runtime configuration from environment variables.
package config

import (
	"os"
	"path/filepath"
	"strconv"
)

// DefaultContextBudget is the per-request token budget for sliding-window
// trimming when OLLAMA_AGENT_CONTEXT_BUDGET is unset.
const DefaultContextBudget = 8000

// DefaultMaxIterations is the default think-act loop cap.
const DefaultMaxIterations = 12

// Config holds all runtime configuration.
type Config struct {
	Model   string
	BaseURL string
	Root    string
	// SkillsDir holds Markdown skill files injected into the system prompt.
	SkillsDir string
	// ContextBudget caps tokens per request (sliding-window trimming).
	ContextBudget int
	// MaxIterations caps the think-act loop iterations per request.
	MaxIterations int
	// DBPath is the SQLite database file path.
	DBPath string
}

// Load reads configuration from environment variables, applying defaults.
func Load() *Config {
	model := os.Getenv("OLLAMA_AGENT_MODEL")
	if model == "" {
		model = "qwen3.5:4b"
	}

	baseURL := os.Getenv("OLLAMA_BASE_URL")
	if baseURL == "" {
		baseURL = "http://localhost:11434"
	}

	root, _ := os.Getwd()
	if envRoot := os.Getenv("OLLAMA_AGENT_ROOT"); envRoot != "" {
		root = envRoot
	}
	root = filepath.Clean(root)

	budget := DefaultContextBudget
	if v := os.Getenv("OLLAMA_AGENT_CONTEXT_BUDGET"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n >= 0 {
			budget = n
		}
	}

	maxIter := DefaultMaxIterations
	if v := os.Getenv("OLLAMA_AGENT_MAX_ITER"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 {
			maxIter = n
		}
	}

	skillsDir := os.Getenv("OLLAMA_AGENT_SKILLS_DIR")
	if skillsDir == "" {
		skillsDir = os.Getenv("OLLAMA_AGENT_SKILLS")
	}
	if skillsDir == "" {
		skillsDir = filepath.Join(root, ".ollama_agent", "skills")
	}

	dbPath := os.Getenv("OLLAMA_AGENT_DB")
	if dbPath == "" {
		dbPath = filepath.Join(root, "ollama_agent.db")
	}

	return &Config{
		Model:         model,
		BaseURL:       baseURL,
		Root:          root,
		SkillsDir:     skillsDir,
		ContextBudget: budget,
		MaxIterations: maxIter,
		DBPath:        dbPath,
	}
}
