-- name: ListLinks :many
SELECT * FROM links ORDER BY created_at_ms DESC;

-- name: GetLink :one
SELECT * FROM links WHERE id = ?;

-- name: InsertLink :exec
INSERT INTO links (id, url, title, description, tags, created_at_ms, updated_at_ms)
VALUES (?, ?, ?, ?, ?, ?, ?);

-- name: UpdateLink :exec
UPDATE links SET url=?, title=?, description=?, tags=?, updated_at_ms=? WHERE id=?;

-- name: DeleteLink :execresult
DELETE FROM links WHERE id = ?;
