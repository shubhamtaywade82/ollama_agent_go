// Package tui is the thin BubbleTea view layer. It renders conversation
// state and sends user intent to the runtime engine. It does not create
// agents, registries, or clients — all of that lives in internal/runtime.
package tui

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/spinner"
	"github.com/charmbracelet/bubbles/textinput"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/glamour"
	"github.com/charmbracelet/lipgloss"
	"ollama_agent_go/internal/agent"
	ollamaprovider "ollama_agent_go/internal/providers/ollama"
	"ollama_agent_go/internal/runtime"
	"ollama_agent_go/internal/tokens"
)

// ── Agent event plumbing ─────────────────────────────────────────────────────

type agentEvent agent.Event
type agentDone struct{ err error }

// ── Chat entry types ─────────────────────────────────────────────────────────

type entryKind int

const (
	entryUser entryKind = iota
	entryAgent
	entryToolCall
	entryToolResult
	entryError
	entrySystem
)

// ChatEntry is one rendered block in the conversation viewport.
type ChatEntry struct {
	Kind    entryKind
	Content string
	Tool    string
}

// ── Status ───────────────────────────────────────────────────────────────────

type appStatus int

const (
	statusReady appStatus = iota
	statusThinking
	statusTool
	statusError
)

type sessionStats struct {
	turns     int
	toolCalls int
	startedAt time.Time
}

// ── Model ────────────────────────────────────────────────────────────────────

// Model is the root BubbleTea model. It delegates all business logic to Engine.
type Model struct {
	width, height int

	viewport  viewport.Model
	textInput textinput.Model
	spinner   spinner.Model

	status      appStatus
	statusText  string
	entries     []ChatEntry
	stats       sessionStats
	currentResp strings.Builder
	tokenCount  int

	// engine is the ONLY business-logic dependency of the TUI.
	engine *runtime.Engine

	// ollamaClient is kept only for the /models picker which lists local models.
	ollamaClient *ollamaprovider.Client

	ctx    context.Context
	cancel context.CancelFunc

	eventCh chan agent.Event

	modelsLoading bool
	renderer      *glamour.TermRenderer
}

// NewModel builds the initial TUI model wrapping the given engine.
func NewModel(engine *runtime.Engine, ollamaClient *ollamaprovider.Client) *Model {
	ti := textinput.New()
	ti.Placeholder = "Type a message or /help…"
	ti.Focus()
	ti.CharLimit = 4096

	vp := viewport.New(80, 20)
	vp.SetContent(welcomeText())

	sp := spinner.New()
	sp.Spinner = spinner.Dot
	sp.Style = lipgloss.NewStyle().Foreground(colorAmber)

	renderer, _ := glamour.NewTermRenderer(
		glamour.WithAutoStyle(),
		glamour.WithWordWrap(0),
	)

	ctx, cancel := context.WithCancel(context.Background())

	return &Model{
		viewport:     vp,
		textInput:    ti,
		spinner:      sp,
		status:       statusReady,
		engine:       engine,
		ollamaClient: ollamaClient,
		renderer:     renderer,
		ctx:          ctx,
		cancel:       cancel,
		stats:        sessionStats{startedAt: time.Now()},
	}
}

// ── Init ─────────────────────────────────────────────────────────────────────

func (m *Model) Init() tea.Cmd {
	return tea.Batch(textinput.Blink, m.spinner.Tick)
}

// ── Update ────────────────────────────────────────────────────────────────────

