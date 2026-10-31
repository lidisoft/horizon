-- name: CreateInvitation :one
INSERT INTO invitations (workspace_id, id, email, token_hash, invited_by, team_ids, created_at, sent_at, expires_at)
VALUES ($1, $2, $3, $4, $5, $6, $7, $7, $8)
RETURNING *;

-- name: OpenInvitationByEmail :one
SELECT * FROM invitations
WHERE workspace_id = $1 AND email = $2 AND accepted_at IS NULL AND revoked_at IS NULL
FOR UPDATE;

-- name: LockInvitation :one
SELECT * FROM invitations WHERE workspace_id = $1 AND id = $2 FOR UPDATE;

-- name: RenewInvitation :one
-- A resend: new token, new expiry, teams replaced.
UPDATE invitations
SET token_hash = $3, invited_by = $4, team_ids = $5, sent_at = $6, expires_at = $7, version = version + 1
WHERE workspace_id = $1 AND id = $2
RETURNING *;

-- name: RevokeInvitation :one
UPDATE invitations SET revoked_at = $3, version = version + 1
WHERE workspace_id = $1 AND id = $2
RETURNING *;

-- name: AcceptInvitation :one
UPDATE invitations SET accepted_at = $3, accepted_by = $4, version = version + 1
WHERE workspace_id = $1 AND id = $2
RETURNING *;

-- name: InvitationByTokenHash :one
-- With the workspace and inviter, for the acceptance page.
SELECT sqlc.embed(i), sqlc.embed(w), coalesce(u.name, '')::text AS inviter_name
FROM invitations i
JOIN workspaces w ON w.id = i.workspace_id
LEFT JOIN users u ON u.id = i.invited_by
WHERE i.token_hash = $1;

-- name: ListOpenInvitations :many
SELECT i.*, coalesce(u.name, '')::text AS inviter_name
FROM invitations i
LEFT JOIN users u ON u.id = i.invited_by
WHERE i.workspace_id = $1 AND i.accepted_at IS NULL AND i.revoked_at IS NULL
ORDER BY i.sent_at DESC;
