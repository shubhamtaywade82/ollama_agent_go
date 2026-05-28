package tui

import (
	"context"
	"fmt"
	"strings"

	"github.com/charmbracelet/bubbles/textinput"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"ollama_agent_go/pkg/config"
	"ollama_agent_go/pkg/ollama"
)

type ChatMessage struct {
	Role    string
	Content string
}

type StreamMsg struct {
	Token string
	Done  bool
	Err   error
}

type Model struct {
	viewport    viewport.Model
	textInput   textinput.Model
	history     []ChatMessage
	status      string
	isStreaming bool
	width       int
	height      int
	config      *config.Config
	client      *ollama.Client
	ctx         context.Context
	cancel      context.CancelFunc
	tokens      chan StreamMsg
}

func NewModel(cfg *config.Config) Model {
	ti := textinput.New()
	ti.Placeholder = "Ask the agent..."
	ti.Focus()

	vp := viewport.New(80, 20)
	vp.SetContent("Ollama Agent (Go) Ready.")

	ctx, cancel := context.WithCancel(context.Background())

	return Model{
		textInput: ti,
		viewport:  vp,
		status:    "READY",
		config:    cfg,
		client:    ollama.NewClient(cfg.BaseURL),
		ctx:       ctx,
		cancel:    cancel,
		tokens:    make(chan StreamMsg, 100),
	}
}

func (m Model) Init() tea.Cmd {
	return textinput.Blink
}

func waitForToken(ch chan StreamMsg) tea.Cmd {
	return func() tea.Msg {
		return <-ch
	}
}

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var (
		tiCmd tea.Cmd
		vpCmd tea.Cmd
	)

	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.Type {
		case tea.KeyCtrlC, tea.KeyEsc:
			m.cancel()
			return m, tea.Quit
		case tea.KeyEnter:
			if m.isStreaming {
				return m, nil
			}
			query := m.textInput.Value()
			if query == "" {
				return m, nil
			}
			m.history = append(m.history, ChatMessage{Role: "user", Content: query})
			m.textInput.Reset()
			m.isStreaming = true
			m.status = "THINKING"
			m.updateViewport()

			// Prepare messages for Ollama
			ollamaMsgs := make([]ollama.Message, len(m.history))
			for i, hm := range m.history {
				ollamaMsgs[i] = ollama.Message{Role: hm.Role, Content: hm.Content}
			}

			// Run chat in goroutine
			go func() {
				err := m.client.ChatStream(m.ctx, ollama.ChatRequest{
					Model:    m.config.Model,
					Messages: ollamaMsgs,
				}, func(resp ollama.ChatResponse) {
					m.tokens <- StreamMsg{Token: resp.Message.Content, Done: resp.Done}
				})
				if err != nil {
					m.tokens <- StreamMsg{Err: err}
				}
			}()

			return m, waitForToken(m.tokens)
		}

	case StreamMsg:
		if msg.Err != nil {
			m.history = append(m.history, ChatMessage{Role: "agent", Content: fmt.Sprintf("Error: %v", msg.Err)})
			m.isStreaming = false
			m.status = "ERROR"
			m.updateViewport()
			return m, nil
		}

		if len(m.history) > 0 && m.history[len(m.history)-1].Role == "agent" {
			m.history[len(m.history)-1].Content += msg.Token
		} else {
			m.history = append(m.history, ChatMessage{Role: "agent", Content: msg.Token})
		}
		m.updateViewport()

		if msg.Done {
			m.isStreaming = false
			m.status = "READY"
			return m, nil
		}
		return m, waitForToken(m.tokens)

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

func (m *Model) updateViewport() {
	var b strings.Builder
	for _, msg := range m.history {
		if msg.Role == "user" {
			b.WriteString(UserLabelStyle.Render("YOU › "))
			b.WriteString(msg.Content)
		} else {
			b.WriteString(AgentLabelStyle.Render("AGENT › "))
			b.WriteString(msg.Content)
		}
		b.WriteString("\n\n")
	}

	content := b.String()
	if m.width > 0 {
		content = lipgloss.NewStyle().Width(m.width - 2).Render(content)
	}

	m.viewport.SetContent(content)
	m.viewport.GotoBottom()
}

func (m Model) View() string {
	return fmt.Sprintf(
		"%s  %s\n\n%s\n\n%s\n%s",
		TitleStyle.Render("OLLAMA AGENT (GO)"),
		InfoStyle.Render(fmt.Sprintf("Status: %s | Model: %s", m.status, m.config.Model)),
		m.viewport.View(),
		m.textInput.View(),
		InfoStyle.Render(" (ctrl+c to quit)"),
	)
}
