package indexer_test

import (
	"strings"
	"testing"

	"ollama_agent_go/internal/indexer"
)

func TestChunkSplit(t *testing.T) {
	// Generate text longer than one chunk.
	words := make([]string, 300)
	for i := range words {
		words[i] = "word"
	}
	text := strings.Join(words, " ")

	chunks := indexer.SplitText("doc1", text, indexer.ChunkConfig{Size: 100, Overlap: 10})

	if len(chunks) < 2 {
		t.Fatalf("expected multiple chunks, got %d", len(chunks))
	}

	// Each chunk should have at most 100 words.
	for i, c := range chunks {
		wc := len(strings.Fields(c.Text))
		if wc > 100 {
			t.Errorf("chunk %d has %d words, want ≤100", i, wc)
		}
		if c.DocID != "doc1" {
			t.Errorf("chunk %d DocID: got %q, want doc1", i, c.DocID)
		}
	}
}

func TestChunkSingleChunk(t *testing.T) {
	text := "short text with only a few words"
	chunks := indexer.SplitText("doc2", text, indexer.ChunkConfig{Size: 256, Overlap: 32})

	if len(chunks) != 1 {
		t.Errorf("short text should produce 1 chunk, got %d", len(chunks))
	}
	if chunks[0].Text != text {
		t.Errorf("chunk text mismatch: got %q, want %q", chunks[0].Text, text)
	}
}

func TestChunkEmptyText(t *testing.T) {
	chunks := indexer.SplitText("empty", "", indexer.DefaultChunkConfig)
	if len(chunks) != 0 {
		t.Errorf("empty text should yield 0 chunks, got %d", len(chunks))
	}
}

func TestChunkOverlapIsSmaller(t *testing.T) {
	words := make([]string, 50)
	for i := range words {
		words[i] = "x"
	}
	text := strings.Join(words, " ")

	cfg := indexer.ChunkConfig{Size: 20, Overlap: 5}
	chunks := indexer.SplitText("doc", text, cfg)

	if len(chunks) < 2 {
		t.Skip("not enough words to test overlap")
	}

	// The second chunk should start 15 words into the first (Size-Overlap = 20-5 = 15).
	firstWords := strings.Fields(chunks[0].Text)
	secondWords := strings.Fields(chunks[1].Text)
	// overlap means last 'Overlap' words of chunk[0] == first 'Overlap' words of chunk[1]
	overlapFirst := firstWords[len(firstWords)-cfg.Overlap:]
	overlapSecond := secondWords[:cfg.Overlap]
	if strings.Join(overlapFirst, " ") != strings.Join(overlapSecond, " ") {
		t.Errorf("overlap mismatch:\n  end of chunk 0: %v\n  start of chunk 1: %v",
			overlapFirst, overlapSecond)
	}
}
