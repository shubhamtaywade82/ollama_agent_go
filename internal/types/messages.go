// Package types defines the neutral request/response types shared across
// internal packages. These mirror the Ollama/OpenAI chat schema and serve as
// the lingua franca between providers, the agent, and storage layers.
package types

// Message is one turn in a conversation. Role is one of "system", "user",
// "assistant", or "tool".
type Message struct {
	Role      string     `json:"role"`
	Content   string     `json:"content"`
	ToolCalls []ToolCall `json:"tool_calls,omitempty"`
	// ToolName labels a "tool" role message with the tool it answers.
	// Ollama expects the key "name" (not "tool_name") in tool-role messages.
	ToolName string `json:"name,omitempty"`
	// ToolCallID links a "tool" role message to the originating call (OpenAI-style).
	ToolCallID string `json:"tool_call_id,omitempty"`
}

// ToolCall is a function-call request emitted by the model.
type ToolCall struct {
	ID       string           `json:"id,omitempty"`
	Function ToolCallFunction `json:"function"`
}

// ToolCallFunction holds the called function's name and decoded arguments.
type ToolCallFunction struct {
	Name      string         `json:"name"`
	Arguments map[string]any `json:"arguments"`
}

// ChatRequest is the neutral request type used by all providers.
type ChatRequest struct {
	Model    string         `json:"model"`
	Messages []Message      `json:"messages"`
	Stream   bool           `json:"stream"`
	Tools    []any          `json:"tools,omitempty"`
	Options  map[string]any `json:"options,omitempty"`
	// ModelHint is an optional routing hint set by the caller (e.g. "/model gpt-4").
	// The router uses it to prefer a matching provider without overriding task-type
	// selection when no matching provider is found.
	ModelHint string `json:"-"`
}

// ChatResponse is the neutral response type returned by all providers.
type ChatResponse struct {
	Model            string  `json:"model"`
	Message          Message `json:"message"`
	Done             bool    `json:"done"`
	TotalDuration    int64   `json:"total_duration,omitempty"`
	PromptTokens     int     `json:"prompt_eval_count,omitempty"`
	CompletionTokens int     `json:"eval_count,omitempty"`
}

// ModelInfo holds metadata for one local model.
type ModelInfo struct {
	Name       string `json:"name"`
	ModifiedAt string `json:"modified_at"`
	Size       int64  `json:"size"`
	Details    struct {
		Family            string   `json:"family"`
		ParameterSize     string   `json:"parameter_size"`
		QuantizationLevel string   `json:"quantization_level"`
		Families          []string `json:"families"`
	} `json:"details"`
}
