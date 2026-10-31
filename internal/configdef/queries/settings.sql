-- name: WorkspaceSettings :one
SELECT settings FROM workspaces WHERE id = $1;

-- name: ProjectSettings :one
SELECT settings FROM projects WHERE id = $1;

-- name: SetWorkspaceSetting :exec
UPDATE workspaces SET settings = jsonb_set(settings, ARRAY[sqlc.arg(key)::text], sqlc.arg(value)::jsonb), updated_at = now()
WHERE id = $1;

-- name: SetProjectSetting :exec
UPDATE projects SET settings = jsonb_set(settings, ARRAY[sqlc.arg(key)::text], sqlc.arg(value)::jsonb), updated_at = now()
WHERE id = $1;

-- name: ClearProjectSetting :exec
UPDATE projects SET settings = settings - sqlc.arg(key)::text, updated_at = now() WHERE id = $1;
