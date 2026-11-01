-- name: CreateIteration :one
INSERT INTO iterations (workspace_id, id, project_id, name, goal, state, start_date, end_date, created_at, updated_at)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $9)
RETURNING *;

-- name: Iteration :one
SELECT * FROM iterations WHERE workspace_id = $1 AND project_id = $2 AND id = $3;

-- name: LockIteration :one
SELECT * FROM iterations WHERE workspace_id = $1 AND project_id = $2 AND id = $3 FOR UPDATE;

-- name: LockIterationByID :one
SELECT * FROM iterations WHERE workspace_id = $1 AND id = $2 FOR UPDATE;

-- name: Iterations :many
-- A project's iterations: active first, then upcoming by start date, then
-- the finished ones, latest first.
SELECT * FROM iterations
WHERE workspace_id = $1 AND project_id = $2 AND (sqlc.arg(archived)::boolean OR state <> 'archived')
ORDER BY CASE state WHEN 'active' THEN 0 WHEN 'scheduled' THEN 1 WHEN 'planned' THEN 2 ELSE 3 END,
         CASE WHEN state IN ('completed', 'archived') THEN NULL ELSE start_date END NULLS LAST,
         completed_at DESC NULLS LAST, created_at;

-- name: SaveIteration :one
-- Writes every mutable column of an iteration and bumps its version.
UPDATE iterations
SET name = $3, goal = $4, state = $5, start_date = $6, end_date = $7, started_at = $8, completed_at = $9,
    archived_at = $10, completion = $11, updated_at = $12, version = version + 1
WHERE workspace_id = $1 AND id = $2
RETURNING *;

-- name: BumpIteration :one
-- A change of scope: the iteration's version moves with its events.
UPDATE iterations SET updated_at = $3, version = version + 1 WHERE workspace_id = $1 AND id = $2 RETURNING *;

-- name: DeleteIteration :exec
DELETE FROM iterations WHERE workspace_id = $1 AND id = $2;

-- name: LockProjectIterations :exec
-- Serialises starting iterations in a project (the active limit).
SELECT id FROM projects WHERE workspace_id = $1 AND id = $2 FOR UPDATE;

-- name: CountActive :one
SELECT count(*) FROM iterations WHERE workspace_id = $1 AND project_id = $2 AND state = 'active';

-- name: LastEnd :one
-- The end of the project's latest dated iteration, for the next one's dates.
SELECT max(end_date)::date FROM iterations WHERE workspace_id = $1 AND project_id = $2;

-- name: CountIterations :one
SELECT count(*) FROM iterations WHERE workspace_id = $1 AND project_id = $2;

-- name: IterationIssueIDs :many
-- The issues in an iteration, in rank order.
SELECT id FROM issues WHERE workspace_id = $1 AND iteration_id = $2 ORDER BY rank;

-- name: IterationStats :many
-- Per iteration of a project: issues by estimate and status outcome.
SELECT i.iteration_id::uuid AS iteration_id, i.estimate, sc.outcome, count(*) AS issues
FROM issues i
JOIN statuses s ON s.id = i.status_id
JOIN status_categories sc ON sc.id = s.category_id
WHERE i.workspace_id = $1 AND i.project_id = $2 AND i.iteration_id IS NOT NULL
GROUP BY i.iteration_id, i.estimate, sc.outcome;
