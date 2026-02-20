package agent

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"time"
	"unicode/utf8"

	"localagent/pkg/activity"
	"localagent/pkg/bus"
	"localagent/pkg/config"
	"localagent/pkg/constants"
	"localagent/pkg/finance"
	"localagent/pkg/logger"
	"localagent/pkg/prompts"
	"localagent/pkg/providers"
	"localagent/pkg/session"
	"localagent/pkg/state"
	"localagent/pkg/tools"
	"localagent/pkg/utils"
)

type AgentLoop struct {
	bus            *bus.MessageBus
	provider       providers.LLMProvider
	workspace      string
	model          string
	contextWindow  int // Maximum context window size in tokens
	maxIterations  int
	sessions       *session.SessionManager
	state          *state.Manager
	contextBuilder *ContextBuilder
	tools          *tools.ToolRegistry
	activity       activity.Emitter
	running        atomic.Bool
	summarizing    sync.Map // Tracks which sessions are currently being summarized
	stopCleanup    chan struct{}
}

// processOptions configures how a message is processed
type processOptions struct {
	SessionKey      string   // Session identifier for history/context
	Channel         string   // Target channel for tool execution
	ChatID          string   // Target chat ID for tool execution
	SenderID        string   // Sender identifier (for activity events)
	UserMessage     string   // User message content (may include prefix)
	Media           []string // Media file paths attached to the message
	DefaultResponse string   // Response when LLM returns empty
	EnableSummary   bool     // Whether to trigger summarization
	SendResponse    bool     // Whether to send response via bus
	NoHistory       bool     // If true, don't load session history (for heartbeat)
	Persisted       bool     // If true, user message was already saved to session by the channel
}

// createToolRegistry creates a tool registry with common tools.
// This is shared between main agent and subagents.
func createToolRegistry(workspace string, cfg *config.Config, msgBus *bus.MessageBus) *tools.ToolRegistry {
	registry := tools.NewToolRegistry()

	// File system tools
	registry.Register(tools.NewReadFileTool(workspace))
	registry.Register(tools.NewWriteFileTool(workspace))
	registry.Register(tools.NewListDirTool(workspace))
	registry.Register(tools.NewEditFileTool(workspace))
	registry.Register(tools.NewAppendFileTool(workspace))

	// Shell execution
	registry.Register(tools.NewExecTool(workspace))

	// News tool
	registry.Register(tools.NewNewsTool(15))
	registry.Register(tools.NewAIPapersTool(15))

	// Yahoo Finance tools (shared client for auth)
	yf := finance.NewYahooClient()
	registry.Register(tools.NewStockTool(yf))
	registry.Register(tools.NewCurrencyTool(yf))

	registry.Register(tools.NewMessageTool(msgBus))

	if cfg.Tools.PDF.URL != "" {
		registry.Register(tools.NewPDFToTextTool(workspace, cfg.Tools.PDF.URL, cfg.Tools.PDF.ResolveAPIKey()))
	}

	if cfg.Tools.STT.URL != "" {
		registry.Register(tools.NewTranscribeAudioTool(workspace, cfg.Tools.STT.URL, cfg.Tools.STT.ResolveAPIKey()))
	}

	if cfg.Tools.HomeAssistant.URL != "" {
		registry.Register(tools.NewLocationTool(cfg.Tools.HomeAssistant.URL, cfg.Tools.HomeAssistant.ResolveAPIKey(), cfg.Tools.HomeAssistant.LocationUser))
	}

	if cfg.Tools.Calendar.URL != "" {
		registry.Register(tools.NewCalendarTool(cfg.Tools.Calendar.URL, cfg.Tools.Calendar.Username, cfg.Tools.Calendar.ResolvePassword()))
	}

	return registry
}

