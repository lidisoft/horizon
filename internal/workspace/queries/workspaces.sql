-- name: CreateWorkspace :one
INSERT INTO workspaces (id, slug, name, time_zone, created_at, updated_at)
VALUES ($1, $2, $3, $4, $5, $5)
RETURNING *;

-- name: WorkspaceBySlug :one
SELECT * FROM workspaces WHERE slug = $1;

-- name: LockWorkspace :one
SELECT * FROM workspaces WHERE id = $1 FOR UPDATE;

-- name: UpdateWorkspace :one
UPDATE workspaces
SET name = $2, slug = $3, time_zone = $4, updated_at = $5, version = version + 1
WHERE id = $1
RETURNING *;

-- name: UserWorkspaces :many
-- Workspaces where the user is an active member, by name.
SELECT w.*
FROM workspaces w
JOIN workspace_members m ON m.workspace_id = w.id
WHERE m.user_id = $1 AND m.status = 'active'
ORDER BY lower(w.name), w.slug;

-- name: AllWorkspaces :many
SELECT * FROM workspaces ORDER BY lower(name), slug;
