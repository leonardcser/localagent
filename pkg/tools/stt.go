package tools

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"os"
	"path/filepath"
	"time"
)

type TranscribeAudioTool struct {
	workspace  string
	serviceURL string
	apiKey     string
}

func NewTranscribeAudioTool(workspace, serviceURL, apiKey string) *TranscribeAudioTool {
	return &TranscribeAudioTool{
		workspace:  workspace,
		serviceURL: serviceURL,
		apiKey:     apiKey,
	}
}

func (t *TranscribeAudioTool) Name() string {
	return "transcribe_audio"
}

func (t *TranscribeAudioTool) Description() string {
	return "Transcribe an audio file to text using Whisper. Accepts a file path relative to the workspace and returns the transcribed text."
}

func (t *TranscribeAudioTool) Parameters() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"path": map[string]any{
				"type":        "string",
				"description": "Path to the audio file (relative to workspace or absolute)",
			},
		},
		"required": []string{"path"},
	}
}

func (t *TranscribeAudioTool) Execute(ctx context.Context, args map[string]any) *ToolResult {
	path, ok := args["path"].(string)
	if !ok || path == "" {
		return ErrorResult("path is required")
	}

	if !filepath.IsAbs(path) {
		path = filepath.Join(t.workspace, path)
	}

	text, err := TranscribeAudio(ctx, path, t.serviceURL, t.apiKey)
	if err != nil {
		return ErrorResult(fmt.Sprintf("transcription failed: %v", err))
	}

	return SilentResult(text)
}

// TranscribeAudio uploads an audio file to a Whisper service and returns the transcribed text.
// This is shared between the tool and the media pipeline.
func TranscribeAudio(ctx context.Context, filePath, serviceURL, apiKey string) (string, error) {
	f, err := os.Open(filePath)
	if err != nil {
		return "", fmt.Errorf("open file: %w", err)
	}
	defer f.Close()

	var buf bytes.Buffer
	w := multipart.NewWriter(&buf)
	part, err := w.CreateFormFile("file", filepath.Base(filePath))
	if err != nil {
		return "", fmt.Errorf("create form file: %w", err)
	}
	if _, err := io.Copy(part, f); err != nil {
		return "", fmt.Errorf("copy file: %w", err)
	}
	if err := w.WriteField("response_format", "json"); err != nil {
		return "", fmt.Errorf("write field: %w", err)
	}
	w.Close()

	req, err := http.NewRequestWithContext(ctx, "POST", serviceURL, &buf)
	if err != nil {
		return "", fmt.Errorf("create request: %w", err)
	}
	req.Header.Set("Content-Type", w.FormDataContentType())
	if apiKey != "" {
		req.Header.Set("Authorization", "Bearer "+apiKey)
	}

	client := &http.Client{Timeout: 120 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return "", fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("read response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("service returned %d: %s", resp.StatusCode, string(body))
	}

	var result struct {
		Text string `json:"text"`
	}
	if err := json.Unmarshal(body, &result); err != nil {
		return "", fmt.Errorf("parse response: %w", err)
	}

	return result.Text, nil
}
