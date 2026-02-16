package session

import (
	"bufio"
	"encoding/json"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"

	"localagent/pkg/activity"
	"localagent/pkg/logger"
	"localagent/pkg/providers"
)

// JSONL record type discriminators
const (
	recMsg = "msg"
	recAct = "act"
	recSum = "sum"
)

// JSONL record types

type baseRecord struct {
	T string `json:"t"`
}

type msgRecord struct {
	T   string            `json:"t"`
	Msg providers.Message `json:"msg"`
	Ts  time.Time         `json:"ts"`
}

type actRecord struct {
	T         string         `json:"t"`
	EventType string         `json:"event_type"`
	Message   string         `json:"message"`
	Detail    map[string]any `json:"detail,omitempty"`
	Ts        time.Time      `json:"ts"`
}

type sumRecord struct {
	T       string    `json:"t"`
	Content string    `json:"content"`
	Ts      time.Time `json:"ts"`
}

// Internal storage

type storedMessage struct {
	Msg providers.Message
	Ts  time.Time
}

type Session struct {
	Key      string
	messages []storedMessage
	Activity []activity.Event
	Summary  string
}

// TimelineEntry represents a single entry in the interleaved timeline.
type TimelineEntry struct {
	Kind      string // "message" or "activity"
	Message   *providers.Message
	Activity  *activity.Event
	Timestamp time.Time
}

type SessionManager struct {
	sessions map[string]*Session
	mu       sync.RWMutex
	storage  string
}

func NewSessionManager(storage string) *SessionManager {
	sm := &SessionManager{
		sessions: make(map[string]*Session),
		storage:  storage,
	}

	if storage != "" {
		os.MkdirAll(storage, 0755)
		sm.migrateJSON()
		sm.loadSessions()
	}

	return sm
}

func (sm *SessionManager) getOrCreate(key string) *Session {
	s, ok := sm.sessions[key]
	if !ok {
		s = &Session{Key: key}
		sm.sessions[key] = s
	}
	return s
}

func (sm *SessionManager) AddMessage(sessionKey, role, content string) {
	sm.AddFullMessage(sessionKey, providers.Message{
		Role:    role,
		Content: content,
	})
}

func (sm *SessionManager) AddFullMessage(sessionKey string, msg providers.Message) {
	now := time.Now()

	sm.mu.Lock()
	s := sm.getOrCreate(sessionKey)
	s.messages = append(s.messages, storedMessage{Msg: msg, Ts: now})
	sm.mu.Unlock()

	sm.appendRecord(sessionKey, msgRecord{
		T:   recMsg,
		Msg: msg,
		Ts:  now,
	})
}

func (sm *SessionManager) AddActivity(sessionKey string, evt activity.Event) {
	sm.mu.Lock()
	s := sm.getOrCreate(sessionKey)
	s.Activity = append(s.Activity, evt)
	sm.mu.Unlock()

	sm.appendRecord(sessionKey, actRecord{
		T:         recAct,
		EventType: string(evt.Type),
		Message:   evt.Message,
		Detail:    evt.Detail,
		Ts:        evt.Timestamp,
	})
}

func (sm *SessionManager) GetHistory(key string) []providers.Message {
	sm.mu.RLock()
	defer sm.mu.RUnlock()

	s, ok := sm.sessions[key]
	if !ok {
		return []providers.Message{}
	}

	msgs := make([]providers.Message, len(s.messages))
	for i, m := range s.messages {
		msgs[i] = m.Msg
	}
	return msgs
}

func (sm *SessionManager) GetActivity(key string) []activity.Event {
	sm.mu.RLock()
	defer sm.mu.RUnlock()

	s, ok := sm.sessions[key]
	if !ok {
		return nil
	}

	events := make([]activity.Event, len(s.Activity))
	copy(events, s.Activity)
	return events
}

