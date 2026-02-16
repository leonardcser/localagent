package tools

import (
	"context"
	"testing"

	"localagent/pkg/bus"
)

func TestMessageTool_Execute_Success(t *testing.T) {
	msgBus := bus.NewMessageBus()
	tool := NewMessageTool(msgBus)
	tool.SetContext("web", "default")

	ctx := context.Background()
	args := map[string]any{
		"content": "Hello, world!",
	}

	result := tool.Execute(ctx, args)

	if !result.Silent {
		t.Error("Expected Silent=true for successful send")
	}
	if result.ForLLM != "Message sent to web:default" {
		t.Errorf("Expected ForLLM 'Message sent to web:default', got '%s'", result.ForLLM)
	}
	if result.ForUser != "" {
		t.Errorf("Expected ForUser to be empty, got '%s'", result.ForUser)
	}
	if result.IsError {
		t.Error("Expected IsError=false for successful send")
	}

	// Verify message was published to bus
	outMsg, ok := msgBus.SubscribeOutbound(ctx)
	if !ok {
		t.Fatal("Expected outbound message on bus")
	}
	if outMsg.Channel != "web" {
		t.Errorf("Expected channel 'web', got '%s'", outMsg.Channel)
	}
	if outMsg.ChatID != "default" {
		t.Errorf("Expected chatID 'default', got '%s'", outMsg.ChatID)
	}
	if outMsg.Content != "Hello, world!" {
		t.Errorf("Expected content 'Hello, world!', got '%s'", outMsg.Content)
	}
}

func TestMessageTool_Execute_MissingContent(t *testing.T) {
	msgBus := bus.NewMessageBus()
	tool := NewMessageTool(msgBus)
	tool.SetContext("web", "default")

	ctx := context.Background()
	args := map[string]any{}

	result := tool.Execute(ctx, args)

	if !result.IsError {
		t.Error("Expected IsError=true for missing content")
	}
	if result.ForLLM != "content is required" {
		t.Errorf("Expected ForLLM 'content is required', got '%s'", result.ForLLM)
	}
}

func TestMessageTool_Execute_NoContext(t *testing.T) {
	msgBus := bus.NewMessageBus()
	tool := NewMessageTool(msgBus)

	ctx := context.Background()
	args := map[string]any{
		"content": "Test message",
	}

	result := tool.Execute(ctx, args)

	if !result.IsError {
		t.Error("Expected IsError=true when no context set")
	}
	if result.ForLLM != "No target channel/chat specified" {
		t.Errorf("Expected ForLLM 'No target channel/chat specified', got '%s'", result.ForLLM)
	}
}

func TestMessageTool_HasSentInRound(t *testing.T) {
	msgBus := bus.NewMessageBus()
	tool := NewMessageTool(msgBus)
	tool.SetContext("web", "default")

	if tool.HasSentInRound() {
		t.Error("Expected HasSentInRound=false before sending")
	}

	ctx := context.Background()
	tool.Execute(ctx, map[string]any{"content": "test"})

	if !tool.HasSentInRound() {
		t.Error("Expected HasSentInRound=true after sending")
	}

	// SetContext resets the flag
	tool.SetContext("web", "default")
	if tool.HasSentInRound() {
		t.Error("Expected HasSentInRound=false after SetContext")
	}

	// Drain the bus
	msgBus.SubscribeOutbound(ctx)
}

func TestMessageTool_Name(t *testing.T) {
	tool := NewMessageTool(bus.NewMessageBus())
	if tool.Name() != "message" {
		t.Errorf("Expected name 'message', got '%s'", tool.Name())
	}
}

func TestMessageTool_Parameters(t *testing.T) {
	tool := NewMessageTool(bus.NewMessageBus())
	params := tool.Parameters()

	typ, ok := params["type"].(string)
	if !ok || typ != "object" {
		t.Error("Expected type 'object'")
	}

	props, ok := params["properties"].(map[string]any)
	if !ok {
		t.Fatal("Expected properties to be a map")
	}

	required, ok := params["required"].([]string)
	if !ok || len(required) != 1 || required[0] != "content" {
		t.Error("Expected 'content' to be required")
	}

	contentProp, ok := props["content"].(map[string]any)
	if !ok {
		t.Error("Expected 'content' property")
	}
	if contentProp["type"] != "string" {
		t.Error("Expected content type to be 'string'")
	}

	// channel and chat_id should no longer exist
	if _, ok := props["channel"]; ok {
		t.Error("Expected 'channel' property to be removed")
	}
	if _, ok := props["chat_id"]; ok {
		t.Error("Expected 'chat_id' property to be removed")
	}
}
