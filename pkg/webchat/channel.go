package webchat

import (
	"context"
	"fmt"
	"sync"

	"localagent/pkg/activity"
	"localagent/pkg/bus"
	"localagent/pkg/channels"
	"localagent/pkg/config"
	"localagent/pkg/logger"
	"localagent/pkg/session"
)

type OutgoingEvent struct {
	Type    string        `json:"type"`
	Role    string        `json:"role,omitempty"`
	Content string        `json:"content,omitempty"`
	Event   *ActivityData `json:"event,omitempty"`
}

type ActivityData struct {
	EventType string         `json:"event_type"`
	Timestamp string         `json:"timestamp"`
	Message   string         `json:"message"`
	Detail    map[string]any `json:"detail,omitempty"`
}

type sseClient struct {
	id     string
	events chan OutgoingEvent
}

type WebChatChannel struct {
	*channels.BaseChannel
	config    *config.WebChatConfig
	server    *Server
	sessions  *session.SessionManager
	workspace string
	clients   map[string]*sseClient
	mu        sync.RWMutex
}

func NewWebChatChannel(cfg *config.WebChatConfig, msgBus *bus.MessageBus, workspace string) *WebChatChannel {
	base := channels.NewBaseChannel("web", cfg, msgBus, nil)
	ch := &WebChatChannel{
		BaseChannel: base,
		config:      cfg,
		workspace:   workspace,
		clients:     make(map[string]*sseClient),
	}
	return ch
}

func (ch *WebChatChannel) SetSessionManager(sm *session.SessionManager) {
	ch.sessions = sm
}

func (ch *WebChatChannel) Start(ctx context.Context) error {
	addr := fmt.Sprintf("%s:%d", ch.config.Host, ch.config.Port)
	ch.server = NewServer(addr, ch)
	go func() {
		logger.Info("webchat server starting on %s", addr)
		if err := ch.server.Start(); err != nil {
			logger.Error("webchat server error: %v", err)
		}
	}()
	ch.SetRunning(true)
	return nil
}

func (ch *WebChatChannel) Stop(ctx context.Context) error {
	ch.SetRunning(false)
	ch.mu.Lock()
	for id, client := range ch.clients {
		close(client.events)
		delete(ch.clients, id)
	}
	ch.mu.Unlock()
	if ch.server != nil {
		return ch.server.Stop(ctx)
	}
	return nil
}

func (ch *WebChatChannel) Send(ctx context.Context, msg bus.OutboundMessage) error {
	event := OutgoingEvent{
		Type:    "message",
		Role:    "assistant",
		Content: msg.Content,
	}
	ch.broadcast(event)
	return nil
}

func (ch *WebChatChannel) Emit(evt activity.Event) {
	event := OutgoingEvent{
		Type: "activity",
		Event: &ActivityData{
			EventType: string(evt.Type),
			Timestamp: evt.Timestamp.Format("15:04:05"),
			Message:   evt.Message,
			Detail:    evt.Detail,
		},
	}
	ch.broadcast(event)
}

func (ch *WebChatChannel) IsAllowed(senderID string) bool {
	return true
}

func (ch *WebChatChannel) HandleIncoming(content string, media []string, metadata map[string]string) {
	ch.HandleMessage("web-user", "default", content, media, metadata)
}

func (ch *WebChatChannel) registerClient(id string) *sseClient {
	client := &sseClient{
		id:     id,
		events: make(chan OutgoingEvent, 64),
	}
	ch.mu.Lock()
	ch.clients[id] = client
	ch.mu.Unlock()
	logger.Info("webchat SSE client connected: %s", id)
	return client
}

func (ch *WebChatChannel) unregisterClient(id string) {
	ch.mu.Lock()
	if client, ok := ch.clients[id]; ok {
		close(client.events)
		delete(ch.clients, id)
	}
	ch.mu.Unlock()
	logger.Info("webchat SSE client disconnected: %s", id)
}

func (ch *WebChatChannel) broadcast(event OutgoingEvent) {
	ch.mu.RLock()
	defer ch.mu.RUnlock()
	for _, client := range ch.clients {
		select {
		case client.events <- event:
		default:
			logger.Warn("webchat SSE client %s buffer full, dropping message", client.id)
		}
	}
}
