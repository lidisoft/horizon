-- name: CreateLink :one
INSERT INTO issue_links (workspace_id, id, source_id, target_id, type, lag_days, created_by, created_at)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
RETURNING *;

-- name: LinksOf :many
-- Links from and to an issue.
SELECT * FROM issue_links WHERE source_id = $1 OR target_id = $1 ORDER BY created_at;

-- name: LockLink :one
SELECT * FROM issue_links WHERE workspace_id = $1 AND id = $2 FOR UPDATE;

-- name: DeleteLink :exec
DELETE FROM issue_links WHERE id = $1;