func NewAgentLoop(cfg *config.Config, msgBus *bus.MessageBus, provider providers.LLMProvider) *AgentLoop {
	workspace := cfg.WorkspacePath()
	os.MkdirAll(workspace, 0755)
	os.MkdirAll(filepath.Join(workspace, "media"), 0755)

	// Create tool registry for main agent
	toolsRegistry := createToolRegistry(workspace, cfg, msgBus)

	// Create subagent manager with its own tool registry
	subagentManager := tools.NewSubagentManager(provider, cfg.Agents.Defaults.Model, workspace, msgBus)
	subagentTools := createToolRegistry(workspace, cfg, msgBus)
	// Subagent doesn't need spawn/subagent tools to avoid recursion
	subagentManager.SetTools(subagentTools)

	// Register spawn tool (for main agent)
	spawnTool := tools.NewSpawnTool(subagentManager)
	toolsRegistry.Register(spawnTool)

	// Register subagent tool (synchronous execution)
	subagentTool := tools.NewSubagentTool(subagentManager)
	toolsRegistry.Register(subagentTool)

	sessionsManager := session.NewSessionManager(filepath.Join(workspace, "sessions"))

	// Create state manager for atomic state persistence
	stateManager := state.NewManager(workspace)

	// Create context builder and set tools registry
	contextBuilder := NewContextBuilder(workspace)
	contextBuilder.SetToolsRegistry(toolsRegistry)
	if cfg.Tools.PDF.URL != "" {
		contextBuilder.SetPDFService(cfg.Tools.PDF.URL, cfg.Tools.PDF.ResolveAPIKey())
	}
	if cfg.Tools.STT.URL != "" {
		contextBuilder.SetSTTService(cfg.Tools.STT.URL, cfg.Tools.STT.ResolveAPIKey())
	}

	stopCleanup := make(chan struct{})
	mediaDir := filepath.Join(workspace, "media")

	go func() {
		ticker := time.NewTicker(5 * time.Minute)
		defer ticker.Stop()
		for {
			select {
			case <-stopCleanup:
				return
			case <-ticker.C:
				utils.CleanOldMedia(mediaDir, 10*time.Minute)
			}
		}
	}()

	return &AgentLoop{
		bus:            msgBus,
		provider:       provider,
		workspace:      workspace,
		model:          cfg.Agents.Defaults.Model,
		contextWindow:  cfg.Agents.Defaults.MaxTokens,
		maxIterations:  cfg.Agents.Defaults.MaxToolIterations,
		sessions:       sessionsManager,
		state:          stateManager,
		contextBuilder: contextBuilder,
		tools:          toolsRegistry,
		activity:       activity.NopEmitter{},
		summarizing:    sync.Map{},
		stopCleanup:    stopCleanup,
	}
}

func (al *AgentLoop) SetActivityEmitter(e activity.Emitter) {
	al.activity = e
}

// emitActivity broadcasts an activity event via SSE and persists it to the session.
func (al *AgentLoop) emitActivity(sessionKey string, evt activity.Event) {
	al.activity.Emit(evt)
	if sessionKey != "" {
		al.sessions.AddActivity(sessionKey, evt)
	}
}

func (al *AgentLoop) Run(ctx context.Context) error {
	al.running.Store(true)

	for al.running.Load() {
		select {
		case <-ctx.Done():
			return nil
		default:
			msg, ok := al.bus.ConsumeInbound(ctx)
			if !ok {
				continue
			}

			response, err := al.processMessage(ctx, msg)
			if err != nil {
				response = fmt.Sprintf("Error processing message: %v", err)
			}

			if response != "" {
				// Check if the message tool already sent a response during this round.
				// If so, skip publishing to avoid duplicate messages to the user.
				alreadySent := false
				if tool, ok := al.tools.Get("message"); ok {
					if mt, ok := tool.(*tools.MessageTool); ok {
						alreadySent = mt.HasSentInRound()
					}
				}

				if !alreadySent {
					al.bus.PublishOutbound(bus.OutboundMessage{
						Channel: msg.Channel,
						ChatID:  msg.ChatID,
						Content: response,
					})
				}
			}
		}
	}

	return nil
}

func (al *AgentLoop) Stop() {
	al.running.Store(false)
	select {
	case <-al.stopCleanup:
	default:
		close(al.stopCleanup)
	}
}

func (al *AgentLoop) GetSessionManager() *session.SessionManager {
	return al.sessions
}

