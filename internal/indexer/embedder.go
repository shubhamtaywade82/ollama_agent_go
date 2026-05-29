// Package indexer implements text chunking, embedding, and knowledge indexing.
package indexer

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
)

// Embedder converts text to float32 embedding vectors.
type Embedder interface {
	Embed(ctx context.Context, texts []string) ([][]float32, error)
	Dims() int
}

// OllamaEmbedder calls /api/embed on a local Ollama instance.
type OllamaEmbedder struct {
	baseURL string
	model   string
	client  *http.Client
	dims    int
}

// NewOllamaEmbedder returns an embedder targeting the given Ollama endpoint.
// model is typically "nomic-embed-text" (768 dims) or "mxbai-embed-large" (1024 dims).
func NewOllamaEmbedder(baseURL, model string) *OllamaEmbedder {
	return &OllamaEmbedder{
		baseURL: baseURL,
		model:   model,
		client:  &http.Client{},
		dims:    768, // default; updated after first call
	}
}

type embedRequest struct {
	Model  string   `json:"model"`
	Input  []string `json:"input"`
}

type embedResponse struct {
	Embeddings [][]float32 `json:"embeddings"`
}

func (e *OllamaEmbedder) Embed(ctx context.Context, texts []string) ([][]float32, error) {
	body, err := json.Marshal(embedRequest{Model: e.model, Input: texts})
	if err != nil {
		return nil, err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, e.baseURL+"/api/embed", bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := e.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("embed: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("embed: HTTP %d", resp.StatusCode)
	}

	var result embedResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, err
	}
	if len(result.Embeddings) > 0 {
		e.dims = len(result.Embeddings[0])
	}
	return result.Embeddings, nil
}

func (e *OllamaEmbedder) Dims() int { return e.dims }

// NoopEmbedder returns zero vectors. Used when no embedding model is available.
type NoopEmbedder struct{ dims int }

func NewNoopEmbedder(dims int) *NoopEmbedder { return &NoopEmbedder{dims: dims} }

func (n *NoopEmbedder) Embed(_ context.Context, texts []string) ([][]float32, error) {
	out := make([][]float32, len(texts))
	for i := range out {
		out[i] = make([]float32, n.dims)
	}
	return out, nil
}

func (n *NoopEmbedder) Dims() int { return n.dims }
