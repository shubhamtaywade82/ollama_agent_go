package tui

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"ollama_agent_go/internal/agent"
	"ollama_agent_go/internal/runtime"
)

// newTestModel builds a Model with the minimal engine state View touches.
func newTestModel() *Model {
	eng := &runtime.Engine{Agent: &agent.Agent{Model: "test-model"}}
	return NewModel(eng, nil, ".")
}

// TestViewFitsWindowHeight is the regression guard for the "resize does nothing /
// input pushed off-screen" bug: the rendered frame must be exactly as tall as
// the window at every size, so the input box and status bar are always visible.
func TestViewFitsWindowHeight(t *testing.T) {
	m := newTestModel()
	for _, size := range []struct{ w, h int }{
		{80, 24}, {120, 40}, {100, 30}, {200, 60}, {70, 20},
	} {
		updated, _ := m.Update(tea.WindowSizeMsg{Width: size.w, Height: size.h})
		m = updated.(*Model)
		got := lipgloss.Height(m.View())
		if got != size.h {
			t.Errorf("window %dx%d: View height = %d, want %d", size.w, size.h, got, size.h)
		}
		if w := lipgloss.Width(m.View()); w > size.w {
			t.Errorf("window %dx%d: View width = %d, exceeds %d", size.w, size.h, w, size.w)
		}
	}
}

// TestAgentMessageRewrapsOnResize guards the deferred bug: a stored agent
// message must re-wrap to the new width on resize, not stay at its original
// wrap width.
func TestAgentMessageRewrapsOnResize(t *testing.T) {
	m := newTestModel()
	long := strings.Repeat("the quick brown fox jumps over the lazy dog ", 12)
	m.entries = append(m.entries, ChatEntry{Kind: entryAgent, Content: long})

	u, _ := m.Update(tea.WindowSizeMsg{Width: 50, Height: 40})
	m = u.(*Model)
	narrow := lipgloss.Width(m.renderEntry(m.entries[0]))

	u, _ = m.Update(tea.WindowSizeMsg{Width: 160, Height: 40})
	m = u.(*Model)
	wide := lipgloss.Width(m.renderEntry(m.entries[0]))

	if narrow >= wide {
		t.Errorf("agent message should wrap narrower on a small window: narrow=%d wide=%d", narrow, wide)
	}
}

// TestViewTracksResize confirms the chat viewport actually changes size when the
// window does — proving the layout responds to resize rather than staying fixed.
func TestViewTracksResize(t *testing.T) {
	m := newTestModel()

	u, _ := m.Update(tea.WindowSizeMsg{Width: 100, Height: 24})
	m = u.(*Model)
	_ = m.View()
	short := m.viewport.Height

	u, _ = m.Update(tea.WindowSizeMsg{Width: 100, Height: 50})
	m = u.(*Model)
	_ = m.View()
	tall := m.viewport.Height

	if tall <= short {
		t.Errorf("viewport height should grow with the window: short=%d tall=%d", short, tall)
	}
}
