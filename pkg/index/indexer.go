package index

import (
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"strings"
)

// Symbol represents a code definition (function, type, etc.).
type Symbol struct {
	Name string
	Kind string // "func", "struct", "interface", "type"
	Line int
	Path string
}

// Indexer scans a directory for source files and extracts symbols.
type Indexer struct {
	Root    string
	Symbols []Symbol
}

func NewIndexer(root string) *Indexer {
	return &Indexer{Root: root}
}

// Scan crawls the root directory and indexes supported file types.
func (idx *Indexer) Scan() error {
	idx.Symbols = nil
	return filepath.Walk(idx.Root, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return nil
		}
		if info.IsDir() {
			if skipDir(info.Name()) {
				return filepath.SkipDir
			}
			return nil
		}

		ext := filepath.Ext(path)
		switch ext {
		case ".go":
			return idx.indexGo(path)
		}
		return nil
	})
}

func (idx *Indexer) indexGo(path string) error {
	fset := token.NewFileSet()
	f, err := parser.ParseFile(fset, path, nil, parser.ParseComments)
	if err != nil {
		return nil // skip unparseable files
	}

	rel, _ := filepath.Rel(idx.Root, path)

	ast.Inspect(f, func(n ast.Node) bool {
		switch x := n.(type) {
		case *ast.FuncDecl:
			idx.Symbols = append(idx.Symbols, Symbol{
				Name: x.Name.Name,
				Kind: "func",
				Line: fset.Position(x.Pos()).Line,
				Path: rel,
			})
		case *ast.TypeSpec:
			kind := "type"
			switch x.Type.(type) {
			case *ast.StructType:
				kind = "struct"
			case *ast.InterfaceType:
				kind = "interface"
			}
			idx.Symbols = append(idx.Symbols, Symbol{
				Name: x.Name.Name,
				Kind: kind,
				Line: fset.Position(x.Pos()).Line,
				Path: rel,
			})
		}
		return true
	})
	return nil
}

func (idx *Indexer) Search(query string) []Symbol {
	var matches []Symbol
	query = strings.ToLower(query)
	for _, s := range idx.Symbols {
		if strings.Contains(strings.ToLower(s.Name), query) {
			matches = append(matches, s)
		}
	}
	return matches
}

func skipDir(name string) bool {
	switch name {
	case ".git", "node_modules", "vendor", ".cache", "bin", "dist":
		return true
	}
	return false
}

func (s Symbol) String() string {
	return fmt.Sprintf("%s (%s) at %s:%d", s.Name, s.Kind, s.Path, s.Line)
}
