# Phase 08 — RAG / Knowledge Retrieval

Tags: `#rag` `#vector` `#embeddings` `#retrieval` `#knowledge` `#docs` `#indexer` `#sqlite-vec` `#p2` `#status/planned`

Prerequisites: Phase 02 (memory.longterm stub), Phase 01 (tools.Host for indexer tool).

---

## Goal

Give the agent access to a local knowledge base: index documents/code/markdown into a
vector store, retrieve semantically similar chunks at query time, inject them into the
system prompt or as tool results. Uses `sqlite-vec` (pure Go, no Python dependency).

---

## Pipeline

```
Documents (markdown, code, PDFs)
  └── Chunker → chunks []string
        └── Embedder → [][]float32
              └── VectorStore (sqlite-vec) → persisted
                        ↑
             Query → Embedder → vector → ANN search → top-K chunks
                                                     → inject into prompt
```

---

## Files to create (in order)

### Step 1 — sqlite-vec dependency
**`go.mod`** — add:

```
require github.com/asg017/sqlite-vec-go-bindings v0.1.6
```

Or use `chromem-go` (pure Go in-process vector DB, zero CGo):

```
require github.com/philippgille/chromem-go v0.7.0
```

Prefer `chromem-go` for zero-dependency setup. Can swap backend later.

Tags: `#rag/dependency`

---

### Step 2 — Embedder interface
**`internal/indexer/embedder.go`**

```go
package indexer

type Embedder interface {
    Embed(ctx context.Context, texts []string) ([][]float32, error)
    Dims() int
}

// OllamaEmbedder calls /api/embed on local Ollama instance.
type OllamaEmbedder struct {
    baseURL string
    model   string    // e.g. "nomic-embed-text"
    client  *http.Client
}

func NewOllamaEmbedder(baseURL, model string) *OllamaEmbedder
func (e *OllamaEmbedder) Embed(ctx context.Context, texts []string) ([][]float32, error)
func (e *OllamaEmbedder) Dims() int  // 768 for nomic-embed-text
```

Tags: `#rag/embedder`

---

### Step 3 — Text chunker
**`internal/indexer/chunker.go`**

```go
type ChunkConfig struct {
    Size    int  // tokens per chunk (default 256)
    Overlap int  // token overlap between chunks (default 32)
}

type Chunk struct {
    DocID    string
    Index    int
    Text     string
    Metadata map[string]any
}

// Chunk splits text into overlapping chunks by token count.
func Chunk(docID, text string, cfg ChunkConfig) []Chunk

// ChunkFile reads a file and chunks it, setting metadata.path and metadata.ext.
func ChunkFile(root, relPath string, cfg ChunkConfig) ([]Chunk, error)
```

Tags: `#rag/chunker`

---

### Step 4 — Vector store
**`internal/memory/longterm/chromem.go`**

Implements `longterm.Store` using `chromem-go`:

```go
package longterm

import "github.com/philippgille/chromem-go"

type ChromemStore struct {
    db         *chromem.DB
    collection *chromem.Collection
    embedder   indexer.Embedder
}

func NewChromemStore(path string, embedder indexer.Embedder) (*ChromemStore, error)

func (s *ChromemStore) Insert(ctx context.Context, id, text string, meta map[string]any) error
func (s *ChromemStore) Search(ctx context.Context, query string, topK int) ([]Result, error)
```

Tags: `#rag/vectorstore` `#memory/longterm`

---

### Step 5 — Indexer
**`internal/indexer/indexer.go`**

```go
type Indexer struct {
    store    longterm.Store
    embedder Embedder
    chunks   ChunkConfig
    root     string
}

func New(store longterm.Store, embedder Embedder, root string) *Indexer

// IndexFile chunks and embeds one file, upserts into vector store.
func (idx *Indexer) IndexFile(ctx context.Context, relPath string) error

// IndexDir walks a directory and indexes all supported files.
func (idx *Indexer) IndexDir(ctx context.Context, dir string, exts []string) error

// supported extensions default: .md .txt .go .py .ts .js .yaml .json
```

Tags: `#rag/indexer`

---

### Step 6 — Retriever
**`internal/retriever/retriever.go`**

```go
package retriever

type Retriever struct {
    store longterm.Store
    topK  int
}

func New(store longterm.Store, topK int) *Retriever

// Retrieve returns top-K chunks relevant to query.
func (r *Retriever) Retrieve(ctx context.Context, query string) ([]longterm.Result, error)

// InjectContext formats retrieved chunks as a context block to prepend to messages.
func (r *Retriever) InjectContext(chunks []longterm.Result) string
```

Tags: `#rag/retriever`

---

### Step 7 — RAG tool
**`internal/tools/rag.go`**

Register a `search_knowledge` tool in `tools.Default()`:

```go
// search_knowledge searches the indexed knowledge base.
// args: {"query": "...", "top_k": 5}
// returns: formatted top-K chunks with source metadata
```

This lets the agent call `search_knowledge` just like any other tool, giving it
on-demand access to the knowledge base within the think-act loop.

Tags: `#rag/tool` `#tools/registry`

---

### Step 8 — Automatic context injection
**`internal/runtime/engine.go`** — optional pre-retrieval before each `Submit()`:

```go
if e.Retriever != nil {
    chunks, _ := e.Retriever.Retrieve(ctx, input)
    if len(chunks) > 0 {
        e.Memory.Short().Append(types.Message{
            Role: "system",
            Content: e.Retriever.InjectContext(chunks),
        })
    }
}
```

Add `Retriever *retriever.Retriever` field to `Engine` (optional, nil disables).

Tags: `#rag/auto-inject` `#runtime/engine`

---

### Step 9 — `/index` slash command
**`internal/ui/tui/model.go`** — add `/index <path>` handler:

```go
case "index":
    go func() {
        err := e.engine.Indexer.IndexDir(ctx, arg, nil)
        // emit status message
    }()
```

Tags: `#ui/tui` `#rag/cli`

---

### Step 10 — Config
**`internal/config/config.go`** — add:

```go
type RAGConfig struct {
    Enabled      bool
    EmbedModel   string   // default "nomic-embed-text"
    ChunkSize    int      // default 256
    ChunkOverlap int      // default 32
    TopK         int      // default 5
    StorePath    string   // default <root>/knowledge.db
}
```

Tags: `#rag/config`

---

## Tests

**`internal/indexer/chunker_test.go`**
- `TestChunkSplit` — text longer than chunk size yields multiple chunks with overlap
- `TestChunkSingleChunk` — short text produces exactly one chunk

**`internal/indexer/indexer_test.go`** (integration, requires Ollama embed)
- Skip with `t.Skip` if `OLLAMA_TEST_EMBED` env not set
- `TestIndexFileAndSearch` — index a .md file, search returns it in top-3

**`internal/retriever/retriever_test.go`**
- `TestInjectContext` — formatted output contains source path and text

Tags: `#tests`

---

## Verification

```
go test ./internal/indexer/... ./internal/retriever/... ./internal/memory/longterm/...
```

- `/index ./docs` in TUI → all .md files chunked, embedded, stored
- Ask "what does Phase 03 cover?" → agent retrieves saga doc, answers correctly
- `search_knowledge` tool callable by agent with natural language query
