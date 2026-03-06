package todo

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"slices"
	"strings"
	"time"

	"localagent/pkg/db/dbq"
	"localagent/pkg/utils"

	"github.com/teambition/rrule-go"
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
	Order       float64  `json:"order"`
	CreatedAtMS int64    `json:"createdAtMs"`
	UpdatedAtMS int64    `json:"updatedAtMs"`
	DoneAtMS    *int64   `json:"doneAtMs,omitempty"`
}

type TaskEvent struct {
	Action string `json:"action"`
	Task   Task   `json:"task"`
}

type TodoService struct {
	db            *sql.DB
	q             *dbq.Queries
	listener      func(TaskEvent)
	blockListener func(BlockEvent)
	linkListener  func(LinkEvent)
}

func NewTodoService(database *sql.DB) *TodoService {
	return &TodoService{
		db: database,
		q:  dbq.New(database),
	}
}

func (s *TodoService) SetListener(fn func(TaskEvent))        { s.listener = fn }
func (s *TodoService) SetBlockListener(fn func(BlockEvent))  { s.blockListener = fn }
func (s *TodoService) SetLinkListener(fn func(LinkEvent))    { s.linkListener = fn }
func (s *TodoService) notify(evt TaskEvent)                  { if s.listener != nil { s.listener(evt) } }
func (s *TodoService) notifyBlock(evt BlockEvent)            { if s.blockListener != nil { s.blockListener(evt) } }
func (s *TodoService) notifyLink(evt LinkEvent)              { if s.linkListener != nil { s.linkListener(evt) } }

// Load is a no-op for SQLite (kept for backward compat).
func (s *TodoService) Load() error { return nil }

func (s *TodoService) ListTasks(status string, tag string) []Task {
	ctx := context.Background()
	var rows []dbq.Task
	var err error

	if status != "" {
		rows, err = s.q.ListTasksByStatus(ctx, status)
	} else {
		rows, err = s.q.ListTasks(ctx)
	}
	if err != nil {
		return nil
	}

	var tasks []Task
	for _, r := range rows {
		t := dbTaskToTask(r)
		if tag != "" && !slices.Contains(t.Tags, tag) {
			continue
		}
		tasks = append(tasks, t)
	}
	return tasks
}

func (s *TodoService) AddTask(task Task) (*Task, error) {
	ctx := context.Background()
	now := time.Now().UnixMilli()

	if task.ID == "" {
		task.ID = utils.RandHex(8)
	}
	if task.Status == "" {
		task.Status = "todo"
	}
	task.CreatedAtMS = now
	task.UpdatedAtMS = now

	if task.Order == 0 {
		maxOrder, err := s.q.MaxTaskOrder(ctx)
		if err == nil {
			task.Order = maxOrder + 1
		} else {
			task.Order = 1
		}
	}

	var doneAt sql.NullInt64
	if task.DoneAtMS != nil {
		doneAt = sql.NullInt64{Int64: *task.DoneAtMS, Valid: true}
	}

	err := s.q.InsertTask(ctx, dbq.InsertTaskParams{
		ID:          task.ID,
		Title:       task.Title,
		Description: task.Description,
		Status:      task.Status,
		Priority:    task.Priority,
		Due:         task.Due,
		Recurrence:  task.Recurrence,
		Tags:        marshalTags(task.Tags),
		ParentID:    task.ParentID,
		SortOrder:   task.Order,
		CreatedAtMs: task.CreatedAtMS,
		UpdatedAtMs: task.UpdatedAtMS,
		DoneAtMs:    doneAt,
	})
	if err != nil {
		return nil, err
	}

	s.notify(TaskEvent{Action: "created", Task: task})
	return &task, nil
}

