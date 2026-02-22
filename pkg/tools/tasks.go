package tools

import (
	"context"
	"encoding/json"
	"fmt"

	"localagent/pkg/todo"
)

type baseTodoTool struct {
	service *todo.TodoService
}

// --- list_tasks ---

type ListTasksTool struct{ baseTodoTool }

func NewListTasksTool(service *todo.TodoService) *ListTasksTool {
	return &ListTasksTool{baseTodoTool{service}}
}

func (t *ListTasksTool) Name() string        { return "list_tasks" }
func (t *ListTasksTool) Description() string { return "List personal tasks/todos. Optionally filter by status or tag." }

func (t *ListTasksTool) Parameters() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"status": map[string]any{
				"type":        "string",
				"enum":        []string{"todo", "doing", "done"},
				"description": "Filter by status.",
			},
			"tag": map[string]any{
				"type":        "string",
				"description": "Filter by tag.",
			},
		},
	}
}

func (t *ListTasksTool) Execute(_ context.Context, args map[string]any) *ToolResult {
	status, _ := args["status"].(string)
	tag, _ := args["tag"].(string)

	tasks := t.service.ListTasks(status, tag)
	if len(tasks) == 0 {
		return SilentResult("No tasks found")
	}

	data, _ := json.MarshalIndent(tasks, "", "  ")
	return SilentResult(string(data))
}

// --- add_task ---

type AddTaskTool struct{ baseTodoTool }

func NewAddTaskTool(service *todo.TodoService) *AddTaskTool {
	return &AddTaskTool{baseTodoTool{service}}
}

func (t *AddTaskTool) Name() string        { return "add_task" }
func (t *AddTaskTool) Description() string { return "Create a new personal task/todo." }

func (t *AddTaskTool) Parameters() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"title": map[string]any{
				"type":        "string",
				"description": "Task title.",
			},
			"description": map[string]any{
				"type":        "string",
				"description": "Task description with details.",
			},
			"priority": map[string]any{
				"type":        "string",
				"enum":        []string{"low", "medium", "high"},
				"description": "Task priority.",
			},
			"due": map[string]any{
				"type":        "string",
				"description": "Due date as YYYY-MM-DD.",
			},
			"recurrence": map[string]any{
				"type":        "string",
				"enum":        []string{"daily", "weekly", "monthly"},
				"description": "Recurrence rule (requires due date).",
			},
			"tags": map[string]any{
				"type":        "array",
				"items":       map[string]any{"type": "string"},
				"description": "Tags for categorization.",
			},
		},
		"required": []string{"title"},
	}
}

func (t *AddTaskTool) Execute(_ context.Context, args map[string]any) *ToolResult {
	title, _ := args["title"].(string)
	if title == "" {
		return ErrorResult("'title' is required")
	}

	task := todo.Task{Title: title}
	if v, ok := args["description"].(string); ok {
		task.Description = v
	}
	if v, ok := args["priority"].(string); ok {
		task.Priority = v
	}
	if v, ok := args["due"].(string); ok {
		task.Due = v
	}
	if v, ok := args["recurrence"].(string); ok {
		task.Recurrence = v
	}
	if v, ok := args["tags"]; ok {
		task.Tags = toStringSliceFromAny(v)
	}

	created, err := t.service.AddTask(task)
	if err != nil {
		return ErrorResult(fmt.Sprintf("error adding task: %v", err))
	}

	data, _ := json.MarshalIndent(created, "", "  ")
	return SilentResult(string(data))
}

// --- update_task ---

type UpdateTaskTool struct{ baseTodoTool }

func NewUpdateTaskTool(service *todo.TodoService) *UpdateTaskTool {
	return &UpdateTaskTool{baseTodoTool{service}}
}

func (t *UpdateTaskTool) Name() string        { return "update_task" }
func (t *UpdateTaskTool) Description() string { return "Update an existing task's fields." }

func (t *UpdateTaskTool) Parameters() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"taskId": map[string]any{
				"type":        "string",
				"description": "Task ID to update.",
			},
			"title": map[string]any{
				"type":        "string",
				"description": "New title.",
			},
			"description": map[string]any{
				"type":        "string",
				"description": "New description.",
			},
			"priority": map[string]any{
				"type":        "string",
				"enum":        []string{"low", "medium", "high"},
				"description": "New priority.",
			},
			"due": map[string]any{
				"type":        "string",
				"description": "New due date as YYYY-MM-DD.",
			},
			"recurrence": map[string]any{
				"type":        "string",
				"enum":        []string{"daily", "weekly", "monthly"},
				"description": "New recurrence rule.",
			},
			"status": map[string]any{
				"type":        "string",
				"enum":        []string{"todo", "doing", "done"},
				"description": "New status.",
			},
			"tags": map[string]any{
				"type":        "array",
				"items":       map[string]any{"type": "string"},
				"description": "New tags.",
			},
		},
		"required": []string{"taskId"},
	}
}

func (t *UpdateTaskTool) Execute(_ context.Context, args map[string]any) *ToolResult {
	taskID, ok := args["taskId"].(string)
	if !ok || taskID == "" {
		return ErrorResult("'taskId' is required")
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

	data, _ := json.MarshalIndent(task, "", "  ")
	return SilentResult(string(data))
}

// --- complete_task ---

type CompleteTaskTool struct{ baseTodoTool }

func NewCompleteTaskTool(service *todo.TodoService) *CompleteTaskTool {
	return &CompleteTaskTool{baseTodoTool{service}}
}

func (t *CompleteTaskTool) Name() string { return "complete_task" }
func (t *CompleteTaskTool) Description() string {
	return "Mark a task as done. Recurring tasks auto-create the next instance."
}

func (t *CompleteTaskTool) Parameters() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"taskId": map[string]any{
				"type":        "string",
				"description": "Task ID to complete.",
			},
		},
		"required": []string{"taskId"},
	}
}

func (t *CompleteTaskTool) Execute(_ context.Context, args map[string]any) *ToolResult {
	taskID, ok := args["taskId"].(string)
	if !ok || taskID == "" {
		return ErrorResult("'taskId' is required")
	}

	task, err := t.service.CompleteTask(taskID)
	if err != nil {
		return ErrorResult(fmt.Sprintf("error completing task: %v", err))
	}

	return SilentResult(fmt.Sprintf("Task completed: %s (id: %s)", task.Title, task.ID))
}

// --- remove_task ---

type RemoveTaskTool struct{ baseTodoTool }

func NewRemoveTaskTool(service *todo.TodoService) *RemoveTaskTool {
	return &RemoveTaskTool{baseTodoTool{service}}
}

func (t *RemoveTaskTool) Name() string        { return "remove_task" }
func (t *RemoveTaskTool) Description() string { return "Delete a task." }

func (t *RemoveTaskTool) Parameters() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"taskId": map[string]any{
				"type":        "string",
				"description": "Task ID to remove.",
			},
		},
		"required": []string{"taskId"},
	}
}

func (t *RemoveTaskTool) Execute(_ context.Context, args map[string]any) *ToolResult {
	taskID, ok := args["taskId"].(string)
	if !ok || taskID == "" {
		return ErrorResult("'taskId' is required")
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
