package runtime

import (
	"context"
	"fmt"
	"sync/atomic"
	"time"

	"github.com/google/uuid"
	"ollama_agent_go/internal/agent"
	"ollama_agent_go/internal/agent/roles"
	"ollama_agent_go/internal/memory"
	"ollama_agent_go/internal/observability"
	"ollama_agent_go/internal/providers/router"
	"ollama_agent_go/internal/reliability"
	"ollama_agent_go/internal/retriever"
	"ollama_agent_go/internal/skills"
	"ollama_agent_go/internal/storage"
	"ollama_agent_go/internal/tools"
	"ollama_agent_go/internal/types"
)

// indexerIface is the subset of indexer.Indexer needed by the engine.
// Defined here to avoid importing the indexer package from runtime.
type indexerIface interface {
	IndexDir(ctx context.Context, dir string, exts []string) error
}

// SetRole parses roleName and pins the selector, returning an error for unknown names.
func (e *Engine) SetRole(roleName string) error {
	role, ok := roles.ParseRole(roleName)
	if !ok {
		return fmt.Errorf("unknown role %q; valid: general, research, reasoning, action, data, communication", roleName)
	}
	e.PinRole(role)
	return nil
}

// PinRole locks the selector to role for all subsequent requests.
// Pass roles.RoleGeneral (0) to reset to auto-select.
func (e *Engine) PinRole(role roles.Role) {
	if e.Selector != nil {
		e.Selector.Pin(role)
	}
}

// PinnedRole returns the currently pinned role name, or "auto" if auto-selecting.
func (e *Engine) PinnedRole() string {
	if e.Selector == nil {
		return "auto"
	}
	r := e.Selector.PinnedRole()
	if r == roles.RoleGeneral {
		return "auto"
	}
	return r.String()
}

// IndexDir proxies to the wired Indexer, or returns an error if RAG is disabled.
func (e *Engine) IndexDir(ctx context.Context, dir string) error {
	if e.Indexer == nil {
		return fmt.Errorf("RAG not enabled (set OLLAMA_AGENT_RAG=1)")
	}
	return e.Indexer.IndexDir(ctx, dir, nil)
}

// HITLCheckpoint is delivered to the registered HITLHandler when engine.Interrupt() fires.
type HITLCheckpoint struct {
	SessionID string
	Reason    string
	Pending   []types.Message
	At        time.Time
}

// HITLHandler decides whether to continue (true) or abort (false) after a checkpoint.
type HITLHandler func(HITLCheckpoint) bool

// FallbackConfig controls agent-level fallback when the primary model fails repeatedly.
type FallbackConfig struct {
	MaxAttempts   int
	FallbackModel string
}

// Engine is the application brain. The UI calls Submit() and receives events;
// everything else — policy, storage, observability — is handled here.
type Engine struct {
	Agent    *agent.Agent
	ToolHost *tools.Host
	Router   *router.Router
	Sessions storage.SessionStore
	Bus      EventBus
	Obs      observability.Logger
	Skills   []skills.Skill
	Memory   memory.Manager

	// Reliability
	Fallback reliability.RetryConfig // retry config for agent-level fallback
	hitl     HITLHandler             // nil = no HITL
	hitlFlag atomic.Bool             // set by Interrupt(); cleared after checkpoint

	// RAG — optional; nil disables automatic context injection and /index
	Retriever *retriever.Retriever
	Indexer   indexerIface // set by bootstrap when RAG is enabled

	// Agent role selector — nil means always use e.Agent directly
	Selector *agent.Selector

	session *Session // current in-memory session
}

// NewEngine creates a fully wired Engine. Call Init() before first Submit().
// If mem is nil a default in-memory manager (no token budget) is used.
func NewEngine(
	ag *agent.Agent,
	host *tools.Host,
	r *router.Router,
	sessions storage.SessionStore,
	bus EventBus,
	obs observability.Logger,
	loadedSkills []skills.Skill,
	mem ...memory.Manager,
) *Engine {
	var mgr memory.Manager
	if len(mem) > 0 && mem[0] != nil {
		mgr = mem[0]
	} else {
		mgr = memory.NewDefault(0)
	}
	return &Engine{
		Agent:    ag,
		ToolHost: host,
		Router:   r,
		Sessions: sessions,
		Bus:      bus,
		Obs:      obs,
		Skills:   loadedSkills,
		Memory:   mgr,
	}
}

