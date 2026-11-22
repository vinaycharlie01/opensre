// Package conversation holds the provider-neutral transcript types the agent
// loop exchanges with an LLM port.
package conversation

import "encoding/json"

// Role is the author of a message.
type Role string

// Roles understood by every LLM adapter.
const (
	RoleUser      Role = "user"
	RoleAssistant Role = "assistant"
)

// ToolCall is a model request to run one tool.
type ToolCall struct {
	ID    string          `json:"id"`
	Name  string          `json:"name"`
	Input json.RawMessage `json:"input"`
}

// ToolResult is the observation returned to the model for one ToolCall.
type ToolResult struct {
	CallID  string `json:"call_id"`
	Content string `json:"content"`
	IsError bool   `json:"is_error,omitempty"`
}

// Message is one transcript entry. An assistant message may carry text and
// tool calls; a user message carries text or the tool results of the previous
// assistant turn (all results of a batch travel in a single message).
type Message struct {
	Role        Role         `json:"role"`
	Text        string       `json:"text,omitempty"`
	ToolCalls   []ToolCall   `json:"tool_calls,omitempty"`
	ToolResults []ToolResult `json:"tool_results,omitempty"`
	// Native is an opaque, provider-specific copy of an assistant turn (for
	// example thinking blocks that must be replayed unchanged). Only the LLM
	// adapter that produced it reads it; the core never inspects it.
	Native any `json:"-"`
}

// UserText builds a plain user message.
func UserText(text string) Message { return Message{Role: RoleUser, Text: text} }

// Size is a rough character count used for context budgeting.
func (m Message) Size() int {
	n := len(m.Text)
	for _, c := range m.ToolCalls {
		n += len(c.Name) + len(c.Input)
	}
	for _, r := range m.ToolResults {
		n += len(r.Content)
	}
	return n
}

// StopReason says why a model turn ended.
type StopReason string

// Stop reasons normalized across providers.
const (
	StopEndTurn   StopReason = "end_turn"
	StopToolUse   StopReason = "tool_use"
	StopMaxTokens StopReason = "max_tokens"
	StopRefusal   StopReason = "refusal"
	StopOther     StopReason = "other"
)

// Usage is token accounting for one model call.
type Usage struct {
	InputTokens     int `json:"input_tokens"`
	OutputTokens    int `json:"output_tokens"`
	CacheReadTokens int `json:"cache_read_tokens"`
}

// Add accumulates another call's usage.
func (u *Usage) Add(o Usage) {
	u.InputTokens += o.InputTokens
	u.OutputTokens += o.OutputTokens
	u.CacheReadTokens += o.CacheReadTokens
}
