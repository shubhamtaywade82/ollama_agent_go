package tools

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

const maxReadBytes = 1 << 20 // 1 MiB

type ReadFile struct{ Root string }

func (t *ReadFile) Name() string        { return "read_file" }
func (t *ReadFile) Description() string { return "Read the contents of a text file at the given path (relative to the project root)." }
func (t *ReadFile) Parameters() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"path": map[string]any{"type": "string", "description": "File path relative to the project root."},
		},
		"required": []string{"path"},
	}
}

func (t *ReadFile) Execute(_ context.Context, args map[string]any) (string, error) {
	p, err := argString(args, "path")
	if err != nil {
		return "", err
	}
	abs, err := resolvePath(t.Root, p)
	if err != nil {
		return "", err
	}
	info, err := os.Stat(abs)
	if err != nil {
		return "", fmt.Errorf("stat %s: %w", p, err)
	}
	if info.IsDir() {
		return "", fmt.Errorf("%s is a directory; use ls", p)
	}
	if info.Size() > maxReadBytes {
		return "", fmt.Errorf("%s is too large (%d bytes, max %d)", p, info.Size(), maxReadBytes)
	}
	b, err := os.ReadFile(abs)
	if err != nil {
		return "", fmt.Errorf("read %s: %w", p, err)
	}
	return string(b), nil
}

type WriteFile struct{ Root string }

func (t *WriteFile) Name() string        { return "write_file" }
func (t *WriteFile) Description() string { return "Create or overwrite a text file with the given content (path relative to the project root). Parent directories are created automatically." }
func (t *WriteFile) Parameters() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"path":    map[string]any{"type": "string", "description": "File path relative to the project root."},
			"content": map[string]any{"type": "string", "description": "Full file content to write."},
		},
		"required": []string{"path", "content"},
	}
}

func (t *WriteFile) Execute(_ context.Context, args map[string]any) (string, error) {
	p, err := argString(args, "path")
	if err != nil {
		return "", err
	}
	content, err := argString(args, "content")
	if err != nil {
		return "", err
	}
	abs, err := resolvePath(t.Root, p)
	if err != nil {
		return "", err
	}
	if err := os.MkdirAll(filepath.Dir(abs), 0o755); err != nil {
		return "", fmt.Errorf("mkdir for %s: %w", p, err)
	}
	if err := os.WriteFile(abs, []byte(content), 0o644); err != nil {
		return "", fmt.Errorf("write %s: %w", p, err)
	}
	return fmt.Sprintf("Wrote %d bytes to %s", len(content), p), nil
}

type LS struct{ Root string }

func (t *LS) Name() string        { return "ls" }
func (t *LS) Description() string { return "List the entries of a directory (path relative to the project root; defaults to the root)." }
func (t *LS) Parameters() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"path": map[string]any{"type": "string", "description": "Directory path relative to the project root. Defaults to the root."},
		},
	}
}

func (t *LS) Execute(_ context.Context, args map[string]any) (string, error) {
	p := argStringOpt(args, "path", ".")
	abs, err := resolvePath(t.Root, p)
	if err != nil {
		return "", err
	}
	entries, err := os.ReadDir(abs)
	if err != nil {
		return "", fmt.Errorf("ls %s: %w", p, err)
	}
	lines := make([]string, 0, len(entries))
	for _, e := range entries {
		name := e.Name()
		if e.IsDir() {
			name += "/"
		}
		lines = append(lines, name)
	}
	sort.Strings(lines)
	if len(lines) == 0 {
		return "(empty directory)", nil
	}
	return strings.Join(lines, "\n"), nil
}
