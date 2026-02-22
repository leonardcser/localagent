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
- add: Create a task (requires task object with title)
- update: Modify a task (requires taskId + patch object)
- done: Mark a task as done (requires taskId). Recurring tasks auto-create the next instance.
- remove: Delete a task (requires taskId)

TASK SCHEMA (for add action):
{
  "title": "string",
  "description": "string (optional)",
  "priority": "low" | "medium" | "high",
  "due": "YYYY-MM-DD",
  "recurrence": "daily" | "weekly" | "monthly",
  "tags": ["string"],
  "status": "todo" | "doing" | "done"
}

PATCH SCHEMA (for update action):
Same fields as task schema, all optional. Only provided fields are changed.`
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
			"task": map[string]any{
				"type":                 "object",
				"description":          "Task object for add action.",
				"additionalProperties": true,
			},
			"taskId": map[string]any{
				"type":        "string",
				"description": "Task ID for update/done/remove.",
			},
			"patch": map[string]any{
				"type":                 "object",
				"description":          "Patch object for update action.",
				"additionalProperties": true,
			},
			"status": map[string]any{
				"type":        "string",
				"enum":        []string{"todo", "doing", "done"},
				"description": "Status filter for list action.",
			},
			"tag": map[string]any{
				"type":        "string",
				"description": "Tag filter for list action.",
			},
		},
		"required": []string{"action"},
	}
}

// taskKeys are known Task fields. When the LLM flattens task/patch fields to
// the top level, we detect these keys and re-wrap them.
var taskKeys = map[string]bool{
	"title": true, "description": true, "priority": true,
	"due": true, "recurrence": true, "tags": true, "status": true,
}

// recoverFlatTaskParams checks if the LLM flattened task/patch fields to the
// top level and wraps them back into the appropriate object.
func recoverFlatTaskParams(action string, args map[string]any) map[string]any {
	target := "task"
	if action == "update" {
		target = "patch"
	}
	if _, has := args[target]; has {
		return args
	}
	obj := map[string]any{}
	for k, v := range args {
		if taskKeys[k] {
			obj[k] = v
		}
	}
	if len(obj) > 0 {
		args[target] = obj
	}
	return args
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
		args = recoverFlatTaskParams(action, args)
		return t.addAction(args)
	case "update":
		args = recoverFlatTaskParams(action, args)
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
	taskRaw, ok := args["task"].(map[string]any)
	if !ok {
		return ErrorResult("'task' object is required for add action")
	}

	title, _ := taskRaw["title"].(string)
	if title == "" {
		return ErrorResult("'title' is required in task object")
	}

	task := todo.Task{Title: title}
	if v, ok := taskRaw["description"].(string); ok {
		task.Description = v
	}
	if v, ok := taskRaw["priority"].(string); ok {
		task.Priority = v
	}
	if v, ok := taskRaw["due"].(string); ok {
		task.Due = v
	}
	if v, ok := taskRaw["recurrence"].(string); ok {
		task.Recurrence = v
	}
	if v, ok := taskRaw["tags"]; ok {
		task.Tags = toStringSliceFromAny(v)
	}
	if v, ok := taskRaw["status"].(string); ok {
		task.Status = v
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

	patch, ok := args["patch"].(map[string]any)
	if !ok || len(patch) == 0 {
		return ErrorResult("'patch' object is required for update action")
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
