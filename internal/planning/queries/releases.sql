-- name: CreateRelease :one
INSERT INTO releases (workspace_id, id, project_id, name, description, start_date, release_date, created_at, updated_at)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $8)
RETURNING *;

-- name: Release :one
SELECT * FROM releases WHERE workspace_id = $1 AND project_id = $2 AND id = $3;

-- name: LockRelease :one
SELECT * FROM releases WHERE workspace_id = $1 AND project_id = $2 AND id = $3 FOR UPDATE;

-- name: Releases :many
-- A project's releases: unreleased ones by release date (undated last),
-- then released ones, latest first.
SELECT * FROM releases
WHERE workspace_id = $1 AND project_id = $2 AND (sqlc.arg(archived)::boolean OR state <> 'archived')
ORDER BY CASE state WHEN 'unreleased' THEN 0 WHEN 'released' THEN 1 ELSE 2 END,
         CASE WHEN state = 'unreleased' THEN release_date END NULLS LAST,
         released_at DESC NULLS LAST, created_at;

-- name: ReleaseNameTaken :one
SELECT EXISTS (
    SELECT 1 FROM releases WHERE project_id = $1 AND lower(name) = lower(sqlc.arg(name)) AND id <> sqlc.arg(other_than)
)::boolean;

-- name: SaveRelease :one
UPDATE releases
SET name = $3, description = $4, state = $5, start_date = $6, release_date = $7, released_at = $8, archived_at = $9,
    updated_at = $10, version = version + 1
WHERE workspace_id = $1 AND id = $2
RETURNING *;

-- name: DeleteRelease :exec
DELETE FROM releases WHERE workspace_id = $1 AND id = $2;

-- name: ReleaseIssueIDs :many
SELECT id FROM issues WHERE workspace_id = $1 AND release_id = $2;

-- name: ReleaseStats :many
-- Per release of a project: issues by estimate and status outcome.
SELECT i.release_id::uuid AS release_id, i.estimate, sc.outcome, count(*) AS issues
FROM issues i
JOIN statuses s ON s.id = i.status_id
JOIN status_categories sc ON sc.id = s.category_id
WHERE i.workspace_id = $1 AND i.project_id = $2 AND i.release_id IS NOT NULL
GROUP BY i.release_id, i.estimate, sc.outcome;