func (sm *SessionManager) GetTimeline(key string) []TimelineEntry {
	sm.mu.RLock()
	defer sm.mu.RUnlock()

	s, ok := sm.sessions[key]
	if !ok {
		return nil
	}

	entries := make([]TimelineEntry, 0, len(s.messages)+len(s.Activity))
	for i := range s.messages {
		msg := s.messages[i].Msg
		entries = append(entries, TimelineEntry{
			Kind:      "message",
			Message:   &msg,
			Timestamp: s.messages[i].Ts,
		})
	}
	for i := range s.Activity {
		evt := s.Activity[i]
		entries = append(entries, TimelineEntry{
			Kind:      "activity",
			Activity:  &evt,
			Timestamp: s.Activity[i].Timestamp,
		})
	}

	sort.Slice(entries, func(i, j int) bool {
		return entries[i].Timestamp.Before(entries[j].Timestamp)
	})

	return entries
}

func (sm *SessionManager) GetSummary(key string) string {
	sm.mu.RLock()
	defer sm.mu.RUnlock()

	s, ok := sm.sessions[key]
	if !ok {
		return ""
	}
	return s.Summary
}

func (sm *SessionManager) SetSummary(key string, summary string) {
	now := time.Now()

	sm.mu.Lock()
	s, ok := sm.sessions[key]
	if ok {
		s.Summary = summary
	}
	sm.mu.Unlock()

	if ok {
		sm.appendRecord(key, sumRecord{
			T:       recSum,
			Content: summary,
			Ts:      now,
		})
	}
}

func (sm *SessionManager) TruncateHistory(key string, keepLast int) {
	sm.mu.Lock()
	defer sm.mu.Unlock()

	s, ok := sm.sessions[key]
	if !ok {
		return
	}

	if keepLast <= 0 {
		s.messages = nil
		s.Activity = nil
	} else if len(s.messages) > keepLast {
		kept := make([]storedMessage, keepLast)
		copy(kept, s.messages[len(s.messages)-keepLast:])
		s.messages = kept

		// Keep activity events newer than the oldest kept message
		cutoff := kept[0].Ts
		filtered := make([]activity.Event, 0)
		for _, a := range s.Activity {
			if !a.Timestamp.Before(cutoff) {
				filtered = append(filtered, a)
			}
		}
		s.Activity = filtered
	}

	sm.rewriteFile(key, s)
}

// Save is a no-op; writes are now immediate via append.
func (sm *SessionManager) Save(key string) error {
	return nil
}

// File I/O

func sanitizeFilename(key string) string {
	return strings.ReplaceAll(key, ":", "_")
}

