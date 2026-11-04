-- name: CreatePasswordReset :exec
INSERT INTO password_resets (id, user_id, token_hash, created_at, expires_at)
VALUES ($1, $2, $3, $4, $5);

-- name: ClaimPasswordReset :one
-- Marks a valid reset used and returns its user; no row when the token is
-- unknown, used or expired.
UPDATE password_resets
SET used_at = sqlc.arg(now)::timestamptz
WHERE token_hash = $1 AND used_at IS NULL AND expires_at > sqlc.arg(now)
RETURNING user_id;

-- name: DeleteUserPasswordResets :exec
DELETE FROM password_resets WHERE user_id = $1;

-- name: DeleteStalePasswordResets :execrows
DELETE FROM password_resets WHERE expires_at <= $1;

-- name: ThrottleFailures :one
-- Failures recorded for key in the window that started after since.
SELECT failures, window_start FROM auth_throttle
WHERE key = $1 AND window_start > sqlc.arg(since);

-- name: RecordThrottleFailure :exec
-- Counts a failure, starting a new window when the previous one has ended.
INSERT INTO auth_throttle (key, failures, window_start)
VALUES ($1, 1, sqlc.arg(now))
ON CONFLICT (key) DO UPDATE
SET failures = CASE WHEN auth_throttle.window_start > sqlc.arg(since) THEN auth_throttle.failures + 1 ELSE 1 END,
    window_start = CASE WHEN auth_throttle.window_start > sqlc.arg(since) THEN auth_throttle.window_start ELSE sqlc.arg(now) END;

-- name: ClearThrottle :exec
DELETE FROM auth_throttle WHERE key = $1;

-- name: DeleteStaleThrottles :execrows
DELETE FROM auth_throttle WHERE window_start <= $1;

-- name: CreateMagicLink :exec
INSERT INTO magic_links (id, user_id, token_hash, nonce_hash, created_at, expires_at)
VALUES (sqlc.arg(id), sqlc.arg(user_id), sqlc.arg(token_hash), sqlc.arg(nonce_hash), sqlc.arg(created_at), sqlc.arg(expires_at));

-- name: ClaimMagicLink :one
-- Marks a valid link used and returns its user; no row when the token is
-- unknown, used, expired, or from another browser (nonce).
UPDATE magic_links SET used_at = sqlc.arg(now)::timestamptz
WHERE token_hash = sqlc.arg(token_hash) AND nonce_hash = sqlc.arg(nonce_hash) AND used_at IS NULL AND expires_at > sqlc.arg(now)
RETURNING user_id;

-- name: DeleteStaleMagicLinks :execrows
DELETE FROM magic_links WHERE expires_at <= $1;
