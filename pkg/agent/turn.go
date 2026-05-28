package agent

import (
	"encoding/json"
	"fmt"
	"regexp"
	"strings"

	"ollama_agent_go/pkg/ollama"
	"ollama_agent_go/pkg/tools"
)

// fencedBlock matches a fenced code block, optionally tagged (```json, ```tool_call).
var fencedBlock = regexp.MustCompile("(?s)```(?:[a-zA-Z_]*)\\n(.*?)```")

// syntheticCall is the permissive shape we accept from a code block. Different
// models use different key names, so we accept several aliases.
type syntheticCall struct {
	Name      string         `json:"name"`
	Tool      string         `json:"tool"`
	Action    string         `json:"action"`
	Arguments map[string]any `json:"arguments"`
	Args      map[string]any `json:"args"`
	Params    map[string]any `json:"parameters"`
	Input     map[string]any `json:"input"`
}

func (s syntheticCall) toToolCall() (ollama.ToolCall, bool) {
	name := firstNonEmpty(s.Name, s.Tool, s.Action)
	if name == "" {
		return ollama.ToolCall{}, false
	}
	args := firstNonNil(s.Arguments, s.Args, s.Params, s.Input)
	if args == nil {
		args = map[string]any{}
	}
	return ollama.ToolCall{Function: ollama.ToolCallFunction{Name: name, Arguments: args}}, true
}

// parseSyntheticCalls scans content for fenced JSON code blocks that describe
// tool calls (used by models without native tool support, e.g. Gemma/Llama 3).
// It returns the parsed calls and the content with those blocks removed.
func parseSyntheticCalls(content string) ([]ollama.ToolCall, string) {
	matches := fencedBlock.FindAllStringSubmatchIndex(content, -1)
	if len(matches) == 0 {
		return nil, content
	}
	var calls []ollama.ToolCall
	var consumed []int // [start,end] pairs of blocks we used
	for _, m := range matches {
		body := content[m[2]:m[3]]
		parsed := parseCallBody(body)
		if len(parsed) > 0 {
			calls = append(calls, parsed...)
			consumed = append(consumed, m[0], m[1])
		}
	}
	if len(calls) == 0 {
		return nil, content
	}
	// Remove consumed blocks from the content (back to front to keep indices).
	rest := content
	for i := len(consumed) - 2; i >= 0; i -= 2 {
		rest = rest[:consumed[i]] + rest[consumed[i+1]:]
	}
	return calls, strings.TrimSpace(rest)
}

// parseCallBody decodes a single code block body, accepting either an object or
// an array of objects.
func parseCallBody(body string) []ollama.ToolCall {
	body = strings.TrimSpace(body)
	if body == "" {
		return nil
	}
	// Try array form first.
	if strings.HasPrefix(body, "[") {
		var arr []syntheticCall
		if err := json.Unmarshal([]byte(body), &arr); err == nil {
			var calls []ollama.ToolCall
			for _, sc := range arr {
				if tc, ok := sc.toToolCall(); ok {
					calls = append(calls, tc)
				}
			}
			return calls
		}
		return nil
	}
	var sc syntheticCall
	if err := json.Unmarshal([]byte(body), &sc); err != nil {
		return nil
	}
	if tc, ok := sc.toToolCall(); ok {
		return []ollama.ToolCall{tc}
	}
	return nil
}

func firstNonEmpty(vals ...string) string {
	for _, v := range vals {
		if v != "" {
			return v
		}
	}
	return ""
}

func firstNonNil(vals ...map[string]any) map[string]any {
	for _, v := range vals {
		if v != nil {
			return v
		}
	}
	return nil
}

// SystemPrompt builds the agent's system instructions, listing the available
// tools and the synthetic call format as a fallback for models without native
// tool calling.
func SystemPrompt(registry *tools.Registry) string {
	var b strings.Builder
	b.WriteString("You are a precise terminal coding agent operating inside a sandboxed project directory. ")
	b.WriteString("Use the provided tools to inspect and modify files; do not guess file contents. ")
	b.WriteString("Prefer edit_file (unified diff) for surgical changes. When the task is complete, reply with a concise final answer and no tool call.\n\n")
	b.WriteString("Available tools:\n")
	for _, name := range registry.Names() {
		if t, ok := registry.Get(name); ok {
			fmt.Fprintf(&b, "- %s: %s\n", t.Name(), t.Description())
		}
	}
	b.WriteString("\nIf you cannot emit native tool calls, request a tool by writing a single JSON object in a fenced code block, e.g.:\n")
	b.WriteString("```json\n{\"name\": \"read_file\", \"arguments\": {\"path\": \"main.go\"}}\n```\n")
	return b.String()
}
