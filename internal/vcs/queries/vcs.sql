-- name: Connections :many
SELECT * FROM vcs_connections WHERE workspace_id = sqlc.arg(workspace_id) ORDER BY created_at, id;

-- name: Connection :one
SELECT * FROM vcs_connections WHERE workspace_id = sqlc.arg(workspace_id) AND id = sqlc.arg(id);

-- name: ConnectionByID :one
-- For inbound deliveries, which name the connection only.
SELECT * FROM vcs_connections WHERE id = sqlc.arg(id);

-- name: LockConnection :one
SELECT * FROM vcs_connections WHERE workspace_id = sqlc.arg(workspace_id) AND id = sqlc.arg(id) FOR UPDATE;

-- name: CreateConnection :one
INSERT INTO vcs_connections (workspace_id, id, provider, name, base_url, webhook_secret, token, created_by, created_at, updated_at)
VALUES (sqlc.arg(workspace_id), sqlc.arg(id), sqlc.arg(provider), sqlc.arg(name), sqlc.arg(base_url), sqlc.arg(webhook_secret),
        sqlc.narg(token), sqlc.narg(created_by), sqlc.arg(created_at), sqlc.arg(created_at))
RETURNING *;

-- name: UpdateConnection :one
UPDATE vcs_connections SET name = sqlc.arg(name), base_url = sqlc.arg(base_url), webhook_secret = sqlc.arg(webhook_secret),
    token = sqlc.narg(token), updated_at = sqlc.arg(updated_at), version = version + 1
WHERE id = sqlc.arg(id)
RETURNING *;

-- name: DeleteConnection :exec
DELETE FROM vcs_connections WHERE id = sqlc.arg(id);

-- name: TouchConnection :exec
UPDATE vcs_connections SET last_event_at = sqlc.arg(at)::timestamptz WHERE id = sqlc.arg(id);

-- name: UpsertLink :one
-- Links an issue to a ref, or updates the link; inserted tells a new link.
INSERT INTO vcs_links (workspace_id, id, issue_id, connection_id, kind, repository, external_id, url, title, state, author, created_at, updated_at)
VALUES (sqlc.arg(workspace_id), sqlc.arg(id), sqlc.arg(issue_id), sqlc.arg(connection_id), sqlc.arg(kind), sqlc.arg(repository),
        sqlc.arg(external_id), sqlc.arg(url), sqlc.arg(title), sqlc.arg(state), sqlc.arg(author), sqlc.arg(at), sqlc.arg(at))
ON CONFLICT (issue_id, connection_id, kind, repository, external_id) DO UPDATE SET
    url = EXCLUDED.url, title = EXCLUDED.title, state = EXCLUDED.state,
    author = CASE WHEN EXCLUDED.author = '' THEN vcs_links.author ELSE EXCLUDED.author END,
    updated_at = EXCLUDED.updated_at
RETURNING (xmax = 0)::boolean AS inserted, id, state;

-- name: LinkState :one
SELECT state FROM vcs_links
WHERE issue_id = sqlc.arg(issue_id) AND connection_id = sqlc.arg(connection_id) AND kind = sqlc.arg(kind)
  AND repository = sqlc.arg(repository) AND external_id = sqlc.arg(external_id);

-- name: RefLinks :many
-- The issues linked to a ref, with their keys.
SELECT l.issue_id, l.state, p.key AS project_key, i.number
FROM vcs_links l JOIN issues i ON i.id = l.issue_id JOIN projects p ON p.id = i.project_id
WHERE l.connection_id = sqlc.arg(connection_id) AND l.kind = sqlc.arg(kind) AND l.repository = sqlc.arg(repository)
  AND l.external_id = sqlc.arg(external_id)
ORDER BY p.key, i.number;

-- name: SetRefState :exec
UPDATE vcs_links SET state = sqlc.arg(state), updated_at = sqlc.arg(at)
WHERE connection_id = sqlc.arg(connection_id) AND kind = sqlc.arg(kind) AND repository = sqlc.arg(repository) AND external_id = sqlc.arg(external_id);

-- name: IssueLinks :many
-- An issue's links, pull requests first, newest first.
SELECT l.*, c.provider
FROM vcs_links l JOIN vcs_connections c ON c.id = l.connection_id
WHERE l.issue_id = sqlc.arg(issue_id)
ORDER BY CASE l.kind WHEN 'pull_request' THEN 0 WHEN 'branch' THEN 1 ELSE 2 END, l.created_at DESC, l.id DESC
LIMIT 100;

-- name: PullRequestsOf :many
-- The pull requests linked to an issue, to update their comments.
SELECT DISTINCT connection_id, repository, external_id FROM vcs_links
WHERE issue_id = sqlc.arg(issue_id) AND kind = 'pull_request';

