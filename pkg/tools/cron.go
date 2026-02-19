package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"sync"
	"time"

	"localagent/pkg/bus"
	"localagent/pkg/cron"
)

const defaultJobTimeout = 10 * time.Minute

type JobExecutor interface {
	ProcessDirectWithChannel(ctx context.Context, content, sessionKey, channel, chatID string) (string, error)
}

type EventEnqueuer func(source, message, channel, chatID string, wake bool)

type CronTool struct {
	cronService  *cron.CronService
	executor     JobExecutor
	msgBus       *bus.MessageBus
	enqueueEvent EventEnqueuer
	channel      string
	chatID       string
	mu           sync.RWMutex
}

func NewCronTool(cronService *cron.CronService, executor JobExecutor, msgBus *bus.MessageBus) *CronTool {
	return &CronTool{
		cronService: cronService,
		executor:    executor,
		msgBus:      msgBus,
	}
}

func (t *CronTool) SetEventEnqueuer(fn EventEnqueuer) {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.enqueueEvent = fn
}

func (t *CronTool) Name() string {
	return "cron"
}

func (t *CronTool) Description() string {
	return `Manage cron jobs (status/list/add/update/remove/run) and send wake events.

ACTIONS:
- status: Check cron scheduler status
- list: List jobs (use includeDisabled:true to include disabled)
- add: Create job (requires job object, see schema below)
- update: Modify job (requires jobId + patch object)
- remove: Delete job (requires jobId)
- run: Trigger job immediately (requires jobId)
- wake: Send wake event (requires text, optional mode)

JOB SCHEMA (for add action):
{
  "name": "string (optional)",
  "schedule": { ... },
  "payload": { ... },
  "delivery": { ... },
  "sessionTarget": "main" | "isolated",
  "enabled": true | false
}

SCHEDULE TYPES (schedule.kind):
- "at": One-shot at absolute time
  { "kind": "at", "at": "<ISO-8601 timestamp>" }
- "every": Recurring interval
  { "kind": "every", "everyMs": <ms> }
- "cron": Cron expression
  { "kind": "cron", "expr": "<expression>", "tz": "<optional-timezone>" }

PAYLOAD TYPES (payload.kind):
- "systemEvent": Injects text as system event into session
  { "kind": "systemEvent", "text": "<message>" }
- "agentTurn": Runs agent with message (isolated sessions only)
  { "kind": "agentTurn", "message": "<prompt>" }

DELIVERY (top-level):
  { "mode": "none|announce", "channel": "<optional>", "to": "<optional>" }
  Default for isolated agentTurn jobs: "announce"

CRITICAL CONSTRAINTS:
- sessionTarget="main" REQUIRES payload.kind="systemEvent"
- sessionTarget="isolated" REQUIRES payload.kind="agentTurn"
Default: prefer isolated agentTurn jobs unless the user explicitly wants a main-session system event.

WAKE MODES (for wake action):
- "next-heartbeat" (default): Wake on next heartbeat
- "now": Wake immediately`
}

func (t *CronTool) Parameters() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"action": map[string]any{
				"type":        "string",
				"enum":        []string{"status", "list", "add", "update", "remove", "run", "wake"},
				"description": "Action to perform.",
			},
			"includeDisabled": map[string]any{
				"type":        "boolean",
				"description": "For list: include disabled jobs.",
			},
			"job": map[string]any{
				"type":                 "object",
				"description":          "Job object for add action.",
				"additionalProperties": true,
			},
			"jobId": map[string]any{
				"type":        "string",
				"description": "Job ID for update/remove/run.",
			},
			"patch": map[string]any{
				"type":                 "object",
				"description":          "Patch object for update action.",
				"additionalProperties": true,
			},
			"text": map[string]any{
				"type":        "string",
				"description": "Text for wake action.",
			},
			"mode": map[string]any{
				"type":        "string",
				"enum":        []string{"now", "next-heartbeat"},
				"description": "Wake mode for wake action.",
			},
			"runMode": map[string]any{
				"type":        "string",
				"enum":        []string{"due", "force"},
				"description": "Run mode for run action.",
			},
		},
		"required": []string{"action"},
	}
}

