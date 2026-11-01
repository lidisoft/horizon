-- name: CreateMilestone :one
INSERT INTO milestones (workspace_id, id, project_id, name, description, date, created_at, updated_at)
VALUES ($1, $2, $3, $4, $5, $6, $7, $7)
RETURNING *;

-- name: Milestone :one
SELECT * FROM milestones WHERE workspace_id = $1 AND id = $2;

-- name: LockMilestone :one
SELECT * FROM milestones WHERE workspace_id = $1 AND id = $2 FOR UPDATE;

-- name: Milestones :many
-- The workspace's milestones by date: workspace-level ones and those of
-- the given projects.
SELECT * FROM milestones
WHERE workspace_id = $1 AND (project_id IS NULL OR project_id = ANY(sqlc.arg(project_ids)::uuid[]))
ORDER BY date, created_at;

-- name: SaveMilestone :one
UPDATE milestones SET name = $3, description = $4, date = $5, updated_at = $6, version = version + 1
WHERE workspace_id = $1 AND id = $2
RETURNING *;

-- name: BumpMilestone :one
UPDATE milestones SET updated_at = $3, version = version + 1 WHERE workspace_id = $1 AND id = $2 RETURNING *;

-- name: DeleteMilestone :exec
DELETE FROM milestones WHERE workspace_id = $1 AND id = $2;

-- name: LinkMilestoneIssue :execrows
INSERT INTO milestone_issues (workspace_id, milestone_id, issue_id) VALUES ($1, $2, $3) ON CONFLICT DO NOTHING;

-- name: UnlinkMilestoneIssue :execrows
DELETE FROM milestone_issues WHERE milestone_id = $1 AND issue_id = $2;

-- name: MilestoneStats :many
-- Per milestone: linked issues of the given projects by project, estimate
-- and status outcome.
SELECT mi.milestone_id, i.project_id, i.estimate, sc.outcome, count(*) AS issues
FROM milestone_issues mi
JOIN issues i ON i.id = mi.issue_id
JOIN statuses s ON s.id = i.status_id
JOIN status_categories sc ON sc.id = s.category_id
WHERE mi.workspace_id = $1 AND mi.milestone_id = ANY(sqlc.arg(milestone_ids)::uuid[])
  AND i.project_id = ANY(sqlc.arg(project_ids)::uuid[])
GROUP BY mi.milestone_id, i.project_id, i.estimate, sc.outcome;
