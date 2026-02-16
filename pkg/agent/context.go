package agent

import (
	"encoding/base64"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"time"
	"unicode/utf8"

	"localagent/pkg/logger"
	"localagent/pkg/prompts"
	"localagent/pkg/providers"
	"localagent/pkg/skills"
	"localagent/pkg/tools"
	"localagent/pkg/utils"
)

type ContextBuilder struct {
	workspace    string
	skillsLoader *skills.SkillsLoader
	memory       *MemoryStore
	tools        *tools.ToolRegistry // Direct reference to tool registry
}

func getGlobalConfigDir() string {
	home, err := os.UserHomeDir()
	if err != nil {
		return ""
	}
	return filepath.Join(home, ".localagent")
}

func NewContextBuilder(workspace string) *ContextBuilder {
	// builtin skills: skills directory in current project
	// Use the skills/ directory under the current working directory
	wd, _ := os.Getwd()
	builtinSkillsDir := filepath.Join(wd, "skills")
	globalSkillsDir := filepath.Join(getGlobalConfigDir(), "skills")

	return &ContextBuilder{
		workspace:    workspace,
		skillsLoader: skills.NewSkillsLoader(workspace, globalSkillsDir, builtinSkillsDir),
		memory:       NewMemoryStore(workspace),
	}
}

// GetMemoryStore returns the memory store for direct access (e.g. memory flush).
func (cb *ContextBuilder) GetMemoryStore() *MemoryStore {
	return cb.memory
}

// SetToolsRegistry sets the tools registry for dynamic tool summary generation.
func (cb *ContextBuilder) SetToolsRegistry(registry *tools.ToolRegistry) {
	cb.tools = registry
}

func (cb *ContextBuilder) getIdentity() string {
	now := time.Now().Format("2006-01-02 15:04 (Monday)")
	workspacePath, _ := filepath.Abs(filepath.Join(cb.workspace))
	rt := fmt.Sprintf("%s %s, Go %s", runtime.GOOS, runtime.GOARCH, runtime.Version())

	toolsSection := cb.buildToolsSection()

	return fmt.Sprintf(prompts.SystemIdentity,
		now, rt, workspacePath, workspacePath, workspacePath, workspacePath, toolsSection, workspacePath)
}

func (cb *ContextBuilder) buildToolsSection() string {
	if cb.tools == nil {
		return ""
	}

	summaries := cb.tools.GetSummaries()
	if len(summaries) == 0 {
		return ""
	}

	var sb strings.Builder
	sb.WriteString(prompts.ToolsSection)
	sb.WriteString("\n")
	for _, s := range summaries {
		sb.WriteString(s)
		sb.WriteString("\n")
	}

	return sb.String()
}

func (cb *ContextBuilder) BuildSystemPrompt() string {
	parts := []string{}

	// Core identity section
	parts = append(parts, cb.getIdentity())

	// Bootstrap files
	bootstrapContent := cb.LoadBootstrapFiles()
	if bootstrapContent != "" {
		parts = append(parts, bootstrapContent)
	}

	// Skills - show summary, AI can read full content with read_file tool
	skillsSummary := cb.skillsLoader.BuildSkillsSummary()
	if skillsSummary != "" {
		parts = append(parts, fmt.Sprintf(prompts.SkillsSection, skillsSummary))
	}

	// Memory context
	memoryContext := cb.memory.GetMemoryContext()
	if memoryContext != "" {
		parts = append(parts, "# Memory\n\n"+memoryContext)
	}

	// Join with "---" separator
	return strings.Join(parts, "\n\n---\n\n")
}

func (cb *ContextBuilder) LoadBootstrapFiles() string {
	bootstrapFiles := []string{
		"AGENTS.md",
		"SOUL.md",
		"USER.md",
		"IDENTITY.md",
	}

	var result strings.Builder
	for _, filename := range bootstrapFiles {
		filePath := filepath.Join(cb.workspace, filename)
		if data, err := os.ReadFile(filePath); err == nil {
			fmt.Fprintf(&result, "## %s\n\n%s\n\n", filename, string(data))
		}
	}

	return result.String()
}

