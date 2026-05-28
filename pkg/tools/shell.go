package tools

import (
	"bytes"
	"context"
	"fmt"
	"os/exec"
	"time"
)

const defaultShellTimeout = 60 * time.Second

// RunShell executes a shell command with the sandbox root as the working
// directory. Output is captured and truncated; the command runs under a
// timeout. This is a privileged tool — callers should gate it behind approval.
type RunShell struct {
	Root    string
	Timeout time.Duration
}

func (t *RunShell) Name() string { return "run_shell" }
func (t *RunShell) Description() string {
	return "Run a shell command in the project root and return its combined stdout/stderr and exit status. Use for builds, tests, and git. Commands run under a timeout."
}

func (t *RunShell) Parameters() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"command": map[string]any{"type": "string", "description": "The shell command line to execute."},
		},
		"required": []string{"command"},
	}
}

func (t *RunShell) Execute(ctx context.Context, args map[string]any) (string, error) {
	command, err := argString(args, "command")
	if err != nil {
		return "", err
	}
	timeout := t.Timeout
	if timeout <= 0 {
		timeout = defaultShellTimeout
	}
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	cmd := exec.CommandContext(ctx, "sh", "-c", command)
	cmd.Dir = t.Root
	var buf bytes.Buffer
	cmd.Stdout = &buf
	cmd.Stderr = &buf
	runErr := cmd.Run()

	out := buf.String()
	if ctx.Err() == context.DeadlineExceeded {
		return out, fmt.Errorf("command timed out after %s", timeout)
	}
	if runErr != nil {
		// Return output plus the exit error; the agent can read both.
		return fmt.Sprintf("%s\n[exit: %v]", out, runErr), nil
	}
	if out == "" {
		out = "(no output)"
	}
	return out, nil
}
