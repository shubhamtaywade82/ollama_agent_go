package tools

import (
	"context"
	"fmt"
	"os"
	"strconv"
	"strings"
)

// EditFile applies a unified diff to a file within the sandbox. The diff's file
// headers (---/+++) are ignored for path resolution; the "path" argument is
// authoritative. Hunks are located by context with a small fuzz window so the
// model does not need perfect line numbers.
type EditFile struct{ Root string }

func (t *EditFile) Name() string { return "edit_file" }
func (t *EditFile) Description() string {
	return "Apply a unified diff (hunks with @@ headers, context lines, and +/- lines) to a file at the given path. Use this for precise, surgical edits instead of rewriting the whole file."
}

func (t *EditFile) Parameters() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"path": map[string]any{"type": "string", "description": "File path relative to the project root."},
			"diff": map[string]any{"type": "string", "description": "Unified diff to apply, including @@ hunk headers."},
		},
		"required": []string{"path", "diff"},
	}
}

func (t *EditFile) Execute(_ context.Context, args map[string]any) (string, error) {
	p, err := argString(args, "path")
	if err != nil {
		return "", err
	}
	diff, err := argString(args, "diff")
	if err != nil {
		return "", err
	}
	abs, err := resolvePath(t.Root, p)
	if err != nil {
		return "", err
	}

	var original string
	if b, readErr := os.ReadFile(abs); readErr == nil {
		original = string(b)
	} else if !os.IsNotExist(readErr) {
		return "", fmt.Errorf("read %s: %w", p, readErr)
	}

	updated, applied, err := applyUnifiedDiff(original, diff)
	if err != nil {
		return "", fmt.Errorf("apply diff to %s: %w", p, err)
	}
	if err := os.WriteFile(abs, []byte(updated), 0o644); err != nil {
		return "", fmt.Errorf("write %s: %w", p, err)
	}
	return fmt.Sprintf("Applied %d hunk(s) to %s", applied, p), nil
}

type hunk struct {
	oldStart int      // 1-based line in the original
	oldLines []string // context + removed lines (the text the hunk expects to find)
	newLines []string // context + added lines (the replacement text)
}

// parseHunks extracts hunks from a unified diff, ignoring file header lines.
func parseHunks(diff string) ([]hunk, error) {
	lines := strings.Split(diff, "\n")
	var hunks []hunk
	var cur *hunk
	for _, line := range lines {
		switch {
		case strings.HasPrefix(line, "@@"):
			if cur != nil {
				hunks = append(hunks, *cur)
			}
			start, err := parseHunkHeader(line)
			if err != nil {
				return nil, err
			}
			cur = &hunk{oldStart: start}
		case strings.HasPrefix(line, "--- "), strings.HasPrefix(line, "+++ "):
			// file headers — ignore
		case cur == nil:
			// preamble before first hunk — ignore
		case strings.HasPrefix(line, "+"):
			cur.newLines = append(cur.newLines, line[1:])
		case strings.HasPrefix(line, "-"):
			cur.oldLines = append(cur.oldLines, line[1:])
		case strings.HasPrefix(line, " "):
			cur.oldLines = append(cur.oldLines, line[1:])
			cur.newLines = append(cur.newLines, line[1:])
		case line == "":
			// treat blank as a blank context line
			cur.oldLines = append(cur.oldLines, "")
			cur.newLines = append(cur.newLines, "")
		case strings.HasPrefix(line, "\\"):
			// "\ No newline at end of file" — ignore
		}
	}
	if cur != nil {
		hunks = append(hunks, *cur)
	}
	if len(hunks) == 0 {
		return nil, fmt.Errorf("no hunks found in diff")
	}
	return hunks, nil
}

// parseHunkHeader reads the old-side start line from "@@ -l,s +l,s @@".
func parseHunkHeader(line string) (int, error) {
	fields := strings.Fields(line)
	for _, f := range fields {
		if strings.HasPrefix(f, "-") {
			spec := strings.TrimPrefix(f, "-")
			if i := strings.IndexByte(spec, ','); i >= 0 {
				spec = spec[:i]
			}
			n, err := strconv.Atoi(spec)
			if err != nil {
				return 0, fmt.Errorf("bad hunk header %q: %w", line, err)
			}
			return n, nil
		}
	}
	return 0, fmt.Errorf("bad hunk header %q", line)
}

// applyUnifiedDiff applies the diff to original and returns the new content and
// the number of hunks applied.
func applyUnifiedDiff(original, diff string) (string, int, error) {
	hunks, err := parseHunks(diff)
	if err != nil {
		return "", 0, err
	}

	trailingNewline := strings.HasSuffix(original, "\n")
	var src []string
	if original == "" {
		src = nil
	} else {
		src = strings.Split(strings.TrimSuffix(original, "\n"), "\n")
	}

	// offset tracks net line additions from prior hunks so later hunk locations
	// shift accordingly.
	offset := 0
	for hi, h := range hunks {
		at, err := locate(src, h, h.oldStart-1+offset)
		if err != nil {
			return "", 0, fmt.Errorf("hunk %d: %w", hi+1, err)
		}
		end := at + len(h.oldLines)
		out := make([]string, 0, len(src)-len(h.oldLines)+len(h.newLines))
		out = append(out, src[:at]...)
		out = append(out, h.newLines...)
		out = append(out, src[end:]...)
		src = out
		offset += len(h.newLines) - len(h.oldLines)
	}

	result := strings.Join(src, "\n")
	if trailingNewline || original == "" {
		result += "\n"
	}
	return result, len(hunks), nil
}

// locate finds where hunk h's old lines occur in src, preferring the hinted
// index and expanding the search outward (fuzz) if needed.
func locate(src []string, h hunk, hint int) (int, error) {
	if len(h.oldLines) == 0 {
		// pure insertion — clamp hint into range
		if hint < 0 {
			hint = 0
		}
		if hint > len(src) {
			hint = len(src)
		}
		return hint, nil
	}
	if hint < 0 {
		hint = 0
	}
	// Try the hint first, then widen by +/- fuzz.
	const fuzz = 100
	candidates := []int{hint}
	for d := 1; d <= fuzz; d++ {
		candidates = append(candidates, hint+d, hint-d)
	}
	for _, c := range candidates {
		if matchesAt(src, h.oldLines, c) {
			return c, nil
		}
	}
	return 0, fmt.Errorf("could not locate context:\n%s", strings.Join(h.oldLines, "\n"))
}

func matchesAt(src, want []string, at int) bool {
	if at < 0 || at+len(want) > len(src) {
		return false
	}
	for i, w := range want {
		if src[at+i] != w {
			return false
		}
	}
	return true
}