func (cb *ContextBuilder) BuildMessages(history []providers.Message, summary string, currentMessage string, media []string, channel, chatID string) []providers.Message {
	messages := []providers.Message{}

	systemPrompt := cb.BuildSystemPrompt()

	// Add Current Session info if provided
	if channel != "" && chatID != "" {
		systemPrompt += fmt.Sprintf("\n\n## Current Session\nChannel: %s\nChat ID: %s", channel, chatID)
	}

	logger.Debug("system prompt built: %d chars, %d lines",
		len(systemPrompt), strings.Count(systemPrompt, "\n")+1)

	if summary != "" {
		systemPrompt += "\n\n## Summary of Previous Conversation\n\n" + summary
	}

	for len(history) > 0 && history[0].Role == "tool" {
		history = history[1:]
	}

	messages = append(messages, providers.Message{
		Role:    "system",
		Content: systemPrompt,
	})

	messages = append(messages, history...)

	// Build user message, with multimodal content parts if media is attached
	userMsg := cb.buildUserMessage(currentMessage, media)
	messages = append(messages, userMsg)

	return messages
}

// buildUserMessage constructs a user message, adding multimodal content parts
// when media files are attached.
func (cb *ContextBuilder) buildUserMessage(text string, media []string) providers.Message {
	if len(media) == 0 {
		return providers.Message{Role: "user", Content: text}
	}

	var parts []providers.ContentPart

	// Add text part (use a default if empty)
	if text != "" {
		parts = append(parts, providers.ContentPart{Type: "text", Text: text})
	}

	for _, mediaPath := range media {
		data, err := os.ReadFile(mediaPath)
		if err != nil {
			logger.Warn("failed to read media file %s: %v", mediaPath, err)
			continue
		}

		if utils.IsImageFile(mediaPath) {
			// Encode image as base64 data URL
			mimeType := utils.DetectMIMEType(mediaPath)
			dataURL := fmt.Sprintf("data:%s;base64,%s", mimeType, base64.StdEncoding.EncodeToString(data))
			parts = append(parts, providers.ContentPart{
				Type:     "image_url",
				ImageURL: &providers.ImageURL{URL: dataURL},
			})
		} else if utf8.Valid(data) {
			// Include text-based files inline
			filename := filepath.Base(mediaPath)
			parts = append(parts, providers.ContentPart{
				Type: "text",
				Text: fmt.Sprintf("\n--- File: %s ---\n%s\n--- End of %s ---", filename, string(data), filename),
			})
		} else {
			// Binary file - just note it
			filename := filepath.Base(mediaPath)
			parts = append(parts, providers.ContentPart{
				Type: "text",
				Text: fmt.Sprintf("[Attached binary file: %s]", filename),
			})
		}
	}

	if len(parts) == 0 {
		return providers.Message{Role: "user", Content: text}
	}

	// Ensure there's at least one text part (required by most APIs)
	hasText := false
	for _, p := range parts {
		if p.Type == "text" {
			hasText = true
			break
		}
	}
	if !hasText {
		parts = append([]providers.ContentPart{{Type: "text", Text: "The user has shared the attached file(s)."}}, parts...)
	}

	return providers.Message{
		Role:         "user",
		Content:      text,
		ContentParts: parts,
	}
}

func (cb *ContextBuilder) AddToolResult(messages []providers.Message, toolCallID, toolName, result string) []providers.Message {
	messages = append(messages, providers.Message{
		Role:       "tool",
		Content:    result,
		ToolCallID: toolCallID,
	})
	return messages
}

func (cb *ContextBuilder) AddAssistantMessage(messages []providers.Message, content string, toolCalls []map[string]any) []providers.Message {
	msg := providers.Message{
		Role:    "assistant",
		Content: content,
	}
	// Always add assistant message, whether or not it has tool calls
	messages = append(messages, msg)
	return messages
}

func (cb *ContextBuilder) loadSkills() string {
	allSkills := cb.skillsLoader.ListSkills()
	if len(allSkills) == 0 {
		return ""
	}

	var skillNames []string
	for _, s := range allSkills {
		skillNames = append(skillNames, s.Name)
	}

	content := cb.skillsLoader.LoadSkillsForContext(skillNames)
	if content == "" {
		return ""
	}

	return "# Skill Definitions\n\n" + content
}

// GetSkillsInfo returns information about loaded skills.
func (cb *ContextBuilder) GetSkillsInfo() map[string]any {
	allSkills := cb.skillsLoader.ListSkills()
	skillNames := make([]string, 0, len(allSkills))
	for _, s := range allSkills {
		skillNames = append(skillNames, s.Name)
	}
	return map[string]any{
		"total":     len(allSkills),
		"available": len(allSkills),
		"names":     skillNames,
	}
}
