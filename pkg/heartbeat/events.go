package heartbeat

import (
	"sync"
	"time"
)

type Event struct {
	Source     string
	Message    string
	Channel    string
	ChatID     string
	EnqueuedAt time.Time
}

type EventQueue struct {
	events []Event
	mu     sync.Mutex
	notify chan struct{}
}

func NewEventQueue() *EventQueue {
	return &EventQueue{
		notify: make(chan struct{}, 1),
	}
}

func (q *EventQueue) Enqueue(e Event) {
	q.mu.Lock()
	defer q.mu.Unlock()
	if e.EnqueuedAt.IsZero() {
		e.EnqueuedAt = time.Now()
	}
	q.events = append(q.events, e)
}

func (q *EventQueue) EnqueueAndWake(e Event) {
	q.Enqueue(e)
	select {
	case q.notify <- struct{}{}:
	default:
	}
}

func (q *EventQueue) Drain() []Event {
	q.mu.Lock()
	defer q.mu.Unlock()
	if len(q.events) == 0 {
		return nil
	}
	events := q.events
	q.events = nil
	return events
}

func (q *EventQueue) WakeChan() <-chan struct{} {
	return q.notify
}
