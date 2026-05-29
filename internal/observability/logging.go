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
}

// FileLogger wraps charmbracelet/log to satisfy the Logger interface.
type FileLogger struct {
	inner *log.Logger
}

// NewFileLogger creates a Logger that writes to w. If w is nil, output is discarded.
func NewFileLogger(w io.Writer) *FileLogger {
	logger := log.New(w)
	logger.SetLevel(log.DebugLevel)
	return &FileLogger{inner: logger}
}

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

// Discard is a Logger that silently drops all output.
var Discard Logger = &discardLogger{}

type discardLogger struct{}

func (d *discardLogger) Info(string, ...any)                                       {}
func (d *discardLogger) Debug(string, ...any)                                      {}
func (d *discardLogger) Error(string, error, ...any)                               {}
func (d *discardLogger) RecordToolCall(string, string, time.Duration, error)       {}
func (d *discardLogger) RecordTokens(string, string, int, int)                     {}
