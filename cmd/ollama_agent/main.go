package main

import (
	"fmt"
	"os"
	"path/filepath"

	tea "github.com/charmbracelet/bubbletea"
	"ollama_agent_go/internal/app"
	"ollama_agent_go/internal/config"
	ollamaprovider "ollama_agent_go/internal/providers/ollama"
	tui "ollama_agent_go/internal/ui/tui"
)

func main() {
	cfg := config.Load()

	logPath := os.Getenv("OLLAMA_AGENT_LOG")
	if logPath == "" {
		logPath = filepath.Join(cfg.Root, "ollama_agent.log")
	}
	logFile, err := os.OpenFile(logPath, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o644)
	if err != nil {
		logFile = nil
	}
	defer func() {
		if logFile != nil {
			logFile.Close()
		}
	}()

	application, err := app.New(cfg, logFile)
	if err != nil {
		fmt.Fprintf(os.Stderr, "startup error: %v\n", err)
		os.Exit(1)
	}

	// Register the TUI approval callback with the policy engine after the app
	// is built, so tui never imports tools or policy directly.
	application.Engine.ToolHost.Policy.RegisterApprovalCallback(tui.RequestApproval)

	// The Ollama client is needed only for the /models picker.
	ollamaClient := ollamaprovider.NewClient(cfg.BaseURL, cfg.Model)

	m := tui.NewModel(application.Engine, ollamaClient)

	p := tea.NewProgram(m,
		tea.WithAltScreen(),
		tea.WithMouseCellMotion(),
	)
	if _, err := p.Run(); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}