func (m *Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmds []tea.Cmd

	switch msg := msg.(type) {

	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		sidebarW := 28
		chatW := m.width - sidebarW - 2
		if chatW < 20 {
			chatW = 20
			sidebarW = 0
		}
		_ = sidebarW
		m.viewport.Width = chatW
		m.viewport.Height = m.height - 7
		m.textInput.Width = chatW - 4
		if m.renderer != nil {
			m.renderer, _ = glamour.NewTermRenderer(
				glamour.WithAutoStyle(),
				glamour.WithWordWrap(chatW-4),
			)
		}
		m.rebuildViewport()

	case tea.KeyMsg:
		switch msg.Type {
		case tea.KeyCtrlC:
			m.cancel()
			return m, tea.Quit

		case tea.KeyEsc:
			if m.status == statusThinking || m.status == statusTool {
				m.cancel()
				m.ctx, m.cancel = context.WithCancel(context.Background())
				m.status = statusReady
				m.entries = append(m.entries, ChatEntry{
					Kind:    entrySystem,
					Content: "⚠️  Request cancelled.",
				})
				m.rebuildViewport()
				return m, nil
			}

		case tea.KeyEnter:
			if m.status != statusReady {
				return m, nil
			}
			input := strings.TrimSpace(m.textInput.Value())
			if input == "" {
				return m, nil
			}
			m.textInput.Reset()
			cmd := m.handleInput(input)
			return m, cmd

		case tea.KeyCtrlL:
			m.entries = nil
			m.rebuildViewport()
			return m, nil
		}

	case spinner.TickMsg:
		var spCmd tea.Cmd
		m.spinner, spCmd = m.spinner.Update(msg)
		cmds = append(cmds, spCmd)

	case agentEvent:
		if m.eventCh != nil {
			cmds = append(cmds, m.handleAgentEvent(agent.Event(msg)))
		}

	case agentDone:
		m.eventCh = nil
		if m.currentResp.Len() > 0 {
			rendered := m.renderMarkdown(m.currentResp.String())
			m.entries = append(m.entries, ChatEntry{Kind: entryAgent, Content: rendered})
			m.currentResp.Reset()
		}
		if msg.err != nil {
			m.entries = append(m.entries, ChatEntry{
				Kind:    entryError,
				Content: fmt.Sprintf("Error: %v", msg.err),
			})
			m.status = statusError
		} else {
			m.status = statusReady
		}
		m.rebuildViewport()

	case modelsLoadedMsg:
		m.modelsLoading = false
		if msg.err != nil {
			m.entries = append(m.entries, ChatEntry{
				Kind:    entryError,
				Content: fmt.Sprintf("Could not fetch models: %v", msg.err),
			})
			m.rebuildViewport()
			break
		}
		if len(msg.models) == 0 {
			m.entries = append(m.entries, ChatEntry{
				Kind:    entrySystem,
				Content: "No models found. Pull one with `ollama pull <model>`.",
			})
			m.rebuildViewport()
			break
		}
		picker := newModelsPicker(msg.models, m.width-4, m.height-4)
		sub := tea.NewProgram(picker, tea.WithAltScreen())
		result, err := sub.Run()
		if err == nil {
			if pm, ok := result.(modelsPicker); ok && pm.chosen != "" {
				m.engine.SetModel(pm.chosen)
				m.entries = append(m.entries, ChatEntry{
					Kind:    entrySystem,
					Content: fmt.Sprintf("✓ Switched to **%s**", pm.chosen),
				})
			}
		}
		m.rebuildViewport()
	}

	var tiCmd, vpCmd tea.Cmd
	m.textInput, tiCmd = m.textInput.Update(msg)
	m.viewport, vpCmd = m.viewport.Update(msg)
	cmds = append(cmds, tiCmd, vpCmd)

	return m, tea.Batch(cmds...)
}

// handleInput processes a user message or slash command.
func (m *Model) handleInput(input string) tea.Cmd {
	if strings.HasPrefix(input, "/") {
		return m.handleSlashCommand(input)
	}

	m.entries = append(m.entries, ChatEntry{Kind: entryUser, Content: input})
	m.stats.turns++
	m.status = statusThinking
	m.rebuildViewport()

	return m.runAgent(input)
}

