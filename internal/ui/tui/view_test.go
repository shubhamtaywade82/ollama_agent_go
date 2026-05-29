package tui

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"ollama_agent_go/internal/agent"
	"ollama_agent_go/internal/observability"
	"ollama_agent_go/internal/policy"
	"ollama_agent_go/internal/runtime"
	"ollama_agent_go/internal/tools"
	"ollama_agent_go/internal/types"
)

// newTestModel builds a Model with the minimal engine state View and SetModel
// touch (Agent + a real, empty ToolHost so SystemPrompt can enumerate tools).
func newTestModel() *Model {
	host := tools.NewHost(
		tools.NewRegistry("."),
		policy.NewDefaultEngine(".", nil, true),
		observability.Discard,
	)
	eng := &runtime.Engine{Agent: &agent.Agent{Model: "test-model"}, ToolHost: host}
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

// TestModelsPickerRunsInProgram guards the "TUI breaks after switching model"
// bug: the picker must run as an in-program mode (no nested tea.Program) and
// return to chat on select, applying the chosen model.
func TestModelsPickerRunsInProgram(t *testing.T) {
	m := newTestModel()
	u, _ := m.Update(tea.WindowSizeMsg{Width: 120, Height: 40})
	m = u.(*Model)

	models := []types.ModelInfo{{Name: "model-a"}, {Name: "model-b"}}
	u, _ = m.Update(modelsLoadedMsg{models: models})
	m = u.(*Model)
	if m.mode != modePicker {
		t.Fatalf("loading models should enter picker mode, got %v", m.mode)
	}
	if lipgloss.Height(m.View()) == 0 {
		t.Error("picker view should render")
	}

	u, _ = m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	m = u.(*Model)
	if m.mode != modeChat {
		t.Errorf("enter should return to chat mode, got %v", m.mode)
	}
	if got := m.engine.Model(); got != "model-a" {
		t.Errorf("selected model not applied: got %q", got)
	}
}

func TestModelsPickerCancel(t *testing.T) {
	m := newTestModel()
	u, _ := m.Update(tea.WindowSizeMsg{Width: 120, Height: 40})
	m = u.(*Model)
	u, _ = m.Update(modelsLoadedMsg{models: []types.ModelInfo{{Name: "x"}}})
	m = u.(*Model)

	u, _ = m.Update(tea.KeyMsg{Type: tea.KeyEsc})
	m = u.(*Model)
	if m.mode != modeChat {
		t.Errorf("esc should cancel the picker, got mode %v", m.mode)
	}
}

// TestInputHistoryRecall covers ↑/↓ shell-style history browsing.
func TestInputHistoryRecall(t *testing.T) {
	m := newTestModel()
	m.recordHistory("first")
	m.recordHistory("second")
	m.recordHistory("second") // consecutive duplicate ignored

	if len(m.history) != 2 {
		t.Fatalf("dup should be skipped, history=%v", m.history)
	}

	m.historyPrev()
	if m.textInput.Value() != "second" {
		t.Errorf("first ↑ should recall newest, got %q", m.textInput.Value())
	}
	m.historyPrev()
	if m.textInput.Value() != "first" {
		t.Errorf("second ↑ should recall oldest, got %q", m.textInput.Value())
	}
	m.historyPrev() // clamp at oldest
	if m.textInput.Value() != "first" {
		t.Errorf("↑ at oldest should stay, got %q", m.textInput.Value())
	}

	m.historyNext()
	if m.textInput.Value() != "second" {
		t.Errorf("↓ should move to newer, got %q", m.textInput.Value())
	}
	m.historyNext() // past newest → restore stashed live input ("")
	if m.textInput.Value() != "" {
		t.Errorf("↓ past newest should restore live input, got %q", m.textInput.Value())
	}
}

// TestInputHistoryStashesLiveInput verifies an in-progress line is preserved
// when browsing up then back down.
func TestInputHistoryStashesLiveInput(t *testing.T) {
	m := newTestModel()
	m.recordHistory("old")
	m.setInput("draft in progress")
	m.histPos = len(m.history) // simulate live (not browsing)

	m.historyPrev()
	if m.textInput.Value() != "old" {
		t.Errorf("↑ should recall history, got %q", m.textInput.Value())
	}
	m.historyNext()
	if m.textInput.Value() != "draft in progress" {
		t.Errorf("↓ should restore the stashed draft, got %q", m.textInput.Value())
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
