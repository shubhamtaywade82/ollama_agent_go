package main

import (
	"fmt"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/textinput"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// Styles for the UI
var (
	titleStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("#7D56F4")).
			Background(lipgloss.Color("#202020")).
			Padding(0, 1)

	infoStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#555555")).
			Italic(true)

	agentLabelStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#04B575")).
			Bold(true)

	userLabelStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#EE6FF8")).
			Bold(true)

	contentStyle = lipgloss.NewStyle().
			PaddingLeft(2)
)

// Messages for the state machine
type (
	statusMsg string
	streamMsg string
	doneMsg   struct{}
)

type chatMessage struct {
	role    string // "user" or "agent"
	content string
}

type model struct {
	viewport    viewport.Model
	textInput   textinput.Model
	history     []chatMessage
	status      string
	isStreaming bool
	err         error
	responses   chan string
	width       int
	height      int
}

func initialModel() model {
	ti := textinput.New()
	ti.Placeholder = "Ask the agent..."
	ti.Focus()
	ti.CharLimit = 156
	ti.Width = 40

	vp := viewport.New(80, 20)
	vp.SetContent("Welcome to Ollama Agent (Go Prototype)\nType a query to see reactive streaming...")

	return model{
		textInput: ti,
		viewport:  vp,
		status:    "READY",
		history:   []chatMessage{},
		responses: make(chan string),
	}
}

func (m model) Init() tea.Cmd {
	return textinput.Blink
}

func waitForResponse(ch chan string) tea.Cmd {
	return func() tea.Msg {
		token, ok := <-ch
		if !ok {
			return doneMsg{}
		}
		return streamMsg(token)
	}
}

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var (
		tiCmd tea.Cmd
		vpCmd tea.Cmd
	)

	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.Type {
		case tea.KeyCtrlC, tea.KeyEsc:
			return m, tea.Quit
		case tea.KeyEnter:
			if m.isStreaming {
				return m, nil
			}
			query := m.textInput.Value()
			if query == "" {
				return m, nil
			}
			m.history = append(m.history, chatMessage{role: "user", content: query})
			m.textInput.Reset()
			m.isStreaming = true
			m.status = "THINKING"
			m.updateViewport()
			
			go func() {
				words := strings.Fields("I am processing your request: " + query + ". I can help you with file edits, search, and more. This is a real Go goroutine streaming tokens reactively back to the UI thread! Notice how it now stays on one line and wraps correctly based on your terminal width.")
				for _, word := range words {
					time.Sleep(100 * time.Millisecond)
					m.responses <- word + " "
				}
				close(m.responses)
			}()

			return m, waitForResponse(m.responses)
		}

	case statusMsg:
		m.status = string(msg)
		return m, nil

	case streamMsg:
		if len(m.history) > 0 && m.history[len(m.history)-1].role == "agent" {
			m.history[len(m.history)-1].content += string(msg)
		} else {
			m.history = append(m.history, chatMessage{role: "agent", content: string(msg)})
		}
		m.updateViewport()
		return m, waitForResponse(m.responses)

	case doneMsg:
		m.isStreaming = false
		m.status = "READY"
		m.responses = make(chan string)
		return m, nil

	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		m.viewport.Width = msg.Width
		m.viewport.Height = msg.Height - 6
		m.textInput.Width = msg.Width - 4
		m.updateViewport()
	}

	m.textInput, tiCmd = m.textInput.Update(msg)
	m.viewport, vpCmd = m.viewport.Update(msg)
	return m, tea.Batch(tiCmd, vpCmd)
}

func (m *model) updateViewport() {
	var b strings.Builder
	for _, msg := range m.history {
		if msg.role == "user" {
			b.WriteString(userLabelStyle.Render("YOU › "))
			b.WriteString(msg.content)
		} else {
			b.WriteString(agentLabelStyle.Render("AGENT › "))
			// Basic wrapping for the agent's content
			b.WriteString(msg.content)
		}
		b.WriteString("\n\n")
	}
	
	content := b.String()
	// Use lipgloss for better wrapping if width is known
	if m.width > 0 {
		content = lipgloss.NewStyle().Width(m.width - 2).Render(content)
	}
	
	m.viewport.SetContent(content)
	m.viewport.GotoBottom()
}

func (m model) View() string {
	return fmt.Sprintf(
		"%s  %s\n\n%s\n\n%s\n%s",
		titleStyle.Render("OLLAMA AGENT (GO)"),
		infoStyle.Render(fmt.Sprintf("Status: %s", m.status)),
		m.viewport.View(),
		m.textInput.View(),
		infoStyle.Render(" (ctrl+c to quit)"),
	)
}

func main() {
	p := tea.NewProgram(initialModel(), tea.WithAltScreen())
	if _, err := p.Run(); err != nil {
		fmt.Printf("Alas, there's been an error: %v", err)
	}
}