// handleSlashCommand interprets /command inputs.
func (m *Model) handleSlashCommand(input string) tea.Cmd {
	parts := strings.Fields(input)
	cmd := strings.ToLower(parts[0])

	switch cmd {
	case "/help":
		m.entries = append(m.entries, ChatEntry{Kind: entrySystem, Content: helpText()})

	case "/clear":
		m.entries = nil
		m.stats = sessionStats{startedAt: time.Now()}
		_ = m.engine.ClearHistory(m.ctx)
		m.entries = append(m.entries, ChatEntry{Kind: entrySystem, Content: "Session cleared."})

	case "/model":
		if len(parts) > 1 {
			m.engine.SetModel(parts[1])
			m.entries = append(m.entries, ChatEntry{
				Kind:    entrySystem,
				Content: fmt.Sprintf("✓ Switched model to **%s**", parts[1]),
			})
		} else {
			m.entries = append(m.entries, ChatEntry{
				Kind:    entrySystem,
				Content: fmt.Sprintf("Current model: **%s**\nUsage: `/model <name>`", m.engine.Model()),
			})
		}

	case "/tools":
		var b strings.Builder
		b.WriteString("**Available Tools**\n\n")
		for _, name := range m.engine.ToolNames() {
			if t, ok := m.engine.GetTool(name); ok {
				b.WriteString(fmt.Sprintf("- **%s**: %s\n", t.Name(), t.Description()))
			}
		}
		m.entries = append(m.entries, ChatEntry{Kind: entrySystem, Content: b.String()})

	case "/session":
		m.entries = append(m.entries, ChatEntry{
			Kind: entrySystem,
			Content: fmt.Sprintf(
				"**Session Info**\n\n- Model: `%s`\n- Session: `%s`\n- Turns: %d\n- Tool calls: %d\n- Uptime: %s",
				m.engine.Model(),
				m.engine.SessionID(),
				m.stats.turns,
				m.stats.toolCalls,
				time.Since(m.stats.startedAt).Round(time.Second),
			),
		})

	case "/models":
		if m.modelsLoading {
			break
		}
		m.modelsLoading = true
		m.entries = append(m.entries, ChatEntry{
			Kind:    entrySystem,
			Content: "⟳ Fetching available models from Ollama…",
		})
		m.rebuildViewport()
		return fetchModels(m.ollamaClient)

	case "/skills":
		loaded := m.engine.LoadedSkills()
		if len(loaded) == 0 {
			m.entries = append(m.entries, ChatEntry{
				Kind:    entrySystem,
				Content: "No skills loaded.",
			})
		} else {
			var b strings.Builder
			b.WriteString(fmt.Sprintf("**Loaded Skills** (%d)\n\n", len(loaded)))
			for _, s := range loaded {
				if s.Description != "" {
					b.WriteString(fmt.Sprintf("- **%s**: %s\n", s.Name, s.Description))
				} else {
					b.WriteString(fmt.Sprintf("- **%s**\n", s.Name))
				}
			}
			m.entries = append(m.entries, ChatEntry{Kind: entrySystem, Content: b.String()})
		}

	case "/audit":
		n := 10
		if len(parts) > 1 {
			fmt.Sscanf(parts[1], "%d", &n)
		}
		events, err := m.engine.ExportAudit(m.ctx)
		if err != nil {
			m.entries = append(m.entries, ChatEntry{
				Kind:    entryError,
				Content: fmt.Sprintf("Audit query failed: %v", err),
			})
			break
		}
		if len(events) == 0 {
			m.entries = append(m.entries, ChatEntry{Kind: entrySystem, Content: "No audit events for this session."})
			break
		}
		if len(events) > n {
			events = events[len(events)-n:]
		}
		var b strings.Builder
		b.WriteString(fmt.Sprintf("**Audit Log** (last %d events)\n\n", len(events)))
		for _, ev := range events {
			ts := ev.At.Format("15:04:05")
			b.WriteString(fmt.Sprintf("- `%s` [%s] **%s** %s → %s\n",
				ts, ev.Actor, ev.Action, ev.Resource, ev.Result))
		}
		m.entries = append(m.entries, ChatEntry{Kind: entrySystem, Content: b.String()})

	case "/metrics":
		snap := m.engine.MetricsSnapshot()
		var b strings.Builder
		b.WriteString("**Metrics Snapshot**\n\n")
		b.WriteString(fmt.Sprintf("- Total tokens: %d\n", snap.TotalTokens))
		b.WriteString(fmt.Sprintf("- Total errors: %d\n", snap.Errors))
		if len(snap.ToolCalls) > 0 {
			b.WriteString("\n**Tool Calls**\n\n")
			for tool, stats := range snap.ToolCalls {
				b.WriteString(fmt.Sprintf("- %s: %d calls, %d errors\n", tool, stats.Total, stats.Errors))
			}
		}
		m.entries = append(m.entries, ChatEntry{Kind: entrySystem, Content: b.String()})

	default:
		m.entries = append(m.entries, ChatEntry{
			Kind:    entryError,
			Content: fmt.Sprintf("Unknown command: `%s`. Type `/help` for a list.", cmd),
		})
	}

	m.rebuildViewport()
	return nil
}

// runAgent delegates to engine.Submit in a goroutine and pipes events back.
func (m *Model) runAgent(input string) tea.Cmd {
	ch := make(chan agent.Event, 64)
	m.eventCh = ch

	emit := func(ev agent.Event) {
		select {
		case ch <- ev:
		case <-m.ctx.Done():
		}
	}

	go func() {
		defer close(ch)
		_ = m.engine.Submit(m.ctx, input, emit)
	}()

	return waitForAgentEvent(ch)
}

func waitForAgentEvent(ch <-chan agent.Event) tea.Cmd {
	return func() tea.Msg {
		ev, ok := <-ch
		if !ok {
			return agentDone{}
		}
		if ev.Kind == agent.EventError {
			return agentDone{err: fmt.Errorf("%s", ev.Text)}
		}
		return agentEvent(ev)
	}
}

