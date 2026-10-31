-- name: CreateUser :one
INSERT INTO users (id, email, name, password_hash, is_instance_admin, created_at, updated_at)
VALUES ($1, $2, $3, $4, $5, $6, $6)
RETURNING *;

-- name: GetUser :one
SELECT * FROM users WHERE id = $1;

-- name: GetUserByEmail :one
SELECT * FROM users WHERE email = $1;

-- name: LockUser :one
SELECT * FROM users WHERE id = $1 FOR UPDATE;

-- name: UpdateUserProfile :one
UPDATE users
SET name = $2, locale = $3, timezone = $4, updated_at = $5, version = version + 1
WHERE id = $1
RETURNING *;

-- name: SetUserPassword :one
UPDATE users
SET password_hash = $2, updated_at = $3, version = version + 1
WHERE id = $1
RETURNING *;

-- name: SetUserDeactivated :one
UPDATE users
SET deactivated_at = $2, updated_at = $3, version = version + 1
WHERE id = $1
RETURNING *;

-- name: SetUserInstanceAdmin :one
UPDATE users
SET is_instance_admin = $2, updated_at = $3, version = version + 1
WHERE id = $1
RETURNING *;
