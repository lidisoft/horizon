-- The text expressions must match the indexes of migration 00013.

-- name: SearchIssues :many
-- Issues of the given projects matching the words (tsquery) or, fuzzily,
-- the title; best first. headline marks the matched words of the
-- description with the given markers.
WITH q AS (SELECT to_tsquery('simple', sqlc.arg(tsquery)::text) AS tsq)
SELECT i.id, i.project_id, p.key AS project_key, i.number, i.title, i.status_id, s.name AS status_name,
       sc.outcome AS status_outcome, i.type_id, i.priority, i.assignee_id, i.archived_at,
       (to_tsvector('simple', i.title) @@ q.tsq)::boolean AS in_title,
       (ts_rank(setweight(to_tsvector('simple', i.title), 'A') || setweight(to_tsvector('simple', i.description), 'B'), q.tsq)
         + similarity(i.title, sqlc.arg(raw)::text))::float8 AS score,
       CASE WHEN to_tsvector('simple', i.description) @@ q.tsq
            THEN ts_headline('simple', i.description, q.tsq, sqlc.arg(headline_options)::text) ELSE '' END::text AS headline
FROM issues i
JOIN projects p ON p.id = i.project_id
JOIN statuses s ON s.id = i.status_id
JOIN status_categories sc ON sc.id = s.category_id
CROSS JOIN q
WHERE i.workspace_id = sqlc.arg(workspace_id)
  AND i.project_id = ANY(sqlc.arg(project_ids)::uuid[])
  AND (sqlc.arg(archived)::boolean OR i.archived_at IS NULL)
  AND ((setweight(to_tsvector('simple', i.title), 'A') || setweight(to_tsvector('simple', i.description), 'B')) @@ q.tsq
       OR i.title % sqlc.arg(raw)::text)
ORDER BY score DESC, i.updated_at DESC
LIMIT sqlc.arg(max_rows);

-- name: SearchComments :many
-- Comments matching the words, on issues of the given projects; newest
-- first, at most one per issue.
WITH q AS (SELECT to_tsquery('simple', sqlc.arg(tsquery)::text) AS tsq)
SELECT DISTINCT ON (c.issue_id) c.issue_id, c.id AS comment_id, i.project_id, p.key AS project_key, i.number, i.title,
       i.status_id, s.name AS status_name, sc.outcome AS status_outcome, i.type_id, i.priority, i.assignee_id, i.archived_at,
       ts_rank(to_tsvector('simple', c.body), q.tsq)::float8 AS score,
       ts_headline('simple', c.body, q.tsq, sqlc.arg(headline_options)::text)::text AS headline
FROM comments c
JOIN issues i ON i.id = c.issue_id
JOIN projects p ON p.id = i.project_id
JOIN statuses s ON s.id = i.status_id
JOIN status_categories sc ON sc.id = s.category_id
CROSS JOIN q
WHERE c.workspace_id = sqlc.arg(workspace_id)
  AND i.project_id = ANY(sqlc.arg(project_ids)::uuid[])
  AND (sqlc.arg(archived)::boolean OR i.archived_at IS NULL)
  AND to_tsvector('simple', c.body) @@ q.tsq
ORDER BY c.issue_id, c.id DESC
LIMIT sqlc.arg(max_rows);
