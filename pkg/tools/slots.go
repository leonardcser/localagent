package tools

import (
	"context"
	"encoding/json"
	"fmt"

	"localagent/pkg/todo"
)

// --- list_slots ---

type ListSlotsTool struct{ baseTodoTool }

func NewListSlotsTool(service *todo.TodoService) *ListSlotsTool {
	return &ListSlotsTool{baseTodoTool{service}}
}

func (t *ListSlotsTool) Name() string        { return "list_slots" }
func (t *ListSlotsTool) Description() string { return "List time-blocked slots. Optionally filter by task ID or time range." }

func (t *ListSlotsTool) Parameters() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"taskId": map[string]any{
				"type":        "string",
				"description": "Filter by task ID.",
			},
			"startAfter": map[string]any{
				"type":        "number",
				"description": "Only slots ending after this unix ms timestamp.",
			},
			"endBefore": map[string]any{
				"type":        "number",
				"description": "Only slots starting before this unix ms timestamp.",
			},
		},
	}
}

func (t *ListSlotsTool) Execute(_ context.Context, args map[string]any) *ToolResult {
	taskID, _ := args["taskId"].(string)
	startAfter, _ := args["startAfter"].(float64)
	endBefore, _ := args["endBefore"].(float64)

	slots := t.service.ListSlots(taskID, int64(startAfter), int64(endBefore))
	if len(slots) == 0 {
		return SilentResult("No slots found")
	}

	data, _ := json.MarshalIndent(slots, "", "  ")
	return SilentResult(string(data))
}

// --- add_slot ---

type AddSlotTool struct{ baseTodoTool }

func NewAddSlotTool(service *todo.TodoService) *AddSlotTool {
	return &AddSlotTool{baseTodoTool{service}}
}

func (t *AddSlotTool) Name() string        { return "add_slot" }
func (t *AddSlotTool) Description() string { return "Create a time-blocked slot for a task." }

func (t *AddSlotTool) Parameters() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"taskId": map[string]any{
				"type":        "string",
				"description": "Task ID to attach the slot to.",
			},
			"startAtMs": map[string]any{
				"type":        "number",
				"description": "Slot start time as unix milliseconds.",
			},
			"endAtMs": map[string]any{
				"type":        "number",
				"description": "Slot end time as unix milliseconds.",
			},
			"note": map[string]any{
				"type":        "string",
				"description": "Optional note for this time block.",
			},
		},
		"required": []string{"taskId", "startAtMs", "endAtMs"},
	}
}

func (t *AddSlotTool) Execute(_ context.Context, args map[string]any) *ToolResult {
	taskID, _ := args["taskId"].(string)
	if taskID == "" {
		return ErrorResult("'taskId' is required")
	}
	startAt, _ := args["startAtMs"].(float64)
	endAt, _ := args["endAtMs"].(float64)
	if startAt == 0 || endAt == 0 {
		return ErrorResult("'startAtMs' and 'endAtMs' are required")
	}

	slot := todo.Slot{
		TaskID:    taskID,
		StartAtMS: int64(startAt),
		EndAtMS:   int64(endAt),
	}
	if v, ok := args["note"].(string); ok {
		slot.Note = v
	}

	created, err := t.service.AddSlot(slot)
	if err != nil {
		return ErrorResult(fmt.Sprintf("error adding slot: %v", err))
	}

	data, _ := json.MarshalIndent(created, "", "  ")
	return SilentResult(string(data))
}

// --- remove_slot ---

type RemoveSlotTool struct{ baseTodoTool }

func NewRemoveSlotTool(service *todo.TodoService) *RemoveSlotTool {
	return &RemoveSlotTool{baseTodoTool{service}}
}

func (t *RemoveSlotTool) Name() string        { return "remove_slot" }
func (t *RemoveSlotTool) Description() string { return "Delete a time-blocked slot." }

func (t *RemoveSlotTool) Parameters() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"slotId": map[string]any{
				"type":        "string",
				"description": "Slot ID to remove.",
			},
		},
		"required": []string{"slotId"},
	}
}

func (t *RemoveSlotTool) Execute(_ context.Context, args map[string]any) *ToolResult {
	slotID, ok := args["slotId"].(string)
	if !ok || slotID == "" {
		return ErrorResult("'slotId' is required")
	}

	if t.service.RemoveSlot(slotID) {
		return SilentResult(fmt.Sprintf("Slot removed: %s", slotID))
	}
	return ErrorResult(fmt.Sprintf("slot %s not found", slotID))
}
