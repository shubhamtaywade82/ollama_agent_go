package main

import (
	"fmt"
	"os"

	tea "github.com/charmbracelet/bubbletea"
	"ollama_agent_go/pkg/config"
	"ollama_agent_go/pkg/tui"
)

func main() {
	cfg := config.Load()

	p := tea.NewProgram(tui.NewModel(cfg), tea.WithAltScreen())
	if _, err := p.Run(); err != nil {
		fmt.Printf("Alas, there's been an error: %v", err)
		os.Exit(1)
	}
}
