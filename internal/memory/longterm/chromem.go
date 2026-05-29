package longterm

import (
	"context"
	"fmt"

	chromem "github.com/philippgille/chromem-go"
)

// ChromemStore implements Store using chromem-go for in-process vector search.
// It requires an embedding function compatible with chromem-go.
type ChromemStore struct {
	db         *chromem.DB
	collection *chromem.Collection
}

// NewChromemStore opens (or creates) a chromem DB at path and returns a Store
// backed by the named collection. embedFn must produce vectors for query text.
func NewChromemStore(path string, embedFn chromem.EmbeddingFunc) (*ChromemStore, error) {
	db, err := chromem.NewPersistentDB(path, false)
	if err != nil {
		return nil, fmt.Errorf("chromem open: %w", err)
	}
	coll, err := db.GetOrCreateCollection("knowledge", nil, embedFn)
	if err != nil {
		return nil, fmt.Errorf("chromem collection: %w", err)
	}
	return &ChromemStore{db: db, collection: coll}, nil
}

// Insert adds (or replaces) a document in the vector store.
func (s *ChromemStore) Insert(ctx context.Context, id, text string, meta map[string]any) error {
	strMeta := make(map[string]string, len(meta))
	for k, v := range meta {
		strMeta[k] = fmt.Sprintf("%v", v)
	}
	return s.collection.AddDocument(ctx, chromem.Document{
		ID:       id,
		Content:  text,
		Metadata: strMeta,
	})
}

// Search returns the top-K documents most similar to query.
func (s *ChromemStore) Search(ctx context.Context, query string, topK int) ([]Result, error) {
	count := s.collection.Count()
	if count == 0 {
		return nil, nil
	}
	if topK > count {
		topK = count
	}
	hits, err := s.collection.Query(ctx, query, topK, nil, nil)
	if err != nil {
		return nil, err
	}
	out := make([]Result, 0, len(hits))
	for _, h := range hits {
		meta := make(map[string]any, len(h.Metadata))
		for k, v := range h.Metadata {
			meta[k] = v
		}
		out = append(out, Result{
			ID:    h.ID,
			Text:  h.Content,
			Score: h.Similarity,
			Meta:  meta,
		})
	}
	return out, nil
}
