package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

type NewsTool struct {
	maxItems int
}

func NewNewsTool(maxItems int) *NewsTool {
	if maxItems <= 0 {
		maxItems = 15
	}
	return &NewsTool{maxItems: maxItems}
}

func (t *NewsTool) Name() string {
	return "tech_news"
}

func (t *NewsTool) Description() string {
	return "Fetch latest tech news from Hacker News and Lobsters. Returns titles, URLs, scores, and comments. Use this to stay up to date with what's happening in the tech world."
}

func (t *NewsTool) Parameters() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"source": map[string]any{
				"type":        "string",
				"description": "News source to fetch from",
				"enum":        []string{"hackernews", "lobsters", "all"},
				"default":     "all",
			},
			"count": map[string]any{
				"type":        "integer",
				"description": "Number of stories per source (1-30)",
				"minimum":     1.0,
				"maximum":     30.0,
			},
		},
	}
}

func (t *NewsTool) DeclaredDomains() []string {
	return []string{"hn.algolia.com", "lobste.rs"}
}

func (t *NewsTool) Execute(ctx context.Context, args map[string]any) *ToolResult {
	source := "all"
	if s, ok := args["source"].(string); ok && s != "" {
		source = s
	}

	count := t.maxItems
	if c, ok := args["count"].(float64); ok && int(c) > 0 && int(c) <= 30 {
		count = int(c)
	}

	var sections []string

	switch source {
	case "hackernews":
		hn, err := t.fetchHackerNews(ctx, count)
		if err != nil {
			return ErrorResult(fmt.Sprintf("failed to fetch Hacker News: %v", err))
		}
		sections = append(sections, hn)
	case "lobsters":
		lb, err := t.fetchLobsters(ctx, count)
		if err != nil {
			return ErrorResult(fmt.Sprintf("failed to fetch Lobsters: %v", err))
		}
		sections = append(sections, lb)
	case "all":
		hn, hnErr := t.fetchHackerNews(ctx, count)
		lb, lbErr := t.fetchLobsters(ctx, count)
		if hnErr != nil && lbErr != nil {
			return ErrorResult(fmt.Sprintf("failed to fetch news: HN: %v, Lobsters: %v", hnErr, lbErr))
		}
		if hn != "" {
			sections = append(sections, hn)
		}
		if lb != "" {
			sections = append(sections, lb)
		}
	default:
		return ErrorResult(fmt.Sprintf("unknown source: %s (use hackernews, lobsters, or all)", source))
	}

	result := strings.Join(sections, "\n\n")
	return SilentResult(result)
}

func (t *NewsTool) fetchHackerNews(ctx context.Context, count int) (string, error) {
	req, err := http.NewRequestWithContext(ctx, "GET", "https://hn.algolia.com/api/v1/search?tags=front_page&hitsPerPage="+fmt.Sprint(count), nil)
	if err != nil {
		return "", fmt.Errorf("failed to create request: %w", err)
	}

	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return "", fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("failed to read response: %w", err)
	}

	var data struct {
		Hits []struct {
			Title    string `json:"title"`
			URL      string `json:"url"`
			Points   int    `json:"points"`
			Comments int    `json:"num_comments"`
			ObjectID string `json:"objectID"`
		} `json:"hits"`
	}

	if err := json.Unmarshal(body, &data); err != nil {
		return "", fmt.Errorf("failed to parse response: %w", err)
	}

	var lines []string
	lines = append(lines, "## Hacker News (Front Page)")
	for i, hit := range data.Hits {
		if i >= count {
			break
		}
		link := hit.URL
		if link == "" {
			link = fmt.Sprintf("https://news.ycombinator.com/item?id=%s", hit.ObjectID)
		}
		lines = append(lines, fmt.Sprintf("%d. %s\n   %s\n   %d points | %d comments | https://news.ycombinator.com/item?id=%s",
			i+1, hit.Title, link, hit.Points, hit.Comments, hit.ObjectID))
	}

	return strings.Join(lines, "\n"), nil
}

func (t *NewsTool) fetchLobsters(ctx context.Context, count int) (string, error) {
	req, err := http.NewRequestWithContext(ctx, "GET", "https://lobste.rs/hottest.json", nil)
	if err != nil {
		return "", fmt.Errorf("failed to create request: %w", err)
	}

	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return "", fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("failed to read response: %w", err)
	}

	var stories []struct {
		Title        string   `json:"title"`
		URL          string   `json:"url"`
		Score        int      `json:"score"`
		CommentCount int      `json:"comment_count"`
		Tags         []string `json:"tags"`
		ShortIDURL   string   `json:"short_id_url"`
	}

	if err := json.Unmarshal(body, &stories); err != nil {
		return "", fmt.Errorf("failed to parse response: %w", err)
	}

	var lines []string
	lines = append(lines, "## Lobsters (Hottest)")
	for i, story := range stories {
		if i >= count {
			break
		}
		link := story.URL
		if link == "" {
			link = story.ShortIDURL
		}
		tags := ""
		if len(story.Tags) > 0 {
			tags = " [" + strings.Join(story.Tags, ", ") + "]"
		}
		lines = append(lines, fmt.Sprintf("%d. %s%s\n   %s\n   %d points | %d comments | %s",
			i+1, story.Title, tags, link, story.Score, story.CommentCount, story.ShortIDURL))
	}

	return strings.Join(lines, "\n"), nil
}
