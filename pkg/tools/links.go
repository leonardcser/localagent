package tools

import (
	"context"
	"encoding/json"
	"fmt"

	"localagent/pkg/todo"
)

// --- list_links ---

type ListLinksTool struct{ baseTodoTool }

func NewListLinksTool(service *todo.TodoService) *ListLinksTool {
	return &ListLinksTool{baseTodoTool{service}}
}

func (t *ListLinksTool) Name() string        { return "list_links" }
func (t *ListLinksTool) Description() string { return "List saved links. Optionally filter by tag." }

func (t *ListLinksTool) Parameters() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"tag": map[string]any{
				"type":        "string",
				"description": "Filter links by tag.",
			},
		},
	}
}

func (t *ListLinksTool) Execute(_ context.Context, args map[string]any) *ToolResult {
	tag, _ := args["tag"].(string)
	links := t.service.ListLinks(tag)
	if len(links) == 0 {
		return SilentResult("No links found")
	}
	data, _ := json.MarshalIndent(links, "", "  ")
	return SilentResult(string(data))
}

// --- add_link ---

type AddLinkTool struct{ baseTodoTool }

func NewAddLinkTool(service *todo.TodoService) *AddLinkTool {
	return &AddLinkTool{baseTodoTool{service}}
}

func (t *AddLinkTool) Name() string        { return "add_link" }
func (t *AddLinkTool) Description() string { return "Save a link to the link library." }

func (t *AddLinkTool) Parameters() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"url": map[string]any{
				"type":        "string",
				"description": "The URL to save.",
			},
			"title": map[string]any{
				"type":        "string",
				"description": "Optional title for the link.",
			},
			"description": map[string]any{
				"type":        "string",
				"description": "Optional description.",
			},
			"tags": map[string]any{
				"type":        "array",
				"items":       map[string]any{"type": "string"},
				"description": "Optional list of tags.",
			},
		},
		"required": []string{"url"},
	}
}

func (t *AddLinkTool) Execute(_ context.Context, args map[string]any) *ToolResult {
	url, _ := args["url"].(string)
	if url == "" {
		return ErrorResult("'url' is required")
	}

	link := todo.Link{URL: url}
	if v, ok := args["title"].(string); ok {
		link.Title = v
	}
	if v, ok := args["description"].(string); ok {
		link.Description = v
	}
	if v, ok := args["tags"]; ok {
		link.Tags = toStringSliceFromAny(v)
	}

	created, err := t.service.AddLink(link)
	if err != nil {
		return ErrorResult(fmt.Sprintf("error adding link: %v", err))
	}

	data, _ := json.MarshalIndent(created, "", "  ")
	return SilentResult(string(data))
}

// --- remove_link ---

type RemoveLinkTool struct{ baseTodoTool }

func NewRemoveLinkTool(service *todo.TodoService) *RemoveLinkTool {
	return &RemoveLinkTool{baseTodoTool{service}}
}

func (t *RemoveLinkTool) Name() string        { return "remove_link" }
func (t *RemoveLinkTool) Description() string { return "Delete a link from the library." }

func (t *RemoveLinkTool) Parameters() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"id": map[string]any{
				"type":        "string",
				"description": "Link ID to remove.",
			},
		},
		"required": []string{"id"},
	}
}

func (t *RemoveLinkTool) Execute(_ context.Context, args map[string]any) *ToolResult {
	id, ok := args["id"].(string)
	if !ok || id == "" {
		return ErrorResult("'id' is required")
	}
	if t.service.RemoveLink(id) {
		return SilentResult(fmt.Sprintf("Link removed: %s", id))
	}
	return ErrorResult(fmt.Sprintf("link %s not found", id))
}
