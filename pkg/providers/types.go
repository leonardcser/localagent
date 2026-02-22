package providers

import (
	"context"
	"encoding/json"
)

type ToolCall struct {
	ID        string         `json:"id"`
	Type      string         `json:"type,omitempty"`
	Function  *FunctionCall  `json:"function,omitempty"`
	Name      string         `json:"name,omitempty"`
	Arguments map[string]any `json:"arguments,omitempty"`
}

type FunctionCall struct {
	Name      string `json:"name"`
	Arguments string `json:"arguments"`
}

type LLMResponse struct {
	Content      string     `json:"content"`
	ToolCalls    []ToolCall `json:"tool_calls,omitempty"`
	FinishReason string     `json:"finish_reason"`
	Usage        *UsageInfo `json:"usage,omitempty"`
}

type UsageInfo struct {
	PromptTokens     int `json:"prompt_tokens"`
	CompletionTokens int `json:"completion_tokens"`
	TotalTokens      int `json:"total_tokens"`
}

// ContentPart represents a part of a multimodal message content (OpenAI format).
type ContentPart struct {
	Type     string    `json:"type"`
	Text     string    `json:"text,omitempty"`
	ImageURL *ImageURL `json:"image_url,omitempty"`
}

// ImageURL holds an image reference for multimodal messages.
type ImageURL struct {
	URL    string `json:"url"`
	Detail string `json:"detail,omitempty"`
}

// Message represents a chat message. When ContentParts is set, the content
// field is serialized as an array of content parts (for multimodal messages).
// Otherwise it is serialized as a plain string.
type Message struct {
	Role         string        `json:"-"`
	Content      string        `json:"-"`
	ContentParts []ContentPart `json:"-"`
	ToolCalls    []ToolCall    `json:"-"`
	ToolCallID   string        `json:"-"`
	ToolName     string        `json:"-"` // name of the tool that produced this result (role=tool only)
}

func (m Message) MarshalJSON() ([]byte, error) {
	type alias struct {
		Role       string     `json:"role"`
		Content    any        `json:"content"`
		ToolCalls  []ToolCall `json:"tool_calls,omitempty"`
		ToolCallID string     `json:"tool_call_id,omitempty"`
	}
	a := alias{
		Role:       m.Role,
		ToolCalls:  m.ToolCalls,
		ToolCallID: m.ToolCallID,
	}
	if len(m.ContentParts) > 0 {
		a.Content = m.ContentParts
	} else {
		a.Content = m.Content
	}
	return json.Marshal(a)
}

func (m *Message) UnmarshalJSON(data []byte) error {
	type alias struct {
		Role       string          `json:"role"`
		Content    json.RawMessage `json:"content"`
		ToolCalls  []ToolCall      `json:"tool_calls,omitempty"`
		ToolCallID string          `json:"tool_call_id,omitempty"`
	}
	var a alias
	if err := json.Unmarshal(data, &a); err != nil {
		return err
	}
	m.Role = a.Role
	m.ToolCalls = a.ToolCalls
	m.ToolCallID = a.ToolCallID

	if len(a.Content) == 0 {
		return nil
	}

	// Try string first (common case)
	var s string
	if err := json.Unmarshal(a.Content, &s); err == nil {
		m.Content = s
		return nil
	}

	// Try array of content parts (multimodal)
	var parts []ContentPart
	if err := json.Unmarshal(a.Content, &parts); err == nil {
		m.ContentParts = parts
		for _, p := range parts {
			if p.Type == "text" {
				m.Content = p.Text
				break
			}
		}
	}

	return nil
}

type LLMProvider interface {
	Chat(ctx context.Context, messages []Message, tools []ToolDefinition, model string, options map[string]any) (*LLMResponse, error)
	GetDefaultModel() string
}

type ToolDefinition struct {
	Type     string                 `json:"type"`
	Function ToolFunctionDefinition `json:"function"`
}

type ToolFunctionDefinition struct {
	Name        string         `json:"name"`
	Description string         `json:"description"`
	Parameters  map[string]any `json:"parameters"`
}
