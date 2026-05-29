package secrets_test

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"ollama_agent_go/internal/secrets"
)

func TestEnvManagerGet(t *testing.T) {
	t.Setenv("TEST_SECRET_KEY", "super-secret")
	m := secrets.EnvManager{}
	val, err := m.Get(context.Background(), "TEST_SECRET_KEY")
	if err != nil {
		t.Fatal(err)
	}
	if val != "super-secret" {
		t.Errorf("expected 'super-secret', got %q", val)
	}
}

func TestEnvManagerMissing(t *testing.T) {
	m := secrets.EnvManager{}
	_, err := m.Get(context.Background(), "NONEXISTENT_SECRET_KEY_XYZ")
	if err == nil {
		t.Error("expected error for missing key, got nil")
	}
}

func TestFileManagerEncryptDecrypt(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "secrets.bin")
	ctx := context.Background()

	m := secrets.NewFileManager(path, "my-master-key")

	if err := m.Set(ctx, "api_key", "abc123"); err != nil {
		t.Fatal(err)
	}

	// Read back from the same file with the same key.
	m2 := secrets.NewFileManager(path, "my-master-key")
	val, err := m2.Get(ctx, "api_key")
	if err != nil {
		t.Fatal(err)
	}
	if val != "abc123" {
		t.Errorf("expected 'abc123', got %q", val)
	}
}

func TestFileManagerWrongKey(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "secrets.bin")
	ctx := context.Background()

	m := secrets.NewFileManager(path, "correct-key")
	_ = m.Set(ctx, "k", "v")

	m2 := secrets.NewFileManager(path, "wrong-key")
	_, err := m2.Get(ctx, "k")
	if err == nil {
		t.Error("expected error with wrong key, got nil")
	}
}

func TestFileManagerDelete(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "secrets.bin")
	ctx := context.Background()

	m := secrets.NewFileManager(path, "key")
	_ = m.Set(ctx, "k", "v")
	_ = m.Delete(ctx, "k")

	_, err := m.Get(ctx, "k")
	if err == nil {
		t.Error("expected error after delete")
	}
	// Ensure the file was not deleted (empty store is still written).
	if _, statErr := os.Stat(path); os.IsNotExist(statErr) {
		t.Error("expected secrets file to still exist after delete")
	}
}
