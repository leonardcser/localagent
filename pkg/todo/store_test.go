package todo

import (
	"os"
	"path/filepath"
	"testing"
)

func tempStorePath(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	return filepath.Join(dir, "todo", "tasks.json")
}

func TestAddAndList(t *testing.T) {
	s := NewTodoService(tempStorePath(t))
	s.Load()

	task, err := s.AddTask(Task{Title: "Buy groceries"})
	if err != nil {
		t.Fatalf("AddTask: %v", err)
	}
	if task.ID == "" {
		t.Fatal("expected non-empty ID")
	}
	if task.Status != "todo" {
		t.Fatalf("expected status 'todo', got %q", task.Status)
	}

	tasks := s.ListTasks("", "")
	if len(tasks) != 1 {
		t.Fatalf("expected 1 task, got %d", len(tasks))
	}
	if tasks[0].Title != "Buy groceries" {
		t.Fatalf("expected 'Buy groceries', got %q", tasks[0].Title)
	}
}

func TestListFilterByStatus(t *testing.T) {
	s := NewTodoService(tempStorePath(t))
	s.Load()

	s.AddTask(Task{Title: "A", Status: "todo"})
	s.AddTask(Task{Title: "B", Status: "doing"})

	todos := s.ListTasks("todo", "")
	if len(todos) != 1 || todos[0].Title != "A" {
		t.Fatalf("expected 1 todo task 'A', got %v", todos)
	}

	doing := s.ListTasks("doing", "")
	if len(doing) != 1 || doing[0].Title != "B" {
		t.Fatalf("expected 1 doing task 'B', got %v", doing)
	}
}

func TestListFilterByTag(t *testing.T) {
	s := NewTodoService(tempStorePath(t))
	s.Load()

	s.AddTask(Task{Title: "A", Tags: []string{"work"}})
	s.AddTask(Task{Title: "B", Tags: []string{"personal"}})

	work := s.ListTasks("", "work")
	if len(work) != 1 || work[0].Title != "A" {
		t.Fatalf("expected 1 work task, got %v", work)
	}
}

func TestCompleteTask(t *testing.T) {
	s := NewTodoService(tempStorePath(t))
	s.Load()

	task, _ := s.AddTask(Task{Title: "Do laundry"})

	completed, err := s.CompleteTask(task.ID)
	if err != nil {
		t.Fatalf("CompleteTask: %v", err)
	}
	if completed.Status != "done" {
		t.Fatalf("expected status 'done', got %q", completed.Status)
	}
	if completed.DoneAtMS == nil {
		t.Fatal("expected DoneAtMS to be set")
	}
}

func TestCompleteRecurringTask(t *testing.T) {
	s := NewTodoService(tempStorePath(t))
	s.Load()

	task, _ := s.AddTask(Task{
		Title:      "Daily standup",
		Due:        "2026-02-20",
		Recurrence: "daily",
	})

	_, err := s.CompleteTask(task.ID)
	if err != nil {
		t.Fatalf("CompleteTask: %v", err)
	}

	tasks := s.ListTasks("", "")
	if len(tasks) != 2 {
		t.Fatalf("expected 2 tasks (completed + new), got %d", len(tasks))
	}

	var newTask *Task
	for i := range tasks {
		if tasks[i].Status == "todo" {
			newTask = &tasks[i]
			break
		}
	}
	if newTask == nil {
		t.Fatal("expected a new todo task from recurrence")
	}
	if newTask.Due != "2026-02-21" {
		t.Fatalf("expected due '2026-02-21', got %q", newTask.Due)
	}
	if newTask.Recurrence != "daily" {
		t.Fatalf("expected recurrence 'daily', got %q", newTask.Recurrence)
	}
}