func (s *TodoService) UpdateTask(taskID string, patch map[string]any) (*Task, error) {
	var sets []string
	var args []any

	for key, val := range patch {
		col := patchKeyToColumn(key)
		if col == "" {
			continue
		}
		if key == "tags" {
			tags := toStringSlice(val)
			args = append(args, marshalTags(tags))
		} else {
			args = append(args, val)
		}
		sets = append(sets, col+" = ?")
	}

	if len(sets) == 0 {
		return nil, fmt.Errorf("no fields to update")
	}

	now := time.Now().UnixMilli()
	sets = append(sets, "updated_at_ms = ?")
	args = append(args, now)
	args = append(args, taskID)

	query := fmt.Sprintf("UPDATE tasks SET %s WHERE id = ?", strings.Join(sets, ", "))
	res, err := s.db.Exec(query, args...)
	if err != nil {
		return nil, err
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return nil, fmt.Errorf("task not found: %s", taskID)
	}

	task := s.getTask(taskID)
	if task != nil {
		s.notify(TaskEvent{Action: "updated", Task: *task})
	}
	return task, nil
}

func (s *TodoService) CompleteTask(taskID string) (*Task, error) {
	ctx := context.Background()

	task := s.getTask(taskID)
	if task == nil {
		return nil, fmt.Errorf("task not found: %s", taskID)
	}

	now := time.Now().UnixMilli()
	err := s.q.CompleteTask(ctx, dbq.CompleteTaskParams{
		DoneAtMs:    sql.NullInt64{Int64: now, Valid: true},
		UpdatedAtMs: now,
		ID:          taskID,
	})
	if err != nil {
		return nil, err
	}
	task.Status = "done"
	task.DoneAtMS = &now
	task.UpdatedAtMS = now

	s.notify(TaskEvent{Action: "updated", Task: *task})

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
			s.AddTask(newTask)
		}
	}

	return task, nil
}

func (s *TodoService) RemoveTask(taskID string) bool {
	ctx := context.Background()
	s.q.DeleteTaskChildren(ctx, taskID)
	res, err := s.q.DeleteTask(ctx, taskID)
	if err != nil {
		return false
	}
	n, _ := res.RowsAffected()
	if n > 0 {
		s.notify(TaskEvent{Action: "deleted", Task: Task{ID: taskID}})
		return true
	}
	return false
}

// --- Block methods ---

func (s *TodoService) ListBlocks(taskID string, startAfter, endBefore int64) []Block {
	ctx := context.Background()
	var rows []dbq.Block
	var err error

	hasTask := taskID != ""
	hasRange := startAfter > 0 && endBefore > 0

	switch {
	case hasTask && hasRange:
		rows, err = s.q.ListBlocksByTaskAndRange(ctx, dbq.ListBlocksByTaskAndRangeParams{
			TaskID:    taskID,
			EndAtMs:   startAfter,
			StartAtMs: endBefore,
		})
	case hasTask:
		rows, err = s.q.ListBlocksByTask(ctx, taskID)
	case hasRange:
		rows, err = s.q.ListBlocksByRange(ctx, dbq.ListBlocksByRangeParams{
			EndAtMs:   startAfter,
			StartAtMs: endBefore,
		})
	default:
		rows, err = s.q.ListBlocks(ctx)
	}

	if err != nil {
		return nil
	}

	blocks := make([]Block, len(rows))
	for i, r := range rows {
		blocks[i] = Block{
			ID:          r.ID,
			TaskID:      r.TaskID,
			StartAtMS:   r.StartAtMs,
			EndAtMS:     r.EndAtMs,
			Note:        r.Note,
			CreatedAtMS: r.CreatedAtMs,
		}
	}
	return blocks
}

func (s *TodoService) AddBlock(block Block) (*Block, error) {
	ctx := context.Background()
	if block.ID == "" {
		block.ID = utils.RandHex(8)
	}
	block.CreatedAtMS = time.Now().UnixMilli()

	err := s.q.InsertBlock(ctx, dbq.InsertBlockParams{
		ID:          block.ID,
		TaskID:      block.TaskID,
		StartAtMs:   block.StartAtMS,
		EndAtMs:     block.EndAtMS,
		Note:        block.Note,
		CreatedAtMs: block.CreatedAtMS,
	})
	if err != nil {
		return nil, err
	}

	s.notifyBlock(BlockEvent{Action: "created", Block: block})
	return &block, nil
}

