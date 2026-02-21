package tools

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"os"
	"path/filepath"
	"time"
)

type PDFToTextTool struct {
	workspace  string
	serviceURL string
	apiKey     string
}

func NewPDFToTextTool(workspace, serviceURL, apiKey string) *PDFToTextTool {
	return &PDFToTextTool{
		workspace:  workspace,
		serviceURL: serviceURL,
		apiKey:     apiKey,
	}
}

func (t *PDFToTextTool) Name() string {
	return "pdf_to_text"
}

func (t *PDFToTextTool) Description() string {
	return "Convert a PDF file to text. Accepts a file path relative to the workspace and returns extracted text content."
}

func (t *PDFToTextTool) Parameters() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"path": map[string]any{
				"type":        "string",
				"description": "Path to the PDF file (relative to workspace or absolute)",
			},
		},
		"required": []string{"path"},
	}
}

func (t *PDFToTextTool) Execute(ctx context.Context, args map[string]any) *ToolResult {
	path, ok := args["path"].(string)
	if !ok || path == "" {
		return ErrorResult("path is required")
	}

	if !filepath.IsAbs(path) {
		path = filepath.Join(t.workspace, path)
	}

	text, err := ConvertPDF(ctx, path, t.serviceURL, t.apiKey)
	if err != nil {
		return ErrorResult(fmt.Sprintf("PDF conversion failed: %v", err))
	}

	return SilentResult(text)
}

// ConvertPDF uploads a PDF file to the conversion service and returns the extracted text.
// This is shared between the tool and the media pipeline.
func ConvertPDF(ctx context.Context, filePath, serviceURL, apiKey string) (string, error) {
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

	return string(body), nil
}
