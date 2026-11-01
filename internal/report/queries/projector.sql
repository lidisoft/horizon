-- name: ClaimUnreported :many
-- A batch of events the projections haven't seen, oldest first.
SELECT id, workspace_id, aggregate_type, aggregate_id, type, occurred_at, payload
FROM events
WHERE reported_at IS NULL
ORDER BY id
LIMIT sqlc.arg(batch_size)
FOR UPDATE SKIP LOCKED;

-- name: MarkReported :exec
UPDATE events SET reported_at = now() WHERE id = ANY(sqlc.arg(ids)::uuid[]);

-- name: StatusFacts :one
-- A status's category and outcome now, for events that don't carry them.
SELECT s.category_id, sc.outcome
FROM statuses s JOIN status_categories sc ON sc.id = s.category_id
WHERE s.id = $1;

-- name: IssueProject :one
SELECT project_id FROM issues WHERE id = $1;

-- name: CloseInterval :exec
UPDATE report_status_intervals SET left_at = sqlc.arg(at) WHERE issue_id = sqlc.arg(issue_id) AND left_at IS NULL;

-- name: OpenInterval :exec
INSERT INTO report_status_intervals (workspace_id, project_id, issue_id, status_id, category_id, outcome, entered_at)
VALUES ($1, $2, $3, $4, $5, $6, $7);

-- name: IssueIntervals :many
SELECT outcome, entered_at, left_at FROM report_status_intervals WHERE issue_id = $1 ORDER BY entered_at, id;

-- name: Milestones :one
SELECT * FROM report_issue_milestones WHERE issue_id = $1;

-- name: SaveMilestones :exec
INSERT INTO report_issue_milestones (workspace_id, issue_id, project_id, created_at, first_started_at, completed_at, abandoned_at)
VALUES ($1, $2, $3, $4, $5, $6, $7)
ON CONFLICT (issue_id) DO UPDATE SET project_id = EXCLUDED.project_id, first_started_at = EXCLUDED.first_started_at,
    completed_at = EXCLUDED.completed_at, abandoned_at = EXCLUDED.abandoned_at;

-- name: CloseEstimate :exec
UPDATE report_estimates SET valid_to = sqlc.arg(at) WHERE issue_id = sqlc.arg(issue_id) AND valid_to IS NULL;

-- name: OpenEstimate :exec
INSERT INTO report_estimates (workspace_id, issue_id, weight, valid_from) VALUES ($1, $2, $3, $4);

-- name: CloseScope :exec
UPDATE report_iteration_scope SET removed_at = sqlc.arg(at)
WHERE issue_id = sqlc.arg(issue_id) AND iteration_id = sqlc.arg(iteration_id) AND removed_at IS NULL;

-- name: OpenScope :exec
INSERT INTO report_iteration_scope (workspace_id, iteration_id, issue_id, added_at) VALUES ($1, $2, $3, $4);

-- name: SetCategoryOutcome :many
-- A category's outcome changed: its stays take the new one; returns the
-- issues to recompute.
UPDATE report_status_intervals SET outcome = sqlc.arg(outcome) WHERE category_id = sqlc.arg(category_id) AND outcome <> sqlc.arg(outcome)
RETURNING issue_id;

-- name: ForgetIssueIntervals :exec
DELETE FROM report_status_intervals WHERE issue_id = $1;

-- name: ForgetIssueMilestones :exec
DELETE FROM report_issue_milestones WHERE issue_id = $1;

-- name: ForgetIssueScope :exec
DELETE FROM report_iteration_scope WHERE issue_id = $1;

-- name: ForgetIssueEstimates :exec
DELETE FROM report_estimates WHERE issue_id = $1;