func (s *TodoService) UpdateBlock(blockID string, patch map[string]any) (*Block, error) {
	var sets []string
	var args []any

	if v, ok := patch["taskId"].(string); ok {
		sets = append(sets, "task_id = ?")
		args = append(args, v)
	}
	if v, ok := patch["startAtMs"].(float64); ok {
		sets = append(sets, "start_at_ms = ?")
		args = append(args, int64(v))
	}
	if v, ok := patch["endAtMs"].(float64); ok {
		sets = append(sets, "end_at_ms = ?")
		args = append(args, int64(v))
	}
	if v, ok := patch["note"].(string); ok {
		sets = append(sets, "note = ?")
		args = append(args, v)
	}

	if len(sets) == 0 {
		return nil, fmt.Errorf("no fields to update")
	}

	args = append(args, blockID)
	query := fmt.Sprintf("UPDATE blocks SET %s WHERE id = ?", strings.Join(sets, ", "))
	res, err := s.db.Exec(query, args...)
	if err != nil {
		return nil, err
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return nil, fmt.Errorf("block not found: %s", blockID)
	}

	block := s.getBlock(blockID)
	if block != nil {
		s.notifyBlock(BlockEvent{Action: "updated", Block: *block})
	}
	return block, nil
}

func (s *TodoService) RemoveBlock(blockID string) bool {
	ctx := context.Background()
	res, err := s.q.DeleteBlock(ctx, blockID)
	if err != nil {
		return false
	}
	n, _ := res.RowsAffected()
	if n > 0 {
		s.notifyBlock(BlockEvent{Action: "deleted", Block: Block{ID: blockID}})
		return true
	}
	return false
}

// --- Link methods ---

func (s *TodoService) ListLinks(tag string) []Link {
	ctx := context.Background()
	rows, err := s.q.ListLinks(ctx)
	if err != nil {
		return nil
	}
	var links []Link
	for _, r := range rows {
		l := dbLinkToLink(r)
		if tag != "" && !slices.Contains(l.Tags, tag) {
			continue
		}
		links = append(links, l)
	}
	return links
}

func (s *TodoService) AddLink(link Link) (*Link, error) {
	ctx := context.Background()
	now := time.Now().UnixMilli()
	if link.ID == "" {
		link.ID = utils.RandHex(8)
	}
	link.CreatedAtMS = now
	link.UpdatedAtMS = now

	err := s.q.InsertLink(ctx, dbq.InsertLinkParams{
		ID:          link.ID,
		Url:         link.URL,
		Title:       link.Title,
		Description: link.Description,
		Tags:        marshalTags(link.Tags),
		CreatedAtMs: link.CreatedAtMS,
		UpdatedAtMs: link.UpdatedAtMS,
	})
	if err != nil {
		return nil, err
	}

	s.notifyLink(LinkEvent{Action: "created", Link: link})
	return &link, nil
}

func (s *TodoService) UpdateLink(linkID string, patch map[string]any) (*Link, error) {
	var sets []string
	var args []any

	if v, ok := patch["url"].(string); ok {
		sets = append(sets, "url = ?")
		args = append(args, v)
	}
	if v, ok := patch["title"].(string); ok {
		sets = append(sets, "title = ?")
		args = append(args, v)
	}
	if v, ok := patch["description"].(string); ok {
		sets = append(sets, "description = ?")
		args = append(args, v)
	}
	if v, ok := patch["tags"]; ok {
		sets = append(sets, "tags = ?")
		args = append(args, marshalTags(toStringSlice(v)))
	}

	if len(sets) == 0 {
		return nil, fmt.Errorf("no fields to update")
	}

	now := time.Now().UnixMilli()
	sets = append(sets, "updated_at_ms = ?")
	args = append(args, now)
	args = append(args, linkID)

	query := fmt.Sprintf("UPDATE links SET %s WHERE id = ?", strings.Join(sets, ", "))
	res, err := s.db.Exec(query, args...)
	if err != nil {
		return nil, err
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return nil, fmt.Errorf("link not found: %s", linkID)
	}

	link := s.getLink(linkID)
	if link != nil {
		s.notifyLink(LinkEvent{Action: "updated", Link: *link})
	}
	return link, nil
}

