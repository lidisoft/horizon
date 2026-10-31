-- name: CreateField :one
INSERT INTO custom_fields (workspace_id, id, project_id, name, type, description, options, position, created_at, updated_at)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $9)
RETURNING *;

-- name: Fields :many
SELECT * FROM custom_fields
WHERE workspace_id = sqlc.arg(workspace_id)
  AND (project_id IS NULL OR project_id = sqlc.narg(project_id))
ORDER BY project_id NULLS FIRST, position, lower(name);

-- name: LockField :one
SELECT * FROM custom_fields WHERE workspace_id = $1 AND id = $2 FOR UPDATE;

-- name: UpdateField :one
UPDATE custom_fields SET name = $3, description = $4, options = $5, updated_at = $6, version = version + 1
WHERE workspace_id = $1 AND id = $2
RETURNING *;

-- name: DeleteField :exec
DELETE FROM custom_fields WHERE workspace_id = $1 AND id = $2;

-- name: NextFieldPosition :one
SELECT coalesce(max(position), -1)::int + 1 FROM custom_fields
WHERE workspace_id = $1 AND project_id IS NOT DISTINCT FROM sqlc.narg(project_id);

-- name: TypeFields :many
-- Field attachments of the issue types of a workspace.
SELECT * FROM issue_type_fields WHERE workspace_id = $1 ORDER BY issue_type_id, position;

-- name: ClearTypeFields :exec
DELETE FROM issue_type_fields WHERE issue_type_id = $1;

-- name: AttachField :exec
INSERT INTO issue_type_fields (workspace_id, issue_type_id, field_id, required, position)
VALUES ($1, $2, $3, $4, $5);
