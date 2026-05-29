package tools

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	"ollama_agent_go/internal/storage"
)

// CompensationFunc restores the pre-mutation state of a file.
// root is the sandbox root; args are the tool arguments; before is the captured
// file content before execution (nil means the file was created by the tool).
type CompensationFunc func(ctx context.Context, root string, args map[string]any, before []byte) error

// compensations maps tool names to their undo functions.
// Tools without a compensation function are non-compensable (run_shell).
var compensations = map[string]CompensationFunc{
	"write_file": compensateWriteFile,
	"edit_file":  compensateEditFile,
}

// compensateWriteFile restores the file to its before state.
// If before is nil the file was new, so it is deleted.
func compensateWriteFile(_ context.Context, root string, args map[string]any, before []byte) error {
	rel := pathFromArgs(args)
	if rel == "" {
		return fmt.Errorf("write_file compensation: missing path argument")
	}
	abs, err := resolvePath(root, rel)
	if err != nil {
		return err
	}
	if before == nil {
		return os.Remove(abs) // created by the tool → delete on rollback
	}
	if err := os.MkdirAll(filepath.Dir(abs), 0o755); err != nil {
		return err
	}
	return os.WriteFile(abs, before, 0o644)
}

// compensateEditFile is identical to compensateWriteFile: both restore file bytes.
func compensateEditFile(ctx context.Context, root string, args map[string]any, before []byte) error {
	return compensateWriteFile(ctx, root, args, before)
}

// CompensatePending reads all compensable mutations for sessionID (applied or
// committed) in reverse chronological order, applies the appropriate compensation
// function for each, and transitions the mutation to rolled_back in the saga store.
//
// It uses a background context for all cleanup operations because the original
// request context may already be cancelled.
func (h *Host) CompensatePending(sessionID string) {
	if h.Saga == nil {
		return
	}
	ctx := context.Background()

	mutations, err := h.Saga.Compensable(ctx, sessionID)
	if err != nil {
		h.Obs.Error("saga: list compensable", err, "session", sessionID)
		return
	}

	// Process in reverse order so later writes are undone first.
	for i := len(mutations) - 1; i >= 0; i-- {
		m := mutations[i]
		fn, ok := compensations[m.Tool]
		if !ok {
			// Non-compensable tool (e.g. run_shell); log and mark rolled_back.
			h.Obs.Info("saga: non-compensable tool skipped",
				"tool", m.Tool, "mutation", m.ID)
		} else if fn != nil {
			if cerr := fn(ctx, h.Registry.Root(), m.Args, m.BeforeState); cerr != nil {
				h.Obs.Error("saga: compensation failed", cerr,
					"tool", m.Tool, "mutation", m.ID)
			}
		}
		_ = h.Saga.Transition(ctx, m.ID, storage.SagaRolledBack, nil)
	}
}
