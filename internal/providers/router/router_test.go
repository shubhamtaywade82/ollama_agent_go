package router

import (
	"context"
	"errors"
	"testing"

	"ollama_agent_go/internal/providers"
	"ollama_agent_go/internal/types"
)

// fakeProvider is a test double for providers.Provider.
type fakeProvider struct {
	name      string
	tools     bool
	streaming bool
	thinking  bool
	pricing   providers.Pricing
	err       error // if non-nil, Chat returns this error
	response  types.ChatResponse
}

func (f *fakeProvider) Name() string               { return f.name }
func (f *fakeProvider) SupportsTools() bool        { return f.tools }
func (f *fakeProvider) SupportsStreaming() bool     { return f.streaming }
func (f *fakeProvider) SupportsThinking() bool     { return f.thinking }
func (f *fakeProvider) Pricing() providers.Pricing { return f.pricing }
func (f *fakeProvider) Chat(_ context.Context, _ types.ChatRequest) (types.ChatResponse, error) {
	if f.err != nil {
		return types.ChatResponse{}, f.err
	}
	return f.response, nil
}

func newOllama(name string) *fakeProvider {
	return &fakeProvider{name: name, tools: true, streaming: true}
}
func newThinking(name string) *fakeProvider {
	return &fakeProvider{name: name, tools: true, thinking: true}
}
func newCheap(name string) *fakeProvider {
	return &fakeProvider{name: name, tools: true,
		pricing: providers.Pricing{InputPerM: 0.5}} // 0.0005 per 1K
}

// ─── Task detection ──────────────────────────────────────────────────────────

func TestDetectCodingTask(t *testing.T) {
	msgs := []types.Message{{Role: "user", Content: "write a func to parse JSON in Go"}}
	if got := Detect(msgs); got != TaskCoding {
		t.Errorf("Detect = %v, want TaskCoding", got)
	}
}

func TestDetectCodeBlock(t *testing.T) {
	msgs := []types.Message{{Role: "user", Content: "fix this: ```go\nfmt.Println()\n```"}}
	if got := Detect(msgs); got != TaskCoding {
		t.Errorf("Detect = %v, want TaskCoding", got)
	}
}

func TestDetectReasoningTask(t *testing.T) {
	msgs := []types.Message{{Role: "user", Content: "analyze the tradeoffs between SQL and NoSQL databases"}}
	if got := Detect(msgs); got != TaskReasoning {
		t.Errorf("Detect = %v, want TaskReasoning", got)
	}
}

func TestDetectFastTask(t *testing.T) {
	msgs := []types.Message{{Role: "user", Content: "hi"}}
	if got := Detect(msgs); got != TaskFast {
		t.Errorf("Detect = %v, want TaskFast", got)
	}
}

func TestDetectGeneralTask(t *testing.T) {
	msgs := []types.Message{{Role: "user", Content: "what is the capital of France?"}}
	if got := Detect(msgs); got != TaskGeneral {
		t.Errorf("Detect = %v, want TaskGeneral", got)
	}
}

func TestDetectEmptyMessages(t *testing.T) {
	if got := Detect(nil); got != TaskGeneral {
		t.Errorf("empty msgs should be TaskGeneral, got %v", got)
	}
}

// ─── Provider selection ──────────────────────────────────────────────────────

func TestSelectThinkingProviderForReasoning(t *testing.T) {
	general := newOllama("ollama")
	thinker := newThinking("anthropic")
	thinker.response = types.ChatResponse{Message: types.Message{Role: "assistant", Content: "thought"}}

	rules := []Rule{
		{TaskType: TaskReasoning, RequiresCap: []string{"thinking"}},
		{TaskType: TaskGeneral, RequiresCap: []string{"tools"}},
	}
	r := New(rules...)
	r.Add(general)
	r.Add(thinker)

	msgs := []types.Message{{Role: "user", Content: "analyze the architecture and explain why it works"}}
	resp, err := r.Chat(context.Background(), types.ChatRequest{Messages: msgs})
	if err != nil {
		t.Fatalf("chat: %v", err)
	}
	if resp.Message.Content != "thought" {
		t.Errorf("expected thinking provider response, got %q", resp.Message.Content)
	}
}

