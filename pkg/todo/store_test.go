package todo

import (
	"testing"

	"localagent/pkg/db"
)

func testService(t *testing.T) *TodoService {
	t.Helper()
	database, err := db.Open(":memory:")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { database.Close() })
	return NewTodoService(database)
}

func TestAddAndList(t *testing.T) {
	s := testService(t)

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
	s := testService(t)

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
	s := testService(t)

	s.AddTask(Task{Title: "A", Tags: []string{"work"}})
	s.AddTask(Task{Title: "B", Tags: []string{"personal"}})

	work := s.ListTasks("", "work")
	if len(work) != 1 || work[0].Title != "A" {
		t.Fatalf("expected 1 work task, got %v", work)
	}
}

func TestCompleteTask(t *testing.T) {
	s := testService(t)

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
	s := testService(t)

	task, _ := s.AddTask(Task{
		Title:      "Daily standup",
		Due:        "2026-02-20",
		Recurrence: "FREQ=DAILY",
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
	if newTask.Recurrence != "FREQ=DAILY" {
		t.Fatalf("expected recurrence 'FREQ=DAILY', got %q", newTask.Recurrence)
	}
}

func TestCompleteWeeklyRecurrence(t *testing.T) {
	s := testService(t)

	task, _ := s.AddTask(Task{
		Title:      "Weekly review",
		Due:        "2026-02-20",
		Recurrence: "FREQ=WEEKLY",
	})

	s.CompleteTask(task.ID)

	tasks := s.ListTasks("todo", "")
	if len(tasks) != 1 || tasks[0].Due != "2026-02-27" {
		t.Fatalf("expected weekly recurrence to 2026-02-27, got %v", tasks)
	}
}

func TestCompleteMonthlyRecurrence(t *testing.T) {
	s := testService(t)

	task, _ := s.AddTask(Task{
		Title:      "Pay rent",
		Due:        "2026-02-01",
		Recurrence: "FREQ=MONTHLY",
	})

	s.CompleteTask(task.ID)

	tasks := s.ListTasks("todo", "")
	if len(tasks) != 1 || tasks[0].Due != "2026-03-01" {
		t.Fatalf("expected monthly recurrence to 2026-03-01, got %v", tasks)
	}
}

func TestUpdateTask(t *testing.T) {
	s := testService(t)

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
	s := testService(t)

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

func TestComputeNextDue(t *testing.T) {
	tests := []struct {
		due        string
		recurrence string
		want       string
	}{
		{"2026-02-20", "FREQ=DAILY", "2026-02-21"},
		{"2026-02-20", "FREQ=WEEKLY", "2026-02-27"},
		{"2026-02-01", "FREQ=MONTHLY", "2026-03-01"},
		{"2026-02-20", "", ""},
		{"bad-date", "FREQ=DAILY", ""},
	}

	for _, tt := range tests {
		got := computeNextDue(tt.due, tt.recurrence)
		if got != tt.want {
			t.Errorf("computeNextDue(%q, %q) = %q, want %q", tt.due, tt.recurrence, got, tt.want)
		}
	}
}

// --- Slot tests ---

func TestSlotCRUD(t *testing.T) {
	s := testService(t)

	task, _ := s.AddTask(Task{Title: "Task for slots"})

	slot, err := s.AddSlot(Slot{
		TaskID:    task.ID,
		StartAtMS: 1000000,
		EndAtMS:   2000000,
		Note:      "Focus block",
	})
	if err != nil {
		t.Fatalf("AddSlot: %v", err)
	}
	if slot.ID == "" {
		t.Fatal("expected slot ID")
	}

	// List all
	slots := s.ListSlots("", 0, 0)
	if len(slots) != 1 {
		t.Fatalf("expected 1 slot, got %d", len(slots))
	}

	// List by task
	slots = s.ListSlots(task.ID, 0, 0)
	if len(slots) != 1 {
		t.Fatalf("expected 1 slot for task, got %d", len(slots))
	}

	// Update
	updated, err := s.UpdateSlot(slot.ID, map[string]any{"note": "Updated note"})
	if err != nil {
		t.Fatalf("UpdateSlot: %v", err)
	}
	if updated.Note != "Updated note" {
		t.Fatalf("expected 'Updated note', got %q", updated.Note)
	}

	// Remove
	if !s.RemoveSlot(slot.ID) {
		t.Fatal("expected RemoveSlot to return true")
	}
	slots = s.ListSlots("", 0, 0)
	if len(slots) != 0 {
		t.Fatalf("expected 0 slots, got %d", len(slots))
	}
}

func TestSlotCascadeOnTaskDelete(t *testing.T) {
	s := testService(t)

	task, _ := s.AddTask(Task{Title: "Task with slots"})
	s.AddSlot(Slot{TaskID: task.ID, StartAtMS: 1000, EndAtMS: 2000})
	s.AddSlot(Slot{TaskID: task.ID, StartAtMS: 3000, EndAtMS: 4000})

	s.RemoveTask(task.ID)

	slots := s.ListSlots(task.ID, 0, 0)
	if len(slots) != 0 {
		t.Fatalf("expected slots to be cascade deleted, got %d", len(slots))
	}
}

func TestSlotTimeRangeQuery(t *testing.T) {
	s := testService(t)

	task, _ := s.AddTask(Task{Title: "Task"})
	s.AddSlot(Slot{TaskID: task.ID, StartAtMS: 1000, EndAtMS: 2000})
	s.AddSlot(Slot{TaskID: task.ID, StartAtMS: 3000, EndAtMS: 4000})
	s.AddSlot(Slot{TaskID: task.ID, StartAtMS: 5000, EndAtMS: 6000})

	// Should get slots that overlap with range [2500, 4500]
	slots := s.ListSlots("", 2500, 4500)
	if len(slots) != 1 {
		t.Fatalf("expected 1 slot in range, got %d", len(slots))
	}
	if slots[0].StartAtMS != 3000 {
		t.Fatalf("expected slot starting at 3000, got %d", slots[0].StartAtMS)
	}
}
