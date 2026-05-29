package tui

import (
	"context"
	"fmt"
	"io"
	"strings"

	"github.com/charmbracelet/bubbles/list"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	ollamaprovider "ollama_agent_go/internal/providers/ollama"
	"ollama_agent_go/internal/types"
)

type modelItem struct {
	info types.ModelInfo
}

func (m modelItem) Title() string { return m.info.Name }
func (m modelItem) Description() string {
	var parts []string
	if m.info.Details.ParameterSize != "" {
		parts = append(parts, m.info.Details.ParameterSize)
	}
	if m.info.Details.QuantizationLevel != "" {
		parts = append(parts, m.info.Details.QuantizationLevel)
	}
	if m.info.Details.Family != "" {
		parts = append(parts, m.info.Details.Family)
	}
	if m.info.Size > 0 {
		parts = append(parts, humanSize(m.info.Size))
	}
	if len(parts) == 0 {
		return "local model"
	}
	return strings.Join(parts, " · ")
}
func (m modelItem) FilterValue() string { return m.info.Name }

type modelDelegate struct{}

func (d modelDelegate) Height() int                             { return 2 }
func (d modelDelegate) Spacing() int                            { return 1 }
func (d modelDelegate) Update(_ tea.Msg, _ *list.Model) tea.Cmd { return nil }
func (d modelDelegate) Render(w io.Writer, m list.Model, index int, listItem list.Item) {
	item, ok := listItem.(modelItem)
	if !ok {
		return
	}
	selected := index == m.Index()

	nameStyle := lipgloss.NewStyle().Bold(true).Foreground(colorWhite)
	descStyle := lipgloss.NewStyle().Foreground(colorGray)
	cursorStyle := lipgloss.NewStyle().Foreground(colorPurple).Bold(true)
	selectedNameStyle := lipgloss.NewStyle().Bold(true).Foreground(colorPurple2)

	cursor := "  "
	name := nameStyle.Render(item.Title())
	desc := descStyle.Render(item.Description())

	if selected {
		cursor = cursorStyle.Render("▸ ")
		name = selectedNameStyle.Render(item.Title())
	}

	fmt.Fprintf(w, "%s%s\n   %s", cursor, name, desc)
}

type modelsPicker struct {
	list   list.Model
	chosen string
	done   bool
}

func newModelsPicker(models []types.ModelInfo, width, height int) modelsPicker {
	items := make([]list.Item, len(models))
	for i, m := range models {
		items[i] = modelItem{info: m}
	}

	l := list.New(items, modelDelegate{}, width, height)
	l.Title = "Select a Model"
	l.Styles.Title = lipgloss.NewStyle().
		Bold(true).
		Foreground(colorWhite).
		Background(colorPurple).
		Padding(0, 1)
	l.Styles.FilterPrompt = lipgloss.NewStyle().Foreground(colorPink)
	l.Styles.FilterCursor = lipgloss.NewStyle().Foreground(colorPink)
	l.SetFilteringEnabled(true)
	l.SetShowStatusBar(true)
	l.SetShowHelp(true)

	return modelsPicker{list: l}
}

func (m modelsPicker) Init() tea.Cmd { return nil }

func (m modelsPicker) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "enter":
			if item, ok := m.list.SelectedItem().(modelItem); ok {
				m.chosen = item.info.Name
				m.done = true
				return m, tea.Quit
			}
		case "q", "esc", "ctrl+c":
			m.done = true
			return m, tea.Quit
		}
	case tea.WindowSizeMsg:
		m.list.SetWidth(msg.Width)
		m.list.SetHeight(msg.Height - 2)
	}

	var cmd tea.Cmd
	m.list, cmd = m.list.Update(msg)
	return m, cmd
}

func (m modelsPicker) View() string {
	border := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(colorBorder).
		Padding(1, 2)

	inner := m.list.View() + "\n\n" +
		HelpStyle.Render("enter select • / filter • esc cancel")
	return border.Render(inner)
}

type modelsLoadedMsg struct {
	models []types.ModelInfo
	err    error
}

func fetchModels(client *ollamaprovider.Client) tea.Cmd {
	return func() tea.Msg {
		models, err := client.ListModels(context.Background())
		return modelsLoadedMsg{models: models, err: err}
	}
}

func humanSize(b int64) string {
	const unit = 1024
	if b < unit {
		return fmt.Sprintf("%d B", b)
	}
	div, exp := int64(unit), 0
	for n := b / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f %cB", float64(b)/float64(div), "KMGTPE"[exp])
}
