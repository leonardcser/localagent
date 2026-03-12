package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"localagent/pkg/todo"
)

type baseTodoTool struct {
	service *todo.TodoService
}

// --- query_tasks ---

type QueryTasksTool struct{ baseTodoTool }

func NewQueryTasksTool(service *todo.TodoService) *QueryTasksTool {
	return &QueryTasksTool{baseTodoTool{service}}
}

func (t *QueryTasksTool) Name() string { return "query_tasks" }
func (t *QueryTasksTool) Description() string {
	return "Query tasks with rich filtering. Also retrieves blocks and links when requested. Use with no params to list all active tasks."
}

func (t *QueryTasksTool) Parameters() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"id": map[string]any{
				"type":        "string",
				"description": "Get a single task by ID. When set, other filters are ignored.",
			},
			"status": map[string]any{
				"type":        "string",
				"enum":        []string{"todo", "doing", "done"},
				"description": "Filter by status.",
			},
			"priority": map[string]any{
				"type":        "string",
				"enum":        []string{"low", "medium", "high"},
				"description": "Filter by priority.",
			},
			"tag": map[string]any{
				"type":        "string",
				"description": "Filter by tag.",
			},
			"parentId": map[string]any{
				"type":        "string",
				"description": "Filter by parent task ID. Use 'none' for top-level tasks only.",
			},
			"search": map[string]any{
				"type":        "string",
				"description": "Search in title and description (case-insensitive).",
			},
			"dueAfter": map[string]any{
				"type":        "string",
				"description": "Only tasks with due date >= this (YYYY-MM-DD).",
			},
			"dueBefore": map[string]any{
				"type":        "string",
				"description": "Only tasks with due date <= this (YYYY-MM-DD).",
			},
			"limit": map[string]any{
				"type":        "number",
				"description": "Max number of results.",
			},
			"include": map[string]any{
				"type":        "array",
				"items":       map[string]any{"type": "string", "enum": []string{"blocks", "links"}},
				"description": "Include related entities: 'blocks' (time blocks) and/or 'links' (saved links).",
			},
		},
	}
}

type queryResult struct {
	Tasks  []todo.Task  `json:"tasks"`
	Blocks []todo.Block `json:"blocks,omitempty"`
	Links  []todo.Link  `json:"links,omitempty"`
}

func (t *QueryTasksTool) Execute(_ context.Context, args map[string]any) *ToolResult {
	q := todo.TaskQuery{}

	if v, ok := args["id"].(string); ok {
		q.ID = v
	}
	if v, ok := args["status"].(string); ok {
		q.Status = v
	}
	if v, ok := args["priority"].(string); ok {
		q.Priority = v
	}
	if v, ok := args["tag"].(string); ok {
		q.Tag = v
	}
	if v, ok := args["parentId"].(string); ok {
		q.ParentID = v
	}
	if v, ok := args["search"].(string); ok {
		q.Search = v
	}
	if v, ok := args["dueAfter"].(string); ok {
		q.DueAfter = v
	}
	if v, ok := args["dueBefore"].(string); ok {
		q.DueBefore = v
	}
	if v, ok := args["limit"].(float64); ok {
		q.Limit = int(v)
	}

	tasks := t.service.QueryTasks(q)

	result := queryResult{Tasks: tasks}
	if result.Tasks == nil {
		result.Tasks = []todo.Task{}
	}

	includes := toStringSliceFromAny(args["include"])
	for _, inc := range includes {
		switch inc {
		case "blocks":
			result.Blocks = t.service.ListBlocks("", 0, 0)
		case "links":
			result.Links = t.service.ListLinks("")
		}
	}

	data, _ := json.MarshalIndent(result, "", "  ")
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
				"description": "RFC 5545 RRULE string, e.g. 'FREQ=DAILY', 'FREQ=WEEKLY;BYDAY=MO,WE,FR', 'FREQ=MONTHLY;BYMONTHDAY=1'. Requires a due date.",
			},
			"tags": map[string]any{
				"type":        "array",
				"items":       map[string]any{"type": "string"},
				"description": "Tags for categorization.",
			},
			"parentId": map[string]any{
				"type":        "string",
				"description": "Parent task ID to create this as a subtask.",
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
	if v, ok := args["parentId"].(string); ok {
		task.ParentID = v
	}

	created, err := t.service.AddTask(task)
	if err != nil {
		return ErrorResult(fmt.Sprintf("error adding task: %v", err))
	}

	data, _ := json.MarshalIndent(created, "", "  ")
	return SilentResult(string(data))
}

