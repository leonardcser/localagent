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
	dedupWindow            = 24 * time.Hour
)

// ActiveHours defines a time window during which heartbeats are allowed.
type ActiveHours struct {
	Start    string // "HH:MM"
	End      string // "HH:MM"
	Timezone string // IANA timezone, e.g. "America/New_York"
}

// HeartbeatHandler is the function type for handling heartbeat.
// It returns a ToolResult that can indicate async operations.
// channel and chatID are derived from the last active user channel.
// isCronEvent indicates the prompt is a cron-triggered event (not a periodic heartbeat).
type HeartbeatHandler func(prompt, channel, chatID string, isCronEvent bool) *tools.ToolResult

// HeartbeatService manages periodic heartbeat checks
type HeartbeatService struct {
	workspace  string
	bus        *bus.MessageBus
	state      *state.Manager
	handler    HeartbeatHandler
	eventQueue *EventQueue
	interval   time.Duration
	enabled    bool
	mu         sync.RWMutex
	stopChan   chan struct{}

	// Active hours gating
	activeHours *ActiveHours

	// Deduplication: suppress identical alerts within dedupWindow
	lastAlertText   string
	lastAlertSentAt time.Time
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

// SetEventQueue sets the event queue for receiving cron events.
func (hs *HeartbeatService) SetEventQueue(eq *EventQueue) {
	hs.mu.Lock()
	defer hs.mu.Unlock()
	hs.eventQueue = eq
}

// SetActiveHours configures the active hours window.
// Heartbeats outside this window are skipped (cron events still go through).
func (hs *HeartbeatService) SetActiveHours(ah *ActiveHours) {
	hs.mu.Lock()
	defer hs.mu.Unlock()
	hs.activeHours = ah
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

	var wakeChan <-chan struct{}
	hs.mu.RLock()
	if hs.eventQueue != nil {
		wakeChan = hs.eventQueue.WakeChan()
	}
	hs.mu.RUnlock()

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
		case <-wakeChan:
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

	hp := hs.buildPrompt()
	if hp.text == "" {
		logger.Info("heartbeat: no prompt (HEARTBEAT.md empty or missing)")
		return
	}

	// Active hours gate: skip periodic heartbeats outside the window.
	// Cron events always go through regardless of active hours.
	if !hp.isCronEvent && !hs.isWithinActiveHours() {
		hs.logInfo("Skipped: outside active hours")
		return
	}

	if handler == nil {
		hs.logError("Heartbeat handler not configured")
		return
	}

	// Resolve delivery channel: prefer event-provided values, fall back to lastChannel
	channel, chatID := hp.channel, hp.chatID
	if channel == "" || chatID == "" {
		lastChannel := hs.state.GetLastChannel()
		channel, chatID = hs.parseLastChannel(lastChannel)
		hs.logInfo("Resolved channel: %s, chatID: %s (from lastChannel: %s)", channel, chatID, lastChannel)
	} else {
		hs.logInfo("Using event channel: %s, chatID: %s", channel, chatID)
	}

	result := handler(hp.text, channel, chatID, hp.isCronEvent)

	if result == nil {
		hs.logInfo("Heartbeat handler returned nil result")
		return
	}

	if result.IsError {
		hs.logError("Heartbeat error: %s", result.ForLLM)
		return
	}

	if result.Async {
		hs.logInfo("Async task started: %s", result.ForLLM)
		logger.Info("heartbeat: async task started: %s", result.ForLLM)
		return
	}

	// For cron events, always deliver (skip the silent check)
	if hp.isCronEvent {
		response := result.ForUser
		if response == "" {
			response = result.ForLLM
		}
		if response != "" {
			hs.sendResponseTo(channel, chatID, response)
		}
		hs.logInfo("Cron event delivered: %s", result.ForLLM)
		return
	}

	// Regular heartbeat: respect silent flag
	if result.Silent {
		hs.logInfo("Heartbeat OK - silent")
		return
	}

	response := result.ForUser
	if response == "" {
		response = result.ForLLM
	}

	if response == "" {
		return
	}

	// Deduplication: suppress identical alerts within the window
	if hs.isDuplicate(response) {
		hs.logInfo("Suppressed duplicate alert: %s", response)
		return
	}

	hs.recordAlert(response)
	hs.sendResponse(response)
	hs.logInfo("Heartbeat completed: %s", result.ForLLM)
}

const heartbeatToken = "HEARTBEAT_OK"
const maxAckChars = 300

// StripHeartbeatToken removes the HEARTBEAT_OK token from a response.
// Returns shouldSkip=true if the remaining text is short enough to be
// just an acknowledgement (<=300 chars), meaning nothing to deliver.
func StripHeartbeatToken(raw string) (text string, shouldSkip bool) {
	stripped := strings.ReplaceAll(raw, heartbeatToken, "")
	stripped = strings.TrimSpace(stripped)
	// Trim trailing punctuation that often follows the token
	stripped = strings.TrimRight(stripped, ".!,;:")
	stripped = strings.TrimSpace(stripped)

	if stripped == "" {
		return "", true
	}
	if len([]rune(stripped)) <= maxAckChars {
		return stripped, true
	}
	return stripped, false
}

// isHeartbeatContentEffectivelyEmpty returns true if the content is only
// whitespace, markdown headers, or empty list items — meaning there are no
// real tasks for the heartbeat to process.
func isHeartbeatContentEffectivelyEmpty(content string) bool {
	for _, line := range strings.Split(content, "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		if strings.HasPrefix(line, "#") {
			continue
		}
		if line == "-" || line == "*" || line == "+" {
			continue
		}
		if strings.HasPrefix(line, "- ") || strings.HasPrefix(line, "* ") || strings.HasPrefix(line, "+ ") {
			rest := strings.TrimSpace(line[2:])
			if rest == "" {
				continue
			}
		}
		return false
	}
	return true
}