// jobKeys are known CronJob fields. When the LLM flattens job.* to the top
// level (common with smaller models), we detect these keys and re-wrap them.
var jobKeys = map[string]bool{
	"name": true, "description": true, "schedule": true, "payload": true,
	"delivery": true, "sessionTarget": true, "wakeMode": true, "enabled": true,
}

// recoverFlatJobParams checks if the LLM flattened job fields to the top level
// and wraps them back into a "job" object if so.
func recoverFlatJobParams(args map[string]any) map[string]any {
	if _, hasJob := args["job"]; hasJob {
		return args
	}
	job := map[string]any{}
	for k, v := range args {
		if jobKeys[k] {
			job[k] = v
		}
	}
	if len(job) > 0 {
		args["job"] = job
	}
	return args
}

func (t *CronTool) SetContext(channel, chatID string) {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.channel = channel
	t.chatID = chatID
}

func (t *CronTool) Execute(ctx context.Context, args map[string]any) *ToolResult {
	action, ok := args["action"].(string)
	if !ok {
		return ErrorResult("action is required")
	}

	switch action {
	case "status":
		return t.statusAction()
	case "list":
		return t.listAction(args)
	case "add":
		return t.addAction(args)
	case "update":
		return t.updateAction(args)
	case "remove":
		return t.removeAction(args)
	case "run":
		return t.runAction(args)
	case "wake":
		return t.wakeAction(args)
	default:
		return ErrorResult(fmt.Sprintf("unknown action: %s", action))
	}
}

func (t *CronTool) statusAction() *ToolResult {
	status := t.cronService.Status()
	data, _ := json.MarshalIndent(status, "", "  ")
	return SilentResult(string(data))
}

func (t *CronTool) listAction(args map[string]any) *ToolResult {
	includeDisabled, _ := args["includeDisabled"].(bool)
	jobs := t.cronService.ListJobs(includeDisabled)

	if len(jobs) == 0 {
		return SilentResult("No scheduled jobs")
	}

	data, _ := json.MarshalIndent(jobs, "", "  ")
	return SilentResult(string(data))
}

func (t *CronTool) addAction(args map[string]any) *ToolResult {
	args = recoverFlatJobParams(args)

	t.mu.RLock()
	channel := t.channel
	chatID := t.chatID
	t.mu.RUnlock()

	jobRaw, ok := args["job"].(map[string]any)
	if !ok {
		return ErrorResult("'job' object is required for add action")
	}

	data, err := json.Marshal(jobRaw)
	if err != nil {
		return ErrorResult(fmt.Sprintf("invalid job object: %v", err))
	}

	var job cron.CronJob
	if err := json.Unmarshal(data, &job); err != nil {
		return ErrorResult(fmt.Sprintf("failed to parse job: %v", err))
	}

	if job.SessionTarget == "" {
		if job.Payload.Kind == "systemEvent" {
			job.SessionTarget = "main"
		} else {
			job.SessionTarget = "isolated"
		}
	}
	if job.WakeMode == "" {
		job.WakeMode = "now"
	}

	if job.Delivery == nil {
		mode := "none"
		if job.SessionTarget == "isolated" && job.Payload.Kind == "agentTurn" {
			mode = "announce"
		}
		job.Delivery = &cron.CronDelivery{
			Mode:    mode,
			Channel: channel,
			To:      chatID,
		}
	} else {
		if job.Delivery.Channel == "" {
			job.Delivery.Channel = channel
		}
		if job.Delivery.To == "" {
			job.Delivery.To = chatID
		}
	}

	created, err := t.cronService.AddJob(job)
	if err != nil {
		return ErrorResult(fmt.Sprintf("error adding job: %v", err))
	}

	return SilentResult(fmt.Sprintf("Cron job added: %s (id: %s)", created.Name, created.ID))
}

