package webchat

import (
	"context"
	"embed"
	"net/http"
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
		echo:    e,
		addr:    addr,
		channel: channel,
	}

	s.setupRoutes()
	return s
}

func (s *Server) setupRoutes() {
	s.echo.POST("/api/messages", s.handleSendMessage)
	s.echo.POST("/api/upload", s.handleUpload)
	s.echo.GET("/api/history", s.handleHistory)
	s.echo.GET("/api/events", s.handleSSE)

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
	if s.httpServer != nil {
		return s.httpServer.Shutdown(ctx)
	}
	return nil
}
