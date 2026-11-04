-- name: Subscribe :exec
-- Follows the issue; an existing subscription keeps its reason.
INSERT INTO subscriptions (workspace_id, issue_id, user_id, reason, created_at)
VALUES (sqlc.arg(workspace_id), sqlc.arg(issue_id), sqlc.arg(user_id), sqlc.arg(reason), sqlc.arg(created_at))
ON CONFLICT (issue_id, user_id) DO NOTHING;

-- name: Unsubscribe :exec
DELETE FROM subscriptions WHERE issue_id = sqlc.arg(issue_id) AND user_id = sqlc.arg(user_id);

-- name: Subscription :one
SELECT reason FROM subscriptions WHERE issue_id = sqlc.arg(issue_id) AND user_id = sqlc.arg(user_id);

-- name: Subscribers :many
SELECT user_id FROM subscriptions WHERE issue_id = sqlc.arg(issue_id) ORDER BY created_at, user_id;

-- name: Bump :execrows
-- Tells the user of the event: a new row, or the issue's row brought back
-- unread (out of the archive and of a snooze). Events older than the row's
-- latest change nothing, so redeliveries are harmless. While unread, the row
-- keeps its first event and its strongest reason.
INSERT INTO inbox_items (workspace_id, id, user_id, issue_id, reason, event_id, first_event_id, updated_at)
VALUES (sqlc.arg(workspace_id), sqlc.arg(id), sqlc.arg(user_id), sqlc.arg(issue_id), sqlc.arg(reason),
        sqlc.arg(event_id), sqlc.arg(event_id), sqlc.arg(updated_at))
ON CONFLICT (user_id, issue_id) DO UPDATE SET
    event_id       = EXCLUDED.event_id,
    updated_at     = EXCLUDED.updated_at,
    first_event_id = CASE WHEN inbox_items.read_at IS NULL THEN inbox_items.first_event_id ELSE EXCLUDED.event_id END,
    unread_count   = CASE WHEN inbox_items.read_at IS NULL THEN inbox_items.unread_count + 1 ELSE 1 END,
    reason         = CASE
        WHEN inbox_items.read_at IS NOT NULL THEN EXCLUDED.reason
        WHEN array_position(ARRAY['subscribed', 'mentioned', 'assigned'], EXCLUDED.reason)
             > array_position(ARRAY['subscribed', 'mentioned', 'assigned'], inbox_items.reason) THEN EXCLUDED.reason
        ELSE inbox_items.reason END,
    read_at        = NULL,
    archived_at    = NULL,
    snoozed_until  = NULL
WHERE inbox_items.event_id < EXCLUDED.event_id;

-- name: InboxPage :many
-- A user's inbox in a workspace, on issues of the given (visible)
-- projects, newest first, after the cursor. state: unread and all leave
-- snoozed rows out (all keeps read ones), snoozed lists them. Archived
-- rows are out.
SELECT sqlc.embed(n), p.key AS project_key, i.number, i.title, i.priority, i.status_id, s.name AS status_name, c.outcome AS status_outcome
FROM inbox_items n
JOIN issues i ON i.id = n.issue_id
JOIN projects p ON p.id = i.project_id
JOIN statuses s ON s.id = i.status_id
JOIN status_categories c ON c.id = s.category_id
WHERE n.user_id = sqlc.arg(user_id) AND n.workspace_id = sqlc.arg(workspace_id) AND n.archived_at IS NULL
  AND i.project_id = ANY(sqlc.arg(project_ids)::uuid[])
  AND CASE sqlc.arg(state)::text
        WHEN 'snoozed' THEN n.snoozed_until > sqlc.arg(now)::timestamptz
        ELSE (n.snoozed_until IS NULL OR n.snoozed_until <= sqlc.arg(now)::timestamptz)
             AND (sqlc.arg(state)::text = 'all' OR n.read_at IS NULL) END
  AND (sqlc.narg(before_at)::timestamptz IS NULL OR (n.updated_at, n.id) < (sqlc.narg(before_at)::timestamptz, sqlc.arg(before_id)::uuid))
ORDER BY n.updated_at DESC, n.id DESC
LIMIT sqlc.arg(max_rows);

-- name: UnreadCount :one
-- The user's unread rows in a workspace, on issues of the given projects.
SELECT count(*)::integer
FROM inbox_items n JOIN issues i ON i.id = n.issue_id
WHERE n.user_id = sqlc.arg(user_id) AND n.workspace_id = sqlc.arg(workspace_id) AND n.read_at IS NULL AND n.archived_at IS NULL
  AND (n.snoozed_until IS NULL OR n.snoozed_until <= sqlc.arg(now)::timestamptz)
  AND i.project_id = ANY(sqlc.arg(project_ids)::uuid[]);

-- name: InboxItem :one
SELECT * FROM inbox_items WHERE id = sqlc.arg(id) AND user_id = sqlc.arg(user_id);

