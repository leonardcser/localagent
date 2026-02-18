package webchat

import (
	"context"
	"embed"
	"net/http"
	"path/filepath"
	"strings"

	"github.com/labstack/echo/v5"
	"github.com/labstack/echo/v5/middleware"
)

//go:embed static/*
var staticFiles embed.FS

type Server struct {
	echo       *echo.Echo
	httpServer *http.Server
	addr       string
	channel    *WebChatChannel
	imageJobs  *ImageJobStore
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

	s := &Server{
		echo:      e,
		addr:      addr,
		channel:   channel,
		imageJobs: NewImageJobStore(filepath.Join(channel.workspace, "images")),
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
	s.echo.POST("/api/image/generate", s.handleImageGenerate)
	s.echo.POST("/api/image/edit", s.handleImageEdit)
	s.echo.POST("/api/image/upscale", s.handleImageUpscale)
	s.echo.GET("/api/image/jobs", s.handleImageJobs)
	s.echo.GET("/api/image/jobs/:id", s.handleImageJob)
	s.echo.DELETE("/api/image/jobs/:id", s.handleImageDelete)
	s.echo.GET("/api/image/result/:id/:index", s.handleImageResult)
	s.echo.DELETE("/api/image/result/:id/:index", s.handleImageResultDelete)
	s.echo.GET("/api/image/source/:id/:index", s.handleImageSource)

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
