package tools

import (
	"context"
	"encoding/json"
	"fmt"

	"localagent/pkg/logger"
	"localagent/pkg/providers"
)

type ToolLoopConfig struct {
	Provider      providers.LLMProvider
	Model         string
	Tools         *ToolRegistry
	MaxIterations int
	LLMOptions    map[string]any
}

type ToolLoopResult struct {
	Content    string
	Iterations int
}

// BuildAssistantToolCallMessage builds an assistant message with serialized tool call arguments.
func BuildAssistantToolCallMessage(content string, toolCalls []providers.ToolCall) providers.Message {
	msg := providers.Message{
		Role:    "assistant",
		Content: content,
	}
	for _, tc := range toolCalls {
		argumentsJSON, _ := json.Marshal(tc.Arguments)
		msg.ToolCalls = append(msg.ToolCalls, providers.ToolCall{
			ID:   tc.ID,
			Type: "function",
			Function: &providers.FunctionCall{
				Name:      tc.Name,
				Arguments: string(argumentsJSON),
			},
		})
	}
	return msg
}

// BuildToolResultMessage builds a tool result message with ForLLM/Err fallback logic.
func BuildToolResultMessage(toolCallID string, result *ToolResult) providers.Message {
	contentForLLM := result.ForLLM
	if contentForLLM == "" && result.Err != nil {
		contentForLLM = result.Err.Error()
	}
	return providers.Message{
		Role:       "tool",
		Content:    contentForLLM,
		ToolCallID: toolCallID,
	}
}

func RunToolLoop(ctx context.Context, config ToolLoopConfig, messages []providers.Message, channel, chatID string) (*ToolLoopResult, error) {
	iteration := 0
	var finalContent string

	for iteration < config.MaxIterations {
		iteration++

		logger.Debug("toolloop iteration %d/%d", iteration, config.MaxIterations)

		var providerToolDefs []providers.ToolDefinition
		if config.Tools != nil {
			providerToolDefs = config.Tools.ToProviderDefs()
		}

		llmOpts := config.LLMOptions
		if llmOpts == nil {
			llmOpts = map[string]any{
				"max_tokens":  4096,
				"temperature": 0.7,
			}
		}

		response, err := config.Provider.Chat(ctx, messages, providerToolDefs, config.Model, llmOpts)
		if err != nil {
			return nil, fmt.Errorf("LLM call failed: %w", err)
		}

		if len(response.ToolCalls) == 0 {
			finalContent = response.Content
			break
		}

		logger.Info("toolloop: LLM requested %d tool call(s)", len(response.ToolCalls))

		messages = append(messages, BuildAssistantToolCallMessage(response.Content, response.ToolCalls))

		for _, tc := range response.ToolCalls {
			argsJSON, _ := json.Marshal(tc.Arguments)
			preview := string(argsJSON)
			if len(preview) > 200 {
				preview = preview[:197] + "..."
			}
			logger.Info("toolloop: tool call %s(%s)", tc.Name, preview)

			var toolResult *ToolResult
			if config.Tools != nil {
				toolResult = config.Tools.ExecuteWithContext(ctx, tc.Name, tc.Arguments, channel, chatID, nil)
			} else {
				toolResult = ErrorResult("No tools available")
			}

			messages = append(messages, BuildToolResultMessage(tc.ID, toolResult))
		}
	}

	return &ToolLoopResult{
		Content:    finalContent,
		Iterations: iteration,
	}, nil
}
