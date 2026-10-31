-- name: CreateCategory :one
INSERT INTO status_categories (workspace_id, id, project_id, name, colour, outcome, position, created_at, updated_at)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $8)
RETURNING *;

-- name: Categories :many
-- The categories of a scope and, for a project, the workspace's: the
-- caller filters with the project's switches.
SELECT * FROM status_categories
WHERE workspace_id = sqlc.arg(workspace_id)
  AND (project_id IS NULL OR project_id = sqlc.narg(project_id))
ORDER BY project_id NULLS FIRST, position, lower(name);

-- name: LockCategory :one
SELECT * FROM status_categories WHERE workspace_id = $1 AND id = $2 FOR UPDATE;

-- name: UpdateCategory :one
UPDATE status_categories SET name = $3, colour = $4, outcome = $5, updated_at = $6, version = version + 1
WHERE workspace_id = $1 AND id = $2
RETURNING *;

-- name: SetCategoryPosition :exec
UPDATE status_categories SET position = $3 WHERE workspace_id = $1 AND id = $2;

-- name: DeleteCategory :exec
DELETE FROM status_categories WHERE workspace_id = $1 AND id = $2;

-- name: NextCategoryPosition :one
SELECT coalesce(max(position), -1)::int + 1 FROM status_categories
WHERE workspace_id = $1 AND project_id IS NOT DISTINCT FROM sqlc.narg(project_id);

-- name: MoveStatusesToCategory :execrows
-- Published statuses of every workflow of the workspace.
UPDATE statuses SET category_id = sqlc.arg(to_id)
WHERE category_id = sqlc.arg(from_id)
  AND workflow_id IN (SELECT id FROM workflows WHERE workspace_id = sqlc.arg(workspace_id));

-- name: CategoryUsage :many
-- Published statuses per category, with their workflows, for the list.
SELECT s.category_id, s.name AS status_name, w.id AS workflow_id
FROM statuses s
JOIN workflows w ON w.id = s.workflow_id
WHERE w.workspace_id = $1;
