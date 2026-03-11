-- name: ListTasks :many
SELECT * FROM tasks ORDER BY (status = 'done'), CASE priority WHEN 'high' THEN 1 WHEN 'medium' THEN 2 WHEN 'low' THEN 3 ELSE 4 END, sort_order;

-- name: ListTasksByStatus :many
SELECT * FROM tasks WHERE status = ? ORDER BY (status = 'done'), CASE priority WHEN 'high' THEN 1 WHEN 'medium' THEN 2 WHEN 'low' THEN 3 ELSE 4 END, sort_order;

-- name: GetTask :one
SELECT * FROM tasks WHERE id = ?;

-- name: MaxTaskOrder :one
SELECT CAST(COALESCE(MAX(sort_order), 0) AS REAL) FROM tasks;

-- name: InsertTask :exec
INSERT INTO tasks (id, title, description, status, priority, due, recurrence, tags, parent_id, sort_order, created_at_ms, updated_at_ms, done_at_ms)
VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?);

-- name: CompleteTask :exec
UPDATE tasks SET status = 'done', done_at_ms = ?, updated_at_ms = ? WHERE id = ?;

-- name: DeleteTask :execresult
DELETE FROM tasks WHERE id = ?;

-- name: DeleteTaskChildren :exec
DELETE FROM tasks WHERE parent_id = ?;

-- name: CountTasks :one
SELECT COUNT(*) FROM tasks;
