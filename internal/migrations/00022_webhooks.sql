-- +goose Up
-- Outbound webhooks (docs/api.md#webhooks): endpoints of a workspace or a
-- project, and one delivery per endpoint and event, retried until it
-- succeeds or a day has passed. Redeliveries are rows of their own.

CREATE TABLE webhooks (
    workspace_id uuid NOT NULL REFERENCES workspaces (id) ON DELETE CASCADE,
    id           uuid PRIMARY KEY,
    project_id   uuid REFERENCES projects (id) ON DELETE CASCADE, -- NULL: every project
    name         text NOT NULL,
    url          text NOT NULL,
    secret       text NOT NULL,                  -- sealed with SECRET_KEY (internal/platform/secrets)
    event_types  text[] NOT NULL DEFAULT '{}',   -- public types or "<aggregate>.*"; empty: every event
    active       boolean NOT NULL DEFAULT true,
    created_by   uuid REFERENCES users (id) ON DELETE SET NULL,
    created_at   timestamptz NOT NULL,
    updated_at   timestamptz NOT NULL,
    version      integer NOT NULL DEFAULT 1,
    CONSTRAINT webhooks_name_length CHECK (char_length(name) BETWEEN 1 AND 100),
    CONSTRAINT webhooks_url_length CHECK (char_length(url) BETWEEN 1 AND 2000)
);

CREATE INDEX webhooks_workspace ON webhooks (workspace_id) WHERE active;

CREATE TABLE webhook_deliveries (
    workspace_id     uuid NOT NULL REFERENCES workspaces (id) ON DELETE CASCADE,
    id               uuid PRIMARY KEY,
    webhook_id       uuid NOT NULL REFERENCES webhooks (id) ON DELETE CASCADE,
    event_id         uuid NOT NULL, -- events may be scrubbed: no key
    event_type       text NOT NULL,
    body             jsonb NOT NULL, -- what is POSTed, kept for redelivery
    redelivery_of    uuid REFERENCES webhook_deliveries (id) ON DELETE SET NULL,
    state            text NOT NULL DEFAULT 'pending',
    attempts         integer NOT NULL DEFAULT 0,
    status_code      integer,
    response_excerpt text NOT NULL DEFAULT '',
    error            text NOT NULL DEFAULT '',
    duration_ms      integer,
    created_at       timestamptz NOT NULL,
    next_attempt_at  timestamptz,
    delivered_at     timestamptz,
    CONSTRAINT webhook_deliveries_state CHECK (state IN ('pending', 'succeeded', 'failed'))
);

-- One automatic delivery per endpoint and event: the consumer may see an
-- event twice.
CREATE UNIQUE INDEX webhook_deliveries_once ON webhook_deliveries (webhook_id, event_id) WHERE redelivery_of IS NULL;
CREATE INDEX webhook_deliveries_log ON webhook_deliveries (webhook_id, created_at DESC, id DESC);
CREATE INDEX webhook_deliveries_due ON webhook_deliveries (next_attempt_at) WHERE state = 'pending';

-- +goose Down
DROP TABLE webhook_deliveries;
DROP TABLE webhooks;
