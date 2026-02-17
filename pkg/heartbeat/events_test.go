package heartbeat

import (
	"sync"
	"testing"
	"time"
)

func TestEnqueueDrain(t *testing.T) {
	q := NewEventQueue()

	q.Enqueue(Event{Source: "cron", Message: "task 1"})
	q.Enqueue(Event{Source: "cron", Message: "task 2"})
	q.Enqueue(Event{Source: "cron", Message: "task 3"})

	events := q.Drain()
	if len(events) != 3 {
		t.Fatalf("expected 3 events, got %d", len(events))
	}

	if events[0].Message != "task 1" || events[2].Message != "task 3" {
		t.Fatal("events not in order")
	}

	events = q.Drain()
	if events != nil {
		t.Fatalf("expected nil after second drain, got %d events", len(events))
	}
}

func TestEnqueueAndWake(t *testing.T) {
	q := NewEventQueue()

	q.EnqueueAndWake(Event{Source: "cron", Message: "urgent"})

	select {
	case <-q.WakeChan():
	case <-time.After(100 * time.Millisecond):
		t.Fatal("WakeChan did not fire")
	}

	// Second wake without drain should not block
	q.EnqueueAndWake(Event{Source: "cron", Message: "another"})

	events := q.Drain()
	if len(events) != 2 {
		t.Fatalf("expected 2 events, got %d", len(events))
	}
}

func TestDrainEmpty(t *testing.T) {
	q := NewEventQueue()
	events := q.Drain()
	if events != nil {
		t.Fatalf("expected nil for empty drain, got %d events", len(events))
	}
}

func TestConcurrentEnqueue(t *testing.T) {
	q := NewEventQueue()
	var wg sync.WaitGroup
	n := 100

	for range n {
		wg.Go(func() {
			q.Enqueue(Event{Source: "test", Message: "msg"})
		})
	}

	wg.Wait()

	events := q.Drain()
	if len(events) != n {
		t.Fatalf("expected %d events, got %d", n, len(events))
	}
}

func TestEnqueueSetsTimestamp(t *testing.T) {
	q := NewEventQueue()
	before := time.Now()
	q.Enqueue(Event{Source: "test", Message: "msg"})
	after := time.Now()

	events := q.Drain()
	if events[0].EnqueuedAt.Before(before) || events[0].EnqueuedAt.After(after) {
		t.Fatal("EnqueuedAt not set correctly")
	}
}
