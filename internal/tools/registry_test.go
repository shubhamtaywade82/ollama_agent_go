package tools

import (
	"context"
	"testing"
)

func TestDefaultRegistry(t *testing.T) {
	r := Default(t.TempDir())
	want := []string{"edit_file", "grep_search", "ls", "read_file", "run_shell", "write_file"}
	got := r.Names()
	if len(got) != len(want) {
		t.Fatalf("names = %v, want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("names[%d] = %q, want %q", i, got[i], want[i])
		}
	}
}

func TestRegistrySpecs(t *testing.T) {
	r := Default(t.TempDir())
	specs := r.Specs()
	if len(specs) != 6 {
		t.Fatalf("specs = %d, want 6", len(specs))
	}
	for _, s := range specs {
		if s.Type != "function" {
			t.Errorf("spec type = %q", s.Type)
		}
		if s.Function.Name == "" || s.Function.Description == "" {
			t.Errorf("spec missing name/desc: %+v", s.Function)
		}
		if s.Function.Parameters["type"] != "object" {
			t.Errorf("spec %s params not object schema", s.Function.Name)
		}
	}
}

func TestRegistryUnknownTool(t *testing.T) {
	r := Default(t.TempDir())
	if _, err := r.Execute(context.Background(), "nope", nil); err == nil {
		t.Error("unknown tool should error")
	}
}