-- name: UpdateInboxItem :one
-- Sets the row's state; first_event_id moves to the latest on read.
UPDATE inbox_items SET
    read_at        = sqlc.narg(read_at),
    first_event_id = CASE WHEN sqlc.narg(read_at)::timestamptz IS NOT NULL THEN event_id ELSE first_event_id END,
    unread_count   = CASE WHEN sqlc.narg(read_at)::timestamptz IS NOT NULL THEN 0 ELSE greatest(unread_count, 1) END,
    archived_at    = sqlc.narg(archived_at),
    snoozed_until  = sqlc.narg(snoozed_until)
WHERE id = sqlc.arg(id) AND user_id = sqlc.arg(user_id)
RETURNING *;

-- name: ReadAll :execrows
UPDATE inbox_items SET read_at = sqlc.arg(now)::timestamptz, first_event_id = event_id, unread_count = 0
WHERE user_id = sqlc.arg(user_id) AND workspace_id = sqlc.arg(workspace_id) AND read_at IS NULL AND archived_at IS NULL
  AND (snoozed_until IS NULL OR snoozed_until <= sqlc.arg(now)::timestamptz);

-- name: ArchiveRead :execrows
UPDATE inbox_items SET archived_at = sqlc.arg(now)::timestamptz
WHERE user_id = sqlc.arg(user_id) AND workspace_id = sqlc.arg(workspace_id) AND read_at IS NOT NULL AND archived_at IS NULL;

-- name: Settings :one
SELECT * FROM notification_settings WHERE user_id = sqlc.arg(user_id);

-- name: SaveSettings :one
INSERT INTO notification_settings (user_id, email, digest, updated_at)
VALUES (sqlc.arg(user_id), sqlc.arg(email), sqlc.arg(digest), sqlc.arg(updated_at))
ON CONFLICT (user_id) DO UPDATE SET email = EXCLUDED.email, digest = EXCLUDED.digest, updated_at = EXCLUDED.updated_at
RETURNING *;

-- name: DueEmailUsers :many
-- People with unread rows an email hasn't told of, quiet for the delay,
-- who take email.
SELECT DISTINCT i.user_id
FROM inbox_items i
JOIN users u ON u.id = i.user_id AND u.deactivated_at IS NULL
LEFT JOIN notification_settings s ON s.user_id = i.user_id
WHERE i.read_at IS NULL AND i.archived_at IS NULL AND i.emailed_event_id IS DISTINCT FROM i.event_id
  AND i.updated_at <= sqlc.arg(quiet_since)::timestamptz AND (i.snoozed_until IS NULL OR i.snoozed_until <= sqlc.arg(now)::timestamptz)
  AND coalesce(s.email, 'unread') = 'unread'
LIMIT sqlc.arg(max_rows);

-- name: ClaimDueEmails :many
-- A user's rows due for an email, locked until the claim's transaction
-- ends; another worker skips them.
SELECT sqlc.embed(i), w.slug AS workspace_slug, p.key AS project_key, x.number, x.title
FROM inbox_items i JOIN workspaces w ON w.id = i.workspace_id JOIN issues x ON x.id = i.issue_id JOIN projects p ON p.id = x.project_id
WHERE i.user_id = sqlc.arg(user_id) AND i.read_at IS NULL AND i.archived_at IS NULL
  AND i.emailed_event_id IS DISTINCT FROM i.event_id
  AND i.updated_at <= sqlc.arg(quiet_since)::timestamptz AND (i.snoozed_until IS NULL OR i.snoozed_until <= sqlc.arg(now)::timestamptz)
ORDER BY i.updated_at DESC
FOR UPDATE OF i SKIP LOCKED;

-- name: MarkEmailed :exec
UPDATE inbox_items SET emailed_event_id = event_id WHERE id = ANY(sqlc.arg(ids)::uuid[]);

-- name: DigestUsers :many
-- People who take a daily digest and have not had one since the given
-- time; the caller checks each one's local morning.
SELECT u.id, u.timezone, s.digest_sent_at
FROM notification_settings s JOIN users u ON u.id = s.user_id AND u.deactivated_at IS NULL
WHERE s.digest AND (s.digest_sent_at IS NULL OR s.digest_sent_at < sqlc.arg(sent_before)::timestamptz);

-- name: ClaimDigest :one
-- Takes the user's digest for this morning; false when another worker
-- took it (or it went out already).
UPDATE notification_settings SET digest_sent_at = sqlc.arg(now)::timestamptz
WHERE user_id = sqlc.arg(user_id) AND digest AND (digest_sent_at IS NULL OR digest_sent_at < sqlc.arg(sent_before)::timestamptz)
RETURNING user_id;

-- name: DigestItems :many
-- The user's unread rows, every workspace, newest first.
SELECT sqlc.embed(i), w.slug AS workspace_slug, p.key AS project_key, x.number, x.title
FROM inbox_items i JOIN workspaces w ON w.id = i.workspace_id JOIN issues x ON x.id = i.issue_id JOIN projects p ON p.id = x.project_id
WHERE i.user_id = sqlc.arg(user_id) AND i.read_at IS NULL AND i.archived_at IS NULL
  AND (i.snoozed_until IS NULL OR i.snoozed_until <= sqlc.arg(now)::timestamptz)
ORDER BY i.updated_at DESC
LIMIT 50;
