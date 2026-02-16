package heartbeat

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"localagent/pkg/bus"
	"localagent/pkg/constants"
	"localagent/pkg/logger"
	"localagent/pkg/prompts"
	"localagent/pkg/state"
	"localagent/pkg/tools"
)

const (
	minIntervalMinutes     = 5
	defaultIntervalMinutes = 30
)

// HeartbeatHandler is the function type for handling heartbeat.
// It returns a ToolResult that can indicate async operations.
// channel and chatID are derived from the last active user channel.
type HeartbeatHandler func(prompt, channel, chatID string) *tools.ToolResult

// HeartbeatService manages periodic heartbeat checks
type HeartbeatService struct {
	workspace string
	bus       *bus.MessageBus
	state     *state.Manager
	handler   HeartbeatHandler
	interval  time.Duration
	enabled   bool
	mu        sync.RWMutex
	stopChan  chan struct{}
}

// NewHeartbeatService creates a new heartbeat service
func NewHeartbeatService(workspace string, intervalMinutes int, enabled bool) *HeartbeatService {
	// Apply minimum interval
	if intervalMinutes < minIntervalMinutes && intervalMinutes != 0 {
		intervalMinutes = minIntervalMinutes
	}

	if intervalMinutes == 0 {
		intervalMinutes = defaultIntervalMinutes
	}

	return &HeartbeatService{
		workspace: workspace,
		interval:  time.Duration(intervalMinutes) * time.Minute,
		enabled:   enabled,
		state:     state.NewManager(workspace),
	}
}

// SetBus sets the message bus for delivering heartbeat results.
func (hs *HeartbeatService) SetBus(msgBus *bus.MessageBus) {
	hs.mu.Lock()
	defer hs.mu.Unlock()
	hs.bus = msgBus
}

// SetHandler sets the heartbeat handler.
func (hs *HeartbeatService) SetHandler(handler HeartbeatHandler) {
	hs.mu.Lock()
	defer hs.mu.Unlock()
	hs.handler = handler
}

// Start begins the heartbeat service
func (hs *HeartbeatService) Start() error {
	hs.mu.Lock()
	defer hs.mu.Unlock()

	if hs.stopChan != nil {
		logger.Info("heartbeat: service already running")
		return nil
	}

	if !hs.enabled {
		logger.Info("heartbeat: service disabled")
		return nil
	}

	hs.stopChan = make(chan struct{})
	go hs.runLoop(hs.stopChan)

	logger.Info("heartbeat: service started (interval: %.0f min)", hs.interval.Minutes())

	return nil
}

// Stop gracefully stops the heartbeat service
func (hs *HeartbeatService) Stop() {
	hs.mu.Lock()
	defer hs.mu.Unlock()

	if hs.stopChan == nil {
		return
	}

	logger.Info("heartbeat: stopping service")
	close(hs.stopChan)
	hs.stopChan = nil
}

// runLoop runs the heartbeat ticker
func (hs *HeartbeatService) runLoop(stopChan chan struct{}) {
	ticker := time.NewTicker(hs.interval)
	defer ticker.Stop()

	// Run first heartbeat after initial delay
	time.AfterFunc(time.Second, func() {
		hs.executeHeartbeat()
	})

	for {
		select {
		case <-stopChan:
			return
		case <-ticker.C:
			hs.executeHeartbeat()
		}
	}
}

// executeHeartbeat performs a single heartbeat check
func (hs *HeartbeatService) executeHeartbeat() {
	hs.mu.RLock()
	enabled := hs.enabled
	handler := hs.handler
	if !hs.enabled || hs.stopChan == nil {
		hs.mu.RUnlock()
		return
	}
	hs.mu.RUnlock()

	if !enabled {
		return
	}

	logger.Debug("heartbeat: executing")

	prompt := hs.buildPrompt()
	if prompt == "" {
		logger.Info("heartbeat: no prompt (HEARTBEAT.md empty or missing)")
		return
	}

	if handler == nil {
		hs.logError("Heartbeat handler not configured")
		return
	}

	// Get last channel info for context
	lastChannel := hs.state.GetLastChannel()
	channel, chatID := hs.parseLastChannel(lastChannel)

	// Debug log for channel resolution
	hs.logInfo("Resolved channel: %s, chatID: %s (from lastChannel: %s)", channel, chatID, lastChannel)

	result := handler(prompt, channel, chatID)

	if result == nil {
		hs.logInfo("Heartbeat handler returned nil result")
		return
	}

	// Handle different result types
	if result.IsError {
		hs.logError("Heartbeat error: %s", result.ForLLM)
		return
	}

	if result.Async {
		hs.logInfo("Async task started: %s", result.ForLLM)
		logger.Info("heartbeat: async task started: %s", result.ForLLM)
		return
	}

	// Check if silent
	if result.Silent {
		hs.logInfo("Heartbeat OK - silent")
		return
	}

	// Send result to user
	if result.ForUser != "" {
		hs.sendResponse(result.ForUser)
	} else if result.ForLLM != "" {
		hs.sendResponse(result.ForLLM)
	}

	hs.logInfo("Heartbeat completed: %s", result.ForLLM)
}