func TestCompleteWeeklyRecurrence(t *testing.T) {
	s := NewTodoService(tempStorePath(t))
	s.Load()

	task, _ := s.AddTask(Task{
		Title:      "Weekly review",
		Due:        "2026-02-20",
		Recurrence: "weekly",
	})

	s.CompleteTask(task.ID)

	tasks := s.ListTasks("todo", "")
	if len(tasks) != 1 || tasks[0].Due != "2026-02-27" {
		t.Fatalf("expected weekly recurrence to 2026-02-27, got %v", tasks)
	}
}

func TestCompleteMonthlyRecurrence(t *testing.T) {
	s := NewTodoService(tempStorePath(t))
	s.Load()

	task, _ := s.AddTask(Task{
		Title:      "Pay rent",
		Due:        "2026-02-01",
		Recurrence: "monthly",
	})

	s.CompleteTask(task.ID)

	tasks := s.ListTasks("todo", "")
	if len(tasks) != 1 || tasks[0].Due != "2026-03-01" {
		t.Fatalf("expected monthly recurrence to 2026-03-01, got %v", tasks)
	}
}

func TestUpdateTask(t *testing.T) {
	s := NewTodoService(tempStorePath(t))
	s.Load()

	task, _ := s.AddTask(Task{Title: "Original"})

	updated, err := s.UpdateTask(task.ID, map[string]any{
		"title":    "Updated",
		"priority": "high",
	})
	if err != nil {
		t.Fatalf("UpdateTask: %v", err)
	}
	if updated.Title != "Updated" {
		t.Fatalf("expected title 'Updated', got %q", updated.Title)
	}
	if updated.Priority != "high" {
		t.Fatalf("expected priority 'high', got %q", updated.Priority)
	}
}

func TestRemoveTask(t *testing.T) {
	s := NewTodoService(tempStorePath(t))
	s.Load()

	task, _ := s.AddTask(Task{Title: "To remove"})

	if !s.RemoveTask(task.ID) {
		t.Fatal("expected RemoveTask to return true")
	}

	tasks := s.ListTasks("", "")
	if len(tasks) != 0 {
		t.Fatalf("expected 0 tasks, got %d", len(tasks))
	}

	if s.RemoveTask("nonexistent") {
		t.Fatal("expected RemoveTask for nonexistent to return false")
	}
}

func TestPersistence(t *testing.T) {
	storePath := tempStorePath(t)

	s1 := NewTodoService(storePath)
	s1.Load()
	s1.AddTask(Task{Title: "Persistent task"})

	s2 := NewTodoService(storePath)
	s2.Load()

	tasks := s2.ListTasks("", "")
	if len(tasks) != 1 || tasks[0].Title != "Persistent task" {
		t.Fatalf("expected persisted task, got %v", tasks)
	}
}

func TestLoadMissingFile(t *testing.T) {
	s := NewTodoService(filepath.Join(t.TempDir(), "nonexistent", "tasks.json"))
	if err := s.Load(); err != nil {
		t.Fatalf("Load should not error on missing file: %v", err)
	}

	tasks := s.ListTasks("", "")
	if len(tasks) != 0 {
		t.Fatalf("expected empty store, got %d tasks", len(tasks))
	}
}

func TestComputeNextDue(t *testing.T) {
	tests := []struct {
		due        string
		recurrence string
		want       string
	}{
		{"2026-02-20", "daily", "2026-02-21"},
		{"2026-02-20", "weekly", "2026-02-27"},
		{"2026-01-31", "monthly", "2026-03-03"},
		{"2026-02-20", "yearly", ""},
		{"bad-date", "daily", ""},
	}

	for _, tt := range tests {
		got := computeNextDue(tt.due, tt.recurrence)
		if got != tt.want {
			t.Errorf("computeNextDue(%q, %q) = %q, want %q", tt.due, tt.recurrence, got, tt.want)
		}
	}
}

func TestStoreFileCreated(t *testing.T) {
	storePath := tempStorePath(t)
	s := NewTodoService(storePath)
	s.Load()
	s.AddTask(Task{Title: "test"})

	if _, err := os.Stat(storePath); os.IsNotExist(err) {
		t.Fatal("expected store file to be created")
	}
}