func (al *AgentLoop) RegisterTool(tool tools.Tool) {
	al.tools.Register(tool)
}

// GetToolDomains returns all domains declared by registered tools.
func (al *AgentLoop) GetToolDomains() []string {
	return al.tools.DeclaredDomains()
}

// RecordLastChannel records the last active channel for this workspace.
// This uses the atomic state save mechanism to prevent data loss on crash.
func (al *AgentLoop) RecordLastChannel(channel string) error {
	return al.state.SetLastChannel(channel)
}

// RecordLastChatID records the last active chat ID for this workspace.
// This uses the atomic state save mechanism to prevent data loss on crash.
func (al *AgentLoop) RecordLastChatID(chatID string) error {
	return al.state.SetLastChatID(chatID)
}

func (al *AgentLoop) ProcessDirect(ctx context.Context, content, sessionKey string) (string, error) {
	return al.ProcessDirectWithChannel(ctx, content, sessionKey, "cli", "direct")
}

func (al *AgentLoop) ProcessDirectWithChannel(ctx context.Context, content, sessionKey, channel, chatID string) (string, error) {
	msg := bus.InboundMessage{
		Channel:    channel,
		SenderID:   "cron",
		ChatID:     chatID,
		Content:    content,
		SessionKey: sessionKey,
	}

	return al.processMessage(ctx, msg)
}

// ProcessHeartbeat processes a heartbeat request without session history.
// Each heartbeat is independent and doesn't accumulate context.
func (al *AgentLoop) ProcessHeartbeat(ctx context.Context, content, channel, chatID string) (string, error) {
	return al.runAgentLoop(ctx, processOptions{
		SessionKey:      "heartbeat",
		Channel:         channel,
		ChatID:          chatID,
		UserMessage:     content,
		DefaultResponse: "I've completed processing but have no response to give.",
		EnableSummary:   false,
		SendResponse:    false,
		NoHistory:       true, // Don't load session history for heartbeat
	})
}

func (al *AgentLoop) processMessage(ctx context.Context, msg bus.InboundMessage) (string, error) {
	// Add message preview to log (show full content for error messages)
	var logContent string
	if strings.Contains(msg.Content, "Error:") || strings.Contains(msg.Content, "error") {
		logContent = msg.Content // Full content for errors
	} else {
		logContent = utils.Truncate(msg.Content, 80)
	}
	logger.Info("processing message from %s:%s session=%s: %s", msg.Channel, msg.SenderID, msg.SessionKey, logContent)

	// Route system messages to processSystemMessage
	if msg.Channel == "system" {
		return al.processSystemMessage(ctx, msg)
	}

	// Process as user message
	return al.runAgentLoop(ctx, processOptions{
		SessionKey:      msg.SessionKey,
		Channel:         msg.Channel,
		ChatID:          msg.ChatID,
		SenderID:        msg.SenderID,
		UserMessage:     msg.Content,
		Media:           msg.Media,
		DefaultResponse: "I've completed processing but have no response to give.",
		EnableSummary:   true,
		SendResponse:    false,
		Persisted:       msg.Persisted,
	})
}

func (al *AgentLoop) processSystemMessage(_ context.Context, msg bus.InboundMessage) (string, error) {
	// Verify this is a system message
	if msg.Channel != "system" {
		return "", fmt.Errorf("processSystemMessage called with non-system message channel: %s", msg.Channel)
	}

	logger.Info("processing system message: sender=%s chat=%s", msg.SenderID, msg.ChatID)

	// Parse origin channel from chat_id (format: "channel:chat_id")
	var originChannel string
	if idx := strings.Index(msg.ChatID, ":"); idx > 0 {
		originChannel = msg.ChatID[:idx]
	} else {
		// Fallback
		originChannel = "cli"
	}

	// Extract subagent result from message content
	// Format: "Task 'label' completed.\n\nResult:\n<actual content>"
	content := msg.Content
	if idx := strings.Index(content, "Result:\n"); idx >= 0 {
		content = content[idx+8:] // Extract just the result part
	}

	// Skip internal channels - only log, don't send to user
	if constants.IsInternalChannel(originChannel) {
		logger.Info("subagent completed (internal channel): sender=%s channel=%s content_len=%d", msg.SenderID, originChannel, len(content))
		return "", nil
	}

	// Agent acts as dispatcher only - subagent handles user interaction via message tool
	// Don't forward result here, subagent should use message tool to communicate with user
	logger.Info("subagent completed: sender=%s channel=%s content_len=%d", msg.SenderID, originChannel, len(content))
	// Agent only logs, does not respond to user
	return "", nil
}

