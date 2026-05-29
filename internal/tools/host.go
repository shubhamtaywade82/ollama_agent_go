package tools

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
	"ollama_agent_go/internal/observability"
	"ollama_agent_go/internal/policy"
	"ollama_agent_go/internal/storage"
)

// mutatingTools is the set of tool names that modify the filesystem.
var mutatingTools = map[string]bool{
	"write_file": true,
	"edit_file":  true,
	"run_shell":  true,
}

// Host wraps a Registry with policy enforcement and observability. It is the
// single execution path for all tool calls; neither the agent nor the TUI
// should call Registry.Execute directly.
type Host struct {
	Registry *Registry
	Policy   *policy.DefaultEngine
	Obs      observability.Logger
	Saga     storage.SagaStore // optional; nil disables saga tracking
	// SessionID is set by the runtime engine before each agent run so that
	// observability records are associated with the correct session.
	SessionID string
}

// NewHost builds a Host with the given components.
func NewHost(reg *Registry, pol *policy.DefaultEngine, obs observability.Logger) *Host {
	return &Host{Registry: reg, Policy: pol, Obs: obs}
}

// Names delegates to the underlying registry.
func (h *Host) Names() []string { return h.Registry.Names() }

// Get delegates to the underlying registry.
func (h *Host) Get(name string) (Tool, bool) { return h.Registry.Get(name) }

// Specs delegates to the underlying registry.
func (h *Host) Specs() []Spec { return h.Registry.Specs() }

// Execute enforces policy, records a WAL entry for mutating tools, executes
// the tool, and records observability.
func (h *Host) Execute(ctx context.Context, name string, args map[string]any) (string, error) {
	isMutating := mutatingTools[name]

	// --- saga: reserve + lock before policy check so denied calls are also logged ---
	var mutID string
	if isMutating && h.Saga != nil {
		rel := pathFromArgs(args)
		before, _ := CaptureFileBefore(h.Registry.Root(), rel)
		mut := storage.Mutation{
			ID:          uuid.New().String(),
			SessionID:   h.SessionID,
			Tool:        name,
			Args:        args,
			BeforeState: before,
			Status:      storage.SagaReserved,
		}
		mutID = mut.ID
		_ = h.Saga.Reserve(ctx, mut)
		_ = h.Saga.Transition(ctx, mutID, storage.SagaLocked, nil)
	}

	if !h.Policy.Enforce(ctx, name, args) {
		if isMutating && h.Saga != nil {
			_ = h.Saga.Transition(ctx, mutID, storage.SagaRolledBack, nil)
		}
		return "", fmt.Errorf("tool %q was denied", name)
	}

	start := time.Now()
	out, err := h.Registry.Execute(ctx, name, args)
	dur := time.Since(start)
	h.Obs.RecordToolCall(h.SessionID, name, dur, err)

	if isMutating && h.Saga != nil {
		if err != nil {
			_ = h.Saga.Transition(ctx, mutID, storage.SagaRolledBack, nil)
		} else {
			rel := pathFromArgs(args)
			after, _ := CaptureFileBefore(h.Registry.Root(), rel)
			_ = h.Saga.Transition(ctx, mutID, storage.SagaApplied, after)
			_ = h.Saga.Transition(ctx, mutID, storage.SagaCommitted, nil)
		}
	}

	return out, err
}
