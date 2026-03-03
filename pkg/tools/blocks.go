package tools

import (
	"context"
	"encoding/json"
	"fmt"

	"localagent/pkg/todo"
)

// --- list_blocks ---

type ListBlocksTool struct{ baseTodoTool }

func NewListBlocksTool(service *todo.TodoService) *ListBlocksTool {
	return &ListBlocksTool{baseTodoTool{service}}
}

func (t *ListBlocksTool) Name() string        { return "list_blocks" }
func (t *ListBlocksTool) Description() string { return "List time blocks. Optionally filter by task ID or time range." }

func (t *ListBlocksTool) Parameters() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"taskId": map[string]any{
				"type":        "string",
				"description": "Filter by task ID.",
			},
			"startAfter": map[string]any{
				"type":        "number",
				"description": "Only blocks ending after this unix ms timestamp.",
			},
			"endBefore": map[string]any{
				"type":        "number",
				"description": "Only blocks starting before this unix ms timestamp.",
			},
		},
	}
}

func (t *ListBlocksTool) Execute(_ context.Context, args map[string]any) *ToolResult {
	taskID, _ := args["taskId"].(string)
	startAfter, _ := args["startAfter"].(float64)
	endBefore, _ := args["endBefore"].(float64)

	blocks := t.service.ListBlocks(taskID, int64(startAfter), int64(endBefore))
	if len(blocks) == 0 {
		return SilentResult("No blocks found")
	}

	data, _ := json.MarshalIndent(blocks, "", "  ")
	return SilentResult(string(data))
}

// --- add_block ---

type AddBlockTool struct{ baseTodoTool }

func NewAddBlockTool(service *todo.TodoService) *AddBlockTool {
	return &AddBlockTool{baseTodoTool{service}}
}

func (t *AddBlockTool) Name() string        { return "add_block" }
func (t *AddBlockTool) Description() string { return "Create a time block for a task." }

func (t *AddBlockTool) Parameters() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"taskId": map[string]any{
				"type":        "string",
				"description": "Task ID to attach the block to.",
			},
			"startAtMs": map[string]any{
				"type":        "number",
				"description": "Block start time as unix milliseconds.",
			},
			"endAtMs": map[string]any{
				"type":        "number",
				"description": "Block end time as unix milliseconds.",
			},
			"note": map[string]any{
				"type":        "string",
				"description": "Optional note for this time block.",
			},
		},
		"required": []string{"taskId", "startAtMs", "endAtMs"},
	}
}

func (t *AddBlockTool) Execute(_ context.Context, args map[string]any) *ToolResult {
	taskID, _ := args["taskId"].(string)
	if taskID == "" {
		return ErrorResult("'taskId' is required")
	}
	startAt, _ := args["startAtMs"].(float64)
	endAt, _ := args["endAtMs"].(float64)
	if startAt == 0 || endAt == 0 {
		return ErrorResult("'startAtMs' and 'endAtMs' are required")
	}

	block := todo.Block{
		TaskID:    taskID,
		StartAtMS: int64(startAt),
		EndAtMS:   int64(endAt),
	}
	if v, ok := args["note"].(string); ok {
		block.Note = v
	}

	created, err := t.service.AddBlock(block)
	if err != nil {
		return ErrorResult(fmt.Sprintf("error adding block: %v", err))
	}

	data, _ := json.MarshalIndent(created, "", "  ")
	return SilentResult(string(data))
}

// --- remove_block ---

type RemoveBlockTool struct{ baseTodoTool }

func NewRemoveBlockTool(service *todo.TodoService) *RemoveBlockTool {
	return &RemoveBlockTool{baseTodoTool{service}}
}

func (t *RemoveBlockTool) Name() string        { return "remove_block" }
func (t *RemoveBlockTool) Description() string { return "Delete a time block." }

func (t *RemoveBlockTool) Parameters() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"blockId": map[string]any{
				"type":        "string",
				"description": "Block ID to remove.",
			},
		},
		"required": []string{"blockId"},
	}
}

func (t *RemoveBlockTool) Execute(_ context.Context, args map[string]any) *ToolResult {
	blockID, ok := args["blockId"].(string)
	if !ok || blockID == "" {
		return ErrorResult("'blockId' is required")
	}

	if t.service.RemoveBlock(blockID) {
		return SilentResult(fmt.Sprintf("Block removed: %s", blockID))
	}
	return ErrorResult(fmt.Sprintf("block %s not found", blockID))
}