func (s *TodoService) RemoveLink(linkID string) bool {
	ctx := context.Background()
	res, err := s.q.DeleteLink(ctx, linkID)
	if err != nil {
		return false
	}
	n, _ := res.RowsAffected()
	if n > 0 {
		s.notifyLink(LinkEvent{Action: "deleted", Link: Link{ID: linkID}})
		return true
	}
	return false
}

// --- helpers ---

func (s *TodoService) getTask(id string) *Task {
	ctx := context.Background()
	row, err := s.q.GetTask(ctx, id)
	if err != nil {
		return nil
	}
	t := dbTaskToTask(row)
	return &t
}

func (s *TodoService) getBlock(id string) *Block {
	ctx := context.Background()
	row, err := s.q.GetBlock(ctx, id)
	if err != nil {
		return nil
	}
	return &Block{
		ID:          row.ID,
		TaskID:      row.TaskID,
		StartAtMS:   row.StartAtMs,
		EndAtMS:     row.EndAtMs,
		Note:        row.Note,
		CreatedAtMS: row.CreatedAtMs,
	}
}

func dbTaskToTask(r dbq.Task) Task {
	t := Task{
		ID:          r.ID,
		Title:       r.Title,
		Description: r.Description,
		Status:      r.Status,
		Priority:    r.Priority,
		Due:         r.Due,
		Recurrence:  r.Recurrence,
		ParentID:    r.ParentID,
		Order:       r.SortOrder,
		CreatedAtMS: r.CreatedAtMs,
		UpdatedAtMS: r.UpdatedAtMs,
	}
	json.Unmarshal([]byte(r.Tags), &t.Tags)
	if r.DoneAtMs.Valid {
		t.DoneAtMS = &r.DoneAtMs.Int64
	}
	return t
}

func (s *TodoService) getLink(id string) *Link {
	ctx := context.Background()
	row, err := s.q.GetLink(ctx, id)
	if err != nil {
		return nil
	}
	l := dbLinkToLink(row)
	return &l
}

func dbLinkToLink(r dbq.Link) Link {
	l := Link{
		ID:          r.ID,
		URL:         r.Url,
		Title:       r.Title,
		Description: r.Description,
		CreatedAtMS: r.CreatedAtMs,
		UpdatedAtMS: r.UpdatedAtMs,
	}
	json.Unmarshal([]byte(r.Tags), &l.Tags)
	return l
}

func marshalTags(tags []string) string {
	if tags == nil {
		return "[]"
	}
	data, _ := json.Marshal(tags)
	return string(data)
}

func patchKeyToColumn(key string) string {
	switch key {
	case "title":
		return "title"
	case "description":
		return "description"
	case "status":
		return "status"
	case "priority":
		return "priority"
	case "due":
		return "due"
	case "recurrence":
		return "recurrence"
	case "tags":
		return "tags"
	case "parentId":
		return "parent_id"
	case "order":
		return "sort_order"
	default:
		return ""
	}
}

func computeNextDue(due string, recurrence string) string {
	if recurrence == "" || due == "" {
		return ""
	}

	dueDate, err := time.Parse("2006-01-02", due)
	if err != nil {
		return ""
	}

	opt, err := rrule.StrToROption(recurrence)
	if err != nil {
		return ""
	}
	opt.Dtstart = dueDate

	rule, err := rrule.NewRRule(*opt)
	if err != nil {
		return ""
	}

	after := dueDate.AddDate(0, 0, 1)
	next := rule.After(after, true)
	if next.IsZero() {
		return ""
	}
	return next.Format("2006-01-02")
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