// runAgentLoop is the core message processing logic.
// It handles context building, LLM calls, tool execution, and response handling.
func (al *AgentLoop) runAgentLoop(ctx context.Context, opts processOptions) (string, error) {
	// 0. Record last channel for heartbeat notifications (skip internal channels)
	if opts.Channel != "" && opts.ChatID != "" {
		// Don't record internal channels (cli, system, subagent)
		if !constants.IsInternalChannel(opts.Channel) {
			channelKey := fmt.Sprintf("%s:%s", opts.Channel, opts.ChatID)
			if err := al.RecordLastChannel(channelKey); err != nil {
				logger.Warn("failed to record last channel: %v", err)
			}
		}
	}

	// 1. Update tool contexts
	al.updateToolContexts(opts.Channel, opts.ChatID)

	// 2. Build messages (skip history for heartbeat)
	var history []providers.Message
	var summary string
	if !opts.NoHistory {
		history = al.sessions.GetHistory(opts.SessionKey)
		summary = al.sessions.GetSummary(opts.SessionKey)

		// If the message was already persisted by the channel, trim queued
		// user messages from the tail of history. These are messages that
		// were saved to session on arrival but haven't been processed yet.
		// BuildMessages will re-add the current user message with proper
		// media handling.
		if opts.Persisted {
			for len(history) > 0 && history[len(history)-1].Role == "user" {
				history = history[:len(history)-1]
			}
		}
	}
	messages := al.contextBuilder.BuildMessages(
		history,
		summary,
		opts.UserMessage,
		opts.Media,
		opts.Channel,
		opts.ChatID,
	)

	// 3. Save user message to session (skip if already persisted by channel)
	if !opts.Persisted {
		al.sessions.AddMessageWithMedia(opts.SessionKey, "user", opts.UserMessage, opts.Media)
	}

	// 4. Emit processing start activity (after user message is saved so timeline order is correct)
	if opts.SenderID != "" {
		al.emitActivity(opts.SessionKey, activity.Event{
			Type:      activity.ProcessingStart,
			Timestamp: time.Now(),
			Message:   fmt.Sprintf("Processing message from %s:%s", opts.Channel, opts.SenderID),
			Detail: map[string]any{
				"channel": opts.Channel,
				"sender":  opts.SenderID,
				"preview": utils.Truncate(opts.UserMessage, 100),
			},
		})
	}

	// 5. Run LLM iteration loop
	finalContent, iteration, tokenCount, err := al.runLLMIteration(ctx, messages, opts)
	if err != nil {
		return "", err
	}

	// If last tool had ForUser content and we already sent it, we might not need to send final response
	// This is controlled by the tool's Silent flag and ForUser content

	// 6. Handle empty response
	if finalContent == "" {
		finalContent = opts.DefaultResponse
	}

	// 7. Emit completion activity (before saving message so it sorts earlier in timeline)
	al.emitActivity(opts.SessionKey, activity.Event{
		Type:      activity.Complete,
		Timestamp: time.Now(),
		Message:   fmt.Sprintf("Complete (%d iterations, %d chars)", iteration, len(finalContent)),
		Detail: map[string]any{
			"session":    opts.SessionKey,
			"iterations": iteration,
			"length":     len(finalContent),
		},
	})

	// 8. Save final assistant message to session
	al.sessions.AddMessage(opts.SessionKey, "assistant", finalContent)
	al.sessions.Save(opts.SessionKey)

	// 9. Optional: summarization
	if opts.EnableSummary {
		al.maybeSummarize(opts.SessionKey, tokenCount)
	}

	// 10. Optional: send response via bus
	if opts.SendResponse {
		al.bus.PublishOutbound(bus.OutboundMessage{
			Channel: opts.Channel,
			ChatID:  opts.ChatID,
			Content: finalContent,
		})
	}

	// 11. Log response
	responsePreview := utils.Truncate(finalContent, 120)
	logger.Info("response: %s (session=%s iterations=%d len=%d)", responsePreview, opts.SessionKey, iteration, len(finalContent))

	return finalContent, nil
}