// Init starts a new session and persists it to storage.
func (e *Engine) Init(ctx context.Context) error {
	id := uuid.New().String()
	_, err := e.Sessions.Create(ctx, id, e.Agent.Model)
	if err != nil {
		return fmt.Errorf("init session: %w", err)
	}
	e.session = &Session{
		ID:        id,
		State:     StateIdle,
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}
	e.Memory.Short().Reset()
	e.ToolHost.SessionID = id
	e.Obs.Info("session started", "id", id, "model", e.Agent.Model)
	_ = e.Obs.Audit().Log(ctx, observability.AuditEvent{
		SessionID: id,
		Actor:     "system",
		Action:    "session_start",
		Resource:  e.Agent.Model,
		Result:    "ok",
	})
	return nil
}

// SessionID returns the current session's ID.
func (e *Engine) SessionID() string {
	if e.session != nil {
		return e.session.ID
	}
	return ""
}

// Conversation returns the current short-term conversation window.
func (e *Engine) Conversation() []types.Message {
	return e.Memory.Short().Messages()
}

// Submit accepts user input, runs the agent, and persists results.
// It blocks until the run completes. Events stream to the caller via emit.
func (e *Engine) Submit(ctx context.Context, input string, emit agent.Emit) (retErr error) {
	if e.session == nil {
		if err := e.Init(ctx); err != nil {
			return err
		}
	}

	// Span wraps the entire agent turn so child spans (tool calls) nest correctly.
	ctx, span := e.Obs.Tracer().Start(ctx, observability.SpanAgentTurn, e.session.ID)
	defer func() { e.Obs.Tracer().Finish(ctx, span, retErr) }()

	// Audit: session turn started
	_ = e.Obs.Audit().Log(ctx, observability.AuditEvent{
		SessionID: e.session.ID,
		Actor:     "user",
		Action:    "submit",
		Resource:  e.Agent.Model,
		Result:    "ok",
	})

	userMsg := types.Message{Role: "user", Content: input}
	e.Memory.Short().Append(userMsg)
	_ = e.Sessions.AppendMessage(ctx, e.session.ID, userMsg)
	_ = e.Sessions.SetState(ctx, e.session.ID, storage.StateRunning)
	e.session.State = StateRunning

	// On context cancellation, compensate any in-flight filesystem mutations.
	sessionID := e.session.ID
	defer func() {
		if ctx.Err() != nil {
			e.ToolHost.CompensatePending(sessionID)
		}
	}()

	// RAG: inject retrieved context as a system message before the run.
	if e.Retriever != nil {
		if chunks, err := e.Retriever.Retrieve(ctx, input); err == nil && len(chunks) > 0 {
			e.Memory.Short().Append(types.Message{
				Role:    "system",
				Content: e.Retriever.InjectContext(chunks),
			})
		}
	}

	// Select the right agent for this input.
	ag := e.Agent
	if e.Selector != nil {
		ag = e.Selector.Select(input)
	}

	// Wire HITL interrupt check into the selected agent loop for this run.
	ag.InterruptCheck = func() bool { return e.checkHITL(ctx) }
	defer func() { ag.InterruptCheck = nil }()

	// Wrap emit to publish events to the bus as well.
	wrapped := func(ev agent.Event) {
		if emit != nil {
			emit(ev)
		}
		e.Bus.Publish(BusEvent{Name: "agent.event", Payload: ev})
	}

	result, err := ag.Run(ctx, e.Memory.Short().Messages(), wrapped)

	if err != nil {
		_ = e.Sessions.SetState(ctx, e.session.ID, storage.StateFailed)
		e.session.State = StateFailed
		e.Obs.Error("run failed", err, "session", e.session.ID)
		return err
	}

	// Persist the assistant response so future turns have full context.
	if result != "" {
		assistantMsg := types.Message{Role: "assistant", Content: result}
		e.Memory.Short().Append(assistantMsg)
		_ = e.Sessions.AppendMessage(ctx, e.session.ID, assistantMsg)
	}

	_ = e.Sessions.SetState(ctx, e.session.ID, storage.StateIdle)
	e.session.State = StateIdle
	e.session.UpdatedAt = time.Now()

	e.Obs.Info("run completed",
		"session", e.session.ID,
		"turns", len(e.Memory.Short().Messages()),
	)
	return nil
}

