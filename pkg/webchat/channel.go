package webchat

import (
	"context"
	"fmt"
	"sync"
	"sync/atomic"

	"localagent/pkg/activity"
	"localagent/pkg/bus"
	"localagent/pkg/channels"
	"localagent/pkg/config"
	"localagent/pkg/logger"
	"localagent/pkg/session"
	"localagent/pkg/todo"
)

type OutgoingEvent struct {
	Type       string        `json:"type"`
	Role       string        `json:"role,omitempty"`
	Content    string        `json:"content,omitempty"`
	Event      *ActivityData `json:"event,omitempty"`
	Processing *bool         `json:"processing,omitempty"`
	ClientID   string        `json:"client_id,omitempty"`
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
	active bool
}

type WebChatChannel struct {
	*channels.BaseChannel
	config      *config.WebChatConfig
	server      *Server
	sessions    *session.SessionManager
	todoService *todo.TodoService
	dataDir     string
	stt         config.STTConfig
	image       config.ImageConfig
	clients     map[string]*sseClient
	mu          sync.RWMutex
	processing  atomic.Bool
}

func NewWebChatChannel(cfg *config.WebChatConfig, msgBus *bus.MessageBus, dataDir string, stt config.STTConfig, image config.ImageConfig) *WebChatChannel {
	base := channels.NewBaseChannel("web", cfg, msgBus, nil)
	ch := &WebChatChannel{
		BaseChannel: base,
		config:      cfg,
		dataDir:     dataDir,
		stt:         stt,
		image:       image,
		clients:     make(map[string]*sseClient),
	}
	return ch
}

func (ch *WebChatChannel) SetSessionManager(sm *session.SessionManager) {
	ch.sessions = sm
}

func (ch *WebChatChannel) SetTodoService(ts *todo.TodoService) {
	ch.todoService = ts
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

	if ch.server != nil && ch.server.pushManager != nil && !ch.hasActiveClient() {
		body := msg.Content
		if len(body) > 200 {
			body = body[:200] + "..."
		}
		go ch.server.pushManager.SendPush("localagent", body, "/")
	}

	return nil
}

func (ch *WebChatChannel) Emit(evt activity.Event) {
	if evt.Type == activity.ProcessingStart {
		ch.processing.Store(true)
	} else if evt.Type == activity.Complete {
		ch.processing.Store(false)
	}

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
	if !ch.IsAllowed("web-user") {
		return
	}

	sessionKey := fmt.Sprintf("%s:default", ch.Name())

	// Persist user message to session immediately so it survives page refresh
	// even if the agent hasn't picked it up from the bus yet.
	if ch.sessions != nil {
		ch.sessions.AddMessageWithMedia(sessionKey, "user", content, media)
	}

	ch.Bus().PublishInbound(bus.InboundMessage{
		Channel:    ch.Name(),
		SenderID:   "web-user",
		ChatID:     "default",
		Content:    content,
		Media:      media,
		SessionKey: sessionKey,
		Metadata:   metadata,
		Persisted:  true,
	})
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

func (ch *WebChatChannel) setClientActive(id string, active bool) bool {
	ch.mu.Lock()
	defer ch.mu.Unlock()
	client, ok := ch.clients[id]
	if !ok {
		return false
	}
	client.active = active
	return true
}

func (ch *WebChatChannel) hasActiveClient() bool {
	ch.mu.RLock()
	defer ch.mu.RUnlock()
	for _, client := range ch.clients {
		if client.active {
			return true
		}
	}
	return false
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
