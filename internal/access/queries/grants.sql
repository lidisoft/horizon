-- name: UpsertGrant :one
-- Sets the role of a subject in a scope, replacing the previous one.
INSERT INTO role_grants (workspace_id, id, role_id, user_id, team_id, project_id, granted_by, created_at)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
ON CONFLICT (workspace_id, coalesce(user_id, team_id), coalesce(project_id, '00000000-0000-0000-0000-000000000000'))
DO UPDATE SET role_id = excluded.role_id, granted_by = excluded.granted_by, created_at = excluded.created_at,
              version = role_grants.version + 1
RETURNING *;

-- name: LockGrant :one
SELECT * FROM role_grants WHERE workspace_id = $1 AND id = $2 FOR UPDATE;

-- name: DeleteGrant :exec
DELETE FROM role_grants WHERE workspace_id = $1 AND id = $2;

-- name: ListGrants :many
SELECT g.id, g.role_id, g.user_id, g.team_id, g.project_id, g.created_at,
       r.name::text AS role_name,
       coalesce(u.name, t.name::text, '')::text AS subject_name,
       coalesce(u.email, '')::text AS subject_email
FROM role_grants g
JOIN roles r ON r.id = g.role_id
LEFT JOIN users u ON u.id = g.user_id
LEFT JOIN teams t ON t.id = g.team_id
WHERE g.workspace_id = $1
ORDER BY g.project_id NULLS FIRST, r.position, lower(coalesce(u.name, t.name::text));

-- name: CountAdminWorkspaceGrants :one
-- Workspace-scope grants of the Admin role, for the lock-out guard.
SELECT count(*)
FROM role_grants g
JOIN roles r ON r.id = g.role_id
WHERE g.workspace_id = $1 AND r.builtin = 'admin' AND g.project_id IS NULL;

-- name: Permissions :many
-- The permissions a user holds in a workspace, optionally on one project:
-- the union of their grants and their teams', at workspace scope and on the
-- project. Suspended members hold none.
SELECT DISTINCT unnest(r.permissions)::text AS permission
FROM role_grants g
JOIN roles r ON r.id = g.role_id
JOIN workspace_members m ON m.workspace_id = g.workspace_id AND m.user_id = sqlc.arg(user_id) AND m.status = 'active'
WHERE g.workspace_id = sqlc.arg(workspace_id)
  AND (g.project_id IS NULL OR g.project_id = sqlc.narg(project_id))
  AND (g.user_id = sqlc.arg(user_id)
       OR g.team_id IN (SELECT tm.team_id FROM team_members tm
                        WHERE tm.workspace_id = sqlc.arg(workspace_id) AND tm.user_id = sqlc.arg(user_id)));

-- name: HasWorkspaceGrant :one
-- Whether the user holds any role on the whole workspace (directly or
-- through a team): guests, granted on projects only, don't.
SELECT EXISTS (
    SELECT 1 FROM role_grants g
    WHERE g.workspace_id = sqlc.arg(workspace_id) AND g.project_id IS NULL
      AND (g.user_id = sqlc.arg(user_id)::uuid
           OR g.team_id IN (SELECT tm.team_id FROM team_members tm
                            WHERE tm.workspace_id = sqlc.arg(workspace_id) AND tm.user_id = sqlc.arg(user_id)::uuid))
);

-- name: RoleUsage :many
-- People and teams holding each role, for the role columns.
SELECT role_id, count(user_id) AS people, count(team_id) AS teams
FROM role_grants WHERE workspace_id = $1
GROUP BY role_id;

-- name: RolesOf :many
-- The roles a user holds in a workspace and optionally on one project,
-- directly or through a team. Suspended members hold none.
SELECT DISTINCT g.role_id
FROM role_grants g
JOIN workspace_members m ON m.workspace_id = g.workspace_id AND m.user_id = sqlc.arg(user_id)::uuid AND m.status = 'active'
WHERE g.workspace_id = sqlc.arg(workspace_id)
  AND (g.project_id IS NULL OR g.project_id = sqlc.narg(project_id))
  AND (g.user_id = sqlc.arg(user_id)::uuid
       OR g.team_id IN (SELECT tm.team_id FROM team_members tm
                        WHERE tm.workspace_id = sqlc.arg(workspace_id) AND tm.user_id = sqlc.arg(user_id)::uuid));