// buildPrompt builds the heartbeat prompt from HEARTBEAT.md
func (hs *HeartbeatService) buildPrompt() string {
	heartbeatPath := filepath.Join(hs.workspace, "HEARTBEAT.md")

	data, err := os.ReadFile(heartbeatPath)
	if err != nil {
		if os.IsNotExist(err) {
			hs.createDefaultHeartbeatTemplate()
			return ""
		}
		hs.logError("Error reading HEARTBEAT.md: %v", err)
		return ""
	}

	content := string(data)
	if len(content) == 0 {
		return ""
	}

	now := time.Now().Format("2006-01-02 15:04:05")
	return fmt.Sprintf(prompts.Heartbeat, now, content)
}

// createDefaultHeartbeatTemplate creates the default HEARTBEAT.md file
func (hs *HeartbeatService) createDefaultHeartbeatTemplate() {
	heartbeatPath := filepath.Join(hs.workspace, "HEARTBEAT.md")

	defaultContent := prompts.HeartbeatTemplate

	if err := os.WriteFile(heartbeatPath, []byte(defaultContent), 0644); err != nil {
		hs.logError("Failed to create default HEARTBEAT.md: %v", err)
	} else {
		hs.logInfo("Created default HEARTBEAT.md template")
	}
}

// sendResponse sends the heartbeat response to the last channel
func (hs *HeartbeatService) sendResponse(response string) {
	hs.mu.RLock()
	msgBus := hs.bus
	hs.mu.RUnlock()

	if msgBus == nil {
		hs.logInfo("No message bus configured, heartbeat result not sent")
		return
	}

	// Get last channel from state
	lastChannel := hs.state.GetLastChannel()
	if lastChannel == "" {
		hs.logInfo("No last channel recorded, heartbeat result not sent")
		return
	}

	platform, userID := hs.parseLastChannel(lastChannel)

	// Skip internal channels that can't receive messages
	if platform == "" || userID == "" {
		return
	}

	msgBus.PublishOutbound(bus.OutboundMessage{
		Channel: platform,
		ChatID:  userID,
		Content: response,
	})

	hs.logInfo("Heartbeat result sent to %s", platform)
}

// parseLastChannel parses the last channel string into platform and userID.
// Returns empty strings for invalid or internal channels.
func (hs *HeartbeatService) parseLastChannel(lastChannel string) (platform, userID string) {
	if lastChannel == "" {
		return "", ""
	}

	// Parse channel format: "platform:user_id" (e.g., "telegram:123456")
	parts := strings.SplitN(lastChannel, ":", 2)
	if len(parts) != 2 || parts[0] == "" || parts[1] == "" {
		hs.logError("Invalid last channel format: %s", lastChannel)
		return "", ""
	}

	platform, userID = parts[0], parts[1]

	// Skip internal channels
	if constants.IsInternalChannel(platform) {
		hs.logInfo("Skipping internal channel: %s", platform)
		return "", ""
	}

	return platform, userID
}

// logInfo logs an informational message to the heartbeat log
func (hs *HeartbeatService) logInfo(format string, args ...any) {
	hs.log("INFO", format, args...)
}

// logError logs an error message to the heartbeat log
func (hs *HeartbeatService) logError(format string, args ...any) {
	hs.log("ERROR", format, args...)
}

// log writes a message to the heartbeat log file
func (hs *HeartbeatService) log(level, format string, args ...any) {
	logFile := filepath.Join(hs.workspace, "heartbeat.log")
	f, err := os.OpenFile(logFile, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return
	}
	defer f.Close()

	timestamp := time.Now().Format("2006-01-02 15:04:05")
	fmt.Fprintf(f, "[%s] [%s] %s\n", timestamp, level, fmt.Sprintf(format, args...))
}
