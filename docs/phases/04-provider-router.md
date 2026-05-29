# Phase 04 — Provider Router Enhancements

Tags: `#providers` `#router` `#capabilities` `#routing-rules` `#fallback` `#streaming` `#p1` `#status/planned`

Prerequisites: Phase 01 (providers), Phase 03 (observability metrics to measure latency).

---

## Goal

The current router does round-robin / explicit switch. Enhance it with:
1. Capability flags per provider (`SupportsTools`, `SupportsStreaming`, `SupportsThinking`)
2. Task-type routing rules (coding → local coder, reasoning → thinking model, etc.)
3. Cost-aware selection (cheapest provider that meets capability requirements)
4. Automatic fallback when primary provider is unavailable

---

## Files to create/update (in order)

### Step 1 — Extend Provider interface
**`internal/providers/provider.go`** — add capability methods:

```go
type Provider interface {
    Name() string
    Chat(ctx context.Context, req types.ChatRequest) (types.ChatResponse, error)

    // Capability flags
    SupportsTools() bool
    SupportsStreaming() bool
    SupportsThinking() bool   // extended reasoning / chain-of-thought

    // Cost metadata
    Pricing() Pricing
}

type Pricing struct {
    InputPer1KTokens  float64
    OutputPer1KTokens float64
    Currency          string
}
```

Update existing adapters:
- `internal/providers/ollama/client.go`: `SupportsTools()=true`, `SupportsStreaming()=true`, `SupportsThinking()=false`
- `internal/providers/anthropic/client.go`: all true (claude-3-5-sonnet supports thinking)
- `internal/providers/openai/client.go`: tools=true, streaming=true, thinking=false

Tags: `#providers/interface` `#providers/capabilities`

---

### Step 2 — Task type enum
**`internal/providers/router/task.go`**

```go
package router

type TaskType int

const (
    TaskGeneral   TaskType = iota
    TaskCoding             // route to local coder model
    TaskReasoning          // route to thinking-capable model
    TaskFast               // smallest/cheapest model
    TaskLongContext        // model with large context window
)

// Detect infers task type from message content heuristics.
func Detect(msgs []types.Message) TaskType
```

Heuristics for `Detect`:
- Contains "```" or file extension keywords → `TaskCoding`
- Contains "reason", "analyze", "explain why" → `TaskReasoning`
- Single short message → `TaskFast`

Tags: `#router/task-detection`

---

### Step 3 — Routing rules
**`internal/providers/router/rules.go`**

```go
type Rule struct {
    TaskType    TaskType
    RequiresCap []string    // "tools", "streaming", "thinking"
    PreferModel string      // exact model name hint (optional)
    MaxCostPer1K float64    // 0 = no cost constraint
}

var DefaultRules = []Rule{
    {TaskType: TaskCoding,    RequiresCap: []string{"tools"},   PreferModel: "qwen2.5-coder:7b"},
    {TaskType: TaskReasoning, RequiresCap: []string{"thinking"}},
    {TaskType: TaskFast,      MaxCostPer1K: 0.001},
    {TaskType: TaskGeneral,   RequiresCap: []string{"tools"}},
}
```

Tags: `#router/rules`

---

### Step 4 — Enhanced Router
**`internal/providers/router/router.go`** — update `Chat()`:

```go
type Router struct {
    providers []providers.Provider
    active    string
    rules     []Rule
    mu        sync.RWMutex
}

func (r *Router) Chat(ctx context.Context, req types.ChatRequest) (types.ChatResponse, error) {
    taskType := Detect(req.Messages)
    p := r.selectProvider(taskType, req)
    resp, err := p.Chat(ctx, req)
    if err != nil {
        // fallback: try next qualifying provider
        return r.fallback(ctx, req, p.Name())
    }
    return resp, nil
}

func (r *Router) selectProvider(t TaskType, req types.ChatRequest) providers.Provider
func (r *Router) fallback(ctx context.Context, req types.ChatRequest, excludeName string) (types.ChatResponse, error)
func (r *Router) Active() string
func (r *Router) Switch(name string) error
func (r *Router) List() []providers.Provider
```

Tags: `#router/selection` `#router/fallback`

---

### Step 5 — Model override per-request
**`internal/types/messages.go`** — add optional field to `ChatRequest`:

```go
type ChatRequest struct {
    Model     string
    Messages  []Message
    Tools     []ToolDef
    ModelHint string    // NEW: if set, router tries this model first
}
```

Tags: `#types/chat-request`

---

### Step 6 — Wire into Engine
**`internal/runtime/engine.go`** — pass task type hint to agent:

- `SetModel(name string)` stores model preference in `Engine.modelHint`
- `Submit()` passes `modelHint` as `req.ModelHint`

**`internal/app/bootstrap.go`** — pass `rules` to `NewRouter(providers, DefaultRules)`.

Tags: `#runtime/engine` `#router/wire`

---

## Tests

**`internal/providers/router/router_test.go`**
- `TestDetectCodingTask` — message with code block → TaskCoding
- `TestSelectProviderCapability` — TaskReasoning selects only thinking-capable provider
- `TestFallbackOnError` — primary errors → secondary provider used
- `TestCostFilter` — TaskFast picks cheapest qualifying provider

Tags: `#tests`

---

## Verification

```
go test ./internal/providers/...
```

- `/model qwen2.5-coder:7b` in TUI → router respects explicit switch
- Coding-heavy prompt → router logs selected provider as coder model (check log)
- Kill primary provider → fallback provider responds without user-visible error
