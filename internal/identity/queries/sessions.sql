-- name: CreateSession :exec
INSERT INTO sessions (id, user_id, token_hash, created_at, last_seen_at, expires_at, user_agent, ip)
VALUES ($1, $2, $3, $4, $4, $5, $6, $7);

-- name: SessionByTokenHash :one
-- A live session of an active user.
SELECT sqlc.embed(sessions), sqlc.embed(users)
FROM sessions
JOIN users ON users.id = sessions.user_id
WHERE sessions.token_hash = $1
  AND sessions.expires_at > sqlc.arg(now)
  AND users.deactivated_at IS NULL;

-- name: TouchSession :exec
UPDATE sessions SET last_seen_at = $2, expires_at = $3 WHERE id = $1;

-- name: ListUserSessions :many
SELECT * FROM sessions
WHERE user_id = $1 AND expires_at > sqlc.arg(now)
ORDER BY last_seen_at DESC;

-- name: DeleteSession :execrows
DELETE FROM sessions WHERE id = $1 AND user_id = $2;

-- name: DeleteUserSessions :execrows
-- Every session of a user except keep (pass uuid.Nil to delete all).
DELETE FROM sessions WHERE user_id = $1 AND id <> sqlc.arg(keep);

-- name: DeleteExpiredSessions :execrows
DELETE FROM sessions WHERE expires_at <= $1;
