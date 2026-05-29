// Package config loads runtime configuration from environment variables.
package config

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strconv"
)

// MCPServer describes an external MCP server to connect to at startup.
type MCPServer struct {
	Name      string   `json:"name"`
	Transport string   `json:"transport"` // "stdio" | "http"
	Command   string   `json:"command"`   // for stdio
	Args      []string `json:"args"`
	URL       string   `json:"url"` // for http
}

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
	// MCPServers is the list of external MCP servers to connect to on startup.
	MCPServers []MCPServer
	// MCPServerMode, when true, exposes our tools as an MCP server on stdio.
	MCPServerMode bool
	// RAG configures the retrieval-augmented generation pipeline.
	RAG RAGConfig
}

// RAGConfig controls the embedded knowledge retrieval pipeline.
type RAGConfig struct {
	Enabled      bool
	EmbedModel   string // e.g. "nomic-embed-text"
	ChunkSize    int
	ChunkOverlap int
	TopK         int
	StorePath    string // path to chromem DB directory
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

	mcpServers := loadMCPConfig(filepath.Join(root, "mcp.json"))

	ragEnabled := os.Getenv("OLLAMA_AGENT_RAG") == "1"
	ragModel := os.Getenv("OLLAMA_AGENT_EMBED_MODEL")
	if ragModel == "" {
		ragModel = "nomic-embed-text"
	}
	ragStorePath := os.Getenv("OLLAMA_AGENT_KNOWLEDGE_PATH")
	if ragStorePath == "" {
		ragStorePath = filepath.Join(root, ".knowledge")
	}
	ragTopK := 5

	return &Config{
		Model:         model,
		BaseURL:       baseURL,
		Root:          root,
		SkillsDir:     skillsDir,
		ContextBudget: budget,
		MaxIterations: maxIter,
		DBPath:        dbPath,
		MCPServers:    mcpServers,
		MCPServerMode: os.Getenv("OLLAMA_AGENT_MCP_SERVER") == "1",
		RAG: RAGConfig{
			Enabled:      ragEnabled,
			EmbedModel:   ragModel,
			ChunkSize:    256,
			ChunkOverlap: 32,
			TopK:         ragTopK,
			StorePath:    ragStorePath,
		},
	}
}

// loadMCPConfig reads an mcp.json file if present.
func loadMCPConfig(path string) []MCPServer {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil
	}
	var cfg struct {
		MCPServers []MCPServer `json:"mcpServers"`
	}
	if err := json.Unmarshal(data, &cfg); err != nil {
		return nil
	}
	return cfg.MCPServers
}