// runLLMIteration executes the LLM call loop with tool handling.
// Returns the final content, iteration count, last known token count, and any error.
func (al *AgentLoop) runLLMIteration(ctx context.Context, messages []providers.Message, opts processOptions) (string, int, int, error) {
	iteration := 0
	var finalContent string
	var lastTokenCount int

	for iteration < al.maxIterations {
		iteration++

		logger.Debug("LLM iteration %d/%d", iteration, al.maxIterations)

		// Build tool definitions
		providerToolDefs := al.tools.ToProviderDefs()

		// Log LLM request details
		logger.Debug("LLM request: iteration=%d model=%s messages=%d tools=%d", iteration, al.model, len(messages), len(providerToolDefs))
		logger.Debug("full LLM request: iteration=%d messages=%s tools=%s", iteration, formatMessagesForLog(messages), formatToolsForLog(providerToolDefs))

		lastMsgPreview := ""
		if len(messages) > 0 {
			last := messages[len(messages)-1]
			lastMsgPreview = utils.Truncate(last.Content, 300)
		}
		al.emitActivity(opts.SessionKey, activity.Event{
			Type:      activity.LLMRequest,
			Timestamp: time.Now(),
			Message:   fmt.Sprintf("LLM request #%d (%s)", iteration, al.model),
			Detail: map[string]any{
				"iteration":    iteration,
				"model":        al.model,
				"messages":     len(messages),
				"tools":        len(providerToolDefs),
				"last_message": lastMsgPreview,
			},
		})

		// Call LLM
		response, err := al.provider.Chat(ctx, messages, providerToolDefs, al.model, map[string]any{
			"max_tokens":  8192,
			"temperature": 0.7,
		})

		if err != nil {
			logger.Error("LLM call failed: iteration=%d: %v", iteration, err)
			al.emitActivity(opts.SessionKey, activity.Event{
				Type:      activity.LLMError,
				Timestamp: time.Now(),
				Message:   fmt.Sprintf("LLM error on iteration #%d", iteration),
				Detail:    map[string]any{"error": err.Error()},
			})
			return "", iteration, lastTokenCount, fmt.Errorf("LLM call failed: %w", err)
		}

		if response.Usage != nil {
			lastTokenCount = response.Usage.PromptTokens + response.Usage.CompletionTokens
		}

		// Check if no tool calls - we're done
		if len(response.ToolCalls) == 0 {
			finalContent = response.Content
			logger.Info("LLM response (direct answer): iteration=%d chars=%d", iteration, len(finalContent))
			responseDetail := map[string]any{
				"iteration": iteration,
				"chars":     len(finalContent),
				"content":   utils.Truncate(finalContent, 500),
			}
			if response.Usage != nil {
				responseDetail["usage"] = map[string]any{
					"prompt_tokens":     response.Usage.PromptTokens,
					"completion_tokens": response.Usage.CompletionTokens,
					"total_tokens":      response.Usage.TotalTokens,
				}
			}
			al.emitActivity(opts.SessionKey, activity.Event{
				Type:      activity.LLMResponse,
				Timestamp: time.Now(),
				Message:   fmt.Sprintf("LLM response #%d (%d chars)", iteration, len(finalContent)),
				Detail:    responseDetail,
			})
			break
		}

		// Log tool calls
		toolNames := make([]string, 0, len(response.ToolCalls))
		for _, tc := range response.ToolCalls {
			toolNames = append(toolNames, tc.Name)
		}
		logger.Info("LLM requested tool calls: %v (count=%d iteration=%d)", toolNames, len(response.ToolCalls), iteration)

		al.emitActivity(opts.SessionKey, activity.Event{
			Type:      activity.ToolCall,
			Timestamp: time.Now(),
			Message:   fmt.Sprintf("Calling %d tool(s): %s", len(response.ToolCalls), strings.Join(toolNames, ", ")),
			Detail: map[string]any{
				"tools":     toolNames,
				"count":     len(response.ToolCalls),
				"iteration": iteration,
			},
		})

		// Build assistant message with tool calls
		assistantMsg := tools.BuildAssistantToolCallMessage(response.Content, response.ToolCalls)
		messages = append(messages, assistantMsg)

		// Save assistant message with tool calls to session
		al.sessions.AddFullMessage(opts.SessionKey, assistantMsg)

		// Execute tool calls
		for _, tc := range response.ToolCalls {
			// Log tool call with arguments preview
			argsJSON, _ := json.Marshal(tc.Arguments)
			argsPreview := utils.Truncate(string(argsJSON), 200)
			logger.Info("tool call: %s(%s) iteration=%d", tc.Name, argsPreview, iteration)

			al.emitActivity(opts.SessionKey, activity.Event{
				Type:      activity.ToolCall,
				Timestamp: time.Now(),
				Message:   fmt.Sprintf("Tool: %s", tc.Name),
				Detail: map[string]any{
					"tool":   tc.Name,
					"params": utils.Truncate(string(argsJSON), 500),
				},
			})

			// Create async callback for tools that implement AsyncTool
			// NOTE: Following openclaw's design, async tools do NOT send results directly to users.
			// Instead, they notify the agent via PublishInbound, and the agent decides
			// whether to forward the result to the user (in processSystemMessage).
			asyncCallback := func(_ context.Context, result *tools.ToolResult) {
				if !result.Silent && result.ForUser != "" {
					logger.Info("async tool completed: %s content_len=%d", tc.Name, len(result.ForUser))
				}
			}

			toolResult := al.tools.ExecuteWithContext(ctx, tc.Name, tc.Arguments, opts.Channel, opts.ChatID, asyncCallback)

			status := "success"
			if toolResult.IsError {
				status = "error"
			}
			resultDetail := map[string]any{
				"tool":    tc.Name,
				"status":  status,
				"content": utils.Truncate(toolResult.ForLLM, 500),
			}
			al.emitActivity(opts.SessionKey, activity.Event{
				Type:      activity.ToolResult,
				Timestamp: time.Now(),
				Message:   fmt.Sprintf("Tool result: %s", tc.Name),
				Detail:    resultDetail,
			})

			// Send ForUser content to user immediately if not Silent
			if !toolResult.Silent && toolResult.ForUser != "" && opts.SendResponse {
				al.bus.PublishOutbound(bus.OutboundMessage{
					Channel: opts.Channel,
					ChatID:  opts.ChatID,
					Content: toolResult.ForUser,
				})
				logger.Debug("sent tool result to user: %s content_len=%d", tc.Name, len(toolResult.ForUser))
			}

			toolResultMsg := tools.BuildToolResultMessage(tc.ID, toolResult)
			messages = append(messages, toolResultMsg)

			// Save tool result message to session
			al.sessions.AddFullMessage(opts.SessionKey, toolResultMsg)
		}
	}

	return finalContent, iteration, lastTokenCount, nil
}

