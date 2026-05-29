package completions

import (
	"io/fs"
	"path/filepath"
	"strings"
)

// ── Slash commands ─────────────────────────────────────────────────────────

// Command is a slash command offered for completion. Name includes the leading
// slash (e.g. "/model").
type Command struct {
	Name string
	Desc string
}

// SlashProvider completes slash command names typed at the start of the line.
type SlashProvider struct{ Commands []Command }

func (p SlashProvider) Trigger(line string, cursor int) (bool, string, int) {
	pre := runesBefore(line, cursor)
	if len(pre) == 0 || pre[0] != '/' {
		return false, "", 0
	}
	for _, r := range pre {
		if isSpace(r) { // past the command name — not ours anymore
			return false, "", 0
		}
	}
	return true, string(pre[1:]), 0
}

func (p SlashProvider) Items(query string) []Item {
	items := make([]Item, len(p.Commands))
	for i, c := range p.Commands {
		// Match on the name without the leading slash so "mod" hits "/model".
		items[i] = Item{Title: strings.TrimPrefix(c.Name, "/"), Desc: c.Desc, Insert: c.Name}
	}
	ranked := rankItems(items, query)
	for i := range ranked {
		ranked[i].Title = "/" + ranked[i].Title // show with slash in the menu
	}
	return ranked
}

// ── Files (@) ──────────────────────────────────────────────────────────────

// FileProvider completes file paths under Root for an "@" token. Walking is
// capped to keep large trees responsive.
type FileProvider struct {
	Root     string
	MaxFiles int // 0 ⇒ default cap
}

const defaultMaxFiles = 2000

func (p FileProvider) Trigger(line string, cursor int) (bool, string, int) {
	return tokenAt(runesBefore(line, cursor), '@')
}

func (p FileProvider) Items(query string) []Item {
	if p.Root == "" {
		return nil
	}
	limit := p.MaxFiles
	if limit <= 0 {
		limit = defaultMaxFiles
	}
	var paths []string
	_ = filepath.WalkDir(p.Root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return nil
		}
		name := d.Name()
		if d.IsDir() {
			if path != p.Root && (name == ".git" || name == "node_modules" || strings.HasPrefix(name, ".")) {
				return filepath.SkipDir
			}
			return nil
		}
		rel, err := filepath.Rel(p.Root, path)
		if err != nil {
			return nil
		}
		paths = append(paths, filepath.ToSlash(rel))
		if len(paths) >= limit {
			return filepath.SkipAll
		}
		return nil
	})
	items := make([]Item, len(paths))
	for i, rel := range paths {
		items[i] = Item{Title: rel, Insert: "@" + rel}
	}
	return rankItems(items, query)
}

// ── Models ───────────────────────────────────────────────────────────────

const modelPrefix = "/model "

// ModelProvider completes model names after "/model ". Names is a supplier so
// the (asynchronously fetched, cached) model list stays the source of truth.
type ModelProvider struct{ Names func() []string }

func (p ModelProvider) Trigger(line string, cursor int) (bool, string, int) {
	pre := string(runesBefore(line, cursor))
	if !strings.HasPrefix(pre, modelPrefix) {
		return false, "", 0
	}
	q := pre[len(modelPrefix):]
	if strings.ContainsAny(q, " \t") {
		return false, "", 0
	}
	return true, q, len([]rune(modelPrefix))
}

func (p ModelProvider) Items(query string) []Item {
	if p.Names == nil {
		return nil
	}
	names := p.Names()
	items := make([]Item, len(names))
	for i, n := range names {
		items[i] = Item{Title: n, Insert: modelPrefix + n}
	}
	return rankItems(items, query)
}

// ── Skills (#) ─────────────────────────────────────────────────────────────

// NamedItem is a generic name+description pair for supplier-backed providers.
type NamedItem struct{ Name, Desc string }

// SkillProvider completes loaded skill names for a "#" token.
type SkillProvider struct{ Skills func() []NamedItem }

func (p SkillProvider) Trigger(line string, cursor int) (bool, string, int) {
	return tokenAt(runesBefore(line, cursor), '#')
}

func (p SkillProvider) Items(query string) []Item {
	if p.Skills == nil {
		return nil
	}
	skills := p.Skills()
	items := make([]Item, len(skills))
	for i, s := range skills {
		items[i] = Item{Title: s.Name, Desc: s.Desc, Insert: "#" + s.Name}
	}
	return rankItems(items, query)
}