func validateFilename(filename string) bool {
	return filename != "." && filepath.IsLocal(filename) && !strings.ContainsAny(filename, `/\`)
}

func (sm *SessionManager) appendRecord(key string, record any) {
	if sm.storage == "" {
		return
	}

	filename := sanitizeFilename(key)
	if !validateFilename(filename) {
		return
	}

	data, err := json.Marshal(record)
	if err != nil {
		logger.Warn("session: failed to marshal record for %s: %v", key, err)
		return
	}
	data = append(data, '\n')

	path := filepath.Join(sm.storage, filename+".jsonl")
	f, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		logger.Warn("session: failed to open %s for append: %v", path, err)
		return
	}
	defer f.Close()

	f.Write(data)
}

func (sm *SessionManager) rewriteFile(key string, s *Session) {
	if sm.storage == "" {
		return
	}

	filename := sanitizeFilename(key)
	if !validateFilename(filename) {
		return
	}

	path := filepath.Join(sm.storage, filename+".jsonl")
	tmpPath := path + ".tmp"

	f, err := os.Create(tmpPath)
	if err != nil {
		logger.Warn("session: failed to create temp file for rewrite: %v", err)
		return
	}

	enc := json.NewEncoder(f)

	// Write summary first
	if s.Summary != "" {
		enc.Encode(sumRecord{T: recSum, Content: s.Summary, Ts: time.Now()})
	}

	// Interleave messages and activity by timestamp
	mi, ai := 0, 0
	for mi < len(s.messages) || ai < len(s.Activity) {
		writeMsg := false
		if mi < len(s.messages) && ai < len(s.Activity) {
			writeMsg = !s.messages[mi].Ts.After(s.Activity[ai].Timestamp)
		} else {
			writeMsg = mi < len(s.messages)
		}

		if writeMsg {
			m := s.messages[mi]
			enc.Encode(msgRecord{T: recMsg, Msg: m.Msg, Ts: m.Ts})
			mi++
		} else {
			a := s.Activity[ai]
			enc.Encode(actRecord{
				T:         recAct,
				EventType: string(a.Type),
				Message:   a.Message,
				Detail:    a.Detail,
				Ts:        a.Timestamp,
			})
			ai++
		}
	}

	f.Close()

	if err := os.Rename(tmpPath, path); err != nil {
		logger.Warn("session: failed to rename temp file: %v", err)
		os.Remove(tmpPath)
	}
}

// Loading

func (sm *SessionManager) loadSessions() {
	files, err := os.ReadDir(sm.storage)
	if err != nil {
		return
	}

	for _, file := range files {
		if file.IsDir() || filepath.Ext(file.Name()) != ".jsonl" {
			continue
		}
		path := filepath.Join(sm.storage, file.Name())
		sm.loadJSONL(path)
	}
}

func (sm *SessionManager) loadJSONL(path string) {
	f, err := os.Open(path)
	if err != nil {
		return
	}
	defer f.Close()

	name := strings.TrimSuffix(filepath.Base(path), ".jsonl")
	key := strings.ReplaceAll(name, "_", ":")

	s := &Session{Key: key}

	scanner := bufio.NewScanner(f)
	scanner.Buffer(make([]byte, 0, 4096), 10*1024*1024) // 10MB max line

	for scanner.Scan() {
		line := scanner.Bytes()
		if len(line) == 0 {
			continue
		}

		var base baseRecord
		if err := json.Unmarshal(line, &base); err != nil {
			continue
		}

		switch base.T {
		case recMsg:
			var rec msgRecord
			if err := json.Unmarshal(line, &rec); err != nil {
				continue
			}
			s.messages = append(s.messages, storedMessage{Msg: rec.Msg, Ts: rec.Ts})

		case recAct:
			var rec actRecord
			if err := json.Unmarshal(line, &rec); err != nil {
				continue
			}
			s.Activity = append(s.Activity, activity.Event{
				Type:      activity.EventType(rec.EventType),
				Timestamp: rec.Ts,
				Message:   rec.Message,
				Detail:    rec.Detail,
			})

		case recSum:
			var rec sumRecord
			if err := json.Unmarshal(line, &rec); err != nil {
				continue
			}
			s.Summary = rec.Content // last summary wins
		}
	}

	sm.sessions[key] = s
}

// Migration from old JSON format

func (sm *SessionManager) migrateJSON() {
	files, err := os.ReadDir(sm.storage)
	if err != nil {
		return
	}

	for _, file := range files {
		if file.IsDir() || filepath.Ext(file.Name()) != ".json" {
			continue
		}

		jsonPath := filepath.Join(sm.storage, file.Name())
		data, err := os.ReadFile(jsonPath)
		if err != nil {
			continue
		}

		var old struct {
			Key      string              `json:"key"`
			Messages []providers.Message `json:"messages"`
			Summary  string              `json:"summary,omitempty"`
			Created  time.Time           `json:"created"`
			Updated  time.Time           `json:"updated"`
		}
		if err := json.Unmarshal(data, &old); err != nil {
			continue
		}

		s := &Session{
			Key:     old.Key,
			Summary: old.Summary,
		}

		// Distribute timestamps between Created and Updated
		for i, msg := range old.Messages {
			var ts time.Time
			if len(old.Messages) == 1 {
				ts = old.Updated
			} else {
				frac := float64(i) / float64(len(old.Messages)-1)
				dur := old.Updated.Sub(old.Created)
				ts = old.Created.Add(time.Duration(float64(dur) * frac))
			}
			s.messages = append(s.messages, storedMessage{Msg: msg, Ts: ts})
		}

		sm.sessions[old.Key] = s
		sm.rewriteFile(old.Key, s)

		os.Remove(jsonPath)
		logger.Info("session: migrated %s from JSON to JSONL", old.Key)
	}
}
