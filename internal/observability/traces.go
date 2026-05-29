package observability

import (
	"context"
	"time"
)

// SpanKind classifies the kind of operation a span records.
type SpanKind string

const (
	SpanLLMCall   SpanKind = "llm_call"
	SpanToolCall  SpanKind = "tool_call"
	SpanApproval  SpanKind = "approval"
	SpanAgentTurn SpanKind = "agent_turn"
)

// Span is a single timed operation within an agent run.
type Span struct {
	ID         string
	ParentID   string // empty for root span
	SessionID  string
	Kind       SpanKind
	Model      string
	Tool       string
	Step       int
	DurationMs int64
	Tokens     int
	Error      string
	StartedAt  time.Time
	EndedAt    time.Time
}

// Tracer records spans.
type Tracer interface {
	// Start creates a new span and attaches it to the context.
	Start(ctx context.Context, kind SpanKind, sessionID string) (context.Context, *Span)
	// Finish closes the span, records duration, and persists it.
	Finish(ctx context.Context, span *Span, err error)
	// Export returns all spans recorded for the given session.
	Export(ctx context.Context, sessionID string) ([]Span, error)
}

type spanKey struct{}

// SpanFromContext retrieves the current span from a context (may be nil).
func SpanFromContext(ctx context.Context) *Span {
	v, _ := ctx.Value(spanKey{}).(*Span)
	return v
}

// ContextWithSpan returns a copy of ctx carrying s as the active span.
func ContextWithSpan(ctx context.Context, s *Span) context.Context {
	return context.WithValue(ctx, spanKey{}, s)
}

// noopTracer silently discards all spans.
type noopTracer struct{}

func (noopTracer) Start(ctx context.Context, kind SpanKind, sessionID string) (context.Context, *Span) {
	s := &Span{Kind: kind, SessionID: sessionID, StartedAt: time.Now()}
	return ContextWithSpan(ctx, s), s
}
func (noopTracer) Finish(_ context.Context, span *Span, err error) {
	if span == nil {
		return
	}
	span.EndedAt = time.Now()
	span.DurationMs = span.EndedAt.Sub(span.StartedAt).Milliseconds()
	if err != nil {
		span.Error = err.Error()
	}
}
func (noopTracer) Export(_ context.Context, _ string) ([]Span, error) { return nil, nil }

// NoopTracer is a Tracer that records nothing.
var NoopTracer Tracer = noopTracer{}
