-- name: AddMember :one
INSERT INTO workspace_members (workspace_id, id, user_id, status, joined_at)
VALUES ($1, $2, $3, 'active', $4)
RETURNING *;

-- name: Membership :one
SELECT * FROM workspace_members WHERE workspace_id = $1 AND user_id = $2;

-- name: LockMembership :one
SELECT * FROM workspace_members WHERE workspace_id = $1 AND user_id = $2 FOR UPDATE;

-- name: SetMemberStatus :one
UPDATE workspace_members
SET status = $3, suspended_at = $4, version = version + 1
WHERE workspace_id = $1 AND user_id = $2
RETURNING *;

-- name: RemoveMember :exec
DELETE FROM workspace_members WHERE workspace_id = $1 AND user_id = $2;

-- name: ListMembers :many
SELECT m.id, m.user_id, m.status, m.joined_at, m.suspended_at, m.version,
       u.email, u.name, u.deactivated_at
FROM workspace_members m
JOIN users u ON u.id = m.user_id
WHERE m.workspace_id = $1
ORDER BY lower(u.name), u.email;

-- name: MembersLastSeen :many
-- Most recent session use of each member who has one.
SELECT s.user_id, max(s.last_seen_at)::timestamptz AS last_seen_at
FROM sessions s
JOIN workspace_members m ON m.user_id = s.user_id
WHERE m.workspace_id = $1
GROUP BY s.user_id;

-- name: MemberTeams :many
-- Team memberships of every member, for the member list.
SELECT tm.user_id, t.id AS team_id, t.name::text AS team_name
FROM team_members tm
JOIN teams t ON t.id = tm.team_id
WHERE tm.workspace_id = $1
ORDER BY lower(t.name);
