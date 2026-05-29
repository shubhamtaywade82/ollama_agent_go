package kernel

import (
	"os"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"
)

// Ownership defines which roles can access which paths.
type Ownership struct {
	Restricted []string `yaml:"restricted"`
}

// Gate enforces security policies on file access.
type Gate struct {
	Root      string
	Ownership Ownership
}

func NewGate(root string) *Gate {
	g := &Gate{Root: root}
	g.Load()
	return g
}

func (g *Gate) Load() {
	path := filepath.Join(g.Root, "owners.yml")
	b, err := os.ReadFile(path)
	if err != nil {
		return
	}
	yaml.Unmarshal(b, &g.Ownership)
}

// IsAllowed returns true if the path is not restricted.
func (g *Gate) IsAllowed(path string) bool {
	rel, err := filepath.Rel(g.Root, filepath.Join(g.Root, path))
	if err != nil {
		return false
	}

	for _, r := range g.Ownership.Restricted {
		if strings.HasPrefix(rel, r) || rel == r {
			return false
		}
	}
	return true
}
