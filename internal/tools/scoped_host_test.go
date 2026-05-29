package tools_test

import (
	"context"
	"testing"

	"ollama_agent_go/internal/observability"
	"ollama_agent_go/internal/policy"
	"ollama_agent_go/internal/tools"
)

func newTestHost(t *testing.T) *tools.Host {
	t.Helper()
	reg := tools.NewRegistry("")
	reg.Register(staticTool("allowed_tool"))
	reg.Register(staticTool("blocked_tool"))
	pol := policy.NewDefaultEngine("", nil, true) // auto-approve
	host := tools.NewHost(reg, pol, observability.Discard)
	return host
}

// staticTool is a simple tool that returns its name as output.
type staticTool string

func (s staticTool) Name() string        { return string(s) }
func (s staticTool) Description() string { return "static tool " + string(s) }
func (s staticTool) Parameters() map[string]any {
	return map[string]any{"type": "object", "properties": map[string]any{}}
}
func (s staticTool) Execute(_ context.Context, _ map[string]any) (string, error) {
	return "output:" + string(s), nil
}

func TestScopedHostBlocksDisallowedTool(t *testing.T) {
	host := newTestHost(t)
	scoped := tools.NewScopedHost(host, []string{"allowed_tool"})

	_, err := scoped.Execute(context.Background(), "blocked_tool", nil)
	if err == nil {
		t.Fatal("expected error when calling blocked tool, got nil")
	}
}

func TestScopedHostAllowsAllIfNilList(t *testing.T) {
	host := newTestHost(t)
	scoped := tools.NewScopedHost(host, nil) // nil = unrestricted

	out, err := scoped.Execute(context.Background(), "blocked_tool", nil)
	if err != nil {
		t.Fatalf("expected no error with nil allowed list, got: %v", err)
	}
	if out != "output:blocked_tool" {
		t.Errorf("unexpected output: %q", out)
	}
}

func TestScopedHostSpecsFilteredToAllowed(t *testing.T) {
	host := newTestHost(t)
	scoped := tools.NewScopedHost(host, []string{"allowed_tool"})

	specs := scoped.Specs()
	if len(specs) != 1 {
		t.Fatalf("expected 1 spec, got %d", len(specs))
	}
	if specs[0].Function.Name != "allowed_tool" {
		t.Errorf("unexpected spec name: %q", specs[0].Function.Name)
	}
}

func TestScopedHostSpecsAllIfNilList(t *testing.T) {
	host := newTestHost(t)
	scoped := tools.NewScopedHost(host, nil)

	specs := scoped.Specs()
	if len(specs) < 2 {
		t.Fatalf("expected at least 2 specs, got %d", len(specs))
	}
}
