package tools

import (
	"bufio"
	"context"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

const maxGrepMatches = 200

// GrepSearch searches file contents under the sandbox root for a regular expression.
type GrepSearch struct{ Root string }

func (t *GrepSearch) Name() string        { return "grep_search" }
func (t *GrepSearch) Description() string { return "Search file contents recursively for a regular expression. Returns matching lines as path:line:text. Optionally restrict to a subdirectory and/or a filename glob." }
func (t *GrepSearch) Parameters() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"pattern": map[string]any{"type": "string", "description": "RE2 regular expression to search for."},
			"path":    map[string]any{"type": "string", "description": "Subdirectory to search (relative to root). Defaults to the root."},
			"glob":    map[string]any{"type": "string", "description": "Optional filename glob filter, e.g. *.go"},
		},
		"required": []string{"pattern"},
	}
}

func (t *GrepSearch) Execute(_ context.Context, args map[string]any) (string, error) {
	pattern, err := argString(args, "pattern")
	if err != nil {
		return "", err
	}
	re, err := regexp.Compile(pattern)
	if err != nil {
		return "", fmt.Errorf("invalid pattern: %w", err)
	}
	sub := argStringOpt(args, "path", ".")
	glob := argStringOpt(args, "glob", "")

	base, err := resolvePath(t.Root, sub)
	if err != nil {
		return "", err
	}

	var matches []string
	walkErr := filepath.WalkDir(base, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return nil
		}
		if d.IsDir() {
			if skipDir(d.Name()) {
				return filepath.SkipDir
			}
			return nil
		}
		if glob != "" {
			if ok, _ := filepath.Match(glob, d.Name()); !ok {
				return nil
			}
		}
		if isProbablyBinary(path) {
			return nil
		}
		f, err := os.Open(path)
		if err != nil {
			return nil
		}
		defer f.Close()
		rel := relPath(t.Root, path)
		scanner := bufio.NewScanner(f)
		scanner.Buffer(make([]byte, 0, 64*1024), 1024*1024)
		ln := 0
		for scanner.Scan() {
			ln++
			line := scanner.Text()
			if re.MatchString(line) {
				matches = append(matches, fmt.Sprintf("%s:%d:%s", rel, ln, strings.TrimSpace(line)))
				if len(matches) >= maxGrepMatches {
					return filepath.SkipAll
				}
			}
		}
		return nil
	})
	if walkErr != nil {
		return "", fmt.Errorf("search: %w", walkErr)
	}
	if len(matches) == 0 {
		return "(no matches)", nil
	}
	return strings.Join(matches, "\n"), nil
}

func skipDir(name string) bool {
	switch name {
	case ".git", "node_modules", "vendor", ".cache", "dist", "bin":
		return true
	}
	return false
}

func isProbablyBinary(path string) bool {
	f, err := os.Open(path)
	if err != nil {
		return true
	}
	defer f.Close()
	buf := make([]byte, 512)
	n, _ := f.Read(buf)
	for _, b := range buf[:n] {
		if b == 0 {
			return true
		}
	}
	return false
}
