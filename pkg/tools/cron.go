package tools

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"time"

	"localagent/pkg/bus"
	"localagent/pkg/cron"
)

type JobExecutor interface {
	ProcessDirectWithChannel(ctx context.Context, content, sessionKey, channel, chatID string) (string, error)
}

type CronTool struct {
	cronService  *cron.CronService
	executor     JobExecutor
	msgBus       *bus.MessageBus
	execTool     *ExecTool
	enqueueEvent func(source, message, channel, chatID string, wake bool)
	channel      string
	chatID       string
	mu           sync.RWMutex
}

func NewCronTool(cronService *cron.CronService, executor JobExecutor, msgBus *bus.MessageBus, workspace string, execTimeout time.Duration) *CronTool {
	execTool := NewExecTool(workspace)
	if execTimeout > 0 {
		execTool.SetTimeout(execTimeout)
	}
	return &CronTool{
		cronService: cronService,
		executor:    executor,
		msgBus:      msgBus,
		execTool:    execTool,
	}
}

func (t *CronTool) Name() string {
	return "cron"
}

func (t *CronTool) Description() string {
	return `Schedule jobs that trigger on a timer. Three modes:
1. deliver=true (default): sends 'message' as plain text directly to the user. No shell expansion.
2. deliver=false: sends 'message' to the agent for processing (agent can use tools to fulfill it).
3. command: runs a shell command and sends its stdout to the user. Use this for dynamic output like random selection, system info, etc.
Scheduling: use 'at_seconds' for one-shot, 'every_seconds' for intervals, or 'cron_expr' for cron schedules (server timezone).
Session routing: session_target controls how the job executes. 'main' batches the message into the next heartbeat (efficient, shared context). 'isolated' (default for deliver=false) runs its own LLM call. deliver=true defaults to 'main'.
wake_mode: 'now' triggers an immediate heartbeat when the job fires. 'next-heartbeat' (default) waits for the next scheduled heartbeat tick.`
}

func (t *CronTool) Parameters() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"action": map[string]any{
				"type":        "string",
				"enum":        []string{"add", "list", "remove", "enable", "disable"},
				"description": "Action to perform.",
			},
			"message": map[string]any{
				"type":        "string",
				"description": "Required for 'add' action. Must be a non-empty string. With deliver=true, sent as-is to user (no shell expansion). With deliver=false, sent to agent for processing.",
			},
			"command": map[string]any{
				"type":        "string",
				"description": "Shell command to execute on trigger. Its stdout is sent to the user. Use this instead of message when you need dynamic output (e.g. 'shuf -n 1 file.txt', 'date', 'curl ...').",
			},
			"at_seconds": map[string]any{
				"type":        "integer",
				"description": "One-time reminder: seconds from now when to trigger.",
			},
			"every_seconds": map[string]any{
				"type":        "integer",
				"description": "Recurring interval in seconds.",
			},
			"cron_expr": map[string]any{
				"type":        "string",
				"description": "Cron expression for complex recurring schedules.",
			},
			"job_id": map[string]any{
				"type":        "string",
				"description": "Job ID (for remove/enable/disable).",
			},
			"deliver": map[string]any{
				"type":        "boolean",
				"description": "If true (default), send message as plain text to user. If false, send message to agent for processing (agent can use tools). Ignored when 'command' is set.",
			},
			"session_target": map[string]any{
				"type":        "string",
				"enum":        []string{"main", "isolated"},
				"description": "Where to run the job. 'main' batches into heartbeat (default for deliver=true). 'isolated' runs its own LLM call (default for deliver=false).",
			},
			"wake_mode": map[string]any{
				"type":        "string",
				"enum":        []string{"now", "next-heartbeat"},
				"description": "When session_target=main: 'now' triggers immediate heartbeat, 'next-heartbeat' (default) waits for next tick.",
			},
		},
		"required": []string{"action"},
	}
}

func (t *CronTool) SetContext(channel, chatID string) {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.channel = channel
	t.chatID = chatID
}

func (t *CronTool) SetEventEnqueuer(fn func(source, message, channel, chatID string, wake bool)) {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.enqueueEvent = fn
}

func (t *CronTool) Execute(ctx context.Context, args map[string]any) *ToolResult {
	action, ok := args["action"].(string)
	if !ok {
		return ErrorResult("action is required")
	}

	switch action {
	case "add":
		return t.addJob(args)
	case "list":
		return t.listJobs()
	case "remove":
		return t.removeJob(args)
	case "enable":
		return t.enableJob(args, true)
	case "disable":
		return t.enableJob(args, false)
	default:
		return ErrorResult(fmt.Sprintf("unknown action: %s", action))
	}
}