func (t *CronTool) updateAction(args map[string]any) *ToolResult {
	jobID, ok := args["jobId"].(string)
	if !ok || jobID == "" {
		return ErrorResult("'jobId' is required for update action")
	}

	patch, ok := args["patch"].(map[string]any)
	if !ok {
		return ErrorResult("'patch' object is required for update action")
	}

	job, err := t.cronService.PatchJob(jobID, patch)
	if err != nil {
		return ErrorResult(fmt.Sprintf("error updating job: %v", err))
	}

	return SilentResult(fmt.Sprintf("Cron job updated: %s (id: %s)", job.Name, job.ID))
}

func (t *CronTool) removeAction(args map[string]any) *ToolResult {
	jobID, ok := args["jobId"].(string)
	if !ok || jobID == "" {
		return ErrorResult("'jobId' is required for remove action")
	}

	if t.cronService.RemoveJob(jobID) {
		return SilentResult(fmt.Sprintf("Cron job removed: %s", jobID))
	}
	return ErrorResult(fmt.Sprintf("Job %s not found", jobID))
}

func (t *CronTool) runAction(args map[string]any) *ToolResult {
	jobID, ok := args["jobId"].(string)
	if !ok || jobID == "" {
		return ErrorResult("'jobId' is required for run action")
	}

	runMode, _ := args["runMode"].(string)
	force := runMode == "force"

	if err := t.cronService.RunJob(jobID, force); err != nil {
		return ErrorResult(fmt.Sprintf("error running job: %v", err))
	}

	return SilentResult(fmt.Sprintf("Job %s triggered", jobID))
}

func (t *CronTool) wakeAction(args map[string]any) *ToolResult {
	text, _ := args["text"].(string)
	if text == "" {
		return ErrorResult("'text' is required for wake action")
	}

	mode, _ := args["mode"].(string)
	if mode == "" {
		mode = "next-heartbeat"
	}

	t.mu.RLock()
	channel := t.channel
	chatID := t.chatID
	t.mu.RUnlock()

	t.mu.RLock()
	enqueuer := t.enqueueEvent
	t.mu.RUnlock()

	if enqueuer == nil {
		return ErrorResult("event queue not configured")
	}

	enqueuer("wake", text, channel, chatID, mode == "now")

	return SilentResult(fmt.Sprintf("Wake event enqueued (mode: %s)", mode))
}

func (t *CronTool) ExecuteJob(ctx context.Context, job *cron.CronJob) string {
	timeout := defaultJobTimeout
	if job.Payload.TimeoutSeconds > 0 {
		timeout = time.Duration(job.Payload.TimeoutSeconds) * time.Second
	}
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	channel := ""
	chatID := ""
	if job.Delivery != nil {
		channel = job.Delivery.Channel
		chatID = job.Delivery.To
	}
	if channel == "" {
		channel = "cli"
	}
	if chatID == "" {
		chatID = "direct"
	}

	if job.Payload.Kind == "systemEvent" {
		t.mu.RLock()
		enqueuer := t.enqueueEvent
		t.mu.RUnlock()

		if enqueuer != nil {
			wake := job.WakeMode == "now"
			enqueuer(fmt.Sprintf("cron:%s", job.ID), job.Payload.Text, channel, chatID, wake)
		}
		return "ok"
	}

	if job.Payload.Kind == "agentTurn" {
		sessionKey := fmt.Sprintf("cron-%s", job.ID)
		response, err := t.executor.ProcessDirectWithChannel(ctx, job.Payload.Message, sessionKey, channel, chatID)
		if err != nil {
			return fmt.Sprintf("Error: %v", err)
		}

		if job.Delivery != nil && job.Delivery.Mode == "announce" && response != "" {
			t.announceResult(channel, chatID, job, response)
		}

		return "ok"
	}

	return fmt.Sprintf("unknown payload kind: %s", job.Payload.Kind)
}

func (t *CronTool) announceResult(channel, chatID string, job *cron.CronJob, response string) {
	var content strings.Builder
	if job.Name != "" {
		fmt.Fprintf(&content, "[cron: %s] %s", job.Name, response)
	} else {
		content.WriteString(response)
	}

	t.msgBus.PublishOutbound(bus.OutboundMessage{
		Channel: channel,
		ChatID:  chatID,
		Content: content.String(),
	})
}