// --- modify_tasks ---

type ModifyTasksTool struct{ baseTodoTool }

func NewModifyTasksTool(service *todo.TodoService) *ModifyTasksTool {
	return &ModifyTasksTool{baseTodoTool{service}}
}

func (t *ModifyTasksTool) Name() string { return "modify_tasks" }
func (t *ModifyTasksTool) Description() string {
	return "Batch update, complete, or delete tasks. Operates on one or more task IDs."
}

func (t *ModifyTasksTool) Parameters() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"taskIds": map[string]any{
				"type":        "array",
				"items":       map[string]any{"type": "string"},
				"description": "Task IDs to modify.",
			},
			"action": map[string]any{
				"type":        "string",
				"enum":        []string{"update", "complete", "delete"},
				"description": "Action to perform. 'update' applies the patch fields below. 'complete' marks tasks as done. 'delete' removes tasks.",
			},
			"title": map[string]any{
				"type":        "string",
				"description": "New title (action=update).",
			},
			"description": map[string]any{
				"type":        "string",
				"description": "New description (action=update).",
			},
			"priority": map[string]any{
				"type":        "string",
				"enum":        []string{"low", "medium", "high"},
				"description": "New priority (action=update).",
			},
			"due": map[string]any{
				"type":        "string",
				"description": "New due date as YYYY-MM-DD (action=update).",
			},
			"recurrence": map[string]any{
				"type":        "string",
				"description": "New recurrence RRULE (action=update).",
			},
			"status": map[string]any{
				"type":        "string",
				"enum":        []string{"todo", "doing", "done"},
				"description": "New status (action=update).",
			},
			"tags": map[string]any{
				"type":        "array",
				"items":       map[string]any{"type": "string"},
				"description": "New tags (action=update).",
			},
			"parentId": map[string]any{
				"type":        "string",
				"description": "New parent task ID, empty string to remove parent (action=update).",
			},
		},
		"required": []string{"taskIds", "action"},
	}
}

type modifyResult struct {
	Action    string      `json:"action"`
	Succeeded int         `json:"succeeded"`
	Failed    int         `json:"failed"`
	Errors    []string    `json:"errors,omitempty"`
	Tasks     []todo.Task `json:"tasks,omitempty"`
}

func (t *ModifyTasksTool) Execute(_ context.Context, args map[string]any) *ToolResult {
	ids := toStringSliceFromAny(args["taskIds"])
	if len(ids) == 0 {
		return ErrorResult("'taskIds' is required and must not be empty")
	}

	action, _ := args["action"].(string)

	var result modifyResult
	result.Action = action

	switch action {
	case "complete":
		tasks, errs := t.service.BatchComplete(ids)
		result.Tasks = tasks
		result.Succeeded = len(tasks)
		result.Errors = errs
		result.Failed = len(errs)

	case "delete":
		deleted, errs := t.service.BatchDelete(ids)
		result.Succeeded = len(deleted)
		result.Errors = errs
		result.Failed = len(errs)

	case "update":
		patch := buildPatch(args)
		if len(patch) == 0 {
			return ErrorResult("no fields to update — provide at least one of: title, description, priority, due, recurrence, status, tags, parentId")
		}
		tasks, errs := t.service.BatchUpdate(ids, patch)
		result.Tasks = tasks
		result.Succeeded = len(tasks)
		result.Errors = errs
		result.Failed = len(errs)

	default:
		return ErrorResult(fmt.Sprintf("unknown action: %s (use 'update', 'complete', or 'delete')", action))
	}

	data, _ := json.MarshalIndent(result, "", "  ")
	if result.Failed > 0 {
		r := NewToolResult(string(data))
		r.ForUser = strings.Join(result.Errors, "; ")
		return r
	}
	return SilentResult(string(data))
}

func buildPatch(args map[string]any) map[string]any {
	patch := make(map[string]any)
	for _, key := range []string{"title", "description", "priority", "due", "recurrence", "status", "parentId"} {
		if v, ok := args[key]; ok {
			patch[key] = v
		}
	}
	if v, ok := args["tags"]; ok {
		patch["tags"] = v
	}
	return patch
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
