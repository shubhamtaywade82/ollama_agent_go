package config

import (
	"os"
	"path/filepath"
	"strconv"
)

// DefaultContextBudget is the per-request token budget used for sliding-window
// trimming when OLLAMA_AGENT_CONTEXT_TOKENS is unset.
const DefaultContextBudget = 8192

type Config struct {
	Model   string
	BaseURL string
	Root    string
	// ContextBudget caps tokens per request (sliding-window trimming).
	ContextBudget int
	// SkillsDir holds Markdown skill files injected into the system prompt.
	SkillsDir string
}

func Load() *Config {
	model := os.Getenv("OLLAMA_AGENT_MODEL")
	if model == "" {
		model = "qwen2.5-coder:latest"
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
	if v := os.Getenv("OLLAMA_AGENT_CONTEXT_TOKENS"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n >= 0 {
			budget = n
		}
	}

	skillsDir := os.Getenv("OLLAMA_AGENT_SKILLS")
	if skillsDir == "" {
		skillsDir = filepath.Join(root, ".ollama_agent", "skills")
	}

	return &Config{
		Model:         model,
		BaseURL:       baseURL,
		Root:          root,
		ContextBudget: budget,
		SkillsDir:     skillsDir,
	}
}
