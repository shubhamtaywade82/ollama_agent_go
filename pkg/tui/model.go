package tui

import (
	"context"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/spinner"
	"github.com/charmbracelet/bubbles/textinput"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/glamour"
	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/log"
	"ollama_agent_go/pkg/agent"
	"ollama_agent_go/pkg/config"
	"ollama_agent_go/pkg/ollama"
	"ollama_agent_go/pkg/skills"
	"ollama_agent_go/pkg/tokens"
	"ollama_agent_go/pkg/tools"
)

// ── Agent event plumbing ─────────────────────────────────────────────────────

// agentEvent wraps agent.Event for BubbleTea's message bus.
type agentEvent agent.Event

// agentDone is sent when the Run goroutine finishes (success or error).
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

// ChatEntry is one rendered line/block in the conversation viewport.
type ChatEntry struct {
	Kind    entryKind
	Content string
	Tool    string // for tool events
}

// ── Status ───────────────────────────────────────────────────────────────────

type appStatus int

const (
	statusReady appStatus = iota
	statusThinking
	statusTool
	statusError
)

// ── Session stats ─────────────────────────────────────────────────────────────

type sessionStats struct {
	turns     int
	toolCalls int
	startedAt time.Time
}

// ── Main model ────────────────────────────────────────────────────────────────

// Model is the root BubbleTea model for the Ollama Agent TUI.
type Model struct {
	// Layout
	width  int
	height int

	// Sub-models
	viewport  viewport.Model
	textInput textinput.Model
	spinner   spinner.Model

	// App state
	status      appStatus
	statusText  string // current tool name when status == statusTool
	entries     []ChatEntry
	history     []ollama.Message // raw history for agent
	stats       sessionStats
	currentResp strings.Builder // accumulate streaming agent text
	tokenCount  int             // cumulative estimated tokens streamed

	// Config & backend
	cfg          *config.Config
	agent        *agent.Agent
	registry     agent.ToolRunner
	loadedSkills []skills.Skill // skills injected into the system prompt
	ollamaClient  *ollama.Client

	// Context for cancellable runs
	ctx    context.Context
	cancel context.CancelFunc

	// Active agent event channel (nil when idle)
	eventCh chan agent.Event

	// /models loading state
	modelsLoading bool

	// Glamour renderer (markdown)
	renderer *glamour.TermRenderer

	// Logger (writes to a log file; not to the TUI surface)
	logger *log.Logger
}

// NewModel builds the initial TUI model.
func NewModel(cfg *config.Config) *Model {
	// Text input
	ti := textinput.New()
	ti.Placeholder = "Type a message or /help…"
	ti.Focus()
	ti.CharLimit = 4096

	// Viewport
	vp := viewport.New(80, 20)
	vp.SetContent(welcomeText())

	// Spinner
	sp := spinner.New()
	sp.Spinner = spinner.Dot
	sp.Style = lipgloss.NewStyle().Foreground(colorAmber)

	// Glamour renderer (dark auto-theme)
	renderer, _ := glamour.NewTermRenderer(
		glamour.WithAutoStyle(),
		glamour.WithWordWrap(0), // we control wrapping via lipgloss
	)

	// File logger so we don't pollute the TUI surface
	logger := log.New(nil) // nil = discard by default; caller can redirect
	logger.SetLevel(log.DebugLevel)

	// Tool registry sandboxed to project root.
	// Wrap with approval gate for destructive tools unless OLLAMA_AGENT_NO_APPROVAL=1.
	baseRegistry := tools.Default(cfg.Root)
	var registry agent.ToolRunner = baseRegistry
	if os.Getenv("OLLAMA_AGENT_NO_APPROVAL") != "1" {
		gatedNames := []string{"write_file", "run_shell", "edit_file"}
		registry = tools.NewGatedRegistry(baseRegistry, gatedNames, func(name string, args map[string]any) bool {
			return RequestApproval(name, args) == approvalGranted
		})
	}

	// Ollama client + agent
	client := ollama.NewClient(cfg.BaseURL)
	ag := agent.New(client, registry, cfg.Model)
	ag.ContextBudget = cfg.ContextBudget
	ag.MaxIterations = cfg.MaxIterations
	// Inject optional Markdown skills into the system prompt.
	var loadedSkills []skills.Skill
	if loaded, err := skills.Load(cfg.SkillsDir); err == nil {
		loadedSkills = loaded
		ag.System = skills.Inject(ag.System, loadedSkills)
	}

	ctx, cancel := context.WithCancel(context.Background())

	return &Model{
		viewport:     vp,
		textInput:    ti,
		spinner:      sp,
		status:       statusReady,
		cfg:          cfg,
		agent:        ag,
		registry:     registry,
		loadedSkills: loadedSkills,
		ollamaClient: client,
		renderer:     renderer,
		logger:       logger,
		ctx:          ctx,
		cancel:       cancel,
		stats:        sessionStats{startedAt: time.Now()},
	}
}

