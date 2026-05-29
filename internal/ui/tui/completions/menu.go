package completions

import (
	"strconv"
	"strings"

	"github.com/charmbracelet/lipgloss"
)

// Visual constants. Colours mirror the parent TUI palette (styles.go).
const (
	defaultMaxRows = 8
	defaultWidth   = 50
)

var (
	boxStyle = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(lipgloss.Color("#7C3AED")) // purple

	selStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#F9FAFB")).
			Background(lipgloss.Color("#7C3AED")).
			Bold(true)

	rowStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#F9FAFB"))

	descStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#6B7280")).
			Italic(true)
)

// Model is the dropdown menu state. It is driven entirely by Refresh from the
// parent's Update loop; it is not a standalone tea.Model.
type Model struct {
	providers []Provider

	active  Provider
	items   []Item
	start   int // rune offset where the accepted token begins
	idx     int // selected row
	offset  int // scroll offset into items
	visible bool

	maxRows int
	width   int
}

// New builds a menu over the given providers (checked in order; first active
// provider wins).
func New(providers ...Provider) *Model {
	return &Model{providers: providers, maxRows: defaultMaxRows, width: defaultWidth}
}

// SetWidth sets the menu box content width.
func (m *Model) SetWidth(w int) {
	if w > 0 {
		m.width = w
	}
}

// Visible reports whether the menu should be rendered.
func (m *Model) Visible() bool { return m.visible && len(m.items) > 0 }

// Refresh recomputes the active provider and its items for the current line and
// cursor (a rune index). Call after every keystroke.
func (m *Model) Refresh(line string, cursor int) {
	for _, p := range m.providers {
		active, query, start := p.Trigger(line, cursor)
		if !active {
			continue
		}
		items := p.Items(query)
		if len(items) == 0 {
			m.hideKeepNothing()
			return
		}
		m.active = p
		m.items = items
		m.start = start
		m.visible = true
		if m.idx >= len(items) {
			m.idx = len(items) - 1
		}
		m.clampScroll()
		return
	}
	m.hideKeepNothing()
}

func (m *Model) hideKeepNothing() {
	m.visible = false
	m.items = nil
	m.active = nil
	m.idx = 0
	m.offset = 0
}

// Hide dismisses the menu without changing the input (e.g. on Esc).
func (m *Model) Hide() { m.hideKeepNothing() }

// Move shifts the selection by delta, clamped to the item range.
func (m *Model) Move(delta int) {
	if !m.Visible() {
		return
	}
	m.idx += delta
	if m.idx < 0 {
		m.idx = 0
	}
	if m.idx >= len(m.items) {
		m.idx = len(m.items) - 1
	}
	m.clampScroll()
}

func (m *Model) clampScroll() {
	if m.idx < m.offset {
		m.offset = m.idx
	}
	if m.idx >= m.offset+m.maxRows {
		m.offset = m.idx - m.maxRows + 1
	}
	if m.offset < 0 {
		m.offset = 0
	}
}

// Accept splices the selected item's Insert text into line, replacing from the
// trigger start up to cursor. It returns the new line, the new rune cursor
// position, and whether anything was accepted. The menu is hidden on success.
func (m *Model) Accept(line string, cursor int) (string, int, bool) {
	if !m.Visible() {
		return line, cursor, false
	}
	r := []rune(line)
	if cursor > len(r) {
		cursor = len(r)
	}
	if m.start > cursor {
		m.hideKeepNothing()
		return line, cursor, false
	}
	insert := []rune(m.items[m.idx].Insert)
	newLine := string(r[:m.start]) + string(insert) + string(r[cursor:])
	newCursor := m.start + len(insert)
	m.hideKeepNothing()
	return newLine, newCursor, true
}

// View renders the dropdown box, or "" when not visible.
func (m *Model) View() string {
	if !m.Visible() {
		return ""
	}
	end := m.offset + m.maxRows
	if end > len(m.items) {
		end = len(m.items)
	}
	var b strings.Builder
	for i := m.offset; i < end; i++ {
		it := m.items[i]
		line := it.Title
		if it.Desc != "" {
			line += "  " + descStyle.Render(it.Desc)
		}
		line = truncate(line, m.width)
		if i == m.idx {
			b.WriteString(selStyle.Width(m.width).Render(it.Title))
			if it.Desc != "" {
				b.WriteString(selStyle.Render("  " + it.Desc))
			}
		} else {
			b.WriteString(rowStyle.Render(line))
		}
		if i < end-1 {
			b.WriteByte('\n')
		}
	}
	if len(m.items) > m.maxRows {
		b.WriteString("\n")
		b.WriteString(descStyle.Render(footer(m.idx+1, len(m.items))))
	}
	return boxStyle.Width(m.width).Render(b.String())
}

func truncate(s string, w int) string {
	r := []rune(s)
	if w <= 1 || len(r) <= w {
		return s
	}
	return string(r[:w-1]) + "…"
}

func footer(pos, total int) string {
	return strconv.Itoa(pos) + "/" + strconv.Itoa(total)
}
