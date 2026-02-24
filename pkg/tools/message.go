package tools

import (
	"context"
	"fmt"

	"localagent/pkg/bus"
	"localagent/pkg/session"
)

type MessageTool struct {
	bus            *bus.MessageBus
	sessions       *session.SessionManager
	defaultChannel string
	defaultChatID  string
	called         bool
}

func NewMessageTool(msgBus *bus.MessageBus, sessions *session.SessionManager) *MessageTool {
	return &MessageTool{bus: msgBus, sessions: sessions}
}

func (t *MessageTool) Name() string {
	return "message"
}

func (t *MessageTool) Description() string {
	return "Send a message to the user. Use this when you want to communicate something."
}

func (t *MessageTool) Parameters() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"content": map[string]any{
				"type":        "string",
				"description": "The message content to send",
			},
		},
		"required": []string{"content"},
	}
}

func (t *MessageTool) SetContext(channel, chatID string) {
	t.defaultChannel = channel
	t.defaultChatID = chatID
	t.called = false
}

func (t *MessageTool) WasCalled() bool {
	return t.called
}

func (t *MessageTool) Execute(ctx context.Context, args map[string]any) *ToolResult {
	content, ok := args["content"].(string)
	if !ok {
		return &ToolResult{ForLLM: "content is required", IsError: true}
	}

	channel := t.defaultChannel
	chatID := t.defaultChatID

	if channel == "" || chatID == "" {
		return &ToolResult{ForLLM: "No target channel/chat specified", IsError: true}
	}

	t.bus.PublishOutbound(bus.OutboundMessage{
		Channel: channel,
		ChatID:  chatID,
		Content: content,
	})

	if t.sessions != nil {
		sessionKey := fmt.Sprintf("%s:%s", channel, chatID)
		t.sessions.AddMessage(sessionKey, "assistant", content)
	}

	t.called = true

	return &ToolResult{
		ForLLM: content,
		Silent: true,
	}
}
