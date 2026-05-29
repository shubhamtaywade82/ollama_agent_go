package tui

import (
	"strings"
	"testing"
)

func TestComputeLayoutExactVerticalFit(t *testing.T) {
	// When nothing is clamped, chrome + chat box must sum to exactly height so
	// the input and status bar are never pushed below the fold.
	const chromeH, sidebarH = 6, 30
	for _, height := range []int{24, 30, 40, 60} {
		l := computeLayout(120, height, chromeH, sidebarH)
		total := chromeH + l.viewportH + chatBorder // chat box = viewport + border
		if total != height {
			t.Errorf("height=%d: chrome+chatbox=%d, want %d (viewportH=%d)",
				height, total, height, l.viewportH)
		}
	}
}

func TestComputeLayoutHidesSidebarWhenNarrow(t *testing.T) {
	l := computeLayout(50, 40, 6, 30) // width < minWidthForSidebar
	if !l.single {
		t.Errorf("narrow width should hide sidebar, got %+v", l)
	}
	if l.chatOuterW != 50 {
		t.Errorf("single column chat should span full width, got %d", l.chatOuterW)
	}
}

func TestComputeLayoutHidesSidebarWhenShort(t *testing.T) {
	// Wide enough, but the body is shorter than the sidebar ⇒ hide it.
	l := computeLayout(120, 24, 6, 30) // bodyH = 18 < 30
	if !l.single {
		t.Errorf("short body should hide sidebar, got %+v", l)
	}
}

func TestComputeLayoutShowsSidebarWhenRoomy(t *testing.T) {
	l := computeLayout(120, 40, 6, 30) // bodyH = 34 >= 30, wide enough
	if l.single {
		t.Fatalf("roomy window should show sidebar, got %+v", l)
	}
	if l.sidebarW != sidebarOuterW {
		t.Errorf("sidebarW = %d, want %d", l.sidebarW, sidebarOuterW)
	}
	if l.chatOuterW != 120-sidebarOuterW-layoutGap {
		t.Errorf("chatOuterW = %d, want %d", l.chatOuterW, 120-sidebarOuterW-layoutGap)
	}
}

func TestComputeLayoutClampsTiny(t *testing.T) {
	l := computeLayout(2, 2, 6, 30) // degenerate
	if l.viewportW < 1 || l.viewportH < 1 {
		t.Errorf("dimensions must clamp to >=1, got %+v", l)
	}
}

func TestCapLines(t *testing.T) {
	s := "a\nb\nc\nd"
	if got := capLines(s, 2); got != "a\nb" {
		t.Errorf("capLines(_,2) = %q, want a\\nb", got)
	}
	if got := capLines(s, 10); got != s {
		t.Errorf("capLines should be no-op when under limit, got %q", got)
	}
	if n := strings.Count(capLines(s, 1), "\n"); n != 0 {
		t.Errorf("capLines(_,1) should yield single line, got %d newlines", n)
	}
}
