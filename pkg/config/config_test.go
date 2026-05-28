package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadDefaults(t *testing.T) {
	for _, k := range []string{"OLLAMA_AGENT_MODEL", "OLLAMA_BASE_URL", "OLLAMA_AGENT_ROOT"} {
		t.Setenv(k, "")
	}
	// t.Setenv with "" still sets the var; unset explicitly for default path.
	os.Unsetenv("OLLAMA_AGENT_MODEL")
	os.Unsetenv("OLLAMA_BASE_URL")
	os.Unsetenv("OLLAMA_AGENT_ROOT")

	cfg := Load()
	if cfg.Model != "qwen3.5:4b" {
		t.Errorf("default model = %q, want qwen3.5:4b", cfg.Model)
	}
	if cfg.BaseURL != "http://localhost:11434" {
		t.Errorf("default baseURL = %q", cfg.BaseURL)
	}
	if cfg.Root == "" {
		t.Error("root should default to cwd, got empty")
	}
}

func TestLoadFromEnv(t *testing.T) {
	t.Setenv("OLLAMA_AGENT_MODEL", "llama3:8b")
	t.Setenv("OLLAMA_BASE_URL", "http://remote:1234")
	t.Setenv("OLLAMA_AGENT_ROOT", "/tmp/some/../root")

	cfg := Load()
	if cfg.Model != "llama3:8b" {
		t.Errorf("model = %q", cfg.Model)
	}
	if cfg.BaseURL != "http://remote:1234" {
		t.Errorf("baseURL = %q", cfg.BaseURL)
	}
	if want := filepath.Clean("/tmp/some/../root"); cfg.Root != want {
		t.Errorf("root = %q, want %q", cfg.Root, want)
	}
}
