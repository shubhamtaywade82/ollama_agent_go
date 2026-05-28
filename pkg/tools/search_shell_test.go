package tools

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestGrepSearch(t *testing.T) {
	root := t.TempDir()
	os.WriteFile(filepath.Join(root, "a.go"), []byte("package a\nfunc Foo() {}\n"), 0o644)
	os.WriteFile(filepath.Join(root, "b.txt"), []byte("nothing here\nFoo appears too\n"), 0o644)

	g := &GrepSearch{Root: root}
	out, err := g.Execute(context.Background(), map[string]any{"pattern": "Foo"})
	if err != nil {
		t.Fatalf("grep: %v", err)
	}
	if !strings.Contains(out, "a.go:2:func Foo() {}") {
		t.Errorf("grep missing go match: %q", out)
	}
	if !strings.Contains(out, "b.txt:2:Foo appears too") {
		t.Errorf("grep missing txt match: %q", out)
	}
}

func TestGrepSearchGlob(t *testing.T) {
	root := t.TempDir()
	os.WriteFile(filepath.Join(root, "a.go"), []byte("Foo\n"), 0o644)
	os.WriteFile(filepath.Join(root, "b.txt"), []byte("Foo\n"), 0o644)

	g := &GrepSearch{Root: root}
	out, _ := g.Execute(context.Background(), map[string]any{"pattern": "Foo", "glob": "*.go"})
	if strings.Contains(out, "b.txt") {
		t.Errorf("glob should exclude b.txt: %q", out)
	}
	if !strings.Contains(out, "a.go") {
		t.Errorf("glob should include a.go: %q", out)
	}
}

func TestGrepNoMatch(t *testing.T) {
	root := t.TempDir()
	os.WriteFile(filepath.Join(root, "a.txt"), []byte("hello\n"), 0o644)
	g := &GrepSearch{Root: root}
	out, _ := g.Execute(context.Background(), map[string]any{"pattern": "zzz"})
	if out != "(no matches)" {
		t.Errorf("expected no matches, got %q", out)
	}
}

func TestGrepInvalidPattern(t *testing.T) {
	g := &GrepSearch{Root: t.TempDir()}
	if _, err := g.Execute(context.Background(), map[string]any{"pattern": "("}); err == nil {
		t.Error("invalid regex should error")
	}
}

func TestRunShell(t *testing.T) {
	root := t.TempDir()
	s := &RunShell{Root: root}
	out, err := s.Execute(context.Background(), map[string]any{"command": "echo hello"})
	if err != nil {
		t.Fatalf("run_shell: %v", err)
	}
	if !strings.Contains(out, "hello") {
		t.Errorf("output = %q", out)
	}
}

func TestRunShellRunsInRoot(t *testing.T) {
	root := t.TempDir()
	os.WriteFile(filepath.Join(root, "marker.txt"), []byte("x"), 0o644)
	s := &RunShell{Root: root}
	out, _ := s.Execute(context.Background(), map[string]any{"command": "ls"})
	if !strings.Contains(out, "marker.txt") {
		t.Errorf("command did not run in root: %q", out)
	}
}

func TestRunShellNonZeroExit(t *testing.T) {
	s := &RunShell{Root: t.TempDir()}
	out, err := s.Execute(context.Background(), map[string]any{"command": "exit 3"})
	if err != nil {
		t.Fatalf("non-zero exit should not be a Go error: %v", err)
	}
	if !strings.Contains(out, "exit") {
		t.Errorf("expected exit annotation: %q", out)
	}
}
