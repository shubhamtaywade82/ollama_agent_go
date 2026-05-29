// Package secrets provides a simple secrets manager abstraction used by the
// agent runtime to retrieve API keys without hardcoding them in config.
package secrets

import (
	"context"
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"io"
	"os"
)

// Manager is the secrets retrieval interface.
type Manager interface {
	Get(ctx context.Context, key string) (string, error)
	Set(ctx context.Context, key, value string) error
	Delete(ctx context.Context, key string) error
}

// ─── EnvManager ──────────────────────────────────────────────────────────────

// EnvManager reads secrets from environment variables. Read-only; Set and
// Delete are no-ops because env vars can't be persisted across processes.
type EnvManager struct{}

func (EnvManager) Get(_ context.Context, key string) (string, error) {
	v := os.Getenv(key)
	if v == "" {
		return "", fmt.Errorf("secrets: env var %q not set", key)
	}
	return v, nil
}
func (EnvManager) Set(_ context.Context, _, _ string) error    { return nil }
func (EnvManager) Delete(_ context.Context, _ string) error    { return nil }

// ─── FileManager ─────────────────────────────────────────────────────────────

// FileManager stores secrets in an AES-256-GCM encrypted JSON file.
// The encryption key is derived from the masterKey using SHA-256.
type FileManager struct {
	path string
	key  []byte // 32 bytes, derived from masterKey
}

// NewFileManager creates a FileManager.
// masterKey is hashed with SHA-256 to produce the 32-byte AES key.
func NewFileManager(path, masterKey string) *FileManager {
	h := sha256.Sum256([]byte(masterKey))
	return &FileManager{path: path, key: h[:]}
}

func (f *FileManager) Get(_ context.Context, key string) (string, error) {
	store, err := f.load()
	if err != nil {
		return "", err
	}
	v, ok := store[key]
	if !ok {
		return "", fmt.Errorf("secrets: key %q not found", key)
	}
	return v, nil
}

func (f *FileManager) Set(_ context.Context, key, value string) error {
	store, _ := f.load() // ignore load errors (file may not exist yet)
	if store == nil {
		store = make(map[string]string)
	}
	store[key] = value
	return f.save(store)
}

func (f *FileManager) Delete(_ context.Context, key string) error {
	store, err := f.load()
	if err != nil {
		return err
	}
	delete(store, key)
	return f.save(store)
}

func (f *FileManager) load() (map[string]string, error) {
	data, err := os.ReadFile(f.path)
	if os.IsNotExist(err) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	plain, err := f.decrypt(data)
	if err != nil {
		return nil, fmt.Errorf("secrets: decrypt: %w", err)
	}
	var store map[string]string
	if err := json.Unmarshal(plain, &store); err != nil {
		return nil, fmt.Errorf("secrets: unmarshal: %w", err)
	}
	return store, nil
}

func (f *FileManager) save(store map[string]string) error {
	plain, err := json.Marshal(store)
	if err != nil {
		return err
	}
	enc, err := f.encrypt(plain)
	if err != nil {
		return fmt.Errorf("secrets: encrypt: %w", err)
	}
	return os.WriteFile(f.path, enc, 0600)
}

func (f *FileManager) encrypt(plain []byte) ([]byte, error) {
	block, err := aes.NewCipher(f.key)
	if err != nil {
		return nil, err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}
	nonce := make([]byte, gcm.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return nil, err
	}
	return gcm.Seal(nonce, nonce, plain, nil), nil
}

func (f *FileManager) decrypt(data []byte) ([]byte, error) {
	block, err := aes.NewCipher(f.key)
	if err != nil {
		return nil, err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}
	if len(data) < gcm.NonceSize() {
		return nil, fmt.Errorf("ciphertext too short")
	}
	nonce, ct := data[:gcm.NonceSize()], data[gcm.NonceSize():]
	return gcm.Open(nil, nonce, ct, nil)
}

// New returns the best available secrets manager:
// FileManager if OLLAMA_SECRETS_PATH and OLLAMA_MASTER_KEY are set,
// otherwise EnvManager.
func New() Manager {
	path := os.Getenv("OLLAMA_SECRETS_PATH")
	key := os.Getenv("OLLAMA_MASTER_KEY")
	if path != "" && key != "" {
		return NewFileManager(path, key)
	}
	return EnvManager{}
}
