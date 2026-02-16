package voice

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

	"localagent/pkg/logger"
	"localagent/pkg/utils"
)

type WhisperCppTranscriber struct {
	apiBase    string
	httpClient *http.Client
}

type TranscriptionResponse struct {
	Text     string  `json:"text"`
	Language string  `json:"language,omitempty"`
	Duration float64 `json:"duration,omitempty"`
}

func NewWhisperCppTranscriber(apiBase string) *WhisperCppTranscriber {
	if apiBase == "" {
		apiBase = "http://127.0.0.1:8080"
	}
	logger.Debug("creating whisper.cpp transcriber: base=%s", apiBase)

	return &WhisperCppTranscriber{
		apiBase: apiBase,
		httpClient: &http.Client{
			Timeout: 120 * time.Second,
		},
	}
}

func (t *WhisperCppTranscriber) Transcribe(ctx context.Context, audioFilePath string) (*TranscriptionResponse, error) {
	logger.Info("starting transcription: file=%s", audioFilePath)

	audioFile, err := os.Open(audioFilePath)
	if err != nil {
		return nil, fmt.Errorf("failed to open audio file: %w", err)
	}
	defer audioFile.Close()

	fileInfo, err := audioFile.Stat()
	if err != nil {
		return nil, fmt.Errorf("failed to get file info: %w", err)
	}

	logger.Debug("audio file: name=%s size=%d", filepath.Base(audioFilePath), fileInfo.Size())

	var requestBody bytes.Buffer
	writer := multipart.NewWriter(&requestBody)

	part, err := writer.CreateFormFile("file", filepath.Base(audioFilePath))
	if err != nil {
		return nil, fmt.Errorf("failed to create form file: %w", err)
	}

	if _, err := io.Copy(part, audioFile); err != nil {
		return nil, fmt.Errorf("failed to copy file content: %w", err)
	}

	if err := writer.WriteField("response_format", "json"); err != nil {
		return nil, fmt.Errorf("failed to write response_format field: %w", err)
	}

	if err := writer.WriteField("temperature", "0.0"); err != nil {
		return nil, fmt.Errorf("failed to write temperature field: %w", err)
	}

	if err := writer.Close(); err != nil {
		return nil, fmt.Errorf("failed to close multipart writer: %w", err)
	}

	url := t.apiBase + "/inference"
	req, err := http.NewRequestWithContext(ctx, "POST", url, &requestBody)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Content-Type", writer.FormDataContentType())

	logger.Debug("sending transcription request: url=%s size=%d", url, requestBody.Len())

	resp, err := t.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to send request: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("whisper server error (status %d): %s", resp.StatusCode, string(body))
	}

	var result TranscriptionResponse
	if err := json.Unmarshal(body, &result); err != nil {
		return nil, fmt.Errorf("failed to unmarshal response: %w", err)
	}

	logger.Info("transcription completed: length=%d preview=%s", len(result.Text), utils.Truncate(result.Text, 50))

	return &result, nil
}

func (t *WhisperCppTranscriber) IsAvailable() bool {
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, "GET", t.apiBase+"/", nil)
	if err != nil {
		return false
	}

	resp, err := t.httpClient.Do(req)
	if err != nil {
		return false
	}
	resp.Body.Close()

	return resp.StatusCode == http.StatusOK
}
