-- name: CreateRole :one
INSERT INTO roles (workspace_id, id, name, description, builtin, permissions, position, created_at, updated_at)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $8)
RETURNING *;

-- name: ListRoles :many
SELECT * FROM roles WHERE workspace_id = $1 ORDER BY position, lower(name);

-- name: LockRole :one
SELECT * FROM roles WHERE workspace_id = $1 AND id = $2 FOR UPDATE;

-- name: RoleByBuiltin :one
SELECT * FROM roles WHERE workspace_id = $1 AND builtin = $2;

-- name: RoleInWorkspace :one
SELECT * FROM roles WHERE workspace_id = $1 AND id = $2;

-- name: UpdateRole :one
UPDATE roles SET name = $3, description = $4, permissions = $5, updated_at = $6, version = version + 1
WHERE workspace_id = $1 AND id = $2
RETURNING *;

-- name: DeleteRole :exec
DELETE FROM roles WHERE workspace_id = $1 AND id = $2;

-- name: NextRolePosition :one
SELECT coalesce(max(position), -1)::int + 1 FROM roles WHERE workspace_id = $1;

-- name: CountRoleGrants :one
SELECT count(*) FROM role_grants WHERE role_id = $1;