// SetLogger wires an external charmbracelet/log logger (e.g. file-backed) into
// the model so diagnostic output doesn't appear on the TUI surface.
func (m *Model) SetLogger(l *log.Logger) { m.logger = l }

// ── Init ─────────────────────────────────────────────────────────────────────

func (m *Model) Init() tea.Cmd {
	return tea.Batch(textinput.Blink, m.spinner.Tick)
}

// ── Update ────────────────────────────────────────────────────────────────────

func (m *Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmds []tea.Cmd

	switch msg := msg.(type) {

	// ── Window resize ────────────────────────────────────────────────────────
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		sidebarW := 28 // sidebar + border
		chatW := m.width - sidebarW - 2
		if chatW < 20 {
			chatW = 20
			sidebarW = 0
		}
		m.viewport.Width = chatW
		m.viewport.Height = m.height - 7 // header + input + status + borders
		m.textInput.Width = chatW - 4
		// Re-render with new width
		if m.renderer != nil {
			m.renderer, _ = glamour.NewTermRenderer(
				glamour.WithAutoStyle(),
				glamour.WithWordWrap(chatW-4),
			)
		}
		m.rebuildViewport()

	// ── Keyboard ─────────────────────────────────────────────────────────────
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

	// ── Spinner tick ──────────────────────────────────────────────────────────
	case spinner.TickMsg:
		var spCmd tea.Cmd
		m.spinner, spCmd = m.spinner.Update(msg)
		cmds = append(cmds, spCmd)

	// ── Agent events (streaming) ──────────────────────────────────────────────
	case agentEvent:
		if m.eventCh != nil {
			cmds = append(cmds, m.handleAgentEvent(agent.Event(msg)))
		}

	// ── Agent done ────────────────────────────────────────────────────────────
	case agentDone:
		m.eventCh = nil
		// Flush any accumulated agent text
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

	// ── Models list loaded ────────────────────────────────────────────────────
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
		// Run the picker as a sub-program (suspends main TUI)
		picker := newModelsPicker(msg.models, m.width-4, m.height-4)
		sub := tea.NewProgram(picker, tea.WithAltScreen())
		result, err := sub.Run()
		if err == nil {
			if pm, ok := result.(modelsPicker); ok && pm.chosen != "" {
				m.cfg.Model = pm.chosen
				m.agent.Model = pm.chosen
				m.entries = append(m.entries, ChatEntry{
					Kind:    entrySystem,
					Content: fmt.Sprintf("✓ Switched to **%s**", pm.chosen),
				})
			}
		}
		m.rebuildViewport()
	}

	// Always update sub-models
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

	// Regular message
	m.entries = append(m.entries, ChatEntry{Kind: entryUser, Content: input})
	m.history = append(m.history, ollama.Message{Role: "user", Content: input})
	m.stats.turns++
	m.status = statusThinking
	m.rebuildViewport()

	// Snapshot history to pass to goroutine (avoid data race)
	histSnap := make([]ollama.Message, len(m.history))
	copy(histSnap, m.history)

	return m.runAgent(histSnap)
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
		m.history = nil
		m.stats = sessionStats{startedAt: time.Now()}
		m.entries = append(m.entries, ChatEntry{Kind: entrySystem, Content: "Session cleared."})

	case "/model":
		if len(parts) > 1 {
			m.cfg.Model = parts[1]
			m.agent.Model = parts[1]
			m.entries = append(m.entries, ChatEntry{
				Kind:    entrySystem,
				Content: fmt.Sprintf("✓ Switched model to **%s**", parts[1]),
			})
		} else {
			m.entries = append(m.entries, ChatEntry{
				Kind:    entrySystem,
				Content: fmt.Sprintf("Current model: **%s**\nUsage: `/model <name>`", m.cfg.Model),
			})
		}

	case "/tools":
		var b strings.Builder
		b.WriteString("**Available Tools**\n\n")
		for _, name := range m.registry.Names() {
			if t, ok := m.registry.Get(name); ok {
				b.WriteString(fmt.Sprintf("- **%s**: %s\n", t.Name(), t.Description()))
			}
		}
		m.entries = append(m.entries, ChatEntry{Kind: entrySystem, Content: b.String()})

	case "/session":
		m.entries = append(m.entries, ChatEntry{
			Kind: entrySystem,
			Content: fmt.Sprintf(
				"**Session Info**\n\n- Model: `%s`\n- Root: `%s`\n- Turns: %d\n- Tool calls: %d\n- Uptime: %s",
				m.cfg.Model,
				m.cfg.Root,
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
		if len(m.loadedSkills) == 0 {
			m.entries = append(m.entries, ChatEntry{
				Kind:    entrySystem,
				Content: fmt.Sprintf("No skills loaded (dir: `%s`).", m.cfg.SkillsDir),
			})
		} else {
			var b strings.Builder
			b.WriteString(fmt.Sprintf("**Loaded Skills** (%d)\n\n", len(m.loadedSkills)))
			for _, s := range m.loadedSkills {
				if s.Description != "" {
					b.WriteString(fmt.Sprintf("- **%s**: %s\n", s.Name, s.Description))
				} else {
					b.WriteString(fmt.Sprintf("- **%s**\n", s.Name))
				}
			}
			m.entries = append(m.entries, ChatEntry{Kind: entrySystem, Content: b.String()})
		}

	default:
		m.entries = append(m.entries, ChatEntry{
			Kind:    entryError,
			Content: fmt.Sprintf("Unknown command: `%s`. Type `/help` for a list.", cmd),
		})
	}

	m.rebuildViewport()
	return nil
}

// runAgent launches agent.Run in a goroutine and pipes events back via BubbleTea.
// It stores the channel on the model for re-entrant reading.
func (m *Model) runAgent(history []ollama.Message) tea.Cmd {
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
		_, _ = m.agent.Run(m.ctx, history, emit)
	}()

	return waitForAgentEvent(ch)
}

// waitForAgentEvent returns a tea.Cmd that blocks on one read from ch.
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

// handleAgentEvent processes one agent event and returns the next read command.
func (m *Model) handleAgentEvent(ev agent.Event) tea.Cmd {
	switch ev.Kind {
	case agent.EventToken:
		m.tokenCount += tokens.Estimate(ev.Text)
		m.currentResp.WriteString(ev.Text)
		m.rebuildViewport()

	case agent.EventToolCall:
		// Flush partial agent text before showing tool call
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
		// Will be handled by agentDone message when channel closes
	}

	// Schedule next read from the same channel
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
		// Narrow terminal: single-column layout
		return m.singleColumnView()
	}

	header := m.renderHeader()
	chat := m.renderChat(chatW)
	sidebar := m.renderSidebar()
	input := m.renderInput(chatW)
	statusBar := m.renderStatusBar()

	// Join chat + sidebar side-by-side
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
	sub := SubtitleStyle.Render(fmt.Sprintf(" go-powered terminal intelligence • model: %s", m.cfg.Model))
	return lipgloss.JoinHorizontal(lipgloss.Center, title, sub) + "\n"
}

