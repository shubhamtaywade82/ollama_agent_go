// Package retriever provides RAG context retrieval for the agent runtime.
package retriever

import (
	"context"
	"fmt"
	"strings"

	"ollama_agent_go/internal/memory/longterm"
)

// Retriever fetches relevant context chunks from a longterm.Store.
type Retriever struct {
	store longterm.Store
	topK  int
}

// New creates a Retriever that returns up to topK results per query.
func New(store longterm.Store, topK int) *Retriever {
	if topK <= 0 {
		topK = 5
	}
	return &Retriever{store: store, topK: topK}
}

// Retrieve returns the top-K chunks most relevant to query.
func (r *Retriever) Retrieve(ctx context.Context, query string) ([]longterm.Result, error) {
	return r.store.Search(ctx, query, r.topK)
}

// InjectContext formats retrieved chunks as a system context block.
func (r *Retriever) InjectContext(chunks []longterm.Result) string {
	if len(chunks) == 0 {
		return ""
	}
	var b strings.Builder
	b.WriteString("## Retrieved Context\n\n")
	for i, c := range chunks {
		source := ""
		if p, ok := c.Meta["path"].(string); ok {
			source = fmt.Sprintf(" (source: %s)", p)
		}
		b.WriteString(fmt.Sprintf("### Chunk %d%s\n\n%s\n\n", i+1, source, c.Text))
	}
	return b.String()
}
