package observability

import (
	"sync"
	"time"
)

// ToolCallStats tracks per-tool call counts and error rates.
type ToolCallStats struct {
	Total  int64
	Errors int64
}

// LLMCallStats tracks per-model call counts, total tokens, and error rates.
type LLMCallStats struct {
	Total       int64
	TotalTokens int64
	Errors      int64
}

// MetricsSnapshot is a point-in-time view of all counters.
type MetricsSnapshot struct {
	ToolCalls   map[string]ToolCallStats
	LLMCalls    map[string]LLMCallStats
	TotalTokens int64
	Errors      int64
	UpdatedAt   time.Time
}

// Metrics is the interface used by runtime components to record counters.
type Metrics interface {
	IncrToolCall(tool string, success bool)
	IncrLLMCall(model string, tokens int, success bool)
	IncrApproval(tool string, approved bool)
	Snapshot() MetricsSnapshot
}

// InMemoryMetrics is a thread-safe, in-process counter implementation.
type InMemoryMetrics struct {
	mu        sync.RWMutex
	tools     map[string]*ToolCallStats
	llm       map[string]*LLMCallStats
	approvals map[string][2]int64 // [approved, denied]
	tokens    int64
	errors    int64
	updatedAt time.Time
}

// NewInMemoryMetrics returns a ready-to-use InMemoryMetrics.
func NewInMemoryMetrics() *InMemoryMetrics {
	return &InMemoryMetrics{
		tools:     make(map[string]*ToolCallStats),
		llm:       make(map[string]*LLMCallStats),
		approvals: make(map[string][2]int64),
		updatedAt: time.Now(),
	}
}

func (m *InMemoryMetrics) IncrToolCall(tool string, success bool) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if _, ok := m.tools[tool]; !ok {
		m.tools[tool] = &ToolCallStats{}
	}
	m.tools[tool].Total++
	if !success {
		m.tools[tool].Errors++
		m.errors++
	}
	m.updatedAt = time.Now()
}

func (m *InMemoryMetrics) IncrLLMCall(model string, tokens int, success bool) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if _, ok := m.llm[model]; !ok {
		m.llm[model] = &LLMCallStats{}
	}
	m.llm[model].Total++
	m.llm[model].TotalTokens += int64(tokens)
	m.tokens += int64(tokens)
	if !success {
		m.llm[model].Errors++
		m.errors++
	}
	m.updatedAt = time.Now()
}

func (m *InMemoryMetrics) IncrApproval(tool string, approved bool) {
	m.mu.Lock()
	defer m.mu.Unlock()
	v := m.approvals[tool]
	if approved {
		v[0]++
	} else {
		v[1]++
	}
	m.approvals[tool] = v
	m.updatedAt = time.Now()
}

func (m *InMemoryMetrics) Snapshot() MetricsSnapshot {
	m.mu.RLock()
	defer m.mu.RUnlock()
	snap := MetricsSnapshot{
		ToolCalls:   make(map[string]ToolCallStats, len(m.tools)),
		LLMCalls:    make(map[string]LLMCallStats, len(m.llm)),
		TotalTokens: m.tokens,
		Errors:      m.errors,
		UpdatedAt:   m.updatedAt,
	}
	for k, v := range m.tools {
		snap.ToolCalls[k] = *v
	}
	for k, v := range m.llm {
		snap.LLMCalls[k] = *v
	}
	return snap
}

// noopMetrics silently drops all increments.
type noopMetrics struct{}

func (noopMetrics) IncrToolCall(string, bool)      {}
func (noopMetrics) IncrLLMCall(string, int, bool)  {}
func (noopMetrics) IncrApproval(string, bool)      {}
func (noopMetrics) Snapshot() MetricsSnapshot      { return MetricsSnapshot{} }

// NoopMetrics is a Metrics that records nothing.
var NoopMetrics Metrics = noopMetrics{}
