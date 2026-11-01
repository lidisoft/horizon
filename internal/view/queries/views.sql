-- name: CreateView :one
INSERT INTO views (workspace_id, id, project_id, owner_id, name, layout, query, display, created_at, updated_at)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $9)
RETURNING *;

-- name: View :one
SELECT * FROM views WHERE workspace_id = $1 AND id = $2;

-- name: LockView :one
SELECT * FROM views WHERE workspace_id = $1 AND id = $2 FOR UPDATE;

-- name: Views :many
-- Shared views of the workspace and the given projects, and the user's own.
SELECT * FROM views
WHERE workspace_id = $1 AND (project_id IS NULL OR project_id = ANY(sqlc.arg(project_ids)::uuid[]))
  AND (owner_id IS NULL OR owner_id = sqlc.arg(owner_id))
ORDER BY owner_id NULLS FIRST, lower(name);

-- name: SaveView :one
UPDATE views SET name = $3, layout = $4, query = $5, display = $6, owner_id = $7, updated_at = $8, version = version + 1
WHERE workspace_id = $1 AND id = $2
RETURNING *;

-- name: DeleteView :exec
DELETE FROM views WHERE workspace_id = $1 AND id = $2;
