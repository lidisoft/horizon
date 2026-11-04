-- name: Webhooks :many
SELECT * FROM webhooks WHERE workspace_id = sqlc.arg(workspace_id) ORDER BY created_at, id;

-- name: Webhook :one
SELECT * FROM webhooks WHERE workspace_id = sqlc.arg(workspace_id) AND id = sqlc.arg(id);

-- name: LockWebhook :one
SELECT * FROM webhooks WHERE workspace_id = sqlc.arg(workspace_id) AND id = sqlc.arg(id) FOR UPDATE;

-- name: CreateWebhook :one
INSERT INTO webhooks (workspace_id, id, project_id, name, url, secret, event_types, active, created_by, created_at, updated_at)
VALUES (sqlc.arg(workspace_id), sqlc.arg(id), sqlc.narg(project_id), sqlc.arg(name), sqlc.arg(url), sqlc.arg(secret),
        sqlc.arg(event_types), sqlc.arg(active), sqlc.narg(created_by), sqlc.arg(created_at), sqlc.arg(created_at))
RETURNING *;

-- name: UpdateWebhook :one
UPDATE webhooks SET project_id = sqlc.narg(project_id), name = sqlc.arg(name), url = sqlc.arg(url), secret = sqlc.arg(secret),
    event_types = sqlc.arg(event_types), active = sqlc.arg(active), updated_at = sqlc.arg(updated_at), version = version + 1
WHERE id = sqlc.arg(id)
RETURNING *;

-- name: DeleteWebhook :exec
DELETE FROM webhooks WHERE id = sqlc.arg(id);

-- name: ActiveWebhooks :many
-- The endpoints that may want a workspace's event.
SELECT * FROM webhooks WHERE workspace_id = sqlc.arg(workspace_id) AND active;

-- name: CreateDelivery :execrows
-- Queues an event for an endpoint once (a redelivery names its original).
INSERT INTO webhook_deliveries (workspace_id, id, webhook_id, event_id, event_type, body, redelivery_of, created_at, next_attempt_at)
VALUES (sqlc.arg(workspace_id), sqlc.arg(id), sqlc.arg(webhook_id), sqlc.arg(event_id), sqlc.arg(event_type), sqlc.arg(body),
        sqlc.narg(redelivery_of), sqlc.arg(created_at), sqlc.arg(created_at))
ON CONFLICT DO NOTHING;

-- name: LeaseDeliveries :many
-- Takes due deliveries of active endpoints, oldest first, for a while:
-- their next attempt moves to lease_until, so another worker takes them
-- only if this one dies before recording the attempt.
WITH due AS (
    SELECT d.id FROM webhook_deliveries d JOIN webhooks w ON w.id = d.webhook_id AND w.active
    WHERE d.state = 'pending' AND d.next_attempt_at <= sqlc.arg(now)::timestamptz
    ORDER BY d.next_attempt_at, d.id
    LIMIT sqlc.arg(max_rows)
    FOR UPDATE OF d SKIP LOCKED
)
UPDATE webhook_deliveries d SET next_attempt_at = sqlc.arg(lease_until)::timestamptz
FROM due, webhooks w
WHERE d.id = due.id AND w.id = d.webhook_id
RETURNING d.id, d.webhook_id, d.event_type, d.body, d.attempts, d.created_at, w.url, w.secret;

-- name: RecordAttempt :exec
UPDATE webhook_deliveries SET state = sqlc.arg(state), attempts = attempts + 1, status_code = sqlc.narg(status_code),
    response_excerpt = sqlc.arg(response_excerpt), error = sqlc.arg(error), duration_ms = sqlc.arg(duration_ms),
    next_attempt_at = sqlc.narg(next_attempt_at), delivered_at = sqlc.narg(delivered_at)
WHERE id = sqlc.arg(id);

-- name: Deliveries :many
-- An endpoint's deliveries, newest first, after the cursor.
SELECT * FROM webhook_deliveries
WHERE webhook_id = sqlc.arg(webhook_id)
  AND (sqlc.narg(before_at)::timestamptz IS NULL OR (created_at, id) < (sqlc.narg(before_at)::timestamptz, sqlc.arg(before_id)::uuid))
ORDER BY created_at DESC, id DESC
LIMIT sqlc.arg(max_rows);

-- name: Delivery :one
SELECT * FROM webhook_deliveries WHERE webhook_id = sqlc.arg(webhook_id) AND id = sqlc.arg(id);

-- name: PurgeDeliveries :execrows
DELETE FROM webhook_deliveries WHERE created_at < sqlc.arg(before)::timestamptz AND state <> 'pending';

-- name: SealedSecrets :many
-- Every endpoint's sealed secret, for rekeying.
SELECT id, secret FROM webhooks ORDER BY id;

-- name: SetSecret :exec
UPDATE webhooks SET secret = sqlc.arg(secret) WHERE id = sqlc.arg(id);