// RequestWakeNow triggers an immediate heartbeat with the given event text.
func (hs *HeartbeatService) RequestWakeNow(text string) {
	hs.mu.RLock()
	eq := hs.eventQueue
	hs.mu.RUnlock()

	if eq == nil {
		return
	}

	eq.EnqueueAndWake(Event{
		Source:  "wake",
		Message: text,
	})
}

type heartbeatPrompt struct {
	text        string
	isCronEvent bool
	channel     string
	chatID      string
}

// buildPrompt builds the heartbeat prompt from HEARTBEAT.md and pending events.
func (hs *HeartbeatService) buildPrompt() heartbeatPrompt {
	heartbeatPath := filepath.Join(hs.workspace, "HEARTBEAT.md")

	var staticContent string
	data, err := os.ReadFile(heartbeatPath)
	if err != nil {
		if os.IsNotExist(err) {
			hs.createDefaultHeartbeatTemplate()
		} else {
			hs.logError("Error reading HEARTBEAT.md: %v", err)
		}
	} else {
		staticContent = strings.TrimSpace(string(data))
	}

	hs.mu.RLock()
	eq := hs.eventQueue
	hs.mu.RUnlock()

	var events []Event
	if eq != nil {
		events = eq.Drain()
	}

	if len(events) > 0 {
		// Use channel/chatID from the first event (all events in a batch
		// typically share the same origin).
		return heartbeatPrompt{
			text:        hs.buildCronEventPrompt(events),
			isCronEvent: true,
			channel:     events[0].Channel,
			chatID:      events[0].ChatID,
		}
	}

	if staticContent == "" || isHeartbeatContentEffectivelyEmpty(staticContent) {
		return heartbeatPrompt{}
	}

	now := time.Now()
	tz, _ := now.Zone()
	return heartbeatPrompt{
		text: fmt.Sprintf("%s\n\nCurrent time: %s (%s)", prompts.Heartbeat, now.Format("2006-01-02 15:04:05"), tz),
	}
}

