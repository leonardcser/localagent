package db

import (
	"database/sql"
	"encoding/json"
	"os"
)

type jsonTaskStore struct {
	Version int        `json:"version"`
	Tasks   []jsonTask `json:"tasks"`
}

type jsonTask struct {
	ID          string   `json:"id"`
	Title       string   `json:"title"`
	Description string   `json:"description"`
	Status      string   `json:"status"`
	Priority    string   `json:"priority"`
	Due         string   `json:"due"`
	Recurrence  string   `json:"recurrence"`
	Tags        []string `json:"tags"`
	ParentID    string   `json:"parentId"`
	Order       float64  `json:"order"`
	CreatedAtMS int64    `json:"createdAtMs"`
	UpdatedAtMS int64    `json:"updatedAtMs"`
	DoneAtMS    *int64   `json:"doneAtMs"`
}

// MigrateFromJSON reads the old JSON task file, inserts rows into SQLite,
// and renames the JSON file to .bak. Safe to call if the file doesn't exist.
func MigrateFromJSON(database *sql.DB, jsonPath string) error {
	data, err := os.ReadFile(jsonPath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}

	var store jsonTaskStore
	if err := json.Unmarshal(data, &store); err != nil {
		return err
	}

	// Check if there are already tasks — avoid double migration
	var count int
	database.QueryRow("SELECT COUNT(*) FROM tasks").Scan(&count)
	if count > 0 {
		// Already migrated, just rename the old file
		return os.Rename(jsonPath, jsonPath+".bak")
	}

	tx, err := database.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	for _, t := range store.Tasks {
		tagsJSON, _ := json.Marshal(t.Tags)
		if tagsJSON == nil {
			tagsJSON = []byte("[]")
		}
		rrule := convertLegacyRecurrence(t.Recurrence)
		_, err := tx.Exec(`INSERT OR IGNORE INTO tasks
			(id, title, description, status, priority, due, recurrence, tags, parent_id, sort_order, created_at_ms, updated_at_ms, done_at_ms)
			VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
			t.ID, t.Title, t.Description, t.Status, t.Priority, t.Due, rrule, string(tagsJSON),
			t.ParentID, t.Order, t.CreatedAtMS, t.UpdatedAtMS, t.DoneAtMS)
		if err != nil {
			return err
		}
	}

	if err := tx.Commit(); err != nil {
		return err
	}

	return os.Rename(jsonPath, jsonPath+".bak")
}

func convertLegacyRecurrence(r string) string {
	switch r {
	case "daily":
		return "FREQ=DAILY"
	case "weekly":
		return "FREQ=WEEKLY"
	case "monthly":
		return "FREQ=MONTHLY"
	default:
		return r
	}
}
