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

-- name: DeleteAggregateEvents :exec
-- Scrubs the history of hard-deleted aggregates (ADR-0012).
DELETE FROM events WHERE aggregate_type = sqlc.arg(aggregate_type) AND aggregate_id = ANY(sqlc.arg(ids)::uuid[]);

-- name: QueuePurge :exec
INSERT INTO purges (id, workspace_id, aggregate_type, aggregate_id, tombstone_id, objects, created_at)
VALUES ($1, $2, $3, $4, $5, $6, $7);

-- name: ClaimPurges :many
-- Purges whose tombstone (if any) the relay has published; other workers
-- skip the locked ones.
SELECT p.*, coalesce((SELECT e.type FROM events e WHERE e.id = p.tombstone_id), '')::text AS tombstone_type
FROM purges p
WHERE p.tombstone_id IS NULL
   OR NOT EXISTS (SELECT 1 FROM events e WHERE e.id = p.tombstone_id AND e.published_at IS NULL)
ORDER BY p.id
LIMIT sqlc.arg(batch_size)
FOR UPDATE SKIP LOCKED;

-- name: DonePurge :exec
DELETE FROM purges WHERE id = $1;

-- name: FailedPurge :exec
UPDATE purges SET attempts = attempts + 1 WHERE id = $1;

-- name: AggregateEventsBefore :many
-- A page of one aggregate's history, newest first, before cursor (an event id).
SELECT *
FROM events
WHERE workspace_id = sqlc.arg(workspace_id) AND aggregate_type = sqlc.arg(aggregate_type) AND aggregate_id = sqlc.arg(aggregate_id)
  AND (sqlc.narg(before)::uuid IS NULL OR id < sqlc.narg(before)::uuid)
ORDER BY id DESC
LIMIT sqlc.arg(max_rows);

-- name: WorkspaceEventsAfter :many
-- A workspace's events after one, oldest first, for socket replays.
SELECT *
FROM events
WHERE workspace_id = sqlc.arg(workspace_id) AND id > sqlc.arg(after)
ORDER BY id
LIMIT sqlc.arg(max_rows);