func (m *Model) handleAgentEvent(ev agent.Event) tea.Cmd {
	switch ev.Kind {
	case agent.EventToken:
		m.tokenCount += tokens.Estimate(ev.Text)
		m.currentResp.WriteString(ev.Text)
		m.rebuildViewport()

	case agent.EventToolCall:
		if m.currentResp.Len() > 0 {
			rendered := m.renderMarkdown(m.currentResp.String())
			m.entries = append(m.entries, ChatEntry{Kind: entryAgent, Content: rendered})
			m.currentResp.Reset()
		}
		m.stats.toolCalls++
		m.status = statusTool
		m.statusText = ev.Tool
		var argParts []string
		for k, v := range ev.Args {
			argParts = append(argParts, fmt.Sprintf("`%s`: %v", k, v))
		}
		m.entries = append(m.entries, ChatEntry{
			Kind:    entryToolCall,
			Content: fmt.Sprintf("**%s**(%s)", ev.Tool, strings.Join(argParts, ", ")),
			Tool:    ev.Tool,
		})
		m.rebuildViewport()

	case agent.EventToolResult:
		m.status = statusThinking
		content := ev.Text
		if len(content) > 400 {
			content = content[:400] + "…"
		}
		m.entries = append(m.entries, ChatEntry{
			Kind:    entryToolResult,
			Content: content,
			Tool:    ev.Tool,
		})
		m.rebuildViewport()

	case agent.EventDone:
		// handled by agentDone message when channel closes
	}

	if m.eventCh != nil {
		return waitForAgentEvent(m.eventCh)
	}
	return nil
}

// ── View ──────────────────────────────────────────────────────────────────────

func (m *Model) View() string {
	if m.width == 0 {
		return "Initializing…"
	}

	sidebarW := 28
	chatW := m.width - sidebarW - 2
	if chatW < 30 {
		return m.singleColumnView()
	}

	header := m.renderHeader()
	chat := m.renderChat(chatW)
	sidebar := m.renderSidebar()
	input := m.renderInput(chatW)
	statusBar := m.renderStatusBar()

	body := lipgloss.JoinHorizontal(lipgloss.Top, chat, "  ", sidebar)

	return lipgloss.JoinVertical(lipgloss.Left,
		header,
		body,
		input,
		statusBar,
	)
}

func (m *Model) renderHeader() string {
	title := TitleStyle.Render("⬡ OLLAMA AGENT")
	sub := SubtitleStyle.Render(fmt.Sprintf(" go-powered terminal intelligence • model: %s", m.engine.Model()))
	return lipgloss.JoinHorizontal(lipgloss.Center, title, sub) + "\n"
}

func (m *Model) renderChat(width int) string {
	m.viewport.Width = width
	return ViewportStyle.Width(width).Render(m.viewport.View())
}

func (m *Model) renderSidebar() string {
	uptime := time.Since(m.stats.startedAt).Round(time.Second)

	rows := []string{
		SidebarTitleStyle.Render("◈ Session"),
		row("Model", m.engine.Model()),
		row("Turns", fmt.Sprintf("%d", m.stats.turns)),
		row("Tools", fmt.Sprintf("%d calls", m.stats.toolCalls)),
		row("~Tokens", fmt.Sprintf("%d", m.tokenCount)),
		row("Uptime", uptime.String()),
		"",
		SidebarTitleStyle.Render("◈ Status"),
		m.renderStatusBadge(),
		"",
		SidebarTitleStyle.Render("◈ Keys"),
		shortcut("Enter", "send"),
		shortcut("Esc", "cancel"),
		shortcut("Ctrl+L", "clear screen"),
		shortcut("Ctrl+C", "quit"),
		"",
		SidebarTitleStyle.Render("◈ Commands"),
		shortcut("/help", "commands"),
		shortcut("/model", "switch model"),
		shortcut("/models", "browse models"),
		shortcut("/tools", "list tools"),
		shortcut("/skills", "list skills"),
		shortcut("/session", "stats"),
		shortcut("/audit", "audit log"),
		shortcut("/metrics", "counters"),
		shortcut("/clear", "new session"),
	}

	return SidebarStyle.Render(strings.Join(rows, "\n"))
}

func (m *Model) renderStatusBadge() string {
	switch m.status {
	case statusThinking:
		return StatusThinkingStyle.Render("⟳ THINKING")
	case statusTool:
		return StatusToolStyle.Render(fmt.Sprintf("⚙ %s", m.statusText))
	case statusError:
		return StatusErrorStyle.Render("✖ ERROR")
	default:
		return StatusReadyStyle.Render("✓ READY")
	}
}

