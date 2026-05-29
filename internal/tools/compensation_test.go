package tools

import (
	"context"
	"os"
	"path/filepath"
	"testing"
)

func TestCompensateWriteFileNewFile(t *testing.T) {
	root := t.TempDir()
	// Create the file (simulates write_file having run).
	target := filepath.Join(root, "new.txt")
	if err := os.WriteFile(target, []byte("created by agent"), 0o644); err != nil {
		t.Fatal(err)
	}

	// before == nil means file was new; compensation should delete it.
	args := map[string]any{"path": "new.txt"}
	if err := compensateWriteFile(context.Background(), root, args, nil); err != nil {
		t.Fatalf("compensate: %v", err)
	}
	if _, err := os.Stat(target); !os.IsNotExist(err) {
		t.Error("new file should have been deleted by compensation")
	}
}

func TestCompensateWriteFileExistingContent(t *testing.T) {
	root := t.TempDir()
	original := []byte("original content")
	target := filepath.Join(root, "existing.txt")

	// Write the "after" state (simulates write_file overwrite).
	if err := os.WriteFile(target, []byte("overwritten by agent"), 0o644); err != nil {
		t.Fatal(err)
	}

	args := map[string]any{"path": "existing.txt"}
	if err := compensateWriteFile(context.Background(), root, args, original); err != nil {
		t.Fatalf("compensate: %v", err)
	}

	got, err := os.ReadFile(target)
	if err != nil {
		t.Fatalf("read after compensation: %v", err)
	}
	if string(got) != string(original) {
		t.Errorf("content after compensation = %q, want %q", got, original)
	}
}

func TestCompensateEditFileSameAsWriteFile(t *testing.T) {
	root := t.TempDir()
	orig := []byte("before edit")
	target := filepath.Join(root, "edit.go")
	_ = os.WriteFile(target, []byte("after edit"), 0o644)

	args := map[string]any{"path": "edit.go"}
	if err := compensateEditFile(context.Background(), root, args, orig); err != nil {
		t.Fatalf("compensate: %v", err)
	}
	got, _ := os.ReadFile(target)
	if string(got) != string(orig) {
		t.Errorf("content = %q, want %q", got, orig)
	}
}

func TestCaptureFileBeforeNewFile(t *testing.T) {
	root := t.TempDir()
	// File does not exist; should return nil, nil.
	data, err := CaptureFileBefore(root, "nonexistent.txt")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if data != nil {
		t.Errorf("expected nil for missing file, got %q", data)
	}
}

func TestCaptureFileBeforeExisting(t *testing.T) {
	root := t.TempDir()
	content := []byte("hello")
	_ = os.WriteFile(filepath.Join(root, "f.txt"), content, 0o644)

	data, err := CaptureFileBefore(root, "f.txt")
	if err != nil {
		t.Fatalf("capture: %v", err)
	}
	if string(data) != string(content) {
		t.Errorf("captured = %q, want %q", data, content)
	}
}
