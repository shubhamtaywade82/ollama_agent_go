package config

import (
	"os"
	"path/filepath"
)

type Config struct {
	Model   string
	BaseURL string
	Root    string
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

	return &Config{
		Model:   model,
		BaseURL: baseURL,
		Root:    filepath.Clean(root),
	}
}
