package tools

import (
	"context"
	"fmt"
	"strings"

	"ollama_agent_go/internal/memory/longterm"
)

// RAGStore is the subset of longterm.Store needed by the search_knowledge tool.
type RAGStore interface {
	Search(ctx context.Context, query string, topK int) ([]longterm.Result, error)
}

// KnowledgeTool implements the search_knowledge tool.
type KnowledgeTool struct {
	store RAGStore
}

func (t *KnowledgeTool) Name() string        { return "search_knowledge" }
func (t *KnowledgeTool) Description() string { return "Search the indexed knowledge base for relevant context." }
func (t *KnowledgeTool) Parameters() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"query": map[string]any{
				"type":        "string",
				"description": "The search query.",
			},
			"top_k": map[string]any{
				"type":        "integer",
				"description": "Number of results to return (default 5).",
			},
		},
		"required": []string{"query"},
	}
}

func (t *KnowledgeTool) Execute(ctx context.Context, args map[string]any) (string, error) {
	query, _ := args["query"].(string)
	if query == "" {
		return "", fmt.Errorf("search_knowledge: query is required")
	}
	topK := 5
	if v, ok := args["top_k"].(float64); ok && v > 0 {
		topK = int(v)
	}

	results, err := t.store.Search(ctx, query, topK)
	if err != nil {
		return "", fmt.Errorf("search_knowledge: %w", err)
	}
	if len(results) == 0 {
		return "No relevant context found.", nil
	}

	var b strings.Builder
	for i, r := range results {
		source := ""
		if p, ok := r.Meta["path"].(string); ok {
			source = fmt.Sprintf(" [%s]", p)
		}
		b.WriteString(fmt.Sprintf("Result %d%s (score %.3f):\n%s\n\n", i+1, source, r.Score, r.Text))
	}
	return b.String(), nil
}

// RegisterKnowledgeTool adds the search_knowledge tool to reg using store.
// This is a no-op if store is nil (RAG not configured).
func RegisterKnowledgeTool(reg *Registry, store RAGStore) {
	if store == nil {
		return
	}
	reg.Register(&KnowledgeTool{store: store})
}
