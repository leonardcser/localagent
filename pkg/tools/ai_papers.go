package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"golang.org/x/net/html"
)

type AIPapersTool struct {
	maxItems int
}

func NewAIPapersTool(maxItems int) *AIPapersTool {
	if maxItems <= 0 {
		maxItems = 15
	}
	return &AIPapersTool{maxItems: maxItems}
}

func (t *AIPapersTool) Name() string {
	return "ai_papers"
}

func (t *AIPapersTool) Description() string {
	return "Fetch trending AI and machine learning research papers from Hugging Face. Returns titles, links, and upvotes. Use this to stay up to date with the latest AI/ML research."
}

func (t *AIPapersTool) Parameters() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"period": map[string]any{
				"type":        "string",
				"description": "Time period for trending papers: daily (today's papers), weekly (current week, e.g. 2026-W08), or monthly (current month, e.g. 2026-02)",
				"enum":        []string{"daily", "weekly", "monthly"},
				"default":     "daily",
			},
			"count": map[string]any{
				"type":        "integer",
				"description": "Number of papers to fetch (1-50)",
				"minimum":     1.0,
				"maximum":     50.0,
			},
		},
	}
}

func (t *AIPapersTool) DeclaredDomains() []string {
	return []string{"huggingface.co"}
}

func (t *AIPapersTool) Execute(ctx context.Context, args map[string]any) *ToolResult {
	period := "daily"
	if p, ok := args["period"].(string); ok && p != "" {
		period = p
	}

	count := t.maxItems
	if c, ok := args["count"].(float64); ok && int(c) > 0 && int(c) <= 50 {
		count = int(c)
	}

	var result string
	var err error

	now := time.Now()
	var path, label string

	switch period {
	case "daily":
		path = fmt.Sprintf("/papers/date/%d-%02d-%02d", now.Year(), now.Month(), now.Day())
		label = "Daily"
	case "weekly":
		year, week := now.ISOWeek()
		path = fmt.Sprintf("/papers/week/%d-W%02d", year, week)
		label = "Weekly"
	case "monthly":
		path = fmt.Sprintf("/papers/month/%d-%02d", now.Year(), now.Month())
		label = "Monthly"
	default:
		return ErrorResult(fmt.Sprintf("unknown period: %s (use daily, weekly, or monthly)", period))
	}

	result, err = t.fetchFromHTML(ctx, path, label, count)

	if err != nil {
		return ErrorResult(fmt.Sprintf("failed to fetch %s papers: %v", period, err))
	}

	return SilentResult(result)
}

func (t *AIPapersTool) fetchFromHTML(ctx context.Context, path string, label string, count int) (string, error) {
	url := "https://huggingface.co" + path
	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return "", fmt.Errorf("failed to create request: %w", err)
	}

	client := &http.Client{Timeout: 15 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return "", fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("page returned status %d", resp.StatusCode)
	}

	papers, err := extractPapersFromHTML(resp.Body)
	if err != nil {
		return "", err
	}

	if len(papers) == 0 {
		return "", fmt.Errorf("no papers found on page")
	}

	var lines []string
	lines = append(lines, fmt.Sprintf("## Hugging Face %s Papers", label))
	for i, p := range papers {
		if i >= count {
			break
		}
		paperURL := fmt.Sprintf("https://huggingface.co/papers/%s", p.Paper.ID)
		lines = append(lines, fmt.Sprintf("%d. %s\n   %s\n   %d upvotes | %d comments",
			i+1, p.Paper.Title, paperURL, p.Paper.Upvotes, p.NumComments))
	}

	return strings.Join(lines, "\n"), nil
}

type hfPaperEntry struct {
	Paper struct {
		ID      string `json:"id"`
		Title   string `json:"title"`
		Upvotes int    `json:"upvotes"`
	} `json:"paper"`
	NumComments int `json:"numComments"`
}

func extractPapersFromHTML(r io.Reader) ([]hfPaperEntry, error) {
	doc, err := html.Parse(r)
	if err != nil {
		return nil, fmt.Errorf("failed to parse HTML: %w", err)
	}

	raw := findDataProps(doc, "dailyPapers")
	if raw == "" {
		return nil, fmt.Errorf("could not find paper data in page")
	}

	var props struct {
		DailyPapers []hfPaperEntry `json:"dailyPapers"`
	}
	if err := json.Unmarshal([]byte(raw), &props); err != nil {
		return nil, fmt.Errorf("failed to parse embedded paper data: %w", err)
	}

	return props.DailyPapers, nil
}

// findDataProps walks the HTML tree looking for an element with a data-props
// attribute whose value contains the given key. Returns the attribute value
// or empty string if not found.
func findDataProps(n *html.Node, key string) string {
	if n.Type == html.ElementNode {
		for _, attr := range n.Attr {
			if attr.Key == "data-props" && strings.Contains(attr.Val, "\""+key+"\"") {
				return attr.Val
			}
		}
	}
	for c := n.FirstChild; c != nil; c = c.NextSibling {
		if result := findDataProps(c, key); result != "" {
			return result
		}
	}
	return ""
}
