package webchat

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"io/fs"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"localagent/pkg/logger"
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

type historyMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
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

	mediaDir := filepath.Join(os.TempDir(), "localagent_media")
	if err := os.MkdirAll(mediaDir, 0700); err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "failed to create media directory"})
	}

	safeName := utils.SanitizeFilename(file.Filename)
	prefix := randHex(8)
	localPath := filepath.Join(mediaDir, prefix+"_"+safeName)

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

func (s *Server) handleHistory(c *echo.Context) error {
	if s.channel.sessions == nil {
		return c.JSON(http.StatusOK, []historyMessage{})
	}

	history := s.channel.sessions.GetHistory("web:default")
	messages := make([]historyMessage, 0, len(history))
	for _, msg := range history {
		if msg.Role == "user" || msg.Role == "assistant" {
			messages = append(messages, historyMessage{
				Role:    msg.Role,
				Content: msg.Content,
			})
		}
	}

	return c.JSON(http.StatusOK, messages)
}

func (s *Server) handleSSE(c *echo.Context) error {
	clientID := randHex(16)
	client := s.channel.registerClient(clientID)

	w := c.Response()
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.WriteHeader(http.StatusOK)

	rc := http.NewResponseController(w)
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

func randHex(n int) string {
	b := make([]byte, n)
	rand.Read(b)
	return hex.EncodeToString(b)
}
