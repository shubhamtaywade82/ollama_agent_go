package runtime

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
	"ollama_agent_go/internal/agent"
	"ollama_agent_go/internal/observability"
	"ollama_agent_go/internal/providers/router"
	"ollama_agent_go/internal/skills"
	"ollama_agent_go/internal/storage"
	"ollama_agent_go/internal/tools"
	"ollama_agent_go/internal/types"
)

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

	session *Session // current in-memory session
}

// NewEngine creates a fully wired Engine. Call Init() before first Submit().
func NewEngine(
	ag *agent.Agent,
	host *tools.Host,
	r *router.Router,
	sessions storage.SessionStore,
	bus EventBus,
	obs observability.Logger,
	loadedSkills []skills.Skill,
) *Engine {
	return &Engine{
		Agent:    ag,
		ToolHost: host,
		Router:   r,
		Sessions: sessions,
		Bus:      bus,
		Obs:      obs,
		Skills:   loadedSkills,
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
	e.ToolHost.SessionID = id
	e.Obs.Info("session started", "id", id, "model", e.Agent.Model)
	return nil
}

// SessionID returns the current session's ID.
func (e *Engine) SessionID() string {
	if e.session != nil {
		return e.session.ID
	}
	return ""
}

// Submit accepts user input, runs the agent, and persists results.
// It blocks until the run completes. Events stream to the caller via emit.
func (e *Engine) Submit(ctx context.Context, input string, emit agent.Emit) error {
	if e.session == nil {
		if err := e.Init(ctx); err != nil {
			return err
		}
	}

	userMsg := types.Message{Role: "user", Content: input}
	e.session.Conversation = append(e.session.Conversation, userMsg)
	_ = e.Sessions.AppendMessage(ctx, e.session.ID, userMsg)
	_ = e.Sessions.SetState(ctx, e.session.ID, storage.StateRunning)
	e.session.State = StateRunning

	// Wrap emit to publish events to the bus as well.
	wrapped := func(ev agent.Event) {
		emit(ev)
		e.Bus.Publish(BusEvent{Name: "agent.event", Payload: ev})
	}

	result, err := e.Agent.Run(ctx, e.session.Conversation, wrapped)

	if err != nil {
		_ = e.Sessions.SetState(ctx, e.session.ID, storage.StateFailed)
		e.session.State = StateFailed
		e.Obs.Error("run failed", err, "session", e.session.ID)
		return err
	}

	// Persist the assistant response so future turns have full context.
	if result != "" {
		assistantMsg := types.Message{Role: "assistant", Content: result}
		e.session.Conversation = append(e.session.Conversation, assistantMsg)
		_ = e.Sessions.AppendMessage(ctx, e.session.ID, assistantMsg)
	}

	_ = e.Sessions.SetState(ctx, e.session.ID, storage.StateIdle)
	e.session.State = StateIdle
	e.session.UpdatedAt = time.Now()

	e.Obs.Info("run completed",
		"session", e.session.ID,
		"turns", len(e.session.Conversation),
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
	e.ToolHost.SessionID = id
	e.Obs.Info("session cleared", "new_id", id)
	return nil
}

// SetModel switches the active model name on the agent and rebuilds its system prompt.
func (e *Engine) SetModel(name string) {
	e.Agent.Model = name
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

