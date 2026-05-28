package agent

import (
	"testing"

	"ollama_agent_go/pkg/tools"
)

func TestParseSyntheticCallsObject(t *testing.T) {
	content := "I'll read it.\n```json\n{\"name\": \"read_file\", \"arguments\": {\"path\": \"main.go\"}}\n```\nDone."
	calls, rest := parseSyntheticCalls(content)
	if len(calls) != 1 {
		t.Fatalf("calls = %d, want 1", len(calls))
	}
	if calls[0].Function.Name != "read_file" {
		t.Errorf("name = %q", calls[0].Function.Name)
	}
	if calls[0].Function.Arguments["path"] != "main.go" {
		t.Errorf("args = %v", calls[0].Function.Arguments)
	}
	if rest == content {
		t.Error("block should be stripped from rest")
	}
}

func TestParseSyntheticCallsAliases(t *testing.T) {
	// tool + parameters aliases, untagged fence.
	content := "```\n{\"tool\": \"ls\", \"parameters\": {\"path\": \".\"}}\n```"
	calls, _ := parseSyntheticCalls(content)
	if len(calls) != 1 || calls[0].Function.Name != "ls" {
		t.Fatalf("calls = %+v", calls)
	}
	if calls[0].Function.Arguments["path"] != "." {
		t.Errorf("args = %v", calls[0].Function.Arguments)
	}
}

func TestParseSyntheticCallsArray(t *testing.T) {
	content := "```json\n[{\"name\":\"ls\",\"arguments\":{}},{\"name\":\"read_file\",\"arguments\":{\"path\":\"x\"}}]\n```"
	calls, _ := parseSyntheticCalls(content)
	if len(calls) != 2 {
		t.Fatalf("calls = %d, want 2", len(calls))
	}
}

func TestParseSyntheticCallsNone(t *testing.T) {
	content := "Just a normal answer with a ```\ncode block\n``` that is not a tool call."
	calls, rest := parseSyntheticCalls(content)
	if len(calls) != 0 {
		t.Errorf("expected no calls, got %d", len(calls))
	}
	if rest != content {
		t.Error("rest should be unchanged when no calls found")
	}
}

func TestSystemPromptListsTools(t *testing.T) {
	r := tools.Default(t.TempDir())
	p := SystemPrompt(r)
	for _, name := range []string{"read_file", "write_file", "edit_file", "grep_search", "ls", "run_shell"} {
		if !contains(p, name) {
			t.Errorf("system prompt missing tool %q", name)
		}
	}
}

func contains(s, sub string) bool {
	return len(s) >= len(sub) && (indexOf(s, sub) >= 0)
}

func indexOf(s, sub string) int {
	for i := 0; i+len(sub) <= len(s); i++ {
		if s[i:i+len(sub)] == sub {
			return i
		}
	}
	return -1
}
