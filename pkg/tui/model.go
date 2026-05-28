package tui

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/charmbracelet/bubbles/textinput"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"ollama_agent_go/pkg/agent"
	"ollama_agent_go/pkg/config"
	"ollama_agent_go/pkg/ollama"
	"ollama_agent_go/pkg/tools"
)

// ChatMessage is one rendered line of conversation. Role is "user", "agent",
// or "tool" (informational tool-activity lines).
type ChatMessage struct {
	Role    string
	Content string
}

// eventMsg carries an agent event into the Bubble Tea update loop.
type eventMsg agent.Event

// Model is the Bubble Tea model for the agent REPL.
type Model struct {
	viewport    viewport.Model
	textInput   textinput.Model
	history     []ChatMessage
	status      string
	isStreaming bool
	width       int
	height      int
	config      *config.Config
	agent       *agent.Agent
	ctx         context.Context
	cancel      context.CancelFunc
	events      chan agent.Event
}

// NewModel constructs the TUI model wired to a tool-using agent.
func NewModel(cfg *config.Config) Model {
	ti := textinput.New()
	ti.Placeholder = "Ask the agent..."
	ti.Focus()

	vp := viewport.New(80, 20)
	vp.SetContent("Ollama Agent (Go) ready. Tools: read/write/ls/grep/shell/edit.")

	ctx, cancel := context.WithCancel(context.Background())

	client := ollama.NewClient(cfg.BaseURL)
	registry := tools.Default(cfg.Root)
	ag := agent.New(client, registry, cfg.Model)

	return Model{
		textInput: ti,
		viewport:  vp,
		status:    "READY",
		config:    cfg,
		agent:     ag,
		ctx:       ctx,
		cancel:    cancel,
		events:    make(chan agent.Event, 64),
	}
}

func (m Model) Init() tea.Cmd { return textinput.Blink }

func waitForEvent(ch chan agent.Event) tea.Cmd {
	return func() tea.Msg {
		e, ok := <-ch
		if !ok {
			return eventMsg{Kind: agent.EventDone}
		}
		return eventMsg(e)
	}
}

// conversation builds the ollama message history (user/assistant only) for the
// agent, dropping informational tool lines.
func (m Model) conversation() []ollama.Message {
	var msgs []ollama.Message
	for _, h := range m.history {
		switch h.Role {
		case "user":
			msgs = append(msgs, ollama.Message{Role: "user", Content: h.Content})
		case "agent":
			msgs = append(msgs, ollama.Message{Role: "assistant", Content: h.Content})
		}
	}
	return msgs
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
			query := strings.TrimSpace(m.textInput.Value())
			if query == "" {
				return m, nil
			}
			m.history = append(m.history, ChatMessage{Role: "user", Content: query})
			m.textInput.Reset()
			m.isStreaming = true
			m.status = "THINKING"
			m.updateViewport()

			convo := m.conversation()
			go func() {
				_, _ = m.agent.Run(m.ctx, convo, func(e agent.Event) {
					m.events <- e
				})
				close(m.events)
			}()
			return m, waitForEvent(m.events)
		}

	case eventMsg:
		return m.handleEvent(agent.Event(msg))

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

func (m Model) handleEvent(e agent.Event) (tea.Model, tea.Cmd) {
	switch e.Kind {
	case agent.EventToken:
		m.appendAgentText(e.Text)
		m.updateViewport()
		return m, waitForEvent(m.events)

	case agent.EventToolCall:
		m.history = append(m.history, ChatMessage{Role: "tool", Content: "⚙ " + e.Tool + " " + compactArgs(e.Args)})
		m.status = "RUNNING " + e.Tool
		m.updateViewport()
		return m, waitForEvent(m.events)

	case agent.EventToolResult:
		m.history = append(m.history, ChatMessage{Role: "tool", Content: "↳ " + truncate(e.Text, 400)})
		m.updateViewport()
		return m, waitForEvent(m.events)

	case agent.EventError:
		m.history = append(m.history, ChatMessage{Role: "agent", Content: "Error: " + e.Text})
		m.isStreaming = false
		m.status = "ERROR"
		m.resetEvents()
		m.updateViewport()
		return m, nil

	case agent.EventDone:
		m.isStreaming = false
		m.status = "READY"
		m.resetEvents()
		m.updateViewport()
		return m, nil
	}
	return m, waitForEvent(m.events)
}

// resetEvents replaces the closed channel so the next turn has a fresh one.
func (m *Model) resetEvents() { m.events = make(chan agent.Event, 64) }

func (m *Model) appendAgentText(text string) {
	if n := len(m.history); n > 0 && m.history[n-1].Role == "agent" {
		m.history[n-1].Content += text
	} else {
		m.history = append(m.history, ChatMessage{Role: "agent", Content: text})
	}
}

func compactArgs(args map[string]any) string {
	if len(args) == 0 {
		return ""
	}
	b, err := json.Marshal(args)
	if err != nil {
		return ""
	}
	return truncate(string(b), 120)
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + "…"
}

func (m *Model) updateViewport() {
	var b strings.Builder
	for _, msg := range m.history {
		switch msg.Role {
		case "user":
			b.WriteString(UserLabelStyle.Render("YOU › "))
			b.WriteString(msg.Content)
		case "tool":
			b.WriteString(InfoStyle.Render(msg.Content))
		default:
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
