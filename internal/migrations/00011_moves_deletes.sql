-- +goose Up
-- Issue moves and hard deletes (docs/domain.md#deletion, ADR-0012).

-- Keys an issue left behind when it moved to another project: the old
-- project and number keep resolving to it. Former project keys resolve to
-- the project first (project_key_aliases), then here.
CREATE TABLE issue_key_aliases (
    workspace_id uuid NOT NULL REFERENCES workspaces (id) ON DELETE CASCADE,
    project_id   uuid NOT NULL REFERENCES projects (id) ON DELETE CASCADE,
    number       bigint NOT NULL,
    issue_id     uuid NOT NULL REFERENCES issues (id) ON DELETE CASCADE,
    created_at   timestamptz NOT NULL,
    PRIMARY KEY (project_id, number)
);

CREATE INDEX issue_key_aliases_issue ON issue_key_aliases (issue_id);

-- JetStream subjects (and later object storage keys) left to purge after a
-- hard delete, for the worker's purge job. tombstone_id is the event kept
-- on the subject: the job waits until the relay published it, then purges
-- everything before it.
CREATE TABLE purges (
    id             uuid PRIMARY KEY,
    workspace_id   uuid,
    aggregate_type text NOT NULL,
    aggregate_id   uuid NOT NULL,
    tombstone_id   uuid,
    objects        text[] NOT NULL DEFAULT '{}',
    created_at     timestamptz NOT NULL,
    attempts       integer NOT NULL DEFAULT 0
);

-- +goose Down
DROP TABLE purges;
DROP TABLE issue_key_aliases;
