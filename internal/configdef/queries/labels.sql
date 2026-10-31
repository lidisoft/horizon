-- name: CreateLabel :one
INSERT INTO labels (workspace_id, id, project_id, name, colour, description, created_at, updated_at)
VALUES ($1, $2, $3, $4, $5, $6, $7, $7)
RETURNING *;

-- name: Labels :many
SELECT * FROM labels
WHERE workspace_id = sqlc.arg(workspace_id)
  AND (project_id IS NULL OR project_id = sqlc.narg(project_id))
ORDER BY project_id NULLS FIRST, lower(name);

-- name: LockLabel :one
SELECT * FROM labels WHERE workspace_id = $1 AND id = $2 FOR UPDATE;

-- name: UpdateLabel :one
UPDATE labels SET name = $3, colour = $4, description = $5, updated_at = $6, version = version + 1
WHERE workspace_id = $1 AND id = $2
RETURNING *;

-- name: MoveLabelToWorkspace :one
UPDATE labels SET project_id = NULL, updated_at = $3, version = version + 1
WHERE workspace_id = $1 AND id = $2
RETURNING *;

-- name: DeleteLabel :exec
DELETE FROM labels WHERE workspace_id = $1 AND id = $2;
