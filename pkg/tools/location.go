package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"time"
)

type LocationTool struct {
	haURL  string
	apiKey string
	user   string
}

func NewLocationTool(haURL, apiKey, user string) *LocationTool {
	return &LocationTool{haURL: haURL, apiKey: apiKey, user: user}
}

func (t *LocationTool) Name() string {
	return "get_user_location"
}

func (t *LocationTool) Description() string {
	return "Get the current location of the user. Returns the location zone name (e.g. 'home', 'work') or GPS coordinates if in an unknown zone."
}

func (t *LocationTool) Parameters() map[string]any {
	return map[string]any{
		"type":       "object",
		"properties": map[string]any{},
	}
}

func (t *LocationTool) DeclaredDomains() []string {
	u, err := url.Parse(t.haURL)
	if err != nil || u.Host == "" {
		return nil
	}
	return []string{u.Host}
}

func (t *LocationTool) Execute(ctx context.Context, args map[string]any) *ToolResult {
	apiURL := fmt.Sprintf("%s/api/states/person.%s", t.haURL, t.user)

	req, err := http.NewRequestWithContext(ctx, "GET", apiURL, nil)
	if err != nil {
		return ErrorResult(fmt.Sprintf("failed to create request: %v", err))
	}
	req.Header.Set("Authorization", "Bearer "+t.apiKey)

	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return ErrorResult(fmt.Sprintf("failed to fetch location: %v", err))
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return ErrorResult(fmt.Sprintf("Home Assistant returned status %d", resp.StatusCode))
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return ErrorResult(fmt.Sprintf("failed to read response: %v", err))
	}

	var data struct {
		State string `json:"state"`
	}
	if err := json.Unmarshal(body, &data); err != nil {
		return ErrorResult(fmt.Sprintf("failed to parse response: %v", err))
	}

	return SilentResult(fmt.Sprintf("User is currently at: %s", data.State))
}
