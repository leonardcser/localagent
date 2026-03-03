-- name: ListBlocks :many
SELECT * FROM blocks ORDER BY start_at_ms;

-- name: ListBlocksByTask :many
SELECT * FROM blocks WHERE task_id = ? ORDER BY start_at_ms;

-- name: ListBlocksByRange :many
SELECT * FROM blocks WHERE end_at_ms > ? AND start_at_ms < ? ORDER BY start_at_ms;

-- name: ListBlocksByTaskAndRange :many
SELECT * FROM blocks WHERE task_id = ? AND end_at_ms > ? AND start_at_ms < ? ORDER BY start_at_ms;

-- name: GetBlock :one
SELECT * FROM blocks WHERE id = ?;

-- name: InsertBlock :exec
INSERT INTO blocks (id, task_id, start_at_ms, end_at_ms, note, created_at_ms)
VALUES (?, ?, ?, ?, ?, ?);

-- name: DeleteBlock :execresult
DELETE FROM blocks WHERE id = ?;