// ClearHistory creates a new session, discarding the current conversation.
func (e *Engine) ClearHistory(ctx context.Context) error {
	id := uuid.New().String()
	_, err := e.Sessions.Create(ctx, id, e.Agent.Model)
	if err != nil {
		return err
	}
	e.session = &Session{
		ID:        id,
		State:     StateIdle,
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}
	e.Memory.Short().Reset()
	e.ToolHost.SessionID = id
	e.Obs.Info("session cleared", "new_id", id)
	_ = e.Obs.Audit().Log(ctx, observability.AuditEvent{
		SessionID: id,
		Actor:     "user",
		Action:    "session_clear",
		Result:    "ok",
	})
	return nil
}

// SetModel switches the active model name on the agent and rebuilds its system prompt.
// It also stores the name as a ModelHint so the router can pick the right provider.
func (e *Engine) SetModel(name string) {
	e.Agent.Model = name
	e.Agent.ModelHint = name
	base := agent.SystemPrompt(e.ToolHost)
	e.Agent.System = skills.Inject(base, e.Skills)
}

// Model returns the currently active model name.
func (e *Engine) Model() string { return e.Agent.Model }

// ToolNames returns the names of all registered tools.
func (e *Engine) ToolNames() []string { return e.ToolHost.Names() }

// GetTool returns a tool by name.
func (e *Engine) GetTool(name string) (tools.Tool, bool) { return e.ToolHost.Get(name) }

// LoadedSkills returns the skills loaded into the agent.
func (e *Engine) LoadedSkills() []skills.Skill { return e.Skills }

// OnHITL registers a handler that is called when Interrupt() fires during Submit().
// Pass nil to remove the handler.
func (e *Engine) OnHITL(h HITLHandler) { e.hitl = h }

// Interrupt signals the currently running Submit() to pause at the next iteration
// boundary and call the registered HITLHandler. If no handler is registered the
// signal is silently dropped.
func (e *Engine) Interrupt(reason string) {
	if e.hitl == nil {
		return
	}
	e.hitlFlag.Store(true)
}

// checkHITL is called at the top of each agent iteration. It blocks until the
// HITL handler returns. Returns false if the run should abort.
func (e *Engine) checkHITL(ctx context.Context) bool {
	if !e.hitlFlag.Swap(false) {
		return true // no interrupt pending
	}
	if e.hitl == nil {
		return true
	}
	cp := HITLCheckpoint{
		SessionID: e.SessionID(),
		Reason:    "operator interrupt",
		Pending:   e.Memory.Short().Messages(),
		At:        time.Now(),
	}
	return e.hitl(cp)
}

// ExportTrace returns all spans recorded for the current session.
func (e *Engine) ExportTrace(ctx context.Context) ([]observability.Span, error) {
	if e.session == nil {
		return nil, nil
	}
	return e.Obs.Tracer().Export(ctx, e.session.ID)
}

// ExportAudit returns all audit events for the current session.
func (e *Engine) ExportAudit(ctx context.Context) ([]observability.AuditEvent, error) {
	if e.session == nil {
		return nil, nil
	}
	return e.Obs.Audit().Query(ctx, e.session.ID)
}

// MetricsSnapshot returns a point-in-time snapshot of all counters.
func (e *Engine) MetricsSnapshot() observability.MetricsSnapshot {
	return e.Obs.Metrics().Snapshot()
}
