package indexer

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"ollama_agent_go/internal/memory/longterm"
)

// supportedExts is the default set of file extensions indexed by IndexDir.
var supportedExts = map[string]bool{
	".md": true, ".txt": true, ".go": true, ".py": true,
	".ts": true, ".js": true, ".yaml": true, ".yml": true,
	".json": true, ".toml": true,
}

// Indexer chunks, embeds, and stores documents in a longterm.Store.
type Indexer struct {
	store    longterm.Store
	embedder Embedder
	chunks   ChunkConfig
	root     string
}

// New builds an Indexer. root is the sandbox root for relative file paths.
func New(store longterm.Store, embedder Embedder, root string) *Indexer {
	return &Indexer{
		store:    store,
		embedder: embedder,
		root:     root,
		chunks:   DefaultChunkConfig,
	}
}

// WithChunkConfig replaces the chunk configuration.
func (idx *Indexer) WithChunkConfig(cfg ChunkConfig) *Indexer {
	idx.chunks = cfg
	return idx
}

// IndexFile chunks and embeds relPath (relative to Indexer root), upserts into store.
func (idx *Indexer) IndexFile(ctx context.Context, relPath string) error {
	chunks, err := ChunkFile(idx.root, relPath, idx.chunks)
	if err != nil {
		return err
	}
	if len(chunks) == 0 {
		return nil
	}

	texts := make([]string, len(chunks))
	for i, c := range chunks {
		texts[i] = c.Text
	}

	embeddings, err := idx.embedder.Embed(ctx, texts)
	if err != nil {
		return fmt.Errorf("embed %s: %w", relPath, err)
	}

	for i, c := range chunks {
		id := fmt.Sprintf("%s#%d", c.DocID, c.Index)
		meta := c.Metadata
		// attach embedding dimensions for debugging
		if i < len(embeddings) {
			meta["dims"] = len(embeddings[i])
		}
		if err := idx.store.Insert(ctx, id, c.Text, meta); err != nil {
			return fmt.Errorf("store %s chunk %d: %w", relPath, i, err)
		}
	}
	return nil
}

// IndexDir walks dir (relative to root) and indexes all files with supported extensions.
// Pass a non-nil exts slice to override the default set.
func (idx *Indexer) IndexDir(ctx context.Context, dir string, exts []string) error {
	extSet := supportedExts
	if len(exts) > 0 {
		extSet = make(map[string]bool, len(exts))
		for _, e := range exts {
			if !strings.HasPrefix(e, ".") {
				e = "." + e
			}
			extSet[e] = true
		}
	}

	absDir := filepath.Join(idx.root, dir)
	return filepath.Walk(absDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info.IsDir() {
			// Skip hidden directories.
			if strings.HasPrefix(info.Name(), ".") && path != absDir {
				return filepath.SkipDir
			}
			return nil
		}
		if !extSet[filepath.Ext(path)] {
			return nil
		}
		rel, _ := filepath.Rel(idx.root, path)
		return idx.IndexFile(ctx, rel)
	})
}
