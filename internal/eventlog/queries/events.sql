-- name: AppendEvents :copyfrom
INSERT INTO events (
    id, workspace_id, aggregate_type, aggregate_id, aggregate_version,
    type, actor_id, occurred_at, payload
) VALUES (
    $1, $2, $3, $4, $5, $6, $7, $8, $9
);

-- name: ClaimUnpublished :many
-- Locks a batch for this relay; other relays skip it.
SELECT id, workspace_id, aggregate_type, aggregate_id, aggregate_version,
       type, actor_id, occurred_at, payload
FROM events
WHERE published_at IS NULL
ORDER BY id
LIMIT sqlc.arg(batch_size)
FOR UPDATE SKIP LOCKED;

-- name: MarkPublished :exec
UPDATE events SET published_at = now() WHERE id = ANY(sqlc.arg(ids)::uuid[]);

-- name: OutboxBacklog :one
-- Unpublished events and the age of the oldest, for the outbox lag metrics.
SELECT count(*)::bigint AS pending,
       coalesce(extract(epoch FROM now() - min(occurred_at)), 0)::float8 AS oldest_seconds
FROM events
WHERE published_at IS NULL;

-- name: AggregateEvents :many
-- History of one aggregate, oldest first.
SELECT *
FROM events
WHERE aggregate_type = sqlc.arg(aggregate_type) AND aggregate_id = sqlc.arg(aggregate_id)
ORDER BY aggregate_version, id;
