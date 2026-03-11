package db

import (
	"database/sql"
	"fmt"
)

type migration struct {
	version int
	up      func(tx *sql.Tx) error
}

var migrations = []migration{
	{1, migrateCreateTasks},
	{2, migrateCreateBlocks},
	{3, migrateCreateLinks},
	{4, migrateBackfillTaskOrder},
	{5, migrateAddReminders},
}

func Migrate(db *sql.DB) error {
	if _, err := db.Exec(`CREATE TABLE IF NOT EXISTS schema_version (version INTEGER NOT NULL)`); err != nil {
		return err
	}

	var current int
	db.QueryRow("SELECT COALESCE(MAX(version), 0) FROM schema_version").Scan(&current)

	for _, m := range migrations {
		if m.version <= current {
			continue
		}
		tx, err := db.Begin()
		if err != nil {
			return fmt.Errorf("begin migration %d: %w", m.version, err)
		}
		if err := m.up(tx); err != nil {
			tx.Rollback()
			return fmt.Errorf("migration %d: %w", m.version, err)
		}
		if _, err := tx.Exec("INSERT INTO schema_version (version) VALUES (?)", m.version); err != nil {
			tx.Rollback()
			return fmt.Errorf("record migration %d: %w", m.version, err)
		}
		if err := tx.Commit(); err != nil {
			return fmt.Errorf("commit migration %d: %w", m.version, err)
		}
	}
	return nil
}

func migrateCreateTasks(tx *sql.Tx) error {
	_, err := tx.Exec(`CREATE TABLE tasks (
		id            TEXT PRIMARY KEY,
		title         TEXT NOT NULL,
		description   TEXT NOT NULL DEFAULT '',
		status        TEXT NOT NULL DEFAULT 'todo',
		priority      TEXT NOT NULL DEFAULT '',
		due           TEXT NOT NULL DEFAULT '',
		recurrence    TEXT NOT NULL DEFAULT '',
		tags          TEXT NOT NULL DEFAULT '[]',
		parent_id     TEXT NOT NULL DEFAULT '',
		sort_order    REAL NOT NULL DEFAULT 0,
		created_at_ms INTEGER NOT NULL,
		updated_at_ms INTEGER NOT NULL,
		done_at_ms    INTEGER
	)`)
	if err != nil {
		return err
	}
	if _, err = tx.Exec(`CREATE INDEX idx_tasks_status ON tasks(status)`); err != nil {
		return err
	}
	_, err = tx.Exec(`CREATE INDEX idx_tasks_parent ON tasks(parent_id)`)
	return err
}

func migrateCreateLinks(tx *sql.Tx) error {
	_, err := tx.Exec(`CREATE TABLE links (
		id            TEXT PRIMARY KEY,
		url           TEXT NOT NULL,
		title         TEXT NOT NULL DEFAULT '',
		description   TEXT NOT NULL DEFAULT '',
		tags          TEXT NOT NULL DEFAULT '[]',
		created_at_ms INTEGER NOT NULL,
		updated_at_ms INTEGER NOT NULL
	)`)
	if err != nil {
		return err
	}
	_, err = tx.Exec(`CREATE INDEX idx_links_created ON links(created_at_ms)`)
	return err
}

func migrateBackfillTaskOrder(tx *sql.Tx) error {
	_, err := tx.Exec(`
		UPDATE tasks SET sort_order = (
			SELECT COUNT(*) FROM tasks t2 WHERE t2.created_at_ms <= tasks.created_at_ms
		) WHERE sort_order = 0`)
	return err
}

func migrateAddReminders(tx *sql.Tx) error {
	if _, err := tx.Exec(`ALTER TABLE tasks ADD COLUMN reminders TEXT NOT NULL DEFAULT '[]'`); err != nil {
		return err
	}
	_, err := tx.Exec(`CREATE TABLE sent_reminders (
		task_id    TEXT NOT NULL,
		offset     TEXT NOT NULL,
		fire_at_ms INTEGER NOT NULL,
		sent_at_ms INTEGER NOT NULL,
		PRIMARY KEY (task_id, offset, fire_at_ms)
	)`)
	return err
}

func migrateCreateBlocks(tx *sql.Tx) error {
	_, err := tx.Exec(`CREATE TABLE blocks (
		id            TEXT PRIMARY KEY,
		task_id       TEXT NOT NULL REFERENCES tasks(id) ON DELETE CASCADE,
		start_at_ms   INTEGER NOT NULL,
		end_at_ms     INTEGER NOT NULL,
		note          TEXT NOT NULL DEFAULT '',
		created_at_ms INTEGER NOT NULL
	)`)
	if err != nil {
		return err
	}
	if _, err = tx.Exec(`CREATE INDEX idx_blocks_task ON blocks(task_id)`); err != nil {
		return err
	}
	_, err = tx.Exec(`CREATE INDEX idx_blocks_range ON blocks(start_at_ms, end_at_ms)`)
	return err
}
