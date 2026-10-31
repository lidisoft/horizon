-- +goose Up
-- Domain event log and transactional outbox (ADR-0004, docs/architecture.md).
-- Rows are written in the same transaction as the state they describe; the
-- worker's outbox relay publishes them to JetStream and sets published_at.
CREATE TABLE events (
    id                uuid        PRIMARY KEY,           -- UUIDv7, time-ordered
    workspace_id      uuid,                              -- NULL for instance-level events
    aggregate_type    text        NOT NULL,
    aggregate_id      uuid        NOT NULL,
    aggregate_version bigint      NOT NULL,
    type              text        NOT NULL,
    actor_id          uuid,                              -- NULL for system actions
    occurred_at       timestamptz NOT NULL DEFAULT now(),
    payload           jsonb       NOT NULL DEFAULT '{}',
    published_at      timestamptz,
    -- Subject tokens (docs/architecture.md#domain-events-and-the-outbox).
    CONSTRAINT events_aggregate_type_token CHECK (aggregate_type ~ '^[a-z][a-z0-9_]*$'),
    CONSTRAINT events_type_token CHECK (type ~ '^[A-Z][A-Za-z0-9]*$')
);

-- Activity history of one aggregate.
CREATE INDEX events_aggregate ON events (workspace_id, aggregate_type, aggregate_id, aggregate_version);

-- The relay's work queue.
CREATE INDEX events_unpublished ON events (id) WHERE published_at IS NULL;

-- Wake the relay when a transaction that appended events commits. NOTIFY is
-- delivered at commit, and once per statement is enough: the relay drains the
-- whole queue on every wake-up (and also polls).
-- +goose StatementBegin
CREATE FUNCTION events_notify() RETURNS trigger LANGUAGE plpgsql AS $$
BEGIN
    PERFORM pg_notify('events', '');
    RETURN NULL;
END;
$$;
-- +goose StatementEnd

CREATE TRIGGER events_notify AFTER INSERT ON events
    FOR EACH STATEMENT EXECUTE FUNCTION events_notify();

-- +goose Down
DROP TRIGGER events_notify ON events;
DROP FUNCTION events_notify();
DROP TABLE events;
