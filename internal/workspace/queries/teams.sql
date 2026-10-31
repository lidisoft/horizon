-- name: CreateTeam :one
INSERT INTO teams (workspace_id, id, name, description, created_at, updated_at)
VALUES ($1, $2, $3, $4, $5, $5)
RETURNING *;

-- name: LockTeam :one
SELECT * FROM teams WHERE workspace_id = $1 AND id = $2 FOR UPDATE;

-- name: UpdateTeam :one
UPDATE teams SET name = $3, description = $4, updated_at = $5, version = version + 1
WHERE workspace_id = $1 AND id = $2
RETURNING *;

-- name: BumpTeam :one
UPDATE teams SET updated_at = $3, version = version + 1
WHERE workspace_id = $1 AND id = $2
RETURNING *;

-- name: DeleteTeam :exec
DELETE FROM teams WHERE workspace_id = $1 AND id = $2;

-- name: ListTeams :many
SELECT * FROM teams WHERE workspace_id = $1 ORDER BY lower(name);

-- name: TeamMembers :many
SELECT tm.team_id, u.id AS user_id, u.name, u.email
FROM team_members tm
JOIN users u ON u.id = tm.user_id
WHERE tm.workspace_id = $1
ORDER BY lower(u.name);

-- name: AddTeamMember :execrows
INSERT INTO team_members (workspace_id, team_id, user_id, added_at)
VALUES ($1, $2, $3, $4)
ON CONFLICT DO NOTHING;

-- name: RemoveTeamMember :execrows
DELETE FROM team_members WHERE workspace_id = $1 AND team_id = $2 AND user_id = $3;

-- name: ExistingTeams :many
-- The ids among ids that are teams of the workspace.
SELECT id FROM teams WHERE workspace_id = $1 AND id = ANY(sqlc.arg(ids)::uuid[]);

-- name: AddTeamMemberIfTeam :execrows
-- Adds the member when the team still exists in the workspace.
INSERT INTO team_members (workspace_id, team_id, user_id, added_at)
SELECT t.workspace_id, t.id, sqlc.arg(user_id), sqlc.arg(added_at)
FROM teams t
WHERE t.workspace_id = sqlc.arg(workspace_id) AND t.id = sqlc.arg(team_id)
ON CONFLICT DO NOTHING;
