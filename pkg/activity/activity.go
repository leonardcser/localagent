package activity

import "time"

type EventType string

const (
	LLMTurn  EventType = "llm_turn"
	LLMError EventType = "llm_error"
	ToolExec EventType = "tool_exec"
	Complete EventType = "complete"
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
