package tools

import (
	"context"
	"os"
	"path/filepath"
	"testing"
)

func TestApplyUnifiedDiffReplace(t *testing.T) {
	original := "line1\nline2\nline3\n"
	diff := `@@ -1,3 +1,3 @@
 line1
-line2
+LINE_TWO
 line3`
	got, n, err := applyUnifiedDiff(original, diff)
	if err != nil {
		t.Fatalf("apply: %v", err)
	}
	if n != 1 {
		t.Errorf("hunks applied = %d, want 1", n)
	}
	want := "line1\nLINE_TWO\nline3\n"
	if got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

func TestApplyUnifiedDiffInsert(t *testing.T) {
	original := "a\nb\n"
	diff := `@@ -1,2 +1,3 @@
 a
+inserted
 b`
	got, _, err := applyUnifiedDiff(original, diff)
	if err != nil {
		t.Fatalf("apply: %v", err)
	}
	if got != "a\ninserted\nb\n" {
		t.Errorf("got %q", got)
	}
}

func TestApplyUnifiedDiffDelete(t *testing.T) {
	original := "keep1\ndrop\nkeep2\n"
	diff := `@@ -1,3 +1,2 @@
 keep1
-drop
 keep2`
	got, _, err := applyUnifiedDiff(original, diff)
	if err != nil {
		t.Fatalf("apply: %v", err)
	}
	if got != "keep1\nkeep2\n" {
		t.Errorf("got %q", got)
	}
}

func TestApplyUnifiedDiffFuzzyLocation(t *testing.T) {
	original := "h1\nh2\nh3\ntarget\nh5\n"
	diff := `@@ -1,1 +1,1 @@
-target
+TARGET`
	got, _, err := applyUnifiedDiff(original, diff)
	if err != nil {
		t.Fatalf("apply: %v", err)
	}
	if got != "h1\nh2\nh3\nTARGET\nh5\n" {
		t.Errorf("got %q", got)
	}
}

func TestApplyUnifiedDiffContextMismatch(t *testing.T) {
	original := "a\nb\n"
	diff := `@@ -1,1 +1,1 @@
-does_not_exist
+x`
	if _, _, err := applyUnifiedDiff(original, diff); err == nil {
		t.Error("mismatched context should error")
	}
}

func TestApplyMultipleHunks(t *testing.T) {
	original := "1\n2\n3\n4\n5\n6\n"
	diff := `@@ -1,2 +1,2 @@
-1
+one
 2
@@ -5,2 +5,2 @@
 5
-6
+six`
	got, n, err := applyUnifiedDiff(original, diff)
	if err != nil {
		t.Fatalf("apply: %v", err)
	}
	if n != 2 {
		t.Errorf("hunks = %d", n)
	}
	if got != "one\n2\n3\n4\n5\nsix\n" {
		t.Errorf("got %q", got)
	}
}

func TestEditFileTool(t *testing.T) {
	root := t.TempDir()
	path := filepath.Join(root, "f.txt")
	os.WriteFile(path, []byte("foo\nbar\n"), 0o644)

	e := &EditFile{Root: root}
	_, err := e.Execute(context.Background(), map[string]any{
		"path": "f.txt",
		"diff": "@@ -1,2 +1,2 @@\n foo\n-bar\n+baz",
	})
	if err != nil {
		t.Fatalf("edit: %v", err)
	}
	b, _ := os.ReadFile(path)
	if string(b) != "foo\nbaz\n" {
		t.Errorf("file = %q", string(b))
	}
}
