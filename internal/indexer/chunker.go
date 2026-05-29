package indexer

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// ChunkConfig controls how text is split into chunks.
type ChunkConfig struct {
	Size    int // words per chunk (default 256)
	Overlap int // word overlap between consecutive chunks (default 32)
}

// DefaultChunkConfig is used when zero values are passed.
var DefaultChunkConfig = ChunkConfig{Size: 256, Overlap: 32}

// Chunk is a text fragment from a source document.
type Chunk struct {
	DocID    string
	Index    int
	Text     string
	Metadata map[string]any
}

// SplitText splits text into overlapping word-based chunks.
func SplitText(docID, text string, cfg ChunkConfig) []Chunk {
	if cfg.Size <= 0 {
		cfg = DefaultChunkConfig
	}
	if cfg.Overlap < 0 {
		cfg.Overlap = 0
	}
	if cfg.Overlap >= cfg.Size {
		cfg.Overlap = cfg.Size / 4
	}

	words := strings.Fields(text)
	if len(words) == 0 {
		return nil
	}

	var chunks []Chunk
	step := cfg.Size - cfg.Overlap
	if step <= 0 {
		step = 1
	}

	for i, idx := 0, 0; idx < len(words); i, idx = i+1, idx+step {
		end := idx + cfg.Size
		if end > len(words) {
			end = len(words)
		}
		chunks = append(chunks, Chunk{
			DocID: docID,
			Index: i,
			Text:  strings.Join(words[idx:end], " "),
			Metadata: map[string]any{
				"doc_id": docID,
				"index":  i,
			},
		})
		if end == len(words) {
			break
		}
	}
	return chunks
}

// ChunkFile reads a file and returns its chunks with path metadata.
func ChunkFile(root, relPath string, cfg ChunkConfig) ([]Chunk, error) {
	fullPath := filepath.Join(root, relPath)
	data, err := os.ReadFile(fullPath)
	if err != nil {
		return nil, fmt.Errorf("chunk file %s: %w", relPath, err)
	}
	docID := relPath
	chunks := SplitText(docID, string(data), cfg)
	ext := strings.TrimPrefix(filepath.Ext(relPath), ".")
	for i := range chunks {
		chunks[i].Metadata["path"] = relPath
		chunks[i].Metadata["ext"] = ext
	}
	return chunks, nil
}
