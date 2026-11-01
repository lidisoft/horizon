-- name: NextIssueNumber :one
-- Takes the project's next number; the row lock serialises creation per project.
UPDATE projects SET issue_seq = issue_seq + 1 WHERE workspace_id = $1 AND id = $2 RETURNING issue_seq;

-- name: LastRank :one
SELECT coalesce(max(rank), '')::text FROM issues WHERE workspace_id = $1 AND project_id = $2;

-- name: CreateIssue :one
INSERT INTO issues (workspace_id, id, project_id, number, type_id, title, description, status_id, priority, rank,
                    estimate, assignee_id, reporter_id, parent_id, start_date, due_date, fields, started_at, resolved_at,
                    created_at, updated_at)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15, $16, $17, $18, $19, $20, $20)
RETURNING *;

-- name: IssueByNumber :one
SELECT * FROM issues WHERE workspace_id = $1 AND project_id = $2 AND number = $3;

-- name: IssueByID :one
SELECT * FROM issues WHERE workspace_id = $1 AND id = $2;

-- name: LockIssue :one
SELECT * FROM issues WHERE workspace_id = $1 AND id = $2 FOR UPDATE;

-- name: SaveIssue :one
-- Writes every mutable column of an issue and bumps its version.
UPDATE issues
SET type_id = $3, title = $4, description = $5, status_id = $6, priority = $7, rank = $8, estimate = $9,
    assignee_id = $10, parent_id = $11, start_date = $12, due_date = $13, fields = $14, started_at = $15,
    resolved_at = $16, archived_at = $17, updated_at = $18, iteration_id = $19, release_id = $20, version = version + 1
WHERE workspace_id = $1 AND id = $2
RETURNING *;

-- name: ProjectIssues :many
-- A page of a project's issues in rank order, after cursor (a rank).
SELECT * FROM issues
WHERE workspace_id = sqlc.arg(workspace_id) AND project_id = sqlc.arg(project_id)
  AND (sqlc.arg(archived)::boolean OR archived_at IS NULL)
  AND rank > sqlc.arg(after)::text
ORDER BY rank
LIMIT sqlc.arg(page_size);

-- name: Children :many
SELECT * FROM issues WHERE workspace_id = $1 AND parent_id = $2 ORDER BY rank;

-- name: Ancestors :many
-- The parent chain of an issue, nearest first, for cycle checks.
WITH RECURSIVE chain AS (
    SELECT i.id, i.parent_id, 1 AS depth FROM issues i WHERE i.id = sqlc.arg(start_id)
    UNION ALL
    SELECT p.id, p.parent_id, c.depth + 1 FROM issues p JOIN chain c ON p.id = c.parent_id WHERE c.depth < 100
)
SELECT id FROM chain;

-- name: NeighbourRanks :one
-- The ranks just before and after rank in a project, for inserting.
SELECT coalesce((SELECT max(i.rank) FROM issues i WHERE i.workspace_id = $1 AND i.project_id = $2 AND i.rank < sqlc.arg(rank)::text), '')::text AS before,
       coalesce((SELECT min(i.rank) FROM issues i WHERE i.workspace_id = $1 AND i.project_id = $2 AND i.rank > sqlc.arg(rank)::text), '')::text AS after;

-- name: IssueLabels :many
SELECT issue_id, label_id FROM issue_labels WHERE issue_id = ANY(sqlc.arg(ids)::uuid[]);

-- name: ClearIssueLabels :exec
DELETE FROM issue_labels WHERE issue_id = $1;

-- name: AddIssueLabel :exec
INSERT INTO issue_labels (workspace_id, issue_id, label_id) VALUES ($1, $2, $3) ON CONFLICT DO NOTHING;

-- name: CountByLabel :many
SELECT label_id, count(*)::bigint AS n FROM issue_labels WHERE workspace_id = $1 GROUP BY label_id;

-- name: IssuesWithLabel :many
SELECT issue_id FROM issue_labels WHERE label_id = $1;

-- name: CountByStatus :many
SELECT status_id, count(*)::bigint AS n FROM issues WHERE status_id = ANY(sqlc.arg(ids)::uuid[]) GROUP BY status_id;

-- name: CountTypeByStatus :many
SELECT status_id, count(*)::bigint AS n FROM issues WHERE type_id = $1 GROUP BY status_id;

-- name: CountByType :one
SELECT count(*) FROM issues WHERE type_id = $1;

-- name: IssuesInStatus :many
SELECT id, workspace_id FROM issues WHERE status_id = $1 AND (sqlc.narg(type_id)::uuid IS NULL OR type_id = sqlc.narg(type_id));

-- name: ScrubField :exec
UPDATE issues SET fields = fields - sqlc.arg(field)::text WHERE workspace_id = $1 AND fields ? sqlc.arg(field)::text;

-- name: IssueByAlias :one
-- An issue by a key it left behind when it moved.
SELECT i.* FROM issue_key_aliases a JOIN issues i ON i.id = a.issue_id
WHERE a.workspace_id = $1 AND a.project_id = $2 AND a.number = $3;

-- name: AddIssueAlias :exec
INSERT INTO issue_key_aliases (workspace_id, project_id, number, issue_id, created_at) VALUES ($1, $2, $3, $4, $5);

-- name: Descendants :many
-- Every descendant of an issue, deepest first (for cascading deletes).
WITH RECURSIVE tree AS (
    SELECT c.id, 1 AS depth FROM issues c WHERE c.parent_id = sqlc.arg(root_id)
    UNION ALL
    SELECT c.id, t.depth + 1 FROM issues c JOIN tree t ON c.parent_id = t.id WHERE t.depth < 100
)
SELECT i.* FROM tree JOIN issues i ON i.id = tree.id ORDER BY tree.depth DESC, i.rank;

-- name: ReparentChildren :many
-- Moves an issue's children to its parent (or to the top level).
UPDATE issues SET parent_id = sqlc.narg(new_parent), updated_at = sqlc.arg(updated_at), version = version + 1
WHERE parent_id = sqlc.arg(old_parent)
RETURNING *;

-- name: DeleteIssue :exec
DELETE FROM issues WHERE workspace_id = $1 AND id = $2;

-- name: ProjectIssueIDs :many
SELECT id FROM issues WHERE workspace_id = $1 AND project_id = $2;

-- name: MoveIssue :one
-- Writes an issue into another project under its new number.
UPDATE issues
SET project_id = $3, number = $4, type_id = $5, status_id = $6, rank = $7, estimate = $8, fields = $9,
    parent_id = $10, iteration_id = NULL, release_id = NULL, started_at = $11, resolved_at = $12,
    updated_at = $13, version = version + 1
WHERE workspace_id = $1 AND id = $2
RETURNING *;

-- name: SetIssueIteration :one
-- Moves an issue into an iteration, or out of any (planning commands).
UPDATE issues SET iteration_id = $3, updated_at = $4, version = version + 1
WHERE workspace_id = $1 AND id = $2
RETURNING *;
