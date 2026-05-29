package tools

import (
	"context"
	"fmt"
	"os/exec"
)

// Delegate tool allows the agent to call out to other specialized CLI agents.
type Delegate struct {
	// Approval is a callback to ask the user for permission.
	Approval func(agent, prompt string) bool
}

func (t *Delegate) Name() string { return "delegate" }
func (t *Delegate) Description() string {
	return "Delegate a complex sub-task to a specialized external agent (e.g. Gemini CLI, Claude CLI). Requires user approval."
}

func (t *Delegate) Parameters() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"agent":  map[string]any{"type": "string", "enum": []string{"gemini", "claude", "gpt"}, "description": "The external agent to invoke."},
			"prompt": map[string]any{"type": "string", "description": "The detailed instructions for the sub-agent."},
		},
		"required": []string{"agent", "prompt"},
	}
}

func (t *Delegate) Execute(_ context.Context, args map[string]any) (string, error) {
	agent, _ := argString(args, "agent")
	prompt, _ := argString(args, "prompt")

	if t.Approval != nil {
		if !t.Approval(agent, prompt) {
			return "Delegation cancelled by user.", nil
		}
	}

	var cmd *exec.Cmd
	switch agent {
	case "gemini":
		cmd = exec.Command("gemini", prompt)
	case "claude":
		cmd = exec.Command("claude", prompt)
	case "gpt":
		cmd = exec.Command("chatgpt", prompt)
	default:
		return "", fmt.Errorf("unsupported agent %q", agent)
	}

	out, err := cmd.CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("delegation to %s failed: %w\nOutput: %s", agent, err, string(out))
	}

	return fmt.Sprintf("Delegation to %s completed.\n\nResponse:\n%s", agent, string(out)), nil
}
