package webchat

import (
	"context"
	"embed"
	"net/http"
	"path/filepath"
	"strings"

	"localagent/pkg/logger"

	"github.com/labstack/echo/v5"
	"github.com/labstack/echo/v5/middleware"
)

//go:embed static/*
var staticFiles embed.FS

type Server struct {
	echo        *echo.Echo
	httpServer  *http.Server
	addr        string
	channel     *WebChatChannel
	mediaDir    string
	imageJobs   *ImageJobStore
	pushManager *PushManager
}

func NewServer(addr string, channel *WebChatChannel) *Server {
	e := echo.New()
	e.Use(middleware.Recover())
	e.Use(middleware.Secure())
	e.Use(middleware.BodyLimit(10 * 1024 * 1024))
	e.Use(middleware.GzipWithConfig(middleware.GzipConfig{
		Skipper: func(c *echo.Context) bool {
			return strings.HasSuffix(c.Request().URL.Path, "/events")
		},
	}))

	webchatDir := filepath.Join(channel.dataDir, "webchat")

	pm, err := NewPushManager(webchatDir)
	if err != nil {
		logger.Warn("push notifications disabled: %v", err)
	}

	s := &Server{
		echo:        e,
		addr:        addr,
		channel:     channel,
		mediaDir:    filepath.Join(webchatDir, "media"),
		imageJobs:   NewImageJobStore(filepath.Join(webchatDir, "images")),
		pushManager: pm,
	}

	s.setupRoutes()
	return s
}

func (s *Server) setupRoutes() {
	s.echo.POST("/api/messages", s.handleSendMessage)
	s.echo.POST("/api/upload", s.handleUpload)
	s.echo.GET("/api/history", s.handleHistory)
	s.echo.GET("/api/events", s.handleSSE)
	s.echo.GET("/api/media/:filename", s.handleMedia)
	s.echo.POST("/api/transcribe", s.handleTranscribe)

	s.echo.GET("/api/image/models", s.handleImageModels)
	s.echo.POST("/api/image/unload", s.handleImageUnload)
	s.echo.POST("/api/image/generate", s.handleImageGenerate)
	s.echo.POST("/api/image/edit", s.handleImageEdit)
	s.echo.POST("/api/image/upscale", s.handleImageUpscale)
	s.echo.GET("/api/image/jobs", s.handleImageJobs)
	s.echo.GET("/api/image/jobs/:id", s.handleImageJob)
	s.echo.DELETE("/api/image/jobs/:id", s.handleImageDelete)
	s.echo.GET("/api/image/result/:id/:index", s.handleImageResult)
	s.echo.DELETE("/api/image/result/:id/:index", s.handleImageResultDelete)
	s.echo.GET("/api/image/source/:id/:index", s.handleImageSource)

	s.echo.GET("/api/push/vapid-public-key", s.handleVAPIDPublicKey)
	s.echo.POST("/api/push/subscribe", s.handlePushSubscribe)

	s.echo.GET("/*", s.handleSPA)
}

func (s *Server) Start() error {
	s.httpServer = &http.Server{
		Addr:    s.addr,
		Handler: s.echo,
	}
	return s.httpServer.ListenAndServe()
}

func (s *Server) Stop(ctx context.Context) error {
	s.imageJobs.Stop()
	if s.httpServer != nil {
		return s.httpServer.Shutdown(ctx)
	}
	return nil
}
