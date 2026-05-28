package tools

import (
	"path/filepath"
	"testing"
)

func TestResolvePathWithinRoot(t *testing.T) {
	root := t.TempDir()
	got, err := resolvePath(root, "sub/file.txt")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	want := filepath.Join(root, "sub", "file.txt")
	if got != want {
		t.Errorf("resolvePath = %q, want %q", got, want)
	}
}

func TestResolvePathRootItself(t *testing.T) {
	root := t.TempDir()
	got, err := resolvePath(root, "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != filepath.Clean(root) {
		t.Errorf("resolvePath empty = %q, want root %q", got, root)
	}
}

func TestResolvePathRejectsTraversal(t *testing.T) {
	root := t.TempDir()
	for _, p := range []string{"../escape", "../../etc/passwd", "sub/../../escape"} {
		if _, err := resolvePath(root, p); err == nil {
			t.Errorf("resolvePath(%q) should have failed", p)
		}
	}
}

func TestResolvePathRejectsAbsoluteOutside(t *testing.T) {
	root := t.TempDir()
	if _, err := resolvePath(root, "/etc/passwd"); err == nil {
		t.Error("absolute path outside root should be rejected")
	}
}

func TestArgString(t *testing.T) {
	args := map[string]any{"path": "x", "n": 1}
	if v, err := argString(args, "path"); err != nil || v != "x" {
		t.Errorf("argString path = %q, %v", v, err)
	}
	if _, err := argString(args, "missing"); err == nil {
		t.Error("missing arg should error")
	}
	if _, err := argString(args, "n"); err == nil {
		t.Error("non-string arg should error")
	}
}