func (t *CronTool) addJob(args map[string]any) *ToolResult {
	t.mu.RLock()
	channel := t.channel
	chatID := t.chatID
	t.mu.RUnlock()

	if channel == "" || chatID == "" {
		return ErrorResult("no session context (channel/chat_id not set)")
	}

	message, ok := args["message"].(string)
	if !ok || message == "" {
		return ErrorResult("'message' parameter is required and must be a non-empty string when action is 'add'")
	}

	var schedule cron.CronSchedule

	atSeconds, hasAt := args["at_seconds"].(float64)
	everySeconds, hasEvery := args["every_seconds"].(float64)
	cronExpr, hasCron := args["cron_expr"].(string)

	if hasAt {
		atMS := time.Now().UnixMilli() + int64(atSeconds)*1000
		schedule = cron.CronSchedule{Kind: "at", AtMS: &atMS}
	} else if hasEvery {
		everyMS := int64(everySeconds) * 1000
		schedule = cron.CronSchedule{Kind: "every", EveryMS: &everyMS}
	} else if hasCron {
		schedule = cron.CronSchedule{Kind: "cron", Expr: cronExpr}
	} else {
		return ErrorResult("one of at_seconds, every_seconds, or cron_expr is required")
	}

	deliver := true
	if d, ok := args["deliver"].(bool); ok {
		deliver = d
	}

	command, _ := args["command"].(string)
	if command != "" {
		deliver = false
	}

	sessionTarget, _ := args["session_target"].(string)
	wakeMode, _ := args["wake_mode"].(string)

	if sessionTarget == "" {
		if deliver {
			sessionTarget = "main"
		} else {
			sessionTarget = "isolated"
		}
	}
	if wakeMode == "" {
		wakeMode = "next-heartbeat"
	}

	messagePreview := message
	if len([]rune(messagePreview)) > 30 {
		messagePreview = string([]rune(messagePreview)[:27]) + "..."
	}

	job, err := t.cronService.AddJob(messagePreview, schedule, message, deliver, channel, chatID, sessionTarget, wakeMode)
	if err != nil {
		return ErrorResult(fmt.Sprintf("Error adding job: %v", err))
	}

	if command != "" {
		job.Payload.Command = command
		t.cronService.UpdateJob(job)
	}

	return SilentResult(fmt.Sprintf("Cron job added: %s (id: %s)", job.Name, job.ID))
}

func (t *CronTool) listJobs() *ToolResult {
	jobs := t.cronService.ListJobs(false)

	if len(jobs) == 0 {
		return SilentResult("No scheduled jobs")
	}

	var result strings.Builder
	fmt.Fprintf(&result, "Scheduled jobs:\n")
	for _, j := range jobs {
		var scheduleInfo string
		if j.Schedule.Kind == "every" && j.Schedule.EveryMS != nil {
			scheduleInfo = fmt.Sprintf("every %ds", *j.Schedule.EveryMS/1000)
		} else if j.Schedule.Kind == "cron" {
			scheduleInfo = j.Schedule.Expr
		} else if j.Schedule.Kind == "at" {
			scheduleInfo = "one-time"
		} else {
			scheduleInfo = "unknown"
		}
		fmt.Fprintf(&result, "- %s (id: %s, %s)\n", j.Name, j.ID, scheduleInfo)
	}

	return SilentResult(result.String())
}

func (t *CronTool) removeJob(args map[string]any) *ToolResult {
	jobID, ok := args["job_id"].(string)
	if !ok || jobID == "" {
		return ErrorResult("job_id is required for remove")
	}

	if t.cronService.RemoveJob(jobID) {
		return SilentResult(fmt.Sprintf("Cron job removed: %s", jobID))
	}
	return ErrorResult(fmt.Sprintf("Job %s not found", jobID))
}

func (t *CronTool) enableJob(args map[string]any, enable bool) *ToolResult {
	jobID, ok := args["job_id"].(string)
	if !ok || jobID == "" {
		return ErrorResult("job_id is required for enable/disable")
	}

	job := t.cronService.EnableJob(jobID, enable)
	if job == nil {
		return ErrorResult(fmt.Sprintf("Job %s not found", jobID))
	}

	status := "enabled"
	if !enable {
		status = "disabled"
	}
	return SilentResult(fmt.Sprintf("Cron job '%s' %s", job.Name, status))
}

func (t *CronTool) ExecuteJob(ctx context.Context, job *cron.CronJob) string {
	channel := job.Payload.Channel
	chatID := job.Payload.To

	if channel == "" {
		channel = "cli"
	}
	if chatID == "" {
		chatID = "direct"
	}

	t.mu.RLock()
	enqueuer := t.enqueueEvent
	t.mu.RUnlock()

	if job.SessionTarget == "main" && enqueuer != nil {
		wake := job.WakeMode == "now"
		enqueuer(fmt.Sprintf("cron:%s", job.ID), job.Payload.Message, channel, chatID, wake)
		return "ok"
	}

	if job.Payload.Command != "" {
		args := map[string]any{"command": job.Payload.Command}
		result := t.execTool.Execute(ctx, args)
		var output string
		if result.IsError {
			output = fmt.Sprintf("Error executing scheduled command: %s", result.ForLLM)
		} else {
			output = fmt.Sprintf("Scheduled command '%s' executed:\n%s", job.Payload.Command, result.ForLLM)
		}
		t.msgBus.PublishOutbound(bus.OutboundMessage{
			Channel: channel,
			ChatID:  chatID,
			Content: output,
		})
		return "ok"
	}

	if job.Payload.Deliver {
		t.msgBus.PublishOutbound(bus.OutboundMessage{
			Channel: channel,
			ChatID:  chatID,
			Content: job.Payload.Message,
		})
		return "ok"
	}

	sessionKey := fmt.Sprintf("cron-%s", job.ID)
	response, err := t.executor.ProcessDirectWithChannel(ctx, job.Payload.Message, sessionKey, channel, chatID)
	if err != nil {
		return fmt.Sprintf("Error: %v", err)
	}
	_ = response
	return "ok"
}
