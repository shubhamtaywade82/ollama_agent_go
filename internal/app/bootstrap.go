// Package app wires all internal packages into a runnable application.
package app

import (
	"context"
	"fmt"
	"io"
	"os"

	"ollama_agent_go/internal/agent"
	"ollama_agent_go/internal/config"
	"ollama_agent_go/internal/indexer"
	"ollama_agent_go/internal/mcp"
	"ollama_agent_go/internal/memory"
	"ollama_agent_go/internal/memory/longterm"
	"ollama_agent_go/internal/observability"
	"ollama_agent_go/internal/policy"
	"ollama_agent_go/internal/providers/ollama"
	"ollama_agent_go/internal/providers/router"
	"ollama_agent_go/internal/retriever"
	"ollama_agent_go/internal/runtime"
	"ollama_agent_go/internal/skills"
	sqstore "ollama_agent_go/internal/storage/sqlite"
	"ollama_agent_go/internal/tools"
)

// App holds the fully wired application components.
type App struct {
	Engine  *runtime.Engine
	Config  *config.Config
	Indexer *indexer.Indexer // nil when RAG is disabled
}

// New wires all internal packages and returns a ready-to-run App.
// logWriter may be nil (output is discarded). It is the caller's responsibility
// to close logWriter after the app shuts down.
func New(cfg *config.Config, logWriter io.Writer) (*App, error) {
	return NewWithContext(context.Background(), cfg, logWriter)
}

// NewWithContext is like New but accepts a context for MCP server connections.
func NewWithContext(ctx context.Context, cfg *config.Config, logWriter io.Writer) (*App, error) {
	// Storage (opened before observability so we can wire tracer + audit)
	db, err := sqstore.Open(cfg.DBPath)
	if err != nil {
		return nil, fmt.Errorf("open storage: %w", err)
	}
	store := sqstore.NewStore(db)

	// Observability — FileLogger wired with SQLite tracer + audit + in-memory metrics
	var obs observability.Logger
	if logWriter != nil {
		fl := observability.NewFileLogger(logWriter)
		fl.WithTracer(sqstore.NewTracer(db.Underlying()))
		fl.WithAudit(sqstore.NewAuditLogger(db.Underlying()))
		fl.WithMetrics(observability.NewInMemoryMetrics())
		obs = fl
	} else {
		obs = observability.Discard
	}

	// Policy engine
	noApproval := os.Getenv("OLLAMA_AGENT_NO_APPROVAL") == "1"
	gatedTools := []string{"write_file", "run_shell", "edit_file"}
	pol := policy.NewDefaultEngine(cfg.Root, gatedTools, noApproval)

	// Tool registry → Host (with saga store wired for WAL rollback)
	reg := tools.Default(cfg.Root)
	host := tools.NewHost(reg, pol, obs)
	host.Saga = store

	// Connect external MCP servers and register their tools.
	for _, srv := range cfg.MCPServers {
		mcpSrv := mcp.ServerConfig{
			Name:      srv.Name,
			Transport: srv.Transport,
			Command:   srv.Command,
			Args:      srv.Args,
			URL:       srv.URL,
		}
		client, err := mcp.ConnectServer(ctx, mcpSrv)
		if err != nil {
			obs.Error("mcp connect", err, "server", srv.Name)
			continue
		}
		for _, t := range mcp.AdaptAll(client) {
			reg.Register(t)
		}
		obs.Info("mcp connected", "server", srv.Name, "tools", len(client.Tools()))
	}

	// Provider router (Ollama only; cloud providers added via env vars later)
	ollamaClient := ollama.NewClient(cfg.BaseURL, cfg.Model)
	r := router.New()
	r.Add(ollamaClient)

	// Skills
	loadedSkills, _ := skills.Load(cfg.SkillsDir)

	// Agent
	ag := agent.New(ollamaClient, host, cfg.Model)
	ag.ContextBudget = cfg.ContextBudget
	ag.MaxIterations = cfg.MaxIterations
	ag.System = skills.Inject(agent.SystemPrompt(host), loadedSkills)

	// RAG pipeline (optional — only when OLLAMA_AGENT_RAG=1)
	var ltStore longterm.Store = longterm.NoopStore{}
	var ret *retriever.Retriever
	var idxr *indexer.Indexer
	if cfg.RAG.Enabled {
		embedder := indexer.NewOllamaEmbedder(cfg.BaseURL, cfg.RAG.EmbedModel)
		chromemEmbedFn := func(ctx context.Context, text string) ([]float32, error) {
			vecs, err := embedder.Embed(ctx, []string{text})
			if err != nil || len(vecs) == 0 {
				return nil, err
			}
			return vecs[0], nil
		}
		cs, err := longterm.NewChromemStore(cfg.RAG.StorePath, chromemEmbedFn)
		if err != nil {
			obs.Error("rag init", err)
		} else {
			ltStore = cs
			ret = retriever.New(cs, cfg.RAG.TopK)
			idxr = indexer.New(cs, embedder, cfg.Root)
			idxr.WithChunkConfig(indexer.ChunkConfig{
				Size: cfg.RAG.ChunkSize, Overlap: cfg.RAG.ChunkOverlap,
			})
			tools.RegisterKnowledgeTool(reg, cs)
			obs.Info("rag enabled", "store", cfg.RAG.StorePath)
		}
	}
	_ = idxr // exposed via App for /index command

	// Memory manager (short-term rolling window + SQLite episodic/profile)
	mem := memory.New(
		cfg.ContextBudget,
		ltStore,
		sqstore.NewEpisodicStore(db),
		sqstore.NewProfileStore(db),
	)

	// Runtime engine
	bus := runtime.NewInProcBus()
	engine := runtime.NewEngine(ag, host, r, store, bus, obs, loadedSkills, mem)
	engine.Retriever = ret
	if idxr != nil {
		engine.Indexer = idxr
	}

	return &App{Engine: engine, Config: cfg, Indexer: idxr}, nil
}
