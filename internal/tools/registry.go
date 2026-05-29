// Package tools provides the agent's tool registry and the built-in tools the
// model can call (file IO, search, shell, diff-based editing). All file access
// is sandboxed to a configured root directory.
package tools

import (
	"context"
	"fmt"
	"sort"
)

// Tool is a single capability the agent can invoke.
type Tool interface {
	Name() string
	Description() string
	Parameters() map[string]any
	Execute(ctx context.Context, args map[string]any) (string, error)
}

// Spec is the wire representation of a tool advertised to the model.
type Spec struct {
	Type     string       `json:"type"`
	Function FunctionSpec `json:"function"`
}

// FunctionSpec describes a callable function tool.
type FunctionSpec struct {
	Name        string         `json:"name"`
	Description string         `json:"description"`
	Parameters  map[string]any `json:"parameters"`
}

// Registry holds the set of available tools sandboxed to a root directory.
type Registry struct {
	root  string
	tools map[string]Tool
}

// NewRegistry creates an empty registry sandboxed to root.
func NewRegistry(root string) *Registry {
	return &Registry{root: root, tools: make(map[string]Tool)}
}

// Root returns the sandbox root directory.
func (r *Registry) Root() string { return r.root }

// Register adds a tool, replacing any existing tool with the same name.
func (r *Registry) Register(t Tool) { r.tools[t.Name()] = t }

// Get returns a tool by name.
func (r *Registry) Get(name string) (Tool, bool) {
	t, ok := r.tools[name]
	return t, ok
}

// Names returns the registered tool names in sorted order.
func (r *Registry) Names() []string {
	names := make([]string, 0, len(r.tools))
	for n := range r.tools {
		names = append(names, n)
	}
	sort.Strings(names)
	return names
}

// Specs returns the tool specs advertised to the model, in stable name order.
func (r *Registry) Specs() []Spec {
	specs := make([]Spec, 0, len(r.tools))
	for _, n := range r.Names() {
		t := r.tools[n]
		specs = append(specs, Spec{
			Type: "function",
			Function: FunctionSpec{
				Name:        t.Name(),
				Description: t.Description(),
				Parameters:  t.Parameters(),
			},
		})
	}
	return specs
}

// Execute looks up name and runs it with args.
func (r *Registry) Execute(ctx context.Context, name string, args map[string]any) (string, error) {
	t, ok := r.tools[name]
	if !ok {
		return "", fmt.Errorf("unknown tool %q", name)
	}
	return t.Execute(ctx, args)
}

// Default returns a registry with all built-in tools registered, sandboxed to root.
func Default(root string) *Registry {
	r := NewRegistry(root)
	r.Register(&ReadFile{Root: root})
	r.Register(&WriteFile{Root: root})
	r.Register(&LS{Root: root})
	r.Register(&GrepSearch{Root: root})
	r.Register(&RunShell{Root: root})
	r.Register(&EditFile{Root: root})
	return r
}
