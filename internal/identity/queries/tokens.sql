-- name: CreateAPIToken :one
INSERT INTO api_tokens (id, user_id, name, token_hash, hint, scope, created_at, expires_at)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
RETURNING *;

-- name: CountUserAPITokens :one
SELECT count(*) FROM api_tokens WHERE user_id = $1;

-- name: APITokenByHash :one
-- A live token of an active user.
SELECT sqlc.embed(api_tokens), sqlc.embed(users)
FROM api_tokens
JOIN users ON users.id = api_tokens.user_id
WHERE api_tokens.token_hash = $1
  AND (api_tokens.expires_at IS NULL OR api_tokens.expires_at > sqlc.arg(now))
  AND users.deactivated_at IS NULL;

-- name: TouchAPIToken :exec
UPDATE api_tokens SET last_used_at = $2 WHERE id = $1;

-- name: ListUserAPITokens :many
SELECT * FROM api_tokens WHERE user_id = $1 ORDER BY created_at DESC;

-- name: DeleteAPIToken :one
DELETE FROM api_tokens WHERE id = $1 AND user_id = $2 RETURNING *;

-- name: DeleteUserAPITokens :execrows
DELETE FROM api_tokens WHERE user_id = $1;
