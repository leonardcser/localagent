CREATE TABLE tasks (
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
);

CREATE INDEX idx_tasks_status ON tasks(status);
CREATE INDEX idx_tasks_parent ON tasks(parent_id);

CREATE TABLE blocks (
    id            TEXT PRIMARY KEY,
    task_id       TEXT NOT NULL REFERENCES tasks(id) ON DELETE CASCADE,
    start_at_ms   INTEGER NOT NULL,
    end_at_ms     INTEGER NOT NULL,
    note          TEXT NOT NULL DEFAULT '',
    created_at_ms INTEGER NOT NULL
);

CREATE INDEX idx_blocks_task ON blocks(task_id);
CREATE INDEX idx_blocks_range ON blocks(start_at_ms, end_at_ms);
