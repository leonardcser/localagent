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
	"time"

	"localagent/pkg/logger"
	"localagent/pkg/todo"
	"localagent/pkg/tools"
	"localagent/pkg/utils"

	webpush "github.com/SherClockHolmes/webpush-go"
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

	mediaDir := s.mediaDir
	if err := os.MkdirAll(mediaDir, 0700); err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "failed to create media directory"})
	}

	utils.CleanOldMedia(mediaDir, 10*time.Minute)

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
	filePath := filepath.Join(s.mediaDir, name)
	return c.File(filePath)
}

func (s *Server) handleTranscribe(c *echo.Context) error {
	file, err := c.FormFile("file")
	if err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "no file provided"})
	}

	stt := s.channel.stt
	if stt.URL == "" {
		return c.JSON(http.StatusServiceUnavailable, map[string]string{"error": "stt not configured"})
	}

	tmpDir := s.mediaDir
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

	text, err := tools.TranscribeAudio(c.Request().Context(), tmpPath, stt.URL, stt.ResolveAPIKey())
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
				Timestamp: entry.Timestamp.Format(time.RFC3339),
			})
		} else if entry.Activity != nil {
			evt := entry.Activity
			items = append(items, timelineItem{
				Type:      "activity",
				EventType: string(evt.Type),
				Message:   evt.Message,
				Detail:    evt.Detail,
				Timestamp: entry.Timestamp.Format(time.RFC3339),
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
	statusEvent := OutgoingEvent{Type: "status", Processing: &processing, ClientID: clientID}
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

func (s *Server) handleActive(c *echo.Context) error {
	var req struct {
		ClientID string `json:"client_id"`
		Active   bool   `json:"active"`
	}
	if err := c.Bind(&req); err != nil || req.ClientID == "" {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "invalid request"})
	}
	s.channel.setClientActive(req.ClientID, req.Active)
	return c.JSON(http.StatusOK, map[string]bool{"ok": true})
}

func (s *Server) handleVAPIDPublicKey(c *echo.Context) error {
	if s.pushManager == nil {
		return c.JSON(http.StatusServiceUnavailable, map[string]string{"error": "push not available"})
	}
	return c.JSON(http.StatusOK, map[string]string{"key": s.pushManager.VAPIDPublicKey()})
}

// --- Task handlers ---

func (s *Server) handleTaskList(c *echo.Context) error {
	if s.todoService == nil {
		return c.JSON(http.StatusServiceUnavailable, map[string]string{"error": "tasks not available"})
	}

	status := c.QueryParam("status")
	tag := c.QueryParam("tag")
	tasks := s.todoService.ListTasks(status, tag)
	if tasks == nil {
		tasks = []todo.Task{}
	}
	return c.JSON(http.StatusOK, map[string]any{"tasks": tasks})
}

func (s *Server) handleTaskCreate(c *echo.Context) error {
	if s.todoService == nil {
		return c.JSON(http.StatusServiceUnavailable, map[string]string{"error": "tasks not available"})
	}

	var task todo.Task
	if err := c.Bind(&task); err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "invalid request"})
	}
	if task.Title == "" {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "title is required"})
	}

	created, err := s.todoService.AddTask(task)
	if err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": err.Error()})
	}
	return c.JSON(http.StatusOK, created)
}

func (s *Server) handleTaskUpdate(c *echo.Context) error {
	if s.todoService == nil {
		return c.JSON(http.StatusServiceUnavailable, map[string]string{"error": "tasks not available"})
	}

	id := c.Param("id")
	var patch map[string]any
	if err := c.Bind(&patch); err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "invalid request"})
	}

	task, err := s.todoService.UpdateTask(id, patch)
	if err != nil {
		return c.JSON(http.StatusNotFound, map[string]string{"error": err.Error()})
	}
	return c.JSON(http.StatusOK, task)
}

func (s *Server) handleTaskDone(c *echo.Context) error {
	if s.todoService == nil {
		return c.JSON(http.StatusServiceUnavailable, map[string]string{"error": "tasks not available"})
	}

	id := c.Param("id")
	task, err := s.todoService.CompleteTask(id)
	if err != nil {
		return c.JSON(http.StatusNotFound, map[string]string{"error": err.Error()})
	}
	return c.JSON(http.StatusOK, task)
}

func (s *Server) handleTaskDelete(c *echo.Context) error {
	if s.todoService == nil {
		return c.JSON(http.StatusServiceUnavailable, map[string]string{"error": "tasks not available"})
	}

	id := c.Param("id")
	if s.todoService.RemoveTask(id) {
		return c.JSON(http.StatusOK, map[string]bool{"ok": true})
	}
	return c.JSON(http.StatusNotFound, map[string]string{"error": "task not found"})
}

func (s *Server) handlePushSubscribe(c *echo.Context) error {
	if s.pushManager == nil {
		return c.JSON(http.StatusServiceUnavailable, map[string]string{"error": "push not available"})
	}

	var sub webpush.Subscription
	if err := c.Bind(&sub); err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "invalid subscription"})
	}
	if sub.Endpoint == "" {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "missing endpoint"})
	}

	if err := s.pushManager.AddSubscription(sub); err != nil {
		logger.Error("push: subscribe failed: %v", err)
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "failed to save subscription"})
	}

	return c.JSON(http.StatusOK, map[string]bool{"ok": true})
}