// buildCronEventPrompt builds a prompt for cron-triggered events.
func (hs *HeartbeatService) buildCronEventPrompt(events []Event) string {
	var content strings.Builder
	for i, e := range events {
		if i > 0 {
			content.WriteString("\n\n")
		}
		content.WriteString(e.Message)
	}

	now := time.Now()
	tz, _ := now.Zone()
	return fmt.Sprintf("A scheduled reminder has been triggered. The reminder content is:\n\n%s\n\nPlease relay this reminder to the user in a helpful and friendly way.\n\nCurrent time: %s (%s)",
		content.String(), now.Format("2006-01-02 15:04:05"), tz)
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

// --- Active hours ---

// isWithinActiveHours checks whether the current time falls inside the
// configured active hours window. Returns true if no window is configured.
func (hs *HeartbeatService) isWithinActiveHours() bool {
	hs.mu.RLock()
	ah := hs.activeHours
	hs.mu.RUnlock()

	if ah == nil || ah.Start == "" || ah.End == "" {
		return true
	}

	loc := time.UTC
	if ah.Timezone != "" {
		var err error
		loc, err = time.LoadLocation(ah.Timezone)
		if err != nil {
			hs.logError("Invalid active_hours timezone %q: %v", ah.Timezone, err)
			return true
		}
	}

	now := time.Now().In(loc)
	cur := now.Hour()*60 + now.Minute()

	start := parseTimeMinutes(ah.Start)
	end := parseTimeMinutes(ah.End)
	if start < 0 || end < 0 {
		hs.logError("Invalid active_hours start/end: %s-%s", ah.Start, ah.End)
		return true
	}

	if start <= end {
		return cur >= start && cur < end
	}
	// Overnight window (e.g. 22:00–06:00)
	return cur >= start || cur < end
}

// parseTimeMinutes parses "HH:MM" into minutes since midnight. Returns -1 on error.
func parseTimeMinutes(t string) int {
	parts := strings.SplitN(t, ":", 2)
	if len(parts) != 2 {
		return -1
	}
	var h, m int
	if _, err := fmt.Sscanf(parts[0], "%d", &h); err != nil {
		return -1
	}
	if _, err := fmt.Sscanf(parts[1], "%d", &m); err != nil {
		return -1
	}
	if h < 0 || h > 23 || m < 0 || m > 59 {
		return -1
	}
	return h*60 + m
}

// --- Deduplication ---

// isDuplicate returns true if the response is identical to the last alert
// and was sent within the dedup window.
func (hs *HeartbeatService) isDuplicate(text string) bool {
	if hs.lastAlertText == "" {
		return false
	}
	return text == hs.lastAlertText && time.Since(hs.lastAlertSentAt) < dedupWindow
}

// recordAlert stores the alert text and timestamp for dedup comparison.
func (hs *HeartbeatService) recordAlert(text string) {
	hs.lastAlertText = text
	hs.lastAlertSentAt = time.Now()
}

// --- Response delivery ---

// sendResponse sends the heartbeat response to the last active channel.
func (hs *HeartbeatService) sendResponse(response string) {
	lastChannel := hs.state.GetLastChannel()
	if lastChannel == "" {
		hs.logInfo("No last channel recorded, heartbeat result not sent")
		return
	}
	platform, userID := hs.parseLastChannel(lastChannel)
	hs.sendResponseTo(platform, userID, response)
}

// sendResponseTo sends a response to a specific channel/chatID.
func (hs *HeartbeatService) sendResponseTo(channel, chatID, response string) {
	hs.mu.RLock()
	msgBus := hs.bus
	hs.mu.RUnlock()

	if msgBus == nil {
		hs.logInfo("No message bus configured, heartbeat result not sent")
		return
	}

	if channel == "" || chatID == "" {
		return
	}

	msgBus.PublishOutbound(bus.OutboundMessage{
		Channel: channel,
		ChatID:  chatID,
		Content: response,
	})

	hs.logInfo("Heartbeat result sent to %s:%s", channel, chatID)
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

// --- Logging ---

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
