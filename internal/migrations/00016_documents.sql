-- +goose Up
-- Block documents (docs/realtime.md#document-model). An issue's description
-- is a document of its own, versioned for collaborative editing; issues
-- keep its plain-text projection in description, for search and API
-- clients. Comments are single-author documents stored with them, body
-- keeping their text. Rows from before are converted when read.

CREATE TABLE documents (
    workspace_id uuid NOT NULL REFERENCES workspaces (id) ON DELETE CASCADE,
    id           uuid PRIMARY KEY,
    issue_id     uuid NOT NULL UNIQUE REFERENCES issues (id) ON DELETE CASCADE,
    version      bigint NOT NULL DEFAULT 0,
    content      jsonb NOT NULL,
    updated_at   timestamptz NOT NULL
);

ALTER TABLE comments ADD COLUMN body_doc jsonb;
-- A comment may be an image alone: its text can be empty.
ALTER TABLE comments DROP CONSTRAINT comments_body_length;
ALTER TABLE comments ADD CONSTRAINT comments_body_length CHECK (char_length(body) <= 1048576);

-- +goose Down
ALTER TABLE comments DROP CONSTRAINT comments_body_length;
ALTER TABLE comments ADD CONSTRAINT comments_body_length CHECK (char_length(body) BETWEEN 1 AND 20000);
ALTER TABLE comments DROP COLUMN body_doc;
DROP TABLE documents;
