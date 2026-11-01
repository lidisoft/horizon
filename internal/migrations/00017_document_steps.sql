-- +goose Up
-- The step protocol (docs/realtime.md#protocol). A document's version
-- counts the steps applied to it; content is a snapshot taken at
-- snapshot_version, and the batches since then are in document_steps,
-- keyed by the version they were made against: the unique key serialises
-- concurrent writers across replicas.

ALTER TABLE documents ADD COLUMN snapshot_version bigint NOT NULL DEFAULT 0;
UPDATE documents SET snapshot_version = version;

CREATE TABLE document_steps (
    document_id uuid NOT NULL REFERENCES documents (id) ON DELETE CASCADE,
    version     bigint NOT NULL, -- the version the batch applies to
    steps       jsonb NOT NULL,
    client_id   text NOT NULL,
    user_id     uuid REFERENCES users (id) ON DELETE SET NULL,
    created_at  timestamptz NOT NULL,
    PRIMARY KEY (document_id, version)
);

-- +goose Down
DROP TABLE document_steps;
ALTER TABLE documents DROP COLUMN snapshot_version;
