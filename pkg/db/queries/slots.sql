-- name: ListSlots :many
SELECT * FROM slots ORDER BY start_at_ms;

-- name: ListSlotsByTask :many
SELECT * FROM slots WHERE task_id = ? ORDER BY start_at_ms;

-- name: ListSlotsByRange :many
SELECT * FROM slots WHERE end_at_ms > ? AND start_at_ms < ? ORDER BY start_at_ms;

-- name: ListSlotsByTaskAndRange :many
SELECT * FROM slots WHERE task_id = ? AND end_at_ms > ? AND start_at_ms < ? ORDER BY start_at_ms;

-- name: GetSlot :one
SELECT * FROM slots WHERE id = ?;

-- name: InsertSlot :exec
INSERT INTO slots (id, task_id, start_at_ms, end_at_ms, note, created_at_ms)
VALUES (?, ?, ?, ?, ?, ?);

-- name: DeleteSlot :execresult
DELETE FROM slots WHERE id = ?;