// updateToolContexts updates the context for tools that need channel/chatID info.
func (al *AgentLoop) updateToolContexts(channel, chatID string) {
	// Use ContextualTool interface instead of type assertions
	if tool, ok := al.tools.Get("message"); ok {
		if mt, ok := tool.(tools.ContextualTool); ok {
			mt.SetContext(channel, chatID)
		}
	}
	if tool, ok := al.tools.Get("spawn"); ok {
		if st, ok := tool.(tools.ContextualTool); ok {
			st.SetContext(channel, chatID)
		}
	}
	if tool, ok := al.tools.Get("subagent"); ok {
		if st, ok := tool.(tools.ContextualTool); ok {
			st.SetContext(channel, chatID)
		}
	}
}

// maybeSummarize triggers summarization if the session history exceeds thresholds.
func (al *AgentLoop) maybeSummarize(sessionKey string, tokenCount int) {
	newHistory := al.sessions.GetHistory(sessionKey)
	if tokenCount == 0 {
		tokenCount = al.estimateTokens(newHistory)
	}
	threshold := al.contextWindow * 75 / 100

	if len(newHistory) > 50 || tokenCount > threshold {
		if _, loading := al.summarizing.LoadOrStore(sessionKey, true); !loading {
			go func() {
				defer al.summarizing.Delete(sessionKey)
				al.memoryFlush(sessionKey)
				al.summarizeSession(sessionKey)
			}()
		}
	}
}

