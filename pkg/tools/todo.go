package tools

import (
	"context"
	"encoding/json"
	"fmt"

	"localagent/pkg/todo"
)

type TodoTool struct {
	service *todo.TodoService
}

func NewTodoTool(service *todo.TodoService) *TodoTool {
	return &TodoTool{service: service}
}

func (t *TodoTool) Name() string {
	return "tasks"
}

func (t *TodoTool) Description() string {
	return `Manage personal tasks/todos.

ACTIONS:
- list: List tasks (optional filters: status, tag)
- add: Create a task (requires title)
- update: Modify a task (requires taskId + fields to change)
- done: Mark a task as done (requires taskId). Recurring tasks auto-create the next instance.
- remove: Delete a task (requires taskId)

FIELDS:
- title: Task title (string)
- description: Optional details (string)
- priority: "low", "medium", or "high"
- due: Due date as "YYYY-MM-DD"
- recurrence: "daily", "weekly", or "monthly" (requires due date)
- tags: Array of string tags
- status: "todo", "doing", or "done"`
}

func (t *TodoTool) Parameters() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"action": map[string]any{
				"type":        "string",
				"enum":        []string{"list", "add", "update", "done", "remove"},
				"description": "Action to perform.",
			},
			"title": map[string]any{
				"type":        "string",
				"description": "Task title (for add).",
			},
			"description": map[string]any{
				"type":        "string",
				"description": "Task description (for add/update).",
			},
			"priority": map[string]any{
				"type":        "string",
				"enum":        []string{"low", "medium", "high"},
				"description": "Task priority (for add/update).",
			},
			"due": map[string]any{
				"type":        "string",
				"description": "Due date as YYYY-MM-DD (for add/update).",
			},
			"recurrence": map[string]any{
				"type":        "string",
				"enum":        []string{"daily", "weekly", "monthly"},
				"description": "Recurrence rule (for add/update).",
			},
			"tags": map[string]any{
				"type":        "array",
				"items":       map[string]any{"type": "string"},
				"description": "Tags (for add/update).",
			},
			"status": map[string]any{
				"type":        "string",
				"enum":        []string{"todo", "doing", "done"},
				"description": "Status filter (for list) or new status (for update).",
			},
			"tag": map[string]any{
				"type":        "string",
				"description": "Tag filter (for list).",
			},
			"taskId": map[string]any{
				"type":        "string",
				"description": "Task ID (for update/done/remove).",
			},
		},
		"required": []string{"action"},
	}
}

func (t *TodoTool) Execute(_ context.Context, args map[string]any) *ToolResult {
	action, ok := args["action"].(string)
	if !ok {
		return ErrorResult("action is required")
	}

	switch action {
	case "list":
		return t.listAction(args)
	case "add":
		return t.addAction(args)
	case "update":
		return t.updateAction(args)
	case "done":
		return t.doneAction(args)
	case "remove":
		return t.removeAction(args)
	default:
		return ErrorResult(fmt.Sprintf("unknown action: %s", action))
	}
}

func (t *TodoTool) listAction(args map[string]any) *ToolResult {
	status, _ := args["status"].(string)
	tag, _ := args["tag"].(string)

	tasks := t.service.ListTasks(status, tag)
	if len(tasks) == 0 {
		return SilentResult("No tasks found")
	}

	data, _ := json.MarshalIndent(tasks, "", "  ")
	return SilentResult(string(data))
}

func (t *TodoTool) addAction(args map[string]any) *ToolResult {
	title, _ := args["title"].(string)
	if title == "" {
		return ErrorResult("'title' is required for add action")
	}

	task := todo.Task{
		Title: title,
	}
	if desc, ok := args["description"].(string); ok {
		task.Description = desc
	}
	if priority, ok := args["priority"].(string); ok {
		task.Priority = priority
	}
	if due, ok := args["due"].(string); ok {
		task.Due = due
	}
	if recurrence, ok := args["recurrence"].(string); ok {
		task.Recurrence = recurrence
	}
	if tags, ok := args["tags"]; ok {
		task.Tags = toStringSliceFromAny(tags)
	}
	if status, ok := args["status"].(string); ok {
		task.Status = status
	}

	created, err := t.service.AddTask(task)
	if err != nil {
		return ErrorResult(fmt.Sprintf("error adding task: %v", err))
	}

	return SilentResult(fmt.Sprintf("Task added: %s (id: %s)", created.Title, created.ID))
}

func (t *TodoTool) updateAction(args map[string]any) *ToolResult {
	taskID, ok := args["taskId"].(string)
	if !ok || taskID == "" {
		return ErrorResult("'taskId' is required for update action")
	}

	patch := make(map[string]any)
	for _, key := range []string{"title", "description", "priority", "due", "recurrence", "status"} {
		if v, ok := args[key]; ok {
			patch[key] = v
		}
	}
	if v, ok := args["tags"]; ok {
		patch["tags"] = v
	}

	if len(patch) == 0 {
		return ErrorResult("no fields to update")
	}

	task, err := t.service.UpdateTask(taskID, patch)
	if err != nil {
		return ErrorResult(fmt.Sprintf("error updating task: %v", err))
	}

	return SilentResult(fmt.Sprintf("Task updated: %s (id: %s)", task.Title, task.ID))
}

func (t *TodoTool) doneAction(args map[string]any) *ToolResult {
	taskID, ok := args["taskId"].(string)
	if !ok || taskID == "" {
		return ErrorResult("'taskId' is required for done action")
	}

	task, err := t.service.CompleteTask(taskID)
	if err != nil {
		return ErrorResult(fmt.Sprintf("error completing task: %v", err))
	}

	return SilentResult(fmt.Sprintf("Task completed: %s (id: %s)", task.Title, task.ID))
}

func (t *TodoTool) removeAction(args map[string]any) *ToolResult {
	taskID, ok := args["taskId"].(string)
	if !ok || taskID == "" {
		return ErrorResult("'taskId' is required for remove action")
	}

	if t.service.RemoveTask(taskID) {
		return SilentResult(fmt.Sprintf("Task removed: %s", taskID))
	}
	return ErrorResult(fmt.Sprintf("task %s not found", taskID))
}

func toStringSliceFromAny(v any) []string {
	if arr, ok := v.([]any); ok {
		result := make([]string, 0, len(arr))
		for _, item := range arr {
			if s, ok := item.(string); ok {
				result = append(result, s)
			}
		}
		return result
	}
	if arr, ok := v.([]string); ok {
		return arr
	}
	return nil
}
