# Phase 09 — Specialized Agent Roles

Tags: `#agents` `#roles` `#research` `#reasoning` `#action` `#data` `#communication` `#selector` `#p2` `#status/planned`

Prerequisites: Phase 02 (memory), Phase 04 (provider router), Phase 08 (RAG for research agent).

---

## Goal

Move from one general-purpose agent to a set of specialized agents, each with a focused
system prompt, tool subset, and memory access pattern. The `Engine` selects the right agent
per request (or routes to a coordinator that delegates).

---

## Agent role catalogue

| Role | Purpose | Key tools | Memory access |
|---|---|---|---|
| Research | Search, analyze, summarize information | search_knowledge, web_search (MCP), read_file | long-term read |
| Reasoning | Reason through problems, make plans | none (pure LLM) | short-term only |
| Action | Execute tasks, call tools | all tools | short + episodic write |
| Data | Query data, process structured info | run_shell (SQL), read_file | short-term |
| Communication | Summarize, draft messages | none (pure LLM) | short-term |

---

## Files to create (in order)

### Step 1 — Role enum and config
**`internal/agent/roles/role.go`**

```go
package roles

type Role int

const (
    RoleGeneral       Role = iota
    RoleResearch
    RoleReasoning
    RoleAction
    RoleData
    RoleCommunication
)

type RoleConfig struct {
    Role            Role
    SystemPrompt    string
    AllowedTools    []string    // nil = all tools allowed
    MaxIterations   int
    PreferredModel  string
    MemoryTiers     []string    // "short", "long", "episodic", "profile"
}

var DefaultConfigs = map[Role]RoleConfig{
    RoleResearch:      {SystemPrompt: researchPrompt, AllowedTools: []string{"search_knowledge", "read_file", "web_search"}, MaxIterations: 10},
    RoleReasoning:     {SystemPrompt: reasoningPrompt, AllowedTools: []string{}, MaxIterations: 5},
    RoleAction:        {SystemPrompt: actionPrompt, MaxIterations: 20},
    RoleData:          {SystemPrompt: dataPrompt, AllowedTools: []string{"run_shell", "read_file"}, MaxIterations: 10},
    RoleCommunication: {SystemPrompt: commPrompt, AllowedTools: []string{}, MaxIterations: 3},
}
```

Tags: `#agents/roles`

---

### Step 2 — Per-role system prompts
**`internal/agent/roles/prompts.go`**

One constant per role. Keep concise — under 200 tokens each.

```go
const researchPrompt = `You are a research agent. Your job is to find, analyze, and
summarize information using available search tools. Always cite sources.
Return a structured summary with key findings and confidence level.`

const reasoningPrompt = `You are a reasoning agent. Think step by step.
Show your chain of thought explicitly before giving a conclusion.
Do not call tools; reason from information already in context.`

const actionPrompt = `You are an action agent. Execute tasks precisely.
Use tools to accomplish goals. Report results concisely.
Always verify file operations succeeded.`

const dataPrompt = `You are a data agent. Query and process structured data.
Use shell commands for data extraction. Return structured results.`

const commPrompt = `You are a communication agent. Summarize clearly and concisely.
Draft responses appropriate for the intended audience.
Be direct; omit unnecessary preamble.`
```

Tags: `#agents/prompts`

---

### Step 3 — Role-scoped ToolHost
**`internal/tools/scoped_host.go`**

```go
// ScopedHost wraps Host and restricts Execute to allowed tool names.
type ScopedHost struct {
    *Host
    allowed map[string]bool
}

func NewScopedHost(h *Host, allowedTools []string) *ScopedHost

func (s *ScopedHost) Execute(ctx context.Context, name string, args map[string]any) (string, error) {
    if len(s.allowed) > 0 && !s.allowed[name] {
        return "", fmt.Errorf("tool %q not available for this agent role", name)
    }
    return s.Host.Execute(ctx, name, args)
}
```

Tags: `#tools/scoped-host` `#agents/tool-scope`

---

### Step 4 — Role-aware Agent constructor
**`internal/agent/agent.go`** — add `WithRole()` option:

```go
func WithRole(cfg roles.RoleConfig) func(*Agent) {
    return func(a *Agent) {
        a.systemPrompt = cfg.SystemPrompt
        a.MaxIterations = cfg.MaxIterations
        a.ToolHost = tools.NewScopedHost(a.ToolHost.(*tools.Host), cfg.AllowedTools)
    }
}
```

Tags: `#agents/constructor`

---

### Step 5 — Agent selector
**`internal/agent/selector.go`**

```go
package agent

type Selector struct {
    agents map[roles.Role]*Agent
}

func NewSelector(base *Agent, host *tools.Host, router providers.Router) *Selector

// Select returns the appropriate agent for the given input.
func (s *Selector) Select(input string) *Agent

// Heuristics:
//   - starts with "find", "search", "what is", "who is" → Research
//   - starts with "analyze", "explain why", "compare", "reason" → Reasoning
//   - starts with "write", "create", "run", "execute", "fix" → Action
//   - contains "data", "query", "sql", "table", "csv" → Data
//   - starts with "summarize", "draft", "write email" → Communication
//   - otherwise → General (same as current)
func detect(input string) roles.Role
```

Tags: `#agents/selector`

---

### Step 6 — Wire Selector into Engine
**`internal/runtime/engine.go`** — replace direct `e.Agent.Run()` call:

```go
// Submit selects the right agent for the input, then runs it.
func (e *Engine) Submit(ctx context.Context, input string, emit agent.Emit) error {
    ag := e.Selector.Select(input)
    // ... rest of submit logic unchanged ...
    final, err := ag.Run(ctx, history, emit)
    // ...
}
```

Add `Selector *agent.Selector` field to `Engine`.

Tags: `#runtime/engine` `#agents/wire`

---

### Step 7 — `/agent` slash command
**`internal/ui/tui/model.go`** — add `/agent <role>` to force role selection:

```
/agent research  → pin to research agent for this session
/agent reset     → back to auto-select
```

Tags: `#ui/tui` `#agents/cli`

---

## Tests

**`internal/agent/selector_test.go`**
- `TestDetectResearch` — "find the docs for X" → RoleResearch
- `TestDetectAction` — "write a file called foo.txt" → RoleAction
- `TestDetectFallbackGeneral` — ambiguous input → RoleGeneral

**`internal/tools/scoped_host_test.go`**
- `TestScopedHostBlocksDisallowedTool`
- `TestScopedHostAllowsAllIfNilList`

Tags: `#tests`

---

## Verification

```
go test ./internal/agent/... ./internal/tools/...
```

- "Search for information about Go generics" → Research agent selected (visible in log)
- Research agent cannot call `write_file` (blocked by ScopedHost)
- "Write a test for this function" → Action agent selected
- `/agent reasoning` forces Reasoning agent regardless of input