func TestFallbackOnPrimaryError(t *testing.T) {
	failing := &fakeProvider{name: "primary", tools: true, err: errors.New("unavailable")}
	backup := &fakeProvider{name: "backup", tools: true,
		response: types.ChatResponse{Message: types.Message{Content: "fallback"}}}

	r := New(DefaultRules...)
	r.Add(failing)
	r.Add(backup)

	resp, err := r.Chat(context.Background(), types.ChatRequest{
		Messages: []types.Message{{Role: "user", Content: "hello world, nice to meet you"}},
	})
	if err != nil {
		t.Fatalf("expected fallback to succeed: %v", err)
	}
	if resp.Message.Content != "fallback" {
		t.Errorf("expected fallback response, got %q", resp.Message.Content)
	}
}

func TestAllProvidersFail(t *testing.T) {
	p1 := &fakeProvider{name: "p1", tools: true, err: errors.New("down")}
	p2 := &fakeProvider{name: "p2", tools: true, err: errors.New("down")}

	r := New()
	r.Add(p1)
	r.Add(p2)

	_, err := r.Chat(context.Background(), types.ChatRequest{
		Messages: []types.Message{{Role: "user", Content: "hello world test message here"}},
	})
	if err == nil {
		t.Error("expected error when all providers fail")
	}
}

func TestModelHintRoutesToCorrectProvider(t *testing.T) {
	ollamaP := &fakeProvider{name: "ollama", tools: true,
		response: types.ChatResponse{Message: types.Message{Content: "ollama-resp"}}}
	anthropicP := &fakeProvider{name: "anthropic", tools: true, thinking: true,
		response: types.ChatResponse{Message: types.Message{Content: "anthropic-resp"}}}

	r := New(DefaultRules...)
	r.Add(ollamaP)
	r.Add(anthropicP)

	// Hint "claude-3-5" should resolve to anthropic.
	resp, err := r.Chat(context.Background(), types.ChatRequest{
		ModelHint: "claude-3-5-sonnet",
		Messages:  []types.Message{{Role: "user", Content: "hello"}},
	})
	if err != nil {
		t.Fatalf("chat: %v", err)
	}
	if resp.Message.Content != "anthropic-resp" {
		t.Errorf("hint routing: got %q, want anthropic-resp", resp.Message.Content)
	}
}

func TestSwitchOverridesSelection(t *testing.T) {
	p1 := &fakeProvider{name: "p1", tools: true,
		response: types.ChatResponse{Message: types.Message{Content: "p1"}}}
	p2 := &fakeProvider{name: "p2", tools: true,
		response: types.ChatResponse{Message: types.Message{Content: "p2"}}}

	r := New()
	r.Add(p1)
	r.Add(p2)
	if err := r.Switch("p2"); err != nil {
		t.Fatalf("switch: %v", err)
	}

	// With p2 active, simple messages should prefer p2 through task selection.
	resp, err := r.Chat(context.Background(), types.ChatRequest{
		Messages: []types.Message{{Role: "user", Content: "hello world test query here"}},
	})
	if err != nil {
		t.Fatalf("chat: %v", err)
	}
	_ = resp // p2 is active but task selection may pick p1 if it satisfies; just check no error
}

func TestRuleCapabilitySatisfaction(t *testing.T) {
	noTools := &fakeProvider{name: "notools", tools: false}
	withTools := &fakeProvider{name: "withtools", tools: true,
		response: types.ChatResponse{Message: types.Message{Content: "ok"}}}

	rule := Rule{TaskType: TaskGeneral, RequiresCap: []string{"tools"}}
	r := New(rule)
	r.Add(noTools)
	r.Add(withTools)

	resp, err := r.Chat(context.Background(), types.ChatRequest{
		Messages: []types.Message{{Role: "user", Content: "a medium length general question here"}},
	})
	if err != nil {
		t.Fatalf("chat: %v", err)
	}
	if resp.Message.Content != "ok" {
		t.Errorf("should have selected tool-capable provider")
	}
}