-- name: MarkComment :exec
-- The pull request's comment needs writing.
INSERT INTO vcs_comments (workspace_id, connection_id, repository, number, project_ref, next_attempt_at, updated_at)
VALUES (sqlc.arg(workspace_id), sqlc.arg(connection_id), sqlc.arg(repository), sqlc.arg(number), sqlc.arg(project_ref), sqlc.arg(at), sqlc.arg(at))
ON CONFLICT (connection_id, repository, number) DO UPDATE SET
    dirty = true, attempts = 0, next_attempt_at = EXCLUDED.next_attempt_at, updated_at = EXCLUDED.updated_at,
    project_ref = CASE WHEN EXCLUDED.project_ref = '' THEN vcs_comments.project_ref ELSE EXCLUDED.project_ref END;

-- name: LeaseComments :many
-- Takes dirty comments due, for a while (like webhook deliveries).
WITH due AS (
    SELECT c.connection_id, c.repository, c.number FROM vcs_comments c
    WHERE c.dirty AND c.next_attempt_at <= sqlc.arg(now)::timestamptz
    ORDER BY c.next_attempt_at
    LIMIT sqlc.arg(max_rows)
    FOR UPDATE SKIP LOCKED
)
UPDATE vcs_comments c SET next_attempt_at = sqlc.arg(lease_until)::timestamptz
FROM due
WHERE c.connection_id = due.connection_id AND c.repository = due.repository AND c.number = due.number
RETURNING c.*;

-- name: CommentWritten :exec
-- Clean unless marked dirty again since the lease (updated_at moved).
UPDATE vcs_comments SET forge_id = sqlc.arg(forge_id), body = sqlc.arg(body), attempts = 0, last_error = '',
    dirty = (updated_at > sqlc.arg(leased_at)::timestamptz)
WHERE connection_id = sqlc.arg(connection_id) AND repository = sqlc.arg(repository) AND number = sqlc.arg(number);

-- name: CommentFailed :exec
UPDATE vcs_comments SET attempts = attempts + 1, last_error = sqlc.arg(error), next_attempt_at = sqlc.arg(next_attempt_at),
    dirty = sqlc.arg(retry)
WHERE connection_id = sqlc.arg(connection_id) AND repository = sqlc.arg(repository) AND number = sqlc.arg(number);

-- name: CommentIssues :many
-- The issues a pull request names, with their status, for its comment.
SELECT p.key AS project_key, i.number, i.title, s.name AS status_name, c.outcome AS status_outcome, w.slug AS workspace_slug
FROM vcs_links l
JOIN issues i ON i.id = l.issue_id
JOIN projects p ON p.id = i.project_id
JOIN workspaces w ON w.id = i.workspace_id
JOIN statuses s ON s.id = i.status_id
JOIN status_categories c ON c.id = s.category_id
WHERE l.connection_id = sqlc.arg(connection_id) AND l.kind = 'pull_request' AND l.repository = sqlc.arg(repository)
  AND l.external_id = sqlc.arg(number)
ORDER BY p.key, i.number;

-- name: SealedConnectionSecrets :many
SELECT id, webhook_secret, token FROM vcs_connections ORDER BY id;

-- name: SetConnectionSecrets :exec
UPDATE vcs_connections SET webhook_secret = sqlc.arg(webhook_secret), token = sqlc.narg(token) WHERE id = sqlc.arg(id);

-- name: ResolveKey :one
-- The issue a key names in the workspace: by its project's current or a
-- former key, and its own former keys after a move.
WITH p AS (
    SELECT pr.id FROM projects pr
    WHERE pr.workspace_id = sqlc.arg(workspace_id) AND (pr.key = sqlc.arg(project_key)::text OR pr.id = (
        SELECT a.project_id FROM project_key_aliases a WHERE a.workspace_id = sqlc.arg(workspace_id) AND a.old_key = sqlc.arg(project_key)::text))
)
SELECT i.id, i.version, i.project_id FROM issues i
WHERE i.workspace_id = sqlc.arg(workspace_id) AND i.id = coalesce(
    (SELECT x.id FROM issues x, p WHERE x.workspace_id = sqlc.arg(workspace_id) AND x.project_id = p.id AND x.number = sqlc.arg(number)),
    (SELECT a.issue_id FROM issue_key_aliases a, p WHERE a.workspace_id = sqlc.arg(workspace_id) AND a.project_id = p.id AND a.number = sqlc.arg(number)));

-- name: IssueRef :one
SELECT id, version, project_id FROM issues WHERE id = sqlc.arg(id);
