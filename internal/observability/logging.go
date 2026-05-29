// Package observability provides structured logging and metrics for the agent runtime.
package observability

import (
	"io"
	"time"

	"github.com/charmbracelet/log"
)

// Logger is the observability interface used by runtime components.
type Logger interface {
	Info(msg string, fields ...any)
	Debug(msg string, fields ...any)
	Error(msg string, err error, fields ...any)
	RecordToolCall(session, tool string, duration time.Duration, err error)
	RecordTokens(session, model string, prompt, output int)

	// Phase 05 — structured observability
	Tracer() Tracer
	Audit() AuditLogger
	Metrics() Metrics
}

// FileLogger wraps charmbracelet/log to satisfy the Logger interface.
// Tracer, Audit, and Metrics default to no-op implementations; wire the real
// ones via NewFileLoggerFull after the storage layer is available.
type FileLogger struct {
	inner   *log.Logger
	tracer  Tracer
	audit   AuditLogger
	metrics Metrics
}

// NewFileLogger creates a Logger that writes to w. If w is nil, output is discarded.
// Tracer, AuditLogger, and Metrics are no-ops until replaced via WithTracer etc.
func NewFileLogger(w io.Writer) *FileLogger {
	logger := log.New(w)
	logger.SetLevel(log.DebugLevel)
	return &FileLogger{
		inner:   logger,
		tracer:  NoopTracer,
		audit:   NoopAuditLogger,
		metrics: NoopMetrics,
	}
}

// WithTracer replaces the tracer on fl and returns fl for chaining.
func (l *FileLogger) WithTracer(t Tracer) *FileLogger   { l.tracer = t; return l }
func (l *FileLogger) WithAudit(a AuditLogger) *FileLogger { l.audit = a; return l }
func (l *FileLogger) WithMetrics(m Metrics) *FileLogger { l.metrics = m; return l }

func (l *FileLogger) Info(msg string, fields ...any)  { l.inner.Info(msg, fields...) }
func (l *FileLogger) Debug(msg string, fields ...any) { l.inner.Debug(msg, fields...) }
func (l *FileLogger) Error(msg string, err error, fields ...any) {
	l.inner.Error(msg, append(fields, "error", err)...)
}

func (l *FileLogger) RecordToolCall(session, tool string, duration time.Duration, err error) {
	if err != nil {
		l.inner.Info("tool_call",
			"session", session,
			"tool", tool,
			"duration_ms", duration.Milliseconds(),
			"error", err.Error(),
		)
		return
	}
	l.inner.Info("tool_call",
		"session", session,
		"tool", tool,
		"duration_ms", duration.Milliseconds(),
		"ok", true,
	)
}

func (l *FileLogger) RecordTokens(session, model string, prompt, output int) {
	l.inner.Info("tokens",
		"session", session,
		"model", model,
		"prompt", prompt,
		"output", output,
	)
}

func (l *FileLogger) Tracer() Tracer       { return l.tracer }
func (l *FileLogger) Audit() AuditLogger   { return l.audit }
func (l *FileLogger) Metrics() Metrics     { return l.metrics }

// Discard is a Logger that silently drops all output.
var Discard Logger = &discardLogger{}

type discardLogger struct{}

func (d *discardLogger) Info(string, ...any)                                 {}
func (d *discardLogger) Debug(string, ...any)                                {}
func (d *discardLogger) Error(string, error, ...any)                         {}
func (d *discardLogger) RecordToolCall(string, string, time.Duration, error) {}
func (d *discardLogger) RecordTokens(string, string, int, int)               {}
func (d *discardLogger) Tracer() Tracer                                      { return NoopTracer }
func (d *discardLogger) Audit() AuditLogger                                  { return NoopAuditLogger }
func (d *discardLogger) Metrics() Metrics                                    { return NoopMetrics }
