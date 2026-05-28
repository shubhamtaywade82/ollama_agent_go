// Package skills loads Markdown "skill" files that inject additional system
// instructions into the agent. A skill is a Markdown document with optional
// YAML-ish frontmatter (name, description); its body is appended to the system
// prompt so the model gains task-specific guidance.
package skills

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// Skill is a single loaded instruction document.
type Skill struct {
	Name        string
	Description string
	Body        string
	Path        string
}

// Load reads all *.md files in dir (non-recursive) as skills, sorted by name.
// A missing directory yields no skills and no error, so skills are optional.
func Load(dir string) ([]Skill, error) {
	if dir == "" {
		return nil, nil
	}
	entries, err := os.ReadDir(dir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("read skills dir: %w", err)
	}

	var skills []Skill
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(strings.ToLower(e.Name()), ".md") {
			continue
		}
		path := filepath.Join(dir, e.Name())
		b, err := os.ReadFile(path)
		if err != nil {
			return nil, fmt.Errorf("read skill %s: %w", e.Name(), err)
		}
		s := parse(string(b))
		s.Path = path
		if s.Name == "" {
			s.Name = strings.TrimSuffix(e.Name(), filepath.Ext(e.Name()))
		}
		skills = append(skills, s)
	}
	sort.Slice(skills, func(i, j int) bool { return skills[i].Name < skills[j].Name })
	return skills, nil
}

// parse splits optional frontmatter from the body.
func parse(content string) Skill {
	content = strings.TrimLeft(content, "\uFEFF \t\r\n")
	if !strings.HasPrefix(content, "---") {
		return Skill{Body: strings.TrimSpace(content)}
	}
	// Find the closing delimiter.
	rest := content[3:]
	idx := strings.Index(rest, "\n---")
	if idx < 0 {
		return Skill{Body: strings.TrimSpace(content)}
	}
	front := rest[:idx]
	body := rest[idx+4:] // skip "\n---"
	if nl := strings.IndexByte(body, '\n'); nl >= 0 {
		body = body[nl+1:]
	}

	s := Skill{Body: strings.TrimSpace(body)}
	for _, line := range strings.Split(front, "\n") {
		line = strings.TrimSpace(line)
		key, val, ok := strings.Cut(line, ":")
		if !ok {
			continue
		}
		val = strings.TrimSpace(val)
		switch strings.ToLower(strings.TrimSpace(key)) {
		case "name":
			s.Name = val
		case "description":
			s.Description = val
		}
	}
	return s
}

// Inject appends the skills' bodies to a base system prompt.
func Inject(base string, skills []Skill) string {
	if len(skills) == 0 {
		return base
	}
	var b strings.Builder
	b.WriteString(base)
	b.WriteString("\n\n## Skills\n")
	for _, s := range skills {
		fmt.Fprintf(&b, "\n### %s\n", s.Name)
		if s.Description != "" {
			fmt.Fprintf(&b, "%s\n\n", s.Description)
		}
		b.WriteString(s.Body)
		b.WriteString("\n")
	}
	return b.String()
}
