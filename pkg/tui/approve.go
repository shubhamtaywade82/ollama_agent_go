// Package tui/approve provides a huh-based confirmation gate for high-risk
// agent tool calls. It runs as a blocking BubbleTea sub-program.
package tui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/huh"
	"github.com/charmbracelet/lipgloss"
)

// HighRiskTools is the set of tool names that require explicit user approval.
var HighRiskTools = map[string]bool{
	"write_file": true,
	"run_shell":  true,
	"edit_file":  true,
}

// approvalResult is the outcome of an approval gate.
type approvalResult int

const (
	approvalGranted approvalResult = iota
	approvalDenied
)

// RequestApproval shows a huh confirmation form for a pending tool call.
// It blocks until the user responds. Returns approvalGranted or approvalDenied.
func RequestApproval(toolName string, args map[string]any) approvalResult {
	// Format args for display
	var argLines []string
	for k, v := range args {
		argLines = append(argLines, fmt.Sprintf("  %s: %v", k, v))
	}
	argsStr := strings.Join(argLines, "\n")

	headerStyle := lipgloss.NewStyle().
		Bold(true).
		Foreground(colorAmber).
		Border(lipgloss.RoundedBorder()).
		BorderForeground(colorAmber).
		Padding(1, 2).
		MarginBottom(1)

	header := headerStyle.Render(fmt.Sprintf(
		"⚠  Agent wants to call: %s\n\nArguments:\n%s",
		lipgloss.NewStyle().Foreground(colorCyan).Bold(true).Render(toolName),
		lipgloss.NewStyle().Foreground(colorGray).Render(argsStr),
	))
	fmt.Println(header)

	var confirmed bool
	form := huh.NewForm(
		huh.NewGroup(
			huh.NewConfirm().
				Title(fmt.Sprintf("Allow agent to run %q?", toolName)).
				Description("Review the arguments above before approving.").
				Affirmative("Yes, run it").
				Negative("No, skip").
				Value(&confirmed),
		),
	).WithTheme(huh.ThemeCatppuccin())

	if err := form.Run(); err != nil || !confirmed {
		return approvalDenied
	}
	return approvalGranted
}
