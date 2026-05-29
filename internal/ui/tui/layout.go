package tui

import "strings"

// Layout geometry constants. The two-column view places a fixed-width sidebar to
// the right of the chat box; below the sidebar's width/height thresholds the
// sidebar is hidden and the chat goes full width (single column).
const (
	sidebarOuterW = 28 // SidebarStyle content (26) + rounded border (2)
	layoutGap     = 2  // spaces between chat box and sidebar
	chatBorder    = 2  // ViewportStyle rounded border (left+right / top+bottom)
	minChatW      = 30 // smallest usable chat content width

	// inputBoxHeight is the rendered height of the bordered single-line input
	// (1 content line + top/bottom border). Kept constant so View can budget
	// vertical space without rendering the input twice.
	inputBoxHeight = 3

	// minWidthForSidebar is the narrowest total width that still fits chat +
	// gap + sidebar. Below it the sidebar is hidden.
	minWidthForSidebar = minChatW + layoutGap + sidebarOuterW // 60
)

// layout holds the resolved dimensions for one render.
type layout struct {
	single     bool // true ⇒ no sidebar, chat spans full width
	chatOuterW int  // chat box width including its border
	viewportW  int  // chat viewport content width (chatOuterW - border)
	viewportH  int  // chat viewport content height
	sidebarW   int  // 0 when single
}

// computeLayout resolves the frame geometry so the rendered output is exactly
// height lines tall. chromeH is the combined height of everything except the
// chat body (header + input + status + dropdown). sidebarH is the rendered
// sidebar height; the sidebar is shown only when the window is wide enough and
// the body is at least as tall as the sidebar (otherwise it is hidden).
func computeLayout(width, height, chromeH, sidebarH int) layout {
	bodyH := height - chromeH
	if bodyH < 3 {
		bodyH = 3
	}

	showSidebar := width >= minWidthForSidebar && sidebarH > 0 && bodyH >= sidebarH

	var l layout
	if showSidebar {
		l.sidebarW = sidebarOuterW
		l.chatOuterW = width - sidebarOuterW - layoutGap
	} else {
		l.single = true
		l.chatOuterW = width
	}

	l.viewportW = l.chatOuterW - chatBorder
	if l.viewportW < 1 {
		l.viewportW = 1
	}
	l.viewportH = bodyH - chatBorder
	if l.viewportH < 1 {
		l.viewportH = 1
	}
	return l
}

// capLines truncates s to at most n lines, guarding against a block taller than
// the space allotted to it.
func capLines(s string, n int) string {
	if n < 1 {
		n = 1
	}
	lines := strings.Split(s, "\n")
	if len(lines) <= n {
		return s
	}
	return strings.Join(lines[:n], "\n")
}
