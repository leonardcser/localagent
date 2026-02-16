package webchat

import (
	"encoding/json"
	"fmt"
	"io"
	"io/fs"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"localagent/pkg/logger"
	"localagent/pkg/tools"
	"localagent/pkg/utils"

	"github.com/labstack/echo/v5"
)

type sendMessageRequest struct {
	Content string   `json:"content"`
	Media   []string `json:"media"`
}

type uploadResponse struct {
	Path string `json:"path"`
}

type historyResponse struct {
	Summary string         `json:"summary,omitempty"`
	Items   []timelineItem `json:"items"`
}

type timelineItem struct {
	Type      string         `json:"type"`
	Role      string         `json:"role,omitempty"`
	Content   string         `json:"content,omitempty"`
	Media     []string       `json:"media,omitempty"`
	EventType string         `json:"event_type,omitempty"`
	Message   string         `json:"message,omitempty"`
	Detail    map[string]any `json:"detail,omitempty"`
	Timestamp string         `json:"timestamp"`
}

func (s *Server) handleSPA(c *echo.Context) error {
	path := c.Request().URL.Path

	if strings.HasPrefix(path, "/api/") {
		return echo.ErrNotFound
	}

	staticSub, _ := fs.Sub(staticFiles, "static")
	f, err := staticSub.Open(strings.TrimPrefix(path, "/"))
	if err == nil {
		f.Close()
		return echo.StaticDirectoryHandler(staticSub, false)(c)
	}

	index, err := fs.ReadFile(staticSub, "index.html")
	if err != nil {
		return echo.ErrNotFound
	}

	return c.HTML(http.StatusOK, string(index))
}

func (s *Server) handleSendMessage(c *echo.Context) error {
	var req sendMessageRequest
	if err := c.Bind(&req); err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "invalid request"})
	}

	if req.Content == "" && len(req.Media) == 0 {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "empty message"})
	}

	s.channel.HandleIncoming(req.Content, req.Media, nil)
	return c.JSON(http.StatusOK, map[string]bool{"ok": true})
}

func (s *Server) handleUpload(c *echo.Context) error {
	file, err := c.FormFile("file")
	if err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "no file provided"})
	}

	mediaDir := filepath.Join(s.channel.workspace, "media")
	if err := os.MkdirAll(mediaDir, 0700); err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "failed to create media directory"})
	}

	safeName := utils.SanitizeFilename(file.Filename)
	localPath := filepath.Join(mediaDir, safeName)

	if _, err := os.Stat(localPath); err == nil {
		ext := filepath.Ext(safeName)
		base := strings.TrimSuffix(safeName, ext)
		for i := 1; ; i++ {
			candidate := filepath.Join(mediaDir, fmt.Sprintf("%s_%d%s", base, i, ext))
			if _, err := os.Stat(candidate); os.IsNotExist(err) {
				localPath = candidate
				break
			}
		}
	}

	src, err := file.Open()
	if err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "failed to open uploaded file"})
	}
	defer src.Close()

	dst, err := os.Create(localPath)
	if err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "failed to save file"})
	}
	defer dst.Close()

	if _, err := io.Copy(dst, src); err != nil {
		os.Remove(localPath)
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "failed to write file"})
	}

	logger.Info("webchat file uploaded: %s", localPath)
	return c.JSON(http.StatusOK, uploadResponse{Path: localPath})
}

func (s *Server) handleMedia(c *echo.Context) error {
	name := c.Param("filename")
	if name == "" || strings.Contains(name, "/") || strings.Contains(name, "..") {
		return echo.ErrNotFound
	}
	filePath := filepath.Join(s.channel.workspace, "media", name)
	return c.File(filePath)
}

func (s *Server) handleTranscribe(c *echo.Context) error {
	file, err := c.FormFile("file")
	if err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "no file provided"})
	}

	whisper := s.channel.whisper
	if whisper.URL == "" {
		return c.JSON(http.StatusServiceUnavailable, map[string]string{"error": "whisper not configured"})
	}

	tmpDir := filepath.Join(s.channel.workspace, "media")
	if err := os.MkdirAll(tmpDir, 0700); err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "failed to create temp directory"})
	}

	tmpFile, err := os.CreateTemp(tmpDir, "transcribe-*"+filepath.Ext(file.Filename))
	if err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "failed to create temp file"})
	}
	tmpPath := tmpFile.Name()
	defer os.Remove(tmpPath)

	src, err := file.Open()
	if err != nil {
		tmpFile.Close()
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "failed to open uploaded file"})
	}

	if _, err := io.Copy(tmpFile, src); err != nil {
		src.Close()
		tmpFile.Close()
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "failed to write temp file"})
	}
	src.Close()
	tmpFile.Close()

	text, err := tools.TranscribeAudio(c.Request().Context(), tmpPath, whisper.URL, whisper.ResolveAPIKey())
	if err != nil {
		logger.Error("transcription failed: %v", err)
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "transcription failed"})
	}

	return c.JSON(http.StatusOK, map[string]string{"text": text})
}

func (s *Server) handleHistory(c *echo.Context) error {
	if s.channel.sessions == nil {
		return c.JSON(http.StatusOK, historyResponse{Items: []timelineItem{}})
	}

	timeline := s.channel.sessions.GetTimeline("web:default")
	summary := s.channel.sessions.GetSummary("web:default")

	items := make([]timelineItem, 0, len(timeline))
	for _, entry := range timeline {
		if entry.Kind == "message" {
			msg := entry.Message
			if msg.Role != "user" && msg.Role != "assistant" {
				continue
			}
			items = append(items, timelineItem{
				Type:      "message",
				Role:      msg.Role,
				Content:   msg.Content,
				Media:     entry.Media,
				Timestamp: entry.Timestamp.Format("15:04:05"),
			})
		} else if entry.Activity != nil {
			evt := entry.Activity
			items = append(items, timelineItem{
				Type:      "activity",
				EventType: string(evt.Type),
				Message:   evt.Message,
				Detail:    evt.Detail,
				Timestamp: entry.Timestamp.Format("15:04:05"),
			})
		}
	}

	return c.JSON(http.StatusOK, historyResponse{
		Summary: summary,
		Items:   items,
	})
}

func (s *Server) handleSSE(c *echo.Context) error {
	clientID := utils.RandHex(16)
	client := s.channel.registerClient(clientID)

	w := c.Response()
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.WriteHeader(http.StatusOK)

	rc := http.NewResponseController(w)

	// Send initial processing status
	processing := s.channel.processing.Load()
	statusEvent := OutgoingEvent{Type: "status", Processing: &processing}
	if data, err := json.Marshal(statusEvent); err == nil {
		fmt.Fprintf(w, "data: %s\n\n", data)
	}
	rc.Flush()

	ctx := c.Request().Context()
	for {
		select {
		case <-ctx.Done():
			s.channel.unregisterClient(clientID)
			return nil
		case event, ok := <-client.events:
			if !ok {
				return nil
			}
			data, err := json.Marshal(event)
			if err != nil {
				logger.Error("webchat SSE marshal error: %v", err)
				continue
			}
			fmt.Fprintf(w, "data: %s\n\n", data)
			rc.Flush()
		}
	}
}

