package tui

import "github.com/charmbracelet/lipgloss"

// Brand colours
const (
	colorPurple  = lipgloss.Color("#7C3AED")
	colorPurple2 = lipgloss.Color("#A78BFA")
	colorGreen   = lipgloss.Color("#10B981")
	colorCyan    = lipgloss.Color("#06B6D4")
	colorPink    = lipgloss.Color("#EC4899")
	colorAmber   = lipgloss.Color("#F59E0B")
	colorRed     = lipgloss.Color("#EF4444")
	colorGray    = lipgloss.Color("#6B7280")
	colorDark    = lipgloss.Color("#111827")
	colorDarker  = lipgloss.Color("#0D1117")
	colorBorder  = lipgloss.Color("#374151")
	colorSubtle  = lipgloss.Color("#4B5563")
	colorWhite   = lipgloss.Color("#F9FAFB")
)

var (
	// ── Header ──────────────────────────────────────────────────────────────
	TitleStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(colorWhite).
			Background(colorPurple).
			Padding(0, 2)

	SubtitleStyle = lipgloss.NewStyle().
			Foreground(colorPurple2).
			Italic(true)

	// ── Status bar ──────────────────────────────────────────────────────────
	StatusBarStyle = lipgloss.NewStyle().
			Background(colorDark).
			Foreground(colorGray).
			Padding(0, 1)

	StatusReadyStyle = lipgloss.NewStyle().
				Foreground(colorGreen).
				Bold(true)

	StatusThinkingStyle = lipgloss.NewStyle().
				Foreground(colorAmber).
				Bold(true)

	StatusToolStyle = lipgloss.NewStyle().
			Foreground(colorCyan).
			Bold(true)

	StatusErrorStyle = lipgloss.NewStyle().
				Foreground(colorRed).
				Bold(true)

	// ── Sidebar ──────────────────────────────────────────────────────────────
	SidebarStyle = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(colorBorder).
			Padding(1, 2).
			Width(26)

	SidebarTitleStyle = lipgloss.NewStyle().
				Foreground(colorPurple2).
				Bold(true).
				MarginBottom(1)

	SidebarLabelStyle = lipgloss.NewStyle().
				Foreground(colorGray)

	SidebarValueStyle = lipgloss.NewStyle().
				Foreground(colorWhite).
				Bold(true)

	SidebarKeyStyle = lipgloss.NewStyle().
			Foreground(colorCyan).
			Bold(true)

	// ── Chat viewport ────────────────────────────────────────────────────────
	ViewportStyle = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(colorBorder)

	// ── Message labels ────────────────────────────────────────────────────────
	UserLabelStyle = lipgloss.NewStyle().
			Foreground(colorPink).
			Bold(true)

	AgentLabelStyle = lipgloss.NewStyle().
			Foreground(colorGreen).
			Bold(true)

	ToolCallLabelStyle = lipgloss.NewStyle().
				Foreground(colorCyan).
				Bold(true)

	ToolResultLabelStyle = lipgloss.NewStyle().
				Foreground(colorAmber).
				Italic(true)

	ErrorLabelStyle = lipgloss.NewStyle().
			Foreground(colorRed).
			Bold(true)

	// ── Input box ─────────────────────────────────────────────────────────────
	InputBorderStyle = lipgloss.NewStyle().
				Border(lipgloss.RoundedBorder()).
				BorderForeground(colorPurple).
				Padding(0, 1)

	InputFocusBorderStyle = lipgloss.NewStyle().
				Border(lipgloss.RoundedBorder()).
				BorderForeground(colorPink).
				Padding(0, 1)

	// ── Help bar ──────────────────────────────────────────────────────────────
	HelpStyle = lipgloss.NewStyle().
			Foreground(colorSubtle).
			Italic(true)

	HelpKeyStyle = lipgloss.NewStyle().
			Foreground(colorGray).
			Bold(true)

	// ── Tool card ─────────────────────────────────────────────────────────────
	ToolCardStyle = lipgloss.NewStyle().
			Border(lipgloss.NormalBorder()).
			BorderForeground(colorCyan).
			Padding(0, 1).
			MarginTop(1)

	// ── Info styles ────────────────────────────────────────────────────────────
	InfoStyle = lipgloss.NewStyle().
			Foreground(colorGray).
			Italic(true)
)
