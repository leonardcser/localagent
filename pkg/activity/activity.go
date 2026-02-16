package activity

import "time"

type EventType string

const (
	ProcessingStart EventType = "processing_start"
	LLMRequest      EventType = "llm_request"
	LLMResponse     EventType = "llm_response"
	LLMError        EventType = "llm_error"
	ToolCall        EventType = "tool_call"
	ToolResult      EventType = "tool_result"
	Complete        EventType = "complete"
)

type Event struct {
	Type      EventType      `json:"type"`
	Timestamp time.Time      `json:"timestamp"`
	Message   string         `json:"message"`
	Detail    map[string]any `json:"detail,omitempty"`
}

type Emitter interface {
	Emit(Event)
}

type NopEmitter struct{}

func (NopEmitter) Emit(Event) {}
