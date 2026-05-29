// Package completions implements a trigger-driven autocomplete dropdown for the
// TUI prompt. Providers inspect the input line and cursor, decide whether they
// own the current token, and return ranked candidate items. The Model renders
// the active provider's items in a floating box above the input and splices the
// chosen item back into the text.
//
// Crush (charmbracelet/crush) uses the same Bubble Tea + Lip Gloss pattern; this
// is an independent implementation — no Crush source is copied.
package completions

import "github.com/sahilm/fuzzy"

// Item is one candidate row in the dropdown.
type Item struct {
	Title  string // primary text shown in the menu
	Desc   string // dim hint shown to the right
	Insert string // text spliced into the input when accepted
}

// Provider supplies completions for a particular trigger token.
type Provider interface {
	// Trigger reports whether this provider owns the token under the cursor.
	// line is the full input value; cursor is a rune index into it. When active,
	// query is the substring to match against and start is the rune offset where
	// the replaced token begins (used to splice the accepted Insert text).
	Trigger(line string, cursor int) (active bool, query string, start int)

	// Items returns candidates for query, already ranked best-first.
	Items(query string) []Item
}

// runesBefore returns the rune slice of line up to (but not including) cursor,
// clamped to valid bounds.
func runesBefore(line string, cursor int) []rune {
	r := []rune(line)
	if cursor < 0 {
		cursor = 0
	}
	if cursor > len(r) {
		cursor = len(r)
	}
	return r[:cursor]
}

// isSpace reports whether r is an in-line whitespace rune that ends a token.
func isSpace(r rune) bool { return r == ' ' || r == '\t' }

// tokenAt finds the most recent token introduced by marker (e.g. '@' or '#')
// ending at the cursor. The marker must sit at the start of the pre-cursor text
// or be preceded by whitespace, and the token must not contain whitespace.
func tokenAt(pre []rune, marker rune) (active bool, query string, start int) {
	idx := -1
	for i := len(pre) - 1; i >= 0; i-- {
		if pre[i] == marker {
			idx = i
			break
		}
		if isSpace(pre[i]) {
			return false, "", 0
		}
	}
	if idx < 0 {
		return false, "", 0
	}
	if idx > 0 && !isSpace(pre[idx-1]) {
		return false, "", 0
	}
	return true, string(pre[idx+1:]), idx
}

// rankItems fuzzy-ranks items by their Title against query (best-first). An
// empty query preserves the input order.
func rankItems(items []Item, query string) []Item {
	if query == "" {
		return items
	}
	titles := make([]string, len(items))
	for i, it := range items {
		titles[i] = it.Title
	}
	matches := fuzzy.Find(query, titles)
	out := make([]Item, 0, len(matches))
	for _, m := range matches {
		out = append(out, items[m.Index])
	}
	return out
}