func (m *Model) renderChat(width int) string {
	m.viewport.Width = width
	bordered := ViewportStyle.Width(width).Render(m.viewport.View())
	return bordered
}

func (m *Model) renderSidebar() string {
	uptime := time.Since(m.stats.startedAt).Round(time.Second)

	rows := []string{
		SidebarTitleStyle.Render("◈ Session"),
		row("Model", m.cfg.Model),
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
		shortcut("/clear", "new session"),
	}

	content := strings.Join(rows, "\n")
	return SidebarStyle.Render(content)
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
	inner := prompt + m.textInput.View()
	return style.Width(width).Render(inner)
}

func (m *Model) renderStatusBar() string {
	left := HelpStyle.Render(" ctrl+c quit • esc cancel • /help commands")
	right := HelpStyle.Render(fmt.Sprintf("root: %s ", m.cfg.Root))
	gap := m.width - lipgloss.Width(left) - lipgloss.Width(right)
	if gap < 0 {
		gap = 0
	}
	return StatusBarStyle.Width(m.width).Render(
		left + strings.Repeat(" ", gap) + right,
	)
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
	// If currently streaming, show partial response
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
		label := UserLabelStyle.Render("YOU ›")
		return label + " " + e.Content

	case entryAgent:
		label := AgentLabelStyle.Render("AGENT ›")
		// Content is already glamour-rendered
		return label + "\n" + e.Content

	case entryToolCall:
		label := ToolCallLabelStyle.Render(fmt.Sprintf("⚙ TOOL › %s", e.Tool))
		content := m.renderMarkdown(e.Content)
		return ToolCardStyle.Render(label + "\n" + content)

	case entryToolResult:
		label := ToolResultLabelStyle.Render(fmt.Sprintf("  ↳ result from %s", e.Tool))
		snippet := InfoStyle.Render(e.Content)
		return label + "\n" + snippet

	case entryError:
		label := ErrorLabelStyle.Render("✖ ERROR")
		return label + " " + e.Content

	case entrySystem:
		rendered := m.renderMarkdown(e.Content)
		return InfoStyle.Render("──────────────────────────────") + "\n" +
			rendered +
			InfoStyle.Render("──────────────────────────────")
	}
	return e.Content
}

// renderMarkdown runs content through Glamour, falling back to plain text.
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
