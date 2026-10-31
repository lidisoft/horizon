-- name: LockWorkspaceKeys :exec
-- Serialises key changes in a workspace: keys and aliases share one
-- namespace that no single unique index covers.
SELECT id FROM workspaces WHERE id = $1 FOR UPDATE;

-- name: KeyTaken :one
-- Whether key is a current key or an alias of a project other than other_than.
SELECT (EXISTS (
    SELECT 1 FROM projects p
    WHERE p.workspace_id = sqlc.arg(workspace_id) AND p.key = sqlc.arg(key) AND p.id <> sqlc.arg(other_than)
) OR EXISTS (
    SELECT 1 FROM project_key_aliases a
    WHERE a.workspace_id = sqlc.arg(workspace_id) AND a.old_key = sqlc.arg(key) AND a.project_id <> sqlc.arg(other_than)
))::boolean AS taken;

-- name: CreateProject :one
INSERT INTO projects (workspace_id, id, key, name, description, colour, lead_id, visibility, created_at, updated_at)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $9)
RETURNING *;

-- name: ProjectByKey :one
-- A project by its current key or a former one.
SELECT p.* FROM projects p
WHERE p.workspace_id = sqlc.arg(workspace_id) AND (p.key = sqlc.arg(key) OR p.id = (
    SELECT a.project_id FROM project_key_aliases a WHERE a.workspace_id = sqlc.arg(workspace_id) AND a.old_key = sqlc.arg(key)));

-- name: LockProject :one
SELECT * FROM projects WHERE workspace_id = $1 AND id = $2 FOR UPDATE;

-- name: UpdateProject :one
UPDATE projects
SET key = $3, name = $4, description = $5, colour = $6, lead_id = $7, visibility = $8,
    updated_at = $9, version = version + 1
WHERE workspace_id = $1 AND id = $2
RETURNING *;

-- name: SetProjectArchived :one
UPDATE projects SET archived_at = $3, updated_at = $4, version = version + 1
WHERE workspace_id = $1 AND id = $2
RETURNING *;

-- name: ListProjects :many
SELECT * FROM projects WHERE workspace_id = $1 ORDER BY archived_at NULLS FIRST, lower(name);

-- name: AddKeyAlias :exec
INSERT INTO project_key_aliases (workspace_id, project_id, old_key, created_at)
VALUES ($1, $2, $3, $4)
ON CONFLICT (workspace_id, old_key) DO NOTHING;

-- name: DropKeyAlias :exec
-- A project taking one of its former keys back.
DELETE FROM project_key_aliases WHERE workspace_id = $1 AND project_id = $2 AND old_key = $3;

-- name: ProjectAliases :many
SELECT project_id, old_key FROM project_key_aliases WHERE workspace_id = $1 ORDER BY created_at;

-- name: ProjectGrantProjects :many
-- Projects where the user holds a grant, directly or through a team: the
-- private ones they may see.
SELECT DISTINCT g.project_id::uuid AS project_id
FROM role_grants g
WHERE g.workspace_id = sqlc.arg(workspace_id) AND g.project_id IS NOT NULL
  AND (g.user_id = sqlc.arg(user_id)::uuid
       OR g.team_id IN (SELECT tm.team_id FROM team_members tm
                        WHERE tm.workspace_id = sqlc.arg(workspace_id) AND tm.user_id = sqlc.arg(user_id)::uuid));

-- name: IsMember :one
SELECT EXISTS (SELECT 1 FROM workspace_members WHERE workspace_id = $1 AND user_id = $2);
