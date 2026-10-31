-- name: CreateWorkflow :one
INSERT INTO workflows (workspace_id, id, project_id, name, copied_from, published_at, created_at, updated_at)
VALUES ($1, $2, $3, $4, $5, $6, $6, $6)
RETURNING *;

-- name: Workflows :many
SELECT * FROM workflows
WHERE workspace_id = sqlc.arg(workspace_id)
  AND (project_id IS NULL OR project_id = sqlc.narg(project_id))
ORDER BY project_id NULLS FIRST, lower(name);

-- name: WorkflowsWithDrafts :many
SELECT * FROM workflows WHERE workspace_id = $1 AND draft IS NOT NULL;

-- name: Workflow :one
SELECT * FROM workflows WHERE workspace_id = $1 AND id = $2;

-- name: LockWorkflow :one
SELECT * FROM workflows WHERE workspace_id = $1 AND id = $2 FOR UPDATE;

-- name: RenameWorkflow :one
UPDATE workflows SET name = $3, updated_at = $4, version = version + 1
WHERE workspace_id = $1 AND id = $2
RETURNING *;

-- name: SaveDraft :one
UPDATE workflows
SET draft = $3, draft_updated_by = $4, draft_updated_at = $5, draft_version = draft_version + 1
WHERE workspace_id = $1 AND id = $2
RETURNING *;

-- name: MarkPublished :one
UPDATE workflows
SET draft = NULL, draft_updated_by = NULL, draft_updated_at = NULL, draft_version = draft_version + 1,
    published_at = $3, updated_at = $3, version = version + 1
WHERE workspace_id = $1 AND id = $2
RETURNING *;

-- name: DeleteWorkflow :exec
DELETE FROM workflows WHERE workspace_id = $1 AND id = $2;

-- name: Statuses :many
SELECT * FROM statuses WHERE workflow_id = ANY(sqlc.arg(workflow_ids)::uuid[]) ORDER BY workflow_id, position;

-- name: Transitions :many
SELECT * FROM transitions WHERE workflow_id = ANY(sqlc.arg(workflow_ids)::uuid[]) ORDER BY workflow_id, name;

-- name: UpsertStatus :exec
INSERT INTO statuses (workflow_id, id, name, category_id, position, is_initial)
VALUES ($1, $2, $3, $4, $5, $6)
ON CONFLICT (id) DO UPDATE
SET name = excluded.name, category_id = excluded.category_id, position = excluded.position, is_initial = excluded.is_initial;

-- name: DeleteStatusesExcept :exec
DELETE FROM statuses WHERE workflow_id = $1 AND NOT (id = ANY(sqlc.arg(keep)::uuid[]));

-- name: DeleteTransitions :exec
DELETE FROM transitions WHERE workflow_id = $1;

-- name: CreateTransition :exec
INSERT INTO transitions (workflow_id, id, from_status_id, to_status_id, name, rules)
VALUES ($1, $2, $3, $4, $5, $6);

-- name: WorkflowInUse :one
-- Issue types (of any scope) bound to the workflow.
SELECT count(*) FROM issue_types WHERE workflow_id = $1;
