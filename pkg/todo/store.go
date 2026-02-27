package todo

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"sync"
	"time"

	"localagent/pkg/utils"
)

type Task struct {
	ID          string   `json:"id"`
	Title       string   `json:"title"`
	Description string   `json:"description,omitempty"`
	Status      string   `json:"status"`
	Priority    string   `json:"priority,omitempty"`
	Due         string   `json:"due,omitempty"`
	Recurrence  string   `json:"recurrence,omitempty"`
	Tags        []string `json:"tags,omitempty"`
	ParentID    string   `json:"parentId,omitempty"`
	CreatedAtMS int64    `json:"createdAtMs"`
	UpdatedAtMS int64    `json:"updatedAtMs"`
	DoneAtMS    *int64   `json:"doneAtMs,omitempty"`
}

type TaskStore struct {
	Version int    `json:"version"`
	Tasks   []Task `json:"tasks"`
}

type TodoService struct {
	storePath string
	store     *TaskStore
	mu        sync.RWMutex
}

func NewTodoService(storePath string) *TodoService {
	return &TodoService{
		storePath: storePath,
	}
}

func (s *TodoService) Load() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.loadStore()
}

func (s *TodoService) loadStore() error {
	s.store = &TaskStore{
		Version: 1,
		Tasks:   []Task{},
	}

	data, err := os.ReadFile(s.storePath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}

	return json.Unmarshal(data, s.store)
}

func (s *TodoService) saveStoreUnsafe() error {
	dir := filepath.Dir(s.storePath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return err
	}

	data, err := json.MarshalIndent(s.store, "", "  ")
	if err != nil {
		return err
	}

	return os.WriteFile(s.storePath, data, 0644)
}

func (s *TodoService) ListTasks(status string, tag string) []Task {
	s.mu.RLock()
	defer s.mu.RUnlock()

	var result []Task
	for _, t := range s.store.Tasks {
		if status != "" && t.Status != status {
			continue
		}
		if tag != "" && !hasTag(t.Tags, tag) {
			continue
		}
		result = append(result, t)
	}
	return result
}

func (s *TodoService) AddTask(task Task) (*Task, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	now := time.Now().UnixMilli()

	if task.ID == "" {
		task.ID = utils.RandHex(8)
	}
	if task.Status == "" {
		task.Status = "todo"
	}
	task.CreatedAtMS = now
	task.UpdatedAtMS = now

	s.store.Tasks = append(s.store.Tasks, task)
	if err := s.saveStoreUnsafe(); err != nil {
		return nil, err
	}

	return &s.store.Tasks[len(s.store.Tasks)-1], nil
}

func (s *TodoService) UpdateTask(taskID string, patch map[string]any) (*Task, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	var task *Task
	for i := range s.store.Tasks {
		if s.store.Tasks[i].ID == taskID {
			task = &s.store.Tasks[i]
			break
		}
	}
	if task == nil {
		return nil, fmt.Errorf("task not found: %s", taskID)
	}

	if title, ok := patch["title"].(string); ok {
		task.Title = title
	}
	if desc, ok := patch["description"].(string); ok {
		task.Description = desc
	}
	if status, ok := patch["status"].(string); ok {
		task.Status = status
	}
	if priority, ok := patch["priority"].(string); ok {
		task.Priority = priority
	}
	if due, ok := patch["due"].(string); ok {
		task.Due = due
	}
	if recurrence, ok := patch["recurrence"].(string); ok {
		task.Recurrence = recurrence
	}
	if tags, ok := patch["tags"]; ok {
		task.Tags = toStringSlice(tags)
	}
	if parentID, ok := patch["parentId"].(string); ok {
		task.ParentID = parentID
	}

	task.UpdatedAtMS = time.Now().UnixMilli()
	if err := s.saveStoreUnsafe(); err != nil {
		return nil, err
	}

	return task, nil
}

func (s *TodoService) CompleteTask(taskID string) (*Task, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	var task *Task
	for i := range s.store.Tasks {
		if s.store.Tasks[i].ID == taskID {
			task = &s.store.Tasks[i]
			break
		}
	}
	if task == nil {
		return nil, fmt.Errorf("task not found: %s", taskID)
	}

	now := time.Now().UnixMilli()
	task.Status = "done"
	task.DoneAtMS = &now
	task.UpdatedAtMS = now

	if task.Recurrence != "" && task.Due != "" {
		nextDue := computeNextDue(task.Due, task.Recurrence)
		if nextDue != "" {
			newTask := Task{
				ID:          utils.RandHex(8),
				Title:       task.Title,
				Description: task.Description,
				Status:      "todo",
				Priority:    task.Priority,
				Due:         nextDue,
				Recurrence:  task.Recurrence,
				Tags:        task.Tags,
				CreatedAtMS: now,
				UpdatedAtMS: now,
			}
			s.store.Tasks = append(s.store.Tasks, newTask)
		}
	}

	if err := s.saveStoreUnsafe(); err != nil {
		return nil, err
	}

	return task, nil
}

func (s *TodoService) RemoveTask(taskID string) bool {
	s.mu.Lock()
	defer s.mu.Unlock()

	before := len(s.store.Tasks)
	var tasks []Task
	for _, t := range s.store.Tasks {
		if t.ID != taskID && t.ParentID != taskID {
			tasks = append(tasks, t)
		}
	}
	s.store.Tasks = tasks
	removed := len(s.store.Tasks) < before

	if removed {
		_ = s.saveStoreUnsafe()
	}

	return removed
}

func computeNextDue(due string, recurrence string) string {
	t, err := time.Parse("2006-01-02", due)
	if err != nil {
		return ""
	}

	switch recurrence {
	case "daily":
		return t.AddDate(0, 0, 1).Format("2006-01-02")
	case "weekly":
		return t.AddDate(0, 0, 7).Format("2006-01-02")
	case "monthly":
		return t.AddDate(0, 1, 0).Format("2006-01-02")
	default:
		return ""
	}
}

func hasTag(tags []string, tag string) bool {
	return slices.Contains(tags, tag)
}

func toStringSlice(v any) []string {
	if arr, ok := v.([]any); ok {
		result := make([]string, 0, len(arr))
		for _, item := range arr {
			if s, ok := item.(string); ok {
				result = append(result, s)
			}
		}
		return result
	}
	if arr, ok := v.([]string); ok {
		return arr
	}
	return nil
}