func (m *Model) renderInput(width int) string {
	prompt := "› "
	style := InputBorderStyle
	if m.status != statusReady {
		prompt = m.spinner.View() + " "
		style = InputFocusBorderStyle
	}
	return style.Width(width).Render(prompt + m.textInput.View())
}

func (m *Model) renderStatusBar() string {
	left := HelpStyle.Render(" ctrl+c quit • esc cancel • /help commands")
	right := HelpStyle.Render(fmt.Sprintf("session: %s ", m.engine.SessionID()))
	gap := m.width - lipgloss.Width(left) - lipgloss.Width(right)
	if gap < 0 {
		gap = 0
	}
	return StatusBarStyle.Width(m.width).Render(left + strings.Repeat(" ", gap) + right)
}

func (m *Model) singleColumnView() string {
	return fmt.Sprintf("%s\n%s\n%s\n%s",
		TitleStyle.Render("⬡ OLLAMA AGENT"),
		m.viewport.View(),
		"› "+m.textInput.View(),
		HelpStyle.Render("ctrl+c quit"),
	)
}

// ── Viewport rendering ────────────────────────────────────────────────────────

func (m *Model) rebuildViewport() {
	var b strings.Builder
	for _, entry := range m.entries {
		b.WriteString(m.renderEntry(entry))
		b.WriteString("\n")
	}
	if m.currentResp.Len() > 0 {
		b.WriteString(AgentLabelStyle.Render("AGENT ›"))
		b.WriteString(" ")
		b.WriteString(m.currentResp.String())
		b.WriteString("\n")
	}
	m.viewport.SetContent(b.String())
	m.viewport.GotoBottom()
}

func (m *Model) renderEntry(e ChatEntry) string {
	switch e.Kind {
	case entryUser:
		return UserLabelStyle.Render("YOU ›") + " " + e.Content

	case entryAgent:
		return AgentLabelStyle.Render("AGENT ›") + "\n" + e.Content

	case entryToolCall:
		label := ToolCallLabelStyle.Render(fmt.Sprintf("⚙ TOOL › %s", e.Tool))
		content := m.renderMarkdown(e.Content)
		return ToolCardStyle.Render(label + "\n" + content)

	case entryToolResult:
		label := ToolResultLabelStyle.Render(fmt.Sprintf("  ↳ result from %s", e.Tool))
		return label + "\n" + InfoStyle.Render(e.Content)

	case entryError:
		return ErrorLabelStyle.Render("✖ ERROR") + " " + e.Content

	case entrySystem:
		rendered := m.renderMarkdown(e.Content)
		return InfoStyle.Render("──────────────────────────────") + "\n" +
			rendered +
			InfoStyle.Render("──────────────────────────────")
	}
	return e.Content
}

func (m *Model) renderMarkdown(content string) string {
	if m.renderer == nil || content == "" {
		return content
	}
	out, err := m.renderer.Render(content)
	if err != nil {
		return content
	}
	return out
}

// ── Helpers ───────────────────────────────────────────────────────────────────

func row(label, value string) string {
	return SidebarLabelStyle.Render(label+": ") + SidebarValueStyle.Render(value)
}

func shortcut(key, desc string) string {
	return SidebarKeyStyle.Render(key) + SidebarLabelStyle.Render(" "+desc)
}

func welcomeText() string {
	return strings.Join([]string{
		"",
		"  Welcome to **Ollama Agent** (Go edition)",
		"",
		"  Ask me anything — I have access to your project files and shell.",
		"  Type `/help` to see all commands.",
		"",
	}, "\n")
}

func helpText() string {
	return strings.Join([]string{
		"**Ollama Agent Commands**",
		"",
		"| Command | Description |",
		"|---------|-------------|",
		"| `/help` | Show this help |",
		"| `/model [name]` | Show or switch the active model |",
		"| `/models` | Browse & select local models (interactive list) |",
		"| `/tools` | List all available tools |",
		"| `/skills` | List loaded skills |",
		"| `/session` | Show session statistics |",
		"| `/audit [n]` | Show last N audit events (default 10) |",
		"| `/metrics` | Show call counters and token totals |",
		"| `/clear` | Clear history and start a new session |",
		"",
		"**Keyboard Shortcuts**",
		"",
		"| Key | Action |",
		"|----|--------|",
		"| `Enter` | Send message |",
		"| `Esc` | Cancel running request |",
		"| `Ctrl+L` | Clear screen |",
		"| `Ctrl+C` | Quit |",
	}, "\n")
}