// memoryFlush runs a mini agent turn to persist important conversation context
// to daily notes before summarization truncates the history.
func (al *AgentLoop) memoryFlush(sessionKey string) {
	history := al.sessions.GetHistory(sessionKey)
	if len(history) == 0 {
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	registry := tools.NewToolRegistry()
	registry.Register(tools.NewWriteFileTool(al.workspace))
	registry.Register(tools.NewAppendFileTool(al.workspace))
	registry.Register(tools.NewReadFileTool(al.workspace))

	todayPath := al.contextBuilder.GetMemoryStore().GetTodayFile()

	systemMsg := providers.Message{
		Role:    "system",
		Content: strings.TrimSpace(prompts.MemoryFlushSystem) + " " + todayPath,
	}

	userMsg := providers.Message{
		Role:    "user",
		Content: strings.TrimSpace(prompts.MemoryFlushUser),
	}

	messages := []providers.Message{systemMsg}
	messages = append(messages, history...)
	messages = append(messages, userMsg)

	result, err := tools.RunToolLoop(ctx, tools.ToolLoopConfig{
		Provider:      al.provider,
		Model:         al.model,
		Tools:         registry,
		MaxIterations: 3,
	}, messages, "", "")

	if err != nil {
		logger.Warn("memory flush failed for session %s: %v", sessionKey, err)
		return
	}

	logger.Info("memory flush completed for session %s: %d iterations", sessionKey, result.Iterations)
}

// GetStartupInfo returns information about loaded tools and skills for logging.
func (al *AgentLoop) GetStartupInfo() map[string]any {
	info := make(map[string]any)

	// Tools info
	tools := al.tools.List()
	info["tools"] = map[string]any{
		"count": len(tools),
		"names": tools,
	}

	// Skills info
	info["skills"] = al.contextBuilder.GetSkillsInfo()

	return info
}

// formatMessagesForLog formats messages for logging
func formatMessagesForLog(messages []providers.Message) string {
	if len(messages) == 0 {
		return "[]"
	}

	var result strings.Builder
	fmt.Fprintf(&result, "[\n")
	for i, msg := range messages {
		fmt.Fprintf(&result, "  [%d] Role: %s\n", i, msg.Role)
		if len(msg.ToolCalls) > 0 {
			fmt.Fprintf(&result, "  ToolCalls:\n")
			for _, tc := range msg.ToolCalls {
				fmt.Fprintf(&result, "    - ID: %s, Type: %s, Name: %s\n", tc.ID, tc.Type, tc.Name)
				if tc.Function != nil {
					fmt.Fprintf(&result, "      Arguments: %s\n", utils.Truncate(tc.Function.Arguments, 200))
				}
			}
		}
		if msg.Content != "" {
			content := utils.Truncate(msg.Content, 200)
			fmt.Fprintf(&result, "  Content: %s\n", content)
		}
		if msg.ToolCallID != "" {
			fmt.Fprintf(&result, "  ToolCallID: %s\n", msg.ToolCallID)
		}
		fmt.Fprintf(&result, "\n")
	}
	fmt.Fprintf(&result, "]")
	return result.String()
}

// formatToolsForLog formats tool definitions for logging
func formatToolsForLog(tools []providers.ToolDefinition) string {
	if len(tools) == 0 {
		return "[]"
	}

	var result strings.Builder
	fmt.Fprintf(&result, "[\n")
	for i, tool := range tools {
		fmt.Fprintf(&result, "  [%d] Type: %s, Name: %s\n", i, tool.Type, tool.Function.Name)
		fmt.Fprintf(&result, "      Description: %s\n", tool.Function.Description)
		if len(tool.Function.Parameters) > 0 {
			fmt.Fprintf(&result, "      Parameters: %s\n", utils.Truncate(fmt.Sprintf("%v", tool.Function.Parameters), 200))
		}
	}
	fmt.Fprintf(&result, "]")
	return result.String()
}

// summarizeSession summarizes the conversation history for a session.
func (al *AgentLoop) summarizeSession(sessionKey string) {
	ctx, cancel := context.WithTimeout(context.Background(), 120*time.Second)
	defer cancel()

	history := al.sessions.GetHistory(sessionKey)
	summary := al.sessions.GetSummary(sessionKey)

	// Keep last 4 messages for continuity
	if len(history) <= 4 {
		return
	}

	toSummarize := history[:len(history)-4]

	// Oversized Message Guard
	// Skip messages larger than 50% of context window to prevent summarizer overflow
	maxMessageTokens := al.contextWindow / 2
	validMessages := make([]providers.Message, 0)
	omitted := false

	for _, m := range toSummarize {
		if m.Role != "user" && m.Role != "assistant" {
			continue
		}
		// Estimate tokens for this message
		msgTokens := len(m.Content) / 4
		if msgTokens > maxMessageTokens {
			omitted = true
			continue
		}
		validMessages = append(validMessages, m)
	}

	if len(validMessages) == 0 {
		return
	}

	// Multi-Part Summarization
	// Split into two parts if history is significant
	var finalSummary string
	if len(validMessages) > 10 {
		mid := len(validMessages) / 2
		part1 := validMessages[:mid]
		part2 := validMessages[mid:]

		s1, _ := al.summarizeBatch(ctx, part1, "")
		s2, _ := al.summarizeBatch(ctx, part2, "")

		// Merge them
		mergePrompt := fmt.Sprintf(prompts.SummarizeMerge, s1, s2)
		resp, err := al.provider.Chat(ctx, []providers.Message{{Role: "user", Content: mergePrompt}}, nil, al.model, map[string]any{
			"max_tokens":  1024,
			"temperature": 0.3,
		})
		if err == nil {
			finalSummary = resp.Content
		} else {
			finalSummary = s1 + " " + s2
		}
	} else {
		finalSummary, _ = al.summarizeBatch(ctx, validMessages, summary)
	}

	if omitted && finalSummary != "" {
		finalSummary += "\n[Note: Some oversized messages were omitted from this summary for efficiency.]"
	}

	if finalSummary != "" {
		al.sessions.SetSummary(sessionKey, finalSummary)
		al.sessions.TruncateHistory(sessionKey, 4)
		al.sessions.Save(sessionKey)
	}
}

// summarizeBatch summarizes a batch of messages.
func (al *AgentLoop) summarizeBatch(ctx context.Context, batch []providers.Message, existingSummary string) (string, error) {
	var prompt strings.Builder
	prompt.WriteString(strings.TrimSpace(prompts.SummarizeBatch) + "\n")
	if existingSummary != "" {
		prompt.WriteString("Existing context: " + existingSummary + "\n")
	}
	prompt.WriteString("\nCONVERSATION:\n")
	for _, m := range batch {
		fmt.Fprintf(&prompt, "%s: %s\n", m.Role, m.Content)
	}

	response, err := al.provider.Chat(ctx, []providers.Message{{Role: "user", Content: prompt.String()}}, nil, al.model, map[string]any{
		"max_tokens":  1024,
		"temperature": 0.3,
	})
	if err != nil {
		return "", err
	}
	return response.Content, nil
}

// estimateTokens estimates the number of tokens in a message list.
// Uses rune count instead of byte length so that CJK and other multi-byte
// characters are not over-counted (a Chinese character is 3 bytes but roughly
// one token).
func (al *AgentLoop) estimateTokens(messages []providers.Message) int {
	total := 0
	for _, m := range messages {
		total += utf8.RuneCountInString(m.Content) / 3
	}
	return total
}
