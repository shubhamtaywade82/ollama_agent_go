package tools

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestWriteThenReadFile(t *testing.T) {
	root := t.TempDir()
	w := &WriteFile{Root: root}
	out, err := w.Execute(context.Background(), map[string]any{"path": "dir/hello.txt", "content": "hi there"})
	if err != nil {
		t.Fatalf("write: %v", err)
	}
	if !strings.Contains(out, "hello.txt") {
		t.Errorf("write output = %q", out)
	}
	if _, err := os.Stat(filepath.Join(root, "dir", "hello.txt")); err != nil {
		t.Fatalf("file not created: %v", err)
	}

	r := &ReadFile{Root: root}
	got, err := r.Execute(context.Background(), map[string]any{"path": "dir/hello.txt"})
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	if got != "hi there" {
		t.Errorf("read = %q, want %q", got, "hi there")
	}
}

func TestReadFileErrors(t *testing.T) {
	root := t.TempDir()
	r := &ReadFile{Root: root}
	if _, err := r.Execute(context.Background(), map[string]any{"path": "nope.txt"}); err == nil {
		t.Error("reading missing file should error")
	}
	if _, err := r.Execute(context.Background(), map[string]any{"path": "."}); err == nil {
		t.Error("reading a directory should error")
	}
}

func TestWriteFileSandbox(t *testing.T) {
	root := t.TempDir()
	w := &WriteFile{Root: root}
	if _, err := w.Execute(context.Background(), map[string]any{"path": "../escape.txt", "content": "x"}); err == nil {
		t.Error("write outside sandbox should error")
	}
}

func TestLS(t *testing.T) {
	root := t.TempDir()
	os.WriteFile(filepath.Join(root, "b.txt"), []byte("x"), 0o644)
	os.Mkdir(filepath.Join(root, "adir"), 0o755)

	l := &LS{Root: root}
	out, err := l.Execute(context.Background(), map[string]any{})
	if err != nil {
		t.Fatalf("ls: %v", err)
	}
	if out != "adir/\nb.txt" {
		t.Errorf("ls = %q", out)
	}
}

func TestLSEmpty(t *testing.T) {
	root := t.TempDir()
	l := &LS{Root: root}
	out, _ := l.Execute(context.Background(), map[string]any{})
	if out != "(empty directory)" {
		t.Errorf("ls empty = %q", out)
	}
}
