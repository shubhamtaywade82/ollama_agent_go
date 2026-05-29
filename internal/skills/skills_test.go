package skills

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestLoadWithFrontmatter(t *testing.T) {
	dir := t.TempDir()
	content := "---\nname: Go Style\ndescription: How to write Go\n---\nAlways run gofmt.\n"
	os.WriteFile(filepath.Join(dir, "go.md"), []byte(content), 0o644)

	sk, err := Load(dir)
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	if len(sk) != 1 {
		t.Fatalf("skills = %d", len(sk))
	}
	s := sk[0]
	if s.Name != "Go Style" {
		t.Errorf("name = %q", s.Name)
	}
	if s.Description != "How to write Go" {
		t.Errorf("description = %q", s.Description)
	}
	if s.Body != "Always run gofmt." {
		t.Errorf("body = %q", s.Body)
	}
}

func TestLoadNoFrontmatterUsesFilename(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "rules.md"), []byte("Be careful."), 0o644)
	sk, _ := Load(dir)
	if len(sk) != 1 || sk[0].Name != "rules" {
		t.Fatalf("skills = %+v", sk)
	}
	if sk[0].Body != "Be careful." {
		t.Errorf("body = %q", sk[0].Body)
	}
}

func TestLoadMissingDir(t *testing.T) {
	sk, err := Load(filepath.Join(t.TempDir(), "nope"))
	if err != nil {
		t.Errorf("missing dir should not error: %v", err)
	}
	if sk != nil {
		t.Errorf("expected nil skills, got %v", sk)
	}
}

func TestLoadSortedAndFiltered(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "b.md"), []byte("b"), 0o644)
	os.WriteFile(filepath.Join(dir, "a.md"), []byte("a"), 0o644)
	os.WriteFile(filepath.Join(dir, "ignore.txt"), []byte("x"), 0o644)
	sk, _ := Load(dir)
	if len(sk) != 2 {
		t.Fatalf("skills = %d, want 2", len(sk))
	}
	if sk[0].Name != "a" || sk[1].Name != "b" {
		t.Errorf("not sorted: %+v", sk)
	}
}

func TestInject(t *testing.T) {
	out := Inject("BASE", []Skill{{Name: "S1", Description: "d", Body: "do x"}})
	if !strings.Contains(out, "BASE") || !strings.Contains(out, "S1") || !strings.Contains(out, "do x") {
		t.Errorf("inject = %q", out)
	}
	if Inject("BASE", nil) != "BASE" {
		t.Error("nil skills should return base unchanged")
	}
}
