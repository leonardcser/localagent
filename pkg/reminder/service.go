package reminder

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"localagent/pkg/logger"
	"localagent/pkg/webchat"
)

var offsets = map[string]time.Duration{
	"15m": 15 * time.Minute,
	"30m": 30 * time.Minute,
	"1h":  time.Hour,
	"2h":  2 * time.Hour,
	"1d":  24 * time.Hour,
	"2d":  48 * time.Hour,
	"1w":  7 * 24 * time.Hour,
}

// dayLevelOffsets are offsets that make sense for date-only dues (no time component).
var dayLevelOffsets = map[string]bool{
	"1d": true, "2d": true, "1w": true,
}

type taskRow struct {
	id        string
	title     string
	due       string
	reminders string
}

type Service struct {
	db   *sql.DB
	push *webchat.PushManager
	stop chan struct{}
}

func NewService(db *sql.DB, push *webchat.PushManager) *Service {
	return &Service{db: db, push: push, stop: make(chan struct{})}
}

func (s *Service) Start() {
	ticker := time.NewTicker(30 * time.Second)
	go func() {
		s.check() // run immediately on start
		for {
			select {
			case <-ticker.C:
				s.check()
			case <-s.stop:
				ticker.Stop()
				return
			}
		}
	}()
	logger.Info("reminder service started")
}

func (s *Service) Stop() {
	close(s.stop)
}

func (s *Service) check() {
	rows, err := s.db.Query(
		`SELECT id, title, due, reminders FROM tasks
		 WHERE status != 'done' AND reminders != '[]' AND due != ''`,
	)
	if err != nil {
		logger.Error("reminder: query tasks: %v", err)
		return
	}
	defer rows.Close()

	now := time.Now()
	nowMs := now.UnixMilli()

	for rows.Next() {
		var t taskRow
		if err := rows.Scan(&t.id, &t.title, &t.due, &t.reminders); err != nil {
			continue
		}

		dueTime, hasTime := parseDue(t.due)
		if dueTime.IsZero() {
			continue
		}

		var reminderOffsets []string
		if err := json.Unmarshal([]byte(t.reminders), &reminderOffsets); err != nil {
			continue
		}

		for _, offsetKey := range reminderOffsets {
			dur, ok := offsets[offsetKey]
			if !ok {
				continue
			}
			// Skip sub-day offsets for date-only dues
			if !hasTime && !dayLevelOffsets[offsetKey] {
				continue
			}

			fireAtMs := dueTime.Add(-dur).UnixMilli()
			if fireAtMs > nowMs {
				continue // not yet
			}

			// Check if already sent
			if s.alreadySent(t.id, offsetKey, fireAtMs) {
				continue
			}

			// Send notification
			body := fmt.Sprintf("Due %s", humanizeOffset(offsetKey))
			s.push.SendPush(webchat.PushMessage{
				Type:   "reminder",
				Title:  t.title,
				Body:   body,
				URL:    "/tasks",
				TaskID: t.id,
			})

			s.recordSent(t.id, offsetKey, fireAtMs, nowMs)
			logger.Info("reminder: sent %s for task %q (%s)", offsetKey, t.title, t.id)
		}
	}

	// Cleanup old entries (> 30 days)
	s.db.Exec(`DELETE FROM sent_reminders WHERE sent_at_ms < ?`, nowMs-30*24*60*60*1000)
}

func (s *Service) alreadySent(taskID, offset string, fireAtMs int64) bool {
	var count int
	err := s.db.QueryRow(
		`SELECT COUNT(*) FROM sent_reminders WHERE task_id = ? AND offset = ? AND fire_at_ms = ?`,
		taskID, offset, fireAtMs,
	).Scan(&count)
	return err == nil && count > 0
}

func (s *Service) recordSent(taskID, offset string, fireAtMs, sentAtMs int64) {
	s.db.Exec(
		`INSERT OR IGNORE INTO sent_reminders (task_id, offset, fire_at_ms, sent_at_ms) VALUES (?, ?, ?, ?)`,
		taskID, offset, fireAtMs, sentAtMs,
	)
}

func parseDue(due string) (time.Time, bool) {
	loc := time.Now().Location()
	if strings.Contains(due, "T") {
		t, err := time.ParseInLocation("2006-01-02T15:04", due, loc)
		if err != nil {
			return time.Time{}, false
		}
		return t, true
	}
	t, err := time.ParseInLocation("2006-01-02", due, loc)
	if err != nil {
		return time.Time{}, false
	}
	// Default to 6am local time for date-only dues
	t = t.Add(6 * time.Hour)
	return t, false
}

func humanizeOffset(key string) string {
	switch key {
	case "15m":
		return "in 15 minutes"
	case "30m":
		return "in 30 minutes"
	case "1h":
		return "in 1 hour"
	case "2h":
		return "in 2 hours"
	case "1d":
		return "tomorrow"
	case "2d":
		return "in 2 days"
	case "1w":
		return "in 1 week"
	default:
		return "soon"
	}
}
