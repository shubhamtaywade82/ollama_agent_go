package main

import (
	"fmt"
	"os"
	"path/filepath"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/log"
	"ollama_agent_go/pkg/config"
	"ollama_agent_go/pkg/tui"
)

func main() {
	cfg := config.Load()

	// Set up a file-based structured logger so debug output doesn't pollute
	// the TUI surface. Log path: $OLLAMA_AGENT_LOG or <cwd>/ollama_agent.log
	logPath := os.Getenv("OLLAMA_AGENT_LOG")
	if logPath == "" {
		logPath = filepath.Join(cfg.Root, "ollama_agent.log")
	}
	logFile, err := os.OpenFile(logPath, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o644)
	if err != nil {
		// Non-fatal: carry on without logging
		logFile = nil
	}
	defer func() {
		if logFile != nil {
			logFile.Close()
		}
	}()

	// Build TUI model
	m := tui.NewModel(cfg)

	// Wire the log file into the model's logger if we have one
	if logFile != nil {
		logger := log.New(logFile)
		logger.SetLevel(log.DebugLevel)
		m.SetLogger(logger)
		logger.Info("session started", "model", cfg.Model, "root", cfg.Root)
	}

	p := tea.NewProgram(m,
		tea.WithAltScreen(),
		tea.WithMouseCellMotion(), // smooth scroll in viewport
	)
	if _, err := p.Run(); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}
