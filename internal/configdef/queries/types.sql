-- name: CreateIssueType :one
INSERT INTO issue_types (workspace_id, id, project_id, name, icon, colour, workflow_id, parent_rule,
                         allowed_parent_type_ids, position, copied_from, created_at, updated_at)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $12)
RETURNING *;

-- name: IssueTypes :many
SELECT * FROM issue_types
WHERE workspace_id = sqlc.arg(workspace_id)
  AND (project_id IS NULL OR project_id = sqlc.narg(project_id))
ORDER BY project_id NULLS FIRST, position, lower(name);

-- name: LockIssueType :one
SELECT * FROM issue_types WHERE workspace_id = $1 AND id = $2 FOR UPDATE;

-- name: UpdateIssueType :one
UPDATE issue_types
SET name = $3, icon = $4, colour = $5, workflow_id = $6, parent_rule = $7, allowed_parent_type_ids = $8,
    updated_at = $9, version = version + 1
WHERE workspace_id = $1 AND id = $2
RETURNING *;

-- name: DeleteIssueType :exec
DELETE FROM issue_types WHERE workspace_id = $1 AND id = $2;

-- name: NextIssueTypePosition :one
SELECT coalesce(max(position), -1)::int + 1 FROM issue_types
WHERE workspace_id = $1 AND project_id IS NOT DISTINCT FROM sqlc.narg(project_id);

-- name: SetIssueTypePosition :exec
UPDATE issue_types SET position = $3 WHERE workspace_id = $1 AND id = $2;
